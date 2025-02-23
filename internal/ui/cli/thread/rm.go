package thread

import (
	"fmt"
	"strings"
	"time"

	"github.com/isaacphi/slop/internal/appState"
	"github.com/isaacphi/slop/internal/repository/sqlite"
	"github.com/spf13/cobra"
)

var deleteCmd = &cobra.Command{
	Use:   "rm [thread_id]",
	Short: "Delete a thread and all its messages",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := appState.Get().Config
		repo, err := sqlite.Initialize(cfg.DBPath)
		if err != nil {
			return err
		}

		// Find thread by partial ID
		thread, err := repo.GetThreadByPartialID(cmd.Context(), args[0])
		if err != nil {
			return fmt.Errorf("failed to find thread: %w", err)
		}

		// Get thread info for display
		messages, err := repo.GetMessages(cmd.Context(), thread.ID, nil, false)
		if err != nil {
			return fmt.Errorf("failed to get messages: %w", err)
		}

		preview := "[empty]"
		if thread.Summary != "" {
			preview = thread.Summary
		} else {
			for _, msg := range messages {
				if msg.Role == "human" {
					preview = msg.Content
					break
				}
			}
		}
		if len(preview) > 50 {
			preview = preview[:47] + "..."
		}

		fmt.Printf("About to delete thread %s:\n", thread.ID.String()[:8])
		fmt.Printf("Created: %s\n", thread.CreatedAt.Format(time.RFC822))
		fmt.Printf("Messages: %d\n", len(messages))
		fmt.Printf("Preview: %s\n", preview)

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

		if err := repo.DeleteThread(cmd.Context(), thread.ID); err != nil {
			return fmt.Errorf("failed to delete thread: %w", err)
		}

		fmt.Println("Thread deleted successfully")
		return nil
	},
}

func init() {
	deleteCmd.Flags().BoolVarP(&forceFlag, "force", "f", false, "Delete without confirmation")
	ThreadCmd.AddCommand(deleteCmd)
}
