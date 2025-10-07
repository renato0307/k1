package commands

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
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
func YamlCommand(repo k8s.Repository) ExecuteFunc {
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

		// Get GVR for the resource type
		gvr, ok := k8s.GetGVRForResourceType(ctx.ResourceType)
		if !ok {
			return messages.ErrorCmd("Unknown resource type: %s", ctx.ResourceType)
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
func DescribeCommand(repo k8s.Repository) ExecuteFunc {
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

		// Get GVR for the resource type
		gvr, ok := k8s.GetGVRForResourceType(ctx.ResourceType)
		if !ok {
			return messages.ErrorCmd("Unknown resource type: %s", ctx.ResourceType)
		}

		// Get describe output from repository
		describeContent, err := repo.DescribeResource(gvr, namespace, resourceName)
		if err != nil {
			return messages.ErrorCmd("Failed to describe resource: %v", err)
		}

		return func() tea.Msg {
			return types.ShowFullScreenMsg{
				ViewType:     1, // Describe
				ResourceName: displayName,
				Content:      describeContent,
			}
		}
	}
}

// DeleteCommand returns execute function for deleting a resource
func DeleteCommand(provider k8s.KubeconfigProvider) ExecuteFunc {
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
			executor := NewKubectlExecutor(provider.GetKubeconfig(), provider.GetContext())
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
