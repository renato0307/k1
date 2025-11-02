# Enhanced Priority-Weighted Column Sizing Implementation Plan

## Overview

Refactor k1's column sizing system to use **weighted distribution** instead
of equal space sharing. This eliminates manual per-screen width tuning and
ensures critical columns (Name, Namespace) stay fully visible while less
important columns gracefully shrink on narrow terminals.

**Goal**: Replace current "equal distribution" algorithm with
"weighted-proportional" algorithm using min/max constraints.

## Current State Analysis

**Location**: `internal/screens/config.go:318-401` (SetSize method)

**Current algorithm problems**:
1. ❌ **Equal distribution flaw**: All dynamic columns (Width=0) share
   remaining space equally, regardless of importance
2. ❌ **Inverse priority**: Name is often FIXED (50 chars) while less
   important columns (Namespace, Node, IP) are DYNAMIC, causing Name to
   dominate on narrow terminals
3. ❌ **No constraints**: Dynamic columns can grow arbitrarily large or
   shrink too small (20 char minimum only applies to hiding logic, not
   display)
4. ❌ **Manual tuning required**: Each screen hardcodes widths (Name is 30
   in Endpoints, 40 in Services, 50 in Pods)

**Example current config** (Pods screen, `internal/screens/screens.go:18-27`):
```go
{Field: "Namespace", Title: "Namespace", Width: 0, Priority: 2},  // Dynamic
{Field: "Name", Title: "Name", Width: 50, Priority: 1},           // Fixed
{Field: "Status", Title: "Status", Width: 15, Priority: 1},       // Fixed
{Field: "Age", Title: "Age", Width: 10, Priority: 1},             // Fixed
{Field: "Node", Title: "Node", Width: 0, Priority: 3},            // Dynamic
```

**Known bugs** (from `thoughts/shared/tickets/roadmap.md`):
- Columns with bad sizing on ConfigMaps
- Columns with bad sizing on DaemonSets
- Columns with bad sizing on CronJobs
- Columns with bad sizing on CRDs

## Desired End State

After this plan is complete:

1. ✅ **ColumnConfig has min/max/weight fields**: MinWidth, MaxWidth, Weight
2. ✅ **SetSize() uses weighted distribution**: Important columns get more
   space proportionally
3. ✅ **Convention-based defaults**: Common column types (Name, Namespace,
   Status, Age) have standard constants
4. ✅ **All 11+ screens migrated**: Pods, Deployments, Services, Nodes,
   ConfigMaps, Secrets, Namespaces, StatefulSets, DaemonSets, CronJobs, Jobs,
   and dynamic CRD screens
5. ✅ **No more manual tuning**: Widths determined by convention and terminal
   size

**Verification**:
- Run k1 on terminals of varying widths (80, 120, 200 columns)
- Name/Namespace columns stay within min/max bounds
- Important columns get more space than optional columns
- No visual artifacts or layout bugs
- Tests pass for weighted distribution algorithm

## What We're NOT Doing

- ❌ Content-based sizing (measuring actual cell lengths)
- ❌ User configuration via config files
- ❌ Column reordering features
- ❌ Switching to different table libraries (tview, bubble-table)
- ❌ Dynamic weight adjustment based on content

## Implementation Approach

**Strategy**: Extend current architecture with minimal breaking changes.

**Core principle**: Replace "Width: 0 = dynamic" with "Weight > 0 = grows
proportionally" + "MinWidth/MaxWidth = constraints".

**Why this approach**:
- Low risk (extends ColumnConfig, single file change)
- Reuses Priority-based hiding logic
- Incremental migration (screen-by-screen)
- No library changes (stays with bubbles/table)

## Phase 1: Core Algorithm Implementation

### Overview
Add MinWidth/MaxWidth/Weight fields to ColumnConfig and implement weighted
distribution algorithm in SetSize().

### Changes Required

#### 1. Update ColumnConfig Struct
**File**: `internal/screens/config.go:29-36`

**Current**:
```go
type ColumnConfig struct {
    Field    string
    Title    string
    Width    int                      // 0 = dynamic, >0 = fixed
    Format   func(interface{}) string
    Priority int                      // 1=critical, 2=important, 3=optional
}
```

**New**:
```go
type ColumnConfig struct {
    Field    string
    Title    string

    // Width fields (backward compatible):
    // NEW: Set MinWidth/MaxWidth/Weight for weighted distribution
    // OLD: Set Width for fixed width (backward compatible during migration)
    Width    int                      // DEPRECATED: 0 = dynamic, >0 = fixed
    MinWidth int                      // NEW: Minimum width (readability)
    MaxWidth int                      // NEW: Maximum width (prevent domination)
    Weight   float64                  // NEW: Growth weight (higher = more space)

    Format   func(interface{}) string
    Priority int                      // 1=critical, 2=important, 3=optional
}
```

**Backward compatibility strategy**:
- If MinWidth/MaxWidth/Weight are 0, fall back to old Width-based logic
- Allows incremental screen migration without breaking existing screens

#### 2. Add Common Column Constants
**File**: `internal/screens/constants.go` (create if doesn't exist)

```go
package screens

// Common column width constants
// Only defined for the most frequently used columns that appear across
// almost every screen (Name, Namespace, Age, Status)
const (
    // Name column (primary identifier)
    NameMinWidth = 20
    NameMaxWidth = 50
    NameWeight   = 3

    // Namespace column (critical context)
    NamespaceMinWidth = 10
    NamespaceMaxWidth = 30
    NamespaceWeight   = 2

    // Status column (state/phase)
    StatusMinWidth = 10
    StatusMaxWidth = 15
    StatusWeight   = 1

    // Age column (timestamp display)
    AgeMinWidth = 8
    AgeMaxWidth = 12
    AgeWeight   = 1
)
```

**Note**: Other column types (IP, Node, Labels, etc.) should use inline
values as they vary more per screen.

#### 3. Implement Weighted Distribution Algorithm
**File**: `internal/screens/config.go:318-401` (replace SetSize method)

**Algorithm steps**:
```go
func (s *ConfigScreen) SetSize(width, height int) {
    s.width = width
    s.height = height
    s.table.SetHeight(height)

    // 1. Determine visible columns (use existing Priority logic)
    visibleColumns := s.calculateVisibleColumns(width)
    s.visibleColumns = visibleColumns
    s.hiddenCount = len(s.config.Columns) - len(visibleColumns)

    // 2. Calculate widths using weighted distribution
    widths := s.calculateWeightedWidths(visibleColumns, width)

    // 3. Build table columns
    columns := make([]table.Column, len(visibleColumns))
    for i, col := range visibleColumns {
        columns[i] = table.Column{
            Title: col.Title,
            Width: widths[i],
        }
    }

    // 4. Update table
    s.table.SetRows([]table.Row{})
    s.table.SetColumns(columns)
    s.table.SetWidth(width)
    s.updateTable()
}
```

**New helper method** (add after SetSize):
```go
// calculateWeightedWidths implements weighted proportional distribution
// with min/max constraints. Returns final widths for each visible column.
func (s *ConfigScreen) calculateWeightedWidths(
    visible []ColumnConfig,
    terminalWidth int,
) []int {
    // 1. Check for backward compatibility (old Width-based configs)
    usesWeights := false
    for _, col := range visible {
        if col.Weight > 0 {
            usesWeights = true
            break
        }
    }

    // Fall back to old algorithm if no weights specified
    if !usesWeights {
        return s.calculateLegacyWidths(visible, terminalWidth)
    }

    // 2. Calculate padding
    padding := len(visible) * 2
    available := terminalWidth - padding
    if available <= 0 {
        // Fallback: equal distribution
        minWidths := make([]int, len(visible))
        for i := range visible {
            minWidths[i] = 10 // Emergency minimum
        }
        return minWidths
    }

    // 3. Allocate minimums
    totalMin := 0
    for _, col := range visible {
        totalMin += col.MinWidth
    }

    remaining := available - totalMin
    if remaining < 0 {
        // Not enough space for minimums, return minimums anyway
        widths := make([]int, len(visible))
        for i, col := range visible {
            widths[i] = col.MinWidth
        }
        return widths
    }

    // 4. Sum weights
    totalWeight := 0.0
    for _, col := range visible {
        totalWeight += col.Weight
    }

    // 5. Distribute remaining space by weight
    widths := make([]int, len(visible))
    for i, col := range visible {
        baseWidth := col.MinWidth

        // Calculate proportional extra space
        extraSpace := 0
        if totalWeight > 0 {
            extraSpace = int(float64(remaining) * (col.Weight / totalWeight))
        }

        finalWidth := baseWidth + extraSpace

        // 6. Apply MaxWidth cap
        if col.MaxWidth > 0 && finalWidth > col.MaxWidth {
            finalWidth = col.MaxWidth
        }

        widths[i] = finalWidth
    }

    return widths
}

// calculateLegacyWidths implements old Width-based algorithm for backward
// compatibility. DEPRECATED: Will be removed after all screens migrate.
func (s *ConfigScreen) calculateLegacyWidths(
    visible []ColumnConfig,
    terminalWidth int,
) []int {
    // Replicate old algorithm from SetSize() (lines 358-377)
    fixedTotal := 0
    dynamicCount := 0

    for _, col := range visible {
        if col.Width > 0 {
            fixedTotal += col.Width
        } else {
            dynamicCount++
        }
    }

    visiblePadding := len(visible) * 2
    dynamicWidth := 20 // Default minimum
    if dynamicCount > 0 {
        dynamicWidth = (terminalWidth - fixedTotal - visiblePadding) / dynamicCount
        if dynamicWidth < 20 {
            dynamicWidth = 20
        }
    }

    widths := make([]int, len(visible))
    for i, col := range visible {
        if col.Width > 0 {
            widths[i] = col.Width
        } else {
            widths[i] = dynamicWidth
        }
    }

    return widths
}

// calculateVisibleColumns determines which columns fit based on Priority.
// Reuses existing logic from SetSize() but extracted for clarity.
func (s *ConfigScreen) calculateVisibleColumns(
    terminalWidth int,
) []ColumnConfig {
    // Calculate padding
    padding := len(s.config.Columns) * 2
    availableWidth := terminalWidth - padding

    // Sort columns by priority (1 first, then 2, then 3)
    sorted := make([]ColumnConfig, len(s.config.Columns))
    copy(sorted, s.config.Columns)
    sort.SliceStable(sorted, func(i, j int) bool {
        return sorted[i].Priority < sorted[j].Priority
    })

    // Calculate which columns fit
    visibleColumns := []ColumnConfig{}
    usedWidth := 0

    for _, col := range sorted {
        // Use MinWidth if available, otherwise estimate
        estimatedWidth := col.MinWidth
        if estimatedWidth == 0 {
            if col.Width > 0 {
                estimatedWidth = col.Width
            } else {
                estimatedWidth = 20 // Legacy estimate for dynamic
            }
        }

        exclude := s.shouldExcludeColumn(col, availableWidth, usedWidth, estimatedWidth)
        if exclude {
            continue
        }

        visibleColumns = append(visibleColumns, col)
        usedWidth += estimatedWidth
    }

    // Restore original column order
    return s.restoreColumnOrder(visibleColumns)
}
```

**Update shouldExcludeColumn** to accept estimated width:
```go
// shouldExcludeColumn determines if a column should be hidden based on
// priority and available width. Now accepts estimatedWidth parameter.
func (s *ConfigScreen) shouldExcludeColumn(
    col ColumnConfig,
    availableWidth int,
    usedWidth int,
    estimatedWidth int,
) bool {
    // Priority 1 (critical) always shows, even if squished
    if col.Priority == 1 {
        return false
    }

    // Priority 2 and 3 only show if they fit
    return usedWidth+estimatedWidth > availableWidth
}
```

### Success Criteria

#### Automated Verification:
- [x] Code compiles: `go build ./...`
- [x] Tests pass: `make test`
- [x] Linting passes: `go vet ./...`

#### Manual Verification:
- [x] Create test screen with weighted columns
- [x] Run k1 on 80-column terminal, verify Name stays >= MinWidth
- [x] Run k1 on 200-column terminal, verify Name doesn't exceed MaxWidth
- [x] Run k1 on 120-column terminal, verify proportional distribution
- [x] Verify backward compatibility: old configs still render correctly

**Implementation Note**: After completing this phase and all automated
verification passes, pause here for manual confirmation from the human that
the manual testing was successful before proceeding to the next phase.

---

## Phase 2: Screen Migration - High Priority Screens

### Overview
Migrate the 4 most-used screens to weighted distribution: Pods, Deployments,
Services, Nodes.

### Changes Required

#### 1. Pods Screen
**File**: `internal/screens/screens.go:18-27`

**Current**:
```go
Columns: []ColumnConfig{
    {Field: "Namespace", Title: "Namespace", Width: 0, Priority: 2},
    {Field: "Name", Title: "Name", Width: 50, Priority: 1},
    {Field: "Ready", Title: "Ready", Width: 8, Priority: 1},
    {Field: "Status", Title: "Status", Width: 15, Priority: 1},
    {Field: "Restarts", Title: "Restarts", Width: 10, Priority: 1},
    {Field: "Age", Title: "Age", Width: 10, Format: FormatDuration, Priority: 1},
    {Field: "Node", Title: "Node", Width: 0, Priority: 3},
    {Field: "IP", Title: "IP", Width: 0, Priority: 3},
}
```

**New**:
```go
Columns: []ColumnConfig{
    {
        Field: "Namespace", Title: "Namespace",
        MinWidth: NamespaceMinWidth, MaxWidth: NamespaceMaxWidth, Weight: NamespaceWeight,
        Priority: 2,
    },
    {
        Field: "Name", Title: "Name",
        MinWidth: NameMinWidth, MaxWidth: NameMaxWidth, Weight: NameWeight,
        Priority: 1,
    },
    {
        Field: "Ready", Title: "Ready",
        MinWidth: 6, MaxWidth: 8, Weight: 1,
        Priority: 1,
    },
    {
        Field: "Status", Title: "Status",
        MinWidth: StatusMinWidth, MaxWidth: StatusMaxWidth, Weight: StatusWeight,
        Priority: 1,
    },
    {
        Field: "Restarts", Title: "Restarts",
        MinWidth: 8, MaxWidth: 10, Weight: 1,
        Priority: 1,
    },
    {
        Field: "Age", Title: "Age",
        MinWidth: AgeMinWidth, MaxWidth: AgeMaxWidth, Weight: AgeWeight,
        Format: FormatDuration,
        Priority: 1,
    },
    {
        Field: "Node", Title: "Node",
        MinWidth: 15, MaxWidth: 40, Weight: 2,
        Priority: 3,
    },
    {
        Field: "IP", Title: "IP",
        MinWidth: 12, MaxWidth: 40, Weight: 2,
        Priority: 3,
    },
}
```

#### 2. Deployments Screen
**File**: `internal/screens/screens.go` (find GetDeploymentsScreenConfig)

Apply similar transformation with inline values.

#### 3. Services Screen
**File**: `internal/screens/screens.go` (find GetServicesScreenConfig)

Apply similar transformation with inline values.

#### 4. Nodes Screen
**File**: `internal/screens/screens.go` (find GetNodesScreenConfig)

Apply similar transformation with inline values.

### Success Criteria

#### Automated Verification:
- [ ] Code compiles: `go build ./...`
- [ ] Tests pass: `make test`

#### Manual Verification:
- [ ] Run k1, navigate to Pods screen on 80-col terminal
- [ ] Verify Name column is readable (>= 20 chars)
- [ ] Run k1, navigate to Pods screen on 200-col terminal
- [ ] Verify Name column doesn't dominate (<= 50 chars)
- [ ] Run k1, navigate to Deployments/Services/Nodes screens
- [ ] Verify similar behavior across all 4 screens
- [ ] Resize terminal dynamically, verify responsive behavior

**Implementation Note**: After completing this phase and all automated
verification passes, pause here for manual confirmation from the human that
the manual testing was successful before proceeding to the next phase.

---

## Phase 3: Screen Migration - Known Buggy Screens

### Overview
Fix the 4 screens with known sizing bugs: ConfigMaps, DaemonSets, CronJobs,
CRDs.

### Changes Required

#### 1. ConfigMaps Screen
**File**: `internal/screens/screens.go` (find GetConfigMapsScreenConfig)

Migrate to weighted distribution.

#### 2. DaemonSets Screen
**File**: `internal/screens/screens.go` (find GetDaemonSetsScreenConfig)

Migrate to weighted distribution.

#### 3. CronJobs Screen
**File**: `internal/screens/screens.go` (find GetCronJobsScreenConfig)

Migrate to weighted distribution.

#### 4. CRDs Screen (Static)
**File**: `internal/screens/screens.go` (find GetCRDsScreenConfig)

Migrate to weighted distribution.

#### 5. Dynamic CRD Instance Screens
**File**: `internal/screens/dynamic_screens.go:10-130`

**Current logic** (lines 71-93): Generates columns with Width=0 or fixed widths.

**New logic**: Generate columns with MinWidth/MaxWidth/Weight based on column
type detection:

```go
// Example: In GetDynamicCRDScreenConfig, update column generation:
func inferColumnConfig(columnName string, fieldType string) ColumnConfig {
    // Heuristic-based inference
    switch {
    case columnName == "Name" || columnName == "NAME":
        return ColumnConfig{
            MinWidth: NameMinWidth,
            MaxWidth: NameMaxWidth,
            Weight:   NameWeight,
            Priority: 1,
        }
    case columnName == "Namespace" || columnName == "NAMESPACE":
        return ColumnConfig{
            MinWidth: NamespaceMinWidth,
            MaxWidth: NamespaceMaxWidth,
            Weight:   NamespaceWeight,
            Priority: 2,
        }
    case strings.Contains(strings.ToLower(columnName), "status"):
        return ColumnConfig{
            MinWidth: StatusMinWidth,
            MaxWidth: StatusMaxWidth,
            Weight:   StatusWeight,
            Priority: 1,
        }
    case strings.Contains(strings.ToLower(columnName), "age"):
        return ColumnConfig{
            MinWidth: AgeMinWidth,
            MaxWidth: AgeMaxWidth,
            Weight:   AgeWeight,
            Priority: 1,
        }
    default:
        // Generic column
        return ColumnConfig{
            MinWidth: 10,
            MaxWidth: 40,
            Weight:   2,
            Priority: 3,
        }
    }
}
```

### Success Criteria

#### Automated Verification:
- [ ] Code compiles: `go build ./...`
- [ ] Tests pass: `make test`

#### Manual Verification:
- [ ] Run k1, navigate to ConfigMaps screen
- [ ] Verify layout no longer buggy
- [ ] Repeat for DaemonSets, CronJobs, CRDs screens
- [ ] Create a test CRD, navigate to its instances screen
- [ ] Verify dynamic columns have reasonable sizing

**Implementation Note**: After completing this phase and all automated
verification passes, pause here for manual confirmation from the human that
the manual testing was successful before proceeding to the next phase.

---

## Phase 4: Screen Migration - Remaining Screens

### Overview
Migrate all remaining screens: Secrets, Namespaces, StatefulSets, Jobs,
ReplicaSets, PVCs, Ingresses, Endpoints, HPAs, Contexts.

### Changes Required

#### Batch Migration
For each screen in `internal/screens/screens.go`:
1. Identify current column configs
2. Apply weighted distribution using constants
3. Test screen individually

**Screens to migrate**:
- GetSecretsScreenConfig
- GetNamespacesScreenConfig
- GetStatefulSetsScreenConfig
- GetJobsScreenConfig
- GetReplicaSetsScreenConfig (if exists)
- GetPVCsScreenConfig (if exists)
- GetIngressesScreenConfig (if exists)
- GetEndpointsScreenConfig
- GetHPAsScreenConfig (if exists)
- GetContextsScreenConfig

### Success Criteria

#### Automated Verification:
- [ ] Code compiles: `go build ./...`
- [ ] Tests pass: `make test`

#### Manual Verification:
- [ ] Navigate to each screen individually
- [ ] Verify layout looks reasonable on 120-col terminal
- [ ] Spot-check 3 screens on 80-col terminal
- [ ] Spot-check 3 screens on 200-col terminal

**Implementation Note**: After completing this phase and all automated
verification passes, pause here for manual confirmation from the human that
the manual testing was successful before proceeding to the next phase.

---

## Phase 5: Cleanup and Documentation

### Overview
Remove deprecated Width-based logic and document new convention.

### Changes Required

#### 1. Remove Legacy Code
**File**: `internal/screens/config.go`

Remove `calculateLegacyWidths()` method after verifying all screens migrated.

#### 2. Update ColumnConfig Documentation
**File**: `internal/screens/config.go:29-36`

```go
// ColumnConfig defines a column in the resource list table.
//
// Column Sizing:
//   - Use MinWidth/MaxWidth/Weight for responsive sizing (recommended)
//   - MinWidth: Minimum readable width (enforced even on narrow terminals)
//   - MaxWidth: Maximum width (prevents domination on wide terminals)
//   - Weight: Proportional growth factor (higher = more space)
//
// Common patterns (see constants.go for values):
//   - Name columns: Weight=3 (gets 3x space of Weight=1 columns)
//   - Namespace columns: Weight=2
//   - Status/Age columns: Weight=1
//
// Priority (for column hiding on narrow terminals):
//   - Priority=1: Critical (never hidden)
//   - Priority=2: Important (hidden if no space)
//   - Priority=3: Optional (first to hide)
type ColumnConfig struct {
    Field    string
    Title    string
    MinWidth int                      // Minimum width (readability)
    MaxWidth int                      // Maximum width (prevent domination)
    Weight   float64                  // Growth weight (higher = more space)
    Format   func(interface{}) string
    Priority int                      // 1=critical, 2=important, 3=optional
}
```

#### 3. Update CLAUDE.md
**File**: `CLAUDE.md`

Add section on column sizing conventions:

```markdown
### Column Sizing: Weighted Proportional Distribution

k1 uses **weighted proportional distribution** for responsive column sizing.

**Key files**:
- `internal/screens/config.go` - Weighted distribution algorithm
- `internal/screens/constants.go` - Constants for common columns (Name, Namespace, Status, Age)

**Column configuration pattern**:
```go
{
    Field: "Name", Title: "Name",
    MinWidth: NameMinWidth,   // 20 - use constant for common columns
    MaxWidth: NameMaxWidth,   // 50
    Weight:   NameWeight,     // 3 - Gets 3x space of Weight=1 columns
    Priority: 1,              // Critical (never hidden)
}
```

**Constants available** (for most common columns):
- `NameMinWidth/MaxWidth/Weight` - Name columns (Weight=3)
- `NamespaceMinWidth/MaxWidth/Weight` - Namespace columns (Weight=2)
- `StatusMinWidth/MaxWidth/Weight` - Status columns (Weight=1)
- `AgeMinWidth/MaxWidth/Weight` - Age columns (Weight=1)

**Other columns** (use inline values):
- **IP/Node/Labels**: Specify inline as they vary per screen
- Example: `MinWidth: 15, MaxWidth: 40, Weight: 2`

**Weight semantics**:
- Weight=3: Gets 3x space of Weight=1 columns
- Weight=2: Gets 2x space of Weight=1 columns
- Weight=1: Base allocation

**Example**: Terminal width=120, after minimums, remaining space=60
- Name (Weight=3): Gets 60 * (3/6) = 30 extra chars
- Namespace (Weight=2): Gets 60 * (2/6) = 20 extra chars
- Status (Weight=1): Gets 60 * (1/6) = 10 extra chars
```

### Success Criteria

#### Automated Verification:
- [ ] Code compiles after removing legacy code: `go build ./...`
- [ ] Tests pass: `make test`
- [ ] No linting warnings: `go vet ./...`

#### Manual Verification:
- [ ] Documentation is clear and accurate
- [ ] Run k1 on all screens, verify no regressions
- [ ] No visual artifacts or layout bugs

**Implementation Note**: This is the final phase. After all automated
verification passes and manual testing confirms success, the refactor is
complete.

---

## Testing Strategy

### Unit Tests

**File**: `internal/screens/config_test.go`

Add tests for weighted distribution algorithm:

```go
func TestCalculateWeightedWidths(t *testing.T) {
    tests := []struct {
        name          string
        terminalWidth int
        columns       []ColumnConfig
        expected      []int
    }{
        {
            name:          "Basic weighted distribution",
            terminalWidth: 100,
            columns: []ColumnConfig{
                {MinWidth: 10, MaxWidth: 50, Weight: 3, Priority: 1}, // Name
                {MinWidth: 10, MaxWidth: 30, Weight: 2, Priority: 2}, // Namespace
                {MinWidth: 10, MaxWidth: 15, Weight: 1, Priority: 1}, // Status
            },
            // Padding: 3 * 2 = 6
            // Available: 100 - 6 = 94
            // Minimums: 10+10+10 = 30
            // Remaining: 94 - 30 = 64
            // Total weight: 3+2+1 = 6
            // Name: 10 + 64*(3/6) = 10 + 32 = 42
            // Namespace: 10 + 64*(2/6) = 10 + 21 = 31 (capped at 30)
            // Status: 10 + 64*(1/6) = 10 + 10 = 20 (capped at 15)
            expected: []int{42, 30, 15},
        },
        {
            name:          "Insufficient space for minimums",
            terminalWidth: 40,
            columns: []ColumnConfig{
                {MinWidth: 20, MaxWidth: 50, Weight: 3, Priority: 1},
                {MinWidth: 20, MaxWidth: 30, Weight: 2, Priority: 2},
            },
            // Returns minimums
            expected: []int{20, 20},
        },
        {
            name:          "MaxWidth capping",
            terminalWidth: 300,
            columns: []ColumnConfig{
                {MinWidth: 10, MaxWidth: 50, Weight: 3, Priority: 1},
            },
            // Should cap at MaxWidth=50
            expected: []int{50},
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            screen := &ConfigScreen{
                config: ScreenConfig{Columns: tt.columns},
            }
            result := screen.calculateWeightedWidths(tt.columns, tt.terminalWidth)
            assert.Equal(t, tt.expected, result)
        })
    }
}
```

### Integration Tests

**Manual testing checklist**:
1. Run k1 on 80-column terminal, navigate to all screens
2. Run k1 on 120-column terminal, navigate to all screens
3. Run k1 on 200-column terminal, navigate to all screens
4. Resize terminal dynamically while viewing Pods screen
5. Test dynamic CRD screen with custom CRD

## Performance Considerations

**Computational complexity**: O(n) where n = number of visible columns

**Impact**: Negligible (called only on terminal resize, not per frame)

**Memory**: No additional allocations (reuses existing slice patterns)

## Migration Notes

**Backward compatibility**: Old Width-based configs supported during
migration via `calculateLegacyWidths()` fallback.

**Migration order**:
1. Phase 1: Core algorithm (no visual changes yet)
2. Phase 2: High-traffic screens (Pods, Deployments, Services, Nodes)
3. Phase 3: Known buggy screens (immediate value)
4. Phase 4: Remaining screens (complete migration)
5. Phase 5: Remove legacy code

**Rollback strategy**: Revert Phase 1 commit if algorithm has critical bugs.

## References

**Research document**:
- `thoughts/shared/research/2025-11-01-column-self-adjusting-strategy.md`

**Prior research**:
- `thoughts/shared/research/2025-10-26-column-display-smaller-windows.md`

**Similar patterns**:
- CSS flexbox `flex-grow` property (inspiration for Weight field)
- bubble-table library (flex-grow API design)

**Current implementation**:
- `internal/screens/config.go:318-401` - SetSize() algorithm
- `internal/screens/screens.go` - Screen configs

## TODO Tracking

### Phase 1: Core Algorithm
- [x] Add MinWidth/MaxWidth/Weight fields to ColumnConfig
- [x] Create constants.go with Name/Namespace/Status/Age constants
- [x] Implement calculateWeightedWidths() method
- [x] Implement calculateVisibleColumns() extracted method
- [x] Update shouldExcludeColumn() signature
- [x] Add calculateLegacyWidths() for backward compatibility
- [x] Test compilation
- [x] Test backward compatibility with old configs

### Phase 2: High Priority Screens
- [x] Migrate Pods screen config
- [x] Migrate Deployments screen config
- [x] Migrate Services screen config
- [x] Migrate Nodes screen config
- [ ] Manual testing on 80/120/200-col terminals

### Phase 3: Known Buggy Screens
- [ ] Migrate ConfigMaps screen config
- [ ] Migrate DaemonSets screen config
- [ ] Migrate CronJobs screen config
- [ ] Migrate CRDs screen config
- [ ] Add inferColumnConfig() for dynamic CRD screens
- [ ] Update dynamic_screens.go column generation
- [ ] Manual testing with test CRD

### Phase 4: Remaining Screens
- [ ] Migrate Secrets screen
- [ ] Migrate Namespaces screen
- [ ] Migrate StatefulSets screen
- [ ] Migrate Jobs screen
- [ ] Migrate remaining screens (ReplicaSets, PVCs, Ingresses, etc.)
- [ ] Manual testing spot-check

### Phase 5: Cleanup
- [ ] Remove calculateLegacyWidths() method
- [ ] Remove Width field from ColumnConfig (optional deprecation)
- [ ] Update ColumnConfig documentation
- [ ] Update CLAUDE.md with conventions
- [ ] Final manual testing pass

---

**Estimated effort**: 1-2 days
- Phase 1: 4-6 hours (core algorithm + testing)
- Phase 2: 2-3 hours (4 screens + testing)
- Phase 3: 2-3 hours (5 screens + testing)
- Phase 4: 2-3 hours (10+ screens + testing)
- Phase 5: 1-2 hours (cleanup + documentation)
