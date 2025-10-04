package k8s

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
)

// createTestNamespace creates a unique namespace for test isolation
func createTestNamespace(t *testing.T) string {
	t.Helper()

	// Create unique namespace name based on test name and timestamp
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "test-",
		},
	}

	created, err := testClient.CoreV1().Namespaces().Create(
		context.Background(), ns, metav1.CreateOptions{})
	require.NoError(t, err, "Failed to create test namespace")

	// Cleanup namespace when test completes
	t.Cleanup(func() {
		_ = testClient.CoreV1().Namespaces().Delete(
			context.Background(),
			created.Name,
			metav1.DeleteOptions{},
		)
	})

	return created.Name
}

func TestInformerRepository_Init(t *testing.T) {
	// Verify shared test environment is initialized
	assert.NotNil(t, testCfg, "testCfg should be initialized by TestMain")
	assert.NotNil(t, testClient, "testClient should be initialized by TestMain")

	t.Log("Shared envtest environment initialized successfully")
}

func TestInformerRepository_GetPods_EmptyCluster(t *testing.T) {
	// Create unique namespace for test isolation
	ns := createTestNamespace(t)

	// Create a repository using the shared test config
	repo := createTestRepository(t, ns)
	defer repo.Close()

	// Query pods - should be empty (no pods in this namespace)
	pods, err := repo.GetPods()
	require.NoError(t, err, "GetPods failed")
	assert.Empty(t, pods, "Expected no pods in empty namespace")
}

func TestInformerRepository_GetPods_PodStates(t *testing.T) {
	tests := []struct {
		name            string
		podName         string
		containerName   string
		phase           corev1.PodPhase
		containerReady  bool
		restartCount    int32
		nodeIP          string
		expectedStatus  string
		expectedReady   string
		expectedRestarts int32
		expectedNode    string
		expectedIP      string
	}{
		{
			name:            "running pod with ready container",
			podName:         "test-pod",
			containerName:   "nginx",
			phase:           corev1.PodRunning,
			containerReady:  true,
			restartCount:    0,
			nodeIP:          "10.0.0.1",
			expectedStatus:  "Running",
			expectedReady:   "1/1",
			expectedRestarts: 0,
			expectedNode:    "test-node",
			expectedIP:      "10.0.0.1",
		},
		{
			name:            "crash loop pod with high restarts",
			podName:         "crash-pod",
			containerName:   "failing-app",
			phase:           corev1.PodRunning,
			containerReady:  false,
			restartCount:    15,
			nodeIP:          "10.0.0.2",
			expectedStatus:  "Running",
			expectedReady:   "0/1",
			expectedRestarts: 15,
			expectedNode:    "test-node",
			expectedIP:      "10.0.0.2",
		},
		{
			name:            "pending pod",
			podName:         "pending-pod",
			containerName:   "app",
			phase:           corev1.PodPending,
			containerReady:  false,
			restartCount:    0,
			nodeIP:          "",
			expectedStatus:  "Pending",
			expectedReady:   "0/1",
			expectedRestarts: 0,
			expectedNode:    "test-node",
			expectedIP:      "",
		},
		{
			name:            "pod with multiple restarts",
			podName:         "restart-pod",
			containerName:   "app",
			phase:           corev1.PodRunning,
			containerReady:  true,
			restartCount:    3,
			nodeIP:          "10.0.0.3",
			expectedStatus:  "Running",
			expectedReady:   "1/1",
			expectedRestarts: 3,
			expectedNode:    "test-node",
			expectedIP:      "10.0.0.3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create unique namespace for test isolation
			ns := createTestNamespace(t)

			// Create pod spec
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      tt.podName,
					Namespace: ns,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  tt.containerName,
							Image: "test:latest",
						},
					},
					NodeName: "test-node",
				},
			}

			createdPod, err := testClient.CoreV1().Pods(ns).Create(context.Background(), pod, metav1.CreateOptions{})
			require.NoError(t, err, "Failed to create pod")

			// Update pod status (envtest doesn't have controllers)
			createdPod.Status = corev1.PodStatus{
				Phase: tt.phase,
				PodIP: tt.nodeIP,
				ContainerStatuses: []corev1.ContainerStatus{
					{
						Name:         tt.containerName,
						Ready:        tt.containerReady,
						RestartCount: tt.restartCount,
					},
				},
			}

			_, err = testClient.CoreV1().Pods(ns).UpdateStatus(context.Background(), createdPod, metav1.UpdateOptions{})
			require.NoError(t, err, "Failed to update pod status")

			// Create repository and wait for sync
			repo := createTestRepository(t, ns)
			defer repo.Close()

			time.Sleep(100 * time.Millisecond)

			// Query pods
			pods, err := repo.GetPods()
			require.NoError(t, err, "GetPods failed")
			require.Len(t, pods, 1, "Expected 1 pod")

			// Verify pod fields
			gotPod := pods[0]
			assert.Equal(t, tt.podName, gotPod.Name, "pod name")
			assert.Equal(t, ns, gotPod.Namespace, "namespace")
			assert.Equal(t, tt.expectedStatus, gotPod.Status, "status")
			assert.Equal(t, tt.expectedReady, gotPod.Ready, "ready")
			assert.Equal(t, tt.expectedRestarts, gotPod.Restarts, "restarts")
			assert.Equal(t, tt.expectedNode, gotPod.Node, "node")
			assert.Equal(t, tt.expectedIP, gotPod.IP, "IP")
		})
	}
}

func TestInformerRepository_GetPods_SortByAge(t *testing.T) {
	// Create unique namespace for test isolation
	ns := createTestNamespace(t)

	now := time.Now()

	// Create pods with different ages
	pods := []*corev1.Pod{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "old-pod",
				Namespace:         ns,
				CreationTimestamp: metav1.NewTime(now.Add(-1 * time.Hour)),
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{{Name: "c1", Image: "img"}},
			},
			Status: corev1.PodStatus{Phase: corev1.PodRunning},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "new-pod",
				Namespace:         ns,
				CreationTimestamp: metav1.NewTime(now.Add(-10 * time.Minute)),
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{{Name: "c1", Image: "img"}},
			},
			Status: corev1.PodStatus{Phase: corev1.PodRunning},
		},
	}

	for _, pod := range pods {
		_, err := testClient.CoreV1().Pods(ns).Create(context.Background(), pod, metav1.CreateOptions{})
		require.NoError(t, err, "Failed to create pod")
	}

	repo := createTestRepository(t, ns)
	defer repo.Close()

	time.Sleep(100 * time.Millisecond)

	retrievedPods, err := repo.GetPods()
	require.NoError(t, err, "GetPods failed")
	require.Len(t, retrievedPods, 2, "Expected 2 pods")

	// Should be sorted by age (newest first)
	assert.Equal(t, "new-pod", retrievedPods[0].Name, "First pod should be newest")
	assert.Equal(t, "old-pod", retrievedPods[1].Name, "Second pod should be oldest")
}

func TestInformerRepository_Close(t *testing.T) {
	// Create unique namespace for test isolation
	ns := createTestNamespace(t)

	repo := createTestRepository(t, ns)

	// Close should not panic
	assert.NotPanics(t, func() {
		repo.Close()
	}, "First Close should not panic")

	// Calling Close again should be safe
	assert.NotPanics(t, func() {
		repo.Close()
	}, "Second Close should not panic")
}

// createTestRepository creates an InformerRepository using the shared test config
// and scoped to a specific namespace for test isolation
func createTestRepository(t *testing.T, namespace string) *InformerRepository {
	t.Helper()

	// Create namespace-scoped informer factory for test isolation
	factory := informers.NewSharedInformerFactoryWithOptions(
		testClient,
		30*time.Second,
		informers.WithNamespace(namespace), // Scope to test namespace
	)

	// Create pod informer
	podInformer := factory.Core().V1().Pods().Informer()
	podLister := factory.Core().V1().Pods().Lister()

	// Create context for lifecycle management
	ctx, cancel := context.WithCancel(context.Background())

	// Start informers in background
	factory.Start(ctx.Done())

	// Wait for cache to sync
	synced := cache.WaitForCacheSync(ctx.Done(), podInformer.HasSynced)
	if !synced {
		cancel()
	}
	require.True(t, synced, "Failed to sync pod cache")

	return &InformerRepository{
		clientset: testClient,
		factory:   factory,
		podLister: podLister,
		ctx:       ctx,
		cancel:    cancel,
	}
}
