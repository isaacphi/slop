package chat

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/isaacphi/slop/internal/ui/tui/components/help"
	"github.com/isaacphi/slop/internal/ui/tui/components/input"
	"github.com/isaacphi/slop/internal/ui/tui/keymap"
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
	keyMap    *keymap.ChatKeyMap
	input     input.Model
	help      help.Model
	theme     *theme.Theme
	width     int
	height    int
	messages  []Message
	focusMode screens.FocusMode
}

// New creates a new chat screen
func New(thm *theme.Theme, globalKeyMap *keymap.GlobalKeyMap) Model {
	km := keymap.NewChatKeyMap(globalKeyMap)
	inputModel := input.New(thm)
	helpModel := help.New(km, thm)
	
	return Model{
		keyMap:    km,
		input:     inputModel,
		help:      helpModel,
		theme:     thm,
		messages:  []Message{
			{Content: "Hello! How can I help you today?", IsUser: false},
		},
		focusMode: screens.InputFocus, // Chat screen starts with input focus
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
	// Set focus based on mode
	if m.focusMode == screens.InputFocus {
		m.input.Focus()
	} else {
		m.input.Blur()
	}
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
		if msg.Type == tea.KeyEsc {
			if m.focusMode == screens.InputFocus {
				m.focusMode = screens.NavFocus
				m.input.Blur()
				return m, nil
			} else {
				m.focusMode = screens.InputFocus
				m.input.Focus()
				return m, nil
			}
		}

		// Handle keys based on focus mode
		if m.focusMode == screens.NavFocus {
			// Navigation mode key handling
			switch msg.String() {
			case "q":
				return m, tea.Quit
			case "h":
				return m, func() tea.Msg {
					return screens.ScreenChangeMsg{Screen: screens.HomeScreen}
				}
			case "c":
				return m, func() tea.Msg {
					return screens.ScreenChangeMsg{Screen: screens.SettingsScreen}
				}
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

	// Only update input if in input focus mode
	if m.focusMode == screens.InputFocus {
		m.input, cmd = m.input.Update(msg)
		cmds = append(cmds, cmd)
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

	// Calculate content height (total height minus input and help areas)
	contentHeight := m.height - 6 // 3 for input, 3 for help/footer

	// Render chat messages
	messagesStr := m.renderMessages(contentHeight)
	
	b.WriteString(messagesStr)
	b.WriteString("\n\n")
	
	// Add focus indicator to input if in nav mode
	inputView := m.input.View()
	if m.focusMode == screens.NavFocus {
		inputView = m.theme.DocStyle.Copy().
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(m.theme.Subtle).
			Render(inputView)
	}
	
	b.WriteString(inputView)
	b.WriteString("\n\n")
	
	// Let the help component handle displaying the help
	// We'll prepend our focus mode indicator
	escHint := "ESC: " + (map[screens.FocusMode]string{
		screens.InputFocus: "exit input mode",
		screens.NavFocus:   "enter input mode",
	})[m.focusMode] + " | "
	
	helpStyle := m.theme.FooterStyle.Copy().Width(m.width)
	
	// Use the help component's view with our custom styling
	if m.help.ShowAll {
		b.WriteString(helpStyle.Render(m.help.View()))
	} else {
		b.WriteString(helpStyle.Render(escHint + m.help.View()))
	}

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