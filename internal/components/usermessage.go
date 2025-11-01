package components

import (
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/renato0307/k1/internal/types"
	"github.com/renato0307/k1/internal/ui"
)

// UserMessage manages and displays user-facing status messages (success, errors, info, loading).
// This component is responsible for message state management and spinner coordination.
// The actual rendering is delegated to ui.RenderMessage() for DRY and reusability.
type UserMessage struct {
	message     string
	messageType types.MessageType
	width       int
	theme       *ui.Theme
	spinner     spinner.Model
}

// NewUserMessage creates a new user message component
func NewUserMessage(theme *ui.Theme) *UserMessage {
	s := spinner.New()
	// Rotating asterisk symbols (exact match to Claude Code)
	s.Spinner = spinner.Spinner{
		Frames: []string{"✽", "✻", "✶", "·", "✢"},
		FPS:    time.Second / 6, // Slower animation
	}
	s.Style = lipgloss.NewStyle()

	return &UserMessage{
		theme:   theme,
		spinner: s,
	}
}

// SetMessage sets the message text and type
func (um *UserMessage) SetMessage(msg string, msgType types.MessageType) {
	um.message = msg
	um.messageType = msgType

	// Clear spinner styling when showing loading message
	// Styling will be applied by ui.RenderMessage()
	if msgType == types.MessageTypeLoading {
		um.spinner.Style = lipgloss.NewStyle()
	}
}

// GetSpinnerCmd returns the spinner tick command if showing loading message
func (um *UserMessage) GetSpinnerCmd() tea.Cmd {
	if um.messageType == types.MessageTypeLoading {
		// Return the spinner's Tick command to start animation
		return um.spinner.Tick
	}
	return nil
}

// ClearMessage clears the current message
func (um *UserMessage) ClearMessage() {
	um.message = ""
	um.messageType = types.MessageTypeInfo
}

// IsLoadingMessage returns true if the current message is a loading message
func (um *UserMessage) IsLoadingMessage() bool {
	return um.messageType == types.MessageTypeLoading
}

// SetWidth sets the component width
func (um *UserMessage) SetWidth(width int) {
	um.width = width
}

// GetHeight returns the height (always 1 line to reserve space)
func (um *UserMessage) GetHeight() int {
	return 1
}

// Update handles spinner updates for loading messages
func (um *UserMessage) Update(msg tea.Msg) (*UserMessage, tea.Cmd) {
	// Only update spinner when showing loading message
	if um.messageType == types.MessageTypeLoading {
		var cmd tea.Cmd
		um.spinner, cmd = um.spinner.Update(msg)
		return um, cmd
	}
	return um, nil
}

// View renders the user message using the shared ui.RenderMessage function
func (um *UserMessage) View() string {
	if um.message == "" {
		// Render empty line to reserve space
		return lipgloss.NewStyle().Width(um.width).Render("")
	}

	// For loading messages, get the current spinner frame
	var spinnerView string
	if um.messageType == types.MessageTypeLoading {
		spinnerView = um.spinner.View()
	}

	// Delegate rendering to the shared ui.RenderMessage function (DRY)
	return ui.RenderMessage(um.message, um.messageType, um.theme, spinnerView, um.width)
}
