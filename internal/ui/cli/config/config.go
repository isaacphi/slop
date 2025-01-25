package config

import (
	"fmt"

	"github.com/isaacphi/wheel/internal/config"
	"github.com/spf13/cobra"
)

var ConfigCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage configuration",
	Long:  `Read, write, or edit local and global wheel configuration.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.New(true)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		cfg.PrintConfig()
		return nil
	},
}

func init() {}
