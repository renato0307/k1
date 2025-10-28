package commandbar

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/renato0307/k1/internal/commands"
	"github.com/renato0307/k1/internal/ui"
)

func TestNewPalette(t *testing.T) {
	pool := createTestPool(t)
	registry := commands.NewRegistry(pool)
	theme := ui.GetTheme("charm")

	p := NewPalette(registry, theme, 80)
	assert.NotNil(t, p)
	assert.True(t, p.IsEmpty())
	assert.Equal(t, 0, p.Size())
	assert.Equal(t, 0, p.GetHeight())
}

func TestPalette_Filter_Resource(t *testing.T) {
	pool := createTestPool(t)
	registry := commands.NewRegistry(pool)
	theme := ui.GetTheme("charm")

	p := NewPalette(registry, theme, 80)

	// Filter with empty query returns all resource commands
	p.Filter("", CommandTypeResource, "pods")
	assert.False(t, p.IsEmpty())
	assert.Greater(t, p.Size(), 0)

	// Filter with query
	p.Filter("pod", CommandTypeResource, "pods")
	foundPods := false
	for _, item := range p.items {
		if item.Name == "pods" {
			foundPods = true
			break
		}
	}
	assert.True(t, foundPods, "Expected 'pods' command in filtered results")
}

func TestPalette_Filter_Action(t *testing.T) {
	pool := createTestPool(t)
	registry := commands.NewRegistry(pool)
	theme := ui.GetTheme("charm")

	p := NewPalette(registry, theme, 80)

	// Filter with empty query returns all action commands + ai
	p.Filter("", CommandTypeAction, "pods")
	assert.False(t, p.IsEmpty())

	// Should include /ai command
	foundAI := false
	for _, item := range p.items {
		if item.Name == "ai" {
			foundAI = true
			break
		}
	}
	assert.True(t, foundAI, "Expected 'ai' command in action commands")
}

func TestPalette_NavigateUpDown(t *testing.T) {
	pool := createTestPool(t)
	registry := commands.NewRegistry(pool)
	theme := ui.GetTheme("charm")

	p := NewPalette(registry, theme, 80)
	p.Filter("", CommandTypeResource, "pods")

	initialSize := p.Size()
	assert.Greater(t, initialSize, 2, "Need at least 3 items for navigation test")

	// Initial index is 0
	assert.Equal(t, 0, p.index)

	// Navigate down
	p.NavigateDown()
	assert.Equal(t, 1, p.index)

	p.NavigateDown()
	assert.Equal(t, 2, p.index)

	// Navigate up
	p.NavigateUp()
	assert.Equal(t, 1, p.index)

	p.NavigateUp()
	assert.Equal(t, 0, p.index)

	// Navigate up at top (should stay at 0)
	p.NavigateUp()
	assert.Equal(t, 0, p.index)

	// Navigate to bottom
	for i := 0; i < initialSize; i++ {
		p.NavigateDown()
	}
	// Should be at last index
	assert.Equal(t, initialSize-1, p.index)

	// Navigate down at bottom (should stay at last)
	p.NavigateDown()
	assert.Equal(t, initialSize-1, p.index)
}

func TestPalette_GetSelected(t *testing.T) {
	pool := createTestPool(t)
	registry := commands.NewRegistry(pool)
	theme := ui.GetTheme("charm")

	p := NewPalette(registry, theme, 80)

	// Empty palette returns nil
	selected := p.GetSelected()
	assert.Nil(t, selected)

	// Filter and get selected
	p.Filter("", CommandTypeResource, "pods")
	assert.False(t, p.IsEmpty())

	selected = p.GetSelected()
	assert.NotNil(t, selected)

	// Navigate and verify selection changes
	firstSelected := selected.Name
	p.NavigateDown()
	secondSelected := p.GetSelected()
	assert.NotNil(t, secondSelected)
	if p.Size() > 1 {
		assert.NotEqual(t, firstSelected, secondSelected.Name)
	}
}

func TestPalette_Reset(t *testing.T) {
	pool := createTestPool(t)
	registry := commands.NewRegistry(pool)
	theme := ui.GetTheme("charm")

	p := NewPalette(registry, theme, 80)
	p.Filter("", CommandTypeResource, "pods")
	p.NavigateDown()

	assert.False(t, p.IsEmpty())
	assert.NotEqual(t, 0, p.index)

	p.Reset()
	assert.True(t, p.IsEmpty())
	assert.Equal(t, 0, p.index)
}

func TestPalette_SetWidth(t *testing.T) {
	pool := createTestPool(t)
	registry := commands.NewRegistry(pool)
	theme := ui.GetTheme("charm")

	p := NewPalette(registry, theme, 80)
	assert.Equal(t, 80, p.width)

	p.SetWidth(120)
	assert.Equal(t, 120, p.width)
}

func TestPalette_View(t *testing.T) {
	pool := createTestPool(t)
	registry := commands.NewRegistry(pool)
	theme := ui.GetTheme("charm")

	p := NewPalette(registry, theme, 80)

	// Empty palette returns empty string
	view := p.View(":")
	assert.Equal(t, "", view)

	// Filter and verify view is not empty
	p.Filter("", CommandTypeResource, "pods")
	view = p.View(":")
	assert.NotEqual(t, "", view)
	assert.Contains(t, view, "▶") // Should contain selection indicator
}

func TestPalette_ScrollingBehavior(t *testing.T) {
	pool := createTestPool(t)
	registry := commands.NewRegistry(pool)
	theme := ui.GetTheme("charm")

	p := NewPalette(registry, theme, 80)

	// Create 15 test items (more than MaxPaletteItems which is 8)
	items := make([]commands.Command, 15)
	for i := 0; i < 15; i++ {
		items[i] = commands.Command{
			Name:        fmt.Sprintf("cmd%d", i),
			Description: fmt.Sprintf("Command %d", i),
		}
	}
	p.items = items
	p.index = 0
	p.scrollOffset = 0

	require.Greater(t, len(p.items), MaxPaletteItems,
		"Need more than 8 items for scroll test")

	// Initial state
	assert.Equal(t, 0, p.index, "Initial index should be 0")
	assert.Equal(t, 0, p.scrollOffset, "Initial scrollOffset should be 0")

	// Navigate down 7 times (to index 7, last visible in viewport 0-7)
	for i := 0; i < 7; i++ {
		p.NavigateDown()
	}
	assert.Equal(t, 7, p.index)
	assert.Equal(t, 0, p.scrollOffset, "Should not scroll yet")

	// Navigate down once more (to index 8, triggers scroll)
	p.NavigateDown()
	assert.Equal(t, 8, p.index)
	assert.Equal(t, 1, p.scrollOffset, "Should scroll down by 1")

	// Navigate down several more times
	for i := 0; i < 5; i++ {
		p.NavigateDown()
	}
	assert.Equal(t, 13, p.index)
	assert.Equal(t, 6, p.scrollOffset, "Should scroll to keep cursor visible")

	// Navigate back up
	p.NavigateUp()
	assert.Equal(t, 12, p.index)
	assert.Equal(t, 6, p.scrollOffset, "Should not scroll yet")

	// Navigate up to trigger scroll
	for i := 0; i < 7; i++ {
		p.NavigateUp()
	}
	assert.Equal(t, 5, p.index)
	assert.Equal(t, 5, p.scrollOffset, "Should scroll up")

	// Continue up to top
	for i := 0; i < 5; i++ {
		p.NavigateUp()
	}
	assert.Equal(t, 0, p.index)
	assert.Equal(t, 0, p.scrollOffset, "Should be at top")
}

func TestPalette_ScrollResetOnFilter(t *testing.T) {
	pool := createTestPool(t)
	registry := commands.NewRegistry(pool)
	theme := ui.GetTheme("charm")

	p := NewPalette(registry, theme, 80)

	// Create 15 test items
	items := make([]commands.Command, 15)
	for i := 0; i < 15; i++ {
		items[i] = commands.Command{
			Name:        fmt.Sprintf("cmd%d", i),
			Description: fmt.Sprintf("Command %d", i),
			Category:    commands.CategoryAction,
		}
	}
	p.items = items
	p.index = 0
	p.scrollOffset = 0

	// Navigate down to create scroll offset
	for i := 0; i < 10; i++ {
		p.NavigateDown()
	}

	assert.Equal(t, 10, p.index)
	assert.Greater(t, p.scrollOffset, 0, "Should have scrolled")

	// Filter again (this resets items, index, and scrollOffset)
	p.Filter("delete", CommandTypeAction, "pods")

	// Should reset
	assert.Equal(t, 0, p.index, "Index should reset on filter")
	assert.Equal(t, 0, p.scrollOffset, "ScrollOffset should reset on filter")
}

func TestPalette_BoundaryConditions(t *testing.T) {
	pool := createTestPool(t)
	registry := commands.NewRegistry(pool)
	theme := ui.GetTheme("charm")

	p := NewPalette(registry, theme, 80)

	// Test with fewer items than MaxPaletteItems
	p.items = []commands.Command{
		{Name: "cmd1", Description: "Command 1"},
		{Name: "cmd2", Description: "Command 2"},
		{Name: "cmd3", Description: "Command 3"},
	}
	p.index = 0
	p.scrollOffset = 0

	// Navigate down to bottom
	p.NavigateDown()
	p.NavigateDown()
	assert.Equal(t, 2, p.index)
	assert.Equal(t, 0, p.scrollOffset, "No scroll needed with few items")

	// Try to go past bottom
	p.NavigateDown()
	assert.Equal(t, 2, p.index, "Should stop at last item")
	assert.Equal(t, 0, p.scrollOffset)

	// Navigate up to top
	p.NavigateUp()
	p.NavigateUp()
	assert.Equal(t, 0, p.index)

	// Try to go past top
	p.NavigateUp()
	assert.Equal(t, 0, p.index, "Should stop at first item")
	assert.Equal(t, 0, p.scrollOffset)
}

func TestPalette_ViewRenderingWithScroll(t *testing.T) {
	pool := createTestPool(t)
	registry := commands.NewRegistry(pool)
	theme := ui.GetTheme("charm")

	p := NewPalette(registry, theme, 80)

	// Create 15 test items
	items := make([]commands.Command, 15)
	for i := 0; i < 15; i++ {
		items[i] = commands.Command{
			Name:        fmt.Sprintf("cmd%d", i),
			Description: fmt.Sprintf("Command %d", i),
		}
	}
	p.items = items
	p.index = 0
	p.scrollOffset = 0

	// Initial view should show cmd0-cmd7
	view := p.View("/")
	assert.Contains(t, view, "cmd0")
	assert.Contains(t, view, "cmd7")
	assert.NotContains(t, view, "cmd8")

	// Scroll to middle (offset=5, shows items 5-12)
	p.index = 8
	p.scrollOffset = 5
	view = p.View("/")
	assert.NotContains(t, view, "cmd4")
	assert.Contains(t, view, "cmd5")
	assert.Contains(t, view, "cmd12")
	assert.NotContains(t, view, "cmd13")
	assert.Contains(t, view, "▶", "Selected item should have indicator")
}
