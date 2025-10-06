package commandbar

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/renato0307/k1/internal/commands"
	"github.com/renato0307/k1/internal/k8s"
	"github.com/renato0307/k1/internal/ui"
)

func TestNewPalette(t *testing.T) {
	repo := k8s.NewDummyRepository()
	registry := commands.NewRegistry(repo)
	theme := ui.GetTheme("charm")

	p := NewPalette(registry, theme, 80)
	assert.NotNil(t, p)
	assert.True(t, p.IsEmpty())
	assert.Equal(t, 0, p.Size())
	assert.Equal(t, 0, p.GetHeight())
}

func TestPalette_Filter_Resource(t *testing.T) {
	repo := k8s.NewDummyRepository()
	registry := commands.NewRegistry(repo)
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
	repo := k8s.NewDummyRepository()
	registry := commands.NewRegistry(repo)
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
	repo := k8s.NewDummyRepository()
	registry := commands.NewRegistry(repo)
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
	repo := k8s.NewDummyRepository()
	registry := commands.NewRegistry(repo)
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
	repo := k8s.NewDummyRepository()
	registry := commands.NewRegistry(repo)
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
	repo := k8s.NewDummyRepository()
	registry := commands.NewRegistry(repo)
	theme := ui.GetTheme("charm")

	p := NewPalette(registry, theme, 80)
	assert.Equal(t, 80, p.width)

	p.SetWidth(120)
	assert.Equal(t, 120, p.width)
}

func TestPalette_View(t *testing.T) {
	repo := k8s.NewDummyRepository()
	registry := commands.NewRegistry(repo)
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
