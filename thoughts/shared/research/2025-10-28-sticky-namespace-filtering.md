---
date: 2025-10-28T07:20:22+00:00
researcher: @renato0307
git_commit: 7206c20452f397727d32d009cc2183212f045b13
branch: feat/kubernetes-context-management
repository: k1
topic: "Sticky Namespace Filtering and Navigation"
tags: [research, codebase, namespace, filtering, state-management,
context-switching]
status: complete
last_updated: 2025-10-28
last_updated_by: @renato0307
---

# Research: Sticky Namespace Filtering and Navigation

**Date**: 2025-10-28T07:20:22+00:00
**Researcher**: @renato0307
**Git Commit**: 7206c20452f397727d32d009cc2183212f045b13
**Branch**: feat/kubernetes-context-management
**Repository**: k1

## Research Question

How can we implement sticky namespace filtering where:
1. Namespace selection on namespace screen persists across all resource
   screens
2. All listing screens use selected namespace as default filter
3. Provide shortcut and command to clear namespace and return to "all
   namespaces"
4. Namespace selection persists across context switches
5. Clear namespace filter when switching contexts

## Summary

The k1 TUI currently supports contextual namespace filtering (e.g.,
navigating from namespace → pods shows pods in that namespace), but lacks
**sticky** namespace filtering where the namespace filter persists across
screen navigation.

Current implementation has all components needed:
- Repository layer can filter by namespace (informer factories support
  namespace scoping)
- FilterContext already flows through screen navigation
- App model manages global state and coordinates screen switching
- Context switching infrastructure exists and re-initializes screens
- Command system ready for new commands and shortcuts

**Key finding**: The main gap is state persistence - FilterContext is
currently **ephemeral** (cleared on navigation unless explicitly passed).
We need to make namespace filtering **sticky** by storing it in app state
and automatically applying it to all compatible screens.

## Detailed Findings

### Current Namespace Handling Architecture

#### 1. Repository Layer Namespace Support

**File**: `internal/k8s/informer_repository.go:142-143`

Currently, informer factories are created **cluster-wide** (no namespace
filtering):

```go
factory := informers.NewSharedInformerFactory(clientset,
    InformerResyncPeriod)
dynamicFactory := dynamicinformer.NewDynamicSharedInformerFactory(
    dynamicClient, InformerResyncPeriod)
```

**However**, the infrastructure supports namespace scoping:

**Test pattern shows the way** (`internal/k8s/informer_repository_test.go:797-810`):

```go
// Namespace-scoped factory creation (used in tests):
factory := informers.NewSharedInformerFactoryWithOptions(
    clientset,
    InformerResyncPeriod,
    informers.WithNamespace(namespace),  // ← Namespace filtering!
)

dynamicFactory := dynamicinformer.NewFilteredDynamicSharedInformerFactory(
    dynamicClient,
    InformerResyncPeriod,
    namespace,  // ← Namespace parameter
    func(options *metav1.ListOptions) {},
)
```

**Key insight**: Repository creation must choose at construction time:
- **Cluster-wide informers**: See all namespaces (current behavior)
- **Namespace-scoped informers**: See only one namespace

This means **switching namespace requires recreating the repository** (same
pattern as context switching).

#### 2. Namespace-Scoped Query Methods

**File**: `internal/k8s/informer_queries.go:16-104`

Repository already implements namespace-scoped queries for contextual
navigation:

```go
// Get pods for a specific deployment in a namespace
func (r *InformerRepository) GetPodsForDeployment(namespace, name string)
    ([]Pod, error) {
    deployment, err := r.deploymentLister.Deployments(namespace).Get(name)
    // ... filter pods by owner UID
}

// Get pods in a namespace
func (r *InformerRepository) GetPodsForNamespace(namespace string)
    ([]Pod, error) {
    return r.podsByNamespace[namespace], nil  // Uses index map
}
```

These methods work for **contextual filtering** (showing related resources)
but not for **global namespace filtering** (changing the scope of all list
operations).

#### 3. Contextual Namespace Filtering (Current Implementation)

**File**: `internal/screens/navigation.go:131-157`

Navigation from namespace screen to pods creates FilterContext:

```go
func navigateToPodsForNamespace() NavigationFunc {
    return func(s *ConfigScreen) tea.Cmd {
        resource := s.GetSelectedResource()
        name, _ := resource["name"].(string)

        return func() tea.Msg {
            return types.ScreenSwitchMsg{
                ScreenID: "pods",
                FilterContext: &types.FilterContext{
                    Field: "namespace",
                    Value: name,
                    Metadata: map[string]string{
                        "kind": "Namespace",
                    },
                },
            }
        }
    }
}
```

**File**: `internal/screens/config.go:390-493`

ConfigScreen applies FilterContext when refreshing data:

```go
func (s *ConfigScreen) refreshWithFilterContext() tea.Cmd {
    if s.filterContext == nil {
        return s.Refresh()  // No filter - get all resources
    }

    namespace := s.filterContext.Metadata["namespace"]
    name := s.filterContext.Value

    switch s.filterContext.Field {
    case "namespace":
        // Call namespace-scoped repository method
        pods, err := s.repo.GetPodsForNamespace(name)
        // ...
    }
}
```

**Limitation**: FilterContext is **cleared** when navigating to a new
screen without explicitly passing it. To make it **sticky**, we need to:
1. Store namespace filter in app state (not just FilterContext)
2. Automatically apply namespace to all screen switches
3. Scope informers at repository level (for performance)

### Global State Management

#### 1. App Model State Structure

**File**: `internal/app/app.go:50-63`

```go
type Model struct {
    state             types.AppState      // Global app state
    registry          *types.ScreenRegistry
    currentScreen     types.Screen
    repoPool          *k8s.RepositoryPool // Context management
    navigationHistory []NavigationState    // Back navigation
    theme             *ui.Theme
    // ... UI components
}
```

**File**: `internal/types/types.go:69-75`

```go
type AppState struct {
    CurrentScreen string
    LastRefresh   time.Time
    RefreshTime   time.Duration
    Width         int
    Height        int
}
```

**Gap**: No `ActiveNamespace` or `NamespaceFilter` field in AppState.

**Proposed addition**:

```go
type AppState struct {
    CurrentScreen    string
    ActiveNamespace  string  // ← Add: sticky namespace ("" = all)
    // ... existing fields
}
```

#### 2. FilterContext Structure

**File**: `internal/types/types.go:77-107`

```go
type FilterContext struct {
    Field    string            // "owner", "node", "selector", "namespace"
    Value    string            // Resource name
    Metadata map[string]string // namespace, kind, etc.
}

func (f *FilterContext) Description() string {
    kind := strings.ToLower(f.Metadata["kind"])
    switch f.Field {
    case "namespace":
        return "filtered by " + kind + ": " + f.Value
    // ...
    }
}
```

**Current usage**: Contextual filtering (deployment → pods) carries
metadata about the source resource.

**Proposed usage for sticky namespace**:
- **Contextual filtering**: Keep existing FilterContext with source
  metadata
- **Sticky namespace**: Store in AppState, apply automatically to all
  screens

This separation allows:
- Navigate from deployment → pods (contextual: pods for deployment X)
- While sticky namespace is set (scope: only namespace Y)
- Result: Pods for deployment X **in namespace Y**

### Context Switching Implementation

#### 1. Context Switch Flow

**File**: `internal/app/app.go:394-507`

Context switching recreates all screens:

```go
case types.ContextSwitchCompleteMsg:
    // Update header/layout with new context
    m.header.SetContext(msg.NewContext)
    m.layout.SetContext(msg.NewContext)

    // Re-register screens with new repository
    m.registry = types.NewScreenRegistry()
    m.initializeScreens()  // ← Creates all screens fresh

    // Switch to same screen type in new context
    if screen, ok := m.registry.Get(m.currentScreen.ID()); ok {
        m.currentScreen = screen
    }
```

**File**: `internal/app/app.go:648-682`

```go
func (m *Model) initializeScreens() {
    repo := m.repoPool.GetActiveRepository()

    // Register all screens with active repository
    m.registry.Register(screens.NewConfigScreen(
        screens.GetPodsScreenConfig(), repo, m.theme))
    m.registry.Register(screens.NewConfigScreen(
        screens.GetDeploymentsScreenConfig(), repo, m.theme))
    // ... 15 more screens
}
```

**Implication**: When context switches, all screens are recreated. If
namespace filter was stored in screen state, it would be lost. Storing in
**app state** survives context switch.

**Decision point for sticky namespace across contexts**:
- **Option A**: Clear namespace on context switch (simpler, safer)
  - Rationale: Different clusters may have different namespaces
  - User must re-select namespace after switching contexts
- **Option B**: Preserve namespace across contexts (if it exists)
  - Rationale: Users often work with same namespace name (e.g., "prod")
  - Silently clear if namespace doesn't exist in new context
  - More complex error handling

**Recommendation**: Start with Option A (clear on context switch), add
Option B later if users request it.

#### 2. Repository Pool Structure

**File**: `internal/k8s/repository_pool.go:37-47`

```go
type RepositoryPool struct {
    mu         sync.RWMutex
    repos      map[string]*RepositoryEntry  // Context name → Repository
    active     string                        // Current context name
    maxSize    int                           // Pool size limit (LRU)
    kubeconfig string
    contexts   []*ContextInfo
}
```

Each context has its own repository instance. For namespace filtering, we
need:
1. **Per-context namespace state** (if preserving across contexts)
2. **Or global namespace cleared on switch** (simpler)

**Current delegation pattern** (`repository_pool.go:289-310`):

```go
func (p *RepositoryPool) GetResources(resourceType ResourceType)
    ([]any, error) {
    repo := p.GetActiveRepository()
    return repo.GetResources(resourceType)
}
```

For namespace filtering, pool could wrap active repository with namespace
scope:
- Option: Add `GetNamespacedRepository(namespace string) Repository`
- Returns a scoped view of the active repository
- Or: Add namespace parameter to all query methods

### Command System and Shortcuts

#### 1. Command Structure

**File**: `internal/commands/types.go:18-36`

```go
type Command struct {
    Name            string
    Description     string
    Category        CommandCategory  // Navigation, Action, Resource
    Execute         ExecuteFunc
    ResourceTypes   []k8s.ResourceType  // Filter by resource
    Shortcut        string              // Optional keyboard shortcut
    NeedsConfirmation bool
}

type CommandContext struct {
    ResourceType k8s.ResourceType
    Selected     map[string]interface{}
    Args         map[string]string
}

type ExecuteFunc func(CommandContext) tea.Cmd
```

#### 2. Navigation Command Registry

**File**: `internal/commands/navigation.go:14-91`

Table-driven navigation command registration:

```go
var navigationRegistry = map[string]string{
    "pods":        "pods",
    "deployments": "deployments",
    "namespaces":  "namespaces",
    // ... 8 more screens
}

func NavigationCommand(screenID string) ExecuteFunc {
    return func(ctx CommandContext) tea.Cmd {
        return func() tea.Msg {
            return types.ScreenSwitchMsg{ScreenID: screenID}
        }
    }
}

// Placeholder for namespace filtering
func NamespaceFilterCommand() ExecuteFunc {
    return func(ctx CommandContext) tea.Cmd {
        return messages.InfoCmd("Namespace filtering - Coming soon")
    }
}
```

**File**: `internal/commands/registry.go:306-331`

Commands registered with shortcuts:

```go
func NewRegistry(pool *k8s.RepositoryPool) *Registry {
    // ... navigation commands
    registry.Register(Command{
        Name:        "next-context",
        Description: "Switch to next context",
        Category:    CategoryResource,
        Execute:     NextContextCommand(pool),
        Shortcut:    "ctrl+n",
    })
    // ... more commands
}
```

#### 3. Global Shortcuts

**File**: `internal/app/app.go:173-238`

Global shortcuts handled before delegating to screens:

```go
case tea.KeyMsg:
    switch msg.String() {
    case "ctrl+c", "q":
        return m, tea.Quit
    case "ctrl+p":  // Previous context
        updatedBar, barCmd := m.commandBar.ExecuteCommand(
            "prev-context", commands.CategoryResource)
        return m, barCmd
    case "ctrl+n":  // Next context
        // ... similar pattern
    case "ctrl+y":  // YAML view
        // ... execute yaml command
    }
```

**Pattern for namespace shortcuts**:
- `ctrl+shift+n`: Set namespace filter (show palette of namespaces)
- `ctrl+shift+c`: Clear namespace filter (return to all namespaces)
- Or use command bar: `:namespace <name>` and `:clear-namespace`

### Namespace Screen Implementation

**File**: `internal/screens/screens.go:163-185`

```go
func GetNamespacesScreenConfig() ScreenConfig {
    return ScreenConfig{
        ID:                    "namespaces",
        Title:                 "Namespaces",
        ResourceType:          k8s.ResourceTypeNamespace,
        Columns:               []ColumnConfig{
            {Field: "name", Title: "Name", Width: 0, Priority: 1},
            {Field: "status", Title: "Status", Width: 15, Priority: 1},
            {Field: "age", Title: "Age", Width: 10, Priority: 2},
        },
        SearchFields:          []string{"name"},
        Operations:            []OperationConfig{
            {ID: "describe", Name: "Describe", Shortcut: "d"},
            {ID: "delete", Name: "Delete", Shortcut: "x"},
        },
        EnablePeriodicRefresh: true,
        RefreshInterval:       30 * time.Second,
        TrackSelection:        true,
        NavigationHandler:     navigateToPodsForNamespace(),
    }
}
```

**Current behavior**: Pressing Enter on namespace navigates to pods screen
with contextual filter (pods in that namespace).

**Desired behavior**: Add operation to **set sticky namespace filter**:
- `s` key: Set selected namespace as sticky filter
- Updates app state
- Recreates repository with namespace scope
- All screens now show only resources in that namespace
- Header shows "Namespace: <name>" indicator

## Architecture Insights

### 1. Repository Scoping Strategy

Two approaches for namespace filtering:

**Approach A: Repository-Level Scoping (Recommended)**
- Create namespace-scoped informers at repository construction
- Informers only watch resources in target namespace
- **Pros**:
  - Memory efficient: Don't cache resources from other namespaces
  - Network efficient: API server filters before sending
  - Simpler screen logic: GetPods() returns scoped list automatically
  - 70-90% memory reduction on large clusters
- **Cons**:
  - Switching namespace requires recreating repository (~5-15s)
  - Similar cost to context switching

**Approach B: Screen-Level Filtering**
- Keep cluster-wide informers
- Filter at screen level in ConfigScreen.refreshWithFilterContext()
- **Pros**:
  - Instant namespace switching (no repository recreation)
  - Can quickly toggle between namespaces
- **Cons**:
  - Memory inefficient: Cache all resources from all namespaces
  - Network inefficient: Fetch all resources even if viewing one namespace
  - Filter logic in every screen refresh

**Recommendation**: Use **Approach A** (repository-level scoping)
- Matches Kubernetes best practices (namespace-scoped connections)
- Consistent with context switching pattern (recreate repository)
- Memory efficiency critical for large clusters (1000s of pods)
- One-time cost (5-15s) when changing namespace is acceptable
- Users typically work in one namespace for extended periods

### 2. State Persistence Pattern

**Where to store sticky namespace**:

```go
// Option 1: In AppState (recommended)
type AppState struct {
    CurrentScreen   string
    ActiveNamespace string  // "" = all namespaces
    // ...
}

// Option 2: In RepositoryPool (per-context)
type RepositoryPool struct {
    activeNamespace string  // Per-context namespace
    // ...
}

// Option 3: In Repository (per-instance)
type InformerRepository struct {
    namespace string  // Immutable after creation
    // ...
}
```

**Recommended: AppState (Option 1)**
- Centralized state management
- Easy to save/restore across context switches
- Accessible to all components (header, screens, commands)
- Consistent with existing AppState pattern

### 3. FilterContext vs. Sticky Namespace

**Two types of filtering**:

1. **Contextual filtering** (existing FilterContext):
   - Temporary filter for related resources
   - Example: deployment "nginx" → pods owned by nginx
   - Cleared on back navigation (ESC key)
   - Shown in header: "filtered by deployment: nginx"

2. **Sticky namespace filtering** (proposed):
   - Persistent filter across all screens
   - Example: Set namespace "production" → all screens scoped to
     production
   - Persists until explicitly cleared
   - Shown in header: "Namespace: production" (always visible)

**Interaction**: Both can be active simultaneously:
- Sticky namespace: "production" (scope: see only production resources)
- Contextual filter: deployment "nginx" (further filter: pods for nginx)
- Result: Pods for nginx deployment **in production namespace**

**Implementation**:
- Sticky namespace: Stored in AppState, applied at repository creation
- Contextual filter: Stored in FilterContext, applied at screen refresh
- Repository creation checks AppState.ActiveNamespace
- Screen refresh checks both namespace scope and FilterContext

### 4. Header Display Strategy

**File**: `internal/components/header.go`

Header already displays context name. Add namespace indicator:

Current layout:
```
[k1] Context: minikube | Pods | Last refresh: 2s ago
```

With namespace filter:
```
[k1] Context: minikube | Namespace: production | Pods | Last refresh: 2s ago
```

With both namespace and contextual filter:
```
[k1] Context: minikube | Namespace: production | Pods (filtered by deployment: nginx) | ...
```

**Clear visual hierarchy**:
- Context: Cluster-level scope (outermost)
- Namespace: Namespace-level scope (middle)
- Contextual filter: Resource-level scope (innermost)

## Code References

### Core State Management
- `internal/app/app.go:50-63` - App Model structure
- `internal/types/types.go:69-75` - AppState structure
- `internal/types/types.go:77-107` - FilterContext structure

### Repository Layer
- `internal/k8s/repository_pool.go:37-67` - Repository pool structure
- `internal/k8s/informer_repository.go:90-200` -
  Repository creation with namespace scoping pattern
- `internal/k8s/informer_repository_test.go:797-810` -
  Namespace-scoped informer factories (test pattern)
- `internal/k8s/informer_queries.go:16-104` - Namespace-scoped query
  methods

### Screen Layer
- `internal/screens/config.go:66-87` - ConfigScreen structure with
  filterContext
- `internal/screens/config.go:390-493` - refreshWithFilterContext()
  applies filters
- `internal/screens/screens.go:163-185` - Namespace screen configuration
- `internal/screens/navigation.go:131-157` - navigateToPodsForNamespace()

### Context Switching
- `internal/app/app.go:394-507` - Context switch handling
- `internal/app/app.go:648-682` - initializeScreens() recreates all
  screens
- `internal/k8s/repository_pool.go:168-196` - SwitchContext() method

### Commands and Shortcuts
- `internal/commands/types.go:18-36` - Command structure
- `internal/commands/navigation.go:85-91` - Placeholder
  NamespaceFilterCommand()
- `internal/commands/registry.go:306-331` - Command registration with
  shortcuts
- `internal/app/app.go:173-238` - Global shortcut handling

## Implementation Proposal

### Phase 1: Core Namespace State Management

**1.1 Add namespace to AppState**
```go
// internal/types/types.go
type AppState struct {
    CurrentScreen   string
    ActiveNamespace string  // "" = all namespaces, "name" = specific
    // ... existing fields
}
```

**1.2 Add namespace to Repository creation**
```go
// internal/k8s/repository_pool.go
func (p *RepositoryPool) LoadContextWithNamespace(
    contextName string,
    namespace string,  // "" = cluster-wide, "name" = namespace-scoped
    progress chan<- ContextLoadProgress,
) error
```

**1.3 Update informer factory creation**
```go
// internal/k8s/informer_repository.go:142-143
// Replace cluster-wide factories with namespace-aware:
var factory informers.SharedInformerFactory
var dynamicFactory dynamicinformer.DynamicSharedInformerFactory

if namespace != "" {
    // Namespace-scoped
    factory = informers.NewSharedInformerFactoryWithOptions(
        clientset, InformerResyncPeriod,
        informers.WithNamespace(namespace))
    dynamicFactory = dynamicinformer.NewFilteredDynamicSharedInformerFactory(
        dynamicClient, InformerResyncPeriod, namespace, nil)
} else {
    // Cluster-wide (current behavior)
    factory = informers.NewSharedInformerFactory(
        clientset, InformerResyncPeriod)
    dynamicFactory = dynamicinformer.NewDynamicSharedInformerFactory(
        dynamicClient, InformerResyncPeriod)
}
```

### Phase 2: Commands and UI

**2.1 Implement SetNamespaceCommand**
```go
// internal/commands/namespace.go (new file)
func SetNamespaceCommand(pool *k8s.RepositoryPool) ExecuteFunc {
    return func(ctx CommandContext) tea.Cmd {
        namespaceName := ctx.Selected["name"].(string)

        return func() tea.Msg {
            return types.SetNamespaceMsg{
                Namespace: namespaceName,
            }
        }
    }
}

func ClearNamespaceCommand() ExecuteFunc {
    return func(ctx CommandContext) tea.Cmd {
        return func() tea.Msg {
            return types.ClearNamespaceMsg{}
        }
    }
}
```

**2.2 Add namespace messages**
```go
// internal/types/types.go
type SetNamespaceMsg struct {
    Namespace string  // Namespace to filter by
}

type ClearNamespaceMsg struct{}

type NamespaceSwitchCompleteMsg struct {
    Namespace string  // "" = all namespaces
}
```

**2.3 Add message handlers in app**
```go
// internal/app/app.go Update() method
case types.SetNamespaceMsg:
    // Store in state
    m.state.ActiveNamespace = msg.Namespace

    // Recreate repository with namespace scope
    // (similar to context switch)
    return m, m.switchNamespaceCmd(msg.Namespace)

case types.ClearNamespaceMsg:
    // Clear namespace filter
    m.state.ActiveNamespace = ""

    // Recreate repository cluster-wide
    return m, m.switchNamespaceCmd("")

case types.NamespaceSwitchCompleteMsg:
    // Update header with namespace indicator
    m.header.SetNamespace(msg.Namespace)

    // Refresh current screen with new scope
    return m, m.currentScreen.Init()
```

**2.4 Add namespace to header**
```go
// internal/components/header.go
type Header struct {
    context   string  // Existing
    namespace string  // NEW: Active namespace filter
    // ... other fields
}

func (h *Header) SetNamespace(namespace string) {
    h.namespace = namespace
}

func (h *Header) View() string {
    // Build header with namespace indicator
    contextPart := "Context: " + h.context
    if h.namespace != "" {
        contextPart += " | Namespace: " + h.namespace
    }
    // ... rest of header
}
```

**2.5 Add operations to namespace screen**
```go
// internal/screens/screens.go
func GetNamespacesScreenConfig() ScreenConfig {
    return ScreenConfig{
        // ... existing config
        Operations: []OperationConfig{
            {ID: "set", Name: "Set namespace", Shortcut: "s"},
            {ID: "describe", Name: "Describe", Shortcut: "d"},
            {ID: "delete", Name: "Delete", Shortcut: "x"},
        },
    }
}
```

**2.6 Register commands and shortcuts**
```go
// internal/commands/registry.go
registry.Register(Command{
    Name:        "set-namespace",
    Description: "Set active namespace filter",
    Category:    CategoryAction,
    Execute:     SetNamespaceCommand(pool),
    ResourceTypes: []k8s.ResourceType{k8s.ResourceTypeNamespace},
})

registry.Register(Command{
    Name:        "clear-namespace",
    Description: "Clear namespace filter (show all)",
    Category:    CategoryAction,
    Execute:     ClearNamespaceCommand(),
    Shortcut:    "ctrl+shift+c",
})
```

**2.7 Add global shortcut**
```go
// internal/app/app.go Update() - add to switch statement
case "ctrl+shift+c":
    // Clear namespace filter
    updatedBar, barCmd := m.commandBar.ExecuteCommand(
        "clear-namespace", commands.CategoryAction)
    m.commandBar = updatedBar
    return m, barCmd
```

### Phase 3: Context Switch Integration

**3.1 Clear namespace on context switch (recommended)**
```go
// internal/app/app.go
case types.ContextSwitchCompleteMsg:
    // Clear namespace filter when switching contexts
    m.state.ActiveNamespace = ""
    m.header.SetNamespace("")

    // ... existing context switch logic
```

**3.2 Alternative: Preserve namespace across contexts**
```go
case types.ContextSwitchCompleteMsg:
    // Try to preserve namespace if it exists in new context
    if m.state.ActiveNamespace != "" {
        // Check if namespace exists in new context
        namespaces, _ := m.repoPool.GetResources(
            k8s.ResourceTypeNamespace)

        exists := false
        for _, ns := range namespaces {
            if ns["name"] == m.state.ActiveNamespace {
                exists = true
                break
            }
        }

        if !exists {
            // Namespace doesn't exist in new context, clear it
            m.state.ActiveNamespace = ""
            m.header.SetNamespace("")
        }
    }

    // ... existing context switch logic
```

### Phase 4: Testing and Documentation

**4.1 Add tests**
- Test namespace-scoped repository creation
- Test namespace state persistence across navigation
- Test namespace clearing on context switch
- Test commands (set-namespace, clear-namespace)
- Test header display with namespace

**4.2 Update documentation**
- Add namespace filtering to README.md
- Document keyboard shortcuts (ctrl+shift+c)
- Document commands (:set-namespace, :clear-namespace)
- Update CLAUDE.md with implementation patterns

## Open Questions

1. **Namespace persistence across context switches**:
   - Clear on switch (simpler, recommended)?
   - Or preserve if exists (more complex)?
   - User preference? (add to config file later)

2. **Namespace switching UX**:
   - Progress indicator during repository recreation?
   - Show "Switching namespace..." status message?
   - How long is acceptable wait? (Currently ~5-15s for context switch)

3. **Namespace selection shortcuts**:
   - `ctrl+shift+n` to show namespace palette?
   - Or rely on `:namespaces` + `s` key?
   - Add quick-switch for recent namespaces?

4. **Global vs per-context namespace**:
   - Single global namespace for all contexts?
   - Or remember namespace per context?
   - Trade-off: simplicity vs flexibility

5. **Namespace indicator in header**:
   - Always show "Namespace: (all)" when no filter?
   - Or only show when filtered?
   - Color coding? (green = all, yellow = filtered)

6. **Command bar integration**:
   - Add `:ns <name>` shorthand for `:set-namespace`?
   - Autocomplete namespace names in command input?
   - Show current namespace in command bar hints?

## Related Research

- `thoughts/shared/plans/2025-10-09-kubernetes-context-management.md` -
  Context switching implementation (similar pattern)
- `thoughts/shared/plans/2025-10-10-issue-5-context-switching-improvements.md`
  - UX improvements for context switching
- `thoughts/shared/research/2025-10-09-k8s-context-management.md` -
  Context management research

## Recommendations

1. **Start with repository-level scoping** (Approach A)
   - Most memory efficient
   - Consistent with Kubernetes best practices
   - Similar pattern to context switching

2. **Clear namespace on context switch** (Phase 3.1)
   - Simpler implementation
   - Safer (avoid errors from missing namespaces)
   - Can add preservation later if users request

3. **Use AppState for namespace storage**
   - Centralized state management
   - Survives screen recreation
   - Easy to add persistence to config file later

4. **Provide both shortcut and command**
   - `s` key on namespace screen: Set namespace
   - `ctrl+shift+c` global: Clear namespace
   - `:clear-namespace` command: Clear namespace

5. **Show namespace in header always**
   - "Namespace: production" when filtered
   - "Namespace: (all)" when not filtered
   - Clear visual indicator of current scope

6. **Add progress indicator for namespace switch**
   - Similar to context switch loading
   - Show "Switching namespace..." status
   - Update header when complete

## Next Steps

To implement sticky namespace filtering:

1. **Create implementation plan** from this research
   - Break down into phases
   - Estimate effort for each phase
   - Identify dependencies and risks

2. **Prototype repository scoping** (Phase 1)
   - Add namespace parameter to repository creation
   - Test namespace-scoped informer factories
   - Measure memory impact on large clusters

3. **Implement core state management** (Phase 1)
   - Add ActiveNamespace to AppState
   - Update repository pool to support namespace scoping
   - Test namespace state flow

4. **Add commands and UI** (Phase 2)
   - Implement SetNamespaceCommand and ClearNamespaceCommand
   - Add namespace indicator to header
   - Add operations to namespace screen
   - Register shortcuts

5. **Integrate with context switching** (Phase 3)
   - Clear namespace on context switch
   - Test interaction between context and namespace
   - Handle edge cases (namespace doesn't exist in new context)

6. **Write tests and documentation** (Phase 4)
   - Add comprehensive test coverage
   - Update user documentation
   - Update developer documentation
