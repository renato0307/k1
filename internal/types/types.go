package types

import (
	"strings"
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
	GetSelectedResource() map[string]interface{}
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

// FilterContext defines filtering to apply on screen switch
type FilterContext struct {
	Field    string            // "owner", "node", "selector"
	Value    string            // Resource name (deployment, node, service)
	Metadata map[string]string // namespace, kind, etc.
}

// Description returns a human-readable description of the filter
func (f *FilterContext) Description() string {
	if f == nil {
		return ""
	}

	kind := strings.ToLower(f.Metadata["kind"])
	switch f.Field {
	case "owner":
		return "filtered by " + kind + ": " + f.Value
	case "node":
		return "filtered by " + kind + ": " + f.Value
	case "selector":
		return "filtered by " + kind + ": " + f.Value
	case "namespace":
		return "filtered by " + kind + ": " + f.Value
	case "configmap":
		return "filtered by " + kind + ": " + f.Value
	case "secret":
		return "filtered by " + kind + ": " + f.Value
	default:
		return "filtered by " + f.Value
	}
}

// Messages
type ScreenSwitchMsg struct {
	ScreenID         string
	FilterContext    *FilterContext // Optional filter for contextual navigation
	CommandBarFilter string         // Optional command bar fuzzy filter to restore
	IsBackNav        bool           // True if navigating back via ESC
	PushHistory      bool           // True if should push current screen to history
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
	MessageTypeLoading // Loading state with spinner
)

type StatusMsg struct {
	Message string
	Type    MessageType
}

type ClearStatusMsg struct {
	MessageID int // Only clear if this matches the current message ID
}

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

// LoadingMsg creates a loading status message (with spinner)
func LoadingMsg(message string) StatusMsg {
	return StatusMsg{Message: message, Type: MessageTypeLoading}
}

type FilterUpdateMsg struct {
	Filter string
}

type ClearFilterMsg struct{}

// ShowFullScreenMsg triggers display of full-screen content
type ShowFullScreenMsg struct {
	ViewType     int // 0=YAML, 1=Describe, 2=Logs
	ResourceName string
	Content      string
}

// ExitFullScreenMsg returns from full-screen view to list
type ExitFullScreenMsg struct{}

// Context management messages

// ContextSwitchMsg initiates a context switch
type ContextSwitchMsg struct {
	ContextName string
}

// ContextLoadProgressMsg reports loading progress
type ContextLoadProgressMsg struct {
	Context string
	Message string
	Phase   int
}

// ContextLoadCompleteMsg signals successful context load
type ContextLoadCompleteMsg struct {
	Context string
}

// ContextLoadFailedMsg signals failed context load
type ContextLoadFailedMsg struct {
	Context string
	Error   error
}

// ContextSwitchCompleteMsg signals successful context switch
type ContextSwitchCompleteMsg struct {
	OldContext string
	NewContext string
}

// ContextRetryMsg requests retry of failed context
type ContextRetryMsg struct {
	ContextName string
}

// DynamicScreenCreateMsg requests creation of dynamic screen for CRD instances
type DynamicScreenCreateMsg struct {
	CRD any // CustomResourceDefinition instance
}
