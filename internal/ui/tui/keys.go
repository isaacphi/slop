package tui

import (
	"github.com/isaacphi/slop/internal/config"
	"github.com/isaacphi/slop/internal/ui/tui/keymap"
)

// GetKeyMap returns all relevant keybindings for the current state
func (m Model) GetKeyMap() keymap.KeyMap {
	keyMap := keymap.NewKeyMap(m.keyMap)
	mode := m.mode

	// Only add global keys in normal mode
	if mode == keymap.NormalMode {
		// Add global keys
		keyMap.AddAction(keymap.SystemGroup, config.KeyActionQuit, "quit")
		keyMap.AddAction(keymap.SystemGroup, config.KeyActionToggleHelp, "toggle help")
		keyMap.AddAction(keymap.NavigationGroup, config.KeyActionSwitchChat, "switch to chat")
		keyMap.AddAction(keymap.NavigationGroup, config.KeyActionSwitchHome, "switch to home")

		// Add keys from the current screen
		switch m.currentScreen {
		case HomeScreen:
			keyMap.Merge(m.homeScreen.GetKeyMap())
		case ChatScreen:
			keyMap.Merge(m.chatScreen.GetKeyMap())
		}
	} else if mode == keymap.InputMode {
		// Handle input mode keys
		keyMap.AddAction(keymap.SystemGroup, config.KeyActionExitInput, "exit input mode")

		// Add input mode keys from current screen
		switch m.currentScreen {
		case HomeScreen:
			keyMap.Merge(m.homeScreen.GetKeyMap())
		case ChatScreen:
			keyMap.Merge(m.chatScreen.GetKeyMap())
		}
	}

	return keyMap
}
