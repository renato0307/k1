package components

import (
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/renato0307/k1/internal/types"
	"github.com/renato0307/k1/internal/ui"
)

// StatusBar displays status messages (success, errors, info)
type StatusBar struct {
	message     string
	messageType types.MessageType
	width       int
	theme       *ui.Theme
	spinner     spinner.Model
}

// NewStatusBar creates a new status bar
func NewStatusBar(theme *ui.Theme) *StatusBar {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle()

	return &StatusBar{
		theme:   theme,
		spinner: s,
	}
}

// SetMessage sets the status message with type
func (sb *StatusBar) SetMessage(msg string, msgType types.MessageType) {
	sb.message = msg
	sb.messageType = msgType

	// Clear spinner styling when showing loading message
	// We'll apply all styling to the entire status bar line
	if msgType == types.MessageTypeLoading {
		sb.spinner.Style = lipgloss.NewStyle()
	}
}

// GetSpinnerCmd returns the spinner tick command if showing loading message
func (sb *StatusBar) GetSpinnerCmd() tea.Cmd {
	if sb.messageType == types.MessageTypeLoading {
		// Return the spinner's Tick command to start animation
		return sb.spinner.Tick
	}
	return nil
}

// ClearMessage clears the status message
func (sb *StatusBar) ClearMessage() {
	sb.message = ""
	sb.messageType = types.MessageTypeInfo
}

// SetWidth sets the status bar width
func (sb *StatusBar) SetWidth(width int) {
	sb.width = width
}

// GetHeight returns the height (always 1 line to reserve space)
func (sb *StatusBar) GetHeight() int {
	return 1
}

// Update handles spinner updates
func (sb *StatusBar) Update(msg tea.Msg) (*StatusBar, tea.Cmd) {
	// Only update spinner when showing loading message
	if sb.messageType == types.MessageTypeLoading {
		var cmd tea.Cmd
		sb.spinner, cmd = sb.spinner.Update(msg)
		return sb, cmd
	}
	return sb, nil
}

// View renders the status bar
func (sb *StatusBar) View() string {
	if sb.message == "" {
		// Render empty line to reserve space
		return lipgloss.NewStyle().Width(sb.width).Render("")
	}

	// Determine colors and prefix based on message type
	var bgColor lipgloss.TerminalColor
	var fgColor lipgloss.TerminalColor
	var prefix string

	switch sb.messageType {
	case types.MessageTypeSuccess:
		bgColor = sb.theme.Success
		fgColor = sb.theme.Background
		prefix = "✓ "
	case types.MessageTypeError:
		bgColor = sb.theme.Error
		fgColor = sb.theme.Background
		prefix = "✗ "
	case types.MessageTypeInfo:
		bgColor = sb.theme.Primary
		fgColor = sb.theme.Background
		prefix = "ℹ "
	case types.MessageTypeLoading:
		bgColor = sb.theme.Accent
		fgColor = sb.theme.Background
		// Use unstyled spinner - we'll apply colors to the entire line
		sb.spinner.Style = lipgloss.NewStyle()
		prefix = sb.spinner.View() + " "
	default:
		bgColor = sb.theme.Primary
		fgColor = sb.theme.Background
		prefix = "ℹ "
	}

	// Build content string
	content := prefix + sb.message

	// Calculate padding needed to fill width
	// Account for 2 spaces (1 on each side)
	contentLen := lipgloss.Width(content) + 2
	if contentLen < sb.width {
		// Pad right side with spaces to fill width
		content = " " + content + " " + strings.Repeat(" ", sb.width-contentLen)
	} else {
		content = " " + content + " "
	}

	// Render with background color (no width needed, string is already padded)
	messageStyle := lipgloss.NewStyle().
		Background(bgColor).
		Foreground(fgColor).
		Bold(true)

	return messageStyle.Render(content)
}
