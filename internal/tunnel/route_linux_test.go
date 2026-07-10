//go:build linux

package tunnel

import "testing"

func TestParseRouteGetWithGateway(t *testing.T) {
	output := `8.8.8.8 via 192.168.1.1 dev eth0 src 192.168.1.100 uid 1000
    cache
`
	gw, iface := parseRouteGet(output)
	if gw != "192.168.1.1" {
		t.Errorf("gateway = %q, want 192.168.1.1", gw)
	}
	if iface != "eth0" {
		t.Errorf("interface = %q, want eth0", iface)
	}
}

func TestParseRouteGetInterfaceOnly(t *testing.T) {
	output := `10.0.0.1 dev tun0 src 10.0.0.2 uid 1000
    cache
`
	gw, iface := parseRouteGet(output)
	if gw != "" {
		t.Errorf("gateway = %q, want empty", gw)
	}
	if iface != "tun0" {
		t.Errorf("interface = %q, want tun0", iface)
	}
}

func TestParseRouteGetUnreachable(t *testing.T) {
	output := `RTNETLINK answers: Network is unreachable
`
	gw, iface := parseRouteGet(output)
	if gw != "" || iface != "" {
		t.Errorf("both should be empty for unreachable, got gateway=%q iface=%q", gw, iface)
	}
}

func TestParseRouteGetLocal(t *testing.T) {
	// A route to a local address goes via lo, no gateway.
	output := `local 127.0.0.1 dev lo src 127.0.0.1 uid 1000
    cache <local>
`
	gw, iface := parseRouteGet(output)
	if gw != "" {
		t.Errorf("gateway should be empty for local, got %q", gw)
	}
	if iface != "lo" {
		t.Errorf("interface = %q, want lo", iface)
	}
}

func TestParseRouteGetEmpty(t *testing.T) {
	gw, iface := parseRouteGet("")
	if gw != "" || iface != "" {
		t.Errorf("both should be empty for empty input, got gateway=%q iface=%q", gw, iface)
	}
}

func TestParseRouteGetCacheOnly(t *testing.T) {
	output := `    cache
`
	gw, iface := parseRouteGet(output)
	if gw != "" || iface != "" {
		t.Errorf("both should be empty for cache-only, got gateway=%q iface=%q", gw, iface)
	}
}
