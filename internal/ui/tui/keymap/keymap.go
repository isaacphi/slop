package keymap

import (
	"github.com/charmbracelet/bubbles/key"
	"github.com/isaacphi/slop/internal/config"
)

// AppMode represents the application's current input mode
type AppMode int

const (
	NormalMode AppMode = iota
	InputMode
)

// KeyMap represents a set of keybindings
type KeyMap struct {
	Groups         map[int][]key.Binding
	KeyToActionMap map[string]string // Maps key combinations to action names
	userKeyMap     *config.KeyMap    // Reference to user key configurations
}

// NewKeyMap creates a new empty keymap
func NewKeyMap(userKeyMap *config.KeyMap) KeyMap {
	return KeyMap{
		Groups:         make(map[int][]key.Binding),
		KeyToActionMap: make(map[string]string),
		userKeyMap:     userKeyMap,
	}
}

type KeyGroup int

const (
	SystemGroup = iota
	NavigationGroup
	ActionGroup
)

// AddAction adds an action with its keys to the keymap
func (k *KeyMap) AddAction(group int, actionName string, helpText string) {
	keyList := k.userKeyMap.GetKeys(actionName)

	if len(keyList) == 0 {
		return // Skip if no keys available
	}

	binding := key.NewBinding(
		key.WithKeys(keyList...),
		key.WithHelp(keyList[0], helpText),
	)

	if _, exists := k.Groups[group]; !exists {
		k.Groups[group] = []key.Binding{}
	}
	k.Groups[group] = append(k.Groups[group], binding)

	// Add each key to the action map
	for _, keyName := range keyList {
		k.KeyToActionMap[keyName] = actionName
	}
}

// Merge combines two keymaps
func (k *KeyMap) Merge(other KeyMap) {
	for group, bindings := range other.Groups {
		for _, binding := range bindings {
			// Find the associated action
			for _, key := range binding.Keys() {
				if action, ok := other.KeyToActionMap[key]; ok {
					k.AddAction(group, action, binding.Help().Desc)
					break
				}
			}
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
	GetKeyMap() KeyMap
}

// SetModeMsg is a message to change the application mode
type SetModeMsg struct {
	Mode AppMode
}
