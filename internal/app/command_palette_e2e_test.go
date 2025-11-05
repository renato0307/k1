//go:build e2e

// Package app contains E2E tests for command palette features.
// Tests include command palette navigation, command execution, confirmations, and arguments.
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

// getKubeconfigForPalette returns the path to the kubeconfig file
func getKubeconfigForPalette() string {
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		if home := os.Getenv("HOME"); home != "" {
			kubeconfig = filepath.Join(home, ".kube", "config")
		}
	}
	return kubeconfig
}

// setupPaletteTest creates a test app with loaded context and synced informers
func setupPaletteTest(t *testing.T) (*k8s.RepositoryPool, Model, *testutil.TestProgram) {
	t.Helper()

	pool, err := k8s.NewRepositoryPool(getKubeconfigForPalette(), 10)
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

// TestCommandPalette_NavigationAndFuzzySearch tests opening the command palette
// with ctrl+p, using fuzzy search, and navigating commands.
func TestCommandPalette_NavigationAndFuzzySearch(t *testing.T) {
	_, _, tp := setupPaletteTest(t)
	defer tp.Quit()

	// Navigate to Deployments screen first
	tp.Type(":deployments")
	tp.SendKey(tea.KeyEnter)

	if !tp.WaitForScreen("Deployments", 3*time.Second) {
		t.Fatal("Failed to navigate to Deployments screen")
	}

	// Wait for deployments to load
	if !tp.WaitForOutput("nginx-deployment", 5*time.Second) {
		t.Fatal("nginx-deployment not found")
	}

	// Open command palette with >
	// Note: Palette can be opened with ctrl+p or >
	tp.Type(">")

	// Wait for palette to appear
	time.Sleep(300 * time.Millisecond)

	// Type "desc" for fuzzy search
	tp.Type("desc")
	time.Sleep(200 * time.Millisecond)

	// Verify "describe" command appears (may be highlighted)
	output := tp.Output()
	if !tp.WaitForOutput("describe", 2*time.Second) {
		t.Logf("Output:\n%s", output)
		t.Error("Expected to see 'describe' command in palette")
	}

	// Press Enter to execute
	tp.SendKey(tea.KeyEnter)
	time.Sleep(500 * time.Millisecond)

	// Should see describe view
	if !tp.WaitForOutput("Name:", 3*time.Second) {
		t.Logf("Output:\n%s", tp.Output())
		t.Error("Describe view did not appear after executing command")
	}
}

// TestCommandWithConfirmation_Cancel tests executing a command that requires
// confirmation and canceling it with ESC.
func TestCommandWithConfirmation_Cancel(t *testing.T) {
	_, _, tp := setupPaletteTest(t)
	defer tp.Quit()

	// We'll test delete confirmation
	// First go to Deployments and select nginx-deployment
	tp.Type(":deployments")
	tp.SendKey(tea.KeyEnter)

	if !tp.WaitForScreen("Deployments", 3*time.Second) {
		t.Fatal("Failed to navigate to Deployments screen")
	}

	// Wait for deployment to appear
	if !tp.WaitForOutput("nginx-deployment", 5*time.Second) {
		t.Fatal("nginx-deployment not found")
	}

	// Open command palette and type "delete"
	tp.Type(">")
	time.Sleep(200 * time.Millisecond)
	tp.Type("delete")
	time.Sleep(200 * time.Millisecond)

	// Press Enter to execute delete command
	tp.SendKey(tea.KeyEnter)
	time.Sleep(300 * time.Millisecond)

	// Should see confirmation dialog
	if !tp.WaitForConfirmation(3*time.Second) {
		t.Logf("Output:\n%s", tp.Output())
		t.Error("Expected confirmation dialog for delete command")
	}

	// Press ESC to cancel
	tp.SendKey(tea.KeyEsc)
	time.Sleep(300 * time.Millisecond)

	// Should still be on Deployments screen with nginx-deployment still there
	if !tp.WaitForOutput("nginx-deployment", 2*time.Second) {
		t.Error("Deployment should still exist after canceling delete")
	}
}

// TestCommandWithArguments_Scale tests executing a command that takes arguments.
func TestCommandWithArguments_Scale(t *testing.T) {
	_, _, tp := setupPaletteTest(t)
	defer tp.Quit()

	// Navigate to Deployments
	tp.Type(":deployments")
	tp.SendKey(tea.KeyEnter)

	if !tp.WaitForScreen("Deployments", 3*time.Second) {
		t.Fatal("Failed to navigate to Deployments screen")
	}

	// Wait for deployment
	if !tp.WaitForOutput("nginx-deployment", 5*time.Second) {
		t.Fatal("nginx-deployment not found")
	}

	// Open command palette and type "scale"
	tp.Type(">")
	time.Sleep(200 * time.Millisecond)
	tp.Type("scale ")
	time.Sleep(200 * time.Millisecond)

	// Type the scale argument (e.g., "2")
	tp.Type("2")
	time.Sleep(200 * time.Millisecond)

	// Press Enter to execute
	tp.SendKey(tea.KeyEnter)
	time.Sleep(500 * time.Millisecond)

	// Should see a success message or the deployment should update
	// Wait a bit for the operation to complete
	time.Sleep(1 * time.Second)

	// Check if we got a success message or error message
	output := tp.Output()
	// We just verify the command was executed (app didn't crash)
	if !tp.WaitForOutput("Deployments", 2*time.Second) {
		t.Logf("Output:\n%s", output)
		t.Error("App should still be responsive after scale command")
	}
}

// TestDeleteShortcut_CtrlX tests the ctrl+x shortcut for deleting resources.
func TestDeleteShortcut_CtrlX(t *testing.T) {
	_, _, tp := setupPaletteTest(t)
	defer tp.Quit()

	// Create a test pod that we can delete
	// For now, we'll just test that ctrl+x triggers the delete confirmation
	// without actually deleting (by canceling)

	// Navigate to Pods
	if !tp.WaitForScreen("Pods", 3*time.Second) {
		t.Fatal("Not on Pods screen")
	}

	// Wait for pods to load
	if !tp.WaitForOutput("test-app", 5*time.Second) {
		t.Fatal("test-app pods not found")
	}

	// Press ctrl+x (delete shortcut)
	tp.Send(tea.KeyMsg{Type: tea.KeyCtrlX})
	time.Sleep(300 * time.Millisecond)

	// Should see confirmation dialog
	if !tp.WaitForConfirmation(3*time.Second) {
		t.Logf("Output:\n%s", tp.Output())
		t.Error("Expected confirmation dialog after ctrl+x")
	}

	// Cancel with ESC to avoid actually deleting
	tp.SendKey(tea.KeyEsc)
	time.Sleep(300 * time.Millisecond)

	// Should still be on Pods screen
	tp.AssertContains("Pods")
}
