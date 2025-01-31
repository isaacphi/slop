package config

import (
	"fmt"

	"github.com/isaacphi/slop/internal/config"
	"github.com/spf13/cobra"
)

var (
	includeSources bool

	ConfigCmd = &cobra.Command{
		Use:   "config",
		Short: "Manage configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.New(true)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			cfg.PrintConfig(includeSources)

			return nil
		},
	}
)

func init() {
	ConfigCmd.Flags().BoolVarP(&includeSources, "include-sources", "s", false, "Show source file for each configuration value")
}

