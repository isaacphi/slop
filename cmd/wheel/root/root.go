package root

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/isaacphi/wheel/internal/tui"
)

var rootCmd = &cobra.Command{
	Use:   "wheel",
	Short: "A TUI interface for chatting with LLMs",
	Long: `Wheel is a terminal user interface for interacting with various LLM models.
It supports multiple models, conversation management, and custom prompt templates.`,
	Run: func(cmd *cobra.Command, args []string) {
		model, err := tui.NewModel()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error initializing app: %v\n", err)
			os.Exit(1)
		}
		
		p := tea.NewProgram(
			model,
			tea.WithAltScreen(),
			tea.WithMouseCellMotion(),
		)

		if _, err := p.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Error running app: %v\n", err)
			os.Exit(1)
		}
	},
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	// Commands will be added here
}