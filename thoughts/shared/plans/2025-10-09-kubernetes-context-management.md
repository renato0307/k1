# Kubernetes Context Management Implementation Plan

## Overview

Add comprehensive Kubernetes context management to k1 with runtime
context switching, multi-context preloading, and seamless navigation
between clusters. Users can specify multiple contexts via CLI flags for
instant switching, or dynamically switch to any context from kubeconfig
with non-blocking progress indicators.

## Current State Analysis

k1 currently supports **single-context operation**:
- Context specified at startup via `-context` flag (single value)
- Repository initialized once with fixed context
- No runtime context switching capability
- No context enumeration or listing
- Context stored but immutable after initialization

**Key files**:
- `cmd/k1/main.go:26-27` - CLI flags for kubeconfig and context
- `internal/k8s/informer_repository.go:89-274` - Repository
  initialization
- `internal/k8s/informer_repository.go:451-458` - Context getter methods
- `internal/k8s/repository.go:108-109` - Repository interface

**Current architecture limitations**:
- Single repository instance created at startup
- Informers cannot be reused across contexts (must stop/restart)
- No mechanism for maintaining multiple active contexts
- 5-15 second cache sync blocks application startup

## Desired End State

### User Experience

**CLI Usage**:
```bash
# Single context (current behavior, blocking until synced)
k1 -context production

# Multiple contexts (first blocks, others preload in background)
k1 -context prod -context staging -context dev
# UI appears after "prod" syncs, staging/dev load in background

# With pool size limit
k1 -context prod -context staging -max-contexts 5
```

**Context Selection Screen**:
- List all contexts from kubeconfig
- Show current context with "✓" indicator
- Display status: "Loaded", "Loading...", "Failed", "Not Loaded"
- Show cluster, user, and namespace info
- Press Enter to switch contexts with non-blocking progress
- Press ctrl+r to retry failed contexts

**Context Display in Header**:
- Left section: `Pods (production) • filtered by...`
- During switch: `Pods ⠋ production → staging (Syncing...)`
- After switch: `Pods (staging) • 247 items`

**Context Commands**:
- `/context <name>` - Switch to any context (with progress feedback)
- `:contexts` - Navigate to contexts screen
- `:next-context` - Cycle to next context (alphabetical)
- `:prev-context` - Cycle to previous context (alphabetical)

### Verification

After implementation, verify:

#### Automated Verification:
- [ ] All tests pass: `make test`
- [ ] Build succeeds: `make build`
- [ ] No linting errors: `golangci-lint run`

#### Manual Verification:
- [ ] Single context startup works: `k1 -context prod`
- [ ] Multi-context startup works: `k1 -context prod -context staging`
- [ ] Background loading shows progress in status bar
- [ ] Context switching is non-blocking (UI responsive during sync)
- [ ] Failed contexts show error and allow retry
- [ ] Header shows context name and loading spinner
- [ ] `:contexts` screen lists all kubeconfig contexts
- [ ] Context screen shows status (Loaded/Loading/Failed/Not Loaded)
- [ ] `/context` command switches with progress feedback
- [ ] Context cycling commands work (next/prev)
- [ ] Pool limit enforced (max 10 contexts)
- [ ] Switching to loaded context is instant (<100ms)
- [ ] Switching to non-loaded context shows progress
- [ ] All 16 resource screens work with switched context

## What We're NOT Doing

- **Cross-context queries**: No aggregation across multiple contexts
- **Context creation/editing**: Read-only kubeconfig, no modifications
- **Namespace-level context**: Context switches cluster, not namespace
- **Persistent context history**: No tracking of most-used contexts
- **Custom context aliases**: Use kubeconfig names only
- **Background preloading of all contexts**: Only CLI-specified contexts
- **Fixing existing single-letter shortcuts**: This plan uses ctrl+r for
  retry (correct pattern). Existing screens have single-letter shortcuts
  (l, d, x, s, r, etc.) that conflict with fuzzy search - this is a
  separate bug that should be fixed in a future refactoring

## Implementation Approach

Use **repository pool pattern** with LRU eviction to maintain multiple
active contexts. Each context gets its own InformerRepository instance
with isolated informers and indexes. First CLI-specified context loads
synchronously before UI appears, remaining contexts preload
asynchronously with progress feedback.

**Key design principles**:
1. **Non-blocking switches**: UI remains responsive during 5-15s sync
2. **Incremental feedback**: Progress messages show sync status
3. **Graceful failure**: Errors don't block app, contexts marked failed
4. **Pool warming**: CLI contexts preload for instant switching
5. **Complete isolation**: Each context has separate informers/indexes

---

## Phase 1: Repository Pool Infrastructure

### Overview
Create repository pool to manage multiple InformerRepository instances
with lifecycle management, LRU eviction, and status tracking.

### Changes Required

#### 1. Repository Pool Implementation

**File**: `internal/k8s/repository_pool.go` (NEW FILE)

**Purpose**: Manage multiple repository instances with LRU eviction

```go
package k8s

import (
    "container/list"
    "context"
    "fmt"
    "sync"
    "time"
)

// RepositoryStatus represents the state of a repository in the pool
type RepositoryStatus string

const (
    StatusNotLoaded RepositoryStatus = "Not Loaded"
    StatusLoading   RepositoryStatus = "Loading"
    StatusLoaded    RepositoryStatus = "Loaded"
    StatusFailed    RepositoryStatus = "Failed"
)

// RepositoryEntry wraps a repository with metadata
type RepositoryEntry struct {
    Repo       *InformerRepository
    Status     RepositoryStatus
    Error      error
    LoadedAt   time.Time
    ContextObj *ContextInfo // Parsed from kubeconfig
}

// ContextInfo holds context metadata from kubeconfig
type ContextInfo struct {
    Name      string
    Cluster   string
    User      string
    Namespace string
}

// RepositoryPool manages multiple Kubernetes contexts
type RepositoryPool struct {
    mu         sync.RWMutex
    repos      map[string]*RepositoryEntry
    active     string          // Current context name
    maxSize    int             // Pool size limit
    lru        *list.List      // LRU eviction order
    kubeconfig string
    contexts   []*ContextInfo  // All contexts from kubeconfig
}

// NewRepositoryPool creates a new repository pool
func NewRepositoryPool(kubeconfig string, maxSize int) (*RepositoryPool,
    error) {

    if maxSize <= 0 {
        maxSize = 10 // Default limit
    }

    // Parse kubeconfig to get all contexts
    contexts, err := parseKubeconfig(kubeconfig)
    if err != nil {
        return nil, fmt.Errorf("failed to parse kubeconfig: %w", err)
    }

    return &RepositoryPool{
        repos:      make(map[string]*RepositoryEntry),
        lru:        list.New(),
        maxSize:    maxSize,
        kubeconfig: kubeconfig,
        contexts:   contexts,
    }, nil
}

// LoadContext loads a context into the pool (blocking operation)
func (p *RepositoryPool) LoadContext(contextName string,
    progress chan<- ContextLoadProgress) error {

    p.mu.Lock()
    // Mark as loading
    if _, exists := p.repos[contextName]; !exists {
        p.repos[contextName] = &RepositoryEntry{
            Status: StatusLoading,
        }
    }
    p.mu.Unlock()

    // Report progress
    if progress != nil {
        progress <- ContextLoadProgress{
            Context: contextName,
            Message: "Connecting to API server...",
        }
    }

    // Create repository (5-15s operation)
    repo, err := NewInformerRepositoryWithProgress(p.kubeconfig,
        contextName, progress)

    p.mu.Lock()
    defer p.mu.Unlock()

    if err != nil {
        p.repos[contextName].Status = StatusFailed
        p.repos[contextName].Error = err
        return err
    }

    // Check pool size and evict if needed
    if len(p.repos) >= p.maxSize {
        p.evictLRU()
    }

    // Store repository
    p.repos[contextName] = &RepositoryEntry{
        Repo:     repo,
        Status:   StatusLoaded,
        LoadedAt: time.Now(),
    }
    p.lru.PushFront(contextName)

    return nil
}

// GetActiveRepository returns the currently active repository
func (p *RepositoryPool) GetActiveRepository() Repository {
    p.mu.RLock()
    defer p.mu.RUnlock()

    if entry, ok := p.repos[p.active]; ok && entry.Status ==
        StatusLoaded {
        return entry.Repo
    }

    return nil // Should never happen if pool is initialized correctly
}

// SwitchContext switches to a different context
func (p *RepositoryPool) SwitchContext(contextName string,
    progress chan<- ContextLoadProgress) error {

    p.mu.RLock()
    entry, exists := p.repos[contextName]
    p.mu.RUnlock()

    // Context already loaded - instant switch
    if exists && entry.Status == StatusLoaded {
        p.mu.Lock()
        p.active = contextName
        p.markUsed(contextName)
        p.mu.Unlock()
        return nil
    }

    // Load new context (blocking operation)
    if err := p.LoadContext(contextName, progress); err != nil {
        return err
    }

    // Switch to newly loaded context
    p.mu.Lock()
    p.active = contextName
    p.mu.Unlock()

    return nil
}

// GetAllContexts returns all contexts from kubeconfig
func (p *RepositoryPool) GetAllContexts() []ContextWithStatus {
    p.mu.RLock()
    defer p.mu.RUnlock()

    result := make([]ContextWithStatus, 0, len(p.contexts))
    for _, ctx := range p.contexts {
        status := StatusNotLoaded
        var err error

        if entry, ok := p.repos[ctx.Name]; ok {
            status = entry.Status
            err = entry.Error
        }

        result = append(result, ContextWithStatus{
            ContextInfo: ctx,
            Status:      status,
            Error:       err,
            IsCurrent:   ctx.Name == p.active,
        })
    }

    return result
}

// RetryFailedContext retries loading a failed context
func (p *RepositoryPool) RetryFailedContext(contextName string,
    progress chan<- ContextLoadProgress) error {

    p.mu.Lock()
    if entry, ok := p.repos[contextName]; ok {
        if entry.Status != StatusFailed {
            p.mu.Unlock()
            return fmt.Errorf("context %s is not in failed state",
                contextName)
        }
        delete(p.repos, contextName) // Remove failed entry
    }
    p.mu.Unlock()

    return p.LoadContext(contextName, progress)
}

// Close closes all repositories in the pool
func (p *RepositoryPool) Close() {
    p.mu.Lock()
    defer p.mu.Unlock()

    for _, entry := range p.repos {
        if entry.Repo != nil {
            entry.Repo.Close()
        }
    }
}

// Private helper methods

func (p *RepositoryPool) markUsed(contextName string) {
    // Move to front of LRU list
    for e := p.lru.Front(); e != nil; e = e.Next() {
        if e.Value.(string) == contextName {
            p.lru.MoveToFront(e)
            return
        }
    }
}

func (p *RepositoryPool) evictLRU() {
    if p.lru.Len() == 0 {
        return
    }

    // Get least recently used context
    back := p.lru.Back()
    if back == nil {
        return
    }

    contextName := back.Value.(string)

    // Don't evict active context
    if contextName == p.active {
        return
    }

    // Close and remove
    if entry, ok := p.repos[contextName]; ok {
        if entry.Repo != nil {
            entry.Repo.Close()
        }
        delete(p.repos, contextName)
    }
    p.lru.Remove(back)
}
```

**Key features**:
- Thread-safe repository management with RWMutex
- LRU eviction when pool reaches maxSize
- Status tracking (NotLoaded/Loading/Loaded/Failed)
- Progress channel for incremental feedback
- Active context never evicted

#### 2. Kubeconfig Parser

**File**: `internal/k8s/kubeconfig_parser.go` (NEW FILE)

**Purpose**: Parse kubeconfig to enumerate contexts

```go
package k8s

import (
    "fmt"
    "k8s.io/client-go/tools/clientcmd"
    "k8s.io/client-go/tools/clientcmd/api"
)

// parseKubeconfig loads kubeconfig and extracts all contexts
func parseKubeconfig(kubeconfigPath string) ([]*ContextInfo, error) {
    // Load kubeconfig
    config, err := clientcmd.LoadFromFile(kubeconfigPath)
    if err != nil {
        return nil, fmt.Errorf("failed to load kubeconfig: %w", err)
    }

    // Extract contexts
    contexts := make([]*ContextInfo, 0, len(config.Contexts))
    for name, ctx := range config.Contexts {
        contexts = append(contexts, &ContextInfo{
            Name:      name,
            Cluster:   ctx.Cluster,
            User:      ctx.AuthInfo,
            Namespace: ctx.Namespace,
        })
    }

    return contexts, nil
}

// getCurrentContext returns the current context from kubeconfig
func getCurrentContext(kubeconfigPath string) (string, error) {
    config, err := clientcmd.LoadFromFile(kubeconfigPath)
    if err != nil {
        return "", err
    }
    return config.CurrentContext, nil
}
```

#### 3. Progress Reporting Types

**File**: `internal/k8s/progress.go` (NEW FILE)

**Purpose**: Define progress reporting structures

```go
package k8s

// ContextLoadProgress reports loading progress for a context
type ContextLoadProgress struct {
    Context string
    Message string
    Phase   LoadPhase
}

// LoadPhase represents the current loading phase
type LoadPhase int

const (
    PhaseConnecting LoadPhase = iota
    PhaseSyncingCore
    PhaseSyncingDynamic
    PhaseComplete
)

// ContextWithStatus combines context info with runtime status
type ContextWithStatus struct {
    *ContextInfo
    Status    RepositoryStatus
    Error     error
    IsCurrent bool
}
```

#### 4. Repository Interface Extension

**File**: `internal/k8s/repository.go`

**Changes**: Add context switching support to interface

```go
// Add to Repository interface (after line 109)

// Context management
SwitchContext(contextName string, progress chan<-
    ContextLoadProgress) error
GetAllContexts() []ContextWithStatus
GetActiveContext() string
RetryFailedContext(contextName string, progress chan<-
    ContextLoadProgress) error
```

#### 5. Enhanced Repository Constructor

**File**: `internal/k8s/informer_repository.go`

**Changes**: Add progress reporting to constructor

```go
// NewInformerRepositoryWithProgress creates repository with progress
// reporting
func NewInformerRepositoryWithProgress(kubeconfig, contextName string,
    progress chan<- ContextLoadProgress) (*InformerRepository, error) {

    // Report connection phase
    if progress != nil {
        progress <- ContextLoadProgress{
            Context: contextName,
            Message: "Connecting to API server...",
            Phase:   PhaseConnecting,
        }
    }

    // ... existing kubeconfig loading code ...

    // Report core sync phase
    if progress != nil {
        progress <- ContextLoadProgress{
            Context: contextName,
            Message: "Syncing core resources...",
            Phase:   PhaseSyncingCore,
        }
    }

    // ... existing typed informer sync code ...

    // Report dynamic sync phase
    if progress != nil {
        progress <- ContextLoadProgress{
            Context: contextName,
            Message: "Syncing dynamic resources...",
            Phase:   PhaseSyncingDynamic,
        }
    }

    // ... existing dynamic informer sync code ...

    // Report completion
    if progress != nil {
        progress <- ContextLoadProgress{
            Context: contextName,
            Message: "Context loaded successfully",
            Phase:   PhaseComplete,
        }
    }

    return repo, nil
}

// Keep existing NewInformerRepository for backward compatibility
func NewInformerRepository(kubeconfig, contextName string)
    (*InformerRepository, error) {
    return NewInformerRepositoryWithProgress(kubeconfig, contextName,
        nil)
}
```

### Success Criteria

#### Automated Verification:
- [x] All tests pass: `make test`
- [x] Build succeeds: `make build`
- [x] Repository pool creates successfully with valid kubeconfig
- [x] Kubeconfig parser extracts all contexts correctly
- [x] Repository pool enforces maxSize limit
- [x] LRU eviction works correctly

#### Manual Verification:
- [x] Deferred to Phase 2+ (infrastructure only, no CLI integration yet)

---

## Phase 2: CLI Multi-Context Support

### Overview
Modify CLI flag handling to accept multiple context values and
orchestrate pool loading (first blocking, others background).

### Changes Required

#### 1. CLI Flag Handling

**File**: `cmd/k1/main.go`

**Changes**: Support multiple `-context` flags

```go
// Replace lines 26-27 with custom flag type
type contextList []string

func (c *contextList) String() string {
    return fmt.Sprintf("%v", *c)
}

func (c *contextList) Set(value string) error {
    *c = append(*c, value)
    return nil
}

var (
    kubeconfigFlag = flag.String("kubeconfig", "",
        "Path to kubeconfig file")
    contextFlags contextList
    maxContexts = flag.Int("max-contexts", 10,
        "Maximum number of contexts to keep loaded (1-20)")
)

func init() {
    flag.Var(&contextFlags, "context",
        "Kubernetes context to use (can be specified multiple times)")
}

func main() {
    flag.Parse()

    // Validate max-contexts range
    if *maxContexts < 1 || *maxContexts > 20 {
        fmt.Println("Error: max-contexts must be between 1 and 20")
        os.Exit(1)
    }

    // Determine kubeconfig path
    kubeconfig := *kubeconfigFlag
    if kubeconfig == "" {
        if home := os.Getenv("HOME"); home != "" {
            kubeconfig = filepath.Join(home, ".kube", "config")
        }
    }

    // Determine context list
    contexts := []string(contextFlags)
    if len(contexts) == 0 {
        // No contexts specified - use current from kubeconfig
        currentCtx, err := k8s.GetCurrentContext(kubeconfig)
        if err != nil {
            fmt.Printf("Error: %v\n", err)
            os.Exit(1)
        }
        contexts = []string{currentCtx}
    }

    fmt.Printf("Connecting to Kubernetes cluster (%s)...\n",
        contexts[0])
    fmt.Println("Syncing cache...")

    // Create repository pool
    pool, err := k8s.NewRepositoryPool(kubeconfig, *maxContexts)
    if err != nil {
        fmt.Printf("Error initializing pool: %v\n", err)
        os.Exit(1)
    }
    defer pool.Close()

    // Load first context (BLOCKING - must complete before UI)
    progressCh := make(chan k8s.ContextLoadProgress, 10)
    errCh := make(chan error, 1)

    go func() {
        err := pool.LoadContext(contexts[0], progressCh)
        errCh <- err
        close(progressCh)
    }()

    // Show progress for first context
    for progress := range progressCh {
        fmt.Printf("  %s\n", progress.Message)
    }

    if err := <-errCh; err != nil {
        fmt.Printf("Error connecting to context %s: %v\n",
            contexts[0], err)
        os.Exit(1)
    }

    // Set first context as active
    pool.SetActive(contexts[0])

    fmt.Println("Cache synced! Starting UI...")

    // Initialize Bubble Tea app
    theme := ui.GetTheme(*themeFlag)
    p := tea.NewProgram(
        app.NewModel(pool, theme),
        tea.WithAltScreen(),
        tea.WithMouseCellMotion(),
    )

    // Load remaining contexts in background (non-blocking)
    if len(contexts) > 1 {
        go loadBackgroundContexts(pool, contexts[1:], p)
    }

    // Run UI
    if _, err := p.Run(); err != nil {
        fmt.Printf("Error running app: %v\n", err)
        os.Exit(1)
    }
}

// loadBackgroundContexts loads contexts after UI starts
func loadBackgroundContexts(pool *k8s.RepositoryPool,
    contexts []string, program *tea.Program) {

    for _, ctx := range contexts {
        progressCh := make(chan k8s.ContextLoadProgress, 10)

        go func(contextName string) {
            // Send progress messages to UI
            for progress := range progressCh {
                program.Send(types.ContextLoadProgressMsg{
                    Context: progress.Context,
                    Message: progress.Message,
                    Phase:   progress.Phase,
                })
            }
        }(ctx)

        err := pool.LoadContext(ctx, progressCh)
        close(progressCh)

        if err != nil {
            program.Send(types.ContextLoadFailedMsg{
                Context: ctx,
                Error:   err,
            })
        } else {
            program.Send(types.ContextLoadCompleteMsg{
                Context: ctx,
            })
        }
    }
}
```

**Key changes**:
- Custom `contextList` type for repeatable `-context` flags
- First context loads synchronously with console progress
- Remaining contexts load asynchronously after UI starts
- Progress messages sent to Bubble Tea program via `Send()`

#### 2. Repository Pool Active Context Setter

**File**: `internal/k8s/repository_pool.go`

**Add method**:

```go
// SetActive sets the active context without loading
func (p *RepositoryPool) SetActive(contextName string) error {
    p.mu.Lock()
    defer p.mu.Unlock()

    if _, ok := p.repos[contextName]; !ok {
        return fmt.Errorf("context %s not loaded", contextName)
    }

    p.active = contextName
    p.markUsed(contextName)
    return nil
}
```

#### 3. Helper Function Export

**File**: `internal/k8s/kubeconfig_parser.go`

**Export getCurrentContext**:

```go
// GetCurrentContext returns the current context from kubeconfig
func GetCurrentContext(kubeconfigPath string) (string, error) {
    return getCurrentContext(kubeconfigPath)
}
```

### Success Criteria

#### Automated Verification:
- [x] Build succeeds: `make build`
- [x] Multiple `-context` flags accepted: `k1 -context a -context b`
- [x] Single context works: `k1 -context prod`
- [x] No context defaults to current: `k1`

#### Manual Verification:
- [ ] First context loads with progress printed to console
- [ ] UI appears after first context syncs
- [ ] Remaining contexts load in background
- [ ] Background loading shows status messages in status bar (IMPLEMENTED - basic handlers added)
- [ ] Pool limit enforced (max 10 contexts by default)
- [ ] `-max-contexts` flag works correctly

**Note**: Basic message handlers for `ContextLoadProgressMsg`, `ContextLoadCompleteMsg`,
and `ContextLoadFailedMsg` were added from Phase 3 to enable status bar feedback during
background loading.

---

## Phase 3: Message Types and App Integration

### Overview
Add Bubble Tea message types for context operations and integrate
repository pool into app model.

### Changes Required

#### 1. Message Type Definitions

**File**: `internal/types/types.go`

**Add after line 119**:

```go
// Context management messages

// ContextSwitchMsg initiates a context switch
type ContextSwitchMsg struct {
    ContextName string
}

// ContextLoadProgressMsg reports loading progress
type ContextLoadProgressMsg struct {
    Context string
    Message string
    Phase   k8s.LoadPhase
}

// ContextLoadCompleteMsg signals successful context load
type ContextLoadCompleteMsg struct {
    Context string
}

// ContextLoadFailedMsg signals failed context load
type ContextLoadFailedMsg struct {
    Context string
    Error   error
}

// ContextSwitchCompleteMsg signals successful context switch
type ContextSwitchCompleteMsg struct {
    OldContext string
    NewContext string
}

// ContextRetryMsg requests retry of failed context
type ContextRetryMsg struct {
    ContextName string
}
```

#### 2. App Model Integration

**File**: `internal/app/app.go`

**Changes**: Replace single repository with pool

```go
// Replace line 44 with:
repoPool *k8s.RepositoryPool

// Update NewModel signature (line 48):
func NewModel(pool *k8s.RepositoryPool, theme *ui.Theme) Model {
    registry := types.NewScreenRegistry()

    // Get active repository from pool
    repo := pool.GetActiveRepository()

    // ... existing screen registration using repo ...

    header := components.NewHeader("k1", theme)
    header.SetScreenTitle(initialScreen.Title())
    header.SetWidth(80)
    header.SetContext(pool.GetActiveContext()) // NEW

    // ... rest of initialization ...

    return Model{
        // ... existing fields ...
        repoPool:      pool,
        // ... rest of fields ...
    }
}

// Add helper method to get active repository
func (m Model) getActiveRepo() k8s.Repository {
    return m.repoPool.GetActiveRepository()
}
```

#### 3. Context Switch Handling

**File**: `internal/app/app.go`

**Add to Update() method** (after line 285):

```go
case types.ContextSwitchMsg:
    // Initiate context switch asynchronously
    return m, m.switchContextCmd(msg.ContextName)

case types.ContextLoadProgressMsg:
    // Update header with loading progress
    m.header.SetContextLoading(msg.Context, msg.Message)
    return m, nil

case types.ContextLoadCompleteMsg:
    // Context loaded successfully (background load)
    m.statusBar.SetMessage(
        fmt.Sprintf("Context %s loaded", msg.Context),
        types.MessageTypeSuccess,
    )
    return m, tea.Tick(StatusBarDisplayDuration, func(t time.Time)
        tea.Msg {
        return types.ClearStatusMsg{}
    })

case types.ContextLoadFailedMsg:
    // Context load failed
    m.statusBar.SetMessage(
        fmt.Sprintf("Failed to load context %s: %v", msg.Context,
            msg.Error),
        types.MessageTypeError,
    )
    return m, tea.Tick(StatusBarDisplayDuration, func(t time.Time)
        tea.Msg {
        return types.ClearStatusMsg{}
    })

case types.ContextSwitchCompleteMsg:
    // Context switch completed - refresh current screen
    m.header.SetContext(msg.NewContext)
    m.statusBar.SetMessage(
        fmt.Sprintf("Switched to context: %s", msg.NewContext),
        types.MessageTypeSuccess,
    )

    // Re-register screens with new repository
    m.registry = types.NewScreenRegistry()
    m.initializeScreens() // NEW helper method

    // Switch to same screen type in new context
    if screen, ok := m.registry.Get(m.currentScreen.ID()); ok {
        m.currentScreen = screen
        m.header.SetScreenTitle(screen.Title())
    }

    return m, tea.Batch(
        m.currentScreen.Init(),
        tea.Tick(StatusBarDisplayDuration, func(t time.Time)
            tea.Msg {
            return types.ClearStatusMsg{}
        }),
    )

case types.ContextRetryMsg:
    // Retry failed context
    return m, m.retryContextCmd(msg.ContextName)
```

#### 4. Context Switch Commands

**File**: `internal/app/app.go`

**Add helper methods**:

```go
// switchContextCmd returns command to switch contexts asynchronously
func (m Model) switchContextCmd(contextName string) tea.Cmd {
    return func() tea.Msg {
        // Create progress channel
        progressCh := make(chan k8s.ContextLoadProgress, 10)
        errCh := make(chan error, 1)

        // Switch context in goroutine
        go func() {
            err := m.repoPool.SwitchContext(contextName, progressCh)
            errCh <- err
            close(progressCh)
        }()

        // Forward progress messages
        go func() {
            for progress := range progressCh {
                // Send progress to UI
                m.program.Send(types.ContextLoadProgressMsg{
                    Context: progress.Context,
                    Message: progress.Message,
                    Phase:   progress.Phase,
                })
            }
        }()

        // Wait for completion
        if err := <-errCh; err != nil {
            return types.ContextLoadFailedMsg{
                Context: contextName,
                Error:   err,
            }
        }

        return types.ContextSwitchCompleteMsg{
            OldContext: m.repoPool.GetActiveContext(),
            NewContext: contextName,
        }
    }
}

// retryContextCmd returns command to retry failed context
func (m Model) retryContextCmd(contextName string) tea.Cmd {
    return func() tea.Msg {
        progressCh := make(chan k8s.ContextLoadProgress, 10)
        errCh := make(chan error, 1)

        go func() {
            err := m.repoPool.RetryFailedContext(contextName,
                progressCh)
            errCh <- err
            close(progressCh)
        }()

        go func() {
            for progress := range progressCh {
                m.program.Send(types.ContextLoadProgressMsg{
                    Context: progress.Context,
                    Message: progress.Message,
                    Phase:   progress.Phase,
                })
            }
        }()

        if err := <-errCh; err != nil {
            return types.ContextLoadFailedMsg{
                Context: contextName,
                Error:   err,
            }
        }

        return types.ContextLoadCompleteMsg{
            Context: contextName,
        }
    }
}

// initializeScreens registers all screens with active repository
func (m *Model) initializeScreens() {
    repo := m.getActiveRepo()

    // Register all 16 screens (existing registration code)
    m.registry.Register(screens.NewConfigScreen(
        screens.GetPodsScreenConfig(), repo, m.theme))
    m.registry.Register(screens.NewConfigScreen(
        screens.GetDeploymentsScreenConfig(), repo, m.theme))
    // ... all other screens
}
```

### Success Criteria

#### Automated Verification:
- [x] Build succeeds: `make build`
- [x] All message types compile correctly

#### Manual Verification:
- [x] App initializes with repository pool
- [ ] Context switches trigger async operations
- [ ] Progress messages update header during switch (DEFERRED to Phase 4)
- [ ] Success/failure messages appear in status bar
- [ ] Screens re-register with new repository after switch
- [ ] Current screen persists across context switches

**Implementation Note**: Context switch handlers and screen re-registration
implemented. Manual testing deferred until Phase 5 (contexts screen) and
Phase 6 (context commands) are implemented.

---

## Phase 4: Header Loading Indicator

### Overview
Add context name display and non-blocking loading spinner to header
component.

### Changes Required

#### 1. Header State Extension

**File**: `internal/components/header.go`

**Add fields** (after line 22):

```go
context         string  // Current Kubernetes context
contextLoading  bool    // Whether context is loading
loadingMessage  string  // Loading progress message
loadingSpinner  int     // Spinner frame index (0-7)
```

#### 2. Header Setter Methods

**File**: `internal/components/header.go`

**Add methods** (after line 53):

```go
// SetContext sets the current context name
func (h *Header) SetContext(context string) {
    h.context = context
    h.contextLoading = false
}

// SetContextLoading sets context loading state with message
func (h *Header) SetContextLoading(context, message string) {
    h.context = context
    h.contextLoading = true
    h.loadingMessage = message
}

// TickSpinner advances the loading spinner frame
func (h *Header) TickSpinner() {
    h.loadingSpinner = (h.loadingSpinner + 1) % 8
}
```

#### 3. Header View Update

**File**: `internal/components/header.go`

**Modify View() method** (lines 66-88):

```go
// Build left parts
leftParts := []string{}

if h.screenTitle != "" {
    leftParts = append(leftParts, h.screenTitle)
}

// Add context display
if h.context != "" {
    contextStyle := lipgloss.NewStyle().Foreground(h.theme.Muted)

    if h.contextLoading {
        // Show spinner and loading message
        spinnerFrames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴",
            "⠦", "⠧"}
        spinner := spinnerFrames[h.loadingSpinner]
        contextText := fmt.Sprintf("%s %s (%s)", spinner, h.context,
            h.loadingMessage)
        leftParts = append(leftParts,
            contextStyle.Render(contextText))
    } else {
        // Show normal context
        leftParts = append(leftParts,
            contextStyle.Render("("+h.context+")"))
    }
}

if h.filterText != "" {
    leftParts = append(leftParts, h.filterText)
}

// ... rest of View() unchanged
```

#### 4. Spinner Ticker

**File**: `internal/app/app.go`

**Add spinner tick message** (in Update()):

```go
case types.ContextLoadProgressMsg:
    // Update header with loading progress
    m.header.SetContextLoading(msg.Context, msg.Message)

    // Start spinner ticker if not already running
    if !m.spinnerActive {
        m.spinnerActive = true
        return m, m.tickSpinner()
    }
    return m, nil

case types.SpinnerTickMsg:
    if m.header.contextLoading {
        m.header.TickSpinner()
        return m, m.tickSpinner()
    }
    m.spinnerActive = false
    return m, nil

// Add to types.go:
type SpinnerTickMsg struct{}

// Add to app.go:
spinnerActive bool  // In Model struct

func (m Model) tickSpinner() tea.Cmd {
    return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
        return types.SpinnerTickMsg{}
    })
}
```

### Success Criteria

#### Automated Verification:
- [ ] Build succeeds: `make build`
- [ ] Header renders without panics

#### Manual Verification:
- [ ] Context name appears in header: `Pods (production)`
- [ ] Loading spinner animates during context switch
- [ ] Loading message updates: "Syncing pods...", "Syncing
  deployments..."
- [ ] Spinner stops after switch completes
- [ ] Header layout remains correct during loading

---

## Phase 5: Contexts Screen

### Overview
Create contexts selection screen showing all kubeconfig contexts with
status indicators.

### Changes Required

#### 1. Context Resource Type

**File**: `internal/k8s/resource_types.go`

**Add new resource type**:

```go
const (
    // ... existing types ...
    ResourceTypeContext ResourceType = "contexts"
)
```

#### 2. Context Data Structure

**File**: `internal/k8s/context.go` (NEW FILE)

**Purpose**: Define context display structure

```go
package k8s

import "time"

// Context represents a Kubernetes context for display
type Context struct {
    Name      string
    Cluster   string
    User      string
    Namespace string
    Status    string // "Loaded", "Loading", "Failed", "Not Loaded"
    Current   string // "✓" if current, "" otherwise
    Error     string // Error message if failed
    LoadedAt  time.Time
}
```

#### 3. Repository Interface Extension

**File**: `internal/k8s/repository.go`

**Add method to interface** (after line 115):

```go
// GetContexts returns all available contexts from kubeconfig
GetContexts() ([]Context, error)
```

#### 4. Repository Pool Implementation

**File**: `internal/k8s/repository_pool.go`

**Add method**:

```go
// GetContexts returns all contexts for display
func (p *RepositoryPool) GetContexts() ([]Context, error) {
    p.mu.RLock()
    defer p.mu.RUnlock()

    allContexts := p.GetAllContexts()
    result := make([]Context, 0, len(allContexts))

    for _, ctx := range allContexts {
        current := ""
        if ctx.IsCurrent {
            current = "✓"
        }

        status := string(ctx.Status)
        errorMsg := ""
        if ctx.Error != nil {
            errorMsg = ctx.Error.Error()
        }

        var loadedAt time.Time
        if entry, ok := p.repos[ctx.Name]; ok {
            loadedAt = entry.LoadedAt
        }

        result = append(result, Context{
            Name:      ctx.Name,
            Cluster:   ctx.Cluster,
            User:      ctx.User,
            Namespace: ctx.Namespace,
            Status:    status,
            Current:   current,
            Error:     errorMsg,
            LoadedAt:  loadedAt,
        })
    }

    return result, nil
}
```

#### 5. Contexts Screen Configuration

**File**: `internal/screens/screens.go`

**Add screen config** (after line 460):

```go
// GetContextsScreenConfig returns config for Contexts screen
func GetContextsScreenConfig() ScreenConfig {
    return ScreenConfig{
        ID:           "contexts",
        Title:        "Contexts",
        ResourceType: k8s.ResourceTypeContext,
        Columns: []ColumnConfig{
            {Field: "Name", Title: "Name", Width: 0}, // Dynamic width
            {Field: "Cluster", Title: "Cluster", Width: 40},
            {Field: "User", Title: "User", Width: 30},
            {Field: "Status", Title: "Status", Width: 15},
            {Field: "Current", Title: "Current", Width: 10},
        },
        SearchFields: []string{"Name", "Cluster", "User"},
        Operations: []OperationConfig{
            {ID: "retry", Name: "Retry",
             Description: "Retry failed context", Shortcut: "ctrl+r"},
        },
        NavigationHandler:     navigateToContextSwitch(),
        TrackSelection:        true,
        EnablePeriodicRefresh: false, // No auto-refresh for contexts
    }
}
```

#### 6. Context Navigation Handler

**File**: `internal/screens/navigation.go`

**Add handler** (after line 343):

```go
// navigateToContextSwitch creates handler for context switching
func navigateToContextSwitch() NavigationFunc {
    return func(s *ConfigScreen) tea.Cmd {
        resource := s.GetSelectedResource()
        if resource == nil {
            return nil
        }

        contextName, _ := resource["name"].(string)
        if contextName == "" {
            return nil
        }

        return func() tea.Msg {
            return types.ContextSwitchMsg{
                ContextName: contextName,
            }
        }
    }
}
```

#### 7. Screen Registration

**File**: `internal/app/app.go`

**Add to screen registration** (in initializeScreens()):

```go
// Add contexts screen (special - uses pool directly)
m.registry.Register(screens.NewConfigScreen(
    screens.GetContextsScreenConfig(), m.repoPool, m.theme))
```

### Success Criteria

#### Automated Verification:
- [x] Build succeeds: `make build`
- [x] Contexts screen compiles correctly
- [x] All tests pass

#### Manual Verification:
- [ ] `:contexts` command navigates to contexts screen
- [ ] All kubeconfig contexts appear in list
- [ ] Current context shows "✓" indicator
- [ ] Status column shows correct state (Loaded/Loading/Failed/Not
  Loaded)
- [ ] Loaded contexts show in bold or highlighted
- [ ] Press Enter on context triggers switch
- [ ] Failed contexts show error in tooltip/detail view
- [ ] "r" key retries failed context (DEFERRED - needs `:contexts` command from Phase 6)

---

## Phase 6: Context Commands

### Overview
Add `/context`, `:next-context`, and `:prev-context` commands for
context management.

### Changes Required

#### 1. Context Switch Command

**File**: `internal/commands/context.go` (NEW FILE)

**Purpose**: Implement `/context <name>` command

```go
package commands

import (
    "github.com/renato0307/k1/internal/k8s"
    "github.com/renato0307/k1/internal/messages"
    "github.com/renato0307/k1/internal/types"
    "github.com/charmbracelet/bubbletea"
)

type ContextArgs struct {
    ContextName string `inline:"0"`
}

func ContextCommand(pool *k8s.RepositoryPool) ExecuteFunc {
    return func(ctx CommandContext) tea.Cmd {
        var args ContextArgs
        if err := ctx.ParseArgs(&args); err != nil {
            return messages.ErrorCmd("Invalid args: %v", err)
        }

        return func() tea.Msg {
            return types.ContextSwitchMsg{
                ContextName: args.ContextName,
            }
        }
    }
}
```

#### 2. Context Navigation Commands

**File**: `internal/commands/navigation.go`

**Add commands** (after line 96):

```go
// NextContextCommand switches to next context alphabetically
func NextContextCommand(pool *k8s.RepositoryPool) ExecuteFunc {
    return func(ctx CommandContext) tea.Cmd {
        return func() tea.Msg {
            contexts, err := pool.GetContexts()
            if err != nil {
                return messages.ErrorCmd("Failed to list contexts:
                    %v", err)()
            }

            // Sort contexts alphabetically
            sort.Slice(contexts, func(i, j int) bool {
                return contexts[i].Name < contexts[j].Name
            })

            // Find current context
            current := pool.GetActiveContext()
            currentIdx := -1
            for i, c := range contexts {
                if c.Name == current {
                    currentIdx = i
                    break
                }
            }

            // Get next context (wrap around)
            nextIdx := (currentIdx + 1) % len(contexts)
            nextContext := contexts[nextIdx].Name

            return types.ContextSwitchMsg{
                ContextName: nextContext,
            }
        }
    }
}

// PrevContextCommand switches to previous context alphabetically
func PrevContextCommand(pool *k8s.RepositoryPool) ExecuteFunc {
    return func(ctx CommandContext) tea.Cmd {
        return func() tea.Msg {
            contexts, err := pool.GetContexts()
            if err != nil {
                return messages.ErrorCmd("Failed to list contexts:
                    %v", err)()
            }

            // Sort contexts alphabetically
            sort.Slice(contexts, func(i, j int) bool {
                return contexts[i].Name < contexts[j].Name
            })

            // Find current context
            current := pool.GetActiveContext()
            currentIdx := -1
            for i, c := range contexts {
                if c.Name == current {
                    currentIdx = i
                    break
                }
            }

            // Get previous context (wrap around)
            prevIdx := (currentIdx - 1 + len(contexts)) %
                len(contexts)
            prevContext := contexts[prevIdx].Name

            return types.ContextSwitchMsg{
                ContextName: prevContext,
            }
        }
    }
}
```

#### 3. Command Registry

**File**: `internal/commands/registry.go`

**Add commands to registry** (after line 293):

```go
// Context management commands
{
    Name:        "context",
    Description: "Switch Kubernetes context",
    Category:    CategoryAction,
    ArgsType:    &ContextArgs{},
    ArgPattern:  " <context-name>",
    Execute:     ContextCommand(pool),
},
{
    Name:        "next-context",
    Description: "Switch to next context",
    Category:    CategoryResource,
    Execute:     NextContextCommand(pool),
},
{
    Name:        "prev-context",
    Description: "Switch to previous context",
    Category:    CategoryResource,
    Execute:     PrevContextCommand(pool),
},
```

**Update NewRegistry signature**:

```go
func NewRegistry(pool *k8s.RepositoryPool) *Registry {
    repo := pool.GetActiveRepository()

    // ... existing command registrations using repo ...

    // New context commands using pool
    // ... add context commands here ...
}
```

### Success Criteria

#### Automated Verification:
- [ ] Build succeeds: `make build`
- [ ] Command registration compiles

#### Manual Verification:
- [ ] `/context production` switches to production context
- [ ] `:next-context` cycles to next context alphabetically
- [ ] `:prev-context` cycles to previous context
- [ ] Context cycling wraps around (last → first, first → last)
- [ ] Invalid context name shows error
- [ ] Commands work from any screen

---

## Phase 7: Testing and Documentation

### Overview
Add comprehensive tests and update documentation.

### Changes Required

#### 1. Repository Pool Tests

**File**: `internal/k8s/repository_pool_test.go` (NEW FILE)

**Tests**:
- Pool creation and initialization
- Context loading (sync and async)
- LRU eviction behavior
- Pool size limits
- Status tracking
- Failed context handling
- Retry mechanism
- Thread safety

#### 2. Kubeconfig Parser Tests

**File**: `internal/k8s/kubeconfig_parser_test.go` (NEW FILE)

**Tests**:
- Parse valid kubeconfig
- Extract all contexts
- Handle missing contexts
- Handle invalid kubeconfig

#### 3. Context Screen Tests

**File**: `internal/screens/contexts_test.go` (NEW FILE)

**Tests**:
- Screen renders correctly
- Context list displays all contexts
- Current context indicator works
- Status display correct
- Navigation handler triggers switch

#### 4. Command Tests

**File**: `internal/commands/context_test.go` (NEW FILE)

**Tests**:
- `/context` command with valid name
- `/context` command with invalid name
- `:next-context` cycling
- `:prev-context` cycling
- Wrap-around behavior

#### 5. Documentation Updates

**File**: `CLAUDE.md`

**Add section**:
```markdown
### Kubernetes Context Management

k1 supports runtime context switching with multi-context preloading:

**CLI Usage**:
- Single context: `k1 -context production`
- Multiple contexts: `k1 -context prod -context staging -context dev`
- Pool size limit: `k1 -max-contexts 5`

**Context Commands**:
- `/context <name>` - Switch to any context
- `:contexts` - Open contexts screen
- `:next-context` - Cycle to next context
- `:prev-context` - Cycle to previous context

**Context Screen**:
- Press Enter to switch contexts
- Press ctrl+r to retry failed contexts
- Status indicators: Loaded, Loading, Failed, Not Loaded

**Architecture**:
- Repository pool pattern with LRU eviction
- Non-blocking context switches
- Progress feedback in header
- Maximum 10 contexts by default (configurable 1-20)
```

**File**: `README.md`

**Update features section and usage examples**

### Success Criteria

#### Automated Verification:
- [ ] All tests pass: `make test`
- [ ] Test coverage maintained: `make test-coverage`
- [ ] Build succeeds: `make build`
- [ ] No linting errors: `golangci-lint run`

#### Manual Verification:
- [ ] All test scenarios covered
- [ ] Documentation is clear and accurate
- [ ] Examples work as documented

---

## Testing Strategy

### Unit Tests

**Repository Pool** (`internal/k8s/repository_pool_test.go`):
- Pool initialization with various sizes
- Context loading (sync and async)
- LRU eviction with different pool sizes
- Thread-safety with concurrent operations
- Status transitions (NotLoaded → Loading → Loaded/Failed)
- Retry failed contexts

**Kubeconfig Parser** (`internal/k8s/kubeconfig_parser_test.go`):
- Parse multi-context kubeconfig
- Extract context metadata correctly
- Handle missing/invalid kubeconfig files
- Get current context

**Context Screen** (`internal/screens/contexts_test.go`):
- Render with all status types
- Filter contexts by search
- Navigation handler triggers correct message
- Operations available based on context status

**Context Commands** (`internal/commands/context_test.go`):
- Switch command with valid/invalid names
- Next/prev cycling with wrap-around
- Command execution returns correct messages

### Integration Tests

**Multi-Context Startup**:
1. Start with 3 contexts via CLI
2. Verify first context loads synchronously
3. Verify UI appears after first context
4. Verify remaining contexts load in background
5. Verify all 3 contexts appear in pool

**Context Switching**:
1. Load 2 contexts into pool
2. Switch from context A to B (instant)
3. Verify screens re-register with new repository
4. Verify data reflects new cluster
5. Switch to unloaded context C (shows progress)

**Failed Context Handling**:
1. Specify invalid context in CLI
2. Verify error appears in status bar
3. Verify context marked as Failed
4. Retry failed context
5. Verify retry attempts reload

### Manual Testing Steps

**Scenario 1: Single Context Startup**
1. Run: `k1 -context minikube`
2. Verify console shows progress
3. Verify UI appears after sync
4. Verify header shows "(minikube)"

**Scenario 2: Multi-Context Startup**
1. Run: `k1 -context prod -context staging -context dev`
2. Verify "prod" syncs first (console progress)
3. Verify UI appears immediately after "prod" syncs
4. Verify status bar shows "Loading staging context..."
5. Verify status bar shows "Context staging loaded"
6. Verify status bar shows "Loading dev context..."
7. Verify status bar shows "Context dev loaded"

**Scenario 3: Context Switching (Loaded)**
1. Start with 2 preloaded contexts
2. Navigate to `:contexts` screen
3. Select different context, press Enter
4. Verify instant switch (<100ms)
5. Verify header updates immediately
6. Verify pods screen shows new cluster's pods

**Scenario 4: Context Switching (Not Loaded)**
1. Start with 1 context
2. Navigate to `:contexts` screen
3. Select non-loaded context, press Enter
4. Verify header shows spinner: "⠋ staging (Syncing...)"
5. Verify UI remains responsive (can navigate, filter)
6. Verify switch completes after 5-15s
7. Verify header shows new context: "(staging)"

**Scenario 5: Failed Context**
1. Start app normally
2. Switch to invalid/unreachable context
3. Verify error message in status bar
4. Verify context marked "Failed" in contexts screen
5. Press ctrl+r to retry
6. Verify retry attempts reload

**Scenario 6: Pool Limit**
1. Run: `k1 -context a -context b -context c -max-contexts 2`
2. Verify only first 2 contexts load
3. Verify error/warning for context "c"
4. Switch to 3 different contexts
5. Verify LRU eviction (oldest non-active evicted)

**Scenario 7: Context Cycling**
1. Load 3 contexts (prod, staging, dev)
2. Press `:next-context`
3. Verify cycles: prod → staging → dev → prod
4. Press `:prev-context`
5. Verify cycles: prod → dev → staging → prod

## Performance Considerations

**Memory Overhead**:
- Single context: ~50MB (baseline)
- Per additional context: ~50MB (informer cache + indexes)
- Pool of 5 contexts: ~250MB total
- Pool of 10 contexts: ~500MB total

**Context Switch Latency**:
- Loaded context: <100ms (instant)
- Not loaded context: 5-15s (cache sync time)
- Failed context: 5-15s (retry sync time)

**Background Loading**:
- Non-blocking: UI remains at 60fps
- Progress messages: ~10 per context load
- Channel buffer: 10 messages (prevents blocking)

**LRU Eviction**:
- O(1) eviction (linked list)
- Cleanup: Informer stop + goroutine exit (~100ms)

## Migration Notes

**Breaking Changes**:
- `app.NewModel()` signature changed: now accepts `*RepositoryPool`
  instead of `Repository`
- CLI `-context` flag now repeatable (backward compatible with single
  value)

**Backward Compatibility**:
- Single `-context` flag works unchanged
- No `-context` flag uses current context from kubeconfig
- Existing screens work unchanged (use repository interface)

**Migration Path**:
1. Update `cmd/k1/main.go` to use pool
2. Update `internal/app/app.go` to accept pool
3. All screens continue using `Repository` interface
4. No screen code changes required

## References

- Original research:
  `thoughts/shared/research/2025-10-09-k8s-context-management.md`
- Repository interface: `internal/k8s/repository.go:79-115`
- Informer architecture:
  `internal/k8s/informer_repository.go:94-280`
- Screen config pattern: `internal/screens/config.go:40-62`
- Command registry: `internal/commands/registry.go:16-293`
- Header component: `internal/components/header.go:13-118`
