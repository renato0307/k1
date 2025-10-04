package commands

import (
	"strings"

	"github.com/sahilm/fuzzy"
)

// CommandCategory represents the type of command
type CommandCategory int

const (
	CategoryNavigation CommandCategory = iota // : prefix (screens, namespaces)
	CategoryResource                          // / prefix (yaml, describe, delete, logs)
	CategoryLLM                               // /x prefix (natural language commands)
)

// Command represents a command in the palette
type Command struct {
	Name        string          // Short command name (e.g., "pods", "yaml")
	Description string          // Human-readable description
	Category    CommandCategory // Command category
	Execute     func() error    // Execution function (for future use)
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
			},
			{
				Name:        "deployments",
				Description: "Switch to Deployments screen",
				Category:    CategoryNavigation,
			},
			{
				Name:        "services",
				Description: "Switch to Services screen",
				Category:    CategoryNavigation,
			},
			{
				Name:        "namespaces",
				Description: "Filter by namespace",
				Category:    CategoryNavigation,
			},

			// Resource commands (/ prefix)
			{
				Name:        "yaml",
				Description: "View resource YAML",
				Category:    CategoryResource,
			},
			{
				Name:        "describe",
				Description: "View kubectl describe output",
				Category:    CategoryResource,
			},
			{
				Name:        "delete",
				Description: "Delete selected resource",
				Category:    CategoryResource,
			},
			{
				Name:        "logs",
				Description: "View pod logs",
				Category:    CategoryResource,
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

// Get returns a command by name and category, or nil if not found
func (r *Registry) Get(name string, category CommandCategory) *Command {
	for _, cmd := range r.commands {
		if cmd.Category == category && strings.EqualFold(cmd.Name, name) {
			return &cmd
		}
	}
	return nil
}
