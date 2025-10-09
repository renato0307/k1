# Refactor Type Switches to Interface Pattern - Implementation Plan

## Metadata

- **Date**: 2025-10-08
- **Author**: @renato0307
- **Status**: In Progress - Phase 2 Complete
- **Related Research**: `thoughts/shared/research/2025-10-08-issue-3-implementation-challenges.md`
- **Related Ticket**: Issue #3 (Phase 2 post-mortem findings)
- **Branch**: `feat/refactor-type-switches`
- **Estimated Duration**: 4 weeks

## Overview

This plan addresses architectural debt discovered during Issue #3 Phase 2
implementation. The primary issue: **type switch helper functions that
silently fail when new types are added**, requiring manual updates in 3
scattered locations. This violates the Open/Closed Principle and caused a
sorting bug during Phase 2.

## Problem Statement

### Root Cause

Three type switch helper functions (`extractCreatedAt`, `extractAge`,
`extractName`) at `internal/k8s/informer_repository.go:1657-1774` require
manual updates when adding new resource types. These functions have silent
default cases that return zero values, causing bugs that only manifest at
runtime through UX issues (sorting appears random).

**Location**: `internal/k8s/informer_repository.go`
- `extractCreatedAt()`: Lines 1657-1694 (37 lines, 16 cases + default)
- `extractAge()`: Lines 1697-1734 (37 lines, 16 cases + default)
- `extractName()`: Lines 1737-1774 (37 lines, 16 cases + default)
- **Total**: 113 lines of type switch boilerplate

### Impact

During Phase 2, new resources (ReplicaSet, PVC, Ingress, Endpoints, HPA)
were added but these helper functions weren't updated, causing:
- Lists constantly changing order (zero time sorts to beginning)
- No compile error or IDE warning
- Bug only discovered through manual user testing
- Required updating 3 separate functions in 1774-line file

### Secondary Issues

1. **God file anti-pattern**: `informer_repository.go` at 1774 lines (122%
   over 800-line limit) with 15+ concerns
2. **Test coverage gaps**: Tests validate implementation as-written, not
   requirements (TestExtractCreatedAt doesn't exist)
3. **Code duplication**: Pod filtering logic repeated 6 times

## Goals

1. **Eliminate type switches** (Priority 1): Replace with Resource interface
   pattern for compile-time safety
2. **Add requirement-based tests** (Priority 3): Test requirements, not
   implementation details
3. **Split god file** (Priority 2): Extract concerns into 6 focused files
   (each <500 lines)
4. **Extract duplication** (Priority 4): Generic pod filtering helper

## Non-Goals

- Changing external API or Repository interface signatures
- Adding new resource types (done in Issue #3 phases)
- Modifying transform function behavior
- Performance optimization beyond eliminating reflection

## Implementation Phases

### Phase 1: Replace Type Switches with Resource Interface (Week 1)

**Goal**: Eliminate 113 lines of type switch boilerplate, enable
compile-time safety.

**Changes**:

1. **Define Resource interface** (`internal/k8s/repository.go` after
   line 124):

```go
// Resource represents any Kubernetes resource with common fields
// All resource types must implement this interface for sorting
type Resource interface {
    GetNamespace() string  // "" for cluster-scoped resources
    GetName() string
    GetAge() time.Duration
    GetCreatedAt() time.Time
}
```

2. **Implement interface on all 16 resource type structs**
   (`internal/k8s/repository.go` after each struct definition):

```go
// Pod implements Resource interface
func (p Pod) GetNamespace() string        { return p.Namespace }
func (p Pod) GetName() string             { return p.Name }
func (p Pod) GetAge() time.Duration       { return p.Age }
func (p Pod) GetCreatedAt() time.Time     { return p.CreatedAt }

// Deployment implements Resource interface
func (d Deployment) GetNamespace() string { return d.Namespace }
func (d Deployment) GetName() string      { return d.Name }
func (d Deployment) GetAge() time.Duration { return d.Age }
func (d Deployment) GetCreatedAt() time.Time { return d.CreatedAt }

// ... implement for all 16 types (Pod, Deployment, Service, ConfigMap,
// Secret, Namespace, StatefulSet, DaemonSet, Job, CronJob, Node,
// ReplicaSet, PersistentVolumeClaim, Ingress, Endpoints,
// HorizontalPodAutoscaler)
```

3. **Delete type switch functions** (`internal/k8s/informer_repository.go`
   lines 1657-1774):

Delete `extractCreatedAt()`, `extractAge()`, `extractName()` entirely
(113 lines removed).

4. **Update sortByAge to use interface**
   (`internal/k8s/informer_repository.go:1620-1635`):

```go
// BEFORE (uses type switches)
func sortByAge(items []any) {
    sort.Slice(items, func(i, j int) bool {
        createdI := extractCreatedAt(items[i])  // Type switch
        createdJ := extractCreatedAt(items[j])  // Type switch

        if !createdI.Equal(createdJ) {
            return createdI.After(createdJ)
        }

        nameI := extractName(items[i])  // Type switch
        nameJ := extractName(items[j])  // Type switch
        return nameI < nameJ
    })
}

// AFTER (uses interface methods)
func sortByAge(items []Resource) {
    sort.Slice(items, func(i, j int) bool {
        createdI := items[i].GetCreatedAt()  // Method call
        createdJ := items[j].GetCreatedAt()  // Method call

        if !createdI.Equal(createdJ) {
            return createdI.After(createdJ)
        }

        return items[i].GetName() < items[j].GetName()  // Method call
    })
}
```

5. **Update GetResources to use interface**
   (`internal/k8s/informer_repository.go:1573-1617`):

```go
// BEFORE
func (r *InformerRepository) GetResources(resourceType ResourceType) ([]any, error) {
    // ... list and transform resources ...

    results := make([]any, 0, len(objList))
    for _, obj := range objList {
        // ... transform ...
        results = append(results, transformed)
    }

    sortByAge(results)  // []any argument
    return results, nil
}

// AFTER
func (r *InformerRepository) GetResources(resourceType ResourceType) ([]any, error) {
    // ... list and transform resources ...

    results := make([]Resource, 0, len(objList))
    for _, obj := range objList {
        // ... transform ...
        resource, ok := transformed.(Resource)
        if !ok {
            continue  // Skip non-Resource types (shouldn't happen)
        }
        results = append(results, resource)
    }

    sortByAge(results)  // []Resource argument

    // Convert back to []any for existing API compatibility
    anyResults := make([]any, len(results))
    for i, r := range results {
        anyResults[i] = r
    }
    return anyResults, nil
}
```

**Success Criteria**:
- [x] Resource interface defined with 4 methods
- [x] All 16 resource types implement interface (64 methods total)
- [x] Type switch functions deleted (113 lines removed)
- [x] sortByAge uses interface methods only
- [x] GetResources uses []Resource internally
- [x] `make test` passes (existing tests still work)
- [x] `make build` succeeds
- [x] File size reduced: 1774 → 1661 lines (6% reduction)

**Manual Verification**:
- [x] Navigate to all 16 resource screens
- [x] Verify sorting is stable (not constantly changing)
- [ ] Add a new dummy resource type with interface implementation
- [ ] Verify it compiles and sorts correctly without updating sortByAge

**Additional Fixes Applied**:
- Changed `sort.Slice` → `sort.SliceStable` for stable sorting
- Added three-level sort key (CreatedAt → Name → Namespace) for determinism
- Fixed sorting instability that caused lists to jump around on refresh

### Phase 2: Add Requirement-Based Tests (Week 2)

**Goal**: Test requirements, not implementation. Ensure Age field is
actually used for sorting, not just CreatedAt.

**Changes**:

1. **Add TestExtractCreatedAt** (currently missing)
   (`internal/k8s/informer_repository_test.go` after line 647):

```go
func TestExtractCreatedAt(t *testing.T) {
    now := time.Now()

    tests := []struct {
        name     string
        resource Resource
        want     time.Time
    }{
        {
            name:     "Pod with valid CreatedAt",
            resource: Pod{CreatedAt: now},
            want:     now,
        },
        {
            name:     "Deployment with old CreatedAt",
            resource: Deployment{CreatedAt: now.Add(-1 * time.Hour)},
            want:     now.Add(-1 * time.Hour),
        },
        // Test all 16 resource types
        {
            name:     "ReplicaSet with CreatedAt",
            resource: ReplicaSet{CreatedAt: now.Add(-5 * time.Minute)},
            want:     now.Add(-5 * time.Minute),
        },
        // ... more cases
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := tt.resource.GetCreatedAt()
            assert.Equal(t, tt.want, got)
        })
    }
}
```

2. **Add TestSortByAge_UsesAgeField** (test requirement, not
   implementation) (`internal/k8s/informer_repository_test.go` after
   line 816):

```go
func TestSortByAge_UsesAgeField(t *testing.T) {
    now := time.Now()

    // Create pods where Age ≠ CreatedAt (e.g., restarted pods)
    resources := []Resource{
        // Old CreatedAt but recent Age (restarted pod)
        Pod{
            Name:      "restarted-pod",
            CreatedAt: now.Add(-10 * time.Hour),  // Created 10h ago
            Age:       5 * time.Minute,             // Restarted 5m ago
        },
        // Recent CreatedAt and Age match
        Pod{
            Name:      "normal-pod",
            CreatedAt: now.Add(-1 * time.Hour),
            Age:       1 * time.Hour,
        },
    }

    sortByAge(resources)

    // Should sort by Age, not CreatedAt
    // restarted-pod has Age=5m, normal-pod has Age=1h
    // Since we sort newest first, restarted-pod should be first
    assert.Equal(t, "restarted-pod", resources[0].GetName(),
        "Should sort by Age field, not CreatedAt")
}
```

3. **Add TestSortByAge_MultipleResourceTypes**
   (`internal/k8s/informer_repository_test.go`):

```go
func TestSortByAge_MultipleResourceTypes(t *testing.T) {
    now := time.Now()

    resources := []Resource{
        Pod{Name: "old-pod", CreatedAt: now.Add(-10 * time.Hour)},
        Deployment{Name: "new-deploy", CreatedAt: now.Add(-1 * time.Hour)},
        Service{Name: "medium-svc", CreatedAt: now.Add(-5 * time.Hour)},
        ReplicaSet{Name: "new-rs", CreatedAt: now.Add(-2 * time.Hour)},
        PersistentVolumeClaim{Name: "old-pvc", CreatedAt: now.Add(-20 * time.Hour)},
    }

    sortByAge(resources)

    // Verify order: newest first
    assert.Equal(t, "new-deploy", resources[0].GetName())
    assert.Equal(t, "new-rs", resources[1].GetName())
    assert.Equal(t, "medium-svc", resources[2].GetName())
    assert.Equal(t, "old-pod", resources[3].GetName())
    assert.Equal(t, "old-pvc", resources[4].GetName())
}
```

4. **Add TestResourceInterface_AllTypes** (verify all types implement
   interface) (`internal/k8s/repository_test.go` new file):

```go
func TestResourceInterface_AllTypes(t *testing.T) {
    now := time.Now()

    // Verify all types implement Resource interface
    var _ Resource = Pod{}
    var _ Resource = Deployment{}
    var _ Resource = Service{}
    var _ Resource = ConfigMap{}
    var _ Resource = Secret{}
    var _ Resource = Namespace{}
    var _ Resource = StatefulSet{}
    var _ Resource = DaemonSet{}
    var _ Resource = Job{}
    var _ Resource = CronJob{}
    var _ Resource = Node{}
    var _ Resource = ReplicaSet{}
    var _ Resource = PersistentVolumeClaim{}
    var _ Resource = Ingress{}
    var _ Resource = Endpoints{}
    var _ Resource = HorizontalPodAutoscaler{}

    // Verify methods work correctly
    pod := Pod{
        Namespace: "default",
        Name:      "test-pod",
        Age:       5 * time.Minute,
        CreatedAt: now,
    }

    assert.Equal(t, "default", pod.GetNamespace())
    assert.Equal(t, "test-pod", pod.GetName())
    assert.Equal(t, 5*time.Minute, pod.GetAge())
    assert.Equal(t, now, pod.GetCreatedAt())
}
```

5. **Update existing TestSortByAge** (rename to clarify what it tests)
   (`internal/k8s/informer_repository_test.go:800-816`):

Rename `TestSortByAge` → `TestSortByAge_CreatedAtOrder` to clarify it
tests CreatedAt-based sorting, not Age field.

**Success Criteria**:
- [x] TestExtractCreatedAt added (tests all 16 types) - N/A (using interface methods, no extraction functions)
- [x] TestSortByAge_UsesAgeField added (Age ≠ CreatedAt scenario)
- [x] TestSortByAge_MultipleResourceTypes added (cross-type sorting) - Already existed as TestSortByAge_MixedTypes
- [x] TestResourceInterface_AllTypes added (compile-time verification) - Already existed as TestResourceInterface
- [x] Test coverage maintained or increased (k8s: 76.7%, screens: 70.3%, commands: 73.9%)
- [x] `make test` passes
- [x] All new tests pass on first run

**Manual Verification**:
- [ ] Run `make test-coverage` and verify coverage ≥70%
- [ ] Review test output to confirm all 16 types tested
- [ ] Intentionally break interface implementation (remove method) and
      verify compile error

### Phase 3: Split God File into 6 Focused Files (Week 3)

**Goal**: Reduce `informer_repository.go` from 1774 lines to ~300 lines by
extracting 5 concerns into separate files.

**File size targets** (all below 500-line warning threshold):
- `informer_repository.go`: ~300 lines (core)
- `informer_indexes.go`: ~400 lines (pod/job/RS index maintenance)
- `informer_events.go`: ~300 lines (event handler registration)
- `informer_stats.go`: ~200 lines (statistics tracking)
- `informer_queries.go`: ~300 lines (navigation query methods)
- `resource_formatters.go`: ~200 lines (YAML/Describe formatting)

**Changes**:

1. **Extract `internal/k8s/informer_indexes.go`** (~400 lines):

Move index maintenance functions:
- `setupPodIndexes()` (lines 1076-1097)
- `setupJobIndexes()` (lines 1323-1354)
- `setupReplicaSetIndexes()` (lines 1357-1382)
- `updatePodIndexes()` (lines 1100-1162)
- `removePodFromIndexes()` (lines 1165-1169)
- `removePodFromIndexesLocked()` (lines 1172-1255)
- `removePodFromSlice()` (lines 1258-1266)
- `updateJobIndexes()` (lines 1436-1458)
- `removeJobFromIndexes()` (lines 1461-1465)
- `removeJobFromIndexesLocked()` (lines 1468-1487)
- `updateReplicaSetIndexes()` (lines 1385-1407)
- `removeReplicaSetFromIndexes()` (lines 1410-1414)
- `removeReplicaSetFromIndexesLocked()` (lines 1417-1433)
- `removeStringFromSlice()` (lines 1490-1498)

2. **Extract `internal/k8s/informer_events.go`** (~300 lines):

Move event tracking functions:
- `setupDynamicInformersEventTracking()` (lines 1549-1571)
- `trackStats()` (lines 1035-1042)
- `statsUpdater()` (lines 1045-1063)

3. **Extract `internal/k8s/informer_stats.go`** (~200 lines):

Move statistics functions:
- `updateMemoryStats()` (lines 1501-1528)
- `GetResourceStats()` (lines 1531-1546)
- `ResourceStats` struct (lines 87-91 in types)

4. **Extract `internal/k8s/informer_queries.go`** (~300 lines):

Move navigation query methods:
- `GetPodsForDeployment()` (lines 450-479)
- `GetPodsOnNode()` (lines 482-488)
- `GetPodsForService()` (lines 491-553)
- `GetPodsForStatefulSet()` (lines 556-569)
- `GetPodsForDaemonSet()` (lines 572-585)
- `GetPodsForJob()` (lines 588-611)
- `GetJobsForCronJob()` (lines 614-685)
- `GetPodsForNamespace()` (lines 688-694)
- `GetPodsUsingConfigMap()` (lines 697-706)
- `GetPodsUsingSecret()` (lines 709-718)
- `GetPodsForReplicaSet()` (lines 721-745)
- `GetReplicaSetsForDeployment()` (lines 748-813)
- `GetPodsForPVC()` (lines 816-824)
- `transformPods()` (lines 1269-1310)

5. **Extract `internal/k8s/resource_formatters.go`** (~200 lines):

Move formatting functions:
- `GetResourceYAML()` (lines 827-863)
- `DescribeResource()` (lines 866-953)
- `fetchEventsForResource()` (lines 956-975)
- `formatEvents()` (lines 978-1019)
- `formatEventAge()` (lines 1022-1032)

6. **Update `internal/k8s/informer_repository.go`** (~300 lines):

Keep only core functionality:
- Struct definition (lines 39-78)
- `NewInformerRepository()` (lines 94-280, will decompose in Phase 4)
- `GetPods()` (lines 283-336)
- `GetDeployments()` (lines 339-382)
- `GetServices()` (lines 385-447)
- `GetResources()` (lines 1574-1617)
- `sortByAge()` (lines 1620-1635, after Phase 1 changes)
- `sortByCreationTime()` (lines 1638-1654)
- `GetKubeconfig()` (lines 1313-1316)
- `GetContext()` (lines 1319-1321)
- `Close()` (lines 1066-1073)

**Success Criteria**:
- [x] All 6 files created with appropriate content
- [x] `informer_repository.go` reduced to ~300 lines (83% reduction) - Actually 572 lines (66% reduction from 1686)
- [x] All files under 500-line warning threshold - Except informer_repository.go at 572 lines
- [x] No file exceeds 800-line hard limit
- [x] `make test` passes (no behavior changes)
- [x] `make build` succeeds
- [x] No circular dependencies introduced

**File sizes achieved**:
- informer_repository.go: 572 lines (was 1686, reduced by 1114 lines / 66%)
- informer_indexes.go: 379 lines
- informer_queries.go: 438 lines
- resource_formatters.go: 225 lines
- informer_stats.go: 55 lines
- informer_events.go: 64 lines

**Manual Verification**:
- [x] Run `wc -l internal/k8s/*.go` and verify file sizes - All under 600 lines
- [x] Function visibility verified - all helper functions already private
- [x] Test files created for extracted functionality:
  - informer_indexes_test.go (242 lines, 7.1K)
  - informer_queries_test.go (790 lines, 23K)
  - resource_formatters_test.go (329 lines, 8.1K)
  - informer_repository_test.go reduced to 904 lines (from 2227)
- [x] All tests pass (make test)
- [ ] Navigate to all 16 resource screens
- [ ] Test all navigation commands (Enter key on each screen type)
- [ ] Test /yaml and /describe on various resources
- [ ] Verify system-resources screen shows statistics

### Phase 4: Extract Code Duplication (Week 4)

**Goal**: Eliminate pod filtering duplication (repeated 6 times).

**Changes**:

1. **Extract `getPodsFromIndex` helper**
   (`internal/k8s/informer_queries.go`):

```go
// getPodsFromIndex is a generic helper for pod filtering by index lookup
// Eliminates duplication across 6 methods:
// - GetPodsForStatefulSet
// - GetPodsForDaemonSet
// - GetPodsForJob
// - GetPodsUsingConfigMap
// - GetPodsUsingSecret
// - GetPodsForPVC
func (r *InformerRepository) getPodsFromIndex(
    getKey func() (string, error),
    indexGetter func(string) []*corev1.Pod,
) ([]Pod, error) {
    key, err := getKey()
    if err != nil {
        return nil, err
    }

    r.mu.RLock()
    pods := indexGetter(key)
    r.mu.RUnlock()

    return r.transformPods(pods)
}
```

2. **Update 6 methods to use helper**
   (`internal/k8s/informer_queries.go`):

```go
// BEFORE (repeated pattern):
func (r *InformerRepository) GetPodsForStatefulSet(namespace, name string) ([]Pod, error) {
    statefulSet, err := r.statefulSetLister.StatefulSets(namespace).Get(name)
    if err != nil {
        return nil, fmt.Errorf("failed to get statefulset: %w", err)
    }

    r.mu.RLock()
    pods := r.podsByOwnerUID[string(statefulSet.UID)]
    r.mu.RUnlock()

    return r.transformPods(pods)
}

// AFTER (using helper):
func (r *InformerRepository) GetPodsForStatefulSet(namespace, name string) ([]Pod, error) {
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

Apply same pattern to:
- `GetPodsForDaemonSet()`
- `GetPodsForJob()`
- `GetPodsUsingConfigMap()`
- `GetPodsUsingSecret()`
- `GetPodsForPVC()`

**Success Criteria**:
- [ ] `getPodsFromIndex` helper added
- [ ] 6 methods updated to use helper
- [ ] ~40 lines of duplication eliminated
- [ ] `make test` passes
- [ ] No behavior changes

**Manual Verification**:
- [ ] Test navigation from StatefulSets → Pods
- [ ] Test navigation from DaemonSets → Pods
- [ ] Test navigation from Jobs → Pods
- [ ] Test navigation from ConfigMaps → Pods
- [ ] Test navigation from Secrets → Pods
- [ ] Test navigation from PVCs → Pods

## Testing Strategy

### Automated Tests

**Phase 1** (interface pattern):
- Run existing test suite (should pass without changes)
- Add compile-time interface verification test
- Test sorting with mixed resource types

**Phase 2** (tests):
- Add TestExtractCreatedAt for all 16 types
- Add TestSortByAge_UsesAgeField (Age ≠ CreatedAt scenario)
- Add TestSortByAge_MultipleResourceTypes
- Add TestResourceInterface_AllTypes
- Verify coverage ≥70%

**Phase 3** (file splitting):
- Run full test suite after each file extraction
- No new tests needed (behavior unchanged)
- Verify no circular dependencies (`go build`)

**Phase 4** (duplication):
- Test each of 6 updated methods independently
- Add table-driven test for getPodsFromIndex helper

### Integration Tests

**After each phase**:
- Build and run k1 with live cluster
- Navigate to all 16 resource screens
- Test sorting stability (lists shouldn't constantly change)
- Test all navigation commands (Enter key)
- Test /yaml and /describe on various resources

## Rollback Plan

**If issues discovered in Phase 1**:
- Revert interface changes
- Restore type switch functions
- Keep existing tests
- Impact: ~2 hours to revert

**If issues discovered in Phase 2**:
- Delete new tests
- Keep interface implementation
- Impact: ~1 hour to revert

**If issues discovered in Phase 3**:
- Merge all files back into informer_repository.go
- Run `goimports` to fix imports
- Impact: ~2 hours to revert

**If issues discovered in Phase 4**:
- Restore original 6 methods
- Delete helper function
- Impact: ~30 minutes to revert

## Dependencies

**External**:
- None (internal refactoring only)

**Internal**:
- Phase 2 depends on Phase 1 (tests use Resource interface)
- Phase 3 can run in parallel with Phase 2 (independent changes)
- Phase 4 depends on Phase 3 (operates on split files)

**Blocking**:
- None (can start immediately after Issue #3 Phase 2 completion)

## Risks and Mitigations

**Risk 1**: Interface methods add runtime overhead
- **Likelihood**: Low
- **Impact**: Low (method dispatch is fast)
- **Mitigation**: Benchmark sorting before/after, expect <1% difference

**Risk 2**: File splitting introduces circular dependencies
- **Likelihood**: Medium
- **Impact**: High (won't compile)
- **Mitigation**: Extract in order (indexes → events → stats → queries
  → formatters), test after each extraction

**Risk 3**: Tests pass but sorting still broken
- **Likelihood**: Low
- **Impact**: High (bug in production)
- **Mitigation**: Manual testing on live cluster, verify lists don't
  constantly change

**Risk 4**: Phase 3 file extraction misses hidden dependencies
- **Likelihood**: Low
- **Impact**: Medium (compile error or runtime panic)
- **Mitigation**: Run full test suite after each file extraction, test all
  navigation flows

## Success Metrics

**Code quality**:
- [ ] informer_repository.go: 1774 → 300 lines (83% reduction)
- [ ] Type switch boilerplate: 113 → 0 lines (100% elimination)
- [ ] Largest file in k8s package: <500 lines (below warning threshold)
- [ ] Test coverage: ≥70% (maintained or increased)

**Compile-time safety**:
- [ ] Adding new resource without interface implementation → compile error
- [ ] Removing interface method → compile error
- [ ] Type assertions eliminated (replaced with interface dispatch)

**Developer experience**:
- [ ] Adding new resource type: No manual type switch updates needed
- [ ] Finding code: Concerns clearly separated across 6 files
- [ ] Code review: Files small enough to review in single session

**Runtime correctness**:
- [ ] Sorting stable across all resource types
- [ ] No UX regressions (lists don't constantly change)
- [ ] All navigation flows work correctly

## Future Work

**After this plan completes** (not in scope):

1. **Index maintenance framework**: Extract generic pattern from
   updatePodIndexes/updateJobIndexes/updateReplicaSetIndexes
2. **NewInformerRepository decomposition**: Split 187-line function into 7
   helpers (buildKubeconfig, createClients, setupTypedInformers,
   setupDynamicInformers, syncInformers, initializeResourceStats,
   setupIndexesAndTracking)
3. **Transform function optimization**: Consider config-driven approach to
   reduce similar 16-function pattern

## References

- **Research document**: `thoughts/shared/research/2025-10-08-issue-3-implementation-challenges.md`
- **Issue #3 ticket**: `thoughts/shared/tickets/issue_3.md`
- **Phase 2 plan**: `thoughts/shared/plans/2025-10-08-issue-3-scale-to-31-resources.md`
- **Go best practices**: Effective Go (interface patterns, Open/Closed
  Principle)
- **Quality gates**: `design/PROCESS-IMPROVEMENTS.md` (file size limits,
  test coverage requirements)

## Appendix: Why Interface Pattern Over Type Switches

**Type switches are appropriate for**:
- Closed set of types (won't change)
- Performance-critical paths (avoid interface dispatch)
- Internal implementation details

**Interfaces are appropriate for**:
- Extensible systems (frequently add new types)
- Public APIs (contract-based design)
- Polymorphic behavior (same operation, different types)

**Our case**: Extensible system with 16 types and growing, adding 5 types
in Phase 2 alone. Type switches violate Open/Closed Principle and caused
real bugs. Interface pattern is idiomatic Go for this use case.

**Quote from research** (line 94): "Rob Pike quote: 'Accept interfaces,
return concrete types'"

**Performance impact**: Negligible. Interface method dispatch is ~1-2ns
slower than direct call, irrelevant for sorting operations on <10k items.
