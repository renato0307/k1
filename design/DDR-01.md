# Bubble Tea Architecture Patterns

| Metadata | Value                       |
|----------|-----------------------------|
| Date     | 2025-10-04                  |
| Author   | @renato0307                 |
| Status   | `Implemented`               |
| Tags     | architecture, bubble-tea    |
| Updates  | -                           |

| Revision | Date       | Author      | Info                         |
|----------|------------|-------------|------------------------------|
| 1        | 2025-10-04 | @renato0307 | Initial architectural design |

## Context and Problem Statement

K1 is an ultra-fast TUI client for Kubernetes built with Go and Bubble Tea. The project requires a clear architectural pattern for building a moderately complex TUI application with multiple screens, modals, live data updates, and external integrations (clipboard, editor). How should we structure the application to maintain separation of concerns, enable testability, and provide a responsive user experience?

## References

Analysis based on:
- Official Bubble Tea examples (github.com/charmbracelet/bubbletea/tree/master/examples)
- Real-world applications: Soft Serve (Git server TUI), Lazygit (complex TUI architecture)
- Existing K1 prototypes (proto-pods-tui, proto-k8s-informers, proto-bubbletea)
- Bubble Tea ecosystem: Bubbles components, Lipgloss styling

## Design

### Core Architectural Patterns

#### 1. Single Root Model with Embedded Component Models

**Pattern**: Use a single root model that coordinates all application state and embeds specialized component models for different screens/views.

```go
type model struct {
    // Application-level state
    width, height    int
    currentScreen    screenState

    // Embedded screen models
    podListScreen    *PodListModel
    deploymentScreen *DeploymentListModel

    // Shared services (injected)
    k8sClient        *KubernetesClient

    // Common UI components
    help             help.Model
    statusBar        StatusBarModel
}
```

**Key Principles**:
- Root model handles routing between screens
- Each screen is a separate sub-model with its own Update/View logic
- Shared state and services live at the root level
- Screen models are initialized lazily (only when first accessed)

#### 2. Layered State Architecture

Separate state into three layers:

1. **Application State** (global, persistent across screens)
   - Current Kubernetes context, namespaces, theme, keybindings

2. **Screen State** (local to a specific view, reset on navigation)
   - Data to display, filtered data, filter text, selection tracking

3. **Transient State** (UI state like cursor position, selections)
   - Managed by Bubbles components (table, viewport)

**Selection Tracking Pattern**: Track selection by key (namespace/name), not by index, to maintain cursor position across data updates.

#### 3. State Machine Navigation

Use explicit state machine with typed screen constants:

```go
type Screen int

const (
    ScreenResourceList Screen = iota
    ScreenDetailView
    ScreenLogStream
    ScreenCommandPalette  // Modal overlay
)
```

Navigation stack enables back/forward navigation. Global keys (quit, help, command palette) handled at root level, screen-specific keys delegated to screen models.

#### 4. Modal & Overlay Pattern

Modals are special screen states that render on top of the current screen using lipgloss overlays:

- Modals block input to underlying screen
- Use lipgloss `Place` and `PlaceOver` for visual layering
- Modal data is passed in, not stored globally
- Escape key always cancels modal

#### 5. Message-Based Async Pattern

All I/O happens in commands (functions returning `tea.Msg`):

```go
type podsFetchedMsg struct {
    pods []Pod
    err  error
}

func fetchPodsCmd(lister v1listers.PodLister) tea.Cmd {
    return func() tea.Msg {
        pods, err := lister.List(labels.Everything())
        if err != nil {
            return podsFetchedMsg{err: err}
        }
        return podsFetchedMsg{pods: convertPods(pods)}
    }
}
```

Long-running operations return immediately with loading state. Show spinners/progress for user feedback.

#### 6. Repository Pattern for Data Access

Separate data fetching from UI logic using repository/service layer:

```go
type PodRepository interface {
    List(namespace string) ([]Pod, error)
    Get(namespace, name string) (*Pod, error)
    Delete(namespace, name string) error
    Watch(namespace string) (<-chan PodEvent, error)
}
```

UI never directly accesses Kubernetes API. Repository provides domain models, not k8s types. Use informers for live updates (not polling).

#### 7. External Integrations

- **Editor**: Use `tea.ExecProcess` to suspend TUI and launch external editor (respects `$EDITOR`)
- **Clipboard**: Use `atotto/clipboard` or OS-specific commands for cross-platform clipboard access
- **Temp Files**: Clean up on exit

### Project Structure

```
k1/
├── cmd/k1/                 # Entry point only
├── internal/
│   ├── app/                # Root model, navigation, global state
│   ├── screens/            # Screen implementations (pods, deployments, etc.)
│   ├── modals/             # Modal dialogs (command palette, screen picker)
│   ├── components/         # Reusable UI components (header, layout)
│   ├── k8s/                # Data access layer (repositories, informers)
│   ├── types/              # Shared types (Screen interface, messages)
│   └── ui/                 # Theme definitions and styling
```

**Dependencies Flow**: Downward only (packages never import from above), UI independent of k8s, k8s independent of UI.

### Key Anti-Patterns to Avoid

1. Don't store UI state in business logic (domain models should be pure data)
2. Don't block in Update function (use commands for I/O)
3. Don't manually track cursor position by index (use key-based tracking)
4. Don't hardcode dimensions (use window size from messages)
5. Don't forget to batch commands (use `tea.Batch`)
6. Don't use Update for view logic (compute on demand in View)
7. Don't leak goroutines (use context for cancellation)

## Decision

Adopt these Bubble Tea architecture patterns as the foundation for K1:

1. **Single root model** with embedded screen models for routing and shared state
2. **Three-layer state architecture** (app, screen, transient)
3. **State machine navigation** with navigation stack
4. **Modal overlays** using lipgloss composition
5. **Message-based async** for all I/O operations
6. **Repository pattern** for data access abstraction
7. **Screen interface** contract for all views

All screens will implement the `types.Screen` interface:
```go
type Screen interface {
    tea.Model                    // Init, Update, View
    ID() string                  // Unique screen identifier
    Title() string               // Display title
    HelpText() string            // Help bar text
    Operations() []Operation     // Available commands
}
```

## Consequences

### Positive

1. **Clear Separation of Concerns**: UI, business logic, and data access are cleanly separated
2. **Testability**: Repository pattern enables easy mocking; screens are independently testable
3. **Maintainability**: Consistent patterns make codebase predictable and easier to navigate
4. **Responsive UI**: Message-based async ensures UI never blocks on I/O
5. **Extensibility**: Adding new screens follows established pattern; easy to extend
6. **Reusability**: Components and utilities can be shared across screens

### Negative

1. **Learning Curve**: Team members need to understand Bubble Tea's message-passing model
2. **Boilerplate**: Each screen requires model, update, view, and commands files
3. **State Management Complexity**: Three layers of state require discipline to maintain correctly
4. **Message Explosion**: Many custom message types as app grows

### Mitigations

- Comprehensive documentation (CLAUDE.md, this DDR) for onboarding
- Screen template/generator to reduce boilerplate
- Clear naming conventions for state variables and message types
- Regular refactoring to consolidate similar message handlers

## Implementation Status

### ✅ Implemented
- Core Bubble Tea application structure with screen routing
- Screen registry system for managing multiple views
- Three screens: Pods, Deployments, Services (using dummy data)
- Modal system: Screen Picker (ctrl+s), Command Palette (ctrl+p)
- Global keybindings: filter mode (/), quit (q/ctrl+c)
- Header and layout components
- Repository pattern (currently dummy data)

### 🚧 To Do
- Live Kubernetes integration (replace DummyRepository)
- Implement screen operations (logs, describe, delete)
- Fuzzy search filtering integration
- Additional screens (Namespaces, ConfigMaps, Secrets)
- Detail view for resources
- Log streaming for pods
