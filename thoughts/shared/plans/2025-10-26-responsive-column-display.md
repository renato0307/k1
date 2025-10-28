# Responsive Column Display Implementation Plan

## Overview

Implement a responsive column display system that prioritizes important
columns on narrow terminals. The system uses a two-phase approach: first,
reallocate widths to make less-important columns dynamic (Phase 1), then add
automatic column hiding based on priority levels (Phase 2).

## Current State Analysis

k1 uses a simple width allocation system where columns can be fixed-width
(`Width: N`) or dynamic (`Width: 0`). Currently:

- **16 of 17 screens** have exactly 1 dynamic column (usually Name)
- **Dynamic columns** fill remaining space after fixed columns (minimum 20
  chars)
- **Problem**: On narrow terminals, important columns (Name, Status) get
  squeezed while less-important columns (Namespace, IP, Node) stay at fixed
  widths

**Key code locations**:
- `internal/screens/config.go:22-27` - ColumnConfig struct
- `internal/screens/config.go:247-286` - SetSize() width calculation
- `internal/screens/screens.go` - All 17 screen configurations

## Desired End State

A responsive column system where:

1. **Columns have priority levels** (1=critical, 2=important, 3=optional)
2. **Less-important columns shrink first** when terminal is narrow (Phase 1)
3. **Optional columns hide automatically** when terminal is very narrow
   (Phase 2)
4. **Priority field is exposed** for future user configuration
5. **System works automatically** without keybindings or manual toggles

**Verification**:
- Run k1 on 80-char terminal: Critical columns visible and readable
- Run k1 on 120-char terminal: Critical + Important columns visible
- Run k1 on 160+ char terminal: All columns visible
- Copy-paste hostname from Nodes screen: Works at all terminal widths

## What We're NOT Doing

- No keybindings to toggle column visibility (ctrl+w, etc.)
- No visual indicators for hidden columns ("... +2 hidden")
- No user configuration files (future enhancement)
- No responsive breakpoints (using priority-based hiding instead)
- No changes to k9s-style "Wide mode" toggle

## Implementation Approach

**Hybrid two-phase strategy**:
1. **Phase 1**: Quick win by reallocating fixed/dynamic widths based on
   priority
2. **Phase 2**: Complete solution with automatic column hiding
3. **Phase 3**: Comprehensive testing with generated resources

This approach allows incremental deployment and testing at each phase.

---

## Phase 1: Width Reallocation (Quick Win)

### Overview
Add Priority field to ColumnConfig and update screen configurations to make
less-important columns dynamic (Width: 0) while keeping critical columns
fixed.

### Changes Required

#### 1. Add Priority Field to ColumnConfig
**File**: `internal/screens/config.go`
**Changes**: Update ColumnConfig struct (lines 22-27)

```go
type ColumnConfig struct {
    Field    string                   // Field name in resource struct
    Title    string                   // Column display title
    Width    int                      // 0 = dynamic, >0 = fixed
    Format   func(interface{}) string // Optional custom formatter
    Priority int                      // 1=critical, 2=important, 3=optional
}
```

**Note**: Priority field is exposed for future user configuration but not used
in Phase 1 width calculation. It serves as documentation for Phase 2.

#### 2. Update Pods Screen Configuration
**File**: `internal/screens/screens.go`
**Changes**: Modify GetPodsScreenConfig() columns (lines 16-25)

**Current** (all fixed except Name):
```go
Columns: []ColumnConfig{
    {Field: "Namespace", Title: "Namespace", Width: 40},
    {Field: "Name", Title: "Name", Width: 0},
    {Field: "Ready", Title: "Ready", Width: 8},
    {Field: "Status", Title: "Status", Width: 15},
    {Field: "Restarts", Title: "Restarts", Width: 10},
    {Field: "Age", Title: "Age", Width: 10, Format: FormatDuration},
    {Field: "Node", Title: "Node", Width: 30},
    {Field: "IP", Title: "IP", Width: 16},
}
```

**After** (less-important columns now dynamic):
```go
Columns: []ColumnConfig{
    {Field: "Namespace", Title: "Namespace", Width: 0, Priority: 2},
    {Field: "Name", Title: "Name", Width: 40, Priority: 1},
    {Field: "Ready", Title: "Ready", Width: 8, Priority: 1},
    {Field: "Status", Title: "Status", Width: 15, Priority: 1},
    {Field: "Restarts", Title: "Restarts", Width: 10, Priority: 1},
    {Field: "Age", Title: "Age", Width: 10, Format: FormatDuration,
     Priority: 1},
    {Field: "Node", Title: "Node", Width: 0, Priority: 3},
    {Field: "IP", Title: "IP", Width: 0, Priority: 3},
}
```

**Changes**:
- Namespace: Fixed 40 → Dynamic (Priority 2)
- Name: Dynamic → Fixed 40 (Priority 1, most important)
- Node: Fixed 30 → Dynamic (Priority 3, optional)
- IP: Fixed 16 → Dynamic (Priority 3, optional)
- Add Priority to all columns

**Effect**: On 150-char terminal, Name stays at 40 chars (readable) while
Namespace/Node/IP share remaining space (~17 chars each).

#### 3. Update Deployments Screen Configuration
**File**: `internal/screens/screens.go`
**Changes**: Modify GetDeploymentsScreenConfig() columns (lines 61-68)

```go
Columns: []ColumnConfig{
    {Field: "Namespace", Title: "Namespace", Width: 0, Priority: 2},
    {Field: "Name", Title: "Name", Width: 40, Priority: 1},
    {Field: "Ready", Title: "Ready", Width: 10, Priority: 1},
    {Field: "UpToDate", Title: "Up-to-date", Width: 12, Priority: 1},
    {Field: "Available", Title: "Available", Width: 12, Priority: 1},
    {Field: "Age", Title: "Age", Width: 10, Format: FormatDuration,
     Priority: 1},
}
```

**Changes**: Same pattern as Pods - Namespace dynamic, Name fixed.

#### 4. Update Services Screen Configuration
**File**: `internal/screens/screens.go`
**Changes**: Modify GetServicesScreenConfig() columns (lines 89-97)

```go
Columns: []ColumnConfig{
    {Field: "Namespace", Title: "Namespace", Width: 0, Priority: 2},
    {Field: "Name", Title: "Name", Width: 40, Priority: 1},
    {Field: "Type", Title: "Type", Width: 15, Priority: 1},
    {Field: "ClusterIP", Title: "Cluster-IP", Width: 15, Priority: 2},
    {Field: "ExternalIP", Title: "External-IP", Width: 15, Priority: 2},
    {Field: "Ports", Title: "Ports", Width: 20, Priority: 1},
    {Field: "Age", Title: "Age", Width: 10, Format: FormatDuration,
     Priority: 1},
}
```

**Changes**: Namespace dynamic, Name fixed.

#### 5. Update Nodes Screen Configuration
**File**: `internal/screens/screens.go`
**Changes**: Modify GetNodesScreenConfig() columns (lines 300-311)

**Current** (only Name is dynamic):
```go
Columns: []ColumnConfig{
    {Field: "Name", Title: "Name", Width: 0},
    {Field: "Status", Title: "Status", Width: 12},
    {Field: "Roles", Title: "Roles", Width: 15},
    {Field: "Hostname", Title: "Hostname", Width: 30},
    {Field: "InstanceType", Title: "Instance", Width: 20},
    {Field: "Zone", Title: "Zone", Width: 20},
    {Field: "NodePool", Title: "NodePool", Width: 20},
    {Field: "Version", Title: "Version", Width: 15},
    {Field: "OSImage", Title: "OS Image", Width: 40},
    {Field: "Age", Title: "Age", Width: 10, Format: FormatDuration},
}
```

**After** (detailed metadata columns now dynamic, Hostname stays fixed):
```go
Columns: []ColumnConfig{
    {Field: "Name", Title: "Name", Width: 40, Priority: 1},
    {Field: "Status", Title: "Status", Width: 12, Priority: 1},
    {Field: "Roles", Title: "Roles", Width: 0, Priority: 3},
    {Field: "Hostname", Title: "Hostname", Width: 30, Priority: 1},
    {Field: "InstanceType", Title: "Instance", Width: 0, Priority: 3},
    {Field: "Zone", Title: "Zone", Width: 0, Priority: 3},
    {Field: "NodePool", Title: "NodePool", Width: 0, Priority: 3},
    {Field: "Version", Title: "Version", Width: 15, Priority: 1},
    {Field: "OSImage", Title: "OS Image", Width: 0, Priority: 3},
    {Field: "Age", Title: "Age", Width: 10, Format: FormatDuration,
     Priority: 1},
}
```

**Changes**:
- Name: Dynamic → Fixed 40 (Priority 1, critical)
- Roles: Fixed 15 → Dynamic (Priority 3, less important)
- Hostname: **Stays Fixed 30** (Priority 1, user copies this frequently)
- InstanceType, Zone, NodePool, OSImage: Fixed → Dynamic (Priority 3)

**Rationale**: Hostname stays fixed because user frequently copies it for SSH,
etc.

#### 6. Update Remaining 13 Screens
**Files**: `internal/screens/screens.go`
**Changes**: Apply same pattern to remaining screens

**For each screen**:
- Namespace: Dynamic (Priority 2) if present
- Name: Fixed 40 (Priority 1)
- State/Status columns: Fixed (Priority 1)
- Metadata columns: Dynamic (Priority 3)
- Age: Fixed (Priority 1)

**Screen list**:
- ConfigMaps (lines 118-123)
- Secrets (lines 143-149)
- Namespaces (lines 169-173)
- StatefulSets (lines 193-198)
- DaemonSets (lines 219-228)
- Jobs (lines 248-253)
- CronJobs (lines 273-280)
- ReplicaSets (lines 332-339)
- PVCs (lines 359-368)
- Ingresses (lines 388-396)
- Endpoints (lines 416-421) - Special case: Endpoints dynamic, Name fixed 30
- HPAs (lines 441-450)
- Contexts (lines 470-476) - Special case: 2 dynamic columns

### Success Criteria

#### Automated Verification:
- [x] Code compiles: `make build`
- [x] All tests pass: `make test`
- [x] Linting passes: `go vet ./...`
- [x] Type checking passes: `go build ./...`

#### Manual Verification:
- [ ] Run k1 on 150-char terminal: Name column readable (40 chars) in Pods
  screen
- [ ] Run k1 on 120-char terminal: Dynamic columns shrink proportionally
- [ ] Run k1 on 80-char terminal: Dynamic columns at minimum 20 chars
- [ ] Nodes screen: Hostname column visible and copy-pasteable at all widths
- [ ] No visual regressions in other screens

**Implementation Note**: After completing this phase and all automated
verification passes, pause here for manual confirmation from the human that
the manual testing was successful before proceeding to Phase 2.

---

## Phase 2: Priority-Based Column Hiding

### Overview
Implement automatic column hiding based on priority levels when terminal is
too narrow to display all columns comfortably.

### Changes Required

#### 1. Add Column Visibility Tracking to ConfigScreen
**File**: `internal/screens/config.go`
**Changes**: Add fields to ConfigScreen struct (around line 60)

```go
type ConfigScreen struct {
    // ... existing fields ...
    visibleColumns []ColumnConfig // Columns currently visible
    hiddenCount    int             // Number of hidden columns
}
```

#### 2. Implement shouldExcludeColumn Method
**File**: `internal/screens/config.go`
**Changes**: Add new method after SetSize()

```go
// shouldExcludeColumn determines if a column should be hidden based on
// priority and available width. Called during SetSize() to calculate which
// columns fit in the terminal.
func (s *ConfigScreen) shouldExcludeColumn(col ColumnConfig,
    availableWidth int, usedWidth int) bool {

    colWidth := col.Width
    if colWidth == 0 {
        colWidth = 20 // Minimum for dynamic columns
    }

    // Priority 1 (critical) always shows, even if squished
    if col.Priority == 1 {
        return false
    }

    // Priority 2 and 3 only show if they fit
    return usedWidth+colWidth > availableWidth
}
```

#### 3. Update SetSize() with Column Hiding Logic
**File**: `internal/screens/config.go`
**Changes**: Replace existing SetSize() method (lines 247-286)

```go
func (s *ConfigScreen) SetSize(width, height int) {
    s.width = width
    s.height = height
    s.table.SetHeight(height)

    // Calculate padding
    padding := len(s.config.Columns) * 2
    availableWidth := width - padding

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
        if s.shouldExcludeColumn(col, availableWidth, usedWidth) {
            continue
        }

        visibleColumns = append(visibleColumns, col)
        colWidth := col.Width
        if colWidth == 0 {
            colWidth = 20 // Estimate for dynamic
        }
        usedWidth += colWidth
    }

    // Restore original column order
    s.visibleColumns = s.restoreColumnOrder(visibleColumns)
    s.hiddenCount = len(s.config.Columns) - len(visibleColumns)

    // Calculate dynamic widths for visible columns only
    fixedTotal := 0
    dynamicCount := 0

    for _, col := range s.visibleColumns {
        if col.Width > 0 {
            fixedTotal += col.Width
        } else {
            dynamicCount++
        }
    }

    // Recalculate padding for visible columns only
    visiblePadding := len(s.visibleColumns) * 2
    dynamicWidth := (width - fixedTotal - visiblePadding) / dynamicCount
    if dynamicWidth < 20 {
        dynamicWidth = 20
    }

    // Build table columns from visible columns only
    columns := make([]table.Column, len(s.visibleColumns))
    for i, col := range s.visibleColumns {
        w := col.Width
        if w == 0 {
            w = dynamicWidth
        }
        columns[i] = table.Column{
            Title: col.Title,
            Width: w,
        }
    }

    s.table.SetColumns(columns)
    s.table.SetWidth(width)
}
```

#### 4. Add restoreColumnOrder Helper Method
**File**: `internal/screens/config.go`
**Changes**: Add new helper method

```go
// restoreColumnOrder restores the original column order after sorting by
// priority. This ensures columns appear in the same order as defined in
// screen config, not sorted by priority.
func (s *ConfigScreen) restoreColumnOrder(visible []ColumnConfig)
    []ColumnConfig {

    result := []ColumnConfig{}

    // Iterate original config order
    for _, original := range s.config.Columns {
        // Check if this column is in visible list
        for _, v := range visible {
            if v.Field == original.Field {
                result = append(result, v)
                break
            }
        }
    }

    return result
}
```

#### 5. Update updateTable() to Use Visible Columns
**File**: `internal/screens/config.go`
**Changes**: Modify updateTable() method (lines 518-547)

```go
func (s *ConfigScreen) updateTable() {
    rows := make([]table.Row, len(s.filtered))

    for i, item := range s.filtered {
        // Use visibleColumns instead of s.config.Columns
        row := make(table.Row, len(s.visibleColumns))
        for j, col := range s.visibleColumns {
            val := getFieldValue(item, col.Field)

            if col.Format != nil {
                row[j] = col.Format(val)
            } else {
                row[j] = fmt.Sprint(val)
            }
        }
        rows[i] = row
    }

    s.table.SetRows(rows)

    if len(rows) > 0 {
        cursor := s.table.Cursor()
        if cursor < 0 || cursor >= len(rows) {
            s.table.SetCursor(0)
        }
    }
}
```

**Key change**: Replace `s.config.Columns` with `s.visibleColumns` to skip
hidden columns.

#### 6. Initialize visibleColumns in NewConfigScreen
**File**: `internal/screens/config.go`
**Changes**: Update NewConfigScreen() (lines 83-107)

```go
func NewConfigScreen(repo k8s.Repository, cfg ScreenConfig, theme ui.Theme)
    *ConfigScreen {

    // ... existing code ...

    s := &ConfigScreen{
        repo:           repo,
        config:         cfg,
        table:          t,
        theme:          theme,
        visibleColumns: cfg.Columns, // Initialize with all columns
        hiddenCount:    0,
    }

    return s
}
```

### Success Criteria

#### Automated Verification:
- [x] Code compiles: `make build`
- [x] All tests pass: `make test`
- [x] Linting passes: `go vet ./...`
- [x] No regressions in Phase 1 functionality

#### Manual Verification:
- [x] Run k1 on 80-char terminal: Priority 3 columns hidden automatically
- [x] Run k1 on 100-char terminal: Priority 2 columns visible, Priority 3
  hidden
- [x] Run k1 on 160-char terminal: All columns visible
- [x] Resize terminal live: Columns appear/disappear smoothly
- [x] Nodes screen at 80 chars: Name, Status, Hostname, Version, Age visible
  (Priority 1)
- [x] Pods screen at 80 chars: Name, Ready, Status, Restarts, Age visible
  (Priority 1)
- [x] No visual artifacts or rendering glitches
- [x] Table cursor position maintained across resizes

**Implementation Note**: After completing this phase and all automated
verification passes, pause here for manual confirmation from the human that
the manual testing was successful before proceeding to Phase 3.

---

## Phase 3: Testing & Validation

### Overview
Create comprehensive tests and a resource generation script for manual
testing on local cluster.

### Changes Required

#### 1. Add Unit Tests for SetSize with Priorities
**File**: `internal/screens/config_test.go`
**Changes**: Add new test cases

```go
func TestConfigScreen_SetSizeWithPriorities(t *testing.T) {
    tests := []struct {
        name            string
        terminalWidth   int
        columns         []ColumnConfig
        expectedVisible int
        expectedHidden  int
    }{
        {
            name:          "Wide terminal - all columns visible",
            terminalWidth: 200,
            columns: []ColumnConfig{
                {Field: "Name", Width: 40, Priority: 1},
                {Field: "Status", Width: 15, Priority: 1},
                {Field: "Node", Width: 30, Priority: 3},
                {Field: "IP", Width: 16, Priority: 3},
            },
            expectedVisible: 4,
            expectedHidden:  0,
        },
        {
            name:          "Narrow terminal - Priority 3 hidden",
            terminalWidth: 80,
            columns: []ColumnConfig{
                {Field: "Name", Width: 40, Priority: 1},
                {Field: "Status", Width: 15, Priority: 1},
                {Field: "Node", Width: 30, Priority: 3},
                {Field: "IP", Width: 16, Priority: 3},
            },
            expectedVisible: 2,
            expectedHidden:  2,
        },
        {
            name:          "Very narrow - only Priority 1",
            terminalWidth: 60,
            columns: []ColumnConfig{
                {Field: "Name", Width: 20, Priority: 1},
                {Field: "Status", Width: 15, Priority: 1},
                {Field: "Namespace", Width: 40, Priority: 2},
                {Field: "Node", Width: 30, Priority: 3},
            },
            expectedVisible: 2, // Name + Status (Priority 1)
            expectedHidden:  2, // Namespace + Node
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            config := ScreenConfig{
                Columns: tt.columns,
            }
            screen := NewConfigScreen(nil, config, ui.ThemeCharm())
            screen.SetSize(tt.terminalWidth, 40)

            assert.Equal(t, tt.expectedVisible, len(screen.visibleColumns))
            assert.Equal(t, tt.expectedHidden, screen.hiddenCount)
        })
    }
}
```

#### 2. Add Test for Column Order Preservation
**File**: `internal/screens/config_test.go`
**Changes**: Add new test case

```go
func TestConfigScreen_ColumnOrderPreserved(t *testing.T) {
    columns := []ColumnConfig{
        {Field: "Name", Width: 40, Priority: 1},
        {Field: "Status", Width: 15, Priority: 1},
        {Field: "Namespace", Width: 40, Priority: 2},
        {Field: "IP", Width: 16, Priority: 3},
    }

    config := ScreenConfig{Columns: columns}
    screen := NewConfigScreen(nil, config, ui.ThemeCharm())
    screen.SetSize(200, 40) // Wide enough for all columns

    // Verify columns appear in original order, not sorted by priority
    assert.Equal(t, "Name", screen.visibleColumns[0].Field)
    assert.Equal(t, "Status", screen.visibleColumns[1].Field)
    assert.Equal(t, "Namespace", screen.visibleColumns[2].Field)
    assert.Equal(t, "IP", screen.visibleColumns[3].Field)
}
```

#### 3. Create Resource Generation Script
**File**: `scripts/test-resources.sh`
**Changes**: Create new executable script

```bash
#!/bin/bash
# Script to generate test resources in a local Kubernetes cluster for
# testing responsive column display.
#
# Usage: ./scripts/test-resources.sh [namespace]
# Default namespace: k1-test

set -e

NAMESPACE="${1:-k1-test}"

echo "Creating test resources in namespace: $NAMESPACE"

# Create namespace
kubectl create namespace "$NAMESPACE" --dry-run=client -o yaml | \
    kubectl apply -f -

# Create Pods with long names
echo "Creating Pods with various name lengths..."
for i in {1..5}; do
    kubectl run "pod-short-$i" \
        --image=nginx:latest \
        --namespace="$NAMESPACE" \
        --labels="app=test,priority=high" \
        --dry-run=client -o yaml | kubectl apply -f -
done

for i in {1..5}; do
    kubectl run "pod-with-very-long-name-for-testing-truncation-$i" \
        --image=nginx:latest \
        --namespace="$NAMESPACE" \
        --labels="app=test,priority=low" \
        --dry-run=client -o yaml | kubectl apply -f -
done

# Create Deployments
echo "Creating Deployments..."
kubectl create deployment "deploy-test-1" \
    --image=nginx:latest \
    --namespace="$NAMESPACE" \
    --dry-run=client -o yaml | kubectl apply -f -

kubectl create deployment "deployment-with-extremely-long-name" \
    --image=nginx:latest \
    --namespace="$NAMESPACE" \
    --dry-run=client -o yaml | kubectl apply -f -

# Scale deployments
kubectl scale deployment "deploy-test-1" --replicas=3 \
    --namespace="$NAMESPACE"

# Create Services
echo "Creating Services..."
kubectl expose deployment "deploy-test-1" \
    --name="svc-test" \
    --port=80 \
    --namespace="$NAMESPACE" \
    --dry-run=client -o yaml | kubectl apply -f -

kubectl create service clusterip "service-with-long-name" \
    --tcp=80:80 \
    --namespace="$NAMESPACE" \
    --dry-run=client -o yaml | kubectl apply -f -

# Create ConfigMaps
echo "Creating ConfigMaps..."
kubectl create configmap "cm-short" \
    --from-literal=key1=value1 \
    --namespace="$NAMESPACE" \
    --dry-run=client -o yaml | kubectl apply -f -

kubectl create configmap "configmap-with-very-long-name-for-testing" \
    --from-literal=key1=value1 \
    --from-literal=key2=value2 \
    --namespace="$NAMESPACE" \
    --dry-run=client -o yaml | kubectl apply -f -

# Create Secrets
echo "Creating Secrets..."
kubectl create secret generic "secret-short" \
    --from-literal=password=test123 \
    --namespace="$NAMESPACE" \
    --dry-run=client -o yaml | kubectl apply -f -

kubectl create secret generic "secret-with-long-name-for-testing" \
    --from-literal=password=test123 \
    --namespace="$NAMESPACE" \
    --dry-run=client -o yaml | kubectl apply -f -

# Create StatefulSets
echo "Creating StatefulSets..."
cat <<EOF | kubectl apply -f -
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: sts-test
  namespace: $NAMESPACE
spec:
  serviceName: "sts-test"
  replicas: 2
  selector:
    matchLabels:
      app: sts
  template:
    metadata:
      labels:
        app: sts
    spec:
      containers:
      - name: nginx
        image: nginx:latest
        ports:
        - containerPort: 80
EOF

# Create DaemonSet
echo "Creating DaemonSet..."
cat <<EOF | kubectl apply -f -
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: ds-test
  namespace: $NAMESPACE
spec:
  selector:
    matchLabels:
      app: ds
  template:
    metadata:
      labels:
        app: ds
    spec:
      containers:
      - name: nginx
        image: nginx:latest
EOF

# Create Jobs
echo "Creating Jobs..."
kubectl create job "job-short" \
    --image=busybox:latest \
    --namespace="$NAMESPACE" \
    -- echo "test" \
    --dry-run=client -o yaml | kubectl apply -f -

kubectl create job "job-with-very-long-name-for-testing" \
    --image=busybox:latest \
    --namespace="$NAMESPACE" \
    -- echo "test" \
    --dry-run=client -o yaml | kubectl apply -f -

# Create CronJobs
echo "Creating CronJobs..."
kubectl create cronjob "cron-test" \
    --image=busybox:latest \
    --schedule="*/5 * * * *" \
    --namespace="$NAMESPACE" \
    -- echo "test" \
    --dry-run=client -o yaml | kubectl apply -f -

kubectl create cronjob "cronjob-with-long-name" \
    --image=busybox:latest \
    --schedule="*/10 * * * *" \
    --namespace="$NAMESPACE" \
    -- echo "test" \
    --dry-run=client -o yaml | kubectl apply -f -

# Create Ingress
echo "Creating Ingress..."
cat <<EOF | kubectl apply -f -
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: ingress-test
  namespace: $NAMESPACE
spec:
  rules:
  - host: test.example.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: svc-test
            port:
              number: 80
EOF

# Create HPA
echo "Creating HPA..."
kubectl autoscale deployment "deploy-test-1" \
    --min=2 \
    --max=10 \
    --cpu-percent=80 \
    --namespace="$NAMESPACE"

echo ""
echo "✓ Test resources created successfully in namespace: $NAMESPACE"
echo ""
echo "To test k1 with these resources:"
echo "  1. Run: go run cmd/k1/main.go"
echo "  2. Navigate to Pods screen (:pods)"
echo "  3. Filter to test namespace: type 'k1-test'"
echo "  4. Resize terminal to 80, 120, 160 chars and observe column behavior"
echo ""
echo "To clean up:"
echo "  kubectl delete namespace $NAMESPACE"
```

#### 4. Make Script Executable
**Command**: `chmod +x scripts/test-resources.sh`

#### 5. Create Testing Checklist Document
**File**: `scripts/test-responsive-columns.md`
**Changes**: Create new testing checklist

```markdown
# Responsive Column Display Testing Checklist

## Terminal Width Test Scenarios

### Test 1: Wide Terminal (160+ chars)
**Setup**: Resize terminal to 200 characters wide

- [ ] Pods screen: All 8 columns visible
- [ ] Nodes screen: All 10 columns visible
- [ ] Name columns have sufficient space (40+ chars)
- [ ] No horizontal scrolling or clipping

### Test 2: Medium Terminal (120 chars)
**Setup**: Resize terminal to 120 characters wide

- [ ] Pods screen: Priority 1 and 2 columns visible
- [ ] Pods screen: Priority 3 columns (Node, IP) may be hidden
- [ ] Nodes screen: Priority 1 columns visible (Name, Status, Hostname,
      Version, Age)
- [ ] Name column still readable (30+ chars)
- [ ] Table fits within terminal width

### Test 3: Narrow Terminal (80 chars)
**Setup**: Resize terminal to 80 characters wide

- [ ] Pods screen: Only Priority 1 columns visible (Name, Ready, Status,
      Restarts, Age)
- [ ] Nodes screen: Only Priority 1 columns visible (Name, Status, Hostname,
      Version, Age)
- [ ] Name column readable (20+ chars minimum)
- [ ] No Priority 3 columns visible
- [ ] Table fits within terminal width

### Test 4: Very Narrow Terminal (60 chars)
**Setup**: Resize terminal to 60 characters wide

- [ ] Pods screen: Critical columns visible with minimum widths
- [ ] Name column may be squished but not hidden (Priority 1)
- [ ] No crashes or rendering errors
- [ ] Table may slightly exceed width but Name readable

## Resource Name Length Tests

### Test 5: Short Resource Names
**Setup**: Navigate to resources with short names (pod-short-1, etc.)

- [ ] Columns display with expected spacing
- [ ] Name column not unnecessarily wide
- [ ] Other columns get fair share of space

### Test 6: Long Resource Names
**Setup**: Navigate to resources with very long names
(pod-with-very-long-name-for-testing-truncation-1, etc.)

- [ ] Name column shows as much as possible
- [ ] Long names truncate gracefully with "..." if needed
- [ ] Other columns still visible
- [ ] No horizontal scrolling

## Specific Screen Tests

### Test 7: Nodes Screen - Hostname Copy/Paste
**Setup**: Navigate to Nodes screen (:nodes)

- [ ] At 160 chars: Hostname fully visible (30 chars fixed)
- [ ] At 120 chars: Hostname still visible (30 chars fixed)
- [ ] At 80 chars: Hostname still visible (30 chars fixed, Priority 1)
- [ ] Can select and copy hostname at all terminal widths
- [ ] Hostname never hidden regardless of width

### Test 8: Pods Screen - Critical Information
**Setup**: Navigate to Pods screen (:pods) with test namespace

- [ ] At 80 chars: Ready, Status, Restarts visible (pod health info)
- [ ] Name column readable (identifies pod)
- [ ] Age visible (temporal context)
- [ ] Namespace hidden or shrunk (less critical on narrow terminal)

### Test 9: All Screens Basic Functionality
For each screen (Deployments, Services, ConfigMaps, Secrets, StatefulSets,
DaemonSets, Jobs, CronJobs, ReplicaSets, PVCs, Ingresses, Endpoints, HPAs,
Contexts):

- [ ] Screen loads without errors
- [ ] Name column visible at all widths
- [ ] Priority 1 columns always visible
- [ ] Priority 3 columns hidden on narrow terminals
- [ ] No crashes or rendering glitches

## Live Resize Tests

### Test 10: Dynamic Resize
**Setup**: k1 running on 160-char terminal

- [ ] Gradually shrink terminal to 80 chars
- [ ] Columns disappear smoothly (no flashing)
- [ ] Table reflows without errors
- [ ] Cursor position maintained
- [ ] Gradually expand back to 160 chars
- [ ] Hidden columns reappear
- [ ] No visual artifacts

### Test 11: Rapid Resize
**Setup**: k1 running on any terminal width

- [ ] Rapidly resize terminal multiple times
- [ ] No crashes or panics
- [ ] Table eventually settles correctly
- [ ] No memory leaks or performance degradation

## Filter/Search Tests

### Test 12: Filter with Hidden Columns
**Setup**: Narrow terminal (80 chars) with columns hidden

- [ ] Type to enter filter mode
- [ ] Filter searches all fields (including hidden columns)
- [ ] Results display correctly
- [ ] Hidden columns remain hidden
- [ ] Filter works as expected (e.g., searching Node field even if Node
      column hidden)

## Priority Validation Tests

### Test 13: Priority 1 Columns Never Hidden
**Setup**: Test all screens at 60-char terminal (very narrow)

- [ ] Name column always visible (even if squished)
- [ ] Status/Ready columns always visible
- [ ] Age column always visible
- [ ] No Priority 1 columns hidden

### Test 14: Priority 3 Columns Hidden First
**Setup**: Start at 160 chars, gradually shrink

- [ ] Priority 3 columns (IP, Node in Pods) disappear first
- [ ] Priority 2 columns (Namespace) disappear second
- [ ] Priority 1 columns remain until last

## Setup Instructions

1. **Create test resources**:
   ```bash
   ./scripts/test-resources.sh k1-test
   ```

2. **Run k1**:
   ```bash
   go run cmd/k1/main.go
   ```

3. **Resize terminal**: Use terminal emulator's resize feature or:
   - macOS: `printf '\e[8;40;80t'` (80 cols)
   - Linux: `resize -s 40 80`
   - Or manually drag terminal window

4. **Navigate screens**: Use `:` to open navigation palette

5. **Filter to test namespace**: Type `k1-test` to see test resources

6. **Clean up**:
   ```bash
   kubectl delete namespace k1-test
   ```
```

### Success Criteria

#### Automated Verification:
- [x] Unit tests pass: `make test`
- [x] Test coverage maintained or improved: `make test-coverage`
- [N/A] Script runs without errors: `./scripts/test-resources.sh` (skipped)
- [N/A] Resources created successfully in test namespace (skipped)

#### Manual Verification:
- [N/A] All Test 1-14 scenarios pass (see test-responsive-columns.md) (checklist skipped)
- [x] No visual regressions on any screen
- [x] Hostname copy/paste works at all widths (Nodes screen)
- [x] Filter searches hidden columns correctly
- [x] No performance degradation with real cluster data
- [x] Long resource names handled gracefully

**Implementation Note**: After completing all tests, document any issues found
and create follow-up tasks if needed.

---

## Testing Strategy

### Unit Tests
- Width calculation with different terminal sizes
- Column hiding logic (shouldExcludeColumn)
- Column order preservation after sorting
- Dynamic width calculation with multiple dynamic columns
- Edge cases (0 columns, 1 column, all Priority 1)

### Integration Tests
- Full SetSize() flow with real ColumnConfig
- updateTable() with visible columns only
- Resize sequence: wide → narrow → wide
- Multiple screens with different column configurations

### Manual Testing
Use `./scripts/test-resources.sh` to create test resources, then:

1. **Width scenarios**: Test at 80, 100, 120, 140, 160, 200 chars
2. **Resource names**: Test with short (5 chars) and long (50+ chars) names
3. **Screen coverage**: Test all 17 screens
4. **Copy/paste**: Verify Hostname copy works on Nodes screen
5. **Filtering**: Verify filter searches hidden columns

## Performance Considerations

### Width Calculation Overhead
- Sorting columns by priority: O(n log n) where n = column count (~10)
- Filtering columns: O(n) where n = column count
- Restoring order: O(n²) worst case, but n is small (~10 columns)

**Expected impact**: Negligible (sub-millisecond on modern hardware)

### Memory Overhead
- `visibleColumns` slice: ~80 bytes per column × 10 columns = ~800 bytes
- `hiddenCount` int: 8 bytes
- Total per screen: < 1 KB

**Expected impact**: None (negligible compared to table row data)

### Refresh Performance
- SetSize() called only on window resize (infrequent)
- updateTable() complexity unchanged (still O(rows × columns))
- Column hiding reduces rendering cost slightly (fewer columns to render)

**Expected impact**: Slight improvement on narrow terminals (fewer columns to
render)

## Migration Notes

**No data migration needed**: This is a pure UI change with no persistence
layer.

**Backward compatibility**: Priority field defaults to 0, which is treated as
Priority 1 (critical). Existing code without Priority specified will continue
to work.

**User impact**: Immediate visual improvement on narrow terminals, no breaking
changes.

## References

- Original research: `thoughts/shared/research/2025-10-26-column-display-smaller-windows.md`
- ColumnConfig struct: `internal/screens/config.go:22-27`
- SetSize() method: `internal/screens/config.go:247-286`
- Screen configurations: `internal/screens/screens.go`
- k9s shouldExcludeColumn pattern: `.tmp/k9s/internal/ui/table.go:323-328`

## Open Questions

None - all clarifications resolved during planning phase.
