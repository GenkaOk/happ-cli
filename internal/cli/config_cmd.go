package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"

	"github.com/aimuzov/happ-cli/internal/config"
)

func newConfigInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "config-init",
		Short: "Write current defaults to config.yaml",
		Long: "Create or overwrite ~/.config/happ-cli/config.yaml with the built-in\n" +
			"defaults. Edit the file to change defaults for future connect runs.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, err := storeDir()
			if err != nil {
				return err
			}
			cfg := config.Default()
			// Write the file.
			if err := cfg.Save(dir); err != nil {
				return err
			}
			// Also print it.
			data, _ := yaml.Marshal(cfg)
			fmt.Println(string(data))
			return nil
		},
	}
}

func newConfigShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "config-show",
		Short: "Show effective configuration",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, err := storeDir()
			if err != nil {
				return err
			}
			cfg, err := config.Load(dir)
			if err != nil {
				return err
			}
			data, _ := yaml.Marshal(cfg)
			fmt.Println(string(data))
			return nil
		},
	}
}
