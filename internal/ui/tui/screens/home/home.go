package home

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/isaacphi/slop/internal/ui/tui/keymap"
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

// GetKeyMap returns home screen specific keybindings
func (m Model) GetKeyMap(mode keymap.AppMode) keymap.KeyMap {
	km := keymap.NewKeyMap()

	if mode == keymap.NormalMode {
		km.Add(
			keymap.NavigationGroup,
			key.NewBinding(
				key.WithKeys("c"),
				key.WithHelp("c", "chat screen"),
			))
		km.Add(
			keymap.NavigationGroup,
			key.NewBinding(
				key.WithKeys("j", "down"),
				key.WithHelp("j", "move down"),
			))
		km.Add(
			keymap.NavigationGroup,
			key.NewBinding(
				key.WithKeys("k", "up"),
				key.WithHelp("k", "move up"),
			))
	}

	return km
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
