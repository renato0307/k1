package screens

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/renato0307/k1/internal/k8s"
)

// GetPodsScreenConfig returns the config for the Pods screen (Level 2 - with periodic refresh)
func GetPodsScreenConfig() ScreenConfig {
	return ScreenConfig{
		ID:           "pods",
		Title:        "Pods",
		ResourceType: k8s.ResourceTypePod,
		Columns: []ColumnConfig{
			{Field: "Namespace", Title: "Namespace", Width: 20},
			{Field: "Name", Title: "Name", Width: 0}, // Dynamic width
			{Field: "Ready", Title: "Ready", Width: 8},
			{Field: "Status", Title: "Status", Width: 15},
			{Field: "Restarts", Title: "Restarts", Width: 10},
			{Field: "Age", Title: "Age", Width: 10, Format: FormatDuration},
			{Field: "Node", Title: "Node", Width: 30},
			{Field: "IP", Title: "IP", Width: 16},
		},
		SearchFields: []string{"Namespace", "Name", "Status", "Node", "IP"},
		Operations: []OperationConfig{
			{ID: "logs", Name: "View Logs", Description: "View logs for selected pod", Shortcut: "l"},
			{ID: "describe", Name: "Describe", Description: "Describe selected pod", Shortcut: "d"},
			{ID: "delete", Name: "Delete", Description: "Delete selected pod", Shortcut: "x"},
		},
		EnablePeriodicRefresh: true,
		RefreshInterval:       1 * time.Second,
		TrackSelection:        true,
		// Custom update handler to support periodic refresh
		CustomUpdate: func(s *ConfigScreen, msg tea.Msg) (tea.Model, tea.Cmd) {
			switch msg.(type) {
			case tickMsg:
				// Refresh and schedule next tick
				return s, tea.Batch(s.Refresh(), tickCmd())
			default:
				return s.DefaultUpdate(msg)
			}
		},
	}
}

// GetDeploymentsScreenConfig returns the config for the Deployments screen (Level 1 - pure config)
func GetDeploymentsScreenConfig() ScreenConfig {
	return ScreenConfig{
		ID:           "deployments",
		Title:        "Deployments",
		ResourceType: k8s.ResourceTypeDeployment,
		Columns: []ColumnConfig{
			{Field: "Namespace", Title: "Namespace", Width: 20},
			{Field: "Name", Title: "Name", Width: 0}, // Dynamic width
			{Field: "Ready", Title: "Ready", Width: 10},
			{Field: "UpToDate", Title: "Up-to-date", Width: 12},
			{Field: "Available", Title: "Available", Width: 12},
			{Field: "Age", Title: "Age", Width: 10, Format: FormatDuration},
		},
		SearchFields: []string{"Namespace", "Name"},
		Operations: []OperationConfig{
			{ID: "scale", Name: "Scale", Description: "Scale selected deployment", Shortcut: "s"},
			{ID: "restart", Name: "Restart", Description: "Restart selected deployment", Shortcut: "r"},
			{ID: "describe", Name: "Describe", Description: "Describe selected deployment", Shortcut: "d"},
		},
		TrackSelection: false, // No need for cursor tracking on deployments
	}
}

// GetServicesScreenConfig returns the config for the Services screen (Level 1 - pure config)
func GetServicesScreenConfig() ScreenConfig {
	return ScreenConfig{
		ID:           "services",
		Title:        "Services",
		ResourceType: k8s.ResourceTypeService,
		Columns: []ColumnConfig{
			{Field: "Namespace", Title: "Namespace", Width: 20},
			{Field: "Name", Title: "Name", Width: 0}, // Dynamic width
			{Field: "Type", Title: "Type", Width: 15},
			{Field: "ClusterIP", Title: "Cluster-IP", Width: 15},
			{Field: "ExternalIP", Title: "External-IP", Width: 15},
			{Field: "Ports", Title: "Ports", Width: 20},
			{Field: "Age", Title: "Age", Width: 10, Format: FormatDuration},
		},
		SearchFields: []string{"Namespace", "Name", "Type"},
		Operations: []OperationConfig{
			{ID: "describe", Name: "Describe", Description: "Describe selected service", Shortcut: "d"},
			{ID: "endpoints", Name: "Show Endpoints", Description: "Show service endpoints", Shortcut: "e"},
			{ID: "delete", Name: "Delete", Description: "Delete selected service", Shortcut: "x"},
		},
		TrackSelection: false,
	}
}

// tickCmd returns a command that sends a tickMsg after 1 second
func tickCmd() tea.Cmd {
	return tea.Tick(1*time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}
