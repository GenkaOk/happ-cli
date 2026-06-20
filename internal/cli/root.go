// Package cli implements the happ command-line interface.
package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/aimuzov/happ-cli/internal/store"
)

var homeDir string

// Execute runs the root command with the given context (cancelled on SIGINT).
// version is reported by `happ --version` / `happ version`.
func Execute(ctx context.Context, version string) error {
	return newRootCmd(version).ExecuteContext(ctx)
}

func newRootCmd(version string) *cobra.Command {
	root := &cobra.Command{
		Use:   "happ",
		Short: "HAPP-compatible terminal VPN client",
		Long: "happ is a terminal VPN client compatible with HAPP subscription profiles.\n" +
			"It fetches a subscription, parses its share links (VLESS/VMess/Trojan/Shadowsocks),\n" +
			"and connects through an embedded xray-core, either as a local proxy or a full TUN tunnel.",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: false,
	}
	root.PersistentFlags().StringVar(&homeDir, "home", "", "config directory (default: per-user config dir + /happ-cli)")
	root.AddCommand(
		newSubCmd(),
		newListCmd(),
		newConnectCmd(),
		newConfigCmd(),
		newSysProxyCmd(),
	)
	return root
}

func openStore() (*store.Store, error) {
	dir := homeDir
	if dir == "" {
		d, err := store.DefaultDir()
		if err != nil {
			return nil, err
		}
		dir = d
	}
	return store.Open(dir)
}

// resolveSub returns the named subscription, or the active one when name is "".
func resolveSub(st *store.Store, name string) (store.SubEntry, error) {
	if name == "" {
		name = st.Active()
	}
	if name == "" {
		return store.SubEntry{}, fmt.Errorf("no subscription specified and none is active; add one with 'happ sub add <url>'")
	}
	sub, ok := st.Find(name)
	if !ok {
		return store.SubEntry{}, fmt.Errorf("subscription %q not found", name)
	}
	return sub, nil
}
