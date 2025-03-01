package focus

import "github.com/isaacphi/slop/internal/ui/tui/screens"

// FocusableComponent interface for components that can be focused
type FocusableComponent interface {
	Focus()
	Blur()
	IsFocused() bool
}

// Manager handles focus state across the application
type Manager struct {
	components   map[string]FocusableComponent
	currentFocus string
	mode         screens.FocusMode
}

// New creates a new focus manager
func New() *Manager {
	return &Manager{
		components:   make(map[string]FocusableComponent),
		currentFocus: "",
		mode:         screens.NavFocus,
	}
}

// Register adds a component to the focus manager
func (m *Manager) Register(id string, component FocusableComponent) {
	m.components[id] = component
}

// Unregister removes a component from the focus manager
func (m *Manager) Unregister(id string) {
	if m.currentFocus == id {
		m.BlurAll()
	}
	delete(m.components, id)
}

// SetFocus focuses a specific component
func (m *Manager) SetFocus(id string) {
	// Blur current focus
	if m.currentFocus != "" {
		if comp, exists := m.components[m.currentFocus]; exists {
			comp.Blur()
		}
	}
	
	// Focus new component
	if comp, exists := m.components[id]; exists {
		comp.Focus()
		m.currentFocus = id
		m.mode = screens.InputFocus
	}
}

// BlurAll blurs all components and switches to nav mode
func (m *Manager) BlurAll() {
	for _, comp := range m.components {
		comp.Blur()
	}
	m.currentFocus = ""
	m.mode = screens.NavFocus
}

// ToggleFocus toggles between input and navigation focus modes
func (m *Manager) ToggleFocus() {
	if m.mode == screens.InputFocus {
		m.BlurAll()
	} else if m.currentFocus != "" {
		m.SetFocus(m.currentFocus)
	}
}

// GetMode returns the current focus mode
func (m *Manager) GetMode() screens.FocusMode {
	return m.mode
}

// GetCurrentFocus returns the ID of the currently focused component
func (m *Manager) GetCurrentFocus() string {
	return m.currentFocus
}