package k8s

import (
	"fmt"
	"os"
	"testing"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
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
