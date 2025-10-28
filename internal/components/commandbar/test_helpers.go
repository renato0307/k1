package commandbar

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/renato0307/k1/internal/k8s"
	"github.com/stretchr/testify/require"
)

// createTestPool creates a repository pool for testing
func createTestPool(t *testing.T) *k8s.RepositoryPool {
	t.Helper()

	// Create a temporary kubeconfig for testing
	kubeconfigPath := filepath.Join(t.TempDir(), "kubeconfig")
	kubeconfigContent := `apiVersion: v1
kind: Config
clusters:
- cluster:
    server: https://localhost:6443
  name: test-cluster
contexts:
- context:
    cluster: test-cluster
    user: test-user
  name: test-context
current-context: test-context
users:
- name: test-user
  user:
    token: test-token
`
	err := os.WriteFile(kubeconfigPath, []byte(kubeconfigContent), 0600)
	require.NoError(t, err, "Failed to create test kubeconfig")

	// Create pool
	pool, err := k8s.NewRepositoryPool(kubeconfigPath, 10)
	require.NoError(t, err, "Failed to create repository pool")

	// For tests, manually set a dummy repository to avoid API server connection
	// This is a workaround - in production, contexts are loaded via LoadContext
	pool.SetTestRepository("test-context", k8s.NewDummyRepository())

	return pool
}
