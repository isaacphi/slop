package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/isaacphi/slop/internal/appState"
	"github.com/isaacphi/slop/internal/config"
	"github.com/isaacphi/slop/internal/ui/cli/chat"
	configCmd "github.com/isaacphi/slop/internal/ui/cli/config"
	"github.com/isaacphi/slop/internal/ui/cli/mcp"
	"github.com/isaacphi/slop/internal/ui/cli/msg"
	"github.com/isaacphi/slop/internal/ui/cli/thread"
	"github.com/spf13/cobra"
)

var (
	logLevel string
	logFile  string
)

var rootCmd = &cobra.Command{
	Use:               "slop",
	Short:             "For all your slop needs",
	Long:              `A CLI interface for slop`,
	DisableAutoGenTag: true,
}

func Execute() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up the root command to use this context
	rootCmd.SetContext(ctx)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	// Add global flags for logging
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "", "Set logging level (DEBUG, INFO, WARN, ERROR)")
	rootCmd.PersistentFlags().StringVar(&logFile, "log-file", "", "Log file path (defaults to stdout)")

	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		// Initialize app with logging overrides
		overrides := &config.RuntimeOverrides{}
		if logLevel != "" {
			overrides.LogLevel = &logLevel
		}
		if logFile != "" {
			overrides.LogFile = &logFile
		}
		return appState.Initialize(overrides)
	}

	rootCmd.PersistentPostRunE = func(cmd *cobra.Command, args []string) error {
		return appState.Cleanup()
	}

	// Remove "completions" command
	rootCmd.CompletionOptions.DisableDefaultCmd = true

	rootCmd.AddCommand(
		configCmd.ConfigCmd,
		msg.MsgCmd,
		thread.ThreadCmd,
		mcp.MCPCmd,
		chat.ChatCmd,
	)
}
