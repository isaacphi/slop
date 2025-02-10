package msg

import (
	"fmt"

	"github.com/isaacphi/slop/internal/app"
	messageService "github.com/isaacphi/slop/internal/message"
	"github.com/spf13/cobra"
)

var deleteCmd = &cobra.Command{
	Use:   "delete [thread_id]",
	Short: "Delete the last message pair from a conversation",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := app.Get().Config

		service, err := messageService.InitializeMessageService(cfg, nil)
		if err != nil {
			return err
		}

		// Find thread by partial ID
		thread, err := service.FindThreadByPartialID(cmd.Context(), args[0])
		if err != nil {
			return fmt.Errorf("failed to find thread: %w", err)
		}

		// Get thread messages
		messages, err := service.GetThreadMessages(cmd.Context(), thread.ID, nil)
		if err != nil {
			return fmt.Errorf("failed to get thread messages: %w", err)
		}

		if len(messages) < 2 {
			return fmt.Errorf("thread has fewer than 2 messages")
		}

		// Delete the last two messages
		if err := service.DeleteLastMessages(cmd.Context(), thread.ID, 2); err != nil {
			return fmt.Errorf("failed to delete messages: %w", err)
		}

		fmt.Println("Last message pair deleted successfully")
		return nil
	},
}
