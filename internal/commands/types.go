package commands

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/renato0307/k1/internal/k8s"
)

// ResourceInfo contains identifying information about a Kubernetes resource
type ResourceInfo struct {
	Name      string
	Namespace string
	Kind      k8s.ResourceType
}

// CommandCategory represents the type of command
type CommandCategory int

const (
	CategoryResource  CommandCategory = iota // : prefix (screens, namespaces)
	CategoryAction                           // / prefix (yaml, describe, delete, logs)
	CategoryLLMAction                        // /ai prefix (natural language commands)
)

// CommandContext provides context for command execution
type CommandContext struct {
	ResourceType k8s.ResourceType // Type of resource (pods, deployments, services)
	Selected     map[string]any   // Selected resource data (name, namespace, etc.)
	Args         string           // Additional command arguments (inline args string)
}

// GetResourceInfo extracts resource identification from the context
func (ctx *CommandContext) GetResourceInfo() ResourceInfo {
	name, _ := ctx.Selected["Name"].(string)
	namespace, _ := ctx.Selected["Namespace"].(string)
	return ResourceInfo{
		Name:      name,
		Namespace: namespace,
		Kind:      ctx.ResourceType,
	}
}

// ParseArgs parses inline args string into a typed struct using reflection
// Usage: ctx.ParseArgs(&myArgsStruct)
func (ctx *CommandContext) ParseArgs(dest any) error {
	return ParseInlineArgs(dest, ctx.Args)
}

// ExecuteFunc is a function that executes a command and returns a Bubble Tea command
type ExecuteFunc func(ctx CommandContext) tea.Cmd

// Command represents a command in the palette
type Command struct {
	Name              string             // Short command name (e.g., "pods", "yaml")
	Description       string             // Human-readable description
	Category          CommandCategory    // Command category
	NeedsConfirmation bool               // Whether the command requires confirmation
	Execute           ExecuteFunc        // Execution function
	ResourceTypes     []k8s.ResourceType // Resource types this command applies to (empty = all)
	Shortcut          string             // Keyboard shortcut (e.g., "ctrl+y")
	ArgsType          any                // Pointer to args struct (e.g., &ScaleArgs{}) for reflection
	ArgPattern        string             // Display pattern for palette (e.g., " <replicas>" or " [grace] [force]")
}
