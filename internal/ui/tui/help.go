package tui

import (
	"github.com/charmbracelet/bubbles/key"
	"github.com/isaacphi/slop/internal/ui/tui/keymap"
)

// ShortHelp returns keybindings for the mini help view
func (m Model) ShortHelp() []key.Binding {
	km := m.GetKeyMap()
	return km.Groups[keymap.SystemGroup]
}

// FullHelp returns keybindings organized by their groups
func (m Model) FullHelp() [][]key.Binding {
	keyMap := m.GetKeyMap()

	// Convert the groups map into a slice of columns
	var result [][]key.Binding

	// Add groups in the order you want them displayed
	// For each group that exists, add it as a column
	for _, groupID := range []int{keymap.SystemGroup, keymap.NavigationGroup, keymap.ActionGroup} {
		if bindings, exists := keyMap.Groups[groupID]; exists && len(bindings) > 0 {
			result = append(result, bindings)
		}
	}

	// If no groups had bindings, return empty result
	if len(result) == 0 {
		return [][]key.Binding{}
	}

	return result
}
