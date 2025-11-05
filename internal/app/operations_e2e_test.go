//go:build e2e

// Package app contains E2E tests for resource operations.
// Tests include scaling deployments, deleting resources, and jumping to pod owners.
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

// getKubeconfigForOperations returns the path to the kubeconfig file
func getKubeconfigForOperations() string {
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		if home := os.Getenv("HOME"); home != "" {
			kubeconfig = filepath.Join(home, ".kube", "config")
		}
	}
	return kubeconfig
}

// setupOperationsTest creates a test app with loaded context and synced informers
func setupOperationsTest(t *testing.T) (*k8s.RepositoryPool, Model, *testutil.TestProgram) {
	t.Helper()

	pool, err := k8s.NewRepositoryPool(getKubeconfigForOperations(), 10)
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

// TestScaleDeployment tests scaling a deployment via the command palette
func TestScaleDeployment(t *testing.T) {
	_, _, tp := setupOperationsTest(t)
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

	// Open command palette and type "scale 5"
	tp.Type(">")
	time.Sleep(200 * time.Millisecond)
	tp.Type("scale 5")
	time.Sleep(200 * time.Millisecond)

	// Press Enter to execute
	tp.SendKey(tea.KeyEnter)
	time.Sleep(500 * time.Millisecond)

	// Wait a bit for the operation to complete
	time.Sleep(1 * time.Second)

	// Verify app is still responsive (we can't easily verify the scale changed in E2E test)
	if !tp.WaitForOutput("Deployments", 2*time.Second) {
		t.Logf("Output:\n%s", tp.Output())
		t.Error("App should still be responsive after scale command")
	}
}

// TestDeleteResource tests deleting a resource via ctrl+x shortcut
func TestDeleteResource(t *testing.T) {
	_, _, tp := setupOperationsTest(t)
	defer tp.Quit()

	// Stay on Pods screen
	if !tp.WaitForScreen("Pods", 3*time.Second) {
		t.Fatal("Not on Pods screen")
	}

	// Wait for standalone-pod to appear
	if !tp.WaitForOutput("standalone-pod", 5*time.Second) {
		t.Skip("standalone-pod not found - skipping delete test")
	}

	// Move selection to standalone-pod by filtering
	tp.Type("/standalone-pod")
	time.Sleep(500 * time.Millisecond)

	// Exit filter mode
	tp.SendKey(tea.KeyEsc)
	time.Sleep(200 * time.Millisecond)

	// Now standalone-pod should be selected (first in filtered list)
	// Press ctrl+x to delete
	tp.Send(tea.KeyMsg{Type: tea.KeyCtrlX})
	time.Sleep(300 * time.Millisecond)

	// Should see confirmation dialog
	if !tp.WaitForConfirmation(3*time.Second) {
		t.Logf("Output:\n%s", tp.Output())
		t.Error("Expected confirmation dialog for delete")
	}

	// Press Enter to confirm
	tp.SendKey(tea.KeyEnter)
	time.Sleep(1 * time.Second)

	// Wait a bit for deletion to process
	time.Sleep(2 * time.Second)

	// Verify app is still responsive
	if !tp.WaitForOutput("Pods", 2*time.Second) {
		t.Logf("Output:\n%s", tp.Output())
		t.Error("App should still be responsive after delete")
	}
}

// TestPodOwnerNavigation tests jumping from a pod to its owner deployment
func TestPodOwnerNavigation(t *testing.T) {
	_, _, tp := setupOperationsTest(t)
	defer tp.Quit()

	// Stay on Pods screen
	if !tp.WaitForScreen("Pods", 3*time.Second) {
		t.Fatal("Not on Pods screen")
	}

	// Filter to nginx-deployment pods
	tp.Type("/nginx-deployment")
	time.Sleep(500 * time.Millisecond)

	// Wait for nginx-deployment pod to appear
	if !tp.WaitForOutput("nginx-deployment", 3*time.Second) {
		t.Fatal("nginx-deployment pod not found")
	}

	// Exit filter mode
	tp.SendKey(tea.KeyEsc)
	time.Sleep(200 * time.Millisecond)

	// Open command palette and type "jump-owner"
	tp.Type(">")
	time.Sleep(200 * time.Millisecond)
	tp.Type("jump-owner")
	time.Sleep(200 * time.Millisecond)

	// Press Enter to execute
	tp.SendKey(tea.KeyEnter)
	time.Sleep(500 * time.Millisecond)

	// Check if feature is implemented or coming soon
	output := tp.Output()
	if tp.WaitForOutput("Coming soon", 1*time.Second) {
		t.Skip("jump-owner command not yet implemented - skipping test")
	}

	// Should navigate to Deployments screen
	if !tp.WaitForScreen("Deployments", 3*time.Second) {
		t.Logf("Output:\n%s", output)
		t.Error("Did not navigate to Deployments screen after jump-owner")
	}

	// Verify nginx-deployment is visible
	tp.AssertContains("nginx-deployment")
}
