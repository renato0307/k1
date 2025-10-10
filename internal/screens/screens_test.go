package screens

import (
	"testing"

	"github.com/renato0307/k1/internal/k8s"
	"github.com/renato0307/k1/internal/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScreenConfigs(t *testing.T) {
	theme := ui.GetTheme("charm")

	tests := []struct {
		name             string
		getConfig        func(*ui.Theme) ScreenConfig
		expectedID       string
		expectedTitle    string
		expectedResource k8s.ResourceType
		minColumns       int
		minSearchFields  int
		minOperations    int
	}{
		{
			name:             "Pods",
			getConfig:        GetPodsScreenConfig,
			expectedID:       "pods",
			expectedTitle:    "Pods",
			expectedResource: k8s.ResourceTypePod,
			minColumns:       5,
			minSearchFields:  3,
			minOperations:    2,
		},
		{
			name:             "Deployments",
			getConfig:        GetDeploymentsScreenConfig,
			expectedID:       "deployments",
			expectedTitle:    "Deployments",
			expectedResource: k8s.ResourceTypeDeployment,
			minColumns:       4,
			minSearchFields:  2,
			minOperations:    2,
		},
		{
			name:             "Services",
			getConfig:        GetServicesScreenConfig,
			expectedID:       "services",
			expectedTitle:    "Services",
			expectedResource: k8s.ResourceTypeService,
			minColumns:       5,
			minSearchFields:  2,
			minOperations:    2,
		},
		{
			name:             "ConfigMaps",
			getConfig:        GetConfigMapsScreenConfig,
			expectedID:       "configmaps",
			expectedTitle:    "ConfigMaps",
			expectedResource: k8s.ResourceTypeConfigMap,
			minColumns:       3,
			minSearchFields:  2,
			minOperations:    2,
		},
		{
			name:             "Secrets",
			getConfig:        GetSecretsScreenConfig,
			expectedID:       "secrets",
			expectedTitle:    "Secrets",
			expectedResource: k8s.ResourceTypeSecret,
			minColumns:       4,
			minSearchFields:  2,
			minOperations:    2,
		},
		{
			name:             "Namespaces",
			getConfig:        GetNamespacesScreenConfig,
			expectedID:       "namespaces",
			expectedTitle:    "Namespaces",
			expectedResource: k8s.ResourceTypeNamespace,
			minColumns:       2,
			minSearchFields:  2,
			minOperations:    2,
		},
		{
			name:             "StatefulSets",
			getConfig:        GetStatefulSetsScreenConfig,
			expectedID:       "statefulsets",
			expectedTitle:    "StatefulSets",
			expectedResource: k8s.ResourceTypeStatefulSet,
			minColumns:       3,
			minSearchFields:  2,
			minOperations:    2,
		},
		{
			name:             "DaemonSets",
			getConfig:        GetDaemonSetsScreenConfig,
			expectedID:       "daemonsets",
			expectedTitle:    "DaemonSets",
			expectedResource: k8s.ResourceTypeDaemonSet,
			minColumns:       5,
			minSearchFields:  2,
			minOperations:    2,
		},
		{
			name:             "Jobs",
			getConfig:        GetJobsScreenConfig,
			expectedID:       "jobs",
			expectedTitle:    "Jobs",
			expectedResource: k8s.ResourceTypeJob,
			minColumns:       3,
			minSearchFields:  2,
			minOperations:    2,
		},
		{
			name:             "CronJobs",
			getConfig:        GetCronJobsScreenConfig,
			expectedID:       "cronjobs",
			expectedTitle:    "CronJobs",
			expectedResource: k8s.ResourceTypeCronJob,
			minColumns:       4,
			minSearchFields:  2,
			minOperations:    2,
		},
		{
			name:             "Nodes",
			getConfig:        GetNodesScreenConfig,
			expectedID:       "nodes",
			expectedTitle:    "Nodes",
			expectedResource: k8s.ResourceTypeNode,
			minColumns:       5,
			minSearchFields:  3,
			minOperations:    2,
		},
		{
			name:             "ReplicaSets",
			getConfig:        GetReplicaSetsScreenConfig,
			expectedID:       "replicasets",
			expectedTitle:    "ReplicaSets",
			expectedResource: k8s.ResourceTypeReplicaSet,
			minColumns:       5,
			minSearchFields:  2,
			minOperations:    2,
		},
		{
			name:             "PersistentVolumeClaims",
			getConfig:        GetPVCsScreenConfig,
			expectedID:       "persistentvolumeclaims",
			expectedTitle:    "PersistentVolumeClaims",
			expectedResource: k8s.ResourceTypePersistentVolumeClaim,
			minColumns:       6,
			minSearchFields:  3,
			minOperations:    2,
		},
		{
			name:             "Ingresses",
			getConfig:        GetIngressesScreenConfig,
			expectedID:       "ingresses",
			expectedTitle:    "Ingresses",
			expectedResource: k8s.ResourceTypeIngress,
			minColumns:       5,
			minSearchFields:  3,
			minOperations:    2,
		},
		{
			name:             "Endpoints",
			getConfig:        GetEndpointsScreenConfig,
			expectedID:       "endpoints",
			expectedTitle:    "Endpoints",
			expectedResource: k8s.ResourceTypeEndpoints,
			minColumns:       3,
			minSearchFields:  3,
			minOperations:    2,
		},
		{
			name:             "HorizontalPodAutoscalers",
			getConfig:        GetHPAsScreenConfig,
			expectedID:       "horizontalpodautoscalers",
			expectedTitle:    "HorizontalPodAutoscalers",
			expectedResource: k8s.ResourceTypeHPA,
			minColumns:       6,
			minSearchFields:  3,
			minOperations:    2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := tt.getConfig(theme)

			// Basic fields
			assert.Equal(t, tt.expectedID, config.ID)
			assert.Equal(t, tt.expectedTitle, config.Title)
			assert.Equal(t, tt.expectedResource, config.ResourceType)

			// Columns
			require.GreaterOrEqual(t, len(config.Columns), tt.minColumns, "Should have minimum columns")
			for _, col := range config.Columns {
				assert.NotEmpty(t, col.Field, "Column field should not be empty")
				assert.NotEmpty(t, col.Title, "Column title should not be empty")
			}

			// At least one dynamic width column (Width: 0)
			hasDynamicWidth := false
			for _, col := range config.Columns {
				if col.Width == 0 {
					hasDynamicWidth = true
					break
				}
			}
			assert.True(t, hasDynamicWidth, "Should have at least one dynamic width column")

			// Search fields
			require.GreaterOrEqual(t, len(config.SearchFields), tt.minSearchFields, "Should have minimum search fields")

			// Operations
			require.GreaterOrEqual(t, len(config.Operations), tt.minOperations, "Should have minimum operations")
			for _, op := range config.Operations {
				assert.NotEmpty(t, op.ID, "Operation ID should not be empty")
				assert.NotEmpty(t, op.Name, "Operation name should not be empty")
			}
		})
	}
}

func TestPodsScreenConfig_PeriodicRefresh(t *testing.T) {
	theme := ui.GetTheme("charm")
	config := GetPodsScreenConfig(theme)
	assert.True(t, config.EnablePeriodicRefresh, "Pods should have periodic refresh enabled")
	assert.True(t, config.TrackSelection, "Pods should track selection")
	assert.NotNil(t, config.CustomUpdate, "Pods should have custom update function")
	assert.Greater(t, config.RefreshInterval.Seconds(), 0.0, "Pods should have positive refresh interval")
}

func TestTickCmd(t *testing.T) {
	cmd := tickCmd()
	assert.NotNil(t, cmd, "tickCmd should return a command")

	// Execute the command to verify it returns a tickMsg
	msg := cmd()
	assert.NotNil(t, msg, "tickCmd should produce a message")
}

func TestScreenConfigs_NavigationHandlers(t *testing.T) {
	theme := ui.GetTheme("charm")

	tests := []struct {
		name            string
		getConfig       func(*ui.Theme) ScreenConfig
		shouldHaveNav   bool
		expectedNavType string // "owner", "node", "service", "namespace", "volume", "cronjob"
	}{
		{
			name:          "Pods should not have navigation handler",
			getConfig:     GetPodsScreenConfig,
			shouldHaveNav: false,
		},
		{
			name:            "Deployments should navigate to pods",
			getConfig:       GetDeploymentsScreenConfig,
			shouldHaveNav:   true,
			expectedNavType: "owner",
		},
		{
			name:            "Services should navigate to pods",
			getConfig:       GetServicesScreenConfig,
			shouldHaveNav:   true,
			expectedNavType: "service",
		},
		{
			name:            "ConfigMaps should navigate to pods",
			getConfig:       GetConfigMapsScreenConfig,
			shouldHaveNav:   true,
			expectedNavType: "volume",
		},
		{
			name:            "Secrets should navigate to pods",
			getConfig:       GetSecretsScreenConfig,
			shouldHaveNav:   true,
			expectedNavType: "volume",
		},
		{
			name:            "Namespaces should navigate to pods",
			getConfig:       GetNamespacesScreenConfig,
			shouldHaveNav:   true,
			expectedNavType: "namespace",
		},
		{
			name:            "StatefulSets should navigate to pods",
			getConfig:       GetStatefulSetsScreenConfig,
			shouldHaveNav:   true,
			expectedNavType: "owner",
		},
		{
			name:            "DaemonSets should navigate to pods",
			getConfig:       GetDaemonSetsScreenConfig,
			shouldHaveNav:   true,
			expectedNavType: "owner",
		},
		{
			name:            "Jobs should navigate to pods",
			getConfig:       GetJobsScreenConfig,
			shouldHaveNav:   true,
			expectedNavType: "owner",
		},
		{
			name:            "CronJobs should navigate to jobs",
			getConfig:       GetCronJobsScreenConfig,
			shouldHaveNav:   true,
			expectedNavType: "cronjob",
		},
		{
			name:            "Nodes should navigate to pods",
			getConfig:       GetNodesScreenConfig,
			shouldHaveNav:   true,
			expectedNavType: "node",
		},
		{
			name:            "ReplicaSets should navigate to pods",
			getConfig:       GetReplicaSetsScreenConfig,
			shouldHaveNav:   true,
			expectedNavType: "owner",
		},
		{
			name:            "PVCs should navigate to pods",
			getConfig:       GetPVCsScreenConfig,
			shouldHaveNav:   true,
			expectedNavType: "pvc",
		},
		{
			name:            "Ingresses should navigate to services",
			getConfig:       GetIngressesScreenConfig,
			shouldHaveNav:   true,
			expectedNavType: "ingress",
		},
		{
			name:            "Endpoints should navigate to pods",
			getConfig:       GetEndpointsScreenConfig,
			shouldHaveNav:   true,
			expectedNavType: "endpoints",
		},
		{
			name:            "HPAs should navigate to target",
			getConfig:       GetHPAsScreenConfig,
			shouldHaveNav:   true,
			expectedNavType: "hpa",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := tt.getConfig(theme)

			if tt.shouldHaveNav {
				assert.NotNil(t, config.NavigationHandler, "Screen should have navigation handler")

				// Verify the handler works by testing with a mock screen
				// We can't easily test the exact type, but we can verify it returns a command
				repo := k8s.NewDummyRepository()
				screen := NewConfigScreen(config, repo, theme)

				// Add mock data based on screen type
				switch tt.expectedNavType {
				case "owner":
					screen.items = []interface{}{k8s.Deployment{ResourceMetadata: k8s.ResourceMetadata{Namespace: "test", Name: "test-deploy"}}}
				case "node":
					screen.items = []interface{}{k8s.Node{ResourceMetadata: k8s.ResourceMetadata{Name: "test-node"}}}
				case "service":
					screen.items = []interface{}{k8s.Service{ResourceMetadata: k8s.ResourceMetadata{Namespace: "test", Name: "test-svc"}}}
				case "namespace":
					screen.items = []interface{}{k8s.Namespace{ResourceMetadata: k8s.ResourceMetadata{Name: "test-ns"}}}
				case "volume":
					screen.items = []interface{}{k8s.ConfigMap{ResourceMetadata: k8s.ResourceMetadata{Namespace: "test", Name: "test-cm"}}}
				case "cronjob":
					screen.items = []interface{}{k8s.CronJob{ResourceMetadata: k8s.ResourceMetadata{Namespace: "test", Name: "test-cron"}}}
				case "pvc":
					screen.items = []interface{}{k8s.PersistentVolumeClaim{ResourceMetadata: k8s.ResourceMetadata{Namespace: "test", Name: "test-pvc"}}}
				case "ingress":
					screen.items = []interface{}{k8s.Ingress{ResourceMetadata: k8s.ResourceMetadata{Namespace: "test", Name: "test-ingress"}}}
				case "endpoints":
					screen.items = []interface{}{k8s.Endpoints{ResourceMetadata: k8s.ResourceMetadata{Namespace: "test", Name: "test-ep"}}}
				case "hpa":
					screen.items = []interface{}{k8s.HorizontalPodAutoscaler{ResourceMetadata: k8s.ResourceMetadata{Namespace: "test", Name: "test-hpa"}, Reference: "Deployment/nginx"}}
				}
				screen.applyFilter()
				screen.table.SetCursor(0)

				// Call the handler
				cmd := config.NavigationHandler(screen)
				assert.NotNil(t, cmd, "Navigation handler should return a command")
			} else {
				assert.Nil(t, config.NavigationHandler, "Screen should not have navigation handler")
			}
		})
	}
}
