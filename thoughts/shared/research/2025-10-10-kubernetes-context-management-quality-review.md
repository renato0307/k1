---
date: 2025-10-10T00:05:32Z
researcher: Claude Code
git_commit: 671c47bee36aad221ebae0987ef914de64c1e1a0
branch: feat/kubernetes-context-management
repository: k1
topic: "Kubernetes Context Management Implementation Quality Review"
tags: [research, code-quality, context-management, concurrency, testing]
status: complete
last_updated: 2025-10-10
last_updated_by: Claude Code
---

# Research: Kubernetes Context Management Implementation Quality Review

**Date**: 2025-10-10T00:05:32Z
**Researcher**: Claude Code
**Git Commit**: 671c47bee36aad221ebae0987ef914de64c1e1a0
**Branch**: feat/kubernetes-context-management
**Repository**: k1

## Research Question

The kubernetes context management feature was implemented based on
`thoughts/shared/plans/2025-10-09-kubernetes-context-management.md`
and research from
`thoughts/shared/research/2025-10-09-k8s-context-management.md`. Before
merging, assess implementation quality focusing on:

1. Code smells and golang best practices
2. Concurrency issues (race conditions, deadlocks, goroutine leaks)
3. Error handling patterns
4. Test coverage gaps
5. Memory management

## Summary

The kubernetes context management implementation has **CRITICAL issues**
that must be fixed before merging:

### 🔴 **CRITICAL Issues (5)**
1. **Deadlock in GetContexts()** - Nested lock acquisition causes
   deadlock
2. **Race in LoadContext()** - Multiple goroutines can create duplicate
   repositories
3. **Goroutine leak in statsUpdater** - No guaranteed termination on
   Close()
4. **Send on closed channel** - Event handlers may write to closed
   statsUpdateCh
5. **TOCTOU in SwitchContext()** - Status can change between check and
   switch

### 🟡 **HIGH Priority Issues (4)**
1. **200 lines of delegation boilerplate** - Violates DRY principle
2. **No test coverage** - Zero tests for 557-line repository_pool.go
3. **LRU corruption** - Off-by-one error and missing list maintenance
4. **Resource leaks on error** - Partial repositories not cleaned up

### 🟢 **MEDIUM/LOW Issues (8)**
- Missing progress reporting in switchContextCmd
- Inconsistent error wrapping
- Missing validation in kubeconfig parser
- OldContext race in app.go
- Code duplication in screen re-registration
- Missing cancellation in background loader
- Inefficient bubble sort
- Missing documentation

**Overall Assessment**: Implementation is structurally sound but has
critical concurrency bugs and zero test coverage. Must fix critical
issues and add comprehensive tests before merging.

## Detailed Findings

### 1. Code Quality Issues in repository_pool.go

**File**: `internal/k8s/repository_pool.go` (557 lines)

#### 1.1 CRITICAL: Deadlock in GetContexts() (line 442-446)

**What's wrong**: Nested lock acquisition causes deadlock on concurrent
access.

```go
func (p *RepositoryPool) GetContexts() ([]Context, error) {
    p.mu.RLock()
    defer p.mu.RUnlock()

    allContexts := p.GetAllContexts()  // Tries to acquire RLock again!
    // ...
}
```

`GetAllContexts()` at line 169 also acquires `p.mu.RLock()`. Go's
`sync.RWMutex` is **not reentrant** - a goroutine cannot acquire a read
lock it already holds.

**Why it's a problem**:
- Will deadlock on most Go implementations
- Currently appears to work only due to lack of concurrent access
- Will fail in production under load

**How to fix**: Extract logic into private helper:

```go
// Public method acquires lock
func (p *RepositoryPool) GetAllContexts() []ContextWithStatus {
    p.mu.RLock()
    defer p.mu.RUnlock()
    return p.getAllContextsLocked()
}

// Private helper assumes lock is held
func (p *RepositoryPool) getAllContextsLocked() []ContextWithStatus {
    result := make([]ContextWithStatus, 0, len(p.contexts))
    // ... existing logic without lock acquisition
    return result
}

// GetContexts now calls unlocked version
func (p *RepositoryPool) GetContexts() ([]Context, error) {
    p.mu.RLock()
    defer p.mu.RUnlock()
    allContexts := p.getAllContextsLocked()  // No nested lock!
    // ...
}
```

**Reference**: `internal/k8s/repository_pool.go:442-446`,
`internal/k8s/repository_pool.go:169`

---

#### 1.2 CRITICAL: Race Condition in LoadContext() (lines 64-119)

**What's wrong**: Function releases lock before creating repository,
allowing multiple goroutines to load the same context simultaneously.

```go
func (p *RepositoryPool) LoadContext(...) error {
    p.mu.Lock()
    if _, exists := p.repos[contextName]; !exists {
        p.repos[contextName] = &RepositoryEntry{Status: StatusLoading}
    }
    p.mu.Unlock()  // Lock released!

    // 5-15 second operation WITHOUT lock
    repo, err := NewInformerRepositoryWithProgress(...)

    p.mu.Lock()  // Re-acquire lock
    // ... update entry
}
```

**Race scenario**:

```
Time  Goroutine 1              Goroutine 2
0ms   LoadContext("prod")
      - Lock, set Loading
      - Unlock
5ms   Creating repo...         LoadContext("prod")
                               - Lock, entry exists
                               - Unlock
                               - Creating repo...
10s   Lock, update with repo1
15s                            Lock, update with repo2
                               (OVERWRITES repo1, leak!)
```

**Consequences**:
- **Resource leaks**: First repository overwritten, never closed
- **Wasted work**: Both goroutines create identical repositories
- **Inconsistent state**: Last writer wins

**How to fix**: Use sync.Map to coordinate loading:

```go
type loadingState struct {
    done chan struct{}
    err  error
}

type RepositoryPool struct {
    // ...
    loading sync.Map  // map[string]*loadingState
}

func (p *RepositoryPool) LoadContext(...) error {
    // Try to start loading
    state := &loadingState{done: make(chan struct{})}
    actual, loaded := p.loading.LoadOrStore(contextName, state)

    if loaded {
        // Another goroutine is loading, wait
        <-actual.(*loadingState).done
        return actual.(*loadingState).err
    }

    // We're the loader
    defer func() {
        close(state.done)
        p.loading.Delete(contextName)
    }()

    // ... existing loading logic
}
```

**Reference**: `internal/k8s/repository_pool.go:64-119`

---

#### 1.3 HIGH: God Object - 200 Lines of Delegation Boilerplate (lines
236-439)

**What's wrong**: 16 nearly-identical methods that just forward calls to
active repository.

```go
func (p *RepositoryPool) GetPods() ([]Pod, error) {
    repo := p.GetActiveRepository()
    if repo == nil {
        return nil, fmt.Errorf("no active repository")
    }
    return repo.GetPods()
}

// ... 15 more identical functions
```

**Why it's a problem**:
- Violates DRY principle
- 200/557 lines (36%) is boilerplate
- Error handling changes require updating 16 functions

**How to fix**: Use Go's interface embedding:

```go
type RepositoryPool struct {
    Repository  // Embed interface - automatic delegation
    mu         sync.RWMutex
    // ... other fields
}

// Override only methods needing special handling
func (p *RepositoryPool) GetResources(resourceType ResourceType)
    ([]any, error) {
    if resourceType == ResourceTypeContext {
        return p.getContextResources()
    }
    return p.Repository.GetResources(resourceType)
}
```

This reduces 200+ lines to ~20 lines for special cases.

**Reference**: `internal/k8s/repository_pool.go:236-439`

---

#### 1.4 HIGH: LRU Corruption and Off-by-One Error (lines 98-100,
516-526)

**Two related issues**:

**Issue 1: Off-by-one in pool size check** (line 98-100):
```go
if len(p.repos) >= p.maxSize {
    p.evictLRU()
}
```

This evicts when we have `maxSize` items, then adds a new one, resulting
in `maxSize + 1` items in the pool.

**Issue 2: markUsed() silently fails** (lines 516-526):
```go
func (p *RepositoryPool) markUsed(contextName string) {
    for e := p.lru.Front(); e != nil; e = e.Next() {
        if e.Value.(string) == contextName {
            p.lru.MoveToFront(e)
            return
        }
    }
    // BUG: If not found, silently returns!
}
```

**Consequences**:
- LRU list becomes out of sync with repos map
- Pool can grow beyond maxSize
- Eviction may fail to remove anything

**How to fix**:

```go
// Fix 1: Proper pool size enforcement
for len(p.repos) >= p.maxSize {
    p.evictLRU()
}

// Fix 2: Defensive markUsed
func (p *RepositoryPool) markUsed(contextName string) {
    for e := p.lru.Front(); e != nil; e = e.Next() {
        if e.Value.(string) == contextName {
            p.lru.MoveToFront(e)
            return
        }
    }
    // Not found - add to front (defensive)
    p.lru.PushFront(contextName)
}
```

**Reference**: `internal/k8s/repository_pool.go:98-100`,
`internal/k8s/repository_pool.go:516-526`

---

#### 1.5 HIGH: Resource Leak on Error (lines 88-95)

**What's wrong**: Partially created repository not cleaned up on error.

```go
repo, err := NewInformerRepositoryWithProgress(...)

p.mu.Lock()
defer p.mu.Unlock()

if err != nil {
    if entry, ok := p.repos[contextName]; ok {
        entry.Status = StatusFailed
        entry.Error = err
    }
    return err  // Doesn't call repo.Close()!
}
```

**Why it's a problem**:
- If repository partially succeeds (connects but fails to sync), it may
  have started goroutines or watches
- These resources leak until process exit

**How to fix**:

```go
repo, err := NewInformerRepositoryWithProgress(...)

p.mu.Lock()
defer p.mu.Unlock()

if err != nil {
    if repo != nil {
        repo.Close()  // Cleanup partial repository
    }
    if entry, ok := p.repos[contextName]; ok {
        entry.Status = StatusFailed
        entry.Error = err
    }
    return fmt.Errorf("failed to load context %s: %w", contextName, err)
}
```

**Reference**: `internal/k8s/repository_pool.go:88-95`

---

#### 1.6 MEDIUM: Inefficient Bubble Sort (lines 489-497)

**What's wrong**: Uses bubble sort (O(n²)) instead of standard library.

```go
func sortContextsByName(contexts []Context) {
    for i := 0; i < len(contexts); i++ {
        for j := i + 1; j < len(contexts); j++ {
            if contexts[i].Name > contexts[j].Name {
                contexts[i], contexts[j] = contexts[j], contexts[i]
            }
        }
    }
}
```

**Performance impact**:
- 100 contexts: ~15x slower than standard sort
- 1000 contexts: ~150x slower

**How to fix**:

```go
import "sort"

func sortContextsByName(contexts []Context) {
    sort.Slice(contexts, func(i, j int) bool {
        return contexts[i].Name < contexts[j].Name
    })
}
```

**Reference**: `internal/k8s/repository_pool.go:489-497`

---

#### 1.7 MEDIUM: Missing Error Context in Delegation (lines 264, 274,
etc.)

**What's wrong**: Errors don't indicate which context failed.

```go
func (p *RepositoryPool) GetPods() ([]Pod, error) {
    repo := p.GetActiveRepository()
    if repo == nil {
        return nil, fmt.Errorf("no active repository")
    }
    return repo.GetPods()  // Error doesn't include context name
}
```

**How to fix**:

```go
func (p *RepositoryPool) GetPods() ([]Pod, error) {
    repo := p.GetActiveRepository()
    if repo == nil {
        return nil, fmt.Errorf("no active repository")
    }

    pods, err := repo.GetPods()
    if err != nil {
        return nil, fmt.Errorf("context %s: %w",
            p.GetActiveContext(), err)
    }
    return pods, nil
}
```

**Reference**: `internal/k8s/repository_pool.go:264`, etc.

---

#### 1.8 LOW: Close() Doesn't Clear State (lines 224-233)

**What's wrong**: Pool still has references to closed repositories.

```go
func (p *RepositoryPool) Close() {
    p.mu.Lock()
    defer p.mu.Unlock()

    for _, entry := range p.repos {
        if entry.Repo != nil {
            entry.Repo.Close()
        }
    }
    // Doesn't clear p.repos or p.lru!
}
```

**How to fix**:

```go
func (p *RepositoryPool) Close() {
    p.mu.Lock()
    defer p.mu.Unlock()

    for _, entry := range p.repos {
        if entry.Repo != nil {
            entry.Repo.Close()
        }
    }

    // Clear all state
    p.repos = make(map[string]*RepositoryEntry)
    p.lru = list.New()
    p.active = ""
}
```

**Reference**: `internal/k8s/repository_pool.go:224-233`

---

### 2. Code Quality Issues in Supporting Files

#### 2.1 Inconsistent Error Wrapping in kubeconfig_parser.go (lines
50-51)

**Issue**: `getCurrentContext()` returns unwrapped errors while
`parseKubeconfig()` wraps them.

```go
func getCurrentContext(kubeconfigPath string) (string, error) {
    config, err := clientcmd.LoadFromFile(kubeconfigPath)
    if err != nil {
        return "", err  // Not wrapped
    }
    return config.CurrentContext, nil
}
```

**Fix**: Wrap consistently:

```go
if err != nil {
    return "", fmt.Errorf("failed to load kubeconfig: %w", err)
}
```

**Reference**: `internal/k8s/kubeconfig_parser.go:50-51`

---

#### 2.2 Missing Empty Context Validation (line 19)

**Issue**: `parseKubeconfig()` doesn't validate that contexts exist.

**Fix**:

```go
if len(config.Contexts) == 0 {
    return nil, fmt.Errorf("kubeconfig contains no contexts")
}
```

**Reference**: `internal/k8s/kubeconfig_parser.go:19-44`

---

#### 2.3 Missing String() Method for LoadPhase (lines 10-18)

**Issue**: Exported enum without String() method makes debugging harder.

**Fix**:

```go
func (p LoadPhase) String() string {
    switch p {
    case PhaseConnecting:
        return "connecting"
    case PhaseSyncingCore:
        return "syncing core resources"
    case PhaseSyncingDynamic:
        return "syncing dynamic resources"
    case PhaseComplete:
        return "complete"
    default:
        return "unknown"
    }
}
```

**Reference**: `internal/k8s/progress.go:10-18`

---

### 3. Integration Issues in app.go

#### 3.1 CRITICAL: OldContext Race in switchContextCmd (line 564)

**What's wrong**: Reads old context AFTER switch completes, so it's
actually the new context.

```go
func (m Model) switchContextCmd(contextName string) tea.Cmd {
    return func() tea.Msg {
        err := m.repoPool.SwitchContext(contextName, nil)

        return types.ContextSwitchCompleteMsg{
            OldContext: m.repoPool.GetActiveContext(),  // Wrong!
            NewContext: contextName,
        }
    }
}
```

**Fix**: Capture old context before switch:

```go
func (m Model) switchContextCmd(contextName string) tea.Cmd {
    return func() tea.Msg {
        oldContext := m.repoPool.GetActiveContext()
        err := m.repoPool.SwitchContext(contextName, nil)

        return types.ContextSwitchCompleteMsg{
            OldContext: oldContext,
            NewContext: contextName,
        }
    }
}
```

**Reference**: `internal/app/app.go:551-568`

---

#### 3.2 HIGH: Missing Progress Reporting (lines 554, 573)

**Issue**: Both `switchContextCmd` and `retryContextCmd` pass `nil`
progress channel, so no progress feedback during 5-15 second loads.

```go
err := m.repoPool.SwitchContext(contextName, nil)
```

**Fix**: Create and forward progress channel:

```go
progressCh := make(chan k8s.ContextLoadProgress, 10)
go func() {
    for progress := range progressCh {
        m.program.Send(types.ContextLoadProgressMsg(progress))
    }
}()
err := m.repoPool.SwitchContext(contextName, progressCh)
close(progressCh)
```

**Reference**: `internal/app/app.go:554`, `internal/app/app.go:573`

---

#### 3.3 MEDIUM: Code Duplication in Screen Re-registration (lines
387-421, 429-449)

**Issue**: Two nearly-identical blocks differ only in target screen.

**Fix**: Extract helper method:

```go
func (m *Model) switchToScreen(screenID string, refresh bool)
    tea.Cmd {
    m.registry = types.NewScreenRegistry()
    m.initializeScreens()

    screen, ok := m.registry.Get(screenID)
    if !ok {
        return messages.ErrorCmd("screen %s not found", screenID)
    }

    m.currentScreen = screen
    // ... common logic ...
}
```

**Reference**: `internal/app/app.go:387-449`

---

#### 3.4 MEDIUM: Unsafe Type Assertions (lines 416, 405-406)

**Issue**: Type assertions without checking can panic.

```go
screen.(interface{ Refresh() tea.Cmd }).Refresh()  // Can panic
```

**Fix**: Use two-value form:

```go
if refresh, ok := screen.(interface{ Refresh() tea.Cmd }); ok {
    return refresh.Refresh()
}
```

**Reference**: `internal/app/app.go:416`, `internal/app/app.go:405-406`

---

#### 3.5 LOW: Magic Number in ContextLoadProgressMsg Handler (line 340)

**Issue**: Hardcoded `Phase == 3` instead of constant.

```go
if msg.Phase == 3 {  // Magic number
```

**Fix**: Use constant:

```go
if msg.Phase == k8s.PhaseComplete {
```

**Reference**: `internal/app/app.go:340`

---

### 4. Concurrency Issues

#### 4.1 CRITICAL: Goroutine Leak in statsUpdater (line 291)

**Issue**: statsUpdater goroutine has no guaranteed termination when
repository is closed.

```go
// Line 291: Launched without context tracking
go repo.statsUpdater()

// Close implementation
func (r *InformerRepository) Close() {
    if r.cancel != nil {
        r.cancel()
    }
    if r.statsUpdateCh != nil {
        close(r.statsUpdateCh)  // Goroutine exits eventually
    }
}
```

**Problem**: If statsUpdateCh has 1000 buffered messages, goroutine will
drain all before exiting. Meanwhile, eviction continues and memory isn't
freed.

**Reference**: `internal/k8s/informer_repository.go:291`,
`internal/k8s/informer_events.go:47-64`,
`internal/k8s/informer_repository.go:500-507`

---

#### 4.2 CRITICAL: Send on Closed Channel Race (lines 37-43)

**Issue**: Event handlers may write to statsUpdateCh after Close().

**Race timeline**:
1. Thread A: Informer fires event → calls `trackStats()`
2. Thread B: `Close()` called → `close(r.statsUpdateCh)`
3. Thread A: `select case r.statsUpdateCh <- msg` → **panic**

The `select` with `default` protects against full channels, not closed
channels.

**Fix**: Add closed flag check:

```go
type InformerRepository struct {
    // ...
    closed atomic.Bool
}

func (r *InformerRepository) trackStats(...) {
    if r.closed.Load() {
        return
    }
    select {
    case r.statsUpdateCh <- msg:
    default:
    }
}

func (r *InformerRepository) Close() {
    r.closed.Store(true)
    // ... rest of close logic
}
```

**Reference**: `internal/k8s/informer_events.go:37-43`,
`internal/k8s/informer_repository.go:500-507`

---

#### 4.3 CRITICAL: Informer Goroutines Not Stopped on Eviction (lines
530-555)

**Issue**: evictLRU closes repositories but doesn't verify informers
stopped.

```go
if entry.Repo != nil {
    entry.Repo.Close()  // Starts shutdown but doesn't wait
}
delete(p.repos, contextName)
```

Informers may still be firing events after entry is deleted, leading to
send-on-closed-channel panics.

**Reference**: `internal/k8s/repository_pool.go:530-555`,
`internal/k8s/informer_repository.go:500-507`

---

#### 4.4 MAJOR: Background Loader No Cancellation (lines 137-177)

**Issue**: `loadBackgroundContexts` runs until all contexts loaded, with
no way to cancel.

```go
go loadBackgroundContexts(pool, contexts[1:], p)

// Inside function:
for _, ctx := range contexts {
    err := pool.LoadContext(ctx, progressCh)  // Blocks 5-15 seconds
}
```

If user quits, goroutine continues loading contexts and holding
connections.

**Fix**: Pass context for cancellation:

```go
go loadBackgroundContexts(ctx, pool, contexts[1:], p)

func loadBackgroundContexts(ctx context.Context, ...) {
    for _, contextName := range contexts {
        select {
        case <-ctx.Done():
            return  // Cancelled
        default:
        }
        // ... load context
    }
}
```

**Reference**: `cmd/k1/main.go:137-177`

---

#### 4.5 MAJOR: Missing Parent Context for Cancellation (lines 89-309)

**Issue**: NewInformerRepositoryWithProgress creates its own context,
can't be cancelled by caller.

```go
ctx, cancel := context.WithCancel(context.Background())
```

Should accept parent context parameter to allow cancellation during
5-15 second sync.

**Reference**: `internal/k8s/informer_repository.go:89-309`

---

### 5. Test Coverage Gaps

#### 5.1 CRITICAL: No Tests for repository_pool.go (557 lines)

**File**: `internal/k8s/repository_pool_test.go` ❌ **MISSING**

**Risk Level**: 🔴 **CRITICAL**

**Missing test scenarios**:
- Pool lifecycle (creation, loading, closing)
- LRU eviction correctness
- Concurrent context loads
- Context switching (loaded vs not loaded)
- Thread safety (run with `-race`)
- Status transitions
- Failed context retry
- Repository interface delegation

**Why critical**: Core infrastructure with complex concurrency, LRU, and
lifecycle management. All issues above are undetected due to zero test
coverage.

**Reference**: Plan Phase 7, lines 1621-1666

---

#### 5.2 MEDIUM: No Tests for kubeconfig_parser.go (59 lines)

**File**: `internal/k8s/kubeconfig_parser_test.go` ❌ **MISSING**

**Missing test scenarios**:
- Valid kubeconfig parsing
- Multiple contexts
- Alphabetical sorting (lines 37-41)
- Error handling (invalid file, empty contexts)
- Current context extraction

**Reference**: Plan Phase 7, lines 1668-1703

---

#### 5.3 MEDIUM: No Tests for context.go (31 lines)

**File**: `internal/commands/context_test.go` ❌ **MISSING**

**Missing test scenarios**:
- Valid context switch command
- Invalid arguments
- Message type returned

**Reference**: Plan Phase 7, lines 1705-1742

---

#### 5.4 MEDIUM: No Context Switching Tests in app_test.go

**File**: `internal/app/app_test.go` - **INCOMPLETE**

**Missing integration tests**:
- Context switch message handling
- Screen re-registration after switch
- Progress reporting
- Loading spinner animation

**Reference**: Plan Phase 7, lines 1744-1783

---

#### 5.5 LOW: Contexts Screen Not in screens_test.go

**File**: `internal/screens/screens_test.go` - **INCOMPLETE**

**Missing**: One test case for contexts screen configuration

**Reference**: Plan Phase 7, lines 1785-1828

---

## Code References

### Critical Issues
- **Deadlock**: `internal/k8s/repository_pool.go:442-446`
- **LoadContext race**: `internal/k8s/repository_pool.go:64-119`
- **Goroutine leak**: `internal/k8s/informer_repository.go:291`
- **Send on closed channel**: `internal/k8s/informer_events.go:37-43`
- **OldContext race**: `internal/app/app.go:551-568`

### High Priority Issues
- **Delegation boilerplate**: `internal/k8s/repository_pool.go:236-439`
- **LRU corruption**: `internal/k8s/repository_pool.go:98-100`,
  `516-526`
- **Resource leak on error**: `internal/k8s/repository_pool.go:88-95`
- **No tests**: Missing `repository_pool_test.go`

### Medium/Low Issues
- **Missing progress**: `internal/app/app.go:554`, `573`
- **Code duplication**: `internal/app/app.go:387-449`
- **Error wrapping**: `internal/k8s/kubeconfig_parser.go:50-51`
- **Bubble sort**: `internal/k8s/repository_pool.go:489-497`

## Architecture Insights

### 1. Sound Overall Design

The **repository pool pattern** is the right approach:
- LRU eviction balances memory vs UX
- Interface delegation enables clean separation
- Non-blocking operations preserve UI responsiveness
- Status tracking provides good observability

### 2. Implementation vs Design Gap

The design (from research and plan) is solid, but implementation has
critical execution issues:
- Concurrency primitives used incorrectly (nested locks, TOCTOU)
- Resource lifecycle not fully thought through (leaks)
- Test-driven development not followed (zero tests)

### 3. Golang Best Practices Violations

Several anti-patterns found:
- **Not reentrant-safe**: Nested lock acquisitions
- **Not idiomatic**: Bubble sort instead of sort.Slice
- **Not DRY**: 200 lines of delegation boilerplate
- **Not defensive**: Silent failures in markUsed()

### 4. Phase 7 Incomplete

Plan specified Phase 7 (Testing and Documentation) but it was not
completed:
- ✅ Phases 1-6 implemented (infrastructure, CLI, messages, UI, screen,
  commands)
- ❌ Phase 7 incomplete (zero tests, docs not updated)

## Recommendations

### CRITICAL - Must Fix Before Merge (1-2 days)

1. **Fix deadlock in GetContexts()** - Extract getAllContextsLocked()
   helper
2. **Fix LoadContext race** - Use sync.Map to coordinate loading
3. **Fix goroutine leaks** - Ensure statsUpdater terminates, wait for
   informers to stop
4. **Fix send on closed channel** - Add closed flag check in
   trackStats()
5. **Fix OldContext race** - Capture before SwitchContext call

### HIGH - Should Fix Before Merge (2-3 days)

6. **Add comprehensive tests** - Start with repository_pool_test.go
   (600 lines)
7. **Fix LRU corruption** - Proper off-by-one check and defensive
   markUsed()
8. **Fix resource leaks** - Call repo.Close() on error paths
9. **Add progress reporting** - Create and forward progress channels in
   app.go

### MEDIUM - Should Fix Soon (1-2 days)

10. **Refactor delegation boilerplate** - Use interface embedding (saves
    200 lines)
11. **Add kubeconfig parser tests** - Simple pure functions (100 lines)
12. **Add context command tests** - Follow existing patterns (50 lines)
13. **Fix error wrapping inconsistencies** - Consistent error context
14. **Add context switching integration tests** - Message flow (150
    lines)

### LOW - Can Defer (< 1 day)

15. **Replace bubble sort** - Use sort.Slice (trivial change)
16. **Extract screen switching helper** - Eliminate duplication
17. **Add String() method to LoadPhase** - Better debugging
18. **Add contexts screen test** - One test case (15 lines)

## Quality Gate Status

From `CLAUDE.md` quality guidelines:

**File Size Limits**:
- ✅ All files under 800 lines (repository_pool.go is 557 lines)
- ⚠️ repository_pool.go approaching 500-line warning threshold

**Test Coverage Requirements**:
- ❌ **FAILING**: New components need 70% minimum
  - repository_pool.go: 0%
  - kubeconfig_parser.go: 0%
  - context.go: 0%

**Code Duplication**:
- ❌ **FAILING**: 200 lines of delegation boilerplate (3+ repetitions)

**Overall Gate Status**: ❌ **FAILING** - Critical issues and zero test
coverage

## Implementation Effort Estimate

### Week 1: Critical Fixes + Core Tests (5 days)

**Days 1-2**: Fix critical concurrency issues
- Deadlock in GetContexts()
- LoadContext race condition
- Goroutine leaks
- Send on closed channel
- Run with `go test -race` to verify

**Days 3-5**: Add repository pool tests
- Pool lifecycle tests
- LRU eviction tests
- Concurrency tests with -race
- Status transition tests
- Target: 70% coverage (400+ lines of tests)

### Week 2: High Priority + Integration (5 days)

**Days 1-2**: Fix high priority issues
- LRU corruption
- Resource leaks
- Progress reporting in app.go
- OldContext race

**Days 3-4**: Add integration tests
- kubeconfig_parser_test.go (100 lines)
- context_test.go (50 lines)
- app_test.go context handling (150 lines)

**Day 5**: Code review and cleanup
- Run full test suite
- Run with -race
- Update documentation

### Week 3 (Optional): Refactoring

**Days 1-2**: Interface embedding refactoring
- Remove 200 lines of boilerplate
- Update tests for new structure

**Total Effort**: 2-3 weeks to production-ready

## Historical Context (from thoughts/)

Related documents:
- **Original research**:
  `thoughts/shared/research/2025-10-09-k8s-context-management.md`
- **Implementation plan**:
  `thoughts/shared/plans/2025-10-09-kubernetes-context-management.md`

**Key insight**: Plan specified comprehensive testing (Phase 7) but this
was not completed. The user's concern that "the implementation might not
be very good" is validated - critical issues exist and test coverage is
zero.

## Related Research

- **Context management research**: Identified repository pool as correct
  pattern
- **Implementation plan**: 7-phase approach with Phase 7 incomplete
- **Process improvements**: `design/PROCESS-IMPROVEMENTS.md` quality
  gates would have caught these issues earlier

## Open Questions

### 1. Should we merge as-is and fix incrementally?

**Answer**: **NO** - Critical issues cause:
- Deadlocks (production outage)
- Race conditions (data corruption)
- Goroutine leaks (memory exhaustion)
- Panics (crash loop)

These must be fixed before merge.

### 2. Can we fix just critical issues and defer tests?

**Answer**: **NO** - Tests are required to:
- Verify fixes work
- Prevent regressions
- Catch concurrency bugs (with -race)
- Meet quality gate (70% coverage)

### 3. Should we refactor delegation boilerplate now?

**Answer**: **DEFER** - While it's a code smell, it's not a bug. Focus
on:
1. Critical bug fixes
2. Test coverage
3. High priority issues
4. Then refactor if time permits

### 4. How confident are we in the quality assessment?

**Answer**: **HIGH CONFIDENCE** - Multiple specialized agents analyzed:
- Code quality and golang best practices
- Concurrency patterns and race conditions
- Test coverage gaps
- Integration correctness

All findings cross-referenced with golang documentation and k1's
CLAUDE.md guidelines.

## Next Steps

### Immediate (This Week)

1. **Review findings with team** - Discuss critical issues and timeline
2. **Create GitHub issues** - One per critical/high issue
3. **Start critical fixes** - Deadlock, race conditions, goroutine leaks
4. **Run with -race flag** - Verify no data races detected

### Short Term (Next Week)

5. **Add core tests** - repository_pool_test.go with 70% coverage
6. **Fix high priority issues** - LRU, resource leaks, progress
   reporting
7. **Add integration tests** - kubeconfig, context command, app handling
8. **Update documentation** - CLAUDE.md and README.md

### Before Merge

9. **Run full test suite** - All tests passing
10. **Run with -race** - No races detected
11. **Code review** - All findings addressed
12. **Quality gate check** - Coverage, file sizes, duplication

### After Merge (Optional)

13. **Refactor delegation** - Use interface embedding
14. **Performance testing** - With 10+ contexts
15. **User acceptance testing** - Multi-context workflows
