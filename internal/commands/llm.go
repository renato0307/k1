package commands

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/renato0307/k1/internal/k8s"
	"github.com/renato0307/k1/internal/messages"
)

// LLMDeleteFailingPodsCommand returns execute function for LLM example command.
// TODO: Implement when DDR-12 (Local LLM Architecture) is complete.
func LLMDeleteFailingPodsCommand(provider k8s.KubeconfigProvider) ExecuteFunc {
	return func(ctx CommandContext) tea.Cmd {
		return messages.InfoCmd("LLM commands not yet implemented (see DDR-12)")
	}
}

// LLMScaleNginxCommand returns execute function for LLM example command.
// TODO: Implement when DDR-12 (Local LLM Architecture) is complete.
func LLMScaleNginxCommand(provider k8s.KubeconfigProvider) ExecuteFunc {
	return func(ctx CommandContext) tea.Cmd {
		return messages.InfoCmd("LLM commands not yet implemented (see DDR-12)")
	}
}

// LLMGetPodLogsCommand returns execute function for LLM example command.
// TODO: Implement when DDR-12 (Local LLM Architecture) is complete.
func LLMGetPodLogsCommand(provider k8s.KubeconfigProvider) ExecuteFunc {
	return func(ctx CommandContext) tea.Cmd {
		return messages.InfoCmd("LLM commands not yet implemented (see DDR-12)")
	}
}

// LLMRestartDeploymentCommand returns execute function for LLM example command.
// TODO: Implement when DDR-12 (Local LLM Architecture) is complete.
func LLMRestartDeploymentCommand(provider k8s.KubeconfigProvider) ExecuteFunc {
	return func(ctx CommandContext) tea.Cmd {
		return messages.InfoCmd("LLM commands not yet implemented (see DDR-12)")
	}
}

// LLMShowPodEventsCommand returns execute function for LLM example command.
// TODO: Implement when DDR-12 (Local LLM Architecture) is complete.
func LLMShowPodEventsCommand(provider k8s.KubeconfigProvider) ExecuteFunc {
	return func(ctx CommandContext) tea.Cmd {
		return messages.InfoCmd("LLM commands not yet implemented (see DDR-12)")
	}
}
