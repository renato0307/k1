package components

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/renato0307/k1/internal/ui"
)

// FullScreenViewType represents the type of full-screen view
type FullScreenViewType int

const (
	FullScreenYAML FullScreenViewType = iota
	FullScreenDescribe
	FullScreenLogs

	// FullScreenReservedLines is the number of lines reserved for UI chrome
	// (header, command bar, borders) when showing full-screen views.
	FullScreenReservedLines = 3
)

// FullScreen component displays content in full-screen mode
type FullScreen struct {
	viewType     FullScreenViewType
	resourceName string
	content      string
	width        int
	height       int
	theme        *ui.Theme
	scrollOffset int
}

// NewFullScreen creates a new full-screen component
func NewFullScreen(viewType FullScreenViewType, resourceName string, content string, theme *ui.Theme) *FullScreen {
	return &FullScreen{
		viewType:     viewType,
		resourceName: resourceName,
		content:      content,
		width:        80,
		height:       24,
		theme:        theme,
		scrollOffset: 0,
	}
}

// SetSize updates the size of the full-screen view
func (fs *FullScreen) SetSize(width, height int) {
	fs.width = width
	fs.height = height
}

// Update handles input for the full-screen view
func (fs *FullScreen) Update(msg tea.Msg) (*FullScreen, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if fs.scrollOffset > 0 {
				fs.scrollOffset--
			}
			return fs, nil
		case "down", "j":
			// Calculate max scroll offset based on content
			lines := strings.Split(fs.content, "\n")
			maxOffset := len(lines) - (fs.height - FullScreenReservedLines) // 3 lines for header + borders
			if maxOffset < 0 {
				maxOffset = 0
			}
			if fs.scrollOffset < maxOffset {
				fs.scrollOffset++
			}
			return fs, nil
		case "pgup":
			fs.scrollOffset -= fs.height - FullScreenReservedLines
			if fs.scrollOffset < 0 {
				fs.scrollOffset = 0
			}
			return fs, nil
		case "pgdown":
			lines := strings.Split(fs.content, "\n")
			maxOffset := len(lines) - (fs.height - FullScreenReservedLines)
			if maxOffset < 0 {
				maxOffset = 0
			}
			fs.scrollOffset += fs.height - FullScreenReservedLines
			if fs.scrollOffset > maxOffset {
				fs.scrollOffset = maxOffset
			}
			return fs, nil
		case "home", "g":
			fs.scrollOffset = 0
			return fs, nil
		case "end", "G":
			lines := strings.Split(fs.content, "\n")
			maxOffset := len(lines) - (fs.height - FullScreenReservedLines)
			if maxOffset < 0 {
				maxOffset = 0
			}
			fs.scrollOffset = maxOffset
			return fs, nil
		}
	}
	return fs, nil
}

// View renders the full-screen view
func (fs *FullScreen) View() string {
	// Create header with resource name and ESC hint
	titleStyle := lipgloss.NewStyle().
		Foreground(fs.theme.Primary).
		Bold(true)

	hintStyle := lipgloss.NewStyle().
		Foreground(fs.theme.Muted)

	var viewTypeStr string
	switch fs.viewType {
	case FullScreenYAML:
		viewTypeStr = "YAML"
	case FullScreenDescribe:
		viewTypeStr = "Describe"
	case FullScreenLogs:
		viewTypeStr = "Logs"
	}

	title := titleStyle.Render(viewTypeStr + ": " + fs.resourceName)
	hint := hintStyle.Render("[ESC] Back  [↑↓/jk] Scroll  [PgUp/PgDn] Page  [g/G] Top/Bottom")

	headerLine := lipgloss.JoinHorizontal(
		lipgloss.Top,
		title,
		strings.Repeat(" ", max(0, fs.width-lipgloss.Width(title)-lipgloss.Width(hint))),
		hint,
	)

	separatorStyle := lipgloss.NewStyle().Foreground(fs.theme.Muted)
	separator := separatorStyle.Render(strings.Repeat("─", fs.width))

	// Process content with syntax highlighting if YAML
	displayContent := fs.content
	if fs.viewType == FullScreenYAML {
		displayContent = fs.highlightYAML(fs.content)
	}

	// Split content into lines and apply scroll offset
	lines := strings.Split(displayContent, "\n")
	visibleHeight := fs.height - FullScreenReservedLines // Subtract header, separator, and bottom border

	var visibleLines []string
	for i := fs.scrollOffset; i < len(lines) && i < fs.scrollOffset+visibleHeight; i++ {
		visibleLines = append(visibleLines, lines[i])
	}

	// Pad with empty lines if content is shorter than viewport
	for len(visibleLines) < visibleHeight {
		visibleLines = append(visibleLines, "")
	}

	content := strings.Join(visibleLines, "\n")

	// Show scroll indicator if there's more content
	scrollInfo := ""
	if len(lines) > visibleHeight {
		scrollInfo = hintStyle.Render(
			"  " + intToString(fs.scrollOffset+1) + "-" +
			intToString(min(fs.scrollOffset+visibleHeight, len(lines))) +
			" of " + intToString(len(lines)),
		)
	}

	return lipgloss.JoinVertical(
		lipgloss.Left,
		headerLine,
		separator,
		content,
		scrollInfo,
	)
}

// highlightYAML applies simple syntax highlighting to YAML content
func (fs *FullScreen) highlightYAML(yaml string) string {
	lines := strings.Split(yaml, "\n")

	keyStyle := lipgloss.NewStyle().Foreground(fs.theme.Primary)
	valueStyle := lipgloss.NewStyle().Foreground(fs.theme.Success)
	commentStyle := lipgloss.NewStyle().Foreground(fs.theme.Muted)

	var highlighted []string
	for _, line := range lines {
		// Comment
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			highlighted = append(highlighted, commentStyle.Render(line))
			continue
		}

		// Key-value pair
		if strings.Contains(line, ":") {
			parts := strings.SplitN(line, ":", 2)
			key := keyStyle.Render(parts[0] + ":")
			value := ""
			if len(parts) > 1 {
				value = valueStyle.Render(parts[1])
			}
			highlighted = append(highlighted, key+value)
			continue
		}

		// Default (list items, etc.)
		highlighted = append(highlighted, line)
	}

	return strings.Join(highlighted, "\n")
}

// Helper functions
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func intToString(n int) string {
	if n < 0 {
		return "0"
	}
	// Simple int to string conversion
	if n == 0 {
		return "0"
	}
	digits := []rune{}
	for n > 0 {
		digits = append([]rune{rune('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}
