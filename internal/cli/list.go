package cli

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/aimuzov/happ-cli/internal/xray"
)

func newListCmd() *cobra.Command {
	var subName string
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls", "servers"},
		Short:   "List servers in a subscription",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := openStore()
			if err != nil {
				return err
			}
			sub, err := resolveSub(st, subName)
			if err != nil {
				return err
			}
			servers := sub.Servers()
			if len(servers) == 0 {
				fmt.Printf("Subscription %q has no servers.\n", sub.Name)
				return nil
			}
			tw := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
			fmt.Fprintf(tw, "#\tPROTOCOL\tADDRESS\tTAG\n")
			for i, s := range servers {
				note := s.Protocol
				if !xray.Supported(s.Protocol) {
					note += " (unsupported)"
				}
				fmt.Fprintf(tw, "%d\t%s\t%s:%d\t%s\n", i+1, note, s.Address, s.Port, s.Tag)
			}
			return tw.Flush()
		},
	}
	cmd.Flags().StringVar(&subName, "sub", "", "subscription name (default: active)")
	return cmd
}

func newConfigCmd() *cobra.Command {
	var subName string
	var socksPort, httpPort int
	cmd := &cobra.Command{
		Use:   "config [selector]",
		Short: "Print the generated xray-core config for a server",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := openStore()
			if err != nil {
				return err
			}
			sub, err := resolveSub(st, subName)
			if err != nil {
				return err
			}
			selector := ""
			if len(args) > 0 {
				selector = args[0]
			}
			srv, _, err := selectServer(sub.Servers(), selector)
			if err != nil {
				return err
			}
			cfg, err := xray.BuildConfig(srv, xray.Options{SocksPort: socksPort, HTTPPort: httpPort})
			if err != nil {
				return err
			}
			raw, err := cfg.JSON()
			if err != nil {
				return err
			}
			fmt.Println(string(raw))
			return nil
		},
	}
	cmd.Flags().StringVar(&subName, "sub", "", "subscription name (default: active)")
	cmd.Flags().IntVar(&socksPort, "socks", 10808, "SOCKS5 inbound port")
	cmd.Flags().IntVar(&httpPort, "http", 10809, "HTTP inbound port")
	return cmd
}
