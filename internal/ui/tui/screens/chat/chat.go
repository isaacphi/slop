package Chat

import (
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/isaacphi/slop/internal/ui/tui/keymap"
)

// Model represents the chat screen
type Model struct {
	width    int
	height   int
	textArea textarea.Model
	messages []string
}

// New creates a new chat screen model
func New() Model {
	ta := textarea.New()
	ta.Placeholder = "Type your message here..."
	ta.ShowLineNumbers = false
	ta.Focus()

	return Model{
		textArea: ta,
		messages: []string{
			"Welcome to the chat screen!",
			"Press 'i' to start typing, ESC to exit typing mode.",
			"Press 'h' to return to home screen.",
		},
	}
}

// Init initializes the chat screen
func (m Model) Init() tea.Cmd {
	return nil
}

// GetKeyMap returns chat screen specific keybindings
func (m Model) GetKeyMap(mode keymap.AppMode) keymap.KeyMap {
	km := keymap.NewKeyMap()

	if mode == keymap.NormalMode {
		km.Add(key.NewBinding(
			key.WithKeys("h"),
			key.WithHelp("h", "home screen"),
		))
		km.Add(key.NewBinding(
			key.WithKeys("i"),
			key.WithHelp("i", "input mode"),
		))
		km.Add(key.NewBinding(
			key.WithKeys("j", "down"),
			key.WithHelp("j/↓", "scroll down"),
		))
		km.Add(key.NewBinding(
			key.WithKeys("k", "up"),
			key.WithHelp("k/↑", "scroll up"),
		))
	} else if mode == keymap.InputMode {
		km.Add(key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "send message"),
		))
	}

	return km
}

// ToggleInputMode enters or exits input mode
func (m Model) ToggleInputMode() (Model, tea.Cmd) {
	return m, tea.Cmd(func() tea.Msg {
		return keymap.SetModeMsg{Mode: keymap.InputMode}
	})
}

// Update handles updates to the chat screen
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		// Also update textarea width
		m.textArea.SetWidth(msg.Width - 4)

	case tea.KeyMsg:
		switch msg.String() {
		case "i":
			// Enter input mode
			return m.ToggleInputMode()

		case "enter":
			// If input mode, add message and clear textarea
			if m.textArea.Focused() {
				content := m.textArea.Value()
				if content != "" {
					m.messages = append(m.messages, "> "+content)
					m.textArea.Reset()
					return m, nil
				}
			}
		}

		// Handle updates to textarea if it's focused
		if m.textArea.Focused() {
			var cmd tea.Cmd
			m.textArea, cmd = m.textArea.Update(msg)
			cmds = append(cmds, cmd)
		}
	}

	return m, tea.Batch(cmds...)
}

// View renders the chat screen
func (m Model) View() string {
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#7D56F4")).
		Padding(0, 1).
		Render("slop - Chat Screen")

	// Style for message area
	messageStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#7D56F4")).
		Padding(1, 1).
		Width(m.width - 4).
		Height(m.height - 8) // Leave room for input and title

	// Render messages
	messageContent := ""
	for _, msg := range m.messages {
		messageContent += msg + "\n"
	}

	// Render input area
	inputArea := m.textArea.View()

	return lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		messageStyle.Render(messageContent),
		inputArea,
	)
}
