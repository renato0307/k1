package commands

import (
	tea "github.com/charmbracelet/bubbletea"
)

// CommandCategory represents the type of command
type CommandCategory int

const (
	CategoryResource  CommandCategory = iota // : prefix (screens, namespaces)
	CategoryAction                           // / prefix (yaml, describe, delete, logs)
	CategoryLLMAction                        // /ai prefix (natural language commands)
)

// CommandContext provides context for command execution
type CommandContext struct {
	ResourceType string                 // Type of resource (pods, deployments, services)
	Selected     map[string]interface{} // Selected resource data (name, namespace, etc.)
	Args         string                 // Additional command arguments
}

// ExecuteFunc is a function that executes a command and returns a Bubble Tea command
type ExecuteFunc func(ctx CommandContext) tea.Cmd

// Command represents a command in the palette
type Command struct {
	Name              string          // Short command name (e.g., "pods", "yaml")
	Description       string          // Human-readable description
	Category          CommandCategory // Command category
	NeedsConfirmation bool            // Whether the command requires confirmation
	Execute           ExecuteFunc     // Execution function
	ResourceTypes     []string        // Resource types this command applies to (empty = all)
	Shortcut          string          // Keyboard shortcut (e.g., "ctrl+y")
}
