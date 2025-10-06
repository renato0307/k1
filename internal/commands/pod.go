package commands

import (
	"fmt"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/renato0307/k1/internal/k8s"
	"github.com/renato0307/k1/internal/types"
)

// ShellArgs defines arguments for shell command
type ShellArgs struct {
	Container string `form:"container" title:"Container" optional:"true"`
	Shell     string `form:"shell" title:"Shell" default:"/bin/sh" optional:"true"`
}

// LogsArgs defines arguments for logs command
type LogsArgs struct {
	Container string `form:"container" title:"Container" optional:"true"`
	Tail      int    `form:"tail" title:"Tail Lines" default:"100" optional:"true" validate:"min=0"`
	Follow    bool   `form:"follow" title:"Follow" default:"false" optional:"true"`
}

// PortForwardArgs defines arguments for port-forward command
type PortForwardArgs struct {
	Ports string `form:"ports" title:"Port Mapping (local:remote)" validate:"required"`
}

// ShellCommand returns execute function for opening shell in pod (clipboard mode)
func ShellCommand(repo k8s.Repository) ExecuteFunc {
	return func(ctx CommandContext) tea.Cmd {
		// Parse args
		var args ShellArgs
		if err := ctx.ParseArgs(&args); err != nil {
			return func() tea.Msg {
				return types.ErrorStatusMsg(fmt.Sprintf("Invalid args: %v", err))
			}
		}

		// Get pod info
		podName := "unknown"
		namespace := "default"
		if name, ok := ctx.Selected["name"].(string); ok {
			podName = name
		}
		if ns, ok := ctx.Selected["namespace"].(string); ok {
			namespace = ns
		}

		// Build kubectl exec command
		var kubectlCmd strings.Builder
		kubectlCmd.WriteString("kubectl exec -it ")
		kubectlCmd.WriteString(podName)
		kubectlCmd.WriteString(" --namespace ")
		kubectlCmd.WriteString(namespace)

		// Add container if specified
		if args.Container != "" {
			kubectlCmd.WriteString(" -c ")
			kubectlCmd.WriteString(args.Container)
		}

		// Add kubeconfig/context if set
		if repo.GetKubeconfig() != "" {
			kubectlCmd.WriteString(" --kubeconfig ")
			kubectlCmd.WriteString(repo.GetKubeconfig())
		}
		if repo.GetContext() != "" {
			kubectlCmd.WriteString(" --context ")
			kubectlCmd.WriteString(repo.GetContext())
		}

		// Add shell
		kubectlCmd.WriteString(" -- ")
		kubectlCmd.WriteString(args.Shell)

		command := kubectlCmd.String()

		return func() tea.Msg {
			msg, err := CopyToClipboard(command)
			if err != nil {
				return types.ErrorStatusMsg(fmt.Sprintf("Failed to copy: %v", err))
			}
			return types.InfoMsg(msg)
		}
	}
}

// LogsCommand returns execute function for viewing pod logs (clipboard mode)
func LogsCommand(repo k8s.Repository) ExecuteFunc {
	return func(ctx CommandContext) tea.Cmd {
		// Parse args
		var args LogsArgs
		if err := ctx.ParseArgs(&args); err != nil {
			return func() tea.Msg {
				return types.ErrorStatusMsg(fmt.Sprintf("Invalid args: %v", err))
			}
		}

		// Get pod info
		podName := "unknown"
		namespace := "default"
		if name, ok := ctx.Selected["name"].(string); ok {
			podName = name
		}
		if ns, ok := ctx.Selected["namespace"].(string); ok {
			namespace = ns
		}

		// Build kubectl logs command
		var kubectlCmd strings.Builder
		kubectlCmd.WriteString("kubectl logs ")
		kubectlCmd.WriteString(podName)
		kubectlCmd.WriteString(" --namespace ")
		kubectlCmd.WriteString(namespace)

		// Add container if specified
		if args.Container != "" {
			kubectlCmd.WriteString(" -c ")
			kubectlCmd.WriteString(args.Container)
		}

		// Add tail
		kubectlCmd.WriteString(" --tail=")
		kubectlCmd.WriteString(strconv.Itoa(args.Tail))

		// Add follow flag
		if args.Follow {
			kubectlCmd.WriteString(" -f")
		}

		// Add kubeconfig/context if set
		if repo.GetKubeconfig() != "" {
			kubectlCmd.WriteString(" --kubeconfig ")
			kubectlCmd.WriteString(repo.GetKubeconfig())
		}
		if repo.GetContext() != "" {
			kubectlCmd.WriteString(" --context ")
			kubectlCmd.WriteString(repo.GetContext())
		}

		command := kubectlCmd.String()

		return func() tea.Msg {
			msg, err := CopyToClipboard(command)
			if err != nil {
				return types.ErrorStatusMsg(fmt.Sprintf("Failed to copy: %v", err))
			}
			return types.InfoMsg(msg)
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
			return types.InfoMsg("Previous logs for pod/" + resourceName + " - Coming soon")
		}
	}
}

// PortForwardCommand returns execute function for port forwarding to pod (clipboard mode)
func PortForwardCommand(repo k8s.Repository) ExecuteFunc {
	return func(ctx CommandContext) tea.Cmd {
		// Parse args
		var args PortForwardArgs
		if err := ctx.ParseArgs(&args); err != nil {
			return func() tea.Msg {
				return types.ErrorStatusMsg(fmt.Sprintf("Invalid args: %v", err))
			}
		}

		// Get pod info
		podName := "unknown"
		namespace := "default"
		if name, ok := ctx.Selected["name"].(string); ok {
			podName = name
		}
		if ns, ok := ctx.Selected["namespace"].(string); ok {
			namespace = ns
		}

		// Build kubectl port-forward command
		var kubectlCmd strings.Builder
		kubectlCmd.WriteString("kubectl port-forward ")
		kubectlCmd.WriteString(podName)
		kubectlCmd.WriteString(" --namespace ")
		kubectlCmd.WriteString(namespace)
		kubectlCmd.WriteString(" ")
		kubectlCmd.WriteString(args.Ports)

		// Add kubeconfig/context if set
		if repo.GetKubeconfig() != "" {
			kubectlCmd.WriteString(" --kubeconfig ")
			kubectlCmd.WriteString(repo.GetKubeconfig())
		}
		if repo.GetContext() != "" {
			kubectlCmd.WriteString(" --context ")
			kubectlCmd.WriteString(repo.GetContext())
		}

		command := kubectlCmd.String()

		return func() tea.Msg {
			msg, err := CopyToClipboard(command)
			if err != nil {
				return types.ErrorStatusMsg(fmt.Sprintf("Failed to copy: %v", err))
			}
			return types.InfoMsg(msg)
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
			return types.InfoMsg("Jump to owner of pod/" + resourceName + " - Coming soon")
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
			return types.InfoMsg("Show node " + nodeName + " for pod/" + resourceName + " - Coming soon")
		}
	}
}
