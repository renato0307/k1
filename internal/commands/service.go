package commands

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/renato0307/k1/internal/k8s"
	"github.com/renato0307/k1/internal/types"
)

// EndpointsCommand returns execute function for showing service endpoints
func EndpointsCommand(repo k8s.Repository) ExecuteFunc {
	return func(ctx CommandContext) tea.Cmd {
		resourceName := "unknown"
		if name, ok := ctx.Selected["name"].(string); ok {
			resourceName = name
		}
		return func() tea.Msg {
			return types.ErrorMsg{Error: "Show endpoints for service/" + resourceName + " - Coming soon"}
		}
	}
}
