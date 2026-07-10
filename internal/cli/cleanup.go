package cli

import (
	"fmt"
	"os/exec"

	"github.com/spf13/cobra"
)

func newCleanupTunCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "cleanup-tun",
		Short: "Remove stale TUN device and routes left by a crash (Linux, requires root)",
		Long: "If happ was killed with SIGKILL during TUN mode, the thapp device,\n" +
			"routes, and iptables rules may remain. This command cleans them up.\n\n" +
			"  sudo happ cleanup-tun",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Flush happ's dedicated routing table.
			_ = runCmd("ip", "route", "flush", "table", "100")
			_ = runCmd("ip", "rule", "del", "from", "all", "table", "100")

			// Remove iptables FORWARD rules.
			_ = runCmd("iptables", "-D", "FORWARD", "-i", "thapp", "-j", "ACCEPT")
			_ = runCmd("iptables", "-D", "FORWARD", "-o", "thapp", "-j", "ACCEPT")

			// Delete the TUN device.
			if err := runCmd("ip", "link", "del", "thapp"); err != nil {
				// Device may already be gone — not an error.
				fmt.Println("TUN device thapp not found (already cleaned up).")
			} else {
				fmt.Println("TUN device thapp removed.")
			}

			fmt.Println("All happ TUN routes cleared.")
			return nil
		},
	}
}

// runCmd executes a command; errors are printed but not returned (best-effort cleanup).
func runCmd(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		fmt.Printf("warning: %s %v: %v\n", name, args, err)
		if len(out) > 0 {
			fmt.Printf("  %s\n", string(out))
		}
		return err
	}
	return nil
}
