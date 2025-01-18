package msg

import (
	"fmt"

	"github.com/isaacphi/wheel/internal/config"
	"github.com/spf13/cobra"
)

var MsgCmd = &cobra.Command{
	Use:   "msg",
	Short: "Send messages",
	Long:  `Send messages to the LLM in various ways.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Load the configuration
		cfg, err := config.New(true)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		cfg.PrintConfig()
		return nil
	},
}

func init() {
	MsgCmd.AddCommand(sendCmd)
}
