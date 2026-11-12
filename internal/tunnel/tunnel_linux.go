//go:build linux

package tunnel

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/xjasonlyu/tun2socks/v2/engine"

	"github.com/aimuzov/happ-cli/internal/firewall"
)

const (
	happTable = "100" // dedicated routing table for happ-cli routes

	// tunnelRulePref is the explicit priority of the "send everything to table
	// 100" rule. It sits after the LAN-preservation rules (90..) and before the
	// kernel's main table (32766). Being explicit makes duplicate installs
	// impossible and keeps teardown exact.
	tunnelRulePref = "32760"
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

	// Find the default route's gateway and interface so the gateway itself can
	// be pinned to it in table 100 (see below).
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

	// Keep the router's own LAN reachable. Directly connected subnets (and
	// link-local/multicast) get a policy rule to the MAIN table with a priority
	// far above the tunnel rule. Replies to LAN clients, SSH to the router and
	// its management UI therefore stay on the physical interface even when the
	// tunnel is down or still coming up. Unlike the old approach, these
	// prefixes are NOT copied into table 100 via the WAN gateway: that sent LAN
	// traffic to the ISP instead of the LAN interface and killed LAN↔LAN.
	connected, connErr := connectedIPv4()
	if connErr != nil {
		fmt.Printf("warning: could not list directly connected subnets (%v); preserving only link-local and multicast\n", connErr)
	}
	for i, cidr := range lanCIDRs(connected, opts.TunName) {
		args := lanRuleArgs(cidr, i)
		// Remove a leftover rule first so re-running is idempotent.
		_ = run("ip", append([]string{"rule", "del"}, args...)...)
		if err := run("ip", append([]string{"rule", "add"}, args...)...); err != nil {
			return rollback(fmt.Errorf("preserve LAN route %s: %w", cidr, err))
		}
		delArgs := append([]string{"ip", "rule", "del"}, args...)
		t.teardown = append(t.teardown, func() { _ = run(delArgs[0], delArgs[1:]...) })
	}

	// Use a dedicated routing table so happ routes never conflict with
	// other software (WireGuard, OpenVPN, system routes). The explicit pref
	// keeps this after the LAN rules and makes duplicate installs impossible.
	if err := run("ip", "rule", "add", "from", "all", "table", happTable, "pref", tunnelRulePref); err != nil {
		return rollback(fmt.Errorf("add routing rule: %w", err))
	}
	t.teardown = append(t.teardown, func() {
		_ = run("ip", "rule", "del", "from", "all", "table", happTable, "pref", tunnelRulePref)
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

	// The default gateway itself must stay reachable from table 100.
	if defGW != "" && defIface != "" {
		_ = run("ip", "route", "add", defGW+"/32", "dev", defIface, "table", happTable)
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
// crash (SIGKILL). It handles state left by both this version (explicit tunnel
// pref + LAN rules at 90..97) and older versions (un-prioritised rule and
// local-subnet routes copied into table 100).
func cleanupStaleTun(iface string) {
	// Older versions installed "from all table 100" with no pref; there may be
	// more than one. Delete until it fails, with a bounded loop.
	for i := 0; i < 8; i++ {
		if err := run("ip", "rule", "del", "from", "all", "table", happTable); err != nil {
			break
		}
	}
	// LAN-preservation rules live at pref 90..97 (max connected subnets + two
	// constants is capped by the loop below safely above the real count).
	for pref := lanRulePrefBase; pref < lanRulePrefBase+8; pref++ {
		for i := 0; i < 8; i++ {
			if err := run("ip", "rule", "del", "pref", strconv.Itoa(pref)); err != nil {
				break
			}
		}
	}
	_ = run("ip", "route", "flush", "table", happTable)
	_ = run("ip", "link", "del", iface)
}

// connectedIPv4 lists the directly connected IPv4 subnets (proto kernel), which
// are the prefixes that must keep using the main table while the tunnel is up.
func connectedIPv4() ([]connectedRoute, error) {
	out, err := runOutput("ip", "-4", "route", "show", "proto", "kernel")
	if err != nil {
		return nil, err
	}
	return parseConnectedRoutes(out), nil
}

// runOutput executes a command and returns its stdout, wrapping failures with
// the command line for context.
func runOutput(name string, args ...string) (string, error) {
	out, err := exec.Command(name, args...).Output()
	if err != nil {
		return "", fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
	}
	return string(out), nil
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
