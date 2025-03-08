package keymap

import (
	"github.com/charmbracelet/bubbles/key"
)

// AppMode represents the application's current input mode
type AppMode int

const (
	NormalMode AppMode = iota
	InputMode
)

// KeyMap represents a set of keybindings
type KeyMap struct {
	Keys []key.Binding
}

// NewKeyMap creates a new empty keymap
func NewKeyMap() KeyMap {
	return KeyMap{
		Keys: []key.Binding{},
	}
}

// Add adds a key binding to the keymap
func (k *KeyMap) Add(binding key.Binding) {
	k.Keys = append(k.Keys, binding)
}

// Merge combines two keymaps
func (k *KeyMap) Merge(other KeyMap) {
	k.Keys = append(k.Keys, other.Keys...)
}

// KeyMapProvider extends tea.Model with keymap functionality
type KeyMapProvider interface {
	GetKeyMap(mode AppMode) KeyMap
}

// SetModeMsg is a message to change the application mode
type SetModeMsg struct {
	Mode AppMode
}
