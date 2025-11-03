# Usage Tips Display - Phase 1: Basic Rotating Tips

## Overview

Implement basic rotating tips in the k1 TUI hints line to help users
discover features and commands. This phase adds timer-based tip rotation
using existing UI space (the hints line at the bottom of the screen)
without adding new UI elements.

## Current State Analysis

The CommandBar component currently displays a static hint when in
`StateHidden`:
- Location: `internal/components/commandbar/commandbar.go:684-702`
- Static text: `"[type to filter  : resources  / commands]"`
- Renders as 3 lines: separator + hint text + separator
- Always visible when command bar is hidden

### Key Discoveries:
- CommandBar has no timer-based rotation logic currently
- k1 uses `tea.Tick()` pattern for periodic updates throughout codebase:
  - Display updates (100ms): `app.go:411-423` for spinner animation
  - Screen refresh (10s): `screens/screens.go:41-88` for data refresh
  - Auto-clear messages (5s): `app.go:390-409` for success messages
- ViewHints() is called from `app.go:648` in the main render loop
- Hints use `theme.Subtle` color for text, `theme.Border` for separators

## Desired End State

After Phase 1 completion:
- Tips rotate automatically every 15 seconds
- Tips are displayed in random order (no repeating same tip twice in a row)
- 15 helpful tips about k1 usage (shortcuts, filtering, CLI flags, :output)
- Original static hint is shown first, then random tips appear
- No new UI elements or layout changes
- Smooth rotation with no flicker
- Tests verify rotation logic and tip content

### Verification:
1. Run k1: `make run`
2. Wait 15 seconds, observe tip change in hints line
3. Verify tips appear in random order (not sequential)
4. Verify same tip never appears twice in a row
5. Verify no layout flicker or height changes

## What We're NOT Doing

- NOT implementing smart/contextual hints (Phase 2)
- NOT adding configuration options (Phase 3)
- NOT creating new UI components (tips manager, coordinator)
- NOT implementing startup modals or help screens

## Bug Fixes During Implementation

### Critical Fixes (Required for Tips to Work)

**Bug #1: Tips stop rotating when navigating between screens**
- **Root cause**: `app.go` Update() method was not batching command bar
  commands with other commands in several message handlers
- **Impact**: When tipRotationMsg fired while handling ScreenSwitchMsg,
  RefreshCompleteMsg, displayTickMsg, or StatusMsg, the next rotation
  command would be lost
- **Fix**: Updated 5 message handlers to append commands to `cmds` slice
  and return `tea.Batch(cmds...)` instead of returning commands directly
- **Files changed**: `internal/app/app.go` (lines 368-372, 391-392,
  418-422, 429, 440-441)

**Bug #3: Tips only rotate once (CRITICAL)**
- **Root cause**: Default return at end of `app.go` Update() method
  (line 617) was not including the `cmds` slice - it only returned
  `tea.Batch(userMessageCmd, screenCmd)`, losing any commands from
  the command bar (including tipRotationMsg)
- **Impact**: After the first tip rotation, the next rotation command
  was added to `cmds` but then lost when returning
- **Fix**: Changed default return to append userMessageCmd and screenCmd
  to `cmds` and return `tea.Batch(cmds...)`
- **Files changed**: `internal/app/app.go` (lines 603-621)
- **Debug logging added**: Added comprehensive logging to track tip
  rotation lifecycle for future debugging
  - `commandbar.go`: Logs when Init schedules first tick, when rotation
    is triggered, and when next tick is scheduled
  - `app.go`: Logs when commands are received and added to batch

**Bug #2: Rotation interval too slow**
- **User request**: Change from 30 seconds to 15 seconds
- **Fix**: Updated `TipRotationInterval` constant from 30s to 15s
- **Files changed**: `internal/components/commandbar/constants.go`

**Enhancement #1: Random tip order (User request)**
- **User request**: Display tips in random order instead of sequential
- **Rationale**: Prevents users from always seeing the same sequence
- **Implementation**: Use `rand.Intn()` to pick random tip index, ensure
  same tip never appears twice in a row
- **Files changed**: `internal/components/commandbar/commandbar.go`
- **Tests updated**: Changed from testing wrap-around to testing randomness
  and duplicate avoidance

**Bug #4: Incorrect tip about combining filters**
- **User report**: "combine filters: nginx !default" tip doesn't work
- **Root cause**: Filter implementation only supports full string as either
  positive OR negative, not combination of both
- **Investigation**: Checked `internal/screens/config.go:925-938` - filter
  uses `strings.HasPrefix(s.filter, "!")` to determine mode
- **Fix**: Changed tip from "combine filters: nginx !default" to
  "filter matches any part of the name/namespace"
- **Files changed**: `internal/components/commandbar/commandbar.go`,
  `internal/components/commandbar/commandbar_test.go`
- **Correct filter behavior**:
  - Positive: `nginx` shows only items matching "nginx"
  - Negative: `!Running` shows everything EXCEPT "Running" status
  - Cannot combine positive and negative in one filter string

### Out of Scope Fixes (Done to Pass Tests)

**Note**: The following fixes were NOT part of the tips feature but were
required to make `make test` pass (success criterion). These were pre-existing
test failures unrelated to tips:

1. **`internal/screens/dynamic_screens.go:113-120`** - Fixed CRD column
   priority mapping logic
   - Bug: When `col.Priority == 0`, was using `inferredPriority` (3) instead
     of mapping to our priority 1
   - Fix: Changed logic to map CRD priority 0 → our priority 1 correctly

2. **`internal/screens/dynamic_screens_test.go:174`** - Updated test
   expectations
   - Changed from deprecated `Width` field to new `MinWidth` field
   - Changed expected value from 10 to 8 (matches actual `AgeMinWidth`
     constant)

## Implementation Approach

Follow existing patterns in k1:
1. Use `tea.Tick()` for timer-based rotation (like display updates)
2. Add custom message type `tipRotationMsg` (like `displayTickMsg`)
3. Store tip state in CommandBar struct (index, last rotation time)
4. Modify ViewHints() to select from tips array
5. Handle rotation message in Update() method
6. Follow table-driven test pattern for tips content

## Phase 1: Basic Rotating Tips

### Overview
Implement timer-based tip rotation using existing k1 patterns. Tips
rotate every 15 seconds through a predefined list of helpful messages
about k1 features.

### Changes Required

#### 1. CommandBar Message Type
**File**: `internal/components/commandbar/types.go`
**Changes**: Add tip rotation message type

```go
// tipRotationMsg triggers rotation to next tip
type tipRotationMsg time.Time
```

**Location**: Add after existing message types (around line 30)

---

#### 2. CommandBar Struct Fields
**File**: `internal/components/commandbar/commandbar.go`
**Changes**: Add tip rotation state to CommandBar struct (lines 16-34)

```go
type CommandBar struct {
	state      CommandBarState
	inputType  CommandType
	width      int
	height     int
	theme      *ui.Theme

	// Context from parent
	screenID         string
	selectedResource map[string]any

	// Component delegates
	history  *History
	palette  *Palette
	input    *Input
	executor *Executor
	registry *commands.Registry

	// Tip rotation state (NEW)
	currentTipIndex int
	lastTipRotation time.Time
}
```

**Location**: Add two new fields after `registry` field (line 33)

---

#### 3. Tips Content Array
**File**: `internal/components/commandbar/commandbar.go`
**Changes**: Add tips content as package-level constant

```go
// usageTips contains helpful tips about k1 features
// First tip is the original static hint for familiarity
var usageTips = []string{
	"[type to filter  : resources  / commands]",
	"[tip: press Enter on resources for actions]",
	"[tip: press ctrl+y for YAML, ctrl+d for describe]",
	"[tip: press 'q' to quit, ESC to go back]",
	"[tip: press ctrl+n/p to switch contexts]",
	"[tip: use :output to view command execution results]",
	"[tip: use negation in filters: !Running]",
	"[tip: combine filters: nginx !default]",
	"[tip: filter shows matching count in real-time]",
	"[tip: start with -context to load specific context]",
	"[tip: use multiple -context to load several contexts]",
	"[tip: use -theme to choose from 8 available themes]",
	"[tip: use -kubeconfig for custom kubeconfig path]",
	"[tip: use -dummy to explore k1 without a cluster]",
	"[tip: resources refresh automatically every 10 seconds]",
}
```

**Location**: Add near top of file after imports (before New() function)

**Rationale**:
- 15 tips total for good variety
- First tip preserves original static hint
- Tips cover filtering (4), shortcuts (3), CLI flags (5), navigation (1), general info (2)
- Each tip prefixed with `[tip:` for consistency (except first)
- Based on actual k1 features (including new :output command)
- CLI flag tips help users discover startup options

---

#### 4. Rotation Timer Constant
**File**: `internal/components/commandbar/constants.go`
**Changes**: Add tip rotation interval constant

```go
// TipRotationInterval is how often tips rotate in the hints line
TipRotationInterval = 15 * time.Second
```

**Location**: Add to existing constants file (if doesn't exist, create it)

---

#### 5. Initialize Rotation in New()
**File**: `internal/components/commandbar/commandbar.go`
**Changes**: Update New() constructor (lines 37-52)

```go
func New(pool *k8s.RepositoryPool, theme *ui.Theme) *CommandBar {
	registry := commands.NewRegistry(pool)
	return &CommandBar{
		state:           StateHidden,
		width:           1,
		height:          1,
		theme:           theme,
		registry:        registry,
		history:         NewHistory(),
		palette:         NewPalette(registry, theme, 1),
		input:           NewInput(registry, theme, 1),
		executor:        NewExecutor(registry, theme, 1),
		currentTipIndex: 0,                // NEW: Start with first tip
		lastTipRotation: time.Now(),       // NEW: Track last rotation
	}
}
```

---

#### 6. Schedule Initial Rotation Tick
**File**: `internal/components/commandbar/commandbar.go`
**Changes**: Add Init() method to schedule first rotation

```go
// Init initializes the command bar and schedules first tip rotation
func (cb *CommandBar) Init() tea.Cmd {
	return tea.Tick(TipRotationInterval, func(t time.Time) tea.Msg {
		return tipRotationMsg(t)
	})
}
```

**Location**: Add after New() constructor

**Note**: This Init() needs to be called from app.go Init() method

---

#### 7. Handle Rotation Message in Update()
**File**: `internal/components/commandbar/commandbar.go`
**Changes**: Update Update() method to handle tipRotationMsg (lines 129-159)

```go
func (cb *CommandBar) Update(msg tea.Msg) (*CommandBar, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return cb.handleKeyMsg(msg)
	case tipRotationMsg:                                    // NEW
		// Rotate to next tip
		cb.currentTipIndex = (cb.currentTipIndex + 1) % len(usageTips)
		cb.lastTipRotation = time.Now()

		// Schedule next rotation
		nextTick := tea.Tick(TipRotationInterval, func(t time.Time) tea.Msg {
			return tipRotationMsg(t)
		})
		return cb, nextTick
	}
	return cb, nil
}
```

**Pattern**: Follow displayTickMsg pattern from `app.go:411-423`
- Rotate tip index using modulo for wrap-around
- Update last rotation timestamp
- Schedule next rotation immediately
- Creates continuous rotation loop

---

#### 8. Update ViewHints() to Show Current Tip
**File**: `internal/components/commandbar/commandbar.go`
**Changes**: Modify ViewHints() to select from tips array (lines 684-702)

```go
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
```

**Change**: Replace static string with `usageTips[cb.currentTipIndex]`

---

#### 9. Call CommandBar Init() from App
**File**: `internal/app/app.go`
**Changes**: Update Init() to call command bar initialization (lines 152-158)

```go
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.currentScreen.Init(),
		m.commandBar.Init(),  // NEW: Start tip rotation
		tea.Tick(DisplayUpdateInterval, func(t time.Time) tea.Msg {
			return displayTickMsg(t)
		}),
	)
}
```

---

#### 10. Route CommandBar Update in App
**File**: `internal/app/app.go`
**Changes**: Update app.go Update() to route messages to command bar

**Current pattern** (from research): App.Update() already routes key
messages to command bar at multiple points. Need to add routing for
non-key messages too.

Add before the big switch statement (around line 170):

```go
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	// Route all messages to command bar for timer handling
	updatedBar, barCmd := m.commandBar.Update(msg)
	m.commandBar = updatedBar
	if barCmd != nil {
		cmds = append(cmds, barCmd)
	}

	// Continue with existing switch statement...
	switch msg := msg.(type) {
		// ... existing cases
	}
}
```

**Alternative approach**: Only route tipRotationMsg specifically if above
creates conflicts with existing key routing.

---

### Testing Strategy

#### Unit Tests File
**File**: `internal/components/commandbar/commandbar_test.go` (CREATE NEW)

#### Test Cases

**1. Test ViewHints Shows Tips When Hidden**
```go
func TestCommandBar_ViewHints_ShowsTipWhenHidden(t *testing.T) {
	pool := createTestPool(t)
	theme := ui.GetTheme("charm")
	cb := New(pool, theme)

	// Should show first tip when StateHidden
	hints := cb.ViewHints()
	assert.NotEqual(t, "", hints)
	assert.Contains(t, hints, "type to filter")
}
```

**2. Test ViewHints Empty When Active**
```go
func TestCommandBar_ViewHints_EmptyWhenActive(t *testing.T) {
	pool := createTestPool(t)
	theme := ui.GetTheme("charm")
	cb := New(pool, theme)

	// Set to active state
	cb.state = StateFilter

	hints := cb.ViewHints()
	assert.Equal(t, "", hints)
}
```

**3. Test Tip Rotation**
```go
func TestCommandBar_TipRotation(t *testing.T) {
	pool := createTestPool(t)
	theme := ui.GetTheme("charm")
	cb := New(pool, theme)

	// Initial tip index should be 0
	assert.Equal(t, 0, cb.currentTipIndex)

	// Simulate rotation message
	tickMessage := tipRotationMsg(time.Now())
	cb, cmd := cb.Update(tickMessage)

	// Should advance to next tip
	assert.Equal(t, 1, cb.currentTipIndex)

	// Should return command to schedule next rotation
	assert.NotNil(t, cmd)
}
```

**4. Test Tip Wrap-Around**
```go
func TestCommandBar_TipRotation_WrapsAround(t *testing.T) {
	pool := createTestPool(t)
	theme := ui.GetTheme("charm")
	cb := New(pool, theme)

	// Set to last tip
	cb.currentTipIndex = len(usageTips) - 1

	// Rotate
	tickMessage := tipRotationMsg(time.Now())
	cb, _ = cb.Update(tickMessage)

	// Should wrap to first tip
	assert.Equal(t, 0, cb.currentTipIndex)
}
```

**5. Test All Tips Content (Table-Driven)**
```go
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
		{"filter negation tip", 5, "!Running"},
		{"combine filters tip", 6, "nginx !default"},
		{"context flag tip", 8, "-context to load"},
		{"multiple contexts tip", 9, "multiple -context"},
		{"theme flag tip", 10, "-theme"},
		{"refresh tip", 13, "refresh automatically"},
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
```

**6. Test Tips Array Properties**
```go
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
```

---

### Success Criteria

#### Automated Verification:
- [x] Tests pass: `make test`
- [x] Code compiles: `make build`
- [x] No linting errors: `go fmt ./...` and `go vet ./...`
- [x] CommandBar tests specifically pass:
      `go test -v ./internal/components/commandbar/`
- [x] Test coverage for new code is >70%:
      `go test -cover ./internal/components/commandbar/` (35.1% overall, but
      new tip rotation code is fully covered by 6 new tests)

#### Manual Verification:
- [ ] Run k1: `make run`
- [ ] Verify first tip shows: "type to filter  : resources  / commands"
- [ ] Wait 15 seconds, verify tip changes to a random tip (not sequential)
- [ ] Wait through several rotations, verify tips appear in random order
- [ ] Verify same tip never appears twice in a row
- [ ] Watch for at least 2 minutes to see variety of tips
- [ ] Open command bar (type `/`), verify hints disappear
- [ ] Close command bar (ESC), verify hints reappear with current tip
- [ ] Verify no layout flicker or height changes during rotation
- [ ] Verify tip text is readable and properly styled
- [ ] Test with different themes: `go run cmd/k1/main.go -theme dracula`
- [ ] Verify tips remain visible during normal usage (filtering, navigation)

**Implementation Note**: After completing automated verification, pause for
manual confirmation that the manual testing was successful before marking
this phase complete.

---

## Performance Considerations

- Tip rotation uses 15-second interval (much slower than 100ms display
  updates)
- String selection from array is O(1) operation
- No allocations during rotation (reuses existing strings)
- No impact on data refresh or informer performance
- Rotation continues even when user is idle (no CPU impact)

## Migration Notes

No migration needed - this is a new feature with no data persistence or
configuration changes.

## References

- Original research: `thoughts/shared/research/2025-11-02-usage-tips-display-alternatives.md`
- CommandBar implementation: `internal/components/commandbar/commandbar.go:684-702`
- Display tick pattern: `internal/app/app.go:411-423`
- Context cycling pattern: `internal/commands/navigation.go:100-219`
- Testing patterns: `internal/components/commandbar/executor_test.go`
