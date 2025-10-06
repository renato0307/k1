package commands

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/renato0307/k1/internal/k8s"
	"github.com/renato0307/k1/internal/types"
)

// CordonCommand returns execute function for cordoning nodes
func CordonCommand(repo k8s.Repository) ExecuteFunc {
	return func(ctx CommandContext) tea.Cmd {
		resourceName := "unknown"
		if name, ok := ctx.Selected["name"].(string); ok {
			resourceName = name
		}
		return func() tea.Msg {
			return types.InfoMsg("Cordon node/" + resourceName + " - Coming soon")
		}
	}
}

// DrainCommand returns execute function for draining nodes
func DrainCommand(repo k8s.Repository) ExecuteFunc {
	return func(ctx CommandContext) tea.Cmd {
		resourceName := "unknown"
		if name, ok := ctx.Selected["name"].(string); ok {
			resourceName = name
		}
		return func() tea.Msg {
			return types.InfoMsg("Drain node/" + resourceName + " - Coming soon")
		}
	}
}
