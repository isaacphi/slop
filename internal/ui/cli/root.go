package cli

import (
	"fmt"
	"os"

	"github.com/isaacphi/slop/internal/ui/cli/chat"
	"github.com/isaacphi/slop/internal/ui/cli/config"
	"github.com/isaacphi/slop/internal/ui/cli/msg"
	"github.com/isaacphi/slop/internal/ui/cli/thread"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:               "slop",
	Short:             "For all your slop needs",
	Long:              `A CLI and TUI interface for interacting with LLMs`,
	DisableAutoGenTag: true,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.CompletionOptions.DisableDefaultCmd = true
	rootCmd.AddCommand(
		chat.ChatCmd,
		config.ConfigCmd,
		msg.MsgCmd,
		thread.ThreadCmd,
	)
}

