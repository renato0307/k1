# Timoneiro: Bubble Tea Architecture Design

This document describes the recommended architecture patterns for building Timoneiro as a moderately complex Bubble Tea TUI application. It's based on analysis of official Bubble Tea examples, real-world applications (soft-serve, lazygit, gum), and lessons learned from the existing prototypes.

## Table of Contents

1. [Application Structure Patterns](#1-application-structure-patterns)
2. [State Management](#2-state-management)
3. [Screen Navigation & Routing](#3-screen-navigation--routing)
4. [Modal & Overlay Patterns](#4-modal--overlay-patterns)
5. [Background Task Handling](#5-background-task-handling)
6. [External Editor Integration](#6-external-editor-integration)
7. [Clipboard Interaction](#7-clipboard-interaction)
8. [Error Handling & Display](#8-error-handling--display)
9. [Data Layer Separation](#9-data-layer-separation)
10. [Command Palette Implementation](#10-command-palette-implementation)
11. [Common Anti-Patterns to Avoid](#11-common-anti-patterns-to-avoid)
12. [Recommended Project Structure](#12-recommended-project-structure)

---

## 1. Application Structure Patterns

### Single Root Model with Embedded Component Models

**Pattern**: Use a single root model that embeds specialized component models for different screens/views.

```go
// Root model coordinates all application state
type model struct {
    // Application-level state
    width, height    int
    quitting         bool
    currentScreen    screenState

    // Embedded screen models
    podListScreen    *PodListModel
    deploymentScreen *DeploymentListModel
    detailScreen     *DetailViewModel

    // Shared services (injected)
    k8sClient        *KubernetesClient
    clipboardManager *ClipboardManager

    // Common UI components
    help             help.Model
    statusBar        StatusBarModel
}

type screenState int

const (
    screenPodList screenState = iota
    screenDeploymentList
    screenDetailView
    screenLogStream
)
```

**Key Principles**:
- Root model handles routing between screens
- Each screen is a separate sub-model with its own Update/View logic
- Shared state and services live at the root level
- Screen models are initialized lazily (only when first accessed)

**References**:
- `/tmp/bubbletea/examples/views/main.go` - demonstrates view switching with state machine
- `/tmp/bubbletea/examples/composable-views/main.go` - shows composing multiple bubble models
- `/tmp/soft-serve/pkg/ui/common/common.go` - uses embedded Common struct pattern

---

## 2. State Management

### Layered State Architecture

**Pattern**: Separate state into three layers:

1. **Application State** (global, persistent across screens)
2. **Screen State** (local to a specific view, reset on navigation)
3. **Transient State** (UI state like cursor position, selections)

```go
// Application-level state (lives in root model)
type AppState struct {
    CurrentContext   string              // Current k8s context
    Namespaces       []string            // Available namespaces
    SelectedNS       string              // Selected namespace
    Theme            Theme               // UI theme
    KeyBindings      KeyMap              // Keybindings
}

// Screen-local state (lives in screen model)
type PodListState struct {
    pods             []Pod               // Data to display
    filteredPods     []Pod               // After filtering
    filterText       string              // Current filter
    filterActive     bool                // Filter mode on/off
    selectedPodKey   string              // Track selection across updates
    lastSearchTime   time.Duration       // Performance telemetry
}

// Transient UI state (managed by bubbles components)
type PodListUI struct {
    table            table.Model         // Bubbles table component
    viewport         viewport.Model      // For scrolling
    windowWidth      int                 // Updated on resize
    windowHeight     int                 // Updated on resize
}
```

### State Synchronization Pattern

For keeping UI state synchronized with data state (like maintaining cursor position across data updates):

```go
// Track selection by key, not by index
type selectionTracker struct {
    selectedKey string  // e.g., "namespace/name"
}

// After data refresh, restore selection
func (m *PodListModel) restoreSelection() {
    if m.selectedKey == "" {
        return
    }

    // Find the item with the same key
    for i, pod := range m.filteredPods {
        if podKey(pod) == m.selectedKey {
            m.table.SetCursor(i)
            return
        }
    }

    // If not found, select nearest item
    if len(m.filteredPods) > 0 {
        cursor := min(m.table.Cursor(), len(m.filteredPods)-1)
        m.table.SetCursor(cursor)
        m.selectedKey = podKey(m.filteredPods[cursor])
    }
}
```

**Key Lessons from proto-pods-tui**:
- Tracking by key prevents cursor jumping during live updates
- Separate filter state from filter mode (active vs. editing)
- Cache search performance metrics for transparency

**References**:
- `/Users/renato/Work/willful/timoneiro/cmd/proto-pods-tui/main.go` (lines 42-57, 183-186, 552-572)
- `/tmp/lazygit/pkg/gui/types/context.go` - sophisticated context state management

---

## 3. Screen Navigation & Routing

### State Machine Navigation

**Pattern**: Use explicit state machine with typed screen constants.

```go
type Screen int

const (
    ScreenResourceList Screen = iota
    ScreenDetailView
    ScreenLogStream
    ScreenYAMLEditor
    ScreenCommandPalette  // Modal overlay
)

type navigationStack struct {
    stack []Screen
}

func (n *navigationStack) Push(screen Screen) {
    n.stack = append(n.stack, screen)
}

func (n *navigationStack) Pop() Screen {
    if len(n.stack) == 0 {
        return ScreenResourceList // Default screen
    }
    top := n.stack[len(n.stack)-1]
    n.stack = n.stack[:len(n.stack)-1]
    return top
}

// In root Update function
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        // Handle navigation keys at root level
        switch msg.String() {
        case "esc":
            // Pop navigation stack
            if len(m.navStack.stack) > 0 {
                m.currentScreen = m.navStack.Pop()
            }
            return m, nil

        case "enter":
            // Drill down into detail view
            if m.currentScreen == ScreenResourceList {
                m.navStack.Push(m.currentScreen)
                m.currentScreen = ScreenDetailView
                return m, m.loadDetailCmd()
            }
        }
    }

    // Route to appropriate screen's Update function
    return m.updateCurrentScreen(msg)
}
```

### Delegating Updates to Screen Models

```go
func (m model) updateCurrentScreen(msg tea.Msg) (tea.Model, tea.Cmd) {
    var cmd tea.Cmd

    switch m.currentScreen {
    case ScreenPodList:
        *m.podListScreen, cmd = m.podListScreen.Update(msg)
    case ScreenDetailView:
        *m.detailScreen, cmd = m.detailScreen.Update(msg)
    case ScreenLogStream:
        *m.logScreen, cmd = m.logScreen.Update(msg)
    }

    return m, cmd
}
```

**Best Practices**:
- Handle global keys (quit, help, command palette) at root level
- Delegate screen-specific keys to screen models
- Use navigation stack for back/forward navigation
- Initialize screen models lazily (only when first accessed)

**References**:
- `/tmp/bubbletea/examples/views/main.go` - demonstrates screen switching
- `/tmp/lazygit/pkg/gui/context.go` - complex context management system

---

## 4. Modal & Overlay Patterns

### Modal State Pattern

**Pattern**: Modals are special screen states that render on top of the current screen.

```go
type Modal int

const (
    ModalNone Modal = iota
    ModalConfirm
    ModalCommandPalette
    ModalFilter
    ModalHelp
)

type model struct {
    currentScreen Screen
    currentModal  Modal
    modalData     interface{}  // Context-specific modal data

    // Modal-specific models
    confirmModal       ConfirmModel
    commandPalette     CommandPaletteModel
    filterInput        textinput.Model
}

func (m model) View() string {
    // Render base screen
    baseView := m.renderCurrentScreen()

    // If modal active, overlay it on top
    if m.currentModal != ModalNone {
        modalView := m.renderModal()
        return overlayModal(baseView, modalView)
    }

    return baseView
}

// Overlay rendering using lipgloss
func overlayModal(base, modal string) string {
    // Dim the background
    dimmedBase := lipgloss.NewStyle().
        Foreground(lipgloss.Color("241")).
        Render(base)

    // Center the modal
    centered := lipgloss.Place(
        width, height,
        lipgloss.Center, lipgloss.Center,
        lipgloss.NewStyle().
            Border(lipgloss.RoundedBorder()).
            BorderForeground(lipgloss.Color("63")).
            Padding(1, 2).
            Render(modal),
    )

    // Layer modal over dimmed background
    return lipgloss.PlaceOver(dimmedBase, centered)
}
```

### Confirmation Modal Pattern

```go
type ConfirmModel struct {
    prompt   string
    callback func() tea.Cmd  // Called on confirmation
    width    int
}

func NewConfirmModal(prompt string, onConfirm func() tea.Cmd) ConfirmModel {
    return ConfirmModel{
        prompt:   prompt,
        callback: onConfirm,
    }
}

func (m ConfirmModel) Update(msg tea.Msg) (ConfirmModel, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        switch msg.String() {
        case "y", "Y":
            // Confirm and execute callback
            return m, m.callback()
        case "n", "N", "esc":
            // Cancel - parent will clear modal
            return m, nil
        }
    }
    return m, nil
}

func (m ConfirmModel) View() string {
    return fmt.Sprintf("%s\n\n[y]es / [n]o", m.prompt)
}

// Usage in parent model
func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    if m.currentModal == ModalConfirm {
        var cmd tea.Cmd
        m.confirmModal, cmd = m.confirmModal.Update(msg)

        // Check if modal should close
        if cmd != nil {
            m.currentModal = ModalNone
        }

        return m, cmd
    }
    // ... handle other cases
}
```

**Key Patterns**:
- Modals block input to underlying screen
- Use lipgloss overlays for visual layering
- Modal data is passed in, not stored globally
- Escape key always cancels modal

**References**:
- `/tmp/bubbletea/examples/views/main.go` - shows view layering
- `/tmp/lazygit/pkg/gui/popup/` - sophisticated popup system
- Lipgloss library for overlay rendering

---

## 5. Background Task Handling

### Message-Based Async Pattern

**Pattern**: Use commands that return messages to update UI with async results.

```go
// Message types for async operations
type podsFetchedMsg struct {
    pods []Pod
    err  error
}

type podDeletedMsg struct {
    podName string
    err     error
}

type progressUpdateMsg struct {
    operation string
    percent   float64
}

// Command to fetch pods asynchronously
func fetchPodsCmd(lister v1listers.PodLister) tea.Cmd {
    return func() tea.Msg {
        pods, err := lister.List(labels.Everything())
        if err != nil {
            return podsFetchedMsg{err: err}
        }

        // Convert to display format
        displayPods := convertPods(pods)
        return podsFetchedMsg{pods: displayPods}
    }
}

// Command with progress updates
func deletePodCmd(clientset kubernetes.Interface, ns, name string) tea.Cmd {
    return func() tea.Msg {
        // This runs in a goroutine
        err := clientset.CoreV1().Pods(ns).Delete(
            context.Background(),
            name,
            metav1.DeleteOptions{},
        )
        return podDeletedMsg{podName: name, err: err}
    }
}

// In Update function
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case podsFetchedMsg:
        if msg.err != nil {
            m.err = msg.err
            return m, nil
        }
        m.pods = msg.pods
        m.loading = false
        return m, nil

    case podDeletedMsg:
        if msg.err != nil {
            m.statusBar.SetError(fmt.Sprintf("Failed to delete: %v", msg.err))
        } else {
            m.statusBar.SetSuccess(fmt.Sprintf("Deleted %s", msg.podName))
        }
        // Refresh the list
        return m, fetchPodsCmd(m.lister)
    }
    return m, nil
}
```

### Progress Tracking Pattern

```go
// For long-running operations
type OperationTracker struct {
    operations map[string]*Operation
    mu         sync.Mutex
}

type Operation struct {
    ID       string
    Label    string
    Progress float64  // 0.0 to 1.0
    Status   OpStatus
}

type OpStatus int

const (
    OpStatusRunning OpStatus = iota
    OpStatusSuccess
    OpStatusError
)

// Spinner for indeterminate progress
type LoadingIndicator struct {
    spinner spinner.Model
    label   string
}

func (m *model) Init() tea.Cmd {
    return tea.Batch(
        m.spinner.Tick,
        fetchPodsCmd(m.lister),
    )
}

// Display loading state in View
func (m model) View() string {
    if m.loading {
        return fmt.Sprintf("%s Loading pods...\n", m.spinner.View())
    }
    return m.renderPodList()
}
```

**Key Principles**:
- All I/O happens in commands (functions returning tea.Msg)
- Long-running ops return immediately with loading state
- Use custom message types for results
- Show spinners/progress for user feedback
- Handle errors gracefully with user-visible messages

**References**:
- `/tmp/bubbletea/examples/http/main.go` - async HTTP request
- `/tmp/bubbletea/examples/package-manager/main.go` - progress tracking
- `/Users/renato/Work/willful/timoneiro/cmd/proto-pods-tui/main.go` (lines 276-295) - informer polling

---

## 6. External Editor Integration

### Editor Launch Pattern

**Pattern**: Use `tea.ExecProcess` to suspend TUI and launch external editor.

```go
import (
    "os"
    "os/exec"
    tea "github.com/charmbracelet/bubbletea"
)

type editorFinishedMsg struct {
    err      error
    filePath string
}

// Launch editor command
func openEditorCmd(filePath string) tea.Cmd {
    editor := os.Getenv("EDITOR")
    if editor == "" {
        editor = "vim"  // Fallback
    }

    c := exec.Command(editor, filePath)

    // tea.ExecProcess suspends the TUI and runs the command
    return tea.ExecProcess(c, func(err error) tea.Msg {
        return editorFinishedMsg{
            err:      err,
            filePath: filePath,
        }
    })
}

// Usage: Edit pod YAML
func (m *model) editPodYAML(pod Pod) tea.Cmd {
    // 1. Fetch full pod YAML
    // 2. Write to temp file
    tempFile := "/tmp/timoneiro-pod.yaml"

    // 3. Launch editor
    return tea.Sequence(
        func() tea.Msg {
            // Get pod YAML and write to file
            yaml := m.getPodYAML(pod)
            os.WriteFile(tempFile, []byte(yaml), 0644)
            return nil
        },
        openEditorCmd(tempFile),
    )
}

// Handle editor result
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case editorFinishedMsg:
        if msg.err != nil {
            m.statusBar.SetError(fmt.Sprintf("Editor error: %v", msg.err))
            return m, nil
        }

        // Read modified file and apply changes
        return m, m.applyYAMLChangesCmd(msg.filePath)
    }
    return m, nil
}
```

### Temporary File Management

```go
type TempFileManager struct {
    files []string
}

func (t *TempFileManager) Create(prefix, content string) (string, error) {
    f, err := os.CreateTemp("", prefix+"*.yaml")
    if err != nil {
        return "", err
    }
    defer f.Close()

    path := f.Name()
    t.files = append(t.files, path)

    _, err = f.WriteString(content)
    return path, err
}

func (t *TempFileManager) Cleanup() {
    for _, f := range t.files {
        os.Remove(f)
    }
}

// Cleanup on exit
func (m model) Quit() tea.Cmd {
    m.tempFiles.Cleanup()
    return tea.Quit
}
```

**Key Principles**:
- Use `tea.ExecProcess` for blocking external commands
- Always respect `$EDITOR` environment variable
- Clean up temp files on exit
- Show status message after editor closes
- Handle editor errors gracefully

**References**:
- `/tmp/bubbletea/examples/exec/main.go` - complete editor integration example
- `/tmp/bubbletea/examples/suspend/main.go` - suspend/resume pattern

---

## 7. Clipboard Interaction

### Clipboard Manager Pattern

**Pattern**: Use `atotto/clipboard` or OS-specific commands for clipboard access.

```go
import "github.com/atotto/clipboard"

type ClipboardManager struct {
    // Optional: track clipboard history
    history []string
}

func (c *ClipboardManager) Copy(text string) error {
    err := clipboard.WriteAll(text)
    if err == nil {
        c.history = append(c.history, text)
    }
    return err
}

func (c *ClipboardManager) Paste() (string, error) {
    return clipboard.ReadAll()
}

// Usage in keybindings
func (m *PodListModel) handleKey(key string) tea.Cmd {
    switch key {
    case "y":  // Yank (copy) to clipboard
        pod := m.getSelectedPod()
        text := fmt.Sprintf("%s/%s", pod.Namespace, pod.Name)

        if err := m.clipboard.Copy(text); err != nil {
            return m.showStatusMsg(fmt.Sprintf("Copy failed: %v", err))
        }
        return m.showStatusMsg(fmt.Sprintf("Copied: %s", text))

    case "Y":  // Copy full resource YAML
        pod := m.getSelectedPod()
        yaml := m.fetchPodYAML(pod)

        if err := m.clipboard.Copy(yaml); err != nil {
            return m.showStatusMsg("Copy failed")
        }
        return m.showStatusMsg("Copied YAML to clipboard")
    }
    return nil
}
```

### Clipboard Actions

Common clipboard operations for Kubernetes TUI:

```go
type ClipboardAction int

const (
    CopyName ClipboardAction = iota
    CopyNamespace
    CopyFullName      // namespace/name
    CopyYAML
    CopyJSON
    CopyNodeName
    CopyIP
)

func (m *model) copyToClipboard(action ClipboardAction) tea.Cmd {
    var text string
    pod := m.getSelectedPod()

    switch action {
    case CopyName:
        text = pod.Name
    case CopyNamespace:
        text = pod.Namespace
    case CopyFullName:
        text = fmt.Sprintf("%s/%s", pod.Namespace, pod.Name)
    case CopyYAML:
        text = m.fetchResourceYAML(pod)
    case CopyNodeName:
        text = pod.Node
    case CopyIP:
        text = pod.IP
    }

    return func() tea.Msg {
        if err := m.clipboard.Copy(text); err != nil {
            return statusMsg{
                level:   StatusError,
                message: fmt.Sprintf("Copy failed: %v", err),
            }
        }
        return statusMsg{
            level:   StatusSuccess,
            message: fmt.Sprintf("Copied to clipboard: %s", truncate(text, 50)),
        }
    }
}
```

**Key Principles**:
- Provide multiple copy options (name, YAML, etc.)
- Show visual feedback on successful copy
- Handle clipboard errors gracefully (some terminals don't support clipboard)
- Consider vim-style keybindings: `y` for yank, `Y` for yank full content

**Dependencies**:
- `github.com/atotto/clipboard` - cross-platform clipboard library
- Alternative: shell out to `pbcopy` (macOS), `xclip` (Linux), `clip.exe` (Windows)

---

## 8. Error Handling & Display

### Error Message Pattern

**Pattern**: Show errors in status bar or as overlays depending on severity.

```go
type StatusLevel int

const (
    StatusInfo StatusLevel = iota
    StatusSuccess
    StatusWarning
    StatusError
)

type statusMsg struct {
    level   StatusLevel
    message string
    timeout time.Duration  // Auto-dismiss after this duration
}

type StatusBar struct {
    message    string
    level      StatusLevel
    timestamp  time.Time
    visible    bool
}

func (s *StatusBar) Set(level StatusLevel, message string) tea.Cmd {
    s.message = message
    s.level = level
    s.timestamp = time.Now()
    s.visible = true

    // Auto-dismiss after 3 seconds
    return tea.Tick(3*time.Second, func(t time.Time) tea.Msg {
        return clearStatusMsg{}
    })
}

func (s *StatusBar) View(width int) string {
    if !s.visible {
        return ""
    }

    style := lipgloss.NewStyle().Width(width).Padding(0, 1)

    switch s.level {
    case StatusError:
        style = style.Background(lipgloss.Color("1")).Foreground(lipgloss.Color("15"))
    case StatusSuccess:
        style = style.Background(lipgloss.Color("2")).Foreground(lipgloss.Color("0"))
    case StatusWarning:
        style = style.Background(lipgloss.Color("3")).Foreground(lipgloss.Color("0"))
    default:  // Info
        style = style.Background(lipgloss.Color("4")).Foreground(lipgloss.Color("15"))
    }

    return style.Render(s.message)
}

// In root model
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case statusMsg:
        return m, m.statusBar.Set(msg.level, msg.message)

    case clearStatusMsg:
        m.statusBar.visible = false
        return m, nil
    }
    return m, nil
}
```

### Error Recovery Pattern

```go
// Recoverable errors show in status bar
func (m *model) handleRecoverableError(err error, context string) tea.Cmd {
    return func() tea.Msg {
        return statusMsg{
            level:   StatusError,
            message: fmt.Sprintf("%s: %v", context, err),
        }
    }
}

// Fatal errors show full-screen overlay
type fatalErrorMsg struct {
    err     error
    context string
}

func (m model) renderFatalError() string {
    s := lipgloss.NewStyle().
        Border(lipgloss.RoundedBorder()).
        BorderForeground(lipgloss.Color("1")).
        Padding(1, 2).
        Width(60)

    content := fmt.Sprintf(
        "Fatal Error\n\n%s\n\n%v\n\nPress 'q' to quit",
        m.fatalError.context,
        m.fatalError.err,
    )

    return lipgloss.Place(
        m.width, m.height,
        lipgloss.Center, lipgloss.Center,
        s.Render(content),
    )
}
```

### Error Logging

```go
// Optional: log errors to file for debugging
type ErrorLogger struct {
    file *os.File
}

func NewErrorLogger() (*ErrorLogger, error) {
    logPath := filepath.Join(os.TempDir(), "timoneiro.log")
    f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
    if err != nil {
        return nil, err
    }
    return &ErrorLogger{file: f}, nil
}

func (e *ErrorLogger) Log(format string, args ...interface{}) {
    timestamp := time.Now().Format("2006-01-02 15:04:05")
    msg := fmt.Sprintf(format, args...)
    fmt.Fprintf(e.file, "[%s] %s\n", timestamp, msg)
}

func (e *ErrorLogger) Close() {
    e.file.Close()
}
```

**Key Principles**:
- Minor errors: show in status bar (auto-dismiss)
- Major errors: full-screen overlay (requires user action)
- Always provide context with errors
- Log errors to file for debugging
- Never crash - always allow graceful quit

**References**:
- `/Users/renato/Work/willful/timoneiro/cmd/proto-pods-tui/main.go` (lines 580-586) - error display
- `/tmp/soft-serve/pkg/ui/common/error.go` - error handling utilities

---

## 9. Data Layer Separation

### Repository Pattern

**Pattern**: Separate data fetching from UI logic using repository/service layer.

```go
// Domain models (UI-friendly)
type Pod struct {
    Name      string
    Namespace string
    Ready     string
    Status    string
    Restarts  int
    Age       time.Duration
    Node      string
    IP        string
}

// Repository interface (data layer abstraction)
type PodRepository interface {
    List(namespace string) ([]Pod, error)
    Get(namespace, name string) (*Pod, error)
    Delete(namespace, name string) error
    GetLogs(namespace, name, container string) (io.ReadCloser, error)
    Watch(namespace string) (<-chan PodEvent, error)
}

// Kubernetes implementation
type K8sPodRepository struct {
    clientset kubernetes.Interface
    lister    v1listers.PodLister
    informer  cache.SharedIndexInformer
}

func NewK8sPodRepository(clientset kubernetes.Interface) *K8sPodRepository {
    factory := informers.NewSharedInformerFactory(clientset, 30*time.Second)
    informer := factory.Core().V1().Pods().Informer()
    lister := factory.Core().V1().Pods().Lister()

    return &K8sPodRepository{
        clientset: clientset,
        lister:    lister,
        informer:  informer,
    }
}

func (r *K8sPodRepository) List(namespace string) ([]Pod, error) {
    var podList []*corev1.Pod
    var err error

    if namespace == "" {
        podList, err = r.lister.List(labels.Everything())
    } else {
        podList, err = r.lister.Pods(namespace).List(labels.Everything())
    }

    if err != nil {
        return nil, err
    }

    return r.convertPods(podList), nil
}

func (r *K8sPodRepository) convertPods(k8sPods []*corev1.Pod) []Pod {
    pods := make([]Pod, 0, len(k8sPods))
    now := time.Now()

    for _, p := range k8sPods {
        pods = append(pods, Pod{
            Name:      p.Name,
            Namespace: p.Namespace,
            Ready:     calculateReady(p),
            Status:    string(p.Status.Phase),
            Restarts:  calculateRestarts(p),
            Age:       now.Sub(p.CreationTimestamp.Time),
            Node:      p.Spec.NodeName,
            IP:        p.Status.PodIP,
        })
    }

    return pods
}
```

### Informer Pattern for Live Updates

```go
// Event-driven updates from Kubernetes informers
type PodEvent struct {
    Type EventType  // Added, Modified, Deleted
    Pod  Pod
}

type EventType int

const (
    EventAdded EventType = iota
    EventModified
    EventDeleted
)

func (r *K8sPodRepository) Watch(namespace string) (<-chan PodEvent, error) {
    events := make(chan PodEvent, 100)

    r.informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
        AddFunc: func(obj interface{}) {
            pod := obj.(*corev1.Pod)
            if namespace == "" || pod.Namespace == namespace {
                events <- PodEvent{
                    Type: EventAdded,
                    Pod:  r.convertPod(pod),
                }
            }
        },
        UpdateFunc: func(oldObj, newObj interface{}) {
            pod := newObj.(*corev1.Pod)
            if namespace == "" || pod.Namespace == namespace {
                events <- PodEvent{
                    Type: EventModified,
                    Pod:  r.convertPod(pod),
                }
            }
        },
        DeleteFunc: func(obj interface{}) {
            pod := obj.(*corev1.Pod)
            if namespace == "" || pod.Namespace == namespace {
                events <- PodEvent{
                    Type: EventDeleted,
                    Pod:  r.convertPod(pod),
                }
            }
        },
    })

    return events, nil
}

// In model, subscribe to events
func subscribeToPodsCmd(repo PodRepository, namespace string) tea.Cmd {
    return func() tea.Msg {
        events, err := repo.Watch(namespace)
        if err != nil {
            return errorMsg{err}
        }

        // Start goroutine to forward events as messages
        go func() {
            for event := range events {
                // Send to Bubble Tea program
                program.Send(podEventMsg(event))
            }
        }()

        return podsWatchStartedMsg{}
    }
}
```

### Cache Strategy

```go
// Cache layer on top of repository
type CachedPodRepository struct {
    repo      PodRepository
    cache     map[string][]Pod  // namespace -> pods
    cacheMu   sync.RWMutex
    ttl       time.Duration
    lastFetch map[string]time.Time
}

func (c *CachedPodRepository) List(namespace string) ([]Pod, error) {
    c.cacheMu.RLock()
    cached, ok := c.cache[namespace]
    lastFetch := c.lastFetch[namespace]
    c.cacheMu.RUnlock()

    // Return cached data if fresh
    if ok && time.Since(lastFetch) < c.ttl {
        return cached, nil
    }

    // Fetch fresh data
    pods, err := c.repo.List(namespace)
    if err != nil {
        return nil, err
    }

    // Update cache
    c.cacheMu.Lock()
    c.cache[namespace] = pods
    c.lastFetch[namespace] = time.Now()
    c.cacheMu.Unlock()

    return pods, nil
}
```

**Key Principles**:
- UI never directly accesses Kubernetes API
- Repository provides domain models, not k8s types
- Use informers for live updates (not polling)
- Cache aggressively (informers already cache)
- Repository is easily mockable for testing

**Lessons from proto-k8s-informers**:
- Informers sync all-at-once (not progressive)
- Use protobuf content type for performance
- Informer cache queries are microsecond-fast
- Accept brief initial sync time

**References**:
- `/Users/renato/Work/willful/timoneiro/cmd/proto-pods-tui/main.go` (lines 297-354) - informer usage
- `/tmp/lazygit/pkg/gui/` - sophisticated service layer architecture

---

## 10. Command Palette Implementation

### Command Palette Pattern

**Pattern**: Fuzzy-searchable overlay for all available commands.

```go
type CommandPaletteModel struct {
    commands      []Command
    filtered      []Command
    input         textinput.Model
    selected      int
    width, height int
}

type Command struct {
    ID          string
    Label       string
    Description string
    Keybinding  string
    Action      func() tea.Cmd
    Context     CommandContext  // When this command is available
}

type CommandContext int

const (
    ContextGlobal CommandContext = iota
    ContextPodList
    ContextDetailView
    ContextLogStream
)

func NewCommandPalette(commands []Command) CommandPaletteModel {
    input := textinput.New()
    input.Placeholder = "Type to search commands..."
    input.Focus()

    return CommandPaletteModel{
        commands: commands,
        filtered: commands,
        input:    input,
    }
}

func (m CommandPaletteModel) Update(msg tea.Msg) (CommandPaletteModel, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        switch msg.String() {
        case "esc":
            return m, func() tea.Msg { return closeCommandPaletteMsg{} }

        case "enter":
            if m.selected < len(m.filtered) {
                cmd := m.filtered[m.selected]
                return m, tea.Batch(
                    cmd.Action(),
                    func() tea.Msg { return closeCommandPaletteMsg{} },
                )
            }

        case "up", "ctrl+k":
            if m.selected > 0 {
                m.selected--
            }

        case "down", "ctrl+j":
            if m.selected < len(m.filtered)-1 {
                m.selected++
            }
        }
    }

    // Update search input
    var cmd tea.Cmd
    m.input, cmd = m.input.Update(msg)

    // Filter commands based on input
    m.filterCommands()

    return m, cmd
}

func (m *CommandPaletteModel) filterCommands() {
    query := strings.ToLower(m.input.Value())
    if query == "" {
        m.filtered = m.commands
        return
    }

    // Fuzzy search using sahilm/fuzzy
    searchable := make([]string, len(m.commands))
    for i, cmd := range m.commands {
        searchable[i] = strings.ToLower(cmd.Label + " " + cmd.Description)
    }

    matches := fuzzy.Find(query, searchable)
    m.filtered = make([]Command, 0, len(matches))
    for _, match := range matches {
        m.filtered = append(m.filtered, m.commands[match.Index])
    }

    // Reset selection
    if m.selected >= len(m.filtered) {
        m.selected = 0
    }
}

func (m CommandPaletteModel) View() string {
    var s strings.Builder

    // Title
    s.WriteString(lipgloss.NewStyle().Bold(true).Render("Command Palette"))
    s.WriteString("\n\n")

    // Search input
    s.WriteString(m.input.View())
    s.WriteString("\n\n")

    // Command list
    for i, cmd := range m.filtered {
        cursor := " "
        if i == m.selected {
            cursor = ">"
        }

        line := fmt.Sprintf("%s %s", cursor, cmd.Label)
        if cmd.Keybinding != "" {
            line += fmt.Sprintf(" [%s]", cmd.Keybinding)
        }

        if i == m.selected {
            line = lipgloss.NewStyle().
                Foreground(lipgloss.Color("13")).
                Render(line)
        }

        s.WriteString(line)
        s.WriteString("\n")

        if cmd.Description != "" {
            desc := "  " + cmd.Description
            if i != m.selected {
                desc = lipgloss.NewStyle().
                    Foreground(lipgloss.Color("241")).
                    Render(desc)
            }
            s.WriteString(desc)
            s.WriteString("\n")
        }
    }

    // Help
    s.WriteString("\n")
    s.WriteString(lipgloss.NewStyle().
        Foreground(lipgloss.Color("241")).
        Render("↑/↓: navigate • enter: execute • esc: close"))

    return lipgloss.NewStyle().
        Border(lipgloss.RoundedBorder()).
        BorderForeground(lipgloss.Color("63")).
        Padding(1, 2).
        Width(min(m.width-4, 80)).
        Render(s.String())
}
```

### Command Registration

```go
// Register commands for different contexts
func (m *model) registerCommands() {
    m.commandPalette = NewCommandPalette([]Command{
        // Global commands
        {
            ID:          "quit",
            Label:       "Quit",
            Description: "Exit Timoneiro",
            Keybinding:  "q",
            Context:     ContextGlobal,
            Action:      func() tea.Cmd { return tea.Quit },
        },
        {
            ID:          "help",
            Label:       "Show Help",
            Description: "Display keyboard shortcuts",
            Keybinding:  "?",
            Context:     ContextGlobal,
            Action:      m.showHelpCmd,
        },

        // Pod list commands
        {
            ID:          "delete-pod",
            Label:       "Delete Pod",
            Description: "Delete the selected pod",
            Keybinding:  "d",
            Context:     ContextPodList,
            Action:      m.confirmDeletePodCmd,
        },
        {
            ID:          "describe-pod",
            Label:       "Describe Pod",
            Description: "Show detailed pod information",
            Keybinding:  "enter",
            Context:     ContextPodList,
            Action:      m.showPodDetailsCmd,
        },
        {
            ID:          "logs",
            Label:       "View Logs",
            Description: "Stream pod logs",
            Keybinding:  "l",
            Context:     ContextPodList,
            Action:      m.viewLogsCmd,
        },
        {
            ID:          "edit-yaml",
            Label:       "Edit YAML",
            Description: "Edit pod manifest in $EDITOR",
            Keybinding:  "e",
            Context:     ContextPodList,
            Action:      m.editPodYAMLCmd,
        },
    })
}

// Filter commands by current context
func (m *model) getAvailableCommands() []Command {
    available := []Command{}
    for _, cmd := range m.commandPalette.commands {
        if cmd.Context == ContextGlobal || cmd.Context == m.getCurrentContext() {
            available = append(available, cmd)
        }
    }
    return available
}
```

**Key Principles**:
- Global keybinding to open (e.g., `ctrl+p` or `:`)
- Fuzzy search for discoverability
- Show keybindings alongside commands
- Context-aware (only show relevant commands)
- Extensible command registration system

**References**:
- `/tmp/bubbletea/examples/autocomplete/` - fuzzy search input
- `github.com/sahilm/fuzzy` - used in proto-pods-tui for fuzzy matching

---

## 11. Common Anti-Patterns to Avoid

### 1. Don't Store UI State in Business Logic

**Bad**:
```go
type Pod struct {
    Name      string
    IsSelected bool  // UI state mixed with domain model
    CursorPos  int   // UI state in data model
}
```

**Good**:
```go
// Domain model is pure data
type Pod struct {
    Name      string
    Namespace string
}

// UI state is separate
type PodListUI struct {
    selectedKey string  // Track by key, not by storing in Pod
    cursorPos   int
}
```

### 2. Don't Block in Update Function

**Bad**:
```go
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        if msg.String() == "r" {
            // BLOCKING I/O - freezes UI!
            pods, _ := m.k8sClient.CoreV1().Pods("").List(context.Background(), metav1.ListOptions{})
            m.pods = pods
        }
    }
    return m, nil
}
```

**Good**:
```go
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        if msg.String() == "r" {
            // Return command that does I/O in background
            return m, fetchPodsCmd(m.k8sClient)
        }
    case podsFetchedMsg:
        // Handle result asynchronously
        m.pods = msg.pods
    }
    return m, nil
}
```

### 3. Don't Manually Track Cursor Position Across Data Changes

**Bad**:
```go
func (m *model) refreshPods() {
    m.pods = fetchPods()
    m.table.SetRows(convertToRows(m.pods))
    // Cursor position is lost!
}
```

**Good**:
```go
func (m *model) refreshPods() {
    // Track selected item by key, not index
    oldKey := m.getSelectedKey()

    m.pods = fetchPods()
    m.table.SetRows(convertToRows(m.pods))

    // Restore selection by key
    m.restoreSelectionByKey(oldKey)
}
```

### 4. Don't Hardcode Dimensions

**Bad**:
```go
func (m model) View() string {
    return lipgloss.NewStyle().
        Width(80).    // Hardcoded!
        Height(24).   // Hardcoded!
        Render(content)
}
```

**Good**:
```go
func (m model) View() string {
    return lipgloss.NewStyle().
        Width(m.width).
        Height(m.height).
        Render(content)
}

// Handle resize
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.WindowSizeMsg:
        m.width = msg.Width
        m.height = msg.Height
        m.table.SetHeight(m.calculateTableHeight())
    }
    return m, nil
}
```

### 5. Don't Forget to Batch Commands

**Bad**:
```go
func (m model) Init() tea.Cmd {
    cmd1 := m.spinner.Tick
    cmd2 := m.fetchPodsCmd()
    // Only cmd1 runs! cmd2 is lost
    return cmd1
}
```

**Good**:
```go
func (m model) Init() tea.Cmd {
    return tea.Batch(
        m.spinner.Tick,
        m.fetchPodsCmd(),
        m.tickCmd(),
    )
}
```

### 6. Don't Use Update for View Logic

**Bad**:
```go
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    // Computing view content in Update
    m.renderedTable = m.renderTable()  // Don't pre-render!
    return m, nil
}
```

**Good**:
```go
// Update only changes state
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    m.pods = newPods
    return m, nil
}

// View computes output
func (m model) View() string {
    return m.renderTable()  // Compute on demand
}
```

### 7. Don't Leak Goroutines

**Bad**:
```go
func (m *model) startPolling() {
    go func() {
        for {  // Never stops!
            time.Sleep(1 * time.Second)
            m.program.Send(refreshMsg{})
        }
    }()
}
```

**Good**:
```go
func (m *model) startPolling(ctx context.Context) {
    go func() {
        ticker := time.NewTicker(1 * time.Second)
        defer ticker.Stop()

        for {
            select {
            case <-ctx.Done():
                return  // Graceful shutdown
            case <-ticker.C:
                m.program.Send(refreshMsg{})
            }
        }
    }()
}
```

---

## 12. Recommended Project Structure

```
timoneiro/
├── cmd/
│   └── timoneiro/
│       └── main.go              # Entry point, flag parsing, setup
│
├── internal/
│   ├── app/
│   │   ├── app.go               # Root model and initialization
│   │   ├── navigation.go        # Navigation stack, routing
│   │   └── keybindings.go       # Global keybindings
│   │
│   ├── ui/
│   │   ├── screens/
│   │   │   ├── podlist/
│   │   │   │   ├── model.go     # Pod list screen model
│   │   │   │   ├── update.go    # Update logic
│   │   │   │   ├── view.go      # View rendering
│   │   │   │   └── commands.go  # Commands for this screen
│   │   │   │
│   │   │   ├── deployment/
│   │   │   │   └── ...
│   │   │   │
│   │   │   ├── logs/
│   │   │   │   └── ...
│   │   │   │
│   │   │   └── detail/
│   │   │       └── ...
│   │   │
│   │   ├── components/
│   │   │   ├── statusbar/
│   │   │   │   └── statusbar.go # Reusable status bar
│   │   │   ├── commandpalette/
│   │   │   │   └── palette.go   # Command palette modal
│   │   │   ├── confirm/
│   │   │   │   └── confirm.go   # Confirmation modal
│   │   │   └── help/
│   │   │       └── help.go      # Help overlay
│   │   │
│   │   ├── styles/
│   │   │   └── styles.go        # Global lipgloss styles
│   │   │
│   │   └── common.go            # Shared UI utilities
│   │
│   ├── k8s/
│   │   ├── client.go            # Kubernetes client setup
│   │   ├── repository.go        # Repository interfaces
│   │   ├── pods.go              # Pod repository implementation
│   │   ├── deployments.go       # Deployment repository
│   │   ├── services.go          # Service repository
│   │   └── informer.go          # Informer management
│   │
│   ├── domain/
│   │   ├── models.go            # Domain models (Pod, Deployment, etc.)
│   │   └── converters.go        # k8s types -> domain models
│   │
│   └── utils/
│       ├── clipboard.go         # Clipboard manager
│       ├── editor.go            # External editor integration
│       ├── tempfile.go          # Temp file management
│       └── logger.go            # Error logging
│
├── pkg/                         # Public API (if needed for plugins)
│   └── ...
│
├── CLAUDE.md                    # Development guidelines
├── DESIGN.md                    # This file
├── README.md
├── go.mod
└── go.sum
```

### Key Organization Principles

1. **`cmd/`**: Entry point only, minimal logic
2. **`internal/app/`**: Root model, navigation, global state
3. **`internal/ui/screens/`**: Each screen is a package with model, update, view
4. **`internal/ui/components/`**: Reusable UI components (modals, status bar)
5. **`internal/k8s/`**: All Kubernetes interaction (repositories, informers)
6. **`internal/domain/`**: Pure domain models (no k8s, no UI dependencies)
7. **`internal/utils/`**: Shared utilities (clipboard, editor, logging)

### Screen Package Structure

Each screen follows this structure:

```
podlist/
├── model.go      # State: type PodListModel struct { ... }
├── update.go     # Update logic: func (m PodListModel) Update(...)
├── view.go       # View rendering: func (m PodListModel) View() string
├── commands.go   # Tea commands: func fetchPodsCmd() tea.Cmd
└── filter.go     # Screen-specific logic (optional)
```

### Dependencies Flow

```
cmd/timoneiro → internal/app → internal/ui/screens → internal/k8s
                                      ↓
                                internal/domain
                                      ↓
                                internal/utils
```

- **Downward only**: Packages never import from above
- **UI independent of k8s**: UI only knows about domain models
- **k8s independent of UI**: k8s layer returns domain models

---

## Summary of Key Patterns

| Concern | Pattern | Key Benefit |
|---------|---------|-------------|
| **App Structure** | Single root model with embedded screens | Clear routing, shared state |
| **State Management** | Three layers: app, screen, transient | Separation of concerns |
| **Navigation** | State machine + navigation stack | Easy back/forward navigation |
| **Modals** | Overlay rendering with lipgloss | Non-blocking UI |
| **Async Tasks** | Command pattern with custom messages | Responsive UI, no blocking |
| **External Editor** | tea.ExecProcess for suspend/resume | Standard UNIX workflow |
| **Clipboard** | Clipboard manager service | Cross-platform support |
| **Errors** | Status bar + modal overlays | Contextual error display |
| **Data Layer** | Repository pattern with informers | Testable, cacheable, reactive |
| **Command Palette** | Fuzzy-searchable command list | Discoverability |

---

## Next Steps

1. **Start Simple**: Implement pod list screen first (already prototyped)
2. **Add Navigation**: Implement screen switching (pod list → detail view)
3. **Add Modals**: Implement confirmation dialog for delete operations
4. **Add Background Tasks**: Implement async delete with progress
5. **Add Editor Integration**: Implement YAML editing workflow
6. **Iterate**: Add more resource types (deployments, services, etc.)

---

## References

### Official Bubble Tea Resources
- [Bubble Tea Examples](https://github.com/charmbracelet/bubbletea/tree/master/examples)
- [Bubbles Components](https://github.com/charmbracelet/bubbles)
- [Lipgloss Styling](https://github.com/charmbracelet/lipgloss)

### Real-World Applications Studied
- [Soft Serve](https://github.com/charmbracelet/soft-serve) - Git server TUI
- [Lazygit](https://github.com/jesseduffield/lazygit) - Complex TUI architecture (uses gocui, not Bubble Tea, but patterns apply)
- [Gum](https://github.com/charmbracelet/gum) - CLI tool examples

### Current Timoneiro Prototypes
- `/Users/renato/Work/willful/timoneiro/cmd/proto-pods-tui/main.go` - Pod list with filtering
- `/Users/renato/Work/willful/timoneiro/cmd/proto-k8s-informers/main.go` - Informer exploration
- `/Users/renato/Work/willful/timoneiro/cmd/proto-bubbletea/main.go` - Basic Bubble Tea exploration

### Libraries to Consider
- `github.com/charmbracelet/bubbletea` - TUI framework
- `github.com/charmbracelet/bubbles` - Pre-built components
- `github.com/charmbracelet/lipgloss` - Styling and layout
- `github.com/sahilm/fuzzy` - Fuzzy searching (already in use)
- `github.com/atotto/clipboard` - Cross-platform clipboard
- `k8s.io/client-go` - Kubernetes client (already in use)

---

*This design document is based on research and analysis of Bubble Tea patterns as of October 2025. It represents recommended practices for building Timoneiro but is not prescriptive - adapt patterns as needed for specific use cases.*
