package commandbar

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/renato0307/k1/internal/commands"
	"github.com/renato0307/k1/internal/k8s"
	"github.com/renato0307/k1/internal/k8s/dummy"
	"github.com/renato0307/k1/internal/types"
	"github.com/renato0307/k1/internal/ui"
)

// createTestAppContext creates an AppContext for testing
func createTestAppContext() *types.AppContext {
	dummyManager := dummy.NewManager()
	return types.NewAppContext(
		ui.GetTheme("charm"),
		dummy.NewDataRepository(),
		dummy.NewFormatter(),
		dummyManager,
	)
}

func TestNewExecutor(t *testing.T) {
	ctx := createTestAppContext()
	registry := commands.NewRegistry(ctx.Formatter, ctx.Provider)

	exec := NewExecutor(ctx, registry, 80)
	assert.NotNil(t, exec)
	assert.False(t, exec.HasPending())
}

func TestExecutor_BuildContext(t *testing.T) {
	ctx := createTestAppContext()
	registry := commands.NewRegistry(ctx.Formatter, ctx.Provider)

	exec := NewExecutor(ctx, registry, 80)

	selected := map[string]interface{}{
		"name":      "test-pod",
		"namespace": "default",
	}

	cmdCtx := exec.BuildContext(k8s.ResourceTypePod, selected, "arg1 arg2")
	assert.Equal(t, k8s.ResourceTypePod, cmdCtx.ResourceType)
	assert.Equal(t, selected, cmdCtx.Selected)
	assert.Equal(t, "arg1 arg2", cmdCtx.Args)
}

func TestExecutor_Execute(t *testing.T) {
	ctx := createTestAppContext()
	registry := commands.NewRegistry(ctx.Formatter, ctx.Provider)

	exec := NewExecutor(ctx, registry, 80)
	cmdCtx := exec.BuildContext(k8s.ResourceTypePod, nil, "")

	// Test executing a command that doesn't need confirmation
	cmd, needsConfirm := exec.Execute("yaml", commands.CategoryAction, cmdCtx)
	assert.False(t, needsConfirm)
	assert.NotNil(t, cmd) // yaml command should return a cmd

	// Test executing unknown command
	cmd, needsConfirm = exec.Execute("unknown", commands.CategoryAction, cmdCtx)
	assert.False(t, needsConfirm)
	assert.Nil(t, cmd)
}

func TestExecutor_Execute_NeedsConfirmation(t *testing.T) {
	ctx := createTestAppContext()
	registry := commands.NewRegistry(ctx.Formatter, ctx.Provider)

	exec := NewExecutor(ctx, registry, 80)
	cmdCtx := exec.BuildContext(k8s.ResourceTypeDeployment, nil, "3")

	// Test executing delete command (needs confirmation)
	cmd, needsConfirm := exec.Execute("delete", commands.CategoryAction, cmdCtx)
	assert.True(t, needsConfirm)
	assert.Nil(t, cmd) // Should not execute immediately
	assert.True(t, exec.HasPending())
	assert.NotNil(t, exec.GetPendingCommand())
	assert.Equal(t, "delete", exec.GetPendingCommand().Name)
}

func TestExecutor_ExecutePending(t *testing.T) {
	ctx := createTestAppContext()
	registry := commands.NewRegistry(ctx.Formatter, ctx.Provider)

	exec := NewExecutor(ctx, registry, 80)
	cmdCtx := exec.BuildContext(k8s.ResourceTypeDeployment, nil, "3")

	// Setup pending command
	_, needsConfirm := exec.Execute("delete", commands.CategoryAction, cmdCtx)
	assert.True(t, needsConfirm)
	assert.True(t, exec.HasPending())

	// Execute pending
	cmd := exec.ExecutePending(cmdCtx)
	assert.NotNil(t, cmd) // delete command should return error cmd
	assert.False(t, exec.HasPending()) // Should clear after execution
}

func TestExecutor_CancelPending(t *testing.T) {
	ctx := createTestAppContext()
	registry := commands.NewRegistry(ctx.Formatter, ctx.Provider)

	exec := NewExecutor(ctx, registry, 80)
	cmdCtx := exec.BuildContext(k8s.ResourceTypeDeployment, nil, "3")

	// Setup pending command
	exec.Execute("delete", commands.CategoryAction, cmdCtx)
	assert.True(t, exec.HasPending())

	// Cancel pending
	exec.CancelPending()
	assert.False(t, exec.HasPending())
	assert.Nil(t, exec.GetPendingCommand())
}

func TestExecutor_LLMTranslation(t *testing.T) {
	ctx := createTestAppContext()
	registry := commands.NewRegistry(ctx.Formatter, ctx.Provider)

	exec := NewExecutor(ctx, registry, 80)

	translation := &commands.MockLLMTranslation{
		Prompt:      "show me pods",
		Command:     "kubectl get pods",
		Explanation: "Lists all pods in the current namespace",
	}

	exec.SetLLMTranslation(translation)
	assert.Equal(t, translation, exec.GetLLMTranslation())

	exec.ClearLLMTranslation()
	assert.Nil(t, exec.GetLLMTranslation())
}

func TestExecutor_ViewConfirmation(t *testing.T) {
	ctx := createTestAppContext()
	registry := commands.NewRegistry(ctx.Formatter, ctx.Provider)

	exec := NewExecutor(ctx, registry, 80)

	// No pending command
	view := exec.ViewConfirmation()
	assert.Equal(t, "", view)

	// With pending command
	cmdCtx := exec.BuildContext("deployments", nil, "")
	exec.Execute("delete", commands.CategoryAction, cmdCtx)

	view = exec.ViewConfirmation()
	assert.NotEqual(t, "", view)
	assert.Contains(t, view, "Confirm Action")
	assert.Contains(t, view, "delete")
}

func TestExecutor_ViewLLMPreview(t *testing.T) {
	ctx := createTestAppContext()
	registry := commands.NewRegistry(ctx.Formatter, ctx.Provider)

	exec := NewExecutor(ctx, registry, 80)

	// No translation
	view := exec.ViewLLMPreview()
	assert.Equal(t, "", view)

	// With translation
	translation := &commands.MockLLMTranslation{
		Prompt:      "show me pods",
		Command:     "kubectl get pods",
		Explanation: "Lists all pods",
	}
	exec.SetLLMTranslation(translation)

	view = exec.ViewLLMPreview()
	assert.NotEqual(t, "", view)
	assert.Contains(t, view, "AI Command Preview")
	assert.Contains(t, view, "show me pods")
	assert.Contains(t, view, "kubectl get pods")
}

func TestExecutor_ViewResult(t *testing.T) {
	ctx := createTestAppContext()
	registry := commands.NewRegistry(ctx.Formatter, ctx.Provider)

	exec := NewExecutor(ctx, registry, 80)

	// Success result
	view := exec.ViewResult("Operation completed", true)
	assert.Contains(t, view, "✓")
	assert.Contains(t, view, "Operation completed")

	// Error result
	view = exec.ViewResult("Operation failed", false)
	assert.Contains(t, view, "✗")
	assert.Contains(t, view, "Operation failed")
}

func TestExecutor_SetWidth(t *testing.T) {
	ctx := createTestAppContext()
	registry := commands.NewRegistry(ctx.Formatter, ctx.Provider)

	exec := NewExecutor(ctx, registry, 80)
	assert.Equal(t, 80, exec.width)

	exec.SetWidth(120)
	assert.Equal(t, 120, exec.width)
}

// Test that executor properly handles command execution with tea.Cmd
func TestExecutor_ExecuteReturnsCmd(t *testing.T) {
	ctx := createTestAppContext()
	registry := commands.NewRegistry(ctx.Formatter, ctx.Provider)

	exec := NewExecutor(ctx, registry, 80)

	selected := map[string]interface{}{
		"name":      "test-pod",
		"namespace": "default",
	}
	cmdCtx := exec.BuildContext(k8s.ResourceTypePod, selected, "")

	// Execute yaml command which should return a cmd
	cmd, needsConfirm := exec.Execute("yaml", commands.CategoryAction, cmdCtx)
	assert.False(t, needsConfirm)
	assert.NotNil(t, cmd)

	// Execute the returned cmd to verify it returns a message
	msg := cmd()
	assert.NotNil(t, msg)
	// Commands return tea.Msg (various types), not tea.Cmd
}
