package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/help"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/isaacphi/slop/internal/config"
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
	// TODO: I'm not happy with how mode is passed down
	mode       keymap.AppMode
	homeScreen home.Model
	chatScreen chat.Model
	keyMap     *config.KeyMap
}

type ScreenType int

const (
	HomeScreen ScreenType = iota
	ChatScreen
)

// StartTUI initializes and runs the TUI
func StartTUI(keyMap *config.KeyMap) error {
	p := tea.NewProgram(Model{
		help:          help.New(),
		currentScreen: HomeScreen,
		mode:          keymap.NormalMode,
		homeScreen:    home.New(keyMap),
		chatScreen:    chat.New(keyMap),
		keyMap:        keyMap,
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

		// Pass the adjusted size message to the screens
		homeScreen, _ := m.homeScreen.Update(msg)
		m.homeScreen = homeScreen

		chatScreen, _ := m.chatScreen.Update(msg)
		m.chatScreen = chatScreen

		return m, nil

	case tea.KeyMsg:
		// First, handle ctrl-c
		if msg.Type == tea.KeyCtrlC {
			return m, tea.Quit
		}

		// If in input mode, pass the key directly to the child view
		if m.mode == keymap.InputMode {
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

		keyMap := m.GetKeyMap()
		keyStr := msg.String()

		if action, exists := keyMap.KeyToActionMap[keyStr]; exists {
			switch action {
			case config.KeyActionQuit:
				return m, tea.Quit

			case config.KeyActionToggleHelp:
				m.help.ShowAll = !m.help.ShowAll
				return m, func() tea.Msg {
					return tea.WindowSizeMsg{
						Width:  m.width,
						Height: m.height,
					}
				}

			case config.KeyActionSwitchChat:
				m.currentScreen = ChatScreen
				return m, nil

			case config.KeyActionSwitchHome:
				m.currentScreen = HomeScreen
				return m, nil
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

		helpHeight := m.getHelpHeight()

		// Create a new WindowSizeMsg with adjusted height for the content area
		contentSizeMsg := tea.WindowSizeMsg{
			Width:  msg.Width - 2,
			Height: msg.Height - helpHeight - 2,
		}

		// Pass the adjusted size message to the screens
		homeScreen, cmd1 := m.homeScreen.Update(contentSizeMsg)
		m.homeScreen = homeScreen
		cmds = append(cmds, cmd1)

		chatScreen, cmd2 := m.chatScreen.Update(contentSizeMsg)
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
