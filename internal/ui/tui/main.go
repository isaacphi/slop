package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/isaacphi/slop/internal/ui/tui/keymap"
	"github.com/isaacphi/slop/internal/ui/tui/screens"
	"github.com/isaacphi/slop/internal/ui/tui/screens/chat"
	"github.com/isaacphi/slop/internal/ui/tui/screens/home"
	"github.com/isaacphi/slop/internal/ui/tui/screens/settings"
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
	config      *Config
	currentView screens.ScreenType
	homeScreen  *home.Model
	chatScreen  *chat.Model
	settingsScreen *settings.Model
	width       int
	height      int
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
	// Create the screens
	homeScreen := home.New(config.Theme, config.KeyMaps)
	chatScreen := chat.New(config.Theme, config.KeyMaps)
	settingsScreen := settings.New(config.Theme, config.KeyMaps)

	return Model{
		config:      config,
		currentView: screens.HomeScreen,
		homeScreen:  &homeScreen,
		chatScreen:  &chatScreen,
		settingsScreen: &settingsScreen,
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
		m.settingsScreen.SetSize(msg.Width, msg.Height)
		
	case screens.ScreenChangeMsg:
		// Change the current view
		m.currentView = msg.Screen
		
		// Return appropriate initialization commands
		switch m.currentView {
		case screens.HomeScreen:
			cmd = m.homeScreen.Init()
		case screens.ChatScreen:
			cmd = m.chatScreen.Init()
		case screens.SettingsScreen:
			cmd = m.settingsScreen.Init()
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
	case screens.SettingsScreen:
		nextModel, cmd = m.settingsScreen.Update(msg)
		if nextSettingsScreen, ok := nextModel.(*settings.Model); ok {
			m.settingsScreen = nextSettingsScreen
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
	case screens.SettingsScreen:
		return m.settingsScreen.View()
	default:
		return "Error: Unknown screen"
	}
}
