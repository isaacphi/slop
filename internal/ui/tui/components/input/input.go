package input

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/isaacphi/slop/internal/ui/tui/theme"
)

// InputSubmitMsg is emitted when the input is submitted
type InputSubmitMsg struct {
	Value string
}

// Model represents a text input component
type Model struct {
	textInput textinput.Model
	theme     *theme.Theme
	width     int
}

// New creates a new input model
func New(thm *theme.Theme) Model {
	ti := textinput.New()
	ti.Placeholder = "Type a message..."
	ti.Focus()
	ti.CharLimit = 500
	ti.Width = 80

	return Model{
		textInput: ti,
		theme:     thm,
		width:     80,
	}
}

// SetWidth sets the width of the input
func (m *Model) SetWidth(width int) {
	m.width = width
	m.textInput.Width = width - 4 // Account for padding and borders
}

// Focus focuses the input
func (m *Model) Focus() {
	m.textInput.Focus()
}

// Blur blurs the input
func (m *Model) Blur() {
	m.textInput.Blur()
}

// Value returns the current input value
func (m Model) Value() string {
	return m.textInput.Value()
}

// SetValue sets the input value
func (m *Model) SetValue(value string) {
	m.textInput.SetValue(value)
}

// Update handles input updates
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			if m.textInput.Value() != "" {
				value := m.textInput.Value()
				cmds = append(cmds, func() tea.Msg {
					return InputSubmitMsg{Value: value}
				})
				m.textInput.Reset()
			}
		}
	}

	m.textInput, cmd = m.textInput.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

// View renders the input
func (m Model) View() string {
	var b strings.Builder

	style := m.theme.InputStyle.Copy().Width(m.width)
	b.WriteString(style.Render(m.textInput.View()))

	return b.String()
}