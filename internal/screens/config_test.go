package screens

import (
	"testing"
	"time"

	"github.com/renato0307/k1/internal/k8s"
	"github.com/renato0307/k1/internal/types"
	"github.com/renato0307/k1/internal/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewConfigScreen(t *testing.T) {
	cfg := ScreenConfig{
		ID:           "test",
		Title:        "Test Screen",
		ResourceType: k8s.ResourceTypePod,
		Columns: []ColumnConfig{
			{Field: "Namespace", Title: "Namespace", Width: 40},
			{Field: "Name", Title: "Name", Width: 0},
			{Field: "Age", Title: "Age", Width: 10, Format: FormatDuration},
		},
		SearchFields: []string{"Namespace", "Name"},
	}

	repo := k8s.NewDummyRepository()
	theme := ui.GetTheme("charm")

	screen := NewConfigScreen(cfg, repo, theme)

	assert.NotNil(t, screen)
	assert.Equal(t, "test", screen.ID())
	assert.Equal(t, "Test Screen", screen.Title())
	assert.NotEmpty(t, screen.HelpText())
}

func TestConfigScreen_Refresh(t *testing.T) {
	cfg := ScreenConfig{
		ID:           "pods",
		Title:        "Pods",
		ResourceType: k8s.ResourceTypePod,
		Columns: []ColumnConfig{
			{Field: "Namespace", Title: "Namespace", Width: 40},
			{Field: "Name", Title: "Name", Width: 0},
			{Field: "Ready", Title: "Ready", Width: 10},
			{Field: "Status", Title: "Status", Width: 15},
			{Field: "Age", Title: "Age", Width: 10, Format: FormatDuration},
		},
		SearchFields: []string{"Namespace", "Name", "Status"},
	}

	repo := k8s.NewDummyRepository()
	theme := ui.GetTheme("charm")
	screen := NewConfigScreen(cfg, repo, theme)

	// Execute refresh command
	cmd := screen.Refresh()
	require.NotNil(t, cmd)

	// Execute the command to get the message
	msg := cmd()

	// Should return a RefreshCompleteMsg
	refreshMsg, ok := msg.(types.RefreshCompleteMsg)
	require.True(t, ok, "Expected RefreshCompleteMsg")
	assert.Greater(t, refreshMsg.Duration, time.Duration(0))

	// Items should be populated
	assert.NotEmpty(t, screen.items)
	assert.NotEmpty(t, screen.filtered)
}

func TestConfigScreen_SetFilter(t *testing.T) {
	cfg := ScreenConfig{
		ID:           "pods",
		Title:        "Pods",
		ResourceType: k8s.ResourceTypePod,
		Columns: []ColumnConfig{
			{Field: "Namespace", Title: "Namespace", Width: 40},
			{Field: "Name", Title: "Name", Width: 0},
			{Field: "Status", Title: "Status", Width: 15},
		},
		SearchFields: []string{"Namespace", "Name", "Status"},
	}

	repo := k8s.NewDummyRepository()
	theme := ui.GetTheme("charm")
	screen := NewConfigScreen(cfg, repo, theme)

	// First refresh to populate items
	cmd := screen.Refresh()
	cmd() // Execute

	initialCount := len(screen.filtered)

	// Apply filter that should match some items
	screen.SetFilter("nginx")

	// Should have fewer items after filtering
	assert.LessOrEqual(t, len(screen.filtered), initialCount)

	// Clear filter
	screen.SetFilter("")
	assert.Equal(t, initialCount, len(screen.filtered))
}

func TestConfigScreen_SetFilter_Negation(t *testing.T) {
	cfg := ScreenConfig{
		ID:           "pods",
		Title:        "Pods",
		ResourceType: k8s.ResourceTypePod,
		Columns: []ColumnConfig{
			{Field: "Namespace", Title: "Namespace", Width: 40},
			{Field: "Name", Title: "Name", Width: 0},
		},
		SearchFields: []string{"Namespace", "Name"},
	}

	repo := k8s.NewDummyRepository()
	theme := ui.GetTheme("charm")
	screen := NewConfigScreen(cfg, repo, theme)

	// Refresh to populate
	cmd := screen.Refresh()
	cmd()

	initialCount := len(screen.filtered)

	// Apply negation filter
	screen.SetFilter("!nginx")

	// Should exclude items matching nginx
	assert.LessOrEqual(t, len(screen.filtered), initialCount)

	// Check that nginx items are excluded
	for _, item := range screen.filtered {
		pod, ok := item.(k8s.Pod)
		if ok {
			assert.NotContains(t, pod.Name, "nginx")
		}
	}
}

func TestConfigScreen_GetSelectedResource(t *testing.T) {
	cfg := ScreenConfig{
		ID:           "pods",
		Title:        "Pods",
		ResourceType: k8s.ResourceTypePod,
		Columns: []ColumnConfig{
			{Field: "Namespace", Title: "Namespace", Width: 40},
			{Field: "Name", Title: "Name", Width: 0},
		},
		SearchFields: []string{"Namespace", "Name"},
	}

	repo := k8s.NewDummyRepository()
	theme := ui.GetTheme("charm")
	screen := NewConfigScreen(cfg, repo, theme)

	// Refresh to populate
	cmd := screen.Refresh()
	cmd()

	// Get selected resource (should be first item by default)
	resource := screen.GetSelectedResource()

	if len(screen.filtered) > 0 {
		assert.NotNil(t, resource)
		assert.Contains(t, resource, "namespace")
		assert.Contains(t, resource, "name")
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected string
	}{
		{"30 seconds", 30 * time.Second, "30s"},
		{"5 minutes", 5 * time.Minute, "5m"},
		{"2 hours", 2 * time.Hour, "2h"},
		{"3 days", 72 * time.Hour, "3d"},
		{"45 seconds", 45 * time.Second, "45s"},
		{"90 minutes", 90 * time.Minute, "1h"},
		{"non-duration", "not a duration", "not a duration"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatDuration(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConfigScreen_Operations(t *testing.T) {
	cfg := ScreenConfig{
		ID:           "test",
		Title:        "Test",
		ResourceType: k8s.ResourceTypePod,
		Columns: []ColumnConfig{
			{Field: "Name", Title: "Name", Width: 0},
		},
		SearchFields: []string{"Name"},
		Operations: []OperationConfig{
			{ID: "describe", Name: "Describe", Description: "Show details", Shortcut: "d"},
			{ID: "delete", Name: "Delete", Description: "Delete resource", Shortcut: "x"},
		},
	}

	repo := k8s.NewDummyRepository()
	theme := ui.GetTheme("charm")
	screen := NewConfigScreen(cfg, repo, theme)

	ops := screen.Operations()
	require.Len(t, ops, 2)

	assert.Equal(t, "describe", ops[0].ID)
	assert.Equal(t, "Describe", ops[0].Name)
	assert.Equal(t, "d", ops[0].Shortcut)

	assert.Equal(t, "delete", ops[1].ID)
	assert.Equal(t, "Delete", ops[1].Name)
	assert.Equal(t, "x", ops[1].Shortcut)
}

func TestConfigScreen_Init(t *testing.T) {
	cfg := ScreenConfig{
		ID:           "test",
		Title:        "Test",
		ResourceType: k8s.ResourceTypePod,
		Columns:      []ColumnConfig{{Field: "Name", Title: "Name", Width: 0}},
		SearchFields: []string{"Name"},
	}

	screen := NewConfigScreen(cfg, k8s.NewDummyRepository(), ui.GetTheme("charm"))

	cmd := screen.Init()
	assert.NotNil(t, cmd, "Init should return a command")
}

func TestConfigScreen_SetSize(t *testing.T) {
	cfg := ScreenConfig{
		ID:           "test",
		Title:        "Test",
		ResourceType: k8s.ResourceTypePod,
		Columns:      []ColumnConfig{{Field: "Name", Title: "Name", Width: 0}},
		SearchFields: []string{"Name"},
	}

	screen := NewConfigScreen(cfg, k8s.NewDummyRepository(), ui.GetTheme("charm"))

	screen.SetSize(100, 50)
	assert.Equal(t, 100, screen.width)
	assert.Equal(t, 50, screen.height)
}

func TestConfigScreen_View(t *testing.T) {
	cfg := ScreenConfig{
		ID:           "pods",
		Title:        "Pods",
		ResourceType: k8s.ResourceTypePod,
		Columns: []ColumnConfig{
			{Field: "Name", Title: "Name", Width: 0},
			{Field: "Status", Title: "Status", Width: 15},
		},
		SearchFields: []string{"Name"},
	}

	screen := NewConfigScreen(cfg, k8s.NewDummyRepository(), ui.GetTheme("charm"))
	screen.SetSize(80, 24)

	// Perform a refresh to populate data
	screen.Refresh()()

	view := screen.View()
	assert.NotEmpty(t, view, "View should return non-empty string")
}

func TestGetFieldValue(t *testing.T) {
	tests := []struct {
		name      string
		item      interface{}
		fieldName string
		expected  interface{}
	}{
		{
			name:      "valid pod name",
			item:      k8s.Pod{ResourceMetadata: k8s.ResourceMetadata{Name: "test-pod", Namespace: "default"}},
			fieldName: "Name",
			expected:  "test-pod",
		},
		{
			name:      "valid pod namespace",
			item:      k8s.Pod{ResourceMetadata: k8s.ResourceMetadata{Name: "test-pod", Namespace: "default"}},
			fieldName: "Namespace",
			expected:  "default",
		},
		{
			name:      "non-existent field returns empty string",
			item:      k8s.Pod{ResourceMetadata: k8s.ResourceMetadata{Name: "test-pod"}},
			fieldName: "NonExistent",
			expected:  "",
		},
		{
			name: "deployment ready field",
			item: k8s.Deployment{
				ResourceMetadata: k8s.ResourceMetadata{Name: "deploy1"},
				Ready:            "2/2",
			},
			fieldName: "Ready",
			expected:  "2/2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getFieldValue(tt.item, tt.fieldName)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestConfigScreen_NavigateToPodsForDeployment(t *testing.T) {
	cfg := ScreenConfig{
		ID:           "deployments",
		Title:        "Deployments",
		ResourceType: k8s.ResourceTypeDeployment,
		Columns: []ColumnConfig{
			{Field: "Namespace", Title: "Namespace", Width: 40},
			{Field: "Name", Title: "Name", Width: 0},
		},
		SearchFields: []string{"Namespace", "Name"},
	}

	repo := k8s.NewDummyRepository()
	theme := ui.GetTheme("charm")
	screen := NewConfigScreen(cfg, repo, theme)

	// Set up items
	screen.items = []interface{}{
		k8s.Deployment{ResourceMetadata: k8s.ResourceMetadata{Namespace: "default", Name: "nginx-deployment"}},
		k8s.Deployment{ResourceMetadata: k8s.ResourceMetadata{Namespace: "prod", Name: "api-deployment"}},
	}
	screen.applyFilter()

	tests := []struct {
		name              string
		cursorPosition    int
		expectNil         bool
		expectedScreenID  string
		expectedField     string
		expectedValue     string
		expectedNamespace string
	}{
		{
			name:              "valid deployment selection",
			cursorPosition:    0,
			expectNil:         false,
			expectedScreenID:  "pods",
			expectedField:     "owner",
			expectedValue:     "nginx-deployment",
			expectedNamespace: "default",
		},
		{
			name:              "second deployment",
			cursorPosition:    1,
			expectNil:         false,
			expectedScreenID:  "pods",
			expectedField:     "owner",
			expectedValue:     "api-deployment",
			expectedNamespace: "prod",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			screen.table.SetCursor(tt.cursorPosition)
			handler := navigateToPodsForOwner("Deployment")
			cmd := handler(screen)

			if tt.expectNil {
				assert.Nil(t, cmd)
				return
			}

			require.NotNil(t, cmd)

			// Execute the command to get the message
			msg := cmd()
			switchMsg, ok := msg.(types.ScreenSwitchMsg)
			require.True(t, ok, "expected ScreenSwitchMsg")

			assert.Equal(t, tt.expectedScreenID, switchMsg.ScreenID)
			require.NotNil(t, switchMsg.FilterContext)
			assert.Equal(t, tt.expectedField, switchMsg.FilterContext.Field)
			assert.Equal(t, tt.expectedValue, switchMsg.FilterContext.Value)
			assert.Equal(t, tt.expectedNamespace, switchMsg.FilterContext.Metadata["namespace"])
			assert.Equal(t, "Deployment", switchMsg.FilterContext.Metadata["kind"])
		})
	}
}

func TestConfigScreen_NavigateToPodsForNode(t *testing.T) {
	cfg := ScreenConfig{
		ID:           "nodes",
		Title:        "Nodes",
		ResourceType: k8s.ResourceTypeNode,
		Columns: []ColumnConfig{
			{Field: "Name", Title: "Name", Width: 0},
			{Field: "Status", Title: "Status", Width: 15},
		},
		SearchFields: []string{"Name"},
	}

	repo := k8s.NewDummyRepository()
	theme := ui.GetTheme("charm")
	screen := NewConfigScreen(cfg, repo, theme)

	// Set up items
	screen.items = []interface{}{
		k8s.Node{ResourceMetadata: k8s.ResourceMetadata{Name: "node-1"}, Status: "Ready"},
		k8s.Node{ResourceMetadata: k8s.ResourceMetadata{Name: "node-2"}, Status: "Ready"},
	}
	screen.applyFilter()

	tests := []struct {
		name             string
		cursorPosition   int
		expectNil        bool
		expectedScreenID string
		expectedField    string
		expectedValue    string
	}{
		{
			name:             "valid node selection",
			cursorPosition:   0,
			expectNil:        false,
			expectedScreenID: "pods",
			expectedField:    "node",
			expectedValue:    "node-1",
		},
		{
			name:             "second node",
			cursorPosition:   1,
			expectNil:        false,
			expectedScreenID: "pods",
			expectedField:    "node",
			expectedValue:    "node-2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			screen.table.SetCursor(tt.cursorPosition)
			handler := navigateToPodsForNode()
			cmd := handler(screen)

			if tt.expectNil {
				assert.Nil(t, cmd)
				return
			}

			require.NotNil(t, cmd)

			// Execute the command to get the message
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

func TestConfigScreen_NavigateToPodsForService(t *testing.T) {
	cfg := ScreenConfig{
		ID:           "services",
		Title:        "Services",
		ResourceType: k8s.ResourceTypeService,
		Columns: []ColumnConfig{
			{Field: "Namespace", Title: "Namespace", Width: 40},
			{Field: "Name", Title: "Name", Width: 0},
		},
		SearchFields: []string{"Namespace", "Name"},
	}

	repo := k8s.NewDummyRepository()
	theme := ui.GetTheme("charm")
	screen := NewConfigScreen(cfg, repo, theme)

	// Set up items
	screen.items = []interface{}{
		k8s.Service{ResourceMetadata: k8s.ResourceMetadata{Namespace: "default", Name: "nginx-service"}},
		k8s.Service{ResourceMetadata: k8s.ResourceMetadata{Namespace: "prod", Name: "api-service"}},
	}
	screen.applyFilter()

	tests := []struct {
		name              string
		cursorPosition    int
		expectNil         bool
		expectedScreenID  string
		expectedField     string
		expectedValue     string
		expectedNamespace string
	}{
		{
			name:              "valid service selection",
			cursorPosition:    0,
			expectNil:         false,
			expectedScreenID:  "pods",
			expectedField:     "selector",
			expectedValue:     "nginx-service",
			expectedNamespace: "default",
		},
		{
			name:              "second service",
			cursorPosition:    1,
			expectNil:         false,
			expectedScreenID:  "pods",
			expectedField:     "selector",
			expectedValue:     "api-service",
			expectedNamespace: "prod",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			screen.table.SetCursor(tt.cursorPosition)
			handler := navigateToPodsForService()
			cmd := handler(screen)

			if tt.expectNil {
				assert.Nil(t, cmd)
				return
			}

			require.NotNil(t, cmd)

			// Execute the command to get the message
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

func TestConfigScreen_HandleEnterKey(t *testing.T) {
	repo := k8s.NewDummyRepository()
	theme := ui.GetTheme("charm")

	tests := []struct {
		name              string
		screenID          string
		expectNil         bool
		navigationHandler NavigationFunc
		setupItems        func(*ConfigScreen)
	}{
		{
			name:              "deployments screen returns navigation command",
			screenID:          "deployments",
			expectNil:         false,
			navigationHandler: navigateToPodsForOwner("Deployment"),
			setupItems: func(s *ConfigScreen) {
				s.items = []interface{}{
					k8s.Deployment{ResourceMetadata: k8s.ResourceMetadata{Namespace: "default", Name: "test-deployment"}},
				}
				s.applyFilter()
			},
		},
		{
			name:              "nodes screen returns navigation command",
			screenID:          "nodes",
			expectNil:         false,
			navigationHandler: navigateToPodsForNode(),
			setupItems: func(s *ConfigScreen) {
				s.items = []interface{}{
					k8s.Node{ResourceMetadata: k8s.ResourceMetadata{Name: "test-node"}},
				}
				s.applyFilter()
			},
		},
		{
			name:              "services screen returns navigation command",
			screenID:          "services",
			expectNil:         false,
			navigationHandler: navigateToPodsForService(),
			setupItems: func(s *ConfigScreen) {
				s.items = []interface{}{
					k8s.Service{ResourceMetadata: k8s.ResourceMetadata{Namespace: "default", Name: "test-service"}},
				}
				s.applyFilter()
			},
		},
		{
			name:              "pods screen returns nil (no contextual navigation)",
			screenID:          "pods",
			expectNil:         true,
			navigationHandler: nil,
			setupItems: func(s *ConfigScreen) {
				s.items = []interface{}{
					k8s.Pod{ResourceMetadata: k8s.ResourceMetadata{Namespace: "default", Name: "test-pod"}},
				}
				s.applyFilter()
			},
		},
		{
			name:              "screen without handler returns nil",
			screenID:          "other",
			expectNil:         true,
			navigationHandler: nil,
			setupItems: func(s *ConfigScreen) {
				// No items needed for nil case
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := ScreenConfig{
				ID:                tt.screenID,
				Title:             tt.screenID,
				ResourceType:      k8s.ResourceTypePod, // Doesn't matter for this test
				NavigationHandler: tt.navigationHandler,
				Columns: []ColumnConfig{
					{Field: "Name", Title: "Name", Width: 0},
				},
				SearchFields: []string{"Name"},
			}

			screen := NewConfigScreen(cfg, repo, theme)
			tt.setupItems(screen)

			cmd := screen.handleEnterKey()

			if tt.expectNil {
				assert.Nil(t, cmd, "expected nil cmd for screen %s", tt.screenID)
			} else {
				assert.NotNil(t, cmd, "expected non-nil cmd for screen %s", tt.screenID)

				// Execute the command and verify it returns a ScreenSwitchMsg
				msg := cmd()
				_, ok := msg.(types.ScreenSwitchMsg)
				assert.True(t, ok, "expected ScreenSwitchMsg for screen %s", tt.screenID)
			}
		})
	}
}

// Phase 4 tests: New navigation functions

func TestConfigScreen_NavigateToPodsForStatefulSet(t *testing.T) {
	cfg := ScreenConfig{
		ID:           "statefulsets",
		Title:        "StatefulSets",
		ResourceType: k8s.ResourceTypeStatefulSet,
		Columns: []ColumnConfig{
			{Field: "Name", Title: "Name", Width: 0},
			{Field: "Namespace", Title: "Namespace", Width: 20},
		},
	}

	repo := k8s.NewDummyRepository()
	theme := ui.GetTheme("charm")
	screen := NewConfigScreen(cfg, repo, theme)

	// Set up items
	screen.items = []interface{}{
		k8s.StatefulSet{ResourceMetadata: k8s.ResourceMetadata{Namespace: "default", Name: "web"}},
		k8s.StatefulSet{ResourceMetadata: k8s.ResourceMetadata{Namespace: "prod", Name: "cache"}},
	}
	screen.applyFilter()

	tests := []struct {
		name              string
		cursorPosition    int
		expectedScreenID  string
		expectedField     string
		expectedValue     string
		expectedNamespace string
	}{
		{
			name:              "first statefulset",
			cursorPosition:    0,
			expectedScreenID:  "pods",
			expectedField:     "owner",
			expectedValue:     "web",
			expectedNamespace: "default",
		},
		{
			name:              "second statefulset",
			cursorPosition:    1,
			expectedScreenID:  "pods",
			expectedField:     "owner",
			expectedValue:     "cache",
			expectedNamespace: "prod",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			screen.table.SetCursor(tt.cursorPosition)
			handler := navigateToPodsForOwner("StatefulSet")
			cmd := handler(screen)

			require.NotNil(t, cmd)

			msg := cmd()
			switchMsg, ok := msg.(types.ScreenSwitchMsg)
			require.True(t, ok, "expected ScreenSwitchMsg")

			assert.Equal(t, tt.expectedScreenID, switchMsg.ScreenID)
			require.NotNil(t, switchMsg.FilterContext)
			assert.Equal(t, tt.expectedField, switchMsg.FilterContext.Field)
			assert.Equal(t, tt.expectedValue, switchMsg.FilterContext.Value)
			assert.Equal(t, tt.expectedNamespace, switchMsg.FilterContext.Metadata["namespace"])
			assert.Equal(t, "StatefulSet", switchMsg.FilterContext.Metadata["kind"])
		})
	}
}

func TestConfigScreen_NavigateToPodsForDaemonSet(t *testing.T) {
	cfg := ScreenConfig{
		ID:           "daemonsets",
		Title:        "DaemonSets",
		ResourceType: k8s.ResourceTypeDaemonSet,
		Columns: []ColumnConfig{
			{Field: "Name", Title: "Name", Width: 0},
			{Field: "Namespace", Title: "Namespace", Width: 20},
		},
	}

	repo := k8s.NewDummyRepository()
	theme := ui.GetTheme("charm")
	screen := NewConfigScreen(cfg, repo, theme)

	screen.items = []interface{}{
		k8s.DaemonSet{ResourceMetadata: k8s.ResourceMetadata{Namespace: "kube-system", Name: "fluentd"}},
	}
	screen.applyFilter()

	screen.table.SetCursor(0)
	handler := navigateToPodsForOwner("DaemonSet")
	cmd := handler(screen)

	require.NotNil(t, cmd)

	msg := cmd()
	switchMsg, ok := msg.(types.ScreenSwitchMsg)
	require.True(t, ok, "expected ScreenSwitchMsg")

	assert.Equal(t, "pods", switchMsg.ScreenID)
	require.NotNil(t, switchMsg.FilterContext)
	assert.Equal(t, "owner", switchMsg.FilterContext.Field)
	assert.Equal(t, "fluentd", switchMsg.FilterContext.Value)
	assert.Equal(t, "kube-system", switchMsg.FilterContext.Metadata["namespace"])
	assert.Equal(t, "DaemonSet", switchMsg.FilterContext.Metadata["kind"])
}

func TestConfigScreen_NavigateToPodsForJob(t *testing.T) {
	cfg := ScreenConfig{
		ID:           "jobs",
		Title:        "Jobs",
		ResourceType: k8s.ResourceTypeJob,
		Columns: []ColumnConfig{
			{Field: "Name", Title: "Name", Width: 0},
			{Field: "Namespace", Title: "Namespace", Width: 20},
		},
	}

	repo := k8s.NewDummyRepository()
	theme := ui.GetTheme("charm")
	screen := NewConfigScreen(cfg, repo, theme)

	screen.items = []interface{}{
		k8s.Job{ResourceMetadata: k8s.ResourceMetadata{Namespace: "default", Name: "backup-job"}},
	}
	screen.applyFilter()

	screen.table.SetCursor(0)
	handler := navigateToPodsForOwner("Job")
	cmd := handler(screen)

	require.NotNil(t, cmd)

	msg := cmd()
	switchMsg, ok := msg.(types.ScreenSwitchMsg)
	require.True(t, ok, "expected ScreenSwitchMsg")

	assert.Equal(t, "pods", switchMsg.ScreenID)
	require.NotNil(t, switchMsg.FilterContext)
	assert.Equal(t, "owner", switchMsg.FilterContext.Field)
	assert.Equal(t, "backup-job", switchMsg.FilterContext.Value)
	assert.Equal(t, "default", switchMsg.FilterContext.Metadata["namespace"])
	assert.Equal(t, "Job", switchMsg.FilterContext.Metadata["kind"])
}

func TestConfigScreen_NavigateToJobsForCronJob(t *testing.T) {
	cfg := ScreenConfig{
		ID:           "cronjobs",
		Title:        "CronJobs",
		ResourceType: k8s.ResourceTypeCronJob,
		Columns: []ColumnConfig{
			{Field: "Name", Title: "Name", Width: 0},
			{Field: "Namespace", Title: "Namespace", Width: 20},
		},
	}

	repo := k8s.NewDummyRepository()
	theme := ui.GetTheme("charm")
	screen := NewConfigScreen(cfg, repo, theme)

	screen.items = []interface{}{
		k8s.CronJob{ResourceMetadata: k8s.ResourceMetadata{Namespace: "default", Name: "daily-backup"}},
	}
	screen.applyFilter()

	screen.table.SetCursor(0)
	handler := navigateToJobsForCronJob()
	cmd := handler(screen)

	require.NotNil(t, cmd)

	msg := cmd()
	switchMsg, ok := msg.(types.ScreenSwitchMsg)
	require.True(t, ok, "expected ScreenSwitchMsg")

	assert.Equal(t, "jobs", switchMsg.ScreenID) // Note: navigates to jobs, not pods!
	require.NotNil(t, switchMsg.FilterContext)
	assert.Equal(t, "owner", switchMsg.FilterContext.Field)
	assert.Equal(t, "daily-backup", switchMsg.FilterContext.Value)
	assert.Equal(t, "default", switchMsg.FilterContext.Metadata["namespace"])
	assert.Equal(t, "CronJob", switchMsg.FilterContext.Metadata["kind"])
}

func TestConfigScreen_NavigateToPodsForNamespace(t *testing.T) {
	cfg := ScreenConfig{
		ID:           "namespaces",
		Title:        "Namespaces",
		ResourceType: k8s.ResourceTypeNamespace,
		Columns: []ColumnConfig{
			{Field: "Name", Title: "Name", Width: 0},
		},
	}

	repo := k8s.NewDummyRepository()
	theme := ui.GetTheme("charm")
	screen := NewConfigScreen(cfg, repo, theme)

	screen.items = []interface{}{
		k8s.Namespace{ResourceMetadata: k8s.ResourceMetadata{Name: "kube-system"}},
		k8s.Namespace{ResourceMetadata: k8s.ResourceMetadata{Name: "default"}},
	}
	screen.applyFilter()

	tests := []struct {
		name             string
		cursorPosition   int
		expectedValue    string
		expectedScreenID string
	}{
		{
			name:             "kube-system namespace",
			cursorPosition:   0,
			expectedValue:    "kube-system",
			expectedScreenID: "pods",
		},
		{
			name:             "default namespace",
			cursorPosition:   1,
			expectedValue:    "default",
			expectedScreenID: "pods",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			screen.table.SetCursor(tt.cursorPosition)
			handler := navigateToPodsForNamespace()
			cmd := handler(screen)

			require.NotNil(t, cmd)

			msg := cmd()
			switchMsg, ok := msg.(types.ScreenSwitchMsg)
			require.True(t, ok, "expected ScreenSwitchMsg")

			assert.Equal(t, tt.expectedScreenID, switchMsg.ScreenID)
			require.NotNil(t, switchMsg.FilterContext)
			assert.Equal(t, "namespace", switchMsg.FilterContext.Field)
			assert.Equal(t, tt.expectedValue, switchMsg.FilterContext.Value)
			assert.Equal(t, "Namespace", switchMsg.FilterContext.Metadata["kind"])
		})
	}
}

func TestConfigScreen_NavigateToPodsUsingConfigMap(t *testing.T) {
	cfg := ScreenConfig{
		ID:           "configmaps",
		Title:        "ConfigMaps",
		ResourceType: k8s.ResourceTypeConfigMap,
		Columns: []ColumnConfig{
			{Field: "Name", Title: "Name", Width: 0},
			{Field: "Namespace", Title: "Namespace", Width: 20},
		},
	}

	repo := k8s.NewDummyRepository()
	theme := ui.GetTheme("charm")
	screen := NewConfigScreen(cfg, repo, theme)

	screen.items = []interface{}{
		k8s.ConfigMap{ResourceMetadata: k8s.ResourceMetadata{Namespace: "default", Name: "app-config"}},
	}
	screen.applyFilter()

	screen.table.SetCursor(0)
	handler := navigateToPodsForVolumeSource("ConfigMap")
	cmd := handler(screen)

	require.NotNil(t, cmd)

	msg := cmd()
	switchMsg, ok := msg.(types.ScreenSwitchMsg)
	require.True(t, ok, "expected ScreenSwitchMsg")

	assert.Equal(t, "pods", switchMsg.ScreenID)
	require.NotNil(t, switchMsg.FilterContext)
	assert.Equal(t, "configmap", switchMsg.FilterContext.Field)
	assert.Equal(t, "app-config", switchMsg.FilterContext.Value)
	assert.Equal(t, "default", switchMsg.FilterContext.Metadata["namespace"])
	assert.Equal(t, "ConfigMap", switchMsg.FilterContext.Metadata["kind"])
}

func TestConfigScreen_NavigateToPodsUsingSecret(t *testing.T) {
	cfg := ScreenConfig{
		ID:           "secrets",
		Title:        "Secrets",
		ResourceType: k8s.ResourceTypeSecret,
		Columns: []ColumnConfig{
			{Field: "Name", Title: "Name", Width: 0},
			{Field: "Namespace", Title: "Namespace", Width: 20},
		},
	}

	repo := k8s.NewDummyRepository()
	theme := ui.GetTheme("charm")
	screen := NewConfigScreen(cfg, repo, theme)

	screen.items = []interface{}{
		k8s.Secret{ResourceMetadata: k8s.ResourceMetadata{Namespace: "default", Name: "db-password"}},
	}
	screen.applyFilter()

	screen.table.SetCursor(0)
	handler := navigateToPodsForVolumeSource("Secret")
	cmd := handler(screen)

	require.NotNil(t, cmd)

	msg := cmd()
	switchMsg, ok := msg.(types.ScreenSwitchMsg)
	require.True(t, ok, "expected ScreenSwitchMsg")

	assert.Equal(t, "pods", switchMsg.ScreenID)
	require.NotNil(t, switchMsg.FilterContext)
	assert.Equal(t, "secret", switchMsg.FilterContext.Field)
	assert.Equal(t, "db-password", switchMsg.FilterContext.Value)
	assert.Equal(t, "default", switchMsg.FilterContext.Metadata["namespace"])
	assert.Equal(t, "Secret", switchMsg.FilterContext.Metadata["kind"])
}

func TestConfigScreen_EmptyFilteredView(t *testing.T) {
	cfg := ScreenConfig{
		ID:           "pods",
		Title:        "Pods",
		ResourceType: k8s.ResourceTypePod,
		Columns: []ColumnConfig{
			{Field: "Name", Title: "Name", Width: 0},
			{Field: "Namespace", Title: "Namespace", Width: 20},
		},
	}

	repo := k8s.NewDummyRepository()
	theme := ui.GetTheme("charm")
	screen := NewConfigScreen(cfg, repo, theme)
	screen.SetSize(100, 20)

	// Set filter context but no items (empty results)
	screen.filterContext = &types.FilterContext{
		Field: "owner",
		Value: "nonexistent",
		Metadata: map[string]string{
			"namespace": "default",
			"kind":      "Deployment",
		},
	}
	screen.items = []interface{}{}
	screen.applyFilter()

	// View should show empty message
	view := screen.View()
	assert.Contains(t, view, "No resources found")
	assert.Contains(t, view, "Press ESC to go back")
}

func TestConfigScreen_View_WithResults(t *testing.T) {
	cfg := ScreenConfig{
		ID:           "pods",
		Title:        "Pods",
		ResourceType: k8s.ResourceTypePod,
		Columns: []ColumnConfig{
			{Field: "Name", Title: "Name", Width: 0},
			{Field: "Namespace", Title: "Namespace", Width: 20},
		},
	}

	repo := k8s.NewDummyRepository()
	theme := ui.GetTheme("charm")
	screen := NewConfigScreen(cfg, repo, theme)
	screen.SetSize(100, 20)

	// Set filter context WITH items
	screen.filterContext = &types.FilterContext{
		Field: "owner",
		Value: "nginx",
		Metadata: map[string]string{
			"namespace": "default",
			"kind":      "Deployment",
		},
	}

	screen.items = []interface{}{
		k8s.Pod{ResourceMetadata: k8s.ResourceMetadata{Namespace: "default", Name: "nginx-pod-1"}},
		k8s.Pod{ResourceMetadata: k8s.ResourceMetadata{Namespace: "default", Name: "nginx-pod-2"}},
	}
	screen.applyFilter()

	// View should show normal table (not empty message)
	view := screen.View()
	assert.NotContains(t, view, "No resources found")
}
