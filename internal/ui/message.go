package ui

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/renato0307/k1/internal/types"
)

// RenderMessage renders a user message with appropriate styling based on message type.
// Long messages are truncated to fit the terminal width.
func RenderMessage(text string, msgType types.MessageType, theme *Theme, spinnerView string, width int) string {
	if text == "" {
		return ""
	}

	// Truncate long messages to fit terminal width
	// Max length = terminal width - prefix (2) - margin (5)
	maxMessageLength := width - 7
	if maxMessageLength < 20 {
		maxMessageLength = 20 // Minimum reasonable length
	}
	if len(text) > maxMessageLength {
		text = text[:maxMessageLength-1] + "…"
	}

	var messageColor lipgloss.AdaptiveColor
	var prefix string

	circleBullet := "⏺ "

	switch msgType {
	case types.MessageTypeSuccess:
		messageColor = theme.MessageSuccess
		prefix = circleBullet
	case types.MessageTypeError:
		messageColor = theme.MessageError
		prefix = circleBullet
	case types.MessageTypeInfo:
		messageColor = theme.MessageInfo
		prefix = circleBullet
	case types.MessageTypeLoading:
		messageColor = theme.MessageLoading
		if spinnerView != "" {
			prefix = spinnerView + " "
		} else {
			prefix = circleBullet
		}
	default:
		messageColor = theme.MessageInfo
		prefix = circleBullet
	}

	messageStyle := lipgloss.NewStyle().Foreground(messageColor)
	return messageStyle.Render(prefix + text)
}
