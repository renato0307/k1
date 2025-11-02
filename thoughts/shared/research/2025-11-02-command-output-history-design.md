---
date: 2025-11-02T10:30:00+0000
researcher: claude
git_commit: 30f38dd335431729d2d4272927762111b47e529c
branch: fix/bug-squash-3
repository: k1
topic: "Command Output History and Error Visibility Design"
tags: [research, ux, error-handling, audit-trail, design-proposal]
status: complete
last_updated: 2025-11-02
last_updated_by: claude
last_updated_note: "Added architecture challenges and implementation gaps"
---

# Research: Command Output History and Error Visibility Design

**Date**: 2025-11-02T10:30:00+0000
**Researcher**: claude
**Git Commit**: 30f38dd335431729d2d4272927762111b47e529c
**Branch**: fix/bug-squash-3
**Repository**: k1

## Research Question

How should k1 handle error visibility and command output feedback,
particularly for runtime errors during operation and multi-context
workflows? What's the best UX approach for reviewing command results
across multiple contexts?

## Summary

The current ephemeral status bar messages (5s timeout) make it difficult
to review command results, especially when executing operations across
multiple contexts. After analyzing alternatives (file-based error logs,
in-memory error buffers, error columns), the **:output screen** approach
emerges as the superior solution.

**Key insight**: The problem is not just "error visibility" but
"command feedback and audit trail." Users need to review what commands
they ran, what the results were, and compare outputs across contexts.

**Recommended solution**: In-memory command output history displayed in
a dedicated `:output` screen, with filtering, search, and export
capabilities for audit trails.

## Problem Statement

### Current Pain Points

1. **Ephemeral feedback**: Status bar messages auto-clear after 5
   seconds
2. **Multi-context workflow**: Execute operations in prod-us, prod-eu,
   staging - how do you review results?
3. **Hidden errors**: Context loading errors not displayed, RBAC errors
   hidden in maps
4. **No audit trail**: Can't review what commands were executed and
   their results
5. **No comparison**: Can't compare outputs across contexts

### User Workflow Example

```
1. Execute: /scale deployment nginx 3 in prod-us   → Success (5s message)
2. Switch to prod-eu
3. Execute: /scale deployment nginx 3 in prod-eu   → Error (5s message)
4. Switch to staging
5. Execute: /scale deployment nginx 3 in staging   → Success (5s message)
6. Want to review: What happened in prod-eu? → Message gone!
```

## Alternatives Analyzed

### Option 1: File-Based Error Log

**Concept**: Write errors to temporary file, track file positions per
context, navigate from failed context to error in file.

**Pros**:
- Persistent across restarts
- Can be shared/archived

**Cons**:
- ❌ High complexity: I/O, rotation, cleanup, lifecycle management
- ❌ Concurrency issues: Multiple goroutines writing, need locking
- ❌ Fragile: File positions invalidate on format changes
- ❌ Error-prone: Disk full, orphaned files on crash
- ❌ Cross-session pollution: Mixing old/new errors
- ❌ Poor UX: File-based feels wrong for TUI (external state)
- ❌ Cognitive load: New concepts, indirection (error "over there")

**Verdict**: Too complex, doesn't fit TUI mental model.

---

### Option 2: Error Column + In-Memory Buffer

**Concept**: Add Error column to contexts screen for context loading
errors. Add in-memory ring buffer for runtime errors with badge in
header.

**Pros**:
- ✅ Simple implementation (just a slice)
- ✅ No I/O complexity
- ✅ Session-scoped (GC handles cleanup)
- ✅ Error visible where problem is (contexts screen)

**Cons**:
- ⚠️ Limited scope: Only handles errors, not success outputs
- ⚠️ Discovery: Need badge/indicator for runtime errors
- ⚠️ No audit trail: Can't review successful operations
- ⚠️ Not workflow-optimized: Doesn't match multi-context operations

**Verdict**: Solves immediate problem but misses broader use case.

---

### Option 3: :output Screen (Recommended)

**Concept**: Dedicated screen showing chronological history of ALL
command outputs (success, error, info) with context, timestamp, and
full output. In-memory ring buffer, filterable, searchable, exportable.

**Pros**:
- ✅ Solves broader problem: All command feedback, not just errors
- ✅ Perfect for multi-context workflow: Compare outputs side-by-side
- ✅ Natural audit trail: Review what you did, when, in which context
- ✅ Extensible: Filter, search, export come naturally
- ✅ Familiar pattern: Terminal history, k9s logs, docker logs
- ✅ Conceptually clean: Commands produce output (neutral, not negative)
- ✅ Simple implementation: In-memory, no I/O
- ✅ Enables compliance/traceability: Export for audits

**Cons**:
- ⚠️ Discovery: Users need to learn :output exists (solvable with
  badge/auto-nav)
- ⚠️ Long outputs: Need truncation + detail view

**Verdict**: Best fit for problem domain and user workflow.

## Detailed Design: :output Screen

### Data Structure

```go
type CommandOutput struct {
    // Core fields
    Timestamp    time.Time
    Command      string              // "/scale deployment nginx 3"
    Output       string              // Full output (before truncation)
    Status       string              // "success", "error", "info", "loading"

    // Kubernetes context
    Context      string              // "prod-us"
    ResourceType k8s.ResourceType    // "deployments"
    ResourceName string              // "nginx"
    Namespace    string              // "default"

    // Execution tracking
    ExecutionID  string              // Correlation ID for multi-step ops
    StartTime    time.Time           // When command started
    EndTime      time.Time           // When completed (zero if running)
    Duration     time.Duration       // Execution time

    // Classification
    UserInitiated bool               // vs background operation
}

type OutputBuffer struct {
    mu       sync.RWMutex
    entries  []CommandOutput
    maxSize  int                     // 100 entries (bounded slice)
    sequence int64                   // Monotonic counter for ExecutionID
}

func (b *OutputBuffer) Add(entry CommandOutput) {
    b.mu.Lock()
    defer b.mu.Unlock()

    // Assign execution ID if not set
    if entry.ExecutionID == "" {
        b.sequence++
        entry.ExecutionID = fmt.Sprintf("cmd-%d", b.sequence)
    }

    b.entries = append(b.entries, entry)

    // Truncate to maxSize (bounded slice pattern)
    if len(b.entries) > b.maxSize {
        b.entries = b.entries[1:]
    }
}
```

### Capture Mechanism

**Capture point**: In `app.Update()` at `internal/app/app.go:376-402`

```go
case types.StatusMsg:
    m.messageID++
    currentID := m.messageID

    // Log errors (preserve full message)
    if msg.Type == types.MessageTypeError {
        logging.Error("User error message", "message", msg.Message)
    }

    m.userMessage.SetMessage(msg.Message, msg.Type)

    // NEW: Capture command output for history
    if shouldCaptureMessage(msg) {
        m.outputBuffer.Add(CommandOutput{
            Timestamp:     time.Now(),
            Command:       msg.Command,           // NEW field needed
            Output:        msg.Message,           // Full message
            Status:        messageTypeToStatus(msg.Type),
            Context:       m.getCurrentContextName(),
            ResourceType:  m.currentScreen.ID(),
            ResourceName:  msg.ResourceName,      // NEW field needed
            Namespace:     msg.Namespace,         // NEW field needed
            ExecutionID:   msg.ExecutionID,       // NEW field needed
            UserInitiated: msg.UserInitiated,     // NEW field needed
        })
    }

    // Forward to screen and handle auto-clear...
```

**What counts as command output?**
- ✅ User-initiated commands: /scale, /describe, /restart, /drain, etc.
- ✅ Context loading operations (success/failure)
- ✅ RBAC/sync errors during operation
- ❌ Filtering (local operation, not K8s interaction)
- ❌ Screen refresh messages (background, not actionable)

**Filter function**:
```go
func shouldCaptureMessage(msg types.StatusMsg) bool {
    // Only capture user-initiated operations
    return msg.UserInitiated
}
```

### Visual Design

```
┌─ Command Output History ────────────────────────────────────┐
│                                                              │
│ 14:25:03  [prod-us]  /describe pod nginx-abc123             │
│ ✅ Name: nginx-abc123                                        │
│    Namespace: default                                        │
│    Status: Running                                           │
│    IP: 10.1.2.3                                              │
│                                                              │
│ 14:24:12  [prod-eu]  /scale deployment nginx 5              │
│ ❌ Error: deployments.apps "nginx" not found                │
│                                                              │
│ 14:23:45  [my-cluster]  /scale deployment nginx 3           │
│ ✅ Scaled deployment nginx to 3 replicas                     │
│                                                              │
│ 14:20:00  [prod-eu]  Context Load                           │
│ ❌ Failed to connect: context deadline exceeded             │
│    Connection timed out after 5 seconds                      │
│                                                              │
└─ 'c' filter context | 's' filter status | '/' search | ↑↓───┘
```

**Rendering approach**:
- Store as structured data (not markdown)
- Render with lipgloss for styling
- Can export as markdown later

## Architecture Challenges (From Codebase Analysis)

### Challenge 1: Command Correlation (High Priority)

**Problem**: StatusMsg has no command context. Commands execute
asynchronously via `tea.Cmd`, and by the time the message is created,
the execution context is lost.

**Current structure** (`internal/types/types.go:132-135`):
```go
type StatusMsg struct {
    Message string
    Type    MessageType
    // Missing: Command, Timestamp, Context, ResourceInfo
}
```

**Gap**: Cannot correlate messages back to commands. Need to thread
metadata through async execution pipeline.

**Solution needed**:
```go
type StatusMsg struct {
    Message      string
    Type         MessageType
    Command      string              // "/scale deployment nginx 3"
    Timestamp    time.Time
    Context      string              // Kubernetes context name
    ResourceName string
    ResourceType k8s.ResourceType
    Namespace    string
    ExecutionID  string              // Correlation ID for multi-step ops
}
```

**Alternative**: Capture metadata at app.Update() level instead of
threading through command execution.

### Challenge 2: Context Threading (High Priority)

**Problem**: Current Kubernetes context stored in app model, not
accessible at message creation time.

**Example** (`internal/commands/deployment.go:66`):
```go
return messages.SuccessCmd("Scaled deployment to %d", replicas)()
// No access to current context here!
```

**Solutions**:
1. Thread context through CommandContext struct
2. Capture at app.Update() when StatusMsg received
3. Add context to Repository interface methods

**Recommended**: Capture at app.Update() - single point, less invasive.

### Challenge 3: Message Capture Point (High Priority)

**Problem**: Where to intercept messages for history storage?

**Message flow** (`internal/app/app.go:376-402`):
```go
case types.StatusMsg:
    m.messageID++                                   // Line 377
    currentID := m.messageID

    // Error logging (line 382)
    if msg.Type == types.MessageTypeError {
        logging.Error("User error message", "message", msg.Message)
    }

    m.userMessage.SetMessage(msg.Message, msg.Type) // Line 385

    // CAPTURE POINT: Insert here, after SetMessage, before auto-clear
    // m.outputBuffer.Add(buildCommandOutput(msg))

    // Auto-clear logic (lines 396-401)
```

**Considerations**:
- Capture BEFORE truncation (messages truncated at display time)
- Capture AFTER messageID increment (for proper sequencing)
- Check msg type to filter background operations

**Message truncation loss** (`internal/ui/message.go:17-21`):
```go
if lipgloss.Width(msg) > width-7 {
    msg = msg[:width-10] + "..."  // Original message lost!
}
```

**Solution**: Capture full message at app.Update(), like error logging
does (`internal/app/app.go:382`).

### Challenge 4: Background vs User-Initiated (Medium Priority)

**Problem**: No clear distinction between user commands and system
operations. All use same StatusMsg type.

**Examples**:
- User-initiated: `/scale deployment nginx 3` → SuccessMsg
- System operation: Context loading → InfoMsg
- Background sync: RBAC error → ErrorMsg

**Current criteria unclear**:
- ✅ User-initiated commands: /scale, /describe, /delete
- ✅ Context loading (explicit user action)
- ❓ Informer sync errors (background, but relevant)
- ❌ Filtering (pure UI operation)

**Solution options**:
1. Add `UserInitiated bool` flag to StatusMsg
2. Use different message types (UserCommandMsg vs SystemMsg)
3. Filter at capture time based on message content heuristics
4. Track command execution state in executor

**Recommended**: Add flag to StatusMsg, set by command layer.

### Challenge 5: Loading Message Lifecycle (Medium Priority)

**Problem**: Loading messages have different lifecycle than
success/error messages.

**Current behavior** (`internal/app/app.go:358-374`):
- Success messages: Auto-clear after 5 seconds
- Error messages: Persist until user action
- Loading messages: Persist until RefreshCompleteMsg, then cleared

**Question**: Should loading messages go in history?
- If yes: How to show "running" vs "completed" state?
- If no: How to distinguish from other InfoMsg types?

**Recommendation**: Capture loading start + completion as single entry
with duration:
```go
type CommandOutput struct {
    // ...
    StartTime   time.Time
    EndTime     time.Time
    Duration    time.Duration
    Status      string  // "running", "success", "error"
}
```

### Challenge 6: Multi-Command Correlation (Low Priority)

**Scenario**: Multiple commands running concurrently
```
14:20:00  /scale deployment A → Loading
14:20:01  /scale deployment B → Loading
14:20:05  Success (which deployment?)
14:20:06  Error (which deployment?)
```

**Problem**: No correlation between loading and completion messages.

**Solution**: Add execution ID for request tracking:
```go
type StatusMsg struct {
    ExecutionID string  // UUID or monotonic counter
    // ...
}
```

Allows grouping: Loading → Success/Error for same operation.

### Challenge 7: Existing History Pattern Not Leveraged (Medium Priority)

**Discovery**: Codebase already has command history implementation at
`internal/components/commandbar/history.go:12-45`.

**Current pattern**:
```go
type History struct {
    entries []string  // Bounded slice, not ring buffer
    index   int
}

func (h *History) Add(entry string) {
    // Deduplication logic
    if len(h.entries) > 0 && h.entries[len(h.entries)-1] == entry {
        return
    }

    h.entries = append(h.entries, entry)

    // Manual truncation to 100 entries
    if len(h.entries) > 100 {
        h.entries = h.entries[1:]
    }
}
```

**Implications**:
- Research proposes "ring buffer" but codebase uses "bounded slice"
- Should align terminology for consistency
- Can leverage existing deduplication pattern
- NavigationHistory also uses bounded slice pattern (max 50 entries)

**Recommendation**: Use "bounded slice" terminology, reference existing
patterns.

### Challenge 8: Screen Implementation Pattern (Medium Priority)

**Analysis**: ConfigScreen vs Custom Screen

**ConfigScreen pattern** (`internal/screens/config.go`):
- Table-based display
- Repository.GetResources() data source
- Fuzzy filtering built-in
- Selection tracking
- Contextual navigation

**Custom Screen pattern** (`internal/screens/system.go`):
- Full control over rendering
- Custom data sources
- Manual state management
- Read-only or custom interactions

**Recommendation for :output screen**: Use Custom Screen pattern
because:
1. Non-resource data (command history, not k8s objects)
2. Custom rendering needs (command + output + metadata)
3. No contextual navigation to other screens
4. Different interaction model (read-only history)
5. Data source is in-memory buffer, not repository

**Example**: Follow SystemScreen pattern at
`internal/screens/system.go:16-220`.

### Features

**Phase 1: MVP**
1. In-memory ring buffer (100 entries)
2. Simple chronological list (newest first)
3. Status icons (✅ success, ❌ error, ℹ️ info)
4. Context badges
5. Timestamp
6. `:output` navigation command

**Phase 2: Enhancements**
7. Filter by context (press 'c')
8. Filter by status (press 's' → toggle all/errors/success)
9. Press Enter → modal showing full output (for long outputs)
10. `/clear-output` command
11. Context integration: Enter on failed context → jump to :output
    filtered

**Phase 3: Advanced**
12. Search (press '/')
13. Copy output (press 'y')
14. `/export-output` → save as markdown file
15. Badge in header showing new output count

### Implementation Considerations

**Long output (describe returns 500 lines)**:
- List view: Show first 3-5 lines + "... (495 more lines)"
- Press Enter → modal shows full output with scrolling
- Or: Smart truncation (show key fields only)
- Use FullScreen component (`internal/components/fullscreen.go`)

**Buffer overflow (1000 commands in session)**:
- Bounded slice auto-truncates oldest (like `commandbar/history.go`)
- Show indicator: "Showing last 100 of 1000 outputs"
- Add `/clear-output` to manually clear
- Pattern: `if len(entries) > maxSize { entries = entries[1:] }`

**Command identification** (see Challenge 1):
- Add fields to StatusMsg: Command, ExecutionID, ResourceName, etc.
- OR: Build metadata at app.Update() from current state
- Recommended: Hybrid approach (thread command name, capture rest at
  app level)

**Performance (100 entries re-rendering)**:
- Use table with viewport (only render visible rows)
- SystemScreen pattern already handles 31+ resources efficiently
- Bounded slice keeps memory usage constant
- No expensive operations (all data already in memory)

**Discovery**:
- Add to help text
- Badge in header showing new error count (non-intrusive)
- Navigation command `:output` (follows existing pattern)
- Auto-navigate would conflict with context-aware navigation philosophy

**Filter implementation**:
- Use ConfigScreen fuzzy search pattern (`internal/screens/config.go:
  729-845`)
- SearchFields: ["Command", "Output", "Context", "Status",
  "ResourceName"]
- Support negation (`!error` shows non-errors)

**Export mechanism**:
- Clipboard: Use existing pattern (`internal/commands/clipboard.go`)
- File: New code needed (suggest ~/Downloads/k1-output-TIMESTAMP.md)
- Format: Markdown for readability, optionally JSON for parsing

**Navigation integration** (see Challenge 8):
- Register in navigation registry (`internal/commands/navigation.go:
  15-29`)
- Add to command registry (`internal/commands/registry.go:28-160`)
- Register screen in app.NewModel() (`internal/app/app.go:69-106`)
- No additional wiring needed (generic screen switching)

## Comparison Matrix

| Aspect | File-Based Log | Error Column + Buffer | :output Screen |
|--------|----------------|----------------------|----------------|
| **Scope** | Errors only | Errors only | All command output |
| **Complexity** | High (I/O, rotation) | Low | Low-Medium |
| **Workflow Fit** | Poor | Medium | Excellent |
| **Multi-Context** | No | Partial | Yes |
| **Audit Trail** | Yes | No | Yes |
| **Extensibility** | Limited | Limited | High |
| **Mental Model** | External state | Something broke | Command history |
| **Discovery** | Medium | High (inline) | Medium (badge) |
| **Persistence** | Across restarts | Session only | Session only |
| **Performance** | Slow (disk I/O) | Fast | Fast |

## Open Questions

### High Priority

1. **StatusMsg field additions**: Should we add all fields (Command,
   ExecutionID, ResourceName, Namespace, Context, UserInitiated) or
   build metadata at app.Update() from current state?
   - Pros: Threading gives accurate data, explicit contract
   - Cons: Invasive changes, threads through many command functions
   - Hybrid: Thread command name, capture context at app level?

2. **Loading message handling**: Capture as separate entries or combine
   loading + completion?
   - Option A: Two entries (loading, then success/error)
   - Option B: Single entry with start/end times and duration
   - Recommendation: Option B (cleaner, shows execution time)

3. **Multi-command correlation**: How to link loading → completion for
   same command when multiple commands run concurrently?
   - Use ExecutionID (UUID or monotonic counter)?
   - Store pending commands in executor state?

### Medium Priority

4. **What counts as "command output"?**
   - User-initiated commands: ✅ definitely
   - Context loading: ✅ yes (explicit user action)
   - Informer sync errors: ❓ maybe (background but relevant)
   - Screen refresh messages: ❌ no (background, not actionable)
   - Filtering operations: ❌ no (pure UI, not K8s interaction)

5. **Export format**: Markdown, JSON, or both?
   - Markdown: Human-readable, familiar (like kubectl logs)
   - JSON: Machine-readable, structured data
   - Both: Copy to clipboard (MD), save to file (JSON)?

6. **Filter strategy**: Fuzzy search (like other screens) or exact
   match (more precise)?
   - Recommendation: Fuzzy for consistency, but test with command
     history

### Low Priority

7. **Auto-navigate to :output?** After command execution, stay on
   current screen or jump to :output?
   - Recommendation: Stay on screen, add badge showing new error count
   - Rationale: Aligns with context-aware navigation philosophy

8. **Error column in contexts screen?** With :output, is inline error
   display still needed?
   - Recommendation: Skip for now, can add if users request
   - :output provides comprehensive solution

9. **Badge design**: Show count of new outputs or just indicator?
   - "⚠ 3" → Count of new errors
   - "⚠" → Just indicator
   - "⚠ 3/5" → Errors out of total new outputs

10. **Screen implementation**: Custom screen or ConfigScreen with
    synthetic ResourceType?
    - Recommendation: Custom screen (SystemScreen pattern)
    - Rationale: Non-resource data, custom rendering needs

## Recommendation

Implement **:output screen** as the primary solution for command
feedback and error visibility. It addresses:

1. ✅ Runtime errors during operation
2. ✅ Context loading errors
3. ✅ Multi-context workflow (compare outputs)
4. ✅ Audit trail for compliance/traceability
5. ✅ Extensibility (filter, search, export)

**Why this wins**:
- Solves broader problem than just "errors"
- Perfect fit for multi-cluster operations
- Familiar mental model (terminal history)
- Natural foundation for audit/export features
- Simple implementation (in-memory, no I/O)

**Incremental approach**:
- Phase 1: MVP (basic list, capture, display) - 2-3 hours
- Phase 2: Filtering, detail view - 2-3 hours
- Phase 3: Search, export, advanced features - 3-4 hours

## Code References

### Core Message System

**Status message types** (`internal/types/types.go`):
- Lines 122-161: StatusMsg struct and helper functions
  - Current fields: Message (string), Type (MessageType)
  - Need to add: Command, ExecutionID, ResourceName, Namespace, Context,
    UserInitiated
- Lines 144-161: Helper functions (InfoMsg, SuccessMsg, ErrorStatusMsg,
  LoadingMsg)

**Message handling** (`internal/app/app.go`):
- Lines 376-402: StatusMsg case in Update() - capture point for history
- Line 377: messageID increment (sequence tracking)
- Line 382: Error logging (pattern for preserving full message)
- Line 385: SetMessage() call (display to user)
- Lines 396-401: Auto-clear logic for success messages
- Lines 358-374: RefreshCompleteMsg handling (clears loading messages)

**Message display** (`internal/components/usermessage.go`):
- Lines 16-22: UserMessage struct (single message slot)
- Lines 41-50: SetMessage() implementation
- Lines 94-108: View() rendering

**Message truncation** (`internal/ui/message.go`):
- Lines 17-21: Message truncation (loses original content)
- Lines 30-50: Status-based styling (success/error/info/loading)

**Message helpers** (`internal/messages/helpers.go`):
- Lines 20-24: ErrorCmd() helper
- Lines 33-37: SuccessCmd() helper
- Lines 46-50: InfoCmd() helper

### Command Execution

**Command context** (`internal/commands/types.go`):
- Lines 24-48: CommandContext struct (ResourceType, Selected, Args)
- Line 49: ExecuteFunc signature

**Command executor** (`internal/components/commandbar/executor.go`):
- Lines 54-73: Execute() method (invokes commands)
- Lines 77-90: ExecutePending() for confirmed commands
- Lines 14-23: Executor struct (pending command state)

**Command registry** (`internal/commands/registry.go`):
- Lines 26-356: NewRegistry() with all command definitions
- Lines 371-396: Filter() method (fuzzy search)
- Lines 398-421: FilterByResourceType() (context-sensitive)

**Navigation commands** (`internal/commands/navigation.go`):
- Lines 15-29: navigationRegistry map (table-driven)
- Lines 32-38: NavigationCommand() factory function
- Lines 101-160: NextContextCommand() implementation
- Lines 163-222: PrevContextCommand() implementation

### Existing History Patterns

**Command input history** (`internal/components/commandbar/history.go`):
- Lines 12-45: History struct with bounded slice (100 entries)
- Lines 25-42: Add() method with deduplication and truncation
- Pattern: `if len(entries) > 100 { entries = entries[1:] }`

**Navigation history** (`internal/app/app.go`):
- Lines 47-51: NavigationState struct
- Line 63: navigationHistory slice (max 50 entries)
- Lines 596-612: pushNavigationHistory() method
- Lines 615-632: popNavigationHistory() method

**Context pool LRU** (`internal/k8s/repository_pool.go`):
- Lines 39-48: RepositoryPool struct with LRU list
- Lines 673-684: markUsed() (LRU updates)
- Lines 688-714: evictLRU() (bounded pool eviction)

### Screen Patterns

**Screen interface** (`internal/types/types.go`):
- Lines 10-17: Screen interface definition
- Lines 19-23: ScreenWithSelection interface

**ConfigScreen** (`internal/screens/config.go`):
- Lines 74-97: ConfigScreen struct
- Lines 50-71: ScreenConfig struct with customization options
- Lines 155-161: Init() lifecycle
- Lines 198-287: Update() with filtering
- Lines 729-845: Fuzzy search implementation

**SystemScreen** (`internal/screens/system.go`):
- Lines 16-220: Custom screen implementation (pattern for :output)
- Non-resource data (system stats)
- Custom rendering and refresh logic

**Screen registration** (`internal/app/app.go`):
- Lines 69-106: NewModel() registers all screens
- Screen registry pattern (by ID string)

**Screen configs** (`internal/screens/screens.go`):
- Lines 13-38: GetPodsScreenConfig() example
- Lines 92-116: GetDeploymentsScreenConfig() example
- Lines 42-89: getPeriodicRefreshUpdate() shared handler

### Navigation Integration

**Command palette** (`internal/components/commandbar/palette.go`):
- Lines 40-80: Filter() logic for navigation vs action commands
- Context-sensitive filtering

**Screen switching** (`internal/app/app.go`):
- Lines 309-356: ScreenSwitchMsg handler
- Navigation history push/pop on context changes

### Related Components

**Full-screen viewer** (`internal/components/fullscreen.go`):
- Pattern for displaying detailed output (YAML, describe, logs)

**Clipboard operations** (`internal/commands/clipboard.go`):
- Pattern for export functionality

**Logging system** (`internal/logging/logger.go`):
- Lines 82: Error logging pattern (preserves full message)
- Opt-in via -log-file flag

## Related Research

This research complements findings from:
- `thoughts/shared/research/2025-11-01-context-management-issues-and-code-smells.md`
  - Finding #1: Error visibility gap (contexts screen)
  - Finding #2: Stuck loading states (timeout handling)
  - Finding #3: Silent background failures
  - Finding #12: Partial success states invisible (RBAC errors)

The :output screen provides a unified solution for all these visibility
problems.

## Summary of Findings

### What Was Missing from Original Design

The original design proposed a high-level :output screen concept but
lacked critical implementation details revealed by codebase analysis:

1. **Command correlation architecture** - No mechanism to thread command
   metadata through async execution pipeline
2. **Context capture strategy** - No plan for capturing Kubernetes
   context name (stored in app, not accessible at command layer)
3. **Message capture point** - No specific location identified for
   intercepting StatusMsg
4. **Message truncation handling** - Didn't address that display
   truncates messages, losing original content
5. **Background vs user-initiated** - No filtering mechanism to
   distinguish command types
6. **Loading message lifecycle** - Didn't account for different
   lifecycle (persist until completion)
7. **Existing patterns** - Didn't reference commandbar/history.go
   bounded slice pattern
8. **Screen implementation** - Proposed options but didn't commit to
   Custom Screen pattern

### Key Architectural Decisions Needed

**High Priority**:
1. **StatusMsg enrichment**: Add fields (Command, ExecutionID,
   ResourceName, etc.) OR capture metadata at app.Update() level
2. **Command correlation**: How to link loading → completion for same
   operation
3. **User-initiated flag**: How to filter background operations from
   command history

**Medium Priority**:
4. **Loading message handling**: Single entry with duration vs separate
   loading/completion entries
5. **Export format**: Markdown, JSON, or both
6. **Filter implementation**: Fuzzy search vs exact match

### Implementation Approach

**Recommended strategy**: Hybrid metadata capture
- Thread command name through ExecuteFunc (explicit, accurate)
- Capture context/resource info at app.Update() (single point, less
  invasive)
- Add UserInitiated flag to StatusMsg (set by command layer)

**Screen implementation**: Custom Screen (SystemScreen pattern)
- Non-resource data (command history, not k8s objects)
- Custom rendering (command + output + metadata display)
- No contextual navigation needed
- Read-only history interaction model

**Data structure**: Bounded slice (not ring buffer)
- Follows existing patterns (commandbar/history.go, navigationHistory)
- Simple manual truncation: `if len(entries) > max { entries =
  entries[1:] }`
- Max 100 entries (balances memory usage vs history depth)

### Next Steps

1. **Review findings with user** - Discuss architectural decisions,
   especially StatusMsg enrichment approach
2. **Create implementation plan** - Break down into phases with specific
   tasks and file changes
3. **Phase 1 MVP scope**:
   - Add Command and UserInitiated fields to StatusMsg
   - Create OutputBuffer component with bounded slice
   - Add capture logic to app.Update() at line 385
   - Create OutputScreen (custom, following SystemScreen pattern)
   - Register :output navigation command
4. **Validate with user testing** - Test multi-context workflow
   (execute ops in prod-us, prod-eu, staging)
5. **Iterate on filtering/export** (Phase 2-3) based on feedback
