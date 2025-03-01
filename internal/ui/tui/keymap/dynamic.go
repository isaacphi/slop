package keymap

import (
	"github.com/charmbracelet/bubbles/key"
	"github.com/isaacphi/slop/internal/ui/tui/focus"
	"github.com/isaacphi/slop/internal/ui/tui/screens"
)

// Context represents the application state context for determining available keys
type Context struct {
	FocusManager *focus.Manager
	Screen       screens.ScreenType
	// Add other app state that might affect key availability
	IsProcessing bool
	HasSelection bool
}

// DynamicKeyMap provides context-sensitive keybindings
type DynamicKeyMap struct {
	globalKeys         *GlobalKeyMap
	contextualBindings map[string]func(Context) []key.Binding
}

// NewDynamicKeyMap creates a new dynamic keymap
func NewDynamicKeyMap(global *GlobalKeyMap) *DynamicKeyMap {
	return &DynamicKeyMap{
		globalKeys:         global,
		contextualBindings: make(map[string]func(Context) []key.Binding),
	}
}

// RegisterContextualBindings registers a set of contextual keybindings
func (km *DynamicKeyMap) RegisterContextualBindings(id string, bindingFn func(Context) []key.Binding) {
	km.contextualBindings[id] = bindingFn
}

// GetAvailableBindings returns all currently available bindings based on context
func (km *DynamicKeyMap) GetAvailableBindings(ctx Context) []key.Binding {
	var bindings []key.Binding

	// In nav mode, add global keys
	if ctx.FocusManager.GetMode() == screens.NavFocus {
		bindings = append(bindings, km.globalKeys.Quit)
		bindings = append(bindings, km.globalKeys.Help)

		// Add screen navigation based on current screen
		switch ctx.Screen {
		case screens.HomeScreen:
			bindings = append(bindings, km.globalKeys.Chat)
			bindings = append(bindings, km.globalKeys.Settings)
		case screens.ChatScreen:
			bindings = append(bindings, km.globalKeys.Home)
			bindings = append(bindings, km.globalKeys.Settings)
		}
	} else {
		// In input mode, only add ESC binding
		escBinding := key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "exit input mode"),
		)
		bindings = append(bindings, escBinding)
	}

	// Add contextual bindings
	for _, bindingFn := range km.contextualBindings {
		bindings = append(bindings, bindingFn(ctx)...)
	}

	return bindings
}

// ShortHelp returns keybindings for the mini help view
func (km *DynamicKeyMap) ShortHelp(ctx Context) []key.Binding {
	bindings := km.GetAvailableBindings(ctx)
	if len(bindings) > 4 {
		return bindings[:4] // Return first 4 if there are many
	}
	return bindings
}

// FullHelp organizes bindings for the full help view
func (km *DynamicKeyMap) FullHelp(ctx Context) [][]key.Binding {
	bindings := km.GetAvailableBindings(ctx)

	// Organize bindings into categories
	var navigation, actions, misc []key.Binding

	for _, b := range bindings {
		switch b.Help().Key {
		case "h", "c", "s":
			navigation = append(navigation, b)
		case "enter", "ctrl+s":
			actions = append(actions, b)
		default:
			misc = append(misc, b)
		}
	}

	result := [][]key.Binding{}
	if len(navigation) > 0 {
		result = append(result, navigation)
	}
	if len(actions) > 0 {
		result = append(result, actions)
	}
	if len(misc) > 0 {
		result = append(result, misc)
	}

	return result
}

