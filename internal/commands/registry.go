package commands

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/sahilm/fuzzy"

	"timoneiro/internal/types"
)

// CommandCategory represents the type of command
type CommandCategory int

const (
	CategoryNavigation CommandCategory = iota // : prefix (screens, namespaces)
	CategoryResource                          // / prefix (yaml, describe, delete, logs)
	CategoryLLM                               // /x prefix (natural language commands)
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
}

// Registry holds all available commands and provides filtering
type Registry struct {
	commands []Command
}

// NewRegistry creates a new command registry with default commands
func NewRegistry() *Registry {
	return &Registry{
		commands: []Command{
			// Navigation commands (: prefix)
			{
				Name:        "pods",
				Description: "Switch to Pods screen",
				Category:    CategoryNavigation,
				Execute: func(ctx CommandContext) tea.Cmd {
					return func() tea.Msg {
						return types.ScreenSwitchMsg{ScreenID: "pods"}
					}
				},
			},
			{
				Name:        "deployments",
				Description: "Switch to Deployments screen",
				Category:    CategoryNavigation,
				Execute: func(ctx CommandContext) tea.Cmd {
					return func() tea.Msg {
						return types.ScreenSwitchMsg{ScreenID: "deployments"}
					}
				},
			},
			{
				Name:        "services",
				Description: "Switch to Services screen",
				Category:    CategoryNavigation,
				Execute: func(ctx CommandContext) tea.Cmd {
					return func() tea.Msg {
						return types.ScreenSwitchMsg{ScreenID: "services"}
					}
				},
			},
			{
				Name:        "ns",
				Description: "Filter by namespace",
				Category:    CategoryNavigation,
				Execute: func(ctx CommandContext) tea.Cmd {
					// Phase 3: Return placeholder message (namespace filtering needs state management)
					return func() tea.Msg {
						return types.ErrorMsg{Error: "Namespace filtering - Coming soon"}
					}
				},
			},

			// Resource commands (/ prefix)
			{
				Name:          "yaml",
				Description:   "View resource YAML",
				Category:      CategoryResource,
				ResourceTypes: []string{}, // Applies to all resource types
				Execute: func(ctx CommandContext) tea.Cmd {
					// Phase 3: Show context in placeholder message
					resourceName := "unknown"
					if name, ok := ctx.Selected["name"].(string); ok {
						resourceName = name
					}
					return func() tea.Msg {
						return types.ErrorMsg{Error: "YAML for " + ctx.ResourceType + "/" + resourceName + " - Coming in Phase 4"}
					}
				},
			},
			{
				Name:          "describe",
				Description:   "View kubectl describe output",
				Category:      CategoryResource,
				ResourceTypes: []string{}, // Applies to all resource types
				Execute: func(ctx CommandContext) tea.Cmd {
					// Phase 3: Show context in placeholder message
					resourceName := "unknown"
					if name, ok := ctx.Selected["name"].(string); ok {
						resourceName = name
					}
					return func() tea.Msg {
						return types.ErrorMsg{Error: "Describe " + ctx.ResourceType + "/" + resourceName + " - Coming in Phase 4"}
					}
				},
			},
			{
				Name:              "delete",
				Description:       "Delete selected resource",
				Category:          CategoryResource,
				ResourceTypes:     []string{}, // Applies to all resource types
				NeedsConfirmation: true,
				Execute: func(ctx CommandContext) tea.Cmd {
					// Phase 3: Show what would be deleted
					resourceName := "unknown"
					if name, ok := ctx.Selected["name"].(string); ok {
						resourceName = name
					}
					return func() tea.Msg {
						return types.ErrorMsg{Error: "Deleted " + ctx.ResourceType + "/" + resourceName + " (dummy)"}
					}
				},
			},
			{
				Name:          "logs",
				Description:   "View pod logs",
				Category:      CategoryResource,
				ResourceTypes: []string{"pods"}, // Only for pods
				Execute: func(ctx CommandContext) tea.Cmd {
					// Phase 3: Show which pod's logs
					resourceName := "unknown"
					if name, ok := ctx.Selected["name"].(string); ok {
						resourceName = name
					}
					return func() tea.Msg {
						return types.ErrorMsg{Error: "Logs for pod/" + resourceName + " - Coming in Phase 4"}
					}
				},
			},
			{
				Name:          "scale",
				Description:   "Scale replicas",
				Category:      CategoryResource,
				ResourceTypes: []string{"deployments"}, // Only for deployments
				Execute: func(ctx CommandContext) tea.Cmd {
					// Phase 3: Show which deployment to scale
					resourceName := "unknown"
					if name, ok := ctx.Selected["name"].(string); ok {
						resourceName = name
					}
					return func() tea.Msg {
						return types.ErrorMsg{Error: "Scale deployment/" + resourceName + " - Coming soon"}
					}
				},
			},

			// LLM commands (/x prefix) - examples for natural language input
			{
				Name:              "delete failing pods",
				Description:       "Delete all pods in Failed status",
				Category:          CategoryLLM,
				NeedsConfirmation: true,
				Execute: func(ctx CommandContext) tea.Cmd {
					// TODO: Phase 3 - LLM translation and execution
					return nil
				},
			},
			{
				Name:              "scale nginx to 3",
				Description:       "Scale nginx deployment to 3 replicas",
				Category:          CategoryLLM,
				NeedsConfirmation: true,
				Execute: func(ctx CommandContext) tea.Cmd {
					// TODO: Phase 3 - LLM translation and execution
					return nil
				},
			},
			{
				Name:        "get pod logs",
				Description: "Show logs for the selected pod",
				Category:    CategoryLLM,
				Execute: func(ctx CommandContext) tea.Cmd {
					// TODO: Phase 3 - LLM translation and execution
					return nil
				},
			},
			{
				Name:              "restart deployment",
				Description:       "Restart the selected deployment",
				Category:          CategoryLLM,
				NeedsConfirmation: true,
				Execute: func(ctx CommandContext) tea.Cmd {
					// TODO: Phase 3 - LLM translation and execution
					return nil
				},
			},
			{
				Name:        "show pod events",
				Description: "Show events for the selected pod",
				Category:    CategoryLLM,
				Execute: func(ctx CommandContext) tea.Cmd {
					// TODO: Phase 3 - LLM translation and execution
					return nil
				},
			},
		},
	}
}

// GetByCategory returns all commands in a category
func (r *Registry) GetByCategory(category CommandCategory) []Command {
	result := []Command{}
	for _, cmd := range r.commands {
		if cmd.Category == category {
			result = append(result, cmd)
		}
	}
	return result
}

// Filter returns commands matching the query using fuzzy search
func (r *Registry) Filter(query string, category CommandCategory) []Command {
	// Get commands in category
	candidates := r.GetByCategory(category)

	// If query is empty, return all candidates
	if query == "" {
		return candidates
	}

	// Prepare data for fuzzy search
	names := make([]string, len(candidates))
	for i, cmd := range candidates {
		names[i] = cmd.Name
	}

	// Perform fuzzy search
	matches := fuzzy.Find(query, names)

	// Return matching commands in ranked order
	result := make([]Command, len(matches))
	for i, match := range matches {
		result[i] = candidates[match.Index]
	}

	return result
}

// FilterByResourceType returns commands that apply to the given resource type
// Empty resourceType returns all commands
func (r *Registry) FilterByResourceType(commands []Command, resourceType string) []Command {
	if resourceType == "" {
		return commands
	}

	result := []Command{}
	for _, cmd := range commands {
		// Empty ResourceTypes means applies to all
		if len(cmd.ResourceTypes) == 0 {
			result = append(result, cmd)
			continue
		}
		// Check if resourceType is in the list
		for _, rt := range cmd.ResourceTypes {
			if rt == resourceType {
				result = append(result, cmd)
				break
			}
		}
	}
	return result
}

// Get returns a command by name and category, or nil if not found
func (r *Registry) Get(name string, category CommandCategory) *Command {
	for _, cmd := range r.commands {
		if cmd.Category == category && strings.EqualFold(cmd.Name, name) {
			return &cmd
		}
	}
	return nil
}
