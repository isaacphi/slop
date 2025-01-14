package root

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/isaacphi/wheel/internal/tui"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "wheel",
	Short: "A TUI interface for chatting with LLMs",
	Long: `Wheel is a terminal user interface for interacting with various LLM models.
It supports multiple models, conversation management, and custom prompt templates.`,
	Run: func(cmd *cobra.Command, args []string) {
		p := tea.NewProgram(tui.NewModel())
		if _, err := p.Run(); err != nil {
			panic(err)
		}
	},
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	// Commands will be added here
}

