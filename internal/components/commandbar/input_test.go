package commandbar

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"

	"github.com/renato0307/k1/internal/commands"
	"github.com/renato0307/k1/internal/ui"
)

func TestNewInput(t *testing.T) {
	pool := createTestPool(t)
	registry := commands.NewRegistry(pool)
	theme := ui.GetTheme("charm")

	input := NewInput(registry, theme, 80)
	assert.NotNil(t, input)
	assert.True(t, input.IsEmpty())
	assert.Equal(t, "", input.Get())
}

func TestInput_AddChar(t *testing.T) {
	pool := createTestPool(t)
	registry := commands.NewRegistry(pool)
	theme := ui.GetTheme("charm")

	input := NewInput(registry, theme, 80)
	input.AddChar("a")
	assert.Equal(t, "a", input.Get())
	assert.Equal(t, 1, input.cursorPos)

	input.AddChar("b")
	assert.Equal(t, "ab", input.Get())
	assert.Equal(t, 2, input.cursorPos)
}

func TestInput_AddText(t *testing.T) {
	pool := createTestPool(t)
	registry := commands.NewRegistry(pool)
	theme := ui.GetTheme("charm")

	input := NewInput(registry, theme, 80)
	input.AddText("hello world")
	assert.Equal(t, "hello world", input.Get())
	assert.Equal(t, 11, input.cursorPos)
}

func TestInput_Backspace(t *testing.T) {
	pool := createTestPool(t)
	registry := commands.NewRegistry(pool)
	theme := ui.GetTheme("charm")

	input := NewInput(registry, theme, 80)
	input.Set("abc")

	isEmpty := input.Backspace()
	assert.False(t, isEmpty)
	assert.Equal(t, "ab", input.Get())

	isEmpty = input.Backspace()
	assert.False(t, isEmpty)
	assert.Equal(t, "a", input.Get())

	isEmpty = input.Backspace()
	assert.True(t, isEmpty)
	assert.Equal(t, "", input.Get())

	// Backspace on empty buffer
	isEmpty = input.Backspace()
	assert.True(t, isEmpty)
	assert.Equal(t, "", input.Get())
}

func TestInput_Clear(t *testing.T) {
	pool := createTestPool(t)
	registry := commands.NewRegistry(pool)
	theme := ui.GetTheme("charm")

	input := NewInput(registry, theme, 80)
	input.Set("test")

	assert.False(t, input.IsEmpty())
	input.Clear()
	assert.True(t, input.IsEmpty())
	assert.Equal(t, "", input.Get())
	assert.Equal(t, 0, input.cursorPos)
}

func TestInput_Set(t *testing.T) {
	pool := createTestPool(t)
	registry := commands.NewRegistry(pool)
	theme := ui.GetTheme("charm")

	input := NewInput(registry, theme, 80)
	input.Set("hello")

	assert.Equal(t, "hello", input.Get())
	assert.Equal(t, 5, input.cursorPos)
}

func TestInput_HandleKeyMsg(t *testing.T) {
	pool := createTestPool(t)
	registry := commands.NewRegistry(pool)
	theme := ui.GetTheme("charm")

	input := NewInput(registry, theme, 80)

	// Test character input
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}}
	result := input.HandleKeyMsg(msg)
	assert.Equal(t, InputActionChar, result.Action)
	assert.Equal(t, "a", result.Text)

	// Test backspace
	msg = tea.KeyMsg{Type: tea.KeyBackspace}
	result = input.HandleKeyMsg(msg)
	assert.Equal(t, InputActionBackspace, result.Action)

	// Test paste
	msg = tea.KeyMsg{Paste: true, Runes: []rune{'h', 'e', 'l', 'l', 'o'}}
	result = input.HandleKeyMsg(msg)
	assert.Equal(t, InputActionPaste, result.Action)
	assert.Equal(t, "hello", result.Text)
}

func TestInput_ParseCommand(t *testing.T) {
	pool := createTestPool(t)
	registry := commands.NewRegistry(pool)
	theme := ui.GetTheme("charm")

	tests := []struct {
		name       string
		input      string
		wantPrefix string
		wantCmd    string
		wantArgs   string
	}{
		{
			name:       "resource command without args",
			input:      ":pods",
			wantPrefix: ":",
			wantCmd:    "pods",
			wantArgs:   "",
		},
		{
			name:       "action command without args",
			input:      "/yaml",
			wantPrefix: "/",
			wantCmd:    "yaml",
			wantArgs:   "",
		},
		{
			name:       "action command with args",
			input:      "/scale 5",
			wantPrefix: "/",
			wantCmd:    "scale",
			wantArgs:   "5",
		},
		{
			name:       "command with multiple args",
			input:      "/logs nginx 100",
			wantPrefix: "/",
			wantCmd:    "logs",
			wantArgs:   "nginx 100",
		},
		{
			name:       "empty input",
			input:      "",
			wantPrefix: "",
			wantCmd:    "",
			wantArgs:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := NewInput(registry, theme, 80)
			input.Set(tt.input)

			prefix, cmd, args := input.ParseCommand()
			assert.Equal(t, tt.wantPrefix, prefix)
			assert.Equal(t, tt.wantCmd, cmd)
			assert.Equal(t, tt.wantArgs, args)
		})
	}
}

func TestInput_GetArgumentHint(t *testing.T) {
	pool := createTestPool(t)
	registry := commands.NewRegistry(pool)
	theme := ui.GetTheme("charm")

	input := NewInput(registry, theme, 80)

	// No hint for filter mode (no prefix)
	input.Set("foo")
	hint := input.GetArgumentHint(CommandTypeFilter)
	assert.Equal(t, "", hint)

	// No hint for empty input
	input.Clear()
	hint = input.GetArgumentHint(CommandTypeAction)
	assert.Equal(t, "", hint)

	// Note: Testing with actual commands requires registry to have commands
	// with ArgPattern. For now, just test the basic cases above.
	// Full integration test would need a registry with test commands.
}

func TestInput_View(t *testing.T) {
	pool := createTestPool(t)
	registry := commands.NewRegistry(pool)
	theme := ui.GetTheme("charm")

	input := NewInput(registry, theme, 80)
	input.Set("test")

	view := input.View(CommandTypeFilter)
	assert.Contains(t, view, "test")
	assert.Contains(t, view, "█") // Cursor
}

func TestInput_SetWidth(t *testing.T) {
	pool := createTestPool(t)
	registry := commands.NewRegistry(pool)
	theme := ui.GetTheme("charm")

	input := NewInput(registry, theme, 80)
	assert.Equal(t, 80, input.width)

	input.SetWidth(120)
	assert.Equal(t, 120, input.width)
}
