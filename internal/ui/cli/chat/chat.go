package chat

import (
	"github.com/spf13/cobra"
)

var ChatCmd = &cobra.Command{
	Use:   "chat",
	Short: "Manage chat sessions",
	Long:  `Start or manage interactive chat sessions.`,
}

func init() {
	ChatCmd.AddCommand(interactiveCmd)
}
