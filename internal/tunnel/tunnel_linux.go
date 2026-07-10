//go:build linux

package tunnel

import (
	"fmt"
	"os"
	"time"

	"github.com/xjasonlyu/tun2socks/v2/engine"

	"github.com/aimuzov/happ-cli/internal/firewall"
)

const happTable = "100" // dedicated routing table for happ-cli routes

// Tunnel is a running TUN tunnel and the routing changes it installed.
type Tunnel struct {
	teardown []func()
}

// serverHop records how to reach a proxy server IP around the tunnel.
type serverHop struct {
	ip      string
	gateway string // empty for an interface-scoped route
	iface   string
}

// Start creates the TUN device, points all traffic at the SOCKS proxy via
// tun2socks, and installs routing changes. Requires root or CAP_NET_ADMIN.
// Set SkipRoutes to only create the device without touching the routing table.
func Start(opts Options) (*Tunnel, error) {
	opts.withDefaults()

	if len(opts.ServerIPs) == 0 {
		return nil, fmt.Errorf("TUN mode requires the resolved server IP(s) to pin around the tunnel")
	}

	// Discover the current next hop to each server IP before touching routes.
	hops := make([]serverHop, 0, len(opts.ServerIPs))
	for _, ip := range opts.ServerIPs {
		gw, iface, err := nextHop(ip)
		if err != nil {
			return nil, fmt.Errorf("find route to server %s: %w", ip, err)
		}
		hops = append(hops, serverHop{ip: ip, gateway: gw, iface: iface})
	}

	// Find the default route's gateway and interface for LAN preservation.
	defGW, defIface, _ := nextHop("1.1.1.1")

	// Clean up any stale state from a previous crash (SIGKILL).
	cleanupStaleTun(opts.TunName)

	key := &engine.Key{
		Proxy:      "socks5://" + opts.SocksAddr,
		Device:     "tun://" + opts.TunName,
		LogLevel:   opts.LogLevel,
		MTU:        opts.MTU,
		UDPTimeout: 30 * time.Second,
	}
	engine.Insert(key)
	engine.Start()

	t := &Tunnel{}
	t.teardown = append(t.teardown, func() { engine.Stop() })

	rollback := func(cause error) (*Tunnel, error) {
		t.Close()
		return nil, cause
	}

	// Bring the TUN interface up with a point-to-point address.
	if err := run("ip", "link", "set", opts.TunName, "up"); err != nil {
		return rollback(fmt.Errorf("bring up %s: %w", opts.TunName, err))
	}

	// Disable reverse path filtering on the TUN device. On routers,
	// rp_filter=1 (default) drops packets arriving on an interface that
	// doesn't match the routing table's expected incoming interface.
	// Try sysctl first; fall back to /proc/sys for minimal systems.
	disableRPFilter(opts.TunName)
	if err := run("ip", "addr", "add", opts.TunIP+"/30", "dev", opts.TunName); err != nil {
		return rollback(fmt.Errorf("assign address to %s: %w", opts.TunName, err))
	}

	if opts.SkipRoutes {
		fmt.Println("TUN device created; routing left unchanged (--no-routing).")
		return t, nil
	}

	// Use a dedicated routing table so happ routes never conflict with
	// other software (WireGuard, OpenVPN, system routes).
	if err := run("ip", "rule", "add", "from", "all", "table", happTable); err != nil {
		return rollback(fmt.Errorf("add routing rule: %w", err))
	}
	t.teardown = append(t.teardown, func() {
		_ = run("ip", "rule", "del", "from", "all", "table", happTable)
	})

	// Allow forwarded traffic through the TUN device.
	if !opts.NoFirewall {
		cleanupFW := firewall.Rules(opts.TunName)
		if cleanupFW != nil {
			t.teardown = append(t.teardown, cleanupFW)
		}
	}

	// Pin each server IP to its current next hop (bypass the tunnel).
	for _, h := range hops {
		var addErr error
		if h.gateway != "" {
			addErr = run("ip", "route", "add", h.ip+"/32", "via", h.gateway, "table", happTable)
		} else {
			addErr = run("ip", "route", "add", h.ip+"/32", "dev", h.iface, "table", happTable)
		}
		if addErr != nil {
			return rollback(fmt.Errorf("pin server route %s: %w", h.ip, addErr))
		}
	}

	// Preserve local subnets in table 100 so LAN stays reachable.
	if defIface != "" {
		for _, net := range localNets {
			args := []string{"ip", "route", "add", net, "table", happTable}
			if defGW != "" {
				args = append(args, "via", defGW)
			} else {
				args = append(args, "dev", defIface)
			}
			_ = run(args[0], args[1:]...) // best-effort
		}
		if defGW != "" {
			_ = run("ip", "route", "add", defGW+"/32", "dev", defIface, "table", happTable)
		}
	}

	// Override the default route with two /1 routes scoped to the TUN device.
	for _, cidr := range []string{"0.0.0.0/1", "128.0.0.0/1"} {
		if err := run("ip", "route", "add", cidr, "dev", opts.TunName, "table", happTable); err != nil {
			return rollback(fmt.Errorf("install default override %s: %w", cidr, err))
		}
	}

	// Block global IPv6 in table 100.
	for _, cidr := range []string{"::/1", "8000::/1"} {
		_ = run("ip", "-6", "route", "add", cidr, "dev", "lo", "table", happTable)
	}

	// Cleanup: flush the entire table.
	t.teardown = append(t.teardown, func() { _ = run("ip", "route", "flush", "table", happTable) })

	return t, nil
}

// Close stops the tunnel and reverses the routing changes in reverse order.
func (t *Tunnel) Close() error {
	if t == nil {
		return nil
	}
	for i := len(t.teardown) - 1; i >= 0; i-- {
		t.teardown[i]()
	}
	t.teardown = nil
	return nil
}

// cleanupStaleTun removes leftover happ routes and TUN device from a previous
// crash (SIGKILL). Only touches table 100 — other routes are safe.
func cleanupStaleTun(iface string) {
	_ = run("ip", "route", "flush", "table", happTable)
	_ = run("ip", "rule", "del", "from", "all", "table", happTable)
	_ = run("ip", "link", "del", iface)
}

// disableRPFilter sets rp_filter=0 for the given interface. Tries sysctl first,
// then falls back to writing /proc/sys for minimal embedded systems.
func disableRPFilter(iface string) {
	// Try sysctl (available on most distributions).
	if err := run("sysctl", "-w", "net.ipv4.conf."+iface+".rp_filter=0"); err == nil {
		return
	}
	// Fallback: write to procfs (always available on Linux).
	path := "/proc/sys/net/ipv4/conf/" + iface + "/rp_filter"
	if err := os.WriteFile(path, []byte("0\n"), 0o644); err != nil {
		fmt.Printf("warning: could not disable rp_filter for %s: %v\n", iface, err)
	}
}
