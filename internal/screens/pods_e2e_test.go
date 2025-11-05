//go:build e2e

package screens

import (
	"strings"
	"testing"
	"time"

	"github.com/renato0307/k1/internal/k8s"
	"github.com/renato0307/k1/internal/testutil"
	"github.com/renato0307/k1/internal/ui"
)

// TestPodsScreenLoadsWithDummyData tests that the Pods screen can load and display data
func TestPodsScreenLoadsWithDummyData(t *testing.T) {
	// Create a dummy repository for testing
	repo := k8s.NewDummyRepository()

	// Create the Pods screen config
	config := GetPodsScreenConfig()
	screen := NewConfigScreen(config, repo, ui.GetTheme("charm"))

	// Create test program
	tp := testutil.NewTestProgram(t, screen, 120, 40)
	defer tp.Quit()

	// Wait for screen to load (dummy repo loads instantly)
	if !tp.WaitForOutput("Namespace", 2*time.Second) {
		t.Logf("Output received:\n%s", tp.Output())
		t.Fatal("Timeout waiting for Pods screen to load")
	}

	// Verify table headers
	tp.AssertContains("Name")
	tp.AssertContains("Ready")
	tp.AssertContains("Status")
	tp.AssertContains("Age")

	// Verify we see some pod data from dummy repository
	output := tp.Output()
	if !strings.Contains(output, "kube-system") && !strings.Contains(output, "default") {
		t.Error("Expected to see namespace data in output")
	}
}

// TestPodsScreenWithRealCluster tests against the kind cluster
func TestPodsScreenWithRealCluster(t *testing.T) {
	// Skip if not running against real cluster
	if testing.Short() {
		t.Skip("Skipping real cluster test in short mode")
	}

	// Create repository using kind-k1-test context
	pool, err := k8s.NewRepositoryPool("", 10)
	if err != nil {
		t.Fatalf("Failed to create repository pool: %v", err)
	}

	progress := make(chan k8s.ContextLoadProgress, 10)
	go func() {
		for range progress {
			// Drain progress channel
		}
	}()

	err = pool.LoadContext("kind-k1-test", progress)
	if err != nil {
		t.Fatalf("Failed to load context: %v", err)
	}

	repo := pool.GetActiveRepository()
	if repo == nil {
		t.Fatal("Failed to get active repository")
	}

	// Create the Pods screen
	config := GetPodsScreenConfig()
	screen := NewConfigScreen(config, repo, ui.GetTheme("charm"))

	// Create test program with larger timeout for real cluster
	tp := testutil.NewTestProgram(t, screen, 120, 40)
	defer tp.Quit()

	// Wait for informers to sync and screen to render (may take several seconds)
	if !tp.WaitForOutput("Namespace", 15*time.Second) {
		t.Fatal("Timeout waiting for Pods screen to load from real cluster")
	}

	// Verify we see test-app namespace from our test fixtures
	if !tp.WaitForOutput("test-app", 5*time.Second) {
		t.Error("Expected to see test-app namespace from test fixtures")
	}

	// Verify we see nginx pods
	tp.AssertContains("nginx")

	// The output should contain multiple pods
	output := tp.Output()
	podCount := strings.Count(output, "Running") + strings.Count(output, "Completed")
	if podCount < 3 {
		t.Errorf("Expected at least 3 running/completed pods, found %d", podCount)
	}
}

// TestPodsScreenRefresh tests that pressing 'r' refreshes the screen
func TestPodsScreenRefresh(t *testing.T) {
	repo := k8s.NewDummyRepository()
	config := GetPodsScreenConfig()
	screen := NewConfigScreen(config, repo, ui.GetTheme("charm"))

	tp := testutil.NewTestProgram(t, screen, 120, 40)
	defer tp.Quit()

	// Wait for initial load
	if !tp.WaitForOutput("Namespace", 2*time.Second) {
		t.Fatal("Timeout waiting for initial load")
	}

	// Press 'r' to refresh
	tp.Type("r")

	// Screen should still show the table
	time.Sleep(100 * time.Millisecond)
	tp.AssertContains("Namespace")
	tp.AssertContains("Name")
}
