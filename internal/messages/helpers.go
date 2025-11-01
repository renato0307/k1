package messages

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/renato0307/k1/internal/types"
)

// Command layer helpers - return tea.Cmd with appropriate StatusMsg

// ErrorCmd returns a tea.Cmd that produces an error status message.
// Use this in command handlers when an operation fails.
//
// Example:
//
//	if err := validateArgs(args); err != nil {
//	    return messages.ErrorCmd("Invalid arguments: %v", err)
//	}
func ErrorCmd(format string, args ...any) tea.Cmd {
	msg := fmt.Sprintf(format, args...)
	return func() tea.Msg {
		return types.ErrorStatusMsg(msg)
	}
}

// SuccessCmd returns a tea.Cmd that produces a success status message.
// Use this in command handlers when an operation completes successfully.
//
// Example:
//
//	return messages.SuccessCmd("Scaled deployment/%s to %d replicas", name, replicas)
func SuccessCmd(format string, args ...any) tea.Cmd {
	msg := fmt.Sprintf(format, args...)
	return func() tea.Msg {
		return types.SuccessMsg(msg)
	}
}

// InfoCmd returns a tea.Cmd that produces an info status message.
// Use this in command handlers for informational messages.
//
// Example:
//
//	return messages.InfoCmd("Refreshing resource list…")
func InfoCmd(format string, args ...any) tea.Cmd {
	msg := fmt.Sprintf(format, args...)
	return func() tea.Msg {
		return types.InfoMsg(msg)
	}
}

// Repository layer helpers - return wrapped errors with context

// WrapError wraps an error with additional context using fmt.Errorf.
// Preserves the error chain for debugging with %w.
//
// Example:
//
//	pods, err := r.podLister.List(labels.Everything())
//	if err != nil {
//	    return nil, messages.WrapError(err, "failed to list pods in namespace %s", namespace)
//	}
func WrapError(err error, format string, args ...any) error {
	context := fmt.Sprintf(format, args...)
	return fmt.Errorf("%s: %w", context, err)
}
