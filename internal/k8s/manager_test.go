package k8s

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestNewInformerManager_WithInvalidConfig(t *testing.T) {
	// Test with non-existent kubeconfig file
	_, err := NewInformerManager("/nonexistent/kubeconfig", "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error building kubeconfig")
}

func TestInformerManager_GetKubeconfig(t *testing.T) {
	// Create manager using test config (from integration_test.go TestMain)
	if testCfg == nil {
		t.Skip("Skipping test - envtest not initialized")
	}

	ns := createTestNamespace(t)
	comp := createTestRepository(t, ns)
	defer comp.manager.Close()

	// The test config doesn't use a file path, so this should be empty
	kubeconfig := comp.manager.GetKubeconfig()
	assert.NotNil(t, kubeconfig) // Can be empty in test env
}

func TestInformerManager_GetContext(t *testing.T) {
	if testCfg == nil {
		t.Skip("Skipping test - envtest not initialized")
	}

	ns := createTestNamespace(t)
	comp := createTestRepository(t, ns)
	defer comp.manager.Close()

	context := comp.manager.GetContext()
	assert.NotNil(t, context) // Can be empty in test env
}

func TestInformerManager_GetPodLister(t *testing.T) {
	if testCfg == nil {
		t.Skip("Skipping test - envtest not initialized")
	}

	ns := createTestNamespace(t)
	comp := createTestRepository(t, ns)
	defer comp.manager.Close()

	lister := comp.manager.GetPodLister()
	require.NotNil(t, lister, "Pod lister should not be nil")
}

func TestInformerManager_GetDeploymentLister(t *testing.T) {
	if testCfg == nil {
		t.Skip("Skipping test - envtest not initialized")
	}

	ns := createTestNamespace(t)
	comp := createTestRepository(t, ns)
	defer comp.manager.Close()

	lister := comp.manager.GetDeploymentLister()
	require.NotNil(t, lister, "Deployment lister should not be nil")
}

func TestInformerManager_GetServiceLister(t *testing.T) {
	if testCfg == nil {
		t.Skip("Skipping test - envtest not initialized")
	}

	ns := createTestNamespace(t)
	comp := createTestRepository(t, ns)
	defer comp.manager.Close()

	lister := comp.manager.GetServiceLister()
	require.NotNil(t, lister, "Service lister should not be nil")
}

func TestInformerManager_GetDynamicLister(t *testing.T) {
	if testCfg == nil {
		t.Skip("Skipping test - envtest not initialized")
	}

	ns := createTestNamespace(t)
	comp := createTestRepository(t, ns)
	defer comp.manager.Close()

	// Test getting a dynamic lister for ConfigMaps
	gvr, ok := GetGVRForResourceType(ResourceTypeConfigMap)
	require.True(t, ok)

	lister, found := comp.manager.GetDynamicLister(gvr)
	assert.True(t, found)
	assert.NotNil(t, lister)
}

func TestInformerManager_GetDynamicLister_NotFound(t *testing.T) {
	if testCfg == nil {
		t.Skip("Skipping test - envtest not initialized")
	}

	ns := createTestNamespace(t)
	comp := createTestRepository(t, ns)
	defer comp.manager.Close()

	// Test with non-existent GVR
	gvr := schema.GroupVersionResource{
		Group:    "nonexistent",
		Version:  "v1",
		Resource: "foos",
	}
	_, found := comp.manager.GetDynamicLister(gvr)
	assert.False(t, found)
}

func TestInformerManager_GetClientset(t *testing.T) {
	if testCfg == nil {
		t.Skip("Skipping test - envtest not initialized")
	}

	ns := createTestNamespace(t)
	comp := createTestRepository(t, ns)
	defer comp.manager.Close()

	clientset := comp.manager.GetClientset()
	require.NotNil(t, clientset)

	// Verify we can use the clientset
	_, err := clientset.CoreV1().Namespaces().List(context.Background(), metav1.ListOptions{})
	assert.NoError(t, err)
}

func TestInformerManager_Close(t *testing.T) {
	if testCfg == nil {
		t.Skip("Skipping test - envtest not initialized")
	}

	ns := createTestNamespace(t)
	comp := createTestRepository(t, ns)

	// First close should not panic
	assert.NotPanics(t, func() {
		comp.manager.Close()
	})

	// Second close should be safe (idempotent)
	assert.NotPanics(t, func() {
		comp.manager.Close()
	})
}
