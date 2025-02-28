package keymap

import (
	"github.com/charmbracelet/bubbles/key"
)

// KeyMap is the interface that all keymaps must implement
type KeyMap interface {
	ShortHelp() []key.Binding
	FullHelp() [][]key.Binding
}

// GlobalKeyMap contains the keybindings that are common across all screens
type GlobalKeyMap struct {
	Quit     key.Binding
	Help     key.Binding
	Settings key.Binding
	Home     key.Binding
	Chat     key.Binding
}

// NewGlobalKeyMap returns a new GlobalKeyMap with default bindings
func NewGlobalKeyMap() *GlobalKeyMap {
	return &GlobalKeyMap{
		Quit: key.NewBinding(
			key.WithKeys("ctrl+c", "q"),
			key.WithHelp("q/ctrl+c", "quit"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
		Settings: key.NewBinding(
			key.WithKeys("s"),
			key.WithHelp("s", "settings"),
		),
		Home: key.NewBinding(
			key.WithKeys("h"),
			key.WithHelp("h", "home"),
		),
		Chat: key.NewBinding(
			key.WithKeys("c"),
			key.WithHelp("c", "chat"),
		),
	}
}

// ShortHelp returns keybindings to be shown in the mini help view
func (k GlobalKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Help, k.Quit}
}

// FullHelp returns keybindings for the expanded help view
func (k GlobalKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Home, k.Chat, k.Settings},
		{k.Help, k.Quit},
	}
}

// HomeKeyMap contains keybindings specific to the home screen
type HomeKeyMap struct {
	*GlobalKeyMap
}

// NewHomeKeyMap returns a new HomeKeyMap with default bindings
func NewHomeKeyMap(global *GlobalKeyMap) *HomeKeyMap {
	return &HomeKeyMap{
		GlobalKeyMap: global,
	}
}

// ChatKeyMap contains keybindings specific to the chat screen
type ChatKeyMap struct {
	*GlobalKeyMap
	Send key.Binding
}

// NewChatKeyMap returns a new ChatKeyMap with default bindings
func NewChatKeyMap(global *GlobalKeyMap) *ChatKeyMap {
	return &ChatKeyMap{
		GlobalKeyMap: global,
		Send: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "send message"),
		),
	}
}

// ShortHelp returns keybindings to be shown in the mini help view
func (k ChatKeyMap) ShortHelp() []key.Binding {
	return append(k.GlobalKeyMap.ShortHelp(), k.Send)
}

// FullHelp returns keybindings for the expanded help view
func (k ChatKeyMap) FullHelp() [][]key.Binding {
	bindings := k.GlobalKeyMap.FullHelp()
	bindings = append(bindings, []key.Binding{k.Send})
	return bindings
}

// SettingsKeyMap contains keybindings specific to the settings screen
type SettingsKeyMap struct {
	*GlobalKeyMap
	Save key.Binding
}

// NewSettingsKeyMap returns a new SettingsKeyMap with default bindings
func NewSettingsKeyMap(global *GlobalKeyMap) *SettingsKeyMap {
	return &SettingsKeyMap{
		GlobalKeyMap: global,
		Save: key.NewBinding(
			key.WithKeys("ctrl+s"),
			key.WithHelp("ctrl+s", "save settings"),
		),
	}
}

// ShortHelp returns keybindings to be shown in the mini help view
func (k SettingsKeyMap) ShortHelp() []key.Binding {
	return append(k.GlobalKeyMap.ShortHelp(), k.Save)
}

// FullHelp returns keybindings for the expanded help view
func (k SettingsKeyMap) FullHelp() [][]key.Binding {
	bindings := k.GlobalKeyMap.FullHelp()
	bindings = append(bindings, []key.Binding{k.Save})
	return bindings
}