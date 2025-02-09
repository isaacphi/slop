package thread

import (
	"fmt"
	"strings"

	internalService "github.com/isaacphi/slop/internal/internalService"
	messageService "github.com/isaacphi/slop/internal/message"
	"github.com/spf13/cobra"
)

var summaryCmd = &cobra.Command{
	Use:   "summary [thread_id] [summary]",
	Short: "Set a summary for a thread",
	Long:  "Write a summary for a thread. Leave [summary] blank to auto generate a slop summary",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		messageService, err := messageService.InitializeMessageService(nil)
		if err != nil {
			return err
		}

		// Find thread by partial ID
		thread, err := messageService.FindThreadByPartialID(cmd.Context(), args[0])
		if err != nil {
			return fmt.Errorf("failed to find thread: %w", err)
		}

		summary := ""
		if len(args) > 1 {
			// User supplied summary
			summary = strings.Join(args[1:], " ")
		} else {
			// No user supplied summary. Use slop.
			messages, err := messageService.GetThreadMessages(cmd.Context(), thread.ID, nil)
			if err != nil {
				return fmt.Errorf("failed to get thread messages: %w", err)
			}
			internal, err := internalService.NewInternalService()
			if err != nil {
				return fmt.Errorf("failed to initialize internal service: %w", err)
			}
			summary, err = internal.CreateThreadSummary(cmd.Context(), messages)
			if err != nil {
				return fmt.Errorf("failed to generate summary: %w", err)
			}
		}
		err = messageService.SetThreadSummary(cmd.Context(), thread, summary)
		if err != nil {
			return fmt.Errorf("failed to set thread summary: %w", err)
		}
		fmt.Println("Thread summary updated successfully")
		return nil
	},
}
