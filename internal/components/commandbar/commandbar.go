package commandbar

import (
	"math/rand"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/renato0307/k1/internal/commands"
	"github.com/renato0307/k1/internal/k8s"
	"github.com/renato0307/k1/internal/keyboard"
	"github.com/renato0307/k1/internal/logging"
	"github.com/renato0307/k1/internal/types"
	"github.com/renato0307/k1/internal/ui"
)

// usageTips contains helpful tips about k1 features
// First tip is the original static hint for familiarity
var usageTips = []string{
	"[/ search  : resources  > palette  ? help]",
	"[tip: press Enter on resources for actions]",
	"[tip: press y for YAML, d for describe]",
	"[tip: press e to edit, l for logs]",
	"[tip: press :q or ctrl+c to quit, ESC to go back]",
	"[tip: press [ / ] to switch contexts]",
	"[tip: use :output to view command execution results]",
	"[tip: use negation in filters: !Running]",
	"[tip: filter matches any part of the name/namespace]",
	"[tip: filter shows matching count in real-time]",
	"[tip: start with -context to load specific context]",
	"[tip: use multiple -context to load several contexts]",
	"[tip: use -theme to choose from 8 available themes]",
	"[tip: use -kubeconfig for custom kubeconfig path]",
	"[tip: use -dummy to explore k1 without a cluster]",
	"[tip: resources refresh automatically every 10 seconds]",
}

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

	// Tip rotation state
	currentTipIndex int
	lastTipRotation time.Time

	// Keyboard config
	keys *keyboard.Keys
}

// New creates a new command bar coordinator.
func New(pool *k8s.RepositoryPool, theme *ui.Theme, keys *keyboard.Keys) *CommandBar {
	registry := commands.NewRegistry(pool, keys)

	return &CommandBar{
		state:           StateHidden,
		inputType:       CommandTypeFilter,
		width:           80,
		height:          1,
		theme:           theme,
		history:         NewHistory(),
		palette:         NewPalette(registry, theme, 80),
		input:           NewInput(registry, theme, 80),
		executor:        NewExecutor(registry, theme, 80),
		registry:        registry,
		currentTipIndex: 0,          // Start with first tip
		lastTipRotation: time.Now(), // Track last rotation
		keys:            keys,
	}
}

// Init initializes the command bar and schedules first tip rotation
func (cb *CommandBar) Init() tea.Cmd {
	logging.Debug("CommandBar.Init: Scheduling first tip rotation",
		"interval", TipRotationInterval.String())
	return tea.Tick(TipRotationInterval, func(t time.Time) tea.Msg {
		return tipRotationMsg(t)
	})
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
	case tipRotationMsg:
		// Rotate to random tip (avoid showing same tip twice in a row)
		oldIndex := cb.currentTipIndex

		// Pick a random tip that's different from current
		newIndex := oldIndex
		if len(usageTips) > 1 {
			// Keep picking until we get a different tip
			for newIndex == oldIndex {
				newIndex = rand.Intn(len(usageTips))
			}
		}

		cb.currentTipIndex = newIndex
		cb.lastTipRotation = time.Now()

		logging.Debug("CommandBar.Update: Tip rotation triggered",
			"oldIndex", oldIndex,
			"newIndex", cb.currentTipIndex,
			"newTip", usageTips[cb.currentTipIndex])

		// Schedule next rotation
		nextTick := tea.Tick(TipRotationInterval, func(t time.Time) tea.Msg {
			return tipRotationMsg(t)
		})

		logging.Debug("CommandBar.Update: Scheduled next tip rotation",
			"interval", TipRotationInterval.String())

		return cb, nextTick
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
	if msg.String() == cb.keys.Back && cb.input.Get() != "" {
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
	case cb.keys.ResourceNav: // ":"
		cb.transitionToPalette(cb.keys.ResourceNav, CommandTypeResource)
		return cb, nil

	case cb.keys.FilterActivate: // "/"
		cb.state = StateFilter
		cb.input.Set(cb.keys.FilterActivate)
		cb.inputType = CommandTypeFilter
		cb.height = 1
		return cb, func() tea.Msg {
			return types.FilterUpdateMsg{Filter: ""}
		}

	case cb.keys.PaletteActivate, ">": // "ctrl+p" or ">"
		cb.transitionToPalette(">", CommandTypeAction)
		return cb, nil

	default:
		// REMOVED: type-to-filter behavior
		// No longer accept single chars to start filtering
	}

	return cb, nil
}

// handleFilterState handles input in filter mode.
func (cb *CommandBar) handleFilterState(msg tea.KeyMsg) (*CommandBar, tea.Cmd) {
	result := cb.input.HandleKeyMsg(msg)

	switch result.Action {
	case InputActionPaste:
		cb.input.AddText(result.Text)
		// Strip the leading "/" from filter text
		filterText := strings.TrimPrefix(cb.input.Get(), "/")
		return cb, func() tea.Msg {
			return types.FilterUpdateMsg{Filter: filterText}
		}

	case InputActionChar:
		cb.input.AddChar(result.Text)
		// Strip the leading "/" from filter text
		filterText := strings.TrimPrefix(cb.input.Get(), "/")
		return cb, func() tea.Msg {
			return types.FilterUpdateMsg{Filter: filterText}
		}

	case InputActionBackspace:
		isEmpty := cb.input.Backspace()
		if isEmpty {
			cb.state = StateHidden
			return cb, func() tea.Msg {
				return types.ClearFilterMsg{}
			}
		}
		// Strip the leading "/" from filter text
		filterText := strings.TrimPrefix(cb.input.Get(), "/")
		return cb, func() tea.Msg {
			return types.FilterUpdateMsg{Filter: filterText}
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
		// Remove prefix if present (: or / or >)
		query := cb.input.Get()
		if len(query) > 0 && (query[0] == ':' || query[0] == '/' || query[0] == '>') {
			query = query[1:]
		}
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
		// Remove prefix if present (: or / or >)
		query := cb.input.Get()
		if len(query) > 0 && (query[0] == ':' || query[0] == '/' || query[0] == '>') {
			query = query[1:]
		}
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

		input := cb.input.Get()
		// If only prefix left (: or / or >), dismiss palette
		if len(input) == 1 && (input[0] == ':' || input[0] == '/' || input[0] == '>') {
			cb.state = StateHidden
			cb.input.Clear()
			cb.height = 1
			cb.palette.Reset()
			return cb, nil
		}

		// Re-filter palette
		// Remove prefix if present (: or / or >)
		query := input
		if len(query) > 0 && (query[0] == ':' || query[0] == '/' || query[0] == '>') {
			query = query[1:]
		}
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

	// Build command string with prefix if present
	input := cb.input.Get()
	prefix := ""
	if len(input) > 0 && (input[0] == ':' || input[0] == '/') {
		prefix = input[:1]
	}
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
		ctx := cb.executor.BuildContext(k8s.ResourceType(cb.screenID), cb.selectedResource, "", commandStr)
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

	input := cb.input.Get()
	prefix := ""
	if len(input) > 0 && (input[0] == ':' || input[0] == '/' || input[0] == '>') {
		prefix = input[:1]
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

	// Build command string with space for arguments
	commandStr := prefix + selected.Name + " "
	cb.input.Set(commandStr)
	logging.Debug("Tab completion", "commandStr", commandStr, "selectedName", selected.Name, "selectedCategory", selected.Category)

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
	logging.Debug("handleInputEnter", "inputStr", inputStr, "state", cb.state)

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
	logging.Debug("ParseCommand result", "prefix", prefix, "cmdName", cmdName, "args", args)
	if len(prefix) == 0 || len(cmdName) == 0 {
		logging.Debug("Empty prefix or cmdName, hiding command bar")
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
	case "/", ">":
		category = commands.CategoryAction
	}

	ctx := cb.executor.BuildContext(k8s.ResourceType(cb.screenID), cb.selectedResource, args, inputStr)
	cmd, needsConfirm := cb.executor.Execute(cmdName, category, ctx)
	logging.Debug("Execute result", "cmdName", cmdName, "category", category, "needsConfirm", needsConfirm, "cmdIsNil", cmd == nil)

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
		logging.Debug("Executing command", "cmdName", cmdName)
		return cb, cmd
	}

	// Unknown command
	logging.Debug("Unknown command, hiding", "cmdName", cmdName)
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
		originalCmd := cb.input.Get()
		cb.history.Add(originalCmd)

		// Execute pending command
		ctx := cb.executor.BuildContext(k8s.ResourceType(cb.screenID), cb.selectedResource, "", originalCmd)
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
		// Select current tip from rotation
		currentTip := usageTips[cb.currentTipIndex]
		hints := hintStyle.Render(currentTip)
		return lipgloss.JoinVertical(lipgloss.Left, separator, hints, separator)
	}

	return ""
}

// ViewPaletteItems renders the palette items (shown below command bar).
func (cb *CommandBar) ViewPaletteItems() string {
	if cb.state != StateSuggestionPalette {
		return ""
	}

	// Get prefix - only if it's an actual prefix character (: or /)
	// For ctrl+p palette, there's no prefix
	prefix := ""
	input := cb.input.Get()
	if len(input) > 0 && (input[0] == ':' || input[0] == '/') {
		prefix = input[:1]
	}
	return cb.palette.View(prefix)
}

// ExecuteCommand executes a command by name and category.
func (cb *CommandBar) ExecuteCommand(name string, category commands.CommandCategory) (*CommandBar, tea.Cmd) {
	// Construct original command string from name and category
	var prefix string
	switch category {
	case commands.CategoryResource:
		prefix = ":"
	case commands.CategoryAction:
		prefix = "/"
	case commands.CategoryLLMAction:
		prefix = "/ai "
	}
	originalCmd := prefix + name

	ctx := cb.executor.BuildContext(k8s.ResourceType(cb.screenID), cb.selectedResource, "", originalCmd)
	cmd, needsConfirm := cb.executor.Execute(name, category, ctx)

	if needsConfirm {
		cb.state = StateConfirmation
		cb.height = 5
		return cb, nil
	}

	return cb, cmd
}

// GetCommandByShortcut returns a command by keyboard shortcut.
func (cb *CommandBar) GetCommandByShortcut(shortcut string) *commands.Command {
	return cb.registry.GetByShortcut(shortcut)
}
