package components

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"timoneiro/internal/types"
	"timoneiro/internal/ui"
)

// CommandBarState represents the current state of the command bar
type CommandBarState int

const (
	StateHidden CommandBarState = iota
	StateFilter              // No prefix, filtering list
	StateSuggestionPalette   // : or / pressed, showing suggestions
	StateInput               // Direct command input
	StateConfirmation        // Destructive operation confirmation
	StateLLMPreview          // /x command preview
	StateResult              // Success/error message
)

// CommandType represents the type of command being entered
type CommandType int

const (
	CommandTypeFilter CommandType = iota // no prefix
	CommandTypeNavigation                 // : prefix
	CommandTypeResource                   // / prefix
	CommandTypeLLM                        // /x prefix
)

// CommandBar represents the expandable command bar at the bottom of the screen
type CommandBar struct {
	state      CommandBarState
	input      string
	inputType  CommandType
	width      int
	height     int // Dynamic, 1-10 lines
	theme      *ui.Theme
	cursorPos  int

	// History
	history    []string
	historyIdx int

	// Suggestion palette state
	paletteVisible bool
	paletteItems   []string // For now, just strings. Will be replaced with Command type later
	paletteIdx     int
}

// NewCommandBar creates a new command bar component
func NewCommandBar(theme *ui.Theme) *CommandBar {
	return &CommandBar{
		state:      StateHidden,
		input:      "",
		inputType:  CommandTypeFilter,
		width:      80,
		height:     1, // Start with 1 line
		theme:      theme,
		cursorPos:  0,
		history:    []string{},
		historyIdx: -1,
		paletteVisible: false,
		paletteItems:   []string{},
		paletteIdx:     0,
	}
}

// SetWidth updates the command bar width
func (cb *CommandBar) SetWidth(width int) {
	cb.width = width
}

// GetHeight returns the current height of the command bar (including separators, not hints)
func (cb *CommandBar) GetHeight() int {
	if cb.height == 0 {
		// Hidden state: just top separator
		return 1
	}
	// Add 2 for: top separator (1) + bottom separator (1)
	return cb.height + 2
}

// GetTotalHeight returns the height including hints line (for layout calculations)
func (cb *CommandBar) GetTotalHeight() int {
	// Command bar + hints line (1)
	return cb.GetHeight() + 1
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
	switch msg.String() {
	case ":":
		// Enter suggestion palette for navigation
		cb.state = StateSuggestionPalette
		cb.input = ":"
		cb.inputType = CommandTypeNavigation
		cb.cursorPos = 1
		cb.paletteVisible = true
		// TODO: Load navigation items
		cb.paletteItems = []string{"pods", "deployments", "services", "namespaces"}
		cb.paletteIdx = 0
		// Calculate height: 1 (input line) + number of items (max 8)
		itemCount := len(cb.paletteItems)
		if itemCount > 8 {
			itemCount = 8
		}
		cb.height = 1 + itemCount
		return cb, nil

	case "/":
		// Enter suggestion palette for commands
		cb.state = StateSuggestionPalette
		cb.input = "/"
		cb.inputType = CommandTypeResource
		cb.cursorPos = 1
		cb.paletteVisible = true
		// TODO: Load resource commands (context-aware)
		cb.paletteItems = []string{"yaml", "describe", "delete", "logs"}
		cb.paletteIdx = 0
		// Calculate height: 1 (input line) + number of items (max 8)
		itemCount := len(cb.paletteItems)
		if itemCount > 8 {
			itemCount = 8
		}
		cb.height = 1 + itemCount
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
	switch msg.String() {
	case "esc":
		// Dismiss palette and return to hidden
		cb.state = StateHidden
		cb.input = ""
		cb.cursorPos = 0
		cb.height = 1
		cb.paletteVisible = false
		cb.paletteItems = []string{}
		cb.paletteIdx = 0
		return cb, nil

	case "enter":
		// Select item from palette
		if cb.paletteIdx >= 0 && cb.paletteIdx < len(cb.paletteItems) {
			selected := cb.paletteItems[cb.paletteIdx]
			// TODO: Execute the selected command
			// For now, just return to hidden
			cb.state = StateHidden
			cb.input = ""
			cb.cursorPos = 0
			cb.height = 1
			cb.paletteVisible = false
			cb.paletteItems = []string{}
			cb.paletteIdx = 0
			_ = selected // Will use this to execute command
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

	case "backspace":
		// Remove character and filter palette
		if len(cb.input) > 1 { // Keep prefix (: or /)
			cb.input = cb.input[:len(cb.input)-1]
			cb.cursorPos--
			// TODO: Re-filter palette items
			// For now, recalculate height based on current items
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
			cb.paletteItems = []string{}
			cb.paletteIdx = 0
		}
		return cb, nil

	default:
		// Add character and filter palette
		if len(msg.String()) == 1 {
			cb.input += msg.String()
			cb.cursorPos++
			// TODO: Re-filter palette items based on new input
			// For now, recalculate height based on current items
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
	// TODO: Implement direct input handling
	return cb, nil
}

// handleConfirmationState handles confirmation prompts
func (cb *CommandBar) handleConfirmationState(msg tea.KeyMsg) (*CommandBar, tea.Cmd) {
	// TODO: Implement confirmation handling
	return cb, nil
}

// handleLLMPreviewState handles LLM command preview
func (cb *CommandBar) handleLLMPreviewState(msg tea.KeyMsg) (*CommandBar, tea.Cmd) {
	// TODO: Implement LLM preview handling
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
		Foreground(lipgloss.Color("240")).
		Width(cb.width)
	separator := separatorStyle.Render(strings.Repeat("─", cb.width))

	var content string
	switch cb.state {
	case StateHidden:
		content = "" // No content when hidden
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

	if content == "" {
		return separator
	}

	return lipgloss.JoinVertical(lipgloss.Left, separator, content, separator)
}

// ViewHints renders the hints line (shown below command bar)
func (cb *CommandBar) ViewHints() string {
	hintStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Width(cb.width).
		Padding(0, 1)

	// Show hints only when command bar is hidden or in filter mode
	// (When palette is active, options are already visible in the palette)
	if cb.state == StateHidden || cb.state == StateFilter {
		return hintStyle.Render("[: screens  / commands]")
	}

	// Empty for other states (palette, confirmation, etc.)
	return hintStyle.Render("")
}

// viewHidden renders the hidden state (no content, just separators and hints)
func (cb *CommandBar) viewHidden() string {
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

	return barStyle.Render(display)
}

// viewPalette renders just the input line for palette mode
func (cb *CommandBar) viewPalette() string {
	inputStyle := lipgloss.NewStyle().
		Foreground(cb.theme.Foreground).
		Width(cb.width).
		Padding(0, 1)

	inputDisplay := cb.input + "█"
	return inputStyle.Render(inputDisplay)
}

// ViewPaletteItems renders the palette items (shown below command bar)
func (cb *CommandBar) ViewPaletteItems() string {
	if cb.state != StateSuggestionPalette || len(cb.paletteItems) == 0 {
		return ""
	}

	sections := []string{}

	paletteStyle := lipgloss.NewStyle().
		Width(cb.width).
		Padding(0, 1)

	selectedStyle := lipgloss.NewStyle().
		Foreground(cb.theme.Primary).
		Width(cb.width).
		Padding(0, 1).
		Bold(true)

	// Show up to 8 items
	maxItems := 8
	if len(cb.paletteItems) < maxItems {
		maxItems = len(cb.paletteItems)
	}

	for i := 0; i < maxItems; i++ {
		prefix := cb.input[:1] // Get the : or / prefix
		itemText := prefix + cb.paletteItems[i]

		if i == cb.paletteIdx {
			sections = append(sections, selectedStyle.Render("> "+itemText))
		} else {
			sections = append(sections, paletteStyle.Render("  "+itemText))
		}
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
	// TODO: Implement confirmation view
	return ""
}

// viewLLMPreview renders LLM preview state
func (cb *CommandBar) viewLLMPreview() string {
	// TODO: Implement LLM preview view
	return ""
}

// viewResult renders result state
func (cb *CommandBar) viewResult() string {
	resultStyle := lipgloss.NewStyle().
		Foreground(cb.theme.Success).
		Background(lipgloss.Color("235")).
		Width(cb.width).
		Padding(0, 1).
		Bold(true)

	return resultStyle.Render("✓ " + cb.input)
}

// Helper function to detect command type from input
func detectCommandType(input string) CommandType {
	if strings.HasPrefix(input, "/x ") {
		return CommandTypeLLM
	}
	if strings.HasPrefix(input, "/") {
		return CommandTypeResource
	}
	if strings.HasPrefix(input, ":") {
		return CommandTypeNavigation
	}
	return CommandTypeFilter
}
