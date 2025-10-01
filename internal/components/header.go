package components

import (
	"fmt"
	"time"

	"github.com/charmbracelet/lipgloss"

	"timoneiro/internal/ui"
)

type Header struct {
	appName     string
	screenTitle string
	refreshTime time.Duration
	width       int
	theme       *ui.Theme
}

func NewHeader(appName string, theme *ui.Theme) *Header {
	return &Header{
		appName: appName,
		theme:   theme,
	}
}

func (h *Header) SetScreenTitle(title string) {
	h.screenTitle = title
}

func (h *Header) SetRefreshTime(d time.Duration) {
	h.refreshTime = d
}

func (h *Header) SetWidth(width int) {
	h.width = width
}

func (h *Header) View() string {
	// Build header style from theme
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(h.theme.Primary)

	timingStyle := lipgloss.NewStyle().
		Foreground(h.theme.Muted).
		Padding(0, 1)

	title := h.appName
	if h.screenTitle != "" {
		title = fmt.Sprintf("%s > %s", h.appName, h.screenTitle)
	}
	left := headerStyle.Render(title)

	var right string
	if h.refreshTime > 0 {
		right = timingStyle.Render(fmt.Sprintf("Refreshed in %v", h.refreshTime))
	}

	// Calculate spacing to push timing to the right
	leftWidth := lipgloss.Width(left)
	rightWidth := lipgloss.Width(right)
	spacing := h.width - leftWidth - rightWidth
	if spacing < 0 {
		spacing = 0
	}

	spacer := lipgloss.NewStyle().
		Width(spacing).
		Render("")

	return lipgloss.JoinHorizontal(lipgloss.Top, left, spacer, right)
}
