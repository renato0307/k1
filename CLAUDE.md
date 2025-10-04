# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Timoneiro is an ultra-fast TUI client for Kubernetes, built with Go and Bubble Tea. The name means "helmsman" (Kubernetes) in Portuguese.

## Development Setup

Go version: 1.24.0+

### Running the Application

```bash
# Run with live Kubernetes connection (default theme)
go run cmd/timoneiro/main.go

# Run with specific Kubernetes context
go run cmd/timoneiro/main.go -context my-cluster

# Run with custom kubeconfig path
go run cmd/timoneiro/main.go -kubeconfig /path/to/kubeconfig

# Run with specific theme
go run cmd/timoneiro/main.go -theme dracula
go run cmd/timoneiro/main.go -theme catppuccin

# Run with dummy data (no cluster connection)
go run cmd/timoneiro/main.go -dummy

# Build and test (clean up binary after)
go build -o timoneiro cmd/timoneiro/main.go
./timoneiro
rm timoneiro

# Fix dependencies
go mod tidy
```

### Running Tests

**IMPORTANT**: Always use Makefile targets for testing:

```bash
# One-time setup: Install envtest binaries
make setup-envtest

# Run all tests (preferred method)
make test

# Run tests with coverage report
make test-coverage

# View coverage in browser
make test-coverage-html

# Build the application
make build

# Clean build artifacts
make clean

# Run with dummy data
make run-dummy

# Run with live cluster
make run
```

Manual commands (only use when necessary):
```bash
# Manually run tests (if Makefile is not available)
export KUBEBUILDER_ASSETS=$(setup-envtest use -p path)
go test -v ./... -timeout 60s
```

**Testing Strategy:**
- Tests use [envtest](https://book.kubebuilder.io/reference/envtest.html) which runs a real Kubernetes API server locally
- **Shared TestMain pattern**: envtest starts once per test suite (~5s), not per test
- **Namespace isolation**: Each test creates a unique `test-*` namespace to prevent conflicts
- **Table-driven tests** with `testify/assert` for cleaner assertions
- First run downloads Kubernetes binaries (~50MB, then cached)
- Test suite runs in ~5-10 seconds total

See `design/DDR-04.md` for detailed testing architecture.

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
  components/               - Reusable UI components (header, layout, commandbar)
  k8s/repository.go         - Kubernetes data access layer
  types/types.go            - Shared types (Screen interface, messages)
  ui/theme.go               - Theme definitions and styling
```

### Key Patterns

1. **Root Model**: `internal/app/app.go` contains the main application model that:
   - Routes messages to current screen and command bar
   - Manages global state (window size, layout dimensions)
   - Handles global keybindings (ctrl+c, q)
   - Coordinates screen switching and dynamic body height calculations

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

5. **Command Bar**: `internal/components/commandbar.go` provides expandable bottom UI:
   - State machine: Hidden, Filter, SuggestionPalette, Input, Confirmation, LLMPreview, Result
   - Filter mode: just start typing to filter current list with fuzzy search
   - Palette mode: `:` for navigation, `/` for commands
   - Dynamic height calculation with proper body coordination
   - Arrow keys navigate palette when active, list otherwise

### Message Flow

- `tea.WindowSizeMsg`: Updates dimensions throughout app
- `types.ScreenSwitchMsg`: Triggers screen change
- `types.RefreshCompleteMsg`: Updates after data refresh
- `types.ErrorMsg`: Displays temporary error message
- `types.FilterUpdateMsg`: Updates filter on current screen (from command bar)
- `types.ClearFilterMsg`: Clears filter on current screen

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
- Three screens: Pods, Deployments, Services
- **Command bar component** with expandable states (Phase 1 complete)
- Filter mode: real-time fuzzy search with negation support
- Suggestion palette: `:` for navigation, `/` for commands
- Theming system with multiple themes (charm, dracula, catppuccin)
- Global keybindings: quit (q/ctrl+c)
- Header component with refresh time display
- Layout component with dynamic body height calculation
- Repository pattern with both dummy and live Kubernetes data sources
- Live Kubernetes integration via informers (Pods only, with protobuf)
- Command-line flags: -kubeconfig, -context, -theme, -dummy
- **Comprehensive test suite** with envtest (shared TestMain, namespace isolation)
- **Makefile** with test/build/run targets

### 🚧 In Progress / To Do
- Command registry and palette filtering (Phase 2)
- Navigation commands (:pods, :deployments, :services) (Phase 3)
- Resource commands (/yaml, /describe, /delete) (Phase 3)
- Full-screen views for YAML/logs (Phase 4)
- Command history (Phase 5)
- Real-time updates (1-second refresh ticker)
- Live informers for Deployments and Services
- Persistent configuration (~/.config/timoneiro/)
- Additional screens (Namespaces, ConfigMaps, Secrets, etc.)
- Detail view for resources
- Log streaming for pods

### 📚 Reference Documentation
- **design/DDR-01.md**: Bubble Tea architecture patterns and best practices
- **design/DDR-02.md**: Theming system implementation and styling guidelines
- **design/DDR-03.md**: Kubernetes informer-based repository design
- **design/DDR-04.md**: Testing strategy with envtest (shared TestMain pattern)
- **design/DDR-05.md**: Command-enhanced list browser UI/UX design
- **plans/PLAN-03.md**: Command-enhanced UI implementation plan (Phase 1 complete)
- **CLAUDE.md**: This file - development guidelines and project overview

## Development Guidelines

1. **Git Workflow**: ALWAYS create a new branch from main before starting work on a new plan or feature
   ```bash
   git checkout main
   git pull
   git checkout -b feat/plan-XX-short-description
   ```
   **If you realize you're on the wrong branch:**

   **Option A: Stash (simpler, preferred):**
   ```bash
   # NEVER use git reset --hard with uncommitted work!
   # Save work with stash:
   git stash

   # Switch to correct branch:
   git checkout main
   git pull
   git checkout -b feat/correct-branch-name

   # Apply stashed changes:
   git stash pop  # May have conflicts - resolve them manually

   # If conflicts occur, resolve them and continue:
   git add .
   # Then continue implementation
   ```

   **Option B: Commit and cherry-pick (when stash conflicts are complex):**
   ```bash
   # Commit on wrong branch:
   git add .
   git commit -m "temp: implementation on wrong branch"

   # Create correct branch and cherry-pick:
   git checkout main
   git pull
   git checkout -b feat/correct-branch-name
   git cherry-pick <commit-hash>
   ```
2. **Prefer Makefile**: Always use Makefile targets when available (e.g., `make test`, `make build`, `make run`)
3. **Build and Clean**: After building with `go build`, always delete the binary (or use `make build` + `make clean`)
4. **Dependencies**: Run `go mod tidy` after adding/removing imports
5. **External Downloads**: Save external repos to `.tmp/` directory
6. **Screens**: New screens go in `internal/screens/`, implement `types.Screen` interface
7. **Modals**: New modals go in `internal/modals/`, follow existing pattern
8. **Components**: Reusable UI elements go in `internal/components/`
9. **Themes**: Add theme styles to `internal/ui/theme.go`
10. **Messages**: Custom messages go in `internal/types/types.go`
11. **Testing**: Use envtest with shared TestMain, create unique namespaces per test, use `testify/assert` for assertions

## Quick Reference

### Global Keybindings
- `q` or `ctrl+c`: Quit
- **Type any character**: Enter filter mode (fuzzy search with negation support)
- `:`: Open navigation palette (screens, namespaces)
- `/`: Open command palette (resource operations, includes `/ai` for AI commands)
- `/ai`: Natural language AI commands (type `/ai ` followed by prompt)
- `esc`: Exit filter mode or dismiss palette
- `↑/↓`: Navigate lists (when filter active) or palette items (when palette active)
- `enter`: Apply filter or execute selected command

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

### Commit Message Format

Use semantic commit messages:
```
feat: add hat wobble
^--^  ^------------^
|     |
|     +-> Summary in present tense.
|
+-------> Type: chore, docs, feat, fix, refactor, style, or test.
```

**Types:**
- `feat`: New feature for the user
- `fix`: Bug fix for the user
- `docs`: Documentation changes
- `style`: Formatting, missing semi colons (no production code change)
- `refactor`: Refactoring production code (e.g., renaming a variable)
- `test`: Adding missing tests, refactoring tests (no production code change)
- `chore`: Updating build tasks, etc. (no production code change)

**Note:** Skip generated signatures like "🤖 Generated with Claude Code" or "Co-Authored-By: Claude"

## Design Documents

Store design decisions in `design/` folder:
- Follow the `design/TEMPLATE.md` structure
- Create files incrementally named `DDR-XX.md` (Design Decision Record)
- Update `design/README.md` index table with new entries
- See existing examples: DDR-01 (Architecture), DDR-02 (Theming)
- The author should not be @claude and by default should be @renato0307
- Designs should be formated to less than 80 characters per line
- Designs should not include implementations plans

## Implementation plan documents

- Store implementation plans in `plans/` folder
- Implementation plans should be named `PLAN-XX-YYYYMMDD-<short-description>.md`
- Plans should be **high-level and strategic**, not detailed step-by-step instructions
- Focus on:
  - Overall goals and outcomes
  - Major phases or milestones (3-7 key steps maximum)
  - Critical architectural/design decisions
  - Key risks or considerations
  - Success criteria
  - TODO list with phase-level checkboxes for progress tracking (ALWAYS INCLUDE A TODO LIST)
- Avoid:
  - Line-by-line code changes
  - Exhaustive file-by-file checklists
  - Over-specifying implementation details
  - Micro-tasks that restrict adaptation
- Plans should be reviewable in 2-3 minutes
- Leave room for discovery and adaptation during implementation
- **Progress Tracking**:
  - Update the plan's TODO section after completing significant work (phase completion, major features)
  - Mark items as complete `[x]` when done
  - Add new items discovered during implementation
  - DO NOT use TodoWrite tool - track progress directly in the plan markdown file
  - Update plan status at top of file to reflect current phase
- during this prototype phase, please don't run tests, not needed
- keep claude authoring stuff of of generated code or commit messages