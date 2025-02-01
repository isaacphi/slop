package thread

import (
	"context"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/isaacphi/slop/internal/domain"
	"github.com/isaacphi/slop/internal/shared"
	"github.com/spf13/cobra"
)

var (
	limitFlag int
	forceFlag bool
)

var ThreadCmd = &cobra.Command{
	Use:   "thread",
	Short: "Manage conversation threads",
}

var listCmd = &cobra.Command{
	Use:   "ls",
	Short: "List conversation threads",
	RunE: func(cmd *cobra.Command, args []string) error {
		chatService, err := shared.InitializeChatService(nil)
		if err != nil {
			return err
		}

		threads, err := chatService.ListThreads(cmd.Context(), limitFlag)
		if err != nil {
			return fmt.Errorf("failed to list threads: %w", err)
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "ID\tCreated\tMessages\tPreview")

		for _, thread := range threads {
			summary, err := chatService.GetThreadSummary(cmd.Context(), thread)
			if err != nil {
				return fmt.Errorf("failed to get thread summary: %w", err)
			}

			fmt.Fprintf(w, "%s\t%s\t%d\t%s\n",
				summary.ID.String()[:8],
				summary.CreatedAt.Format(time.RFC822),
				summary.MessageCount,
				summary.Preview,
			)
		}
		w.Flush()

		return nil
	},
}

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

func init() {
	listCmd.Flags().IntVarP(&limitFlag, "limit", "n", 0, "Limit the number of threads to show (0 for all)")
	viewCmd.Flags().IntVarP(&limitFlag, "limit", "n", 0, "Limit the number of messages to show (0 for all)")
	deleteCmd.Flags().BoolVarP(&forceFlag, "force", "f", false, "Delete without confirmation")

	ThreadCmd.AddCommand(listCmd, viewCmd, deleteCmd)
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
		// Print role
		roleStr := "You"
		if msg.Role == domain.RoleAssistant {
			roleStr = "Slop"
		}
		fmt.Printf("%s: %s\n", roleStr, msg.Content)

		// Add newline between messages for readability
		fmt.Println()
	}

	return nil
}

var deleteCmd = &cobra.Command{
	Use:   "rm [thread_id]",
	Short: "Delete a thread and all its messages",
	Args:  cobra.ExactArgs(1),
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

		// Show thread info and confirm deletion
		summary, err := chatService.GetThreadSummary(cmd.Context(), thread)
		if err != nil {
			return fmt.Errorf("failed to get thread summary: %w", err)
		}

		fmt.Printf("About to delete thread %s:\n", summary.ID.String()[:8])
		fmt.Printf("Created: %s\n", summary.CreatedAt.Format(time.RFC822))
		fmt.Printf("Messages: %d\n", summary.MessageCount)
		fmt.Printf("Preview: %s\n", summary.Preview)

		if !forceFlag {
			fmt.Print("\nAre you sure you want to delete this thread? [y/N] ")
			var response string
			fmt.Scanln(&response)

			response = strings.ToLower(strings.TrimSpace(response))
			if response != "y" && response != "yes" {
				fmt.Println("Operation cancelled")
				return nil
			}
		}

		if err := chatService.DeleteThread(cmd.Context(), thread.ID); err != nil {
			return fmt.Errorf("failed to delete thread: %w", err)
		}

		fmt.Println("Thread deleted successfully")
		return nil
	},
}
