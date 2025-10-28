package components

import (
	"strings"

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

func (l *Layout) SetContext(context string) {
	l.context = context
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

// CalculateBodyHeightWithCommandBar returns the available height accounting for dynamic command bar and status bar
func (l *Layout) CalculateBodyHeightWithCommandBar(commandBarHeight int) int {
	// Reserve space for: title (1) + empty line after title (1) + header (1) + empty line after header (1) + status bar (1) + command bar (dynamic)
	reserved := 5 + commandBarHeight
	bodyHeight := l.height - reserved
	if bodyHeight < 3 {
		bodyHeight = 3
	}
	return bodyHeight
}

// Render builds the full layout
func (l *Layout) Render(header, body, statusBar, commandBar, paletteItems, hints, loadingText string) string {
	sections := []string{}

	// Title line with app name and context (full width with background)
	titleParts := []string{l.appName + " 💨"}
	if l.version != "" {
		titleParts[0] += " v" + l.version
	}
	if l.context != "" {
		titleParts = append(titleParts, "current context: "+l.context)
	}
	if loadingText != "" {
		titleParts = append(titleParts, loadingText)
	}

	titleStyle := l.theme.AppTitle.Width(l.width)
	titleLine := titleStyle.Render(strings.Join(titleParts, " • "))
	sections = append(sections, titleLine)

	// Add empty line after title
	sections = append(sections, "")

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

	// Status bar (shown above command bar when message is set)
	if statusBar != "" {
		sections = append(sections, statusBar)
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
