package chat

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/isaacphi/slop/internal/ui/tui/components/help"
	"github.com/isaacphi/slop/internal/ui/tui/components/input"
	"github.com/isaacphi/slop/internal/ui/tui/focus"
	"github.com/isaacphi/slop/internal/ui/tui/keymap"
	"github.com/isaacphi/slop/internal/ui/tui/layout"
	"github.com/isaacphi/slop/internal/ui/tui/screens"
	"github.com/isaacphi/slop/internal/ui/tui/theme"
)

// Message represents a chat message
type Message struct {
	Content string
	IsUser  bool
}

// Model represents the chat screen
type Model struct {
	keyMap       *keymap.ChatKeyMap
	dynamicKeys  *keymap.DynamicKeyMap
	input        input.Model
	help         help.Model
	theme        *theme.Theme
	width        int
	height       int
	messages     []Message
	focusManager *focus.Manager
	inputID      string
}

// New creates a new chat screen
func New(thm *theme.Theme, globalKeyMap *keymap.GlobalKeyMap, focusMgr *focus.Manager) Model {
	km := keymap.NewChatKeyMap(globalKeyMap)
	dynamicKeys := keymap.NewDynamicKeyMap(globalKeyMap)
	inputModel := input.New(thm)
	helpModel := help.New(km, thm)

	// Define a unique ID for the input component
	const inputID = "chat-input"

	// Register the input component with the focus manager
	focusMgr.Register(inputID, &inputModel)

	// Register contextual keybindings
	dynamicKeys.RegisterContextualBindings("chat-input", func(ctx keymap.Context) []key.Binding {
		if ctx.FocusManager.GetMode() == screens.InputFocus && ctx.FocusManager.GetCurrentFocus() == inputID {
			return []key.Binding{km.Send}
		}
		return nil
	})

	// Do not set initial focus - we start in Nav mode

	return Model{
		keyMap:      km,
		dynamicKeys: dynamicKeys,
		input:       inputModel,
		help:        helpModel,
		theme:       thm,
		messages: []Message{
			{Content: "Hello! How can I help you today?", IsUser: false},
		},
		focusManager: focusMgr,
		inputID:      inputID,
	}
}

// SetSize sets the size of the chat screen
func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.input.SetWidth(width)
	m.help.SetWidth(width)
}

// Init initializes the chat screen
func (m *Model) Init() tea.Cmd {
	// Always start in nav mode, requiring user to press ESC to focus input
	// This ensures consistent behavior
	m.focusManager.BlurAll()
	return nil
}

// Update handles updates to the chat screen
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Handle focus mode switching with ESC
		if msg.String() == "esc" {
			// Always toggle focus explicitly and directly
			if m.focusManager.GetMode() == screens.InputFocus {
				m.focusManager.BlurAll()
				// Force blur again to be sure
				m.input.Blur()
			} else {
				m.focusManager.SetFocus(m.inputID)
				// Force focus again to be sure
				m.input.Focus()
			}
			return m, nil
		}

		// Handle keys based on focus mode - only when in navigation mode
		if m.focusManager.GetMode() == screens.NavFocus {
			// Navigation mode key handling
			switch msg.String() {
			case "q":
				return m, tea.Quit
			case "h":
				return m, func() tea.Msg {
					return screens.ScreenChangeMsg{Screen: screens.HomeScreen}
				}
			case "c":
				// No-op for already being on chat screen
				return m, nil
			case "?":
				// Toggle help
				m.help.ShowAll = !m.help.ShowAll
				return m, nil
			}
		}

	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)

	case input.InputSubmitMsg:
		// Add the user's message
		m.messages = append(m.messages, Message{
			Content: msg.Value,
			IsUser:  true,
		})

		// Simulate a response (in a real app, this would come from an LLM)
		cmds = append(cmds, func() tea.Msg {
			return chatResponseMsg{content: fmt.Sprintf("Response to: %s", msg.Value)}
		})

	case chatResponseMsg:
		// Add the assistant's message
		m.messages = append(m.messages, Message{
			Content: msg.content,
			IsUser:  false,
		})
	}

	// Only update input if we're in input focus mode
	if m.focusManager.GetMode() == screens.InputFocus {
		m.input, cmd = m.input.Update(msg)
		cmds = append(cmds, cmd)
	} else {
		// Even if we're not handling input updates, ensure the input is properly blurred
		if m.input.IsFocused() {
			m.input.Blur()
		}
	}

	// Always update help
	var helpModel help.Model
	helpModel, cmd = m.help.Update(msg)
	m.help = helpModel
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

// View renders the chat screen
func (m Model) View() string {
	var b strings.Builder

	// Get focus mode and prepare focus hint
	focusMode := m.focusManager.GetMode()
	focusHint := "ESC: " + (map[screens.FocusMode]string{
		screens.InputFocus: "exit input mode",
		screens.NavFocus:   "enter input mode",
	})[focusMode]

	// Use layout package to get consistent layout
	layoutResult := layout.LayoutScreen(m.width, m.height, m.help, m.theme, focusHint)

	// Render chat messages
	messagesStr := m.renderMessages(layoutResult.ContentHeight - 3) // Subtract space for input

	// Add focus indicator to input if in nav mode
	inputView := m.input.View()
	if focusMode == screens.NavFocus {
		inputView = m.theme.DocStyle.Copy().
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(m.theme.Subtle).
			Render(inputView)
	}

	// Combine content and input areas
	contentView := layout.RenderContentWithInput(messagesStr, inputView, layoutResult.ContentHeight)

	// Build the complete view
	b.WriteString(contentView)
	b.WriteString("\n\n")
	b.WriteString(layoutResult.HelpView)

	return b.String()
}

// renderMessages renders the chat messages
func (m Model) renderMessages(maxHeight int) string {
	var b strings.Builder

	// Style for user messages
	userStyle := m.theme.MessageStyle.Copy().
		Foreground(m.theme.Primary).
		Width(m.width)

	// Style for assistant messages
	assistantStyle := m.theme.MessageStyle.Copy().
		Foreground(m.theme.Secondary).
		Width(m.width)

	// Calculate how many messages we can display
	messages := m.messages
	if len(messages) > maxHeight {
		messages = messages[len(messages)-maxHeight:]
	}

	// Render each message
	for _, msg := range messages {
		if msg.IsUser {
			b.WriteString(userStyle.Render(fmt.Sprintf("You: %s", msg.Content)))
		} else {
			b.WriteString(assistantStyle.Render(fmt.Sprintf("Assistant: %s", msg.Content)))
		}
		b.WriteString("\n")
	}

	return b.String()
}

// chatResponseMsg is sent when a chat response is received
type chatResponseMsg struct {
	content string
}
