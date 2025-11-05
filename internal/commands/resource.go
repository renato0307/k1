package commands

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"k8s.io/apimachinery/pkg/runtime/schema"
	_ "k8s.io/cli-runtime/pkg/printers"
	_ "k8s.io/kubectl/pkg/describe"

	"github.com/renato0307/k1/internal/k8s"
	"github.com/renato0307/k1/internal/messages"
	"github.com/renato0307/k1/internal/types"
)

// isClusterScoped returns true if the resource type is cluster-scoped (not namespaced)
func isClusterScoped(resourceType k8s.ResourceType) bool {
	clusterScopedResources := map[k8s.ResourceType]bool{
		k8s.ResourceTypeNode:      true,
		k8s.ResourceTypeNamespace: true,
	}
	return clusterScopedResources[resourceType]
}

// YamlCommand returns execute function for viewing resource YAML
func YamlCommand(pool *k8s.RepositoryPool) ExecuteFunc {
	return func(ctx CommandContext) tea.Cmd {
		resourceName := "unknown"
		namespace := ""
		displayName := ""

		if name, ok := ctx.Selected["name"].(string); ok {
			resourceName = name
		}

		// Only set namespace for namespaced resources
		if !isClusterScoped(ctx.ResourceType) {
			namespace = "default"
			if ns, ok := ctx.Selected["namespace"].(string); ok {
				namespace = ns
			}
			displayName = namespace + "/" + resourceName
		} else {
			displayName = resourceName
		}

		// Get GVR - either from selected resource (dynamic CRD instances) or config (static resources)
		var gvr schema.GroupVersionResource

		// Check if this is a dynamic CRD instance (has GVR metadata in selected resource)
		if group, hasGroup := ctx.Selected["__gvr_group"].(string); hasGroup {
			version, _ := ctx.Selected["__gvr_version"].(string)
			resource, _ := ctx.Selected["__gvr_resource"].(string)
			gvr = schema.GroupVersionResource{
				Group:    group,
				Version:  version,
				Resource: resource,
			}
		} else {
			// Static resource - look up via config
			config, ok := k8s.GetResourceConfig(ctx.ResourceType)
			if !ok {
				return messages.ErrorCmd("Unknown resource type: %s", ctx.ResourceType)
			}
			gvr = config.GVR
		}

		// Get active repository at execution time
		repo := pool.GetActiveRepository()
		if repo == nil {
			return messages.ErrorCmd("No active repository")
		}

		// Get YAML from repository using kubectl printer
		yamlContent, err := repo.GetResourceYAML(gvr, namespace, resourceName)
		if err != nil {
			return messages.ErrorCmd("Failed to get YAML: %v", err)
		}

		return func() tea.Msg {
			return types.ShowFullScreenMsg{
				ViewType:     0, // YAML
				ResourceName: displayName,
				Content:      yamlContent,
			}
		}
	}
}

// DescribeCommand returns execute function for viewing kubectl describe output
func DescribeCommand(pool *k8s.RepositoryPool) ExecuteFunc {
	return func(ctx CommandContext) tea.Cmd {
		start := time.Now() // Track start time for history
		resourceName := "unknown"
		namespace := ""
		displayName := ""

		if name, ok := ctx.Selected["name"].(string); ok {
			resourceName = name
		}

		// Only set namespace for namespaced resources
		if !isClusterScoped(ctx.ResourceType) {
			namespace = "default"
			if ns, ok := ctx.Selected["namespace"].(string); ok {
				namespace = ns
			}
			displayName = namespace + "/" + resourceName
		} else {
			displayName = resourceName
		}

		// Get GVR - either from selected resource (dynamic CRD instances) or config (static resources)
		var gvr schema.GroupVersionResource

		// Check if this is a dynamic CRD instance (has GVR metadata in selected resource)
		if group, hasGroup := ctx.Selected["__gvr_group"].(string); hasGroup {
			version, _ := ctx.Selected["__gvr_version"].(string)
			resource, _ := ctx.Selected["__gvr_resource"].(string)
			gvr = schema.GroupVersionResource{
				Group:    group,
				Version:  version,
				Resource: resource,
			}
		} else {
			// Static resource - look up via config
			config, ok := k8s.GetResourceConfig(ctx.ResourceType)
			if !ok {
				return messages.ErrorCmd("Unknown resource type: %s", ctx.ResourceType)
			}
			gvr = config.GVR
		}

		// Get active repository at execution time
		repo := pool.GetActiveRepository()
		if repo == nil {
			return messages.ErrorCmd("No active repository")
		}

		// Get describe output from repository
		describeContent, err := repo.DescribeResource(gvr, namespace, resourceName)

		// Build history metadata
		metadata := &types.CommandMetadata{
			Command:        ctx.OriginalCommand,
			KubectlCommand: "", // Describe uses repo.DescribeResource, not direct kubectl
			Context:        repo.GetContext(),
			ResourceType:   ctx.ResourceType,
			ResourceName:   resourceName,
			Namespace:      namespace,
			Duration:       time.Since(start),
			Timestamp:      time.Now(),
		}

		if err != nil {
			return messages.WithHistory(
				messages.ErrorCmd("Failed to describe resource: %v", err),
				metadata,
			)
		}

		// Success: Show full screen (no status message - user sees the result directly)
		// Track in history silently for output screen
		return tea.Batch(
			func() tea.Msg {
				return types.StatusMsg{
					Type:            types.MessageTypeSuccess,
					Message:         describeContent, // Full output for history
					TrackInHistory:  true,
					HistoryMetadata: metadata,
					Silent:          true, // Don't display to user
				}
			},
			func() tea.Msg {
				return types.ShowFullScreenMsg{
					ViewType:     1, // Describe
					ResourceName: displayName,
					Content:      describeContent,
				}
			},
		)
	}
}

// DeleteCommand returns execute function for deleting a resource
func DeleteCommand(pool *k8s.RepositoryPool) ExecuteFunc {
	return func(ctx CommandContext) tea.Cmd {
		// Get resource info
		resourceName := "unknown"
		namespace := ""
		if name, ok := ctx.Selected["name"].(string); ok {
			resourceName = name
		}

		// Only set namespace for namespaced resources
		if !isClusterScoped(ctx.ResourceType) {
			namespace = "default"
			if ns, ok := ctx.Selected["namespace"].(string); ok {
				namespace = ns
			}
		}

		// Build kubectl delete command
		args := []string{
			"delete",
			string(ctx.ResourceType),
			resourceName,
		}

		// Add namespace flag only for namespaced resources
		if namespace != "" {
			args = append(args, "--namespace", namespace)
		}

		// Return a command that executes kubectl asynchronously
		return func() tea.Msg {
			repo := pool.GetActiveRepository()
			if repo == nil {
				return messages.ErrorCmd("No active repository")()
			}
			executor := NewKubectlExecutor(repo.GetKubeconfig(), repo.GetContext())
			output, err := executor.Execute(args, ExecuteOptions{})

			if err != nil {
				return messages.ErrorCmd("Delete failed: %v", err)()
			}
			msg := fmt.Sprintf("Deleted %s/%s", ctx.ResourceType, resourceName)
			if output != "" {
				msg = strings.TrimSpace(output)
			}
			return messages.SuccessCmd("%s", msg)()
		}
	}
}

// EditCommand returns execute function for editing a resource (clipboard)
func EditCommand(pool *k8s.RepositoryPool) ExecuteFunc {
	return func(ctx CommandContext) tea.Cmd {
		// Get resource info
		resourceName := "unknown"
		namespace := ""
		if name, ok := ctx.Selected["name"].(string); ok {
			resourceName = name
		}

		// Only set namespace for namespaced resources
		if !isClusterScoped(ctx.ResourceType) {
			namespace = "default"
			if ns, ok := ctx.Selected["namespace"].(string); ok {
				namespace = ns
			}
		}

		// Validate we have active repository first
		repo := pool.GetActiveRepository()
		if repo == nil {
			return messages.ErrorCmd("No active repository")
		}

		// Build kubectl edit command
		var kubectlCmd strings.Builder
		kubectlCmd.WriteString("kubectl edit ")
		kubectlCmd.WriteString(string(ctx.ResourceType))
		kubectlCmd.WriteString(" ")
		kubectlCmd.WriteString(resourceName)

		// Add namespace flag only for namespaced resources
		if namespace != "" {
			kubectlCmd.WriteString(" --namespace ")
			kubectlCmd.WriteString(namespace)
		}

		// Add kubeconfig/context if set
		if repo.GetKubeconfig() != "" {
			kubectlCmd.WriteString(" --kubeconfig ")
			kubectlCmd.WriteString(repo.GetKubeconfig())
		}
		if repo.GetContext() != "" {
			kubectlCmd.WriteString(" --context ")
			kubectlCmd.WriteString(repo.GetContext())
		}

		command := kubectlCmd.String()

		return func() tea.Msg {
			msg, err := CopyToClipboard(command)
			if err != nil {
				// Clipboard failed (e.g., SSH session), show command anyway
				return messages.InfoCmd("Clipboard unavailable. Command: %s", command)()
			}
			return messages.InfoCmd("%s", msg)()
		}
	}
}
