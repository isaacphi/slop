package chat

import (
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/isaacphi/slop/internal/config"
	"github.com/isaacphi/slop/internal/ui/tui/keymap"
)

// Model represents the chat screen
type Model struct {
	width    int
	height   int
	textArea textarea.Model
	messages []string
	viewport viewport.Model
	keyMap   *config.KeyMap
	mode     keymap.AppMode
}

// New creates a new chat screen model
func New(keyMap *config.KeyMap) Model {
	ta := textarea.New()
	ta.Placeholder = "Type your message here..."
	ta.ShowLineNumbers = false
	ta.MaxHeight = 5

	messages := []string{
		"Welcome to the chat screen!",
		"Press 'i' to start typing, ESC to exit typing mode.",
		"Press 'h' to return to home screen.",
	}

	vp := viewport.New(0, 0)
	vp.SetContent(strings.Join(messages, "\n"))

	return Model{
		textArea: ta,
		messages: messages,
		viewport: vp,
		keyMap:   keyMap,
	}
}

// Init initializes the chat screen
func (m Model) Init() tea.Cmd {
	return nil
}

// updateViewportContent updates the viewport content with current messages
func (m *Model) updateViewportContent() {
	m.viewport.SetContent(strings.Join(m.messages, "\n"))
}

// Update handles updates to the chat screen
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		// Calculate appropriate heights
		inputHeight := 5                                           // Fixed input height
		titleHeight := 1                                           // Title height
		viewportHeight := m.height - inputHeight - titleHeight - 2 // Account for padding/gaps

		// Update textarea dimensions
		m.textArea.SetWidth(msg.Width - 4) // Account for border
		m.textArea.SetHeight(inputHeight)

		// Update viewport dimensions
		m.viewport.Width = msg.Width
		m.viewport.Height = viewportHeight

		// Update the viewport content
		m.updateViewportContent()

	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			m.textArea.Blur()
			return m, tea.Cmd(func() tea.Msg {
				return keymap.SetModeMsg{Mode: keymap.NormalMode}
			})

		case "i":
			// Enter input mode
			m.textArea.Focus()
			return m, tea.Cmd(func() tea.Msg {
				return keymap.SetModeMsg{Mode: keymap.InputMode}
			})

		case "enter":
			// If input mode, add message and clear textarea
			if m.textArea.Focused() {
				content := m.textArea.Value()
				if content != "" {
					m.messages = append(m.messages, "> "+content)
					m.textArea.Reset()

					// Update viewport content with new messages
					m.updateViewportContent()

					// Scroll to the bottom of viewport
					m.viewport.GotoBottom()

					return m, nil
				}
			}

		case "up", "k":
			// Scroll viewport up when in normal mode
			if !m.textArea.Focused() {
				// m.viewport.LineUp(1)
			}

		case "down", "j":
			// Scroll viewport down when in normal mode
			if !m.textArea.Focused() {
				// m.viewport.LineDown(1)
			}

		case "pgup":
			// Page up in viewport
			if !m.textArea.Focused() {
				m.viewport.HalfViewUp()
			}

		case "pgdown":
			// Page down in viewport
			if !m.textArea.Focused() {
				m.viewport.HalfViewDown()
			}
		}

		// Handle updates to textarea if it's focused
		if m.textArea.Focused() {
			var cmd tea.Cmd
			m.textArea, cmd = m.textArea.Update(msg)
			cmds = append(cmds, cmd)
		}

	case keymap.SetModeMsg:
		m.mode = msg.Mode
		// If we're switching to normal mode, blur the textarea
		if msg.Mode == keymap.NormalMode {
			m.textArea.Blur()
		}
	}

	// Handle viewport updates
	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

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

	// Style for input area (with border)
	inputStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#7D56F4")).
		Padding(0, 1)

	// Render viewport (no border)
	viewportContent := m.viewport.View()

	// Render input area with border
	inputArea := inputStyle.Render(m.textArea.View())

	return lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		viewportContent,
		inputArea,
	)
}
