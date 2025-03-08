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
	Groups map[int][]key.Binding
}

// NewKeyMap creates a new empty keymap
func NewKeyMap() KeyMap {
	return KeyMap{
		Groups: make(map[int][]key.Binding),
	}
}

type KeyGroup int

const (
	SystemGroup = iota
	NavigationGroup
	ActionGroup
)

// Add adds a key binding to the keymap
func (k *KeyMap) Add(group int, binding key.Binding) {
	if _, exists := k.Groups[group]; !exists {
		k.Groups[group] = []key.Binding{}
	}
	k.Groups[group] = append(k.Groups[group], binding)
}

// Merge combines two keymaps
func (k *KeyMap) Merge(other KeyMap) {
	for group, bindings := range other.Groups {
		for _, binding := range bindings {
			k.Add(group, binding)
		}
	}
}

// AllBindings returns all bindings regardless of group
func (k *KeyMap) AllBindings() []key.Binding {
	var allBindings []key.Binding
	for _, bindings := range k.Groups {
		allBindings = append(allBindings, bindings...)
	}
	return allBindings
}

// KeyMapProvider extends tea.Model with keymap functionality
type KeyMapProvider interface {
	GetKeyMap(mode AppMode) KeyMap
}

// SetModeMsg is a message to change the application mode
type SetModeMsg struct {
	Mode AppMode
}
