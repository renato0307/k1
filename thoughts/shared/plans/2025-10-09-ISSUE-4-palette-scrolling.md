# Command Palette Scrolling Fix Implementation Plan

## Overview

Fix the command palette navigation bug where the selection indicator
disappears when navigating beyond the 8 visible items. The palette currently
shows only items 0-7 regardless of cursor position, causing items 8+ to be
unreachable even though the cursor can move there.

## Current State Analysis

The command palette at `internal/components/commandbar/palette.go` implements:
- **Fixed viewport**: Always renders items[0] through items[7] (line 156)
- **Cursor navigation**: `NavigateUp()/NavigateDown()` update `p.index`
  (lines 79-91)
- **No scroll offset**: No viewport sliding to follow cursor
- **MaxPaletteItems = 8**: Hardcoded visible item limit (line 5 in
  `constants.go`)

### The Bug
When commands > 8:
1. User presses `/` → sees items 0-7
2. User presses ↓ repeatedly → cursor moves: 0→1→2→...→7→8→9→...
3. At index=8+: Selection indicator disappears (item 8 not rendered)
4. Items 8+ are never visible or selectable

### Key Discoveries:
- `palette.go:136` - `maxItems = min(MaxPaletteItems, len(p.items))`
  determines render count
- `palette.go:156` - Loop always starts at `i=0`, not offset
- `palette.go:177,193` - Highlights item at `p.index` only if within
  rendered range
- Similar scrolling pattern exists in `fullscreen.go:33-106` using
  `scrollOffset`

## Desired End State

After this fix:
- Palette displays 8 items at a time (viewport window)
- When cursor moves beyond visible range, viewport scrolls to follow
- Selection indicator always visible on current item
- Top/bottom boundaries handled correctly
- Scroll position resets when filter changes

### Verification:
1. **Automated**: Run `make test` - new tests pass for scroll behavior
2. **Manual**:
   - Press `/` with 20+ commands visible
   - Navigate to item 15 using ↓ arrow
   - Verify selection indicator stays visible
   - Verify viewport shows items 8-15 when at position 15

## What We're NOT Doing

- NOT changing MaxPaletteItems constant (stays at 8)
- NOT implementing page up/down navigation (just arrow keys)
- NOT adding scroll position indicator (e.g., "1-8 of 20")
- NOT changing filter/search behavior
- NOT modifying command registry or executor

## Implementation Approach

Implement viewport scrolling similar to the `fullscreen.go` pattern:
1. Add `scrollOffset` field to track first visible item
2. Update `NavigateUp()/NavigateDown()` to adjust scrollOffset when
   cursor leaves viewport
3. Modify `View()` to render items[scrollOffset:scrollOffset+8]
4. Reset scrollOffset on filter changes

## Phase 1: Add Scroll Offset Field

### Overview
Add scrollOffset state to Palette struct and reset behavior.

### Changes Required:

#### 1. Palette Struct
**File**: `internal/components/commandbar/palette.go`
**Changes**: Add scrollOffset field to track viewport position

```go
// Palette manages command palette filtering, rendering, and navigation.
type Palette struct {
    items        []commands.Command
    index        int
    scrollOffset int  // First visible item index
    registry     *commands.Registry
    theme        *ui.Theme
    width        int
}
```

#### 2. Constructor
**File**: `internal/components/commandbar/palette.go`
**Changes**: Initialize scrollOffset to 0

```go
// NewPalette creates a new palette manager.
func NewPalette(registry *commands.Registry, theme *ui.Theme, width int) *Palette {
    return &Palette{
        items:        []commands.Command{},
        index:        0,
        scrollOffset: 0,
        registry:     registry,
        theme:        theme,
        width:        width,
    }
}
```

#### 3. Filter Method
**File**: `internal/components/commandbar/palette.go:41-77`
**Changes**: Reset scrollOffset when items change

```go
// At end of Filter() method, line 76:
p.items = items
p.index = 0
p.scrollOffset = 0  // Add this line
```

#### 4. Reset Method
**File**: `internal/components/commandbar/palette.go:112-115`
**Changes**: Reset scrollOffset

```go
// Reset clears the palette.
func (p *Palette) Reset() {
    p.items = []commands.Command{}
    p.index = 0
    p.scrollOffset = 0  // Add this line
}
```

### Success Criteria:

#### Automated Verification:
- [x] Existing tests pass: `make test`
- [x] No compilation errors: `go build ./...`

#### Manual Verification:
- [ ] No behavior change yet (scrollOffset not used)
- [ ] Filter still resets cursor to top

**Implementation Note**: After completing this phase and all automated
verification passes, this is a safe refactoring step - continue to Phase 2.

---

## Phase 2: Implement Scroll Logic in Navigation

### Overview
Update NavigateUp/NavigateDown to adjust scrollOffset when cursor moves
outside visible viewport.

### Changes Required:

#### 1. NavigateUp Method
**File**: `internal/components/commandbar/palette.go:79-84`
**Changes**: Add viewport scrolling logic

```go
// NavigateUp moves selection up in palette.
// Scrolls viewport if cursor moves above visible range.
func (p *Palette) NavigateUp() {
    if p.index > 0 {
        p.index--
        // If cursor moved above viewport, scroll up
        if p.index < p.scrollOffset {
            p.scrollOffset = p.index
        }
    }
}
```

#### 2. NavigateDown Method
**File**: `internal/components/commandbar/palette.go:86-91`
**Changes**: Add viewport scrolling logic

```go
// NavigateDown moves selection down in palette.
// Scrolls viewport if cursor moves below visible range.
func (p *Palette) NavigateDown() {
    if p.index < len(p.items)-1 {
        p.index++
        // Calculate bottom of viewport
        maxVisibleIndex := p.scrollOffset + MaxPaletteItems - 1
        // If cursor moved below viewport, scroll down
        if p.index > maxVisibleIndex {
            p.scrollOffset = p.index - MaxPaletteItems + 1
        }
    }
}
```

### Success Criteria:

#### Automated Verification:
- [x] All tests pass: `make test`
- [x] No compilation errors: `go build ./...`

#### Manual Verification:
- [ ] Cursor still moves correctly with arrow keys
- [ ] No visible change yet (View not using scrollOffset)

**Implementation Note**: After completing this phase and all automated
verification passes, continue to Phase 3.

---

## Phase 3: Update View to Render from Scroll Offset

### Overview
Modify View() method to render items starting from scrollOffset instead of
always from index 0.

### Changes Required:

#### 1. View Method - Visible Range Calculation
**File**: `internal/components/commandbar/palette.go:127-213`
**Changes**: Calculate visible slice based on scrollOffset

```go
// View renders the palette items with selection indicator.
func (p *Palette) View(prefix string) string {
    if p.IsEmpty() {
        return ""
    }

    sections := []string{}

    // Calculate visible range
    visibleCount := min(MaxPaletteItems, len(p.items)-p.scrollOffset)
    visibleEnd := p.scrollOffset + visibleCount

    // First pass: find longest description to align shortcuts
    longestMainText := 0
    for i := p.scrollOffset; i < visibleEnd; i++ {
        cmd := p.items[i]
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
    for i := p.scrollOffset; i < visibleEnd; i++ {
        cmd := p.items[i]
        mainText := prefix + cmd.Name
        if cmd.ArgPattern != "" {
            mainText += cmd.ArgPattern
        }
        mainText += " - " + cmd.Description

        var line string
        if cmd.Shortcut != "" {
            // Pad to shortcut column position (minimum 2 spaces)
            padding := max(shortcutColumn-len(mainText), 2)
            spacer := strings.Repeat(" ", padding)

            // Style shortcut with dimmed color
            shortcutStyle := lipgloss.NewStyle().
                Foreground(p.theme.Dimmed)
            styledShortcut := shortcutStyle.Render(cmd.Shortcut)

            itemContent := mainText + spacer + styledShortcut

            if i == p.index {
                selectedStyle := lipgloss.NewStyle().
                    Foreground(p.theme.Foreground).
                    Background(p.theme.Subtle).
                    Width(p.width).
                    Padding(0, 1).
                    Bold(true)
                line = selectedStyle.Render("▶ " + itemContent)
            } else {
                paletteStyle := lipgloss.NewStyle().
                    Width(p.width).
                    Padding(0, 1)
                line = paletteStyle.Render("  " + itemContent)
            }
        } else {
            // No shortcut, simple rendering
            if i == p.index {
                selectedStyle := lipgloss.NewStyle().
                    Foreground(p.theme.Foreground).
                    Background(p.theme.Subtle).
                    Width(p.width).
                    Padding(0, 1).
                    Bold(true)
                line = selectedStyle.Render("▶ " + mainText)
            } else {
                paletteStyle := lipgloss.NewStyle().
                    Width(p.width).
                    Padding(0, 1)
                line = paletteStyle.Render("  " + mainText)
            }
        }

        sections = append(sections, line)
    }

    return lipgloss.JoinVertical(lipgloss.Left, sections...)
}
```

**Key changes**:
- Line 136: `visibleCount = min(MaxPaletteItems, len(p.items)-p.scrollOffset)`
- Line 137: `visibleEnd = p.scrollOffset + visibleCount`
- Line 141, 156: Loop from `p.scrollOffset` to `visibleEnd` instead of
  `0` to `maxItems`
- Line 177, 193: Selection check `i == p.index` still works (comparing
  absolute indices)

### Success Criteria:

#### Automated Verification:
- [x] All tests pass: `make test`
- [x] No compilation errors: `go build ./...`
- [x] No linting errors: `make lint` (golangci-lint not installed, skipped)

#### Manual Verification:
- [ ] Press `/` with 20+ commands visible
- [ ] Navigate down to item 15 using ↓ arrow repeatedly
- [ ] Selection indicator stays visible throughout navigation
- [ ] Viewport scrolls to show items 8-15 when at position 15
- [ ] Navigate back up - viewport scrolls up correctly
- [ ] Filter changes reset viewport to top
- [ ] Enter key still executes selected command correctly

**Implementation Note**: After completing this phase and all automated
verification passes, pause here for manual confirmation from the human that
the manual testing was successful before proceeding to the next phase.

---

## Phase 4: Add Comprehensive Tests

### Overview
Add unit tests to verify scroll behavior, boundary conditions, and viewport
calculations.

### Changes Required:

#### 1. Test File - Scrolling Tests
**File**: `internal/components/commandbar/palette_test.go`
**Changes**: Add new test cases for scrolling behavior

```go
func TestPalette_ScrollingBehavior(t *testing.T) {
    repo := k8s.NewDummyRepository()
    registry := commands.NewRegistry(repo, nil)
    theme := ui.GetTheme("charm")

    p := NewPalette(registry, theme, 80)

    // Create test with many items (more than MaxPaletteItems)
    p.Filter("", CommandTypeAction, "pods")

    require.Greater(t, p.Size(), MaxPaletteItems,
        "Need more than 8 items for scroll test")

    // Initial state
    assert.Equal(t, 0, p.index, "Initial index should be 0")
    assert.Equal(t, 0, p.scrollOffset, "Initial scrollOffset should be 0")

    // Navigate down 7 times (to index 7, last visible in viewport 0-7)
    for i := 0; i < 7; i++ {
        p.NavigateDown()
    }
    assert.Equal(t, 7, p.index)
    assert.Equal(t, 0, p.scrollOffset, "Should not scroll yet")

    // Navigate down once more (to index 8, triggers scroll)
    p.NavigateDown()
    assert.Equal(t, 8, p.index)
    assert.Equal(t, 1, p.scrollOffset, "Should scroll down by 1")

    // Navigate down several more times
    for i := 0; i < 5; i++ {
        p.NavigateDown()
    }
    assert.Equal(t, 13, p.index)
    assert.Equal(t, 6, p.scrollOffset, "Should scroll to keep cursor visible")

    // Navigate back up
    p.NavigateUp()
    assert.Equal(t, 12, p.index)
    assert.Equal(t, 6, p.scrollOffset, "Should not scroll yet")

    // Navigate up to trigger scroll
    for i := 0; i < 7; i++ {
        p.NavigateUp()
    }
    assert.Equal(t, 5, p.index)
    assert.Equal(t, 5, p.scrollOffset, "Should scroll up")

    // Continue up to top
    for i := 0; i < 5; i++ {
        p.NavigateUp()
    }
    assert.Equal(t, 0, p.index)
    assert.Equal(t, 0, p.scrollOffset, "Should be at top")
}

func TestPalette_ScrollResetOnFilter(t *testing.T) {
    repo := k8s.NewDummyRepository()
    registry := commands.NewRegistry(repo, nil)
    theme := ui.GetTheme("charm")

    p := NewPalette(registry, theme, 80)
    p.Filter("", CommandTypeAction, "pods")

    // Navigate down to create scroll offset
    for i := 0; i < 10; i++ {
        p.NavigateDown()
    }

    assert.Equal(t, 10, p.index)
    assert.Greater(t, p.scrollOffset, 0, "Should have scrolled")

    // Filter again
    p.Filter("delete", CommandTypeAction, "pods")

    // Should reset
    assert.Equal(t, 0, p.index, "Index should reset on filter")
    assert.Equal(t, 0, p.scrollOffset, "ScrollOffset should reset on filter")
}

func TestPalette_BoundaryConditions(t *testing.T) {
    repo := k8s.NewDummyRepository()
    registry := commands.NewRegistry(repo, nil)
    theme := ui.GetTheme("charm")

    p := NewPalette(registry, theme, 80)

    // Test with fewer items than MaxPaletteItems
    p.items = []commands.Command{
        {Name: "cmd1", Description: "Command 1"},
        {Name: "cmd2", Description: "Command 2"},
        {Name: "cmd3", Description: "Command 3"},
    }
    p.index = 0
    p.scrollOffset = 0

    // Navigate down to bottom
    p.NavigateDown()
    p.NavigateDown()
    assert.Equal(t, 2, p.index)
    assert.Equal(t, 0, p.scrollOffset, "No scroll needed with few items")

    // Try to go past bottom
    p.NavigateDown()
    assert.Equal(t, 2, p.index, "Should stop at last item")
    assert.Equal(t, 0, p.scrollOffset)

    // Navigate up to top
    p.NavigateUp()
    p.NavigateUp()
    assert.Equal(t, 0, p.index)

    // Try to go past top
    p.NavigateUp()
    assert.Equal(t, 0, p.index, "Should stop at first item")
    assert.Equal(t, 0, p.scrollOffset)
}

func TestPalette_ViewRenderingWithScroll(t *testing.T) {
    repo := k8s.NewDummyRepository()
    registry := commands.NewRegistry(repo, nil)
    theme := ui.GetTheme("charm")

    p := NewPalette(registry, theme, 80)

    // Create 15 test items
    items := make([]commands.Command, 15)
    for i := 0; i < 15; i++ {
        items[i] = commands.Command{
            Name:        fmt.Sprintf("cmd%d", i),
            Description: fmt.Sprintf("Command %d", i),
        }
    }
    p.items = items
    p.index = 0
    p.scrollOffset = 0

    // Initial view should show cmd0-cmd7
    view := p.View("/")
    assert.Contains(t, view, "cmd0")
    assert.Contains(t, view, "cmd7")
    assert.NotContains(t, view, "cmd8")

    // Scroll to middle (offset=5, shows items 5-12)
    p.index = 8
    p.scrollOffset = 5
    view = p.View("/")
    assert.NotContains(t, view, "cmd4")
    assert.Contains(t, view, "cmd5")
    assert.Contains(t, view, "cmd12")
    assert.NotContains(t, view, "cmd13")
    assert.Contains(t, view, "▶", "Selected item should have indicator")
}
```

### Success Criteria:

#### Automated Verification:
- [x] All new tests pass: `make test`
- [x] Test coverage maintained or improved: `make test-coverage`
- [x] No compilation errors: `go build ./...`

#### Manual Verification:
- [ ] All test scenarios covered in automated tests

**Implementation Note**: After completing this phase and all automated
verification passes, the implementation is complete.

---

## Testing Strategy

### Unit Tests:
- Scroll offset updates correctly on navigation
- Viewport follows cursor when moving beyond visible range
- Boundary conditions (top/bottom of list)
- Reset behavior on filter changes
- View rendering with various scroll positions
- Small lists (< 8 items) work correctly

### Manual Testing Steps:
1. Run k1 with live cluster: `make run`
2. Press `/` to open action palette
3. Verify 8 items visible with selection indicator on first item
4. Press ↓ arrow 10+ times
5. Verify selection indicator stays visible throughout
6. Verify command names change as viewport scrolls
7. Press ↑ arrow 5 times
8. Verify viewport scrolls back up
9. Type "delete" to filter
10. Verify viewport resets to top
11. Press Enter on selected item
12. Verify command executes correctly

## Performance Considerations

- No performance impact: Scrolling is O(1) arithmetic
- Rendering already limited to 8 items max
- No additional allocations or loops
- View() complexity unchanged (same loop, different start index)

## Migration Notes

No migration needed - internal component change only. No API changes, no
configuration changes, no user-visible breaking changes beyond bug fix.

## References

- Original ticket: `thoughts/shared/tickets/issue_4.md`
- Similar pattern: `internal/components/fullscreen.go:33-106` (scroll
  offset implementation)
- Palette component: `internal/components/commandbar/palette.go`
- Command bar: `internal/components/commandbar/commandbar.go`
