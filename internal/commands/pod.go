package commands

import (
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/renato0307/k1/internal/k8s"
	"github.com/renato0307/k1/internal/messages"
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
func ShellCommand(provider k8s.KubeconfigProvider) ExecuteFunc {
	return func(ctx CommandContext) tea.Cmd {
		// Parse args
		var args ShellArgs
		if err := ctx.ParseArgs(&args); err != nil {
			return messages.ErrorCmd("Invalid args: %v", err)
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
		if provider.GetKubeconfig() != "" {
			kubectlCmd.WriteString(" --kubeconfig ")
			kubectlCmd.WriteString(provider.GetKubeconfig())
		}
		if provider.GetContext() != "" {
			kubectlCmd.WriteString(" --context ")
			kubectlCmd.WriteString(provider.GetContext())
		}

		// Add shell
		kubectlCmd.WriteString(" -- ")
		kubectlCmd.WriteString(args.Shell)

		command := kubectlCmd.String()

		return func() tea.Msg {
			msg, err := CopyToClipboard(command)
			if err != nil {
				return messages.ErrorCmd("Failed to copy: %v", err)()
			}
			return messages.InfoCmd("%s", msg)()
		}
	}
}

// LogsCommand returns execute function for viewing pod logs (clipboard mode)
func LogsCommand(provider k8s.KubeconfigProvider) ExecuteFunc {
	return func(ctx CommandContext) tea.Cmd {
		// Parse args
		var args LogsArgs
		if err := ctx.ParseArgs(&args); err != nil {
			return messages.ErrorCmd("Invalid args: %v", err)
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
		if provider.GetKubeconfig() != "" {
			kubectlCmd.WriteString(" --kubeconfig ")
			kubectlCmd.WriteString(provider.GetKubeconfig())
		}
		if provider.GetContext() != "" {
			kubectlCmd.WriteString(" --context ")
			kubectlCmd.WriteString(provider.GetContext())
		}

		command := kubectlCmd.String()

		return func() tea.Msg {
			msg, err := CopyToClipboard(command)
			if err != nil {
				return messages.ErrorCmd("Failed to copy: %v", err)()
			}
			return messages.InfoCmd("%s", msg)()
		}
	}
}

// LogsPreviousCommand returns execute function for viewing previous pod logs
func LogsPreviousCommand(provider k8s.KubeconfigProvider) ExecuteFunc {
	return func(ctx CommandContext) tea.Cmd {
		resourceName := "unknown"
		if name, ok := ctx.Selected["name"].(string); ok {
			resourceName = name
		}
		return messages.InfoCmd("Previous logs for pod/%s - Coming soon", resourceName)
	}
}

// PortForwardCommand returns execute function for port forwarding to pod (clipboard mode)
func PortForwardCommand(provider k8s.KubeconfigProvider) ExecuteFunc {
	return func(ctx CommandContext) tea.Cmd {
		// Parse args
		var args PortForwardArgs
		if err := ctx.ParseArgs(&args); err != nil {
			return messages.ErrorCmd("Invalid args: %v", err)
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
		if provider.GetKubeconfig() != "" {
			kubectlCmd.WriteString(" --kubeconfig ")
			kubectlCmd.WriteString(provider.GetKubeconfig())
		}
		if provider.GetContext() != "" {
			kubectlCmd.WriteString(" --context ")
			kubectlCmd.WriteString(provider.GetContext())
		}

		command := kubectlCmd.String()

		return func() tea.Msg {
			msg, err := CopyToClipboard(command)
			if err != nil {
				return messages.ErrorCmd("Failed to copy: %v", err)()
			}
			return messages.InfoCmd("%s", msg)()
		}
	}
}

// JumpOwnerCommand returns execute function for jumping to owner resource
func JumpOwnerCommand(provider k8s.KubeconfigProvider) ExecuteFunc {
	return func(ctx CommandContext) tea.Cmd {
		resourceName := "unknown"
		if name, ok := ctx.Selected["name"].(string); ok {
			resourceName = name
		}
		return messages.InfoCmd("Jump to owner of pod/%s - Coming soon", resourceName)
	}
}

// ShowNodeCommand returns execute function for showing node details
func ShowNodeCommand(provider k8s.KubeconfigProvider) ExecuteFunc {
	return func(ctx CommandContext) tea.Cmd {
		resourceName := "unknown"
		nodeName := "unknown"
		if name, ok := ctx.Selected["name"].(string); ok {
			resourceName = name
		}
		if node, ok := ctx.Selected["node"].(string); ok {
			nodeName = node
		}
		return messages.InfoCmd("Show node %s for pod/%s - Coming soon", nodeName, resourceName)
	}
}
