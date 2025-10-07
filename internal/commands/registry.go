package commands

import (
	"strings"

	"github.com/renato0307/k1/internal/k8s"
	"github.com/sahilm/fuzzy"
)

// Registry holds all available commands and provides filtering
type Registry struct {
	commands []Command
}

// NewRegistry creates a new command registry with default commands
func NewRegistry(repo k8s.Repository) *Registry {
	return &Registry{
		commands: []Command{
			// Navigation commands (: prefix)
			{
				Name:        "pods",
				Description: "Switch to Pods screen",
				Category:    CategoryResource,
				Execute:     PodsCommand(),
			},
			{
				Name:        "deployments",
				Description: "Switch to Deployments screen",
				Category:    CategoryResource,
				Execute:     DeploymentsCommand(),
			},
			{
				Name:        "services",
				Description: "Switch to Services screen",
				Category:    CategoryResource,
				Execute:     ServicesCommand(),
			},
			{
				Name:        "configmaps",
				Description: "Switch to ConfigMaps screen",
				Category:    CategoryResource,
				Execute:     ConfigMapsCommand(),
			},
			{
				Name:        "secrets",
				Description: "Switch to Secrets screen",
				Category:    CategoryResource,
				Execute:     SecretsCommand(),
			},
			{
				Name:        "namespaces",
				Description: "Switch to Namespaces screen",
				Category:    CategoryResource,
				Execute:     NamespacesCommand(),
			},
			{
				Name:        "statefulsets",
				Description: "Switch to StatefulSets screen",
				Category:    CategoryResource,
				Execute:     StatefulSetsCommand(),
			},
			{
				Name:        "daemonsets",
				Description: "Switch to DaemonSets screen",
				Category:    CategoryResource,
				Execute:     DaemonSetsCommand(),
			},
			{
				Name:        "jobs",
				Description: "Switch to Jobs screen",
				Category:    CategoryResource,
				Execute:     JobsCommand(),
			},
			{
				Name:        "cronjobs",
				Description: "Switch to CronJobs screen",
				Category:    CategoryResource,
				Execute:     CronJobsCommand(),
			},
			{
				Name:        "nodes",
				Description: "Switch to Nodes screen",
				Category:    CategoryResource,
				Execute:     NodesCommand(),
			},
			{
				Name:        "ns",
				Description: "Filter by namespace",
				Category:    CategoryResource,
				Execute:     NamespaceFilterCommand(),
			},

			// Resource commands (/ prefix)
			{
				Name:          "yaml",
				Description:   "View resource YAML",
				Category:      CategoryAction,
				ResourceTypes: []k8s.ResourceType{}, // Applies to all resource types
				Shortcut:      "ctrl+y",
				Execute:       YamlCommand(repo),
			},
			{
				Name:          "describe",
				Description:   "View kubectl describe output",
				Category:      CategoryAction,
				ResourceTypes: []k8s.ResourceType{}, // Applies to all resource types
				Shortcut:      "ctrl+d",
				Execute:       DescribeCommand(repo),
			},
			{
				Name:              "delete",
				Description:       "Delete selected resource",
				Category:          CategoryAction,
				ResourceTypes:     []k8s.ResourceType{}, // Applies to all resource types
				Shortcut:          "ctrl+x",
				NeedsConfirmation: true,
				Execute:           DeleteCommand(repo),
			},
			{
				Name:          "logs",
				Description:   "View pod logs (clipboard)",
				Category:      CategoryAction,
				ResourceTypes: []k8s.ResourceType{k8s.ResourceTypePod}, // Only for pods
				Shortcut:      "ctrl+l",
				ArgsType:      &LogsArgs{},
				ArgPattern:    " [container] [tail] [follow]",
				Execute:       LogsCommand(repo),
			},
			{
				Name:          "logs-previous",
				Description:   "View previous pod logs",
				Category:      CategoryAction,
				ResourceTypes: []k8s.ResourceType{k8s.ResourceTypePod}, // Only for pods
				Execute:       LogsPreviousCommand(repo),
			},
			{
				Name:          "port-forward",
				Description:   "Port forward to pod (clipboard)",
				Category:      CategoryAction,
				ResourceTypes: []k8s.ResourceType{k8s.ResourceTypePod}, // Only for pods
				ArgsType:      &PortForwardArgs{},
				ArgPattern:    " <local:remote>",
				Execute:       PortForwardCommand(repo),
			},
			{
				Name:          "shell",
				Description:   "Open shell in pod (clipboard)",
				Category:      CategoryAction,
				ResourceTypes: []k8s.ResourceType{k8s.ResourceTypePod}, // Only for pods
				ArgsType:      &ShellArgs{},
				ArgPattern:    " [container] [shell]",
				Execute:       ShellCommand(repo),
			},
			{
				Name:          "jump-owner",
				Description:   "Jump to owner resource",
				Category:      CategoryAction,
				ResourceTypes: []k8s.ResourceType{k8s.ResourceTypePod}, // Only for pods
				Execute:       JumpOwnerCommand(repo),
			},
			{
				Name:          "show-node",
				Description:   "Show node details",
				Category:      CategoryAction,
				ResourceTypes: []k8s.ResourceType{k8s.ResourceTypePod}, // Only for pods
				Execute:       ShowNodeCommand(repo),
			},
			{
				Name:          "scale",
				Description:   "Scale replicas",
				Category:      CategoryAction,
				ResourceTypes: []k8s.ResourceType{k8s.ResourceTypeDeployment, k8s.ResourceTypeStatefulSet}, // For deployments and statefulsets
				ArgsType:      &ScaleArgs{},
				ArgPattern:    " <replicas>",
				Execute:       ScaleCommand(repo),
			},
			{
				Name:          "cordon",
				Description:   "Cordon node (mark unschedulable)",
				Category:      CategoryAction,
				ResourceTypes: []k8s.ResourceType{k8s.ResourceTypeNode}, // Only for nodes
				Execute:       CordonCommand(repo),
			},
			{
				Name:              "drain",
				Description:       "Drain node (evict all pods)",
				Category:          CategoryAction,
				ResourceTypes:     []k8s.ResourceType{k8s.ResourceTypeNode}, // Only for nodes
				ArgsType:          &DrainArgs{},
				ArgPattern:        " [grace] [force] [ignore-daemonsets]",
				NeedsConfirmation: true,
				Execute:           DrainCommand(repo),
			},
			{
				Name:          "endpoints",
				Description:   "Show service endpoints",
				Category:      CategoryAction,
				ResourceTypes: []k8s.ResourceType{k8s.ResourceTypeService}, // Only for services
				Execute:       EndpointsCommand(repo),
			},
			{
				Name:          "restart",
				Description:   "Restart deployment",
				Category:      CategoryAction,
				ResourceTypes: []k8s.ResourceType{k8s.ResourceTypeDeployment}, // Only for deployments
				Execute:       RestartCommand(repo),
			},

			// LLM commands (/ai prefix) - examples for natural language input
			{
				Name:              "delete failing pods",
				Description:       "Delete all pods in Failed status",
				Category:          CategoryLLMAction,
				NeedsConfirmation: true,
				Execute:           LLMDeleteFailingPodsCommand(repo),
			},
			{
				Name:              "scale nginx to 3",
				Description:       "Scale nginx deployment to 3 replicas",
				Category:          CategoryLLMAction,
				NeedsConfirmation: true,
				Execute:           LLMScaleNginxCommand(repo),
			},
			{
				Name:        "get pod logs",
				Description: "Show logs for the selected pod",
				Category:    CategoryLLMAction,
				Execute:     LLMGetPodLogsCommand(repo),
			},
			{
				Name:              "restart deployment",
				Description:       "Restart the selected deployment",
				Category:          CategoryLLMAction,
				NeedsConfirmation: true,
				Execute:           LLMRestartDeploymentCommand(repo),
			},
			{
				Name:        "show pod events",
				Description: "Show events for the selected pod",
				Category:    CategoryLLMAction,
				Execute:     LLMShowPodEventsCommand(repo),
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
func (r *Registry) FilterByResourceType(commands []Command, resourceType k8s.ResourceType) []Command {
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
