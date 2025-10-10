package commands

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/renato0307/k1/internal/k8s"
	"github.com/renato0307/k1/internal/messages"
	"github.com/renato0307/k1/internal/types"
)

// ContextArgs defines arguments for context switch command
type ContextArgs struct {
	ContextName string `inline:"0"`
}

// ContextCommand creates a command to switch Kubernetes context
func ContextCommand(pool *k8s.RepositoryPool) ExecuteFunc {
	return func(ctx CommandContext) tea.Cmd {
		var args ContextArgs
		if err := ctx.ParseArgs(&args); err != nil {
			return messages.ErrorCmd("Invalid args: %v", err)
		}

		return func() tea.Msg {
			return types.ContextSwitchMsg{
				ContextName: args.ContextName,
			}
		}
	}
}
