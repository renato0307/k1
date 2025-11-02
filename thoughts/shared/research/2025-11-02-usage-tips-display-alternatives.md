---
date: 2025-11-02T00:00:00-00:00
researcher: @claude
git_commit: 30f38dd335431729d2d4272927762111b47e529c
branch: fix/bug-squash-3
repository: k1
topic: "UX: Usage tips display alternatives"
tags: [research, ux, tips, hints, onboarding, command-bar, status-bar]
status: complete
last_updated: 2025-11-02
last_updated_by: @claude
last_updated_note: "Added decision: start with Alternative 1, evolve to Alternative 5"
---

# Research: UX - Usage Tips Display Alternatives

**Date**: 2025-11-02
**Researcher**: @claude
**Git Commit**: 30f38dd335431729d2d4272927762111b47e529c
**Branch**: fix/bug-squash-3
**Repository**: k1

## Research Question

How can we show tips about k1 usage (e.g., about the :output command)?
Do we have UI space for that? What alternatives exist?

## Summary

The k1 TUI has two potential locations for displaying usage tips:
1. **Hints line at the bottom** (currently shows static navigation help)
2. **Status bar above command bar** (currently shows ephemeral messages)

No empty/unused lines exist in the layout - all vertical space is
accounted for. The best approach is to **rotate tips in the hints line**
as it's always visible when the command bar is hidden and uses existing
UI real estate without adding complexity.

## Current UI Layout Analysis

### Screenshot Analysis

From the provided screenshots:

**Screenshot 1**: Hints line visible at bottom
- Shows: `[type to filter  : resources  / commands]`
- Located below the pod list table
- Visible when command bar is hidden
- 3 lines total: separator + text + separator

**Screenshot 2**: Command palette open
- Hints line is hidden
- Command palette shows available commands
- No tips visible during command interaction

### Vertical Space Allocation

The layout uses every line of terminal height with no unused space:

```
┌──────────────────────────────────────────────────────────┐
│ k1 💨 • current context: rundev-dev-us-east-1-01         │ 1 line (title)
│                                                          │ 1 line (empty)
│ Pods • 961 items • refreshing in 7s                     │ 1 line (header)
│                                                          │ 1 line (empty)
├──────────────────────────────────────────────────────────┤
│ Namespace      Name                    Ready  Status    │
│ ...                                                      │ N lines (body)
│ [Pod list content - dynamic height]                     │
│ ...                                                      │
├──────────────────────────────────────────────────────────┤
│ [status bar - 1 line if message present, 0 otherwise]   │ 0-1 line
├──────────────────────────────────────────────────────────┤
│ [command bar - 0 lines when hidden, 3-8 when active]    │ 0-8 lines
├──────────────────────────────────────────────────────────┤
│ ──────────────────────────────────────────────────────   │
│ [type to filter  : resources  / commands]               │ 3 lines (hints)
│ ──────────────────────────────────────────────────────   │
└──────────────────────────────────────────────────────────┘
```

**Total reserved lines**: 5 fixed + dynamic command bar height

### Code References

#### Hints Line Implementation
- `internal/components/commandbar/commandbar.go:684-702` - `ViewHints()`
- Only shown when `state == StateHidden`
- Renders: separator + hint text + separator (3 lines)
- Current text: `"[type to filter  : resources  / commands]"`

#### Layout Height Calculation
- `internal/components/layout.go:50-59` -
  `CalculateBodyHeightWithCommandBar()`
- Reserved lines: `5 + commandBarHeight`
- Breakdown:
  - Title line: 1
  - Empty after title: 1
  - Header: 1
  - Empty after header: 1
  - Status bar: 1 (always reserved)
  - Command bar + hints: dynamic

#### Status Bar (UserMessage) Component
- `internal/components/usermessage.go` - Single message display
- Always occupies 1 line (empty or with message)
- Positioned above command bar
- 4 message types: info, success, error, loading
- Auto-clear: only success messages (5 seconds)
- Currently used for: operation results, errors, loading states

## Alternative Approaches

### Alternative 1: Rotating Tips in Hints Line ⭐ RECOMMENDED

**Description**: Replace static hints with rotating usage tips when
command bar is hidden.

**Location**: Bottom of screen (existing hints line space)

**Implementation**:
```go
// In commandbar.go or new tips.go component
var usageTips = []string{
    "[tip: try :output to see command history]",
    "[tip: press '/' for command palette]",
    "[tip: press ':' for navigation]",
    "[tip: type to filter list]",
    "[tip: press 'e' to edit, 'd' to delete]",
}

func (cb *CommandBar) ViewHints() string {
    if cb.state == StateHidden {
        currentTip := usageTips[cb.tipIndex]
        return renderHintLine(currentTip)
    }
    return ""
}
```

**Rotation Logic**:
- Timer-based: Rotate every 10-15 seconds
- Action-triggered: Show relevant tip after certain actions
- Random: Pick random tip on each rotation
- Sequential: Cycle through all tips in order

**Pros**:
- ✅ Uses existing UI space (no new elements)
- ✅ Always visible when command bar is hidden
- ✅ Minimal implementation complexity
- ✅ Doesn't interfere with other UI elements
- ✅ Can include the :output command tip requested

**Cons**:
- ⚠️ Hides static navigation hints (could alternate with tips)
- ⚠️ Requires rotation timer/logic
- ⚠️ Tips disappear when command bar is active

**Effort**: Low (2-3 hours)

---

### Alternative 2: Context-Sensitive Tips in Status Bar

**Description**: Show relevant tips in the status bar (UserMessage
component) based on user context or actions.

**Location**: Above command bar (existing status bar)

**Implementation**:
```go
// Show tip when user lands on specific screen for first time
case types.ScreenSwitchMsg:
    if msg.ScreenID == "pods" && !m.hasSeenPodsTip {
        m.hasSeenPodsTip = true
        return m, messages.InfoCmd(
            "Tip: Press Enter on a pod to see related commands")
    }

// Show tip after certain actions
case types.RefreshCompleteMsg:
    if m.refreshCount == 1 {  // First refresh
        return m, messages.InfoCmd(
            "Tip: Use :output to see command history")
    }
```

**Pros**:
- ✅ Context-aware and relevant to user's current task
- ✅ Uses existing UserMessage component (no new UI)
- ✅ Info messages persist until user acts (not time-limited)
- ✅ Can show tips when most relevant

**Cons**:
- ⚠️ Competes with operational messages (errors, success)
- ⚠️ Requires state tracking (which tips have been shown)
- ⚠️ Less discoverable (only shown in specific contexts)
- ⚠️ Can feel intrusive if shown too frequently

**Effort**: Medium (4-6 hours including state management)

---

### Alternative 3: Startup Modal/Splash Screen

**Description**: Show tips modal on first run or with quick tips overlay
on startup (inspired by k9s splash screen pattern).

**Location**: Full-screen overlay on startup

**Implementation**:
```go
// In app.go Init()
func (m Model) Init() tea.Cmd {
    if !m.hasSeenStartupTips {
        return showStartupTipsModal()
    }
    return m.currentScreen.Init()
}

// Modal with tips
type TipsModal struct {
    tips []string
    currentIndex int
}
```

**Pros**:
- ✅ Comprehensive onboarding experience
- ✅ Doesn't interfere with normal operation
- ✅ Can include multiple tips with pagination
- ✅ Users can dismiss when ready
- ✅ Can be re-triggered with a command (e.g., :tips)

**Cons**:
- ⚠️ Higher implementation complexity
- ⚠️ Requires modal/overlay component
- ⚠️ Only shown once (unless user explicitly requests)
- ⚠️ Adds to startup time/friction

**Effort**: High (8-12 hours for modal component + tips content)

**Reference**: `thoughts/shared/research/2025-10-31-startup-
performance-vs-k9s.md` documents k9s splash screen patterns

---

### Alternative 4: Dedicated :help or :tips Screen

**Description**: Create a navigable screen showing all tips and usage
guidance.

**Location**: New screen accessible via `:help` or `:tips` command

**Implementation**:
```go
// New screen in internal/screens/help.go
type HelpScreen struct {
    sections []HelpSection
    table    table.Model
}

type HelpSection struct {
    Title string
    Tips  []string
}

var helpContent = []HelpSection{
    {
        Title: "Navigation",
        Tips: []string{
            "Press ':' for navigation palette",
            "Press '/' for command palette",
            "Use :output to see command history",
        },
    },
    // ... more sections
}
```

**Pros**:
- ✅ Comprehensive help system
- ✅ Organized by category
- ✅ Searchable/filterable
- ✅ Available on-demand
- ✅ Can include detailed explanations

**Cons**:
- ⚠️ High implementation effort
- ⚠️ Requires navigation away from current screen
- ⚠️ Less discoverable (users need to know to look for it)
- ⚠️ Requires content maintenance

**Effort**: High (10-15 hours including screen, content, navigation)

---

### Alternative 5: Smart Hints (Contextual + Static)

**Description**: Alternate between static navigation hints and
contextual tips based on user state.

**Location**: Bottom hints line (existing space)

**Implementation**:
```go
func (cb *CommandBar) ViewHints() string {
    if cb.state == StateHidden {
        if shouldShowTip() {
            return renderTip(getContextualTip())
        }
        return renderStaticHints()
    }
    return ""
}

func shouldShowTip() bool {
    // Show tip every 30s, or after specific actions
    return time.Since(lastTipShown) > 30*time.Second
}

func getContextualTip() string {
    // Return tip based on:
    // - Current screen
    // - Recent actions
    // - User patterns
}
```

**Pros**:
- ✅ Preserves static hints most of the time
- ✅ Shows tips at opportune moments
- ✅ Uses existing UI space
- ✅ Can be very relevant to user context

**Cons**:
- ⚠️ Complex logic to determine when to show tips
- ⚠️ May feel unpredictable to users
- ⚠️ Requires careful tuning to avoid annoyance

**Effort**: Medium-High (6-8 hours for smart logic + tips)

---

### Alternative 6: Header Integration

**Description**: Add tips to the header area, possibly rotating or as
a subtitle.

**Location**: Header line (currently shows screen title + metadata)

**Current header format**:
```
Pods • 961 items • refreshing in 7s
```

**With tips**:
```
Pods • 961 items • refreshing in 7s • tip: try :output
```

**Pros**:
- ✅ Always visible
- ✅ Uses existing header space
- ✅ Minimal implementation changes

**Cons**:
- ⚠️ Limited space (conflicts with existing header content)
- ⚠️ May clutter header
- ⚠️ Truncation issues on smaller terminals
- ⚠️ Less visually distinct than dedicated tips area

**Effort**: Low-Medium (3-4 hours)

---

### Alternative 7: Dedicated Tips Bar Above Hints

**Description**: Add a new permanent UI element for tips above the
hints line.

**Location**: New line above command bar/hints

**Layout**:
```
┌────────────────────────────────────────┐
│ [Body content]                         │
├────────────────────────────────────────┤
│ [Status bar - ephemeral messages]     │
├────────────────────────────────────────┤
│ 💡 Tip: Use :output to see command... │ ← NEW
├────────────────────────────────────────┤
│ [Command bar]                          │
├────────────────────────────────────────┤
│ [Hints - static navigation help]      │
└────────────────────────────────────────┘
```

**Pros**:
- ✅ Dedicated space for tips
- ✅ Always visible
- ✅ Doesn't conflict with other UI elements
- ✅ Can be styled distinctly

**Cons**:
- ⚠️ Reduces body height by 1 line permanently
- ⚠️ Adds UI complexity
- ⚠️ May feel cluttered
- ⚠️ Not space-efficient

**Effort**: Medium (5-6 hours for new component + integration)

**Note**: This adds permanent overhead - probably not worth the trade-off.

---

## Recommendation & Decision

**Decision**: **Start with Alternative 1, then evolve to Alternative 5**

This approach provides a clear evolution path:
1. **V1**: Implement Alternative 1 (basic rotating tips)
2. **V2**: Evolve to Alternative 5 (smart contextual hints)

### Why This Path?

**Alternative 1 first** provides:
- Quick wins with minimal complexity (2-3 hours)
- Immediate value to users with rotating tips
- Foundation for more sophisticated logic
- Low risk, easy to iterate

**Alternative 5 later** adds:
- Intelligence about when to show tips vs static hints
- Context awareness (current screen, user actions)
- Better user experience (right tip at right time)
- Preserves static hints when tips aren't needed

### Evolution Path

**Version 1: Basic Rotating Tips** (Alternative 1)
- **Effort**: 2-3 hours
- **Features**:
  - Create tips slice with 8-12 useful tips
  - Simple rotation timer (20-second interval)
  - Sequential cycling through all tips
  - Update `ViewHints()` to show current tip
- **Outcome**: Users see helpful tips rotating in hints line

**Version 2: Smart Contextual Hints** (Alternative 5)
- **Effort**: 4-6 hours (builds on V1)
- **Features**:
  - Add context awareness (screen ID, recent actions)
  - Smart decision logic: when to show tips vs static hints
  - Prioritize contextual tips over generic tips
  - Track which tips have been shown to avoid repetition
  - Respect user activity (don't rotate during active use)
- **Outcome**: Tips appear at opportune moments, static hints preserved
  when appropriate

**Version 3: Configuration & Polish** (Optional)
- **Effort**: 1-2 hours
- **Features**:
  - Add config option to disable tips rotation
  - Add config for rotation interval
  - Persist seen tips state across sessions
  - Add :tips command to manually trigger tips
- **Outcome**: Users can customize tip behavior

### Implementation Strategy

**V1 Implementation** (Alternative 1):
```go
// In commandbar.go or new hints.go file
type CommandBar struct {
    // ... existing fields
    tipIndex    int
    lastTipTime time.Time
}

var usageTips = []string{
    "[type to filter  : resources  / commands]", // Original static hint
    "[tip: press '/' for command palette]",
    "[tip: press ':' to navigate screens]",
    "[tip: press Enter on resources for actions]",
    "[tip: press 'e' to edit, 'd' to delete]",
    "[tip: press ctrl+y for YAML, ctrl+d for describe]",
    "[tip: press 'q' to quit, ESC to go back]",
    "[tip: press ctrl+n/p to switch contexts]",
}

func (cb *CommandBar) ViewHints() string {
    if cb.state == StateHidden {
        // Rotate every 20 seconds
        if time.Since(cb.lastTipTime) > 20*time.Second {
            cb.tipIndex = (cb.tipIndex + 1) % len(usageTips)
            cb.lastTipTime = time.Now()
        }

        currentTip := usageTips[cb.tipIndex]
        return cb.renderHintLine(currentTip)
    }
    return ""
}
```

**V2 Enhancement** (Alternative 5):
```go
// Add context awareness
func (cb *CommandBar) ViewHints() string {
    if cb.state == StateHidden {
        // Decide: show tip or static hint?
        if cb.shouldShowTip() {
            tip := cb.getContextualTip()
            return cb.renderHintLine(tip)
        }
        return cb.renderHintLine(cb.staticHint)
    }
    return ""
}

func (cb *CommandBar) shouldShowTip() bool {
    // Show tips 30% of the time, or after specific triggers
    elapsed := time.Since(cb.lastTipTime)

    // Always show static hints for first 5 seconds on new screen
    if time.Since(cb.lastScreenSwitch) < 5*time.Second {
        return false
    }

    // Rotate to tip every 30 seconds
    if elapsed > 30*time.Second {
        return true
    }

    // Show contextual tip after specific actions
    if cb.hasRecentAction() {
        return true
    }

    return false
}

func (cb *CommandBar) getContextualTip() string {
    // Return tip based on context
    switch cb.screenID {
    case "pods":
        return "[tip: press Enter on a pod to see available actions]"
    case "deployments":
        return "[tip: press '/' then 'scale' to scale deployments]"
    // ... more contextual tips
    }

    // Fallback to generic rotating tip
    return usageTips[cb.tipIndex]
}
```

### Total Effort

- **V1** (Alternative 1): 2-3 hours
- **V2** (Alternative 5): 4-6 hours (incremental on V1)
- **V3** (Optional polish): 1-2 hours

**Total**: 7-11 hours for complete evolution

## About the :output Command

**Current status**: The `:output` command does NOT exist in the codebase.

**Reference**: `thoughts/shared/research/2025-11-02-command-output-
history-design.md` contains a complete design proposal for the :output
command, but it has not been implemented.

**Design summary**:
- Dedicated screen showing chronological history of ALL command outputs
- In-memory ring buffer for command history
- Filtering, search, and export capabilities
- `:output` navigation command to access the screen

**Recommendation**: Implement :output command first, then add tips about
it. This ensures tips are referencing real functionality.

## Related Research

- `thoughts/shared/research/2025-10-31-startup-performance-vs-k9s.md` -
  k9s splash screen patterns, lazy loading, progressive UI
- `thoughts/shared/research/2025-11-02-command-output-history-design.md`
  - :output command design (not implemented)
- `thoughts/shared/research/2025-10-09-ai-command-implementation.md` -
  AI command UX patterns, loading indicators, confirmation patterns

## Code References

### Current Implementation
- `internal/components/commandbar/commandbar.go:684-702` - ViewHints()
  method
- `internal/components/usermessage.go:94-108` - UserMessage View()
  method
- `internal/components/layout.go:50-59` - Height calculation logic
- `internal/app/app.go:163-176` - Window resize handling

### Suggested Implementation Locations
- New file: `internal/components/tips.go` - Tips manager component
- Or extend: `internal/components/commandbar/hints.go` - Hints rotation
  logic
- Config: Add tips settings to `~/.config/k1/config.yaml`

## Open Questions

1. **Should tips be dismissible?** (e.g., "don't show this tip again")
2. **Should there be a tip history/log?** (track which tips shown)
3. **Should tips adapt to user skill level?** (beginner vs advanced)
4. **What's the right rotation interval?** (10s, 20s, 30s?)
5. **Should we implement :output before adding tips about it?** (YES)

## Next Steps

**Decided path**: Implement Alternative 1, then evolve to Alternative 5

### V1: Basic Rotating Tips (Alternative 1)
1. ✅ Research complete (this document)
2. Create tips content (8-12 useful tips about k1 usage)
3. Add `tipIndex` and `lastTipTime` fields to CommandBar struct
4. Implement rotation logic in `ViewHints()` method
5. Test rotation timing and readability
6. Gather initial user feedback

### V2: Smart Contextual Hints (Alternative 5)
1. Add context awareness to CommandBar (track screenID, recent actions)
2. Implement `shouldShowTip()` decision logic
3. Implement `getContextualTip()` with screen-specific tips
4. Create contextual tips for each major screen (pods, deployments, etc.)
5. Test smart rotation behavior
6. Fine-tune timing and triggers based on user feedback

### V3: Configuration & Polish (Optional)
1. Add config file support for tips settings
2. Add disable/enable toggle for tips
3. Add rotation interval configuration
4. Persist seen tips state across sessions
5. Add :tips command to manually show tips screen
