package app

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/renato0307/k1/internal/k8s"
	"github.com/renato0307/k1/internal/screens"
	"github.com/renato0307/k1/internal/types"
	"github.com/renato0307/k1/internal/ui"
	"github.com/stretchr/testify/assert"
)

// TestPushNavigationHistory_MaxSize verifies history size limit enforcement
func TestPushNavigationHistory_MaxSize(t *testing.T) {
	repo := k8s.NewDummyRepository()
	theme := ui.ThemeCharm()
	model := NewModel(repo, theme)

	// Push MaxNavigationHistorySize + 10 entries
	for i := 0; i < MaxNavigationHistorySize+10; i++ {
		model.pushNavigationHistory()
	}

	// Verify size is capped at MaxNavigationHistorySize
	assert.Equal(t, MaxNavigationHistorySize, len(model.navigationHistory),
		"History size should be capped at MaxNavigationHistorySize")
}

// TestPushNavigationHistory_CapturesFilterContext verifies filter context is captured
func TestPushNavigationHistory_CapturesFilterContext(t *testing.T) {
	repo := k8s.NewDummyRepository()
	theme := ui.ThemeCharm()
	model := NewModel(repo, theme)

	// Switch to deployments screen
	msg := types.ScreenSwitchMsg{ScreenID: "deployments"}
	updatedModel, _ := model.Update(msg)
	model = updatedModel.(Model)

	// Apply a filter context to the deployments screen
	filterContext := &types.FilterContext{
		Field: "owner",
		Value: "nginx",
		Metadata: map[string]string{
			"namespace": "default",
			"kind":      "Deployment",
		},
	}

	if configScreen, ok := model.currentScreen.(*screens.ConfigScreen); ok {
		configScreen.ApplyFilterContext(filterContext)
	}

	// Switch to pods with contextual navigation (this should push history)
	msg = types.ScreenSwitchMsg{
		ScreenID: "pods",
		FilterContext: &types.FilterContext{
			Field: "owner",
			Value: "nginx",
			Metadata: map[string]string{
				"namespace": "default",
				"kind":      "Deployment",
			},
		},
	}
	updatedModel, _ = model.Update(msg)
	model = updatedModel.(Model)

	// Verify history was pushed
	assert.Equal(t, 1, len(model.navigationHistory),
		"History should contain one entry after contextual navigation")

	// Verify the pushed state captured the deployments screen with filter
	assert.Equal(t, "deployments", model.navigationHistory[0].ScreenID,
		"History should contain previous screen ID")
	assert.Equal(t, filterContext, model.navigationHistory[0].FilterContext,
		"History should contain previous filter context")
}

// TestPopNavigationHistory_ReturnsNilWhenEmpty verifies empty history handling
func TestPopNavigationHistory_ReturnsNilWhenEmpty(t *testing.T) {
	repo := k8s.NewDummyRepository()
	theme := ui.ThemeCharm()
	model := NewModel(repo, theme)

	// Pop from empty history
	cmd := model.popNavigationHistory()

	assert.Nil(t, cmd, "Pop from empty history should return nil")
	assert.Equal(t, 0, len(model.navigationHistory),
		"History should remain empty")
}

// TestPopNavigationHistory_ReturnsScreenSwitchMsg verifies back navigation message
func TestPopNavigationHistory_ReturnsScreenSwitchMsg(t *testing.T) {
	repo := k8s.NewDummyRepository()
	theme := ui.ThemeCharm()
	model := NewModel(repo, theme)

	// Manually push a state to history
	model.navigationHistory = append(model.navigationHistory, NavigationState{
		ScreenID: "deployments",
		FilterContext: &types.FilterContext{
			Field: "owner",
			Value: "nginx",
			Metadata: map[string]string{
				"namespace": "default",
				"kind":      "Deployment",
			},
		},
	})

	// Pop history
	cmd := model.popNavigationHistory()

	assert.NotNil(t, cmd, "Pop should return a command")
	assert.Equal(t, 0, len(model.navigationHistory),
		"History should be empty after pop")

	// Execute the command and verify it returns ScreenSwitchMsg
	msg := cmd()
	switchMsg, ok := msg.(types.ScreenSwitchMsg)
	assert.True(t, ok, "Command should return ScreenSwitchMsg")
	assert.Equal(t, "deployments", switchMsg.ScreenID,
		"Should navigate to previous screen")
	assert.True(t, switchMsg.IsBackNav,
		"IsBackNav flag should be true")
	assert.NotNil(t, switchMsg.FilterContext,
		"Should restore previous filter context")
	assert.Equal(t, "owner", switchMsg.FilterContext.Field,
		"Filter context should match pushed state")
}

// TestScreenSwitchMsg_PushesHistory verifies contextual navigation pushes history
func TestScreenSwitchMsg_PushesHistory(t *testing.T) {
	repo := k8s.NewDummyRepository()
	theme := ui.ThemeCharm()
	model := NewModel(repo, theme)

	// Initial state: pods screen, no filter
	assert.Equal(t, "pods", model.state.CurrentScreen)
	assert.Equal(t, 0, len(model.navigationHistory))

	// Navigate to pods with filter (contextual navigation)
	msg := types.ScreenSwitchMsg{
		ScreenID: "pods",
		FilterContext: &types.FilterContext{
			Field: "owner",
			Value: "nginx",
			Metadata: map[string]string{
				"namespace": "default",
				"kind":      "Deployment",
			},
		},
		IsBackNav: false,
	}
	updatedModel, _ := model.Update(msg)
	model = updatedModel.(Model)

	// Verify history was pushed
	assert.Equal(t, 1, len(model.navigationHistory),
		"History should contain one entry after contextual navigation")
	assert.Equal(t, "pods", model.navigationHistory[0].ScreenID,
		"Previous screen should be pods")
}

// TestScreenSwitchMsg_DoesNotPushHistoryForBackNav verifies IsBackNav prevents double-push
func TestScreenSwitchMsg_DoesNotPushHistoryForBackNav(t *testing.T) {
	repo := k8s.NewDummyRepository()
	theme := ui.ThemeCharm()
	model := NewModel(repo, theme)

	// Navigate with IsBackNav=true (should not push)
	msg := types.ScreenSwitchMsg{
		ScreenID: "deployments",
		FilterContext: &types.FilterContext{
			Field: "owner",
			Value: "nginx",
			Metadata: map[string]string{
				"namespace": "default",
				"kind":      "Deployment",
			},
		},
		IsBackNav: true,
	}
	updatedModel, _ := model.Update(msg)
	model = updatedModel.(Model)

	// Verify history was NOT pushed
	assert.Equal(t, 0, len(model.navigationHistory),
		"History should not be pushed for back navigation")
}

// TestScreenSwitchMsg_DoesNotPushHistoryWithoutFilter verifies explicit nav doesn't push
func TestScreenSwitchMsg_DoesNotPushHistoryWithoutFilter(t *testing.T) {
	repo := k8s.NewDummyRepository()
	theme := ui.ThemeCharm()
	model := NewModel(repo, theme)

	// Navigate without filter (explicit navigation like :pods)
	msg := types.ScreenSwitchMsg{
		ScreenID:      "deployments",
		FilterContext: nil,
		IsBackNav:     false,
	}
	updatedModel, _ := model.Update(msg)
	model = updatedModel.(Model)

	// Verify history was NOT pushed
	assert.Equal(t, 0, len(model.navigationHistory),
		"History should not be pushed for explicit navigation without filter")
}

// TestESCKey_TriggersBackNavigation verifies ESC key pops history
func TestESCKey_TriggersBackNavigation(t *testing.T) {
	repo := k8s.NewDummyRepository()
	theme := ui.ThemeCharm()
	model := NewModel(repo, theme)

	// Manually push a state to history
	model.navigationHistory = append(model.navigationHistory, NavigationState{
		ScreenID:      "deployments",
		FilterContext: nil,
	})

	// Send ESC key
	keyMsg := tea.KeyMsg{Type: tea.KeyEsc}
	updatedModel, cmd := model.Update(keyMsg)
	model = updatedModel.(Model)

	assert.NotNil(t, cmd, "ESC should return a command")
	assert.Equal(t, 0, len(model.navigationHistory),
		"History should be popped")

	// Execute command and verify it's a ScreenSwitchMsg with IsBackNav=true
	msg := cmd()
	switchMsg, ok := msg.(types.ScreenSwitchMsg)
	assert.True(t, ok, "Command should return ScreenSwitchMsg")
	assert.True(t, switchMsg.IsBackNav, "IsBackNav should be true")
	assert.Equal(t, "deployments", switchMsg.ScreenID,
		"Should navigate to previous screen")
}
