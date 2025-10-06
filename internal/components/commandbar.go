package components

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/renato0307/k1/internal/commands"
	"github.com/renato0307/k1/internal/k8s"
	"github.com/renato0307/k1/internal/types"
	"github.com/renato0307/k1/internal/ui"
)

// CommandBarState represents the current state of the command bar
type CommandBarState int

const (
	StateHidden            CommandBarState = iota
	StateFilter                            // No prefix, filtering list
	StateSuggestionPalette                 // : or / pressed, showing suggestions
	StateInput                             // Direct command input
	StateConfirmation                      // Destructive operation confirmation
	StateLLMPreview                        // /ai command preview
	StateResult                            // Success/error message
)

// CommandType represents the type of command being entered
type CommandType int

const (
	CommandTypeFilter    CommandType = iota // no prefix
	CommandTypeResource                     // : prefix
	CommandTypeAction                       // / prefix
	CommandTypeLLMAction                    // /ai prefix
)

// CommandBar represents the expandable command bar at the bottom of the screen
type CommandBar struct {
	state     CommandBarState
	input     string
	inputType CommandType
	width     int
	height    int // Dynamic, 1-10 lines
	theme     *ui.Theme
	cursorPos int

	// Command registry
	registry *commands.Registry

	// Current screen context (for filtering resource commands and execution)
	screenID         string
	selectedResource map[string]interface{} // Currently selected resource data

	// History
	history    []string
	historyIdx int

	// Suggestion palette state
	paletteVisible bool
	paletteItems   []commands.Command // Palette command items
	paletteIdx     int

	// Pending command (for confirmation/preview states)
	pendingCommand *commands.Command
	pendingArgs    string // Args for pending command

	// LLM translation result (for LLM preview state)
	llmTranslation *commands.MockLLMTranslation
}

// NewCommandBar creates a new command bar component
func NewCommandBar(repo k8s.Repository, theme *ui.Theme) *CommandBar {
	return &CommandBar{
		state:          StateHidden,
		input:          "",
		inputType:      CommandTypeFilter,
		width:          80,
		height:         1, // Start with 1 line
		theme:          theme,
		cursorPos:      0,
		registry:       commands.NewRegistry(repo),
		history:        []string{},
		historyIdx:     -1,
		paletteVisible: false,
		paletteItems:   []commands.Command{},
		paletteIdx:     0,
		pendingCommand: nil,
		llmTranslation: nil,
	}
}

// SetWidth updates the command bar width
func (cb *CommandBar) SetWidth(width int) {
	cb.width = width
}

// SetScreen updates the current screen context for command filtering
func (cb *CommandBar) SetScreen(screenID string) {
	cb.screenID = screenID
}

// SetSelectedResource updates the selected resource info for command execution
func (cb *CommandBar) SetSelectedResource(resource map[string]interface{}) {
	cb.selectedResource = resource
}

// buildCommandContext creates a CommandContext for command execution
func (cb *CommandBar) buildCommandContext(args string) commands.CommandContext {
	return commands.CommandContext{
		ResourceType: cb.screenID,
		Selected:     cb.selectedResource,
		Args:         args,
	}
}

// addToHistory adds a command to history (avoids duplicates of most recent)
func (cb *CommandBar) addToHistory(cmd string) {
	// Don't add empty commands
	if strings.TrimSpace(cmd) == "" {
		return
	}

	// Don't add if it's the same as the most recent command
	if len(cb.history) > 0 && cb.history[len(cb.history)-1] == cmd {
		return
	}

	// Add to history
	cb.history = append(cb.history, cmd)

	// Keep max 100 entries (configurable in future)
	maxHistory := 100
	if len(cb.history) > maxHistory {
		cb.history = cb.history[len(cb.history)-maxHistory:]
	}

	// Reset history index
	cb.historyIdx = -1
}

// ExecuteCommand executes a command by name and category
// Returns the updated CommandBar and a tea.Cmd
func (cb *CommandBar) ExecuteCommand(name string, category commands.CommandCategory) (*CommandBar, tea.Cmd) {
	cmd := cb.registry.Get(name, category)
	if cmd == nil {
		return cb, nil
	}

	// Check if command needs confirmation
	if cmd.NeedsConfirmation {
		cb.pendingCommand = cmd
		cb.state = StateConfirmation
		cb.height = 5
		return cb, nil
	}

	// Execute command
	var execCmd tea.Cmd
	if cmd.Execute != nil {
		ctx := cb.buildCommandContext("")
		execCmd = cmd.Execute(ctx)
	}

	return cb, execCmd
}

// GetHeight returns the current height of the command bar (including separators, not hints)
func (cb *CommandBar) GetHeight() int {
	if cb.state == StateHidden {
		// Hidden state: View() returns "" (0 lines)
		return 0
	}
	// Add 2 for: top separator (1) + bottom separator (1)
	return cb.height + 2
}

// GetTotalHeight returns the height including hints line (for layout calculations)
func (cb *CommandBar) GetTotalHeight() int {
	baseHeight := cb.GetHeight()

	// Hints are only shown in StateHidden (3 lines: separator + text + separator)
	// In other states, no hints are shown (0 lines)
	if cb.state == StateHidden {
		return baseHeight + 3
	}

	return baseHeight
}

// GetState returns the current state
func (cb *CommandBar) GetState() CommandBarState {
	return cb.state
}

// GetInput returns the current input string
func (cb *CommandBar) GetInput() string {
	return cb.input
}

// GetInputType returns the current command type
func (cb *CommandBar) GetInputType() CommandType {
	return cb.inputType
}

// IsActive returns true if the command bar is accepting input
func (cb *CommandBar) IsActive() bool {
	return cb.state != StateHidden && cb.state != StateResult
}

// Update handles messages for the command bar
func (cb *CommandBar) Update(msg tea.Msg) (*CommandBar, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return cb.handleKeyMsg(msg)
	}

	return cb, nil
}

// handleKeyMsg processes keyboard input
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

// handleHiddenState handles input when command bar is hidden
func (cb *CommandBar) handleHiddenState(msg tea.KeyMsg) (*CommandBar, tea.Cmd) {
	// Handle paste events - transition directly to filter mode
	if msg.Paste {
		pastedText := string(msg.Runes)
		cb.state = StateFilter
		cb.input = pastedText
		cb.inputType = CommandTypeFilter
		cb.cursorPos = len(pastedText)
		cb.height = 1
		// Send initial filter with pasted text
		return cb, func() tea.Msg {
			return types.FilterUpdateMsg{Filter: cb.input}
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
			cb.input = msg.String()
			cb.inputType = CommandTypeFilter
			cb.cursorPos = 1
			cb.height = 1 // Stay at 1 line for filter
			// Send initial filter
			return cb, func() tea.Msg {
				return types.FilterUpdateMsg{Filter: cb.input}
			}
		}
	}

	return cb, nil
}

// handleFilterState handles input in filter mode
func (cb *CommandBar) handleFilterState(msg tea.KeyMsg) (*CommandBar, tea.Cmd) {
	// Handle paste events
	if msg.Paste {
		pastedText := string(msg.Runes)
		cb.input += pastedText
		cb.cursorPos += len(pastedText)
		// Send updated filter
		return cb, func() tea.Msg {
			return types.FilterUpdateMsg{Filter: cb.input}
		}
	}

	switch msg.String() {
	case "esc":
		// Clear filter and return to hidden
		cb.state = StateHidden
		cb.input = ""
		cb.cursorPos = 0
		cb.height = 1
		// Send message to clear filter on list
		return cb, func() tea.Msg {
			return types.ClearFilterMsg{}
		}

	case "enter":
		// Apply filter and exit filter mode (keep filter active)
		cb.state = StateHidden
		cb.height = 1
		// Keep input and filter active
		return cb, nil

	case "backspace":
		// Remove last character
		if len(cb.input) > 0 {
			cb.input = cb.input[:len(cb.input)-1]
			cb.cursorPos--
			if cb.cursorPos < 0 {
				cb.cursorPos = 0
			}
		}
		// If input is empty, return to hidden and clear filter
		if len(cb.input) == 0 {
			cb.state = StateHidden
			return cb, func() tea.Msg {
				return types.ClearFilterMsg{}
			}
		}
		// Send updated filter
		return cb, func() tea.Msg {
			return types.FilterUpdateMsg{Filter: cb.input}
		}

	default:
		// Add character to input
		if len(msg.String()) == 1 {
			cb.input += msg.String()
			cb.cursorPos++
			// Send updated filter
			return cb, func() tea.Msg {
				return types.FilterUpdateMsg{Filter: cb.input}
			}
		}
	}

	return cb, nil
}

// handlePaletteState handles input when suggestion palette is visible
func (cb *CommandBar) handlePaletteState(msg tea.KeyMsg) (*CommandBar, tea.Cmd) {
	// Handle paste events
	if msg.Paste {
		pastedText := string(msg.Runes)
		cb.input += pastedText
		cb.cursorPos += len(pastedText)
		// Re-filter palette
		query := cb.input[1:] // Remove prefix
		cb.paletteItems = cb.getPaletteItems(cb.inputType, query)
		cb.paletteIdx = 0
		// Recalculate height
		itemCount := len(cb.paletteItems)
		if itemCount > 8 {
			itemCount = 8
		}
		cb.height = 1 + itemCount
		return cb, nil
	}

	switch msg.String() {
	case "esc":
		// Dismiss palette and return to hidden
		cb.state = StateHidden
		cb.input = ""
		cb.cursorPos = 0
		cb.height = 1
		cb.paletteVisible = false
		cb.paletteItems = []commands.Command{}
		cb.paletteIdx = 0
		return cb, nil

	case "enter":
		// If user has typed a command with args (e.g., "/scale 5"), execute it directly
		if len(cb.input) > 1 && strings.Contains(cb.input, " ") {
			// Transition to input mode and handle enter there
			cb.state = StateInput
			return cb.handleInputState(msg)
		}

		// Otherwise, select item from palette
		if cb.paletteIdx >= 0 && cb.paletteIdx < len(cb.paletteItems) {
			selected := cb.paletteItems[cb.paletteIdx]

			// Special handling for /ai (LLM commands)
			if selected.Category == commands.CategoryLLMAction && selected.Name == "ai" {
				// Transition to input mode for LLM
				cb.state = StateInput
				cb.input = "/ai "
				cb.inputType = CommandTypeLLMAction
				cb.cursorPos = 4
				cb.height = 1
				cb.paletteVisible = false
				cb.paletteItems = []commands.Command{}
				cb.paletteIdx = 0
				return cb, nil
			}

			// Build command string for history
			prefix := cb.input[:1] // : or /
			commandStr := prefix + selected.Name

			// Check if command needs confirmation
			if selected.NeedsConfirmation {
				// Store command and transition to confirmation state
				cb.pendingCommand = &selected
				cb.input = commandStr // Store for history after confirmation
				cb.state = StateConfirmation
				cb.height = 5 // Expand to show confirmation (3-5 lines)
				cb.paletteVisible = false
				cb.paletteItems = []commands.Command{}
				return cb, nil
			}

			// Add to history
			cb.addToHistory(commandStr)

			// Execute command
			var cmd tea.Cmd
			if selected.Execute != nil {
				ctx := cb.buildCommandContext("")
				cmd = selected.Execute(ctx)
			}

			// Return to hidden state
			cb.state = StateHidden
			cb.input = ""
			cb.cursorPos = 0
			cb.height = 1
			cb.paletteVisible = false
			cb.paletteItems = []commands.Command{}
			cb.paletteIdx = 0

			return cb, cmd
		}
		return cb, nil

	case "up":
		// Navigate palette up
		if cb.paletteIdx > 0 {
			cb.paletteIdx--
		}
		return cb, nil

	case "down":
		// Navigate palette down
		if cb.paletteIdx < len(cb.paletteItems)-1 {
			cb.paletteIdx++
		}
		return cb, nil

	case "tab":
		// Auto-complete selected command and transition to input mode
		if cb.paletteIdx >= 0 && cb.paletteIdx < len(cb.paletteItems) {
			selected := cb.paletteItems[cb.paletteIdx]
			prefix := cb.input[:1] // Get the : or / prefix

			// Special handling for /ai - needs special input mode
			if selected.Category == commands.CategoryLLMAction && selected.Name == "ai" {
				cb.state = StateInput
				cb.input = "/ai "
				cb.inputType = CommandTypeLLMAction
				cb.cursorPos = 4
				cb.height = 1
				cb.paletteVisible = false
				cb.paletteItems = []commands.Command{}
				cb.paletteIdx = 0
				return cb, nil
			}

			// Build command string with space for arguments
			commandStr := prefix + selected.Name + " "
			cb.input = commandStr
			cb.cursorPos = len(commandStr)

			// Transition to input mode for argument entry
			cb.state = StateInput
			cb.height = 1
			cb.paletteVisible = false
			cb.paletteItems = []commands.Command{}
			cb.paletteIdx = 0
		}
		return cb, nil

	case "backspace":
		// Remove character and filter palette
		if len(cb.input) > 1 { // Keep prefix (: or /)
			cb.input = cb.input[:len(cb.input)-1]
			cb.cursorPos--
			// Re-filter palette
			query := cb.input[1:] // Remove prefix
			cb.paletteItems = cb.getPaletteItems(cb.inputType, query)
			cb.paletteIdx = 0
			// Recalculate height
			itemCount := len(cb.paletteItems)
			if itemCount > 8 {
				itemCount = 8
			}
			cb.height = 1 + itemCount
		} else {
			// If only prefix left, dismiss palette
			cb.state = StateHidden
			cb.input = ""
			cb.cursorPos = 0
			cb.height = 1
			cb.paletteVisible = false
			cb.paletteItems = []commands.Command{}
			cb.paletteIdx = 0
		}
		return cb, nil

	default:
		// Add character and filter palette
		if len(msg.String()) == 1 {
			cb.input += msg.String()
			cb.cursorPos++

			// Check if user typed space after /ai - transition to LLM input mode
			if cb.input == "/ai " {
				cb.state = StateInput
				cb.inputType = CommandTypeLLMAction
				cb.height = 1
				cb.paletteVisible = false
				cb.paletteItems = []commands.Command{}
				cb.paletteIdx = 0
				return cb, nil
			}

			// Re-filter palette
			query := cb.input[1:] // Remove prefix
			cb.paletteItems = cb.getPaletteItems(cb.inputType, query)
			cb.paletteIdx = 0
			// Recalculate height
			itemCount := len(cb.paletteItems)
			if itemCount > 8 {
				itemCount = 8
			}
			cb.height = 1 + itemCount
		}
	}

	return cb, nil
}

// handleInputState handles direct command input
func (cb *CommandBar) handleInputState(msg tea.KeyMsg) (*CommandBar, tea.Cmd) {
	// Handle paste events
	if msg.Paste {
		pastedText := string(msg.Runes)
		cb.input += pastedText
		cb.cursorPos += len(pastedText)
		return cb, nil
	}

	switch msg.String() {
	case "esc":
		// Cancel input and return to hidden
		cb.state = StateHidden
		cb.input = ""
		cb.cursorPos = 0
		cb.height = 1
		cb.historyIdx = -1 // Reset history index
		return cb, nil

	case "up":
		// Navigate history backwards (older commands)
		if len(cb.history) > 0 {
			if cb.historyIdx == -1 {
				// Start from most recent
				cb.historyIdx = len(cb.history) - 1
			} else if cb.historyIdx > 0 {
				cb.historyIdx--
			}
			// Load command from history
			cb.input = cb.history[cb.historyIdx]
			cb.cursorPos = len(cb.input)
		}
		return cb, nil

	case "down":
		// Navigate history forwards (newer commands)
		if len(cb.history) > 0 && cb.historyIdx != -1 {
			if cb.historyIdx < len(cb.history)-1 {
				cb.historyIdx++
				// Load command from history
				cb.input = cb.history[cb.historyIdx]
				cb.cursorPos = len(cb.input)
			} else {
				// At most recent, clear input
				cb.historyIdx = -1
				cb.input = ""
				cb.cursorPos = 0
			}
		}
		return cb, nil

	case "enter":
		// Execute the command
		// Check if it's an LLM command (/ai prefix)
		if strings.HasPrefix(cb.input, "/ai ") {
			// Extract prompt (remove "/ai " prefix)
			prompt := strings.TrimPrefix(cb.input, "/ai ")
			prompt = strings.TrimSpace(prompt)

			if prompt == "" {
				// Empty prompt, just dismiss
				cb.state = StateHidden
				cb.input = ""
				cb.height = 1
				cb.historyIdx = -1
				return cb, nil
			}

			// Translate with mock LLM
			translation := commands.TranslateWithMockLLM(prompt)
			cb.llmTranslation = &translation

			// Transition to LLM preview state (will save to history when executed)
			cb.state = StateLLMPreview
			cb.height = 6 // 4-6 lines for preview
			return cb, nil
		}

		// For other commands, try to find and execute
		// Parse command (e.g., ":pods" or "/yaml" or "/scale 5")
		if len(cb.input) > 1 {
			prefix := cb.input[:1]
			rest := cb.input[1:]

			// Split command name and args by whitespace
			parts := strings.SplitN(rest, " ", 2)
			cmdName := parts[0]
			cmdArgs := ""
			if len(parts) > 1 {
				cmdArgs = parts[1]
			}

			var category commands.CommandCategory
			switch prefix {
			case ":":
				category = commands.CategoryResource
			case "/":
				category = commands.CategoryAction
			}

			cmd := cb.registry.Get(cmdName, category)
			if cmd != nil {
				// Check if needs confirmation
				if cmd.NeedsConfirmation {
					cb.pendingCommand = cmd
					cb.pendingArgs = cmdArgs // Store args for confirmation
					cb.state = StateConfirmation
					cb.height = 5
					return cb, nil
				}

				// Add to history
				cb.addToHistory(cb.input)

				// Execute command with args
				var execCmd tea.Cmd
				if cmd.Execute != nil {
					ctx := cb.buildCommandContext(cmdArgs)
					execCmd = cmd.Execute(ctx)
				}

				// Return to hidden
				cb.state = StateHidden
				cb.input = ""
				cb.height = 1
				return cb, execCmd
			}
		}

		// Unknown command, return to hidden
		cb.state = StateHidden
		cb.input = ""
		cb.height = 1
		cb.historyIdx = -1
		return cb, nil

	case "backspace":
		// Remove character
		if len(cb.input) > 0 {
			cb.input = cb.input[:len(cb.input)-1]
			cb.cursorPos--
		}
		// If input is empty, return to hidden
		if len(cb.input) == 0 {
			cb.state = StateHidden
			cb.height = 1
			return cb, nil
		}
		// If we backspaced to a prefix (: or / or /ai), show palette
		if cb.input == ":" {
			cb.transitionToPalette(":", CommandTypeResource)
			return cb, nil
		}
		if cb.input == "/" {
			cb.transitionToPalette("/", CommandTypeAction)
			return cb, nil
		}
		if cb.input == "/ai" {
			cb.transitionToPalette("/ai", CommandTypeAction)
			return cb, nil
		}
		return cb, nil

	default:
		// Add character
		if len(msg.String()) == 1 {
			cb.input += msg.String()
			cb.cursorPos++
		}
		return cb, nil
	}
}

// handleConfirmationState handles confirmation prompts
func (cb *CommandBar) handleConfirmationState(msg tea.KeyMsg) (*CommandBar, tea.Cmd) {
	switch msg.String() {
	case "esc":
		// Cancel confirmation
		cb.state = StateHidden
		cb.input = ""
		cb.height = 1
		cb.pendingCommand = nil
		cb.pendingArgs = ""
		return cb, nil

	case "enter":
		// Add to history (cb.input contains the command string)
		cb.addToHistory(cb.input)

		// Execute pending command with stored args
		var cmd tea.Cmd
		if cb.pendingCommand != nil && cb.pendingCommand.Execute != nil {
			ctx := cb.buildCommandContext(cb.pendingArgs)
			cmd = cb.pendingCommand.Execute(ctx)
		}

		// Return to hidden state
		cb.state = StateHidden
		cb.input = ""
		cb.height = 1
		cb.pendingCommand = nil
		cb.pendingArgs = ""
		return cb, cmd
	}
	return cb, nil
}

// handleLLMPreviewState handles LLM command preview
func (cb *CommandBar) handleLLMPreviewState(msg tea.KeyMsg) (*CommandBar, tea.Cmd) {
	switch msg.String() {
	case "esc":
		// Cancel LLM preview
		cb.state = StateHidden
		cb.input = ""
		cb.height = 1
		cb.llmTranslation = nil
		return cb, nil

	case "enter":
		// Add to history (cb.input contains "/ai <prompt>")
		cb.addToHistory(cb.input)

		// Execute the generated command
		// For now, just show a success message
		// In Phase 4+, this would actually execute the kubectl command
		cb.state = StateResult
		cb.input = "Command would execute: " + cb.llmTranslation.Command
		cb.height = 2
		cb.llmTranslation = nil
		return cb, nil

	case "e":
		// Edit mode - transition to input state with generated command
		// For this prototype, we'll skip edit mode
		// Just dismiss for now
		cb.state = StateHidden
		cb.input = ""
		cb.height = 1
		cb.llmTranslation = nil
		return cb, nil
	}
	return cb, nil
}

// handleResultState handles result display
func (cb *CommandBar) handleResultState(msg tea.KeyMsg) (*CommandBar, tea.Cmd) {
	switch msg.String() {
	case "esc":
		// Dismiss result and return to hidden
		cb.state = StateHidden
		cb.input = ""
		cb.height = 1
		return cb, nil
	}
	return cb, nil
}

// View renders the command bar
func (cb *CommandBar) View() string {
	// Add horizontal separator lines
	separatorStyle := lipgloss.NewStyle().
		Foreground(cb.theme.Border).
		Width(cb.width)
	separator := separatorStyle.Render(strings.Repeat("─", cb.width))

	var content string
	switch cb.state {
	case StateHidden:
		return "" // Don't render anything when hidden (hints will show below)
	case StateFilter:
		content = cb.viewFilter()
	case StateSuggestionPalette:
		content = cb.viewPalette()
	case StateInput:
		content = cb.viewInput()
	case StateConfirmation:
		content = cb.viewConfirmation()
	case StateLLMPreview:
		content = cb.viewLLMPreview()
	case StateResult:
		content = cb.viewResult()
	default:
		return ""
	}

	return lipgloss.JoinVertical(lipgloss.Left, separator, content, separator)
}

// ViewHints renders the hints line (shown below command bar)
func (cb *CommandBar) ViewHints() string {
	hintStyle := lipgloss.NewStyle().
		Foreground(cb.theme.Subtle).
		Width(cb.width).
		Padding(0, 1)

	separatorStyle := lipgloss.NewStyle().
		Foreground(cb.theme.Border).
		Width(cb.width)
	separator := separatorStyle.Render(strings.Repeat("─", cb.width))

	// Show hints only when command bar is hidden
	// (When palette/filter is active, the UI already shows what's happening)
	if cb.state == StateHidden {
		hints := hintStyle.Render("[type to filter  : resources  / commands]")
		return lipgloss.JoinVertical(lipgloss.Left, separator, hints, separator)
	}

	// Empty for other states (filter, palette, confirmation, etc.)
	return ""
}

// viewFilter renders the filter state
func (cb *CommandBar) viewFilter() string {
	barStyle := lipgloss.NewStyle().
		Foreground(cb.theme.Foreground).
		Width(cb.width).
		Padding(0, 1)

	// Show input with cursor
	display := cb.input + "█"

	// Add argument hint if applicable
	hint := cb.getArgumentHint()
	if hint != "" {
		hintStyle := lipgloss.NewStyle().
			Foreground(cb.theme.Dimmed).
			Italic(true)
		display += hintStyle.Render(hint)
	}

	return barStyle.Render(display)
}

// viewPalette renders just the input line for palette mode
func (cb *CommandBar) viewPalette() string {
	inputStyle := lipgloss.NewStyle().
		Foreground(cb.theme.Foreground).
		Width(cb.width).
		Padding(0, 1)

	hintStyle := lipgloss.NewStyle().
		Foreground(cb.theme.Dimmed).
		Italic(true)

	inputDisplay := cb.input + "█"

	// Add argument hint if applicable
	argHint := cb.getArgumentHint()
	if argHint != "" {
		inputDisplay += hintStyle.Render(argHint)
	}

	return inputStyle.Render(inputDisplay)
}

// ViewPaletteItems renders the palette items (shown below command bar)
func (cb *CommandBar) ViewPaletteItems() string {
	if cb.state != StateSuggestionPalette || len(cb.paletteItems) == 0 {
		return ""
	}

	sections := []string{}

	// Show up to 8 items
	maxItems := 8
	if len(cb.paletteItems) < maxItems {
		maxItems = len(cb.paletteItems)
	}

	// First pass: find longest description to align shortcuts
	longestMainText := 0
	for i := 0; i < maxItems; i++ {
		cmd := cb.paletteItems[i]
		prefix := cb.input[:1]
		mainText := prefix + cmd.Name
		if cmd.ArgPattern != "" {
			mainText += cmd.ArgPattern
		}
		mainText += " - " + cmd.Description
		if len(mainText) > longestMainText {
			longestMainText = len(mainText)
		}
	}

	// Add 10 spaces for separation
	shortcutColumn := longestMainText + 10

	// Second pass: render items with aligned shortcuts
	for i := 0; i < maxItems; i++ {
		cmd := cb.paletteItems[i]
		prefix := cb.input[:1] // Get the : or / prefix
		mainText := prefix + cmd.Name
		if cmd.ArgPattern != "" {
			mainText += cmd.ArgPattern
		}
		mainText += " - " + cmd.Description

		var line string
		if cmd.Shortcut != "" {
			// Pad to shortcut column position
			padding := shortcutColumn - len(mainText)
			if padding < 2 {
				padding = 2 // Minimum 2 spaces
			}
			spacer := strings.Repeat(" ", padding)

			// Style shortcut with dimmed color
			shortcutStyle := lipgloss.NewStyle().
				Foreground(cb.theme.Dimmed)
			styledShortcut := shortcutStyle.Render(cmd.Shortcut)

			itemContent := mainText + spacer + styledShortcut

			if i == cb.paletteIdx {
				selectedStyle := lipgloss.NewStyle().
					Foreground(cb.theme.Foreground).
					Background(cb.theme.Subtle).
					Width(cb.width).
					Padding(0, 1).
					Bold(true)
				line = selectedStyle.Render("▶ " + itemContent)
			} else {
				paletteStyle := lipgloss.NewStyle().
					Width(cb.width).
					Padding(0, 1)
				line = paletteStyle.Render("  " + itemContent)
			}
		} else {
			// No shortcut, simple rendering
			if i == cb.paletteIdx {
				selectedStyle := lipgloss.NewStyle().
					Foreground(cb.theme.Foreground).
					Background(cb.theme.Subtle).
					Width(cb.width).
					Padding(0, 1).
					Bold(true)
				line = selectedStyle.Render("▶ " + mainText)
			} else {
				paletteStyle := lipgloss.NewStyle().
					Width(cb.width).
					Padding(0, 1)
				line = paletteStyle.Render("  " + mainText)
			}
		}

		sections = append(sections, line)
	}

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

// viewInput renders direct input state
func (cb *CommandBar) viewInput() string {
	// TODO: Implement input view
	return cb.viewFilter() // Use filter view for now
}

// viewConfirmation renders confirmation state
func (cb *CommandBar) viewConfirmation() string {
	if cb.pendingCommand == nil {
		return ""
	}

	// Create styles
	titleStyle := lipgloss.NewStyle().
		Foreground(cb.theme.Error).
		Bold(true).
		Width(cb.width).
		Padding(0, 1)

	textStyle := lipgloss.NewStyle().
		Foreground(cb.theme.Foreground).
		Width(cb.width).
		Padding(0, 1)

	hintStyle := lipgloss.NewStyle().
		Foreground(cb.theme.Subtle).
		Width(cb.width).
		Padding(0, 1)

	// Build confirmation content
	lines := []string{}
	lines = append(lines, titleStyle.Render("⚠ Confirm Action"))
	lines = append(lines, textStyle.Render(""))
	lines = append(lines, textStyle.Render("Command: /"+cb.pendingCommand.Name))
	lines = append(lines, textStyle.Render("This action cannot be undone."))
	lines = append(lines, hintStyle.Render("[Enter] Confirm  [ESC] Cancel"))

	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

// viewLLMPreview renders LLM preview state
func (cb *CommandBar) viewLLMPreview() string {
	if cb.llmTranslation == nil {
		return ""
	}

	// Create styles
	titleStyle := lipgloss.NewStyle().
		Foreground(cb.theme.Primary).
		Bold(true).
		Width(cb.width).
		Padding(0, 1)

	promptStyle := lipgloss.NewStyle().
		Foreground(cb.theme.Dimmed).
		Italic(true).
		Width(cb.width).
		Padding(0, 1)

	commandStyle := lipgloss.NewStyle().
		Foreground(cb.theme.Success).
		Width(cb.width).
		Padding(0, 1)

	explanationStyle := lipgloss.NewStyle().
		Foreground(cb.theme.Foreground).
		Width(cb.width).
		Padding(0, 1)

	hintStyle := lipgloss.NewStyle().
		Foreground(cb.theme.Subtle).
		Width(cb.width).
		Padding(0, 1)

	// Build LLM preview content
	lines := []string{}
	lines = append(lines, titleStyle.Render("🤖 AI Command Preview"))
	lines = append(lines, promptStyle.Render("Prompt: "+cb.llmTranslation.Prompt))
	lines = append(lines, commandStyle.Render("Command: "+cb.llmTranslation.Command))
	lines = append(lines, explanationStyle.Render(cb.llmTranslation.Explanation))
	lines = append(lines, hintStyle.Render("[Enter] Execute  [e] Edit  [ESC] Cancel"))

	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

// viewResult renders result state
func (cb *CommandBar) viewResult() string {
	resultStyle := lipgloss.NewStyle().
		Foreground(cb.theme.Success).
		Background(cb.theme.Background).
		Width(cb.width).
		Padding(0, 1).
		Bold(true)

	return resultStyle.Render("✓ " + cb.input)
}

// getArgumentHint returns the argument pattern hint for the current input
// Shows remaining args as user types: "/logs " → "[container] [tail] [follow]"
//                                      "/logs nginx " → "[tail] [follow]"
func (cb *CommandBar) getArgumentHint() string {
	// Only show hints for command inputs (: or /)
	if len(cb.input) == 0 {
		return ""
	}

	prefix := cb.input[:1]
	if prefix != ":" && prefix != "/" {
		return ""
	}

	// Parse command name and args from input
	parts := strings.Fields(cb.input)
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
	} else if strings.HasPrefix(cb.input, "/ai ") {
		category = commands.CategoryLLMAction
	} else {
		category = commands.CategoryAction
	}

	// Look up command in registry
	cmd := cb.registry.Get(cmdName, category)
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
	if !strings.HasSuffix(cb.input, " ") && typedArgsCount > 0 {
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

// getPaletteItems returns palette items for the given command type and query
// Handles special case of /ai for LLM commands
// Filters resource commands by current screen (resource type)
func (cb *CommandBar) getPaletteItems(cmdType CommandType, query string) []commands.Command {
	var items []commands.Command

	switch cmdType {
	case CommandTypeResource:
		category := commands.CategoryResource
		if query == "" {
			items = cb.registry.GetByCategory(category)
		} else {
			items = cb.registry.Filter(query, category)
		}

	case CommandTypeAction:
		category := commands.CategoryAction
		if query == "" {
			items = cb.registry.GetByCategory(category)
		} else {
			items = cb.registry.Filter(query, category)
		}

		// Filter by current screen (resource type)
		items = cb.registry.FilterByResourceType(items, cb.screenID)

		// Add /ai option if it matches the query
		if strings.HasPrefix("ai", strings.ToLower(query)) || query == "" {
			items = append(items, commands.Command{
				Name:        "ai",
				Description: "Natural language AI commands",
				Category:    commands.CategoryLLMAction,
				Execute:     nil,
			})
		}
	}

	return items
}

// transitionToPalette transitions to palette state with the given input and type
func (cb *CommandBar) transitionToPalette(input string, cmdType CommandType) {
	cb.state = StateSuggestionPalette
	cb.input = input
	cb.inputType = cmdType
	cb.cursorPos = len(input)
	cb.paletteVisible = true

	// Extract query (everything after the prefix)
	query := ""
	if len(input) > 1 {
		query = input[1:]
	}

	cb.paletteItems = cb.getPaletteItems(cmdType, query)
	cb.paletteIdx = 0

	// Calculate height: 1 (input line) + number of items (max 8)
	itemCount := len(cb.paletteItems)
	if itemCount > 8 {
		itemCount = 8
	}
	cb.height = 1 + itemCount
}
