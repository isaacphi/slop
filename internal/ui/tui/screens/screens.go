package screens

import ()

// ScreenType represents a type of screen
type ScreenType int

const (
	HomeScreen ScreenType = iota
	ChatScreen
	SettingsScreen
)

// ScreenChangeMsg is sent when a screen change is requested
type ScreenChangeMsg struct {
	Screen ScreenType
}

// FocusMode represents the current focus state
type FocusMode int

const (
	InputFocus FocusMode = iota // Focus is on text input
	NavFocus                     // Focus is on navigation (hotkeys active)
)