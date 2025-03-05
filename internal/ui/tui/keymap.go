package tui

import (
	"github.com/charmbracelet/bubbles/key"
)

type KeyMap struct {
	Up         key.Binding
	Down       key.Binding
	Quit       key.Binding
	Help       key.Binding
	ChatScreen key.Binding
	HomeScreen key.Binding
}

var keymap = KeyMap{
	Up: key.NewBinding(
		key.WithKeys("k", "up"),
		key.WithHelp("↑/k", "move up"),
	),
	Down: key.NewBinding(
		key.WithKeys("j", "down"),
		key.WithHelp("↓/j", "move down"),
	),
	Quit: key.NewBinding(
		key.WithKeys("ctrl+c", "q"),
		key.WithHelp("q", "quit"),
	),
	Help: key.NewBinding(
		key.WithKeys("?"),
		key.WithHelp("?", "toggle help"),
	),
	ChatScreen: key.NewBinding(
		key.WithKeys("c"),
		key.WithHelp("c", "chat"),
	),
	HomeScreen: key.NewBinding(
		key.WithKeys("h"),
		key.WithHelp("h", "help"),
	),
}

// ShortHelp returns keybindings to be shown in the mini help view. It's part
// of the key.Map interface.
func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{
		k.Quit,
		k.Help,
	}
}

// FullHelp returns keybindings for the expanded help view. It's part of the
// key.Map interface.
func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down},   // first column
		{k.Quit, k.Help}, // second column
	}
}
