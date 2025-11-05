//go:build e2e

// Package app contains E2E tests for full-screen resource views.
// Tests include YAML view and describe view functionality.
// All tests run against kind-k1-test cluster.
package app

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/renato0307/k1/internal/k8s"
	"github.com/renato0307/k1/internal/testutil"
	"github.com/renato0307/k1/internal/ui"
)

// getKubeconfigForFullscreen returns the path to the kubeconfig file
func getKubeconfigForFullscreen() string {
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		if home := os.Getenv("HOME"); home != "" {
			kubeconfig = filepath.Join(home, ".kube", "config")
		}
	}
	return kubeconfig
}

// setupFullscreenTest creates a test app with loaded context and synced informers
func setupFullscreenTest(t *testing.T) (*k8s.RepositoryPool, Model, *testutil.TestProgram) {
	t.Helper()

	pool, err := k8s.NewRepositoryPool(getKubeconfigForFullscreen(), 10)
	if err != nil {
		t.Fatalf("Failed to create pool: %v", err)
	}

	progress := make(chan k8s.ContextLoadProgress, 100)
	go func() {
		for range progress {
			// Drain progress channel
		}
	}()

	err = pool.LoadContext("kind-k1-test", progress)
	if err != nil {
		t.Fatalf("Failed to load context: %v", err)
	}

	// Set as active context
	err = pool.SetActive("kind-k1-test")
	if err != nil {
		t.Fatalf("Failed to set active context: %v", err)
	}

	// Wait for informers to be ready
	repo := pool.GetActiveRepository()
	if repo == nil {
		t.Fatal("Failed to get active repository")
	}

	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		if repo.AreTypedInformersReady() {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !repo.AreTypedInformersReady() {
		t.Fatal("Timed out waiting for informers to sync")
	}

	// Give a bit more time for data to stabilize
	time.Sleep(500 * time.Millisecond)

	app := NewModel(pool, ui.GetTheme("charm"))
	tp := testutil.NewTestProgram(t, app, 120, 40)

	// Wait for initial screen
	if !tp.WaitForScreen("Pods", 10*time.Second) {
		t.Fatal("Timeout waiting for Pods screen")
	}

	return pool, app, tp
}

// TestYAMLView tests pressing 'y' to view resource YAML
func TestYAMLView(t *testing.T) {
	_, _, tp := setupFullscreenTest(t)
	defer tp.Quit()

	// Navigate to Deployments screen
	tp.Type(":deployments")
	tp.SendKey(tea.KeyEnter)

	if !tp.WaitForScreen("Deployments", 3*time.Second) {
		t.Fatal("Failed to navigate to Deployments screen")
	}

	// Wait for nginx-deployment to appear
	if !tp.WaitForOutput("nginx-deployment", 5*time.Second) {
		t.Fatal("nginx-deployment not found")
	}

	// Press 'y' to view YAML
	tp.Type("y")

	// Wait for YAML view to appear
	time.Sleep(500 * time.Millisecond)

	// Should see YAML content
	if !tp.WaitForOutput("apiVersion", 3*time.Second) {
		t.Logf("Output:\n%s", tp.Output())
		t.Error("YAML view did not appear after pressing y")
	} else {
		// Verify key YAML fields are present
		tp.AssertContains("kind")
		tp.AssertContains("metadata")
	}

	// Press ESC to return to list
	tp.SendKey(tea.KeyEsc)
	time.Sleep(300 * time.Millisecond)

	// Should be back on Deployments screen
	if !tp.WaitForScreen("Deployments", 3*time.Second) {
		t.Logf("Output:\n%s", tp.Output())
		t.Error("Did not return to Deployments after ESC from YAML view")
	}
}

// TestDescribeView tests pressing 'd' to view resource description
func TestDescribeView(t *testing.T) {
	_, _, tp := setupFullscreenTest(t)
	defer tp.Quit()

	// Navigate to Deployments screen
	tp.Type(":deployments")
	tp.SendKey(tea.KeyEnter)

	if !tp.WaitForScreen("Deployments", 3*time.Second) {
		t.Fatal("Failed to navigate to Deployments screen")
	}

	// Wait for nginx-deployment to appear
	if !tp.WaitForOutput("nginx-deployment", 5*time.Second) {
		t.Fatal("nginx-deployment not found")
	}

	// Press 'd' to view describe
	tp.Type("d")
	time.Sleep(500 * time.Millisecond)

	// Should see describe output
	if !tp.WaitForOutput("Name:", 3*time.Second) {
		t.Logf("Output:\n%s", tp.Output())
		t.Error("Describe view did not appear after pressing d")
	} else {
		// Verify key describe fields are present
		tp.AssertContains("Namespace:")
	}

	// Press ESC to return to list
	tp.SendKey(tea.KeyEsc)
	time.Sleep(300 * time.Millisecond)

	// Should be back on Deployments screen
	if !tp.WaitForScreen("Deployments", 3*time.Second) {
		t.Logf("Output:\n%s", tp.Output())
		t.Error("Did not return to Deployments after ESC from describe view")
	}
}
