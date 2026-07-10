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
		Long: "If happ was killed with SIGKILL during TUN mode, the thapp device and\n" +
			"its routes may remain. This command cleans them up.\n\n" +
			"  sudo happ cleanup-tun",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Remove the /1 default overrides (best-effort).
			_ = runCmd("ip", "route", "del", "0.0.0.0/1")
			_ = runCmd("ip", "route", "del", "128.0.0.0/1")

			// Remove IPv6 blocks.
			_ = runCmd("ip", "-6", "route", "del", "::/1")
			_ = runCmd("ip", "-6", "route", "del", "8000::/1")

			// Remove local-net preservation routes.
			for _, net := range []string{
				"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16",
				"169.254.0.0/16", "224.0.0.0/4",
			} {
				_ = runCmd("ip", "route", "del", net)
			}

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
