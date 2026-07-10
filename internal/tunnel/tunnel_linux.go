//go:build linux

package tunnel

import (
	"fmt"
	"os"
	"time"

	"github.com/xjasonlyu/tun2socks/v2/engine"
)

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

	// Clean up any stale TUN device from a previous crash (SIGKILL).
	_ = run("ip", "link", "del", opts.TunName)

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
		fmt.Println("TUN device created; routing left unchanged (--no-routes).")
		return t, nil
	}

	// Pin each server IP to its current next hop (bypass the tunnel).
	for _, h := range hops {
		_ = run("ip", "route", "del", h.ip+"/32")
		var addErr error
		if h.gateway != "" {
			addErr = run("ip", "route", "add", h.ip+"/32", "via", h.gateway)
		} else {
			addErr = run("ip", "route", "add", h.ip+"/32", "dev", h.iface)
		}
		if addErr != nil {
			return rollback(fmt.Errorf("pin server route %s: %w", h.ip, addErr))
		}
		ip := h.ip
		t.teardown = append(t.teardown, func() { _ = run("ip", "route", "del", ip+"/32") })
	}

	// Preserve local subnets — route them through the physical interface so
	// LAN, management IP, and multicast stay reachable.
	if defIface != "" {
		for _, net := range localNets {
			args := []string{"ip", "route", "add", net}
			if defGW != "" {
				args = append(args, "via", defGW)
			} else {
				args = append(args, "dev", defIface)
			}
			if err := run(args[0], args[1:]...); err != nil {
				fmt.Printf("warning: could not preserve local net %s: %v\n", net, err)
				continue
			}
			net := net
			t.teardown = append(t.teardown, func() { _ = run("ip", "route", "del", net) })
		}

		// Preserve a direct route to the default gateway so the router's
		// management interface stays reachable.
		if defGW != "" {
			gwNet := defGW + "/32"
			if err := run("ip", "route", "add", gwNet, "dev", defIface); err == nil {
				t.teardown = append(t.teardown, func() { _ = run("ip", "route", "del", gwNet) })
			}
		}
	}

	// Override the default route with two /1 routes scoped to the TUN device.
	for _, cidr := range []string{"0.0.0.0/1", "128.0.0.0/1"} {
		if err := run("ip", "route", "add", cidr, "dev", opts.TunName); err != nil {
			return rollback(fmt.Errorf("install default override %s: %w", cidr, err))
		}
		cidr := cidr
		t.teardown = append(t.teardown, func() { _ = run("ip", "route", "del", cidr) })
	}

	// Block global IPv6 by routing it to loopback. Best-effort: warn.
	for _, cidr := range []string{"::/1", "8000::/1"} {
		if err := run("ip", "-6", "route", "add", cidr, "dev", "lo"); err != nil {
			fmt.Printf("warning: could not block IPv6 %s (possible IPv6 leak): %v\n", cidr, err)
			continue
		}
		cidr := cidr
		t.teardown = append(t.teardown, func() { _ = run("ip", "-6", "route", "del", cidr) })
	}

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
