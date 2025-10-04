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
				Category:    CategoryResource,
				Execute: func(ctx CommandContext) tea.Cmd {
					return func() tea.Msg {
						return types.ScreenSwitchMsg{ScreenID: "pods"}
					}
				},
			},
			{
				Name:        "deployments",
				Description: "Switch to Deployments screen",
				Category:    CategoryResource,
				Execute: func(ctx CommandContext) tea.Cmd {
					return func() tea.Msg {
						return types.ScreenSwitchMsg{ScreenID: "deployments"}
					}
				},
			},
			{
				Name:        "services",
				Description: "Switch to Services screen",
				Category:    CategoryResource,
				Execute: func(ctx CommandContext) tea.Cmd {
					return func() tea.Msg {
						return types.ScreenSwitchMsg{ScreenID: "services"}
					}
				},
			},
			{
				Name:        "ns",
				Description: "Filter by namespace",
				Category:    CategoryResource,
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
				Category:      CategoryAction,
				ResourceTypes: []string{}, // Applies to all resource types
				Shortcut:      "ctrl+y",
				Execute: func(ctx CommandContext) tea.Cmd {
					// Phase 4: Show full-screen YAML view with dummy data
					resourceName := "unknown"
					namespace := "default"
					if name, ok := ctx.Selected["name"].(string); ok {
						resourceName = name
					}
					if ns, ok := ctx.Selected["namespace"].(string); ok {
						namespace = ns
					}

					// Generate dummy YAML content
					yamlContent := `apiVersion: v1
kind: ` + capitalizeFirst(ctx.ResourceType) + `
metadata:
  name: ` + resourceName + `
  namespace: ` + namespace + `
  labels:
    app: example
    version: v1.0.0
  annotations:
    description: "This is dummy YAML for demonstration"
spec:
  replicas: 3
  selector:
    matchLabels:
      app: example
  template:
    metadata:
      labels:
        app: example
    spec:
      containers:
      - name: main
        image: nginx:latest
        ports:
        - containerPort: 80
        resources:
          limits:
            cpu: "1"
            memory: "512Mi"
          requests:
            cpu: "100m"
            memory: "128Mi"
status:
  phase: Running
  conditions:
  - type: Ready
    status: "True"
    lastProbeTime: null
    lastTransitionTime: "2025-10-04T10:00:00Z"`

					return func() tea.Msg {
						return types.ShowFullScreenMsg{
							ViewType:     0, // YAML
							ResourceName: namespace + "/" + resourceName,
							Content:      yamlContent,
						}
					}
				},
			},
			{
				Name:          "describe",
				Description:   "View kubectl describe output",
				Category:      CategoryAction,
				ResourceTypes: []string{}, // Applies to all resource types
				Shortcut:      "ctrl+d",
				Execute: func(ctx CommandContext) tea.Cmd {
					// Phase 4: Show full-screen describe view with dummy data
					resourceName := "unknown"
					namespace := "default"
					if name, ok := ctx.Selected["name"].(string); ok {
						resourceName = name
					}
					if ns, ok := ctx.Selected["namespace"].(string); ok {
						namespace = ns
					}

					// Generate dummy describe output
					describeContent := `Name:         ` + resourceName + `
Namespace:    ` + namespace + `
Labels:       app=example
              version=v1.0.0
Annotations:  description: This is dummy describe output for demonstration
Status:       Running
IP:           10.244.0.5
Node:         node-1/192.168.1.10
Start Time:   Thu, 04 Oct 2025 10:00:00 +0000

Conditions:
  Type              Status
  Initialized       True
  Ready             True
  ContainersReady   True
  PodScheduled      True

Containers:
  main:
    Container ID:   docker://abc123def456
    Image:          nginx:latest
    Image ID:       docker-pullable://nginx@sha256:123456
    Port:           80/TCP
    Host Port:      0/TCP
    State:          Running
      Started:      Thu, 04 Oct 2025 10:00:30 +0000
    Ready:          True
    Restart Count:  0
    Limits:
      cpu:     1
      memory:  512Mi
    Requests:
      cpu:        100m
      memory:     128Mi
    Environment:  <none>
    Mounts:
      /var/run/secrets/kubernetes.io/serviceaccount from default-token-xyz (ro)

Events:
  Type    Reason     Age   From               Message
  ----    ------     ----  ----               -------
  Normal  Scheduled  5m    default-scheduler  Successfully assigned ` + namespace + `/` + resourceName + ` to node-1
  Normal  Pulling    5m    kubelet            Pulling image "nginx:latest"
  Normal  Pulled     4m    kubelet            Successfully pulled image "nginx:latest"
  Normal  Created    4m    kubelet            Created container main
  Normal  Started    4m    kubelet            Started container main`

					return func() tea.Msg {
						return types.ShowFullScreenMsg{
							ViewType:     1, // Describe
							ResourceName: namespace + "/" + resourceName,
							Content:      describeContent,
						}
					}
				},
			},
			{
				Name:              "delete",
				Description:       "Delete selected resource",
				Category:          CategoryAction,
				ResourceTypes:     []string{}, // Applies to all resource types
				Shortcut:          "ctrl+x",
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
				Category:      CategoryAction,
				ResourceTypes: []string{"pods"}, // Only for pods
				Shortcut:      "ctrl+l",
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
				Name:          "logs-previous",
				Description:   "View previous pod logs",
				Category:      CategoryAction,
				ResourceTypes: []string{"pods"}, // Only for pods
				Execute: func(ctx CommandContext) tea.Cmd {
					resourceName := "unknown"
					if name, ok := ctx.Selected["name"].(string); ok {
						resourceName = name
					}
					return func() tea.Msg {
						return types.ErrorMsg{Error: "Previous logs for pod/" + resourceName + " - Coming soon"}
					}
				},
			},
			{
				Name:          "port-forward",
				Description:   "Port forward to pod",
				Category:      CategoryAction,
				ResourceTypes: []string{"pods"}, // Only for pods
				Execute: func(ctx CommandContext) tea.Cmd {
					resourceName := "unknown"
					if name, ok := ctx.Selected["name"].(string); ok {
						resourceName = name
					}
					return func() tea.Msg {
						return types.ErrorMsg{Error: "Port forward to pod/" + resourceName + " - Coming soon"}
					}
				},
			},
			{
				Name:          "shell",
				Description:   "Open shell in pod",
				Category:      CategoryAction,
				ResourceTypes: []string{"pods"}, // Only for pods
				Execute: func(ctx CommandContext) tea.Cmd {
					resourceName := "unknown"
					if name, ok := ctx.Selected["name"].(string); ok {
						resourceName = name
					}
					return func() tea.Msg {
						return types.ErrorMsg{Error: "Shell for pod/" + resourceName + " - Coming soon"}
					}
				},
			},
			{
				Name:          "jump-owner",
				Description:   "Jump to owner resource",
				Category:      CategoryAction,
				ResourceTypes: []string{"pods"}, // Only for pods
				Execute: func(ctx CommandContext) tea.Cmd {
					resourceName := "unknown"
					if name, ok := ctx.Selected["name"].(string); ok {
						resourceName = name
					}
					return func() tea.Msg {
						return types.ErrorMsg{Error: "Jump to owner of pod/" + resourceName + " - Coming soon"}
					}
				},
			},
			{
				Name:          "show-node",
				Description:   "Show node details",
				Category:      CategoryAction,
				ResourceTypes: []string{"pods"}, // Only for pods
				Execute: func(ctx CommandContext) tea.Cmd {
					resourceName := "unknown"
					nodeName := "unknown"
					if name, ok := ctx.Selected["name"].(string); ok {
						resourceName = name
					}
					if node, ok := ctx.Selected["node"].(string); ok {
						nodeName = node
					}
					return func() tea.Msg {
						return types.ErrorMsg{Error: "Show node " + nodeName + " for pod/" + resourceName + " - Coming soon"}
					}
				},
			},
			{
				Name:          "scale",
				Description:   "Scale replicas",
				Category:      CategoryAction,
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

			// LLM commands (/ai prefix) - examples for natural language input
			{
				Name:              "delete failing pods",
				Description:       "Delete all pods in Failed status",
				Category:          CategoryLLMAction,
				NeedsConfirmation: true,
				Execute: func(ctx CommandContext) tea.Cmd {
					// TODO: Phase 3 - LLM translation and execution
					return nil
				},
			},
			{
				Name:              "scale nginx to 3",
				Description:       "Scale nginx deployment to 3 replicas",
				Category:          CategoryLLMAction,
				NeedsConfirmation: true,
				Execute: func(ctx CommandContext) tea.Cmd {
					// TODO: Phase 3 - LLM translation and execution
					return nil
				},
			},
			{
				Name:        "get pod logs",
				Description: "Show logs for the selected pod",
				Category:    CategoryLLMAction,
				Execute: func(ctx CommandContext) tea.Cmd {
					// TODO: Phase 3 - LLM translation and execution
					return nil
				},
			},
			{
				Name:              "restart deployment",
				Description:       "Restart the selected deployment",
				Category:          CategoryLLMAction,
				NeedsConfirmation: true,
				Execute: func(ctx CommandContext) tea.Cmd {
					// TODO: Phase 3 - LLM translation and execution
					return nil
				},
			},
			{
				Name:        "show pod events",
				Description: "Show events for the selected pod",
				Category:    CategoryLLMAction,
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

// capitalizeFirst capitalizes the first letter of a string
func capitalizeFirst(s string) string {
	if s == "" {
		return ""
	}
	return strings.ToUpper(s[0:1]) + s[1:]
}
