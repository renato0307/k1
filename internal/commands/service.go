package commands

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/renato0307/k1/internal/k8s"
	"github.com/renato0307/k1/internal/messages"
	"github.com/renato0307/k1/internal/types"
)

// EndpointsCommand returns execute function for showing service endpoints
func EndpointsCommand(pool *k8s.RepositoryPool) ExecuteFunc {
	return func(ctx CommandContext) tea.Cmd {
		resourceName := "unknown"
		namespace := "default"
		if name, ok := ctx.Selected["name"].(string); ok {
			resourceName = name
		}
		if ns, ok := ctx.Selected["namespace"].(string); ok {
			namespace = ns
		}

		// Build kubectl get endpoints command
		kubectlArgs := []string{
			"get",
			"endpoints",
			resourceName,
			"--namespace", namespace,
			"-o", "wide",
		}

		// Return a command that executes kubectl asynchronously
		return func() tea.Msg {
			repo := pool.GetActiveRepository()
			if repo == nil {
				return messages.ErrorCmd("No active repository")()
			}
			executor := NewKubectlExecutor(repo.GetKubeconfig(), repo.GetContext())
			output, err := executor.Execute(kubectlArgs, ExecuteOptions{})

			if err != nil {
				return messages.ErrorCmd("Get endpoints failed: %v", err)()
			}

			// Show endpoints in full-screen view
			return types.ShowFullScreenMsg{
				ViewType:     2, // Endpoints (custom view type)
				ResourceName: namespace + "/" + resourceName,
				Content:      strings.TrimSpace(output),
			}
		}
	}
}
