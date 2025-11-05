# k9s Keyboard Shortcuts Alignment Implementation Plan

## Overview

This plan implements keyboard shortcut changes to align k1 with k9s
conventions, reducing friction for k9s users transitioning to k1. The
primary changes involve:

1. **Centralized keyboard configuration** - All shortcuts defined in
   `internal/keyboard/keys.go` for easy future customization
2. **Command bar activation** - `/` for search, `ctrl+p` for palette
3. **Context switching** - `[`/`]` bracket keys (was `ctrl+p`/`ctrl+n`)
4. **Resource operations** - Single keys `d`, `e`, `l`, `y` (was
   `ctrl+key`)
5. **Vim navigation** - `j`/`k`/`g`/`G` for list navigation
6. **Help screen** - `?` key shows all shortcuts
7. **Intentional quit** - Remove single `q`, require `:q` or `ctrl+c`

## Current State Analysis

### Key Findings:

**Command Bar State Machine** (`commandbar/types.go:16-26`):
- States: Hidden, Filter, SuggestionPalette, Input, Confirmation,
  LLMPreview, Result
- Current activation: `/` → SuggestionPalette (line 255-256),
  typing → Filter (line 258-268)

**Global Key Handling** (`app/app.go:206-250`):
- `ctrl+c`, `q` → Quit (line 209)
- `ctrl+p` → Previous context (line 212-216)
- `ctrl+n` → Next context (line 218-222)
- `ctrl+r` → Global refresh (line 224-229)
- Shortcut lookup via `GetCommandByShortcut()` (line 233-249)

**Command Registry Shortcuts** (`commands/registry.go`):
- `ctrl+y` → YAML (line 168)
- `ctrl+d` → Describe (line 176)
- `ctrl+x` → Delete (line 184)
- `ctrl+e` → Edit (line 193)
- `ctrl+l` → Logs (line 201)

**Command Bar Activation** (`commandbar/commandbar.go:229-272`):
- `:` → Palette with CommandTypeResource (line 252-254)
- `/` → Palette with CommandTypeAction (line 255-257)
- Any single char → Filter mode (line 258-268)
- Paste events → Filter mode (line 240-248)

**Usage Tips** (`commandbar/commandbar.go:18-36`):
- First tip shows old shortcuts (line 21)
- Multiple tips reference old key bindings

### Constraints Discovered:

1. **Type-to-filter behavior**: Currently any character triggers filter
   mode (line 260) - must be removed for Phase 1
2. **Shortcut registry pattern**: Commands use `Shortcut` field in
   registry, looked up via `GetByShortcut()` - clean abstraction for
   changes
3. **Context switch helpers**: Functions `prevContextKey()` and
   `nextContextKey()` in both `app.go` (lines 38-45) and
   `registry.go` (lines 15-23) - must update both
4. **Command bar height management**: State changes trigger height
   recalculations - must test carefully after changes

## Desired End State

After Phase 2 completion:

1. **Core shortcuts aligned with k9s**:
   - `/` activates filter (no auto-filter on typing)
   - `ctrl+p` opens command palette
   - `:` navigates to resources (unchanged)
   - `[`/`]` switch contexts (replaces `ctrl+p`/`ctrl+n`)

2. **Resource operations use single keys**:
   - `d` → Describe (was `ctrl+d`)
   - `e` → Edit (was `ctrl+e`)
   - `l` → Logs (was `ctrl+l`)
   - `y` → YAML (was `ctrl+y`)
   - `ctrl+x` → Delete (unchanged)

3. **Vim navigation works**:
   - `j`/`k` → Up/down
   - `g`/`G` → Jump to top/bottom

4. **Help overlay available**:
   - `?` → Shows all shortcuts

5. **Documentation updated**:
   - Usage tips reflect new shortcuts
   - README.md updated
   - CLAUDE.md updated

### Verification:

**Automated:**
- `make build` succeeds without errors
- `make test` passes all tests
- No compilation errors

**Manual:**
- User tests all shortcuts in live environment
- Muscle memory from k9s works correctly
- All screens respond to new shortcuts
- Help overlay shows accurate information

## What We're NOT Doing

- Phase 3 features (copy `c`, mark/select `space`, previous logs
  `shift+p`, attach `a`)
- Removing single `q` quit key (research suggests keeping it)
- Creating configuration for shortcut customization
- Adding modeless operation (no vim-style modes)
- Implementing batch operations (requires mark/select first)

## Implementation Approach

Three-phase implementation with user testing between phases:

**Phase 1: Core Shortcuts (Breaking Changes)**
- Focus on k9s muscle memory (most painful mismatches)
- Swap activation keys, move context switching, change resource ops
- User tests and confirms before Phase 2

**Phase 2: Navigation & Help**
- Add vim navigation and help overlay
- Enhance usability without breaking changes
- User tests and confirms completion

**Phase 3: Future Enhancements**
- Copy, mark/select, batch operations
- Deferred to future work based on user feedback

## Phase 1: Core Shortcuts (Breaking Changes)

### Overview

Align the most painful mismatches between k1 and k9s: command bar
activation, context switching, and resource operation shortcuts.

### Changes Required:

#### 1. Create Centralized Keyboard Configuration Package

**New File**: `internal/keyboard/keys.go`

Create a centralized package for all keyboard shortcuts to enable
future configuration without hardcoding keys throughout the codebase.

```go
package keyboard

// Keys holds all keyboard shortcut configurations for k1
type Keys struct {
	// Command Bar Activation
	FilterActivate  string // Activate filter mode
	PaletteActivate string // Activate command palette
	ResourceNav     string // Navigate to resources

	// Context Switching
	PrevContext string // Previous Kubernetes context
	NextContext string // Next Kubernetes context

	// Resource Operations
	Describe string // Describe resource
	Edit     string // Edit resource
	Logs     string // View logs
	YAML     string // View YAML
	Delete   string // Delete resource

	// Navigation
	Up            string // Move selection up
	Down          string // Move selection down
	JumpTop       string // Jump to top
	JumpBottom    string // Jump to bottom
	PageUp        string // Page up
	PageDown      string // Page down
	NamespaceFilter string // Namespace filter

	// Global
	Quit      string // Quit application
	Refresh   string // Refresh data
	Back      string // Back/clear filter
	Help      string // Show help
}

// Default returns the default k9s-aligned keyboard configuration
func Default() *Keys {
	return &Keys{
		// Command Bar Activation
		FilterActivate:  "/",
		PaletteActivate: "ctrl+p",
		ResourceNav:     ":",

		// Context Switching
		PrevContext: "[",
		NextContext: "]",

		// Resource Operations
		Describe: "d",
		Edit:     "e",
		Logs:     "l",
		YAML:     "y",
		Delete:   "ctrl+x",

		// Navigation
		Up:              "k",
		Down:            "j",
		JumpTop:         "g",
		JumpBottom:      "G",
		PageUp:          "ctrl+b",
		PageDown:        "ctrl+f",
		NamespaceFilter: "n",

		// Global
		Quit:    "ctrl+c", // Note: :q command also works
		Refresh: "ctrl+r",
		Back:    "esc",
		Help:    "?",
	}
}

// GetKeys returns the current keyboard configuration
// Future: This will load from config file
func GetKeys() *Keys {
	return Default()
}
```

**Rationale**:
- Single source of truth for all keyboard shortcuts
- Easy to add configuration file support later
- Self-documenting with clear categories
- Type-safe access to key bindings

#### 2. Update App Model to Use Keyboard Config

**File**: `internal/app/app.go`

Add keyboard config to Model struct (after line 72):

```go
type Model struct {
	state             types.AppState
	registry          *types.ScreenRegistry
	currentScreen     types.Screen
	header            *components.Header
	layout            *components.Layout
	userMessage       *components.UserMessage
	commandBar        *commandbar.CommandBar
	fullScreen        *components.FullScreen
	fullScreenMode    bool
	navigationHistory []NavigationState
	repoPool          *k8s.RepositoryPool
	theme             *ui.Theme
	messageID         int
	outputBuffer      *components.OutputBuffer
	keys              *keyboard.Keys // Add keyboard configuration
}
```

Initialize in `NewModel` (after line 159):

```go
return Model{
	state: types.AppState{
		CurrentScreen: "pods",
		Width:         80,
		Height:        24,
	},
	registry:          registry,
	currentScreen:     initialScreen,
	header:            header,
	layout:            layout,
	userMessage:       userMessage,
	commandBar:        cmdBar,
	navigationHistory: make([]NavigationState, 0, MaxNavigationHistorySize),
	repoPool:          pool,
	theme:             theme,
	outputBuffer:      outputBuffer,
	keys:              keyboard.GetKeys(), // Initialize keyboard config
}
```

Remove old helper functions (lines 38-45) - no longer needed:

```go
// DELETE these functions:
// func prevContextKey() string { return "ctrl+p" }
// func nextContextKey() string { return "ctrl+n" }
```

Update key handling in `Update` function (lines 206-230):

```go
case tea.KeyMsg:
	// Handle global shortcuts
	switch msg.String() {
	case m.keys.Quit:
		return m, tea.Quit

	case m.keys.PrevContext:
		// Previous context ([)
		updatedBar, barCmd := m.commandBar.ExecuteCommand("prev-context", commands.CategoryResource)
		m.commandBar = updatedBar
		return m, barCmd

	case m.keys.NextContext:
		// Next context (])
		updatedBar, barCmd := m.commandBar.ExecuteCommand("next-context", commands.CategoryResource)
		m.commandBar = updatedBar
		return m, barCmd

	case m.keys.Refresh:
		// Global refresh - trigger current screen refresh
		if screen, ok := m.currentScreen.(interface{ Refresh() tea.Cmd }); ok {
			return m, screen.Refresh()
		}
		return m, nil

	case m.keys.NamespaceFilter:
		// Namespace filter (n)
		updatedBar, barCmd := m.commandBar.ExecuteCommand("ns", commands.CategoryResource)
		m.commandBar = updatedBar
		return m, barCmd

	case m.keys.Help:
		// Show help screen (?)
		return m.Update(types.ScreenSwitchMsg{
			ScreenID:    "help",
			PushHistory: true,
		})

	case m.keys.Down:
		// Vim navigation: j -> down arrow
		return m.Update(tea.KeyMsg{Type: tea.KeyDown})

	case m.keys.Up:
		// Vim navigation: k -> up arrow
		return m.Update(tea.KeyMsg{Type: tea.KeyUp})

	case m.keys.JumpTop:
		// Vim navigation: g -> jump to top
		model, cmd := m.currentScreen.Update(tea.KeyMsg{
			Type: tea.KeyRunes,
			Runes: []rune{'g'},
		})
		m.currentScreen = model.(types.Screen)
		return m, cmd

	case m.keys.JumpBottom:
		// Vim navigation: G -> jump to bottom
		model, cmd := m.currentScreen.Update(tea.KeyMsg{
			Type: tea.KeyRunes,
			Runes: []rune{'G'},
		})
		m.currentScreen = model.(types.Screen)
		return m, cmd
	}
```

#### 3. Update Command Bar to Use Keyboard Config

**File**: `internal/components/commandbar/commandbar.go`

Add keyboard config to CommandBar struct (after line 45):

```go
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
```

Update `New` function to accept and store keys (after line 64):

```go
func New(pool *k8s.RepositoryPool, theme *ui.Theme, keys *keyboard.Keys) *CommandBar {
	registry := commands.NewRegistry(pool, keys) // Pass keys to registry

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
		currentTipIndex: 0,
		lastTipRotation: time.Now(),
		keys:            keys, // Store keyboard config
	}
}
```

Update `handleHiddenState` to use keyboard config (lines 229-272):

```go
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

	case cb.keys.PaletteActivate: // "ctrl+p"
		cb.transitionToPalette("", CommandTypeAction)
		return cb, nil

	default:
		// REMOVED: type-to-filter behavior
		// No longer accept single chars to start filtering
	}

	return cb, nil
}
```

**Rationale**: Command bar now uses centralized keyboard config
instead of hardcoded strings. All key checks reference `cb.keys.*`.

#### 4. Update Commands Registry to Use Keyboard Config

**File**: `internal/commands/registry.go`

Update `NewRegistry` function signature to accept keys (line 26):

```go
func NewRegistry(pool *k8s.RepositoryPool, keys *keyboard.Keys) *Registry {
	commands := []Command{
		// ... navigation commands unchanged ...

		// Resource commands (/ prefix) - use keyboard config
		{
			Name:          "yaml",
			Description:   "View resource YAML",
			Category:      CategoryAction,
			ResourceTypes: []k8s.ResourceType{},
			Shortcut:      keys.YAML, // Use config instead of hardcoded
			Execute:       YamlCommand(pool),
		},
		{
			Name:          "describe",
			Description:   "View kubectl describe output",
			Category:      CategoryAction,
			ResourceTypes: []k8s.ResourceType{},
			Shortcut:      keys.Describe, // Use config
			Execute:       DescribeCommand(pool),
		},
		{
			Name:              "delete",
			Description:       "Delete selected resource",
			Category:          CategoryAction,
			ResourceTypes:     []k8s.ResourceType{},
			Shortcut:          keys.Delete, // Use config
			NeedsConfirmation: true,
			Execute:           DeleteCommand(pool),
		},
		{
			Name:          "edit",
			Description:   "Edit resource (clipboard)",
			Category:      CategoryAction,
			ResourceTypes: []k8s.ResourceType{},
			Shortcut:      keys.Edit, // Use config
			Execute:       EditCommand(pool),
		},
		{
			Name:          "logs",
			Description:   "View pod logs (clipboard)",
			Category:      CategoryAction,
			ResourceTypes: []k8s.ResourceType{k8s.ResourceTypePod},
			Shortcut:      keys.Logs, // Use config
			ArgsType:      &LogsArgs{},
			ArgPattern:    " [container] [tail] [follow]",
			Execute:       LogsCommand(pool),
		},
		// ... other commands ...
	}

	// Context management commands - use keyboard config
	commands = append(commands, []Command{
		// ... other context commands ...
		{
			Name:        "next-context",
			Description: "Switch to next context",
			Category:    CategoryResource,
			Execute:     NextContextCommand(pool),
			Shortcut:    keys.NextContext, // Use config
		},
		{
			Name:        "prev-context",
			Description: "Switch to previous context",
			Category:    CategoryResource,
			Execute:     PrevContextCommand(pool),
			Shortcut:    keys.PrevContext, // Use config
		},
	}...)

	return &Registry{
		commands: commands,
	}
}
```

Remove old helper functions (lines 15-23) - no longer needed:

```go
// DELETE these functions:
// func getNextContextShortcut() string { return "ctrl+n" }
// func getPrevContextShortcut() string { return "ctrl+p" }
```

**Rationale**: Commands registry now gets shortcuts from keyboard
config, making all shortcuts configurable from one place.

#### 5. Update App Initialization to Pass Keys

**File**: `internal/app/app.go`
**Function**: `NewModel`
**Line**: 127

Update commandbar initialization to pass keyboard config:

```go
cmdBar := commandbar.New(pool, theme, keyboard.GetKeys())
```

#### 6. Update Usage Tips

**File**: `internal/components/commandbar/commandbar.go`
**Lines**: 18-36

Replace all tips with updated shortcuts:

```go
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
```

#### 7. Implement `:q` Quit Command

**File**: `internal/commands/registry.go`

Add quit command to registry (in navigation commands section):

```go
{
	Name:        "q",
	Description: "Quit application",
	Category:    CategoryResource,
	Execute:     QuitCommand(),
},
```

**New File**: `internal/commands/quit.go`

```go
package commands

import (
	tea "github.com/charmbracelet/bubbletea"
)

// QuitCommand returns a command that quits the application
func QuitCommand() ExecuteFunc {
	return func(ctx ExecutionContext) tea.Cmd {
		return tea.Quit
	}
}
```

**Rationale**: Implements `:q` command for vim-style quit. Combined
with removing single `q` key, this requires intentional quit action.

#### 8. Handle `/` Key in StateFilter

**File**: `internal/components/commandbar/commandbar.go`
**Function**: `handleFilterState`
**Lines**: 274-321

Filter state already handles input correctly. When user types `/`, it
enters filter mode via `handleHiddenState`. The `/` character is
included in the input (line 262), so user sees `/xyz` as they type.
This matches k9s behavior.

**No changes needed** - existing code already correct.

### Success Criteria:

#### Automated Verification:
- [x] Build succeeds: `make build`
- [x] Tests pass: `make test`
- [x] No linting errors: `go vet ./...`

#### Manual Verification:
- [x] Keyboard config package created at `internal/keyboard/keys.go`
- [x] All key bindings centralized in `keyboard.Default()`
- [x] `/` activates filter mode with `/` shown as first character
- [x] Filter applies in real-time as you type after pressing `/`
- [x] Single-key commands (n, d, e, j, k, g, G) can be typed in filter mode
- [x] `ctrl+p` opens command palette with all actions (no prefix)
- [x] Commands can be typed in ctrl+p palette to filter (e.g., typing `d` shows describe, delete, etc.)
- [x] Command palette displays correctly without duplicating typed letters
- [x] `:` opens resource navigation palette (unchanged behavior)
- [x] `[` switches to previous context
- [x] `]` switches to next context
- [x] `d` key describes selected resource (when command bar hidden)
- [x] `e` key edits selected resource (clipboard)
- [x] `l` key shows logs for selected pod
- [x] `y` key shows YAML for selected resource
- [x] `ctrl+x` still deletes resource
- [x] `n` key opens namespace filter (when command bar hidden)
- [x] `?` key shows help screen
- [x] `j` key moves selection down (vim navigation, when command bar hidden)
- [x] `k` key moves selection up (vim navigation, when command bar hidden)
- [x] Single `q` no longer quits (must use `:q` or `ctrl+c`)
- [x] `:q` command quits application
- [x] Typing random characters no longer auto-filters (must press `/` first)
- [x] Usage tips show correct shortcuts
- [x] No hardcoded key strings in app.go or registry.go (all use keyboard config)

**Bug Fixes Applied:**
- [x] Fixed: Filter mode now strips `/` prefix before filtering (was searching for "/text" instead of "text")
- [x] Fixed: Global shortcuts guarded by `IsActive()` check (commands don't execute while typing in filter/palette)
- [x] Fixed: `ctrl+p` panic when input is empty (safe prefix extraction)
- [x] Fixed: Command execution panic in palette mode (safe prefix handling in handlePaletteEnter)
- [x] Fixed: Command display showing duplicated letters (ViewPaletteItems only treats `:` and `/` as prefixes)
- [x] Fixed: All palette state handlers safely check for prefix characters before extracting

**Implementation Note**: After completing this phase and all automated
verification passes, pause here for manual confirmation from the user
that the manual testing was successful before proceeding to Phase 2.

---

## Phase 2: Navigation & Help

### Overview

Add help screen and implement jump-to-top/bottom functionality for vim
navigation (vim keys j/k/g/G and namespace filter already implemented
in Phase 1).

### Changes Required:

#### 1. Implement Jump-to-Top/Bottom in Screens

**File**: `internal/screens/config.go` (ConfigScreen)
**Function**: `Update` (KeyMsg handler)
**Location**: Find switch on key types

Add handling for `g` and `G`:

```go
case tea.KeyMsg:
	switch msg.String() {
	case "g":
		// Jump to top
		if s.table != nil {
			s.table.GotoTop()
		}
		return s, nil
	case "G":
		// Jump to bottom
		if s.table != nil {
			s.table.GotoBottom()
		}
		return s, nil
	// ... existing cases
	}
```

**Research needed**: Verify table component has `GotoTop()` and
`GotoBottom()` methods. If not, implement using `SetCursor(0)` and
`SetCursor(len(rows)-1)`.

**Note**: Vim navigation keys (j/k/g/G) and namespace filter (n) were
already implemented in Phase 1 as part of the keyboard config
integration.

#### 2. Create Help Screen

**New File**: `internal/screens/help.go`

```go
package screens

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/renato0307/k1/internal/types"
	"github.com/renato0307/k1/internal/ui"
)

// HelpScreen shows keyboard shortcuts and help information
type HelpScreen struct {
	theme  *ui.Theme
	width  int
	height int
}

func NewHelpScreen(theme *ui.Theme) *HelpScreen {
	return &HelpScreen{
		theme:  theme,
		width:  80,
		height: 24,
	}
}

func (h *HelpScreen) ID() string                        { return "help" }
func (h *HelpScreen) Title() string                     { return "Help - Keyboard Shortcuts" }
func (h *HelpScreen) HelpText() string                  { return "Press ESC to close" }
func (h *HelpScreen) Operations() []types.Operation     { return nil }
func (h *HelpScreen) Init() tea.Cmd                     { return nil }
func (h *HelpScreen) Update(msg tea.Msg) (types.Screen, tea.Cmd) { return h, nil }

func (h *HelpScreen) SetSize(width, height int) {
	h.width = width
	h.height = height
}

func (h *HelpScreen) View() string {
	titleStyle := lipgloss.NewStyle().
		Foreground(h.theme.Primary).
		Bold(true).
		Padding(1, 0)

	sectionStyle := lipgloss.NewStyle().
		Foreground(h.theme.Secondary).
		Bold(true).
		Padding(0, 0, 0, 2)

	itemStyle := lipgloss.NewStyle().
		Foreground(h.theme.Foreground).
		Padding(0, 0, 0, 4)

	keyStyle := lipgloss.NewStyle().
		Foreground(h.theme.Accent).
		Bold(true)

	content := titleStyle.Render("k1 - Kubernetes TUI") + "\n\n"

	// Core Navigation
	content += sectionStyle.Render("Core Navigation") + "\n"
	content += itemStyle.Render(keyStyle.Render("/")+" - Search/filter current list") + "\n"
	content += itemStyle.Render(keyStyle.Render(":")+" - Navigate to resource/screen") + "\n"
	content += itemStyle.Render(keyStyle.Render("> or ctrl+p")+" - Open command palette") + "\n"
	content += itemStyle.Render(keyStyle.Render("esc")+" - Back/clear filter") + "\n\n"

	// Resource Operations
	content += sectionStyle.Render("Resource Operations") + "\n"
	content += itemStyle.Render(keyStyle.Render("d")+" - Describe selected resource") + "\n"
	content += itemStyle.Render(keyStyle.Render("e")+" - Edit resource (clipboard)") + "\n"
	content += itemStyle.Render(keyStyle.Render("l")+" - View logs (pods only)") + "\n"
	content += itemStyle.Render(keyStyle.Render("y")+" - View YAML") + "\n"
	content += itemStyle.Render(keyStyle.Render("ctrl+x")+" - Delete resource") + "\n"
	content += itemStyle.Render(keyStyle.Render("n")+" - Filter by namespace") + "\n\n"

	// Context Switching
	content += sectionStyle.Render("Context Switching") + "\n"
	content += itemStyle.Render(keyStyle.Render("[")+" - Previous Kubernetes context") + "\n"
	content += itemStyle.Render(keyStyle.Render("]")+" - Next Kubernetes context") + "\n\n"

	// List Navigation
	content += sectionStyle.Render("List Navigation") + "\n"
	content += itemStyle.Render(keyStyle.Render("↑/↓")+" or "+keyStyle.Render("j/k")+" - Move selection") + "\n"
	content += itemStyle.Render(keyStyle.Render("g")+" - Jump to top") + "\n"
	content += itemStyle.Render(keyStyle.Render("G")+" - Jump to bottom (shift+g)") + "\n"
	content += itemStyle.Render(keyStyle.Render("PgUp/PgDn")+" or "+keyStyle.Render("ctrl+b/ctrl+f")+" - Page up/down") + "\n\n"

	// Global
	content += sectionStyle.Render("Global") + "\n"
	content += itemStyle.Render(keyStyle.Render(":q")+" or "+keyStyle.Render("ctrl+c")+" - Quit application") + "\n"
	content += itemStyle.Render(keyStyle.Render("ctrl+r")+" - Refresh data") + "\n"
	content += itemStyle.Render(keyStyle.Render("?")+" - Show this help") + "\n\n"

	// Command Palette (when active)
	content += sectionStyle.Render("Command Palette") + "\n"
	content += itemStyle.Render(keyStyle.Render("↑/↓")+" - Navigate suggestions") + "\n"
	content += itemStyle.Render(keyStyle.Render("enter")+" - Execute command") + "\n"
	content += itemStyle.Render(keyStyle.Render("tab")+" - Auto-complete") + "\n"
	content += itemStyle.Render(keyStyle.Render("esc")+" - Cancel") + "\n"

	// Wrap in container with padding
	containerStyle := lipgloss.NewStyle().
		Width(h.width).
		Height(h.height).
		Padding(2, 4)

	return containerStyle.Render(content)
}
```

#### 3. Register Help Screen

**File**: `internal/app/app.go`
**Function**: `NewModel`
**After line**: 107 (after system screen registration)

Add:

```go
// Help screen
registry.Register(screens.NewHelpScreen(theme))
```

**File**: `internal/app/app.go`
**Function**: `initializeScreens`
**After line**: 839 (after system screen registration)

Add matching registration:

```go
// Help screen
m.registry.Register(screens.NewHelpScreen(m.theme))
```

**Note**: Help screen activation (`?` key) was already implemented in
Phase 1 as part of the keyboard config integration.

### Success Criteria:

#### Automated Verification:
- [x] Build succeeds: `make build`
- [x] Tests pass: `make test`
- [x] No linting errors: `go vet ./...`

#### Manual Verification:
- [x] `g` key jumps to top of list
- [x] `G` (shift+g) key jumps to bottom of list
- [x] Help screen displays all categories correctly
- [x] Help screen shows accurate shortcuts
- [x] ESC key exits help screen and returns to previous screen

**Implementation Note**: After completing this phase and all automated
verification passes, pause here for manual confirmation from the user
that the manual testing was successful.

### Bug Fixes Applied:

#### 1. Viewport Initialization Bug (Help/Output Screens)
**Problem**: App header "k1 💨 • current context: xxx" missing on first render of help/output screens

**Root cause**: Bubble Tea table viewport initialization bug. When `SetColumns()` is called on an empty table before data loads, the viewport renders with ~148 bytes of extra padding. This only affects screens with instant/synchronous data (help screen, output screen) via `CustomRefresh`, not resource screens with async data.

**Solution**:
- Automatic detection via `CustomRefresh != nil` (screens with instant data)
- Apply workaround in `RefreshCompleteMsg` handler: Send Down/Up key events + `SetHeight()`
- Removed hardcoded `RequiresViewportFix` flag (better UX - less cognitive load)

**Files changed**:
- `internal/screens/config.go`: Added automatic detection and workaround
- `internal/screens/help.go`: Removed `RequiresViewportFix` flag

#### 2. Help Screen Recursion Prevention
**Problem**: Pressing `?` when already on help screen pushes to navigation history unnecessarily

**Solution**: Check `m.currentScreen.ID() != screens.HelpScreenID` before switching screens

**Files changed**:
- `internal/app/app.go`: Added screen ID check in help key handler
- `internal/screens/help.go`: Added `HelpScreenID = "help"` constant

#### 3. Tab Completion Bug with Command Palette
**Problem**: After Tab completion in palette (e.g., typing `desc` → Tab → `describe `), pressing Enter doesn't execute command. ParseCommand was getting prefix="d", cmdName="escribe".

**Root cause**: Tab completion wasn't preserving command prefix. Input was "describe " without `/` prefix, causing incorrect parsing.

**Solution**: Implement `>` as palette prefix (like `:` for resources, `/` for filter)
- `>` and `ctrl+p` both open command palette
- All prefix checks updated to include `>`
- ParseCommand, handleInputEnter, GetArgumentHint recognize `>`

**Files changed**:
- `internal/components/commandbar/commandbar.go`: Handle `>` key, update prefix checks
- `internal/components/commandbar/input.go`: Update GetArgumentHint to recognize `>`
- `internal/screens/help.go`: Updated help entry to show "> or ctrl+p"

#### 4. Sticky Loading Messages for Full-Screen Commands
**Problem**: Commands like `/describe` showed "⏺ Running /describe..." message that got sticky and wasn't needed (user sees result immediately in full screen)

**Root cause**: Executor always sent loading message before executing commands

**Solution**:
- Removed automatic loading messages from executor
- Commands now control their own UX (can send loading via `tea.Batch` if needed)
- Added `Silent` flag to `StatusMsg` for history tracking without display

**Files changed**:
- `internal/components/commandbar/executor.go`: Removed automatic loading messages
- `internal/commands/resource.go`: Updated describe command to use Silent flag
- `internal/types/types.go`: Added Silent field to StatusMsg
- `internal/app/app.go`: Updated StatusMsg handler to respect Silent flag

#### 5. Help Screen Converted to Table Format
**Enhancement**: Help screen now uses ConfigScreen with table format, enabling:
- Search/filter functionality within help (press `/` to filter shortcuts)
- Consistent UX with resource screens
- Better organization with sortable columns

**Files changed**:
- `internal/screens/help.go`: Converted to ConfigScreen with CustomRefresh
- `internal/screens/screens.go`: Added GetHelpScreenConfig and getHelpEntries

### Phase 2 Status: ✅ COMPLETE

All Phase 2 features implemented and tested:
- ✅ Jump-to-top/bottom navigation (`g`/`G`)
- ✅ Help screen with searchable table format
- ✅ Viewport initialization bug fixed
- ✅ Tab completion with `>` prefix working
- ✅ Help screen recursion prevented
- ✅ Sticky loading messages eliminated
- ✅ `>` key as primary palette activation (cleaner than `ctrl+p` in docs)

---

## Phase 3: Enhanced Features (Future)

### Overview

Additional k9s features deferred to future work based on user feedback
and priority.

### Planned Features:

1. **Copy functionality** (`c` key)
   - Copy resource details to clipboard
   - Multiple format options (name, namespace, YAML)

2. **Mark/select items** (`space` key)
   - Toggle selection on items
   - Visual indicator for marked items
   - Batch operations on marked items

3. **Previous logs** (`shift+p`)
   - Quick access to previous container logs
   - Useful for crashed containers

4. **Attach/shell** (`a` key)
   - Quick shell access (currently `/shell` command)
   - Direct key binding for convenience

### Status

Deferred to future implementation based on:
- User feedback on Phase 1 and Phase 2
- Usage patterns and feature requests
- Priority relative to other roadmap items

---

## Testing Strategy

### Unit Tests

**Not required** for this implementation because:
1. Changes are primarily key routing (integration-level behavior)
2. Existing tests cover command execution logic
3. Manual testing is more effective for keyboard shortcuts

### Manual Testing Steps

**Phase 1 Testing**:
1. Start k1: `go run cmd/k1/main.go`
2. Test `/`: Type `/`, should see filter mode activate
3. Test `ctrl+p`: Press ctrl+p, should see command palette
4. Test `:`: Type `:`, should see resource navigation palette
5. Test `[`/`]`: Press bracket keys, should switch contexts
6. Test `d`, `e`, `l`, `y`: Press each, verify command executes
7. Test `ctrl+x`: Verify delete still works
8. Test `q`: Verify it does NOT quit (use `:q` or ctrl+c instead)
9. Test type-to-filter: Verify typing letters does NOT auto-filter
10. Check usage tips: Verify all tips show new shortcuts

**Phase 2 Testing**:
1. Test `j`/`k`: Verify selection moves up/down
2. Test `g`: Verify jumps to top of list
3. Test `G`: Verify jumps to bottom of list
4. Test `n`: Verify opens namespace filter
5. Test `?`: Verify help screen appears
6. Check help content: Verify all shortcuts are accurate
7. Test ESC from help: Verify returns to previous screen
8. Test vim keys during command input: Verify they don't interfere

### Performance Considerations

**No performance impact expected** because:
1. Key handling is already event-driven
2. No new data structures or processing
3. Changes only affect key routing logic
4. Command execution remains identical

### Migration Notes

**Breaking changes** in Phase 1:
- Users accustomed to old shortcuts must learn new ones
- No backward compatibility or grace period
- Hard cutover recommended (clean break)

**User communication**:
- Update README.md with new shortcuts
- Update CLAUDE.md developer docs
- Usage tips show new shortcuts immediately
- Help overlay (`?`) provides reference

**Rollback plan** (if needed):
- Revert git commits from feature branch
- Each phase is atomic and can be reverted independently

## References

- Research document: `thoughts/shared/research/2025-11-04-keyboard-shortcuts-k9s-alignment.md`
- Related research: `2025-10-09-k8s-context-management.md`
- Related research: `2025-11-02-command-output-history-design.md`
- Related research: `2025-11-02-usage-tips-display-alternatives.md`
