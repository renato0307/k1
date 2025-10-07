package types

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// Screen represents a view in the application
type Screen interface {
	tea.Model
	ID() string
	Title() string
	HelpText() string
	Operations() []Operation
}

// ScreenWithSelection interface for screens that track selected resources
type ScreenWithSelection interface {
	Screen
	GetSelectedResource() map[string]any
}

// Operation represents an action that can be executed on a screen
type Operation struct {
	ID          string
	Name        string
	Description string
	Shortcut    string
	Execute     func() tea.Cmd
}

// ScreenRegistry manages available screens
type ScreenRegistry struct {
	screens map[string]Screen
	order   []string
}

func NewScreenRegistry() *ScreenRegistry {
	return &ScreenRegistry{
		screens: make(map[string]Screen),
		order:   []string{},
	}
}

func (r *ScreenRegistry) Register(screen Screen) {
	id := screen.ID()
	if _, exists := r.screens[id]; !exists {
		r.order = append(r.order, id)
	}
	r.screens[id] = screen
}

func (r *ScreenRegistry) Get(id string) (Screen, bool) {
	screen, ok := r.screens[id]
	return screen, ok
}

func (r *ScreenRegistry) All() []Screen {
	result := make([]Screen, 0, len(r.order))
	for _, id := range r.order {
		result = append(result, r.screens[id])
	}
	return result
}

// AppState holds shared application state
type AppState struct {
	CurrentScreen string
	LastRefresh   time.Time
	RefreshTime   time.Duration
	Width         int
	Height        int
}

// Messages
type ScreenSwitchMsg struct {
	ScreenID string
}

type RefreshCompleteMsg struct {
	Duration time.Duration
}

// MessageType defines the type of status message
type MessageType int

const (
	MessageTypeInfo MessageType = iota
	MessageTypeSuccess
	MessageTypeError
)

type StatusMsg struct {
	Message string
	Type    MessageType
}

type ClearStatusMsg struct{}

// Helper functions for creating status messages

// InfoMsg creates an info status message
func InfoMsg(message string) StatusMsg {
	return StatusMsg{Message: message, Type: MessageTypeInfo}
}

// SuccessMsg creates a success status message
func SuccessMsg(message string) StatusMsg {
	return StatusMsg{Message: message, Type: MessageTypeSuccess}
}

// ErrorMsg creates an error status message
func ErrorStatusMsg(message string) StatusMsg {
	return StatusMsg{Message: message, Type: MessageTypeError}
}

type FilterUpdateMsg struct {
	Filter string
}

type ClearFilterMsg struct{}

// ShowFullScreenMsg triggers display of full-screen content
type ShowFullScreenMsg struct {
	ViewType     int    // 0=YAML, 1=Describe, 2=Logs
	ResourceName string
	Content      string
}

// ExitFullScreenMsg returns from full-screen view to list
type ExitFullScreenMsg struct{}

// NavigationContext holds information about how a screen was navigated to
type NavigationContext struct {
	// ParentScreen is the ID of the screen that navigated to this screen
	ParentScreen string
	// ParentResource is the name of the resource that was selected in the parent screen
	ParentResource string
	// FilterLabel is the label to show in the breadcrumb (e.g., "Deployment: my-app")
	FilterLabel string
	// FilterValue is the actual filter to apply (e.g., label selector, namespace, etc.)
	FilterValue string
}

// NavigationStackEntry represents a single entry in the navigation history
type NavigationStackEntry struct {
	ScreenID  string
	Context   *NavigationContext
	ScrollPos int // Preserve scroll position when returning
}

// NavigateMsg triggers navigation to a new screen with context
type NavigateMsg struct {
	ScreenID string
	Context  NavigationContext
}

// NavigateBackMsg triggers navigation back to previous screen
type NavigateBackMsg struct{}
