package screens

import (
	"fmt"
	"strings"

	"github.com/renato0307/k1/internal/types"
)

// NavigationHelpers provides utilities for building contextual navigation

// formatLabelSelector converts a map of labels to a comma-separated label selector string
// Example: {"app": "nginx", "tier": "frontend"} -> "app=nginx,tier=frontend"
func formatLabelSelector(selector map[string]string) string {
	if len(selector) == 0 {
		return ""
	}

	parts := make([]string, 0, len(selector))
	for key, value := range selector {
		parts = append(parts, fmt.Sprintf("%s=%s", key, value))
	}
	return strings.Join(parts, ",")
}

// buildDeploymentToPods creates a NavigateMsg for Deployment → Pods navigation
func buildDeploymentToPods(screen *ConfigScreen) *types.NavigateMsg {
	selected := screen.GetSelectedResource()
	if selected == nil {
		return nil
	}

	name, _ := selected["name"].(string)
	namespace, _ := selected["namespace"].(string)

	// Extract real label selector from Deployment.Selector field
	// Note: GetSelectedResource() lowercases field names
	selectorMap, ok := selected["selector"].(map[string]string)
	if !ok {
		// Selector field not found or wrong type
		return nil
	}
	labelSelector := formatLabelSelector(selectorMap)

	if labelSelector == "" {
		// No selector found - skip navigation
		return nil
	}

	return &types.NavigateMsg{
		ScreenID: "pods",
		Context: types.NavigationContext{
			ParentScreen:   "deployments",
			ParentResource: name,
			FilterLabel:    fmt.Sprintf("Deployment: %s", name),
			FilterValue:    fmt.Sprintf("namespace=%s,selector=%s", namespace, labelSelector),
		},
	}
}

// buildStatefulSetToPods creates a NavigateMsg for StatefulSet → Pods navigation
func buildStatefulSetToPods(screen *ConfigScreen) *types.NavigateMsg {
	selected := screen.GetSelectedResource()
	if selected == nil {
		return nil
	}

	name, _ := selected["name"].(string)
	namespace, _ := selected["namespace"].(string)

	// Extract real label selector from StatefulSet.Selector field
	selectorMap, _ := selected["selector"].(map[string]string)
	labelSelector := formatLabelSelector(selectorMap)

	if labelSelector == "" {
		return nil
	}

	return &types.NavigateMsg{
		ScreenID: "pods",
		Context: types.NavigationContext{
			ParentScreen:   "statefulsets",
			ParentResource: name,
			FilterLabel:    fmt.Sprintf("StatefulSet: %s", name),
			FilterValue:    fmt.Sprintf("namespace=%s,selector=%s", namespace, labelSelector),
		},
	}
}

// buildDaemonSetToPods creates a NavigateMsg for DaemonSet → Pods navigation
func buildDaemonSetToPods(screen *ConfigScreen) *types.NavigateMsg {
	selected := screen.GetSelectedResource()
	if selected == nil {
		return nil
	}

	name, _ := selected["name"].(string)
	namespace, _ := selected["namespace"].(string)

	// Extract real label selector from DaemonSet.Selector field
	selectorMap, _ := selected["selector"].(map[string]string)
	labelSelector := formatLabelSelector(selectorMap)

	if labelSelector == "" {
		return nil
	}

	return &types.NavigateMsg{
		ScreenID: "pods",
		Context: types.NavigationContext{
			ParentScreen:   "daemonsets",
			ParentResource: name,
			FilterLabel:    fmt.Sprintf("DaemonSet: %s", name),
			FilterValue:    fmt.Sprintf("namespace=%s,selector=%s", namespace, labelSelector),
		},
	}
}

// buildServiceToPods creates a NavigateMsg for Service → Pods navigation
func buildServiceToPods(screen *ConfigScreen) *types.NavigateMsg {
	selected := screen.GetSelectedResource()
	if selected == nil {
		return nil
	}

	name, _ := selected["name"].(string)
	namespace, _ := selected["namespace"].(string)

	// Extract real label selector from Service.Selector field
	selectorMap, _ := selected["selector"].(map[string]string)
	labelSelector := formatLabelSelector(selectorMap)

	if labelSelector == "" {
		return nil
	}

	return &types.NavigateMsg{
		ScreenID: "pods",
		Context: types.NavigationContext{
			ParentScreen:   "services",
			ParentResource: name,
			FilterLabel:    fmt.Sprintf("Service: %s", name),
			FilterValue:    fmt.Sprintf("namespace=%s,selector=%s", namespace, labelSelector),
		},
	}
}

// buildNodeToPods creates a NavigateMsg for Node → Pods navigation
func buildNodeToPods(screen *ConfigScreen) *types.NavigateMsg {
	selected := screen.GetSelectedResource()
	if selected == nil {
		return nil
	}

	name, _ := selected["name"].(string)

	return &types.NavigateMsg{
		ScreenID: "pods",
		Context: types.NavigationContext{
			ParentScreen:   "nodes",
			ParentResource: name,
			FilterLabel:    fmt.Sprintf("Node: %s", name),
			FilterValue:    fmt.Sprintf("nodeName=%s", name),
		},
	}
}

// buildJobToPods creates a NavigateMsg for Job → Pods navigation
func buildJobToPods(screen *ConfigScreen) *types.NavigateMsg {
	selected := screen.GetSelectedResource()
	if selected == nil {
		return nil
	}

	name, _ := selected["name"].(string)
	namespace, _ := selected["namespace"].(string)

	// Jobs create pods with controller-uid label or job-name label
	labelSelector := fmt.Sprintf("job-name=%s", name)

	return &types.NavigateMsg{
		ScreenID: "pods",
		Context: types.NavigationContext{
			ParentScreen:   "jobs",
			ParentResource: name,
			FilterLabel:    fmt.Sprintf("Job: %s", name),
			FilterValue:    fmt.Sprintf("namespace=%s,selector=%s", namespace, labelSelector),
		},
	}
}

