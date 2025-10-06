package components

import (
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
}

// NewStatusBar creates a new status bar
func NewStatusBar(theme *ui.Theme) *StatusBar {
	return &StatusBar{
		theme: theme,
	}
}

// SetMessage sets the status message with type
func (sb *StatusBar) SetMessage(msg string, msgType types.MessageType) {
	sb.message = msg
	sb.messageType = msgType
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

// View renders the status bar
func (sb *StatusBar) View() string {
	baseStyle := lipgloss.NewStyle().
		Width(sb.width).
		Padding(0, 1)

	if sb.message == "" {
		// Render empty line to reserve space
		return baseStyle.Render("")
	}

	// Use colored background with theme foreground for high visibility
	var messageStyle lipgloss.Style
	var prefix string

	switch sb.messageType {
	case types.MessageTypeSuccess:
		// Green background with contrasting text
		messageStyle = baseStyle.Copy().
			Background(sb.theme.Success).
			Foreground(sb.theme.Background).
			Bold(true)
		prefix = "✓ "
	case types.MessageTypeError:
		// Red background with contrasting text
		messageStyle = baseStyle.Copy().
			Background(sb.theme.Error).
			Foreground(sb.theme.Background).
			Bold(true)
		prefix = "✗ "
	case types.MessageTypeInfo:
		// Blue/Primary background with contrasting text
		messageStyle = baseStyle.Copy().
			Background(sb.theme.Primary).
			Foreground(sb.theme.Background).
			Bold(true)
		prefix = "ℹ "
	default:
		messageStyle = baseStyle.Copy().
			Background(sb.theme.Primary).
			Foreground(sb.theme.Background).
			Bold(true)
		prefix = "ℹ "
	}

	return messageStyle.Render(prefix + sb.message)
}
