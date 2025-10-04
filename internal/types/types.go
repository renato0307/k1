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
	CurrentScreen  string
	LastRefresh    time.Time
	RefreshTime    time.Duration
	Width          int
	Height         int
	ErrorMessage   string
}

// Messages
type ScreenSwitchMsg struct {
	ScreenID string
}

type RefreshCompleteMsg struct {
	Duration time.Duration
}

type ErrorMsg struct {
	Error string
}

type ClearErrorMsg struct{}

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
