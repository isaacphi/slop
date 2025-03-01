package layout

import (
	"strings"

	"github.com/isaacphi/slop/internal/ui/tui/components/help"
	"github.com/isaacphi/slop/internal/ui/tui/theme"
)

// LayoutResult holds the results of layout calculations
type LayoutResult struct {
	ContentHeight int
	HelpView      string
}

// LayoutScreen creates a consistent layout with help at bottom
func LayoutScreen(width, height int, helpModel help.Model, thm *theme.Theme, focusHint string) LayoutResult {
	// Calculate content height (total height minus help footer)
	contentHeight := height - 3
	
	// Render help component
	helpStyle := thm.FooterStyle.Copy().Width(width)
	
	var helpView string
	if helpModel.ShowAll {
		helpView = helpStyle.Render(helpModel.View())
	} else if focusHint != "" {
		helpView = helpStyle.Render(focusHint + " | " + helpModel.View())
	} else {
		helpView = helpStyle.Render(helpModel.View())
	}
	
	return LayoutResult{
		ContentHeight: contentHeight,
		HelpView:      helpView,
	}
}

// RenderContentWithInput renders content area and input with proper spacing
func RenderContentWithInput(contentView string, inputView string, contentHeight int) string {
	var b strings.Builder
	
	// Main content area
	b.WriteString(contentView)
	
	// Spacing before input
	b.WriteString("\n\n")
	
	// Input area
	b.WriteString(inputView)
	
	return b.String()
}