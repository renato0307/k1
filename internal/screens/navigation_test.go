package screens

import (
	"testing"

	"github.com/renato0307/k1/internal/k8s"
	"github.com/renato0307/k1/internal/types"
	"github.com/renato0307/k1/internal/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNavigateToPodsForOwner(t *testing.T) {
	repo := k8s.NewDummyRepository()
	theme := ui.GetTheme("charm")

	tests := []struct {
		name              string
		kind              string
		resource          interface{}
		expectNil         bool
		expectedScreenID  string
		expectedField     string
		expectedValue     string
		expectedNamespace string
		expectedKind      string
	}{
		{
			name:              "deployment navigation",
			kind:              "Deployment",
			resource:          k8s.Deployment{Namespace: "default", Name: "nginx"},
			expectNil:         false,
			expectedScreenID:  "pods",
			expectedField:     "owner",
			expectedValue:     "nginx",
			expectedNamespace: "default",
			expectedKind:      "Deployment",
		},
		{
			name:              "statefulset navigation",
			kind:              "StatefulSet",
			resource:          k8s.StatefulSet{Namespace: "prod", Name: "web"},
			expectNil:         false,
			expectedScreenID:  "pods",
			expectedField:     "owner",
			expectedValue:     "web",
			expectedNamespace: "prod",
			expectedKind:      "StatefulSet",
		},
		{
			name:              "daemonset navigation",
			kind:              "DaemonSet",
			resource:          k8s.DaemonSet{Namespace: "kube-system", Name: "fluentd"},
			expectNil:         false,
			expectedScreenID:  "pods",
			expectedField:     "owner",
			expectedValue:     "fluentd",
			expectedNamespace: "kube-system",
			expectedKind:      "DaemonSet",
		},
		{
			name:              "job navigation",
			kind:              "Job",
			resource:          k8s.Job{Namespace: "batch", Name: "backup"},
			expectNil:         false,
			expectedScreenID:  "pods",
			expectedField:     "owner",
			expectedValue:     "backup",
			expectedNamespace: "batch",
			expectedKind:      "Job",
		},
		{
			name:      "missing namespace returns nil",
			kind:      "Deployment",
			resource:  k8s.Deployment{Name: "nginx"}, // Missing namespace
			expectNil: true,
		},
		{
			name:      "missing name returns nil",
			kind:      "Deployment",
			resource:  k8s.Deployment{Namespace: "default"}, // Missing name
			expectNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := ScreenConfig{
				ID:           "test",
				Title:        "Test",
				ResourceType: k8s.ResourceTypeDeployment,
				Columns: []ColumnConfig{
					{Field: "Name", Title: "Name", Width: 0},
					{Field: "Namespace", Title: "Namespace", Width: 20},
				},
			}

			screen := NewConfigScreen(cfg, repo, theme)
			screen.items = []interface{}{tt.resource}
			screen.applyFilter()
			screen.table.SetCursor(0)

			handler := navigateToPodsForOwner(tt.kind)
			cmd := handler(screen)

			if tt.expectNil {
				assert.Nil(t, cmd)
				return
			}

			require.NotNil(t, cmd)
			msg := cmd()
			switchMsg, ok := msg.(types.ScreenSwitchMsg)
			require.True(t, ok, "expected ScreenSwitchMsg")

			assert.Equal(t, tt.expectedScreenID, switchMsg.ScreenID)
			require.NotNil(t, switchMsg.FilterContext)
			assert.Equal(t, tt.expectedField, switchMsg.FilterContext.Field)
			assert.Equal(t, tt.expectedValue, switchMsg.FilterContext.Value)
			assert.Equal(t, tt.expectedNamespace, switchMsg.FilterContext.Metadata["namespace"])
			assert.Equal(t, tt.expectedKind, switchMsg.FilterContext.Metadata["kind"])
		})
	}
}

func TestNavigateToJobsForCronJob(t *testing.T) {
	repo := k8s.NewDummyRepository()
	theme := ui.GetTheme("charm")

	tests := []struct {
		name              string
		resource          interface{}
		expectNil         bool
		expectedScreenID  string
		expectedField     string
		expectedValue     string
		expectedNamespace string
	}{
		{
			name:              "valid cronjob",
			resource:          k8s.CronJob{Namespace: "default", Name: "daily-backup"},
			expectNil:         false,
			expectedScreenID:  "jobs",
			expectedField:     "owner",
			expectedValue:     "daily-backup",
			expectedNamespace: "default",
		},
		{
			name:      "missing namespace returns nil",
			resource:  k8s.CronJob{Name: "backup"},
			expectNil: true,
		},
		{
			name:      "missing name returns nil",
			resource:  k8s.CronJob{Namespace: "default"},
			expectNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := ScreenConfig{
				ID:           "cronjobs",
				Title:        "CronJobs",
				ResourceType: k8s.ResourceTypeCronJob,
				Columns: []ColumnConfig{
					{Field: "Name", Title: "Name", Width: 0},
					{Field: "Namespace", Title: "Namespace", Width: 20},
				},
			}

			screen := NewConfigScreen(cfg, repo, theme)
			screen.items = []interface{}{tt.resource}
			screen.applyFilter()
			screen.table.SetCursor(0)

			handler := navigateToJobsForCronJob()
			cmd := handler(screen)

			if tt.expectNil {
				assert.Nil(t, cmd)
				return
			}

			require.NotNil(t, cmd)
			msg := cmd()
			switchMsg, ok := msg.(types.ScreenSwitchMsg)
			require.True(t, ok, "expected ScreenSwitchMsg")

			assert.Equal(t, tt.expectedScreenID, switchMsg.ScreenID)
			require.NotNil(t, switchMsg.FilterContext)
			assert.Equal(t, tt.expectedField, switchMsg.FilterContext.Field)
			assert.Equal(t, tt.expectedValue, switchMsg.FilterContext.Value)
			assert.Equal(t, tt.expectedNamespace, switchMsg.FilterContext.Metadata["namespace"])
			assert.Equal(t, "CronJob", switchMsg.FilterContext.Metadata["kind"])
		})
	}
}

func TestNavigateToPodsForNode(t *testing.T) {
	repo := k8s.NewDummyRepository()
	theme := ui.GetTheme("charm")

	tests := []struct {
		name             string
		resource         interface{}
		expectNil        bool
		expectedScreenID string
		expectedField    string
		expectedValue    string
	}{
		{
			name:             "valid node",
			resource:         k8s.Node{Name: "node-1"},
			expectNil:        false,
			expectedScreenID: "pods",
			expectedField:    "node",
			expectedValue:    "node-1",
		},
		{
			name:      "missing name returns nil",
			resource:  k8s.Node{},
			expectNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := ScreenConfig{
				ID:           "nodes",
				Title:        "Nodes",
				ResourceType: k8s.ResourceTypeNode,
				Columns: []ColumnConfig{
					{Field: "Name", Title: "Name", Width: 0},
				},
			}

			screen := NewConfigScreen(cfg, repo, theme)
			screen.items = []interface{}{tt.resource}
			screen.applyFilter()
			screen.table.SetCursor(0)

			handler := navigateToPodsForNode()
			cmd := handler(screen)

			if tt.expectNil {
				assert.Nil(t, cmd)
				return
			}

			require.NotNil(t, cmd)
			msg := cmd()
			switchMsg, ok := msg.(types.ScreenSwitchMsg)
			require.True(t, ok, "expected ScreenSwitchMsg")

			assert.Equal(t, tt.expectedScreenID, switchMsg.ScreenID)
			require.NotNil(t, switchMsg.FilterContext)
			assert.Equal(t, tt.expectedField, switchMsg.FilterContext.Field)
			assert.Equal(t, tt.expectedValue, switchMsg.FilterContext.Value)
			assert.Equal(t, "Node", switchMsg.FilterContext.Metadata["kind"])
		})
	}
}

func TestNavigateToPodsForService(t *testing.T) {
	repo := k8s.NewDummyRepository()
	theme := ui.GetTheme("charm")

	tests := []struct {
		name              string
		resource          interface{}
		expectNil         bool
		expectedScreenID  string
		expectedField     string
		expectedValue     string
		expectedNamespace string
	}{
		{
			name:              "valid service",
			resource:          k8s.Service{Namespace: "default", Name: "nginx-svc"},
			expectNil:         false,
			expectedScreenID:  "pods",
			expectedField:     "selector",
			expectedValue:     "nginx-svc",
			expectedNamespace: "default",
		},
		{
			name:      "missing namespace returns nil",
			resource:  k8s.Service{Name: "nginx-svc"},
			expectNil: true,
		},
		{
			name:      "missing name returns nil",
			resource:  k8s.Service{Namespace: "default"},
			expectNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := ScreenConfig{
				ID:           "services",
				Title:        "Services",
				ResourceType: k8s.ResourceTypeService,
				Columns: []ColumnConfig{
					{Field: "Name", Title: "Name", Width: 0},
					{Field: "Namespace", Title: "Namespace", Width: 20},
				},
			}

			screen := NewConfigScreen(cfg, repo, theme)
			screen.items = []interface{}{tt.resource}
			screen.applyFilter()
			screen.table.SetCursor(0)

			handler := navigateToPodsForService()
			cmd := handler(screen)

			if tt.expectNil {
				assert.Nil(t, cmd)
				return
			}

			require.NotNil(t, cmd)
			msg := cmd()
			switchMsg, ok := msg.(types.ScreenSwitchMsg)
			require.True(t, ok, "expected ScreenSwitchMsg")

			assert.Equal(t, tt.expectedScreenID, switchMsg.ScreenID)
			require.NotNil(t, switchMsg.FilterContext)
			assert.Equal(t, tt.expectedField, switchMsg.FilterContext.Field)
			assert.Equal(t, tt.expectedValue, switchMsg.FilterContext.Value)
			assert.Equal(t, tt.expectedNamespace, switchMsg.FilterContext.Metadata["namespace"])
			assert.Equal(t, "Service", switchMsg.FilterContext.Metadata["kind"])
		})
	}
}

func TestNavigateToPodsForNamespace(t *testing.T) {
	repo := k8s.NewDummyRepository()
	theme := ui.GetTheme("charm")

	tests := []struct {
		name             string
		resource         interface{}
		expectNil        bool
		expectedScreenID string
		expectedField    string
		expectedValue    string
	}{
		{
			name:             "valid namespace",
			resource:         k8s.Namespace{Name: "production"},
			expectNil:        false,
			expectedScreenID: "pods",
			expectedField:    "namespace",
			expectedValue:    "production",
		},
		{
			name:      "missing name returns nil",
			resource:  k8s.Namespace{},
			expectNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := ScreenConfig{
				ID:           "namespaces",
				Title:        "Namespaces",
				ResourceType: k8s.ResourceTypeNamespace,
				Columns: []ColumnConfig{
					{Field: "Name", Title: "Name", Width: 0},
				},
			}

			screen := NewConfigScreen(cfg, repo, theme)
			screen.items = []interface{}{tt.resource}
			screen.applyFilter()
			screen.table.SetCursor(0)

			handler := navigateToPodsForNamespace()
			cmd := handler(screen)

			if tt.expectNil {
				assert.Nil(t, cmd)
				return
			}

			require.NotNil(t, cmd)
			msg := cmd()
			switchMsg, ok := msg.(types.ScreenSwitchMsg)
			require.True(t, ok, "expected ScreenSwitchMsg")

			assert.Equal(t, tt.expectedScreenID, switchMsg.ScreenID)
			require.NotNil(t, switchMsg.FilterContext)
			assert.Equal(t, tt.expectedField, switchMsg.FilterContext.Field)
			assert.Equal(t, tt.expectedValue, switchMsg.FilterContext.Value)
			assert.Equal(t, "Namespace", switchMsg.FilterContext.Metadata["kind"])
		})
	}
}

func TestNavigateToPodsForVolumeSource(t *testing.T) {
	repo := k8s.NewDummyRepository()
	theme := ui.GetTheme("charm")

	tests := []struct {
		name              string
		kind              string
		resource          interface{}
		expectNil         bool
		expectedScreenID  string
		expectedField     string
		expectedValue     string
		expectedNamespace string
		expectedKind      string
	}{
		{
			name:              "configmap navigation",
			kind:              "ConfigMap",
			resource:          k8s.ConfigMap{Namespace: "default", Name: "app-config"},
			expectNil:         false,
			expectedScreenID:  "pods",
			expectedField:     "configmap",
			expectedValue:     "app-config",
			expectedNamespace: "default",
			expectedKind:      "ConfigMap",
		},
		{
			name:              "secret navigation",
			kind:              "Secret",
			resource:          k8s.Secret{Namespace: "prod", Name: "db-password"},
			expectNil:         false,
			expectedScreenID:  "pods",
			expectedField:     "secret",
			expectedValue:     "db-password",
			expectedNamespace: "prod",
			expectedKind:      "Secret",
		},
		{
			name:      "missing namespace returns nil",
			kind:      "ConfigMap",
			resource:  k8s.ConfigMap{Name: "config"},
			expectNil: true,
		},
		{
			name:      "missing name returns nil",
			kind:      "Secret",
			resource:  k8s.Secret{Namespace: "default"},
			expectNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := ScreenConfig{
				ID:           "test",
				Title:        "Test",
				ResourceType: k8s.ResourceTypeConfigMap,
				Columns: []ColumnConfig{
					{Field: "Name", Title: "Name", Width: 0},
					{Field: "Namespace", Title: "Namespace", Width: 20},
				},
			}

			screen := NewConfigScreen(cfg, repo, theme)
			screen.items = []interface{}{tt.resource}
			screen.applyFilter()
			screen.table.SetCursor(0)

			handler := navigateToPodsForVolumeSource(tt.kind)
			cmd := handler(screen)

			if tt.expectNil {
				assert.Nil(t, cmd)
				return
			}

			require.NotNil(t, cmd)
			msg := cmd()
			switchMsg, ok := msg.(types.ScreenSwitchMsg)
			require.True(t, ok, "expected ScreenSwitchMsg")

			assert.Equal(t, tt.expectedScreenID, switchMsg.ScreenID)
			require.NotNil(t, switchMsg.FilterContext)
			assert.Equal(t, tt.expectedField, switchMsg.FilterContext.Field)
			assert.Equal(t, tt.expectedValue, switchMsg.FilterContext.Value)
			assert.Equal(t, tt.expectedNamespace, switchMsg.FilterContext.Metadata["namespace"])
			assert.Equal(t, tt.expectedKind, switchMsg.FilterContext.Metadata["kind"])
		})
	}
}

func TestNavigationHandlers_EmptyScreen(t *testing.T) {
	repo := k8s.NewDummyRepository()
	theme := ui.GetTheme("charm")

	cfg := ScreenConfig{
		ID:           "test",
		Title:        "Test",
		ResourceType: k8s.ResourceTypePod,
		Columns: []ColumnConfig{
			{Field: "Name", Title: "Name", Width: 0},
		},
	}

	screen := NewConfigScreen(cfg, repo, theme)
	// No items - empty screen

	tests := []struct {
		name    string
		handler NavigationFunc
	}{
		{"navigateToPodsForOwner", navigateToPodsForOwner("Deployment")},
		{"navigateToJobsForCronJob", navigateToJobsForCronJob()},
		{"navigateToPodsForNode", navigateToPodsForNode()},
		{"navigateToPodsForService", navigateToPodsForService()},
		{"navigateToPodsForNamespace", navigateToPodsForNamespace()},
		{"navigateToPodsForVolumeSource", navigateToPodsForVolumeSource("ConfigMap")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := tt.handler(screen)
			assert.Nil(t, cmd, "expected nil for empty screen")
		})
	}
}
