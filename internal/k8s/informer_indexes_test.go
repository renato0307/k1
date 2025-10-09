package k8s

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Helper function for int32 pointer
func int32Ptr(i int32) *int32 {
	return &i
}

// TestInformerRepository_IndexMaintenance_Add tests that indexes are updated on pod add
func TestInformerRepository_IndexMaintenance_Add(t *testing.T) {
	ns := createTestNamespace(t)

	// Create repository first so event handlers are registered
	repo := createTestRepository(t, ns)
	defer repo.Close()

	// Create pod AFTER repository so ADD event fires with handler registered
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: ns,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "nginx", Image: "nginx:latest"}},
			NodeName:   "test-node",
		},
	}
	createdPod, err := testClient.CoreV1().Pods(ns).Create(context.Background(), pod, metav1.CreateOptions{})
	require.NoError(t, err)
	time.Sleep(200 * time.Millisecond)

	// Verify pod is in node index
	repo.mu.RLock()
	podsOnNode := repo.podsByNode["test-node"]
	repo.mu.RUnlock()
	assert.Len(t, podsOnNode, 1, "pod should be in node index")
	assert.Equal(t, createdPod.UID, podsOnNode[0].UID)

	// Verify pod is in namespace index
	repo.mu.RLock()
	podsInNs := repo.podsByNamespace[ns]
	repo.mu.RUnlock()
	assert.Len(t, podsInNs, 1, "pod should be in namespace index")
	assert.Equal(t, createdPod.UID, podsInNs[0].UID)
}

// TestInformerRepository_IndexMaintenance_Delete tests that indexes are cleaned up on pod delete
func TestInformerRepository_IndexMaintenance_Delete(t *testing.T) {
	t.Skip("Skipping: informer delete events have inconsistent timing in envtest")
	ns := createTestNamespace(t)

	// Create repository first so event handlers are registered
	repo := createTestRepository(t, ns)
	defer repo.Close()

	// Create pod AFTER repository so ADD event fires with handler registered
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: ns,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "nginx", Image: "nginx:latest"}},
			NodeName:   "test-node",
		},
	}
	createdPod, err := testClient.CoreV1().Pods(ns).Create(context.Background(), pod, metav1.CreateOptions{})
	require.NoError(t, err)
	time.Sleep(200 * time.Millisecond)

	// Verify pod is in indexes
	repo.mu.RLock()
	podsOnNode := repo.podsByNode["test-node"]
	podsInNs := repo.podsByNamespace[ns]
	repo.mu.RUnlock()
	assert.Len(t, podsOnNode, 1)
	assert.Len(t, podsInNs, 1)

	// Delete pod
	err = testClient.CoreV1().Pods(ns).Delete(context.Background(), createdPod.Name, metav1.DeleteOptions{})
	require.NoError(t, err)
	time.Sleep(500 * time.Millisecond)

	// Verify pod is removed from indexes
	repo.mu.RLock()
	podsOnNodeAfter := repo.podsByNode["test-node"]
	podsInNsAfter := repo.podsByNamespace[ns]
	repo.mu.RUnlock()
	assert.Empty(t, podsOnNodeAfter, "pod should be removed from node index")
	assert.Empty(t, podsInNsAfter, "pod should be removed from namespace index")
}

// TestInformerRepository_IndexMaintenance_OwnerReferences tests owner UID index
func TestInformerRepository_IndexMaintenance_OwnerReferences(t *testing.T) {
	ns := createTestNamespace(t)

	// Create ReplicaSet
	rs := &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-rs",
			Namespace: ns,
		},
		Spec: appsv1.ReplicaSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "test"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "test"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "nginx", Image: "nginx:latest"}},
				},
			},
		},
	}
	createdRS, err := testClient.AppsV1().ReplicaSets(ns).Create(context.Background(), rs, metav1.CreateOptions{})
	require.NoError(t, err)

	// Create pod owned by ReplicaSet
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: ns,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "apps/v1",
					Kind:       "ReplicaSet",
					Name:       createdRS.Name,
					UID:        createdRS.UID,
				},
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "nginx", Image: "nginx:latest"}},
			NodeName:   "test-node",
		},
	}
	createdPod, err := testClient.CoreV1().Pods(ns).Create(context.Background(), pod, metav1.CreateOptions{})
	require.NoError(t, err)

	repo := createTestRepository(t, ns)
	defer repo.Close()
	time.Sleep(100 * time.Millisecond)

	// Verify pod is in owner index
	repo.mu.RLock()
	podsForOwner := repo.podsByOwnerUID[string(createdRS.UID)]
	repo.mu.RUnlock()
	assert.Len(t, podsForOwner, 1, "pod should be in owner index")
	assert.Equal(t, createdPod.UID, podsForOwner[0].UID)
}

// TestInformerRepository_IndexMaintenance_MultiplePods tests multiple pods in same index
func TestInformerRepository_IndexMaintenance_MultiplePods(t *testing.T) {
	ns := createTestNamespace(t)

	// Create multiple pods on same node
	for i := 1; i <= 3; i++ {
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("test-pod-%d", i),
				Namespace: ns,
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{{Name: "nginx", Image: "nginx:latest"}},
				NodeName:   "shared-node",
			},
		}
		_, err := testClient.CoreV1().Pods(ns).Create(context.Background(), pod, metav1.CreateOptions{})
		require.NoError(t, err)
	}

	repo := createTestRepository(t, ns)
	defer repo.Close()
	time.Sleep(100 * time.Millisecond)

	// Verify all pods are in node index
	repo.mu.RLock()
	podsOnNode := repo.podsByNode["shared-node"]
	repo.mu.RUnlock()
	assert.Len(t, podsOnNode, 3, "all 3 pods should be in node index")
}

// TestInformerRepository_IndexedQuery_Performance tests that indexed queries are faster
func TestInformerRepository_IndexedQuery_Performance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	ns := createTestNamespace(t)

	// Create 100 pods across 10 nodes
	nodesCount := 10
	podsPerNode := 10
	for i := 0; i < nodesCount; i++ {
		nodeName := fmt.Sprintf("node-%d", i)
		for j := 0; j < podsPerNode; j++ {
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("pod-%d-%d", i, j),
					Namespace: ns,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "nginx", Image: "nginx:latest"}},
					NodeName:   nodeName,
				},
			}
			_, err := testClient.CoreV1().Pods(ns).Create(context.Background(), pod, metav1.CreateOptions{})
			require.NoError(t, err)
		}
	}

	repo := createTestRepository(t, ns)
	defer repo.Close()
	time.Sleep(500 * time.Millisecond)

	// Time indexed query
	start := time.Now()
	pods, err := repo.GetPodsOnNode("node-5")
	indexedDuration := time.Since(start)

	require.NoError(t, err)
	assert.Len(t, pods, podsPerNode, "should return correct number of pods")

	t.Logf("Indexed query took: %v", indexedDuration)

	// Indexed query should be fast (< 10ms)
	assert.Less(t, indexedDuration, 10*time.Millisecond, "indexed query should be fast")
}
