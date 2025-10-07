package commands

import (
	"fmt"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/renato0307/k1/internal/k8s"
	"github.com/renato0307/k1/internal/messages"
)

// ScaleArgs defines arguments for scale command
type ScaleArgs struct {
	Replicas int `form:"replicas" title:"Replicas" validate:"required,min=0,max=100"`
}

// ScaleCommand returns execute function for scaling deployments/statefulsets
func ScaleCommand(repo k8s.Repository) ExecuteFunc {
	return func(ctx CommandContext) tea.Cmd {
		// Parse args
		var args ScaleArgs
		if err := ctx.ParseArgs(&args); err != nil {
			return messages.ErrorCmd("Invalid args: %v", err)
		}

		// Get resource info
		resourceName := "unknown"
		namespace := "default"
		if name, ok := ctx.Selected["name"].(string); ok {
			resourceName = name
		}
		if ns, ok := ctx.Selected["namespace"].(string); ok {
			namespace = ns
		}

		// Build kubectl scale command
		kubectlArgs := []string{
			"scale",
			string(ctx.ResourceType),
			resourceName,
			"--namespace", namespace,
			"--replicas", strconv.Itoa(args.Replicas),
		}

		// Return a command that executes kubectl asynchronously
		// Bubble Tea will run this in a separate goroutine
		return func() tea.Msg {
			executor := NewKubectlExecutor(repo.GetKubeconfig(), repo.GetContext())

			// Debug: show the command being run
			cmdStr := "kubectl " + strings.Join(kubectlArgs, " ")

			output, err := executor.Execute(kubectlArgs, ExecuteOptions{})

			if err != nil {
				return messages.ErrorCmd("Scale failed: %v (cmd: %s)", err, cmdStr)()
			}
			// Show success with kubectl output and command
			msg := fmt.Sprintf("%s (replicas=%d)", strings.TrimSpace(output), args.Replicas)
			if output == "" {
				msg = fmt.Sprintf("Scaled %s/%s to %d replicas", ctx.ResourceType, resourceName, args.Replicas)
			}
			return messages.SuccessCmd("%s", msg)()
		}
	}
}

// RestartCommand returns execute function for restarting deployments
func RestartCommand(repo k8s.Repository) ExecuteFunc {
	return func(ctx CommandContext) tea.Cmd {
		// Get resource info
		resourceName := "unknown"
		namespace := "default"
		if name, ok := ctx.Selected["name"].(string); ok {
			resourceName = name
		}
		if ns, ok := ctx.Selected["namespace"].(string); ok {
			namespace = ns
		}

		// Build kubectl rollout restart command
		kubectlArgs := []string{
			"rollout",
			"restart",
			string(ctx.ResourceType) + "/" + resourceName,
			"--namespace", namespace,
		}

		// Return a command that executes kubectl asynchronously
		return func() tea.Msg {
			executor := NewKubectlExecutor(repo.GetKubeconfig(), repo.GetContext())
			output, err := executor.Execute(kubectlArgs, ExecuteOptions{})

			if err != nil {
				return messages.ErrorCmd("Restart failed: %v", err)()
			}
			msg := fmt.Sprintf("Restarted %s/%s", ctx.ResourceType, resourceName)
			if output != "" {
				msg = strings.TrimSpace(output)
			}
			return messages.SuccessCmd("%s", msg)()
		}
	}
}
