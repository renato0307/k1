package commands

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/renato0307/k1/internal/k8s"
	"github.com/renato0307/k1/internal/types"
)

// ScaleCommand returns execute function for scaling deployments/statefulsets
func ScaleCommand(repo k8s.Repository) ExecuteFunc {
	return func(ctx CommandContext) tea.Cmd {
		resourceName := "unknown"
		if name, ok := ctx.Selected["name"].(string); ok {
			resourceName = name
		}
		return func() tea.Msg {
			return types.ErrorMsg{Error: "Scale " + ctx.ResourceType + "/" + resourceName + " - Coming soon"}
		}
	}
}

// RestartCommand returns execute function for restarting deployments
func RestartCommand(repo k8s.Repository) ExecuteFunc {
	return func(ctx CommandContext) tea.Cmd {
		resourceName := "unknown"
		if name, ok := ctx.Selected["name"].(string); ok {
			resourceName = name
		}
		return func() tea.Msg {
			return types.ErrorMsg{Error: "Restart deployment/" + resourceName + " - Coming soon"}
		}
	}
}
