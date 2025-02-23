package msg

import (
	"fmt"

	"github.com/isaacphi/slop/internal/appState"
	"github.com/isaacphi/slop/internal/repository/sqlite"
	"github.com/spf13/cobra"
)

var deleteCmd = &cobra.Command{
	Use:   "delete [thread_id]",
	Short: "Delete the last message pair from a conversation",
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

		// Get thread messages
		messages, err := repo.GetMessages(cmd.Context(), thread.ID, nil, false)
		if err != nil {
			return fmt.Errorf("failed to get thread messages: %w", err)
		}

		if len(messages) < 2 {
			return fmt.Errorf("thread has fewer than 2 messages")
		}

		// Delete the last two messages
		if err := repo.DeleteLastMessages(cmd.Context(), thread.ID, 2); err != nil {
			return fmt.Errorf("failed to delete messages: %w", err)
		}

		fmt.Println("Last message pair deleted successfully")
		return nil
	},
}

func init() {
	MsgCmd.AddCommand(deleteCmd)
}
