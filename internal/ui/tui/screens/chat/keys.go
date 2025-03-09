package chat

import (
	"github.com/isaacphi/slop/internal/config"
	"github.com/isaacphi/slop/internal/ui/tui/keymap"
)

// GetKeyMap returns chat screen specific keybindings
func (m Model) GetKeyMap() keymap.KeyMap {
	km := keymap.NewKeyMap(m.keyMap)

	mode := m.mode

	if mode == keymap.NormalMode {
		km.AddAction(keymap.NavigationGroup, config.KeyActionSwitchHome, "home screen")
		km.AddAction(keymap.SystemGroup, config.KeyActionInputMode, "input mode")
		km.AddAction(keymap.NavigationGroup, config.KeyActionScrollDown, "scroll down")
		km.AddAction(keymap.NavigationGroup, config.KeyActionScrollUp, "scroll up")
	} else if mode == keymap.InputMode {
		// No global key bindings in input mode
		// except for 'enter' to send message and 'esc' (handled in main model)
		km.AddAction(keymap.SystemGroup, config.KeyActionSendMessage, "send message")
	}
	return km
}
