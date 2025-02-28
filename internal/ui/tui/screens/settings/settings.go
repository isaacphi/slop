package settings

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/isaacphi/slop/internal/ui/tui/components/help"
	"github.com/isaacphi/slop/internal/ui/tui/keymap"
	"github.com/isaacphi/slop/internal/ui/tui/screens"
	"github.com/isaacphi/slop/internal/ui/tui/theme"
)

// Model represents the settings screen
type Model struct {
	keyMap        *keymap.SettingsKeyMap
	help          help.Model
	theme         *theme.Theme
	width         int
	height        int
	settingsItems []string
	focusMode     screens.FocusMode
}

// New creates a new settings screen
func New(thm *theme.Theme, globalKeyMap *keymap.GlobalKeyMap) Model {
	km := keymap.NewSettingsKeyMap(globalKeyMap)
	helpModel := help.New(km, thm)
	
	// Example settings items
	settingsItems := []string{
		"Theme: Default",
		"API Key: ****************************",
		"Model: GPT-4",
		"Max Tokens: 4096",
		"Temperature: 0.7",
	}
	
	return Model{
		keyMap:        km,
		help:          helpModel,
		theme:         thm,
		settingsItems: settingsItems,
		focusMode:     screens.NavFocus, // Settings screen starts in nav mode
	}
}

// SetSize sets the size of the settings screen
func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.help.SetWidth(width)
}

// Init initializes the settings screen
func (m *Model) Init() tea.Cmd {
	return nil
}

// Update handles updates to the settings screen
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Settings screen is always in nav mode
		switch msg.String() {
		case "q":
			return m, tea.Quit
		case "h":
			return m, func() tea.Msg {
				return screens.ScreenChangeMsg{Screen: screens.HomeScreen}
			}
		case "c":
			return m, func() tea.Msg {
				return screens.ScreenChangeMsg{Screen: screens.ChatScreen}
			}
		case "?":
			// Toggle help
			m.help.ShowAll = !m.help.ShowAll
			return m, nil
		}
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
	}

	// Handle help component updates
	var helpModel help.Model
	helpModel, cmd = m.help.Update(msg)
	m.help = helpModel
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

// View renders the settings screen
func (m Model) View() string {
	var b strings.Builder

	// Calculate content height (total height minus help area)
	contentHeight := m.height - 3 // 3 for help/footer

	// Title
	titleStyle := m.theme.TitleStyle.Copy().
		Width(m.width)
	b.WriteString(titleStyle.Render("Settings"))
	b.WriteString("\n\n")

	// Setting items
	itemStyle := m.theme.DocStyle.Copy().
		Width(m.width - 4).
		PaddingLeft(2)
	
	for _, item := range m.settingsItems {
		b.WriteString(itemStyle.Render(item))
		b.WriteString("\n")
	}

	// Fill remaining space
	remainingLines := contentHeight - 2 - len(m.settingsItems) - 1
	for i := 0; i < remainingLines; i++ {
		b.WriteString("\n")
	}

	// Let the help component handle displaying the help
	helpStyle := m.theme.FooterStyle.Copy().Width(m.width)
	b.WriteString(helpStyle.Render(m.help.View()))

	return b.String()
}