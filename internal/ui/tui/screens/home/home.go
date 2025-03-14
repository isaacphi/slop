package home

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/isaacphi/slop/internal/config"
	"github.com/isaacphi/slop/internal/ui/tui/keymap"
)

// Model represents the home screen
type Model struct {
	width  int
	height int
	mode   keymap.AppMode
	keyMap *config.KeyMap
}

// New creates a new home screen model
func New(keyMap *config.KeyMap) Model {
	return Model{
		keyMap: keyMap,
	}
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
