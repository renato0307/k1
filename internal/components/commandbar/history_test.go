package commandbar

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewHistory(t *testing.T) {
	h := NewHistory()
	assert.NotNil(t, h)
	assert.True(t, h.IsEmpty())
	assert.Equal(t, 0, h.Size())
}

func TestHistory_Add(t *testing.T) {
	tests := []struct {
		name     string
		commands []string
		want     []string
	}{
		{
			name:     "add single command",
			commands: []string{"/yaml"},
			want:     []string{"/yaml"},
		},
		{
			name:     "add multiple commands",
			commands: []string{"/yaml", "/describe", ":pods"},
			want:     []string{"/yaml", "/describe", ":pods"},
		},
		{
			name:     "ignore empty commands",
			commands: []string{"/yaml", "", "/describe"},
			want:     []string{"/yaml", "/describe"},
		},
		{
			name:     "avoid duplicate of most recent",
			commands: []string{"/yaml", "/yaml", "/describe"},
			want:     []string{"/yaml", "/describe"},
		},
		{
			name:     "allow duplicate if not most recent",
			commands: []string{"/yaml", "/describe", "/yaml"},
			want:     []string{"/yaml", "/describe", "/yaml"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewHistory()
			for _, cmd := range tt.commands {
				h.Add(cmd)
			}
			assert.Equal(t, len(tt.want), h.Size())
			for i, want := range tt.want {
				assert.Equal(t, want, h.entries[i])
			}
		})
	}
}

func TestHistory_Add_MaxSize(t *testing.T) {
	h := NewHistory()

	// Add 150 commands (exceeds max of 100)
	for i := 0; i < 150; i++ {
		h.Add("/cmd" + string(rune(i)))
	}

	// Should keep only last 100
	assert.Equal(t, 100, h.Size())

	// First entry should be command 50 (0-49 were removed)
	assert.Equal(t, "/cmd"+string(rune(50)), h.entries[0])
}

func TestHistory_NavigateUp(t *testing.T) {
	h := NewHistory()
	h.Add("/yaml")
	h.Add("/describe")
	h.Add(":pods")

	// First up: most recent
	cmd, ok := h.NavigateUp()
	assert.True(t, ok)
	assert.Equal(t, ":pods", cmd)

	// Second up: middle
	cmd, ok = h.NavigateUp()
	assert.True(t, ok)
	assert.Equal(t, "/describe", cmd)

	// Third up: oldest
	cmd, ok = h.NavigateUp()
	assert.True(t, ok)
	assert.Equal(t, "/yaml", cmd)

	// Fourth up: stays at oldest
	cmd, ok = h.NavigateUp()
	assert.True(t, ok)
	assert.Equal(t, "/yaml", cmd)
}

func TestHistory_NavigateDown(t *testing.T) {
	h := NewHistory()
	h.Add("/yaml")
	h.Add("/describe")
	h.Add(":pods")

	// Navigate up twice
	h.NavigateUp()
	h.NavigateUp()
	assert.Equal(t, "/describe", h.entries[h.index])

	// Navigate down once
	cmd, ok := h.NavigateDown()
	assert.True(t, ok)
	assert.Equal(t, ":pods", cmd)

	// Navigate down again: reaches end
	cmd, ok = h.NavigateDown()
	assert.False(t, ok)
	assert.Equal(t, "", cmd)
	assert.Equal(t, -1, h.index)
}

func TestHistory_NavigateUpDown_Empty(t *testing.T) {
	h := NewHistory()

	// Navigate up on empty history
	cmd, ok := h.NavigateUp()
	assert.False(t, ok)
	assert.Equal(t, "", cmd)

	// Navigate down on empty history
	cmd, ok = h.NavigateDown()
	assert.False(t, ok)
	assert.Equal(t, "", cmd)
}

func TestHistory_Reset(t *testing.T) {
	h := NewHistory()
	h.Add("/yaml")
	h.Add("/describe")

	// Navigate up
	h.NavigateUp()
	assert.NotEqual(t, -1, h.index)

	// Reset
	h.Reset()
	assert.Equal(t, -1, h.index)

	// After reset, navigate up starts from most recent again
	cmd, ok := h.NavigateUp()
	assert.True(t, ok)
	assert.Equal(t, "/describe", cmd)
}
