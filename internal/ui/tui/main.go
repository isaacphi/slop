package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Model represents the application state
type Model struct {
	currentScreen ScreenType
	help          help.Model
	width         int
	height        int
}

type ScreenType int

const (
	HomeScreen ScreenType = iota
	ChatScreen
)

// StartTUI initializes and runs the TUI
func StartTUI() error {
	p := tea.NewProgram(Model{
		help:          help.New(),
		currentScreen: HomeScreen,
	}, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("error running TUI: %w", err)
	}
	return nil
}

// Init initializes the TUI
func (m Model) Init() tea.Cmd {
	return nil
}

// Update handles updates to the TUI
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keymap.Help):
			m.help.ShowAll = !m.help.ShowAll
		case key.Matches(msg, keymap.Quit):
			return m, tea.Quit
		case key.Matches(msg, keymap.HomeScreen):
			m.currentScreen = HomeScreen
		case key.Matches(msg, keymap.ChatScreen):
			m.currentScreen = ChatScreen
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}

	return m, tea.Batch(cmds...)
}

// View renders the TUI
func (m Model) View() string {
	bodyStyle := lipgloss.NewStyle().
		Align(lipgloss.Center).
		Underline(true).
		BorderStyle(lipgloss.RoundedBorder()).
		Width(m.width - 2).
		Height(m.height - 2 - m.getHelpHeight())

	helpStyle := lipgloss.NewStyle()

	return lipgloss.JoinVertical(
		lipgloss.Top,
		bodyStyle.Render(m.getBodyContent()),
		helpStyle.Render(m.help.View(keymap)),
	)
}

func (m *Model) getBodyContent() string {
	switch m.currentScreen {
	case HomeScreen:
		return "slop"
	case ChatScreen:
		return "chat"
	default:
		panic("invalid screen")
	}
}

func (m *Model) getHelpHeight() int {
	// TODO: make this dynamic
	if m.help.ShowAll {
		return 2
	} else {
		return 1
	}
}
