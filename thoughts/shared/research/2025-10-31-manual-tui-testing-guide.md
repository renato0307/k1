---
date: 2025-10-31 07:01:10 UTC
researcher: Claude
git_commit: 23edad435926b9e3d44fe3aed94f8a280db7d23b
branch: feat/basic-crds
repository: k1
topic: "Manual Testing Guide for k1 TUI Application"
tags: [research, testing, tui, manual-testing, bubble-tea, verification]
status: complete
last_updated: 2025-10-31
last_updated_by: Claude
---

# Research: Manual Testing Guide for k1 TUI Application

**Date**: 2025-10-31 07:01:10 UTC
**Researcher**: Claude
**Git Commit**: 23edad435926b9e3d44fe3aed94f8a280db7d23b
**Branch**: feat/basic-crds
**Repository**: k1

## Research Question

How can Claude manually test the k1 TUI application after making changes?
What are the available methods to run the app, check rendering, navigate
screens, execute commands, and verify behavior?

## Summary

Testing the k1 TUI (Terminal User Interface) application requires a
**multi-layered approach** combining automated unit tests, integration tests,
and manual verification. While k1 has excellent automated test coverage
(71-76%), manual testing remains essential for verifying visual appearance,
user experience, and end-to-end workflows.

**Key Testing Approaches:**
1. **Quick build verification**: `make build` or `go build -o k1 cmd/k1/main.go`
2. **Run with dummy data**: Use DummyRepository for fast UI testing
3. **Run with live cluster**: Test with real Kubernetes connection
4. **Automated tests**: 71-76% coverage using envtest and table-driven tests
5. **VHS recordings**: Script terminal sessions for reproducible manual tests
6. **Terminal recording**: asciinema for capturing and replaying test sessions

## Detailed Findings

### 1. Running the k1 Application

**cmd/k1/main.go** is the main entry point. The application supports multiple
running modes via command-line flags.

#### Using Makefile (Preferred Method)

From `/Users/renato/Work/willful/k1/Makefile`:

```bash
# Build the application
make build                # Creates ./k1 binary

# Run with live cluster connection
make run                  # Runs: go run cmd/k1/main.go

# Run with dummy data (no cluster connection)
make run-dummy            # Runs: go run cmd/k1/main.go -dummy

# Clean up build artifacts
make clean                # Removes binaries and coverage files
```

#### Direct Go Commands

```bash
# Run with live Kubernetes connection (default theme)
go run cmd/k1/main.go

# Run with specific context
go run cmd/k1/main.go -context my-cluster

# Run with custom kubeconfig path
go run cmd/k1/main.go -kubeconfig /path/to/kubeconfig

# Run with specific theme
go run cmd/k1/main.go -theme dracula    # 8 themes available

# Build, test, and clean up
go build -o k1 cmd/k1/main.go
./k1
rm k1
```

#### Available Command-Line Flags

From `cmd/k1/main.go:38-47`:

- `-theme` - Theme selection (charm, dracula, catppuccin, nord, gruvbox,
  tokyo-night, solarized, monokai)
- `-kubeconfig` - Path to kubeconfig file (default: ~/.kube/config)
- `-max-contexts` - Maximum number of contexts to keep loaded (1-20, default:
  10)
- `-context` - Kubernetes context(s) to use (can be specified multiple times)

**Note**: The `-dummy` flag mentioned in CLAUDE.md has been removed. Current
implementation uses live contexts only, but DummyRepository remains available
for testing.

### 2. Testing UI Changes Without a Cluster

**Problem**: Testing UI changes against a live cluster is slow and requires
actual Kubernetes resources.

**Solution**: Use the `DummyRepository` which provides hardcoded test data.

#### DummyRepository Implementation

Located in `internal/k8s/dummy_repository.go` (655 lines), provides fake data
for:

- **Pods**: 4 test pods (nginx, postgres, redis)
- **Deployments**: 3 test deployments
- **Services**: 3 test services
- **Nodes**: 3 test nodes
- **All other resource types**: Configurable test data

**Usage in tests** (from `internal/screens/config_test.go:27`):

```go
repo := k8s.NewDummyRepository()
screen := NewConfigScreen(cfg, repo, theme)

// Screen now has predictable test data
pods, _ := repo.GetPods()
// Returns 4 hardcoded test pods
```

#### Manual Testing with Dummy Data

While the `-dummy` CLI flag has been removed, you can still use DummyRepository
for development:

**Option 1: Modify main.go temporarily**:

```go
// cmd/k1/main.go
func main() {
    // ... existing code ...

    // Replace:
    // pool, err := k8s.NewRepositoryPool(kubeconfig, maxContexts)

    // With:
    repo := k8s.NewDummyRepository()
    pool := k8s.NewSingleRepositoryPool(repo)  // If such constructor exists

    // ... rest of initialization
}
```

**Option 2: Use test harness**:

```go
// Create a test file: cmd/k1/test_main.go
package main

import (
    "github.com/renato0307/k1/internal/app"
    "github.com/renato0307/k1/internal/k8s"
    "github.com/renato0307/k1/internal/ui"
)

func main() {
    repo := k8s.NewDummyRepository()
    theme := ui.CharmTheme()
    model := app.NewModelWithRepo(repo, theme)
    // ... run Bubble Tea program
}
```

**Benefits of dummy data testing**:
- Fast startup (no cluster connection)
- Predictable data (same results every time)
- Test edge cases (empty lists, long names, error states)
- No external dependencies

### 3. Automated Testing Infrastructure

k1 has comprehensive automated test coverage (71-76%) using a layered testing
strategy documented in `thoughts/shared/research/2025-10-09-bubble-tea-
tui-testing.md`.

#### Test Execution Commands

From `/Users/renato/Work/willful/k1/Makefile`:

```bash
# One-time setup: Install envtest binaries (~50MB, cached)
make setup-envtest

# Run all tests (preferred method)
make test                 # Runs: go test -v ./... -timeout 60s

# Run tests with coverage report
make test-coverage        # Generates coverage/k8s.out, coverage/screens.out

# View coverage in browser
make test-coverage-html   # Opens HTML coverage report
```

**Manual fallback** (if Makefile unavailable):

```bash
export KUBEBUILDER_ASSETS=$(setup-envtest use -p path)
go test -v ./... -timeout 60s
```

#### Test Architecture

**Layer 1: Unit Tests** (90% of tests)
- Test business logic independent of rendering
- Use `DummyRepository` for fast execution
- Table-driven tests for comprehensive coverage
- Example: `internal/screens/config_test.go` (1164 lines)

**Layer 2: Integration Tests** (8% of tests)
- Test with real Kubernetes API via envtest
- Shared TestMain pattern (starts API server once)
- Namespace isolation (unique `test-*` namespace per test)
- Example: `internal/k8s/suite_test.go`

**Layer 3: Manual Tests** (2% of tests)
- Visual verification of UX
- End-to-end workflows
- Cross-platform terminal testing
- Documented in implementation plans

#### Current Test Coverage

From `thoughts/shared/research/2025-10-10-kubernetes-context-management-
quality-review.md`:

- **k8s package**: 76.7% coverage
- **screens package**: 71.0% coverage
- **28 test files total** covering most components

**Quality gates** (from `CLAUDE.md:12-29`):
- New components: 70% minimum
- Modified components: Cannot decrease coverage
- Critical paths: 80% minimum

### 4. Core Testing Patterns for Bubble Tea

#### Pattern 1: Test Update() Directly, Not Full Program

**Don't run `tea.NewProgram()` in tests**. Instead, test `Update()` methods
directly:

```go
// Create model
model := NewModel(pool, theme)

// Send message directly to Update()
msg := types.ScreenSwitchMsg{ScreenID: "deployments"}
updatedModel, cmd := model.Update(msg)

// Execute command to get resulting message
if cmd != nil {
    resultMsg := cmd()
    // Assert on message type and content
}

// Test view output as strings
view := updatedModel.View()
assert.Contains(t, view, "expected content")
```

**From `internal/app/app_test.go:50-64`** - Example testing navigation:

```go
func TestNavigationHistoryMaxSize(t *testing.T) {
    pool := createTestPool(t)
    model := NewModel(pool, theme.CharmTheme())

    // Send 15 navigation messages
    for i := 0; i < 15; i++ {
        msg := types.ScreenSwitchMsg{ScreenID: fmt.Sprintf("screen-%d", i)}
        model, _ = model.Update(msg)
    }

    // Verify history limited to 10 items
    assert.LessOrEqual(t, len(model.navigationHistory), 10)
}
```

**Why this works**:
- Fast execution (no terminal rendering)
- Deterministic results
- Easy edge case testing
- Tests business logic separately from rendering

#### Pattern 2: Test View() Output as Strings

Test content presence, not ANSI codes or exact styling:

```go
view := screen.View()
assert.NotEmpty(t, view)
assert.Contains(t, view, "Expected content")
assert.NotContains(t, view, "Unwanted content")
```

**From `internal/screens/config_test.go:260-280`**:

```go
func TestConfigScreen_View(t *testing.T) {
    screen := NewConfigScreen(cfg, k8s.NewDummyRepository(), theme)
    screen.SetSize(80, 24)

    view := screen.View()
    assert.NotEmpty(t, view)
    // Don't test exact ANSI codes, just content presence
}
```

#### Pattern 3: Table-Driven Tests

Use structs to test multiple scenarios with same code:

```go
tests := []struct {
    name       string
    input      string
    expected   string
}{
    {"case 1", "input1", "output1"},
    {"case 2", "input2", "output2"},
}

for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
        result := ProcessInput(tt.input)
        assert.Equal(t, tt.expected, result)
    })
}
```

**From `internal/screens/config_test.go:175-196`** - FormatDuration tests:

```go
func TestFormatDuration(t *testing.T) {
    tests := []struct {
        name     string
        duration time.Duration
        expected string
    }{
        {"seconds", 30 * time.Second, "30s"},
        {"minutes", 5 * time.Minute, "5m"},
        {"hours", 2 * time.Hour, "2h"},
        {"days", 3 * 24 * time.Hour, "3d"},
        // ... 3 more cases
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := FormatDuration(tt.duration)
            assert.Equal(t, tt.expected, result)
        })
    }
}
```

### 5. Manual Testing Workflow

Based on development guidelines from `CLAUDE.md:218-226` and implementation
plans.

#### Step-by-Step Manual Testing Process

**1. Make code changes**

Edit files in `internal/` directory:
- `internal/screens/` - Screen implementations
- `internal/components/` - Reusable UI components
- `internal/k8s/` - Kubernetes data access layer
- `internal/commands/` - Command implementations

**2. Run automated tests**

```bash
# Quick test of affected package
go test ./internal/screens -v

# Full test suite
make test

# Check coverage (should not decrease)
make test-coverage
```

**3. Build and verify compilation**

```bash
# Build to catch compilation errors
go build -o k1 cmd/k1/main.go

# Clean up binary
rm k1

# Or use Makefile
make build && make clean
```

**4. Run application for manual testing**

```bash
# Option A: Live cluster (if available)
go run cmd/k1/main.go

# Option B: Specific context
go run cmd/k1/main.go -context test-cluster

# Option C: Different theme
go run cmd/k1/main.go -theme dracula
```

**5. Verify changes manually**

Navigate through the UI and verify:
- ✓ UI renders correctly
- ✓ Tables display expected data
- ✓ Navigation works (`:pods`, `:deployments`, etc.)
- ✓ Commands execute (`/yaml`, `/describe`, etc.)
- ✓ Filters work (type characters for fuzzy search)
- ✓ Keyboard shortcuts function (ctrl+y, ctrl+d)
- ✓ Window resizing works properly
- ✓ No visual glitches or ANSI rendering issues

**6. Test different terminal sizes**

```bash
# Run in different terminal sizes to test responsive layout
# Typical sizes: 80x24 (small), 120x30 (medium), 160x40 (large)

# Or resize terminal interactively and verify layout adapts
```

**7. Test edge cases**

- Empty lists (filter to match nothing)
- Long resource names (overflow handling)
- Error states (disconnect from cluster)
- High resource counts (performance)

**8. User confirms before committing**

From `CLAUDE.md:218-226`:

> Testing and Commits: NEVER commit code without user testing first
> - After implementing features, build and wait for user to test
> - User will verify functionality works as expected
> - Only create commits AFTER user confirms testing is complete

**9. Commit changes**

```bash
# Stage all changes
git add -A

# View what will be committed
git status

# Create semantic commit (no generated signatures!)
git commit -m "feat: add feature description"

# NOT ALLOWED:
# 🤖 Generated with Claude Code
# Co-Authored-By: Claude <noreply@anthropic.com>
```

### 6. TUI Testing Tools and Techniques

From web research on Terminal User Interface testing:

#### teatest - Official Bubble Tea Testing (Experimental)

**Source**: github.com/charmbracelet/x/exp/teatest
**Status**: Experimental, actively maintained

**Key Features**:
- `NewTestModel(t, model, options...)` - Create testable model with
  configurable terminal size
- `Send(tea.Msg)` - Inject messages directly
- `Type(string)` - Simulate keyboard input
- `WaitFor(condition, options...)` - Poll output until condition met
- `RequireEqualOutput(t, expected)` - Golden file comparison

**Example Pattern**:

```go
func TestMyScreen(t *testing.T) {
    screen := NewPodsScreen(repo, theme)
    tm := teatest.NewTestModel(t, screen,
        teatest.WithInitialTermSize(80, 24))

    // Simulate typing
    tm.Type("nginx")

    // Wait for filtered results
    teatest.WaitFor(t, tm.Output(),
        func(bts []byte) bool {
            return bytes.Contains(bts, []byte("nginx"))
        },
        teatest.WithDuration(3*time.Second))

    // Verify final output
    tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
    out, _ := io.ReadAll(tm.FinalOutput(t))
    assert.Contains(t, string(out), "nginx")
}
```

**Limitations**:
- Still experimental (API may change)
- Limited documentation with real-world examples
- Not yet widely adopted

#### VHS - Terminal Recorder for Integration Testing

**Source**: github.com/charmbracelet/vhs
**Purpose**: "Write terminal GIFs as code for integration testing and demoing"

**Key Features**:
- Scriptable terminal sessions via `.tape` files
- Reproducible test scenarios with exact timing control
- Output in multiple formats (GIF, MP4, WebM, PNG)
- Screenshot capability for visual regression testing

**Example `.tape` file**:

```tape
# tests/workflows/basic-navigation.tape
Set Height 600
Set Width 1200

Output test-output.gif

# Start k1
Type "go run cmd/k1/main.go"
Enter
Sleep 2s

# Navigate to pods
Type ":pods"
Enter
Sleep 1s

# Verify pods screen loaded
Screenshot pods-screen.png

# Filter pods
Type "nginx"
Sleep 500ms

# Verify filter worked
Screenshot filtered-pods.png
```

**Running VHS tests**:

```bash
# Install VHS
brew install vhs

# Record a test session
vhs test-scenario.tape

# Output: test-output.gif, pods-screen.png, filtered-pods.png
```

**Use cases for k1**:
- Visual regression testing
- Integration testing with realistic workflows
- Documentation generation (screenshots for README)
- Onboarding videos

**Limitations**:
- Requires external dependencies (ttyd, ffmpeg)
- Timing-sensitive (may be flaky if app response varies)
- Better for integration tests than unit tests

#### asciinema - Terminal Recording for Manual Verification

**Source**: asciinema.org
**Purpose**: Record and replay terminal sessions as text (not video)

**Usage for testing**:

```bash
# Record a test session
asciinema rec test-pods-screen.cast
# ... interact with k1 TUI ...
# Exit recording with Ctrl+D

# Replay to verify behavior
asciinema play test-pods-screen.cast

# Share for review
asciinema upload test-pods-screen.cast
```

**Benefits**:
- Lightweight, searchable format (JSON)
- Can copy/paste text from recordings
- Easy to share and review
- Preserves timing information

**Use cases for k1**:
- Document manual test procedures
- Share bug reproductions
- Review team member testing
- Archive successful test runs

#### go-expect - Terminal Automation Library

**Source**: github.com/Netflix/go-expect (Netflix)
**Purpose**: Expect-like interface for automating terminal interactions

**Example pattern**:

```go
import (
    "github.com/Netflix/go-expect"
)

func TestInteractiveCLI(t *testing.T) {
    // Start command in PTY
    console, err := expect.NewConsole(expect.WithStdout(os.Stdout))
    require.NoError(t, err)
    defer console.Close()

    // Launch k1
    cmd := exec.Command("./k1")
    cmd.Stdin = console.Tty()
    cmd.Stdout = console.Tty()
    cmd.Stderr = console.Tty()

    go cmd.Run()

    // Wait for prompt and respond
    console.ExpectString("Select resource:")
    console.SendLine("pods")

    // Verify output
    console.ExpectString("NAME")
    console.ExpectString("READY")
}
```

**Use cases**:
- End-to-end integration tests
- Testing interactive prompts
- Automated acceptance testing

**Limitations**:
- Heavier than unit tests (actual process execution)
- Terminal-specific behavior varies by environment

### 7. Testing Checklists from Implementation Plans

Multiple implementation plans include comprehensive manual testing checklists.

#### From 2025-10-10-kubernetes-context-management-quality-fixes.md

**Manual Testing Checklist - Core Flows**:

```
Context Loading:
☐ Single context loads correctly
☐ Multiple contexts specified with -context flag
☐ LoadContext() shows progress messages
☐ Failed context shows clear error message

Context Switching:
☐ /context list shows all contexts with status
☐ /context load <name> loads context successfully
☐ Switching between contexts preserves state
☐ Screen refreshes after successful switch

Progress Reporting:
☐ PhaseConnecting message appears
☐ PhaseSyncingCore message appears
☐ PhaseSyncingDynamic message appears
☐ PhaseComplete message appears
☐ Progress messages clear after completion

Error Handling:
☐ Invalid kubeconfig path shows error
☐ Unreachable cluster shows timeout error
☐ RBAC permission errors show clear message
☐ Context not found shows helpful error
```

**Window Resize Testing**:

```
☐ Resize from 80x24 to 120x30 - layout adapts
☐ Resize from 120x30 to 160x40 - no overflow
☐ Resize to 40x10 - graceful degradation
☐ Rapid resizing doesn't cause panic
```

#### From 2025-10-26-responsive-column-display.md

**Manual Verification Checklist - Column Display**:

```
Tier 0 Columns (Always visible):
☐ NAME column always shows at all sizes
☐ STATUS column always shows
☐ AGE column always shows
☐ Minimum width prevents text wrapping

Tier 1 Columns (Show at medium width):
☐ NAMESPACE appears at 100+ columns
☐ READY appears at 100+ columns

Tier 2 Columns (Show at large width):
☐ RESTARTS appears at 120+ columns
☐ NODE appears at 120+ columns

Tier 3+ Columns (Show at extra-large width):
☐ Additional columns appear at 140+ columns
☐ All columns visible at 160+ columns

Dynamic Width Calculation:
☐ Columns resize proportionally
☐ No horizontal scrolling required
☐ Text doesn't wrap within cells
☐ Ellipsis (...) for overflow
```

#### From 2025-10-28-crd-support.md

**Testing Strategy - CRD Resources**:

```
Basic CRD Listing:
☐ CRD definitions load correctly
☐ :crds navigation shows list
☐ CRD screen shows NAME, GROUP, VERSION, SCOPE, AGE

CRD Instance Navigation:
☐ Enter on CRD shows instances screen
☐ Instance screen shows configured columns
☐ Back navigation returns to CRDs list

Dynamic Column Display:
☐ schema.properties used for column selection
☐ jsonpath extracts values correctly
☐ Empty values show as "-"
☐ Complex values show as "<object>" or "<array>"

Edge Cases:
☐ CRDs with no instances show empty screen
☐ CRDs with 1000+ instances perform acceptably
☐ CRDs with deeply nested schema don't crash
```

### 8. Historical Testing Documentation

From `thoughts/shared/research/2025-10-09-bubble-tea-tui-testing.md`:

**Three-Layer Testing Architecture**:

1. **Unit Tests (90% of tests)**
   - Test Update() logic with message injection
   - Test View() output as strings
   - Use dummy dependencies for speed
   - Table-driven tests for coverage
   - Focus: Business logic independent of rendering

2. **Integration Tests (8% of tests)**
   - Test with real Kubernetes API (envtest)
   - Shared TestMain pattern (5s startup)
   - Namespace isolation per test
   - Focus: Real dependency behavior

3. **Manual Tests (2% of tests)**
   - Visual verification of UX
   - End-to-end workflows
   - Cross-platform terminal testing
   - Focus: User experience, edge cases

**Testing Philosophy** (from research doc lines 749-763):

> By testing business logic separately from rendering, most bugs are caught
> without needing terminal simulation. Manual testing verifies the final UX.
>
> Key principles:
> - Test Update() logic directly, not full programs
> - Test View() output as strings (content presence, not styling)
> - Use real dependencies where practical (envtest)
> - Aim for 70%+ coverage on business logic
> - Manual testing fills gaps in UI layer

### 9. Quality Review and Known Gaps

From `thoughts/shared/research/2025-10-10-kubernetes-context-management-
quality-review.md`:

**Current Coverage Analysis**:

```
Business Logic: 70-80% covered ✓
UI Layer: 10-20% covered (major gap)
Integration Layer: 0% covered (major gap)
```

**Untested UI Components**:

- `app.go` Update() orchestration (command bar height, message forwarding)
- `config.go` Update() key handling (KeyMsg routing, navigation)
- `commandbar.go` state machine (state transitions, paste events)
- `fullscreen.go`, `layout.go`, `header.go`, `statusbar.go` - Zero tests

**Integration Gaps**:

- No tests combining App + Screen + CommandBar
- No tests for complete user flows (type → filter → navigate → select)
- No tests for command execution → result display → state restoration

**Recommendations**:

1. Add teatest-based integration tests for critical user flows
2. Add Update() tests for UI layer components
3. Create VHS scripts for regression testing
4. Expand manual testing checklists in implementation plans

## Code References

**Running and Building**:
- `cmd/k1/main.go:1-100` - Main entry point with flag parsing
- `Makefile:1-40` - Build, test, and run targets

**Testing Infrastructure**:
- `internal/k8s/suite_test.go:19-55` - Shared TestMain with envtest setup
- `internal/k8s/dummy_repository.go:1-655` - Dummy data for testing
- `internal/components/commandbar/test_helpers.go:12-47` - Test pool helpers
- `internal/app/app_test.go:18-48` - App test setup utilities

**Example Test Files**:
- `internal/app/app_test.go` (280 lines) - Root model tests
- `internal/screens/config_test.go` (1164 lines) - ConfigScreen tests
- `internal/screens/system_test.go` (204 lines) - SystemScreen tests
- `internal/components/commandbar/input_test.go` (236 lines) - Input tests

**Documentation**:
- `CLAUDE.md:52-111` - Development setup and running instructions
- `CLAUDE.md:12-29` - Quality gates and testing requirements

## Architecture Insights

### Testing Architecture Patterns

**1. Layered Testing Strategy**

k1 demonstrates a mature testing approach that balances coverage, speed, and
maintainability:

- **Unit tests** catch 70-80% of bugs without terminal rendering
- **Integration tests** verify real Kubernetes API behavior
- **Manual tests** validate UX and visual appearance

This matches industry best practices for TUI applications where testing
business logic separately from rendering is most effective.

**2. Repository Pattern for Testability**

The `k8s.Repository` interface abstracts data access, enabling:

- **DummyRepository**: Fast UI testing without cluster
- **InformerRepository**: Real Kubernetes integration
- **TestRepository**: Injected via RepositoryPool.SetTestRepository()

This pattern allows tests to control data precisely while maintaining
production-like code paths.

**3. Message-Driven Testing**

Bubble Tea's message-driven architecture enables testing without running full
programs:

```go
// Don't need: tea.NewProgram(model).Run()

// Instead: Test Update() directly
msg := types.ScreenSwitchMsg{ScreenID: "pods"}
newModel, cmd := model.Update(msg)
```

This isolation dramatically improves test speed (milliseconds vs. seconds) and
reliability (deterministic vs. timing-sensitive).

**4. Shared Test Infrastructure**

The `suite_test.go` pattern amortizes expensive setup across all tests:

- **envtest starts once** (~5s) instead of per-test (~5s × N tests)
- **Shared API server** reduces resource consumption
- **Namespace isolation** prevents test interference

This enables hundreds of integration tests to run in ~5-10 seconds total.

### Design Decisions

**1. Why No teatest Yet?**

k1 predates teatest's availability and uses direct Update()/View() testing
instead. This approach:

- ✓ Works reliably with zero external dependencies
- ✓ Tests exactly what matters (business logic)
- ✓ Runs fast (no terminal rendering overhead)
- ✗ Doesn't test keyboard input parsing (minor gap)
- ✗ Requires manual testing for visual verification

**Recommendation**: Consider adopting teatest for critical user flow tests
(e.g., filter → navigate → execute command) to catch integration bugs earlier.

**2. Why Remove -dummy Flag?**

The CLI previously supported a `-dummy` flag for running with fake data. This
was removed in favor of:

- **Development**: Temporarily modify main.go or create test harness
- **Testing**: Use DummyRepository directly in test files
- **Production**: Always connect to real cluster

**Rationale**: Reduces CLI complexity and prevents users from accidentally
running in dummy mode.

**3. Why No VHS Tests Yet?**

VHS is excellent for visual regression testing but requires:

- External dependencies (ttyd, ffmpeg)
- Maintenance of `.tape` scripts
- CI/CD infrastructure for running headless

**Current status**: Manual testing fills this gap effectively. VHS could be
added when team grows or CI/CD is mature.

## Historical Context (from thoughts/)

### Comprehensive Testing Research

`thoughts/shared/research/2025-10-09-bubble-tea-tui-testing.md` provides the
definitive guide for k1's testing philosophy, covering:

- Automatic testing with Bubble Tea (unit and integration patterns)
- Testing architecture (three-layer strategy)
- Core testing patterns (Update() testing, View() verification)
- Table-driven tests and namespace isolation
- Coverage analysis and gap identification
- Practical guide for testing screens, messages, and commands

This document established the testing standards followed throughout the
codebase.

### Quality Review and Issue Tracking

`thoughts/shared/research/2025-10-10-kubernetes-context-management-
quality-review.md` identified systematic quality issues:

- 5 critical issues (deadlocks, race conditions)
- 4 high-priority issues (test coverage gaps)
- 8 medium/low issues (edge cases, documentation)

This led to the comprehensive quality fixes plan.

### Implementation Plans with Testing Checklists

Multiple implementation plans include detailed manual testing procedures:

- `2025-10-10-kubernetes-context-management-quality-fixes.md` - Core flows,
  context switching, progress reporting, error handling
- `2025-10-26-responsive-column-display.md` - Column display at different
  window sizes
- `2025-10-28-crd-support.md` - CRD listing, instance navigation, dynamic
  columns

These checklists demonstrate the project's commitment to thorough manual
verification beyond automated tests.

## Practical Testing Guide for Claude

### Quick Reference: Making and Testing Changes

**1. Make changes to code**

Edit files in `internal/` directory as needed.

**2. Run automated tests**

```bash
make test                    # Full test suite (~5-10s)
go test ./internal/screens   # Specific package
```

**3. Build and run**

```bash
# Quick build check
go build -o k1 cmd/k1/main.go && rm k1

# Or build and run
make build
./k1
make clean
```

**4. Manual verification**

```bash
# Run with live cluster
go run cmd/k1/main.go

# Or specific context
go run cmd/k1/main.go -context my-cluster

# Or different theme
go run cmd/k1/main.go -theme dracula
```

**5. Verify UI behavior**

- Navigate screens: `:pods`, `:deployments`, `:services`
- Filter: Type characters for fuzzy search
- Execute commands: `/yaml`, `/describe`, `/scale`
- Test shortcuts: ctrl+y (yaml), ctrl+d (describe)
- Resize terminal: Verify layout adapts
- Test edge cases: Empty lists, long names, errors

**6. Wait for user confirmation**

**Do NOT commit until user confirms**: "tests passed, commit it"

**7. Commit changes**

```bash
git add -A
git status
git commit -m "feat: describe the change"
```

**No generated signatures** - Don't add Claude Code attribution.

### Testing Scenarios

**Scenario 1: Changing a Screen Layout**

```bash
# 1. Edit internal/screens/config.go
# 2. Run screen tests
go test ./internal/screens -v -run TestConfigScreen

# 3. Run app with dummy data (modify main.go temporarily)
go run cmd/k1/main.go

# 4. Navigate to affected screen
# 5. Verify layout at different terminal sizes (80x24, 120x30, 160x40)
# 6. Wait for user confirmation
# 7. Commit
```

**Scenario 2: Adding a New Command**

```bash
# 1. Add command to internal/commands/
# 2. Register in internal/commands/registry.go
# 3. Write tests in internal/commands/*_test.go
make test

# 4. Run app and test command
go run cmd/k1/main.go
# Type / to open command palette
# Type command name to execute

# 5. Verify command result displayed correctly
# 6. Wait for user confirmation
# 7. Commit
```

**Scenario 3: Fixing a Bug**

```bash
# 1. Write failing test that reproduces bug
go test ./internal/k8s -v -run TestBugName
# Test should fail

# 2. Fix the bug
# 3. Run test again
go test ./internal/k8s -v -run TestBugName
# Test should pass

# 4. Run full test suite
make test

# 5. Manual verification
go run cmd/k1/main.go
# Reproduce original bug scenario
# Verify bug is fixed

# 6. Wait for user confirmation
# 7. Commit
```

**Scenario 4: Refactoring Code**

```bash
# 1. Check current coverage
make test-coverage
# Note baseline coverage

# 2. Refactor code
# 3. Run tests to ensure no regressions
make test

# 4. Check coverage didn't decrease
make test-coverage
# Coverage should be same or higher

# 5. Manual smoke test
go run cmd/k1/main.go
# Quick sanity check - basic navigation works

# 6. Wait for user confirmation
# 7. Commit
```

### Advanced Testing Techniques

**Testing with Multiple Contexts**

```bash
# Start k1 with multiple contexts
go run cmd/k1/main.go -context context1 -context context2

# Verify:
# - Both contexts load
# - Can switch between contexts
# - Data refreshes correctly
# - No memory leaks
```

**Testing Error Conditions**

```bash
# Test with invalid kubeconfig
go run cmd/k1/main.go -kubeconfig /nonexistent

# Test with unreachable cluster
go run cmd/k1/main.go -context unreachable-context

# Verify error messages are clear and helpful
```

**Testing Performance**

```bash
# Connect to cluster with 1000+ pods
go run cmd/k1/main.go -context large-cluster

# Verify:
# - App starts quickly (< 5s)
# - Scrolling is smooth
# - Filtering is instant
# - No UI lag
```

**Testing Themes**

```bash
# Test each theme
for theme in charm dracula catppuccin nord gruvbox tokyo-night \
             solarized monokai; do
    echo "Testing theme: $theme"
    go run cmd/k1/main.go -theme $theme
    # Verify colors render correctly
    # Verify text is readable
done
```

### Debugging Tips

**1. Enable Debug Logging**

```go
// Add to main.go temporarily
f, _ := tea.LogToFile("debug.log", "debug")
defer f.Close()

// Run app, check debug.log for Bubble Tea events
```

**2. Isolate Screen Behavior**

```go
// Create minimal test program
func main() {
    repo := k8s.NewDummyRepository()
    screen := screens.NewPodsScreen(repo, theme.CharmTheme())
    p := tea.NewProgram(screen)
    p.Run()
}
```

**3. Test Message Flow**

```go
// Add debug prints in Update()
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    fmt.Printf("DEBUG: Received message: %T\n", msg)
    // ... rest of Update
}
```

**4. Verify View Output**

```go
// Print raw view output
view := screen.View()
fmt.Println(view)

// Or strip ANSI for debugging
import "github.com/acarl005/stripansi"
clean := stripansi.Strip(view)
fmt.Println(clean)
```

## Open Questions

1. **Should k1 adopt teatest for integration testing?**
   - Pros: Better testing of keyboard input, more realistic user flows
   - Cons: Experimental API, additional dependency
   - Recommendation: Pilot on 2-3 critical flows

2. **Should VHS be added for visual regression testing?**
   - Pros: Catches visual regressions, generates documentation
   - Cons: Requires external dependencies, CI/CD setup
   - Recommendation: Add when team grows or regression bugs increase

3. **Should -dummy flag be restored for manual UI testing?**
   - Current: Must modify main.go temporarily for dummy data testing
   - Alternative: Restore -dummy flag for development convenience
   - Recommendation: Ask user preference

4. **How to test informer-based real-time updates?**
   - Current gap: Tests don't verify real-time update behavior
   - Challenge: Informers update asynchronously
   - Potential solution: Mock informer events in tests

5. **Should asciinema recordings be committed for manual test cases?**
   - Pros: Reproducible manual test scenarios
   - Cons: Binary files in git, may become stale
   - Recommendation: Store in separate repository or cloud storage

## Related Research

- `thoughts/shared/research/2025-10-09-bubble-tea-tui-testing.md` -
  Comprehensive TUI testing guide
- `thoughts/shared/research/2025-10-10-kubernetes-context-management-
  quality-review.md` - Quality review identifying test coverage gaps
- `thoughts/shared/plans/2025-10-10-kubernetes-context-management-
  quality-fixes.md` - Quality fixes with manual testing checklists
- `thoughts/shared/plans/2025-10-26-responsive-column-display.md` - Column
  display testing checklist
- `thoughts/shared/plans/2025-10-28-crd-support.md` - CRD testing strategy

## Conclusion

**Claude can manually test the k1 TUI application** using a combination of:

1. **Automated tests** (`make test`) - Catches 71-76% of bugs quickly
2. **Direct execution** (`go run cmd/k1/main.go`) - Verifies UI and UX
3. **DummyRepository** - Enables fast UI testing without cluster
4. **Manual checklists** - Systematic verification of critical flows
5. **Terminal recording** (asciinema, VHS) - Reproducible test scenarios

**Key workflow**: Make changes → Run tests → Build → Run app → Verify
manually → Wait for user confirmation → Commit

**Most important**: The project has excellent guidelines and testing
infrastructure. Following the documented patterns and waiting for user
confirmation before committing ensures quality.
