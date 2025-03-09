package config

import "encoding/json"

// Key bindings
const (
	KeyActionQuit        = "quit"
	KeyActionToggleHelp  = "toggleHelp"
	KeyActionSwitchChat  = "switchToChat"
	KeyActionSwitchHome  = "switchToHome"
	KeyActionExitInput   = "exitInput"
	KeyActionInputMode   = "inputMode"
	KeyActionScrollDown  = "scrollDown"
	KeyActionScrollUp    = "scrollUp"
	KeyActionSendMessage = "sendMessage"
)

type KeyMap struct {
	Quit         []string `mapstructure:"quit" json:"quit" jsonschema:"description=Exit the application,default=q"`
	ToggleHelp   []string `mapstructure:"toggleHelp" json:"toggleHelp" jsonschema:"description=Toggle help display,default=?"`
	SwitchToChat []string `mapstructure:"switchToChat" json:"switchToChat" jsonschema:"description=Switch to chat screen,default=c"`
	SwitchToHome []string `mapstructure:"switchToHome" json:"switchToHome" jsonschema:"description=Switch to home screen,default=h"`
	ExitInput    []string `mapstructure:"exitInput" json:"exitInput" jsonschema:"description=Exit input mode,default=esc"`
	InputMode    []string `mapstructure:"inputMode" json:"inputMode" jsonschema:"description=Enter input mode,default=i"`
	ScrollDown   []string `mapstructure:"scrollDown" json:"scrollDown" jsonschema:"description=Scroll down in chat,default=j,down"`
	ScrollUp     []string `mapstructure:"scrollUp" json:"scrollUp" jsonschema:"description=Scroll up in chat,default=k,up"`
	SendMessage  []string `mapstructure:"sendMessage" json:"sendMessage" jsonschema:"description=Send a message,default=enter"`

	keyCache map[string][]string
}

// Get key bindings for an action
func (k *KeyMap) GetKeys(action string) []string {
	// Initialize cache if needed
	if k.keyCache == nil {
		k.keyCache = make(map[string][]string)
		jsonBytes, err := json.Marshal(k)
		if err != nil {
			return nil
		}
		if err := json.Unmarshal(jsonBytes, &k.keyCache); err != nil {
			return nil
		}
	}

	return k.keyCache[action]
}
