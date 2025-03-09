package chat

import (
	"github.com/isaacphi/slop/internal/appState"
	"github.com/isaacphi/slop/internal/ui/tui"
	"github.com/spf13/cobra"
)

var (
	ChatCmd = &cobra.Command{
		Use:   "chat",
		Short: "Start interactive chat",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			config := appState.Get().Config
			tui.StartTUI(&config.KeyMap)

			return nil
		},
	}
)
