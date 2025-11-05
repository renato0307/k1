package screens

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/renato0307/k1/internal/types"
)

const (
	// HelpScreenID is the screen identifier for the help screen
	HelpScreenID = "help"
)

// HelpEntry represents a keyboard shortcut entry
type HelpEntry struct {
	Section     string
	Shortcut    string
	Description string
}

// getHelpEntries returns all keyboard shortcuts organized by section
func getHelpEntries() []HelpEntry {
	return []HelpEntry{
		// Navigation
		{"Navigation", "/", "Search/filter current list"},
		{"Navigation", ":", "Navigate to resource/screen"},
		{"Navigation", "> or ctrl+p", "Open command palette"},
		{"Navigation", "esc", "Back/clear filter"},
		{"Navigation", "↑/↓ or j/k", "Move selection up/down"},
		{"Navigation", "g", "Jump to top of list"},
		{"Navigation", "G", "Jump to bottom (shift+g)"},
		{"Navigation", "PgUp/PgDn", "Page up/down"},
		{"Navigation", "ctrl+b/ctrl+f", "Page up/down (alternate)"},

		// Resources
		{"Resources", "d", "Describe selected resource"},
		{"Resources", "e", "Edit resource (clipboard)"},
		{"Resources", "l", "View logs (pods only)"},
		{"Resources", "y", "View YAML"},
		{"Resources", "ctrl+x", "Delete resource"},
		{"Resources", "n", "Filter by namespace"},

		// Context
		{"Context", "[", "Previous Kubernetes context"},
		{"Context", "]", "Next Kubernetes context"},

		// Global
		{"Global", ":q", "Quit application"},
		{"Global", "ctrl+c", "Quit application (alternate)"},
		{"Global", "ctrl+r", "Refresh data"},
		{"Global", "?", "Show this help"},

		// Palette
		{"Palette", "↑/↓", "Navigate suggestions"},
		{"Palette", "enter", "Execute command"},
		{"Palette", "tab", "Auto-complete"},
		{"Palette", "esc", "Cancel"},
	}
}

// GetHelpScreenConfig returns the configuration for the help screen
func GetHelpScreenConfig() ScreenConfig {
	return ScreenConfig{
		ID:    HelpScreenID,
		Title: "Help - Keyboard Shortcuts",
		// No ResourceType - help screen doesn't display K8s resources
		Columns: []ColumnConfig{
			{
				Field:    "Section",
				Title:    "Section",
				Width:    0,
				MinWidth: 12,
				MaxWidth: 15,
				Weight:   1.0,
				Priority: 1,
			},
			{
				Field:    "Shortcut",
				Title:    "Shortcut",
				Width:    0,
				MinWidth: 16,
				MaxWidth: 20,
				Weight:   1.5,
				Priority: 1,
			},
			{
				Field:    "Description",
				Title:    "Description",
				Width:    0,
				MinWidth: 30,
				MaxWidth: 0, // No max
				Weight:   4.0,
				Priority: 1,
			},
		},
		SearchFields: []string{"Section", "Shortcut", "Description"},
		Operations:   []OperationConfig{},

		// CustomRefresh populates data asynchronously (like resource screens)
		CustomRefresh: func(s *ConfigScreen) tea.Cmd {
			return func() tea.Msg {
				// Populate data asynchronously
				entries := getHelpEntries()
				items := make([]interface{}, len(entries))
				for i, entry := range entries {
					items[i] = entry
				}
				s.items = items
				s.applyFilter()
				return types.RefreshCompleteMsg{Duration: 0}
			}
		},
	}
}
