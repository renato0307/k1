package commands

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/renato0307/k1/internal/k8s"
	"github.com/renato0307/k1/internal/k8s/dummy"
)

func TestNewRegistry(t *testing.T) {
	formatter := dummy.NewFormatter()
	provider := dummy.NewManager()
	registry := NewRegistry(formatter, provider)

	require.NotNil(t, registry)
	assert.NotEmpty(t, registry.commands)

	// Should have navigation commands
	hasPodsCommand := false
	hasDeploymentsCommand := false
	for _, cmd := range registry.commands {
		if cmd.Name == "pods" {
			hasPodsCommand = true
		}
		if cmd.Name == "deployments" {
			hasDeploymentsCommand = true
		}
	}
	assert.True(t, hasPodsCommand, "registry should have pods command")
	assert.True(t, hasDeploymentsCommand, "registry should have deployments command")
}

func TestRegistry_GetByCategory(t *testing.T) {
	formatter := dummy.NewFormatter()
	provider := dummy.NewManager()
	registry := NewRegistry(formatter, provider)

	tests := []struct {
		name     string
		category CommandCategory
		expect   func([]Command)
	}{
		{
			name:     "resource commands",
			category: CategoryResource,
			expect: func(cmds []Command) {
				assert.NotEmpty(t, cmds)
				// Should include navigation commands like pods, deployments
				found := false
				for _, cmd := range cmds {
					if cmd.Name == "pods" {
						found = true
						break
					}
				}
				assert.True(t, found, "should include pods command")
			},
		},
		{
			name:     "action commands",
			category: CategoryAction,
			expect: func(cmds []Command) {
				assert.NotEmpty(t, cmds)
				// Should include action commands like yaml, describe
				found := false
				for _, cmd := range cmds {
					if cmd.Name == "yaml" {
						found = true
						break
					}
				}
				assert.True(t, found, "should include yaml command")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmds := registry.GetByCategory(tt.category)
			tt.expect(cmds)
		})
	}
}

func TestRegistry_Filter(t *testing.T) {
	formatter := dummy.NewFormatter()
	provider := dummy.NewManager()
	registry := NewRegistry(formatter, provider)

	tests := []struct {
		name     string
		query    string
		category CommandCategory
		expect   func([]Command)
	}{
		{
			name:     "filter pods",
			query:    "pod",
			category: CategoryResource,
			expect: func(cmds []Command) {
				assert.NotEmpty(t, cmds)
				for _, cmd := range cmds {
					// Should match pods
					if cmd.Name == "pods" {
						return
					}
				}
				t.Error("expected to find pods command")
			},
		},
		{
			name:     "filter yaml",
			query:    "yam",
			category: CategoryAction,
			expect: func(cmds []Command) {
				assert.NotEmpty(t, cmds)
				for _, cmd := range cmds {
					if cmd.Name == "yaml" {
						return
					}
				}
				t.Error("expected to find yaml command")
			},
		},
		{
			name:     "empty query returns all",
			query:    "",
			category: CategoryResource,
			expect: func(cmds []Command) {
				assert.NotEmpty(t, cmds)
				// Should return all resource commands
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmds := registry.Filter(tt.query, tt.category)
			tt.expect(cmds)
		})
	}
}

func TestRegistry_FilterByResourceType(t *testing.T) {
	formatter := dummy.NewFormatter()
	provider := dummy.NewManager()
	registry := NewRegistry(formatter, provider)

	allActions := registry.GetByCategory(CategoryAction)

	tests := []struct {
		name         string
		resourceType k8s.ResourceType
		expect       func([]Command)
	}{
		{
			name:         "filter by pods",
			resourceType: k8s.ResourceTypePod,
			expect: func(cmds []Command) {
				assert.NotEmpty(t, cmds)
				// Should include pod-specific commands like logs, shell
				hasLogs := false
				for _, cmd := range cmds {
					if cmd.Name == "logs" {
						hasLogs = true
						break
					}
				}
				assert.True(t, hasLogs, "should include logs command for pods")
			},
		},
		{
			name:         "filter by deployments",
			resourceType: k8s.ResourceTypeDeployment,
			expect: func(cmds []Command) {
				assert.NotEmpty(t, cmds)
				// Should include deployment-specific commands like scale, restart
				hasScale := false
				for _, cmd := range cmds {
					if cmd.Name == "scale" {
						hasScale = true
						break
					}
				}
				assert.True(t, hasScale, "should include scale command for deployments")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmds := registry.FilterByResourceType(allActions, tt.resourceType)
			tt.expect(cmds)
		})
	}
}

func TestRegistry_Get(t *testing.T) {
	formatter := dummy.NewFormatter()
	provider := dummy.NewManager()
	registry := NewRegistry(formatter, provider)

	tests := []struct {
		name     string
		cmdName  string
		category CommandCategory
		found    bool
	}{
		{
			name:     "get existing resource command",
			cmdName:  "pods",
			category: CategoryResource,
			found:    true,
		},
		{
			name:     "get existing action command",
			cmdName:  "yaml",
			category: CategoryAction,
			found:    true,
		},
		{
			name:     "get missing command",
			cmdName:  "nonexistent",
			category: CategoryResource,
			found:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := registry.Get(tt.cmdName, tt.category)
			if tt.found {
				assert.NotNil(t, cmd)
				assert.Equal(t, tt.cmdName, cmd.Name)
			} else {
				assert.Nil(t, cmd)
			}
		})
	}
}
