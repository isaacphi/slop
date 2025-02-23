package msg

import (
	"context"
	"fmt"

	"github.com/isaacphi/slop/internal/agent"
	"github.com/isaacphi/slop/internal/appState"
	"github.com/isaacphi/slop/internal/mcp"
	"github.com/isaacphi/slop/internal/repository/sqlite"
	"github.com/spf13/cobra"
)

// internal/ui/cli/msg/reject.go
var rejectCmd = &cobra.Command{
	Use:   "reject [thread] [message] [reason]",
	Short: "Reject pending tool calls",
	Long:  "Reject pending tool calls with optional reason",
	Args:  cobra.RangeArgs(2, 3),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		// Initialize services
		cfg := appState.Get().Config
		repo, err := sqlite.Initialize(cfg.DBPath)
		if err != nil {
			return fmt.Errorf("failed to initialize repository: %w", err)
		}

		mcpClient := mcp.New(cfg.MCPServers)
		if err := mcpClient.Initialize(context.Background()); err != nil {
			return fmt.Errorf("failed to initialize MCP client: %w", err)
		}
		defer mcpClient.Shutdown()

		preset := cfg.Presets[cfg.ActiveModel]
		agentService, err := agent.New(repo, mcpClient, preset, cfg.Toolsets)
		if err != nil {
			return fmt.Errorf("could not initialize agent: %w", err)
		}

		// Get thread and message
		thread, err := repo.GetThreadByPartialID(ctx, args[0])
		if err != nil {
			return fmt.Errorf("failed to find thread: %w", err)
		}

		// Get message
		msg, err := repo.FindMessageByPartialID(ctx, thread.ID, args[1])
		if err != nil {
			return fmt.Errorf("failed to find message: %w", err)
		}

		// Get optional reason
		var reason string
		if len(args) > 2 {
			reason = args[2]
		}

		// Reject tools
		sendOptions := agent.SendMessageOptions{
			StreamHandler: &CLIStreamHandler{originalCallback: func(chunk []byte) error {
				fmt.Print(string(chunk))
				return nil
			}},
		}

		errCh := make(chan error, 1)
		go func() {
			resp, err := agentService.RejectTools(ctx, msg.ID, reason, sendOptions)
			if err != nil {
				errCh <- err
				return
			}
			if noStreamFlag {
				fmt.Print(resp.Content)
			}
			errCh <- nil
		}()

		select {
		case <-ctx.Done():
			fmt.Println("\nRequest cancelled")
			return ctx.Err()
		case err := <-errCh:
			if err != nil {
				return fmt.Errorf("failed to reject tools: %w", err)
			}
		}

		fmt.Println()
		return nil
	},
}

func init() {
	rejectCmd.Flags().BoolVarP(&noStreamFlag, "no-stream", "n", false, "Disable streaming of responses")
	MsgCmd.AddCommand(rejectCmd)
}
