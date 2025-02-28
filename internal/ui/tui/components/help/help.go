package help

import (
	"github.com/charmbracelet/bubbles/help"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/isaacphi/slop/internal/ui/tui/keymap"
	"github.com/isaacphi/slop/internal/ui/tui/theme"
)

// Model represents the help component
type Model struct {
	help    help.Model
	keys    keymap.KeyMap
	theme   *theme.Theme
	width   int
	ShowAll bool
}

// New creates a new help model
func New(km keymap.KeyMap, thm *theme.Theme) Model {
	helpModel := help.New()
	return Model{
		help:  helpModel,
		keys:  km,
		theme: thm,
		width: 80,
	}
}

// SetWidth sets the width of the help component
func (m *Model) SetWidth(width int) {
	m.width = width
	m.help.Width = width
}

// SetKeybindings sets the keybindings for the help component
func (m *Model) SetKeybindings(km keymap.KeyMap) {
	m.keys = km
}

// ShortHelp returns a minimal view of the keybindings
func (m Model) ShortHelp() string {
	return m.help.View(m.keys)
}

// FullHelp returns a full view of the keybindings
func (m Model) FullHelp() string {
	m.help.ShowAll = true
	defer func() { m.help.ShowAll = false }()
	return m.help.View(m.keys)
}

// Update handles updates to the help component
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetWidth(msg.Width)
	}
	return m, nil
}

// View renders the help component
func (m Model) View() string {
	var helpText string
	if m.ShowAll {
		helpText = m.FullHelp()
	} else {
		helpText = m.ShortHelp()
	}
	style := m.theme.FooterStyle.Copy().Width(m.width)
	return style.Render(helpText)
}