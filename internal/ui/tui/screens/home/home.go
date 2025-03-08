package home

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Model represents the home screen
type Model struct {
	width  int
	height int
}

// New creates a new home screen model
func New() Model {
	return Model{}
}

// Init initializes the home screen
func (m Model) Init() tea.Cmd {
	return nil
}

// Update handles updates to the home screen
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		// Handle home-specific key presses
		// For example, navigation within the home screen
	}

	return m, nil
}

// View renders the home screen
func (m Model) View() string {
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#7D56F4")).
		Padding(0, 1).
		Render("slop - Home Screen")

	content := lipgloss.NewStyle().
		Align(lipgloss.Center).
		Render("Welcome to slop!\n\nPress 'c' to go to chat")

	return lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"\n",
		content,
	)
}
