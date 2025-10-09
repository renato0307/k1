---
date: 2025-10-09T06:23:15Z
researcher: Claude
git_commit: b22fe8a0c005295e708d66e728e4b1cafbc4e16a
branch: docs/designs
repository: k1-designs
topic: "Automatic TUI Testing with Bubble Tea"
tags: [research, testing, bubble-tea, tui, automation]
status: complete
last_updated: 2025-10-09
last_updated_by: Claude
---

# Research: Automatic TUI Testing with Bubble Tea

**Date**: 2025-10-09T06:23:15Z
**Researcher**: Claude
**Git Commit**: b22fe8a0c005295e708d66e728e4b1cafbc4e16a
**Branch**: docs/designs
**Repository**: k1-designs

## Research Question

What are the possibilities for automatic TUI testing with Bubble Tea
applications? What testing approaches, tools, and patterns are available for
testing terminal user interfaces built with the Bubble Tea framework?

## Summary

**Bubble Tea applications can be effectively tested using standard Go testing
tools without specialized TUI testing frameworks**. The k1 project demonstrates
a pragmatic testing strategy that achieves 71-76% test coverage by:

1. **Testing business logic separately** - Data transformation, filtering,
   state management
2. **Testing Update() directly** - Message handling, command generation,
   state transitions
3. **Testing View() output as strings** - Content verification, not exact
   ANSI rendering
4. **Using real dependencies** - envtest for Kubernetes API, not mocks
5. **Manual testing for UX** - Running the app to verify interactions

This approach avoids the complexity of terminal emulation or screenshot
comparison while maintaining high coverage of critical logic paths.

## Detailed Findings

### 1. Testing Architecture for Bubble Tea Applications

The k1 project uses a **multi-layer testing approach** that separates concerns:

#### Unit Tests (State & Logic)
Test individual functions and methods without running the full Bubble Tea
program:

```go
// Test message handling
func TestConfigScreen_Refresh(t *testing.T) {
    screen := NewConfigScreen(cfg, repo, theme)

    // Execute refresh command
    cmd := screen.Refresh()
    require.NotNil(t, cmd)

    // Execute the command to get the message
    msg := cmd()

    // Assert message type and content
    refreshMsg, ok := msg.(types.RefreshCompleteMsg)
    require.True(t, ok)
    assert.Greater(t, refreshMsg.Duration, time.Duration(0))
}
```

**Reference**: `internal/screens/config_test.go:38-72`

#### Integration Tests (Real Dependencies)
Use envtest to run a real Kubernetes API server for testing informers and data
fetching:

```go
// TestMain sets up shared envtest environment
func TestMain(m *testing.M) {
    testEnv = &envtest.Environment{
        CRDDirectoryPaths:     []string{},
        ErrorIfCRDPathMissing: false,
    }

    testCfg, err = testEnv.Start()  // ~5s startup, runs once
    testClient, err = kubernetes.NewForConfig(testCfg)

    code := m.Run()  // Run all tests
    testEnv.Stop()   // Teardown
}
```

**Reference**: `internal/k8s/suite_test.go`

**Benefits**:
- Tests actual Kubernetes behavior, not mocked responses
- Fast test execution (~5-10 seconds for entire suite)
- Namespace isolation prevents test conflicts
- Realistic cache behavior with informers

#### Manual Testing (UX Verification)
Complex user interaction flows are tested manually:

```bash
# Run with dummy data for quick testing
make run-dummy

# Run with live cluster
make run

# Test specific themes
go run cmd/k1/main.go -theme dracula
```

**Reference**: `CLAUDE.md:30-52`

---

### 2. Core Testing Patterns

#### Pattern 1: Test Update() Logic Directly

**Don't** run the full `tea.NewProgram()` - **do** call `Update()` with
messages directly:

```go
func TestHandleKeyPress(t *testing.T) {
    screen := NewScreen()

    // Create key press message
    msg := tea.KeyMsg{Type: tea.KeyEnter}

    // Call Update directly
    newModel, cmd := screen.Update(msg)

    // Assert state changes
    assert.NotNil(t, cmd)
    // Verify model state changed as expected
}
```

**Why this works**:
- Isolates the logic from terminal rendering
- Fast execution (no terminal initialization)
- Deterministic results
- Easy to test edge cases

**Current gap in k1**: Most `Update()` methods are **not tested** (see section
4 for details).

#### Pattern 2: Test View() Output as Strings

Test the **rendered content**, not exact ANSI codes or styling:

```go
func TestConfigScreen_View(t *testing.T) {
    screen := NewConfigScreen(cfg, repo, theme)
    screen.SetSize(80, 24)
    screen.Refresh()()  // Populate data

    view := screen.View()  // Get rendered string
    assert.NotEmpty(t, view)
    assert.Contains(t, view, "Expected content")
}
```

**Reference**: `internal/screens/config_test.go:260-280`

**What to test**:
- ✅ Content presence (headers, data, status messages)
- ✅ State-dependent rendering (empty state, loading, error)
- ✅ Layout structure (verify sections exist)

**What NOT to test**:
- ❌ Exact ANSI escape codes
- ❌ Color values or styling
- ❌ Precise positioning
- ❌ Terminal dimensions beyond basic size handling

#### Pattern 3: Test Commands Return Expected Messages

Bubble Tea commands are functions that return messages. Test the message, not
the side effects:

```go
func TestRefreshCommand(t *testing.T) {
    cmd := screen.Refresh()

    // Execute command to get message
    msg := cmd()

    // Verify message type and content
    refreshMsg, ok := msg.(types.RefreshCompleteMsg)
    require.True(t, ok)
    assert.NotZero(t, refreshMsg.Duration)
}
```

**Reference**: `internal/screens/config_test.go:58-67`

#### Pattern 4: Table-Driven Tests for Coverage

Use table-driven tests to cover multiple scenarios efficiently:

```go
func TestInformerRepository_GetPods_PodStates(t *testing.T) {
    tests := []struct {
        name            string
        podName         string
        phase           corev1.PodPhase
        expectedStatus  string
        expectedReady   string
    }{
        {"running pod", "test-pod", corev1.PodRunning, "Running", "1/1"},
        {"crash loop", "crash-pod", corev1.PodRunning, "Running", "0/1"},
        {"pending", "pending-pod", corev1.PodPending, "Pending", "0/1"},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Create unique namespace per test
            ns := createTestNamespace(t)
            // ... test implementation
        })
    }
}
```

**Reference**: `internal/k8s/informer_repository_test.go:71-100`

#### Pattern 5: Namespace Isolation for Parallel Tests

When testing with real Kubernetes API (envtest), create unique namespaces:

```go
func createTestNamespace(t *testing.T) string {
    t.Helper()
    ns := &corev1.Namespace{
        ObjectMeta: metav1.ObjectMeta{
            GenerateName: "test-",  // Kubernetes generates unique name
        },
    }
    created, err := testClient.CoreV1().Namespaces().Create(...)
    require.NoError(t, err)

    // Auto-cleanup when test completes
    t.Cleanup(func() {
        testClient.CoreV1().Namespaces().Delete(...)
    })

    return created.Name
}
```

**Reference**: `internal/k8s/informer_repository_test.go:22-47`

**Benefits**:
- Tests can run in parallel
- No resource name conflicts
- Automatic cleanup
- Realistic multi-tenancy testing

---

### 3. Testing Tools and Libraries

#### Tools Used in k1 Project

| Tool | Purpose | Why It Works |
|------|---------|--------------|
| **envtest** | Real K8s API server | Tests actual behavior, not mocks |
| **testify/assert** | Cleaner assertions | Reduces boilerplate |
| **testify/require** | Stop-on-failure | Prevents cascade failures |
| **Standard Go testing** | Test runner | No special TUI runner needed |
| **Makefile** | Test orchestration | `make test`, `make test-coverage` |

**Setup commands**:

```bash
# One-time setup: Install envtest binaries (~50MB, cached)
make setup-envtest

# Run all tests (preferred method)
make test

# Run with coverage report
make test-coverage

# View coverage in browser
make test-coverage-html
```

**Reference**: `CLAUDE.md:54-73`, `Makefile:3-25`

#### Tools NOT Used (Notably Absent)

The k1 project achieves good coverage **without**:

- ❌ Specialized TUI testing frameworks
- ❌ Terminal emulators
- ❌ Screenshot comparison tools
- ❌ GUI automation tools (like Selenium for terminals)
- ❌ VHS or similar terminal recording tools for testing

**Why this works**: By testing the logic separately from rendering, most bugs
are caught without needing terminal simulation.

---

### 4. Current Test Coverage Analysis

#### What IS Tested (70-80% coverage)

**Business Logic Layer**:
- ✅ Data fetching and transformation (`k8s/transforms_test.go`)
- ✅ Filter logic and fuzzy search (`screens/config_test.go:74-143`)
- ✅ Command registry and lookup (`commands/registry_test.go`)
- ✅ Argument parsing (`commands/args_test.go`)
- ✅ Resource selection (`screens/config_test.go:145-173`)
- ✅ Navigation handlers (`screens/navigation_test.go`)
- ✅ Clipboard operations (`commands/clipboard_test.go`)

**Integration Layer**:
- ✅ Kubernetes informer initialization (`k8s/informer_repository_test.go:49`)
- ✅ Pod state transformations (`k8s/informer_repository_test.go:71-100`)
- ✅ Empty cluster queries
- ✅ Namespace isolation

**Some UI Testing**:
- ✅ Basic `View()` rendering (`screens/config_test.go:260-280`)
- ✅ System screen `Update()` (`screens/system_test.go`)
- ✅ Command bar executor (`components/commandbar/executor_test.go`)
- ✅ Suggestion palette (`components/commandbar/palette_test.go`)

#### What is NOT Tested (Major Gaps)

**UI Layer (10-20% coverage)**:

1. **App Update() orchestration** (`internal/app/app.go:150-249`)
   - ❌ Command bar height recalculation
   - ❌ Message forwarding between screen and command bar
   - ❌ Keyboard shortcuts (ctrl+y, ctrl+d, ctrl+l, ctrl+x)
   - ❌ Full-screen mode transitions
   - ❌ WindowSizeMsg propagation to all components
   - ❌ Global quit handling (q, ctrl+c)

2. **ConfigScreen Update()** (`internal/screens/config.go:150-198`)
   - ❌ Key message handling (KeyMsg types)
   - ❌ Custom update handler delegation
   - ❌ Enter key interception for navigation
   - ❌ Table update forwarding
   - ❌ Periodic refresh (tickMsg) for pods screen

3. **CommandBar state machine** (`internal/components/commandbar/commandbar.go:130-159`)
   - ❌ State transitions (Hidden → Filter → Palette → Input → Confirmation)
   - ❌ handleHiddenState, handleFilterState, handlePaletteState
   - ❌ Paste event handling
   - ❌ State-specific key routing
   - ❌ Height calculations with different states

4. **Components with zero tests**:
   - ❌ `internal/components/fullscreen.go` - Modal/fullscreen overlay
   - ❌ `internal/components/layout.go` - Layout calculations
   - ❌ `internal/components/header.go` - Header rendering
   - ❌ `internal/components/statusbar.go` - Status bar rendering

**Integration Layer (0% coverage)**:

- ❌ App + Screen + CommandBar working together
- ❌ Screen refresh → UI update → user interaction flows
- ❌ Command execution → result display → state restoration
- ❌ Full-screen mode → content display → exit sequence
- ❌ Complete user interaction flows (type → filter → navigate → select)

---

### 5. Testing Anti-Patterns to Avoid

Based on the k1 codebase experience:

1. **Don't mock Kubernetes client excessively**
   - ❌ Writing elaborate mocks that don't match real behavior
   - ✅ Use envtest for realistic API server behavior

2. **Don't test Lipgloss styling details**
   - ❌ Asserting exact ANSI color codes
   - ✅ Visual verification during manual testing

3. **Don't run full `tea.NewProgram()` in tests**
   - ❌ Starting terminal programs in test suite
   - ✅ Test `Update()` and `View()` directly

4. **Don't test exact ANSI output**
   - ❌ Comparing full rendered strings with escape codes
   - ✅ Test content presence with `assert.Contains()`

5. **Don't skip test namespaces**
   - ❌ Reusing same namespace causes flaky tests
   - ✅ Create unique namespace per test

---

### 6. Recommended Testing Strategy

For a comprehensive Bubble Tea testing approach:

#### Level 1: Unit Tests (Required - 70%+ coverage)

**What to test**:
- Business logic (data transformation, filtering, state management)
- Message handlers (`Update()` method logic)
- Command functions (return correct messages)
- View rendering (content presence, not styling)

**Example from k1**:
```go
// Test filter logic
func TestConfigScreen_SetFilter(t *testing.T) {
    screen := NewConfigScreen(cfg, repo, theme)
    screen.Refresh()()  // Populate with dummy data

    screen.SetFilter("nginx")

    // Test filtered results
    assert.LessOrEqual(t, len(screen.filtered), initialCount)

    // Verify view contains filtered content
    view := screen.View()
    assert.Contains(t, view, "nginx")
}
```

**Reference**: `internal/screens/config_test.go:74-106`

#### Level 2: Integration Tests (Recommended - 50%+ coverage)

**What to test**:
- Components working together (but not full program)
- Message flow between components
- State transitions across boundaries
- Real external dependencies (database, API, etc.)

**Example approach** (not yet in k1):
```go
func TestAppScreenCommandBarIntegration(t *testing.T) {
    // Create app with screen and command bar
    app := NewModel(repo, theme)

    // Simulate user typing ":"
    app, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{':'}})

    // Verify command bar entered palette mode
    assert.Equal(t, commandbar.StateSuggestionPalette, app.commandBar.state)

    // Simulate arrow key navigation
    app, _ = app.Update(tea.KeyMsg{Type: tea.KeyDown})

    // Verify palette selection changed
    // ... assertions
}
```

#### Level 3: End-to-End Tests (Optional - Manual)

**What to test**:
- Complete user workflows
- Visual appearance
- Performance and responsiveness
- Cross-platform terminal compatibility

**Approach**: Run the application manually:
```bash
# Test with dummy data
make run-dummy

# Test with live cluster
make run

# Test different themes
go run cmd/k1/main.go -theme nord
```

**When to automate**:
- Consider automation only if e2e bugs are frequent
- Use tools like VHS for recording expected behavior
- Keep e2e tests minimal (slow and brittle)

---

### 7. Test Organization and Infrastructure

#### File Organization
k1 follows Go conventions with `*_test.go` files:

```
internal/
├── app/
│   ├── app.go
│   └── app_test.go              # App routing tests
├── screens/
│   ├── config.go
│   ├── config_test.go           # Screen logic tests (1057 lines)
│   ├── navigation.go
│   └── navigation_test.go       # Navigation tests (530 lines)
├── components/
│   ├── commandbar/
│   │   ├── commandbar.go
│   │   ├── commandbar_test.go   # (missing - gap)
│   │   ├── executor.go
│   │   ├── executor_test.go     # Executor tests (235 lines)
│   │   ├── palette.go
│   │   └── palette_test.go      # Palette tests (189 lines)
│   ├── header.go                # No tests (gap)
│   └── layout.go                # No tests (gap)
└── k8s/
    ├── suite_test.go            # Shared TestMain setup
    ├── informer_repository.go
    └── informer_repository_test.go  # Integration tests (2227 lines)
```

#### Shared Test Setup

For expensive setup (like starting a Kubernetes API server), use `TestMain`:

```go
// suite_test.go
var (
    testEnv    *envtest.Environment
    testCfg    *rest.Config
    testClient kubernetes.Interface
)

func TestMain(m *testing.M) {
    // Setup runs ONCE before all tests
    testEnv = &envtest.Environment{}
    testCfg, _ = testEnv.Start()
    testClient, _ = kubernetes.NewForConfig(testCfg)

    // Run all tests
    code := m.Run()

    // Teardown
    testEnv.Stop()
    os.Exit(code)
}
```

**Reference**: `internal/k8s/suite_test.go`

**Benefits**:
- 5s startup instead of 5s per test
- Shared resources across tests
- Proper cleanup guaranteed

#### Test Helpers

```go
// Create unique namespace per test
func createTestNamespace(t *testing.T) string {
    t.Helper()
    ns := &corev1.Namespace{
        ObjectMeta: metav1.ObjectMeta{GenerateName: "test-"},
    }
    created, _ := testClient.CoreV1().Namespaces().Create(...)

    t.Cleanup(func() {
        testClient.CoreV1().Namespaces().Delete(...)
    })

    return created.Name
}
```

**Reference**: `internal/k8s/informer_repository_test.go:22-47`

---

### 8. Test Coverage Goals

From `CLAUDE.md:12-22`:

```
Test Coverage Requirements:
- New components: 70% minimum
- Modified components: Cannot decrease coverage
- Critical paths: 80% minimum
- Write tests DURING implementation, not after
```

**Current k1 coverage**:
- k8s package: 76.7%
- screens package: 71.0%
- Overall: Meets project quality gates

**How to measure**:
```bash
make test-coverage
go tool cover -func=coverage.out
```

**Sample output**:
```
github.com/renato0307/k1/internal/k8s/informer_repository.go:45:  Init        100.0%
github.com/renato0307/k1/internal/k8s/transforms.go:23:          transformPod 87.5%
github.com/renato0307/k1/internal/screens/config.go:52:          Update      45.2%
```

---

### 9. Testing Bubble Tea Components - Practical Guide

#### Testing a Screen

```go
// 1. Initialize with dependencies
func TestMyScreen(t *testing.T) {
    repo := k8s.NewDummyRepository()  // Or real repo with envtest
    theme := ui.CharmTheme
    config := GetMyScreenConfig()

    screen := NewConfigScreen(config, repo, theme)

    // 2. Set window size (important for layout)
    screen.SetSize(80, 24)

    // 3. Trigger data loading
    cmd := screen.Refresh()
    msg := cmd()  // Execute command to get data

    // 4. Test Update() with message
    newScreen, _ := screen.Update(msg)

    // 5. Test View() output
    view := newScreen.(ConfigScreen).View()
    assert.Contains(t, view, "expected content")
}
```

#### Testing Message Handling

```go
func TestScreenHandlesKeyPress(t *testing.T) {
    screen := NewConfigScreen(cfg, repo, theme)

    tests := []struct {
        name     string
        key      tea.KeyMsg
        expected string  // Expected behavior
    }{
        {"enter key", tea.KeyMsg{Type: tea.KeyEnter}, "navigates"},
        {"escape key", tea.KeyMsg{Type: tea.KeyEsc}, "clears filter"},
        {"colon key", tea.KeyMsg{Runes: []rune{':'}}, "opens palette"},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            newScreen, cmd := screen.Update(tt.key)

            // Assert behavior based on expected
            if tt.expected == "navigates" {
                assert.NotNil(t, cmd)
                msg := cmd()
                _, ok := msg.(types.ScreenSwitchMsg)
                assert.True(t, ok)
            }
            // ... other cases
        })
    }
}
```

#### Testing Commands

```go
func TestCommandExecution(t *testing.T) {
    // Create command with context
    ctx := commands.CommandContext{
        Screen:     "pods",
        Repository: repo,
        Args:       []string{"nginx-pod"},
    }

    // Execute command function
    cmdFunc := commands.DeleteCommand()
    resultCmd := cmdFunc(ctx)

    // Execute the tea.Cmd to get message
    msg := resultCmd()

    // Assert message type and content
    statusMsg, ok := msg.(types.StatusMsg)
    require.True(t, ok)
    assert.Contains(t, statusMsg.Message, "deleted")
}
```

---

## Code References

### Test Files
- `internal/screens/config_test.go` - ConfigScreen tests (1057 lines)
- `internal/screens/navigation_test.go` - Navigation tests (530 lines)
- `internal/screens/system_test.go` - System screen tests (204 lines)
- `internal/app/app_test.go` - App routing tests (244 lines)
- `internal/k8s/informer_repository_test.go` - Integration tests (2227 lines)
- `internal/k8s/suite_test.go` - Shared test setup with TestMain
- `internal/components/commandbar/executor_test.go` - Executor tests (235 lines)
- `internal/components/commandbar/palette_test.go` - Palette tests (189 lines)

### Implementation Files (Untested Gaps)
- `internal/app/app.go:150-249` - App Update() method (not tested)
- `internal/screens/config.go:150-198` - ConfigScreen Update() (not tested)
- `internal/components/commandbar/commandbar.go:130-159` - State machine (not tested)
- `internal/components/fullscreen.go` - Zero test coverage
- `internal/components/layout.go` - Zero test coverage
- `internal/components/header.go` - Zero test coverage

### Test Infrastructure
- `Makefile:3-25` - Test targets and setup
- `CLAUDE.md:54-73` - Testing guidelines and commands

---

## Architecture Insights

### Key Discoveries

1. **No specialized TUI framework needed**: Standard Go testing works well for
   Bubble Tea applications when you test the right layers.

2. **Separation of concerns enables testing**: By separating business logic,
   message handling, and rendering, each layer can be tested independently.

3. **Message-driven architecture is testable**: Commands return messages,
   Update() handles messages, View() renders state. Each part is independently
   testable.

4. **Real dependencies over mocks**: Using envtest for a real Kubernetes API
   server provides more confidence than elaborate mocks.

5. **Manual testing still important**: Some aspects (UX, visual appearance,
   cross-platform behavior) are better verified manually.

### Testing Philosophy

From the k1 project's approach:

> **Effective Bubble Tea testing doesn't require specialized frameworks**.
> Instead:
> 1. Test business logic separately
> 2. Test Update() logic directly
> 3. Test View() output as strings
> 4. Use real dependencies where practical
> 5. Manual testing for UX
> 6. Aim for 70%+ coverage, focus on critical paths

**Result**: 71-76% test coverage with fast execution (~5-10 seconds) and
maintainable tests.

---

## Gaps and Limitations

### What This Research Doesn't Cover

1. **Official Bubble Tea Testing Docs**: WebFetch API was unable to access
   charm.sh documentation or GitHub. However, the k1 project demonstrates
   real-world patterns that work in production.

2. **Other Bubble Tea Projects**: Could not research how other projects (gh,
   glow, soft-serve, vhs) test their TUIs due to WebFetch limitations.

3. **Visual Regression Testing**: No automated screenshot comparison tools were
   found. The k1 team relies on manual visual QA during development.

4. **Performance Testing**: While k1 has performance tests for data indexing,
   there's no automated TUI performance testing (frame rate, input lag, render
   time).

5. **Cross-platform Testing**: No information on automated testing across
   different terminals (iTerm2, Alacritty, Windows Terminal, etc.).

### Recommended Next Steps for Research

If web access becomes available:

1. **Check Bubble Tea GitHub**:
   - Search issues for "testing" discussions
   - Look at test files in bubbletea repo itself
   - Check for any testing utilities in bubbles library

2. **Review Popular Projects**:
   - `cli/cli` (GitHub CLI) - How does it test TUI components?
   - `charmbracelet/glow` - Markdown renderer test suite
   - `charmbracelet/soft-serve` - Git server TUI testing
   - `charmbracelet/vhs` - Could this be used for testing?

3. **Search for Blog Posts**:
   - "bubble tea testing" on dev.to, medium.com
   - Charm blog posts about testing strategies

4. **Explore Testing Libraries**:
   - Check if Charm provides testing utilities
   - Look for community testing libraries

---

## Open Questions

1. **Should we test Update() methods more comprehensively?**
   - Current gap: Most Update() methods are untested
   - Trade-off: More coverage vs. test maintenance burden
   - Recommendation: Test critical Update() paths (navigation, state
     transitions)

2. **Is integration testing worth the complexity?**
   - Current gap: Zero integration tests (app + screen + commandbar)
   - Trade-off: More confidence vs. slower, more fragile tests
   - Recommendation: Add a few smoke tests for critical flows

3. **How to test visual rendering systematically?**
   - Current approach: Manual visual QA
   - Potential: VHS recordings as "expected" output?
   - Trade-off: Automation vs. brittleness from ANSI code changes

4. **Should we test components in isolation?**
   - Current gap: header.go, layout.go, fullscreen.go have no tests
   - Question: Are these simple enough to skip unit tests?
   - Recommendation: Add basic tests for layout calculations at minimum

5. **What's the right coverage target for TUI apps?**
   - Current: 71-76% overall
   - Question: Should UI layer have same target as logic layer?
   - Recommendation: 70% for logic, 50% for UI, 80% for critical paths

---

## Conclusion

Automatic TUI testing with Bubble Tea is **practical and achievable using
standard Go testing tools**. The k1 project demonstrates that a pragmatic
approach—testing business logic thoroughly, testing Update() and View()
methods directly, and using manual testing for UX verification—can achieve
high test coverage (70%+) while keeping tests fast and maintainable.

**Key takeaways**:

1. ✅ **No specialized frameworks needed** - Standard Go testing works
2. ✅ **Test components directly** - Don't run full tea.Program() in tests
3. ✅ **Use real dependencies** - envtest, not mocks
4. ✅ **Test content, not styling** - assert.Contains(), not ANSI codes
5. ✅ **Manual testing for UX** - Some things are better verified by hand
6. ⚠️ **Integration testing gap** - Consider adding smoke tests
7. ⚠️ **UI layer coverage** - Many Update() methods untested

The most significant opportunity for improvement in k1 is **testing Update()
methods** more comprehensively and adding **integration tests** for critical
user flows.
