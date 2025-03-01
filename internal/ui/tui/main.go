package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/isaacphi/slop/internal/ui/tui/focus"
	"github.com/isaacphi/slop/internal/ui/tui/keymap"
	"github.com/isaacphi/slop/internal/ui/tui/screens"
	"github.com/isaacphi/slop/internal/ui/tui/screens/chat"
	"github.com/isaacphi/slop/internal/ui/tui/screens/home"
	"github.com/isaacphi/slop/internal/ui/tui/theme"
)

// Config represents the configuration for the TUI
type Config struct {
	// Add any configuration options here
	Theme   *theme.Theme
	KeyMaps *keymap.GlobalKeyMap
}

// Model represents the application state
type Model struct {
	config       *Config
	currentView  screens.ScreenType
	homeScreen   *home.Model
	chatScreen   *chat.Model
	focusManager *focus.Manager
	width        int
	height       int
}

// StartTUI initializes and runs the TUI
func StartTUI(config interface{}) error {
	// Convert the config to our internal format
	tuiConfig := &Config{
		Theme:   theme.DefaultTheme(),
		KeyMaps: keymap.NewGlobalKeyMap(),
	}

	// Initialize the program with our model
	p := tea.NewProgram(initialModel(tuiConfig), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("error running TUI: %w", err)
	}
	return nil
}

// initialModel creates the initial model for the TUI
func initialModel(config *Config) Model {
	// Create focus manager that starts in NavFocus mode
	focusManager := focus.New()

	// Create the screens
	// For now, only the chat screen is updated to use the focus manager
	homeScreen := home.New(config.Theme, config.KeyMaps)
	chatScreen := chat.New(config.Theme, config.KeyMaps, focusManager)

	return Model{
		config:       config,
		currentView:  screens.HomeScreen,
		homeScreen:   &homeScreen,
		chatScreen:   &chatScreen,
		focusManager: focusManager,
	}
}

// Init initializes the TUI
func (m Model) Init() tea.Cmd {
	return nil
}

// Update handles updates to the TUI
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmds []tea.Cmd
		cmd  tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		// Update the window size
		m.width = msg.Width
		m.height = msg.Height

		// Update all screens with the new size
		m.homeScreen.SetSize(msg.Width, msg.Height)
		m.chatScreen.SetSize(msg.Width, msg.Height)

	case screens.ScreenChangeMsg:
		// Change the current view
		m.currentView = msg.Screen

		// Return appropriate initialization commands
		switch m.currentView {
		case screens.HomeScreen:
			cmd = m.homeScreen.Init()
		case screens.ChatScreen:
			// Reset focus mode when switching to chat screen to ensure it's consistent
			m.focusManager.BlurAll()
			cmd = m.chatScreen.Init()
		}
		cmds = append(cmds, cmd)
	}

	// Update the current screen
	var nextModel tea.Model

	switch m.currentView {
	case screens.HomeScreen:
		nextModel, cmd = m.homeScreen.Update(msg)
		if nextHomeScreen, ok := nextModel.(*home.Model); ok {
			m.homeScreen = nextHomeScreen
		}
	case screens.ChatScreen:
		nextModel, cmd = m.chatScreen.Update(msg)
		if nextChatScreen, ok := nextModel.(*chat.Model); ok {
			m.chatScreen = nextChatScreen
		}
	}

	cmds = append(cmds, cmd)
	return m, tea.Batch(cmds...)
}

// View renders the TUI
func (m Model) View() string {
	// Render the current screen
	switch m.currentView {
	case screens.HomeScreen:
		return m.homeScreen.View()
	case screens.ChatScreen:
		return m.chatScreen.View()
	default:
		return "Error: Unknown screen"
	}
}
