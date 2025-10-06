package screens

import (
	"testing"

	"github.com/renato0307/k1/internal/k8s"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScreenConfigs(t *testing.T) {
	tests := []struct {
		name             string
		getConfig        func() ScreenConfig
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := tt.getConfig()

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
	config := GetPodsScreenConfig()
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
