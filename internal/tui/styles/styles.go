package styles

import "github.com/charmbracelet/lipgloss"

var (
	DocStyle = lipgloss.NewStyle().Margin(1, 2)

	HighlightStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("205")).
			Bold(true)

	ListStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("240")).
			Padding(1, 0)

	ListTitleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("99")).
			Bold(true).
			Padding(0, 1)

	StatusMessageStyle = lipgloss.NewStyle().
				Foreground(lipgloss.AdaptiveColor{Light: "#04B575", Dark: "#04B575"}).
				Render
)
