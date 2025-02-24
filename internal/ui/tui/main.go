package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Model represents the application state
type Model struct {
	messages   []string
	input      string
	cursorPos  int
	windowSize tea.WindowSizeMsg
	config     interface{} // Replace with your actual config type
}

// StartTUI initializes and runs the TUI
func StartTUI(config interface{}) error {
	p := tea.NewProgram(initialModel(config))
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("error running chat TUI: %w", err)
	}
	return nil
}

func initialModel(config interface{}) Model {
	return Model{
		messages: []string{},
		input:    "",
		config:   config,
	}
}

// Styles
var (
	subtle    = lipgloss.AdaptiveColor{Light: "#D9DCCF", Dark: "#383838"}
	highlight = lipgloss.AdaptiveColor{Light: "#874BFD", Dark: "#7D56F4"}

	docStyle = lipgloss.NewStyle().Padding(1, 2, 1, 2)

	inputStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(subtle).
			Padding(0, 1)

	messageStyle = lipgloss.NewStyle().
			Foreground(highlight).
			PaddingLeft(1)
)

func (m Model) Init() tea.Cmd {
	return nil
}

// Update handles all the application updates
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit
		case tea.KeyEnter:
			if m.input != "" {
				m.messages = append(m.messages, fmt.Sprintf("You: %s", m.input))
				m.input = ""
				m.cursorPos = 0
			}
			return m, nil
		case tea.KeyBackspace:
			if len(m.input) > 0 && m.cursorPos > 0 {
				m.input = m.input[:m.cursorPos-1] + m.input[m.cursorPos:]
				m.cursorPos--
			}
			return m, nil
		case tea.KeyLeft:
			if m.cursorPos > 0 {
				m.cursorPos--
			}
			return m, nil
		case tea.KeyRight:
			if m.cursorPos < len(m.input) {
				m.cursorPos++
			}
			return m, nil
		}

		if msg.Type == tea.KeyRunes {
			m.input = m.input[:m.cursorPos] + string(msg.Runes) + m.input[m.cursorPos:]
			m.cursorPos += len(msg.Runes)
		}

	case tea.WindowSizeMsg:
		m.windowSize = msg
	}

	return m, nil
}

// View renders the application UI
func (m Model) View() string {
	var b strings.Builder

	// Chat history
	for _, msg := range m.messages {
		b.WriteString(messageStyle.Render(msg))
		b.WriteString("\n")
	}

	// Input field
	input := m.input
	if len(input) == 0 {
		input = "Type a message..."
	}

	b.WriteString("\n" + inputStyle.Render(input))

	// Help text
	b.WriteString("\n\nPress Ctrl+C to quit")

	return docStyle.Render(b.String())
}
