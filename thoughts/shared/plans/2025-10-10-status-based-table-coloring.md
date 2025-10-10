---
date: 2025-10-10
status: complete
phase: phase_2
ticket: N/A
---

# Status-Based Table Row Coloring Implementation Plan

## ✅ COMPLETE - Row-Level Coloring via View-Time Parsing

**Status**: Phase 1 complete - Pods screen has status-based row coloring

**Solution**: Parse status from rendered output and apply colors at view time
- Intercept table.View() output for pods screen
- Parse status strings from each rendered line using strings.Contains()
- Apply lipgloss color styles to entire rows based on status
- Simple, robust, works with scrolling/filtering automatically

**Why This Works**:
- No complex mappings between data and visible rows
- Status text is always present in rendered output
- Colors applied fresh on every render (scroll/filter updates work)
- Immune to bubbles table internal rendering changes

**Key Learning**: When working with rendered output, parse what you see
rather than maintaining complex data-to-view mappings. See CLAUDE.md
"Lessons Learned" section.

**Files Modified**:
- `internal/screens/formatters.go` - Theme-aware formatter infrastructure
- `internal/screens/config.go` - renderColoredTable() parses rendered lines
- `internal/screens/screens.go` - Screen configs accept theme parameter
- `internal/app/app.go` - Passes theme to all screen configs
- `CLAUDE.md` - Added "View-Layer Rendering" lesson learned

## Overview

Add status-based foreground coloring to table rows across all resource
screens in k1. Resources in error states (Failed, CrashLoopBackOff) will
appear in red, warning states (Pending, Unknown) in yellow/orange, and
healthy states (Running, Bound) in normal color. This improves visual
scanning of cluster health by making problems immediately visible.

## Current State Analysis

**Config-Driven Architecture:**
- Single ConfigScreen implementation handles all 16 resource types
  (internal/screens/config.go)
- Columns configured per screen with Field, Title, Width, Format
- Table rows built in updateTable() by converting resource fields to
  strings (lines 482-510)

**Existing Formatter Pattern:**
- Current signature: `Format func(interface{}) string`
- Example: FormatDuration (internal/screens/config.go:618)
- Used in 16 screen configs for Age field
- No access to theme for styling

**Theme System:**
- All 8 themes define status styles (internal/ui/theme.go:40-42):
  - StatusRunning (green/success color)
  - StatusError (red)
  - StatusWarning (yellow/orange)
- Styles exist but are currently unused
- Colors vary per theme but semantics are consistent

**Resources with Status Fields:**
- 11 of 16 resources have explicit status (internal/k8s/repository_types.go)
- Pod: Status field + Ready fraction (lines 30-37)
- Namespace: Status field (lines 70-73)
- Node: Status field (lines 108-118)
- PVC: Status field (lines 129-136)
- Deployment/StatefulSet/DaemonSet/ReplicaSet: Ready vs Desired counts
- Job: Completions fraction
- CronJob: Suspend boolean

**Key Constraint:**
- Bubbles table only accepts string rows ([][]string)
- Styling must be embedded as ANSI codes via lipgloss.Render()
- No per-cell style objects supported

## Desired End State

**For Phase 1 (Pods only):**
- Pod Status field colored based on state:
  - Red: Failed, CrashLoopBackOff, ImagePullBackOff, Error
  - Yellow: Pending, Unknown, ContainerCreating
  - Normal: Running, Succeeded
- Pod Ready field colored based on readiness:
  - Red: 0/n (completely not ready)
  - Yellow: x/n where x < n (partially ready)
  - Normal: n/n (fully ready)

**Verification:**
- Run k1 with live cluster or dummy data
- Navigate to Pods screen
- Verify failed pods appear in red
- Verify pending pods appear in yellow
- Verify running pods with 0/1 ready appear in yellow
- Verify running pods with 1/1 ready appear normal
- Test with all 8 themes (colors differ but semantics consistent)

## What We're NOT Doing

- **Not implementing all 11 resources in Phase 1** - only Pods to prove
  concept
- **Not changing table-wide styles** - only per-cell foreground colors
- **Not adding background colors** - only foreground for readability
- **Not adding symbols** (✓/⚠/✗) - color only for now
- **Not implementing whole-row coloring** - only specific status columns
- **Not changing repository or transform logic** - status extraction
  already works
- **Not creating new theme colors** - using existing
  StatusRunning/Error/Warning

## Implementation Approach

**Closure-based formatter pattern:**
- Change formatters to factory functions that return formatters
- Factory captures theme in closure, returned formatter has theme access
- Non-styled formatters (FormatDuration) remain unchanged
- Styled formatters become factories that take theme parameter

**Pattern:**
```go
// Factory function (captures theme)
func FormatPodStatus(theme *ui.Theme) func(interface{}) string {
    return func(val interface{}) string {
        status := fmt.Sprint(val)
        switch status {
        case "Failed": return theme.Table.StatusError.Render(status)
        case "Pending": return theme.Table.StatusWarning.Render(status)
        default: return status
        }
    }
}

// Screen config calls factory
func GetPodsScreenConfig(theme *ui.Theme) ScreenConfig {
    return ScreenConfig{
        Columns: []ColumnConfig{
            {Field: "Status", Format: FormatPodStatus(theme)},
            {Field: "Age", Format: FormatDuration},  // No theme needed
        },
    }
}
```

## Phase 1: Pod Status and Ready Field Coloring

### Overview
Implement status coloring for Pods screen only. This proves the pattern
and can be extended to other resources in future phases.

### Changes Required

#### 1. Update Screen Config Signatures (Pass Theme)

**File**: `internal/screens/screens.go`

**Change all 16 `Get*ScreenConfig()` functions** to accept theme:

```go
// Before:
func GetPodsScreenConfig() ScreenConfig

// After:
func GetPodsScreenConfig(theme *ui.Theme) ScreenConfig
```

**Rationale**: Formatters need theme access. Factory pattern requires
theme at config creation time.

**Files affected**: All 16 screen config functions (lines 11-470)

#### 2. Create Status Formatter Factory

**File**: `internal/screens/formatters.go` (new file)

**Create formatter factories for Pod status coloring**:

```go
package screens

import (
    "fmt"
    "github.com/renato0307/k1/internal/ui"
)

// FormatPodStatus creates a formatter that colors Pod status values
// based on their semantic meaning.
func FormatPodStatus(theme *ui.Theme) func(interface{}) string {
    return func(val interface{}) string {
        status := fmt.Sprint(val)

        switch status {
        // Error states - red
        case "Failed", "CrashLoopBackOff", "ImagePullBackOff",
             "ErrImagePull", "Error":
            return theme.Table.StatusError.Render(status)

        // Warning states - yellow/orange
        case "Pending", "Unknown", "ContainerCreating":
            return theme.Table.StatusWarning.Render(status)

        // Normal states - no styling
        case "Running", "Succeeded":
            return status

        default:
            return status  // Unknown status - no styling
        }
    }
}

// FormatPodReady creates a formatter that colors Pod Ready fractions
// based on readiness state.
func FormatPodReady(theme *ui.Theme) func(interface{}) string {
    return func(val interface{}) string {
        ready := fmt.Sprint(val)

        // Parse "x/n" format
        var current, desired int
        _, err := fmt.Sscanf(ready, "%d/%d", &current, &desired)
        if err != nil {
            return ready  // Can't parse - no styling
        }

        if current == 0 && desired > 0 {
            // Completely not ready - red
            return theme.Table.StatusError.Render(ready)
        } else if current < desired {
            // Partially ready - yellow
            return theme.Table.StatusWarning.Render(ready)
        } else {
            // Fully ready - normal
            return ready
        }
    }
}
```

**Rationale**:
- Separate file keeps formatters organized
- Factory pattern allows theme capture in closure
- Switch statement makes status mappings explicit
- Parsing Ready fraction enables numeric comparison

#### 3. Update Pods Screen Config

**File**: `internal/screens/screens.go`

**Update GetPodsScreenConfig** to use new formatters:

```go
func GetPodsScreenConfig(theme *ui.Theme) ScreenConfig {
    return ScreenConfig{
        ID:           "pods",
        Title:        "Pods",
        ResourceType: k8s.ResourceTypePod,
        Columns: []ColumnConfig{
            {Field: "Namespace", Title: "Namespace", Width: 40},
            {Field: "Name", Title: "Name", Width: 0},
            {Field: "Ready", Title: "Ready", Width: 8,
             Format: FormatPodReady(theme)},  // ← Add styling
            {Field: "Status", Title: "Status", Width: 15,
             Format: FormatPodStatus(theme)},  // ← Add styling
            {Field: "Restarts", Title: "Restarts", Width: 10},
            {Field: "Age", Title: "Age", Width: 10, Format: FormatDuration},
            {Field: "Node", Title: "Node", Width: 30},
            {Field: "IP", Title: "IP", Width: 16},
        },
        // ... rest of config unchanged
    }
}
```

**Rationale**: Calls formatter factories with theme to get closures.

#### 4. Update Screen Creation in App

**File**: `internal/app/app.go`

**Update all screen initialization calls** to pass theme:

```go
// Before:
screens := map[string]types.Screen{
    "pods": NewConfigScreen(
        screens.GetPodsScreenConfig(),  // ← Missing theme
        repo, theme),
}

// After:
screens := map[string]types.Screen{
    "pods": NewConfigScreen(
        screens.GetPodsScreenConfig(theme),  // ← Pass theme
        repo, theme),
}
```

**Update all 16 screen registrations** in NewModel().

**Rationale**: Config functions now require theme parameter.

### Success Criteria

#### Automated Verification:
- [x] Code compiles: `go build ./...`
- [x] All unit tests pass: `make test`
- [x] Type checking passes: `go vet ./...`

#### Manual Verification:
- [x] Run k1 with dummy data: `./k1 -dummy`
- [x] Navigate to Pods screen (should be default)
- [x] Verify pods with "Succeeded" status appear in normal color (teal/green)
- [x] Verify pods with "Failed" status appear in red
- [x] Verify pods with "Pending" status appear in yellow
- [x] Scroll through pods list and verify colors update correctly
- [x] Test with live cluster (if available): `./k1`
- [x] Verify colors are readable and distinct in your terminal
- [x] Verify other screens still work (no regressions)

**Phase 1 Complete**: All automated and manual tests passed.

---

## ✅ Phase 2: Extend to All Resources with Status Fields

**Status**: Phase 2 complete - All 10 resources now have status-based row coloring

### Overview
Applied row-level coloring pattern to all remaining resources with status fields.

### Implementation Summary

Extended `renderColoredTable()` to support all resource types by:
1. Adding screen IDs to the switch case (namespaces, nodes, pvcs, deployments, statefulsets, daemonsets, replicasets, jobs, cronjobs)
2. Created three matcher functions to categorize states:
   - `matchesErrorState()` - Critical failures (red)
   - `matchesWarningState()` - Degraded or transitional states (yellow)
   - `matchesSuccessState()` - Healthy states (green)
3. Added comprehensive test coverage for all matchers

### Changes Required

#### 1. Create Additional Formatter Factories

**File**: `internal/screens/formatters.go`

**Add formatters for each resource type**:

```go
// FormatNamespaceStatus - Active (normal), Terminating (red)
func FormatNamespaceStatus(theme *ui.Theme) func(interface{}) string

// FormatNodeStatus - Ready (normal), NotReady (red), Unknown (yellow)
func FormatNodeStatus(theme *ui.Theme) func(interface{}) string

// FormatPVCStatus - Bound (normal), Pending (yellow), Lost (red)
func FormatPVCStatus(theme *ui.Theme) func(interface{}) string

// FormatDeploymentReady - Parses "x/n", colors based on readiness
func FormatDeploymentReady(theme *ui.Theme) func(interface{}) string

// FormatStatefulSetReady - Same as deployment
func FormatStatefulSetReady(theme *ui.Theme) func(interface{}) string

// FormatDaemonSetReady - Compares Ready vs Desired int32 fields
// Note: Requires passing entire resource, not just field value
// May need to revisit formatter signature for this case
func FormatDaemonSetReady(theme *ui.Theme) func(interface{}) string

// FormatReplicaSetReady - Same as DaemonSet
func FormatReplicaSetReady(theme *ui.Theme) func(interface{}) string

// FormatJobCompletions - Parses "x/n" for completions
func FormatJobCompletions(theme *ui.Theme) func(interface{}) string

// FormatCronJobSuspend - Shows "Suspended" in yellow if true
func FormatCronJobSuspend(theme *ui.Theme) func(interface{}) string
```

**Status mappings by resource**:

- **Namespace**:
  - Normal: Active
  - Error: Terminating

- **Node**:
  - Normal: Ready
  - Warning: Unknown
  - Error: NotReady

- **PVC**:
  - Normal: Bound
  - Warning: Pending
  - Error: Lost

- **Deployment/StatefulSet** (Ready field "x/n"):
  - Error: 0/n (completely down)
  - Warning: x/n where x < n (degraded)
  - Normal: n/n (fully available)

- **DaemonSet/ReplicaSet** (separate Ready/Desired fields):
  - Error: Ready=0, Desired>0
  - Warning: Ready<Desired
  - Normal: Ready=Desired

- **Job** (Completions field "x/n"):
  - Error: 0/n after long time (stuck)
  - Warning: x/n (in progress)
  - Normal: n/n (complete)

- **CronJob** (Suspend field):
  - Warning: Suspend=true
  - Normal: Suspend=false

#### 2. Update All Screen Configs

**File**: `internal/screens/screens.go`

**Update each screen config** to use appropriate formatters:

- GetNamespacesScreenConfig: Add FormatNamespaceStatus
- GetNodesScreenConfig: Add FormatNodeStatus
- GetPersistentVolumeClaimsScreenConfig: Add FormatPVCStatus
- GetDeploymentsScreenConfig: Add FormatDeploymentReady
- GetStatefulSetsScreenConfig: Add FormatStatefulSetReady
- GetDaemonSetsScreenConfig: Add FormatDaemonSetReady
- GetReplicaSetsScreenConfig: Add FormatReplicaSetReady
- GetJobsScreenConfig: Add FormatJobCompletions
- GetCronJobsScreenConfig: Add FormatCronJobSuspend

**Challenge**: DaemonSet and ReplicaSet have separate Ready and Desired
fields. Current formatter receives single field value. May need one of:
- Format only Ready field (shows number, colors if 0)
- Create composite field in transform (combine Ready/Desired to "x/n"
  string)
- Extend formatter to receive entire resource (bigger change)

**Recommended**: Create composite "Status" or "Ready" field in transform
layer that combines Ready/Desired into "x/n" string, then use standard
fraction formatter.

### Success Criteria

#### Automated Verification:
- [x] Code compiles: `go build ./...`
- [x] All unit tests pass: `go test ./internal/screens/...`
- [x] Type checking passes: `go vet ./...`
- [x] Matcher functions tested: All three matcher functions have comprehensive unit tests

#### Manual Verification:
- [ ] Test Namespaces: Terminating namespace appears in red
- [ ] Test Nodes: NotReady node appears in red, Unknown in yellow
- [ ] Test PVCs: Pending PVC appears in yellow (if available in test data)
- [ ] Test Deployments: Degraded deployment (2/3) appears in yellow
- [ ] Test StatefulSets: Fully ready (3/3) appears normal (if available)
- [ ] Test DaemonSets: Partially ready appears in yellow (if available)
- [ ] Test ReplicaSets: Not ready (0/3) appears in red (if available)
- [ ] Test Jobs: Incomplete job (1/5) appears in yellow (if available)
- [ ] Test CronJobs: Suspended cron job shows yellow indicator (if available)
- [ ] Test with different themes (verify colors change appropriately)
- [ ] Verify no regressions in other screens

---

## Testing Strategy

### Unit Tests

**File**: `internal/screens/formatters_test.go` (new file)

**Test each formatter with various inputs**:

```go
func TestFormatPodStatus(t *testing.T) {
    theme := ui.ThemeCharm()
    format := FormatPodStatus(theme)

    tests := []struct {
        name     string
        input    interface{}
        contains string  // Check if output contains input text
        hasColor bool    // Check if ANSI codes present
    }{
        {"running", "Running", "Running", false},  // No styling
        {"failed", "Failed", "Failed", true},      // Has ANSI codes
        {"pending", "Pending", "Pending", true},
        {"succeeded", "Succeeded", "Succeeded", false},
        {"crashloop", "CrashLoopBackOff", "CrashLoopBackOff", true},
        {"unknown", "CustomStatus", "CustomStatus", false},  // No styling
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := format(tt.input)
            assert.Contains(t, result, tt.contains)

            if tt.hasColor {
                // ANSI codes present (lipgloss adds escape sequences)
                assert.Contains(t, result, "\x1b[")
            } else {
                // No ANSI codes
                assert.NotContains(t, result, "\x1b[")
            }
        })
    }
}

func TestFormatPodReady(t *testing.T) {
    theme := ui.ThemeCharm()
    format := FormatPodReady(theme)

    tests := []struct {
        name     string
        input    interface{}
        hasColor bool
    }{
        {"fully_ready", "1/1", false},     // Normal
        {"partial", "2/3", true},          // Warning (yellow)
        {"not_ready", "0/1", true},        // Error (red)
        {"all_ready", "5/5", false},       // Normal
        {"invalid", "invalid", false},     // Can't parse, no styling
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := format(tt.input)

            if tt.hasColor {
                assert.Contains(t, result, "\x1b[")
            } else {
                assert.NotContains(t, result, "\x1b[")
            }
        })
    }
}
```

**Test coverage targets**:
- FormatPodStatus: All status values + unknown
- FormatPodReady: All readiness states + invalid format
- Each additional formatter in Phase 2
- Verify ANSI escape codes present/absent as expected

### Integration Tests

**File**: `internal/screens/config_test.go`

**Test table rendering with styled cells**:

```go
func TestConfigScreen_StatusColoring(t *testing.T) {
    theme := ui.ThemeCharm()
    config := GetPodsScreenConfig(theme)

    // Mock repository with diverse pod states
    repo := &mockRepository{
        pods: []k8s.Pod{
            {Status: "Running", Ready: "1/1"},     // Normal
            {Status: "Failed", Ready: "0/1"},      // Red
            {Status: "Pending", Ready: "0/1"},     // Yellow
            {Status: "Running", Ready: "2/3"},     // Partial (yellow)
        },
    }

    screen := NewConfigScreen(config, repo, theme)

    // Trigger table update
    screen.updateTable()

    // Verify styled strings in table rows
    rows := screen.table.Rows()

    // Row 0: Running 1/1 - no styling
    assert.NotContains(t, rows[0][statusCol], "\x1b[")

    // Row 1: Failed - has red styling
    assert.Contains(t, rows[1][statusCol], "\x1b[")

    // Row 2: Pending - has yellow styling
    assert.Contains(t, rows[2][statusCol], "\x1b[")

    // Row 3: Ready 2/3 - has yellow styling
    assert.Contains(t, rows[3][readyCol], "\x1b[")
}
```

### Manual Testing Checklist

**Test with dummy data**:
1. Run `make run-dummy`
2. Navigate to Pods screen
3. Verify diverse pod states have correct colors
4. Switch themes with `:theme <name>` (if implemented)
5. Verify colors change but semantics remain

**Test with live cluster** (if available):
1. Connect to cluster with diverse pod states
2. Run `make run`
3. Verify real-world pods colored correctly
4. Check Pending pods (scale deployment to high replicas)
5. Check Failed pods (deploy bad image)
6. Check CrashLoopBackOff (deploy crashing container)

**Accessibility testing**:
1. Test with light terminal background
2. Test with dark terminal background
3. Check if colors are distinguishable
4. Consider color-blind users (future: add symbols)

## Performance Considerations

**Lipgloss Render Cost**:
- `lipgloss.Render()` adds ANSI escape codes to strings
- Called once per styled cell during table update
- Typical cluster: 100 pods × 2 styled columns = 200 calls per refresh
- Expected impact: <1ms for typical cluster

**Benchmarking**:
- If performance concerns arise, benchmark `updateTable()` with 1000+ pods
- Profile with `go test -bench=. -cpuprofile=cpu.out`
- Optimize by caching styled strings if needed (unlikely)

**Memory**:
- ANSI codes add ~10-20 bytes per styled cell
- Negligible for typical clusters (<1KB total overhead)

## Migration Notes

**Breaking Changes**:
- All `Get*ScreenConfig()` signatures changed (internal API)
- Callers in `internal/app/app.go` must be updated
- External callers (if any) must pass theme parameter

**Backwards Compatibility**:
- No changes to repository, types, or Kubernetes integration
- Existing formatters (FormatDuration) work unchanged
- Screen behavior unchanged except for visual styling

**Future Extensions**:
- Symbol indicators (✓/⚠/✗) for accessibility
- Whole-row coloring based on overall health
- Configurable color schemes per user preference
- Status indicators for resources without explicit status (Services,
  ConfigMaps)

## References

- Original research:
  `thoughts/shared/research/2025-10-10-status-based-table-coloring.md`
- Config-driven architecture:
  `internal/screens/config.go`
- Theme system:
  `internal/ui/theme.go`
- Resource types:
  `internal/k8s/repository_types.go`
- Existing formatter pattern:
  `internal/screens/config.go:618` (FormatDuration)
