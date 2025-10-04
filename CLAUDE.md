# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Timoneiro is an ultra-fast TUI client for Kubernetes, built with Go and Bubble Tea. The name means "helmsman" (Kubernetes) in Portuguese.

## Development Setup

Go version: 1.24.0+

### Running the Application

```bash
# Run main application with default theme
go run cmd/timoneiro/main.go

# Run with specific theme
go run cmd/timoneiro/main.go -theme dracula
go run cmd/timoneiro/main.go -theme catppuccin

# Build and test (clean up binary after)
go build -o timoneiro cmd/timoneiro/main.go
./timoneiro
rm timoneiro

# Fix dependencies
go mod tidy
```

### Running Prototypes

```bash
# Kubernetes informers with metadata-only mode (blazing fast)
go run cmd/proto-k8s-informers/main.go [--context CONTEXT]

# Bubble Tea TUI exploration
go run cmd/proto-bubbletea/main.go

# Full-featured pod list viewer (main prototype)
go run cmd/proto-pods-tui/main.go [--context CONTEXT]
```

## Key Dependencies

- **Bubble Tea** (github.com/charmbracelet/bubbletea): TUI framework
- **Bubbles** (github.com/charmbracelet/bubbles): Pre-built components (table, list, etc.)
- **Lipgloss** (github.com/charmbracelet/lipgloss): Styling and layout
- **Fuzzy** (github.com/sahilm/fuzzy): Fuzzy search for filtering
- **Overlay** (github.com/rmhubbert/bubbletea-overlay): Modal overlays
- **Kubernetes client-go**: Kubernetes API client
  - k8s.io/client-go/metadata: Metadata-only informers (70-90% faster)
  - k8s.io/client-go/tools/cache: Informer cache implementation

## Architecture

### Project Structure

```
cmd/
  timoneiro/main.go         - Main application entry point
  proto-*/                  - Prototype applications for exploration

internal/
  app/app.go                - Root Bubble Tea model with screen routing
  screens/                  - Screen implementations (pods, deployments, services)
  modals/                   - Modal dialogs (command palette, screen picker)
  components/               - Reusable UI components (header, layout)
  k8s/repository.go         - Kubernetes data access layer
  types/types.go            - Shared types (Screen interface, messages)
  ui/theme.go               - Theme definitions and styling
```

### Key Patterns

1. **Root Model**: `internal/app/app.go` contains the main application model that:
   - Routes messages to current screen
   - Manages global state (window size, filter mode, modals)
   - Handles global keybindings (ctrl+c, ctrl+p, ctrl+s)
   - Coordinates screen switching

2. **Screen Interface**: All screens implement `types.Screen` interface:
   ```go
   type Screen interface {
       tea.Model                    // Init, Update, View
       ID() string                  // Unique screen identifier
       Title() string               // Display title
       HelpText() string            // Help bar text
       Operations() []Operation     // Available commands
   }
   ```

3. **Repository Pattern**: `k8s.Repository` interface abstracts data access:
   - Currently uses `DummyRepository` for development
   - Future: implement live Kubernetes client with informers
   - Screens depend on repository interface, not concrete implementation

4. **Theme System**: `internal/ui/theme.go` defines themes:
   - Themes are structs with color definitions and lipgloss styles
   - Applied to components via factory functions (`ToTableStyles()`)
   - Supports multiple themes: charm (default), dracula, catppuccin
   - Passed to screens at initialization

5. **Modal Overlays**: Modals use `bubbletea-overlay` library:
   - Rendered on top of base screen using overlay compositing
   - Screen Picker (ctrl+s): Switch between screens
   - Command Palette (ctrl+p): Execute screen operations

### Message Flow

- `tea.WindowSizeMsg`: Updates dimensions throughout app
- `types.ScreenSwitchMsg`: Triggers screen change
- `types.RefreshCompleteMsg`: Updates after data refresh
- `types.ErrorMsg`: Displays temporary error message
- `types.ToggleScreenPickerMsg`: Shows/hides screen picker modal
- `types.ToggleCommandPaletteMsg`: Shows/hides command palette modal

## Prototype Learnings (cmd/proto-pods-tui)

### What Works Well

1. **Full Pod Informers with Protobuf** (Not Metadata-Only)
   - Originally planned metadata-only, but needed full pod status (Ready, Status, Restarts, Node, IP)
   - **Trade-off accepted**: Full informers with protobuf encoding still fast enough
   - Protobuf reduces payload size vs JSON: `config.ContentType = "application/vnd.kubernetes.protobuf"`
   - Real-world sync time: ~1-2 seconds for hundreds of pods

2. **Fuzzy Search is Superior**
   - Library: `github.com/sahilm/fuzzy`
   - Much better UX than exact substring matching
   - Search speed: 1-5ms for 100s of pods, 10-50ms for 1000s
   - Automatic ranking by match score (best matches first)
   - Negation still works: `!pattern` excludes fuzzy matches

3. **Bubble Tea Immediate Mode UI**
   - Full-screen mode with `tea.WithAltScreen()`
   - `bubbles/table` component handles most complexity
   - Dynamic column widths based on terminal size
   - Real-time updates from informer cache (1-second refresh)

4. **Smart Cursor Tracking**
   - Track selected pod by `namespace/name` key across filter/sort changes
   - Maintains selection when data updates (avoids jumping cursor)
   - Falls back gracefully when pod disappears

5. **Filter UX Patterns**
   - `/` to enter filter mode (vim-style)
   - Type to filter with live preview
   - Paste support (bracketed paste from terminal)
   - `!` prefix for negation (exclude matches)
   - ESC to clear/cancel
   - Show search timing in header for transparency

### Architecture Decisions

1. **Column Layout** (fixed widths except Name)
   - Namespace: 36 chars (fits UUIDs)
   - Name: Dynamic (fills remaining space, truncates with `...`)
   - Ready: 8 chars
   - Status: 12 chars
   - Restarts: 10 chars
   - Age: 10 chars
   - Node: 40 chars (fits long node names)
   - IP: 15 chars

2. **Sorting Strategy**
   - Primary: Age (newest first) - most common use case
   - Secondary: Name (alphabetical) - stable sort
   - Fuzzy search overrides with match score ranking

3. **Search Scope**
   - Fields: Namespace, Name, Status, Node, IP
   - Combine into single lowercase string for fuzzy matching
   - Pre-lowercase at search time (not at pod creation) to save memory

### Performance Strategy

1. **Kubernetes Informer Caching**
   - **Key Finding**: Informer caches load all-at-once (not progressive)
   - Once synced, cache queries are microsecond-fast (~15-25μs)
   - Real-time updates via watch connections
   - Accept brief initial sync time for instant subsequent queries

2. **Protobuf Encoding**
   - Use `application/vnd.kubernetes.protobuf` content type
   - Reduces network transfer and parsing time vs JSON
   - Transparent to application code (client-go handles it)

3. **Search Performance**
   - Fuzzy search on 100s of pods: 1-5ms (fast enough to run on every keystroke)
   - No debouncing needed for typical cluster sizes
   - Display timing in UI for transparency

4. **UI Rendering**
   - Bubble Tea handles efficient terminal updates
   - Only re-render on state changes (filter, data refresh, resize)
   - Table component optimized for scrolling large lists

### What to Avoid

1. **Metadata-Only Informers for Pod Lists**
   - Too limiting - need Status, Node, IP for useful display
   - Full informers with protobuf are fast enough

2. **Progressive Loading**
   - Informers don't support it (all-or-nothing cache sync)
   - Better to show loading indicator + fast sync than fake progress

3. **Complex String Operations**
   - Custom `toLower`/`contains` slower than stdlib
   - Use `strings.ToLower` and fuzzy library

4. **Table Height/Width Management**
   - Centralize calculation logic (avoid manual adjustments scattered in code)
   - Recalculate on window resize only

### Next Steps for Production

1. **Multi-Resource Support**
   - Pod list works well, replicate pattern for Deployments, Services, etc.
   - Lazy-load informers (only start when user views resource type)

2. **Drill-Down Views**
   - List view with metadata/summary
   - Detail view on selection (fetch full YAML/JSON if needed)
   - Log streaming for pods

3. **Namespace Filtering**
   - Allow watching specific namespaces only (reduces memory)
   - UI to switch namespaces or watch all

4. **Configuration**
   - Save column preferences
   - Default namespace/context
   - Keybindings

5. **Error Handling**
   - Better kubeconfig error messages
   - Handle disconnections gracefully
   - Show informer sync errors in UI

6. **Misc**
   - Compile the code using "go build" but delete the binary after testing
   - Execute go mod tidy to fix dependencies
   - if you need to download repositories, save them into .tmp

## Current Status

The project has moved beyond prototyping into a structured application:

### ✅ Implemented
- Core Bubble Tea application structure with screen routing
- Screen registry system for managing multiple views
- Three screens: Pods, Deployments, Services (using dummy data)
- Modal system: Screen Picker (ctrl+s), Command Palette (ctrl+p)
- Theming system with multiple themes (charm, dracula, catppuccin)
- Global keybindings: filter mode (/), quit (q/ctrl+c)
- Header component with refresh time display
- Layout component for consistent screen structure
- Repository pattern for data access (currently dummy data)

### 🚧 In Progress / To Do
- Live Kubernetes integration (replace DummyRepository)
- Implement screen operations (logs, describe, delete, etc.)
- Fuzzy search filtering (infrastructure exists, needs integration)
- Persistent configuration (~/.config/timoneiro/)
- Additional screens (Namespaces, ConfigMaps, Secrets, etc.)
- Detail view for resources
- Log streaming for pods

### 📚 Reference Documentation
- **DESIGN.md**: Comprehensive Bubble Tea architecture patterns and best practices
- **THEMES.md**: Theming implementation guide and color scheme research
- **CLAUDE.md**: This file - development guidelines and project overview

## Development Guidelines

1. **Build and Clean**: After building with `go build`, always delete the binary
2. **Dependencies**: Run `go mod tidy` after adding/removing imports
3. **External Downloads**: Save external repos to `.tmp/` directory
4. **Screens**: New screens go in `internal/screens/`, implement `types.Screen` interface
5. **Modals**: New modals go in `internal/modals/`, follow existing pattern
6. **Components**: Reusable UI elements go in `internal/components/`
7. **Themes**: Add theme styles to `internal/ui/theme.go`
8. **Messages**: Custom messages go in `internal/types/types.go`

## Quick Reference

### Global Keybindings
- `q` or `ctrl+c`: Quit
- `/`: Enter filter mode
- `esc`: Exit filter mode or close modals
- `ctrl+s`: Toggle screen picker
- `ctrl+p`: Toggle command palette
- `↑/↓`: Navigate lists/tables

### Adding a New Screen
1. Create file in `internal/screens/`
2. Implement `types.Screen` interface
3. Register in `internal/app/app.go` `NewModel()`
4. Add operations to screen's `Operations()` method

### Understanding Bubble Tea in This Project
- **Model**: State container (app state, screen state, UI state)
- **Update**: Message handler that returns new model and commands
- **View**: Renders current state to string
- **Cmd**: Function that returns a message (for async operations)
- **Init**: Returns initial command to run on startup

### Common Lipgloss Patterns
```go
// Create styled text
style := lipgloss.NewStyle().
    Foreground(lipgloss.Color("63")).
    Background(lipgloss.Color("235")).
    Bold(true).
    Padding(1, 2)
styledText := style.Render("Hello")

// Join vertically/horizontally
content := lipgloss.JoinVertical(lipgloss.Left, line1, line2)

// Use theme colors
theme.Primary        // Main accent color
theme.Success       // Green for success states
theme.Error         // Red for error states
```

### Miscellaneous

- When commiting: 
   - Skip stuff like🤖 Generated with [Claude Code](https://claude.com/claude-code) or Co-Authored-By: Claude <noreply@anthropic.com>"
   - Use semanctic commit messages:
      feat: add hat wobble
      ^--^  ^------------^
      |     |
      |     +-> Summary in present tense.
      |
      +-------> Type: chore, docs, feat, fix, refactor, style, or test.
      feat: (new feature for the user, not a new feature for build script)
      fix: (bug fix for the user, not a fix to a build script)
      docs: (changes to the documentation)
      style: (formatting, missing semi colons, etc; no production code change)
      refactor: (refactoring production code, eg. renaming a variable)
      test: (adding missing tests, refactoring tests; no production code change)
      chore: (updating grunt tasks etc; no production code change)
