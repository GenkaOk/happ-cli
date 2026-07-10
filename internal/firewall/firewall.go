// Package firewall manages iptables/nftables rules needed for TUN forwarding.
// On non-Linux platforms it is a no-op.
package firewall

// Rules adds FORWARD accept rules for the given TUN interface so forwarded
// traffic (e.g. from LAN clients through a router) is not dropped.
// Returns a cleanup function that removes the rules.
func Rules(iface string) (cleanup func()) {
	return rules(iface)
}
