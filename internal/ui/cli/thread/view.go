package thread

import (
	"fmt"
	"time"

	"github.com/isaacphi/slop/internal/domain"
	"github.com/isaacphi/slop/internal/service"
	"github.com/spf13/cobra"
)

var viewCmd = &cobra.Command{
	Use:   "view [thread_id]",
	Short: "View messages in a thread",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		chatService, err := service.InitializeChatService(nil)
		if err != nil {
			return err
		}

		thread, err := chatService.FindThreadByPartialID(cmd.Context(), args[0])
		if err != nil {
			return fmt.Errorf("failed to find thread: %w", err)
		}

		messages, err := chatService.GetThreadMessages(cmd.Context(), thread.ID, nil)
		if err != nil {
			return fmt.Errorf("failed to get thread messages: %w", err)
		}

		fmt.Printf("Thread %s (created %s)\n\n",
			thread.ID.String()[:8],
			thread.CreatedAt.Format(time.RFC822),
		)

		if limitFlag > 0 && len(messages) > limitFlag {
			messages = messages[len(messages)-limitFlag:]
		}

		for _, msg := range messages {
			roleStr := "You"
			if msg.Role == domain.RoleAssistant {
				roleStr = "Slop"
			}
			fmt.Printf("%s - %s: %s\n", msg.ID.String()[:8], roleStr, msg.Content)
		}

		return nil
	},
}
