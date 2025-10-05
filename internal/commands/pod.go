package commands

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/renato0307/k1/internal/k8s"
	"github.com/renato0307/k1/internal/types"
)

// LogsCommand returns execute function for viewing pod logs
func LogsCommand(repo k8s.Repository) ExecuteFunc {
	return func(ctx CommandContext) tea.Cmd {
		resourceName := "unknown"
		if name, ok := ctx.Selected["name"].(string); ok {
			resourceName = name
		}
		return func() tea.Msg {
			return types.ErrorMsg{Error: "Logs for pod/" + resourceName + " - Coming in Phase 4"}
		}
	}
}

// LogsPreviousCommand returns execute function for viewing previous pod logs
func LogsPreviousCommand(repo k8s.Repository) ExecuteFunc {
	return func(ctx CommandContext) tea.Cmd {
		resourceName := "unknown"
		if name, ok := ctx.Selected["name"].(string); ok {
			resourceName = name
		}
		return func() tea.Msg {
			return types.ErrorMsg{Error: "Previous logs for pod/" + resourceName + " - Coming soon"}
		}
	}
}

// PortForwardCommand returns execute function for port forwarding to pod
func PortForwardCommand(repo k8s.Repository) ExecuteFunc {
	return func(ctx CommandContext) tea.Cmd {
		resourceName := "unknown"
		if name, ok := ctx.Selected["name"].(string); ok {
			resourceName = name
		}
		return func() tea.Msg {
			return types.ErrorMsg{Error: "Port forward to pod/" + resourceName + " - Coming soon"}
		}
	}
}

// ShellCommand returns execute function for opening shell in pod
func ShellCommand(repo k8s.Repository) ExecuteFunc {
	return func(ctx CommandContext) tea.Cmd {
		resourceName := "unknown"
		if name, ok := ctx.Selected["name"].(string); ok {
			resourceName = name
		}
		return func() tea.Msg {
			return types.ErrorMsg{Error: "Shell for pod/" + resourceName + " - Coming soon"}
		}
	}
}

// JumpOwnerCommand returns execute function for jumping to owner resource
func JumpOwnerCommand(repo k8s.Repository) ExecuteFunc {
	return func(ctx CommandContext) tea.Cmd {
		resourceName := "unknown"
		if name, ok := ctx.Selected["name"].(string); ok {
			resourceName = name
		}
		return func() tea.Msg {
			return types.ErrorMsg{Error: "Jump to owner of pod/" + resourceName + " - Coming soon"}
		}
	}
}

// ShowNodeCommand returns execute function for showing node details
func ShowNodeCommand(repo k8s.Repository) ExecuteFunc {
	return func(ctx CommandContext) tea.Cmd {
		resourceName := "unknown"
		nodeName := "unknown"
		if name, ok := ctx.Selected["name"].(string); ok {
			resourceName = name
		}
		if node, ok := ctx.Selected["node"].(string); ok {
			nodeName = node
		}
		return func() tea.Msg {
			return types.ErrorMsg{Error: "Show node " + nodeName + " for pod/" + resourceName + " - Coming soon"}
		}
	}
}
