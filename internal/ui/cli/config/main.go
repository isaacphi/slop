package config

import (
	"github.com/isaacphi/slop/internal/appState"
	"github.com/spf13/cobra"
)

var (
	includeSources bool
	prefixFilter   string

	ConfigCmd = &cobra.Command{
		Use:   "config [prefix]",
		Short: "View configuration",
		Long:  "Read configuration. If prefix is included, only show configuration under that path. E.g. slop config models.openai",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := appState.Get().Config

			if len(args) > 0 {
				prefixFilter = args[0]
			}

			cfg.PrintConfig(includeSources, prefixFilter)

			return nil
		},
	}
)

func init() {
	ConfigCmd.Flags().BoolVarP(&includeSources, "include-sources", "s", false, "Show source file for each configuration value")
}
