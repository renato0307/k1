---
date: 2025-10-26T18:53:43+00:00
researcher: renato0307
git_commit: 7206c20452f397727d32d009cc2183212f045b13
branch: feat/kubernetes-context-management
repository: k1
topic: "Log Streaming Implementation for TUI"
tags: [research, codebase, log-streaming, kubernetes, bubble-tea, viewport,
       tui-patterns]
status: complete
last_updated: 2025-10-26
last_updated_by: renato0307
---

# Research: Log Streaming Implementation for TUI

**Date**: 2025-10-26T18:53:43+00:00
**Researcher**: renato0307
**Git Commit**: 7206c20452f397727d32d009cc2183212f045b13
**Branch**: feat/kubernetes-context-management
**Repository**: k1

## Research Question

How can we implement log streaming in the k1 TUI with support for:
- Follow mode (live streaming)
- Stop/pause streaming
- Navigate logs (scroll up/down)
- Resume streaming
- Previous logs (from terminated containers)

## Summary

Log streaming in k1 should leverage:
1. **Full-screen component pattern** (already exists:
   `internal/components/fullscreen.go:18` - `FullScreenLogs` type)
2. **Bubble Tea channel pattern** for streaming (producer goroutine +
   consumer command)
3. **Kubernetes client-go GetLogs() API** with PodLogOptions (Follow,
   TailLines, Previous)
4. **Viewport component** for scrollable display with auto-scroll
5. **k9s-inspired UX patterns** for follow mode, filtering, and buffer
   management

The implementation follows established patterns in the codebase:
informer-based data access, config-driven screens, and message-based
state updates.

## Detailed Findings

### 1. Current TUI Architecture

#### Full-Screen Component Pattern
(`internal/components/fullscreen.go`)

The codebase already has infrastructure for full-screen views:

**FullScreenViewType enum** (line 15-19):
```go
const (
    FullScreenYAML FullScreenViewType = iota
    FullScreenDescribe
    FullScreenLogs  // Already defined, ready for implementation
)
```

**FullScreen struct** (line 26-34):
```go
type FullScreen struct {
    viewType     FullScreenViewType
    resourceName string
    content      string      // Currently string, will need []string
    width        int
    height       int
    theme        *ui.Theme
    scrollOffset int         // Already supports scrolling
}
```

**Scrolling implementation** (line 56-107):
- `up/k`, `down/j`: Line-by-line scrolling
- `pgup`, `pgdown`: Page scrolling
- `home/g`, `end/G`: Jump to top/bottom
- Automatic bounds checking with `scrollOffset`
- Scroll indicator: "1-50 of 200" (line 165-172)

**Integration with app model** (`internal/app/app.go:58-63`):
```go
type Model struct {
    fullScreen     *components.FullScreen
    fullScreenMode bool
}
```

**Message flow**:
- `types.ShowFullScreenMsg` (line 513-523): Creates component, sets
  mode
- `types.ExitFullScreenMsg` (line 525-529): Clears mode
- ESC key (line 242-248): Exits full-screen

#### Screen Architecture Pattern
(`internal/screens/config.go`)

**ConfigScreen** is the standard screen implementation:
- Uses `ScreenConfig` for configuration-driven behavior
- `Init()` (line 145) returns batch of `Refresh()` + optional periodic
  tick
- `Update()` handles messages, returns new state + commands
- Supports filtering via `filterContext` (line 82)

**Periodic refresh pattern** (line 145-156):
```go
func (s *ConfigScreen) Init() tea.Cmd {
    cmds := []tea.Cmd{s.Refresh()}

    if s.config.EnablePeriodicRefresh {
        cmds = append(cmds, tea.Tick(s.config.RefreshInterval,
            func(t time.Time) tea.Msg {
                return tickMsg(t)
            }))
    }

    return tea.Batch(cmds...)
}
```

Default interval: `RefreshInterval = 10 * time.Second`
(`internal/screens/constants.go:10`)

#### Repository Pattern
(`internal/k8s/repository.go`)

**Repository interface** provides data access abstraction:
- `GetPods()`, `GetDeployments()`, etc.: Typed access (line 85-87)
- `GetResourceYAML()`: YAML view (line 105)
- `DescribeResource()`: kubectl describe (line 106)
- **Note**: No `GetLogs()` method yet - needs to be added

**InformerRepository** (`internal/k8s/informer_repository.go:34-75`):
- Uses Kubernetes informers for real-time updates
- Context-based lifecycle management (line 73-74)
- Performance indexes for fast filtering (line 57-66)

#### Message Flow
(`internal/types/types.go`)

**Existing messages** (line 109-204):
- `ScreenSwitchMsg`: Navigate screens
- `RefreshCompleteMsg`: Data refresh complete
- `StatusMsg`: Success/Error/Info messages
- `FilterUpdateMsg`: Update filter
- `ShowFullScreenMsg`: Enter full-screen mode
- `ExitFullScreenMsg`: Leave full-screen mode

**New messages needed**:
```go
// Stream control
type LogStreamStartMsg struct {
    PodName   string
    Container string
    Follow    bool
}

type LogLineMsg struct {
    Line      string
    Timestamp time.Time
}

type LogStreamStopMsg struct{}
type LogStreamResumeMsg struct{}
type LogStreamEndMsg struct{}
type LogStreamErrorMsg struct{ Err error }
```

### 2. Kubernetes Client-Go Log Streaming

#### Core API Pattern
(Source: pkg.go.dev/k8s.io/client-go/kubernetes/typed/core/v1)

**GetLogs() returns a REST request**:
```go
func (c *pods) GetLogs(name string, opts *v1.PodLogOptions)
    *rest.Request
```

**Stream() executes the request**:
```go
func (r *Request) Stream(ctx context.Context)
    (io.ReadCloser, error)
```

**Basic pattern**:
```go
podsClient := clientset.CoreV1().Pods(namespace)
logOptions := &corev1.PodLogOptions{
    Container:  containerName,
    Follow:     true,
    Timestamps: true,
    TailLines:  &tailLines,
}

req := podsClient.GetLogs(podName, logOptions)
stream, err := req.Stream(ctx)
if err != nil {
    return err
}
defer stream.Close()

scanner := bufio.NewScanner(stream)
for scanner.Scan() {
    line := scanner.Text()
    // Send to channel for UI
}
```

#### PodLogOptions Configuration
(Source: k8s.io/api/core/v1 - PodLogOptions)

**Available options**:
```go
type PodLogOptions struct {
    Container string     // Specific container (optional)
    Follow bool          // Stream continuously (like tail -f)
    Previous bool        // Terminated container logs
    Timestamps bool      // Include timestamps
    SinceSeconds *int64  // Relative time (e.g., 3600 for 1 hour)
    SinceTime *metav1.Time  // Absolute time
    TailLines *int64     // Number of lines from end
    LimitBytes *int64    // Byte limit (approximate)
}
```

**Common configurations**:
```go
// Follow mode with last 100 lines
tailLines := int64(100)
opts := &corev1.PodLogOptions{
    Follow:     true,
    TailLines:  &tailLines,
    Timestamps: true,
}

// Previous logs (after container restart/crash)
opts := &corev1.PodLogOptions{
    Previous: true,
}

// Logs since specific time
opts := &corev1.PodLogOptions{
    SinceTime: &metav1.Time{Time: time.Now().Add(-1 * time.Hour)},
}
```

#### Stream Handling
(Source: kubernetes/kubectl/pkg/cmd/logs/logs.go)

**kubectl uses line-buffered reading**:
```go
func consumeRequest(ctx context.Context, request rest.ResponseWrapper,
    out io.Writer) error {
    readCloser, err := request.Stream(ctx)
    if err != nil {
        return err
    }
    defer readCloser.Close()

    r := bufio.NewReader(readCloser)
    for {
        bytes, err := r.ReadBytes('\n')
        if len(bytes) > 0 {
            if _, err := out.Write(bytes); err != nil {
                return err
            }
        }
        if err != nil {
            if err == io.EOF {
                return nil  // Normal termination
            }
            return err
        }
    }
}
```

**Error handling**:
- EOF is normal termination (not an error)
- Context cancellation supports graceful shutdown
- Non-2xx status codes cause errors

**Concurrent streaming** (multiple pods):
- kubectl uses io.Pipe() pattern
- Bounded concurrency: MaxFollowConcurrency = 5
- Line buffering prevents interleaving

### 3. Bubble Tea Streaming Patterns

#### Channel-Based Pattern
(Source: github.com/charmbracelet/bubbletea/examples/realtime)

**Producer-consumer pattern**:
```go
// Producer: runs continuously, sends data to channel
func listenForActivity(sub chan struct{}) tea.Cmd {
    return func() tea.Msg {
        for {
            time.Sleep(randomInterval())
            sub <- struct{}{}
        }
    }
}

// Consumer: waits for channel, returns message
func waitForActivity(sub chan struct{}) tea.Cmd {
    return func() tea.Msg {
        return responseMsg(<-sub)
    }
}

// Initialize both concurrently
func (m model) Init() tea.Cmd {
    return tea.Batch(
        listenForActivity(m.sub),
        waitForActivity(m.sub),
    )
}

// Re-schedule consumer after each message
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg.(type) {
    case responseMsg:
        m.counter++
        return m, waitForActivity(m.sub)  // Critical
    }
    return m, nil
}
```

**For Kubernetes logs**:
- Producer reads from `stream.Read()`, sends lines to channel
- Consumer waits on channel, returns `LogLineMsg`
- Update appends to buffer, re-schedules consumer

#### Viewport Component
(Source: github.com/charmbracelet/bubbles/viewport)

**Key API**:
```go
type Model struct {
    Width, Height int
    YOffset int              // Scroll position
    MouseWheelEnabled bool
}

// Content management
SetContent(s string)         // Sets full content

// Scrolling
PageDown() / PageUp()
HalfPageDown() / HalfPageUp()
ScrollDown(n int) / ScrollUp(n int)
GotoTop() / GotoBottom()

// Position queries
AtTop(), AtBottom() bool
ScrollPercent() float64
TotalLineCount() int
VisibleLineCount() int
```

**Pattern for auto-scroll**:
```go
// Append new log line
m.logs = append(m.logs, msg.line)

// Update viewport content
content := strings.Join(m.logs, "\n")
m.viewport.SetContent(content)

// Auto-scroll if follow mode enabled and at bottom
if m.followMode && m.viewport.AtBottom() {
    m.viewport.GotoBottom()
}
```

**Disabling follow on manual scroll**:
```go
case tea.KeyMsg:
    if msg.String() == "up" || msg.String() == "down" {
        m.followMode = false  // User is navigating
    }
    if msg.String() == "G" {  // Shift+g = end
        m.followMode = true   // Re-enable
        m.viewport.GotoBottom()
    }
```

### 4. k9s Implementation Analysis

#### Log Model Architecture
(Source: github.com/derailed/k9s - internal/model/log.go)

**Key features**:
- **Buffer**: Default 1000 lines (configurable)
- **Tail**: Default 100 lines on initial load
- **Autoscroll**: Toggle with 'S' key
- **Timestamps**: Toggle with 'T' key
- **Filtering**: Real-time regex filtering
- **Text wrapping**: Toggle support

**Data structure**:
```go
type LogItem struct {
    Pod, Container string
    Bytes []byte           // Raw log content
    IsError bool
    SingleContainer bool
    // Sentinel value: ItemEOF for stream end
}
```

**Stream management**:
- Spawns goroutine per container: `go updateLogs(ctx, container)`
- Channel-based: `LogChan chan *LogItem`
- Three-way select: receives, timeout flush, context cancel
- Timeout batching: Reduces UI update overhead
- Circular buffer: `Shift()` when capacity reached

**Container selection**:
- Single container: Auto-selects
- Multiple containers: AllContainers=true streams all
- Previous logs: 'P' key for terminated containers

**UX patterns**:
- 'L' key: Current logs
- 'P' key: Previous logs
- 'S' key: Toggle autoscroll/follow
- 'T' key: Toggle timestamps
- Filter mode with command buffer

#### Performance Patterns

**Circular buffer** (k9s pattern):
```go
type LogBuffer struct {
    lines    []string
    maxLines int
    index    int
    full     bool
}

func (b *LogBuffer) Add(line string) {
    b.lines[b.index] = line
    b.index = (b.index + 1) % b.maxLines
    if b.index == 0 {
        b.full = true
    }
}

func (b *LogBuffer) GetLines() []string {
    if !b.full {
        return b.lines[:b.index]
    }
    // Return in correct order
    result := make([]string, b.maxLines)
    copy(result, b.lines[b.index:])
    copy(result[b.maxLines-b.index:], b.lines[:b.index])
    return result
}
```

**Batch updates for UI performance**:
```go
type LogBatch struct {
    lines []string
    mu    sync.Mutex
}

func (b *LogBatch) Add(line string) {
    b.mu.Lock()
    defer b.mu.Unlock()
    b.lines = append(b.lines, line)
}

func (b *LogBatch) Flush() []string {
    b.mu.Lock()
    defer b.mu.Unlock()
    lines := b.lines
    b.lines = nil
    return lines
}

// Ticker to batch updates (~60 FPS)
ticker := time.NewTicker(16 * time.Millisecond)
go func() {
    for range ticker.C {
        if lines := batch.Flush(); len(lines) > 0 {
            program.Send(LogBatchMsg{Lines: lines})
        }
    }
}()
```

**Rate limiting**: Max 60 updates/second prevents overwhelming Bubble
Tea

### 5. Common UX Patterns Across TUIs

#### Follow Mode
- **Toggle key**: Single key (k9s: 'S', often 'f')
- **Auto-scroll to bottom**: New logs appear at bottom
- **Disable on user scroll**: Manual scrolling disables
- **Visual indicator**: Show follow status in UI

#### Navigation
- **Arrow keys**: Line-by-line scrolling
- **Page Up/Down**: Page-sized jumps
- **Home/End (g/G)**: Jump to top/bottom
- **Space**: Page down (common pattern)

#### Stop/Resume
- **Stop**: Pause streaming (stop API calls, keep buffer)
- **Resume**: Continue from where stopped
- **Clear**: Option to clear buffer on resume

#### Previous Logs
- **Access terminated containers**: PodLogOptions.Previous = true
- **Show restart count**: Indicate which container instance
- **Toggle**: Key to switch between current/previous

#### Filtering
- **Real-time**: Filter as you type
- **Regex support**: Pattern matching
- **Highlight matches**: Visual emphasis
- **Clear with Escape**: Standard pattern

#### Timestamps
- **Toggle**: Show/hide timestamp prefix
- **Format**: ISO 8601 or relative time
- **Extraction**: Parse from log line or API

#### Buffer Management
- **Circular buffer**: Fixed size, drop oldest
- **Configurable limit**: User sets max lines (1000-10000)
- **Memory awareness**: Clear buffers when not visible
- **Tail on start**: Load last N lines initially

## Code References

### Current Architecture
- `internal/components/fullscreen.go:15-19` - FullScreenViewType enum
- `internal/components/fullscreen.go:26-34` - FullScreen struct
- `internal/components/fullscreen.go:56-107` - Scrolling
  implementation
- `internal/app/app.go:58-63` - Full-screen mode in root model
- `internal/app/app.go:513-529` - Full-screen message handling
- `internal/screens/config.go:145-156` - Screen Init() pattern
- `internal/k8s/repository.go:80-123` - Repository interface

### Patterns to Follow
- `internal/screens/config.go:66-87` - ConfigScreen structure
- `internal/types/types.go:109-204` - Message definitions
- `internal/k8s/informer_repository.go:34-75` - Informer pattern
- `internal/messages/messages.go` - Message helper functions

## Architecture Insights

### Existing Patterns That Support Log Streaming

1. **Full-screen infrastructure ready**: FullScreenLogs type exists,
   scrolling works
2. **Repository pattern extensible**: Add GetLogs() method to interface
3. **Message-based state updates**: Fits Bubble Tea's Update() model
4. **Config-driven screens**: Can create LogScreen with ScreenConfig
5. **Command execution pattern**: /logs command exists
   (`internal/commands/pod.go:94`)

### Implementation Strategy

**Phase 1: Basic Streaming**
1. Add `GetLogs()` to Repository interface
2. Implement in InformerRepository using client-go
3. Create LogScreen (or extend FullScreen for logs)
4. Use channel pattern for streaming (producer/consumer)
5. Integrate viewport for scrollable display
6. Basic follow mode with auto-scroll

**Phase 2: Navigation and Control**
1. Stop/pause streaming (cancel context, keep buffer)
2. Resume streaming (restart with new stream)
3. Navigate while paused (viewport scrolling)
4. Jump to top/bottom (g/G keys)

**Phase 3: Advanced Features**
1. Previous logs (PodLogOptions.Previous = true)
2. Filtering (real-time regex, reuse existing fuzzy logic)
3. Timestamps toggle (parse from lines)
4. Text wrapping toggle (viewport + Reflow)
5. Container selection (multi-container pods)

**Phase 4: Performance Optimization**
1. Circular buffer (1000 lines default)
2. Batch updates (~60 FPS)
3. Memory limits (clear on exit)
4. Rate limiting for high-volume logs

### Recommended Data Structures

```go
// LogScreen (similar to ConfigScreen pattern)
type LogScreen struct {
    id           string
    podName      string
    namespace    string
    container    string
    viewport     viewport.Model
    logs         []string           // Circular buffer
    maxLines     int                // Config: default 1000
    logChan      chan string        // Channel for streaming
    ctx          context.Context    // For cancellation
    cancel       context.CancelFunc
    streaming    bool               // Stream active
    followMode   bool               // Auto-scroll enabled
    showPrevious bool               // Previous container logs
    filter       string             // Active filter
    timestamps   bool               // Show timestamps
    theme        *ui.Theme
    width        int
    height       int
}

// Messages
type LogStreamStartMsg struct {
    PodName   string
    Container string
    Namespace string
    Follow    bool
    Previous  bool
}

type LogLineMsg struct {
    Line      string
    Timestamp time.Time
}

type LogBatchMsg struct {
    Lines []string
}

type LogStreamStopMsg struct{}
type LogStreamResumeMsg struct{}
type LogStreamEndMsg struct{}
type LogStreamErrorMsg struct{ Err error }
```

### Recommended Implementation Flow

```go
// 1. Repository method
func (r *InformerRepository) GetLogs(ctx context.Context,
    namespace, podName, container string,
    opts *corev1.PodLogOptions) (io.ReadCloser, error) {

    req := r.clientset.CoreV1().Pods(namespace).
        GetLogs(podName, opts)
    return req.Stream(ctx)
}

// 2. Producer goroutine
func streamLogsToChannel(ctx context.Context, stream io.ReadCloser,
    ch chan string) tea.Cmd {
    return func() tea.Msg {
        go func() {
            defer stream.Close()
            scanner := bufio.NewScanner(stream)
            for scanner.Scan() {
                select {
                case <-ctx.Done():
                    return
                case ch <- scanner.Text():
                }
            }
            close(ch)
        }()
        return LogStreamStartMsg{}
    }
}

// 3. Consumer command
func waitForLogLine(ch chan string) tea.Cmd {
    return func() tea.Msg {
        line, ok := <-ch
        if !ok {
            return LogStreamEndMsg{}
        }
        return LogLineMsg{Line: line}
    }
}

// 4. Init
func (s *LogScreen) Init() tea.Cmd {
    tailLines := int64(100)
    opts := &corev1.PodLogOptions{
        Container:  s.container,
        Follow:     true,
        TailLines:  &tailLines,
        Timestamps: s.timestamps,
    }

    stream, err := s.repo.GetLogs(s.ctx, s.namespace, s.podName,
        s.container, opts)
    if err != nil {
        return func() tea.Msg {
            return LogStreamErrorMsg{Err: err}
        }
    }

    return tea.Batch(
        streamLogsToChannel(s.ctx, stream, s.logChan),
        waitForLogLine(s.logChan),
    )
}

// 5. Update
func (s *LogScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case LogLineMsg:
        // Append to buffer (circular)
        s.logs = append(s.logs, msg.Line)
        if len(s.logs) > s.maxLines {
            s.logs = s.logs[len(s.logs)-s.maxLines:]
        }

        // Update viewport
        content := strings.Join(s.logs, "\n")
        s.viewport.SetContent(content)

        // Auto-scroll if follow mode
        if s.followMode && s.viewport.AtBottom() {
            s.viewport.GotoBottom()
        }

        // Re-schedule consumer
        return s, waitForLogLine(s.logChan)

    case LogStreamEndMsg:
        s.streaming = false
        return s, nil

    case tea.KeyMsg:
        switch msg.String() {
        case "f":  // Toggle follow
            s.followMode = !s.followMode
            if s.followMode {
                s.viewport.GotoBottom()
            }
            return s, nil

        case "s":  // Stop/resume
            if s.streaming {
                s.cancel()
                s.streaming = false
            } else {
                // Restart stream
                return s, s.restart()
            }
            return s, nil

        case "p":  // Toggle previous logs
            s.showPrevious = !s.showPrevious
            s.cancel()
            return s, s.restart()

        case "up", "down", "pgup", "pgdown", "home", "end":
            if msg.String() == "up" || msg.String() == "down" {
                s.followMode = false
            }
            var cmd tea.Cmd
            s.viewport, cmd = s.viewport.Update(msg)
            return s, cmd

        case "G":  // Jump to end, re-enable follow
            s.followMode = true
            s.viewport.GotoBottom()
            return s, nil
        }
    }

    return s, nil
}
```

## Historical Context (from thoughts/)

No previous research documents found on log streaming implementation.

## Related Research

- Future: Research on multi-pod log streaming (stern-like)
- Future: Research on log filtering and search patterns
- Future: Research on log export and saving

## Open Questions

1. **Multi-container pods**: Should we show all containers
   simultaneously (like stern) or let user select?
2. **Container list UI**: How to present container selection? Modal or
   inline list?
3. **Filter integration**: Use existing command bar filter or separate
   log filter?
4. **Buffer size**: 1000 lines default? Make configurable? Where to
   store config?
5. **Color coding**: Should we support ANSI colors in logs (some apps
   use them)?
6. **Log export**: Should we support saving logs to file? Copy to
   clipboard?
7. **Screen vs Full-screen**: Implement as Screen (with operations) or
   FullScreen component?
8. **Reconnection**: Should we auto-reconnect on stream errors, or let
   user manually resume?
9. **Status indicators**: Show stream status (streaming, paused,
   stopped) where? Command bar?
10. **Keyboard shortcuts**: Which keys for stop/resume/previous/follow?
    Follow k9s or create our own?

## Implementation Recommendations

### Priority Order

**Must Have (Phase 1)**:
1. Basic streaming with follow mode
2. Stop/pause and resume
3. Navigate logs (scroll up/down)
4. Auto-scroll toggle
5. Previous logs support

**Should Have (Phase 2)**:
1. Container selection (multi-container pods)
2. Timestamps toggle
3. Basic filtering
4. Buffer limit configuration

**Nice to Have (Phase 3)**:
1. Text wrapping toggle
2. ANSI color support
3. Log export/save
4. Multi-pod streaming (stern-like)
5. Search within logs

### Key Decisions

1. **Use FullScreen component**: Extend existing FullScreen for logs
   (simpler)
2. **Follow k9s UX**: Use 'f' for follow, 's' for stop/resume, 'p' for
   previous
3. **Circular buffer**: 1000 lines default, make configurable via
   ~/.config/k1/config.yaml
4. **Batch updates**: 60 FPS max (16ms ticker)
5. **Repository method**: Add GetLogs() to Repository interface
6. **Channel pattern**: Producer/consumer with context cancellation
7. **Viewport for display**: Use bubbles/viewport for scrolling
8. **Auto-select container**: Single container = auto-select, multiple
   = prompt user

### Next Steps

1. Add GetLogs() method to Repository interface
2. Implement GetLogs() in InformerRepository using client-go
3. Create or extend component for log display (FullScreen vs new
   LogScreen)
4. Implement basic streaming with channel pattern
5. Add viewport integration for scrolling
6. Implement follow mode with auto-scroll
7. Add stop/resume functionality
8. Add previous logs support
9. Test with various pods (single/multi-container, high/low volume)
10. Iterate on UX based on testing

### Testing Considerations

- **Namespace isolation**: Each test creates unique namespace
- **Mock stream**: Create mock io.ReadCloser for testing
- **Buffer tests**: Verify circular buffer behavior
- **Follow mode tests**: Verify auto-scroll logic
- **Stop/resume tests**: Verify context cancellation
- **Previous logs tests**: Verify PodLogOptions.Previous works
- **Multi-container tests**: Verify container selection logic
- **Performance tests**: High-volume log streams (1000+ lines/sec)
