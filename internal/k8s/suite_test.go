package k8s

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

var (
	testEnv   *envtest.Environment
	testCfg   *rest.Config
	testClient *kubernetes.Clientset
)

// TestMain sets up a shared envtest environment for all tests
// This runs ONCE before all tests, dramatically improving test speed
func TestMain(m *testing.M) {
	// Setup: Start envtest once for all tests
	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{},
		ErrorIfCRDPathMissing: false,
	}

	var err error
	testCfg, err = testEnv.Start()
	if err != nil {
		fmt.Printf("Failed to start envtest: %v\n", err)
		os.Exit(1)
	}

	// Use protobuf for better performance
	testCfg.ContentType = "application/vnd.kubernetes.protobuf"

	testClient, err = kubernetes.NewForConfig(testCfg)
	if err != nil {
		fmt.Printf("Failed to create clientset: %v\n", err)
		_ = testEnv.Stop()
		os.Exit(1)
	}

	// Run all tests
	code := m.Run()

	// Teardown: Stop envtest
	if err := testEnv.Stop(); err != nil {
		fmt.Printf("Failed to stop envtest: %v\n", err)
		os.Exit(1)
	}

	os.Exit(code)
}

// testComponents wraps the 3 components for testing
type testComponents struct {
	manager   *InformerManager
	dataRepo  *DataRepository
	formatter *Formatter
	ctx       context.Context
	cancel    context.CancelFunc
}

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

// createTestRepository creates test components using the shared test config
// and scoped to a specific namespace for test isolation
func createTestRepository(t *testing.T, namespace string) *testComponents {
	t.Helper()

	// Create dynamic client
	dynamicClient, err := dynamic.NewForConfig(testCfg)
	require.NoError(t, err, "Failed to create dynamic client")

	// Create namespace-scoped informer factory for test isolation
	factory := informers.NewSharedInformerFactoryWithOptions(
		testClient,
		30*time.Second,
		informers.WithNamespace(namespace), // Scope to test namespace
	)

	// Create namespace-scoped dynamic informer factory
	dynamicFactory := dynamicinformer.NewFilteredDynamicSharedInformerFactory(
		dynamicClient,
		30*time.Second,
		namespace,
		nil,
	)

	// Create pod informer
	podInformer := factory.Core().V1().Pods().Informer()
	podLister := factory.Core().V1().Pods().Lister()

	// Create deployment informer
	deploymentInformer := factory.Apps().V1().Deployments().Informer()
	deploymentLister := factory.Apps().V1().Deployments().Lister()

	// Create service informer
	serviceInformer := factory.Core().V1().Services().Informer()
	serviceLister := factory.Core().V1().Services().Lister()

	// Initialize resource registry
	resourceRegistry := getResourceRegistry()

	// Create dynamic informers for all registered resources
	// Skip cluster-scoped resources when using namespace-scoped factory
	dynamicListers := make(map[schema.GroupVersionResource]cache.GenericLister)
	dynamicInformers := []cache.SharedIndexInformer{}

	for _, resCfg := range resourceRegistry {
		// Skip cluster-scoped resources (nodes, namespaces) in namespace-scoped tests
		if !resCfg.Namespaced {
			continue
		}
		informer := dynamicFactory.ForResource(resCfg.GVR).Informer()
		dynamicListers[resCfg.GVR] = dynamicFactory.ForResource(resCfg.GVR).Lister()
		dynamicInformers = append(dynamicInformers, informer)
	}

	// Create context for lifecycle management
	ctx, cancel := context.WithCancel(context.Background())

	// Start informers in background
	factory.Start(ctx.Done())
	dynamicFactory.Start(ctx.Done())

	// Wait for all caches to sync (both typed and dynamic)
	allInformers := []cache.InformerSynced{
		podInformer.HasSynced,
		deploymentInformer.HasSynced,
		serviceInformer.HasSynced,
	}
	for _, inf := range dynamicInformers {
		allInformers = append(allInformers, inf.HasSynced)
	}

	synced := cache.WaitForCacheSync(ctx.Done(), allInformers...)
	if !synced {
		cancel()
	}
	require.True(t, synced, "Failed to sync caches")

	// Create InformerManager manually
	manager := &InformerManager{
		clientset:        testClient,
		factory:          factory,
		podLister:        podLister,
		deploymentLister: deploymentLister,
		serviceLister:    serviceLister,
		dynamicClient:    dynamicClient,
		dynamicFactory:   dynamicFactory,
		dynamicListers:   dynamicListers,
		ctx:              ctx,
		cancel:           cancel,
	}

	// Create DataRepository
	dataRepo := NewDataRepository(manager)

	// Create Formatter
	formatter := NewResourceFormatter(manager)

	return &testComponents{
		manager:   manager,
		dataRepo:  dataRepo,
		formatter: formatter,
		ctx:       ctx,
		cancel:    cancel,
	}
}
