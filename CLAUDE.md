# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

k1 💨 - The supersonic Kubernetes TUI. Built with Go and Bubble Tea for blazing-fast cluster management at Mach 1 speed.

## Quality Guidelines and Process

**IMPORTANT**: Read `design/PROCESS-IMPROVEMENTS.md` for comprehensive quality guidelines.

### Quality Gates (Mandatory)

**File Size Limits:**
- 500 lines: Warning (consider refactoring)
- 800 lines: STOP - refactoring required before new features
- 150 lines per function: STOP - decompose before continuing

**Test Coverage Requirements:**
- New components: 70% minimum
- Modified components: Cannot decrease coverage
- Critical paths: 80% minimum
- Write tests DURING implementation, not after

**Code Duplication:**
- 3+ repetitions: Extract abstraction/helper function immediately

### Claude Code Commitments

When implementing features, I will:
1. **Flag quality gate violations** as they occur (not after)
2. **Suggest refactoring pauses** when components grow too large
3. **Include quality checks** in every plan (refactoring needs, test coverage targets)
4. **Perform post-feature reviews** and proactively report findings
5. **Be honest about technical debt** instead of hiding it for velocity

### After Every Major Feature

I will automatically perform quality check:
- Check largest file sizes (flag if >500 lines)
- Run `make test-coverage` and report coverage
- Identify new duplication introduced
- Suggest refactoring if needed

**Hold me accountable**: If I don't proactively flag issues, remind me of `design/PROCESS-IMPROVEMENTS.md`.

## Development Setup

Go version: 1.24.0+

### Running the Application

```bash
# Run with live Kubernetes connection (default theme)
go run cmd/k1/main.go

# Run with specific Kubernetes context
go run cmd/k1/main.go -context my-cluster

# Run with custom kubeconfig path
go run cmd/k1/main.go -kubeconfig /path/to/kubeconfig

# Run with specific theme (8 available: charm, dracula, catppuccin, nord, gruvbox, tokyo-night, solarized, monokai)
go run cmd/k1/main.go -theme dracula
go run cmd/k1/main.go -theme nord
go run cmd/k1/main.go -theme gruvbox

# Run with dummy data (no cluster connection)
go run cmd/k1/main.go -dummy

# Build and test (clean up binary after)
go build -o k1 cmd/k1/main.go
./k1
rm k1

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
  k1/main.go                - Main application entry point (binary: k1)

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
   - Supports 8 themes: charm (default), dracula, catppuccin, nord, gruvbox, tokyo-night, solarized, monokai
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

## Current Status

The project has moved beyond prototyping into a structured application:

### ✅ Implemented
- Core Bubble Tea application structure with screen routing
- Screen registry system for managing multiple views
- **Config-driven architecture** with 3-level customization (PLAN-04 complete)
- **11 resource screens**: Pods, Deployments, Services, ConfigMaps, Secrets, Namespaces, StatefulSets, DaemonSets, Jobs, CronJobs, Nodes
- Enhanced Nodes screen with 10 columns (Name, Status, Roles, Hostname, InstanceType, Zone, NodePool, Version, OSImage, Age)
- **Command bar component** with expandable states (Phase 1 complete)
- Filter mode: real-time fuzzy search with negation support
- Suggestion palette: `:` for navigation, `/` for commands
- **Resource-specific commands**: cordon/drain (nodes), endpoints (services), restart (deployments), scale (deployments/statefulsets)
- **Resource detail commands**: /yaml (kubectl YAMLPrinter), /describe (simplified format with on-demand events)
- Shortcuts: ctrl+y (yaml), ctrl+d (describe)
- On-demand event fetching for describe (zero memory overhead, 50-100ms latency)
- Theming system with 8 themes (charm, dracula, catppuccin, nord, gruvbox, tokyo-night, solarized, monokai)
- Global keybindings: quit (q/ctrl+c)
- Header component with refresh time display
- Layout component with dynamic body height calculation
- Repository pattern with both dummy and live Kubernetes data sources
- Live Kubernetes integration via informers (Pods only, with protobuf)
- Dynamic client with unstructured resources for all 11 resource types
- Command-line flags: -kubeconfig, -context, -theme, -dummy
- **Comprehensive test suite** with envtest (shared TestMain, namespace isolation, table-driven tests)
- **Test coverage**: 76.7% (k8s), 71.0% (screens)
- **Makefile** with test/build/run targets

### 🚧 In Progress / To Do
- Command registry and palette filtering (Phase 2)
- Navigation commands (:pods, :deployments, :services) (Phase 3)
- Resource commands (/delete)
- Command history (Phase 5)
- Real-time updates (1-second refresh ticker)
- Live informers for Deployments and Services
- Persistent configuration (~/.config/k1/)
- Additional screens (Namespaces, ConfigMaps, Secrets, etc.)
- Detail view for resources
- Log streaming for pods

### 📚 Reference Documentation
- **design/DDR-01.md**: Bubble Tea architecture patterns and best practices
- **design/DDR-02.md**: Theming system implementation and styling guidelines
- **design/DDR-03.md**: Kubernetes informer-based repository design
- **design/DDR-04.md**: Testing strategy with envtest (shared TestMain pattern)
- **design/DDR-05.md**: Command-enhanced list browser UI/UX design
- **design/DDR-06.md**: Describe and YAML commands implementation (on-demand events)
- **design/DDR-07.md**: Scalable multi-resource architecture with config-driven design
- **design/DDR-08.md**: Pragmatic command implementation strategy (kubectl subprocess vs pure Go)
- **plans/PLAN-03.md**: Command-enhanced UI implementation plan (Phase 1 complete)
- **plans/PLAN-04.md**: Config-driven multi-resource architecture (All phases complete)
- **plans/PLAN-05.md**: YAML and Describe commands implementation (Complete)
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

2. **Testing and Commits**: NEVER commit code without user testing first
   - After implementing features, build and wait for user to test
   - User will verify functionality works as expected
   - Only create commits AFTER user confirms testing is complete
   - If user finds issues during testing, fix them before committing
   - **CRITICAL**: Do NOT add "🤖 Generated with Claude Code" or "Co-Authored-By: Claude" signatures to commits
   - Example workflow:
     ```bash
     # After implementation:
     go build -o k1 cmd/k1/main.go && rm k1  # Build to verify compilation
     git add -A && git status                 # Stage changes and show status
     # WAIT for user to test
     # User confirms: "tests passed, commit it"
     git commit -m "feat: your commit message"
     # NO signatures at the end!
     ```

3. **Prefer Makefile**: Always use Makefile targets when available (e.g., `make test`, `make build`, `make run`)
4. **Build and Clean**: After building with `go build`, always delete the binary (or use `make build` + `make clean`)
5. **Dependencies**: Run `go mod tidy` after adding/removing imports
6. **External Downloads**: Save external repos to `.tmp/` directory
7. **Screens**: New screens go in `internal/screens/`, implement `types.Screen` interface
8. **Modals**: New modals go in `internal/modals/`, follow existing pattern
9. **Components**: Reusable UI elements go in `internal/components/`
10. **Themes**: Add theme styles to `internal/ui/theme.go`
11. **Messages**: Custom messages go in `internal/types/types.go`
12. **Testing**: Use envtest with shared TestMain, create unique namespaces per test, use `testify/assert` for assertions
13. **Table-Driven Tests**: Prefer table-driven tests for multiple test cases (only skip when complexity is very high)

## Code Patterns and Conventions

### Constants Organization

**Pattern**: Use per-package constants to avoid circular dependencies.

**DO**:
```go
// internal/components/constants.go
package components

const (
    MaxPaletteItems = 8
    FullScreenReservedLines = 3
)
```

**DON'T**:
```go
// internal/constants/constants.go - AVOID central constants package
package constants

const MaxPaletteItems = 8  // Creates import cycles
```

**Rationale**: Central constants package creates circular dependencies when packages need to import each other. Per-package constants keep dependencies clean.

**Existing constant files**:
- `internal/components/constants.go` - UI constants
- `internal/k8s/constants.go` - Kubernetes client constants
- `internal/commands/constants.go` - Command execution constants
- `internal/screens/constants.go` - Screen configuration constants

### Message Helpers for Commands

**Pattern**: Use message helpers from `internal/messages` for consistent command responses.

**Command layer pattern**:
```go
import "github.com/renato0307/k1/internal/messages"

func ScaleCommand(repo k8s.Repository) ExecuteFunc {
    return func(ctx CommandContext) tea.Cmd {
        if err := validateArgs(); err != nil {
            return messages.ErrorCmd("Invalid args: %v", err)
        }

        // ... execute operation ...

        if err != nil {
            return messages.ErrorCmd("Scale failed: %v", err)
        }
        return messages.SuccessCmd("Scaled %s to %d replicas", name, count)
    }
}
```

**Available helpers**:
- `messages.ErrorCmd(format, args...)` - Red error message
- `messages.SuccessCmd(format, args...)` - Green success message
- `messages.InfoCmd(format, args...)` - Blue info message
- `messages.WrapError(err, format, args...)` - Wrap errors with context (repository layer)

**Repository layer pattern**:
```go
func (r *Repository) GetPods() ([]Pod, error) {
    pods, err := r.lister.List()
    if err != nil {
        return nil, fmt.Errorf("failed to list pods: %w", err)
    }
    return pods, nil
}
```

See `internal/messages/doc.go` for complete patterns and guidelines.

### Helper Function Philosophy

**Only create helpers that reduce boilerplate**. Avoid unnecessary abstractions.

**Good helpers** (reduce repetitive code):
- `messages.ErrorCmd()` - Wraps `tea.Cmd` + `types.ErrorStatusMsg` boilerplate
- `messages.WrapError()` - Makes error wrapping intent explicit

**Bad helpers** (unnecessary aliases):
- `NewError()` - Just use `fmt.Errorf()` directly (everyone knows it)
- `StringContains()` - Just use `strings.Contains()` directly

**Rule of thumb**: If the helper is just calling one standard library function, it's probably not worth it.

### Go Idioms

**Use modern Go types**:
```go
// DO (Go 1.18+)
func Format(format string, args ...any) string

// DON'T (outdated)
func Format(format string, args ...interface{}) string
```

**Prefer standard library over custom implementations**:
- Use `strings.ToLower()` not custom `toLower()`
- Use `fmt.Errorf()` not custom error builders
- Use `strconv.Itoa()` not custom number formatters

### Performance Optimization Patterns

**Extract common operations to the caller, not the callee**:

When multiple functions perform the same expensive operation on the same data,
move that operation to the caller and pass the result as a parameter.

**Example from transforms.go**:
```go
// BEFORE (inefficient):
// Each transform function extracts common fields independently
func transformPod(u *unstructured.Unstructured) (any, error) {
    common := extractCommonFields(u)  // Called 11 times per resource!
    // ... use common fields
}

// AFTER (optimized):
// Caller extracts once, passes to all transforms
func GetResources(resourceType ResourceType) ([]any, error) {
    for _, obj := range objList {
        common := extractCommonFields(unstr)  // Called once per resource
        transformed, err := config.Transform(unstr, common)
    }
}

// Transform function signature changed:
type TransformFunc func(*unstructured.Unstructured, commonFields) (any, error)
```

**Performance impact**: Reduces O(11n) to O(n) for field extraction on large clusters.

**When to apply this pattern**:
- Multiple functions need the same derived data
- The extraction is non-trivial (reflection, parsing, nested field access)
- The operation is called frequently (hot path)

**When NOT to apply**:
- The extraction is trivial (single field access)
- Functions need different subsets of the data
- The pattern increases coupling unnecessarily

**Why not use reflection for transforms?**:
- Reflection is 10-100x slower than direct field access
- Critical for large clusters (1000+ resources on every list operation)
- Explicit code is easier to debug and maintains type safety
- Common field extraction already eliminates most duplication

### Table-Driven Pattern for Reducing Duplication

**Use data structures instead of nearly-identical functions**:

When you have multiple functions that differ only in data values, replace them
with a single function and a data structure (map, slice, struct).

**Example from navigation.go**:
```go
// BEFORE (11 nearly-identical functions):
func PodsCommand() ExecuteFunc {
    return func(ctx CommandContext) tea.Cmd {
        return func() tea.Msg {
            return types.ScreenSwitchMsg{ScreenID: "pods"}
        }
    }
}
func DeploymentsCommand() ExecuteFunc { /* same but screenID: "deployments" */ }
// ... 9 more identical functions

// AFTER (single function + data registry):
var navigationRegistry = map[string]string{
    "pods":        "pods",
    "deployments": "deployments",
    // ... 9 more entries
}

func NavigationCommand(screenID string) ExecuteFunc {
    return func(ctx CommandContext) tea.Cmd {
        return func() tea.Msg {
            return types.ScreenSwitchMsg{ScreenID: screenID}
        }
    }
}

// Legacy functions now just delegate:
func PodsCommand() ExecuteFunc { return NavigationCommand("pods") }
```

**Benefits**:
- Eliminates ~30 lines of boilerplate per pattern
- Changes to behavior require updating one function, not 11
- New entries require one line of data, not 8 lines of code
- Easier to test (test one function with table-driven tests)

**When to apply**:
- 3+ functions with identical structure but different constants
- The only difference is data values (strings, numbers, etc.)
- No complex conditional logic per case

**When NOT to apply**:
- Functions have genuinely different logic
- Each case needs custom error handling or validation
- The abstraction makes the code harder to understand

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
- Keep claude authoring stuff of of generated code or commit messages
- Keep track of golang patterns or approaches we use
- Each time we do changes, please review the README.md to ensure we keep it updated
- don't forget go mod tidy