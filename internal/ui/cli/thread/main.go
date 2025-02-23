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
