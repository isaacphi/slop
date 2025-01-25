package cli

import (
	"fmt"
	"os"

	"github.com/isaacphi/wheel/internal/ui/cli/chat"
	"github.com/isaacphi/wheel/internal/ui/cli/config"
	"github.com/isaacphi/wheel/internal/ui/cli/msg"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:               "wheel",
	Short:             "Wheel is a chat interface for LLMs",
	Long:              `A CLI and TUI interface for interacting with LLMs and managing conversation history.`,
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
	)

	// Here you would define your flags and configuration settings
	// rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.wheel.yaml)")
}
