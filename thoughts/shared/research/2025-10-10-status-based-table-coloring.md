---
date: 2025-10-10T06:00:14Z
researcher: Claude
git_commit: d65e077fff5226a9d63ba2a3c9de1228bf966c19
branch: docs/designs
repository: renato0307/k1
topic: "Status-Based Table Row Coloring Across All Screens"
tags: [research, codebase, ui, theme, table, status, screens, conditional-styling]
status: complete
last_updated: 2025-10-10
last_updated_by: Claude
---

# Research: Status-Based Table Row Coloring Across All Screens

**Date**: 2025-10-10T06:00:14Z
**Researcher**: Claude
**Git Commit**: d65e077fff5226a9d63ba2a3c9de1228bf966c19
**Branch**: docs/designs
**Repository**: renato0307/k1

## Research Question

How can we implement status-based foreground coloring for table rows across
all screens (pods, deployments, services, etc.) where resource statuses like
"Running", "Pending", "CrashLoopBackOff", "Failed", etc. have different
colors to highlight their state? Colors must align with the active theme, and
expected final states (Running, Bound, Ready, etc.) should maintain normal
color.

## Summary

k1 uses a config-driven screen architecture where all 16 resource screens
share a single ConfigScreen implementation that renders tables using the
Charmbracelet Bubbles table component. The theme system already defines
status styles (`StatusRunning`, `StatusError`, `StatusWarning`) across all 8
themes, but these are currently unused.

**Key Findings**:
- 11 of 16 resource types have explicit status fields (Pod, Namespace, Node,
  PVC, etc.)
- Table rows are built in `updateTable()` by converting resource fields to
  strings
- Conditional styling must be applied by embedding ANSI codes via lipgloss
  before setting rows
- Theme status styles exist but aren't wired to table rendering
- Column formatters are the perfect hook for status-based styling

**Implementation Path**: Extend column formatters to accept theme reference
and return styled strings using `theme.Table.StatusRunning/Error/Warning`
based on field value semantic meaning.

## Detailed Findings

### 1. Screen Implementations and Resource Types

**All 16 screens use ConfigScreen** (internal/screens/config.go):
- Single implementation handles all resource types via config-driven pattern
- Screens defined in internal/screens/screens.go (16 `Get*ScreenConfig()`
  functions)
- Table columns configured per screen with Field, Title, Width, Format

**Resources with Status Fields** (internal/k8s/repository_types.go):

1. **Pod** (lines 30-37):
   - `Status string` - Values: "Running", "Pending", "Failed", "Succeeded",
     "Unknown", "CrashLoopBackOff", "ImagePullBackOff"
   - `Ready string` - Fraction: "1/1", "2/3", "0/2"
   - `Restarts int32` - Could indicate problems if high

2. **Namespace** (lines 70-73):
   - `Status string` - Values: "Active", "Terminating"

3. **Node** (lines 108-118):
   - `Status string` - Values: "Ready", "NotReady", "Unknown"

4. **PersistentVolumeClaim** (lines 129-136):
   - `Status string` - Values: "Bound", "Pending", "Lost"

5. **Deployment** (lines 40-45):
   - `Ready string` - Fraction: "3/3", "2/5" (implicit status)

6. **StatefulSet** (lines 79-82):
   - `Ready string` - Fraction: "2/2", "1/3" (implicit status)

7. **DaemonSet** (lines 85-91):
   - `Ready int32`, `Desired int32` - Numeric status indicators

8. **ReplicaSet** (lines 139-143):
   - `Ready int32`, `Desired int32`, `Current int32` - Numeric indicators

9. **Job** (lines 94-97):
   - `Completions string` - Fraction: "1/1", "0/5"

10. **CronJob** (lines 100-105):
    - `Suspend bool` - Binary status indicator

11. **HorizontalPodAutoscaler** (lines 158-164):
    - `Replicas int32` vs `MinPods/MaxPods` - Implicit health

**Resources without explicit status**: Service, ConfigMap, Secret, Ingress,
Endpoints (no status fields in Kubernetes API)

### 2. Theme System and Available Colors

**Theme structure** (internal/ui/theme.go:8-43):

All 8 themes define semantic colors:
- `Success` (green) - for healthy/running states
- `Error` (red) - for failed/error states
- `Warning` (yellow/orange) - for pending/warning states
- `Primary`, `Secondary`, `Accent` - UI accents
- `Foreground`, `Background` - base colors
- `Muted`, `Dimmed`, `Subtle` - de-emphasized colors

**Pre-defined status styles** (lines 40-42, repeated in all 8 theme
factories):
```go
StatusRunning = lipgloss.NewStyle().Foreground(t.Success)
StatusError = lipgloss.NewStyle().Foreground(t.Error)
StatusWarning = lipgloss.NewStyle().Foreground(t.Warning)
```

**Important**: These status styles exist in `TableStyles` struct but are NOT
passed to bubbles table component. The bubbles `table.Styles` only accepts
`Header`, `Cell`, and `Selected` styles (internal/ui/theme.go:45-52).

**Color values across themes**:
- Charm: Success=#02BA84, Error=#FF4672, Warning=#FFAA00
- Dracula: Success=#50fa7b, Error=#ff5555, Warning=#f1fa8c
- Catppuccin: Success=#a6e3a1, Error=#f38ba8, Warning=#f9e2af
- Nord: Success=#a3be8c, Error=#bf616a, Warning=#ebcb8b
- Gruvbox: Success=#b8bb26, Error=#fb4934, Warning=#fabd2f
- Tokyo Night: Success=#9ece6a, Error=#f7768e, Warning=#e0af68
- Solarized: Success=#859900, Error=#dc322f, Warning=#cb4b16
- Monokai: Success=#a6e22e, Error=#f92672, Warning=#e6db74

All colors use `lipgloss.AdaptiveColor` with light/dark variants.

### 3. Table Rendering and Styling Implementation

**Table initialization** (internal/screens/config.go:84-106):
- Bubbles table created with columns from `ScreenConfig`
- Global theme styles applied once: `t.SetStyles(theme.ToTableStyles())`
- Styles apply uniformly to all header/cell/selected rows

**Row creation** (internal/screens/config.go:482-510):
```go
func (s *ConfigScreen) updateTable() {
    rows := make([]table.Row, len(s.filtered))

    for i, item := range s.filtered {
        row := make(table.Row, len(s.config.Columns))
        for j, col := range s.config.Columns {
            val := getFieldValue(item, col.Field)  // Reflection-based

            // LINE 491-495: WHERE CONDITIONAL STYLING WOULD GO
            if col.Format != nil {
                row[j] = col.Format(val)  // Custom formatter
            } else {
                row[j] = fmt.Sprint(val)  // Default string conversion
            }
        }
        rows[i] = row
    }

    s.table.SetRows(rows)  // Set all rows at once
}
```

**Key insight**: Bubbles table rows are `[][]string`. Styling must be
embedded in string content as ANSI escape codes using lipgloss.Render().

**Data flow**:
1. Repository → resources (typed structs)
2. Filter → `s.filtered` (subset of resources)
3. `updateTable()` → extract fields, format, create string rows
4. `s.table.View()` → render with global styles

**Where to apply conditional styling**: Lines 491-495 in the column iteration
loop, by using lipgloss to create styled strings before adding to row.

### 4. Resource Status Determination Patterns

**Pattern 1: Simple Phase Extraction** (Pods, Namespaces, PVCs):
- Location: internal/k8s/transforms.go
- Extract `status.phase` field directly from unstructured object
- Pods: line 72 (`status, _, _ := unstructured.NestedString(u.Object,
  "status", "phase")`)
- Namespaces: line 236
- PVCs: line 467

**Pattern 2: Condition-Based Status** (Nodes):
- Location: internal/k8s/transforms.go:361-376
- Iterate through `status.conditions` array
- Find "Ready" condition, check if status is "True" → "Ready", else
  "NotReady"
- Default to "Unknown" if condition not found

**Pattern 3: No Explicit Status** (Deployments, Services):
- Deployments/StatefulSets: Status implied by replica counts ("3/3" =
  healthy, "1/3" = degraded)
- Services: No status field (Type field shows ClusterIP/LoadBalancer/NodePort)

**Pattern 4: Dummy Data Examples** (internal/k8s/dummy_repository.go:16-72):
- Shows "CrashLoopBackOff" as valid status value
- Note: Complex pod statuses (CrashLoopBackOff, ImagePullBackOff) are valid
  but NOT computed by transform logic - they come directly from Kubernetes API

**Status values by resource**:
- Pod: Running, Pending, Failed, Succeeded, Unknown, CrashLoopBackOff,
  ImagePullBackOff, ContainerCreating
- Namespace: Active, Terminating
- Node: Ready, NotReady, Unknown
- PVC: Bound, Pending, Lost

### 5. Existing Conditional Styling Patterns

**Pattern 1: Switch-based message type styling**
(internal/components/statusbar.go:61-89):
```go
switch sb.messageType {
case types.MessageTypeSuccess:
    messageStyle = baseStyle.Copy().
        Background(sb.theme.Success).
        Foreground(sb.theme.Background).
        Bold(true)
case types.MessageTypeError:
    messageStyle = baseStyle.Copy().
        Background(sb.theme.Error).
        Foreground(sb.theme.Background).
        Bold(true)
case types.MessageTypeInfo:
    messageStyle = baseStyle.Copy().
        Background(sb.theme.Primary).
        Foreground(sb.theme.Background).
        Bold(true)
}
return messageStyle.Render(prefix + sb.message)
```

**Pattern 2: Selection-based conditional styling**
(internal/components/commandbar/palette.go:194-224):
```go
if i == p.index {
    // Selected item: background + bold + arrow
    selectedStyle := lipgloss.NewStyle().
        Foreground(p.theme.Foreground).
        Background(p.theme.Subtle).
        Bold(true)
    line = selectedStyle.Render("▶ " + itemContent)
} else {
    // Normal item: plain
    line = paletteStyle.Render("  " + itemContent)
}
```

**Pattern 3: Column format functions** (internal/screens/config.go:491-495):
- Current: `Format func(interface{}) string` - no theme access
- Used for: Duration formatting (FormatDuration in screens.go)
- Returns plain strings, no styling

**Common pattern**:
1. Inspect value/state
2. Choose appropriate theme color/style
3. Create lipgloss.Style with color
4. Render value with style
5. Return styled string (contains ANSI escape codes)

## Code References

**Core files**:
- `internal/screens/config.go:482-510` - Table row building (WHERE TO APPLY
  STYLING)
- `internal/screens/config.go:84-106` - Table initialization
- `internal/screens/screens.go:1-600` - All 16 screen configurations
- `internal/ui/theme.go:36-52` - TableStyles definition and conversion
- `internal/ui/theme.go:55-545` - All 8 theme factory functions
- `internal/k8s/repository_types.go:1-165` - All resource type definitions
- `internal/k8s/transforms.go:1-600` - Status extraction logic

**Conditional styling examples**:
- `internal/components/statusbar.go:61-89` - Switch-based styling
- `internal/components/commandbar/palette.go:194-224` - Selection styling

**Status determination examples**:
- `internal/k8s/transforms.go:47-91` - Pod status (simple phase)
- `internal/k8s/transforms.go:357-444` - Node status (condition-based)
- `internal/k8s/transforms.go:232-247` - Namespace status
- `internal/k8s/transforms.go:466-500` - PVC status

## Architecture Insights

### Config-Driven Pattern Advantage

The config-driven architecture makes status coloring straightforward:
- Single implementation point: ConfigScreen.updateTable()
- Single extension point: ColumnConfig.Format function
- All 16 screens automatically benefit from enhanced formatters

### Theme System Preparedness

Status styles were defined in theme system but never wired up:
- All 8 themes have StatusRunning/Error/Warning styles ready
- Styles use semantic color mappings (Success→Running, Error→Error,
  Warning→Warning)
- Only foreground coloring (background stays default for readability)

This suggests status coloring was planned but not implemented.

### Formatter Signature Limitation

Current formatter signature: `func(interface{}) string`
- No access to theme reference
- No access to resource context (entire resource object)
- Only receives single field value

**Solution approaches**:
1. Make formatter a method of ConfigScreen (has theme access)
2. Pass theme in closure when creating formatter
3. Change formatter signature to `func(interface{}, *Theme) string`
4. Pass entire resource to formatter: `func(interface{}, interface{}) string`

### Bubbles Table Constraint

Bubbles table doesn't support per-row or per-cell style objects:
- Only global styles: Header, Cell, Selected
- Per-cell styling requires embedding ANSI codes in string content
- This is by design - bubbles table is simple and fast

### Status Semantics by Resource Type

**Healthy states** (should remain normal color):
- Pod: "Running" (when Ready="n/n"), "Succeeded" (for Jobs)
- Namespace: "Active"
- Node: "Ready"
- PVC: "Bound"
- Deployment/StatefulSet: Ready fraction matches desired (e.g., "3/3")

**Warning states** (yellow/orange):
- Pod: "Pending", "Unknown", "ContainerCreating"
- Node: "Unknown"
- PVC: "Pending"
- Deployment/StatefulSet: Ready < Desired (e.g., "2/3")

**Error states** (red):
- Pod: "Failed", "CrashLoopBackOff", "ImagePullBackOff", "ErrImagePull",
  "Error"
- Namespace: "Terminating"
- Node: "NotReady"
- PVC: "Lost"
- Deployment/StatefulSet: Ready=0 or very low

**Special case: Pod Ready + Status combo**:
- "Running" with Ready="0/1" → Warning (not fully ready)
- "Running" with Ready="1/1" → Normal (healthy)
- "Succeeded" → Normal (completed successfully for Jobs)

### Implementation Recommendations

**Approach 1: Status-Specific Formatters** (Simple, resource-aware)
```go
// In screens.go or new file formatters.go
func FormatPodStatus(theme *ui.Theme) func(interface{}) string {
    return func(val interface{}) string {
        status := fmt.Sprint(val)

        switch status {
        case "Running", "Succeeded":
            return status  // Normal color (no styling)
        case "Pending", "Unknown", "ContainerCreating":
            return theme.Table.StatusWarning.Render(status)
        case "Failed", "CrashLoopBackOff", "ImagePullBackOff", "Error":
            return theme.Table.StatusError.Render(status)
        default:
            return status
        }
    }
}

// In screen config:
{Field: "Status", Title: "Status", Width: 20, Format:
 FormatPodStatus(theme)}
```

**Approach 2: Generic Status Formatter** (Reusable, simple)
```go
type StatusMapping struct {
    Normal  []string
    Warning []string
    Error   []string
}

func FormatStatus(theme *ui.Theme, mapping StatusMapping)
func(interface{}) string {
    return func(val interface{}) string {
        status := fmt.Sprint(val)

        if contains(mapping.Error, status) {
            return theme.Table.StatusError.Render(status)
        }
        if contains(mapping.Warning, status) {
            return theme.Table.StatusWarning.Render(status)
        }
        return status  // Normal or unknown
    }
}

// In screen config:
podStatusMapping := StatusMapping{
    Normal:  []string{"Running", "Succeeded"},
    Warning: []string{"Pending", "Unknown"},
    Error:   []string{"Failed", "CrashLoopBackOff"},
}
{Field: "Status", Title: "Status", Width: 20, Format:
 FormatStatus(theme, podStatusMapping)}
```

**Approach 3: Whole-Row Styling** (Complex, most flexible)
- Change formatter signature to receive entire resource
- Status column formatter checks multiple fields (Pod.Status + Pod.Ready)
- Apply styling to all cells in row based on overall health
- More complex but enables "row-level" health indication

**Recommended: Start with Approach 1**
- Simple and explicit per-resource-type
- Easy to test and debug
- Can migrate to Approach 2 once patterns emerge
- Keeps theme as single source of truth for colors

### Testing Considerations

**Unit tests needed**:
- Formatter functions with different status values
- Verify ANSI codes are embedded in output
- Test all status values for each resource type
- Verify normal states remain unstyled

**Integration tests needed**:
- Verify table rendering with styled cells
- Test with all 8 themes (colors differ but structure same)
- Verify readability in light and dark terminal modes
- Test with real Kubernetes data (not just dummy)

**Manual testing**:
- Cluster with diverse pod states (running, pending, failed)
- Verify colors are distinct but not overwhelming
- Check readability on different terminal backgrounds
- Verify normal states blend in (not distracting)

## Historical Context (from thoughts/)

**Relevant thought documents** (11 found):

1. **thoughts/shared/research/2025-10-09-yaml-describe-search-feature.md**:
   - Research on YAML/describe view rendering with theme-aware syntax
     highlighting
   - Shows pattern for status message highlighting: keys use theme.Primary,
     values use theme.Foreground
   - Status sections rendered with 2-space indentation

2. **thoughts/shared/plans/2025-10-08-issue-3-scale-to-31-resources.md**:
   - Implementation plan for scaling to 31 Kubernetes resources
   - Discusses table rendering and theme system integration
   - Config-driven approach chosen for extensibility

3. **thoughts/shared/plans/2025-10-09-ISSUE-4-palette-scrolling.md**:
   - Palette scrolling feature implementation
   - Theme-aware styling for selection highlights using Subtle background
   - Dimmed colors for shortcuts

4. **thoughts/shared/plans/2025-10-07-contextual-navigation.md**:
   - Contextual navigation with filter banner styling
   - Notes on dead code cleanup: FilterBanner theme style removed when filter
     moved to header
   - Pattern: Remove unused theme styles to keep codebase clean

5. **thoughts/shared/research/2025-10-09-container-navigation.md**:
   - Container status aggregation research
   - Shows how "Ready: 2/3" is currently displayed as plain text
   - No conditional coloring discussed

6. **thoughts/shared/research/2025-10-08-issue-3-implementation-challenges.md**:
   - Architecture discussions on config-driven patterns
   - Table rendering optimizations
   - Performance considerations for large resource lists

**Key insight**: FilterBanner theme style was removed as dead code when
feature changed. This shows the project values cleaning up unused theme styles.
The StatusRunning/Error/Warning styles have been unused since theme system
creation - implementing status coloring would finally utilize them.

## Related Research

- thoughts/shared/research/2025-10-09-yaml-describe-search-feature.md - Theme-
  aware syntax highlighting patterns
- thoughts/shared/research/2025-10-09-container-navigation.md - Container
  status display
- thoughts/shared/research/2025-10-08-scaling-to-71-api-resources.md - UI/UX
  patterns for many resources

## Open Questions

1. **Ready field coloring**: Should "Ready" fraction field also be colored?
   (e.g., "1/3" in red, "3/3" in normal)

2. **Whole row vs single column**: Should status color apply to entire row or
   just Status column? Current recommendation: Status column only for subtlety.

3. **Numeric indicators**: How to color DaemonSet/ReplicaSet Ready vs Desired
   counts? (e.g., "Ready: 2" when "Desired: 5")

4. **CronJob Suspend**: Should suspended CronJobs be highlighted? What color?
   (Warning or Muted?)

5. **Service/ConfigMap/Secret**: These have no status fields. Should we add
   synthetic status indicators? (e.g., "Active" based on age or related
   resources)

6. **High restart count**: Should Pod Restarts field be colored if >10 or >50?
   (Indicates instability even if currently Running)

7. **Age-based coloring**: Should very old Terminating namespaces be
   highlighted differently? (Stuck termination)

8. **Performance**: Does lipgloss.Render() per cell impact performance on
   large clusters (1000+ pods)? Need to benchmark.

9. **Light mode testing**: Are status colors readable in both light and dark
   terminal modes? (AdaptiveColor should handle this, but needs verification)

10. **Color blindness**: Are Success/Warning/Error colors distinguishable for
    color-blind users? Consider adding symbols (✓/⚠/✗) alongside colors.

## Next Steps

1. Create formatter functions for each resource type with status fields (Pod,
   Namespace, Node, PVC)

2. Update screen configurations to use new formatters with theme reference

3. Add unit tests for formatters with various status values

4. Manual testing with real Kubernetes cluster containing diverse resource
   states

5. Gather user feedback on color choices and readability

6. Consider extending to Ready fields and numeric indicators based on feedback

7. Document status coloring conventions in CLAUDE.md or design docs

8. Consider adding symbols (✓/⚠/✗) for accessibility if needed
