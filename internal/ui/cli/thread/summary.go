package thread

import (
	"fmt"
	"strings"

	"github.com/isaacphi/slop/internal/service"
	"github.com/isaacphi/slop/internal/shared"
	"github.com/spf13/cobra"
)

var summaryCmd = &cobra.Command{
	Use:   "summary [thread_id] [summary]",
	Short: "Set a summary for a thread",
	Long:  "Write a summary for a thread. Leave [summary] blank to auto generate a slop summary",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		chatService, err := shared.InitializeChatService(nil)
		if err != nil {
			return err
		}

		// Find thread by partial ID
		thread, err := chatService.FindThreadByPartialID(cmd.Context(), args[0])
		if err != nil {
			return fmt.Errorf("failed to find thread: %w", err)
		}

		summary := ""
		if len(args) > 1 {
			// User supplied summary
			summary = strings.Join(args[1:], " ")
		} else {
			// No user supplied summary. Use slop.
			messages, err := chatService.GetThreadMessages(cmd.Context(), thread.ID, nil)
			if err != nil {
				return fmt.Errorf("failed to get thread messages: %w", err)
			}
			internalService, err := service.NewInternalService()
			if err != nil {
				return fmt.Errorf("failed to initialize internal service: %w", err)
			}
			summary, err = internalService.CreateThreadSummary(cmd.Context(), messages)
			if err != nil {
				return fmt.Errorf("failed to generate summary: %w", err)
			}
		}
		err = chatService.SetThreadSummary(cmd.Context(), thread, summary)
		if err != nil {
			return fmt.Errorf("failed to set thread summary: %w", err)
		}
		fmt.Println("Thread summary updated successfully")
		return nil
	},
}
