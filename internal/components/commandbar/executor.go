package commandbar

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/renato0307/k1/internal/commands"
	"github.com/renato0307/k1/internal/ui"
)

// Executor manages command execution, confirmation, and LLM preview.
type Executor struct {
	registry *commands.Registry
	theme    *ui.Theme
	width    int

	// Pending command state (for confirmation/preview)
	pendingCommand *commands.Command
	pendingArgs    string
	llmTranslation *commands.MockLLMTranslation
}

// NewExecutor creates a new executor.
func NewExecutor(registry *commands.Registry, theme *ui.Theme, width int) *Executor {
	return &Executor{
		registry:       registry,
		theme:          theme,
		width:          width,
		pendingCommand: nil,
		pendingArgs:    "",
		llmTranslation: nil,
	}
}

// SetWidth updates the executor width.
func (e *Executor) SetWidth(width int) {
	e.width = width
}

// BuildContext creates a CommandContext for command execution.
func (e *Executor) BuildContext(resourceType string, selected map[string]interface{}, args string) commands.CommandContext {
	return commands.CommandContext{
		ResourceType: resourceType,
		Selected:     selected,
		Args:         args,
	}
}

// Execute executes a command by name and category.
// Returns tea.Cmd to execute, or nil if command needs confirmation.
// Sets pending command if confirmation is needed.
func (e *Executor) Execute(cmdName string, category commands.CommandCategory, ctx commands.CommandContext) (tea.Cmd, bool) {
	cmd := e.registry.Get(cmdName, category)
	if cmd == nil {
		return nil, false
	}

	// Check if command needs confirmation
	if cmd.NeedsConfirmation {
		e.pendingCommand = cmd
		e.pendingArgs = ctx.Args
		return nil, true // Needs confirmation
	}

	// Execute command
	if cmd.Execute != nil {
		return cmd.Execute(ctx), false
	}

	return nil, false
}

// ExecutePending executes the pending command with stored args.
// Returns tea.Cmd to execute.
func (e *Executor) ExecutePending(ctx commands.CommandContext) tea.Cmd {
	if e.pendingCommand == nil || e.pendingCommand.Execute == nil {
		return nil
	}

	// Use stored args
	ctx.Args = e.pendingArgs
	cmd := e.pendingCommand.Execute(ctx)

	// Clear pending state
	e.ClearPending()

	return cmd
}

// CancelPending cancels the pending command.
func (e *Executor) CancelPending() {
	e.pendingCommand = nil
	e.pendingArgs = ""
	e.llmTranslation = nil
}

// ClearPending clears the pending command state.
func (e *Executor) ClearPending() {
	e.pendingCommand = nil
	e.pendingArgs = ""
}

// HasPending returns true if there's a pending command.
func (e *Executor) HasPending() bool {
	return e.pendingCommand != nil
}

// GetPendingCommand returns the pending command.
func (e *Executor) GetPendingCommand() *commands.Command {
	return e.pendingCommand
}

// SetLLMTranslation sets the LLM translation result for preview.
func (e *Executor) SetLLMTranslation(translation *commands.MockLLMTranslation) {
	e.llmTranslation = translation
}

// GetLLMTranslation returns the LLM translation result.
func (e *Executor) GetLLMTranslation() *commands.MockLLMTranslation {
	return e.llmTranslation
}

// ClearLLMTranslation clears the LLM translation result.
func (e *Executor) ClearLLMTranslation() {
	e.llmTranslation = nil
}

// ViewConfirmation renders confirmation prompt.
func (e *Executor) ViewConfirmation() string {
	if e.pendingCommand == nil {
		return ""
	}

	// Create styles
	titleStyle := lipgloss.NewStyle().
		Foreground(e.theme.Error).
		Bold(true).
		Width(e.width).
		Padding(0, 1)

	textStyle := lipgloss.NewStyle().
		Foreground(e.theme.Foreground).
		Width(e.width).
		Padding(0, 1)

	hintStyle := lipgloss.NewStyle().
		Foreground(e.theme.Subtle).
		Width(e.width).
		Padding(0, 1)

	// Build confirmation content
	lines := []string{}
	lines = append(lines, titleStyle.Render("⚠ Confirm Action"))
	lines = append(lines, textStyle.Render(""))
	lines = append(lines, textStyle.Render("Command: /"+e.pendingCommand.Name))
	lines = append(lines, textStyle.Render("This action cannot be undone."))
	lines = append(lines, hintStyle.Render("[Enter] Confirm  [ESC] Cancel"))

	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

// ViewLLMPreview renders LLM preview.
func (e *Executor) ViewLLMPreview() string {
	if e.llmTranslation == nil {
		return ""
	}

	// Create styles
	titleStyle := lipgloss.NewStyle().
		Foreground(e.theme.Primary).
		Bold(true).
		Width(e.width).
		Padding(0, 1)

	promptStyle := lipgloss.NewStyle().
		Foreground(e.theme.Dimmed).
		Italic(true).
		Width(e.width).
		Padding(0, 1)

	commandStyle := lipgloss.NewStyle().
		Foreground(e.theme.Success).
		Width(e.width).
		Padding(0, 1)

	explanationStyle := lipgloss.NewStyle().
		Foreground(e.theme.Foreground).
		Width(e.width).
		Padding(0, 1)

	hintStyle := lipgloss.NewStyle().
		Foreground(e.theme.Subtle).
		Width(e.width).
		Padding(0, 1)

	// Build LLM preview content
	lines := []string{}
	lines = append(lines, titleStyle.Render("🤖 AI Command Preview"))
	lines = append(lines, promptStyle.Render("Prompt: "+e.llmTranslation.Prompt))
	lines = append(lines, commandStyle.Render("Command: "+e.llmTranslation.Command))
	lines = append(lines, explanationStyle.Render(e.llmTranslation.Explanation))
	lines = append(lines, hintStyle.Render("[Enter] Execute  [e] Edit  [ESC] Cancel"))

	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

// ViewResult renders result message.
func (e *Executor) ViewResult(message string, success bool) string {
	var color lipgloss.AdaptiveColor
	if success {
		color = e.theme.Success
	} else {
		color = e.theme.Error
	}

	resultStyle := lipgloss.NewStyle().
		Foreground(color).
		Background(e.theme.Background).
		Width(e.width).
		Padding(0, 1).
		Bold(true)

	icon := "✓"
	if !success {
		icon = "✗"
	}

	return resultStyle.Render(icon + " " + message)
}
