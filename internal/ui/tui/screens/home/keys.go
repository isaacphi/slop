package home

import (
	"github.com/charmbracelet/bubbles/key"
	"github.com/isaacphi/slop/internal/ui/tui/keymap"
)

// GetKeyMap returns home screen specific keybindings
func (m Model) GetKeyMap(mode keymap.AppMode) keymap.KeyMap {
	km := keymap.NewKeyMap()

	if mode == keymap.NormalMode {
		km.Add(
			keymap.NavigationGroup,
			key.NewBinding(
				key.WithKeys("c"),
				key.WithHelp("c", "chat screen"),
			))
		km.Add(
			keymap.NavigationGroup,
			key.NewBinding(
				key.WithKeys("j", "down"),
				key.WithHelp("j", "move down"),
			))
		km.Add(
			keymap.NavigationGroup,
			key.NewBinding(
				key.WithKeys("k", "up"),
				key.WithHelp("k", "move up"),
			))
	}

	return km
}
