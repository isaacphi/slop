package thread

import (
	"fmt"
	"strings"
	"time"

	"github.com/isaacphi/slop/internal/message"
	"github.com/spf13/cobra"
)

var deleteCmd = &cobra.Command{
	Use:   "rm [thread_id]",
	Short: "Delete a thread and all its messages",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		messageService, err := service.InitializeMessageService(nil)
		if err != nil {
			return err
		}

		// Find thread by partial ID
		thread, err := messageService.FindThreadByPartialID(cmd.Context(), args[0])
		if err != nil {
			return fmt.Errorf("failed to find thread: %w", err)
		}

		// Show thread info and confirm deletion
		summary, err := messageService.GetThreadDetails(cmd.Context(), thread)
		if err != nil {
			return fmt.Errorf("failed to get thread details: %w", err)
		}

		fmt.Printf("About to delete thread %s:\n", summary.ID.String()[:8])
		fmt.Printf("Created: %s\n", summary.CreatedAt.Format(time.RFC822))
		fmt.Printf("Messages: %d\n", summary.MessageCount)
		fmt.Printf("Preview: %s\n", summary.Preview)

		if !forceFlag {
			fmt.Print("\nAre you sure you want to delete this thread? [y/N] ")
			var response string
			_, err := fmt.Scanln(&response)
			if err != nil {
				return fmt.Errorf("failed to read input: %w", err)
			}

			response = strings.ToLower(strings.TrimSpace(response))
			if response != "y" && response != "yes" {
				fmt.Println("Operation cancelled")
				return nil
			}
		}

		if err := messageService.DeleteThread(cmd.Context(), thread.ID); err != nil {
			return fmt.Errorf("failed to delete thread: %w", err)
		}

		fmt.Println("Thread deleted successfully")
		return nil
	},
}
