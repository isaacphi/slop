package thread

import (
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/isaacphi/slop/internal/shared"
	"github.com/spf13/cobra"
)

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
			summary, err := chatService.GetThreadDetails(cmd.Context(), thread)
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
