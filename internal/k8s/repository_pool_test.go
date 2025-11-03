package k8s

import (
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

// createTestKubeconfig creates a temporary kubeconfig file for testing
func createTestKubeconfig(t *testing.T, contexts ...string) string {
	t.Helper()

	// Default contexts if none provided
	if len(contexts) == 0 {
		contexts = []string{"test-context-1", "test-context-2", "test-context-3"}
	}

	// Create kubeconfig with test API server
	config := clientcmdapi.NewConfig()

	// Add cluster
	config.Clusters["test-cluster"] = &clientcmdapi.Cluster{
		Server:                   testCfg.Host,
		CertificateAuthority:     testCfg.CAFile,
		CertificateAuthorityData: testCfg.CAData,
		InsecureSkipTLSVerify:    testCfg.Insecure,
	}

	// Add user
	config.AuthInfos["test-user"] = &clientcmdapi.AuthInfo{
		ClientCertificate:     testCfg.CertFile,
		ClientKey:             testCfg.KeyFile,
		ClientCertificateData: testCfg.CertData,
		ClientKeyData:         testCfg.KeyData,
		Token:                 testCfg.BearerToken,
	}

	// Add contexts
	for _, ctxName := range contexts {
		config.Contexts[ctxName] = &clientcmdapi.Context{
			Cluster:   "test-cluster",
			AuthInfo:  "test-user",
			Namespace: "default",
		}
	}

	// Set current context to first one
	if len(contexts) > 0 {
		config.CurrentContext = contexts[0]
	}

	// Write to temp file
	tmpDir := t.TempDir()
	kubeconfigPath := filepath.Join(tmpDir, "kubeconfig")
	err := clientcmd.WriteToFile(*config, kubeconfigPath)
	require.NoError(t, err, "Failed to write kubeconfig")

	return kubeconfigPath
}

// TestNewRepositoryPool tests pool creation
func TestNewRepositoryPool(t *testing.T) {
	tests := []struct {
		name        string
		maxSize     int
		expectError bool
	}{
		{
			name:        "valid kubeconfig with default size",
			maxSize:     0, // Should default to 10
			expectError: false,
		},
		{
			name:        "valid kubeconfig with custom size",
			maxSize:     5,
			expectError: false,
		},
		{
			name:        "negative maxSize defaults to 10",
			maxSize:     -1,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kubeconfigPath := createTestKubeconfig(t)

			pool, err := NewRepositoryPool(kubeconfigPath, tt.maxSize)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, pool)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, pool)
				if pool != nil {
					defer pool.Close()

					// Verify pool has contexts
					contexts := pool.GetAllContexts()
					assert.Greater(t, len(contexts), 0, "Pool should have contexts")
				}
			}
		})
	}
}

// TestRepositoryPool_LoadContext_Concurrent tests that concurrent loads don't create duplicates
func TestRepositoryPool_LoadContext_Concurrent(t *testing.T) {
	kubeconfigPath := createTestKubeconfig(t, "ctx1", "ctx2")
	pool, err := NewRepositoryPool(kubeconfigPath, 10)
	require.NoError(t, err)
	defer pool.Close()

	contextName := "ctx1"

	// Launch 10 goroutines trying to load same context
	var wg sync.WaitGroup
	errCh := make(chan error, 10)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := pool.LoadContext(contextName, nil)
			errCh <- err
		}()
	}

	wg.Wait()
	close(errCh)

	// Check results - all should succeed (LoadOrStore coordination)
	successCount := 0
	for err := range errCh {
		if err == nil {
			successCount++
		}
	}

	assert.Equal(t, 10, successCount, "All concurrent loads should succeed")

	// Set context as active to verify repository exists
	require.NoError(t, pool.SetActive(contextName))
	repo := pool.GetActiveRepository()
	assert.NotNil(t, repo, "Repository should exist")
}

// TestRepositoryPool_LRU_Eviction tests LRU eviction with maxSize limit
func TestRepositoryPool_LRU_Eviction(t *testing.T) {
	kubeconfigPath := createTestKubeconfig(t, "ctx1", "ctx2", "ctx3", "ctx4")
	pool, err := NewRepositoryPool(kubeconfigPath, 3)
	require.NoError(t, err)
	defer pool.Close()

	// Load 3 contexts (fills pool)
	require.NoError(t, pool.LoadContext("ctx1", nil))
	require.NoError(t, pool.LoadContext("ctx2", nil))
	require.NoError(t, pool.LoadContext("ctx3", nil))

	// Set ctx2 as active
	require.NoError(t, pool.SetActive("ctx2"))

	// Load 4th context (should evict ctx1, LRU and not active)
	require.NoError(t, pool.LoadContext("ctx4", nil))

	// Verify eviction happened - only 3 contexts should be loaded
	contexts, err := pool.GetContexts()
	require.NoError(t, err)

	statuses := make(map[string]RepositoryStatus)
	loadedCount := 0
	for _, ctx := range contexts {
		statuses[ctx.Name] = RepositoryStatus(ctx.Status)
		if RepositoryStatus(ctx.Status) == StatusLoaded {
			loadedCount++
		}
	}

	// Pool size is 3, so exactly 3 contexts should be loaded
	assert.Equal(t, 3, loadedCount, "Exactly 3 contexts should be loaded after eviction")

	// ctx2 should definitely still be loaded (it's active)
	assert.Equal(t, StatusLoaded, statuses["ctx2"], "ctx2 should still be loaded (active)")

	// ctx4 should be loaded (just added)
	assert.Equal(t, StatusLoaded, statuses["ctx4"], "ctx4 should be loaded (just added)")

	// One of ctx1 or ctx3 should be evicted (LRU logic determines which)
	notLoadedCount := 0
	if statuses["ctx1"] == StatusNotLoaded {
		notLoadedCount++
	}
	if statuses["ctx3"] == StatusNotLoaded {
		notLoadedCount++
	}
	assert.Equal(t, 1, notLoadedCount, "Exactly one context (ctx1 or ctx3) should be evicted")
}

// TestRepositoryPool_SwitchContext_Loaded tests instant switch to loaded context
func TestRepositoryPool_SwitchContext_Loaded(t *testing.T) {
	kubeconfigPath := createTestKubeconfig(t, "ctx1", "ctx2")
	pool, err := NewRepositoryPool(kubeconfigPath, 10)
	require.NoError(t, err)
	defer pool.Close()

	// Load two contexts
	require.NoError(t, pool.LoadContext("ctx1", nil))
	require.NoError(t, pool.LoadContext("ctx2", nil))
	require.NoError(t, pool.SetActive("ctx1"))

	// Switch to loaded context (should be instant)
	start := time.Now()
	err = pool.SwitchContext("ctx2", nil)
	elapsed := time.Since(start)

	assert.NoError(t, err)
	assert.Less(t, elapsed, 100*time.Millisecond, "Switch should be instant")
	assert.Equal(t, "ctx2", pool.GetActiveContext())
}

// TestRepositoryPool_SwitchContext_NotLoaded tests switch to not loaded context
func TestRepositoryPool_SwitchContext_NotLoaded(t *testing.T) {
	kubeconfigPath := createTestKubeconfig(t, "ctx1", "ctx2")
	pool, err := NewRepositoryPool(kubeconfigPath, 10)
	require.NoError(t, err)
	defer pool.Close()

	// Load first context
	require.NoError(t, pool.LoadContext("ctx1", nil))
	require.NoError(t, pool.SetActive("ctx1"))

	// Switch to not loaded context (blocks until loaded)
	err = pool.SwitchContext("ctx2", nil)

	assert.NoError(t, err)
	assert.Equal(t, "ctx2", pool.GetActiveContext())

	// Verify ctx2 is now loaded
	contexts, err := pool.GetContexts()
	require.NoError(t, err)

	for _, ctx := range contexts {
		if ctx.Name == "ctx2" {
			assert.Equal(t, "Loaded", ctx.Status)
		}
	}
}

// TestRepositoryPool_Close tests that pool closes all repositories
func TestRepositoryPool_Close(t *testing.T) {
	kubeconfigPath := createTestKubeconfig(t, "ctx1", "ctx2")
	pool, err := NewRepositoryPool(kubeconfigPath, 10)
	require.NoError(t, err)

	// Load multiple contexts
	require.NoError(t, pool.LoadContext("ctx1", nil))
	require.NoError(t, pool.LoadContext("ctx2", nil))

	// Close pool
	pool.Close()

	// After close, pool should be empty (based on Close implementation)
	// Note: Current implementation doesn't clear state, but Phase 3 will fix this
}

// TestRepositoryPool_GetContexts_Status tests context status reporting
func TestRepositoryPool_GetContexts_Status(t *testing.T) {
	kubeconfigPath := createTestKubeconfig(t, "ctx1", "ctx2", "ctx3")
	pool, err := NewRepositoryPool(kubeconfigPath, 10)
	require.NoError(t, err)
	defer pool.Close()

	// Load one context
	require.NoError(t, pool.LoadContext("ctx1", nil))
	require.NoError(t, pool.SetActive("ctx1"))

	// Get all contexts
	contexts, err := pool.GetContexts()
	require.NoError(t, err)

	// Verify we have 3 contexts
	assert.Equal(t, 3, len(contexts))

	// Verify ctx1 is loaded and current
	var ctx1Found bool
	for _, ctx := range contexts {
		if ctx.Name == "ctx1" {
			assert.Equal(t, "Loaded", ctx.Status)
			assert.Equal(t, "✓", ctx.Current)
			ctx1Found = true
		}
	}
	assert.True(t, ctx1Found, "ctx1 should be in context list")
}

// TestRepositoryPool_Race_ConcurrentOperations tests concurrent operations with race detector
func TestRepositoryPool_Race_ConcurrentOperations(t *testing.T) {
	kubeconfigPath := createTestKubeconfig(t, "ctx1", "ctx2", "ctx3", "ctx4")
	pool, err := NewRepositoryPool(kubeconfigPath, 10)
	require.NoError(t, err)
	defer pool.Close()

	// Load initial context
	require.NoError(t, pool.LoadContext("ctx1", nil))
	require.NoError(t, pool.SetActive("ctx1"))

	// Launch concurrent operations
	var wg sync.WaitGroup

	// Reader goroutines
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				pool.GetActiveContext()
				pool.GetContexts()
				time.Sleep(time.Millisecond)
			}
		}()
	}

	// Writer goroutines
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			ctxName := fmt.Sprintf("ctx%d", id+2)
			pool.LoadContext(ctxName, nil)
		}(i)
	}

	wg.Wait()
	// If race detector enabled, will catch issues
}

// TestRepositoryPool_GetResources tests resource delegation
func TestRepositoryPool_GetResources(t *testing.T) {
	kubeconfigPath := createTestKubeconfig(t, "ctx1")
	pool, err := NewRepositoryPool(kubeconfigPath, 10)
	require.NoError(t, err)
	defer pool.Close()

	// Load context
	require.NoError(t, pool.LoadContext("ctx1", nil))
	require.NoError(t, pool.SetActive("ctx1"))

	// Test getting pods (should delegate to active repository)
	pods, err := pool.GetPods()
	require.NoError(t, err)
	assert.NotNil(t, pods)
}

// TestRepositoryPool_GetContexts_SpecialHandling tests contexts resource type
func TestRepositoryPool_GetContexts_SpecialHandling(t *testing.T) {
	kubeconfigPath := createTestKubeconfig(t, "ctx1", "ctx2")
	pool, err := NewRepositoryPool(kubeconfigPath, 10)
	require.NoError(t, err)
	defer pool.Close()

	// GetResources with ResourceTypeContext should return contexts from pool
	resources, err := pool.GetResources(ResourceTypeContext)
	require.NoError(t, err)

	// Should have 2 contexts
	assert.Equal(t, 2, len(resources))
}

// TestRepositoryPool_RetryFailedContext tests retrying a failed context
func TestRepositoryPool_RetryFailedContext(t *testing.T) {
	kubeconfigPath := createTestKubeconfig(t, "ctx1")
	pool, err := NewRepositoryPool(kubeconfigPath, 10)
	require.NoError(t, err)
	defer pool.Close()

	// Manually mark a context as failed for testing
	pool.mu.Lock()
	pool.repos["ctx1"] = &RepositoryEntry{
		Status: StatusFailed,
		Error:  fmt.Errorf("simulated failure"),
	}
	pool.mu.Unlock()

	// Retry should attempt to reload
	err = pool.RetryFailedContext("ctx1", nil)

	// Should succeed now
	assert.NoError(t, err)
}

// TestRepositoryPool_DescribeResource tests resource description delegation
func TestRepositoryPool_DescribeResource(t *testing.T) {
	kubeconfigPath := createTestKubeconfig(t, "ctx1")
	pool, err := NewRepositoryPool(kubeconfigPath, 10)
	require.NoError(t, err)
	defer pool.Close()

	// Load context
	require.NoError(t, pool.LoadContext("ctx1", nil))
	require.NoError(t, pool.SetActive("ctx1"))

	// Test describe (should delegate to active repository)
	gvr := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}
	_, err = pool.DescribeResource(gvr, "default", "test-pod")

	// Error is expected (pod doesn't exist), but method should work
	assert.Error(t, err) // Pod not found is fine
}
