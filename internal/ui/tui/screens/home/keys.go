package home

import (
	"github.com/isaacphi/slop/internal/config"
	"github.com/isaacphi/slop/internal/ui/tui/keymap"
)

// GetKeyMap returns home screen specific keybindings
func (m Model) GetKeyMap() keymap.KeyMap {
	km := keymap.NewKeyMap(m.keyMap)
	mode := m.mode

	if mode == keymap.NormalMode {
		km.AddAction(keymap.NavigationGroup, config.KeyActionSwitchChat, "chat screen")
	}
	return km
}
