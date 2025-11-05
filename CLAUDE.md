# CLAUDE.md

Development guidelines for Claude Code when working with k1 - the supersonic Kubernetes TUI.

**For user documentation**, see [README.md](README.md).

## Quality Guidelines

**IMPORTANT**: Read `design/PROCESS-IMPROVEMENTS.md` for comprehensive quality guidelines.

### Quality Gates (Mandatory)

- **File Size**: 500 lines warning, 800 lines STOP, 150 lines/function STOP
- **Test Coverage**: New components 70% min, critical paths 80% min, cannot decrease existing
- **Code Duplication**: 3+ repetitions → extract immediately

### Claude Code Commitments

1. Flag quality gate violations as they occur
2. Suggest refactoring pauses when components grow too large
3. Include quality checks in every plan
4. Perform post-feature reviews and proactively report findings
5. Be honest about technical debt

### After Every Major Feature

- Check largest file sizes (flag if >500 lines)
- Run `make test-coverage` and report coverage
- Identify new duplication introduced
- Suggest refactoring if needed

## Development Setup

Go version: 1.24.0+

### Key Commands

```bash
# Build and test
make build              # Build the application
make test              # Run all tests
make test-coverage     # Generate coverage report
make run               # Run with live cluster
make run-dummy         # Run with mock data

# Quick verification
go build -o k1 cmd/k1/main.go && ./k1 && rm k1
go run cmd/k1/main.go -dummy  # UI dev without cluster
```

### Testing Strategy

- **envtest**: Real K8s API server locally (~5s startup, then cached)
- **Shared TestMain**: Start envtest once per suite, not per test
- **Namespace isolation**: Each test uses unique `test-*` namespace
- **Table-driven tests** with `testify/assert`
- Test suite runs in ~5-10 seconds total

## Performance Architecture

- **Informers**: Client-side caching, 1-2s initial sync, microsecond queries
- **Protobuf**: 60-70% size reduction vs JSON
- **Metadata-only**: 70-90% faster for list views, fetch full on-demand
- **Unstructured**: Dynamic client, no typed imports needed

## Key Dependencies

- **Bubble Tea**: TUI framework
- **Bubbles**: Pre-built components (table, list)
- **Lipgloss**: Styling and layout
- **Fuzzy**: Fuzzy search for filtering
- **Overlay**: Modal overlays
- **client-go**: K8s API (metadata informers, cache)

## Architecture

### Project Structure

```
cmd/k1/main.go                 # Binary entry point
internal/
  app/app.go                   # Root model with screen routing
  screens/                     # Screen implementations
  components/                  # Reusable UI components
  k8s/repository.go            # Data access layer
  types/types.go               # Shared types (Screen interface, messages)
  ui/theme.go                  # Theme definitions
```

### Core Patterns

1. **Root Model** (`app.go`): Routes messages, manages state, handles global keys
2. **Screen Interface**: All screens implement `types.Screen` (Init/Update/View, ID, Title, HelpText, Operations)
3. **Repository Pattern**: Abstracts data access (DummyRepository for dev, InformerRepository for live)
4. **Theme System**: 8 themes (charm default, dracula, catppuccin, nord, gruvbox, tokyo-night, solarized, monokai)
5. **Command Bar**: State machine (Hidden/Filter/Palette/Input/Confirmation/LLMPreview/Result)

### Key Messages

- `tea.WindowSizeMsg`: Update dimensions
- `types.ScreenSwitchMsg`: Change screen
- `types.RefreshCompleteMsg`: Data updated
- `types.ErrorMsg`: Show error
- `types.FilterUpdateMsg`/`ClearFilterMsg`: Filter operations

## Implementation Status

**✅ Complete**: Bubble Tea app, 11 resource types (Pods, Deployments, Services, ConfigMaps, Secrets, Namespaces, StatefulSets, DaemonSets, Jobs, CronJobs, Nodes), command bar, themes, repository with informers, 76.7% k8s / 71.0% screens coverage

**🚧 Planned**: Persistent config, resource editing, log streaming, enhanced AI commands, command history

## Development Guidelines

### Git Workflow

**ALWAYS** create new branch from main before starting work:
```bash
git checkout main && git pull && git checkout -b feat/plan-XX-description
```

**If on wrong branch**:
```bash
# Option A (preferred): Stash
git stash
git checkout main && git pull && git checkout -b feat/correct-name
git stash pop

# Option B: Cherry-pick
git add . && git commit -m "temp: work on wrong branch"
git checkout main && git pull && git checkout -b feat/correct-name
git cherry-pick <commit-hash>
```

### Testing and Commits

**NEVER commit without user testing first**
- Build and wait for user to test: `go build -o k1 cmd/k1/main.go && rm k1`
- User confirms: "tests passed, commit it"
- **CRITICAL**: Do NOT add "🤖 Generated with Claude Code" or "Co-Authored-By: Claude" signatures

### General Rules

3. Prefer Makefile targets (`make test`, `make build`)
4. Delete binary after `go build` (or use `make clean`)
5. Run `go mod tidy` after imports change
6. External downloads → `.tmp/` directory
7. New screens → `internal/screens/`, implement `types.Screen`
8. New modals → `internal/modals/`
9. Components → `internal/components/`
10. Custom messages → `internal/types/types.go`
11. Use envtest with shared TestMain, unique namespaces
12. Prefer table-driven tests
13. **Logging**: Use `internal/logging` for performance analysis
    - Opt-in via `-log-file` flag (silent by default)
    - Levels: DEBUG (timing), INFO (lifecycle), WARN/ERROR (issues)
    - Helpers: `logging.Start()`/`End()`, `logging.Time()`, `logging.EndWithCount()`
    - Instrument: startup, informer sync, context loading, expensive queries

## How-To Guides

### Add a Screen

1. Create config in `internal/screens/screens.go`: `GetMyResourceScreenConfig()`
2. Implement transform: `transformMyResource(u *unstructured.Unstructured, common commonFields) (any, error)`
3. Register in `internal/app/app.go`: `screens["myresource"] = screens.NewConfigScreen(...)`
4. Add tests in `internal/screens/screens_test.go`

### Add a Command

1. Create in `internal/commands/`: `func MyCommand(repo k8s.Repository) ExecuteFunc`
2. Register in `internal/commands/registry.go`
3. Add to screen operations in `internal/screens/config.go` (if needed)
4. Add tests: `internal/commands/mycommand_test.go`

### Add a Resource Type

1. Add GVR constant in `internal/k8s/constants.go`
2. Create screen config (see "Add a Screen")
3. Add transform function
4. Test with real cluster: `go run cmd/k1/main.go`

### Debug TUI Issues

**Common pitfalls**:
- Screen not updating: Check `Update()` returns modified model, messages sent
- Layout issues: Verify `WindowSizeMsg` handling, height calculations
- Keybindings: Check event bubbling, no component consuming key
- Command bar: Gets keys first when active, `Esc` deactivates

**Debug workflow**:
```bash
go run cmd/k1/main.go -log-file /tmp/k1.log
tail -f /tmp/k1.log  # in another terminal
```

### Add a Theme

1. Define in `internal/ui/theme.go`: `func NewMyTheme() Theme`
2. Register: `var themes = map[string]func() Theme{"mytheme": NewMyTheme}`
3. Test: `go run cmd/k1/main.go -theme mytheme`
4. Update README.md and CLAUDE.md

**WCAG AA Compliance Requirements**:
- All themes MUST meet WCAG AA contrast standards (4.5:1 for normal text,
  3:1 for large/bold text)
- Terminal color guidelines for dark backgrounds (#000000 to #1c1c1c):
  - "241"-"243": FAIL (2.5:1 to 3.1:1) - DO NOT USE for text
  - "246": PASS minimum (4.7:1) - Acceptable for secondary text
  - "248": PASS comfortable (5.5:1) - Preferred for readable text
  - "250"+: PASS excellent (6.6:1+) - Best for primary text
- Color property usage:
  - `Primary`, `Secondary`, `Accent`: High contrast accent colors
  - `Foreground`: Primary text (high contrast)
  - `Muted`: Status text, hints (minimum "248" on dark)
  - `Dimmed`: Secondary UI text (minimum "246" on dark)
  - `Subtle`: Background highlights, borders (minimum "246" for text)
- Visually test each theme on actual terminal to verify readability

## Code Patterns

### Structural Patterns

**Constants Organization**: Per-package constants to avoid circular deps
- `internal/components/constants.go`, `internal/k8s/constants.go`, etc.
- ❌ Central `internal/constants/` package creates import cycles

**Visibility**: Private by default (lowercase) unless needs export
- Factory/helper functions used only within package → private
- Export only what's called from other packages

**Method Encapsulation**: Functions operating on type's data should be methods
```go
// ❌ func getFilterContextDescription(ctx *FilterContext) string
// ✅ func (f *FilterContext) Description() string
```

### Performance Patterns

**Extract to Caller**: Move expensive repeated operations to caller
```go
// Before: Each transform calls extractCommonFields (O(11n))
// After: Caller extracts once, passes to all transforms (O(n))
type TransformFunc func(*unstructured.Unstructured, commonFields) (any, error)
```

**Why Not Reflection?**: Performance critical (1000+ resources), reflection 10-100x slower

### Extensibility Patterns

**Table-Driven**: Data structure instead of nearly-identical functions
```go
// Before: 11 functions differing only in screenID constant
// After: map[string]string + single function
var navigationRegistry = map[string]string{"pods": "pods", ...}
func NavigationCommand(screenID string) ExecuteFunc { ... }
```

**Config-Driven**: Function pointers in config vs N-way switch
```go
type NavigationFunc func(*ConfigScreen) tea.Cmd
type ScreenConfig struct {
    NavigationHandler NavigationFunc  // Optional
}
// Screen configures itself:
GetDeploymentsScreenConfig() { NavigationHandler: navigateToPodsForOwner(...) }
```

### Development Workflow Patterns

**Complete Test Coverage**: Create test file IMMEDIATELY with implementation
1. Test functions themselves (`navigation_test.go`)
2. Test configuration/wiring (`screens_test.go`)

**User Intent "Do It Now"**: "do it now"/"now"/"i want you to refactor" = implement immediately, not plan
- ✅ Implement, test, mark COMPLETE in plan
- ❌ Add to "Future Refactoring" or ask if they want it

**Planning Review Checklist**: Before finalizing plans, ask:
1. Pattern Match: Do similar resources use this? (No PodManager → no DynamicResourceManager)
2. YAGNI Check: What problem does this solve that existing components can't?
3. Simplicity Test: Explain in one sentence without "manager"/"coordinator"
4. Code Comparison: Side-by-side with existing implementation

**k1's core patterns**: Repository → Screen configs → Transform functions. NO managers/coordinators.

### Code Quality

**Helper Philosophy**: Only create helpers that reduce boilerplate
- ✅ `messages.ErrorCmd()` - wraps boilerplate
- ❌ `NewError()` - just use `fmt.Errorf()`

**Go Idioms**: Use `...any` not `...interface{}`, prefer stdlib over custom

**Message Helpers**: Use `internal/messages` for commands
- `messages.ErrorCmd()`, `messages.SuccessCmd()`, `messages.InfoCmd()`
- Repository layer: `fmt.Errorf("failed: %w", err)`

## Testing Guidelines

### Strategy by Component

| Component | Approach | Coverage | Focus |
|-----------|----------|----------|-------|
| k8s/* | envtest | 80%+ | CRUD, errors, edge cases |
| screens/* | Mock repo | 70%+ | Config, transforms, operations |
| commands/* | Mock repo | 70%+ | Validation, messages, success |
| components/* | Extract handlers | 50-60% | Pure functions, handlers |

**envtest**: Repository layer, real K8s behavior, informers
**mocks**: Screens, commands, pure functions, speed critical

### Coverage Check

```bash
make test-coverage        # Generate report
make test-coverage-html   # View in browser
```

**Current**: 76.7% (k8s), 71.0% (screens) ✅

### Testing Keybindings

Extract handlers from `Update()` to make testable:
```go
func (m Model) handleEnterKey() tea.Cmd { /* logic */ }
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    if key == "enter" { return m, m.handleEnterKey() }
}
// Test handleEnterKey() directly
```

### Performance Testing

```bash
time go run cmd/k1/main.go -log-file /tmp/k1.log
grep "duration" /tmp/k1.log
```

## Quick Reference

### Bubble Tea Concepts

- **Model**: State container
- **Update**: Message handler → (new model, commands)
- **View**: Render state → string
- **Cmd**: Function returning message (async ops)
- **Init**: Initial command on startup

### Lipgloss Patterns

```go
style := lipgloss.NewStyle().Foreground(color).Bold(true).Padding(1, 2)
content := lipgloss.JoinVertical(lipgloss.Left, line1, line2)
theme.Primary / theme.Success / theme.Error
```

### Commit Format

```
<type>: <summary>
```

Types: `feat`, `fix`, `docs`, `style`, `refactor`, `test`, `chore`
**No** "🤖 Generated with Claude Code" or "Co-Authored-By: Claude"

## Design Documents

Store in `design/` folder:
- Follow `design/TEMPLATE.md` if exists
- Author: @renato0307 (not @claude)
- **Format <80 chars/line**
- **NO implementation plans in designs/research**

## Implementation Plans

Store in `thoughts/shared/plans/`:
- Use `/create_plan_generic` slash command
- High-level and strategic, not step-by-step
- Track progress in plan markdown (not TodoWrite tool)
- Update TODO section after major work (phase completion)
- Update plan status at top to reflect current phase
- You can add log debug statements to troubleshoot UI issues
- CRITICAL: avoid workarounds; give priority to clean solutions; if no other option, ask before implementing workarounds