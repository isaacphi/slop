package chat

import (
	"github.com/charmbracelet/bubbles/key"
	"github.com/isaacphi/slop/internal/ui/tui/keymap"
)

// GetKeyMap returns chat screen specific keybindings
func (m Model) GetKeyMap(mode keymap.AppMode) keymap.KeyMap {
	km := keymap.NewKeyMap()

	if mode == keymap.NormalMode {
		km.Add(
			keymap.NavigationGroup,
			key.NewBinding(
				key.WithKeys("h"),
				key.WithHelp("h", "home screen"),
			))
		km.Add(
			keymap.SystemGroup,
			key.NewBinding(
				key.WithKeys("i"),
				key.WithHelp("i", "input mode"),
			))
		km.Add(
			keymap.NavigationGroup,
			key.NewBinding(
				key.WithKeys("j", "down"),
				key.WithHelp("j", "scroll down"),
			))
		km.Add(
			keymap.NavigationGroup,
			key.NewBinding(
				key.WithKeys("k", "up"),
				key.WithHelp("k", "scroll up"),
			))
	} else if mode == keymap.InputMode {
		// No global key bindings in input mode
		// except for 'enter' to send message and 'esc' (handled in main model)
		km.Add(
			keymap.SystemGroup,
			key.NewBinding(
				key.WithKeys("enter"),
				key.WithHelp("enter", "send message"),
			))
	}

	return km
}
