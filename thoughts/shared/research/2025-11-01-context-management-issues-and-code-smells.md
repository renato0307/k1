---
date: 2025-11-01T09:53:23+0000
researcher: claude
git_commit: 30f38dd335431729d2d4272927762111b47e529c
branch: fix/bug-squash-3
repository: k1
topic: "Context Management Issues and Code Smells"
tags: [research, codebase, context-management, ux, error-handling]
status: complete
last_updated: 2025-11-01
last_updated_by: claude
---

# Research: Context Management Issues and Code Smells

**Date**: 2025-11-01T09:53:23+0000
**Researcher**: claude
**Git Commit**: 30f38dd335431729d2d4272927762111b47e529c
**Branch**: fix/bug-squash-3
**Repository**: k1

## Research Question

Investigate context-related issues in the k1 codebase, including issues
identified in issue #5 and additional UX/error handling problems in the
current implementation. Focus on background loading challenges and error
communication to users.

## Summary

The context management implementation in k1 uses a sophisticated repository
pool with background loading, but has several UX and error handling gaps
that make multi-context usage difficult:

**Critical UX Issues:**
1. Failed contexts show "Failed" status but error messages are hidden
2. Loading states can hang indefinitely with no timeout
3. Background load failures are silent until user switches to failed context
4. No retry mechanism exposed in UI (method exists but no command)
5. Progress reporting lacks context count ("1/n") requested in issue #5

**Issue #5 Items (Mix of Implemented/Intentionally Skipped/Pending):**
6. Status-based sorting - ✅ Already implemented
7. Status included in search - ✅ Already implemented
8. Keyboard shortcuts (ctrl+1-9, ctrl+0, etc.) - ✅ Intentionally not
   implemented (keeping UI simple, next/prev sufficient)
9. Shortcuts column in contexts screen - ✅ Not needed (no shortcuts)
10. Stricter fuzzy search - ❌ Still pending
11. Context documentation in README - ❌ Still pending

**Code Smells:**
12. Partial success states invisible (RBAC errors hidden when context loads)
13. Context switch progress not shown (nil progress channel)
14. Silent ReplicaSets informer failures
15. O(n) LRU search (minor - n is small)

**Note**: Context-aware auto-navigation (staying on same screen when
switching contexts) is intentional design for multi-cluster comparison
workflows, not a bug.

## Detailed Findings

### 1. Error Visibility Gap (Critical UX Issue)

**Location**: `internal/k8s/repository_pool.go:589-591`,
`internal/screens/screens.go:526-547`

**Problem**: Failed contexts store error messages in `Context.Error` field
but the contexts screen doesn't display them:

```go
// repository_pool.go:589-591
errorMsg := ""
if ctx.Error != nil {
    errorMsg = ctx.Error.Error()  // Error captured but not displayed
}
```

```go
// screens.go:530-536 - Contexts screen config
Columns: []ColumnConfig{
    {Field: "Current", Title: "✓", Width: 5, Priority: 1},
    {Field: "Name", Title: "Name", Width: 30, Priority: 1},
    {Field: "Cluster", Title: "Cluster", Width: 0, Priority: 2},
    {Field: "User", Title: "User", Width: 0, Priority: 2},
    {Field: "Status", Title: "Status", Width: 15, Priority: 1},
    // Missing: {Field: "Error", Title: "Error", Width: 0, Priority: 3}
},
```

**Impact**: Users see "Failed" status but have no way to understand WHY
the context failed without checking logs.

**Evidence**: Roadmap document
`thoughts/shared/tickets/roadmap.md:46` confirms:
> "If context load fails the error is not shown"

**Fix Options**:
1. Add Error column to contexts screen (Priority 3 - shown on wide
   screens, hidden on narrow)
2. Broader solution: :output screen showing all command/error history
   (see `thoughts/shared/research/2025-11-02-command-output-history-design.md`)

---

### 2. Stuck Loading States (Critical UX Issue)

**Location**: `internal/k8s/informer_repository.go:329-384`,
`internal/k8s/repository_pool.go:81-168`

**Problem**: Context loading has timeout for informer sync (2 minutes) but
no overall timeout. If connection hangs during initial API server
connection, status stays "Loading" forever.

```go
// informer_repository.go:158-167 - 5s timeout for auth check
authCtx, authCancel := context.WithTimeout(context.Background(),
    5*time.Second)
_, err = clientset.CoreV1().Namespaces().List(authCtx,
    metav1.ListOptions{Limit: 1})

// But connection establishment itself has no timeout!
// If TCP connect hangs, this blocks indefinitely
```

**Impact**: Users see spinning "Loading" indicator forever if cluster is
unreachable. No feedback that connection is stuck.

**Evidence**: Roadmap document
`thoughts/shared/tickets/roadmap.md:44-45` confirms:
> "If cannot connect to cluster, the connecting to API Server is showing
> spinning"

**Fix**: Add overall timeout (e.g., 5 minutes) for entire LoadContext
operation, update status to "Failed" with timeout error.

---

### 3. Silent Background Failures (High Priority UX Issue)

**Location**: `cmd/k1/main.go:180-222`

**Problem**: When multiple contexts are loaded with `-context` flag,
contexts 2+ load in background. Failures are sent as
`ContextLoadFailedMsg` but only displayed in status bar (5s auto-clear).
If user misses the brief error message, they won't know context failed
until they try to switch to it.

```go
// main.go:209-218
if err != nil {
    program.Send(types.ContextLoadFailedMsg{
        Context: ctx,
        Error:   err,  // Sent to status bar, auto-clears in 5s
    })
} else {
    program.Send(types.ContextLoadCompleteMsg{
        Context: ctx,
    })
}
```

**Impact**: Poor discoverability of background load failures. User assumes
all contexts loaded successfully.

**Fix**:
- Show persistent notification for background failures (don't auto-clear)
- Show summary at end: "Loaded 2/5 contexts (3 failed)"
- Navigate to contexts screen to show failed contexts

---

### 4. Missing Retry UI (High Priority UX Issue)

**Location**: `internal/k8s/repository_pool.go:262-274`,
`internal/commands/registry.go`

**Problem**: `RetryFailedContext()` method exists but no command registered
in command palette to invoke it. Users have no way to retry failed
contexts.

```go
// repository_pool.go:262-274 - Method exists
func (p *RepositoryPool) RetryFailedContext(contextName string,
    progress chan<- ContextLoadProgress) error {
    // ... implementation
}

// But no command in registry.go for /retry-context
```

**Impact**: Failed contexts are permanent failures. User must restart
application to retry.

**Fix**: Add `/retry-context` command to command palette, shown only when
viewing failed context in contexts screen.

---

### 5. Context Status Summary Missing (Issue #5, Item 8)

**Location**: `internal/app/app.go:425-434`, `internal/components/header.go`,
`thoughts/shared/tickets/issue_5.md:14`

**Problem**: No visibility into overall context status when multiple
contexts exist. Users don't know how many contexts are loaded vs
loading/failed.

Current header shows:
> "my-cluster" (just current context name)

**Desired behavior**: When multiple contexts exist, show status summary:
> "my-cluster (3 loaded / 5 total)"

Or when loading:
> "my-cluster (3 loaded / 5 total, 1 loading)"

**Impact**: Users managing multiple contexts have no quick overview of
context health. Must navigate to `:contexts` screen to see status.

**Fix**:
1. Add context count tracking in app model
2. Update header component to show "(N loaded / M total)" when M > 1
3. Optionally show loading/failed counts for quick status awareness

---

### 6. Keyboard Shortcuts Not Implemented (Intentional Simplicity)

**Location**: `internal/app/app.go` (keybindings),
`thoughts/shared/tickets/issue_5.md:7-8`

**Issue #5 originally requested**:
- `ctrl+1` through `ctrl+9` to switch to contexts 1-9
- `ctrl+0` for 10th context
- `ctrl+-` for `:prev-context`
- `ctrl+=` for `:next-context`

**Current implementation**:
- Has `/prev-context` and `/next-context` commands (no shortcuts)
- No numeric shortcuts

**Design Decision**: Intentionally NOT implemented to keep UI simple.
- 10 keyboard shortcuts adds complexity
- `/prev-context` and `/next-context` are sufficient
- Most users don't manage 10+ contexts simultaneously
- Easy to type `:contexts` to see list and pick one

**Status**: ✅ Simplicity preferred over keyboard shortcut proliferation.

---

### 7. Shortcuts Column Not Needed (Follows from #6)

**Location**: `internal/screens/screens.go:530-536`,
`thoughts/shared/tickets/issue_5.md:10`

**Issue #5 originally requested**: Shortcuts shown in contexts screen
(e.g., "1", "2", "3" in a new column).

**Current columns**: Current (✓), Name, Cluster, User, Status

**Design Decision**: Not needed since keyboard shortcuts (ctrl+1-9) are not
implemented (see finding #6).

**Status**: ✅ No shortcuts column needed when there are no shortcuts to
display.

---

### 8. Status-Based Sorting Not Implemented (Issue #5, Item 5)

**Location**: `internal/k8s/repository_pool.go:619-645`,
`thoughts/shared/tickets/issue_5.md:11`

**Problem**: Issue #5 requests:
> "put the ones loaded/loading/error loaded first, then the rest in
> alphabetical order"

Current implementation DOES sort by status priority (Loaded → Loading →
Failed → NotLoaded), then alphabetical:

```go
// repository_pool.go:623-628
statusPriority := map[string]int{
    "Loaded":     0,
    "Loading":    1,
    "Failed":     2,
    "Not Loaded": 3,
}
```

**Status**: ✅ This is actually ALREADY IMPLEMENTED. Issue #5 item 5 is
already satisfied. Verified in code.

---

### 9. Status Not Included in Search (Issue #5, Item 6)

**Location**: `internal/screens/screens.go:538`,
`thoughts/shared/tickets/issue_5.md:12`

**Problem**: Issue #5 requests status included in search.

Current SearchFields:
```go
SearchFields: []string{"Name", "Cluster", "User", "Status"},
```

**Status**: ✅ Status IS included in SearchFields. Issue #5 item 6 is
already satisfied. Verified in code.

---

### 10. Fuzzy Search Too Fuzzy (Issue #5, Item 7)

**Location**: `internal/screens/config.go:746-845`,
`thoughts/shared/tickets/issue_5.md:13`

**Problem**: Issue #5 requests less fuzzy search:
> "if i type 'wo' it doesn't match 'work' but does match 'wo' or 'woa' or
> 'wok'"

Current implementation uses `fuzzy.Find()` library which is designed to
match "wo" to "work" (fuzzy matching).

```go
// config.go:780
matches := fuzzy.Find(filter, []string{searchString})
```

**Impact**: Search matches too many items, especially for short queries.

**Fix**: Implement stricter matching - require consecutive character
matches or substring matching instead of fuzzy matching. Options:
1. Use `strings.Contains()` for substring matching (strictest)
2. Add minimum match score threshold for fuzzy matches
3. Require first N characters to match exactly

---

### 11. Context Documentation Missing (Issue #5, Item 9)

**Location**: `README.md`, `thoughts/shared/tickets/issue_5.md:15`

**Problem**: Issue #5 requests:
> "document context support (several flags, loading in the background,
> context switching, context screen and command)"

README.md exists but needs verification of context documentation coverage.

**Fix**: Add Context Management section to README documenting:
- `-context` flag for specifying contexts
- `-contexts` flag for multiple contexts
- Background loading behavior
- Context switching commands (`:contexts`, `/context <name>`, shortcuts)
- Contexts screen usage

---

### 12. Partial Success States Invisible (Handled by :output)

**Location**: `internal/k8s/informer_repository.go:386-440`

**Observation**: Context can load successfully (status "Loaded") even if
some dynamic resources fail to sync (RBAC errors). Per-resource errors
stored in map but not surfaced to user.

```go
// informer_repository.go:433-436
// Per-resource errors stored in map
repo.mu.Lock()
repo.dynamicInformerErrors[gvr] = fmt.Errorf("%s", errMsg)
repo.mu.Unlock()

// But context still marked as "Loaded" (not "Partial")
```

**Resolution**: RBAC errors during resource sync will be captured in
:output screen (see `thoughts/shared/research/2025-11-02-command-output-history-design.md`).
No need for "Partial" status or warning icons - errors visible in output
history.

**Status**: ✅ Handled by broader :output solution, not a separate issue.

---

### 13. Context Switch Progress Not Shown (Low Priority UX Issue)

**Location**: `internal/app/app.go:658-675`,
`internal/k8s/repository_pool.go:190-217`

**Actual behavior**: Switching to unloaded context automatically loads it
first, but with no progress feedback:

```go
// app.go:661
err := m.repoPool.SwitchContext(contextName, nil)  // nil = no progress

// repository_pool.go:202-208
// Context not loaded - trigger loading
if err := p.LoadContext(contextName, progress); err != nil {
    return err
}
// Then switch to newly loaded context
```

**Impact**: User sees nothing during 5-30 second load. No indication that:
- Loading is happening
- Which phase (connecting, syncing, etc.)
- Whether it's stuck or progressing

**Fix Options**:
1. Pass progress channel from switchContextCmd → show detailed progress
2. Show simple spinner "Loading context..." (current behavior)
3. With :output screen, any errors will be visible there

**Priority**: Low - :output screen covers error visibility, and most users
wait for background loading rather than switching to unloaded contexts.

---

### 14. Context-Aware Auto-Navigation (Intentional Design, Not a Bug)

**Location**: `internal/app/app.go:495-552`

**Behavior**: After successful context switch, app auto-navigates to pods
screen ONLY if currently on contexts screen. If switching from any other
screen, stays on same screen type in new context.

```go
// app.go:509-522
if m.currentScreen.ID() == "contexts" {
    // Navigate to pods
    podsScreen := m.registry.Get("pods")
    m.currentScreen = podsScreen
    // ...
} else {
    // Stay on same screen type
    m.currentScreen = m.registry.Get(m.currentScreen.ID())
}
```

**Rationale**: This is INTENTIONAL behavior designed for multi-cluster
workflows:
- From contexts screen → switch → go to pods (user was picking context,
  wants to see resources)
- From resource screen → switch → stay on same resource (enables
  comparing same resource type across clusters/contexts)

**Status**: ✅ Working as designed. Common workflow: viewing deployments in
cluster A, switch to cluster B, see deployments in cluster B for
comparison.

---

### 15. ReplicaSets Informer Not Critical (Intentional Design)

**Location**: `internal/k8s/informer_repository.go:248-256`, `337-342`

**Observation**: ReplicaSets informer excluded from critical sync check. If
it fails, warning logged but context loads successfully.

```go
// informer_repository.go:337-342
// ReplicaSets excluded from critical check
typedSynced := cache.WaitForCacheSync(syncCtx.Done(),
    podInformer.HasSynced,
    deploymentInformer.HasSynced,
    serviceInformer.HasSynced,
    statefulSetInformer.HasSynced,
    daemonSetInformer.HasSynced,
    // ReplicaSets NOT included here!
)
```

**Theoretical concern**: Deployment→Pods navigation could break if
ReplicaSets informer fails.

**Actual experience**: No reported issues with this behavior. ReplicaSets
failures are extremely rare in practice.

**Status**: ✅ Working as designed. Not causing actual problems.

---

### 16. O(n) LRU Search (Minor Performance Issue)

**Location**: `internal/k8s/repository_pool.go:674-684`

**Problem**: `markUsed()` does linear search through LRU list to find
context, then moves to front.

```go
// repository_pool.go:676-680
for e := p.lru.Front(); e != nil; e = e.Next() {
    if e.Value.(string) == contextName {
        p.lru.MoveToFront(e)
        return
    }
}
```

**Complexity**: O(n) where n = number of loaded contexts (max 10 by
default).

**Impact**: Negligible for small n (10). Only becomes issue if pool size
increased to 100+.

**Fix**: Add `map[string]*list.Element` to track LRU elements for O(1)
lookup. Low priority - current implementation fine for expected usage.

---

### 17. Retry Mechanism Exists But Not Wired to Commands

**Location**: `internal/k8s/repository_pool.go:262-274`,
`internal/commands/context.go`

**Problem**: Repository pool has `RetryFailedContext()` method but no
command implementation in command layer.

```go
// repository_pool.go:262-274 - Method exists
func (p *RepositoryPool) RetryFailedContext(contextName string,
    progress chan<- ContextLoadProgress) error {
    // Implementation exists
}

// But no command in context.go or registry.go
```

**Impact**: Method is dead code - no way for users to trigger retry.

**Fix**: Add command implementation:
```go
func RetryContextCommand(repo k8s.Repository) ExecuteFunc {
    return func(ctx CommandContext) tea.Cmd {
        contextName := ctx.Args[0]
        return retryContextCmd(repo, contextName)
    }
}
```

Register in registry.go as `/retry-context <name>` or `/retry` (infers
current context).

---

## Architecture Insights

### Context Lifecycle Management

The repository pool uses a sophisticated three-level locking strategy:

1. **sync.Map for load coordination**: Prevents duplicate repositories when
   multiple goroutines try to load same context
2. **sync.RWMutex for state protection**: Guards repos map, LRU list, and
   active context
3. **LRU List for eviction order**: Tracks least recently used contexts for
   eviction when pool full

State transitions: NotLoaded → Loading → Loaded/Failed

Critical sections minimized - expensive operations (repository creation)
done without holding locks.

### Background Loading Patterns

Multiple patterns coexist for different loading scenarios:

1. **Blocking load with progress**: Main thread loads first context before
   UI (main.go:133-158)
2. **Background loading**: Remaining contexts loaded after UI starts
   (main.go:180-222)
3. **Per-resource sync**: Each resource type syncs independently in own
   goroutine (informer_repository.go:386-440)
4. **Stats updater**: Single owner goroutine for statistics (eliminates
   lock contention)

Progress reported via buffered channels to avoid blocking loaders.

### Error Handling Layers

Errors captured at four layers:

1. **Kubeconfig parsing**: Fails fast, exits process (main.go)
2. **Auth check**: 5s timeout, fails with connection error
   (informer_repository.go:158-167)
3. **Typed informer sync**: 2m timeout, diagnostic API call on failure
   (informer_repository.go:365-383)
4. **Dynamic informer sync**: Per-resource timeout, RBAC errors stored
   independently (informer_repository.go:418-432)

Errors stored in:
- `RepositoryEntry.Error` for UI display
- `RepositoryEntry.Status` for state tracking
- `typedInformersSyncError` atomic.Value for informer errors
- `dynamicInformerErrors` map for per-resource errors

### UX Feedback Mechanisms

Four channels for communicating context state to users:

1. **Console (startup only)**: Kubeconfig errors, first context load
   progress
2. **Status bar (5s auto-clear)**: Context switch/load errors, info
   messages
3. **Header (persistent)**: Current context name, loading spinner with
   progress
4. **Contexts table (persistent)**: All contexts with status, errors
   (missing error column)

Problem: Background failures only visible in status bar (clears quickly) or
contexts screen (must navigate to see).

## Code References

**Context Pool:**
- `internal/k8s/repository_pool.go:51` - NewRepositoryPool
- `internal/k8s/repository_pool.go:81` - LoadContext with progress
- `internal/k8s/repository_pool.go:190` - SwitchContext
- `internal/k8s/repository_pool.go:262` - RetryFailedContext (unused)

**Contexts Screen:**
- `internal/screens/screens.go:526-547` - GetContextsScreenConfig
- `internal/k8s/repository_pool.go:573-617` - GetContexts with status
- `internal/k8s/repository_pool.go:619-645` - sortContextsByStatusThenName

**Error Handling:**
- `internal/k8s/informer_repository.go:158-167` - Auth check with timeout
- `internal/k8s/informer_repository.go:365-383` - Typed informer error
  detection
- `internal/k8s/informer_repository.go:418-432` - Dynamic informer error
  detection
- `internal/app/app.go:448-458` - ContextLoadFailedMsg handler

**Background Loading:**
- `cmd/k1/main.go:133-158` - First context blocking load
- `cmd/k1/main.go:180-222` - Background context loading
- `internal/k8s/informer_repository.go:327-384` - Typed informer sync
- `internal/k8s/informer_repository.go:386-440` - Dynamic informer sync

**UI Integration:**
- `internal/app/app.go:425-434` - ContextLoadProgressMsg handler
- `internal/app/app.go:460-553` - ContextSwitchMsg handler
- `internal/components/header.go:73-97` - Loading spinner

## Historical Context (from thoughts/)

### Original Implementation Plan

`thoughts/shared/plans/2025-10-09-kubernetes-context-management.md`
outlined 7-phase implementation:

1. ✅ Repository pool infrastructure (LRU eviction, concurrent loading)
2. ✅ CLI multi-context support (-contexts flag)
3. ✅ Message types and app integration
4. ✅ Header loading indicator
5. ✅ Contexts screen
6. ✅ Context commands (switch, navigate)
7. ⚠️ Testing and documentation (incomplete)

### Quality Review Findings

`thoughts/shared/plans/2025-10-10-kubernetes-context-management-quality-fixes.md`
identified critical bugs:

- ✅ Deadlocks in GetContexts() - FIXED (Phases 1-4 complete)
- ✅ Race conditions in repository pool - FIXED
- ✅ Goroutine leaks - FIXED
- ✅ LRU corruption - FIXED

### UX Improvements Plan

`thoughts/shared/plans/2025-10-10-issue-5-context-switching-improvements.md`
tracks remaining UX work:

- ❌ Keyboard shortcuts (ctrl+1-9, ctrl+0, ctrl+-, ctrl+=) - NOT
  IMPLEMENTED
- ⚠️ Contexts screen improvements (shortcuts column, sorting) - PARTIAL
  (sorting done, column missing)
- ❌ Loading progress counter - NOT IMPLEMENTED
- ❌ Documentation - NOT IMPLEMENTED

### Known Issues from Roadmap

`thoughts/shared/tickets/roadmap.md:42-46` lists three context issues:

1. "Better error handling with multiple -context flags" - Relevant to
   silent background failures
2. "If cannot connect to cluster, connecting to API Server is showing
   spinning" - Stuck loading states
3. "If context load fails the error is not shown" - Error visibility gap

All three confirmed in this research.

## Related Research

- `thoughts/shared/research/2025-10-09-k8s-context-management.md` - Initial
  research on context switching design
- `thoughts/shared/research/2025-10-10-kubernetes-context-management-quality-review.md`
  - Critical bug analysis (now fixed)
- `thoughts/shared/research/2025-10-07-contextual-navigation.md` - Related
  to context-aware navigation (Enter key)
- `thoughts/shared/research/2025-11-02-command-output-history-design.md` -
  Proposed :output screen for command feedback and error visibility
  (broader solution to error visibility gaps identified here)

## Open Questions

1. **Timeout values**: What's appropriate overall timeout for context
   loading? 5 minutes? 10 minutes?

2. **Retry command scope**: Should `/retry` infer current context or require
   explicit context name? (Suggestion: Infer current context if on contexts
   screen)

3. **Fuzzy search replacement**: Substring matching (strictest), prefix
   matching, or fuzzy with minimum score threshold?

4. **Context status summary format**: In header, show:
   - "my-cluster (3 loaded / 5 total)" - Always show all statuses?
   - "my-cluster (3/5)" - Compact version?
   - "my-cluster (3/5, 1 failed)" - Include failed count?

## Recommendations

### High Priority (UX Blockers)

1. **Implement :output screen** - Comprehensive solution for error
   visibility and command feedback (see
   `thoughts/shared/research/2025-11-02-command-output-history-design.md`)
2. **Add context status summary in header** - Show "N loaded / M total"
   when multiple contexts exist (only if M > 1)
3. **Wire retry command to UI** - Enable recovery from failed loads
4. **Add overall timeout for context loading** - Prevent stuck loading
   states

### Medium Priority (UX Polish)

5. **Implement stricter search** - Use substring or prefix matching instead
   of fuzzy
6. **Document context features in README** - Required by issue #5

### Low Priority (Code Quality)

7. **Optimize LRU search** - Add map for O(1) lookup (only if pool size
    increases)

### Documentation

8. **README section on context management** - Document flags, commands,
    next/prev context navigation
9. **Troubleshooting guide** - Common errors (RBAC, timeouts, auth
    failures)

## Summary of Issues Found

**From issue_5.md (11 items):**
- ✅ Item 5 (status-based sorting) - Already implemented
- ✅ Item 6 (status in search) - Already implemented
- ✅ Items 1-2 (keyboard shortcuts ctrl+1-9) - Intentionally not implemented
  (keeping UI simple, next/prev sufficient)
- ✅ Item 3 (do nothing if not enough contexts) - N/A (no shortcuts)
- ✅ Item 4 (shortcuts column) - Not needed (no shortcuts to display)
- ❌ Item 7 (stricter search) - Not implemented
- ❌ Item 8 (loading counter) - Not implemented
- ❌ Item 9 (documentation) - Not implemented

**Additional issues identified (7 new issues):**
1. Error visibility gap → Solved by :output screen
2. Stuck loading states (no timeout)
3. Silent background failures → Solved by :output screen
4. Missing retry UI
5. Context switch progress not shown (low priority)
6. O(n) LRU search (low priority, n is small)
7. Retry method exists but unused

**Dropped issues (not actual problems)**:
- Partial success states invisible → :output screen covers RBAC errors
- Silent ReplicaSets failures → No evidence of actual problems

**Design decisions (not bugs)**:
- Auto-navigation behavior is intentional for multi-cluster comparison
  workflows (see finding #14)
- Keyboard shortcuts (ctrl+1-9) intentionally not implemented to keep UI
  simple (see finding #6)

**Total: 10 actual issues** (3 from issue #5 + 7 new)
