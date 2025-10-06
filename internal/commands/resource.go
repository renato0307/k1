package commands

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	_ "k8s.io/cli-runtime/pkg/printers"
	_ "k8s.io/kubectl/pkg/describe"

	"github.com/renato0307/k1/internal/k8s"
	"github.com/renato0307/k1/internal/types"
)

// YamlCommand returns execute function for viewing resource YAML
func YamlCommand(repo k8s.Repository) ExecuteFunc {
	return func(ctx CommandContext) tea.Cmd {
		resourceName := "unknown"
		namespace := "default"
		if name, ok := ctx.Selected["name"].(string); ok {
			resourceName = name
		}
		if ns, ok := ctx.Selected["namespace"].(string); ok {
			namespace = ns
		}

		// Get GVR for the resource type
		gvr, ok := k8s.GetGVRForResourceType(ctx.ResourceType)
		if !ok {
			return func() tea.Msg {
				return types.ErrorStatusMsg("Unknown resource type: " + ctx.ResourceType)
			}
		}

		// Get YAML from repository using kubectl printer
		yamlContent, err := repo.GetResourceYAML(gvr, namespace, resourceName)
		if err != nil {
			return func() tea.Msg {
				return types.ErrorStatusMsg("Failed to get YAML: " + err.Error())
			}
		}

		return func() tea.Msg {
			return types.ShowFullScreenMsg{
				ViewType:     0, // YAML
				ResourceName: namespace + "/" + resourceName,
				Content:      yamlContent,
			}
		}
	}
}

// DescribeCommand returns execute function for viewing kubectl describe output
func DescribeCommand(repo k8s.Repository) ExecuteFunc {
	return func(ctx CommandContext) tea.Cmd {
		resourceName := "unknown"
		namespace := "default"
		if name, ok := ctx.Selected["name"].(string); ok {
			resourceName = name
		}
		if ns, ok := ctx.Selected["namespace"].(string); ok {
			namespace = ns
		}

		// Get GVR for the resource type
		gvr, ok := k8s.GetGVRForResourceType(ctx.ResourceType)
		if !ok {
			return func() tea.Msg {
				return types.ErrorStatusMsg("Unknown resource type: " + ctx.ResourceType)
			}
		}

		// Get describe output from repository
		describeContent, err := repo.DescribeResource(gvr, namespace, resourceName)
		if err != nil {
			return func() tea.Msg {
				return types.ErrorStatusMsg("Failed to describe resource: " + err.Error())
			}
		}

		return func() tea.Msg {
			return types.ShowFullScreenMsg{
				ViewType:     1, // Describe
				ResourceName: namespace + "/" + resourceName,
				Content:      describeContent,
			}
		}
	}
}

// DeleteCommand returns execute function for deleting a resource
func DeleteCommand(repo k8s.Repository) ExecuteFunc {
	return func(ctx CommandContext) tea.Cmd {
		// Get resource info
		resourceName := "unknown"
		namespace := "default"
		if name, ok := ctx.Selected["name"].(string); ok {
			resourceName = name
		}
		if ns, ok := ctx.Selected["namespace"].(string); ok {
			namespace = ns
		}

		// Build kubectl delete command
		args := []string{
			"delete",
			ctx.ResourceType,
			resourceName,
			"--namespace", namespace,
		}

		// Return a command that executes kubectl asynchronously
		return func() tea.Msg {
			executor := NewKubectlExecutor(repo.GetKubeconfig(), repo.GetContext())
			output, err := executor.Execute(args, ExecuteOptions{})

			if err != nil {
				return types.ErrorStatusMsg(fmt.Sprintf("Delete failed: %v", err))
			}
			msg := fmt.Sprintf("Deleted %s/%s", ctx.ResourceType, resourceName)
			if output != "" {
				msg = strings.TrimSpace(output)
			}
			return types.SuccessMsg(msg)
		}
	}
}

// capitalizeFirst capitalizes the first letter of a string
func capitalizeFirst(s string) string {
	if s == "" {
		return ""
	}
	return strings.ToUpper(s[0:1]) + s[1:]
}
