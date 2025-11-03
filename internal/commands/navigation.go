package commands

import (
	"sort"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/renato0307/k1/internal/k8s"
	"github.com/renato0307/k1/internal/messages"
	"github.com/renato0307/k1/internal/types"
)

// navigationRegistry maps screen IDs to their names
// This table-driven approach eliminates 11 nearly-identical functions
var navigationRegistry = map[string]string{
	"pods":                        "pods",
	"deployments":                 "deployments",
	"services":                    "services",
	"configmaps":                  "configmaps",
	"secrets":                     "secrets",
	"namespaces":                  "namespaces",
	"statefulsets":                "statefulsets",
	"daemonsets":                  "daemonsets",
	"jobs":                        "jobs",
	"cronjobs":                    "cronjobs",
	"nodes":                       "nodes",
	"customresourcedefinitions":   "customresourcedefinitions",
	"crds":                        "customresourcedefinitions", // Alias
	"system-resources":            "system-resources",
	"output":                      "output",
	"contexts":                    "contexts",
}

// NavigationCommand returns execute function for switching to a screen
func NavigationCommand(screenID string) ExecuteFunc {
	return func(ctx CommandContext) tea.Cmd {
		return func() tea.Msg {
			return types.ScreenSwitchMsg{ScreenID: screenID}
		}
	}
}

// Legacy navigation command functions for backward compatibility
// These now delegate to the table-driven NavigationCommand

func PodsCommand() ExecuteFunc {
	return NavigationCommand("pods")
}

func DeploymentsCommand() ExecuteFunc {
	return NavigationCommand("deployments")
}

func ServicesCommand() ExecuteFunc {
	return NavigationCommand("services")
}

func ConfigMapsCommand() ExecuteFunc {
	return NavigationCommand("configmaps")
}

func SecretsCommand() ExecuteFunc {
	return NavigationCommand("secrets")
}

func NamespacesCommand() ExecuteFunc {
	return NavigationCommand("namespaces")
}

func StatefulSetsCommand() ExecuteFunc {
	return NavigationCommand("statefulsets")
}

func DaemonSetsCommand() ExecuteFunc {
	return NavigationCommand("daemonsets")
}

func JobsCommand() ExecuteFunc {
	return NavigationCommand("jobs")
}

func CronJobsCommand() ExecuteFunc {
	return NavigationCommand("cronjobs")
}

func NodesCommand() ExecuteFunc {
	return NavigationCommand("nodes")
}

// NamespaceFilterCommand returns execute function for filtering by namespace
func NamespaceFilterCommand() ExecuteFunc {
	return func(ctx CommandContext) tea.Cmd {
		// Phase 3: Return placeholder message (namespace filtering needs state management)
		return messages.InfoCmd("Namespace filtering - Coming soon")
	}
}

// ContextsCommand navigates to contexts screen
func ContextsCommand() ExecuteFunc {
	return NavigationCommand("contexts")
}

// NextContextCommand switches to next loaded context alphabetically
func NextContextCommand(pool *k8s.RepositoryPool) ExecuteFunc {
	return func(ctx CommandContext) tea.Cmd {
		return func() tea.Msg {
			allContexts, err := pool.GetContexts()
			if err != nil {
				return messages.ErrorCmd("Failed to list contexts: %v", err)()
			}

			// Filter to only loaded contexts
			loadedContexts := make([]k8s.Context, 0)
			for _, c := range allContexts {
				if c.Status == string(k8s.StatusLoaded) {
					loadedContexts = append(loadedContexts, c)
				}
			}

			if len(loadedContexts) == 0 {
				return messages.ErrorCmd("No loaded contexts available")()
			}

			if len(loadedContexts) == 1 {
				return nil // Silently do nothing if only one context
			}

			// Sort contexts alphabetically
			sort.Slice(loadedContexts, func(i, j int) bool {
				return loadedContexts[i].Name < loadedContexts[j].Name
			})

			// Find current context
			current := pool.GetActiveContext()
			currentIdx := -1
			for i, c := range loadedContexts {
				if c.Name == current {
					currentIdx = i
					break
				}
			}

			// Get next context (wrap around)
			var nextIdx int
			if currentIdx == -1 {
				// Current context not in loaded list, start from first
				nextIdx = 0
			} else {
				nextIdx = (currentIdx + 1) % len(loadedContexts)
			}
			nextContext := loadedContexts[nextIdx].Name

			// If next context is same as current, don't switch
			if nextContext == current {
				return nil
			}

			return types.ContextSwitchMsg{
				ContextName: nextContext,
			}
		}
	}
}

// PrevContextCommand switches to previous loaded context alphabetically
func PrevContextCommand(pool *k8s.RepositoryPool) ExecuteFunc {
	return func(ctx CommandContext) tea.Cmd {
		return func() tea.Msg {
			allContexts, err := pool.GetContexts()
			if err != nil {
				return messages.ErrorCmd("Failed to list contexts: %v", err)()
			}

			// Filter to only loaded contexts
			loadedContexts := make([]k8s.Context, 0)
			for _, c := range allContexts {
				if c.Status == string(k8s.StatusLoaded) {
					loadedContexts = append(loadedContexts, c)
				}
			}

			if len(loadedContexts) == 0 {
				return messages.ErrorCmd("No loaded contexts available")()
			}

			if len(loadedContexts) == 1 {
				return nil // Silently do nothing if only one context
			}

			// Sort contexts alphabetically
			sort.Slice(loadedContexts, func(i, j int) bool {
				return loadedContexts[i].Name < loadedContexts[j].Name
			})

			// Find current context
			current := pool.GetActiveContext()
			currentIdx := -1
			for i, c := range loadedContexts {
				if c.Name == current {
					currentIdx = i
					break
				}
			}

			// Get previous context (wrap around)
			var prevIdx int
			if currentIdx == -1 {
				// Current context not in loaded list, start from last
				prevIdx = len(loadedContexts) - 1
			} else {
				prevIdx = (currentIdx - 1 + len(loadedContexts)) % len(loadedContexts)
			}
			prevContext := loadedContexts[prevIdx].Name

			// If previous context is same as current, don't switch
			if prevContext == current {
				return nil
			}

			return types.ContextSwitchMsg{
				ContextName: prevContext,
			}
		}
	}
}
