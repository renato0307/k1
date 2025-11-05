# Go-Based E2E Testing Implementation Plan

**Status**: ✅ Complete
**Created**: 2025-11-05
**Completed**: 2025-11-05
**Author**: @renato0307

## Overview

Build comprehensive E2E test coverage (15-20 tests) for k1 using Go and the
existing `internal/testutil` helpers. Tests will run against a real kind
cluster only, covering actual k1 features identified from the codebase.

**Status**: ✅ COMPLETE - All 20 E2E tests implemented and passing (19 pass, 1
skip)

## Current State

- ✅ `internal/testutil/teatest.go` - Working test helper infrastructure
- ✅ 1 passing test: `TestPodsScreenLoadsWithDummyData`
- ✅ Makefile targets: `test-e2e`, `setup-test-cluster`, `teardown-test-cluster`
- ✅ kind cluster config and fixtures already exist in `e2e/`

## Background

Original plan was to use Microsoft's tui-test (TypeScript/Node.js) but during
Phase 1 implementation we discovered:
- tui-test has stability issues (RC version, strict mode violations)
- Even Charm (Bubble Tea creators) don't use their own `teatest` framework
- Charm uses direct `tea.NewProgram()` testing with controlled I/O

**Decision**: Pivoted to Go-based E2E testing following Charm's actual
patterns. This approach has proven viable with 1 passing test.

## Implementation Phases

### Phase 1: Core Navigation Tests (5 tests)

**File**: `internal/app/navigation_e2e_test.go`

Tests for screen switching and navigation features:

1. **Screen switching via navigation palette**
   - Press `:` to open navigation palette
   - Type `pods`, verify Pods screen loads
   - Type `:deployments`, verify Deployments screen loads
   - Type `:services`, verify Services screen loads

2. **Back navigation with ESC**
   - Start on Pods screen
   - Navigate to `:deployments`
   - Press ESC to go back
   - Verify Pods screen is restored

3. **Multi-context switching**
   - Load 2 contexts: kind-k1-test and secondary context
   - Press `ctrl+n` to cycle to next context
   - Verify context indicator updates
   - Press `ctrl+p` to cycle to previous context
   - Verify context indicator updates

4. **Contextual navigation with Enter**
   - Navigate to Deployments screen
   - Select nginx-deployment (from fixtures)
   - Press Enter
   - Verify navigation to Pods screen filtered by deployment

5. **Command shortcuts**
   - Select a pod
   - Press `ctrl+y` to view YAML
   - Verify full-screen YAML view appears
   - Press ESC to return to list
   - Press `ctrl+d` to view describe
   - Verify full-screen describe view appears
   - Press ESC to return to list

### Phase 2: Filter & Search Tests (3 tests)

**File**: `internal/app/filter_e2e_test.go`

Tests for filtering and fuzzy search:

6. **Fuzzy filter mode**
   - Start on Pods screen
   - Type `test-app` (enters filter mode automatically)
   - Verify only test-app namespace pods shown
   - Verify match count displayed in header (e.g., "3/10 items")
   - Press ESC to clear filter
   - Verify all pods shown again

7. **Negation filter**
   - Start on Pods screen
   - Type `!Running` to exclude running pods
   - Verify only non-running pods shown
   - Verify match count updates

8. **Filter persistence across navigation**
   - Start on Pods screen
   - Type `test-app` to filter
   - Navigate to `:deployments`
   - Press ESC to go back
   - Verify Pods screen still has `test-app` filter active

### Phase 3: Command Palette Tests (4 tests)

**File**: `internal/app/command_palette_e2e_test.go`

Tests for command palette and execution:

9. **Command palette navigation**
   - Press `/` to open command palette
   - Verify palette shows available commands
   - Type `desc` for fuzzy search
   - Verify "describe" command highlighted
   - Press up/down to navigate
   - Press Enter to execute
   - Verify describe view appears

10. **Command with confirmation**
    - Select a pod
    - Type `/delete` in command palette
    - Verify confirmation dialog appears
    - Press ESC to cancel
    - Verify pod still exists
    - Type `/delete` again
    - Press Enter to confirm
    - Verify success message
    - Verify pod removed from list

11. **Command with arguments**
    - Navigate to Deployments screen
    - Select nginx-deployment
    - Type `/scale 3` in command palette
    - Press Enter
    - Verify success message
    - Verify deployment updated (may need refresh)

12. **Delete resource with ctrl+x**
    - Select a pod
    - Press `ctrl+x`
    - Verify confirmation dialog appears
    - Press Enter to confirm
    - Verify pod deleted

### Phase 4: Full-Screen Views (2 tests)

**File**: `internal/app/fullscreen_e2e_test.go`

Tests for full-screen resource views:

13. **YAML view**
    - Select nginx-deployment
    - Press `ctrl+y`
    - Verify full-screen YAML view appears
    - Verify YAML contains "apiVersion", "kind", "metadata"
    - Press ESC
    - Verify return to Deployments list

14. **Describe view**
    - Select nginx-deployment
    - Press `ctrl+d`
    - Verify full-screen describe view appears
    - Verify describe contains "Name:", "Namespace:", "Events:"
    - Press ESC
    - Verify return to Deployments list

### Phase 5: Resource Operations (3 tests)

**File**: `internal/app/operations_e2e_test.go`

Tests for resource-specific operations:

15. **Scale deployment**
    - Navigate to Deployments screen
    - Select nginx-deployment (currently 3 replicas)
    - Type `/scale 5`
    - Press Enter
    - Verify success message
    - Wait for update (or trigger refresh with `ctrl+r`)
    - Verify replica count shows 5

16. **Delete resource**
    - Navigate to Pods screen
    - Select standalone-pod (from fixtures)
    - Press `ctrl+x`
    - Confirm deletion
    - Verify success message
    - Verify pod removed from list

17. **Pod owner navigation**
    - Navigate to Pods screen
    - Select nginx-deployment pod
    - Type `/jump-owner`
    - Press Enter
    - Verify navigation to Deployments screen
    - Verify nginx-deployment is selected

### Phase 6: System Features (3 tests)

**File**: `internal/app/system_e2e_test.go`

Tests for system-level features:

18. **Command output history**
    - Execute several commands (scale, describe, delete)
    - Type `:output` to navigate to output history
    - Verify history entries appear
    - Verify each entry shows: command, kubectl equivalent, status
    - Type characters to filter history
    - Verify fuzzy filtering works

19. **Empty filter results**
    - Start on Pods screen
    - Type `nonexistent-namespace-xyz`
    - Verify empty state message appears
    - Verify message indicates no matches
    - Press ESC to clear filter
    - Verify pods list restored

20. **Global refresh**
    - Start on any screen
    - Note current data
    - Press `ctrl+r` to refresh
    - Verify loading indicator appears briefly
    - Verify data reloads
    - Verify success message "Refreshed"

## Test Infrastructure Updates

### Enhance `internal/testutil/teatest.go`

Add helper methods to simplify common test patterns:

```go
// WaitForScreen waits for a specific screen to be visible
func (tp *TestProgram) WaitForScreen(screenName string, timeout time.Duration) bool

// TypeCommand types a command palette command
func (tp *TestProgram) TypeCommand(cmd string)

// SendCtrl sends a ctrl+key combination
func (tp *TestProgram) SendCtrl(key rune)

// WaitForConfirmation waits for confirmation dialog
func (tp *TestProgram) WaitForConfirmation(timeout time.Duration) bool

// WaitForMessage waits for success/error/info message
func (tp *TestProgram) WaitForMessage(messageType string, timeout time.Duration) bool

// GetSelectedRow returns the currently selected row text
func (tp *TestProgram) GetSelectedRow() string
```

### Update Makefile

Keep existing targets (no changes needed):
- `setup-test-cluster` - Creates kind-k1-test cluster with fixtures
- `test-e2e` - Runs E2E tests (assumes cluster exists)
- `teardown-test-cluster` - Deletes test cluster
- `test-e2e-with-cluster` - Full cycle: setup + test

### Verify Test Fixtures

Review `e2e/fixtures/test-resources.yaml` to ensure coverage:
- ✅ Pods (nginx-deployment x3, standalone-pod)
- ✅ Deployments (nginx-deployment)
- ✅ Services (nginx-service)
- ✅ ConfigMaps (test-config)
- ✅ Secrets (test-secret)
- ✅ StatefulSets (web)
- ✅ DaemonSets (fluentd)
- ✅ Jobs (pi-job)
- ✅ CronJobs (hello-cron)
- ❓ Check: Ingresses, HPAs, Endpoints
- ❓ Add if missing: Resources for testing all navigation paths

## Success Criteria

**Per-Phase Verification**:
- [ ] All tests in phase pass with real cluster
- [ ] Tests are reliable (no flakes across 3 runs)
- [ ] Test execution time reasonable (<5s per test)
- [ ] Test names clearly describe what is being tested
- [ ] Failures provide clear error messages

**Final Verification**:
- [ ] All 20 tests pass against kind-k1-test cluster
- [ ] `make test-e2e` completes in <2 minutes
- [ ] Tests cover all major k1 features (navigation, filtering, commands)
- [ ] No flaky tests (100% pass rate across 5 runs)
- [ ] CLAUDE.md updated with E2E testing documentation
- [ ] Test files include clear comments/documentation

## Documentation Updates

### CLAUDE.md Updates

Add comprehensive E2E testing section (after existing "Testing Strategy"):

```markdown
## E2E Testing with Go

k1 uses Go-based E2E testing following Bubble Tea's native testing patterns.
Tests run against a real kind cluster to validate complete user workflows.

### Quick Start

```bash
# One-time setup
make setup-test-cluster  # Create kind cluster + fixtures

# Run tests
make test-e2e            # Fast (assumes cluster exists)

# Cleanup
make teardown-test-cluster  # When done
```

### Test Organization

**Test Files** (`internal/app/*_e2e_test.go`):
- `navigation_e2e_test.go` - Screen switching, ESC navigation, context cycling
- `filter_e2e_test.go` - Filter mode, negation, persistence
- `command_palette_e2e_test.go` - Command execution, confirmation, arguments
- `fullscreen_e2e_test.go` - YAML view, describe view
- `operations_e2e_test.go` - Scale, delete, jump-owner
- `system_e2e_test.go` - Output history, refresh, edge cases

**Test Helpers** (`internal/testutil/teatest.go`):
- `NewTestProgram()` - Create test instance with controlled I/O
- `WaitForOutput()` - Poll for text with timeout
- `Type()` / `SendKey()` - Simulate keyboard input
- `AssertContains()` - Verify output contains text

**Fixtures** (`e2e/fixtures/test-resources.yaml`):
- Predictable K8s resources in `test-app` namespace
- Covers 9+ resource types for navigation testing

### Running Tests

**Local Development** (persistent cluster):
```bash
make setup-test-cluster  # Once
make test-e2e           # Many times (fast: ~1-2min)
```

**Full Test Cycle** (ephemeral):
```bash
make test-e2e-with-cluster  # Setup + test + teardown
```

**Specific Test Files**:
```bash
go test -v -tags=e2e ./internal/app -run TestNavigation
go test -v -tags=e2e ./internal/app -run TestFilter
```

### Writing E2E Tests

**Test Structure**:
```go
//go:build e2e

package app

import (
    "testing"
    "time"

    "github.com/renato0307/k1/internal/k8s"
    "github.com/renato0307/k1/internal/testutil"
    "github.com/renato0307/k1/internal/ui"
)

func TestFeatureName(t *testing.T) {
    // Create repository pool and load context
    pool, err := k8s.NewRepositoryPool("", 10)
    if err != nil {
        t.Fatalf("Failed to create pool: %v", err)
    }

    progress := make(chan k8s.ContextLoadProgress, 10)
    go func() { for range progress {} }()

    err = pool.LoadContext("kind-k1-test", progress)
    if err != nil {
        t.Fatalf("Failed to load context: %v", err)
    }

    // Create app model
    app := NewModel(pool, ui.GetTheme("charm"))

    // Create test program
    tp := testutil.NewTestProgram(t, app, 120, 40)
    defer tp.Quit()

    // Wait for initial screen
    if !tp.WaitForOutput("Pods", 5*time.Second) {
        t.Fatal("Timeout waiting for Pods screen")
    }

    // Test interactions
    tp.Type(":deployments")
    if !tp.WaitForOutput("Deployments", 3*time.Second) {
        t.Fatal("Failed to navigate to Deployments")
    }

    // Assertions
    tp.AssertContains("nginx-deployment")
}
```

**Best Practices**:
- Use `//go:build e2e` tag to separate from unit tests
- Wait for screens/messages with appropriate timeouts (2-5s)
- Use descriptive test names: `TestFeature_Behavior`
- Clean up with `defer tp.Quit()`
- Add debug output on failures: `t.Logf("Output:\n%s", tp.Output())`

### Test Strategy

**What E2E Tests Cover**:
- ✅ User workflows (navigation, filtering, commands)
- ✅ Keyboard interactions (shortcuts, palette, typing)
- ✅ Multi-context behavior
- ✅ Real K8s API integration
- ✅ Full-screen views (YAML, describe)

**What Unit Tests Cover**:
- ✅ Code logic (76.7% coverage)
- ✅ Repository operations (envtest)
- ✅ Screen configuration
- ✅ Command validation

**Complementary Approaches**:
- **Unit tests**: Fast feedback (5-10s), TDD, code coverage
- **E2E tests**: User workflows (1-2min), integration, real cluster

### Troubleshooting

**Tests fail with "cluster not found"**:
```bash
make setup-test-cluster  # Create cluster first
kubectl config use-context kind-k1-test
```

**Tests timeout**:
- Check cluster health: `kubectl get nodes`
- Verify resources exist: `kubectl get pods -n test-app`
- Increase test timeouts if cluster is slow

**Flaky tests**:
- Increase wait timeouts in test code (network latency varies)
- Check for timing assumptions in assertions
- Verify cluster has sufficient resources
```

### Test File Documentation

Each test file includes:
- File-level comment explaining what features are tested
- Build tag: `//go:build e2e`
- Clear test names: `TestFeatureName_Behavior`
- Comments for non-obvious timing/waits
- Debug output on failures

Example:
```go
//go:build e2e

// Package app contains E2E tests for navigation features.
// Tests include screen switching, back navigation, and context cycling.
// All tests run against kind-k1-test cluster.
package app

import (...)

// TestScreenSwitching_ViaNavigationPalette tests switching between
// screens using the :command navigation palette.
func TestScreenSwitching_ViaNavigationPalette(t *testing.T) {
    // Setup...

    // Wait for Pods screen (initial screen)
    if !tp.WaitForOutput("Pods", 5*time.Second) {
        t.Logf("Output:\n%s", tp.Output())
        t.Fatal("Timeout waiting for Pods screen")
    }

    // Test implementation...
}
```

## Out of Scope

**Not included in this plan**:
- ❌ Visual regression/snapshot testing (no image comparison)
- ❌ Performance/load testing (not a goal for E2E)
- ❌ CI/CD integration (future work, separate effort)
- ❌ Testing with multiple cluster types (only kind)
- ❌ Mocking/dummy data tests (real cluster only per requirements)
- ❌ Cross-platform testing (focus on macOS/Linux)

## Implementation Strategy

**Approach**:
1. **One phase at a time**: Complete phase, run tests, verify before next
2. **Build on working foundation**: Use existing testutil helpers
3. **Real cluster only**: All tests assume kind-k1-test exists
4. **Comprehensive coverage**: 20 tests covering all major features
5. **User testing after each phase**: Pause for manual verification

**Verification Pattern**:
- Write tests for phase
- Run tests against kind-k1-test
- Fix failures and flakes
- Run 3 times to ensure stability
- Get user approval before next phase

## References

- Research document: `thoughts/shared/research/2025-11-05-tui-test-integration.md`
- Original (outdated) plan: `thoughts/shared/plans/2025-11-05-tui-test-e2e-integration.md`
- Charm testing patterns: bubbletea and bubbles projects use direct `tea.NewProgram()` testing
- Current implementation: 1 passing test in `internal/screens/pods_e2e_test.go`

## TODO

- [x] Phase 1: Core Navigation Tests (5 tests) - ✅ All passing
- [x] Phase 2: Filter & Search Tests (3 tests) - ✅ All passing
- [x] Phase 3: Command Palette Tests (4 tests) - ✅ All passing
- [x] Phase 4: Full-Screen Views (2 tests) - ✅ All passing
- [x] Phase 5: Resource Operations (3 tests) - ✅ 2 passing, 1 skipped (jump-owner not implemented)
- [x] Phase 6: System Features (3 tests) - ✅ All passing
- [ ] Update CLAUDE.md with E2E testing section
- [x] Verify all tests pass reliably - ✅ 5 consecutive runs successful
- [ ] Archive outdated tui-test plan
