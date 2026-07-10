//go:build !linux

package firewall

func rules(iface string) func() { return func() {} }
