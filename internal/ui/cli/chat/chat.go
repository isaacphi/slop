package chat

import (
	"github.com/charmbracelet/bubbletea"
	"github.com/isaacphi/wheel/internal/ui/tui"
	"github.com/spf13/cobra"
)

var ChatCmd = &cobra.Command{
	Use:   "chat",
	Short: "Start an interactive chat session",
	Long:  `Start or manage interactive chat sessions.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		p := tea.NewProgram(tui.New(),
			tea.WithAltScreen(),
			tea.WithMouseCellMotion())
		_, err := p.Run()
		return err
	},
}

func init() {}
