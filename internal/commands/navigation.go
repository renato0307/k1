package commands

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/renato0307/k1/internal/types"
)

// PodsCommand returns execute function for switching to pods screen
func PodsCommand() ExecuteFunc {
	return func(ctx CommandContext) tea.Cmd {
		return func() tea.Msg {
			return types.ScreenSwitchMsg{ScreenID: "pods"}
		}
	}
}

// DeploymentsCommand returns execute function for switching to deployments screen
func DeploymentsCommand() ExecuteFunc {
	return func(ctx CommandContext) tea.Cmd {
		return func() tea.Msg {
			return types.ScreenSwitchMsg{ScreenID: "deployments"}
		}
	}
}

// ServicesCommand returns execute function for switching to services screen
func ServicesCommand() ExecuteFunc {
	return func(ctx CommandContext) tea.Cmd {
		return func() tea.Msg {
			return types.ScreenSwitchMsg{ScreenID: "services"}
		}
	}
}

// ConfigMapsCommand returns execute function for switching to configmaps screen
func ConfigMapsCommand() ExecuteFunc {
	return func(ctx CommandContext) tea.Cmd {
		return func() tea.Msg {
			return types.ScreenSwitchMsg{ScreenID: "configmaps"}
		}
	}
}

// SecretsCommand returns execute function for switching to secrets screen
func SecretsCommand() ExecuteFunc {
	return func(ctx CommandContext) tea.Cmd {
		return func() tea.Msg {
			return types.ScreenSwitchMsg{ScreenID: "secrets"}
		}
	}
}

// NamespacesCommand returns execute function for switching to namespaces screen
func NamespacesCommand() ExecuteFunc {
	return func(ctx CommandContext) tea.Cmd {
		return func() tea.Msg {
			return types.ScreenSwitchMsg{ScreenID: "namespaces"}
		}
	}
}

// StatefulSetsCommand returns execute function for switching to statefulsets screen
func StatefulSetsCommand() ExecuteFunc {
	return func(ctx CommandContext) tea.Cmd {
		return func() tea.Msg {
			return types.ScreenSwitchMsg{ScreenID: "statefulsets"}
		}
	}
}

// DaemonSetsCommand returns execute function for switching to daemonsets screen
func DaemonSetsCommand() ExecuteFunc {
	return func(ctx CommandContext) tea.Cmd {
		return func() tea.Msg {
			return types.ScreenSwitchMsg{ScreenID: "daemonsets"}
		}
	}
}

// JobsCommand returns execute function for switching to jobs screen
func JobsCommand() ExecuteFunc {
	return func(ctx CommandContext) tea.Cmd {
		return func() tea.Msg {
			return types.ScreenSwitchMsg{ScreenID: "jobs"}
		}
	}
}

// CronJobsCommand returns execute function for switching to cronjobs screen
func CronJobsCommand() ExecuteFunc {
	return func(ctx CommandContext) tea.Cmd {
		return func() tea.Msg {
			return types.ScreenSwitchMsg{ScreenID: "cronjobs"}
		}
	}
}

// NodesCommand returns execute function for switching to nodes screen
func NodesCommand() ExecuteFunc {
	return func(ctx CommandContext) tea.Cmd {
		return func() tea.Msg {
			return types.ScreenSwitchMsg{ScreenID: "nodes"}
		}
	}
}

// NamespaceFilterCommand returns execute function for filtering by namespace
func NamespaceFilterCommand() ExecuteFunc {
	return func(ctx CommandContext) tea.Cmd {
		// Phase 3: Return placeholder message (namespace filtering needs state management)
		return func() tea.Msg {
			return types.ErrorMsg{Error: "Namespace filtering - Coming soon"}
		}
	}
}
