package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/isaacphi/slop/internal/ui/tui/keymap"
	"github.com/isaacphi/slop/internal/ui/tui/screens/Chat"
	"github.com/isaacphi/slop/internal/ui/tui/screens/Home"
)

// Model represents the application state
type Model struct {
	currentScreen ScreenType
	help          help.Model
	width         int
	height        int
	mode          keymap.AppMode
	homeScreen    Home.Model
	chatScreen    Chat.Model
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
		homeScreen:    Home.New(),
		chatScreen:    Chat.New(),
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

// GetKeyMap returns all relevant keybindings for the current state
func (m Model) GetKeyMap(mode keymap.AppMode) keymap.KeyMap {
	keyMap := keymap.NewKeyMap()

	// Only add global keys in normal mode
	if mode == keymap.NormalMode {
		// Add global keys
		keyMap.Add(key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		))
		keyMap.Add(key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "toggle help"),
		))

		// Add keys from the current screen
		switch m.currentScreen {
		case HomeScreen:
			keyMap.Merge(m.homeScreen.GetKeyMap(mode))
		case ChatScreen:
			keyMap.Merge(m.chatScreen.GetKeyMap(mode))
		}
	} else if mode == keymap.InputMode {
		// In input mode, only add input-specific keys
		keyMap.Add(key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "exit input mode"),
		))

		// Add input mode keys from current screen
		switch m.currentScreen {
		case HomeScreen:
			keyMap.Merge(m.homeScreen.GetKeyMap(mode))
		case ChatScreen:
			keyMap.Merge(m.chatScreen.GetKeyMap(mode))
		}
	}

	return keyMap
}

// Update handles updates to the TUI
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case keymap.SetModeMsg:
		m.mode = msg.Mode
		return m, nil

	case tea.KeyMsg:
		// Get current keymap based on mode
		keyMap := m.GetKeyMap(m.mode)

		// Check against current keymap
		for _, binding := range keyMap.Keys {
			if key.Matches(msg, binding) {
				// Handle global keys
				switch binding.Help().Key {
				case "q":
					return m, tea.Quit
				case "?":
					m.help.ShowAll = !m.help.ShowAll
				case "esc":
					if m.mode == keymap.InputMode {
						return m, tea.Cmd(func() tea.Msg {
							return keymap.SetModeMsg{Mode: keymap.NormalMode}
						})
					}
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

		// Update the current screen
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
	}

	return m, tea.Batch(cmds...)
}

// ShortHelp returns keybindings for the mini help view
func (m Model) ShortHelp() []key.Binding {
	km := m.GetKeyMap(m.mode)
	if len(km.Keys) <= 2 {
		return km.Keys
	}
	return km.Keys[:2]
}

// calculateBindingWidth estimates the rendered width of a key binding
func calculateBindingWidth(binding key.Binding) int {
	help := binding.Help()
	// Format is typically: "key: description" with padding
	return len(help.Key) + len(help.Desc) + 4 // +4 for ": " and some padding
}

// FullHelp returns keybindings for the expanded help view, optimized for screen width
func (m Model) FullHelp() [][]key.Binding {
	keyMap := m.GetKeyMap(m.mode)

	// No bindings? Return empty result
	if len(keyMap.Keys) == 0 {
		return [][]key.Binding{}
	}

	// Calculate approximate width for each binding
	bindingWidths := make([]int, len(keyMap.Keys))
	for i, binding := range keyMap.Keys {
		bindingWidths[i] = calculateBindingWidth(binding)
	}

	// Use screen width with some margin
	availableWidth := m.width - 4 // 4 for margins

	// Pack bindings into rows to minimize height
	var result [][]key.Binding
	var currentRow []key.Binding
	currentWidth := 0

	for i, binding := range keyMap.Keys {
		// If adding this binding exceeds width and row isn't empty, start a new row
		if currentWidth+bindingWidths[i] > availableWidth && len(currentRow) > 0 {
			result = append(result, currentRow)
			currentRow = []key.Binding{}
			currentWidth = 0
		}

		// Add binding to current row
		currentRow = append(currentRow, binding)
		currentWidth += bindingWidths[i]
	}

	// Add the last row if it has any bindings
	if len(currentRow) > 0 {
		result = append(result, currentRow)
	}

	return result
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

	// Calculate height based on the number of rows in full help
	rows := len(m.FullHelp())
	if rows == 0 {
		return 1 // At least one line even with no bindings
	}
	return rows
}
