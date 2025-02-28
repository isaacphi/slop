package theme

import (
	"github.com/charmbracelet/lipgloss"
)

// Theme defines the semantic colors and styles for the application
type Theme struct {
	// Colors
	Primary        lipgloss.AdaptiveColor
	Secondary      lipgloss.AdaptiveColor
	Tertiary       lipgloss.AdaptiveColor
	Text           lipgloss.AdaptiveColor
	Subtle         lipgloss.AdaptiveColor
	Highlight      lipgloss.AdaptiveColor
	Error          lipgloss.AdaptiveColor
	Success        lipgloss.AdaptiveColor
	Warning        lipgloss.AdaptiveColor
	
	// Styles
	DocStyle       lipgloss.Style
	InputStyle     lipgloss.Style
	MessageStyle   lipgloss.Style
	HeaderStyle    lipgloss.Style
	FooterStyle    lipgloss.Style
	KeyHintStyle   lipgloss.Style
	TitleStyle     lipgloss.Style
}

// DefaultTheme creates a default theme
func DefaultTheme() *Theme {
	primary := lipgloss.AdaptiveColor{Light: "#874BFD", Dark: "#7D56F4"}
	secondary := lipgloss.AdaptiveColor{Light: "#4B56FD", Dark: "#4B56FD"}
	tertiary := lipgloss.AdaptiveColor{Light: "#FD4B56", Dark: "#FD4B56"}
	text := lipgloss.AdaptiveColor{Light: "#1A1A1A", Dark: "#FFFFFF"}
	subtle := lipgloss.AdaptiveColor{Light: "#D9DCCF", Dark: "#383838"}
	highlight := lipgloss.AdaptiveColor{Light: "#874BFD", Dark: "#7D56F4"}
	error := lipgloss.AdaptiveColor{Light: "#FF0000", Dark: "#FF4136"}
	success := lipgloss.AdaptiveColor{Light: "#00FF00", Dark: "#2ECC40"}
	warning := lipgloss.AdaptiveColor{Light: "#FFA500", Dark: "#FF851B"}

	return &Theme{
		Primary:     primary,
		Secondary:   secondary,
		Tertiary:    tertiary,
		Text:        text,
		Subtle:      subtle,
		Highlight:   highlight,
		Error:       error,
		Success:     success,
		Warning:     warning,

		DocStyle: lipgloss.NewStyle().Padding(1, 2),

		InputStyle: lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(subtle).
			Padding(0, 1),

		MessageStyle: lipgloss.NewStyle().
			Foreground(highlight).
			PaddingLeft(1),

		HeaderStyle: lipgloss.NewStyle().
			Bold(true).
			Foreground(primary).
			Padding(0, 1),

		FooterStyle: lipgloss.NewStyle().
			Foreground(subtle).
			Padding(1, 2),

		KeyHintStyle: lipgloss.NewStyle().
			Foreground(secondary).
			Bold(true),

		TitleStyle: lipgloss.NewStyle().
			Foreground(primary).
			Bold(true).
			Padding(1, 2),
	}
}