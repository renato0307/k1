# Command Output History Implementation Plan

## Overview

Implement a `:output` screen to provide persistent audit trail and review
capability for command execution results across multi-context workflows.
Current status messages auto-clear after 5 seconds, making it impossible to
review what commands were executed and their results after switching contexts
or waiting for operations to complete.

## Current State Analysis

### Message System
- `StatusMsg` (types.go:132-135) has only Message and Type fields
- Messages auto-clear after 5 seconds (app.go:24)
- All messages treated equally (no distinction between user commands and
  background operations)
- Messages handled in app.Update() at lines 376-402
- Error messages logged at line 382 (preserves full message before
  truncation)

### Command Execution
- Commands execute asynchronously via `tea.Cmd` (commands/deployment.go:48)
- Return StatusMsg with result (success/error)
- No command context tracked (command name, resource, namespace)
- Kubernetes context stored in repoPool, not accessible at message creation

### Existing Patterns
- Command input history: bounded slice, 100 entries
  (commandbar/history.go:34)
- Navigation history: bounded slice, 50 entries (app.go:63, 607-611)
- SystemScreen: custom screen pattern for non-resource data (system.go:16)

### Pain Points
1. Ephemeral feedback: Status bar messages auto-clear after 5 seconds
2. Multi-context workflow: Execute ops in prod-us, prod-eu, staging - no way
   to review results
3. Hidden errors: Context loading errors, RBAC errors not visible
4. No audit trail: Can't review what commands were executed
5. No comparison: Can't compare outputs across contexts

## Desired End State

Users can navigate to `:output` screen to see chronological list of all
command executions with:
- Command executed (e.g., "/scale deployment nginx 3")
- Result (success/error message)
- Kubernetes context where executed
- Resource type, name, namespace
- Timestamp and execution duration
- Status icon (✅ success, ❌ error, ℹ️ info)

### Verification
1. Execute `/scale deployment nginx 3` in context A
2. Switch to context B
3. Execute `/describe pod abc-123` in context B
4. Navigate to `:output` screen
5. Should see both operations with full details (command, output, context,
   timestamp)
6. Operations persist across screen switches
7. Maximum 100 entries (oldest auto-removed)

## Key Discoveries

### Discovery 1: Bounded Slice Pattern (commandbar/history.go:25-42)
Existing command history uses manual truncation:
```go
h.entries = append(h.entries, cmd)
if len(h.entries) > maxHistory {
    h.entries = h.entries[len(h.entries)-maxHistory:]
}
```
We'll follow this pattern for OutputBuffer (not ring buffer terminology).

### Discovery 2: Message Capture Point (app.go:376-402)
StatusMsg handling in app.Update() is the ideal capture point:
- Line 377: messageID increment (sequence tracking)
- Line 382: Error logging (pattern for preserving full message)
- Line 385: SetMessage() call (display to user)
- After SetMessage(), before return: **CAPTURE POINT**

### Discovery 3: SystemScreen Pattern (system.go:16-220)
Custom screen implementation for non-resource data:
- Custom rendering (not ConfigScreen)
- Own refresh logic
- Manual state management
- No contextual navigation
- Implements Screen interface

### Discovery 4: Context Access (app.go:439, 498)
Active Kubernetes context accessible via:
```go
m.repoPool.GetActiveContext()  // Returns string
```

### Discovery 5: Helper Message Pattern (messages/helpers.go:20-51)
Existing helpers return `tea.Cmd`:
```go
func SuccessCmd(format string, args ...any) tea.Cmd {
    msg := fmt.Sprintf(format, args...)
    return func() tea.Msg {
        return types.SuccessMsg(msg)
    }
}
```
We'll extend this pattern with fluent API for history metadata.

## What We're NOT Doing

1. **Not retrofitting all messages**: Only user-initiated commands tracked,
   not background operations
2. **Not using file-based storage**: In-memory only (session-scoped)
3. **Not capturing filtering operations**: Pure UI operations excluded
4. **Not tracking screen refresh**: Background operations excluded
5. **Not implementing search in MVP**: Phase 1 is basic chronological list
6. **Not implementing export in MVP**: Phase 3 feature
7. **Not tracking loading states**: Only final completion with duration
8. **Not using ring buffer**: Using bounded slice (consistent with existing
   patterns)

## Implementation Approach

### Strategy: Enhanced StatusMsg with Opt-In History

Instead of dual messages (StatusMsg + CommandHistoryMsg), we enhance
StatusMsg with optional history tracking:

```go
type StatusMsg struct {
    Message         string
    Type            MessageType
    TrackInHistory  bool              // Explicit opt-in flag
    HistoryMetadata *CommandMetadata  // Optional rich metadata
}
```

**Benefits:**
- Single message (no duplication risk)
- Explicit opt-in (call WithHistory to track)
- Automatic capture in app.Update() if TrackInHistory=true
- Works for all message sources (commands, app.go, context loading)
- No risk of StatusMsg and HistoryMsg getting out of sync

**Pattern:**
```go
// User command - tracked
messages.SuccessCmd("Scaled to %d", replicas).WithHistory(metadata)

// Background operation - not tracked
messages.InfoCmd("Refreshing...")  // No WithHistory call
```

### Data Flow

1. **Command executes** (commands/deployment.go or app.go:switchContextCmd)
2. **Creates StatusMsg** with WithHistory() → sets TrackInHistory=true +
   metadata
3. **app.Update() receives StatusMsg** (app.go:376)
4. **If TrackInHistory=true**: Add to OutputBuffer with metadata
5. **SetMessage() displays to user** (transient, 5s timeout)
6. **OutputBuffer persists entry** (permanent until overflow at 100 entries)
7. **:output screen renders** list from buffer

### Implementation Phases

**Phase 1 (2-3 hours): Core Infrastructure + Context Loading**
- Enhance StatusMsg with TrackInHistory flag and CommandMetadata
- Create OutputBuffer component (bounded slice, 100 entries)
- Add capture logic in app.Update()
- Update context loading (switchContextCmd, retryContextCmd) to track
  history
- Add WithHistory() helper
- Verify via logging that entries accumulate in buffer

**Phase 2 (2-3 hours): Output Screen**
- Create OutputScreen (custom screen, follows SystemScreen pattern)
- Register :output navigation command
- Display table: Time, Context, Command, Status, Output
- Status icons: ✅ success, ❌ error, ℹ️ info
- Shows context load entries from Phase 1

**Phase 3 (2-3 hours): Command Integration**
- Update scale, restart, describe commands
- Add start time tracking and metadata
- Wrap messages with WithHistory()
- Output screen now shows user commands + context loads

**Phases 4-5: Future Enhancements**
- Filtering (press 'c' context, 's' status)
- Search (press '/')
- Export to markdown/clipboard
- Deferred based on user feedback

## Phase 1: Core Infrastructure + Context Loading

### Overview
Add history tracking infrastructure to StatusMsg, create OutputBuffer
component, and integrate with context loading to verify the system works
end-to-end. Context loading provides immediate testability (just switch
contexts) before building the UI.

### Changes Required

#### 1. Enhance StatusMsg (internal/types/types.go)
**File**: `internal/types/types.go`
**Lines**: 132-161 (StatusMsg definition and helpers)
**Changes**: Add history tracking fields

```go
// CommandMetadata contains rich context for command history
type CommandMetadata struct {
    Command      string           // "/scale deployment nginx 3"
    Context      string           // Kubernetes context name
    ResourceType k8s.ResourceType // "deployments"
    ResourceName string           // "nginx"
    Namespace    string           // "default"
    Duration     time.Duration    // Execution time
    Timestamp    time.Time        // When executed
}

type StatusMsg struct {
    Message         string
    Type            MessageType
    TrackInHistory  bool              // NEW: Explicit opt-in flag
    HistoryMetadata *CommandMetadata  // NEW: Optional rich metadata
}
```

#### 2. Create OutputBuffer Component (internal/components/output_buffer.go)
**File**: `internal/components/output_buffer.go` (NEW)
**Changes**: Create bounded slice buffer for command history

```go
package components

import (
    "sync"
    "time"
    "github.com/renato0307/k1/internal/types"
)

const MaxOutputHistory = 100

// CommandOutput represents a single command execution in history
type CommandOutput struct {
    Command      string
    Output       string
    Status       string              // "success", "error", "info"
    Context      string
    ResourceType string
    ResourceName string
    Namespace    string
    Timestamp    time.Time
    Duration     time.Duration
}

// OutputBuffer manages command output history
type OutputBuffer struct {
    mu      sync.RWMutex
    entries []CommandOutput
}

func NewOutputBuffer() *OutputBuffer {
    return &OutputBuffer{
        entries: make([]CommandOutput, 0, MaxOutputHistory),
    }
}

// Add appends entry to history (bounded slice pattern)
func (b *OutputBuffer) Add(entry CommandOutput) {
    b.mu.Lock()
    defer b.mu.Unlock()

    b.entries = append(b.entries, entry)

    // Truncate to max size (follows commandbar/history.go:34 pattern)
    if len(b.entries) > MaxOutputHistory {
        b.entries = b.entries[len(b.entries)-MaxOutputHistory:]
    }
}

// GetAll returns all entries (newest first for display)
func (b *OutputBuffer) GetAll() []CommandOutput {
    b.mu.RLock()
    defer b.mu.RUnlock()

    // Reverse order for display (newest first)
    result := make([]CommandOutput, len(b.entries))
    for i, entry := range b.entries {
        result[len(b.entries)-1-i] = entry
    }
    return result
}

// Clear removes all entries
func (b *OutputBuffer) Clear() {
    b.mu.Lock()
    defer b.mu.Unlock()
    b.entries = make([]CommandOutput, 0, MaxOutputHistory)
}

// Count returns number of entries
func (b *OutputBuffer) Count() int {
    b.mu.RLock()
    defer b.mu.RUnlock()
    return len(b.entries)
}
```

#### 3. Add OutputBuffer to App Model (internal/app/app.go)
**File**: `internal/app/app.go`
**Lines**: 53-67 (Model struct)
**Changes**: Add outputBuffer field

```go
type Model struct {
    // ... existing fields ...
    outputBuffer      *components.OutputBuffer  // NEW
}

func NewModel(pool *k8s.RepositoryPool, theme *ui.Theme) Model {
    // ... existing code ...

    return Model{
        // ... existing fields ...
        outputBuffer: components.NewOutputBuffer(),  // NEW
    }
}
```

#### 4. Add History Capture in app.Update() (internal/app/app.go)
**File**: `internal/app/app.go`
**Lines**: 376-402 (StatusMsg case)
**Changes**: Capture history after SetMessage()

```go
case types.StatusMsg:
    m.messageID++
    currentID := m.messageID

    // Log error messages for debugging
    if msg.Type == types.MessageTypeError {
        logging.Error("User error message", "message", msg.Message)
    }

    m.userMessage.SetMessage(msg.Message, msg.Type)

    // NEW: Capture command output for history
    if msg.TrackInHistory && msg.HistoryMetadata != nil {
        entry := components.CommandOutput{
            Command:      msg.HistoryMetadata.Command,
            Output:       msg.Message,  // Full message (before truncation)
            Status:       messageTypeToStatus(msg.Type),
            Context:      msg.HistoryMetadata.Context,
            ResourceType: string(msg.HistoryMetadata.ResourceType),
            ResourceName: msg.HistoryMetadata.ResourceName,
            Namespace:    msg.HistoryMetadata.Namespace,
            Timestamp:    msg.HistoryMetadata.Timestamp,
            Duration:     msg.HistoryMetadata.Duration,
        }
        m.outputBuffer.Add(entry)
    }

    // Forward to screen...
    // ... rest of existing code ...
```

#### 5. Add Helper Function (internal/app/app.go)
**File**: `internal/app/app.go`
**Lines**: After initializeScreens() (around line 733)
**Changes**: Add helper to convert MessageType to status string

```go
// messageTypeToStatus converts MessageType to status string for history
func messageTypeToStatus(t types.MessageType) string {
    switch t {
    case types.MessageTypeSuccess:
        return "success"
    case types.MessageTypeError:
        return "error"
    case types.MessageTypeInfo:
        return "info"
    case types.MessageTypeLoading:
        return "loading"
    default:
        return "unknown"
    }
}
```

#### 6. Add WithHistory Helper (internal/messages/helpers.go)
**File**: `internal/messages/helpers.go`
**Lines**: After existing helpers (after line 68)
**Changes**: Add fluent API for history tracking

```go
// WithHistory adds history tracking to a StatusMsg
func WithHistory(cmd tea.Cmd, metadata *types.CommandMetadata) tea.Cmd {
    return func() tea.Msg {
        msg := cmd()
        if statusMsg, ok := msg.(types.StatusMsg); ok {
            statusMsg.TrackInHistory = true
            statusMsg.HistoryMetadata = metadata
            return statusMsg
        }
        return msg
    }
}
```

#### 7. Update Context Loading (internal/app/app.go)
**File**: `internal/app/app.go`
**Lines**: 657-675 (switchContextCmd function)
**Changes**: Track context load completion with history

```go
func (m Model) switchContextCmd(contextName string) tea.Cmd {
    return func() tea.Msg {
        start := time.Now()  // NEW: Track start
        oldContext := m.repoPool.GetActiveContext()
        err := m.repoPool.SwitchContext(contextName, nil)

        if err != nil {
            // Error case - track with history
            metadata := &types.CommandMetadata{
                Command:   fmt.Sprintf("Load context %s", contextName),
                Context:   contextName,
                Duration:  time.Since(start),
                Timestamp: time.Now(),
            }

            // Create error message with history
            errMsg := messages.ErrorCmd("Failed to load context %s: %v",
                contextName, err)

            return messages.WithHistory(errMsg, metadata)()
        }

        // Success case - track with history
        metadata := &types.CommandMetadata{
            Command:   fmt.Sprintf("Load context %s", contextName),
            Context:   contextName,
            Duration:  time.Since(start),
            Timestamp: time.Now(),
        }

        // Send ContextSwitchCompleteMsg (for UI state)
        // StatusMsg with history will be sent separately
        switchComplete := types.ContextSwitchCompleteMsg{
            OldContext: oldContext,
            NewContext: contextName,
        }

        successMsg := messages.SuccessCmd("Loaded context %s", contextName)

        // Return both messages
        return tea.Batch(
            func() tea.Msg { return switchComplete },
            messages.WithHistory(successMsg, metadata),
        )
    }
}
```

Also update retryContextCmd at line 678 with same pattern:
```go
func (m Model) retryContextCmd(contextName string) tea.Cmd {
    return func() tea.Msg {
        start := time.Now()  // NEW
        oldContext := m.repoPool.GetActiveContext()
        err := m.repoPool.RetryFailedContext(contextName, nil)

        if err != nil {
            metadata := &types.CommandMetadata{
                Command:   fmt.Sprintf("Retry context %s", contextName),
                Context:   contextName,
                Duration:  time.Since(start),
                Timestamp: time.Now(),
            }

            errMsg := messages.ErrorCmd("Failed to retry context %s: %v",
                contextName, err)
            return messages.WithHistory(errMsg, metadata)()
        }

        metadata := &types.CommandMetadata{
            Command:   fmt.Sprintf("Retry context %s", contextName),
            Context:   contextName,
            Duration:  time.Since(start),
            Timestamp: time.Now(),
        }

        switchComplete := types.ContextSwitchCompleteMsg{
            OldContext: oldContext,
            NewContext: contextName,
        }

        successMsg := messages.SuccessCmd("Retried context %s", contextName)

        return tea.Batch(
            func() tea.Msg { return switchComplete },
            messages.WithHistory(successMsg, metadata),
        )
    }
}
```

#### 8. Add Debug Helper (internal/app/app.go)
**File**: `internal/app/app.go`
**Lines**: After messageTypeToStatus helper (around line 740)
**Changes**: Add temporary debug method to verify buffer

```go
// Debug helper to verify OutputBuffer contents (remove after Phase 2)
func (m *Model) debugOutputBuffer() {
    entries := m.outputBuffer.GetAll()
    logging.Info("OutputBuffer contents", "count", len(entries))
    for i, entry := range entries {
        logging.Info("Entry",
            "index", i,
            "command", entry.Command,
            "context", entry.Context,
            "status", entry.Status,
            "duration", entry.Duration.String(),
        )
    }
}
```

### Success Criteria

#### Automated Verification:
- [ ] Code compiles: `make build`
- [ ] All tests pass: `make test`
- [ ] No linting errors: `go vet ./...`
- [ ] OutputBuffer tests pass (concurrent Add/GetAll)

#### Manual Verification:
- [ ] Start app with `-log-file /tmp/k1.log` flag
- [ ] Switch to different context using ctrl+n or ctrl+p
- [ ] Check status message shows "Loaded context X"
- [ ] Check logs show OutputBuffer.Add() was called
- [ ] Add call to debugOutputBuffer() after context switch in app.Update()
- [ ] Check logs show entry with: command="Load context X", duration>0,
      status="success"
- [ ] Switch to another context
- [ ] Check logs show 2 entries in buffer
- [ ] Switch 101 times, verify buffer caps at 100 entries (oldest removed)
- [ ] Verify no crashes or memory leaks

**Implementation Note**: This phase verifies the complete infrastructure
works end-to-end. You'll see entries accumulating in the buffer via logs.
After verification, remove the debugOutputBuffer() calls before proceeding
to Phase 2.

---

## Phase 2: Output Screen

### Overview
Create `:output` screen to display command history. Follows SystemScreen
pattern (custom screen for non-resource data).

### Changes Required

#### 1. Create OutputScreen (internal/screens/output.go)
**File**: `internal/screens/output.go` (NEW)
**Changes**: Custom screen displaying command history

```go
package screens

import (
    "fmt"
    "time"

    "github.com/charmbracelet/bubbles/table"
    tea "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/lipgloss"

    "github.com/renato0307/k1/internal/components"
    "github.com/renato0307/k1/internal/types"
    "github.com/renato0307/k1/internal/ui"
)

// OutputScreen displays command output history
type OutputScreen struct {
    buffer *components.OutputBuffer
    theme  *ui.Theme
    table  table.Model
    width  int
    height int
}

// NewOutputScreen creates output history screen
func NewOutputScreen(buffer *components.OutputBuffer, theme *ui.Theme) *OutputScreen {
    columns := []table.Column{
        {Title: "Time", Width: 8},
        {Title: "Context", Width: 15},
        {Title: "Command", Width: 30},
        {Title: "Status", Width: 8},
        {Title: "Output", Width: 50},
    }

    t := table.New(
        table.WithColumns(columns),
        table.WithFocused(true),
        table.WithHeight(20),
    )

    s := theme.ToTableStyles()
    t.SetStyles(s)

    return &OutputScreen{
        buffer: buffer,
        theme:  theme,
        table:  t,
    }
}

func (s *OutputScreen) Init() tea.Cmd {
    return s.refresh()
}

func (s *OutputScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        switch msg.String() {
        case "r", "ctrl+r":
            return s, s.refresh()
        }
    }

    var cmd tea.Cmd
    s.table, cmd = s.table.Update(msg)
    return s, cmd
}

func (s *OutputScreen) View() string {
    return s.table.View()
}

func (s *OutputScreen) refresh() tea.Cmd {
    return func() tea.Msg {
        entries := s.buffer.GetAll()
        rows := make([]table.Row, 0, len(entries))

        for _, entry := range entries {
            // Format timestamp
            timeStr := entry.Timestamp.Format("15:04:05")

            // Status icon
            statusIcon := s.getStatusIcon(entry.Status)

            // Truncate output for display
            output := entry.Output
            if len(output) > 50 {
                output = output[:47] + "..."
            }

            rows = append(rows, table.Row{
                timeStr,
                entry.Context,
                entry.Command,
                statusIcon,
                output,
            })
        }

        s.table.SetRows(rows)
        return types.RefreshCompleteMsg{Duration: 0}
    }
}

func (s *OutputScreen) getStatusIcon(status string) string {
    switch status {
    case "success":
        return "✅"
    case "error":
        return "❌"
    case "info":
        return "ℹ️"
    case "loading":
        return "⏳"
    default:
        return "?"
    }
}

func (s *OutputScreen) ID() string {
    return "output"
}

func (s *OutputScreen) Title() string {
    return "Command Output History"
}

func (s *OutputScreen) HelpText() string {
    return "↑/↓: Navigate | r: Refresh | esc: Back | q: Quit"
}

func (s *OutputScreen) Operations() []types.Operation {
    return []types.Operation{}
}

func (s *OutputScreen) SetSize(width, height int) {
    s.width = width
    s.height = height
    s.table.SetHeight(height - 5)
}
```

#### 2. Register OutputScreen (internal/app/app.go)
**File**: `internal/app/app.go`
**Lines**: 69-106 (NewModel screen registration)
**Changes**: Add output screen to registry

```go
func NewModel(pool *k8s.RepositoryPool, theme *ui.Theme) Model {
    registry := types.NewScreenRegistry()

    // ... existing screen registrations ...

    // System screen
    registry.Register(screens.NewSystemScreen(repo, theme))

    // Output screen (NEW)
    outputBuffer := components.NewOutputBuffer()
    registry.Register(screens.NewOutputScreen(outputBuffer, theme))

    // Contexts screen
    // ...

    return Model{
        // ...
        outputBuffer: outputBuffer,  // Store reference
    }
}
```

Also update initializeScreens() at line 698:
```go
func (m *Model) initializeScreens() {
    // ... existing screens ...

    // System screen
    m.registry.Register(screens.NewSystemScreen(repo, m.theme))

    // Output screen (NEW)
    m.registry.Register(screens.NewOutputScreen(m.outputBuffer, m.theme))

    // Contexts screen
    // ...
}
```

#### 3. Add :output Navigation Command (internal/commands/navigation.go)
**File**: `internal/commands/navigation.go`
**Lines**: 15-29 (navigationRegistry map)
**Changes**: Add output screen to navigation

```go
var navigationRegistry = map[string]string{
    // ... existing entries ...
    "system-resources": "system-resources",
    "output":           "output",  // NEW
}
```

#### 4. Register :output Command (internal/commands/registry.go)
**File**: `internal/commands/registry.go`
**Lines**: Navigation commands section (around line 83)
**Changes**: Add output command to registry

```go
// Navigation commands
registry.Register(Command{
    Name:        "pods",
    Description: "Navigate to Pods screen",
    Category:    CategoryResource,
    Execute:     NavigationCommand("pods"),
})
// ... existing navigation commands ...
registry.Register(Command{
    Name:        "output",
    Description: "Navigate to Command Output History",
    Category:    CategoryResource,
    Execute:     NavigationCommand("output"),
})
```

### Success Criteria

#### Automated Verification:
- [ ] Code compiles: `make build`
- [ ] All tests pass: `make test`
- [ ] Screen registered correctly (check registry.Get("output"))

#### Manual Verification:
- [ ] Start app, type `:output` and press Enter
- [ ] Should navigate to output screen
- [ ] Screen shows table with columns: Time, Context, Command, Status,
      Output
- [ ] Should see context load entries from Phase 1 (if contexts were loaded
      on startup or switched)
- [ ] ESC key returns to previous screen
- [ ] r key refreshes screen and updates table
- [ ] Navigate to `:output` again, entries persist

**Implementation Note**: Screen will show context loading entries from Phase
1. Phase 3 will add user command entries (scale, restart, describe).

---

## Phase 3: Command Integration

### Overview
Update scale, describe, and restart commands to send history tracking.
Context loading already tracks history (from Phase 1). This phase populates
the output screen with user command executions.

### Changes Required

#### 1. Update ScaleCommand (internal/commands/deployment.go)
**File**: `internal/commands/deployment.go`
**Lines**: 19-71 (ScaleCommand function)
**Changes**: Add history tracking with WithHistory

```go
func ScaleCommand(pool *k8s.RepositoryPool) ExecuteFunc {
    return func(ctx CommandContext) tea.Cmd {
        // Parse args
        var args ScaleArgs
        if err := ctx.ParseArgs(&args); err != nil {
            return messages.ErrorCmd("Invalid args: %v", err)
        }

        // Get resource info
        resourceName := "unknown"
        namespace := "default"
        if name, ok := ctx.Selected["name"].(string); ok {
            resourceName = name
        }
        if ns, ok := ctx.Selected["namespace"].(string); ok {
            namespace = ns
        }

        // Build kubectl scale command
        kubectlArgs := []string{
            "scale",
            string(ctx.ResourceType),
            resourceName,
            "--namespace", namespace,
            "--replicas", strconv.Itoa(args.Replicas),
        }

        // Return command that executes kubectl asynchronously
        return func() tea.Msg {
            start := time.Now()  // NEW: Track start time
            repo := pool.GetActiveRepository()
            if repo == nil {
                return messages.ErrorCmd("No active repository")()
            }
            executor := NewKubectlExecutor(repo.GetKubeconfig(), repo.GetContext())

            cmdStr := "kubectl " + strings.Join(kubectlArgs, " ")
            output, err := executor.Execute(kubectlArgs, ExecuteOptions{})

            // NEW: Build history metadata
            metadata := &types.CommandMetadata{
                Command:      fmt.Sprintf("/scale %s %s %d", ctx.ResourceType, resourceName, args.Replicas),
                Context:      repo.GetContext(),
                ResourceType: ctx.ResourceType,
                ResourceName: resourceName,
                Namespace:    namespace,
                Duration:     time.Since(start),
                Timestamp:    time.Now(),
            }

            if err != nil {
                return messages.WithHistory(
                    messages.ErrorCmd("Scale failed: %v (cmd: %s)", err, cmdStr),
                    metadata,
                )()
            }

            msg := fmt.Sprintf("%s (replicas=%d)", strings.TrimSpace(output), args.Replicas)
            if output == "" {
                msg = fmt.Sprintf("Scaled %s/%s to %d replicas", ctx.ResourceType, resourceName, args.Replicas)
            }
            return messages.WithHistory(
                messages.SuccessCmd("%s", msg),
                metadata,
            )()
        }
    }
}
```

#### 2. Update RestartCommand (internal/commands/deployment.go)
**File**: `internal/commands/deployment.go`
**Lines**: 74-130 (RestartCommand function)
**Changes**: Add history tracking (same pattern as scale)

```go
func RestartCommand(pool *k8s.RepositoryPool) ExecuteFunc {
    return func(ctx CommandContext) tea.Cmd {
        // ... existing validation ...

        return func() tea.Msg {
            start := time.Now()  // NEW
            repo := pool.GetActiveRepository()
            // ... existing execution ...

            // NEW: Build metadata
            metadata := &types.CommandMetadata{
                Command:      fmt.Sprintf("/restart %s %s", ctx.ResourceType, resourceName),
                Context:      repo.GetContext(),
                ResourceType: ctx.ResourceType,
                ResourceName: resourceName,
                Namespace:    namespace,
                Duration:     time.Since(start),
                Timestamp:    time.Now(),
            }

            if err != nil {
                return messages.WithHistory(
                    messages.ErrorCmd("Restart failed: %v", err),
                    metadata,
                )()
            }

            return messages.WithHistory(
                messages.SuccessCmd("%s", output),
                metadata,
            )()
        }
    }
}
```

#### 3. Update DescribeCommand (internal/commands/describe.go)
**File**: `internal/commands/describe.go`
**Lines**: Find executeDescribeCommand function
**Changes**: Add history tracking

```go
// Similar pattern to scale/restart - add start time, build metadata,
// wrap messages with WithHistory
```

### Success Criteria

#### Automated Verification:
- [ ] Code compiles: `make build`
- [ ] All tests pass: `make test`
- [ ] Command execution tests verify WithHistory called

#### Manual Verification:
- [ ] Execute `/scale deployment nginx 3`
- [ ] Navigate to `:output`
- [ ] Should see scale entry with command, context, success status, output
- [ ] Should also see context load entries from Phase 1 (if contexts were
      switched)
- [ ] Execute `/restart deployment nginx`
- [ ] Navigate to `:output`
- [ ] Should see restart entry in list (newest first)
- [ ] Execute `/describe pod xyz`
- [ ] Navigate to `:output`
- [ ] Should see all command entries in chronological order (newest first)
- [ ] Execute command that fails (invalid resource name, e.g., `/scale
      deployment nonexistent 5`)
- [ ] Navigate to `:output`
- [ ] Should see error entry with ❌ icon and error message
- [ ] Verify duration values are reasonable for each entry

**Implementation Note**: After Phase 3, output screen is fully functional
for basic use cases with user commands and context loading. Phases 4-5 add
filtering and export.

---

## Phase 4: Filtering & Search (Future Enhancement)

### Overview
Add filtering by context, status, and search functionality.

### Changes Required
- Add filter state to OutputScreen
- Add keybindings: 'c' (filter context), 's' (filter status), '/'
  (search)
- Filter entries before rendering table
- Show active filter in header

### Success Criteria
- Press 'c' shows context filter menu
- Press 's' toggles status filter (all/success/error)
- Press '/' opens search input
- Filtered results display correctly

**Status**: Not implemented in MVP. Defer to future based on user feedback.

---

## Phase 5: Export & Advanced Features (Future Enhancement)

### Overview
Add clipboard copy and file export capabilities.

### Changes Required
- Add 'y' keybinding to copy selected entry
- Add `/export-output` command to save as markdown
- Use clipboard.go pattern for copy
- Format as markdown with proper structure

### Success Criteria
- Press 'y' copies entry to clipboard
- Execute `/export-output` saves to file
- Markdown format is readable and structured

**Status**: Not implemented in MVP. Defer to future based on user feedback.

---

## Testing Strategy

### Unit Tests

#### OutputBuffer Tests (internal/components/output_buffer_test.go)
```go
func TestOutputBuffer_Add(t *testing.T) {
    // Test basic add
    // Test bounded slice (add 101 entries, verify oldest removed)
    // Test concurrent Add (race detector)
}

func TestOutputBuffer_GetAll(t *testing.T) {
    // Test reverse order (newest first)
    // Test empty buffer
    // Test concurrent GetAll while Add (race detector)
}

func TestOutputBuffer_Clear(t *testing.T) {
    // Test clear removes all
    // Test concurrent Clear and Add
}
```

#### OutputScreen Tests (internal/screens/output_test.go)
```go
func TestOutputScreen_Render(t *testing.T) {
    // Test empty buffer renders
    // Test populated buffer renders
    // Test status icons display correctly
}

func TestOutputScreen_Refresh(t *testing.T) {
    // Test refresh updates table rows
    // Test truncation of long output
}
```

#### StatusMsg History Tests (internal/types/types_test.go)
```go
func TestStatusMsg_WithHistory(t *testing.T) {
    // Test TrackInHistory flag set correctly
    // Test metadata preserved
    // Test without WithHistory (flag false)
}
```

### Integration Tests

#### Command History Integration (manual)
1. Start app with live cluster
2. Execute scale command → verify entry in :output
3. Switch context → verify entry in :output
4. Execute describe → verify entry in :output
5. Check order (newest first)
6. Check duration values are reasonable
7. Execute 101 commands → verify oldest removed

### Manual Testing Steps

1. **Basic Flow**:
   - Execute `/scale deployment nginx 3` → success
   - Navigate to `:output` → verify entry shows
   - ESC back to pods → entry persists
   - Return to `:output` → entry still there

2. **Multi-Context**:
   - Execute command in context A
   - Switch to context B (ctrl+n)
   - Execute command in context B
   - Navigate to `:output`
   - Should see both operations with different contexts

3. **Error Handling**:
   - Execute `/scale deployment nonexistent 5` → error
   - Navigate to `:output`
   - Should see error entry with ❌ icon
   - Error message should be preserved

4. **Buffer Overflow**:
   - Execute 101 commands
   - Navigate to `:output`
   - Should see 100 entries (oldest removed)
   - Verify no memory leak

5. **Duration Tracking**:
   - Execute slow command (context load)
   - Navigate to `:output`
   - Duration should be > 0 and reasonable (e.g., 2.5s)

## Performance Considerations

### Memory Usage
- Bounded slice: max 100 entries × ~500 bytes = ~50KB
- Negligible memory overhead
- No unbounded growth
- GC-friendly (slice reallocation follows Go best practices)

### Concurrency
- OutputBuffer uses sync.RWMutex for thread safety
- Add() called from Update() (main goroutine) - no contention expected
- GetAll() called from refresh() (background goroutine) - RLock allows
  concurrent reads
- No performance impact (< 1μs lock overhead)

### Display Rendering
- Table only renders visible rows (viewport optimization built into
  bubbles/table)
- 100 entries renders in < 1ms
- No re-fetching needed (data already in memory)

### Impact on Command Execution
- WithHistory() adds ~100ns overhead (struct copy)
- Metadata construction adds ~500ns (string formatting)
- Total overhead: < 1μs per command (negligible)

## Migration Notes

No migration needed. This is a new feature with no impact on existing data
or behavior.

Rollout:
1. Deploy Phase 1 → infrastructure only, no UI changes
2. Deploy Phase 2 → :output screen available but empty
3. Deploy Phase 3 → commands populate history
4. Users discover feature organically via `:` palette

## References

- Original research:
  `thoughts/shared/research/2025-11-02-command-output-history-design.md`
- StatusMsg definition: `internal/types/types.go:132-135`
- Message handling: `internal/app/app.go:376-402`
- Bounded slice pattern: `internal/components/commandbar/history.go:25-42`
- SystemScreen pattern: `internal/screens/system.go:16-220`
- Message helpers: `internal/messages/helpers.go:20-51`
- Command execution pattern: `internal/commands/deployment.go:19-71`
- Context loading: `internal/app/app.go:460-494`
