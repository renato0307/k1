---
date: 2025-11-01 07:59:02 WET
researcher: Claude
git_commit: e7d4d4fa2893154656afb239ee16ab678d4f1947
branch: refactor/column-sizing
repository: k1-refactor-column-sizing
topic: "Column Self-Adjusting Strategy for k1"
tags: [research, codebase, column-sizing, ux, tui-patterns, bubble-table]
status: complete
last_updated: 2025-11-01
last_updated_by: Claude
last_updated_note: "Added bubble-table investigation (Alternative 4)"
---

# Research: Column Self-Adjusting Strategy for k1

**Date**: 2025-11-01 07:59:02 WET
**Researcher**: Claude
**Git Commit**: e7d4d4fa2893154656afb239ee16ab678d4f1947
**Branch**: refactor/column-sizing
**Repository**: k1-refactor-column-sizing

## Research Question

How can k1 implement self-adjusting column sizing that:
1. Keeps Name and Namespace columns always completely visible (with max cap)
2. Makes these critical columns "pretty constant" (minimal shrinking)
3. Allows less important columns to have title cuts when containing small
   values
4. Provides best-effort sizing for remaining columns
5. Eliminates per-screen manual tweaking
6. Maintains good UX while being easy to maintain

Compare with k9s and other TUI tools to find maintainable alternatives.

## Summary

Research reveals **four approaches** for k1's column sizing refactor,
ranging from incremental improvements to library changes:

1. **Enhanced Priority-Weighted Distribution** (RECOMMENDED): Extends
   current system with min/max constraints and weighted allocation instead
   of equal distribution. Low implementation risk, maintains current
   architecture. 1-2 days effort.

2. **Flex-Grow Proportional System**: CSS flexbox-inspired approach using
   growth ratios. Provides fine-grained control but redundant with
   Alternative 1.

3. **Switch to tview Library**: Adopts k9s's approach with automatic width
   calculation. Major refactor (multi-week), loses Charm ecosystem
   integration. NOT RECOMMENDED.

4. **Switch to bubble-table Library**: Bubble Tea compatible library with
   built-in flex-grow sizing. Attractive features but has open issues
   (#212, #187) that could block k1, lacks explicit min/max constraints,
   and requires 2-3 days migration. NOT RECOMMENDED.

**Key insight from k9s**: They delegate ALL column sizing to tview library
(no custom algorithms), suggesting either (a) implement smart sizing in k1
or (b) switch libraries entirely.

**Key insight from bubble-table investigation**: While bubble-table offers
built-in flex-grow proportional sizing, the open issues, migration risk,
and lack of explicit min/max support make implementing weighted
distribution in bubbles/table more practical.

**Critical finding from existing research**: k1 currently uses only ONE
dynamic column per screen (usually Name), causing important columns to get
squeezed on narrow terminals while less-important columns stay fixed-width
(inverse of desired behavior).

## Detailed Findings

### Current k1 Implementation

**Source**: `internal/screens/config.go:318-439`,
`internal/screens/screens.go`

#### Architecture Overview

k1 implements a **config-driven responsive column sizing system** with
priority-based hiding and mixed fixed/dynamic width allocation:

```go
// internal/screens/config.go:29-36
type ColumnConfig struct {
    Field    string                   // Struct field name
    Title    string                   // Display title
    Width    int                      // 0 = dynamic, >0 = fixed
    Format   func(interface{}) string // Optional formatter
    Priority int                      // 1=critical, 2=important, 3=optional
}
```

**Width semantics**:
- `Width: 0` → Dynamic (fills remaining space proportionally)
- `Width: >0` → Fixed (exact character width)

**Priority semantics**:
- `Priority: 1` → Critical (never hidden, even if squished)
- `Priority: 2` → Important (hidden only if space constrained)
- `Priority: 3` → Optional (first to be hidden)

#### Example Configuration (Pods Screen)

```go
// internal/screens/screens.go:18-27
Columns: []ColumnConfig{
    {Field: "Namespace", Title: "Namespace", Width: 0, Priority: 2},
    {Field: "Name", Title: "Name", Width: 50, Priority: 1},
    {Field: "Ready", Title: "Ready", Width: 8, Priority: 1},
    {Field: "Status", Title: "Status", Width: 15, Priority: 1},
    {Field: "Age", Title: "Age", Width: 10, Priority: 1},
    {Field: "Node", Title: "Node", Width: 0, Priority: 3},
    {Field: "IP", Title: "IP", Width: 0, Priority: 3},
}
```

**Pattern observed across all 11+ screens**:
- **Name columns**: `Width: 40-50` (fixed), `Priority: 1`
- **Namespace columns**: `Width: 0` (dynamic), `Priority: 2`
- **Status/metrics**: `Width: 10-15` (fixed), `Priority: 1`
- **Age**: `Width: 10` (fixed), `Priority: 1` (consistent everywhere)
- **Extra fields**: `Width: 0` (dynamic), `Priority: 3`

#### Sizing Algorithm

**Location**: `internal/screens/config.go:318-401` (`ConfigScreen.SetSize`)

**Triggered by**: `tea.WindowSizeMsg` (terminal resize)

**Algorithm steps**:

```
1. Calculate available width (line 323-325)
   availableWidth = terminalWidth - (numColumns * 2)  // padding

2. Sort columns by priority (line 327-332)
   sorted = columns sorted by Priority ascending

3. Determine visible columns (line 335-351)
   for each column in priority order:
     if shouldExcludeColumn(col, availableWidth, usedWidth):
       skip
     else:
       add to visibleColumns
       usedWidth += colWidth (or 20 if dynamic)

4. Restore original column order (line 354)
   visibleColumns = restoreColumnOrder(visibleColumns)

5. Calculate dynamic widths (line 358-377)
   fixedTotal = sum of visible fixed-width columns
   dynamicCount = count of visible Width=0 columns
   dynamicWidth = (terminalWidth - fixedTotal - padding) / dynamicCount
   if dynamicWidth < 20: dynamicWidth = 20  // Minimum

6. Rebuild table columns (line 380-396)
   Create table.Column[] from visibleColumns with computed widths
   SetColumns() on bubbles table

7. Rebuild table rows (line 400)
   updateTable() // Regenerates rows from filtered items
```

**Column hiding logic** (`internal/screens/config.go:406-419`):

```go
func shouldExcludeColumn(col ColumnConfig, available, used int) bool {
    colWidth := col.Width
    if colWidth == 0 {
        colWidth = 20  // Minimum estimate for dynamic columns
    }

    if col.Priority == 1 {
        return false  // Critical columns NEVER hide
    }

    return used+colWidth > available  // Priority 2/3 hide if no space
}
```

#### Current Problems

1. **Equal distribution flaw**: All dynamic columns share remaining space
   equally, regardless of importance

2. **Inverse priority**: Name is often FIXED (50 chars) while less
   important columns (Namespace, Node, IP) are DYNAMIC, causing Name to
   dominate on narrow terminals

3. **No min/max constraints**: Dynamic columns can grow arbitrarily large
   or shrink too small (20 char minimum applies to hiding logic, not
   display)

4. **Manual per-screen tuning**: Each screen hardcodes widths (Name is 30
   in Endpoints, 40 in Services, 50 in Pods)

5. **No content-aware sizing**: Width never considers actual cell content
   length

**Source of issues identified in prior research**
(`thoughts/shared/research/2025-10-26-column-display-smaller-windows.md`):
> "k1 uses only one dynamic column per screen (usually Name), causing
> important columns to get squeezed on narrow terminals while
> less-important columns (Namespace, IP, Node) stay fixed-width"

### k9s Implementation

**Source**: [k9s GitHub Repository](https://github.com/derailed/k9s)

#### Architecture Overview

k9s uses a **delegation strategy**: it does NOT implement custom column
width algorithms. Instead, it relies entirely on the **tview library's
built-in automatic width calculation**.

**Key files**:
- `internal/render/pod.go` (and similar): Define column metadata
- `internal/ui/table.go`: Uses tview's table rendering
- `internal/model1/header.go`: Column header structure

#### Column Definition

```go
// internal/model1/header.go
type HeaderColumn struct {
    Name string
    Attrs
}

type Attrs struct {
    Align     int            // Column alignment
    Decorator DecoratorFunc  // Custom formatting
    Wide      bool           // Show only in "Wide" mode
    Hide      bool           // Hidden column
    MX/MXC    bool          // Metrics flags
    Time      bool          // Timestamp indicator
}
```

**Notable**: No explicit width values in column definitions.

#### Rendering Logic

```go
// internal/ui/table.go - UpdateUI method (simplified)
func (t *Table) UpdateUI(cdata TableData) {
    // k9s just sets cell content - tview calculates widths
    for each row:
        for each column:
            field = formatCell(field, pads[c])  // Apply padding
            cell := tview.NewTableCell(field)
            t.SetCell(row, col, cell)

    // tview automatically calculates column widths based on content
}
```

#### Wide Mode Toggle

k9s provides a **compact/expanded mode toggle**:

```go
func (t *Table) ToggleWide() {
    t.wide = !t.wide
    t.Refresh()  // Redraws with/without "Wide" columns
}

func shouldExcludeColumn(col Column) bool {
    if col.Hide { return true }
    if !t.wide && col.Wide { return true }  // Hide in compact mode
    return false
}
```

#### Key Takeaway from k9s

**k9s does NOT implement custom column sizing algorithms**. It delegates
to tview, which provides:
- Automatic content-based width calculation
- Proportional expansion when extra space exists
- MaxWidth constraints per column
- Fixed columns option

**Implication for k1**: Either (1) implement smart sizing ourselves in
Bubble Tea, or (2) switch to tview library to get automatic sizing like
k9s.

### tview Library Column Sizing

**Source**: [rivo/tview](https://github.com/rivo/tview) - The library k9s
uses

#### Automatic Width Calculation

tview implements **content-aware automatic sizing**:

```go
// From tview/table.go (conceptual):
1. Measure cell widths:
   cellWidth := TaggedStringWidth(cell.Text)

2. Track maximum per column across all rows:
   maxWidths[col] = max(maxWidths[col], cellWidth)

3. Apply optional MaxWidth constraints:
   if cell.MaxWidth > 0 && maxWidths[col] > cell.MaxWidth:
       maxWidths[col] = cell.MaxWidth

4. Support proportional expansion:
   cell.Expansion = 2  // Grows 2x vs Expansion=1 columns
   expWidth := remainingSpace * expansion / totalExpansion
   widths[index] += expWidth
```

**Key features**:
- **No explicit widths required** - calculated from content
- **MaxWidth per column** - prevents domination
- **Expansion ratios** - proportional growth (like CSS flex-grow)
- **Fixed columns** - leftmost columns stay fixed during scroll
- **EvaluateAllRows option** - all rows vs only visible (performance)

**Comparison to Bubble Tea**:

| Feature | tview | Bubble Tea (bubbles/table) |
|---------|-------|----------------------------|
| Automatic width | ✅ Yes | ❌ No (manual calculation) |
| Explicit width required | ❌ No | ✅ Yes (`Column.Width`) |
| Content-based | ✅ Yes | ❌ No |
| Proportional expansion | ✅ Yes (`Expansion`) | ❌ No |
| MaxWidth constraints | ✅ Yes | ❌ No (must calculate) |

**Implication**: Bubble Tea requires manual width calculation, while tview
provides it out of the box.

### Other TUI Column Sizing Patterns

#### 1. Fixed Width (Current bubbles/table)

**How it works**:
- Developer specifies exact widths: `Column{Title: "Name", Width: 20}`
- Content truncated with "…" if exceeds width
- No responsive behavior

**Pros**: Simple, predictable
**Cons**: Not responsive, requires manual tuning, wastes/overflows space

#### 2. Content-Aware Auto-Sizing (tablewriter, go-pretty)

**Source**: [olekukonko/tablewriter](https://github.com/olekukonko/tablewriter),
[jedib0t/go-pretty](https://github.com/jedib0t/go-pretty)

**Algorithm**:
```go
1. Scan all content:
   maxColumnLengths[i] = max(content widths across all rows in column i)

2. Apply constraints:
   if maxColumnLengths[i] > MaxWidth:
       maxColumnLengths[i] = MaxWidth
   if maxColumnLengths[i] < MinWidth:
       maxColumnLengths[i] = MinWidth

3. Add padding:
   finalWidth = PaddingLeft + content + PaddingRight
```

**Pros**: No wasted space, automatic adaptation
**Cons**: Requires scanning all data (not suitable for streaming), no user
control over importance, may allocate too much to verbose columns

**Not ideal for TUI**: Best for CLI output (non-interactive), not real-time
updates.

#### 3. Proportional Flex-Based Sizing (bubble-table, tview)

**Source**: [Evertras/bubble-table](https://github.com/Evertras/bubble-table)

**Algorithm** (identical to CSS flexbox `flex-grow`):
```go
1. Define flex ratios:
   NewFlexColumn(key, title, ratio)

2. Calculate available space:
   availableWidth = totalWidth - sumOfFixedWidths

3. Sum flex ratios:
   totalRatio = sum of all flex column ratios

4. Distribute proportionally:
   columnWidth = baseWidth + (availableWidth * ratio / totalRatio)
```

**Example**:
```go
columns := []table.Column{
    table.NewColumn("Name", 10),              // Fixed: 10 chars
    table.NewFlexColumn("Element", "El", 1),  // Flex ratio: 1
    table.NewFlexColumn("Desc", "Desc", 3),   // Flex ratio: 3
}
// Terminal width = 100, fixed = 10, available = 90
// Element gets: 90 * 1/4 = 22 chars
// Desc gets: 90 * 3/4 = 68 chars
```

**CSS flexbox equivalence** ([MDN: flex-grow](https://developer.mozilla.org/en-US/docs/Web/CSS/flex-grow)):
> "Flex items with higher flex-grow values receive more of the available
> space proportionally"

**Pros**: Responsive, maintains proportions, flexible, efficient (no
content scanning), familiar mental model
**Cons**: More complex, requires understanding ratios

#### 4. Hybrid Priority-Based Hiding (k9s approach)

**Pattern**: Columns have metadata (Hide, Wide) and toggle between
compact/expanded layouts:

```go
func ToggleWide() {
    t.wide = !t.wide
    t.Refresh()
}

func visibleColumns() []Column {
    visible := []Column{}
    for _, col := range allColumns {
        if col.Hide { continue }
        if !t.wide && col.Wide { continue }  // Hide in compact mode
        visible = append(visible, col)
    }
    return visible
}
```

**Pros**: Adapts to terminal size by hiding columns, maintains readability
on small terminals, user toggleable
**Cons**: Users lose information when columns hidden, more complex state
management

### bubble-table Library (Bubble Tea Compatible)

**Source**: [Evertras/bubble-table](https://github.com/Evertras/bubble-table)
(v0.19.2, September 2024)

**Relevance**: A Bubble Tea table library with **built-in flex-grow
proportional sizing** - exactly what k1 needs for column sizing.

#### Library Maturity & Stats

- **Stars**: 536 (moderate adoption)
- **Latest Release**: v0.19.2 (September 6, 2024)
- **Total Releases**: 61 releases (active development)
- **Contributors**: 13
- **License**: MIT
- **Maintenance**: 2-3 releases per month historically
- **Open Issues**: 18 (some relevant to k1's use case)

#### Key Features vs bubbles/table

| Feature | bubbles/table | bubble-table | k1 Benefit |
|---------|---------------|--------------|------------|
| **Flex-grow column sizing** | ❌ No | ✅ Yes | **Core requirement** |
| **Fixed + dynamic mix** | ✅ Manual calc | ✅ Automatic | Eliminates calculations |
| **Min/max width** | ❌ No | ⚠️ Workaround | Partial solution |
| **Priority hiding** | ❌ No | ❌ No | Still need custom |
| **Frozen columns** | ❌ No | ✅ Yes | Nice bonus |
| **Horizontal scroll** | ❌ No | ✅ Yes | Handles wide tables |
| **Built-in filtering** | ❌ No | ✅ Yes | Conflicts with k1 |
| **Sorting** | ❌ No | ✅ Yes | Future feature |
| **Pagination** | ❌ No | ✅ Yes | Nice bonus |
| **Style functions** | ❌ No | ✅ Yes | Dynamic styling |
| **Row metadata** | ❌ No | ✅ Yes | Useful for ops |

#### Flex-Grow Implementation

**API Design** (identical to CSS flexbox):
```go
// Fixed-width columns
table.NewColumn(columnKeyName, "Name", 13)  // 13 chars fixed

// Flex-grow columns (proportional sizing)
table.NewFlexColumn(columnKeyElement, "Element", 1)    // Ratio: 1
table.NewFlexColumn(columnKeyDescription, "Desc", 3)   // Ratio: 3

// Must set target width (terminal width)
model := table.New(columns).WithTargetWidth(terminalWidth)
```

**Distribution algorithm**:
1. Fixed columns consume their specified width
2. Remaining space distributed among flex columns proportionally
3. Example: Terminal=100, Fixed=13, Remaining=87
   - Element (ratio 1): 87 × (1/4) = 21 chars
   - Description (ratio 3): 87 × (3/4) = 66 chars

**Benefits for k1**:
- ✅ Eliminates manual width calculation code (~79 lines)
- ✅ Responsive to terminal size (automatic recalculation)
- ✅ Familiar CSS flexbox mental model
- ✅ Same Bubble Tea patterns (Init/Update/View)
- ✅ Uses lipgloss for styling (same as k1)

#### Known Issues & Limitations

**Critical open issues**:

1. **Update() timing issue** (#212, Sep 2024)
   - Update() not called every tick
   - ⚠️ Could affect k1's auto-refresh feature

2. **Column hiding breaks layout** (#187)
   - Dynamically hiding columns can break table
   - ⚠️ k1 needs priority-based column hiding

3. **Terminal resize artifacts** (#193)
   - Visual glitches on resize
   - ⚠️ Test thoroughly with k1's dynamic sizing

4. **No explicit min/max constraints**
   - Must simulate via target width or fixed columns
   - ⚠️ k1 requires MinWidth/MaxWidth for Name/Namespace

5. **Filter conflict**
   - Built-in filter might conflict with k1's command bar
   - ⚠️ Would need to disable bubble-table's filter

**Overall risk**: ⚠️ **Moderate** - Several open issues affecting k1's
specific use cases

#### Migration Complexity

**Code changes**:
- `go.mod`: Change dependency from bubbles/table to bubble-table
- `internal/screens/config.go`: Remove width calculation (~79 lines)
- `internal/screens/screens.go`: Convert 11+ configs to flex columns
- `internal/screens/dynamic_screens.go`: Update CRD column generation
- Test files: Update table-related tests

**API comparison**:
```go
// BEFORE (bubbles/table - current k1)
import "github.com/charmbracelet/bubbles/table"

// Manual width calculation in SetSize() (79 lines)
func (s *ConfigScreen) SetSize(width, height int) {
    // Calculate available, sort by priority, filter visible,
    // calculate dynamic widths, rebuild table...
}

// AFTER (bubble-table)
import table "github.com/Evertras/bubble-table/table"

// Simplified SetSize()
func (s *ConfigScreen) SetSize(width, height int) {
    columns := []table.Column{
        table.NewFlexColumn("namespace", "Namespace", 2),  // Weight 2
        table.NewFlexColumn("name", "Name", 3),            // Weight 3
        table.NewColumn("status", "Status", 12),           // Fixed
    }

    model := table.New(columns).
        WithTargetWidth(width).
        WithRows(rows)
}
```

**Effort**: 2-3 days (1 day dependency change, 1 day screen migration,
1 day testing)

#### Decision: NOT RECOMMENDED for k1

**Why stay with bubbles/table + enhanced custom logic**:

1. ✅ **Lower risk**: No dependency change, no migration
2. ✅ **Stability**: Charm ecosystem more mature (18.9k vs 536 stars)
3. ✅ **Full control**: Can implement exact min/max and priority semantics
4. ✅ **No blockers**: Open issues (#212, #187) could block k1
5. ✅ **Faster**: 1-2 days vs 2-3 days implementation
6. ✅ **Already designed**: Alternative 1 plan is ready

**When to reconsider bubble-table**:
- If k1 needs frozen columns (Name always visible during scroll)
- If k1 needs horizontal scrolling for wide tables
- If bubble-table's open issues (#212, #187) get resolved
- If k1 wants built-in sorting/pagination features

**Conclusion**: The flex-grow feature is attractive, but the open issues,
migration risk, and lack of min/max support make it less appealing than
implementing weighted distribution in the existing bubbles/table
architecture.

## Architecture Insights

### Bubble Tea vs tview Trade-offs

| Aspect | Bubble Tea (current k1) | tview (k9s) |
|--------|-------------------------|-------------|
| **Width calculation** | Manual (developer) | Automatic (library) |
| **Ecosystem** | Charm (lipgloss, bubbles) | tcell |
| **Styling** | lipgloss (modern, composable) | tcell.Style (basic) |
| **Patterns** | Elm architecture (Model/Update/View) | Event-driven |
| **Column widths** | Explicit integers required | Content-based |
| **Maintenance** | Custom sizing logic needed | Delegated to library |
| **Flexibility** | Full control over algorithm | Limited to tview features |

### Why k1 Needs Better Sizing

**Current pain points** (from codebase analysis and prior research):

1. **11+ screens with manual width tuning**: Pods, Deployments, Services,
   Nodes, ConfigMaps, Secrets, Namespaces, StatefulSets, DaemonSets,
   CronJobs, Jobs, ReplicaSets, PVCs, Ingresses, Endpoints, HPAs, CRDs,
   Contexts, + dynamic CRD instances

2. **Known bugs** (from `thoughts/shared/tickets/roadmap.md`):
   - Columns with bad sizing on ConfigMaps
   - Columns with bad sizing on DaemonSets
   - Columns with bad sizing on CronJobs
   - Columns with bad sizing on CRDs

3. **Inconsistent width conventions**:
   - Name: 30 (Endpoints), 40 (Services/Nodes), 50 (Pods/Deployments)
   - Status: 12 (Nodes), 15 (Namespaces/Services)

4. **Inverse priority problem**: Critical columns (Name) are fixed while
   less important (Namespace, IP) are dynamic

### Width Measurement in Go

All TUI libraries use **go-runewidth** for terminal width calculation:

**Source**: [mattn/go-runewidth](https://pkg.go.dev/github.com/mattn/go-runewidth)

```go
import "github.com/mattn/go-runewidth"

// Calculate terminal width (handles CJK, emoji, etc.)
width := runewidth.StringWidth("つのだ☆HIRO")  // Returns 12

// Truncate to width
truncated := runewidth.Truncate("long string", 10, "…")
```

k1 already has access via **lipgloss** (already imported):

```go
import "github.com/charmbracelet/lipgloss"

// Get actual width of styled content
width := lipgloss.Width(styledBlock)
```

## Proposed Alternatives for k1

### Alternative 1: Enhanced Priority-Weighted Distribution (Recommended)

**Approach**: Extend current system with min/max constraints and weighted
allocation.

#### Changes to ColumnConfig

```go
type ColumnConfig struct {
    Field    string
    Title    string
    MinWidth int     // NEW: Minimum width (readability)
    MaxWidth int     // NEW: Maximum width (prevent domination)
    Weight   float64 // NEW: Distribution weight (replaces Width)
    Format   func(interface{}) string
    Priority int     // KEEP: For hiding on narrow terminals
}

// Example configuration:
{Field: "Name", MinWidth: 15, MaxWidth: 50, Weight: 3, Priority: 1}
// Means: Name gets 3x space of Weight=1 columns, capped at 50 chars
```

#### Algorithm

```go
func CalculateColumnWidths(configs []ColumnConfig, available int) []int {
    // 1. Allocate minimums (respect Priority for exclusion)
    totalMin := 0
    visible := filterVisibleColumns(configs, available)  // Priority-based
    for _, col := range visible {
        totalMin += col.MinWidth
    }

    remaining := available - totalMin - (len(visible) * 2)  // padding
    if remaining <= 0 {
        return minimumWidths  // Can't fit, use minimums
    }

    // 2. Distribute remaining space by weight
    totalWeight := 0.0
    for _, col := range visible {
        totalWeight += col.Weight
    }

    widths := make([]int, len(visible))
    for i, col := range visible {
        baseWidth := col.MinWidth
        extraSpace := int(float64(remaining) * (col.Weight / totalWeight))
        finalWidth := baseWidth + extraSpace

        // 3. Apply max width cap
        if col.MaxWidth > 0 && finalWidth > col.MaxWidth {
            finalWidth = col.MaxWidth
        }

        widths[i] = finalWidth
    }

    return widths
}
```

#### Migration Strategy

Convert existing configs using these heuristics:

```go
// OLD: {Field: "Name", Width: 50, Priority: 1}
// NEW: {Field: "Name", MinWidth: 20, MaxWidth: 50, Weight: 3, Priority: 1}

// OLD: {Field: "Namespace", Width: 0, Priority: 2}  // dynamic
// NEW: {Field: "Namespace", MinWidth: 10, MaxWidth: 30, Weight: 2, Priority: 2}

// OLD: {Field: "Status", Width: 15, Priority: 1}
// NEW: {Field: "Status", MinWidth: 10, MaxWidth: 15, Weight: 1, Priority: 1}

// OLD: {Field: "Age", Width: 10, Format: FormatDuration, Priority: 1}
// NEW: {Field: "Age", MinWidth: 8, MaxWidth: 12, Weight: 1, Priority: 1}

// OLD: {Field: "Node", Width: 0, Priority: 3}  // dynamic, low priority
// NEW: {Field: "Node", MinWidth: 10, MaxWidth: 40, Weight: 2, Priority: 3}
```

**Default values** for common column types (add to `screens/constants.go`):

```go
const (
    NameMinWidth   = 20
    NameMaxWidth   = 50
    NameWeight     = 3

    NamespaceMinWidth = 10
    NamespaceMaxWidth = 30
    NamespaceWeight   = 2

    StatusMinWidth = 10
    StatusMaxWidth = 15
    StatusWeight   = 1

    AgeMinWidth = 8
    AgeMaxWidth = 12
    AgeWeight   = 1
)
```

#### Benefits

✅ **Addresses user requirements**:
- Name/Namespace always visible (MinWidth) with cap (MaxWidth)
- Pretty constant (Weight controls proportional growth)
- Easy maintenance (convention-based defaults)
- Good UX (important columns get more space)

✅ **Low implementation risk**:
- Extends existing ColumnConfig (no breaking changes to screen interface)
- Reuses existing Priority-based hiding
- Centralized in ConfigScreen.SetSize() (single file change)

✅ **Backward compatible**:
- Can support old Width-based configs during migration
- Incremental screen-by-screen updates

❌ **Limitations**:
- Still requires per-screen configuration (though convention-based)
- Doesn't consider actual content length

### Alternative 2: Flex-Grow Proportional System

**Approach**: CSS flexbox-inspired growth ratios (like bubble-table).

#### Changes to ColumnConfig

```go
type ColumnConfig struct {
    Field    string
    Title    string
    MinWidth int     // NEW: Minimum width
    MaxWidth int     // NEW: Optional cap
    FlexGrow float64 // NEW: Growth ratio (0 = fixed at MinWidth)
    Format   func(interface{}) string
    Priority int     // KEEP: For hiding
}

// Example:
{Field: "Name", MinWidth: 20, FlexGrow: 3, Priority: 1}
// FlexGrow=3 means Name gets 3x space of FlexGrow=1 columns
```

#### Algorithm

```go
func CalculateColumnWidths(configs []ColumnConfig, available int) []int {
    // 1. Allocate minimums
    totalMin := 0
    for _, col := range configs {
        totalMin += col.MinWidth
    }

    remaining := available - totalMin - padding
    if remaining <= 0 {
        return minimumWidths
    }

    // 2. Sum flex-grow values
    totalFlex := 0.0
    for _, col := range configs {
        totalFlex += col.FlexGrow
    }

    // 3. Distribute by flex-grow
    widths := make([]int, len(configs))
    for i, col := range configs {
        baseWidth := col.MinWidth
        flexSpace := int(float64(remaining) * (col.FlexGrow / totalFlex))
        finalWidth := baseWidth + flexSpace

        // Apply MaxWidth cap if specified
        if col.MaxWidth > 0 && finalWidth > col.MaxWidth {
            finalWidth = col.MaxWidth
        }

        widths[i] = finalWidth
    }

    return widths
}
```

#### Benefits

✅ **Fine-grained control**: Precise ratios (1.5x, 2.5x, etc.)
✅ **Familiar mental model**: Same as CSS flexbox
✅ **Flexible**: Can easily adjust importance per screen

❌ **Learning curve**: Requires understanding flex-grow concept
❌ **Redundant with Priority**: Both FlexGrow and Weight serve similar
purposes (Alternative 1's Weight is simpler)

### Alternative 3: Switch to tview Library (Like k9s)

**Approach**: Adopt k9s's strategy - delegate to tview for automatic width
calculation.

#### Changes Required

1. **Replace Bubble Tea with tview** (major refactor)
2. **Rewrite all screens** to use tview.Table API
3. **Replace lipgloss styling** with tcell.Style
4. **Convert Elm architecture** to event-driven pattern

#### Benefits

✅ **Automatic width calculation** (no manual sizing code)
✅ **Battle-tested** (k9s proves it works at scale)
✅ **Less maintenance** (delegate to library)

❌ **Major refactor** (rewrite most UI code)
❌ **Lose Charm ecosystem** (lipgloss, bubbles patterns)
❌ **Different mental model** (event-driven vs Elm architecture)
❌ **High risk** (large surface area for bugs)

#### Recommendation: NOT RECOMMENDED

The benefits don't outweigh the refactoring cost. k1 is already built on
Bubble Tea with a clean architecture. Switching libraries would be a
multi-week effort with high risk.

## Recommended Approach

**Use Alternative 1: Enhanced Priority-Weighted Distribution**

### Rationale

1. **Meets all user requirements**:
   - Name/Namespace always visible with caps ✅
   - Pretty constant sizing via weighted distribution ✅
   - Less important columns get less space ✅
   - No per-screen manual tweaking (convention-based defaults) ✅
   - Good UX (important columns prioritized) ✅
   - Easy to maintain (centralized algorithm, convention over
     configuration) ✅

2. **Low implementation risk**:
   - Extends existing architecture (no rewrites)
   - Single file change (`config.go:SetSize()`)
   - Incremental migration (screen-by-screen)
   - Backward compatible during transition

3. **Addresses known bugs**:
   - ConfigMaps/DaemonSets/CronJobs/CRDs sizing issues get fixed by new
     algorithm
   - Eliminates manual width tuning across 11+ screens

4. **Avoids over-engineering**:
   - Simpler than Alternative 2 (Weight is more intuitive than FlexGrow)
   - More practical than Alternative 3 (no library switch)

5. **Comparison with bubble-table**:
   - ✅ **Lower risk**: No dependency change (bubble-table has open issues
     #212, #187)
   - ✅ **Stability**: Charm ecosystem more mature (18.9k vs 536 stars)
   - ✅ **Full control**: Exact min/max semantics (bubble-table lacks
     explicit min/max)
   - ✅ **No filter conflicts**: k1's command bar works as-is
   - ✅ **Faster**: 1-2 days vs 2-3 days migration
   - ✅ **Same result**: Weight-based distribution achieves same goal as
     flex-grow
   - Note: bubble-table's flex-grow is attractive, but open issues and
     migration risk outweigh benefits

### Implementation Plan Summary

**Phase 1: Core Algorithm**
1. Add MinWidth, MaxWidth, Weight fields to ColumnConfig
2. Update SetSize() algorithm with weighted distribution
3. Add convention-based constants for common column types

**Phase 2: Screen Migration**
1. Convert high-priority screens (Pods, Deployments, Services, Nodes)
2. Fix known buggy screens (ConfigMaps, DaemonSets, CronJobs, CRDs)
3. Update remaining screens

**Phase 3: Testing & Refinement**
1. Test across terminal sizes (80, 120, 200+ columns)
2. Validate Name/Namespace visibility guarantees
3. Adjust weights based on real-world feedback

## Code References

### Current Implementation
- `internal/screens/config.go:29-36` - ColumnConfig struct
- `internal/screens/config.go:318-401` - SetSize() algorithm
- `internal/screens/config.go:406-419` - shouldExcludeColumn() hiding
  logic
- `internal/screens/screens.go:18-548` - Static screen configs
- `internal/screens/dynamic_screens.go:10-130` - CRD column generation

### External References
- [k9s internal/ui/table.go](https://github.com/derailed/k9s/blob/master/internal/ui/table.go) -
  k9s table rendering (delegates to tview)
- [tview/table.go](https://github.com/rivo/tview/blob/master/table.go) -
  Automatic width calculation (k9s uses this)
- [bubble-table](https://github.com/Evertras/bubble-table) - Bubble Tea
  table with flex-grow (considered but not recommended due to open issues)
- [bubbles/table](https://github.com/charmbracelet/bubbles/tree/master/table) -
  Current k1 table library (staying with this)
- [CSS flex-grow spec](https://developer.mozilla.org/en-US/docs/Web/CSS/flex-grow) -
  Proportional distribution algorithm (inspiration for Weight field)
- [go-runewidth](https://pkg.go.dev/github.com/mattn/go-runewidth) - Width
  measurement (used by lipgloss)

## Historical Context (from thoughts/)

### Prior Research

**File**: `thoughts/shared/research/2025-10-26-column-display-smaller-windows.md`

Key findings from October 26 research:
- Identified inverse priority problem: Name is fixed while less important
  columns are dynamic
- Compared k1's simple dynamic width allocation vs k9s's Wide column
  attribute
- Analyzed three solution approaches (auto-hiding, reallocation, fixed
  adjustment)
- Documented k9s patterns: priority tiers, mode toggling, YAML config

**Critical insight**:
> "k1 uses only one dynamic column per screen (usually Name), causing
> important columns to get squeezed on narrow terminals while
> less-important columns (Namespace, IP, Node) stay fixed-width"

### Existing Implementation Plan

**File**: `thoughts/shared/plans/2025-10-26-responsive-column-display.md`

Three-phase plan proposed:
- **Phase 1**: Add Priority field to ColumnConfig, reallocate fixed/dynamic
  widths
- **Phase 2**: Priority-based automatic column hiding
- **Phase 3**: Testing & validation

**Status**: Partially implemented (Priority field exists, but equal
distribution problem remains)

### Known Issues

**File**: `thoughts/shared/tickets/roadmap.md`

High-priority bugs:
- Columns with bad sizing on ConfigMaps
- Columns with bad sizing on DaemonSets
- Columns with bad sizing on CronJobs
- Columns with bad sizing on CRDs

Nice-to-have features:
- Shortcut to expand/collapse columns with copy+paste
- User-selectable default screen
- Item counts display

## Related Research

- `thoughts/shared/research/2025-10-08-scaling-to-71-api-resources.md` -
  Scaling to support all 71 Kubernetes API resources (navigation and
  categorization)
- `thoughts/shared/research/2025-10-26-column-display-smaller-windows.md` -
  Column display and prioritization (comprehensive analysis)
- `thoughts/shared/plans/2025-10-26-responsive-column-display.md` -
  Implementation plan for responsive columns

## Open Questions

1. **Content-based sizing**: Should we scan actual cell content to inform
   width calculations? (Performance vs accuracy trade-off)

2. **User configuration**: Should users be able to customize column widths
   via config file? (~/.config/k1/columns.yaml)

3. **Column reordering**: Should users be able to reorder columns?
   (Separate feature, out of scope)

4. **Dynamic weight adjustment**: Should weights auto-adjust based on
   content distribution? (Complex, likely over-engineering)

5. **Title truncation strategy**: How should we truncate column titles for
   narrow columns? (center "...", left "...", right "..."?)
