# Remaining Architectural Issues After PLAN-08

| Metadata | Value                      |
|----------|----------------------------|
| Date     | 2025-10-07                 |
| Author   | @renato0307                |
| Status   | `Accepted`                 |
| Tags     | refactoring, architecture, technical-debt, plan-09 |
| Updates  | -                          |

| Revision | Date       | Author      | Info                              |
|----------|------------|-------------|-----------------------------------|
| 1        | 2025-10-07 | @renato0307 | Post-PLAN-08 architectural audit  |

## Context and Problem Statement

Following completion of PLAN-08 (DDR-14 medium priority items), we have
significantly improved code quality with:
- Code duplication reduced by ~500 lines (30%)
- Test coverage: commands 0% → 65.9%
- CommandBar split from 1257 lines → 4 focused components
- Error handling standardized with messages package
- Constants extracted to per-package pattern
- Type safety improved with ResourceType/ResourceInfo

However, DDR-14 identified 10 architectural issues. What architectural
problems remain unresolved after PLAN-08, and what is the prioritized
strategy to address them?

## References

- DDR-14: Original code quality audit (2025-10-06)
- PLAN-08: Medium priority refactoring (Phases 1-4 complete)
- Codebase analysis: Post-PLAN-08 state (2025-10-07)

## Analysis

### What Was Fixed by PLAN-08

**Code Smells** (DDR-14 Items 1-4):
✅ Magic numbers extracted to constants
✅ Code duplication reduced (sortByAge, TransformFunc, navigation)
✅ Primitive obsession addressed (ResourceType, ResourceInfo)
✅ CommandBar god object split into 4 components

**Architectural Items** (DDR-14 Items 6-8):
✅ Transform abstraction created
✅ Screen abstraction (config-driven)
✅ Command execution abstraction (KubectlExecutor)
✅ Error handling standardized
✅ Test coverage improved significantly

### Remaining Architectural Issues

#### 1. Tight Coupling (NOT FIXED) ⚠️

**Commands → Repository Interface Violation**

Commands depend on full `k8s.Repository` interface (15+ methods) but
only use 2 methods:
```go
// Current (tight coupling)
func ScaleCommand(repo k8s.Repository) ExecuteFunc {
    executor := NewKubectlExecutor(repo.GetKubeconfig(), repo.GetContext())
    // Uses only GetKubeconfig() and GetContext()
}
```

**Problem**: Commands are tightly coupled to Repository, violating
Interface Segregation Principle. Hard to test, hard to mock.

**Recommended Fix**:
```go
// Define minimal interface
type KubeconfigProvider interface {
    GetKubeconfig() string
    GetContext() string
}

// Commands depend on minimal interface
func ScaleCommand(provider KubeconfigProvider) ExecuteFunc
```

**Impact**: High - Affects all 20+ command functions
**Effort**: Medium (2-3 days) - Create interface, update all commands

---

#### 2. InformerRepository SRP Violation (NOT FIXED) ⚠️

**InformerRepository is still a god object** (748 lines, 12 public
methods)

Responsibilities:
- Manages 11 typed informers + dynamic informers
- Transforms unstructured data to typed structs (11 transform functions)
- Generates YAML output (kubectl library integration)
- Fetches events on-demand
- Sorts resources
- Provides kubeconfig/context for commands

**Problem**: Violates Single Responsibility Principle. Hard to test,
hard to understand, fragile.

**Recommended Fix** - Split into 3 components:
```go
// 1. InformerManager - manages informers lifecycle
type InformerManager interface {
    Start(ctx context.Context) error
    WaitForCacheSync(ctx context.Context) error
    Close()
}

// 2. ResourceRepository - data access only
type ResourceRepository interface {
    GetPods() ([]Pod, error)
    GetDeployments() ([]Deployment, error)
    // ... typed getters
}

// 3. ResourceFormatter - YAML/describe generation
type ResourceFormatter interface {
    GetResourceYAML(gvr, ns, name string) (string, error)
    DescribeResource(gvr, ns, name string) (string, error)
}
```

**Impact**: High - Core data layer refactoring
**Effort**: High (5-7 days) - Major refactoring with extensive testing

---

#### 3. Resource Management Issues (NOT FIXED) ⚠️

**Close() has no guard against multiple calls**

```go
// Current implementation
func (r *InformerRepository) Close() {
    if r.cancel != nil {
        r.cancel()  // Calling twice may panic or cause issues
    }
}
```

**Problem**: Calling Close() multiple times is unsafe. No protection
against double-close.

**Recommended Fix**:
```go
func (r *InformerRepository) Close() {
    r.mu.Lock()
    defer r.mu.Unlock()

    if r.closed {
        return
    }
    r.closed = true

    if r.cancel != nil {
        r.cancel()
    }
}
```

**Unchecked context cancellation in goroutines**

Some goroutines don't properly check `ctx.Done()` for cancellation.
Need comprehensive audit of all goroutines.

**Impact**: High - Resource leaks, goroutine leaks
**Effort**: Low-Medium (1-2 days) - Audit + fix

---

#### 4. Theme Propagation Anti-Pattern (NOT FIXED)

**Theme pointer passed to every component**

```go
// Current pattern - theme passed explicitly
func NewConfigScreen(cfg ScreenConfig, repo k8s.Repository,
                     theme *ui.Theme) *ConfigScreen

func NewHeader(title string, theme *ui.Theme) *Header
func NewStatusBar(theme *ui.Theme) *StatusBar
// ... 15+ components
```

**Problem**: Theme coupling throughout codebase. Hard to change theme
system, hard to add theme switching.

**Recommended Fix** - Use context pattern:
```go
type AppContext struct {
    Theme      *ui.Theme
    Repository k8s.Repository
    // ... other app-wide state
}

func NewConfigScreen(cfg ScreenConfig, ctx *AppContext) *ConfigScreen
```

**Impact**: Medium - Better architecture, easier theme switching
**Effort**: Medium (2-3 days) - Update all component constructors

---

#### 5. Testing Architecture Issues (PARTIALLY FIXED)

**CommandBar test coverage: 32.1% (target: 70%+)**

CommandBar split into 4 components with 817 lines of tests, but
coverage is still below target:
- commandbar.go (701 lines) - main coordinator, low coverage
- executor.go (230 lines) - well tested
- history.go (91 lines) - well tested
- input.go (256 lines) - well tested
- palette.go (213 lines) - well tested

**Problem**: Main commandbar.go coordinator has gaps in test coverage.

**No test helpers - mockRepository duplicated**

Manual mocks duplicated across test files:
- commands/command_execution_test.go
- commands/clipboard_test.go
- (future test files will duplicate again)

**Recommended Fix**:
```go
// internal/testing/mocks.go
type MockRepository struct {
    kubeconfig string
    context    string
}

func NewMockRepository() *MockRepository { ... }
```

**Integration tests mixed with unit tests**

envtest tests (~5s startup) mixed with fast unit tests. No separation
by build tags or directories.

**Recommended Fix**:
```
internal/k8s/
  repository.go
  repository_test.go          // Fast unit tests
  repository_integration_test.go  // +build integration
```

**Impact**: Medium - Better test experience, faster CI
**Effort**: Medium (2-3 days) - Refactor tests, add helpers

---

#### 6. User Configuration (PARTIALLY FIXED)

**Constants extracted, but no user override mechanism**

Per-package constants provide good defaults:
```go
const (
    DefaultKubectlTimeout = 30 * time.Second
    MaxPaletteItems = 8
)
```

But users cannot override these without recompiling.

**Recommended Fix** - Add config file support:
```
~/.config/k1/config.yaml:
  timeouts:
    kubectl: 60s
  ui:
    palette_items: 12
    theme: dracula
```

**Impact**: Medium - User experience improvement
**Effort**: Medium (2-3 days) - Config loading + validation

---

#### 7. State Validation Missing (NOT FIXED)

**No validation on state transitions**

CommandBar state machine has 7 states but no validation that
transitions are valid:
```go
// Can transition from any state to any state - no guards
func (cb *CommandBar) setState(newState State) {
    cb.state = newState  // No validation
}
```

**Problem**: Possible invalid states, hard to debug state bugs.

**Recommended Fix** - State machine validation:
```go
var validTransitions = map[State][]State{
    StateHidden: {StateFilter, StatePalette},
    StateFilter: {StateHidden, StatePalette},
    // ... define all valid transitions
}

func (cb *CommandBar) setState(newState State) error {
    if !cb.isValidTransition(newState) {
        return fmt.Errorf("invalid transition %s -> %s",
                         cb.state, newState)
    }
    cb.state = newState
    return nil
}
```

**Impact**: Low-Medium - Better debugging, fewer state bugs
**Effort**: Low (1 day) - Add validation logic

---

#### 8. Performance Not Measured (NOT AUDITED)

**Need performance benchmarks for**:
- Table rebuilding on filter changes
- Transform function execution
- Palette fuzzy search
- Large cluster handling (1000+ pods)

**Problem**: No performance data to identify bottlenecks.

**Recommended Fix** - Add benchmarks:
```go
func BenchmarkTableRebuild(b *testing.B) { ... }
func BenchmarkFuzzySearch(b *testing.B) { ... }
func BenchmarkTransform(b *testing.B) { ... }
```

**Impact**: Low-Medium - Prevent performance regressions
**Effort**: Low-Medium (2-3 days) - Create benchmarks, measure

---

#### 9. Duplicate Filter State (NOT FIXED)

**Filter state exists in both Screen and CommandBar**

```go
// ConfigScreen has filter
type ConfigScreen struct {
    filter string
}

// CommandBar also has filter
type CommandBar struct {
    filterQuery string
}
```

**Problem**: Unclear which is source of truth. Potential sync issues.

**Recommended Fix** - Single source of truth:
- Option A: Filter lives only in Screen, CommandBar sends
  FilterUpdateMsg
- Option B: Filter lives only in CommandBar, Screen receives filtered
  data

**Impact**: Low - Code clarity, fewer bugs
**Effort**: Low (1 day) - Refactor filter handling

## Decision

Adopt a **phased refactoring strategy** with two priority tiers:

### High Priority (PLAN-09: 1-2 weeks)

**Critical architectural issues blocking quality:**

1. **Tight Coupling** - Extract KubeconfigProvider interface
   - Impact: High (affects 20+ commands)
   - Effort: 2-3 days
   - Benefit: Better testability, cleaner dependencies

2. **Resource Management** - Add Close() guard, audit goroutines
   - Impact: High (resource/goroutine leaks)
   - Effort: 1-2 days
   - Benefit: Production stability

3. **CommandBar Test Coverage** - Improve 32.1% → 70%+
   - Impact: High (confidence in refactoring)
   - Effort: 2-3 days
   - Benefit: Catch regressions, safer changes

**Total: 5-8 days**

### Medium Priority (PLAN-10: 2-3 weeks)

**Structural improvements for maintainability:**

4. **InformerRepository Split** - Separate concerns into 3 interfaces
   - Impact: High (core data layer)
   - Effort: 5-7 days
   - Benefit: Cleaner architecture, easier testing

5. **Theme Context Pattern** - Use context instead of passing theme
   - Impact: Medium (architecture improvement)
   - Effort: 2-3 days
   - Benefit: Easier theme switching, less coupling

6. **User Configuration** - Add ~/.config/k1/config.yaml support
   - Impact: Medium (user experience)
   - Effort: 2-3 days
   - Benefit: User customization

7. **Test Architecture** - Separate unit/integration, add helpers
   - Impact: Medium (developer experience)
   - Effort: 2-3 days
   - Benefit: Faster tests, easier to write tests

**Total: 11-16 days**

### Low Priority (Future Work)

7. **State Validation** - Add state machine transition guards
8. **Performance Benchmarks** - Measure and optimize
9. **Filter State Consolidation** - Single source of truth

### Not Architectural Issues

These items were initially flagged but are actually correct patterns:

**ConfigScreen "mixing concerns"**: ConfigScreen follows the **Elm
Architecture** pattern (which BubbleTea implements), where a component
combines Model + Update + View. This is the **intended design** of the
framework, not an SRP violation. The proposed "fix" (splitting into
MVC/MVVM) would fight against the framework's architecture. All 11
resource screens correctly implement this pattern.

## Consequences

### Positive

1. **Cleaner Dependencies**: KubeconfigProvider interface reduces
   coupling by 85% (15 methods → 2)

2. **Production Ready**: Close() guard and goroutine audit prevent
   resource leaks

3. **Higher Confidence**: 70%+ CommandBar coverage catches regressions

4. **Better Architecture**: Repository split and theme context improve
   long-term maintainability

5. **User Empowerment**: Config file enables customization without
   recompilation

### Negative

1. **Breaking Changes**: KubeconfigProvider changes all command
   signatures

2. **Migration Effort**: Repository split requires updating all
   screens

3. **Ongoing Work**: Still 2-3 weeks of refactoring ahead

4. **Test Debt**: Need to write ~500+ more lines of tests

### Risks and Mitigations

**Risk**: Repository split breaks existing functionality
**Mitigation**: Comprehensive integration tests before/after, gradual
migration

**Risk**: Breaking changes affect feature velocity
**Mitigation**: Do refactoring in dedicated sprints, feature freeze

**Risk**: Test coverage goal too ambitious
**Mitigation**: Focus on critical paths first, defer edge cases

## Implementation Strategy

### Phase 1: Quick Wins (PLAN-09A: 3-5 days)

1. Extract KubeconfigProvider interface (1-2 days)
2. Add Close() guard and audit goroutines (1-2 days)
3. Improve CommandBar test coverage (1 day)

**Goal**: Address critical issues quickly

### Phase 2: Structural Changes (PLAN-09B: 6-10 days)

4. Split InformerRepository (3-5 days)
5. Implement theme context pattern (2-3 days)
6. Add user config file support (1-2 days)

**Goal**: Improve architecture for long-term maintenance

### Phase 3: Testing Improvements (PLAN-09C: 2-3 days)

7. Separate integration/unit tests
8. Create shared test helpers
9. Add performance benchmarks

**Goal**: Better developer experience

## Metrics

Track progress with these metrics:

**Architecture Quality**:
- Tight coupling: Repository interface usage (currently 100% use full
  interface, target: 0%)
- Test coverage: CommandBar (currently 32.1%, target: 70%+)
- Resource safety: Close() calls (currently unguarded, target: guarded)

**Code Quality**:
- Average component size (currently: Repository 748 lines, target: <500
  lines)
- Duplicate code (test helpers duplicated, target: shared package)

**User Experience**:
- Config flexibility (currently: no user config, target: YAML config
  file)
- Theme customization (currently: CLI flag only, target: config file +
  runtime switch)

## Next Steps

1. Create PLAN-09: Address high-priority architectural issues
2. Create PLAN-10: Implement medium-priority improvements
3. Document patterns in CLAUDE.md as they emerge
4. Update DDR-14B with actual implementation results
