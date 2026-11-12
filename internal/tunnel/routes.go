package tunnel

import (
	"strconv"
	"strings"
)

// lanRulePrefBase is the first `ip rule` priority used for "keep this LAN
// prefix in the main table" rules. LAN rules sit well before the tunnel rule
// (tunnelRulePref) so they always win for directly connected traffic.
const lanRulePrefBase = 90

// connectedRoute is one directly connected IPv4 subnet from `ip route`.
type connectedRoute struct {
	Dst string
	Dev string
	Src string
}

// parseConnectedRoutes parses `ip -4 route show proto kernel` output lines like
// "192.168.1.0/24 dev br0 proto kernel scope link src 192.168.1.1".
//
// Lines without a usable `dev` and lines for the default route are ignored;
// `src` is optional. Anything else (including the destination prefix) is
// returned as-is — filtering by device is the caller's job.
func parseConnectedRoutes(out string) []connectedRoute {
	var routes []connectedRoute
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		dst := fields[0]
		if dst == "default" {
			continue
		}

		var r connectedRoute
		r.Dst = dst
		for i := 0; i < len(fields); i++ {
			switch fields[i] {
			case "dev":
				if i+1 < len(fields) {
					r.Dev = fields[i+1]
				}
			case "src":
				if i+1 < len(fields) {
					r.Src = fields[i+1]
				}
			}
		}
		if r.Dev == "" {
			continue
		}
		routes = append(routes, r)
	}
	return routes
}

// lanCIDRs returns the destination prefixes that must stay in the main table:
// directly connected subnets (except the tunnel device), plus link-local and
// multicast. Order is deterministic: input order, then the two constants if
// they were not already present. Duplicates are dropped.
func lanCIDRs(connected []connectedRoute, tunName string) []string {
	var out []string
	seen := make(map[string]bool)
	add := func(cidr string) {
		if cidr == "" || cidr == "default" || seen[cidr] {
			return
		}
		seen[cidr] = true
		out = append(out, cidr)
	}

	for _, r := range connected {
		if r.Dev == tunName {
			continue
		}
		add(r.Dst)
	}
	add("169.254.0.0/16") // link-local
	add("224.0.0.0/4")    // multicast

	return out
}

// lanRuleArgs returns the `ip rule add|del` arguments that pin cidr to the main
// routing table at priority lanRulePrefBase+i.
func lanRuleArgs(cidr string, i int) []string {
	return []string{"to", cidr, "lookup", "main", "pref", strconv.Itoa(lanRulePrefBase + i)}
}
