---
date: 2025-10-09T00:00:00Z
researcher: Claude Code
git_commit: 131eb33487f06a1d90dfdf3db3eb0dda4e7ccf2f
branch: docs/designs
repository: k1-designs
topic: "Kubernetes Context Management: Screens, Commands, and Multi-Context Support"
tags: [research, codebase, kubernetes, context-switching, informers, screens, commands, header]
status: complete
last_updated: 2025-10-09
last_updated_by: Claude Code
---

# Research: Kubernetes Context Management

**Date**: 2025-10-09
**Researcher**: Claude Code
**Git Commit**: 131eb33487f06a1d90dfdf3db3eb0dda4e7ccf2f
**Branch**: docs/designs
**Repository**: k1-designs

## Research Question

How can we add Kubernetes context management to k1 with the following
features:
1. Screen to list, find, and choose contexts
2. Command to switch context (`/context <name>`)
3. Show current context in header
4. Support cycling between contexts
5. Maintain separate cache/informers per context

## Summary

k1 currently supports single-context operation via CLI flags at startup.
Adding full multi-context support requires:
1. **Context Selection Screen**: Use existing ConfigScreen pattern with
   custom context listing
2. **Context Switch Command**: Add `/context <name>` command with repository
   method
3. **Header Display**: Add context field to header component (simple setter)
4. **Multi-Context Architecture**: Implement repository pool pattern with LRU
   eviction
5. **Informer Lifecycle**: Each context needs isolated informer instances
   with proper cleanup

**Key Finding**: Current architecture assumes single static context. Adding
runtime context switching requires significant repository lifecycle
management changes but can leverage existing screen/command patterns.

## Detailed Findings

### 1. Screen Implementation Patterns

#### ConfigScreen Pattern (Recommended)

**Primary implementation**: `internal/screens/config.go` (597 lines)

k1 uses a config-driven screen pattern that eliminates ~500+ lines of
boilerplate per resource screen. All 16 resource screens (Pods, Deployments,
Services, etc.) use this pattern.

**Key components**:
- `ScreenConfig` struct (lines 41-61): Declarative screen configuration
  - `ID`, `Title`, `ResourceType`
  - `Columns []ColumnConfig`: Table column definitions
  - `SearchFields []string`: Fields for fuzzy search
  - `Operations []OperationConfig`: Available commands
  - `NavigationHandler NavigationFunc`: Optional Enter key handler
  - Behavior flags: `TrackSelection`, `EnablePeriodicRefresh`
- `ConfigScreen` struct (lines 65-80): Generic screen implementation
  - Handles filtering, selection tracking, navigation
  - Implements `types.Screen` and `types.ScreenWithSelection` interfaces

**For context selection screen**:

File: `internal/screens/screens.go`
```go
func GetContextsScreenConfig() ScreenConfig {
    return ScreenConfig{
        ID:           "contexts",
        Title:        "Contexts",
        ResourceType: k8s.ResourceTypeContext, // New type needed
        Columns: []ColumnConfig{
            {Field: "Name", Title: "Name", Width: 0},
            {Field: "Cluster", Title: "Cluster", Width: 40},
            {Field: "User", Title: "User", Width: 30},
            {Field: "Current", Title: "Current", Width: 10},
        },
        SearchFields: []string{"Name", "Cluster", "User"},
        Operations: []OperationConfig{
            {ID: "switch", Name: "Switch", Description: "Switch to context",
             Shortcut: "s"},
        },
        NavigationHandler: navigateToPodsForContext(), // Switch on Enter
        TrackSelection:    true,
    }
}
```

**Registration** in `internal/app/app.go` (lines 48-78):
```go
registry.Register(screens.NewConfigScreen(
    screens.GetContextsScreenConfig(), repo, theme))
```

**References**:
- `internal/screens/config.go:41-61` - ScreenConfig struct
- `internal/screens/screens.go:11-460` - All 16 screen configurations
- `internal/screens/navigation.go:14-343` - Navigation handler factories

#### Screen Interface

**Location**: `internal/types/types.go:11-23`

All screens must implement:
```go
type Screen interface {
    tea.Model                    // Init, Update, View
    ID() string                  // Unique screen identifier
    Title() string               // Display title
    HelpText() string            // Help bar text
    Operations() []Operation     // Available commands
}

type ScreenWithSelection interface {
    Screen
    GetSelectedResource() map[string]interface{}
    ApplyFilterContext(*FilterContext)
    GetFilterContext() *FilterContext
}
```

### 2. Command System Architecture

#### Command Registry Pattern

**Central registry**: `internal/commands/registry.go`

The command system uses a registry pattern with ~40 registered commands
across three categories:
- `CategoryResource` (`:` prefix): Navigation commands
- `CategoryAction` (`/` prefix): Resource operation commands
- `CategoryLLMAction` (`/ai` prefix): Natural language commands

**Command structure** (`internal/commands/types.go:13-27`):
```go
type Command struct {
    Name          string
    Description   string
    Category      CommandCategory
    Execute       ExecuteFunc
    ResourceTypes []k8s.ResourceType
    ArgsType      any
    ArgPattern    string
    NeedsConfirm  bool
}

type ExecuteFunc func(CommandContext) tea.Cmd
```

**For `/context <name>` command**:

File: `internal/commands/context.go` (NEW FILE)
```go
type ContextArgs struct {
    ContextName string `inline:"0"`
}

func ContextCommand(repo k8s.Repository) ExecuteFunc {
    return func(ctx CommandContext) tea.Cmd {
        var args ContextArgs
        if err := ctx.ParseArgs(&args); err != nil {
            return messages.ErrorCmd("Invalid args: %v", err)
        }

        return func() tea.Msg {
            if err := repo.SwitchContext(args.ContextName); err != nil {
                return messages.ErrorCmd("Context switch failed: %v",
                    err)()
            }
            return messages.SuccessCmd("Switched to context: %s",
                args.ContextName)()
        }
    }
}
```

**Registry registration** (`internal/commands/registry.go:45-118`):
```go
{
    Name:        "context",
    Description: "Switch Kubernetes context",
    Category:    CategoryAction,
    ArgsType:    &ContextArgs{},
    ArgPattern:  " <context-name>",
    Execute:     ContextCommand(repo),
}
```

**Command execution flow**:
1. CommandBar → `executor.Execute()` (builds CommandContext)
2. Registry → `Get(name, category)` (finds command)
3. Command → `Execute(ctx)` (returns tea.Cmd)
4. Result → Status message (success/error)

**References**:
- `internal/commands/registry.go:45-118` - Command registration
- `internal/commands/types.go:13-27` - Command struct
- `internal/commands/executor.go:29-90` - kubectl executor
- `internal/components/commandbar/executor.go:23-121` - Command bar executor
- `internal/commands/navigation.go:14-96` - Navigation command pattern

### 3. Header Component Structure

#### Passive Component Pattern

**Location**: `internal/components/header.go` (118 lines)

The header is a **passive component** (no Update method) that displays
application-level information through setter methods.

**Current state** (lines 13-22):
```go
type Header struct {
    appName      string     // "k1"
    screenTitle  string     // "Pods", "Deployments", etc.
    namespace    string     // Unused field
    itemCount    int        // Unused field
    filterText   string     // Contextual navigation filter
    lastRefresh  time.Time  // Refresh timestamp
    width        int        // Terminal width
    theme        *ui.Theme  // Theme colors
}
```

**Display layout** (`View()` at line 55):
- **Left section**: Screen title, filter text (bullet-separated)
- **Right section**: Last refresh time ("5s ago", "2m ago")
- **Dynamic spacing**: Calculated to push right section to edge

**Adding context display**:

Step 1: Add field to struct (line 22):
```go
context string  // Kubernetes context name
```

Step 2: Add setter method (after line 49):
```go
func (h *Header) SetContext(context string) {
    h.context = context
}
```

Step 3: Update View() method (lines 66-88):
```go
leftParts := []string{}

if h.screenTitle != "" {
    leftParts = append(leftParts, h.screenTitle)
}

// NEW: Add context display
if h.context != "" {
    contextStyle := lipgloss.NewStyle().Foreground(h.theme.Muted)
    leftParts = append(leftParts,
        contextStyle.Render("("+h.context+")"))
}

if h.filterText != "" {
    leftParts = append(leftParts, h.filterText)
}
```

Step 4: Call setter from app initialization (`internal/app/app.go:82-84`):
```go
header := components.NewHeader("k1", theme)
header.SetScreenTitle(initialScreen.Title())
header.SetWidth(80)
header.SetContext(repo.GetContext())  // NEW
```

**Example output**: `"Pods (minikube) • filtered by Deployment: nginx"`

**Message flow to header**:
```
types.ScreenSwitchMsg → app.Update() → header.SetScreenTitle()
                                      → header.SetFilterText()
types.RefreshCompleteMsg → app.Update() → header.SetLastRefresh()
tea.WindowSizeMsg → app.Update() → header.SetWidth()
```

**References**:
- `internal/components/header.go:13-22` - Header struct
- `internal/components/header.go:55-118` - View method
- `internal/app/app.go:129` - Window resize handling
- `internal/app/app.go:252-259` - Screen switch handling

### 4. Repository and Informer Management

#### Single-Context Architecture

**Current implementation**: `internal/k8s/informer_repository.go`

k1's repository uses Kubernetes **informers** (client-go watch/cache
mechanism) for real-time cluster data. The current architecture assumes a
**single static context** for the application lifetime.

**Initialization flow** (lines 94-280):

**Phase 1: Kubeconfig Loading** (lines 94-117)
```go
// Resolve kubeconfig path (lines 96-102)
if kubeconfig == "" {
    kubeconfig = filepath.Join(home, ".kube", "config")
}

// Build config with context override (lines 104-114)
loadingRules := &clientcmd.ClientConfigLoadingRules{
    ExplicitPath: kubeconfig,
}
configOverrides := &clientcmd.ConfigOverrides{}
if contextName != "" {
    configOverrides.CurrentContext = contextName
}
config := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
    loadingRules, configOverrides).ClientConfig()
```

**Phase 2: Client Creation** (lines 119-136)
```go
// Enable protobuf (70-90% faster) (line 120)
config.ContentType = "application/vnd.kubernetes.protobuf"

// Create clients (lines 123-132)
clientset := kubernetes.NewForConfig(config)
dynamicClient := dynamic.NewForConfig(config)

// Create informer factories (30s resync) (lines 135-136)
factory := informers.NewSharedInformerFactory(clientset,
    InformerResyncPeriod)
dynamicFactory := dynamicinformer.NewDynamicSharedInformerFactory(
    dynamicClient, InformerResyncPeriod)
```

**Phase 3: Informer Setup** (lines 138-173)
- Typed informers for 6 core resources (Pods, Deployments, Services, etc.)
- Dynamic informers for all 16 registered resource types
- Each informer maintains persistent HTTP watch connection

**Phase 4: Start and Sync** (lines 176-221)
```go
// Start both factories (lines 179-180)
factory.Start(ctx.Done())
dynamicFactory.Start(ctx.Done())

// Sync with 10s timeout (lines 184-185)
syncCtx, syncCancel := context.WithTimeout(ctx,
    InformerSyncTimeout)

// Wait for all caches to sync (lines 191-215)
typedSynced := cache.WaitForCacheSync(syncCtx.Done(),
    podInformer.HasSynced, deploymentInformer.HasSynced, ...)
```

**Phase 5: Index Setup** (lines 273-277)

Registers event handlers for 10 performance indexes:
- `podsByNode`, `podsByNamespace`, `podsByOwnerUID`
- `podsByConfigMap`, `podsBySecret`, `podsByPVC`
- `jobsByOwnerUID`, `jobsByNamespace`
- `replicaSetsByOwnerUID`

These indexes enable O(1) filtered queries like "Pods for Deployment" or
"Pods on Node".

#### Two-Layer Caching Architecture

**Layer 1: Informer Cache** (client-go)
- In-memory `cache.ThreadSafeStore` with RW locks
- Automatic updates via watch connections (Add/Update/Delete events)
- Resync every 30 seconds from API server
- O(1) lookups via `Lister.List(labels.Everything())`

**Layer 2: Performance Indexes** (application)
- 10 custom indexes for filtered queries
- Updated in real-time via event handlers
- Protected by `sync.RWMutex`
- Example: `podsByNode[nodeName]` → O(1) lookup instead of O(n) filtering

**Index maintenance pattern** (lines 1083-1170):
```go
podInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
    AddFunc: func(obj interface{}) {
        pod := obj.(*corev1.Pod)
        r.updatePodIndexes(pod, nil)  // Add to all 6 pod indexes
    },
    UpdateFunc: func(oldObj, newObj interface{}) {
        oldPod := oldObj.(*corev1.Pod)
        newPod := newObj.(*corev1.Pod)
        r.updatePodIndexes(newPod, oldPod)  // Remove old, add new
    },
    DeleteFunc: func(obj interface{}) {
        pod := obj.(*corev1.Pod)
        r.removePodFromIndexes(pod)  // Remove from all 6 indexes
    },
})
```

#### Context Information

**Repository interface** (`internal/k8s/repository.go:109-115`):
```go
type Repository interface {
    // ... other methods ...
    GetKubeconfig() string  // Path used
    GetContext() string     // Context name
}
```

**Implementation** (`internal/k8s/informer_repository.go:1323-1330`):
```go
func (r *InformerRepository) GetKubeconfig() string {
    return r.kubeconfig  // Set at initialization
}

func (r *InformerRepository) GetContext() string {
    return r.contextName  // Set at initialization
}
```

**Application entry** (`cmd/k1/main.go:24-62`):
```go
// CLI flags (lines 25-27)
kubeconfigFlag := flag.String("kubeconfig", "", "...")
contextFlag := flag.String("context", "", "...")

// Repository created once at startup (line 55)
repo, err := k8s.NewInformerRepository(*kubeconfigFlag, *contextFlag)

// Cleanup on exit (line 65)
defer repo.Close()
```

**References**:
- `internal/k8s/informer_repository.go:94-280` - Constructor and setup
- `internal/k8s/informer_repository.go:1083-1170` - Index maintenance
- `internal/k8s/repository.go:79-115` - Repository interface
- `cmd/k1/main.go:55` - Application initialization

### 5. Multi-Context Support Architecture

#### Current Limitations

**Single repository per application instance**:
- Repository created once at `main.go:55`
- Stored in `app.Model.repo` field
- All 16 informers share single dynamic factory
- Context loaded once, no reload mechanism
- Indexes tied to repository instance

**Memory profile per context** (from 71-resource research):
- ~50MB for 1000 pods (1KB per resource + indexes)
- Startup time: 5-15 seconds for cache sync
- 16 persistent HTTP watch connections

#### Design Options for Multi-Context

**Option A: Repository Pool (Recommended)**

Create pool of active repositories with LRU eviction:

```go
type RepositoryPool struct {
    mu         sync.RWMutex
    repos      map[string]*k8s.InformerRepository
    active     string                  // Current context
    maxSize    int                     // Limit (5-10 contexts)
    lru        *list.List              // Eviction order
    kubeconfig string
}

func (p *RepositoryPool) GetRepository(context string)
    (*k8s.InformerRepository, error) {
    p.mu.RLock()
    if repo, ok := p.repos[context]; ok {
        p.mu.RUnlock()
        p.markUsed(context)  // Update LRU
        return repo, nil
    }
    p.mu.RUnlock()

    // Create new repository
    p.mu.Lock()
    defer p.mu.Unlock()

    // Evict if at capacity
    if len(p.repos) >= p.maxSize {
        oldest := p.lru.Back().Value.(string)
        p.repos[oldest].Close()  // Cleanup informers
        delete(p.repos, oldest)
        p.lru.Remove(p.lru.Back())
    }

    // Initialize new repository (5-15s sync time)
    repo, err := k8s.NewInformerRepository(p.kubeconfig, context)
    if err != nil {
        return nil, err
    }

    p.repos[context] = repo
    p.lru.PushFront(context)
    return repo, nil
}

func (p *RepositoryPool) SwitchContext(context string) error {
    repo, err := p.GetRepository(context)  // May block 5-15s
    if err != nil {
        return err
    }

    p.mu.Lock()
    p.active = context
    p.mu.Unlock()

    return nil
}
```

**Integration with app model**:
```go
type Model struct {
    // ... existing fields ...
    repoPool    *RepositoryPool  // Replace single repo
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case types.ContextSwitchMsg:
        // Show loading spinner during 5-15s sync
        return m, tea.Batch(
            m.showLoadingCmd(),
            m.repoPool.SwitchContext(msg.ContextName),
        )
    }
}
```

**Advantages**:
- Complete isolation between contexts
- No cache invalidation complexity
- Background preloading possible (limit 5-10 contexts)
- Existing code works unchanged (screens use repo interface)

**Challenges**:
- Memory overhead: ~50MB × N contexts (manageable with LRU)
- Context switch latency: 5-15 seconds for cache sync (show spinner)
- Cleanup complexity: Must stop informers on eviction

**Option B: Shared Repository with Context Parameter**

Modify all repository methods to accept context parameter:

```go
type Repository interface {
    GetResources(context string, resourceType ResourceType)
        ([]any, error)
    GetPodsForDeployment(context, namespace, name string)
        ([]Pod, error)
    // ... 40+ methods need context parameter
}
```

**Disadvantages**:
- Requires rewriting all 40+ repository methods
- Cache invalidation on context switch is complex
- Indexes need context-aware keys (breaks existing code)
- High risk of bugs (every method changes)

**Option C: Repository per Context (No Pool)**

Create new repository on every context switch, discard old:

**Disadvantages**:
- 5-15 second latency on every switch (blocking UX)
- No caching of recent contexts
- Informer cleanup on every switch (resource churn)

#### Recommended Approach: Repository Pool

**Why**:
1. Balances memory (~250MB for 5 contexts) vs. UX (instant switches)
2. LRU eviction provides natural cleanup
3. Existing code works unchanged (screens use repo interface)
4. Background preloading possible for common contexts

**Implementation steps**:
1. Create `RepositoryPool` in `internal/k8s/repository_pool.go`
2. Add `SwitchContext(name string) error` to Repository interface
3. Modify `app.Model` to use pool instead of single repo
4. Add loading spinner for context switches (5-15s sync time)
5. Implement context listing via kubeconfig parsing

## Code References

### Screen Pattern
- `internal/screens/config.go:41-80` - ConfigScreen struct and ScreenConfig
- `internal/screens/screens.go:11-460` - All 16 screen configurations
- `internal/screens/navigation.go:14-343` - Navigation handler factories
- `internal/types/types.go:11-23` - Screen interface

### Command System
- `internal/commands/registry.go:45-118` - Command registration
- `internal/commands/types.go:13-27` - Command struct definition
- `internal/commands/navigation.go:14-96` - Navigation command pattern
- `internal/components/commandbar/executor.go:23-121` - Execution flow

### Header Component
- `internal/components/header.go:13-22` - Header state struct
- `internal/components/header.go:55-118` - View rendering method
- `internal/app/app.go:82-84` - Header initialization
- `internal/app/app.go:252-259` - Screen switch handling

### Repository and Informers
- `internal/k8s/informer_repository.go:94-280` - Constructor and setup
- `internal/k8s/informer_repository.go:1083-1170` - Index maintenance
- `internal/k8s/repository.go:79-115` - Repository interface
- `cmd/k1/main.go:55` - Application initialization

## Architecture Insights

### 1. Config-Driven Pattern Reduces Boilerplate

k1's ConfigScreen pattern eliminates ~500+ lines per resource screen. All 16
resource screens share the same 597-line implementation, configured with
~30 lines of declarative config. This pattern should be used for the context
selection screen.

### 2. Repository Interface Enables Isolation

The Repository interface (`internal/k8s/repository.go`) decouples screens
from concrete implementations. This means:
- Screens don't know if they're using InformerRepository or DummyRepository
- Context switching can be implemented entirely at the repository layer
- Existing screens work unchanged with multi-context support

### 3. Informers Assume Static Configuration

Kubernetes informers (client-go) are designed for long-running processes
with static cluster configuration. They:
- Establish persistent HTTP watch connections
- Build in-memory caches on initialization (5-15s sync time)
- Cannot "switch" contexts (must stop/restart)

This is why repository pooling is necessary - informers cannot be reused
across contexts.

### 4. Performance Indexes Enable Fast Filtering

k1's 10 performance indexes (podsByNode, podsByOwnerUID, etc.) provide O(1)
lookups for filtered queries like "Pods for Deployment". With multi-context
support:
- Each repository maintains its own indexes
- No cross-context contamination
- Memory overhead: ~5-10MB per context for indexes

### 5. Header as Passive Component

The header component follows a "passive" pattern (no Update method), making
it trivial to add context display:
1. Add `context string` field
2. Add `SetContext(string)` setter
3. Update View() to render context
4. Call setter from app initialization

No message handling or state management needed.

## Historical Context (from thoughts/)

No existing documentation about multi-context support was found in the
thoughts/ directory. However, related documents provide context:

**Explicitly Excluded in Phase 2**:
- `thoughts/shared/plans/2025-10-08-issue-3-scale-to-31-resources.md` -
  "What We're NOT Doing: Multi-cluster support (single kubeconfig context
  only)"
- `thoughts/shared/research/2025-10-08-scaling-to-71-api-resources.md` -
  "What We're NOT Doing: single kubeconfig context only"

**Related Concepts**:
- `thoughts/shared/research/2025-10-07-contextual-navigation.md` - Research
  on intra-cluster navigation (resource-to-resource), not context switching
- `thoughts/shared/research/2025-10-08-issue-3-implementation-challenges.md`
  - Mentions "Context management (lifecycle)" as one of the responsibilities
  in the informer repository god file

**Key Insight**: Multi-context support was deliberately deferred in Phase 2
to focus on scaling resource types within a single cluster. This research
provides the foundation for Phase 3+ multi-context implementation.

## Open Questions

### 1. Context Listing Implementation

**Question**: How should we enumerate available contexts from kubeconfig?

**Options**:
- Parse kubeconfig YAML directly (via `clientcmd.LoadFromFile()`)
- Cache context list on application startup
- Refresh context list on demand (if kubeconfig changes)

**Recommendation**: Use `clientcmd.LoadFromFile()` to parse kubeconfig,
extract context list, and cache in repository pool. Refresh on explicit
user command (e.g., `:contexts` or `/refresh-contexts`).

### 2. Context Switch UX

**Question**: How should we handle 5-15 second sync latency during context
switches?

**Decision**: Non-blocking spinner with incremental progress feedback.

**Implementation**:

1. **Message-Driven Architecture**: Use Bubble Tea messages for async switch
   - `ContextSwitchStartMsg`: Initiates switch, shows spinner
   - `ContextSwitchProgressMsg`: Updates status text
   - `ContextSwitchCompleteMsg`: Finalizes switch or shows error

2. **Non-Blocking Execution**: Context switch runs in background goroutine
   - UI remains fully responsive (60fps)
   - User can navigate, filter, even cancel
   - Progress messages sent via channel

3. **Visual Feedback in Header**:

   **Before Switch:**
   ```
   Pods (minikube) • 150 items                    Last refresh: 2s ago
   ```

   **During Switch (shows progress with resource counts):**
   ```
   Pods ⠋ minikube → production (Syncing pods: 1247) Last refresh: 5s ago
   ```

   Or with status messages:
   ```
   Pods ⠋ production (Connecting to API server...) Last refresh: 5s ago
   Pods ⠙ production (Syncing core resources...)   Last refresh: 5s ago
   Pods ⠹ production (Syncing pods: 1247...)       Last refresh: 5s ago
   Pods ⠸ production (Syncing deployments: 42...)  Last refresh: 5s ago
   ```

   **After Success:**
   ```
   Pods (production) • 1247 items                 Last refresh: 0s ago
   ```

   **After Failure (stays on current context):**
   ```
   Pods (minikube) • 150 items                    Last refresh: 8s ago
   Error: Context switch failed: cluster unreachable
   ```

4. **Progress Reporting**: Repository reports incremental sync status
   - "Connecting to API server..."
   - "Syncing pods: 1247 objects"
   - "Syncing deployments: 42 objects"
   - "Syncing services: 18 objects"
   - "Switch complete!"

5. **Cancellation**: Ctrl+C during switch aborts and stays on current
   context

6. **Graceful Failure**: Errors shown in status bar, current context
   preserved

7. **Repository Pool Integration**:
   ```go
   func (p *RepositoryPool) SwitchContext(context string,
       progress chan<- string) error {

       // Already cached? Instant switch
       if repo, ok := p.repos[context]; ok {
           p.active = context
           return nil
       }

       // New context: report progress
       progress <- "Initializing informers..."
       repo, err := k8s.NewInformerRepositoryWithProgress(
           p.kubeconfig, context, progress)

       // ... handle error, LRU eviction ...
   }
   ```

**Key Benefits**:
- Non-blocking: User can continue working during sync
- Incremental feedback: Shows what's loading with resource counts
- Cancellable: Abort switch if taking too long
- Graceful failure: Error shown, current context preserved
- Responsive: 60fps UI during 5-15s sync

**Resource Count Display**: Showing "Syncing pods: 1247 objects" provides:
- Progress indication (not stuck)
- Cluster size awareness (large cluster = longer sync)
- Confidence that data is loading correctly

### 3. Current Context Indicator in Context Screen

**Decision**: Use both column indicator and row styling.

**Implementation**:
- Dedicated "Current" column with "✓" symbol for active context
- Bold/highlighted styling for current context row
- Makes current context obvious even in long context lists

**Example**:
```
┌─────────────────────────────────────────────────────────────────┐
│ Name            Cluster              User        Current │
├─────────────────────────────────────────────────────────────────┤
│ minikube        minikube             minikube    ✓       │ <- Bold
│ production      prod.company.com     admin                │
│ staging         stage.company.com    admin                │
│ dev-cluster     dev.company.com      developer            │
└─────────────────────────────────────────────────────────────────┘
```

### 4. Context Cycling Keybinding

**Decision**: Start with navigation commands only.

**Implementation**:
- `:next-context` - Switch to next context in alphabetical order
- `:prev-context` - Switch to previous context in alphabetical order
- Wraps around (last → first, first → last)

**Rationale**:
- Avoids keybinding conflicts with existing shortcuts
- Consistent with k1's command-first approach
- Easy to discover via `:` palette
- Keybindings can be added later based on user feedback

**Future Consideration**: If users request keybindings frequently, add
`ctrl+]` (next) and `ctrl+[` (previous) which are unused in k1.

### 5. Repository Pool Size

**Decision**: 5 contexts by default, configurable via CLI flag.

**Implementation**:
- Default pool size: 5 contexts (~250MB for typical clusters)
- CLI flag: `-context-pool-size=N` (range: 1-10)
- LRU eviction when limit reached

**Rationale**:
- 5 contexts covers most multi-cluster workflows
- ~250MB overhead is acceptable on modern systems
- Configurable for users with limited RAM or many contexts
- Upper limit of 10 prevents excessive memory usage

**Memory Profile**:
- 1 context: ~50MB (baseline)
- 5 contexts: ~250MB (recommended)
- 10 contexts: ~500MB (maximum)

### 6. Context Switch Failure Handling

**Decision**: Stay on current context and show error message.

**Implementation**:

1. **Failure Detection**: Repository initialization returns error
2. **State Preservation**: Current context remains active
3. **Error Display**: Red error message in status bar with details
4. **User Actions**:
   - Retry: Run `/context <name>` again
   - Different context: Choose another from `:contexts` screen
   - Investigate: Check kubeconfig, credentials, network

**Error Message Examples**:
```
Error: Context switch failed: cluster unreachable (connection timeout)
Error: Context switch failed: authentication failed (expired credentials)
Error: Context switch failed: invalid kubeconfig (context 'foo' not found)
```

**No Partial Switches**: Either switch succeeds completely (all informers
synced) or fails completely (stay on current context). No half-synced
state.

## Related Research

None found - this is the first comprehensive research on k8s context
management for k1.

## Next Steps

1. **Create design document** for multi-context architecture
2. **Implement context listing** in repository (parse kubeconfig)
3. **Create context selection screen** using ConfigScreen pattern
4. **Add `/context <name>` command** to command registry
5. **Implement repository pool** with LRU eviction
6. **Add context display to header** (simple setter + view update)
7. **Add loading spinner** for context switches
8. **Implement context cycling commands** (`:next-context`, `:prev-context`)
9. **Add tests** for repository pool and context switching
10. **Update documentation** (CLAUDE.md, README.md)
