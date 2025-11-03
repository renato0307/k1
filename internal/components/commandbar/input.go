package commandbar

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/renato0307/k1/internal/commands"
	"github.com/renato0307/k1/internal/ui"
)

// Input manages input buffer, cursor position, and keystroke handling.
type Input struct {
	buffer    string
	cursorPos int
	registry  *commands.Registry
	theme     *ui.Theme
	width     int
}

// NewInput creates a new input manager.
func NewInput(registry *commands.Registry, theme *ui.Theme, width int) *Input {
	return &Input{
		buffer:    "",
		cursorPos: 0,
		registry:  registry,
		theme:     theme,
		width:     width,
	}
}

// SetWidth updates the input width.
func (i *Input) SetWidth(width int) {
	i.width = width
}

// Get returns the current input buffer.
func (i *Input) Get() string {
	return i.buffer
}

// Set sets the input buffer and cursor position.
func (i *Input) Set(text string) {
	i.buffer = text
	i.cursorPos = len(text)
}

// Clear clears the input buffer and resets cursor.
func (i *Input) Clear() {
	i.buffer = ""
	i.cursorPos = 0
}

// IsEmpty returns true if input buffer is empty.
func (i *Input) IsEmpty() bool {
	return len(i.buffer) == 0
}

// AddChar adds a character to the input buffer.
func (i *Input) AddChar(ch string) {
	i.buffer += ch
	i.cursorPos++
}

// AddText adds text to the input buffer (for paste events).
func (i *Input) AddText(text string) {
	i.buffer += text
	i.cursorPos += len(text)
}

// Backspace removes the last character from input buffer.
// Returns true if buffer is now empty.
func (i *Input) Backspace() bool {
	if len(i.buffer) > 0 {
		i.buffer = i.buffer[:len(i.buffer)-1]
		i.cursorPos--
		if i.cursorPos < 0 {
			i.cursorPos = 0
		}
	}
	return len(i.buffer) == 0
}

// HandleKeyMsg handles keyboard input messages.
// Returns the resulting action to take.
type InputAction int

const (
	InputActionNone InputAction = iota
	InputActionChar
	InputActionBackspace
	InputActionPaste
)

// KeyMsgResult represents the result of handling a key message.
type KeyMsgResult struct {
	Action InputAction
	Text   string
}

// HandleKeyMsg processes a keyboard message and returns the action.
func (i *Input) HandleKeyMsg(msg tea.KeyMsg) KeyMsgResult {
	// Handle paste events
	if msg.Paste {
		return KeyMsgResult{
			Action: InputActionPaste,
			Text:   string(msg.Runes),
		}
	}

	switch msg.String() {
	case "backspace":
		return KeyMsgResult{Action: InputActionBackspace}
	default:
		// Single character input
		if len(msg.String()) == 1 {
			return KeyMsgResult{
				Action: InputActionChar,
				Text:   msg.String(),
			}
		}
	}

	return KeyMsgResult{Action: InputActionNone}
}

// ParseCommand parses the input buffer into command name and args.
// Returns prefix, command name, and args.
// Example: ":pods" -> ":", "pods", ""
// Example: "/scale 5" -> "/", "scale", "5"
func (i *Input) ParseCommand() (prefix, cmdName, args string) {
	if len(i.buffer) == 0 {
		return "", "", ""
	}

	prefix = i.buffer[:1]
	rest := i.buffer[1:]

	// Split command name and args by whitespace
	parts := strings.SplitN(rest, " ", 2)
	cmdName = parts[0]
	if len(parts) > 1 {
		args = parts[1]
	}

	return prefix, cmdName, args
}

// GetArgumentHint returns the argument pattern hint for the current input.
// Shows remaining args as user types: "/logs " → "[container] [tail] [follow]"
//
//	"/logs nginx " → "[tail] [follow]"
func (i *Input) GetArgumentHint(cmdType CommandType) string {
	// Only show hints for command inputs (: or /)
	if len(i.buffer) == 0 {
		return ""
	}

	prefix := i.buffer[:1]
	if prefix != ":" && prefix != "/" {
		return ""
	}

	// Parse command name and args from input
	parts := strings.Fields(i.buffer)
	if len(parts) == 0 {
		return ""
	}

	// Extract command name (without prefix)
	cmdNameWithPrefix := parts[0]
	cmdName := strings.TrimPrefix(cmdNameWithPrefix, ":")
	cmdName = strings.TrimPrefix(cmdName, "/")

	// Determine command category
	var category commands.CommandCategory
	if prefix == ":" {
		category = commands.CategoryResource
	} else if strings.HasPrefix(i.buffer, "/ai ") {
		category = commands.CategoryLLMAction
	} else {
		category = commands.CategoryAction
	}

	// Look up command in registry
	cmd := i.registry.Get(cmdName, category)
	if cmd == nil || cmd.ArgPattern == "" {
		return ""
	}

	// Parse arg pattern to extract individual placeholders
	// ArgPattern format: " <required> [optional1] [optional2]"
	argPattern := strings.TrimSpace(cmd.ArgPattern)
	if argPattern == "" {
		return ""
	}

	// Split pattern into individual arg placeholders
	// Handle both <required> and [optional] format
	argPlaceholders := []string{}
	inBracket := false
	currentArg := ""
	for _, ch := range argPattern {
		if ch == '<' || ch == '[' {
			inBracket = true
			currentArg = string(ch)
		} else if ch == '>' || ch == ']' {
			currentArg += string(ch)
			argPlaceholders = append(argPlaceholders, currentArg)
			currentArg = ""
			inBracket = false
		} else if inBracket {
			currentArg += string(ch)
		}
	}

	// Count how many args user has already typed (exclude command name)
	typedArgsCount := len(parts) - 1

	// If user is in the middle of typing an arg (no trailing space), don't show hint yet
	// Only show hint when there's a trailing space (ready for next arg)
	if !strings.HasSuffix(i.buffer, " ") && typedArgsCount > 0 {
		return ""
	}

	// Show remaining args
	if typedArgsCount < len(argPlaceholders) {
		remaining := argPlaceholders[typedArgsCount:]
		return " " + strings.Join(remaining, " ")
	}

	// All args typed
	return ""
}

// View renders the input with cursor and optional argument hint.
func (i *Input) View(cmdType CommandType) string {
	barStyle := lipgloss.NewStyle().
		Foreground(i.theme.Foreground).
		Width(i.width).
		Padding(0, 1)

	// Show input with cursor
	display := i.buffer + "█"

	// Add argument hint if applicable
	hint := i.GetArgumentHint(cmdType)
	if hint != "" {
		hintStyle := lipgloss.NewStyle().
			Foreground(i.theme.Dimmed).
			Italic(true)
		display += hintStyle.Render(hint)
	}

	return barStyle.Render(display)
}
