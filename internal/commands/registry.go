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

// getNextContextShortcut returns the next context shortcut
func getNextContextShortcut() string {
	return "ctrl+n"
}

// getPrevContextShortcut returns the prev context shortcut
func getPrevContextShortcut() string {
	return "ctrl+p"
}

// NewRegistry creates a new command registry with default commands
func NewRegistry(pool *k8s.RepositoryPool) *Registry {
	commands := []Command{
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
				Name:        "replicasets",
				Description: "Switch to ReplicaSets screen",
				Category:    CategoryResource,
				Execute:     NavigationCommand("replicasets"),
			},
			{
				Name:        "persistentvolumeclaims",
				Description: "Switch to PersistentVolumeClaims screen",
				Category:    CategoryResource,
				Execute:     NavigationCommand("persistentvolumeclaims"),
			},
			{
				Name:        "pvcs",
				Description: "Switch to PersistentVolumeClaims screen (alias)",
				Category:    CategoryResource,
				Execute:     NavigationCommand("persistentvolumeclaims"),
			},
			{
				Name:        "ingresses",
				Description: "Switch to Ingresses screen",
				Category:    CategoryResource,
				Execute:     NavigationCommand("ingresses"),
			},
			{
				Name:        "endpoints",
				Description: "Switch to Endpoints screen",
				Category:    CategoryResource,
				Execute:     NavigationCommand("endpoints"),
			},
			{
				Name:        "horizontalpodautoscalers",
				Description: "Switch to HorizontalPodAutoscalers screen",
				Category:    CategoryResource,
				Execute:     NavigationCommand("horizontalpodautoscalers"),
			},
			{
				Name:        "hpas",
				Description: "Switch to HorizontalPodAutoscalers screen (alias)",
				Category:    CategoryResource,
				Execute:     NavigationCommand("horizontalpodautoscalers"),
			},
			{
				Name:        "system-resources",
				Description: "View system resource statistics",
				Category:    CategoryResource,
				Execute:     NavigationCommand("system-resources"),
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
				Execute:       YamlCommand(pool),
			},
			{
				Name:          "describe",
				Description:   "View kubectl describe output",
				Category:      CategoryAction,
				ResourceTypes: []k8s.ResourceType{}, // Applies to all resource types
				Shortcut:      "ctrl+d",
				Execute:       DescribeCommand(pool),
			},
			{
				Name:              "delete",
				Description:       "Delete selected resource",
				Category:          CategoryAction,
				ResourceTypes:     []k8s.ResourceType{}, // Applies to all resource types
				Shortcut:          "ctrl+x",
				NeedsConfirmation: true,
				Execute:           DeleteCommand(pool),
			},
			{
				Name:          "logs",
				Description:   "View pod logs (clipboard)",
				Category:      CategoryAction,
				ResourceTypes: []k8s.ResourceType{k8s.ResourceTypePod}, // Only for pods
				Shortcut:      "ctrl+l",
				ArgsType:      &LogsArgs{},
				ArgPattern:    " [container] [tail] [follow]",
				Execute:       LogsCommand(pool),
			},
			{
				Name:          "logs-previous",
				Description:   "View previous pod logs",
				Category:      CategoryAction,
				ResourceTypes: []k8s.ResourceType{k8s.ResourceTypePod}, // Only for pods
				Execute:       LogsPreviousCommand(pool),
			},
			{
				Name:          "port-forward",
				Description:   "Port forward to pod (clipboard)",
				Category:      CategoryAction,
				ResourceTypes: []k8s.ResourceType{k8s.ResourceTypePod}, // Only for pods
				ArgsType:      &PortForwardArgs{},
				ArgPattern:    " <local:remote>",
				Execute:       PortForwardCommand(pool),
			},
			{
				Name:          "shell",
				Description:   "Open shell in pod (clipboard)",
				Category:      CategoryAction,
				ResourceTypes: []k8s.ResourceType{k8s.ResourceTypePod}, // Only for pods
				ArgsType:      &ShellArgs{},
				ArgPattern:    " [container] [shell]",
				Execute:       ShellCommand(pool),
			},
			{
				Name:          "jump-owner",
				Description:   "Jump to owner resource",
				Category:      CategoryAction,
				ResourceTypes: []k8s.ResourceType{k8s.ResourceTypePod}, // Only for pods
				Execute:       JumpOwnerCommand(pool),
			},
			{
				Name:          "show-node",
				Description:   "Show node details",
				Category:      CategoryAction,
				ResourceTypes: []k8s.ResourceType{k8s.ResourceTypePod}, // Only for pods
				Execute:       ShowNodeCommand(pool),
			},
			{
				Name:          "scale",
				Description:   "Scale replicas",
				Category:      CategoryAction,
				ResourceTypes: []k8s.ResourceType{k8s.ResourceTypeDeployment, k8s.ResourceTypeStatefulSet}, // For deployments and statefulsets
				ArgsType:      &ScaleArgs{},
				ArgPattern:    " <replicas>",
				Execute:       ScaleCommand(pool),
			},
			{
				Name:          "cordon",
				Description:   "Cordon node (mark unschedulable)",
				Category:      CategoryAction,
				ResourceTypes: []k8s.ResourceType{k8s.ResourceTypeNode}, // Only for nodes
				Execute:       CordonCommand(pool),
			},
			{
				Name:              "drain",
				Description:       "Drain node (evict all pods)",
				Category:          CategoryAction,
				ResourceTypes:     []k8s.ResourceType{k8s.ResourceTypeNode}, // Only for nodes
				ArgsType:          &DrainArgs{},
				ArgPattern:        " [grace] [force] [ignore-daemonsets]",
				NeedsConfirmation: true,
				Execute:           DrainCommand(pool),
			},
			{
				Name:          "endpoints",
				Description:   "Show service endpoints",
				Category:      CategoryAction,
				ResourceTypes: []k8s.ResourceType{k8s.ResourceTypeService}, // Only for services
				Execute:       EndpointsCommand(pool),
			},
			{
				Name:          "restart",
				Description:   "Restart deployment",
				Category:      CategoryAction,
				ResourceTypes: []k8s.ResourceType{k8s.ResourceTypeDeployment}, // Only for deployments
				Execute:       RestartCommand(pool),
			},

			// LLM commands (/ai prefix) - examples for natural language input
			{
				Name:              "delete failing pods",
				Description:       "Delete all pods in Failed status",
				Category:          CategoryLLMAction,
				NeedsConfirmation: true,
				Execute:           LLMDeleteFailingPodsCommand(pool),
			},
			{
				Name:              "scale nginx to 3",
				Description:       "Scale nginx deployment to 3 replicas",
				Category:          CategoryLLMAction,
				NeedsConfirmation: true,
				Execute:           LLMScaleNginxCommand(pool),
			},
			{
				Name:        "get pod logs",
				Description: "Show logs for the selected pod",
				Category:    CategoryLLMAction,
				Execute:     LLMGetPodLogsCommand(pool),
			},
			{
				Name:              "restart deployment",
				Description:       "Restart the selected deployment",
				Category:          CategoryLLMAction,
				NeedsConfirmation: true,
				Execute:           LLMRestartDeploymentCommand(pool),
			},
			{
				Name:        "show pod events",
				Description: "Show events for the selected pod",
				Category:    CategoryLLMAction,
				Execute:     LLMShowPodEventsCommand(pool),
			},
	}

	// Context management commands
	commands = append(commands, []Command{
			{
				Name:        "contexts",
				Description: "Switch to Contexts screen",
				Category:    CategoryResource,
				Execute:     ContextsCommand(),
			},
			{
				Name:          "context",
				Description:   "Switch Kubernetes context",
				Category:      CategoryAction,
				ArgsType:      &ContextArgs{},
				ArgPattern:    " <context-name>",
				Execute:       ContextCommand(pool),
			},
			{
				Name:        "next-context",
				Description: "Switch to next context",
				Category:    CategoryResource,
				Execute:     NextContextCommand(pool),
				Shortcut:    getNextContextShortcut(),
			},
			{
				Name:        "prev-context",
				Description: "Switch to previous context",
				Category:    CategoryResource,
				Execute:     PrevContextCommand(pool),
				Shortcut:    getPrevContextShortcut(),
			},
	}...)

	return &Registry{
		commands: commands,
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
