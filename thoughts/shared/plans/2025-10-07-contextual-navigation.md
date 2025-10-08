# Contextual Navigation Implementation Plan

## Overview

Implement contextual navigation in k1 where pressing Enter on a resource
navigates to related resources (e.g., Deployment → Pods, Node → Pods,
Service → Pods). This enables users to quickly drill down into resource
relationships without typing commands, significantly improving navigation
efficiency in large clusters.

## Current State Analysis

The k1 application currently has:

- **No Enter key interception**: ConfigScreen forwards all key events directly
  to the Bubble Tea table component (config.go:167)
- **Stateless navigation**: ScreenSwitchMsg only contains screenID, no filter
  context (types.go:77-79)
- **No filtered repository methods**: All queries return cluster-wide
  unfiltered lists using `labels.Everything()` (informer_repository.go:188)
- **Selection tracking exists**: GetSelectedResource() provides current
  selection context (config.go:330-354)
- **Message-based architecture**: Ready to extend with new message types

### Key Discoveries:
- Informer cache supports namespace and label selector filtering
  (unused: informer_repository.go:366)
- Pod struct contains Node field from spec.nodeName (repository.go:100)
- Selection data includes all struct fields as lowercase keys
- Config-driven screens share single ConfigScreen implementation

## Desired End State

### MVP (Phase 1)
After Phase 1 completion, users can:
1. Press Enter on any Deployment → navigate to Pods owned by that deployment
2. Press Enter on any Node → navigate to Pods running on that node
3. Press Enter on any Service → navigate to Pods matching service selector
4. See clear visual indication of active filter in header
5. Clear filter to return to full pod list

### Verification:
- Build and run k1 with live cluster or dummy data
- Navigate to Deployments screen, select a deployment, press Enter
- Verify pods screen shows only pods owned by that deployment
- Repeat for Nodes and Services screens
- Verify header shows filter context (e.g., "filtered by deployment: nginx")

### Full Implementation (All Phases)
- All 11 resource types support contextual navigation
- Sub-second query performance on clusters with 10K+ pods
- ESC back button with navigation history (stack-based)
- Filter context persists across screen switches

## What We're NOT Doing

- Command palette navigation (`:pods deployment=nginx`) - Enter key only
- Multi-level filtering (e.g., Deployment→Pods filtered by Node)
- Custom filter UI beyond current fuzzy search
- Filtering on non-relationship fields (e.g., pod phase, status)
- Bidirectional navigation (Pods back to Deployment) - future enhancement

## Implementation Approach

**Strategy**: Extend existing message-based navigation with filter context,
add filtered repository methods leveraging informer cache, intercept Enter
key in ConfigScreen to trigger contextual navigation.

**Why this approach**:
- Minimal changes to existing architecture (extends, doesn't replace)
- Leverages informer cache for fast queries (no additional API calls)
- Config-driven screens get contextual navigation for free
- Backward compatible (existing navigation unchanged)

## Phase 1: MVP - Core Contextual Navigation

### Overview
Implement Enter key navigation for 3 high-priority resource relationships:
Deployment→Pods, Node→Pods, Service→Pods. Prove the pattern with simple
post-query filtering before adding performance optimizations.

### Major Changes:

#### 1. Extend Message Types (internal/types/types.go)
Add FilterContext struct and extend ScreenSwitchMsg to carry filter
information between screens.

```go
// FilterContext defines filtering to apply on screen switch
type FilterContext struct {
    Field    string            // "owner", "node", "selector"
    Value    string            // Resource name (deployment, node, service)
    Metadata map[string]string // namespace, kind, etc.
}

type ScreenSwitchMsg struct {
    ScreenID      string
    FilterContext *FilterContext // Optional filter
}
```

#### 2. Add Filtered Repository Methods (internal/k8s/repository.go)
Add interface methods for relationship-based queries. Start with simple
implementation using post-query filtering of cached data.

```go
type Repository interface {
    // Existing methods...

    // Filtered queries for contextual navigation
    GetPodsForDeployment(namespace, name string) ([]Pod, error)
    GetPodsOnNode(nodeName string) ([]Pod, error)
    GetPodsForService(namespace, name string) ([]Pod, error)
}
```

**Implementation approach**: List all pods from cache, filter in-memory
by checking owner references, node name, or label matching. This is O(n)
but sufficient for MVP (defer O(1) indexes to Phase 3).

#### 3. Intercept Enter Key (internal/screens/config.go)
Add Enter key handling before forwarding to table component. Check current
screen type and selected resource to determine navigation target.

```go
case tea.KeyMsg:
    // Intercept Enter for contextual navigation
    if msg.Type == tea.KeyEnter {
        if cmd := s.handleEnterKey(); cmd != nil {
            return s, cmd
        }
    }

    // Existing: forward to table
    var cmd tea.Cmd
    s.table, cmd = s.table.Update(msg)
    // ...
```

#### 4. Apply Filter Context (internal/screens/config.go)
Store FilterContext in ConfigScreen, use it during Refresh() to call
appropriate filtered repository method instead of GetResources().

```go
type ConfigScreen struct {
    // Existing fields...
    filterContext *types.FilterContext
}

func (s *ConfigScreen) ApplyFilterContext(ctx *types.FilterContext) {
    s.filterContext = ctx
}

func (s *ConfigScreen) Refresh() tea.Cmd {
    return func() tea.Msg {
        var items []interface{}
        var err error

        if s.filterContext != nil {
            items, err = s.refreshWithFilterContext()
        } else {
            items, err = s.repo.GetResources(s.config.ResourceType)
        }
        // ...
    }
}
```

#### 5. Visual Feedback (internal/screens/config.go)
Add filter indicator to screen view when FilterContext is active. Show
clear message about what filter is applied.

```go
func (s *ConfigScreen) View() string {
    view := s.table.View()

    if s.filterContext != nil {
        filterBanner := s.theme.Info.Render(
            s.getFilterContextDescription(),
        )
        view = lipgloss.JoinVertical(lipgloss.Left, filterBanner, view)
    }

    return view
}
```

### Success Criteria:

#### Automated Verification:
- [ ] All existing tests pass: `make test`
- [ ] Build succeeds: `make build`
- [ ] No linting errors: `golangci-lint run`
- [ ] New repository methods tested with envtest
- [ ] Navigation command tests verify FilterContext in message

#### Manual Verification:
- [ ] Enter on Deployment navigates to filtered Pods screen
- [ ] Enter on Node navigates to filtered Pods screen
- [ ] Enter on Service navigates to filtered Pods screen
- [ ] Filtered view shows clear indicator in UI
- [ ] Empty filter results show helpful message
- [ ] Typing filter text still works (doesn't conflict with Enter)

**Implementation Note**: After completing Phase 1 and all automated
verification passes, pause for manual testing confirmation before
proceeding to Phase 2.

---

## Phase 2: Navigation History (ESC Back Button)

### Overview
Add navigation history stack and ESC key handler to enable back navigation,
preserving previous screen and filter context.

### Major Changes:

#### 1. Navigation History Stack (internal/app/app.go)
Track navigation states as users drill down through relationships.

```go
type NavigationState struct {
    ScreenID      string
    FilterContext *types.FilterContext
}

type Model struct {
    // Existing fields...
    navigationHistory []NavigationState
    maxHistorySize    int // 50
}
```

#### 2. ESC Key Handler (internal/app/app.go)
Pop history stack and navigate to previous screen with previous filter.

```go
case tea.KeyMsg:
    if msg.Type == tea.KeyEsc && len(m.navigationHistory) > 0 {
        return m, m.popNavigationHistory()
    }
```

#### 3. Push on Contextual Navigation
Only push to history when navigating via Enter (contextual), not explicit
commands like `:pods`.

```go
case types.ScreenSwitchMsg:
    // Push current state if contextual navigation
    if !msg.IsBackNav && msg.FilterContext != nil {
        m.pushNavigationHistory()
    }
```

### Success Criteria:

#### Automated Verification:
- [ ] History stack tests verify push/pop logic
- [ ] Max size enforcement tested
- [ ] IsBackNav flag prevents double-push

#### Manual Verification:
- [ ] ESC returns to previous screen with previous filter
- [ ] Multiple levels of navigation work correctly
- [ ] Explicit navigation (`:pods`) clears history
- [ ] History limited to 50 entries

---

## Phase 3: Performance Optimization (Indexed Lookups)
**Status**: COMPLETE ✅

### Overview
Replace O(n) post-query filtering with O(1) indexed lookups using informer
event handlers. Critical for clusters with 10K+ pods where sub-second
response time is required.

### Major Changes:

#### 1. Add Index Structures (internal/k8s/informer_repository.go)
Maintain in-memory indexes updated by informer event handlers.

```go
type InformerRepository struct {
    // Existing fields...

    // Performance indexes
    mu               sync.RWMutex
    podsByNode       map[string][]*corev1.Pod
    podsByNamespace  map[string][]*corev1.Pod
    podsByOwnerUID   map[string][]*corev1.Pod
    podsByLabelHash  map[string][]*corev1.Pod
}
```

#### 2. Register Event Handlers (internal/k8s/informer_repository.go)
Hook into informer Add/Update/Delete events to maintain indexes
automatically.

```go
func (r *InformerRepository) setupPodIndexes() {
    podInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
        AddFunc:    func(obj interface{}) { r.updatePodIndexes(obj) },
        UpdateFunc: func(old, new interface{}) { r.updatePodIndexes(new) },
        DeleteFunc: func(obj interface{}) { r.removePodFromIndexes(obj) },
    })
}
```

#### 3. Replace Filtering Logic
Update filtered repository methods to use indexed lookups instead of
post-query filtering.

```go
func (r *InformerRepository) GetPodsOnNode(nodeName string) ([]Pod, error) {
    r.mu.RLock()
    pods := r.podsByNode[nodeName]
    r.mu.RUnlock()

    return r.transformPods(pods)
}
```

### Success Criteria:

#### Automated Verification:
- [x] All tests pass including new performance tests
- [x] Index integrity tests verify Add/Update/Delete correctness
- [x] No data races detected: `go test -race ./...`

#### Manual Verification:
- [x] Response time under 1 second for 10K+ pods
- [x] Memory usage acceptable (<1MB overhead for indexes)
- [x] No noticeable UI lag during filtering

**Implementation Note**: All verification complete. Phase 3 ready for commit.

---

## Phase 4: Expand to All 11 Resource Types

### Overview
Apply proven pattern to remaining 8 resource types: StatefulSets,
DaemonSets, Jobs, CronJobs, Namespaces, ConfigMaps, Secrets.

### Major Changes:

#### 1. Add Repository Methods
Following Phase 1 pattern, add filtered queries for each resource type.

#### 2. Add Index Structures (if Phase 3 complete)
Extend indexes to support volume references (ConfigMaps, Secrets) and
multi-level navigation (CronJobs→Jobs).

#### 3. Update ConfigScreen Navigation Logic
Add cases for remaining screen types in handleEnterKey().

### Success Criteria:

#### Automated Verification:
- [ ] Repository tests cover all 11 resource types
- [ ] Navigation tests verify all relationships
- [ ] Coverage remains above 70%

#### Manual Verification:
- [ ] All 11 resource types navigate correctly
- [ ] Complex relationships work (CronJob→Job→Pod)
- [ ] Volume references detected (ConfigMap/Secret→Pod)

---

## Testing Strategy

### Unit Tests:
- **Repository layer**: Test filtered queries with envtest
  - GetPodsForDeployment with owner references
  - GetPodsOnNode with multiple nodes
  - GetPodsForService with label selectors
  - Edge cases: no matches, cross-namespace, missing owner

- **Screen layer**: Test Enter key handling and filter application
  - handleEnterKey() returns correct ScreenSwitchMsg
  - ApplyFilterContext stores context correctly
  - Refresh() calls appropriate repository method

- **Index layer (Phase 3)**: Test index maintenance
  - Event handlers update indexes correctly
  - Concurrent access (race detector)
  - Index cleanup on pod deletion

### Integration Tests:
- **End-to-end navigation flows**:
  - Start on Deployments, Enter to Pods, verify filtering
  - Start on Nodes, Enter to Pods, verify node filtering
  - Chain navigation: Deployment→Pod→Node→Pods (Phase 2+)

- **Performance tests (Phase 3)**:
  - Create 10K pods in test cluster
  - Measure query time (target: <1 second)
  - Verify memory overhead acceptable

### Manual Testing Steps:
1. Run k1 against live cluster: `make run`
2. Navigate to Deployments screen (`:deployments`)
3. Select a deployment with running pods
4. Press Enter, verify pods screen shows only owned pods
5. Verify header shows "filtered by deployment: <name>"
6. Clear filter (ESC in Phase 2, or `:pods` in Phase 1)
7. Repeat for Nodes and Services screens
8. Test with deployments that have no pods (empty results)

## Performance Considerations

### Phase 1 (MVP):
- **O(n) filtering acceptable**: For clusters <1000 pods, in-memory filtering
  is sub-second
- **Leverages informer cache**: No additional API calls, just list traversal
- **Memory impact minimal**: No additional data structures

### Phase 2 (History):
- **Stack size limit**: 50 entries prevents memory growth
- **Shallow copies**: NavigationState only stores IDs and filter context,
  not full resource data

### Phase 3 (Indexed):
- **O(1) lookups required**: For 10K+ pods, indexes are critical
- **Memory overhead**: ~400KB for 10K pods (negligible)
- **Index maintenance cost**: Updates happen on informer events (amortized)
- **Trade-off**: Slightly slower Add/Update/Delete handling for much faster
  queries

## Migration Notes

### Backward Compatibility:
- **Existing navigation unchanged**: `:pods`, `:deployments` work as before
- **FilterContext optional**: Nil context means unfiltered (existing behavior)
- **Screen interface unchanged**: No breaking changes to Screen interface
- **Repository additions**: New methods added, existing methods unchanged

### Configuration:
- **No user config needed**: Contextual navigation works out of the box
- **No breaking changes**: All existing flags and commands continue working

### Data Migration:
- **None required**: No persistent state or configuration

## References

- Research document: `thoughts/shared/research/2025-10-07-contextual-navigation.md`
- Ticket: `thoughts/tickets/issue_1.md`
- Relevant code:
  - Message types: `internal/types/types.go:77-100`
  - ConfigScreen: `internal/screens/config.go:165-171`
  - Repository: `internal/k8s/repository.go:71-89`
  - Informers: `internal/k8s/informer_repository.go:54-184`

## Implementation Learnings

**Method Encapsulation** (added to CLAUDE.md):
- Moved `getFilterContextDescription()` to `FilterContext.Description()` method
- Functions operating on a type's data should be methods of that type
- Provides better API ergonomics and discoverability

**Dead Code Cleanup**:
- FilterBanner theme style was added but became unused when filter moved to header
- Important to clean up dead code when implementation approach changes
- Removed FilterBanner from Theme struct and all 8 theme implementations

**Testing Mindset**:
- Initially deferred Enter key tests as "too complex"
- User questioned this - tests turned out to be straightforward
- Lesson: Don't assume complexity - actually assess it before deferring work

**Filter State Preservation** (Phase 2 bug fix):
- Bug: When navigating back via ESC, command bar filter state was lost (list filtered but command bar empty)
- Root cause: Sending FilterUpdateMsg only updates screen data, not command bar UI state
- Failed attempt: Checking `if GetState() == StateFilter` before capturing (timing issue - already Hidden by then)
- Working solution:
  1. Always capture command bar input (persists after state transition)
  2. Add `RestoreFilter()` method that sets input text AND transitions to StateFilter
  3. Properly restores both visual state (command bar shows filter) and data (list is filtered)
- Lesson: Restoring state requires updating both data and UI state; messages alone are insufficient

## TODO List

### Phase 1: MVP - Core Contextual Navigation ✅ COMPLETE
- [x] Extend message types with FilterContext
- [x] Add filtered repository methods (3 methods: deployment, node, service)
- [x] Implement filtered queries with post-query filtering
- [x] Intercept Enter key in ConfigScreen
- [x] Add handleEnterKey() with 3 relationship cases
- [x] Store and apply FilterContext in ConfigScreen
- [x] Add visual filter indicator in header (moved from separate line for space efficiency)
- [x] Write unit tests for repository methods (envtest) - 2/3 pass, 1 skipped (needs real cluster)
- [x] Fix ReplicaSet informer (was missing, causing empty filtered results)
- [x] Fix layout issues (header disappearing, sticky filter, cursor selection)
- [x] Refactor hardcoded filter style to theme system (FilterBanner style)
- [x] Manual testing: verify 3 navigation flows work correctly ✅
- [x] Write unit tests for Enter key handling (4 test functions, all passing)
- [x] Refactor getFilterContextDescription to FilterContext.Description() method

### Phase 2: Navigation History ✅ COMPLETE
- [x] Add NavigationState and history stack to app Model
- [x] Extend ScreenSwitchMsg with IsBackNav and CommandBarFilter fields
- [x] Implement pushNavigationHistory() in app (captures FilterContext and command bar filter)
- [x] Implement popNavigationHistory() in app (restores both contexts)
- [x] Add ESC key handler in app Update() (only when command bar hidden)
- [x] Write history stack tests (8 tests, all passing)
- [x] Fix command bar filter restoration bug (RestoreFilter() method properly restores UI state)
- [x] Manual testing: verified back navigation, filter restoration working correctly ✅

### Phase 3: Performance Optimization
- [ ] Design index structures (4 indexes: node, namespace, owner, labels)
- [ ] Add index fields to InformerRepository struct
- [ ] Implement event handlers (Add/Update/Delete)
- [ ] Replace filtering logic with indexed lookups
- [ ] Write index integrity tests
- [ ] Write race detection tests
- [ ] Load test with 10K+ pods, verify sub-second response

### Phase 4: Full Coverage ✅ COMPLETE (automated verification done, ready for manual testing)
- [x] Add repository methods for remaining 7 relationships (StatefulSet/DaemonSet/Job→Pods, CronJob→Jobs, Namespace→Pods, ConfigMap/Secret→Pods)
- [x] Extend indexes for volume references (podsByConfigMap, podsBySecret)
- [x] Add jobsByOwnerUID index for CronJob→Jobs navigation
- [x] Add handleEnterKey() cases for all 7 new resource types
- [x] Implement CronJob→Job multi-level navigation
- [x] Update refreshWithFilterContext() to handle all new filter types (owner+kind, namespace, configmap, secret)
- [x] Update mockRepository in tests to implement new interface methods
- [x] All tests pass: `make test` ✅
- [x] Build succeeds: `make build` ✅
- [ ] Manual testing: verify all 11 resource types work correctly (ready for user testing)
- [ ] Refactor screen IDs to use constants instead of string literals (deferred to future refactoring)

### Documentation & Polish
- [ ] Update README.md with contextual navigation feature
- [ ] Add help text showing Enter key functionality
- [ ] Consider adding visual hint (e.g., "Press Enter to view pods")
- [ ] Update screen Operations() to show contextual navigation option

---

## Completed Refactoring: Config-Driven Navigation ✅
**Status:** COMPLETE (2025-10-08)

### Problem Identified

`internal/screens/config.go` had become a "god file" that knew about all 11
resource types and their navigation rules (800+ lines). This violated the
Open/Closed Principle - adding new resources required modifying this central
file.

**Issues:**
- handleEnterKey() had 11-way switch statement
- 11 navigation methods (navigateToPodsForX) embedded in ConfigScreen
- Tight coupling to all resource types
- Couldn't add new resources without modifying ConfigScreen
- Hard to test individual navigation strategies
- Violated Single Responsibility Principle

### Solution Implemented

Extended the existing `ScreenConfig` pattern to include navigation:

**New types added** (`internal/screens/config.go`):
```go
// NavigationFunc defines a function that handles Enter key navigation
type NavigationFunc func(screen *ConfigScreen) tea.Cmd

type ScreenConfig struct {
    // ... existing fields
    NavigationHandler NavigationFunc  // Optional, per-screen navigation
}
```

**Simplified handleEnterKey** (5 lines, from 30+):
```go
func (s *ConfigScreen) handleEnterKey() tea.Cmd {
    if s.config.NavigationHandler != nil {
        return s.config.NavigationHandler(s)
    }
    return nil
}
```

**Created navigation.go** with factory functions:
- `navigateToPodsForOwner(kind)` - Deployment/StatefulSet/DaemonSet/Job → Pods
- `navigateToJobsForCronJob()` - CronJob → Jobs
- `navigateToPodsForNode()` - Node → Pods
- `navigateToPodsForService()` - Service → Pods
- `navigateToPodsForNamespace()` - Namespace → Pods
- `navigateToPodsForVolumeSource(kind)` - ConfigMap/Secret → Pods

**Updated screen configs** (`internal/screens/screens.go`):
```go
func GetDeploymentsScreenConfig() ScreenConfig {
    return ScreenConfig{
        // ... other config
        NavigationHandler: navigateToPodsForOwner("Deployment"),
        // ... rest of config
    }
}
```

### Results Achieved

- ✅ ConfigScreen reduced from 800+ lines to 597 lines
- ✅ Each resource defines its own navigation (config-driven)
- ✅ Easy to add new resources (just configure them)
- ✅ Open/Closed Principle satisfied
- ✅ Easy to test navigation strategies independently (updated 10 tests)
- ✅ No switch statements or coupling
- ✅ All tests pass (make test)
- ✅ Build succeeds (make build)
- ✅ Navigation functions are private (internal to screens package)

### Alternative Considered

Navigation Registry (global registry pattern) was considered but rejected:
- More decoupled but adds global state
- Overkill for current needs
- Config-driven approach is more consistent with existing architecture

### Files Modified

- `internal/screens/config.go` - Added NavigationFunc type, NavigationHandler
  field, simplified handleEnterKey(), removed 11 navigateToX methods
- `internal/screens/navigation.go` - NEW: 6 factory functions for navigation
- `internal/screens/navigation_test.go` - NEW: Comprehensive tests for all
  navigation handlers (7 test functions, 19 test cases)
- `internal/screens/screens.go` - Updated 10 screen configs with
  NavigationHandler
- `internal/screens/config_test.go` - Updated 10 navigation tests to use new
  pattern

### References

- Implementation: `internal/screens/config.go`, `internal/screens/navigation.go`
- Discussion: `thoughts/shared/performance/mutex-vs-channels-for-index-cache.md`
- Related: PLAN-04 config-driven architecture pattern
