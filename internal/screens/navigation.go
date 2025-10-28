package screens

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/renato0307/k1/internal/k8s"
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
// navigateToReplicaSetsForDeployment creates handler for Deployment → ReplicaSets
func navigateToReplicaSetsForDeployment() NavigationFunc {
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
				ScreenID: "replicasets",
				FilterContext: &types.FilterContext{
					Field: "owner",
					Value: name,
					Metadata: map[string]string{
						"namespace": namespace,
						"kind":      "Deployment",
					},
				},
			}
		}
	}
}

// navigateToPodsForPVC creates handler for PVC → Pods
func navigateToPodsForPVC() NavigationFunc {
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
					Field: "pvc",
					Value: name,
					Metadata: map[string]string{
						"namespace": namespace,
						"kind":      "PersistentVolumeClaim",
					},
				},
			}
		}
	}
}

// navigateToServicesForIngress creates handler for Ingress → Services
func navigateToServicesForIngress() NavigationFunc {
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
				ScreenID: "services",
				FilterContext: &types.FilterContext{
					Field: "ingress",
					Value: name,
					Metadata: map[string]string{
						"namespace": namespace,
						"kind":      "Ingress",
					},
				},
			}
		}
	}
}

// navigateToPodsForEndpoints creates handler for Endpoints → Pods
func navigateToPodsForEndpoints() NavigationFunc {
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
					Field: "endpoints",
					Value: name,
					Metadata: map[string]string{
						"namespace": namespace,
						"kind":      "Endpoints",
					},
				},
			}
		}
	}
}

// navigateToTargetForHPA creates handler for HPA → Deployment/StatefulSet
func navigateToTargetForHPA() NavigationFunc {
	return func(s *ConfigScreen) tea.Cmd {
		resource := s.GetSelectedResource()
		if resource == nil {
			return nil
		}

		namespace, _ := resource["namespace"].(string)
		reference, _ := resource["reference"].(string)
		if namespace == "" || reference == "" {
			return nil
		}

		// Parse "Deployment/nginx" format
		parts := strings.Split(reference, "/")
		if len(parts) != 2 {
			return nil
		}

		kind := parts[0]
		name := parts[1]

		// Map kind to screen ID
		screenID := strings.ToLower(kind) + "s"

		return func() tea.Msg {
			return types.ScreenSwitchMsg{
				ScreenID: screenID,
				FilterContext: &types.FilterContext{
					Field: "name",
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

// navigateToContextSwitch creates handler for context switching
func navigateToContextSwitch() NavigationFunc {
	return func(s *ConfigScreen) tea.Cmd {
		resource := s.GetSelectedResource()
		if resource == nil {
			return nil
		}

		contextName, ok := resource["name"].(string)
		if !ok || contextName == "" {
			return nil
		}

		return func() tea.Msg {
			return types.ContextSwitchMsg{
				ContextName: contextName,
			}
		}
	}
}

// navigateToCRInstances creates navigation handler for CRD -> CR instances
func navigateToCRInstances() NavigationFunc {
	return func(s *ConfigScreen) tea.Cmd {
		resource := s.GetSelectedResource()
		if resource == nil {
			return nil
		}

		// Extract CRD fields from map
		group, _ := resource["group"].(string)
		version, _ := resource["version"].(string)
		kind, _ := resource["kind"].(string)
		plural, _ := resource["plural"].(string)
		scope, _ := resource["scope"].(string)

		// Build CRD from extracted fields
		crd := k8s.CustomResourceDefinition{
			Group:   group,
			Version: version,
			Kind:    kind,
			Plural:  plural,
			Scope:   scope,
		}

		// Trigger dynamic screen creation
		return func() tea.Msg {
			return types.DynamicScreenCreateMsg{
				CRD: crd,
			}
		}
	}
}
