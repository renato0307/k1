package commandbar

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/renato0307/k1/internal/commands"
	"github.com/renato0307/k1/internal/k8s"
	"github.com/renato0307/k1/internal/types"
	"github.com/renato0307/k1/internal/ui"
)

// CommandBar coordinates all command bar components and manages state machine.
type CommandBar struct {
	// State
	state     CommandBarState
	inputType CommandType
	width     int
	height    int
	theme     *ui.Theme

	// Context
	screenID         string
	selectedResource map[string]any

	// Components
	history  *History
	palette  *Palette
	input    *Input
	executor *Executor
	registry *commands.Registry
}

// New creates a new command bar coordinator.
func New(pool *k8s.RepositoryPool, theme *ui.Theme) *CommandBar {
	registry := commands.NewRegistry(pool)

	return &CommandBar{
		state:     StateHidden,
		inputType: CommandTypeFilter,
		width:     80,
		height:    1,
		theme:     theme,
		history:   NewHistory(),
		palette:   NewPalette(registry, theme, 80),
		input:     NewInput(registry, theme, 80),
		executor:  NewExecutor(registry, theme, 80),
		registry:  registry,
	}
}

// SetWidth updates component widths.
func (cb *CommandBar) SetWidth(width int) {
	cb.width = width
	cb.palette.SetWidth(width)
	cb.input.SetWidth(width)
	cb.executor.SetWidth(width)
}

// SetScreen updates the current screen context.
func (cb *CommandBar) SetScreen(screenID string) {
	cb.screenID = screenID
}

// SetSelectedResource updates the selected resource for command execution.
func (cb *CommandBar) SetSelectedResource(resource map[string]any) {
	cb.selectedResource = resource
}

// GetHeight returns the current height (including separators, not hints).
func (cb *CommandBar) GetHeight() int {
	if cb.state == StateHidden {
		return 0
	}
	// Add 2 for: top separator (1) + bottom separator (1)
	return cb.height + 2
}

// GetTotalHeight returns the height including hints line.
func (cb *CommandBar) GetTotalHeight() int {
	baseHeight := cb.GetHeight()

	// Hints are only shown in StateHidden (3 lines: separator + text + separator)
	if cb.state == StateHidden {
		return baseHeight + 3
	}

	return baseHeight
}

// GetState returns the current state.
func (cb *CommandBar) GetState() CommandBarState {
	return cb.state
}

// GetInput returns the current input string.
func (cb *CommandBar) GetInput() string {
	return cb.input.Get()
}

// GetInputType returns the current command type.
func (cb *CommandBar) GetInputType() CommandType {
	return cb.inputType
}

// RestoreFilter restores filter state (for back navigation).
// Sets input text and transitions to StateFilter, returning a FilterUpdateMsg.
func (cb *CommandBar) RestoreFilter(filter string) tea.Cmd {
	if filter == "" {
		return nil
	}

	cb.input.Set(filter)
	cb.state = StateFilter
	cb.inputType = CommandTypeFilter

	return func() tea.Msg {
		return types.FilterUpdateMsg{Filter: filter}
	}
}

// IsActive returns true if the command bar is accepting input.
func (cb *CommandBar) IsActive() bool {
	return cb.state != StateHidden && cb.state != StateResult
}

// Update handles messages for the command bar.
func (cb *CommandBar) Update(msg tea.Msg) (*CommandBar, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return cb.handleKeyMsg(msg)
	}

	return cb, nil
}

// handleKeyMsg routes keyboard input to appropriate state handler.
func (cb *CommandBar) handleKeyMsg(msg tea.KeyMsg) (*CommandBar, tea.Cmd) {
	switch cb.state {
	case StateHidden:
		return cb.handleHiddenState(msg)
	case StateFilter:
		return cb.handleFilterState(msg)
	case StateSuggestionPalette:
		return cb.handlePaletteState(msg)
	case StateInput:
		return cb.handleInputState(msg)
	case StateConfirmation:
		return cb.handleConfirmationState(msg)
	case StateLLMPreview:
		return cb.handleLLMPreviewState(msg)
	case StateResult:
		return cb.handleResultState(msg)
	}

	return cb, nil
}

// handleHiddenState handles input when command bar is hidden.
func (cb *CommandBar) handleHiddenState(msg tea.KeyMsg) (*CommandBar, tea.Cmd) {
	// Handle ESC when there's an active filter (clear it)
	if msg.String() == "esc" && cb.input.Get() != "" {
		cb.input.Clear()
		return cb, func() tea.Msg {
			return types.ClearFilterMsg{}
		}
	}

	// Handle paste events
	if msg.Paste {
		pastedText := string(msg.Runes)
		cb.state = StateFilter
		cb.input.Set(pastedText)
		cb.inputType = CommandTypeFilter
		cb.height = 1
		return cb, func() tea.Msg {
			return types.FilterUpdateMsg{Filter: cb.input.Get()}
		}
	}

	switch msg.String() {
	case ":":
		cb.transitionToPalette(":", CommandTypeResource)
		return cb, nil
	case "/":
		cb.transitionToPalette("/", CommandTypeAction)
		return cb, nil
	default:
		// Any other character starts filter mode
		if len(msg.String()) == 1 && msg.String() != " " {
			cb.state = StateFilter
			cb.input.Set(msg.String())
			cb.inputType = CommandTypeFilter
			cb.height = 1
			return cb, func() tea.Msg {
				return types.FilterUpdateMsg{Filter: cb.input.Get()}
			}
		}
	}

	return cb, nil
}

// handleFilterState handles input in filter mode.
func (cb *CommandBar) handleFilterState(msg tea.KeyMsg) (*CommandBar, tea.Cmd) {
	result := cb.input.HandleKeyMsg(msg)

	switch result.Action {
	case InputActionPaste:
		cb.input.AddText(result.Text)
		return cb, func() tea.Msg {
			return types.FilterUpdateMsg{Filter: cb.input.Get()}
		}

	case InputActionChar:
		cb.input.AddChar(result.Text)
		return cb, func() tea.Msg {
			return types.FilterUpdateMsg{Filter: cb.input.Get()}
		}

	case InputActionBackspace:
		isEmpty := cb.input.Backspace()
		if isEmpty {
			cb.state = StateHidden
			return cb, func() tea.Msg {
				return types.ClearFilterMsg{}
			}
		}
		return cb, func() tea.Msg {
			return types.FilterUpdateMsg{Filter: cb.input.Get()}
		}
	}

	switch msg.String() {
	case "esc":
		cb.state = StateHidden
		cb.input.Clear()
		cb.height = 1
		return cb, func() tea.Msg {
			return types.ClearFilterMsg{}
		}

	case "enter":
		// Accept filter - keep it active but hide the input
		cb.state = StateHidden
		cb.height = 1
		return cb, nil
	}

	return cb, nil
}

// handlePaletteState handles input when suggestion palette is visible.
func (cb *CommandBar) handlePaletteState(msg tea.KeyMsg) (*CommandBar, tea.Cmd) {
	result := cb.input.HandleKeyMsg(msg)

	switch result.Action {
	case InputActionPaste:
		cb.input.AddText(result.Text)
		query := cb.input.Get()[1:] // Remove prefix
		cb.palette.Filter(query, cb.inputType, cb.screenID)
		cb.height = 1 + cb.palette.GetHeight()
		return cb, nil

	case InputActionChar:
		cb.input.AddChar(result.Text)

		// Check if user typed space after /ai
		if cb.input.Get() == "/ai " {
			cb.state = StateInput
			cb.inputType = CommandTypeLLMAction
			cb.height = 1
			cb.palette.Reset()
			return cb, nil
		}

		// Re-filter palette
		query := cb.input.Get()[1:]
		cb.palette.Filter(query, cb.inputType, cb.screenID)
		cb.height = 1 + cb.palette.GetHeight()
		return cb, nil

	case InputActionBackspace:
		isEmpty := cb.input.Backspace()
		if isEmpty {
			cb.state = StateHidden
			cb.height = 1
			cb.palette.Reset()
			return cb, nil
		}

		// If only prefix left, dismiss palette
		if len(cb.input.Get()) == 1 {
			cb.state = StateHidden
			cb.input.Clear()
			cb.height = 1
			cb.palette.Reset()
			return cb, nil
		}

		// Re-filter palette
		query := cb.input.Get()[1:]
		cb.palette.Filter(query, cb.inputType, cb.screenID)
		cb.height = 1 + cb.palette.GetHeight()
		return cb, nil
	}

	switch msg.String() {
	case "esc":
		cb.state = StateHidden
		cb.input.Clear()
		cb.height = 1
		cb.palette.Reset()
		return cb, nil

	case "enter":
		return cb.handlePaletteEnter()

	case "up":
		cb.palette.NavigateUp()
		return cb, nil

	case "down":
		cb.palette.NavigateDown()
		return cb, nil

	case "tab":
		return cb.handlePaletteTab()
	}

	return cb, nil
}

// handlePaletteEnter handles enter key in palette state.
func (cb *CommandBar) handlePaletteEnter() (*CommandBar, tea.Cmd) {
	// If user has typed a command with args, execute directly
	if len(cb.input.Get()) > 1 && strings.Contains(cb.input.Get(), " ") {
		cb.state = StateInput
		return cb.handleInputState(tea.KeyMsg{Type: tea.KeyEnter})
	}

	// Select item from palette
	selected := cb.palette.GetSelected()
	if selected == nil {
		return cb, nil
	}

	// Special handling for /ai
	if selected.Category == commands.CategoryLLMAction && selected.Name == "ai" {
		cb.state = StateInput
		cb.input.Set("/ai ")
		cb.inputType = CommandTypeLLMAction
		cb.height = 1
		cb.palette.Reset()
		return cb, nil
	}

	// Build command string
	prefix := cb.input.Get()[:1]
	commandStr := prefix + selected.Name

	// Check if needs confirmation
	if selected.NeedsConfirmation {
		cb.executor.pendingCommand = selected
		cb.input.Set(commandStr) // Store for history after confirmation
		cb.state = StateConfirmation
		cb.height = 5
		cb.palette.Reset()
		return cb, nil
	}

	// Add to history
	cb.history.Add(commandStr)

	// Execute command
	var cmd tea.Cmd
	if selected.Execute != nil {
		ctx := cb.executor.BuildContext(k8s.ResourceType(cb.screenID), cb.selectedResource, "")
		cmd = selected.Execute(ctx)
	}

	// Return to hidden
	cb.state = StateHidden
	cb.input.Clear()
	cb.height = 1
	cb.palette.Reset()

	return cb, cmd
}

// handlePaletteTab handles tab key in palette state (auto-complete).
func (cb *CommandBar) handlePaletteTab() (*CommandBar, tea.Cmd) {
	selected := cb.palette.GetSelected()
	if selected == nil {
		return cb, nil
	}

	prefix := cb.input.Get()[:1]

	// Special handling for /ai
	if selected.Category == commands.CategoryLLMAction && selected.Name == "ai" {
		cb.state = StateInput
		cb.input.Set("/ai ")
		cb.inputType = CommandTypeLLMAction
		cb.height = 1
		cb.palette.Reset()
		return cb, nil
	}

	// Build command string with space for arguments
	commandStr := prefix + selected.Name + " "
	cb.input.Set(commandStr)

	// Transition to input mode
	cb.state = StateInput
	cb.height = 1
	cb.palette.Reset()

	return cb, nil
}

// handleInputState handles direct command input.
func (cb *CommandBar) handleInputState(msg tea.KeyMsg) (*CommandBar, tea.Cmd) {
	result := cb.input.HandleKeyMsg(msg)

	switch result.Action {
	case InputActionPaste:
		cb.input.AddText(result.Text)
		return cb, nil

	case InputActionChar:
		cb.input.AddChar(result.Text)
		return cb, nil

	case InputActionBackspace:
		isEmpty := cb.input.Backspace()
		if isEmpty {
			cb.state = StateHidden
			cb.height = 1
			return cb, nil
		}

		// If backspaced to prefix, show palette
		input := cb.input.Get()
		if input == ":" {
			cb.transitionToPalette(":", CommandTypeResource)
			return cb, nil
		}
		if input == "/" {
			cb.transitionToPalette("/", CommandTypeAction)
			return cb, nil
		}
		if input == "/ai" {
			cb.transitionToPalette("/ai", CommandTypeAction)
			return cb, nil
		}
		return cb, nil
	}

	switch msg.String() {
	case "esc":
		cb.state = StateHidden
		cb.input.Clear()
		cb.height = 1
		cb.history.Reset()
		return cb, nil

	case "up":
		if cmd, ok := cb.history.NavigateUp(); ok {
			cb.input.Set(cmd)
		}
		return cb, nil

	case "down":
		if cmd, ok := cb.history.NavigateDown(); ok {
			cb.input.Set(cmd)
		} else {
			cb.input.Clear()
		}
		return cb, nil

	case "enter":
		return cb.handleInputEnter()
	}

	return cb, nil
}

// handleInputEnter handles enter key in input state.
func (cb *CommandBar) handleInputEnter() (*CommandBar, tea.Cmd) {
	inputStr := cb.input.Get()

	// Handle LLM commands
	if strings.HasPrefix(inputStr, "/ai ") {
		prompt := strings.TrimPrefix(inputStr, "/ai ")
		prompt = strings.TrimSpace(prompt)

		if prompt == "" {
			cb.state = StateHidden
			cb.input.Clear()
			cb.height = 1
			cb.history.Reset()
			return cb, nil
		}

		// Translate with mock LLM
		translation := commands.TranslateWithMockLLM(prompt)
		cb.executor.SetLLMTranslation(&translation)

		// Transition to LLM preview
		cb.state = StateLLMPreview
		cb.height = 6
		return cb, nil
	}

	// Parse and execute other commands
	prefix, cmdName, args := cb.input.ParseCommand()
	if len(prefix) == 0 || len(cmdName) == 0 {
		cb.state = StateHidden
		cb.input.Clear()
		cb.height = 1
		cb.history.Reset()
		return cb, nil
	}

	var category commands.CommandCategory
	switch prefix {
	case ":":
		category = commands.CategoryResource
	case "/":
		category = commands.CategoryAction
	}

	ctx := cb.executor.BuildContext(k8s.ResourceType(cb.screenID), cb.selectedResource, args)
	cmd, needsConfirm := cb.executor.Execute(cmdName, category, ctx)

	if needsConfirm {
		cb.executor.pendingArgs = args
		cb.state = StateConfirmation
		cb.height = 5
		return cb, nil
	}

	if cmd != nil {
		cb.history.Add(inputStr)
		cb.state = StateHidden
		cb.input.Clear()
		cb.height = 1
		return cb, cmd
	}

	// Unknown command
	cb.state = StateHidden
	cb.input.Clear()
	cb.height = 1
	cb.history.Reset()
	return cb, nil
}

// handleConfirmationState handles confirmation prompts.
func (cb *CommandBar) handleConfirmationState(msg tea.KeyMsg) (*CommandBar, tea.Cmd) {
	switch msg.String() {
	case "esc":
		cb.state = StateHidden
		cb.input.Clear()
		cb.height = 1
		cb.executor.CancelPending()
		return cb, nil

	case "enter":
		// Add to history
		cb.history.Add(cb.input.Get())

		// Execute pending command
		ctx := cb.executor.BuildContext(k8s.ResourceType(cb.screenID), cb.selectedResource, "")
		cmd := cb.executor.ExecutePending(ctx)

		// Return to hidden
		cb.state = StateHidden
		cb.input.Clear()
		cb.height = 1
		return cb, cmd
	}
	return cb, nil
}

// handleLLMPreviewState handles LLM command preview.
func (cb *CommandBar) handleLLMPreviewState(msg tea.KeyMsg) (*CommandBar, tea.Cmd) {
	switch msg.String() {
	case "esc":
		cb.state = StateHidden
		cb.input.Clear()
		cb.height = 1
		cb.executor.ClearLLMTranslation()
		return cb, nil

	case "enter":
		// Add to history
		cb.history.Add(cb.input.Get())

		translation := cb.executor.GetLLMTranslation()
		if translation != nil {
			cb.state = StateResult
			cb.input.Set("Command would execute: " + translation.Command)
			cb.height = 2
			cb.executor.ClearLLMTranslation()
			return cb, nil
		}

		cb.state = StateHidden
		cb.input.Clear()
		cb.height = 1
		return cb, nil

	case "e":
		// Edit mode - for now just dismiss
		cb.state = StateHidden
		cb.input.Clear()
		cb.height = 1
		cb.executor.ClearLLMTranslation()
		return cb, nil
	}
	return cb, nil
}

// handleResultState handles result display.
func (cb *CommandBar) handleResultState(msg tea.KeyMsg) (*CommandBar, tea.Cmd) {
	switch msg.String() {
	case "esc":
		cb.state = StateHidden
		cb.input.Clear()
		cb.height = 1
		return cb, nil
	}
	return cb, nil
}

// transitionToPalette transitions to palette state.
func (cb *CommandBar) transitionToPalette(input string, cmdType CommandType) {
	cb.state = StateSuggestionPalette
	cb.input.Set(input)
	cb.inputType = cmdType

	// Extract query
	query := ""
	if len(input) > 1 {
		query = input[1:]
	}

	cb.palette.Filter(query, cmdType, cb.screenID)
	cb.height = 1 + cb.palette.GetHeight()
}

// View renders the command bar.
func (cb *CommandBar) View() string {
	separatorStyle := lipgloss.NewStyle().
		Foreground(cb.theme.Border).
		Width(cb.width)
	separator := separatorStyle.Render(strings.Repeat("─", cb.width))

	var content string
	switch cb.state {
	case StateHidden:
		return ""
	case StateFilter, StateInput:
		content = cb.input.View(cb.inputType)
	case StateSuggestionPalette:
		content = cb.input.View(cb.inputType)
	case StateConfirmation:
		content = cb.executor.ViewConfirmation()
	case StateLLMPreview:
		content = cb.executor.ViewLLMPreview()
	case StateResult:
		content = cb.executor.ViewResult(cb.input.Get(), true)
	default:
		return ""
	}

	return lipgloss.JoinVertical(lipgloss.Left, separator, content, separator)
}

// ViewHints renders the hints line (shown below command bar).
func (cb *CommandBar) ViewHints() string {
	hintStyle := lipgloss.NewStyle().
		Foreground(cb.theme.Subtle).
		Width(cb.width).
		Padding(0, 1)

	separatorStyle := lipgloss.NewStyle().
		Foreground(cb.theme.Border).
		Width(cb.width)
	separator := separatorStyle.Render(strings.Repeat("─", cb.width))

	if cb.state == StateHidden {
		hints := hintStyle.Render("[type to filter  : resources  / commands]")
		return lipgloss.JoinVertical(lipgloss.Left, separator, hints, separator)
	}

	return ""
}

// ViewPaletteItems renders the palette items (shown below command bar).
func (cb *CommandBar) ViewPaletteItems() string {
	if cb.state != StateSuggestionPalette {
		return ""
	}

	prefix := cb.input.Get()[:1]
	return cb.palette.View(prefix)
}

// ExecuteCommand executes a command by name and category.
func (cb *CommandBar) ExecuteCommand(name string, category commands.CommandCategory) (*CommandBar, tea.Cmd) {
	ctx := cb.executor.BuildContext(k8s.ResourceType(cb.screenID), cb.selectedResource, "")
	cmd, needsConfirm := cb.executor.Execute(name, category, ctx)

	if needsConfirm {
		cb.state = StateConfirmation
		cb.height = 5
		return cb, nil
	}

	return cb, cmd
}
