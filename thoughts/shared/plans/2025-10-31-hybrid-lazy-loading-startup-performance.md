---
date: 2025-10-31
author: Claude (claude-sonnet-4.5)
status: phase-2-complete
branch: feat/startup-performance
related_research: thoughts/shared/research/2025-10-31-startup-performance-vs-k9s.md
target_improvement: 99.5% startup time reduction (29.5s → 142ms) ✅ ACHIEVED
approach: non-blocking startup with parallel background sync
current_phase: Phase 2 complete, Phase 3 (optional polish) available
---

# Non-Blocking Startup Implementation Plan

## Overview

Implement non-blocking startup for k1 startup performance. Target:
**~1-2 seconds to UI interactive**, with all informers syncing in
parallel in the background. Screens show loading messages if informer
not yet synced, then populate fully once sync completes (all-or-nothing
per screen).

## Current State Analysis

**Validated bottleneck** (from research document):
- k1 startup: 29.5s (10x slower than k9s)
- 99% of time spent in informer sync (29.3s out of 29.5s)
- UI blocks waiting for 6 Tier 1 resources + 6 typed informers

**Key discovery from codebase research**:
- **Background sync infrastructure already exists!** Dynamic informers
  already have goroutines that sync in background (lines 318-350)
- The ONLY bottleneck is the blocking `wg.Wait()` call (lines 356-364)
- `IsInformerSynced()` method already exists to check sync status
- Screens have loading message patterns we can reuse
- Dynamic factory's `Start()` is idempotent - safe to call before
  returning

## Desired End State

1. **UI appears in 1-2 seconds** (doesn't wait for informer sync)
2. **All informers start at startup** (in parallel, in background)
3. **Screens check sync status** and show "Loading..." if not ready
4. **All-or-nothing screen loading** - each screen waits for full sync,
   then populates completely (no incremental loading - informers use
   LIST-then-WATCH pattern)
5. **No startup blocking** - UI interactive immediately while
   informers sync in background

**Verification**:
```bash
# Measure startup time
time go run cmd/k1/main.go -log-file k1.log -log-level debug

# Expected log output:
# Config loaded: ~100ms
# Repository pool created: ~13ms
# Informers started (non-blocking): ~10ms
# UI starting: ~100ms
# Total: 1-2 seconds ← SUCCESS
#
# [Background] Informers syncing in parallel...
# [5-8s later] Pods synced
# [10-15s later] All informers synced

# When navigating to :pods at 2s (before pods informer synced):
# "Loading Pods..." (message appears)
# [waits 3-6s for full LIST operation to complete]
# ALL pods appear at once (all-or-nothing)

# When navigating to :pods at 10s (after pods informer synced):
# Immediately shows ALL pods (no waiting, instant)

# When navigating :pods → :deployments at 5s:
# Pods: instant (already synced)
# Deployments: "Loading..." → waits → all appear at once
```

## What We're NOT Doing

- NOT implementing lazy loading (informers DO start at startup)
- NOT implementing incremental/progressive data loading (impossible -
  informers use LIST-then-WATCH pattern, cache populates all-at-once)
- NOT changing tier system (keep existing Tier 1/2/3 configuration)
- NOT implementing splash screen (can add later if desired)
- NOT changing Repository interface (already perfect)
- NOT implementing cache warming (future optimization)
- NOT adding new loading state UI components (use existing patterns)

## Technical Constraint: All-or-Nothing Loading

**Why screens can't show incremental data:**

Kubernetes informers follow the LIST-then-WATCH pattern:
1. **Initial LIST**: Fetch ALL resources from API server (~5-8s for
   847 pods)
2. **Populate cache**: All items added to cache in single operation
3. **HasSynced = true**: Only after full LIST completes
4. **WATCH**: Then incremental updates via watch stream

Before `HasSynced()` returns true:
- Cache is empty (lister returns 0 items)
- No partial data available
- Screen must wait for full sync

After `HasSynced()` returns true:
- Cache has ALL items
- Screen shows full dataset immediately

This is why POC for incremental loading failed - the informer
architecture doesn't support it.

## Implementation Approach

**Strategy**: Keep informer startup logic, remove blocking
`WaitForCacheSync()` calls. This is simpler than lazy loading because:

1. Informers still start at app initialization (familiar pattern)
2. Only remove the blocking waits (lines 231-373 in
   informer_repository.go)
3. Add loading state checks to screens (check `IsInformerSynced()`)
4. UI shows immediately while informers sync in background

**Key insight**: The bottleneck isn't starting informers - it's
**waiting for them**. By removing the waits, we get:
- Startup time: 29.5s → 1-2s (informers start, but we don't wait)
- Time to Pods data: 6-10s (1-2s UI + 5-8s Pods sync in background)
- User perception: Instant UI, screens load as informers complete
  (much better UX than 30s black screen)

**Trade-off acceptance**: Screens may show "Loading..." for 5-8s if
accessed before sync complete, then all data appears at once. This is
acceptable because:
- User sees immediate UI feedback (not black screen for 30s)
- Loading message provides clear progress indication
- Most users won't navigate that fast (sync completes before they do)
- After initial sync, everything is instant (cached)
- All-or-nothing loading is unavoidable (informer LIST-then-WATCH
  pattern)

---

## Phase 1: Remove Typed Informers (Eliminate Legacy Duplication)

### Overview
Remove typed informers (pods, deployments, services, etc.) to
eliminate redundancy and simplify to 100% dynamic informer
architecture. This unblocks lazy loading by removing the mandatory
typed informer sync at startup.

### Changes Required

#### 1. Remove Typed Informer Creation
**File**: `internal/k8s/informer_repository.go`

**Lines 35-76**: Remove typed informer fields from struct:
```go
// DELETE these fields:
// factory           informers.SharedInformerFactory
// podLister         v1.PodLister
// deploymentLister  appsv1.DeploymentLister
// serviceLister     v1.ServiceLister
// statefulSetLister appsv1.StatefulSetLister
// daemonSetLister   appsv1.DaemonSetLister
// replicaSetLister  appsv1.ReplicaSetLister

// KEEP only:
type InformerRepository struct {
    clientset      *kubernetes.Clientset  // For non-informer API calls
    dynamicClient  dynamic.Interface
    dynamicFactory dynamicinformer.DynamicSharedInformerFactory
    resources      map[ResourceType]ResourceConfig
    dynamicListers map[schema.GroupVersionResource]cache.GenericLister
    // ... other fields
}
```

**Lines 152-192**: Remove typed informer initialization in
`NewInformerRepositoryWithProgress()`:
```go
// DELETE all of this:
// factory := informers.NewSharedInformerFactory(clientset,
//            InformerResyncPeriod)
// podInformer := factory.Core().V1().Pods()
// deploymentInformer := factory.Apps().V1().Deployments()
// ... etc

// KEEP only dynamic factory:
dynamicFactory := dynamicinformer.NewDynamicSharedInformerFactory(
    dynamicClient,
    InformerResyncPeriod,
)
```

**Lines 196-230**: Remove typed informer startup code:
```go
// DELETE:
// factory.Start(ctx.Done())
// All typed informer creation logic

// KEEP:
dynamicFactory.Start(ctx.Done())  // Idempotent
```

**Lines 231-307**: **REMOVE ENTIRE TYPED SYNC BLOCK** (this is the
20-25s bottleneck):
```go
// DELETE entire section:
// cache.WaitForCacheSync(syncCtx.Done(),
//     podInformer.HasSynced,
//     deploymentInformer.HasSynced,
//     ...
// )
```

#### 2. Remove Typed Lister Usage
**File**: `internal/k8s/informer_repository.go`

Search for any methods that use typed listers and replace with dynamic
lister access:

**Lines 400-550** (approximate): Update query methods like:
```go
// BEFORE:
func (r *InformerRepository) GetPods(namespace string)
    ([]*v1.Pod, error) {
    if namespace == "" {
        return r.podLister.List(labels.Everything())
    }
    return r.podLister.Pods(namespace).List(labels.Everything())
}

// AFTER:
func (r *InformerRepository) GetPods(namespace string)
    ([]*v1.Pod, error) {
    // Use GetResources with ResourceTypePod
    items, err := r.GetResources(ResourceTypePod)
    if err != nil {
        return nil, err
    }
    // Convert to typed (if needed by callers)
    // Or refactor callers to use []any
}
```

**Note**: May need to update method signatures to return `[]any`
instead of typed slices. Check callers to determine impact.

### Success Criteria

#### Automated Verification:
- [ ] Code compiles: `go build ./...`
- [ ] All tests pass: `make test`
- [ ] No references to `factory.Core()` or typed informers:
      `grep -r "factory\\.Core\\|factory\\.Apps" internal/k8s/`
- [ ] Startup no longer blocks on typed sync (verify in logs with
      `-log-file`)

#### Manual Verification:
- [ ] App starts without errors
- [ ] Can switch to all resource screens successfully
- [ ] No data loss - all screens show correct data after loading

**Implementation Note**: After Phase 1 completes and all automated
checks pass, pause for manual testing confirmation before proceeding
to Phase 2.

---

## Phase 2: Remove Blocking Waits (Enable Background Loading)

### Overview
Remove all `WaitForCacheSync()` blocking calls from startup. Let
informers sync in background while UI shows immediately. Return from
constructor without waiting for sync to complete.

### Changes Required

#### 1. Remove Typed Informer Sync Blocking
**File**: `internal/k8s/informer_repository.go`

**Lines 231-307**: **DELETE ENTIRE TYPED SYNC BLOCK**:
```go
// DELETE this entire section (the 20-25s bottleneck):
//
// syncCtx, cancel := context.WithTimeout(ctx, InformerSyncTimeout)
// defer cancel()
//
// typedSynced := cache.WaitForCacheSync(syncCtx.Done(),
//     podInformer.HasSynced,
//     deploymentInformer.HasSynced,
//     serviceInformer.HasSynced,
//     statefulSetInformer.HasSynced,
//     daemonSetInformer.HasSynced,
// )
//
// if !typedSynced {
//     return nil, fmt.Errorf("failed to sync critical resources")
// }
```

**Replace with background sync goroutine**:
```go
// Start background sync monitoring (non-blocking)
go func() {
    // Wait for typed informers to sync (in background)
    cache.WaitForCacheSync(ctx.Done(),
        podInformer.HasSynced,
        deploymentInformer.HasSynced,
        serviceInformer.HasSynced,
        statefulSetInformer.HasSynced,
        daemonSetInformer.HasSynced,
    )

    // Log completion (optional)
    logging.Info("Core informers synced",
        "pods", len(podInformer.Lister().List()),
        "deployments", len(deploymentInformer.Lister().List()))
}()
```

#### 2. Remove Dynamic Informer Sync Blocking
**File**: `internal/k8s/informer_repository.go`

**Lines 318-373**: **DELETE TIER 1 BLOCKING LOGIC**:
```go
// DELETE this entire tier-based wait logic:
//
// var wg sync.WaitGroup
// for gvr, informer := range dynamicInformers {
//     if resCfg.Tier == 1 {
//         wg.Add(1)
//         // Block until synced
//     }
// }
// wg.Wait()  // Wait for Tier 1 only
```

**Replace with background sync (already exists!)**:
```go
// Launch sync goroutines for all dynamic informers (NON-BLOCKING)
// These already run in background from lines 318-350
// Just remove the wg.Wait() that blocks on Tier 1

// Keep the goroutines that track sync status
for gvr, informer := range dynamicInformers {
    go func(gvr schema.GroupVersionResource,
            informer cache.SharedIndexInformer, tier int) {
        if cache.WaitForCacheSync(ctx.Done(), informer.HasSynced) {
            r.mu.Lock()
            r.dynamicListers[gvr] =
                r.dynamicFactory.ForResource(gvr).Lister()
            r.mu.Unlock()
            logging.Debug("Resource synced",
                "resource", gvr.Resource, "tier", tier)
        } else {
            // Failed to sync (RBAC error, etc.)
            logging.Warn("Resource sync failed",
                "resource", gvr.Resource)
        }
    }(gvr, informer, resCfg.Tier)
}

// DO NOT wait - return immediately, let them sync in background
```

#### 3. Return Immediately from Constructor
**File**: `internal/k8s/informer_repository.go`

**Lines 374-387**: Update return to not wait for completion:
```go
// Send initial progress (informers starting)
select {
case progressChan <- ContextLoadProgress{
    Context: contextName,
    Message: "Starting informers (background)",
    Phase:   PhaseComplete,  // Mark as complete immediately
}:
default:
}

// Return immediately - informers sync in background
return &InformerRepository{
    clientset:      clientset,
    dynamicClient:  dynamicClient,
    dynamicFactory: dynamicFactory,
    factory:        factory,
    resources:      resourceRegistry,
    dynamicListers: dynamicListers,  // Will populate as they sync
    podLister:      podLister,
    // ... other fields
    ctx:            ctx,
}, nil
```

#### 4. Add Screen Loading State Checks
**File**: `internal/screens/config.go`

**Lines 151-176**: Update `Init()` to check if informer synced:
```go
func (s *ConfigScreen) Init() tea.Cmd {
    // Always refresh, but Refresh() will handle loading state
    return s.Refresh()
}
```

**Lines 435-468**: Update `Refresh()` to check sync status:
```go
func (s *ConfigScreen) Refresh() tea.Cmd {
    return func() tea.Msg {
        start := time.Now()

        // Check if informer is synced
        config, exists := k8s.GetResourceConfig(s.config.ResourceType)
        if !exists {
            return types.ErrorStatusMsg("Unknown resource type")
        }

        gvr, ok := k8s.GetGVRForResourceType(s.config.ResourceType)
        if !ok {
            return types.ErrorStatusMsg("No GVR for resource")
        }

        // If informer not synced yet, show loading message and retry
        if !s.repo.IsInformerSynced(gvr) {
            // Send loading message
            go func() {
                time.Sleep(50 * time.Millisecond)
                // Retry refresh after short delay
            }()
            return types.InfoMsg(fmt.Sprintf(
                "Loading %s...", s.config.Title))
        }

        // Informer is synced, fetch data
        items, err := s.repo.GetResources(s.config.ResourceType)
        if err != nil {
            return types.ErrorStatusMsg(fmt.Sprintf(
                "Failed to fetch %s: %v", s.config.Title, err))
        }

        s.items = items
        s.applyFilter()

        return types.RefreshCompleteMsg{Duration: time.Since(start)}
    }
}
```

### Success Criteria

#### Automated Verification:
- [ ] Code compiles: `go build ./...`
- [ ] All tests pass: `make test`
- [ ] No blocking WaitForCacheSync in main thread:
      `grep -A5 "WaitForCacheSync" internal/k8s/informer_repository.go`
      (should only appear in goroutines)
- [ ] Startup time < 3 seconds:
      `time go run cmd/k1/main.go -log-file k1.log`
- [ ] Log shows informers starting but not blocking:
      `grep "Starting informers" k1.log` (should appear immediately)

#### Manual Verification:
- [ ] UI appears in 1-2 seconds (doesn't wait for informers)
- [ ] Background informer sync visible in logs
- [ ] Navigation to :pods at 2s shows "Loading Pods..." message
- [ ] Pods screen waits, then ALL pods appear at once (5-10s total)
- [ ] Navigation to :pods at 15s shows data immediately (no loading)
- [ ] All screens work correctly (loading message → all-or-nothing
      data population)
- [ ] No errors or panics during navigation

**Implementation Note**: After Phase 2 completes and all automated
checks pass, pause for manual testing confirmation before proceeding
to Phase 3.

---

## Phase 3: Enhance Loading State UI (Optional Polish)

### Overview
Improve user experience during on-demand loading with better visual
feedback. This phase is optional and can be done incrementally.

### Changes Required

#### 1. Add Resource Count to Loading Messages
**File**: `internal/screens/config.go`

**Lines 435-468**: Update `Refresh()` to show progress:
```go
func (s *ConfigScreen) Refresh() tea.Cmd {
    return func() tea.Msg {
        start := time.Now()

        // Show loading message BEFORE blocking
        // (This message is sent asynchronously, doesn't block)

        // Check if informer needs loading
        config, _ := k8s.GetResourceConfig(s.config.ResourceType)
        gvr, _ := k8s.GetGVRForResourceType(s.config.ResourceType)
        needsSync := !s.repo.IsInformerSynced(gvr)

        if needsSync {
            // Send loading message immediately
            // User sees this while sync happens
            go func() {
                // Non-blocking message send
            }()
        }

        // Ensure informer loaded (may block 5-10s first time)
        if err := s.repo.EnsureResourceTypeInformer(
            s.config.ResourceType); err != nil {
            return types.ErrorStatusMsg(fmt.Sprintf(
                "Failed to load %s: %v", s.config.Title, err))
        }

        // Fetch data
        items, err := s.repo.GetResources(s.config.ResourceType)
        if err != nil {
            return types.ErrorStatusMsg(fmt.Sprintf(
                "Failed to fetch %s: %v", s.config.Title, err))
        }

        s.items = items
        s.applyFilter()

        // Show success message with count and duration
        return types.InfoMsg(fmt.Sprintf("Loaded %d %s in %s",
            len(items), s.config.Title,
            time.Since(start).Round(time.Millisecond)))
    }
}
```

#### 2. Add Spinner to Loading Message (Advanced)
**File**: `internal/components/statusbar.go` (if exists) or
`internal/screens/config.go`

**Optional enhancement**: Add a spinner character to loading messages:
```go
type loadingSpinner struct {
    frames []string
    index  int
}

var spinner = loadingSpinner{
    frames: []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
}

func (s *ConfigScreen) View() string {
    if s.isLoading {
        return fmt.Sprintf("%s Loading %s...",
            spinner.frames[spinner.index], s.config.Title)
    }
    return s.table.View()
}
```

**Note**: This requires tick-based updates, which may be complex.
Consider this a future enhancement.

### Success Criteria

#### Automated Verification:
- [ ] Code compiles: `go build ./...`
- [ ] All tests pass: `make test`
- [ ] Loading messages include resource counts:
      `grep "Loaded.*in.*ms" k1.log`

#### Manual Verification:
- [ ] Loading messages appear immediately when switching screens
- [ ] Loading messages disappear after data loads
- [ ] Resource counts are accurate
- [ ] Load duration is displayed
- [ ] UI remains responsive during loading (can still navigate away)

**Implementation Note**: This phase is optional polish. If
time-constrained, can be deferred to a future iteration.

---

## Testing Strategy

### Unit Tests

**File**: `internal/k8s/informer_repository_test.go`

Update existing tests to reflect new behavior:

1. **Test lazy loading**:
```go
func TestInformerRepository_LazyLoading(t *testing.T) {
    repo := createTestRepository(t)

    // Verify no informers loaded at startup
    assert.False(t, repo.IsInformerSynced(podGVR))

    // Ensure informer on-demand
    err := repo.EnsureCRInformer(podGVR)
    assert.NoError(t, err)

    // Verify now synced
    assert.True(t, repo.IsInformerSynced(podGVR))

    // Second call should be instant (cached)
    start := time.Now()
    err = repo.EnsureCRInformer(podGVR)
    assert.NoError(t, err)
    assert.Less(t, time.Since(start), 100*time.Millisecond)
}
```

2. **Test concurrent access**:
```go
func TestInformerRepository_ConcurrentLazyLoad(t *testing.T) {
    repo := createTestRepository(t)

    // Multiple goroutines request same informer
    var wg sync.WaitGroup
    for i := 0; i < 10; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            err := repo.EnsureCRInformer(podGVR)
            assert.NoError(t, err)
        }()
    }
    wg.Wait()

    // Should only create once
    assert.True(t, repo.IsInformerSynced(podGVR))
}
```

### Integration Tests

**Manual testing steps**:

1. **Fast startup test**:
```bash
time go run cmd/k1/main.go -log-file startup.log -log-level debug
# Expected: < 3 seconds
```

2. **Lazy loading test**:
```bash
go run cmd/k1/main.go -log-file lazy.log -log-level debug
# In TUI:
# 1. Note startup time (should be ~1-2s)
# 2. Press ':pods' to navigate
# 3. Observe "Loading Pods..." message
# 4. Wait for pods to load (~5-10s first time)
# 5. Press ':deployments'
# 6. Observe loading message again
# 7. Navigate back to :pods
# 8. Should be instant (cached)
```

3. **Large cluster test**:
```bash
# Switch to production cluster with 1000+ pods
kubectl config use-context large-cluster
go run cmd/k1/main.go -log-file large.log
# Verify:
# - Startup still fast (~1-2s)
# - Pods load in reasonable time (~10-15s)
# - No timeouts or errors
```

### Performance Validation

**Before/after comparison**:

```bash
# Baseline (current main branch)
git checkout main
time go run cmd/k1/main.go  # Record time: ~29.5s

# After implementation
git checkout feat/startup-performance
time go run cmd/k1/main.go  # Expected: ~1-2s

# Verify improvement: ~27s saved (91% reduction)
```

**Log analysis**:
```bash
# Check that no sync happens at startup
grep "Syncing" startup.log  # Should be empty

# Check on-demand loading
grep "Loading" lazy.log  # Should show per-screen loading

# Verify timeouts not hit
grep "timeout" lazy.log  # Should be empty
```

## Performance Considerations

### Expected Improvements

**Startup performance**:
- Current: 29.5s (measured)
- Target: 1-2s (config + client init only)
- Improvement: **93-96% reduction** in startup time

**Time to first data**:
- Current: 29.5s (Pods pre-loaded)
- New: 6-10s (1-2s startup + 5-8s Pods load)
- Improvement: **67-79% reduction** in time to usable state

**Memory footprint**:
- Reduced: No unused informers loaded
- Dynamic: Only loads what user accesses
- Scalable: Supports 100+ resource types without startup penalty

### Trade-offs

**Pros**:
- ✅ Near-instant UI feedback (1-2s)
- ✅ No wasted loading for unused screens
- ✅ Scales to unlimited resource types
- ✅ Matches k9s user experience

**Cons**:
- ⚠️ First screen access has 5-10s delay (loading message visible)
- ⚠️ Screen switches may feel slower initially (until cached)
- ⚠️ Requires robust error handling for on-demand sync failures

**Mitigation strategies**:
- Clear loading indicators (already implemented)
- Background preloading for common screens (future enhancement)
- Aggressive caching (already works)

## Migration Notes

### Backwards Compatibility

**No breaking changes**:
- Repository interface unchanged
- Screen interface unchanged
- CLI flags unchanged
- Configuration format unchanged

**Behavioral changes**:
- Startup no longer blocks on resource loading
- Screens may show "Loading..." on first access
- Progress messages no longer shown during startup

### Rollback Plan

If issues arise, can rollback by:

1. **Revert tier changes**: Change select resources back to Tier 1/2
```go
ResourceTypePod: {
    Tier: 1,  // Revert to blocking startup
}
```

2. **Revert startup loading**: Re-enable dynamic informer creation in
   constructor
```go
for _, resCfg := range resourceRegistry {
    if resCfg.Tier > 0 {
        informer := dynamicFactory.ForResource(resCfg.GVR).Informer()
        dynamicListers[resCfg.GVR] =
            dynamicFactory.ForResource(resCfg.GVR).Lister()
    }
}
```

3. **Git revert**: Full revert to previous version
```bash
git revert <commit-hash>
```

## References

- Original research:
  `thoughts/shared/research/2025-10-31-startup-performance-vs-k9s.md`
- k9s source analysis: Research document lines 463-680
- Existing background sync goroutines:
  `internal/k8s/informer_repository.go:318-350`
- IsInformerSynced() method:
  `internal/k8s/informer_repository.go:733-745`
- Screen loading pattern: `internal/screens/config.go:151-220`

## TODO

### Phase 1: Remove Typed Informers
- [ ] Remove typed informer fields from InformerRepository struct
- [ ] Remove typed informer initialization code
- [ ] Remove typed informer sync blocking code (lines 231-307)
- [ ] Update methods using typed listers to use dynamic listers
- [ ] Run automated verification: `go build ./... && make test`
- [ ] Pause for manual testing confirmation

### Phase 2: Remove Blocking Waits (Enable Background Loading)
- [x] Remove typed informer WaitForCacheSync blocking (lines 231-307)
- [x] Replace with background sync goroutine (non-blocking)
- [x] Remove dynamic informer Tier 1 blocking wait (lines 318-373)
- [x] Keep background sync goroutines, just remove wg.Wait()
- [x] Update constructor to return immediately (lines 374-387)
- [x] Fix lister registration timing (add listers only after sync completes)
- [x] Implement non-blocking screen refresh (shows loading message, periodic refresh retries)
- [x] Simplify Init() to just call Refresh() directly
- [x] Update isInformerReady() to check AreTypedInformersReady() for typed informers
- [x] Add sync error tracking and surfacing (auth/connection errors shown to user)
- [x] Fix StatusMsg forwarding to screen (app.go) - periodic refresh now works!
- [x] Add MessageTypeLoading with animated spinner for loading states
- [x] Update status bar to display spinner during loading
- [x] Loading messages persist until data loads (not auto-cleared)
- [x] Run automated verification: `go build ./... && make test`
- [x] Verify startup time: **142ms (99.5% reduction from 29.5s!)**
- [x] Manual testing confirmed - all Phase 2 features working correctly
- [x] **Phase 2 COMPLETE** - commit cae89ce

### Phase 3: Enhance Loading State UI (Optional)
- [ ] Add resource count to loading messages
- [ ] Add load duration to success messages
- [ ] Consider adding spinner (advanced, optional)
- [ ] Run automated verification
- [ ] Manual testing

### Final Validation
- [ ] Performance comparison (before/after startup time)
- [ ] Large cluster testing (1000+ pods)
- [ ] Log analysis validation
- [ ] Verify all screens show loading messages and all-or-nothing data
      population correctly
- [ ] Confirm no race conditions with concurrent screen navigation
- [ ] Update documentation if needed
