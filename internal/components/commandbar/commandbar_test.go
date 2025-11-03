package commandbar

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/renato0307/k1/internal/ui"
)

func TestCommandBar_ViewHints_ShowsTipWhenHidden(t *testing.T) {
	pool := createTestPool(t)
	theme := ui.GetTheme("charm")
	cb := New(pool, theme)

	// Should show first tip when StateHidden
	hints := cb.ViewHints()
	assert.NotEqual(t, "", hints)
	assert.Contains(t, hints, "type to filter")
}

func TestCommandBar_ViewHints_EmptyWhenActive(t *testing.T) {
	pool := createTestPool(t)
	theme := ui.GetTheme("charm")
	cb := New(pool, theme)

	// Set to active state
	cb.state = StateFilter

	hints := cb.ViewHints()
	assert.Equal(t, "", hints)
}

func TestCommandBar_TipRotation(t *testing.T) {
	pool := createTestPool(t)
	theme := ui.GetTheme("charm")
	cb := New(pool, theme)

	// Initial tip index should be 0
	assert.Equal(t, 0, cb.currentTipIndex)

	// Simulate rotation message
	tickMessage := tipRotationMsg(time.Now())
	cb, cmd := cb.Update(tickMessage)

	// Should change to a different tip (random selection)
	assert.NotEqual(t, 0, cb.currentTipIndex, "Should rotate to a different tip")
	assert.GreaterOrEqual(t, cb.currentTipIndex, 0, "Index should be >= 0")
	assert.Less(t, cb.currentTipIndex, len(usageTips), "Index should be < len(usageTips)")

	// Should return command to schedule next rotation
	assert.NotNil(t, cmd)
}

func TestCommandBar_TipRotation_AvoidsDuplicate(t *testing.T) {
	pool := createTestPool(t)
	theme := ui.GetTheme("charm")
	cb := New(pool, theme)

	// Set to a specific tip
	cb.currentTipIndex = 5

	// Rotate multiple times and verify we never get the same tip twice in a row
	for i := 0; i < 10; i++ {
		oldIndex := cb.currentTipIndex
		tickMessage := tipRotationMsg(time.Now())
		cb, _ = cb.Update(tickMessage)

		assert.NotEqual(t, oldIndex, cb.currentTipIndex,
			"Should not show the same tip twice in a row (iteration %d)", i)
	}
}

func TestCommandBar_TipContent(t *testing.T) {
	tests := []struct {
		name          string
		tipIndex      int
		shouldContain string
	}{
		{"original hint", 0, "type to filter"},
		{"actions tip", 1, "Enter on resources"},
		{"yaml shortcut tip", 2, "ctrl+y"},
		{"quit tip", 3, "quit"},
		{"context switching tip", 4, "ctrl+n/p"},
		{"output tip", 5, ":output"},
		{"filter negation tip", 6, "!Running"},
		{"filter fuzzy match tip", 7, "matches any part"},
		{"context flag tip", 9, "-context to load"},
		{"multiple contexts tip", 10, "multiple -context"},
		{"theme flag tip", 11, "-theme"},
		{"refresh tip", 14, "refresh automatically"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := createTestPool(t)
			theme := ui.GetTheme("charm")
			cb := New(pool, theme)

			cb.currentTipIndex = tt.tipIndex

			hints := cb.ViewHints()
			assert.Contains(t, hints, tt.shouldContain)
		})
	}
}

func TestCommandBar_TipsArrayValid(t *testing.T) {
	// Ensure tips array is not empty
	assert.Greater(t, len(usageTips), 0, "Tips array should not be empty")

	// Ensure first tip is original hint
	assert.Contains(t, usageTips[0], "type to filter")

	// Ensure all tips are non-empty
	for i, tip := range usageTips {
		assert.NotEqual(t, "", tip, "Tip at index %d should not be empty", i)
	}
}
