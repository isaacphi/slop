package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/isaacphi/slop/internal/ui/tui/keymap"
	"github.com/isaacphi/slop/internal/ui/tui/screens/chat"
	"github.com/isaacphi/slop/internal/ui/tui/screens/home"
)

// Model represents the application state
type Model struct {
	currentScreen ScreenType
	help          help.Model
	width         int
	height        int
	mode          keymap.AppMode
	homeScreen    home.Model
	chatScreen    chat.Model
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
		mode:          keymap.NormalMode,
		homeScreen:    home.New(),
		chatScreen:    chat.New(),
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
	case keymap.SetModeMsg:
		m.mode = msg.Mode
		return m, nil

	case tea.KeyMsg:
		// In input mode, only intercept a very limited set of keys
		// and pass everything else to the active input component
		if m.mode == keymap.InputMode {
			// Only handle escape and possibly other critical keys
			if msg.String() == "esc" {
				return m, tea.Cmd(func() tea.Msg {
					return keymap.SetModeMsg{Mode: keymap.NormalMode}
				})
			}

			// For all other keys, just pass them to the active screen
			var cmd tea.Cmd
			switch m.currentScreen {
			case HomeScreen:
				var newHome home.Model
				newHome, cmd = m.homeScreen.Update(msg)
				m.homeScreen = newHome
			case ChatScreen:
				var newChat chat.Model
				newChat, cmd = m.chatScreen.Update(msg)
				m.chatScreen = newChat
			}
			return m, cmd
		}

		// In normal mode, process all key bindings
		keyMap := m.GetKeyMap(m.mode)

		// Check against current keymap
		for _, binding := range keyMap.AllBindings() {
			if key.Matches(msg, binding) {
				// Handle global keys
				switch binding.Help().Key {
				case "q":
					return m, tea.Quit
				case "?":
					m.help.ShowAll = !m.help.ShowAll
					return m, nil
				case "c":
					if m.mode == keymap.NormalMode {
						m.currentScreen = ChatScreen
						return m, nil
					}
				case "h":
					if m.mode == keymap.NormalMode {
						m.currentScreen = HomeScreen
						return m, nil
					}
				}
			}
		}

		// If no global key matched, update the current screen
		switch m.currentScreen {
		case HomeScreen:
			newHome, cmd := m.homeScreen.Update(msg)
			m.homeScreen = newHome
			cmds = append(cmds, cmd)
		case ChatScreen:
			newChat, cmd := m.chatScreen.Update(msg)
			m.chatScreen = newChat
			cmds = append(cmds, cmd)
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		// Update screen dimensions
		homeScreen, cmd1 := m.homeScreen.Update(msg)
		m.homeScreen = homeScreen
		cmds = append(cmds, cmd1)

		chatScreen, cmd2 := m.chatScreen.Update(msg)
		m.chatScreen = chatScreen
		cmds = append(cmds, cmd2)
	}

	return m, tea.Batch(cmds...)
}

// View renders the TUI
func (m Model) View() string {
	helpHeight := m.getHelpHeight()

	bodyStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		Width(m.width - 2).
		Height(m.height - 2 - helpHeight)

	helpStyle := lipgloss.NewStyle()

	var body string
	switch m.currentScreen {
	case HomeScreen:
		body = m.homeScreen.View()
	case ChatScreen:
		body = m.chatScreen.View()
	default:
		body = "Invalid screen"
	}

	return lipgloss.JoinVertical(
		lipgloss.Top,
		bodyStyle.Render(body),
		helpStyle.Render(m.help.View(m)),
	)
}

func (m Model) getHelpHeight() int {
	if !m.help.ShowAll {
		return 1 // One line for short help
	}
	height := 1
	for _, row := range m.FullHelp() {
		if len(row) > height {
			height = len(row)
		}
	}
	return height
}
