---
date: 2025-10-08T18:44:14Z
researcher: @claude
git_commit: 43b4993bbef9a616072cab1080a1f746ea758829
branch: feat/issue-3-system-resources-screen
repository: k1
topic: "Why Phase 2 Implementation Was Harder Than Expected"
tags: [research, codebase, code-quality, golang, type-switches,
       refactoring]
status: complete
---

# Research: Why Phase 2 Implementation Was Harder Than Expected

## Executive Summary

Phase 2 implementation (adding 5 new Kubernetes resources) revealed
significant architectural debt that made the task harder than expected.
The primary issue was **type switch helper functions that silently fail
when new types are added**, requiring manual updates in 3 scattered
locations. This research identifies the root causes, code smells, Go
best practices violations, and provides actionable recommendations.

**Key Findings:**
1. **Type switches for extensible behavior violate Open/Closed Principle**
   - 3 helper functions with silent default cases
   - Adding new types requires updating 3 separate locations
   - No compile-time safety, only runtime bugs

2. **Quality gate violations make code navigation difficult**
   - `informer_repository.go` at 1775 lines (122% over 800-line limit)
   - Helper functions buried at line ~1600+
   - Single file handles 15+ different concerns

3. **Test coverage gaps allowed bugs to slip through**
   - Tests validated implementation as-written, not requirements
   - No scenarios where Age and CreatedAt differ
   - Integration tests only test single resources

4. **9 major code duplication patterns identified**
   - Pod filtering logic repeated 6 times
   - Index maintenance paired functions (3 sets)
   - Navigation command boilerplate (11 nearly-identical functions)

## Root Cause Analysis

### The Sorting Bug That Started It All

**User report**: "the sorting for new resources (eg pvcs) is not ok,
the list is always changing"

**Immediate cause**: Three type switch helper functions
(`extractCreatedAt`, `extractAge`, `extractName`) weren't updated with
new resource types (ReplicaSet, PVC, Ingress, Endpoints, HPA).

**Location**: `internal/k8s/informer_repository.go` lines 1657-1774

```go
func extractCreatedAt(item any) time.Time {
    switch v := item.(type) {
    case Pod:
        return v.CreatedAt
    case Deployment:
        return v.CreatedAt
    // ... 14 more cases for old types
    default:
        return time.Time{} // SILENT FAILURE - returns zero time
    }
}

// Similar structure for extractAge() and extractName()
```

**Why this happened:**
1. Helper functions buried at line ~1600+ in 1775-line file
2. Not part of obvious "transform → register → screen" flow
3. No compile error when new types added (silently uses default case)
4. Tests validated implementation as-written, not requirements

**Impact on development:**
- Required manual user testing to discover
- No IDE warning or compiler error
- Bug manifested as UX issue (constantly changing sort order)
- Fix required updating 3 separate functions scattered in large file

### Why Type Switches Are the Wrong Pattern

**From Go community best practices research:**

Type switches that need frequent updates are a **code smell** in Go.
They violate the Open/Closed Principle: software entities should be
open for extension but closed for modification.

**Rob Pike quote**: "Accept interfaces, return concrete types"

**Problem with our current pattern:**
```go
// Every time we add a new type, we must update 3 places:
func extractCreatedAt(item any) time.Time {
    switch v := item.(type) {
    case Pod: return v.CreatedAt
    case Deployment: return v.CreatedAt
    // ... must add case for EVERY new type
    default: return time.Time{} // Silent failure
    }
}
```

**Recommended Go pattern** (interface-based polymorphism):
```go
// All types implement this interface
type Timestamped interface {
    GetCreatedAt() time.Time
    GetAge() time.Duration
    GetName() string
}

// No type switch needed - works for all current AND future types
func extractCreatedAt(item Timestamped) time.Time {
    return item.GetCreatedAt()
}
```

**Benefits of interface approach:**
- ✅ Compile-time safety (types must implement interface)
- ✅ No manual updates when adding new types
- ✅ Self-documenting (interface shows required methods)
- ✅ Testable (can mock interface easily)
- ✅ Open/Closed Principle satisfied

## Code Smells Identified

### 1. Type Switches for Extensible Behavior

**Location**: `internal/k8s/informer_repository.go` lines 1657-1774

**Code smell**: Using type switches for behavior that should be
polymorphic.

**Evidence**: 3 nearly-identical functions with 16-case type switches:
- `extractCreatedAt()`: 37 lines, 16 cases + default
- `extractAge()`: 38 lines, 16 cases + default
- `extractName()`: 38 lines, 16 cases + default

**Problem**: Adding any new resource type requires updating all 3
functions. Silent default cases return zero values with no error.

**Go best practice violated**: "Accept interfaces, return concrete types"

**Recommendation**: Replace with interface-based polymorphism (see
Recommendations section).

### 2. Silent Default Cases in Type Switches

**Location**: All 3 extraction functions end with:
```go
default:
    return time.Time{}  // or 0, or ""
```

**Code smell**: Silent failures - returns zero value instead of error.

**Problem**:
- New types silently use wrong default behavior
- No logging or error to indicate problem
- Bug only visible through UX (sorting appears random)
- No test can catch this without explicit negative test cases

**Go best practice violated**: "Errors should be explicit and handled,
not silently ignored"

**Recommendation**: Either return error from default case OR use
interface to make implementation mandatory.

### 3. God File Anti-Pattern

**Location**: `internal/k8s/informer_repository.go` - 1775 lines

**Code smell**: File exceeds 800-line quality gate by 122% (975 lines
over limit).

**Concerns in single file:**
1. Repository interface implementation
2. Pod index maintenance (6 indexes)
3. Job index maintenance
4. ReplicaSet index maintenance
5. Statistics tracking (channel-based)
6. Event handlers for 11+ resource types
7. Transform helper functions
8. Navigation query methods (10+ methods)
9. YAML/Describe formatters
10. Event fetching and formatting
11. Pod transformations
12. Type switch extraction helpers
13. Generic sort functions
14. Context management (lifecycle)
15. Memory statistics calculation

**Problem**: Single Responsibility Principle violated. Too many concerns
make helper functions hard to find (buried at line ~1600+).

**Function length violations:**
- `NewInformerRepository()`: 187 lines (exceeds 150-line limit by 25%)

**Go best practice violated**: "Keep functions small and files focused
on single concern"

**Recommendation**: Extract concerns into separate files (see
Recommendations section).

### 4. Approaching Quality Gates in Other Files

**`internal/k8s/transforms.go`**: 791 lines (99% of 800-line limit)
- Currently acceptable but approaching danger zone
- Should be monitored as new resources added

**`internal/k8s/dummy_repository.go`**: 556 lines (11% over 500-line
warning threshold)
- Mock data acceptable for test doubles
- Consider extracting fixture data if grows more

## Test Coverage Gaps

### Gap 1: Tests Validate Implementation, Not Requirements

**Location**: `internal/k8s/informer_repository_test.go`

**Problem**: Unit tests pass because they validate the implementation
as-written, not the actual requirements.

**Example test that validated buggy code:**
```go
func TestSortByAge(t *testing.T) {
    now := time.Now()
    items := []interface{}{
        Pod{Name: "old-pod", CreatedAt: now.Add(-10 * time.Hour)},
        Pod{Name: "new-pod", CreatedAt: now.Add(-1 * time.Hour)},
    }

    sortByAge(items)

    // Test asserts sorting by CreatedAt works (which it does)
    // But NEVER checks that Age field is actually used
    assert.Equal(t, "new-pod", items[0].(Pod).Name)
}
```

**What's missing:**
- No test where `Age` and `CreatedAt` differ
- No test that explicitly validates `Age` field is used for sorting
- No negative test for unsupported types

**Why this happened:**
- Test named `TestSortByAge` but tests `CreatedAt` behavior
- No requirement specification - test written from implementation
- False sense of security from 76% coverage

**Recommendation**: Add requirement-based tests (see Recommendations
section).

### Gap 2: No Integration Tests for Multi-Resource Sorting

**Current state**: Integration tests (`integration_test.go`) only test
single resources. No test creates multiple resource types and verifies
sorting across them.

**Missing scenarios:**
- Create resources with different types in cluster
- Verify they all appear in correct sort order
- Test that new types work immediately without code changes

**Recommendation**: Add integration tests that verify sorting across
multiple resource types simultaneously.

### Gap 3: No Tests for Type Switch Default Cases

**Current state**: No test explicitly verifies what happens when an
unsupported type is passed to extraction functions.

**Missing tests:**
```go
func TestExtractCreatedAt_UnsupportedType(t *testing.T) {
    // Should this return zero time? Return error? Log warning?
    result := extractCreatedAt(struct{ Name string }{"test"})
    // What's the correct behavior?
}
```

**Recommendation**: Define explicit behavior for unsupported types and
test it.

## Code Duplication Patterns

### Pattern 1: Pod Filtering Logic (6 Variants)

**Locations**:
- `GetPodsForDeployment()` - lines 450-479
- `GetPodsForStatefulSet()` - lines 556-569
- `GetPodsForDaemonSet()` - lines 572-585
- `GetPodsForJob()` - lines 588-611
- `GetPodsUsingConfigMap()` - lines 697-706
- `GetPodsUsingSecret()` - lines 709-718

**Common pattern:**
1. Get owner/resource to extract UID or key
2. Look up pods from index using UID/key
3. Transform pods using `transformPods()`

**Duplication:**
- Index lookup logic repeated 6 times
- Error handling pattern repeated 6 times
- Transform call repeated 6 times

**Recommendation**: Extract generic `getPodsFromIndex()` helper:
```go
func (r *InformerRepository) getPodsFromIndex(
    indexName string,
    key string,
) ([]Pod, error) {
    r.mu.RLock()
    pods := r.getIndexByName(indexName)[key]
    r.mu.RUnlock()
    return r.transformPods(pods)
}
```

### Pattern 2: Index Maintenance Event Handlers (3 Paired Functions)

**Locations**:
- Job indexes: `updateJobIndexes()` + `removeJobFromIndexes()`
  (lines 1436-1487)
- ReplicaSet indexes: `updateReplicaSetIndexes()` +
  `removeReplicaSetFromIndexes()` (lines 1385-1433)
- Pod indexes: `updatePodIndexes()` + `removePodFromIndexes()`
  (lines 1100-1255)

**Common pattern:**
1. Lock acquisition
2. Remove old entries (if update)
3. Extract key/UID from resource
4. Add to index maps
5. Lock release

**Duplication:**
- Lock/unlock pattern repeated 6 times
- Key extraction logic repeated 6 times
- Map cleanup (delete if empty) repeated 6 times

**Recommendation**: Extract generic index maintenance framework with
type-specific extractors.

### Pattern 3: Navigation Command Boilerplate (11 Functions)

**Location**: `internal/commands/navigation.go` (before refactoring)

**Pattern (BEFORE refactoring):**
```go
func PodsCommand() ExecuteFunc {
    return func(ctx CommandContext) tea.Cmd {
        return func() tea.Msg {
            return types.ScreenSwitchMsg{ScreenID: "pods"}
        }
    }
}

func DeploymentsCommand() ExecuteFunc {
    return func(ctx CommandContext) tea.Cmd {
        return func() tea.Msg {
            return types.ScreenSwitchMsg{ScreenID: "deployments"}
        }
    }
}

// ... 9 more nearly-identical functions
```

**Solution implemented:**
```go
var navigationRegistry = map[string]string{
    "pods": "pods",
    "deployments": "deployments",
    // ... all entries
}

func NavigationCommand(screenID string) ExecuteFunc {
    return func(ctx CommandContext) tea.Cmd {
        return func() tea.Msg {
            return types.ScreenSwitchMsg{ScreenID: screenID}
        }
    }
}
```

**Result**: Eliminated ~80 lines of boilerplate (11 × 8 lines = 88
lines → 8 lines).

**Status**: ✅ **Already refactored** in previous session as part of
navigation improvements.

### Pattern 4: Error Handling in Repository Methods

**Pattern repeated across 10+ methods:**
```go
func (r *Repository) GetResource() (Result, error) {
    obj, err := r.lister.Get(name)
    if err != nil {
        return nil, fmt.Errorf("failed to get resource: %w", err)
    }
    // ... processing
}
```

**Duplication:**
- Error wrapping pattern repeated 15+ times
- Similar error messages ("failed to get X")
- Same context pattern

**Note**: This is actually **good practice** - explicit error wrapping
with context is recommended. Only extract if pattern becomes more
complex.

### Pattern 5: Common Field Extraction

**Location**: Multiple transform functions

**Pattern**: Each transform extracts namespace, name, age, createdAt
from unstructured object.

**Solution implemented:**
```go
type commonFields struct {
    Namespace string
    Name      string
    Age       time.Duration
    CreatedAt time.Time
}

func extractCommonFields(u *unstructured.Unstructured) commonFields {
    // Extract once, used by all transforms
}
```

**Status**: ✅ **Already optimized** - caller extracts once, passes to
transform functions. Reduced O(11n) to O(n) operations.

### Pattern 6: Table Config Boilerplate (11 Screen Configs)

**Location**: `internal/screens/screens.go`

**Pattern**: Each screen config has similar structure:
```go
func GetPodsScreenConfig() ScreenConfig {
    return ScreenConfig{
        ID:           "pods",
        Title:        "Pods",
        ResourceType: k8s.ResourceTypePod,
        Columns: []ColumnConfig{
            {Field: "Namespace", Title: "Namespace", Width: 40},
            // ... columns
        },
        SearchFields: []string{"Namespace", "Name"},
        Operations: []OperationConfig{
            {ID: "describe", Name: "Describe", Shortcut: "d"},
            {ID: "yaml", Name: "YAML", Shortcut: "y"},
        },
        NavigationHandler:     navigateToSomewhere(),
        EnablePeriodicRefresh: true,
        RefreshInterval:       RefreshInterval,
        TrackSelection:        true,
    }
}
```

**Note**: This is **config-driven design** - intentional duplication for
declarative configuration. Each screen has unique columns, operations,
and navigation handlers. Extracting would reduce clarity.

**Status**: ✅ **Acceptable duplication** - serves configuration purpose.

### Pattern 7: Transform Function Structure (16 Functions)

**Location**: `internal/k8s/transforms.go`

**Pattern**: Each transform has similar structure:
```go
func transformResource(u *unstructured.Unstructured, common
    commonFields) (any, error) {

    // Extract resource-specific fields
    field1, _, _ := unstructured.NestedString(u.Object, "spec", "field1")
    field2, _, _ := unstructured.NestedInt64(u.Object, "status", "field2")

    // Build typed struct
    return ResourceType{
        Namespace: common.Namespace,
        Name:      common.Name,
        Field1:    field1,
        Field2:    field2,
        Age:       common.Age,
        CreatedAt: common.CreatedAt,
    }, nil
}
```

**Note**: This duplication is **acceptable** - each resource has
different fields and extraction logic. Config-driven approach already
eliminates registration boilerplate.

**Status**: ✅ **Acceptable duplication** - resource-specific extraction
logic.

### Pattern 8: Sorting Logic (3 Functions)

**Locations**:
- `sortByAge()` - lines 1620-1635 (generic, uses extraction helpers)
- `sortByCreationTime()` - lines 1638-1654 (generic with type constraint)
- Manual sorts in repository methods (6 locations)

**Pattern in repository methods:**
```go
sort.Slice(items, func(i, j int) bool {
    createdI := getCreatedAt(items[i])
    createdJ := getCreatedAt(items[j])
    if !createdI.Equal(createdJ) {
        return createdI.After(createdJ) // Newer first
    }
    return getName(items[i]) < getName(items[j])
})
```

**Status**: Already has generic helpers. Manual sorts in repository
methods are for typed slices ([]Pod, []Job) which can't use generic
`sortByAge()` without interface.

### Pattern 9: Registry Initialization Boilerplate

**Location**: `internal/k8s/transforms.go` lines 610-790

**Pattern**: Each resource registration follows same structure:
```go
ResourceTypePod: {
    GVR: schema.GroupVersionResource{
        Group: "", Version: "v1", Resource: "pods"},
    Name:       "Pods",
    Namespaced: true,
    Tier:       0,
    Transform:  transformPod,
},
```

**Note**: This is **config-driven design** - intentional duplication for
declarative resource registry. Each entry maps resource type to its
configuration.

**Status**: ✅ **Acceptable duplication** - serves registry purpose.

## Go Best Practices Comparison

### What We're Doing Right

1. **Config-driven architecture** (navigation refactoring)
   - ✅ Function pointers for behavior
   - ✅ Declarative configuration
   - ✅ Open/Closed Principle satisfied

2. **Common field extraction optimization**
   - ✅ Extract once, pass to all transforms
   - ✅ Reduces O(11n) to O(n)
   - ✅ Avoids reflection performance penalty

3. **Table-driven tests**
   - ✅ Used extensively in test suite
   - ✅ Good coverage of edge cases

4. **Error wrapping with context**
   - ✅ Consistent use of `fmt.Errorf("context: %w", err)`
   - ✅ Preserves error chain for debugging

5. **Interface-based repository pattern**
   - ✅ `k8s.Repository` interface abstracts data access
   - ✅ Supports both live and dummy implementations

### What We're Doing Wrong

1. **Type switches for extensible behavior** ❌
   - Problem: Violates Open/Closed Principle
   - Impact: Adding types requires updating 3 scattered locations
   - Solution: Use interface-based polymorphism

2. **Silent default cases in type switches** ❌
   - Problem: Returns zero values without error
   - Impact: Bugs only visible through UX
   - Solution: Return errors from default case OR use interface

3. **God file anti-pattern** ❌
   - Problem: 1775-line file with 15+ concerns
   - Impact: Hard to navigate, find functions
   - Solution: Extract concerns into separate files

4. **Test coverage focused on implementation, not requirements** ❌
   - Problem: Tests validate code as-written
   - Impact: Bugs slip through despite 76% coverage
   - Solution: Requirement-based test scenarios

## Recommendations

### Priority 1: Replace Type Switches with Interfaces (High Impact)

**Problem**: Type switches in `extractCreatedAt()`, `extractAge()`,
`extractName()` require manual updates when adding resources.

**Solution**: Define interface and implement on all resource types.

**Implementation:**

```go
// 1. Define interface (internal/k8s/types.go)
type Resource interface {
    GetNamespace() string
    GetName() string
    GetAge() time.Duration
    GetCreatedAt() time.Time
}

// 2. Implement on all resource types
func (p Pod) GetNamespace() string { return p.Namespace }
func (p Pod) GetName() string { return p.Name }
func (p Pod) GetAge() time.Duration { return p.Age }
func (p Pod) GetCreatedAt() time.Time { return p.CreatedAt }

// ... implement on all 16 resource types

// 3. Replace type switches with interface calls
func extractCreatedAt(item Resource) time.Time {
    return item.GetCreatedAt()  // No type switch needed!
}

func extractAge(item Resource) time.Duration {
    return item.GetAge()
}

func extractName(item Resource) string {
    return item.GetName()
}

// 4. Update sortByAge to use interface
func sortByAge(items []Resource) {
    sort.Slice(items, func(i, j int) bool {
        createdI := items[i].GetCreatedAt()
        createdJ := items[j].GetCreatedAt()

        if !createdI.Equal(createdJ) {
            return createdI.After(createdJ)
        }

        return items[i].GetName() < items[j].GetName()
    })
}
```

**Benefits:**
- ✅ Compile-time safety (types must implement interface)
- ✅ No manual updates when adding new types
- ✅ Eliminates 113 lines of type switch boilerplate
- ✅ Future-proof against similar bugs

**Effort**: ~2 hours (implement interface on 16 types, update callers)

**Breaking changes**: None (internal refactoring only)

### Priority 2: Refactor God File (Medium Impact)

**Problem**: `informer_repository.go` at 1775 lines makes code hard to
navigate.

**Solution**: Extract concerns into separate files.

**Proposed file structure:**

```
internal/k8s/
├── repository.go              # Interface definition
├── informer_repository.go     # Core repository (< 300 lines)
├── informer_indexes.go        # Index maintenance (< 400 lines)
├── informer_events.go         # Event handler registration (< 300 lines)
├── informer_stats.go          # Statistics tracking (< 200 lines)
├── informer_queries.go        # Navigation queries (< 300 lines)
├── resource_formatters.go     # YAML/Describe (< 200 lines)
└── dummy_repository.go        # Mock implementation
```

**What goes where:**

**`informer_repository.go`** (~300 lines):
- Repository struct definition
- `NewInformerRepository()` (decomposed into helpers)
- Core lifecycle methods (`Close()`, `GetKubeconfig()`, etc.)

**`informer_indexes.go`** (~400 lines):
- All index maps and maintenance functions
- `setupPodIndexes()`, `setupJobIndexes()`, `setupReplicaSetIndexes()`
- `updatePodIndexes()`, `removePodFromIndexes()`, etc.
- Helper functions like `removePodFromSlice()`

**`informer_events.go`** (~300 lines):
- `setupDynamicInformersEventTracking()`
- Event handler registration for statistics
- `trackStats()` helper

**`informer_stats.go`** (~200 lines):
- `ResourceStats` struct
- `statsUpdater()` goroutine
- `GetResourceStats()`
- `updateMemoryStats()`

**`informer_queries.go`** (~300 lines):
- All `GetPodsFor*()` methods
- `GetReplicaSetsForDeployment()`
- `transformPods()` helper

**`resource_formatters.go`** (~200 lines):
- `GetResourceYAML()`
- `DescribeResource()`
- `fetchEventsForResource()`
- `formatEvents()`, `formatEventAge()`

**Benefits:**
- ✅ Each file under 500 lines (well under 800-line limit)
- ✅ Concerns clearly separated (easier to find code)
- ✅ Easier to review and test individual concerns
- ✅ Reduces cognitive load when working on specific feature

**Effort**: ~4 hours (move code, update imports, test)

**Breaking changes**: None (internal refactoring only)

### Priority 3: Add Requirement-Based Tests (Medium Impact)

**Problem**: Tests validate implementation as-written, not requirements.

**Solution**: Add test cases that explicitly test requirements.

**New tests to add:**

```go
// Test that sorting uses Age field, not CreatedAt
func TestSortByAge_UsesAgeField(t *testing.T) {
    now := time.Now()

    items := []Resource{
        // Old CreatedAt but recent Age (restarted pod)
        Pod{
            Name: "restarted-pod",
            CreatedAt: now.Add(-10 * time.Hour),
            Age: 5 * time.Minute,  // Recently restarted
        },
        // Recent CreatedAt but old Age
        Pod{
            Name: "normal-pod",
            CreatedAt: now.Add(-1 * time.Hour),
            Age: 1 * time.Hour,
        },
    }

    sortByAge(items)

    // Should sort by Age, not CreatedAt
    // restarted-pod has Age=5m, normal-pod has Age=1h
    // Newer first → restarted-pod should be first
    assert.Equal(t, "restarted-pod", items[0].GetName())
}

// Test that new types work automatically with interface
func TestSortByAge_NewTypeAutomatic(t *testing.T) {
    // Define a new type that implements Resource
    type NewResource struct {
        Namespace string
        Name      string
        Age       time.Duration
        CreatedAt time.Time
    }

    // Implement interface
    func (n NewResource) GetNamespace() string { return n.Namespace }
    func (n NewResource) GetName() string { return n.Name }
    func (n NewResource) GetAge() time.Duration { return n.Age }
    func (n NewResource) GetCreatedAt() time.Time { return n.CreatedAt }

    // Should work immediately without updating sortByAge()
    items := []Resource{
        NewResource{Name: "new1", Age: 5 * time.Minute},
        NewResource{Name: "new2", Age: 10 * time.Minute},
    }

    sortByAge(items)

    // Should sort correctly
    assert.Equal(t, "new2", items[0].GetName()) // Newer first
}

// Integration test for multi-resource sorting
func TestGetResources_SortingAcrossTypes(t *testing.T) {
    // Create cluster with multiple resource types
    createPod(client, "old-pod", 10*time.Hour)
    createDeployment(client, "new-deploy", 1*time.Hour)
    createPVC(client, "medium-pvc", 5*time.Hour)

    // Get all pods - should be sorted by age across types
    pods := repo.GetResources(ResourceTypePod)
    deployments := repo.GetResources(ResourceTypeDeployment)
    pvcs := repo.GetResources(ResourceTypePersistentVolumeClaim)

    // Verify each type is sorted
    assert.Equal(t, "new-deploy", deployments[0].(Deployment).Name)
    // ... verify others
}
```

**Benefits:**
- ✅ Tests verify actual requirements, not implementation
- ✅ Catches bugs that silently return wrong values
- ✅ Documents expected behavior
- ✅ Prevents regressions when refactoring

**Effort**: ~2 hours (write tests, verify behavior)

**Breaking changes**: None (tests only)

### Priority 4: Extract Code Duplication (Low Impact)

**Problem**: Pod filtering logic repeated 6 times.

**Solution**: Extract generic helper function.

**Implementation:**

```go
// Generic pod filtering helper
func (r *InformerRepository) getPodsFromIndex(
    getIndexKey func() (string, error),
    indexGetter func(string) []*corev1.Pod,
) ([]Pod, error) {
    key, err := getIndexKey()
    if err != nil {
        return nil, err
    }

    r.mu.RLock()
    pods := indexGetter(key)
    r.mu.RUnlock()

    return r.transformPods(pods)
}

// Use in specific methods
func (r *InformerRepository) GetPodsForStatefulSet(namespace, name
    string) ([]Pod, error) {

    return r.getPodsFromIndex(
        func() (string, error) {
            sts, err := r.statefulSetLister.StatefulSets(namespace).Get(name)
            if err != nil {
                return "", fmt.Errorf("failed to get statefulset: %w", err)
            }
            return string(sts.UID), nil
        },
        func(uid string) []*corev1.Pod {
            return r.podsByOwnerUID[uid]
        },
    )
}
```

**Benefits:**
- ✅ Eliminates ~40 lines of repeated code
- ✅ Single place to fix bugs in filtering logic
- ✅ Easier to add new filtering methods

**Effort**: ~1 hour (extract and refactor)

**Breaking changes**: None (internal refactoring only)

## Implementation Order

Recommended sequence for addressing findings:

1. **Week 1: Type switch → interface refactoring** (Priority 1)
   - Highest impact: prevents future bugs
   - Enables compile-time safety
   - Foundation for other improvements

2. **Week 2: Requirement-based tests** (Priority 3)
   - Before file splitting, ensure behavior is tested
   - Documents expected behavior
   - Prevents regressions during refactoring

3. **Week 3: File splitting** (Priority 2)
   - With tests in place, safe to refactor
   - Reduces file sizes below quality gates
   - Makes codebase more maintainable

4. **Week 4: Extract duplication** (Priority 4)
   - Lowest impact but nice improvement
   - Can be done incrementally
   - Polish after major refactoring complete

## Conclusion

Phase 2 implementation revealed significant architectural debt centered
around **type switches for extensible behavior**. The immediate sorting
bug was just a symptom of a deeper design issue: using type switches in
a system where types are frequently added.

**Key lessons:**
1. Type switches are appropriate for closed sets of types, not
   extensible systems
2. Interface-based polymorphism is Go's idiomatic solution for
   extensible behavior
3. Quality gates (file size, function length) serve a purpose - they
   catch accumulating complexity early
4. Tests should validate requirements, not just implementation
   as-written

**Immediate action items:**
1. Replace type switches with Resource interface (prevents future bugs)
2. Add requirement-based tests (validates behavior, not code)
3. Split god file into focused concerns (improves navigation)

**Long-term benefit**: Future resource additions will require:
- ✅ Implement Resource interface (compile-time checked)
- ✅ Register in transforms (one location)
- ✅ Create screen config (one location)
- ❌ ~~Update 3 type switch functions~~ (eliminated)
- ❌ ~~Hunt for hidden helper functions~~ (eliminated)

With these changes, adding Phase 3 resources (7 new types) should be
significantly easier than Phase 2.

---

**Research completed**: 2025-10-08T18:44:14Z
**Next steps**: Review findings with team, prioritize refactoring work
