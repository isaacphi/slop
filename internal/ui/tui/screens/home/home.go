package home

import (
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/isaacphi/slop/internal/config"
	"github.com/isaacphi/slop/internal/ui/tui/keymap"
)

// Model represents the home screen
type Model struct {
	width    int
	height   int
	textArea textarea.Model
	mode     keymap.AppMode
	keyMap   *config.KeyMap
}

// New creates a new home screen model
func New(keyMap *config.KeyMap) Model {
	ta := textarea.New()
	ta.Placeholder = "Type your message here..."
	ta.ShowLineNumbers = false
	ta.MaxHeight = 5

	return Model{
		keyMap:   keyMap,
		textArea: ta,
	}
}

// Init initializes the home screen
func (m Model) Init() tea.Cmd {
	return nil
}

// Update handles updates to the home screen
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		// Pass keypress to text area if in input mode
		if m.mode == keymap.InputMode {
			var cmd tea.Cmd
			m.textArea, cmd = m.textArea.Update(msg)
			cmds = append(cmds, cmd)
		}
	}

	return m, tea.Batch(cmds...)
}

// View renders the home screen
func (m Model) View() string {

	centeredBody := lipgloss.NewStyle().
		Width(m.width).
		Align(lipgloss.Center).
		Render("~~~")

	inputAreaWidth := min(int(float64(m.width)*0.8), 80)
	if m.width <= 60 {
		inputAreaWidth = m.width - 2
	}

	inputArea := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Width(inputAreaWidth).
		Render(m.textArea.View())

	return lipgloss.JoinVertical(lipgloss.Center, centeredBody, inputArea)
}
