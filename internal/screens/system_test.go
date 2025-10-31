package screens

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/renato0307/k1/internal/k8s"
	"github.com/renato0307/k1/internal/types"
	"github.com/renato0307/k1/internal/ui"
)

func TestNewSystemScreen(t *testing.T) {
	repo := k8s.NewDummyRepository()
	theme := ui.GetTheme("charm")

	screen := NewSystemScreen(repo, theme)

	assert.NotNil(t, screen)
	assert.Equal(t, "system-resources", screen.ID())
	assert.Equal(t, "System Resources", screen.Title())
	assert.NotEmpty(t, screen.HelpText())
}

func TestSystemScreen_Refresh(t *testing.T) {
	repo := k8s.NewDummyRepository()
	theme := ui.GetTheme("charm")
	screen := NewSystemScreen(repo, theme)

	// Call refresh
	cmd := screen.refresh()
	require.NotNil(t, cmd)

	// Execute the command
	msg := cmd()
	refreshMsg, ok := msg.(types.RefreshCompleteMsg)
	require.True(t, ok, "Expected RefreshCompleteMsg")
	assert.Greater(t, refreshMsg.Duration, time.Duration(0))

	// Verify table has rows (11 resources + separator + totals = 13 rows)
	rows := screen.table.Rows()
	assert.Equal(t, 13, len(rows))
}

func TestSystemScreen_RefreshWithTotals(t *testing.T) {
	repo := k8s.NewDummyRepository()
	theme := ui.GetTheme("charm")
	screen := NewSystemScreen(repo, theme)

	// Refresh to populate data
	cmd := screen.refresh()
	msg := cmd()
	_, ok := msg.(types.RefreshCompleteMsg)
	require.True(t, ok)

	// Check that we have the expected number of rows
	// 11 resources + 1 separator + 1 totals = 13 rows
	rows := screen.table.Rows()
	assert.Equal(t, 13, len(rows))

	// Verify last row is totals (by checking it has "TOTAL" in first column)
	lastRow := rows[len(rows)-1]
	assert.Equal(t, "TOTAL", lastRow[0])
}

func TestSystemScreen_Init(t *testing.T) {
	repo := k8s.NewDummyRepository()
	theme := ui.GetTheme("charm")
	screen := NewSystemScreen(repo, theme)

	cmd := screen.Init()
	assert.NotNil(t, cmd)
}

func TestSystemScreen_Update_WindowSize(t *testing.T) {
	repo := k8s.NewDummyRepository()
	theme := ui.GetTheme("charm")
	screen := NewSystemScreen(repo, theme)

	msg := tea.WindowSizeMsg{Width: 100, Height: 50}
	model, cmd := screen.Update(msg)

	updatedScreen := model.(*SystemScreen)
	assert.Equal(t, 100, updatedScreen.width)
	assert.Equal(t, 50, updatedScreen.height)
	assert.Nil(t, cmd)
}

func TestSystemScreen_Update_RefreshComplete(t *testing.T) {
	repo := k8s.NewDummyRepository()
	theme := ui.GetTheme("charm")
	screen := NewSystemScreen(repo, theme)

	before := screen.lastRefresh
	time.Sleep(10 * time.Millisecond)

	msg := types.RefreshCompleteMsg{Duration: 5 * time.Millisecond}
	model, cmd := screen.Update(msg)

	updatedScreen := model.(*SystemScreen)
	assert.True(t, updatedScreen.lastRefresh.After(before))
	assert.Nil(t, cmd)
}

func TestSystemScreen_Update_TickMsg(t *testing.T) {
	repo := k8s.NewDummyRepository()
	theme := ui.GetTheme("charm")
	screen := NewSystemScreen(repo, theme)

	msg := tickMsg{screenID: screen.ID(), time: time.Now()}
	model, cmd := screen.Update(msg)

	assert.NotNil(t, model)
	assert.NotNil(t, cmd)
}

func TestSystemScreen_Update_EscKey(t *testing.T) {
	repo := k8s.NewDummyRepository()
	theme := ui.GetTheme("charm")
	screen := NewSystemScreen(repo, theme)

	msg := tea.KeyMsg{Type: tea.KeyEsc}
	_, cmd := screen.Update(msg)

	require.NotNil(t, cmd)

	// Execute command and verify it returns ScreenSwitchMsg to pods
	resultMsg := cmd()
	switchMsg, ok := resultMsg.(types.ScreenSwitchMsg)
	require.True(t, ok)
	assert.Equal(t, "pods", switchMsg.ScreenID)
}

func TestSystemScreen_Update_QuitKey(t *testing.T) {
	repo := k8s.NewDummyRepository()
	theme := ui.GetTheme("charm")
	screen := NewSystemScreen(repo, theme)

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}
	_, cmd := screen.Update(msg)

	assert.NotNil(t, cmd)
}

func TestSystemScreen_View(t *testing.T) {
	repo := k8s.NewDummyRepository()
	theme := ui.GetTheme("charm")
	screen := NewSystemScreen(repo, theme)

	// Refresh first to populate data
	cmd := screen.refresh()
	cmd()

	view := screen.View()
	assert.NotEmpty(t, view)
}

func TestSystemScreen_SetSize(t *testing.T) {
	repo := k8s.NewDummyRepository()
	theme := ui.GetTheme("charm")
	screen := NewSystemScreen(repo, theme)

	screen.SetSize(120, 60)

	assert.Equal(t, 120, screen.width)
	assert.Equal(t, 60, screen.height)
	// Note: table.Height() includes internal padding, so we just verify SetSize was called
	assert.Greater(t, screen.table.Height(), 0)
}

func TestSystemScreen_Operations(t *testing.T) {
	repo := k8s.NewDummyRepository()
	theme := ui.GetTheme("charm")
	screen := NewSystemScreen(repo, theme)

	ops := screen.Operations()
	assert.Empty(t, ops, "System screen should have no operations")
}

func TestSystemScreen_GetSelectedResource(t *testing.T) {
	repo := k8s.NewDummyRepository()
	theme := ui.GetTheme("charm")
	screen := NewSystemScreen(repo, theme)

	resource := screen.GetSelectedResource()
	assert.Nil(t, resource, "System screen should return nil for selected resource")
}

func TestSystemScreen_FilterContext(t *testing.T) {
	repo := k8s.NewDummyRepository()
	theme := ui.GetTheme("charm")
	screen := NewSystemScreen(repo, theme)

	// ApplyFilterContext should be a no-op
	screen.ApplyFilterContext(&types.FilterContext{Field: "test", Value: "test"})

	// GetFilterContext should return nil
	ctx := screen.GetFilterContext()
	assert.Nil(t, ctx)
}
