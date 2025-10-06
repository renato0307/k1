package commands

import (
	"fmt"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/renato0307/k1/internal/k8s"
	"github.com/renato0307/k1/internal/types"
)

// DrainArgs defines arguments for drain command
type DrainArgs struct {
	GracePeriod      int  `form:"grace" title:"Grace Period (seconds)" default:"30"`
	Force            bool `form:"force" title:"Force Drain" default:"false"`
	IgnoreDaemonsets bool `form:"ignore-daemonsets" title:"Ignore DaemonSets" default:"true"`
}

// CordonCommand returns execute function for cordoning nodes
func CordonCommand(repo k8s.Repository) ExecuteFunc {
	return func(ctx CommandContext) tea.Cmd {
		resourceName := "unknown"
		if name, ok := ctx.Selected["name"].(string); ok {
			resourceName = name
		}

		// Build kubectl cordon command
		kubectlArgs := []string{
			"cordon",
			resourceName,
		}

		// Return a command that executes kubectl asynchronously
		return func() tea.Msg {
			executor := NewKubectlExecutor(repo.GetKubeconfig(), repo.GetContext())
			output, err := executor.Execute(kubectlArgs, ExecuteOptions{})

			if err != nil {
				return types.ErrorStatusMsg(fmt.Sprintf("Cordon failed: %v", err))
			}
			msg := fmt.Sprintf("Cordoned node/%s", resourceName)
			if output != "" {
				msg = strings.TrimSpace(output)
			}
			return types.SuccessMsg(msg)
		}
	}
}

// DrainCommand returns execute function for draining nodes
func DrainCommand(repo k8s.Repository) ExecuteFunc {
	return func(ctx CommandContext) tea.Cmd {
		// Parse args (with optional inline args)
		var args DrainArgs
		if err := ctx.ParseArgs(&args); err != nil {
			return func() tea.Msg {
				return types.ErrorStatusMsg(fmt.Sprintf("Invalid args: %v", err))
			}
		}

		resourceName := "unknown"
		if name, ok := ctx.Selected["name"].(string); ok {
			resourceName = name
		}

		// Build kubectl drain command
		kubectlArgs := []string{
			"drain",
			resourceName,
			"--grace-period", strconv.Itoa(args.GracePeriod),
			"--delete-emptydir-data",
		}

		// Add optional flags
		if args.Force {
			kubectlArgs = append(kubectlArgs, "--force")
		}
		if args.IgnoreDaemonsets {
			kubectlArgs = append(kubectlArgs, "--ignore-daemonsets")
		}

		// Return a command that executes kubectl asynchronously
		return func() tea.Msg {
			executor := NewKubectlExecutor(repo.GetKubeconfig(), repo.GetContext())
			output, err := executor.Execute(kubectlArgs, ExecuteOptions{})

			if err != nil {
				return types.ErrorStatusMsg(fmt.Sprintf("Drain failed: %v", err))
			}
			msg := fmt.Sprintf("Drained node/%s", resourceName)
			if output != "" {
				msg = strings.TrimSpace(output)
			}
			return types.SuccessMsg(msg)
		}
	}
}
