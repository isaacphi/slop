package thread

import (
	"context"
	"fmt"
	"time"

	"github.com/isaacphi/slop/internal/domain"
	"github.com/isaacphi/slop/internal/shared"
	"github.com/spf13/cobra"
)

var viewCmd = &cobra.Command{
	Use:   "view [thread_id]",
	Short: "View messages in a thread",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		chatService, err := shared.InitializeChatService(nil)
		if err != nil {
			return err
		}

		thread, err := chatService.FindThreadByPartialID(cmd.Context(), args[0])
		if err != nil {
			return fmt.Errorf("failed to find thread: %w", err)
		}

		if err := printThread(cmd.Context(), thread, limitFlag); err != nil {
			return fmt.Errorf("failed to print thread: %w", err)
		}

		return nil
	},
}

func printThread(ctx context.Context, thread *domain.Thread, limit int) error {
	fmt.Printf("Thread %s (created %s)\n\n",
		thread.ID.String()[:8],
		thread.CreatedAt.Format(time.RFC822),
	)

	messages := thread.Messages
	if limit > 0 && len(messages) > limit {
		messages = messages[len(messages)-limit:]
	}

	for _, msg := range messages {
		roleStr := "You"
		if msg.Role == domain.RoleAssistant {
			roleStr = "Slop"
		}
		parentID := "nil     "
		if msg.ParentID != nil {
			parentID = msg.ParentID.String()[:8]
		}
		fmt.Printf("%s %s - %s: %s\n", msg.ID.String()[:8], parentID, roleStr, msg.Content)

		// Add newline between messages for readability
		fmt.Println()
	}

	return nil
}
