package thread

import (
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

func init() {
	listCmd.Flags().IntVarP(&limitFlag, "limit", "n", 0, "Limit the number of threads to show (0 for all)")
	viewCmd.Flags().IntVarP(&limitFlag, "limit", "n", 0, "Limit the number of messages to show (0 for all)")
	deleteCmd.Flags().BoolVarP(&forceFlag, "force", "f", false, "Delete without confirmation")

	ThreadCmd.AddCommand(listCmd, viewCmd, deleteCmd)
}
