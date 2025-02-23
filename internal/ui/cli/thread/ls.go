package thread

import (
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/isaacphi/slop/internal/appState"
	"github.com/isaacphi/slop/internal/repository/sqlite"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "ls",
	Short: "List conversation threads",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := appState.Get().Config
		repo, err := sqlite.Initialize(cfg.DBPath)
		if err != nil {
			return err
		}

		threads, err := repo.ListThreads(cmd.Context(), limitFlag)
		if err != nil {
			return fmt.Errorf("failed to list threads: %w", err)
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "ID\tCreated\tMessages\tPreview")

		for _, thread := range threads {
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

			fmt.Fprintf(w, "%s\t%s\t%d\t%s\n",
				thread.ID.String()[:8],
				thread.CreatedAt.Format(time.RFC822),
				len(messages),
				preview,
			)
		}
		w.Flush()

		return nil
	},
}

func init() {
	listCmd.Flags().IntVarP(&limitFlag, "limit", "n", 0, "Limit the number of threads to show (0 for all)")
	ThreadCmd.AddCommand(listCmd)
}
