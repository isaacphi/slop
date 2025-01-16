package chat

import (
	"github.com/charmbracelet/bubbletea"
	"github.com/isaacphi/wheel/internal/ui/tui"
	"github.com/spf13/cobra"
)

var interactiveCmd = &cobra.Command{
	Use:   "interactive",
	Short: "Start an interactive chat session",
	Long:  `Start a fully interactive chat session with a TUI interface.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		p := tea.NewProgram(tui.New(),
			tea.WithAltScreen(),
			tea.WithMouseCellMotion())
		_, err := p.Run()
		return err
	},
}

func newInteractiveCmd() *cobra.Command {
	return interactiveCmd
}
