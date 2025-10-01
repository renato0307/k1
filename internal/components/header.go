package components

import (
	"fmt"
	"time"

	"github.com/charmbracelet/lipgloss"
)

var (
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("205")).
			Background(lipgloss.Color("235")).
			Padding(0, 1)

	timingStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Background(lipgloss.Color("235")).
			Padding(0, 1)
)

type Header struct {
	appName     string
	refreshTime time.Duration
	width       int
}

func NewHeader(appName string) *Header {
	return &Header{
		appName: appName,
	}
}

func (h *Header) SetRefreshTime(d time.Duration) {
	h.refreshTime = d
}

func (h *Header) SetWidth(width int) {
	h.width = width
}

func (h *Header) View() string {
	left := headerStyle.Render(h.appName)

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
		Background(lipgloss.Color("235")).
		Width(spacing).
		Render("")

	return lipgloss.JoinHorizontal(lipgloss.Top, left, spacer, right)
}
