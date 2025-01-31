package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type model struct {
	message string
}

func New() *model {
	return &model{
		message: "Welcome to slop! Press 'q' to quit.",
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		}
	}
	return m, nil
}

var style = lipgloss.NewStyle().
	Align(lipgloss.Center).
	BorderStyle(lipgloss.RoundedBorder()).
	BorderForeground(lipgloss.Color("62")).
	Padding(1, 0).
	MarginTop(1)

func (m model) View() string {
	return style.Render(m.message)
}
