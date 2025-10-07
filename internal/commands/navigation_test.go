package commands

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/renato0307/k1/internal/k8s"
	"github.com/renato0307/k1/internal/types"
)

func TestNavigationCommand(t *testing.T) {
	tests := []struct {
		name     string
		screenID string
	}{
		{"pods", "pods"},
		{"deployments", "deployments"},
		{"services", "services"},
		{"nodes", "nodes"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			navCmd := NavigationCommand(tt.screenID)
			require.NotNil(t, navCmd)

			ctx := CommandContext{
				ResourceType: k8s.ResourceTypePod,
				Selected:     map[string]any{},
				Args:         "",
			}

			cmd := navCmd(ctx)
			require.NotNil(t, cmd)

			msg := cmd()
			require.NotNil(t, msg)

			switchMsg, ok := msg.(types.ScreenSwitchMsg)
			assert.True(t, ok, "expected ScreenSwitchMsg")
			assert.Equal(t, tt.screenID, switchMsg.ScreenID)
		})
	}
}

func TestLegacyNavigationCommands(t *testing.T) {
	tests := []struct {
		name     string
		cmdFunc  func() ExecuteFunc
		expected string
	}{
		{"PodsCommand", PodsCommand, "pods"},
		{"DeploymentsCommand", DeploymentsCommand, "deployments"},
		{"ServicesCommand", ServicesCommand, "services"},
		{"ConfigMapsCommand", ConfigMapsCommand, "configmaps"},
		{"SecretsCommand", SecretsCommand, "secrets"},
		{"NamespacesCommand", NamespacesCommand, "namespaces"},
		{"StatefulSetsCommand", StatefulSetsCommand, "statefulsets"},
		{"DaemonSetsCommand", DaemonSetsCommand, "daemonsets"},
		{"JobsCommand", JobsCommand, "jobs"},
		{"CronJobsCommand", CronJobsCommand, "cronjobs"},
		{"NodesCommand", NodesCommand, "nodes"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			execFunc := tt.cmdFunc()
			require.NotNil(t, execFunc)

			ctx := CommandContext{
				ResourceType: k8s.ResourceTypePod,
				Selected:     map[string]any{},
				Args:         "",
			}

			cmd := execFunc(ctx)
			require.NotNil(t, cmd)

			msg := cmd()
			require.NotNil(t, msg)

			switchMsg, ok := msg.(types.ScreenSwitchMsg)
			assert.True(t, ok, "expected ScreenSwitchMsg")
			assert.Equal(t, tt.expected, switchMsg.ScreenID)
		})
	}
}

func TestNamespaceFilterCommand(t *testing.T) {
	nsFilterCmd := NamespaceFilterCommand()
	require.NotNil(t, nsFilterCmd)

	ctx := CommandContext{
		ResourceType: k8s.ResourceTypePod,
		Selected:     map[string]any{},
		Args:         "",
	}

	cmd := nsFilterCmd(ctx)
	require.NotNil(t, cmd)

	msg := cmd()
	require.NotNil(t, msg)

	// Should return an info status message
	statusMsg, ok := msg.(types.StatusMsg)
	assert.True(t, ok, "expected StatusMsg")
	if ok {
		assert.Equal(t, types.MessageTypeInfo, statusMsg.Type)
		assert.Contains(t, statusMsg.Message, "Namespace filtering")
	}
}
