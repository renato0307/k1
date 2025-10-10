package commands

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/renato0307/k1/internal/k8s"
	"github.com/renato0307/k1/internal/types"
)

// TestContextCommand tests the ContextCommand function
func TestContextCommand(t *testing.T) {
	tests := []struct {
		name        string
		args        string
		expectError bool
		validate    func(t *testing.T, msg any)
	}{
		{
			name:        "valid context name",
			args:        "production",
			expectError: false,
			validate: func(t *testing.T, result any) {
				msg, ok := result.(types.ContextSwitchMsg)
				require.True(t, ok, "Expected ContextSwitchMsg")
				assert.Equal(t, "production", msg.ContextName)
			},
		},
		{
			name:        "valid context name with dashes",
			args:        "my-cluster-prod",
			expectError: false,
			validate: func(t *testing.T, result any) {
				msg, ok := result.(types.ContextSwitchMsg)
				require.True(t, ok, "Expected ContextSwitchMsg")
				assert.Equal(t, "my-cluster-prod", msg.ContextName)
			},
		},
		{
			name:        "valid context name with underscores",
			args:        "my_cluster_dev",
			expectError: false,
			validate: func(t *testing.T, result any) {
				msg, ok := result.(types.ContextSwitchMsg)
				require.True(t, ok, "Expected ContextSwitchMsg")
				assert.Equal(t, "my_cluster_dev", msg.ContextName)
			},
		},
		{
			name:        "empty context name returns error",
			args:        "",
			expectError: true,
			validate:    nil,
		},
		{
			name:        "whitespace context name returns error",
			args:        "   ",
			expectError: true,
			validate:    nil,
			// strings.Fields trims whitespace, making this effectively empty,
			// which triggers the required field error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// ContextCommand doesn't actually use the pool parameter,
			// so we can pass nil (but need actual pool for type safety)
			pool := createTestRepositoryPool(t)
			defer pool.Close()

			// Create command
			cmd := ContextCommand(pool)
			require.NotNil(t, cmd)

			// Create context
			ctx := CommandContext{
				Args: tt.args,
			}

			// Execute command
			resultCmd := cmd(ctx)

			if tt.expectError {
				// Command should return error status message
				require.NotNil(t, resultCmd)
				result := resultCmd()
				msg, ok := result.(types.StatusMsg)
				require.True(t, ok, "Expected StatusMsg for error case")
				assert.Equal(t, types.MessageTypeError, msg.Type)
			} else {
				// Command should return success
				require.NotNil(t, resultCmd)
				result := resultCmd()
				require.NotNil(t, result)
				if tt.validate != nil {
					tt.validate(t, result)
				}
			}
		})
	}
}

// createTestRepositoryPool creates a minimal repository pool for testing
func createTestRepositoryPool(t *testing.T) *k8s.RepositoryPool {
	t.Helper()

	// Create minimal kubeconfig
	config := clientcmdapi.NewConfig()
	config.Clusters["test-cluster"] = &clientcmdapi.Cluster{
		Server: "https://localhost:6443",
	}
	config.AuthInfos["test-user"] = &clientcmdapi.AuthInfo{
		Token: "test-token",
	}
	config.Contexts["test-context"] = &clientcmdapi.Context{
		Cluster:   "test-cluster",
		AuthInfo:  "test-user",
		Namespace: "default",
	}
	config.CurrentContext = "test-context"

	tmpDir := t.TempDir()
	kubeconfigPath := filepath.Join(tmpDir, "kubeconfig")
	err := clientcmd.WriteToFile(*config, kubeconfigPath)
	require.NoError(t, err)

	pool, err := k8s.NewRepositoryPool(kubeconfigPath, 5)
	require.NoError(t, err)

	return pool
}
