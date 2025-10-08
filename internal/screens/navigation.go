package screens

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/renato0307/k1/internal/types"
)

// Navigation handler factories - these create NavigationFunc for different resource types

// navigateToPodsForOwner creates a navigation handler for owner-based relationships
// (Deployment, StatefulSet, DaemonSet, Job → Pods)
func navigateToPodsForOwner(kind string) NavigationFunc {
	return func(s *ConfigScreen) tea.Cmd {
		resource := s.GetSelectedResource()
		if resource == nil {
			return nil
		}

		namespace, _ := resource["namespace"].(string)
		name, _ := resource["name"].(string)
		if namespace == "" || name == "" {
			return nil
		}

		return func() tea.Msg {
			return types.ScreenSwitchMsg{
				ScreenID: "pods",
				FilterContext: &types.FilterContext{
					Field: "owner",
					Value: name,
					Metadata: map[string]string{
						"namespace": namespace,
						"kind":      kind,
					},
				},
			}
		}
	}
}

// navigateToJobsForCronJob creates a navigation handler for CronJob → Jobs
func navigateToJobsForCronJob() NavigationFunc {
	return func(s *ConfigScreen) tea.Cmd {
		resource := s.GetSelectedResource()
		if resource == nil {
			return nil
		}

		namespace, _ := resource["namespace"].(string)
		name, _ := resource["name"].(string)
		if namespace == "" || name == "" {
			return nil
		}

		return func() tea.Msg {
			return types.ScreenSwitchMsg{
				ScreenID: "jobs",
				FilterContext: &types.FilterContext{
					Field: "owner",
					Value: name,
					Metadata: map[string]string{
						"namespace": namespace,
						"kind":      "CronJob",
					},
				},
			}
		}
	}
}

// navigateToPodsForNode creates a navigation handler for Node → Pods
func navigateToPodsForNode() NavigationFunc {
	return func(s *ConfigScreen) tea.Cmd {
		resource := s.GetSelectedResource()
		if resource == nil {
			return nil
		}

		name, _ := resource["name"].(string)
		if name == "" {
			return nil
		}

		return func() tea.Msg {
			return types.ScreenSwitchMsg{
				ScreenID: "pods",
				FilterContext: &types.FilterContext{
					Field: "node",
					Value: name,
					Metadata: map[string]string{
						"kind": "Node",
					},
				},
			}
		}
	}
}

// navigateToPodsForService creates a navigation handler for Service → Pods
func navigateToPodsForService() NavigationFunc {
	return func(s *ConfigScreen) tea.Cmd {
		resource := s.GetSelectedResource()
		if resource == nil {
			return nil
		}

		namespace, _ := resource["namespace"].(string)
		name, _ := resource["name"].(string)
		if namespace == "" || name == "" {
			return nil
		}

		return func() tea.Msg {
			return types.ScreenSwitchMsg{
				ScreenID: "pods",
				FilterContext: &types.FilterContext{
					Field: "selector",
					Value: name,
					Metadata: map[string]string{
						"namespace": namespace,
						"kind":      "Service",
					},
				},
			}
		}
	}
}

// navigateToPodsForNamespace creates a navigation handler for Namespace → Pods
func navigateToPodsForNamespace() NavigationFunc {
	return func(s *ConfigScreen) tea.Cmd {
		resource := s.GetSelectedResource()
		if resource == nil {
			return nil
		}

		name, _ := resource["name"].(string)
		if name == "" {
			return nil
		}

		return func() tea.Msg {
			return types.ScreenSwitchMsg{
				ScreenID: "pods",
				FilterContext: &types.FilterContext{
					Field: "namespace",
					Value: name,
					Metadata: map[string]string{
						"kind": "Namespace",
					},
				},
			}
		}
	}
}

// navigateToPodsForVolumeSource creates a navigation handler for ConfigMap/Secret → Pods
func navigateToPodsForVolumeSource(kind string) NavigationFunc {
	return func(s *ConfigScreen) tea.Cmd {
		resource := s.GetSelectedResource()
		if resource == nil {
			return nil
		}

		namespace, _ := resource["namespace"].(string)
		name, _ := resource["name"].(string)
		if namespace == "" || name == "" {
			return nil
		}

		// Map kind to filter field
		field := "configmap"
		if kind == "Secret" {
			field = "secret"
		}

		return func() tea.Msg {
			return types.ScreenSwitchMsg{
				ScreenID: "pods",
				FilterContext: &types.FilterContext{
					Field: field,
					Value: name,
					Metadata: map[string]string{
						"namespace": namespace,
						"kind":      kind,
					},
				},
			}
		}
	}
}
