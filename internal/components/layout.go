package components

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/renato0307/k1/internal/ui"
)

type Layout struct {
	width   int
	height  int
	appName string
	version string
	context string
	theme   *ui.Theme
}

func NewLayout(width, height int, theme *ui.Theme) *Layout {
	return &Layout{
		width:   width,
		height:  height,
		appName: "k1",
		version: "", // Will be set in the future
		context: "", // Will be set in the future
		theme:   theme,
	}
}

func (l *Layout) SetSize(width, height int) {
	l.width = width
	l.height = height
}

// CalculateBodyHeight returns the available height for the body content
func (l *Layout) CalculateBodyHeight() int {
	// Reserve space for: title (1) + header (1) + empty line (1) + message (1) + command bar (1) + padding
	reserved := 6
	bodyHeight := l.height - reserved
	if bodyHeight < 3 {
		bodyHeight = 3
	}
	return bodyHeight
}

// CalculateBodyHeightWithCommandBar returns the available height accounting for dynamic command bar
func (l *Layout) CalculateBodyHeightWithCommandBar(commandBarHeight int) int {
	// Reserve space for: title (1) + header (1) + empty line (1) + command bar (dynamic)
	reserved := 3 + commandBarHeight
	bodyHeight := l.height - reserved
	if bodyHeight < 3 {
		bodyHeight = 3
	}
	return bodyHeight
}

// Render builds the full layout
func (l *Layout) Render(header, body, message, commandBar, paletteItems, hints string) string {
	sections := []string{}

	// Title style using theme
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(l.theme.Primary)

	// Error style using theme
	errorStyle := lipgloss.NewStyle().
		Foreground(l.theme.Error).
		Background(l.theme.Background).
		Padding(0, 1).
		Bold(true)

	// Title line with app name and emoji
	titleText := l.appName + " 💨"
	if l.version != "" {
		titleText += " v" + l.version
	}
	if l.context != "" {
		titleText += " • " + l.context
	}
	sections = append(sections, titleStyle.Render(titleText))

	// Header (screen info)
	if header != "" {
		sections = append(sections, header)
		// Add empty line after header
		sections = append(sections, "")
	}

	// Body (list)
	if body != "" {
		sections = append(sections, body)
	}

	// Error message (if present)
	if message != "" {
		sections = append(sections, errorStyle.Render(message))
	}

	// Command bar at the bottom (always rendered)
	if commandBar != "" {
		sections = append(sections, commandBar)
	}

	// Palette items (shown between command bar and hints when active)
	if paletteItems != "" {
		sections = append(sections, paletteItems)
	}

	// Hints (always rendered below everything)
	if hints != "" {
		sections = append(sections, hints)
	}

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}
