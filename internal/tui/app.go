package tui

import (
	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type Model struct {
	viewport    viewport.Model
	input       textinput.Model
	messages    []string
	err         error
}

func NewModel() Model {
	input := textinput.New()
	input.Placeholder = "Send a message..."
	input.Focus()

	return Model{
		viewport: viewport.New(0, 0),
		input:    input,
		messages: make([]string, 0),
	}
}

func (m Model) Init() tea.Cmd {
	return textinput.Blink
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			return m, tea.Quit
		case "enter":
			if m.input.Value() != "" {
				m.messages = append(m.messages, "> "+m.input.Value())
				m.input.Reset()
				// TODO: Handle message sending to LLM
			}
		}
	}

	m.input, cmd = m.input.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m Model) View() string {
	return lipgloss.JoinVertical(
		lipgloss.Left,
		m.viewport.View(),
		m.input.View(),
	)
}