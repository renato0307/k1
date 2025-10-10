# Context Switching Improvements Implementation Plan

## Overview

Enhance context switching UX with keyboard shortcuts (ctrl+1 through
ctrl+0), improved context screen sorting/filtering, reduced fuzzy search
tolerance, better loading progress feedback, and comprehensive
documentation. Builds on existing context management infrastructure and
implements all 9 acceptance criteria from issue #5.

## Current State Analysis

Context management infrastructure is complete (from previous plans):
- Repository pool with LRU eviction (`internal/k8s/repository_pool.go`)
- Context switching via `:contexts` screen and Enter key
- `:next-context` and `:prev-context` commands cycle through loaded
  contexts
- Contexts screen shows Name, Cluster, User, Status columns
- Header shows context loading progress with spinner
- Background loading reports "Loading context..." messages

**Key discoveries**:
- No keyboard shortcuts for direct context switching exist
- Contexts screen sorts alphabetically only (line 534-548 in
  `repository_pool.go`)
- Fuzzy search tolerance cannot be adjusted (uses default from
  `github.com/sahilm/fuzzy`)
- Header shows loading spinner with generic messages (line 86-97 in
  `internal/components/header.go`)
- Context column missing from contexts screen
- No shortcut hint column on contexts screen

## Desired End State

### User Experience

**Keyboard Shortcuts**:
```
ctrl+1    Switch to context at position 1
ctrl+2    Switch to context at position 2
...
ctrl+9    Switch to context at position 9
ctrl+0    Switch to context at position 10
ctrl+-    Previous context (existing :prev-context)
ctrl+=    Next context (existing :next-context)
```

**Contexts Screen Improvements**:
- New leftmost column showing keyboard shortcuts (1-9, 0 for first 10)
- Loaded/Loading/Failed contexts appear first, then alphabetically
- Status included in fuzzy search (can filter by "loaded", "loading",
  etc.)
- Reduced fuzzy match tolerance (exact prefix matching required)

**Loading Progress**:
- Header shows "Loading contexts 1/3: staging" format
- Tracks how many contexts loaded vs total being loaded
- Shows current context name being loaded

**Documentation**:
- README.md includes comprehensive Context Management section
- CLI flags documented with examples
- Background loading behavior explained
- Keyboard shortcuts listed
- Contexts screen features documented
- Context commands documented
- Pool management explained

### Verification

After implementation, verify:

#### Automated Verification:
- [ ] All tests pass: `make test`
- [ ] Build succeeds: `make build`
- [ ] No linting errors: `golangci-lint run`

#### Manual Verification:
- [ ] ctrl+1 through ctrl+9 switch to positions 1-9
- [ ] ctrl+0 switches to position 10
- [ ] ctrl+- cycles to previous loaded context
- [ ] ctrl+= cycles to next loaded context
- [ ] Shortcuts do nothing if position doesn't exist
- [ ] Contexts screen shows shortcuts column (1-9, 0)
- [ ] Loaded contexts appear first, then alphabetical
- [ ] Can filter by status: type "loaded" shows only loaded contexts
- [ ] Fuzzy search requires exact prefix: "wo" doesn't match "work"
- [ ] Header shows "Loading contexts 1/n: <name>" during multi-context
   load
- [ ] Progress counter increments as each context loads

## What We're NOT Doing

- **Shortcuts beyond ctrl+0 through ctrl+9**: Only first 10 contexts get
  shortcuts
- **Custom shortcut assignment**: Fixed to position-based numbering
- **Fuzzy search configuration UI**: Hardcode stricter matching, no user
  settings
- **Context grouping/favorites**: All contexts treated equally
- **Persisting last context**: Use kubeconfig current-context only
- **Context creation/editing**: Read-only view of kubeconfig contexts

## Implementation Approach

Use position-based shortcuts (contexts in screen order determine which
gets ctrl+1, etc.). Contexts screen sorts with loaded first, making
frequently-used contexts easily accessible. Stricter fuzzy matching
reduces false positives. Loading progress uses sync.atomic counter
shared between background loaders.

**Key design principles**:
1. **Position-based shortcuts**: Screen order determines numbering, not
   fixed assignment
2. **Status-first sorting**: Loaded contexts float to top for easy
   access
3. **Exact prefix matching**: Fuzzy search penalizes non-prefix matches
   heavily
4. **Atomic progress counter**: Thread-safe tracking of loading progress

---

## Phase 1: Keyboard Shortcuts for Context Switching

### Overview
Add ctrl+1 through ctrl+0 keyboard shortcuts to switch directly to
contexts by their position in the contexts screen.

### Changes Required

#### 1. Context Switch by Index Handler

**File**: `internal/app/app.go`

**Changes**: Add keyboard handlers after line 215 (before full-screen
mode check)

```go
// Context switching shortcuts (ctrl+1 through ctrl+9, ctrl+0 for 10)
case "ctrl+1", "ctrl+2", "ctrl+3", "ctrl+4", "ctrl+5",
     "ctrl+6", "ctrl+7", "ctrl+8", "ctrl+9", "ctrl+0":
	// Extract position number (1-10)
	var position int
	if msg.String() == "ctrl+0" {
		position = 10
	} else {
		position = int(msg.Runes[0] - '0') // ctrl+1 -> 1, etc.
	}

	// Get contexts in screen order (sorted with loaded first)
	contexts, err := m.repoPool.GetContexts()
	if err != nil || position > len(contexts) {
		// Silently do nothing if invalid position
		return m, nil
	}

	// Switch to context at position (1-indexed)
	targetContext := contexts[position-1].Name

	// Don't switch if already active
	if targetContext == m.repoPool.GetActiveContext() {
		return m, nil
	}

	return m, func() tea.Msg {
		return types.ContextSwitchMsg{
			ContextName: targetContext,
		}
	}
```

**Reference**: `app.go:163-216` (existing keyboard handler block)

---

#### 2. Remap ctrl+- and ctrl+= to Context Cycling

**File**: `internal/app/app.go`

**Changes**: Add after ctrl+0 handler

```go
case "ctrl+-":
	// Previous context (existing :prev-context logic)
	updatedBar, barCmd := m.commandBar.ExecuteCommand("prev-context",
		commands.CategoryResource)
	m.commandBar = updatedBar
	return m, barCmd

case "ctrl+=":
	// Next context (existing :next-context logic)
	updatedBar, barCmd := m.commandBar.ExecuteCommand("next-context",
		commands.CategoryResource)
	m.commandBar = updatedBar
	return m, barCmd
```

**Reference**: `internal/commands/navigation.go:98-220` (existing
next/prev-context commands)

---

### Success Criteria

#### Automated Verification:
- [ ] Build succeeds: `make build`
- [ ] No compilation errors
- [ ] Keyboard handlers registered correctly

#### Manual Verification:
- [ ] ctrl+1 switches to first context in contexts screen
- [ ] ctrl+2 switches to second context
- [ ] ctrl+9 switches to ninth context (if exists)
- [ ] ctrl+0 switches to tenth context (if exists)
- [ ] ctrl+- cycles to previous loaded context
- [ ] ctrl+= cycles to next loaded context
- [ ] Shortcuts do nothing if position doesn't exist (no error)
- [ ] Shortcuts work from any screen (global)
- [ ] Already-active context doesn't trigger unnecessary switch

**Implementation Note**: After completing this phase, test with a
kubeconfig containing 12+ contexts to verify position handling.

---

## Phase 2: Contexts Screen Improvements

### Overview
Add shortcuts column, implement status-first sorting, include status in
search fields, and reduce fuzzy search tolerance.

### Changes Required

#### 1. Add Shortcuts Column to Contexts Screen

**File**: `internal/screens/screens.go`

**Changes**: Update `GetContextsScreenConfig()` columns (line 470-476)

```go
Columns: []ColumnConfig{
	{Field: "Shortcut", Title: "#", Width: 3},    // NEW: First column
	{Field: "Current", Title: "✓", Width: 5},
	{Field: "Name", Title: "Name", Width: 30},
	{Field: "Cluster", Title: "Cluster", Width: 0}, // Dynamic width
	{Field: "User", Title: "User", Width: 0},       // Dynamic width
	{Field: "Status", Title: "Status", Width: 15},
},
```

**Reference**: `screens.go:464-486` (existing contexts screen config)

---

#### 2. Populate Shortcut Field in Context Struct

**File**: `internal/k8s/repository_pool.go`

**Changes**: Update `GetContexts()` to populate shortcut field (line
494-538)

```go
func (p *RepositoryPool) GetContexts() ([]Context, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	allContexts := p.getAllContextsLocked()

	// Build result list
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

		context := Context{
			Name:      ctx.Name,
			Cluster:   ctx.Cluster,
			User:      ctx.User,
			Namespace: ctx.Namespace,
			Status:    status,
			Current:   current,
			Error:     errorMsg,
			LoadedAt:  loadedAt,
			Shortcut:  "", // Populated after sorting
		}

		result = append(result, context)
	}

	// Sort: loaded/loading/failed first, then alphabetically
	sortContextsByStatusThenName(result)

	// Populate shortcuts for first 10 contexts
	for i := 0; i < len(result) && i < 10; i++ {
		if i == 9 {
			result[i].Shortcut = "0" // Tenth position
		} else {
			result[i].Shortcut = fmt.Sprintf("%d", i+1)
		}
	}

	return result, nil
}
```

---

#### 3. Add Shortcut Field to Context Struct

**File**: `internal/k8s/context.go`

**Changes**: Add Shortcut field (after line 12)

```go
type Context struct {
	Name      string
	Cluster   string
	User      string
	Namespace string
	Status    string // "Loaded", "Loading", "Failed", "Not Loaded"
	Current   string // "✓" if current, "" otherwise
	Error     string // Error message if failed
	LoadedAt  time.Time
	Shortcut  string // NEW: Keyboard shortcut (1-9, 0, or "")
}
```

---

#### 4. Status-First Sorting Function

**File**: `internal/k8s/repository_pool.go`

**Changes**: Replace `sortContextsByName` with status-aware sort
(line 540-549)

```go
// sortContextsByStatusThenName sorts contexts with loaded/loading/failed
// first (alphabetically within each group), then not-loaded (alphabetical)
func sortContextsByStatusThenName(contexts []Context) {
	sort.Slice(contexts, func(i, j int) bool {
		// Define status priority: Loaded=0, Loading=1, Failed=2,
		// NotLoaded=3
		statusPriority := map[string]int{
			string(StatusLoaded):    0,
			string(StatusLoading):   1,
			string(StatusFailed):    2,
			string(StatusNotLoaded): 3,
		}

		priorityI := statusPriority[contexts[i].Status]
		priorityJ := statusPriority[contexts[j].Status]

		// Sort by priority first
		if priorityI != priorityJ {
			return priorityI < priorityJ
		}

		// Within same priority, sort alphabetically by name
		return contexts[i].Name < contexts[j].Name
	})
}
```

---

#### 5. Include Status in Search Fields

**File**: `internal/screens/screens.go`

**Changes**: Update `GetContextsScreenConfig()` search fields
(line 477)

```go
SearchFields: []string{"Name", "Cluster", "User", "Status"}, // Add Status
```

---

#### 6. Reduce Fuzzy Search Tolerance

**File**: `internal/screens/config.go`

**Changes**: Update `filterResources()` to use stricter matching
(find the fuzzy.Find call and adjust scoring)

Current fuzzy matching uses default scoring. Change to penalize
non-prefix matches:

```go
// In filterResources() method (find the fuzzy matching code)
// Current: uses fuzzy.Find(query, searchableFields)
// Change to:

// Build search text from configured fields
searchText := []string{}
for _, field := range s.config.SearchFields {
	if val, ok := resource[strings.ToLower(field)]; ok {
		searchText = append(searchText, fmt.Sprintf("%v", val))
	}
}
combinedText := strings.Join(searchText, " ")

// Strict prefix matching: require match at word boundary
if !strictFuzzyMatch(query, combinedText) {
	continue // Skip non-matching resources
}

// strictFuzzyMatch checks if query matches as prefix of any word
func strictFuzzyMatch(query, text string) bool {
	query = strings.ToLower(query)
	text = strings.ToLower(text)

	// Check if query is prefix of entire text
	if strings.HasPrefix(text, query) {
		return true
	}

	// Check if query is prefix of any word
	words := strings.Fields(text)
	for _, word := range words {
		if strings.HasPrefix(word, query) {
			return true
		}
	}

	return false
}
```

**Rationale**: This eliminates matches like "wo" → "work" while still
allowing "wo" → "wok" and "dev" → "development".

---

### Success Criteria

#### Automated Verification:
- [ ] Build succeeds: `make build`
- [ ] All tests pass: `make test`
- [ ] Context struct includes Shortcut field

#### Manual Verification:
- [ ] Contexts screen shows "#" column with shortcuts (1-9, 0)
- [ ] First 10 contexts have shortcuts, rest are blank
- [ ] Loaded contexts appear first (alphabetically within status group)
- [ ] Loading contexts appear second
- [ ] Failed contexts appear third
- [ ] Not Loaded contexts appear last
- [ ] Can filter by status: typing "loaded" shows only loaded contexts
- [ ] Fuzzy search is stricter: "wo" doesn't match "work"
- [ ] Fuzzy search allows prefix: "wo" matches "wok"
- [ ] Search works across all fields (Name, Cluster, User, Status)

**Implementation Note**: Test with contexts named "work", "wok", "dev",
"development" to verify fuzzy matching behavior.

---

## Phase 3: Loading Progress Counter

### Overview
Add progress tracking to show "Loading contexts 1/n: <name>" in header
during multi-context startup.

### Changes Required

#### 1. Progress Counter State

**File**: `internal/k8s/repository_pool.go`

**Changes**: Add progress tracking fields to RepositoryPool struct
(line 38-47)

```go
type RepositoryPool struct {
	mu         sync.RWMutex
	repos      map[string]*RepositoryEntry
	active     string          // Current context name
	maxSize    int             // Pool size limit
	lru        *list.List      // LRU eviction order
	kubeconfig string
	contexts   []*ContextInfo  // All contexts from kubeconfig
	loading    sync.Map        // map[string]*loadingState

	// NEW: Progress tracking for multi-context loading
	loadingProgress atomic.Int32 // Number of contexts loaded so far
	loadingTotal    atomic.Int32 // Total contexts being loaded
}
```

---

#### 2. Update LoadContext to Report Progress

**File**: `internal/k8s/repository_pool.go`

**Changes**: Update `LoadContext()` to send progress messages
(line 98-103)

```go
// Report progress with counter
if progress != nil {
	current := p.loadingProgress.Load() + 1
	total := p.loadingTotal.Load()

	if total > 1 {
		// Multi-context loading - include counter
		progress <- ContextLoadProgress{
			Context: contextName,
			Message: fmt.Sprintf("Loading contexts %d/%d: %s",
				current, total, contextName),
			Phase:   PhaseConnecting,
		}
	} else {
		// Single context loading - no counter
		progress <- ContextLoadProgress{
			Context: contextName,
			Message: "Connecting to API server...",
			Phase:   PhaseConnecting,
		}
	}
}
```

**Also update**: Lines 110-147 (after successful load, increment counter)

```go
// After successful repository creation (line 144)
p.lru.PushFront(contextName)

// Increment progress counter
p.loadingProgress.Add(1)

return nil
```

---

#### 3. Initialize Loading Counter in main.go

**File**: `cmd/k1/main.go`

**Changes**: Set loadingTotal before loading contexts
(find the context loading loop)

```go
// Before loading contexts (after pool creation)
pool.SetLoadingTotal(len(contexts))

// Load first context (BLOCKING - must complete before UI)
progressCh := make(chan k8s.ContextLoadProgress, 10)
// ... existing loading code
```

---

#### 4. Add SetLoadingTotal Method

**File**: `internal/k8s/repository_pool.go`

**Changes**: Add helper method

```go
// SetLoadingTotal sets the total number of contexts being loaded
// (used for progress reporting during multi-context startup)
func (p *RepositoryPool) SetLoadingTotal(total int) {
	p.loadingTotal.Store(int32(total))
	p.loadingProgress.Store(0)
}
```

---

#### 5. Display Progress in Header

**File**: `internal/components/header.go`

**Changes**: Update `GetLoadingText()` to handle progress format
(line 85-97)

Current implementation shows "connecting to api server ⠋". Update to
show progress counter if present in message:

```go
func (h *Header) GetLoadingText() string {
	if !h.contextLoading {
		return ""
	}
	spinner := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧"}

	// Check if message already includes formatting (progress counter)
	if strings.Contains(h.loadingMessage, "Loading contexts") {
		// Use message as-is (already formatted with counter)
		return fmt.Sprintf("%s %s", h.loadingMessage,
			spinner[h.loadingSpinner])
	}

	// Lowercase first letter for display (legacy behavior)
	message := h.loadingMessage
	if len(message) > 0 {
		message = strings.ToLower(message[:1]) + message[1:]
	}
	return fmt.Sprintf("%s %s", message, spinner[h.loadingSpinner])
}
```

---

### Success Criteria

#### Automated Verification:
- [ ] Build succeeds: `make build`
- [ ] All tests pass: `make test`
- [ ] No race conditions with atomic counters

#### Manual Verification:
- [ ] Start with 3 contexts: `k1 -context a -context b -context c`
- [ ] Header shows "Loading contexts 1/3: a ⠋" during first load
- [ ] After first load, shows "Loading contexts 2/3: b ⠋"
- [ ] After second load, shows "Loading contexts 3/3: c ⠋"
- [ ] After all load, loading spinner disappears
- [ ] Single context startup shows "Connecting to API server..."
  (no counter)
- [ ] Counter is accurate across all background loads

**Implementation Note**: Test with both single-context and multi-context
startup to verify counter only appears when total > 1.

---

## Testing Strategy

### Unit Tests

**Context Sorting** (`internal/k8s/repository_pool_test.go`):
- Verify loaded contexts appear before not-loaded
- Verify alphabetical sorting within status groups
- Verify shortcut assignment (1-9, 0, blank)

**Fuzzy Matching** (create `internal/screens/search_test.go`):
- "wo" does not match "work"
- "wo" matches "wok"
- "dev" matches "development" (prefix)
- "ev" does not match "development" (not prefix)

**Progress Counter** (create `internal/k8s/progress_test.go`):
- Counter increments correctly
- Counter resets between loads
- Multi-context shows counter, single-context doesn't

### Manual Testing Steps

**Scenario 1: Keyboard Shortcuts**
1. Start with kubeconfig containing 5 contexts
2. Navigate to :contexts screen
3. Press ctrl+1 - verify switches to first context (loaded context)
4. Press ctrl+2 - verify switches to second context
5. Press ctrl+5 - verify switches to fifth context
6. Press ctrl+9 - verify does nothing (only 5 contexts)
7. Press ctrl+- - verify cycles to previous loaded context
8. Press ctrl+= - verify cycles to next loaded context

**Scenario 2: Contexts Screen Sorting**
1. Start with 3 contexts, load 2 of them
2. Navigate to :contexts screen
3. Verify loaded contexts appear at top (positions 1-2 with shortcuts)
4. Verify not-loaded context appears at bottom (position 3)
5. Load third context
6. Refresh contexts screen (ctrl+r)
7. Verify all 3 now at top (loaded), alphabetically sorted

**Scenario 3: Status Filtering**
1. Navigate to :contexts screen
2. Type "loaded" in filter
3. Verify only loaded contexts appear
4. Type "loading" - verify shows loading contexts (if any)
5. Clear filter (esc)
6. Type context name - verify fuzzy matching works

**Scenario 4: Fuzzy Search Strictness**
1. Have context named "work" and "wok"
2. Navigate to :contexts screen
3. Type "wo" - verify both appear (prefix match)
4. Type "wor" - verify only "work" appears
5. Type "wok" - verify only "wok" appears
6. Type "rk" - verify neither appears (not prefix)

**Scenario 5: Loading Progress**
1. Start app: `k1 -context a -context b -context c`
2. Watch header during startup
3. Verify shows "Loading contexts 1/3: a"
4. Verify updates to "Loading contexts 2/3: b"
5. Verify updates to "Loading contexts 3/3: c"
6. Verify counter disappears after all loaded
7. Start app: `k1 -context single`
8. Verify shows "Connecting to API server..." (no counter)

## Phase 4: Documentation

### Overview
Document context switching features in README.md including CLI flags,
background loading, keyboard shortcuts, contexts screen, and context
commands.

### Changes Required

#### 1. Add Context Management Section to README

**File**: `README.md`

**Changes**: Add new section after "Configuration" (line 137)

```markdown
## Context Management

k1 supports runtime Kubernetes context switching with intelligent
preloading and zero-downtime switches.

### CLI Flags

```bash
# Single context (default behavior)
k1 -context production

# Multiple contexts (preload for instant switching)
k1 -context prod -context staging -context dev

# Limit pool size (default: 10)
k1 -max-contexts 5

# Custom kubeconfig path
k1 -kubeconfig /path/to/config
```

### Background Loading

When starting with multiple contexts:
1. **First context loads synchronously** - UI appears after sync completes
   (~1-2s)
2. **Remaining contexts load in background** - Non-blocking, shows
   progress
3. **Switch instantly once loaded** - Preloaded contexts switch in <100ms

Example startup with 3 contexts:
```bash
$ k1 -context prod -context staging -context dev
Connecting to Kubernetes cluster (prod)...
Syncing cache...
  Connecting to API server...
  Syncing core resources...
  Syncing dynamic resources...
Cache synced! Starting UI...

# UI appears immediately, staging/dev load in background
# Header shows: "Loading contexts 2/3: staging ⠋"
```

### Context Switching

#### Keyboard Shortcuts

**Direct Switching** (first 10 contexts):
- **`Ctrl+1`** through **`Ctrl+9`**: Switch to contexts at positions 1-9
- **`Ctrl+0`**: Switch to context at position 10

**Cycling** (loaded contexts only):
- **`Ctrl+-`**: Switch to previous loaded context (alphabetically)
- **`Ctrl+=`**: Switch to next loaded context (alphabetically)

**Notes**:
- Position determined by contexts screen order (loaded contexts first)
- Shortcuts shown in contexts screen `#` column
- Shortcuts work from any screen (global)

#### Contexts Screen

Press **`:contexts`** or **`/context`** to open contexts screen:

**Features**:
- **Shortcut column** (`#`): Shows keyboard shortcuts for first 10
  contexts
- **Current indicator** (`✓`): Shows which context is active
- **Status column**: Loaded, Loading, Failed, or Not Loaded
- **Smart sorting**: Loaded/Loading/Failed contexts appear first,
  alphabetically
- **Status filtering**: Type "loaded", "loading", "failed" to filter by
  status
- **Press Enter**: Switch to selected context

**Columns**:
```
#  ✓  Name        Cluster               User            Status
1  ✓  prod        prod.example.com      admin@prod      Loaded
2     staging     staging.example.com   admin@staging   Loaded
3     dev         dev.example.com       admin@dev       Loading
      test        test.example.com      admin@test      Not Loaded
```

### Context Commands

From command palette (**`/`**):
- **`/context <name>`**: Switch to any context by name
- **`/contexts`**: Navigate to contexts screen

From navigation palette (**`:`**):
- **`:contexts`**: Navigate to contexts screen
- **`:next-context`**: Cycle to next loaded context
- **`:prev-context`**: Cycle to previous loaded context

### Pool Management

k1 maintains a pool of loaded contexts with LRU (Least Recently Used)
eviction:

**Default behavior**:
- **Maximum contexts**: 10 (configurable with `-max-contexts`)
- **Memory per context**: ~50MB (informer cache + indexes)
- **Eviction**: Automatic when pool is full (LRU, active context never
  evicted)

**Performance**:
- **Loaded → Loaded**: <100ms (instant)
- **Loaded → Not Loaded**: 1-2s (cache sync required)
- **Failed contexts**: Can retry from contexts screen

### Examples

**Quick context switching**:
```bash
# Start with multiple contexts preloaded
k1 -context prod -context staging

# In UI:
Ctrl+1       # Switch to prod (instant if already loaded)
Ctrl+2       # Switch to staging (instant if already loaded)
:contexts    # Open contexts screen
Enter        # Switch to selected context
```

**Managing many contexts**:
```bash
# Limit pool size for low-memory environments
k1 -context ctx1 -context ctx2 -context ctx3 -max-contexts 3

# In UI, contexts screen shows first 3 with shortcuts
# Additional contexts visible but require manual switching
```

**Recovery from failures**:
```bash
# If context fails to load (network issue, invalid credentials)
:contexts         # Open contexts screen
# Navigate to failed context
Ctrl+R           # Global refresh - retries loading
```
```

**Reference**: README.md structure and existing sections

---

### Success Criteria

#### Automated Verification:
- [ ] README.md has no markdown formatting errors
- [ ] All code blocks have proper syntax highlighting
- [ ] Links and references are valid

#### Manual Verification:
- [ ] Context Management section is clear and comprehensive
- [ ] CLI flags are documented with examples
- [ ] Background loading behavior is explained
- [ ] All keyboard shortcuts are listed
- [ ] Contexts screen features are documented with example
- [ ] Context commands are listed for both palettes
- [ ] Pool management explained (LRU, memory, limits)
- [ ] Examples cover common use cases
- [ ] Documentation matches actual implementation

**Implementation Note**: After completing this phase, update the Features
section (line 5-14) to mention multi-context support as a key feature.

---

## Performance Considerations

**Keyboard Shortcuts**:
- O(1) index lookup for context switch
- No performance impact on existing operations

**Status-First Sorting**:
- O(n log n) sort on GetContexts() call
- Contexts screen refreshes every 30s (existing)
- Impact negligible for typical kubeconfig (<50 contexts)

**Fuzzy Search**:
- Stricter matching may be slightly faster (fewer candidates)
- Prefix-only check is O(m*n) where m=query length, n=words in text
- Impact negligible for small query strings

**Progress Counter**:
- atomic.Int32 operations are ~2-5ns (negligible)
- No contention (only incremented during loading, read-only after)

## References

- Original context management plan:
  `thoughts/shared/plans/2025-10-09-kubernetes-context-management.md`
- Quality fixes plan:
  `thoughts/shared/plans/2025-10-10-kubernetes-context-management-quality-fixes.md`
- Ticket: `thoughts/shared/tickets/issue_5.md`
- Repository pool: `internal/k8s/repository_pool.go:1-611`
- Contexts screen: `internal/screens/screens.go:464-486`
- Context navigation: `internal/commands/navigation.go:93-220`
- Header component: `internal/components/header.go:1-152`
