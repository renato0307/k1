package commands

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/renato0307/k1/internal/k8s"
	"github.com/renato0307/k1/internal/types"
)

// mockRepository for testing commands without real kubectl
type mockRepository struct {
	kubeconfig string
	context    string
}

func (m *mockRepository) GetKubeconfig() string { return m.kubeconfig }
func (m *mockRepository) GetContext() string    { return m.context }

// Implement k8s.Repository interface
func (m *mockRepository) GetResources(resourceType k8s.ResourceType) ([]any, error) {
	return nil, nil
}
func (m *mockRepository) GetPods() ([]k8s.Pod, error) {
	return nil, nil
}
func (m *mockRepository) GetDeployments() ([]k8s.Deployment, error) {
	return nil, nil
}
func (m *mockRepository) GetServices() ([]k8s.Service, error) {
	return nil, nil
}
func (m *mockRepository) GetPodsForDeployment(namespace, name string) ([]k8s.Pod, error) {
	return nil, nil
}
func (m *mockRepository) GetPodsOnNode(nodeName string) ([]k8s.Pod, error) {
	return nil, nil
}
func (m *mockRepository) GetPodsForService(namespace, name string) ([]k8s.Pod, error) {
	return nil, nil
}
func (m *mockRepository) GetPodsForStatefulSet(namespace, name string) ([]k8s.Pod, error) {
	return nil, nil
}
func (m *mockRepository) GetPodsForDaemonSet(namespace, name string) ([]k8s.Pod, error) {
	return nil, nil
}
func (m *mockRepository) GetPodsForJob(namespace, name string) ([]k8s.Pod, error) {
	return nil, nil
}
func (m *mockRepository) GetJobsForCronJob(namespace, name string) ([]k8s.Job, error) {
	return nil, nil
}
func (m *mockRepository) GetPodsForNamespace(namespace string) ([]k8s.Pod, error) {
	return nil, nil
}
func (m *mockRepository) GetPodsUsingConfigMap(namespace, name string) ([]k8s.Pod, error) {
	return nil, nil
}
func (m *mockRepository) GetPodsUsingSecret(namespace, name string) ([]k8s.Pod, error) {
	return nil, nil
}
func (m *mockRepository) GetPodsForReplicaSet(namespace, name string) ([]k8s.Pod, error) {
	return nil, nil
}
func (m *mockRepository) GetReplicaSetsForDeployment(namespace, name string) ([]k8s.ReplicaSet, error) {
	return nil, nil
}
func (m *mockRepository) GetPodsForPVC(namespace, name string) ([]k8s.Pod, error) {
	return nil, nil
}
func (m *mockRepository) GetResourceYAML(gvr schema.GroupVersionResource, namespace, name string) (string, error) {
	return "", nil
}
func (m *mockRepository) DescribeResource(gvr schema.GroupVersionResource, namespace, name string) (string, error) {
	return "", nil
}
func (m *mockRepository) GetResourceStats() []k8s.ResourceStats {
	return nil
}
func (m *mockRepository) Close() {}
func (m *mockRepository) EnsureCRInformer(gvr schema.GroupVersionResource) error {
	return nil
}
func (m *mockRepository) GetResourcesByGVR(gvr schema.GroupVersionResource, transform k8s.TransformFunc) ([]any, error) {
	return nil, nil
}
func (m *mockRepository) IsInformerSynced(gvr schema.GroupVersionResource) bool {
	return true
}
func (m *mockRepository) AreTypedInformersReady() bool {
	return true
}
func (m *mockRepository) GetTypedInformersSyncError() error {
	return nil
}
func (m *mockRepository) GetDynamicInformerSyncError(gvr schema.GroupVersionResource) error {
	return nil
}
func (m *mockRepository) EnsureResourceTypeInformer(resourceType k8s.ResourceType) error {
	return nil
}

// newTestRepositoryPool creates a test pool with a mock repository
// Uses the repository_pool's SetTestRepository which properly initializes the pool
func newTestRepositoryPool(repo k8s.Repository) *k8s.RepositoryPool {
	// Create an empty pool struct
	// The fields repos, lru are private, but SetTestRepository is designed for tests
	// and will initialize them properly
	pool := new(k8s.RepositoryPool)
	pool.SetTestRepository("test-context", repo)
	return pool
}

// Context management methods (stub implementations for testing)
func (m *mockRepository) SwitchContext(contextName string, progress chan<- k8s.ContextLoadProgress) error {
	return nil
}
func (m *mockRepository) GetAllContexts() []k8s.ContextWithStatus {
	return []k8s.ContextWithStatus{}
}
func (m *mockRepository) GetActiveContext() string {
	return m.context
}
func (m *mockRepository) RetryFailedContext(contextName string, progress chan<- k8s.ContextLoadProgress) error {
	return nil
}
func (m *mockRepository) GetContexts() ([]k8s.Context, error) {
	return []k8s.Context{}, nil
}

func TestScaleCommand_ArgParsing(t *testing.T) {
	repo := &mockRepository{}
	scaleCmd := ScaleCommand(newTestRepositoryPool(repo))

	tests := []struct {
		name      string
		argString string
		wantErr   bool
	}{
		{
			name:      "valid replicas",
			argString: "3",
			wantErr:   false,
		},
		{
			name:      "zero replicas",
			argString: "0",
			wantErr:   false,
		},
		{
			name:      "empty args",
			argString: "",
			wantErr:   true, // Required field
		},
		{
			name:      "invalid number",
			argString: "notanumber",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := CommandContext{
				ResourceType: k8s.ResourceTypePod,
				Selected: map[string]any{
					"name":      "test-pod",
					"namespace": "default",
				},
				Args: tt.argString,
			}

			cmd := scaleCmd(ctx)
			require.NotNil(t, cmd)

			// Execute the command to check if it errors
			msg := cmd()
			if tt.wantErr {
				// Should return error status
				statusMsg, ok := msg.(types.StatusMsg)
				assert.True(t, ok, "expected StatusMsg")
				if ok {
					assert.Equal(t, types.MessageTypeError, statusMsg.Type)
				}
			}
			// Note: We can't test success case without kubectl available
		})
	}
}

func TestRestartCommand_BasicFlow(t *testing.T) {
	repo := &mockRepository{
		kubeconfig: "/path/to/kubeconfig",
		context:    "test-context",
	}
	restartCmd := RestartCommand(newTestRepositoryPool(repo))

	ctx := CommandContext{
		ResourceType: k8s.ResourceTypeDeployment,
		Selected: map[string]any{
			"name":      "nginx",
			"namespace": "default",
		},
		Args: "",
	}

	cmd := restartCmd(ctx)
	require.NotNil(t, cmd)

	// Execute the command
	msg := cmd()
	require.NotNil(t, msg)

	// Should return a status message (error if kubectl not available)
	statusMsg, ok := msg.(types.StatusMsg)
	assert.True(t, ok, "expected StatusMsg")
	if ok {
		t.Logf("Restart command status: %s (type: %d)", statusMsg.Message, statusMsg.Type)
	}
}

func TestDeleteCommand_BasicFlow(t *testing.T) {
	repo := &mockRepository{}
	deleteCmd := DeleteCommand(newTestRepositoryPool(repo))

	tests := []struct {
		name         string
		resourceType k8s.ResourceType
		selected     map[string]any
	}{
		{
			name:         "namespaced resource",
			resourceType: k8s.ResourceTypePod,
			selected: map[string]any{
				"name":      "test-pod",
				"namespace": "default",
			},
		},
		{
			name:         "cluster-scoped resource",
			resourceType: k8s.ResourceTypeNode,
			selected: map[string]any{
				"name": "node-1",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := CommandContext{
				ResourceType: tt.resourceType,
				Selected:     tt.selected,
				Args:         "",
			}

			cmd := deleteCmd(ctx)
			require.NotNil(t, cmd)

			// Execute the command
			msg := cmd()
			require.NotNil(t, msg)

			// Should return a status message
			statusMsg, ok := msg.(types.StatusMsg)
			assert.True(t, ok, "expected StatusMsg")
			if ok {
				t.Logf("Delete command status: %s", statusMsg.Message)
			}
		})
	}
}

func TestScaleCommand_ContextHandling(t *testing.T) {
	repo := &mockRepository{
		kubeconfig: "/custom/kubeconfig",
		context:    "prod-cluster",
	}
	scaleCmd := ScaleCommand(newTestRepositoryPool(repo))

	ctx := CommandContext{
		ResourceType: k8s.ResourceTypeDeployment,
		Selected: map[string]any{
			"name":      "app",
			"namespace": "production",
		},
		Args: "5",
	}

	cmd := scaleCmd(ctx)
	require.NotNil(t, cmd)

	// The command should be created successfully
	// (actual execution will fail without kubectl, but that's expected)
	assert.NotNil(t, cmd)
}

func TestRestartCommand_MissingResource(t *testing.T) {
	repo := &mockRepository{}
	restartCmd := RestartCommand(newTestRepositoryPool(repo))

	ctx := CommandContext{
		ResourceType: k8s.ResourceTypeDeployment,
		Selected:     map[string]any{}, // Missing name and namespace
		Args:         "",
	}

	cmd := restartCmd(ctx)
	require.NotNil(t, cmd)

	// Execute - should use "unknown" as fallback
	msg := cmd()
	require.NotNil(t, msg)

	statusMsg, ok := msg.(types.StatusMsg)
	assert.True(t, ok)
	if ok {
		// Should mention "unknown" in the message
		assert.Contains(t, statusMsg.Message, "unknown")
	}
}
