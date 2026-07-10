//go:build linux

package tunnel

import (
	"fmt"
	"os/exec"
	"strings"
)

// parseRouteGet extracts the gateway and interface from `ip route get <dest>`
// output. The output looks like:
//
//	8.8.8.8 via 192.168.1.1 dev eth0 src 192.168.1.100 uid 1000
//	    cache
//
// Or interface-direct:
//
//	10.0.0.1 dev tun0 src 10.0.0.2 uid 1000
//	    cache
func parseRouteGet(output string) (gateway, iface string) {
	// Take the first non-"cache" line.
	lines := strings.Split(output, "\n")
	var first string
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l != "" && l != "cache" {
			first = l
			break
		}
	}
	if first == "" {
		return "", ""
	}

	fields := strings.Fields(first)
	for i, f := range fields {
		switch f {
		case "via":
			if i+1 < len(fields) {
				gateway = fields[i+1]
			}
		case "dev":
			if i+1 < len(fields) {
				iface = fields[i+1]
			}
		}
	}
	return gateway, iface
}

// nextHop reports how the system currently routes to dest, so the proxy
// server's traffic can be pinned to that path while everything else is
// redirected into the tunnel.
func nextHop(dest string) (gateway, iface string, err error) {
	out, err := exec.Command("ip", "route", "get", dest).Output()
	if err != nil {
		return "", "", fmt.Errorf("ip route get %s: %w", dest, err)
	}
	gateway, iface = parseRouteGet(string(out))
	if gateway == "" && iface == "" {
		return "", "", fmt.Errorf("no route to %s", dest)
	}
	return gateway, iface, nil
}

// run executes a command, wrapping failures with the command line for context.
func run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}
