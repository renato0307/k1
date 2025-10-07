package commands

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/renato0307/k1/internal/k8s"
)

func TestCommandContext_GetResourceInfo(t *testing.T) {
	tests := []struct {
		name     string
		ctx      CommandContext
		expected ResourceInfo
	}{
		{
			name: "with name and namespace",
			ctx: CommandContext{
				ResourceType: k8s.ResourceTypePod,
				Selected: map[string]any{
					"Name":      "test-pod",
					"Namespace": "default",
				},
			},
			expected: ResourceInfo{
				Name:      "test-pod",
				Namespace: "default",
				Kind:      k8s.ResourceTypePod,
			},
		},
		{
			name: "without namespace (cluster-scoped)",
			ctx: CommandContext{
				ResourceType: k8s.ResourceTypeNode,
				Selected: map[string]any{
					"Name": "node-1",
				},
			},
			expected: ResourceInfo{
				Name:      "node-1",
				Namespace: "",
				Kind:      k8s.ResourceTypeNode,
			},
		},
		{
			name: "missing fields",
			ctx: CommandContext{
				ResourceType: k8s.ResourceTypeDeployment,
				Selected:     map[string]any{},
			},
			expected: ResourceInfo{
				Name:      "",
				Namespace: "",
				Kind:      k8s.ResourceTypeDeployment,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := tt.ctx.GetResourceInfo()
			assert.Equal(t, tt.expected.Name, info.Name)
			assert.Equal(t, tt.expected.Namespace, info.Namespace)
			assert.Equal(t, tt.expected.Kind, info.Kind)
		})
	}
}

func TestCommandContext_ParseArgs(t *testing.T) {
	t.Run("successful parse", func(t *testing.T) {
		ctx := CommandContext{
			ResourceType: k8s.ResourceTypePod,
			Selected:     map[string]any{},
			Args:         "3",
		}

		var args ScaleArgs
		err := ctx.ParseArgs(&args)
		assert.NoError(t, err)
		assert.Equal(t, 3, args.Replicas)
	})

	t.Run("parse error", func(t *testing.T) {
		ctx := CommandContext{
			ResourceType: k8s.ResourceTypePod,
			Selected:     map[string]any{},
			Args:         "invalid",
		}

		var args ScaleArgs
		err := ctx.ParseArgs(&args)
		assert.Error(t, err)
	})

	t.Run("nil dest", func(t *testing.T) {
		ctx := CommandContext{
			ResourceType: k8s.ResourceTypePod,
			Selected:     map[string]any{},
			Args:         "",
		}

		err := ctx.ParseArgs(nil)
		assert.NoError(t, err)
	})
}
