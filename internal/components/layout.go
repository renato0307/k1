package components

import (
	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("170")).
			Padding(0, 1)

	filterStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("229")).
			Background(lipgloss.Color("237")).
			Padding(0, 1)

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Padding(0, 1)

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Background(lipgloss.Color("52")).
			Padding(0, 1).
			Bold(true)
)

type Layout struct {
	width  int
	height int
}

func NewLayout(width, height int) *Layout {
	return &Layout{
		width:  width,
		height: height,
	}
}

func (l *Layout) SetSize(width, height int) {
	l.width = width
	l.height = height
}

// CalculateBodyHeight returns the available height for the body content
func (l *Layout) CalculateBodyHeight() int {
	// Reserve space for: header (1) + title (1) + help (1) + message (1) + padding
	reserved := 5
	bodyHeight := l.height - reserved
	if bodyHeight < 3 {
		bodyHeight = 3
	}
	return bodyHeight
}

// Render builds the full layout
func (l *Layout) Render(header, title, body, help, message, filter string) string {
	sections := []string{}

	if header != "" {
		sections = append(sections, header)
	}

	// Combine title and filter on same row
	if title != "" || filter != "" {
		titleRendered := titleStyle.Render(title)

		var titleRow string
		if filter != "" {
			filterRendered := filterStyle.Render(filter)
			// Calculate spacing to push filter to the right
			titleWidth := lipgloss.Width(titleRendered)
			filterWidth := lipgloss.Width(filterRendered)
			spacing := l.width - titleWidth - filterWidth
			if spacing < 2 {
				spacing = 2
			}
			spacer := lipgloss.NewStyle().Width(spacing).Render("")
			titleRow = lipgloss.JoinHorizontal(lipgloss.Top, titleRendered, spacer, filterRendered)
		} else {
			titleRow = titleRendered
		}
		sections = append(sections, titleRow)
	}

	if body != "" {
		sections = append(sections, body)
	}

	if help != "" {
		sections = append(sections, helpStyle.Render(help))
	}

	if message != "" {
		sections = append(sections, errorStyle.Render(message))
	}

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}
