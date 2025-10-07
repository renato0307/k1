package screens

import (
	"testing"
	"time"

	"github.com/renato0307/k1/internal/k8s"
	"github.com/renato0307/k1/internal/k8s/dummy"
	"github.com/renato0307/k1/internal/types"
	"github.com/renato0307/k1/internal/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createTestAppContext creates an AppContext for testing
func createTestAppContext() *types.AppContext {
	dummyManager := dummy.NewManager()
	return types.NewAppContext(
		ui.GetTheme("charm"),
		dummy.NewDataRepository(),
		dummy.NewFormatter(),
		dummyManager,
	)
}

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

	ctx := createTestAppContext()

	screen := NewConfigScreen(ctx, cfg)

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

	ctx := createTestAppContext()
	screen := NewConfigScreen(ctx, cfg)

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

	ctx := createTestAppContext()
	screen := NewConfigScreen(ctx, cfg)

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

	ctx := createTestAppContext()
	screen := NewConfigScreen(ctx, cfg)

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

	ctx := createTestAppContext()
	screen := NewConfigScreen(ctx, cfg)

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

	ctx := createTestAppContext()
	screen := NewConfigScreen(ctx, cfg)

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

	ctx := createTestAppContext()
	screen := NewConfigScreen(ctx, cfg)

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

	ctx := createTestAppContext()
	screen := NewConfigScreen(ctx, cfg)

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

	ctx := createTestAppContext()
	screen := NewConfigScreen(ctx, cfg)
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
			item:      k8s.Pod{Name: "test-pod", Namespace: "default"},
			fieldName: "Name",
			expected:  "test-pod",
		},
		{
			name:      "valid pod namespace",
			item:      k8s.Pod{Name: "test-pod", Namespace: "default"},
			fieldName: "Namespace",
			expected:  "default",
		},
		{
			name:      "non-existent field returns empty string",
			item:      k8s.Pod{Name: "test-pod"},
			fieldName: "NonExistent",
			expected:  "",
		},
		{
			name:      "deployment ready field",
			item:      k8s.Deployment{Name: "deploy1", Ready: "2/2"},
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
