package tui

import (
	"github.com/charmbracelet/bubbles/key"
	"github.com/isaacphi/slop/internal/ui/tui/keymap"
)

// GetKeyMap returns all relevant keybindings for the current state
func (m Model) GetKeyMap(mode keymap.AppMode) keymap.KeyMap {
	keyMap := keymap.NewKeyMap()

	// Only add global keys in normal mode
	if mode == keymap.NormalMode {
		// Add global keys
		keyMap.Add(
			keymap.SystemGroup,
			key.NewBinding(
				key.WithKeys("q", "ctrl+c"),
				key.WithHelp("q", "quit"),
			))
		keyMap.Add(
			keymap.SystemGroup,
			key.NewBinding(
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
		// In input mode, only add a very selective set of global keys
		// that should work even in input mode (like escape)
		keyMap.Add(
			keymap.SystemGroup,
			key.NewBinding(
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
