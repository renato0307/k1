package screens

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/renato0307/k1/internal/k8s"
)

// GetPodsScreenConfig returns the config for the Pods screen (Level 2 - with periodic refresh)
func GetPodsScreenConfig() ScreenConfig {
	return ScreenConfig{
		ID:           "pods",
		Title:        "Pods",
		ResourceType: k8s.ResourceTypePod,
		Columns: []ColumnConfig{
			{Field: "Namespace", Title: "Namespace", Width: 40},
			{Field: "Name", Title: "Name", Width: 0}, // Dynamic width
			{Field: "Ready", Title: "Ready", Width: 8},
			{Field: "Status", Title: "Status", Width: 15},
			{Field: "Restarts", Title: "Restarts", Width: 10},
			{Field: "Age", Title: "Age", Width: 10, Format: FormatDuration},
			{Field: "Node", Title: "Node", Width: 30},
			{Field: "IP", Title: "IP", Width: 16},
		},
		SearchFields: []string{"Namespace", "Name", "Status", "Node", "IP"},
		Operations: []OperationConfig{
			{ID: "logs", Name: "View Logs", Description: "View logs for selected pod", Shortcut: "l"},
			{ID: "describe", Name: "Describe", Description: "Describe selected pod", Shortcut: "d"},
			{ID: "delete", Name: "Delete", Description: "Delete selected pod", Shortcut: "x"},
		},
		EnablePeriodicRefresh: true,
		RefreshInterval:       RefreshInterval,
		TrackSelection:        true,
		CustomUpdate:          getPeriodicRefreshUpdate(),
	}
}

// getPeriodicRefreshUpdate returns a shared CustomUpdate handler for periodic refresh
func getPeriodicRefreshUpdate() func(s *ConfigScreen, msg tea.Msg) (tea.Model, tea.Cmd) {
	return func(s *ConfigScreen, msg tea.Msg) (tea.Model, tea.Cmd) {
		switch msg.(type) {
		case tickMsg:
			// Refresh and schedule next tick using screen's configured interval
			nextTick := tea.Tick(s.config.RefreshInterval, func(t time.Time) tea.Msg {
				return tickMsg(t)
			})
			return s, tea.Batch(s.Refresh(), nextTick)
		default:
			return s.DefaultUpdate(msg)
		}
	}
}

// GetDeploymentsScreenConfig returns the config for the Deployments screen (Level 1 - pure config)
func GetDeploymentsScreenConfig() ScreenConfig {
	return ScreenConfig{
		ID:           "deployments",
		Title:        "Deployments",
		ResourceType: k8s.ResourceTypeDeployment,
		Columns: []ColumnConfig{
			{Field: "Namespace", Title: "Namespace", Width: 40},
			{Field: "Name", Title: "Name", Width: 0}, // Dynamic width
			{Field: "Ready", Title: "Ready", Width: 10},
			{Field: "UpToDate", Title: "Up-to-date", Width: 12},
			{Field: "Available", Title: "Available", Width: 12},
			{Field: "Age", Title: "Age", Width: 10, Format: FormatDuration},
		},
		SearchFields: []string{"Namespace", "Name"},
		Operations: []OperationConfig{
			{ID: "scale", Name: "Scale", Description: "Scale selected deployment", Shortcut: "s"},
			{ID: "restart", Name: "Restart", Description: "Restart selected deployment", Shortcut: "r"},
			{ID: "describe", Name: "Describe", Description: "Describe selected deployment", Shortcut: "d"},
		},
		NavigationHandler:     navigateToPodsForOwner("Deployment"),
		EnablePeriodicRefresh: true,
		RefreshInterval:       RefreshInterval,
		TrackSelection:        true,
		CustomUpdate:          getPeriodicRefreshUpdate(),
	}
}

// GetServicesScreenConfig returns the config for the Services screen (Level 1 - pure config)
func GetServicesScreenConfig() ScreenConfig {
	return ScreenConfig{
		ID:           "services",
		Title:        "Services",
		ResourceType: k8s.ResourceTypeService,
		Columns: []ColumnConfig{
			{Field: "Namespace", Title: "Namespace", Width: 40},
			{Field: "Name", Title: "Name", Width: 0}, // Dynamic width
			{Field: "Type", Title: "Type", Width: 15},
			{Field: "ClusterIP", Title: "Cluster-IP", Width: 15},
			{Field: "ExternalIP", Title: "External-IP", Width: 15},
			{Field: "Ports", Title: "Ports", Width: 20},
			{Field: "Age", Title: "Age", Width: 10, Format: FormatDuration},
		},
		SearchFields: []string{"Namespace", "Name", "Type"},
		Operations: []OperationConfig{
			{ID: "describe", Name: "Describe", Description: "Describe selected service", Shortcut: "d"},
			{ID: "endpoints", Name: "Show Endpoints", Description: "Show service endpoints", Shortcut: "e"},
			{ID: "delete", Name: "Delete", Description: "Delete selected service", Shortcut: "x"},
		},
		NavigationHandler:     navigateToPodsForService(),
		EnablePeriodicRefresh: true,
		RefreshInterval:       RefreshInterval,
		TrackSelection:        true,
		CustomUpdate:          getPeriodicRefreshUpdate(),
	}
}

// GetConfigMapsScreenConfig returns the config for the ConfigMaps screen (Level 1)
func GetConfigMapsScreenConfig() ScreenConfig {
	return ScreenConfig{
		ID:           "configmaps",
		Title:        "ConfigMaps",
		ResourceType: k8s.ResourceTypeConfigMap,
		Columns: []ColumnConfig{
			{Field: "Namespace", Title: "Namespace", Width: 40},
			{Field: "Name", Title: "Name", Width: 0},
			{Field: "Data", Title: "Data", Width: 10},
			{Field: "Age", Title: "Age", Width: 10, Format: FormatDuration},
		},
		SearchFields: []string{"Namespace", "Name"},
		Operations: []OperationConfig{
			{ID: "describe", Name: "Describe", Description: "Describe selected configmap", Shortcut: "d"},
			{ID: "delete", Name: "Delete", Description: "Delete selected configmap", Shortcut: "x"},
		},
		NavigationHandler:     navigateToPodsForVolumeSource("ConfigMap"),
		EnablePeriodicRefresh: true,
		RefreshInterval:       RefreshInterval,
		TrackSelection:        true,
		CustomUpdate:          getPeriodicRefreshUpdate(),
	}
}

// GetSecretsScreenConfig returns the config for the Secrets screen (Level 1)
func GetSecretsScreenConfig() ScreenConfig {
	return ScreenConfig{
		ID:           "secrets",
		Title:        "Secrets",
		ResourceType: k8s.ResourceTypeSecret,
		Columns: []ColumnConfig{
			{Field: "Namespace", Title: "Namespace", Width: 40},
			{Field: "Name", Title: "Name", Width: 0},
			{Field: "Type", Title: "Type", Width: 30},
			{Field: "Data", Title: "Data", Width: 10},
			{Field: "Age", Title: "Age", Width: 10, Format: FormatDuration},
		},
		SearchFields: []string{"Namespace", "Name", "Type"},
		Operations: []OperationConfig{
			{ID: "describe", Name: "Describe", Description: "Describe selected secret", Shortcut: "d"},
			{ID: "delete", Name: "Delete", Description: "Delete selected secret", Shortcut: "x"},
		},
		NavigationHandler:     navigateToPodsForVolumeSource("Secret"),
		EnablePeriodicRefresh: true,
		RefreshInterval:       RefreshInterval,
		TrackSelection:        true,
		CustomUpdate:          getPeriodicRefreshUpdate(),
	}
}

// GetNamespacesScreenConfig returns the config for the Namespaces screen (Level 1)
func GetNamespacesScreenConfig() ScreenConfig {
	return ScreenConfig{
		ID:           "namespaces",
		Title:        "Namespaces",
		ResourceType: k8s.ResourceTypeNamespace,
		Columns: []ColumnConfig{
			{Field: "Name", Title: "Name", Width: 0},
			{Field: "Status", Title: "Status", Width: 15},
			{Field: "Age", Title: "Age", Width: 10, Format: FormatDuration},
		},
		SearchFields: []string{"Name", "Status"},
		Operations: []OperationConfig{
			{ID: "describe", Name: "Describe", Description: "Describe selected namespace", Shortcut: "d"},
			{ID: "delete", Name: "Delete", Description: "Delete selected namespace", Shortcut: "x"},
		},
		NavigationHandler:     navigateToPodsForNamespace(),
		EnablePeriodicRefresh: true,
		RefreshInterval:       RefreshInterval,
		TrackSelection:        true,
		CustomUpdate:          getPeriodicRefreshUpdate(),
	}
}

// GetStatefulSetsScreenConfig returns the config for the StatefulSets screen (Level 1)
func GetStatefulSetsScreenConfig() ScreenConfig {
	return ScreenConfig{
		ID:           "statefulsets",
		Title:        "StatefulSets",
		ResourceType: k8s.ResourceTypeStatefulSet,
		Columns: []ColumnConfig{
			{Field: "Namespace", Title: "Namespace", Width: 40},
			{Field: "Name", Title: "Name", Width: 0},
			{Field: "Ready", Title: "Ready", Width: 10},
			{Field: "Age", Title: "Age", Width: 10, Format: FormatDuration},
		},
		SearchFields: []string{"Namespace", "Name"},
		Operations: []OperationConfig{
			{ID: "scale", Name: "Scale", Description: "Scale selected statefulset", Shortcut: "s"},
			{ID: "describe", Name: "Describe", Description: "Describe selected statefulset", Shortcut: "d"},
			{ID: "delete", Name: "Delete", Description: "Delete selected statefulset", Shortcut: "x"},
		},
		NavigationHandler:     navigateToPodsForOwner("StatefulSet"),
		EnablePeriodicRefresh: true,
		RefreshInterval:       RefreshInterval,
		TrackSelection:        true,
		CustomUpdate:          getPeriodicRefreshUpdate(),
	}
}

// GetDaemonSetsScreenConfig returns the config for the DaemonSets screen (Level 1)
func GetDaemonSetsScreenConfig() ScreenConfig {
	return ScreenConfig{
		ID:           "daemonsets",
		Title:        "DaemonSets",
		ResourceType: k8s.ResourceTypeDaemonSet,
		Columns: []ColumnConfig{
			{Field: "Namespace", Title: "Namespace", Width: 40},
			{Field: "Name", Title: "Name", Width: 0},
			{Field: "Desired", Title: "Desired", Width: 10},
			{Field: "Current", Title: "Current", Width: 10},
			{Field: "Ready", Title: "Ready", Width: 10},
			{Field: "UpToDate", Title: "Up-to-date", Width: 12},
			{Field: "Available", Title: "Available", Width: 12},
			{Field: "Age", Title: "Age", Width: 10, Format: FormatDuration},
		},
		SearchFields: []string{"Namespace", "Name"},
		Operations: []OperationConfig{
			{ID: "describe", Name: "Describe", Description: "Describe selected daemonset", Shortcut: "d"},
			{ID: "delete", Name: "Delete", Description: "Delete selected daemonset", Shortcut: "x"},
		},
		NavigationHandler:     navigateToPodsForOwner("DaemonSet"),
		EnablePeriodicRefresh: true,
		RefreshInterval:       RefreshInterval,
		TrackSelection:        true,
		CustomUpdate:          getPeriodicRefreshUpdate(),
	}
}

// GetJobsScreenConfig returns the config for the Jobs screen (Level 1)
func GetJobsScreenConfig() ScreenConfig {
	return ScreenConfig{
		ID:           "jobs",
		Title:        "Jobs",
		ResourceType: k8s.ResourceTypeJob,
		Columns: []ColumnConfig{
			{Field: "Namespace", Title: "Namespace", Width: 40},
			{Field: "Name", Title: "Name", Width: 0},
			{Field: "Completions", Title: "Completions", Width: 15},
			{Field: "Age", Title: "Age", Width: 10, Format: FormatDuration},
		},
		SearchFields: []string{"Namespace", "Name"},
		Operations: []OperationConfig{
			{ID: "describe", Name: "Describe", Description: "Describe selected job", Shortcut: "d"},
			{ID: "delete", Name: "Delete", Description: "Delete selected job", Shortcut: "x"},
		},
		NavigationHandler:     navigateToPodsForOwner("Job"),
		EnablePeriodicRefresh: true,
		RefreshInterval:       RefreshInterval,
		TrackSelection:        true,
		CustomUpdate:          getPeriodicRefreshUpdate(),
	}
}

// GetCronJobsScreenConfig returns the config for the CronJobs screen (Level 1)
func GetCronJobsScreenConfig() ScreenConfig {
	return ScreenConfig{
		ID:           "cronjobs",
		Title:        "CronJobs",
		ResourceType: k8s.ResourceTypeCronJob,
		Columns: []ColumnConfig{
			{Field: "Namespace", Title: "Namespace", Width: 40},
			{Field: "Name", Title: "Name", Width: 0},
			{Field: "Schedule", Title: "Schedule", Width: 15},
			{Field: "Suspend", Title: "Suspend", Width: 10},
			{Field: "Active", Title: "Active", Width: 10},
			{Field: "Age", Title: "Age", Width: 10, Format: FormatDuration},
		},
		SearchFields: []string{"Namespace", "Name", "Schedule"},
		Operations: []OperationConfig{
			{ID: "describe", Name: "Describe", Description: "Describe selected cronjob", Shortcut: "d"},
			{ID: "delete", Name: "Delete", Description: "Delete selected cronjob", Shortcut: "x"},
		},
		NavigationHandler:     navigateToJobsForCronJob(),
		EnablePeriodicRefresh: true,
		RefreshInterval:       RefreshInterval,
		TrackSelection:        true,
		CustomUpdate:          getPeriodicRefreshUpdate(),
	}
}

// GetNodesScreenConfig returns the config for the Nodes screen (Level 1)
func GetNodesScreenConfig() ScreenConfig {
	return ScreenConfig{
		ID:           "nodes",
		Title:        "Nodes",
		ResourceType: k8s.ResourceTypeNode,
		Columns: []ColumnConfig{
			{Field: "Name", Title: "Name", Width: 0},
			{Field: "Status", Title: "Status", Width: 12},
			{Field: "Roles", Title: "Roles", Width: 15},
			{Field: "Hostname", Title: "Hostname", Width: 30},
			{Field: "InstanceType", Title: "Instance", Width: 20},
			{Field: "Zone", Title: "Zone", Width: 20},
			{Field: "NodePool", Title: "NodePool", Width: 20},
			{Field: "Version", Title: "Version", Width: 15},
			{Field: "OSImage", Title: "OS Image", Width: 40},
			{Field: "Age", Title: "Age", Width: 10, Format: FormatDuration},
		},
		SearchFields: []string{"Name", "Status", "Roles", "Hostname", "InstanceType", "Zone", "NodePool", "OSImage"},
		Operations: []OperationConfig{
			{ID: "describe", Name: "Describe", Description: "Describe selected node", Shortcut: "d"},
			{ID: "cordon", Name: "Cordon", Description: "Cordon selected node", Shortcut: "c"},
			{ID: "drain", Name: "Drain", Description: "Drain selected node", Shortcut: "r"},
		},
		NavigationHandler:     navigateToPodsForNode(),
		EnablePeriodicRefresh: true,
		RefreshInterval:       RefreshInterval,
		TrackSelection:        true,
		CustomUpdate:          getPeriodicRefreshUpdate(),
	}
}

// GetReplicaSetsScreenConfig returns the configuration for ReplicaSets screen
func GetReplicaSetsScreenConfig() ScreenConfig {
	return ScreenConfig{
		ID:           "replicasets",
		Title:        "ReplicaSets",
		ResourceType: k8s.ResourceTypeReplicaSet,
		Columns: []ColumnConfig{
			{Field: "Namespace", Title: "Namespace", Width: 40},
			{Field: "Name", Title: "Name", Width: 0},
			{Field: "Desired", Title: "Desired", Width: 10},
			{Field: "Current", Title: "Current", Width: 10},
			{Field: "Ready", Title: "Ready", Width: 10},
			{Field: "Age", Title: "Age", Width: 10, Format: FormatDuration},
		},
		SearchFields: []string{"Namespace", "Name"},
		Operations: []OperationConfig{
			{ID: "describe", Name: "Describe", Description: "Describe selected ReplicaSet", Shortcut: "d"},
			{ID: "yaml", Name: "YAML", Description: "View YAML", Shortcut: "y"},
		},
		NavigationHandler:     navigateToPodsForOwner("ReplicaSet"),
		EnablePeriodicRefresh: true,
		RefreshInterval:       RefreshInterval,
		TrackSelection:        true,
		CustomUpdate:          getPeriodicRefreshUpdate(),
	}
}

// GetPVCsScreenConfig returns the configuration for PersistentVolumeClaims screen
func GetPVCsScreenConfig() ScreenConfig {
	return ScreenConfig{
		ID:           "persistentvolumeclaims",
		Title:        "PersistentVolumeClaims",
		ResourceType: k8s.ResourceTypePersistentVolumeClaim,
		Columns: []ColumnConfig{
			{Field: "Namespace", Title: "Namespace", Width: 40},
			{Field: "Name", Title: "Name", Width: 0},
			{Field: "Status", Title: "Status", Width: 12},
			{Field: "Volume", Title: "Volume", Width: 30},
			{Field: "Capacity", Title: "Capacity", Width: 12},
			{Field: "AccessModes", Title: "Access", Width: 12},
			{Field: "StorageClass", Title: "StorageClass", Width: 20},
			{Field: "Age", Title: "Age", Width: 10, Format: FormatDuration},
		},
		SearchFields: []string{"Namespace", "Name", "Status", "StorageClass"},
		Operations: []OperationConfig{
			{ID: "describe", Name: "Describe", Description: "Describe selected PVC", Shortcut: "d"},
			{ID: "yaml", Name: "YAML", Description: "View YAML", Shortcut: "y"},
		},
		NavigationHandler:     navigateToPodsForPVC(),
		EnablePeriodicRefresh: true,
		RefreshInterval:       RefreshInterval,
		TrackSelection:        true,
		CustomUpdate:          getPeriodicRefreshUpdate(),
	}
}

// GetIngressesScreenConfig returns the configuration for Ingresses screen
func GetIngressesScreenConfig() ScreenConfig {
	return ScreenConfig{
		ID:           "ingresses",
		Title:        "Ingresses",
		ResourceType: k8s.ResourceTypeIngress,
		Columns: []ColumnConfig{
			{Field: "Namespace", Title: "Namespace", Width: 40},
			{Field: "Name", Title: "Name", Width: 0},
			{Field: "Class", Title: "Class", Width: 20},
			{Field: "Hosts", Title: "Hosts", Width: 40},
			{Field: "Address", Title: "Address", Width: 30},
			{Field: "Ports", Title: "Ports", Width: 12},
			{Field: "Age", Title: "Age", Width: 10, Format: FormatDuration},
		},
		SearchFields: []string{"Namespace", "Name", "Hosts", "Address"},
		Operations: []OperationConfig{
			{ID: "describe", Name: "Describe", Description: "Describe selected Ingress", Shortcut: "d"},
			{ID: "yaml", Name: "YAML", Description: "View YAML", Shortcut: "y"},
		},
		NavigationHandler:     navigateToServicesForIngress(),
		EnablePeriodicRefresh: true,
		RefreshInterval:       RefreshInterval,
		TrackSelection:        true,
		CustomUpdate:          getPeriodicRefreshUpdate(),
	}
}

// GetEndpointsScreenConfig returns the configuration for Endpoints screen
func GetEndpointsScreenConfig() ScreenConfig {
	return ScreenConfig{
		ID:           "endpoints",
		Title:        "Endpoints",
		ResourceType: k8s.ResourceTypeEndpoints,
		Columns: []ColumnConfig{
			{Field: "Namespace", Title: "Namespace", Width: 40},
			{Field: "Name", Title: "Name", Width: 30},
			{Field: "Endpoints", Title: "Endpoints", Width: 0},
			{Field: "Age", Title: "Age", Width: 10, Format: FormatDuration},
		},
		SearchFields: []string{"Namespace", "Name", "Endpoints"},
		Operations: []OperationConfig{
			{ID: "describe", Name: "Describe", Description: "Describe selected Endpoints", Shortcut: "d"},
			{ID: "yaml", Name: "YAML", Description: "View YAML", Shortcut: "y"},
		},
		NavigationHandler:     navigateToPodsForEndpoints(),
		EnablePeriodicRefresh: true,
		RefreshInterval:       RefreshInterval,
		TrackSelection:        true,
		CustomUpdate:          getPeriodicRefreshUpdate(),
	}
}

// GetHPAsScreenConfig returns the configuration for HorizontalPodAutoscalers screen
func GetHPAsScreenConfig() ScreenConfig {
	return ScreenConfig{
		ID:           "horizontalpodautoscalers",
		Title:        "HorizontalPodAutoscalers",
		ResourceType: k8s.ResourceTypeHPA,
		Columns: []ColumnConfig{
			{Field: "Namespace", Title: "Namespace", Width: 40},
			{Field: "Name", Title: "Name", Width: 0},
			{Field: "Reference", Title: "Reference", Width: 35},
			{Field: "MinPods", Title: "Min", Width: 8},
			{Field: "MaxPods", Title: "Max", Width: 8},
			{Field: "Replicas", Title: "Current", Width: 10},
			{Field: "TargetCPU", Title: "Target", Width: 12},
			{Field: "Age", Title: "Age", Width: 10, Format: FormatDuration},
		},
		SearchFields: []string{"Namespace", "Name", "Reference"},
		Operations: []OperationConfig{
			{ID: "describe", Name: "Describe", Description: "Describe selected HPA", Shortcut: "d"},
			{ID: "yaml", Name: "YAML", Description: "View YAML", Shortcut: "y"},
		},
		NavigationHandler:     navigateToTargetForHPA(),
		EnablePeriodicRefresh: true,
		RefreshInterval:       RefreshInterval,
		TrackSelection:        true,
		CustomUpdate:          getPeriodicRefreshUpdate(),
	}
}

// GetContextsScreenConfig returns config for Contexts screen
func GetContextsScreenConfig() ScreenConfig {
	return ScreenConfig{
		ID:           "contexts",
		Title:        "Contexts",
		ResourceType: k8s.ResourceTypeContext,
		Columns: []ColumnConfig{
			{Field: "Current", Title: "✓", Width: 5},
			{Field: "Name", Title: "Name", Width: 30},
			{Field: "Cluster", Title: "Cluster", Width: 0}, // Dynamic width (same as User)
			{Field: "User", Title: "User", Width: 0},       // Dynamic width (same as Cluster)
			{Field: "Status", Title: "Status", Width: 15},
		},
		SearchFields: []string{"Name", "Cluster", "User", "Status"},
		Operations:   []OperationConfig{}, // No operations - ctrl+r is global refresh
		NavigationHandler:     navigateToContextSwitch(),
		TrackSelection:        true,
		EnablePeriodicRefresh: true,                   // Auto-refresh to update loading status
		RefreshInterval:       ContextsRefreshInterval, // Refresh every 30 seconds (contexts don't change often)
		CustomUpdate:          getPeriodicRefreshUpdate(), // Handle tick messages for refresh
	}
}
