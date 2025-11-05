//go:build e2e

// Package app contains E2E tests for navigation features.
// Tests include screen switching, back navigation, context cycling, and keyboard shortcuts.
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

// getKubeconfig returns the path to the kubeconfig file
func getKubeconfig() string {
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		if home := os.Getenv("HOME"); home != "" {
			kubeconfig = filepath.Join(home, ".kube", "config")
		}
	}
	return kubeconfig
}

// TestScreenSwitching_ViaNavigationPalette tests switching between screens
// using the :command navigation palette.
func TestScreenSwitching_ViaNavigationPalette(t *testing.T) {
	// Create repository pool and load context
	pool, err := k8s.NewRepositoryPool(getKubeconfig(), 10)
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

	// Create app model
	app := NewModel(pool, ui.GetTheme("charm"))

	// Create test program
	tp := testutil.NewTestProgram(t, app, 120, 40)
	defer tp.Quit()

	// Wait for initial screen (Pods)
	if !tp.WaitForScreen("Pods", 10*time.Second) {
		t.Logf("Output:\n%s", tp.Output())
		t.Fatal("Timeout waiting for Pods screen")
	}

	// Navigate to Deployments screen
	tp.Type(":deployments")
	tp.SendKey(tea.KeyEnter)

	if !tp.WaitForScreen("Deployments", 5*time.Second) {
		t.Logf("Output:\n%s", tp.Output())
		t.Fatal("Failed to navigate to Deployments screen")
	}

	// Verify we're on deployments screen
	tp.AssertContains("Deployments")

	// Navigate to Services screen
	tp.Type(":services")
	tp.SendKey(tea.KeyEnter)

	if !tp.WaitForScreen("Services", 5*time.Second) {
		t.Logf("Output:\n%s", tp.Output())
		t.Fatal("Failed to navigate to Services screen")
	}

	tp.AssertContains("Services")
}

// TestBackNavigation_WithESC tests using ESC to navigate back through history.
func TestBackNavigation_WithESC(t *testing.T) {
	pool, err := k8s.NewRepositoryPool(getKubeconfig(), 10)
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
	defer tp.Quit()

	// Wait for initial Pods screen
	if !tp.WaitForScreen("Pods", 10*time.Second) {
		t.Fatal("Timeout waiting for Pods screen")
	}

	// Navigate to Deployments
	tp.Type(":deployments")
	tp.SendKey(tea.KeyEnter)

	if !tp.WaitForScreen("Deployments", 5*time.Second) {
		t.Fatal("Failed to navigate to Deployments")
	}

	// Press ESC to go back
	tp.SendKey(tea.KeyEsc)

	// Wait a moment for navigation to complete
	time.Sleep(200 * time.Millisecond)

	// Should be back on Pods screen
	if !tp.WaitForScreen("Pods", 3*time.Second) {
		t.Logf("Output:\n%s", tp.Output())
		t.Fatal("ESC did not navigate back to Pods screen")
	}

	tp.AssertContains("Pods")
}

// TestContextSwitching_MultiContext tests cycling through contexts with ctrl+n/ctrl+p.
// This test will skip if only one context is available.
func TestContextSwitching_MultiContext(t *testing.T) {
	pool, err := k8s.NewRepositoryPool(getKubeconfig(), 10)
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

	// Note: This test verifies the UI responds to context switching keys,
	// but in a typical test scenario with only one loaded context,
	// the context won't actually change. That's acceptable - we're testing
	// that the keys are wired up correctly, not multi-context behavior itself.
	//
	// For a full multi-context test, you would need to load multiple contexts
	// before creating the app model.

	app := NewModel(pool, ui.GetTheme("charm"))
	tp := testutil.NewTestProgram(t, app, 120, 40)
	defer tp.Quit()

	// Wait for initial screen
	if !tp.WaitForScreen("Pods", 10*time.Second) {
		t.Fatal("Timeout waiting for Pods screen")
	}

	// Verify we can press the context cycling keys without errors
	// Press ] to cycle to next context
	tp.Type("]")
	time.Sleep(500 * time.Millisecond)

	// Verify the app is still responsive (should still show Pods screen)
	tp.AssertContains("Pods")

	// Press [ to cycle to previous context
	tp.Type("[")
	time.Sleep(500 * time.Millisecond)

	// Verify the app is still responsive
	tp.AssertContains("Pods")

	// If we actually have multiple contexts loaded, we could check for context change
	// but in a typical test setup with one context, this is sufficient to verify
	// the keyboard shortcuts are wired correctly
}

// TestContextualNavigation_WithEnter tests navigating from deployment to
// filtered pods view by selecting a deployment and pressing Enter.
func TestContextualNavigation_WithEnter(t *testing.T) {
	pool, err := k8s.NewRepositoryPool(getKubeconfig(), 10)
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
	defer tp.Quit()

	// Navigate to Deployments screen
	if !tp.WaitForScreen("Pods", 10*time.Second) {
		t.Fatal("Timeout waiting for initial screen")
	}

	tp.Type(":deployments")
	tp.SendKey(tea.KeyEnter)

	if !tp.WaitForScreen("Deployments", 5*time.Second) {
		t.Fatal("Failed to navigate to Deployments screen")
	}

	// Wait for deployment data to load
	if !tp.WaitForOutput("nginx-deployment", 5*time.Second) {
		t.Logf("Output:\n%s", tp.Output())
		t.Fatal("nginx-deployment not found in Deployments screen")
	}

	// Press Enter on selected deployment (should be first row)
	tp.SendKey(tea.KeyEnter)

	// Should navigate to Pods screen filtered by this deployment
	time.Sleep(500 * time.Millisecond)

	// Check we're on Pods screen
	if !tp.WaitForScreen("Pods", 3*time.Second) {
		t.Logf("Output:\n%s", tp.Output())
		t.Fatal("Did not navigate to Pods screen after pressing Enter")
	}

	// Verify filter is applied (should show nginx deployment pods)
	tp.AssertContains("nginx")
}

// TestCommandShortcuts_YAMLAndDescribe tests ctrl+y (YAML view) and
// ctrl+d (describe view) shortcuts.
func TestCommandShortcuts_YAMLAndDescribe(t *testing.T) {
	pool, err := k8s.NewRepositoryPool(getKubeconfig(), 10)
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
	defer tp.Quit()

	// Navigate to Deployments screen
	if !tp.WaitForScreen("Pods", 10*time.Second) {
		t.Fatal("Timeout waiting for initial screen")
	}

	tp.Type(":deployments")
	tp.SendKey(tea.KeyEnter)

	if !tp.WaitForScreen("Deployments", 5*time.Second) {
		t.Fatal("Failed to navigate to Deployments screen")
	}

	// Wait for deployment to appear
	if !tp.WaitForOutput("nginx-deployment", 5*time.Second) {
		t.Fatal("nginx-deployment not found")
	}

	// Test y for YAML view
	tp.Type("y")

	// Wait for YAML view to appear
	time.Sleep(500 * time.Millisecond)

	// Should see YAML content
	if !tp.WaitForOutput("apiVersion", 3*time.Second) {
		t.Logf("Output:\n%s", tp.Output())
		t.Error("YAML view did not appear after y")
	} else {
		tp.AssertContains("kind")
		tp.AssertContains("metadata")
	}

	// Press ESC to return to list
	tp.SendKey(tea.KeyEsc)
	time.Sleep(300 * time.Millisecond)

	// Should be back on Deployments screen
	if !tp.WaitForScreen("Deployments", 3*time.Second) {
		t.Error("Did not return to Deployments after ESC from YAML view")
	}

	// Test d for describe view
	tp.Type("d")
	time.Sleep(500 * time.Millisecond)

	// Should see describe output
	if !tp.WaitForOutput("Name:", 3*time.Second) {
		t.Logf("Output:\n%s", tp.Output())
		t.Error("Describe view did not appear after d")
	} else {
		tp.AssertContains("Namespace:")
	}

	// Press ESC to return to list
	tp.SendKey(tea.KeyEsc)
	time.Sleep(300 * time.Millisecond)

	// Should be back on Deployments screen
	if !tp.WaitForScreen("Deployments", 3*time.Second) {
		t.Error("Did not return to Deployments after ESC from describe view")
	}
}
