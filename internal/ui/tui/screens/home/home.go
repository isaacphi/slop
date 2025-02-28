package home

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/isaacphi/slop/internal/ui/tui/components/help"
	"github.com/isaacphi/slop/internal/ui/tui/components/input"
	"github.com/isaacphi/slop/internal/ui/tui/keymap"
	"github.com/isaacphi/slop/internal/ui/tui/screens"
	"github.com/isaacphi/slop/internal/ui/tui/theme"
)

// Model represents the home screen
type Model struct {
	keyMap    *keymap.HomeKeyMap
	input     input.Model
	help      help.Model
	theme     *theme.Theme
	width     int
	height    int
	demoText  string
	focusMode screens.FocusMode
}

// New creates a new home screen
func New(thm *theme.Theme, globalKeyMap *keymap.GlobalKeyMap) Model {
	km := keymap.NewHomeKeyMap(globalKeyMap)
	inputModel := input.New(thm)
	helpModel := help.New(km, thm)
	
	return Model{
		keyMap:    km,
		input:     inputModel,
		help:      helpModel,
		theme:     thm,
		demoText:  "Welcome to SLOP",
		focusMode: screens.InputFocus,
	}
}

// SetSize sets the size of the home screen
func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.input.SetWidth(width)
	m.help.SetWidth(width)
}

// Init initializes the home screen
func (m *Model) Init() tea.Cmd {
	// Start with input focus
	if m.focusMode == screens.InputFocus {
		m.input.Focus()
	} else {
		m.input.Blur()
	}
	return nil
}

// Update handles updates to the home screen
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
			case "c":
				return m, func() tea.Msg {
					return screens.ScreenChangeMsg{Screen: screens.SettingsScreen}
				}
			}
		} else {
			// Input mode handling
			if msg.Type == tea.KeyEnter && m.input.Value() != "" {
				// When enter is pressed, go to chat screen with the input value
				// Store message in the demo text (just for demo purposes)
				m.demoText = m.input.Value()
				
				cmds = append(cmds, func() tea.Msg {
					return screens.ScreenChangeMsg{Screen: screens.ChatScreen}
				})
				// Reset input
				m.input.SetValue("")
				return m, tea.Batch(cmds...)
			}
		}

	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
	}

	// Only update input component if in input focus mode
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

// View renders the home screen
func (m Model) View() string {
	var b strings.Builder

	// Calculate content height (total height minus input and help areas)
	contentHeight := m.height - 6 // 3 for input, 3 for help/footer

	// Center the demo text
	demoTextStyle := m.theme.TitleStyle.Copy().
		Width(m.width).
		Height(contentHeight).
		Align(lipgloss.Center).
		AlignVertical(lipgloss.Center)
	
	b.WriteString(demoTextStyle.Render(m.demoText))
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
	
	// Help text now includes focus mode indicator
	helpText := "ESC: " + (map[screens.FocusMode]string{
		screens.InputFocus: "exit input mode",
		screens.NavFocus:   "enter input mode",
	})[m.focusMode]
	
	helpStyle := m.theme.FooterStyle.Copy().Width(m.width)
	b.WriteString(helpStyle.Render(helpText + " | " + m.help.ShortHelp()))

	return b.String()
}