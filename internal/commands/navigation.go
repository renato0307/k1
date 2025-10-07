package commands

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/renato0307/k1/internal/messages"
	"github.com/renato0307/k1/internal/types"
)

// navigationRegistry maps screen IDs to their names
// This table-driven approach eliminates 11 nearly-identical functions
var navigationRegistry = map[string]string{
	"pods":         "pods",
	"deployments":  "deployments",
	"services":     "services",
	"configmaps":   "configmaps",
	"secrets":      "secrets",
	"namespaces":   "namespaces",
	"statefulsets": "statefulsets",
	"daemonsets":   "daemonsets",
	"jobs":         "jobs",
	"cronjobs":     "cronjobs",
	"nodes":        "nodes",
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
