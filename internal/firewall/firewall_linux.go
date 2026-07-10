//go:build linux

package firewall

import (
	"fmt"
	"os/exec"
)

func rules(iface string) func() {
	var applied []string

	add := func(args ...string) {
		cmd := exec.Command(args[0], args[1:]...)
		if out, err := cmd.CombinedOutput(); err != nil {
			fmt.Printf("warning: firewall: %v: %s\n", err, out)
			return
		}
		applied = append(applied, args[1]) // -I or -D
	}

	add("iptables", "-I", "FORWARD", "-i", iface, "-j", "ACCEPT")
	add("iptables", "-I", "FORWARD", "-o", iface, "-j", "ACCEPT")

	return func() {
		for range applied {
			_ = exec.Command("iptables", "-D", "FORWARD", "-i", iface, "-j", "ACCEPT").Run()
			_ = exec.Command("iptables", "-D", "FORWARD", "-o", iface, "-j", "ACCEPT").Run()
		}
	}
}
