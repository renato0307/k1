---
date: 2025-10-26T09:49:12+00:00
researcher: Claude
git_commit: 4aeea6f036278bccaa977e04ba130d17183315bd
branch: feat/kubernetes-context-management
repository: k1
topic: "Column display and prioritization in smaller terminal windows"
tags: [research, codebase, ui, table, columns, responsive, k9s]
status: complete
last_updated: 2025-10-26
last_updated_by: Claude
last_updated_note: "Added solution analysis comparing three approaches"
---

# Research: Column Display and Prioritization in Smaller Windows

**Date**: 2025-10-26T09:49:12+00:00
**Researcher**: Claude
**Git Commit**: 4aeea6f036278bccaa977e04ba130d17183315bd
**Branch**: feat/kubernetes-context-management
**Repository**: k1

## Research Question

How does k1 handle table column display in smaller terminal windows, and
why are important columns (like Name) truncated while less important ones
(like Namespace) remain fully visible? What patterns can be adopted from
k9s?

## Summary

k1's current implementation uses a simple **dynamic width allocation**
system where only one column per screen (typically "Name") has `Width: 0`
and fills remaining space. All other columns have fixed widths. When the
terminal is too narrow, the dynamic column gets squeezed (minimum 20
characters) while fixed columns remain unchanged. This causes important
columns to truncate while less-critical ones stay visible.

**Root cause**: No column priority system exists. All columns are always
visible regardless of available space.

**k9s solution**: Implements a **Wide column attribute system** where
columns are marked as `Wide: true` (only visible in wide mode) or always
visible (core columns). Users can toggle between modes with a keybinding
(e.g., ctrl+w) and customize via YAML configuration.

## Detailed Findings

### Current k1 Implementation

#### Column Configuration System

**Location**: `internal/screens/config.go:21-27`

k1 uses declarative column configuration via `ColumnConfig` struct:

```go
type ColumnConfig struct {
    Field  string                   // Field name in resource struct
    Title  string                   // Column display title
    Width  int                      // 0 = dynamic, >0 = fixed
    Format func(interface{}) string // Optional custom formatter
}
```

**Width semantics**:
- `Width: 0` → Dynamic (fills remaining space)
- `Width: N` (N > 0) → Fixed width in characters

#### Pods Screen Example

**Location**: `internal/screens/screens.go:16-24`

8 columns total, only Name is dynamic:

```go
Columns: []ColumnConfig{
    {Field: "Namespace", Title: "Namespace", Width: 40},
    {Field: "Name", Title: "Name", Width: 0},  // DYNAMIC
    {Field: "Ready", Title: "Ready", Width: 8},
    {Field: "Status", Title: "Status", Width: 15},
    {Field: "Restarts", Title: "Restarts", Width: 10},
    {Field: "Age", Title: "Age", Width: 10, Format: FormatDuration},
    {Field: "Node", Title: "Node", Width: 30},
    {Field: "IP", Title: "IP", Width: 16},
}
```

**Fixed columns total**: 129 characters + 16 padding = **145 characters**

On a 150-character terminal:
- Fixed columns: 145 characters (unchanged)
- Name column: 5 characters (or minimum 20, making table 165 wide)
- **Result**: Table exceeds terminal width, Name column truncated

#### Dynamic Width Calculation

**Location**: `internal/screens/config.go:247-286`

```go
func (s *ConfigScreen) SetSize(width, height int) {
    fixedTotal := 0
    dynamicCount := 0

    // First pass: sum fixed widths
    for _, col := range s.config.Columns {
        if col.Width > 0 {
            fixedTotal += col.Width
        } else {
            dynamicCount++
        }
    }

    // Calculate remaining space
    padding := len(s.config.Columns) * 2
    dynamicWidth := (width - fixedTotal - padding) / dynamicCount
    if dynamicWidth < 20 {
        dynamicWidth = 20  // Minimum protection
    }

    // Second pass: rebuild columns with calculated widths
    columns := make([]table.Column, len(s.config.Columns))
    for i, col := range s.config.Columns {
        w := col.Width
        if w == 0 {
            w = dynamicWidth
        }
        columns[i] = table.Column{Title: col.Title, Width: w}
    }

    s.table.SetColumns(columns)
}
```

**Formula**:
`dynamicWidth = (terminalWidth - fixedTotal - padding) / dynamicCount`

**Problem**: When terminal is narrower than `fixedTotal + padding + 20`,
the dynamic column gets minimum 20 chars, causing table to exceed terminal
width. The bubbles table library then clips/truncates overflow (opaque to
k1).

#### Absence of Advanced Features

**No column hiding**: All columns render regardless of terminal width.

**No priority system**: Cannot mark columns as "core" vs "detailed".

**No responsive breakpoints**: No logic to hide columns at narrow widths.

**No user toggle**: No way to switch between "compact" and "full" views.

**No per-column visibility**: Cannot configure which columns to show/hide.

### k9s Implementation (Comparison)

#### Wide Column Attribute System

**Location**: `.tmp/k9s/internal/model1/header.go:18-29`

k9s defines column attributes including visibility control:

```go
type Attrs struct {
    Align     int
    Decorator DecoratorFunc
    Wide      bool    // Column only visible in wide mode
    Show      bool    // Override: always show (even if Wide)
    MX        bool    // Metrics column (conditional)
    Time      bool    // Time/age column
    Capacity  bool    // Capacity/resource column
    VS        bool    // Vulnerability scan column
    Hide      bool    // Always hidden
}

type HeaderColumn struct {
    Attrs
    Name string
}
```

**Key insight**: Each column has visibility rules encoded in attributes.

#### Column Exclusion Logic

**Location**: `.tmp/k9s/internal/ui/table.go:323-328`

Centralized decision for column visibility:

```go
func (t *Table) shouldExcludeColumn(h model1.HeaderColumn) bool {
    return (h.Hide || (!t.wide && h.Wide)) ||
        (h.Name == "NAMESPACE" && !t.GetModel().ClusterWide()) ||
        (h.MX && !t.hasMetrics) ||
        (h.VS && vul.ImgScanner == nil)
}
```

**Exclusion conditions**:
1. `h.Hide`: Always excluded
2. `!t.wide && h.Wide`: Excluded unless in wide mode
3. Context-specific: NAMESPACE only in cluster-wide views
4. Feature-dependent: Metrics/vulnerability require feature enabled

**Applied during rendering** (lines 337, 383):
```go
// Header rendering
for _, h := range cdata.Header() {
    if t.shouldExcludeColumn(h) {
        continue  // Skip this column
    }
    t.AddHeaderCell(col, h)
    col++
}

// Row rendering - skip same columns
for c, field := range re.Row.Fields {
    if t.shouldExcludeColumn(h[c]) {
        continue
    }
    // ... render cell
}
```

#### Node Screen Example (Real-world)

**Location**: `.tmp/k9s/internal/render/node.go:34-57`

10+ columns with priority tiers:

```go
var defaultNodeHeader = model1.Header{
    // CORE (always visible):
    model1.HeaderColumn{Name: "NAME"},
    model1.HeaderColumn{Name: "STATUS"},
    model1.HeaderColumn{Name: "ROLE"},
    model1.HeaderColumn{Name: "TAINTS"},
    model1.HeaderColumn{Name: "VERSION"},
    model1.HeaderColumn{Name: "PODS"},

    // DETAILED (wide mode only):
    model1.HeaderColumn{Name: "ARCH", Attrs: model1.Attrs{Wide: true}},
    model1.HeaderColumn{Name: "OS-IMAGE",
        Attrs: model1.Attrs{Wide: true}},
    model1.HeaderColumn{Name: "KERNEL",
        Attrs: model1.Attrs{Wide: true}},
    model1.HeaderColumn{Name: "INTERNAL-IP",
        Attrs: model1.Attrs{Wide: true}},
    model1.HeaderColumn{Name: "EXTERNAL-IP",
        Attrs: model1.Attrs{Wide: true}},
    model1.HeaderColumn{Name: "LABELS",
        Attrs: model1.Attrs{Wide: true}},

    // OPTIONAL (wide + metrics enabled):
    model1.HeaderColumn{Name: "CPU",
        Attrs: model1.Attrs{Align: tview.AlignRight, MX: true}},
    model1.HeaderColumn{Name: "%CPU",
        Attrs: model1.Attrs{Align: tview.AlignRight, MX: true}},

    model1.HeaderColumn{Name: "AGE",
        Attrs: model1.Attrs{Time: true}},
}
```

**Priority strategy**:
1. **Critical** (always): NAME, STATUS, ROLE, VERSION, PODS, AGE
2. **Detailed** (wide mode): ARCH, OS-IMAGE, IPs, LABELS
3. **Optional** (wide + feature): Metrics (CPU, MEM)

#### Toggle Wide Mode

**Location**: `.tmp/k9s/internal/ui/table.go:223-227`

Simple mode switching:

```go
func (t *Table) ToggleWide() {
    t.wide = !t.wide
    t.Refresh()
}
```

**User experience**:
- Bound to keybinding (typically ctrl+w)
- Toggle between compact and full views
- Full table refresh applies new visibility rules

#### User Configuration (Advanced)

**Location**: `.tmp/k9s/internal/config/views.go:34-38`

YAML-based column customization:

```yaml
# $XDG_CONFIG_HOME/k9s/views.yaml
views:
  v1/pods:
    columns:
      - AGE
      - NAMESPACE|WR        # Wide + Right aligned
      - NAME
      - IP
      - MEM/RL|S            # Show (override Wide default)
      - '%MEM/R'
    sortColumn: AGE:desc
```

**Column flags** (from `.tmp/k9s/internal/render/cust_col.go:21-29`):
- `W` - Wide (only visible in wide mode)
- `S` - Show (override Wide, always visible)
- `R` - Right align
- `L` - Left align
- `T` - Time/age column
- `N` - Number (capacity, right aligned)
- `H` - Hide (never visible)

### k9s vs k1 Comparison

| Feature | k1 | k9s |
|---------|----|----|
| Column priority | ❌ None | ✅ Wide attribute |
| Mode toggle | ❌ None | ✅ ToggleWide() |
| Column hiding | ❌ Never | ✅ Context-aware |
| User config | ❌ None | ✅ YAML per-resource |
| Min width | ✅ 20 chars | ✅ Content-driven |
| Dynamic width | ✅ Width: 0 | ✅ Content-driven |
| Fixed width | ✅ Width: N | ❌ All dynamic |

**Key philosophical difference**:
- **k1**: Fixed widths for most columns, one dynamic
- **k9s**: All columns content-driven, with visibility tiers

## Code References

**k1 implementation**:
- `internal/screens/config.go:21-27` - ColumnConfig struct
- `internal/screens/config.go:247-286` - SetSize() dynamic width calc
- `internal/screens/screens.go:16-24` - Pods screen config
- `internal/screens/screens.go:300-310` - Nodes screen config (10 cols)
- `internal/screens/config.go:518-547` - Cell rendering
- `internal/ui/theme.go:45-52` - Table style conversion

**k9s implementation**:
- `.tmp/k9s/internal/model1/header.go:18-29` - Attrs struct
- `.tmp/k9s/internal/ui/table.go:323-328` - shouldExcludeColumn()
- `.tmp/k9s/internal/ui/table.go:223-227` - ToggleWide()
- `.tmp/k9s/internal/render/node.go:34-57` - Node screen example
- `.tmp/k9s/internal/ui/padding.go:14-39` - Content-driven width calc
- `.tmp/k9s/internal/config/views.go:34-38` - YAML configuration

## Architecture Insights

### k1's Current Model

**Pros**:
- Simple and predictable
- Minimum width protection (20 chars)
- Easy to configure per-screen
- No mode switching complexity

**Cons**:
- Cannot adapt to narrow terminals
- Important columns (Name) get squeezed
- All columns always visible (no hiding)
- No user customization

**Design assumption**: Terminal is wide enough for fixed columns + 20
chars dynamic. When this fails, user experience degrades.

### k9s's Model

**Pros**:
- Adapts to user preference (wide mode toggle)
- Smart defaults (core vs detailed columns)
- User can override via YAML
- Context-aware (namespace, metrics, features)

**Cons**:
- More complex implementation
- Requires user to learn toggle keybinding
- Configuration can be overwhelming

**Design philosophy**: User controls information density. System provides
smart defaults but respects user choice.

### Patterns for k1 to Adopt

#### 1. Wide Attribute (Minimal Adoption)

Add `Wide bool` to `ColumnConfig`:

```go
type ColumnConfig struct {
    Field  string
    Title  string
    Width  int
    Format func(interface{}) string
    Wide   bool  // NEW: Only visible in wide mode
}
```

Add toggle to `ConfigScreen`:

```go
func (s *ConfigScreen) ToggleWide() {
    s.wide = !s.wide
    s.updateTable()  // Re-filter visible columns
}
```

**Example Pods config**:
```go
Columns: []ColumnConfig{
    {Field: "Namespace", Title: "Namespace", Width: 40},
    {Field: "Name", Title: "Name", Width: 0},
    {Field: "Ready", Title: "Ready", Width: 8},
    {Field: "Status", Title: "Status", Width: 15},
    {Field: "Restarts", Title: "Restarts", Width: 10},
    {Field: "Age", Title: "Age", Width: 10, Format: FormatDuration},
    {Field: "Node", Title: "Node", Width: 30, Wide: true},  // NEW
    {Field: "IP", Title: "IP", Width: 16, Wide: true},      // NEW
}
```

**User experience**: In normal mode, only Namespace, Name, Ready, Status,
Restarts, Age visible (6 columns). Press ctrl+w → Node and IP appear (8
columns total).

#### 2. shouldExcludeColumn Pattern

Centralize visibility logic:

```go
func (s *ConfigScreen) shouldExcludeColumn(col ColumnConfig) bool {
    return col.Wide && !s.wide
    // Future: add more conditions (e.g., namespace, features)
}

func (s *ConfigScreen) buildVisibleColumns() []table.Column {
    visible := []table.Column{}
    for _, col := range s.config.Columns {
        if s.shouldExcludeColumn(col) {
            continue
        }
        visible = append(visible, table.Column{
            Title: col.Title,
            Width: col.Width,
        })
    }
    return visible
}
```

**Benefits**:
- Single source of truth for visibility
- Easy to extend with new conditions
- Applied consistently to headers and rows

#### 3. Screen-Specific Column Priorities

Update screen configs to mark wide columns:

**Pods** (keep it simple):
- Core: Namespace, Name, Ready, Status, Restarts, Age
- Wide: Node, IP

**Nodes** (aggressive filtering):
- Core: Name, Status, Roles, Version, Age
- Wide: Hostname, InstanceType, Zone, NodePool, OSImage

**Deployments**:
- Core: Namespace, Name, Ready, Up-to-date, Available, Age
- Wide: (none, all core)

#### 4. Keybinding for Toggle

Add to `ConfigScreen.DefaultUpdate()`:

```go
case tea.KeyMsg:
    switch msg.String() {
    case "ctrl+w":
        s.ToggleWide()
        return s, nil
    // ... existing cases
    }
}
```

Update help text to show "ctrl+w: toggle wide view".

## Open Questions

1. **Should k1 adopt content-driven width calculation like k9s?**
   - Pro: More flexible, better space usage
   - Con: More complex, harder to predict layout
   - Current fixed widths are simpler but less adaptive

2. **Should wide mode be persistent across sessions?**
   - k9s saves mode in session state
   - k1 could add to config file (~/.config/k1/config.yaml)

3. **Which columns should be marked Wide for each screen?**
   - Needs UX decision: what's "essential" vs "nice to have"?
   - Pods: Node and IP seem less critical than Status/Ready
   - Nodes: Many columns are detailed metadata (OS, Instance, Zone)

4. **Should k1 support user configuration like k9s?**
   - YAML-based column customization is powerful
   - But adds complexity and config file management
   - Could be phase 2 after basic wide mode works

5. **Should minimum width logic change?**
   - Current: 20 chars minimum for dynamic columns
   - k9s: Content-driven, no hard minimum
   - Could dynamic columns have no minimum when table > terminal width?

6. **How to handle filter mode with hidden columns?**
   - If Node column is hidden, should fuzzy search still search it?
   - k9s searches all fields regardless of visibility
   - Seems reasonable: filtering is about data, not display

## Related Research

- `design/PROCESS-IMPROVEMENTS.md` - Quality guidelines (file size limits)
- `CLAUDE.md` - Development patterns and conventions
- `thoughts/shared/plans/` - Implementation plans directory

## Historical Context (from thoughts/)

No previous research found on column sizing or responsive table layouts
in k1's thoughts/ directory. This appears to be the first investigation
into this UX issue.

## Follow-up Research [2025-10-26T09:57:57+00:00]

### Clarification on Problem Statement

The original analysis focused on k9s's **Wide mode toggle** (manual user
control), but this doesn't solve the core problem: **automatic
prioritization** where important columns stay visible while less-important
ones adapt/hide in narrow terminals.

The user's actual requirement: Fix column priorities so that critical
columns (Name, Status, Ready) remain readable while less-critical columns
(Namespace, IP, Node) shrink or hide first - **automatically**, not via
manual toggle.

### Solution Analysis: Three Approaches

#### Option 1: Automatic Column Hiding Based on Priority

Add priority levels to columns and automatically hide lower-priority
columns when terminal is too narrow.

**Pros**:
- **Most flexible**: Adapts automatically to any terminal width
- **User-friendly**: No manual intervention needed, just works
- **Maintains important info**: Critical columns (Name, Status) always
  visible
- **Scales well**: Easy to add new columns with appropriate priorities
- **Follows k9s pattern**: Similar to Wide attribute but automatic

**Cons**:
- **More complex implementation**: Need priority system + hiding logic
- **Can be disorienting**: Columns appear/disappear as window resizes
- **Hard to predict**: User doesn't know which columns visible without
  checking
- **Testing complexity**: Need to test multiple width scenarios
- **May hide desired info**: User can't override if they want specific
  column

**Implementation Complexity**: Medium-High
- Add `Priority int` field to `ColumnConfig`
- Modify `SetSize()` to calculate available space and hide columns
- Update all 17 screen configs with priorities
- Handle row rendering to skip hidden columns
- Add logic to determine which columns fit

**Example implementation**:
```go
type ColumnConfig struct {
    Field    string
    Title    string
    Width    int
    Format   func(interface{}) string
    Priority int  // NEW: 1=critical, 2=important, 3=optional
}

func (s *ConfigScreen) SetSize(width, height int) {
    // Calculate which columns fit based on priority
    availableWidth := width - (len(s.config.Columns) * 2) // padding
    visibleColumns := []ColumnConfig{}

    // Sort by priority (1 first, then 2, then 3)
    sorted := sortByPriority(s.config.Columns)

    usedWidth := 0
    for _, col := range sorted {
        colWidth := col.Width
        if colWidth == 0 {
            colWidth = 20  // Estimate for dynamic
        }

        if usedWidth + colWidth <= availableWidth {
            visibleColumns = append(visibleColumns, col)
            usedWidth += colWidth
        } else if col.Priority == 1 {
            // Critical column must show, even if squished
            visibleColumns = append(visibleColumns, col)
        }
    }

    s.visibleColumns = visibleColumns
    // ... rebuild table
}
```

#### Option 2: Change Width Allocation Strategy

Make less-important columns dynamic (Width: 0) instead of fixed, so they
shrink first when terminal is narrow.

**Pros**:
- **Simplest implementation**: Just change which columns are fixed vs
  dynamic
- **Predictable behavior**: All columns always visible, just sized
  differently
- **No hiding logic**: Reuses existing dynamic width calculation
- **Easy to configure**: Just change `Width: 40` to `Width: 0` in configs
- **Smooth transitions**: Columns grow/shrink, don't disappear

**Cons**:
- **Doesn't fully solve problem**: Namespace still takes space even if
  narrow
- **All columns still compete**: Total width still constrained
- **Can make columns too narrow**: Multiple dynamic columns at 20 chars
  min = 60+ chars
- **Less effective on very narrow terminals**: With 8 columns, each gets
  ~10 chars minimum
- **Namespace might be unreadable**: If it shrinks to 20 chars, long
  namespaces truncate

**Implementation Complexity**: Low
- Just modify screen configs (change `Width: 40` to `Width: 0`)
- No code changes needed in core logic
- Test on narrow terminals

**Example change for Pods screen**:
```go
// BEFORE (current):
Columns: []ColumnConfig{
    {Field: "Namespace", Title: "Namespace", Width: 40},  // FIXED
    {Field: "Name", Title: "Name", Width: 0},             // Dynamic
    {Field: "Ready", Title: "Ready", Width: 8},
    {Field: "Status", Title: "Status", Width: 15},
    {Field: "Restarts", Title: "Restarts", Width: 10},
    {Field: "Age", Title: "Age", Width: 10},
    {Field: "Node", Title: "Node", Width: 30},            // FIXED
    {Field: "IP", Title: "IP", Width: 16},                // FIXED
}

// AFTER (width reallocation):
Columns: []ColumnConfig{
    {Field: "Namespace", Title: "Namespace", Width: 0},   // Dynamic
    {Field: "Name", Title: "Name", Width: 40},            // FIXED
    {Field: "Ready", Title: "Ready", Width: 8},
    {Field: "Status", Title: "Status", Width: 15},
    {Field: "Restarts", Title: "Restarts", Width: 10},
    {Field: "Age", Title: "Age", Width: 10},
    {Field: "Node", Title: "Node", Width: 0},             // Dynamic
    {Field: "IP", Title: "IP", Width: 0},                 // Dynamic
}
```

**Effect**: On 150-char terminal:
- Fixed columns: 40 + 8 + 15 + 10 + 10 = 83 chars
- Padding: 8 * 2 = 16 chars
- Remaining: 150 - 83 - 16 = 51 chars
- Dynamic columns (3): 51 / 3 = 17 chars each (but min 20, so 20 each)
- **Result**: Name stays at 40 chars (readable), Namespace/Node/IP shrink
  to 20

#### Option 3: Responsive Breakpoints

Define different column sets for different terminal width ranges (like
mobile/tablet/desktop in web design).

**Pros**:
- **Most predictable**: User knows exactly what to expect at each width
- **Optimized layouts**: Can design perfect column sets for each size
- **Similar to web design**: Familiar pattern (mobile/tablet/desktop)
- **Clear transitions**: Discrete changes, not gradual
- **Can optimize for common terminal sizes**: 80, 120, 160 columns

**Cons**:
- **Rigid**: Only works at predefined breakpoints
- **Configuration explosion**: Need to define column sets for each
  breakpoint × each screen
- **Maintenance burden**: Adding new column requires updating all
  breakpoint configs
- **Sudden jumps**: Columns appear/disappear at specific widths (can be
  jarring)
- **Hard to get right**: Need to research common terminal sizes and use
  cases

**Implementation Complexity**: High
- Define breakpoint system (struct with width thresholds)
- Create multiple column configs per screen
- Logic to select appropriate config based on width
- Update all 17 screen configs with breakpoint variants (17 × 3 = 51
  configs)
- Testing across all breakpoints × all screens

**Example implementation**:
```go
type ScreenConfig struct {
    ID           string
    Title        string
    ResourceType k8s.ResourceType
    Breakpoints  []BreakpointConfig  // NEW
    // ... other fields
}

type BreakpointConfig struct {
    MinWidth int
    Columns  []ColumnConfig
}

// Pods screen with breakpoints:
Breakpoints: []BreakpointConfig{
    {
        MinWidth: 0,  // Narrow (< 100 chars)
        Columns: []ColumnConfig{
            {Field: "Name", Title: "Name", Width: 0},
            {Field: "Status", Title: "Status", Width: 15},
            {Field: "Ready", Title: "Ready", Width: 8},
            {Field: "Age", Title: "Age", Width: 10},
        },
    },
    {
        MinWidth: 100,  // Medium (100-150 chars)
        Columns: []ColumnConfig{
            {Field: "Namespace", Title: "Namespace", Width: 30},
            {Field: "Name", Title: "Name", Width: 0},
            {Field: "Ready", Title: "Ready", Width: 8},
            {Field: "Status", Title: "Status", Width: 15},
            {Field: "Restarts", Title: "Restarts", Width: 10},
            {Field: "Age", Title: "Age", Width: 10},
        },
    },
    {
        MinWidth: 160,  // Wide (>= 160 chars)
        Columns: []ColumnConfig{
            {Field: "Namespace", Title: "Namespace", Width: 40},
            {Field: "Name", Title: "Name", Width: 0},
            {Field: "Ready", Title: "Ready", Width: 8},
            {Field: "Status", Title: "Status", Width: 15},
            {Field: "Restarts", Title: "Restarts", Width: 10},
            {Field: "Age", Title: "Age", Width: 10},
            {Field: "Node", Title: "Node", Width: 30},
            {Field: "IP", Title: "IP", Width: 16},
        },
    },
}
```

### Comparison Matrix

| Criteria | Option 1: Priority | Option 2: Width | Option 3: Breakpoints |
|----------|-------------------|-----------------|---------------------|
| **Solves main problem** | ✅ Yes | ⚠️ Partial | ✅ Yes |
| **Implementation effort** | Medium-High | Low | High |
| **User experience** | Automatic | Simple | Predictable |
| **Maintenance** | Medium | Low | High |
| **Flexibility** | High | Low | Medium |
| **Testing complexity** | Medium | Low | High |
| **Config changes** | All screens | All screens | All screens × 3 |
| **Code changes** | Moderate | None | Significant |
| **Column hiding** | ✅ Yes | ❌ No | ✅ Yes |
| **All columns visible** | ❌ No | ✅ Yes | ❌ No |

### Recommended Approach: Hybrid (Phase 1 + Phase 2)

Combine **Option 2** (width allocation) for quick wins with **Option 1**
(priority hiding) for complete solution.

#### Phase 1: Quick Win - Width Reallocation

**Goal**: Solve 80% of problem with minimal effort

**Changes**:
1. Make **less-important columns dynamic** (Width: 0):
   - Namespace (can truncate, less critical than Name)
   - IP (nice-to-have, not essential)
   - Node (helpful but not critical)
2. Make **important columns fixed** (Width: N):
   - Name (Width: 40) - most important, needs space
   - Status (Width: 15) - critical state info
   - Ready (Width: 8) - critical readiness info
   - Age (Width: 10) - important temporal context

**Example Pods config change**:
```go
Columns: []ColumnConfig{
    {Field: "Namespace", Title: "Namespace", Width: 0},   // Now dynamic
    {Field: "Name", Title: "Name", Width: 40},            // Now fixed
    {Field: "Ready", Title: "Ready", Width: 8},
    {Field: "Status", Title: "Status", Width: 15},
    {Field: "Restarts", Title: "Restarts", Width: 10},
    {Field: "Age", Title: "Age", Width: 10},
    {Field: "Node", Title: "Node", Width: 0},             // Now dynamic
    {Field: "IP", Title: "IP", Width: 0},                 // Now dynamic
}
```

**Benefits**:
- Zero code changes (just config updates)
- Immediate improvement in narrow terminals
- Name column stays readable (40 chars guaranteed)
- Less-critical columns shrink first (Namespace, Node, IP)
- Can deploy quickly and test with users

**Limitations**:
- All columns still visible (may still be too wide for very narrow
  terminals)
- Multiple dynamic columns compete for space (each gets ~17-20 chars min)
- Namespace might be hard to read at 20 chars

#### Phase 2: Complete Solution - Priority-Based Hiding

**Goal**: Perfect the experience for very narrow terminals

**Changes**:
1. Add `Priority int` field to `ColumnConfig`:
   - Priority 1 (critical): Name, Status, Ready, Age
   - Priority 2 (important): Namespace, Restarts
   - Priority 3 (optional): Node, IP
2. Implement automatic column hiding in `SetSize()`:
   - Calculate available width
   - Show all Priority 1 columns (even if squished)
   - Show Priority 2 columns if they fit
   - Show Priority 3 columns if they fit
3. Add visual indicator when columns hidden:
   - Status bar shows "... (+2 hidden)" or similar
   - Optional: keybinding to temporarily show all (like k9s ctrl+w)

**Example implementation**:
```go
type ColumnConfig struct {
    Field    string
    Title    string
    Width    int
    Format   func(interface{}) string
    Priority int  // NEW: 1=critical, 2=important, 3=optional
}

func (s *ConfigScreen) SetSize(width, height int) {
    // Sort columns by priority
    sorted := make([]ColumnConfig, len(s.config.Columns))
    copy(sorted, s.config.Columns)
    sort.SliceStable(sorted, func(i, j int) bool {
        return sorted[i].Priority < sorted[j].Priority
    })

    // Calculate which columns fit
    padding := len(s.config.Columns) * 2
    availableWidth := width - padding
    visibleColumns := []ColumnConfig{}
    usedWidth := 0

    for _, col := range sorted {
        colWidth := col.Width
        if colWidth == 0 {
            colWidth = 20  // Minimum for dynamic
        }

        // Priority 1 always shows (critical)
        if col.Priority == 1 || usedWidth+colWidth <= availableWidth {
            visibleColumns = append(visibleColumns, col)
            usedWidth += colWidth
        }
    }

    // Restore original order for visibleColumns
    s.visibleColumns = restoreOrder(visibleColumns, s.config.Columns)
    s.hiddenCount = len(s.config.Columns) - len(visibleColumns)

    // Build table with only visible columns
    // ...
}
```

**Benefits**:
- Works on any terminal width (even 80 chars)
- Important info always visible
- Graceful degradation (hide least important first)
- Visual feedback when columns hidden

**Effort**:
- Add Priority field to ColumnConfig
- Update SetSize() with hiding logic
- Update all 17 screen configs with priorities
- Update row rendering to skip hidden columns
- Add hidden count indicator to status bar
- Test across width ranges

### Recommendation Summary

**Start with Phase 1** (width reallocation):
- Immediate improvement with zero code changes
- Low risk, easy to test
- Can deploy in < 1 hour

**Follow with Phase 2** (priority hiding) if needed:
- Perfects experience for very narrow terminals
- More complex but robust solution
- Enables future enhancements (user preferences, toggles)

### Implementation Priority for Phase 1

Update screen configs in this order:
1. **Pods** (most commonly used)
2. **Deployments** (critical for app management)
3. **Services** (networking troubleshooting)
4. **Nodes** (10 columns, needs it most)
5. Other screens (lower priority)

For each screen, identify:
- **Critical columns** (fixed width): Name, Status, state info
- **Context columns** (dynamic width): Namespace, labels, IPs
- **Optional columns** (dynamic width): Detailed metadata

### Open Questions for Phase 2

1. **Should priority be visible to users?**
   - Show in column headers? (e.g., asterisk for critical)
   - Document in help text?

2. **Should there be a keybinding to show all columns temporarily?**
   - Like k9s ctrl+w toggle
   - Or shift+w for "wide mode" override

3. **Should hidden column count be always visible or only when > 0?**
   - Status bar: "Pods (16 items, 2 columns hidden)"
   - Or only show when hiding occurs

4. **Should filtering search hidden columns?**
   - k9s searches all fields regardless of visibility
   - Seems reasonable: filter is about data, not display

5. **Should priority be user-configurable?**
   - Via ~/.config/k1/config.yaml
   - Per-screen column order/priority
   - Phase 3 feature after basic priority works
