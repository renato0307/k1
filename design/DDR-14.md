# Code Quality Analysis and Refactoring Strategy

| Metadata | Value                      |
|----------|----------------------------|
| Date     | 2025-10-06                 |
| Author   | @renato0307                |
| Status   | `Accepted`                 |
| Tags     | refactoring, code-quality, architecture, solid, testing |
| Updates  | -                          |

| Revision | Date       | Author      | Info                              |
|----------|------------|-------------|-----------------------------------|
| 1        | 2025-10-06 | @renato0307 | Initial code quality audit        |

## Context and Problem Statement

The k1 codebase has evolved from rapid prototyping to a feature-rich
Kubernetes TUI with ~8500 lines of production code. As the project
transitions from prototype to production-ready software, we need to
identify code smells, architectural issues, and technical debt that
could hinder future development, testing, and maintenance. What code
quality issues exist in the codebase, and what is the prioritized
refactoring strategy to address them?

## References

Analysis methodology:
- Martin Fowler - Refactoring: Improving the Design of Existing Code
- Robert C. Martin - Clean Architecture
- SOLID Principles (Single Responsibility, Dependency Inversion, etc.)
- Go best practices and idiomatic patterns

Codebase analyzed:
- All Go source files (~8500 lines of production code)
- 35 files across cmd/, internal/app, internal/components,
  internal/screens, internal/k8s, internal/commands, internal/types,
  internal/ui
- Test coverage: 76.7% (k8s), 71.0% (screens), 30% overall (missing
  CommandBar, commands)

## Design

### Code Smells Identified

#### 1. God Object - CommandBar Component

**Location**: `internal/components/commandbar.go` (1257 lines)

The CommandBar component violates Single Responsibility Principle with
too many responsibilities:
- Input state management (7 states: Hidden, Filter, Palette, Input,
  Confirmation, LLM Preview, Result)
- Command registry and filtering
- Palette rendering and navigation
- Argument parsing and hints
- History management
- Confirmation handling
- LLM preview handling

**Impact**: Hard to test, hard to understand, hard to modify. Changes
to one concern ripple through entire component.

#### 2. Magic Numbers Throughout Codebase

**Examples**:
- `commandbar.go:367`: `if itemCount > 8` (palette item limit)
- `fullscreen.go:64,80,84`: `-3` for height calculations
- `screens.go:211-215`: `+ 15` for padding calculation
- `informer_repository.go:97`: `30*time.Second` resync period
- `executor.go:52-54`: `30 * time.Second` default timeout

**Impact**: Unclear intent, hard to maintain, difficult to configure.

#### 3. Massive Code Duplication

**Duplicate Sorting Logic** (3 instances):
- `informer_repository.go:238-243` (pods)
- `informer_repository.go:289-294` (deployments)
- `informer_repository.go:359-365` (services)

**Duplicate Transform Pattern** (11 instances):
All transform functions in `transforms.go` follow identical structure
with no abstraction:
```go
func transformX(u *unstructured.Unstructured) (interface{}, error) {
    name := u.GetName()
    namespace := u.GetNamespace()
    creationTime := u.GetCreationTimestamp()
    age := time.Since(creationTime.Time)
    // Extract specific fields...
    return TypedStruct{...}, nil
}
```

**Duplicate Navigation Commands** (11 instances):
`navigation.go` has 11 nearly identical functions that only differ in
screen ID string.

**Duplicate Screen Configs** (11 instances):
`screens.go` has 11 screen config functions with 90% identical
structure.

**Impact**: Bug fixes must be applied in multiple places,
inconsistency risk.

#### 4. Primitive Obsession

**String-based Resource Types**:
Resource types are strings (`"pods"`, `"deployments"`) used everywhere
without type safety. No validation, easy to typo.

**Int-based View Types**:
`types.go:126`: `ViewType int` with magic numbers 0, 1, 2 instead of
named constants:
```go
type ShowFullScreenMsg struct {
    ViewType     int    // 0=YAML, 1=Describe, 2=Logs
    ResourceName string
    Content      string
}
```

**Map-based Resource Data**:
`map[string]interface{}` used extensively for resource selection data
instead of proper types.

**Impact**: No compile-time safety, easy to introduce bugs with typos.

#### 5. Long Methods

**Methods over 100 lines**:
- `commandbar.go:handlePaletteState` (182 lines)
- `informer_repository.go:NewInformerRepository` (184 lines)
- `proto-pods-tui/main.go:Update` (160 lines)

**Impact**: Hard to understand, test, and modify.

#### 6. Feature Envy

Commands accessing resource details directly access `ctx.Selected` map:
```go
// pod.go, deployment.go, service.go (repeated pattern)
if name, ok := ctx.Selected["name"].(string); ok {
    resourceName = name
}
if ns, ok := ctx.Selected["namespace"].(string); ok {
    namespace = ns
}
```

**Impact**: Commands know too much about resource structure. Changes
to resource representation require updating all commands.

#### 7. Dead/Unused Code

**Empty Stub Implementations**:
- `llm.go`: All 5 LLM command functions are empty with TODOs

**Unused Prototype Code**:
- `cmd/proto-k8s-informers/` (123 lines)
- `cmd/proto-bubbletea/` (100 lines)
- `cmd/proto-pods-tui/` (703 lines)

**Unused Struct Fields**:
- `InputField.Options` defined but never used
- `ScreenConfig.CustomOperations` defined but never used

**Impact**: Increases codebase size, maintenance burden, confusion.

#### 8. Inconsistent Error Handling

Mixed error patterns across codebase:
```go
// informer_repository.go:157 - prints to stderr
fmt.Fprintf(os.Stderr, "Warning: ...")

// executor.go:84 - returns error
return "", fmt.Errorf("kubectl failed: %w", err)

// pod.go:97 - returns tea.Cmd with error msg
return func() tea.Msg {
    return types.ErrorStatusMsg(fmt.Sprintf("Invalid args: %v", err))
}
```

**Impact**: Inconsistent error experience, hard to handle errors
properly at each layer.

#### 9. Test Smells

**Missing Tests**:
- CommandBar (1257 lines, 0 tests)
- All command execution logic (0 tests)
- Clipboard operations (0 tests)
- 30% of codebase has no test coverage

**Long Test Functions**:
- `informer_repository_test.go`: Several tests over 100 lines
- Excessive test table fields (12+ fields per test case)

**Impact**: Low confidence in refactoring, bugs slip through.

#### 10. Comments as Deodorant

Multiple TODO comments indicate incomplete implementation:
```go
// llm.go:11
// TODO: Phase 3 - LLM translation and execution

// pod.go:164
return types.InfoMsg("Previous logs for pod/" + resourceName +
    " - Coming soon")
```

**Impact**: User-facing features return "coming soon" messages.

### Architectural Issues

#### 1. Tight Coupling

**CommandBar → Repository**:
CommandBar directly depends on concrete Repository implementations.

**Commands → Repository**:
All command implementations take `k8s.Repository` but only use 2
methods (`GetKubeconfig`, `GetContext`). Commands should not depend on
entire repository interface.

**Screens → Repository**:
Every screen depends on Repository for data fetching, no abstraction
layer.

**Impact**: Hard to test, hard to swap implementations.

#### 2. Missing Abstractions

**No Command Execution Abstraction**:
kubectl execution logic scattered across multiple command files
(`pod.go`, `deployment.go`, `node.go`). Each reimplements command
building.

**No Transform Abstraction**:
11 transform functions with identical structure but no shared interface
or base implementation.

**No Screen Abstraction**:
Screen configs are separate functions instead of data-driven
configuration.

**Impact**: Duplication, inconsistency, harder to extend.

#### 3. Responsibility Violations (SRP)

**CommandBar Violations**:
Handles input, command filtering, history, confirmations, LLM preview,
and palette rendering.

**InformerRepository Violations**:
Manages informers, transforms data, generates YAML, fetches events, and
sorts resources.

**ConfigScreen Violations**:
Fetches data, filters, sorts, renders, and tracks selection.

**Impact**: Hard to test, hard to understand, fragile.

#### 4. Poor Separation of Concerns

**Business Logic Mixed with UI**:
Commands in `internal/commands/` contain kubectl execution logic
(infrastructure concern).

**Data Fetching Mixed with Rendering**:
Screens fetch data, transform it, filter it, AND render it.

**Configuration Mixed with Logic**:
Timeout values, retry logic scattered throughout instead of
centralized.

**Impact**: Can't swap implementations, hard to test.

#### 5. Dependency Direction Issues

**Commands Depend on Repository**:
Should depend on minimal interface, not full repository.

**UI Depends on Domain**:
Types in `internal/types/` mix UI messages with domain concepts.

**No Dependency Inversion**:
Concrete implementations passed around instead of interfaces.

**Impact**: Violations of Dependency Inversion Principle.

#### 6. State Management Problems

**CommandBar State Complexity**:
9 state fields, complex state machine with 7 states, difficult to
reason about transitions.

**Duplicate State**:
Filter state exists in both Screen and CommandBar, unclear which is
source of truth.

**No State Validation**:
State transitions not validated, possible invalid states.

**Impact**: Bugs, race conditions, hard to debug.

#### 7. Configuration Management

**No Central Configuration**:
Timeouts, sizes, limits hard-coded throughout.

**No User Configuration**:
Users can't configure timeouts, themes (CLI arg only), or behavior.

**Theme Passed Everywhere**:
Theme pointer passed to every component instead of context.

**Impact**: Hard to change defaults, no user customization.

#### 8. Resource Management Issues

**Unchecked Context Cancellation**:
Some goroutines don't check context cancellation properly.

**Multiple Close() Calls**:
`informer_repository.go:578`: Close() can be called multiple times but
no guard.

**No Cleanup in Some Tests**:
Not all tests properly clean up resources.

**Impact**: Resource leaks, goroutine leaks.

#### 9. Testing Architecture

**No Test Helpers**:
Test setup code duplicated across test files.

**No Mock Framework**:
Manual mocks for Repository, could use mockgen or similar.

**Integration Tests Mixed with Unit Tests**:
envtest tests are slow integration tests mixed with fast unit tests.

**Impact**: Slow test suite, hard to write new tests.

#### 10. Performance Concerns

**Rebuilding Table on Every Update**:
`screens.go:311-329`: Entire table rebuilt on filter change.

**No Caching**:
Transform functions re-executed on every refresh.

**Palette Filtering on Every Keystroke**:
`commandbar.go:1193-1231`: Fuzzy search runs on every character.

**Impact**: Potential performance degradation with large clusters.

### Security Concerns

#### 1. Command Injection Risk (FALSE POSITIVE)

**Status**: Not a real vulnerability - removed from high-priority list.

String concatenation for kubectl commands in clipboard mode:
```go
// clipboard.go
var kubectlCmd strings.Builder
kubectlCmd.WriteString("kubectl exec -it ")
kubectlCmd.WriteString(podName) // From Kubernetes API
```

**Analysis**: This is NOT a vulnerability because:
- Resource names come from Kubernetes API server which enforces strict
  validation: `[a-z0-9]([-a-z0-9]*[a-z0-9])?`
- API server prevents shell metacharacters in resource names
- Commands are copied to clipboard for user review, not executed directly
- This is a local TUI operated by cluster owner, not untrusted input
- Kubernetes itself would reject malicious resource names before they exist

**Impact**: None. Initial assessment overestimated risk.

#### 2. RBAC Error Handling

`informer_repository.go:157`: RBAC errors printed to stderr but
application continues with partial data.

**Impact**: Users may not realize they have incomplete data.

### Positive Patterns

1. **Good Test Coverage for Core Logic**: Kubernetes repository and
   transforms have 70%+ coverage with comprehensive test cases.

2. **Config-Driven Screen Architecture**: PLAN-04 implementation
   successfully uses config-driven approach for screens, reducing
   duplication.

3. **Informer Pattern Usage**: Proper use of Kubernetes informers with
   caching for performance.

4. **Theme System**: Well-designed theme system with 8 themes, proper
   color abstractions.

5. **Message-Based Architecture**: Bubble Tea message pattern properly
   used for UI updates.

## Decision

Adopt a **phased refactoring strategy** with three priority tiers:

### High Priority (Sprint 1-2: 2 weeks)

1. **Extract constants** for all magic numbers:
   ```go
   const (
       MaxPaletteItems = 8
       DefaultTimeout = 30 * time.Second
       ResyncPeriod = 30 * time.Second
       FullScreenReservedLines = 3
   )
   ```

2. **Standardize error handling**:
   - Define error handling patterns per layer
   - Repository: return errors
   - Commands: return tea.Cmd with StatusMsg
   - UI: display StatusMsg in status bar

3. **Split CommandBar** into 4 focused components:
   - `InputManager`: Handle input state machine
   - `PaletteRenderer`: Display and filter commands
   - `CommandExecutor`: Execute commands with confirmation
   - `HistoryManager`: Track command history

4. **Remove dead code**:
   - Delete LLM stubs or implement minimal versions
   - Move prototypes to separate directory (examples/ or archive/)
   - Remove unused struct fields

### Medium Priority (Sprint 3-4: 2 weeks)

6. **Reduce duplication**:
   - Extract `sortByAge` helper
   - Create `TransformFunc` abstraction with shared extraction logic
   - Use table-driven approach for navigation commands

7. **Add missing tests**:
   - CommandBar component tests (aim for 70%+ coverage)
   - Command execution logic tests
   - Clipboard operations tests

8. **Create resource abstractions**:
   ```go
   type ResourceInfo struct {
       Name      string
       Namespace string
       Kind      string
   }
   func (ctx *CommandContext) GetResourceInfo() ResourceInfo
   ```

9. **Centralize configuration**:
   - Create config package with defaults
   - Support user overrides via config file
   - Use context for theme propagation

10. **Document complex functions**:
    - Add package documentation
    - Document state machines
    - Add examples for key patterns

### Low Priority (Sprint 5+: Ongoing)

11. **Extract screen config to data**: Consolidate screen configs into
    data structures

12. **Improve test architecture**: Separate unit vs integration tests,
    create test helpers

13. **Add caching**: Cache transform results, debounce expensive
    operations

14. **Consider state machine library**: Evaluate looplab/fsm or similar

15. **Improve CLI error messages**: Better user-facing error messages

### Introduce Type Safety

Replace primitive types with domain types:
```go
type ResourceType string
const (
    ResourceTypePod ResourceType = "pods"
    ResourceTypeDeployment ResourceType = "deployments"
    // ...
)

type ViewType int
const (
    ViewTypeYAML ViewType = iota
    ViewTypeDescribe
    ViewTypeLogs
)
```

### Apply Dependency Inversion

Define minimal interfaces per component:
```go
type KubeconfigProvider interface {
    GetKubeconfig() string
    GetContext() string
}
```

## Consequences

### Positive

1. **Improved Maintainability**: Smaller, focused components easier to
   understand and modify

2. **Better Testability**: Decoupled components can be tested in
   isolation with minimal mocking

3. **Enhanced Security**: Command injection vulnerabilities eliminated

4. **Reduced Duplication**: Shared abstractions reduce code by ~30%,
   bug fixes apply once

5. **Type Safety**: Compile-time guarantees prevent entire classes of
   bugs

6. **Consistent Error Handling**: Predictable error behavior across
   all layers

7. **Better Performance**: Caching and optimization opportunities
   unlocked by decoupling

8. **Easier Onboarding**: Cleaner architecture reduces learning curve
   for new contributors

### Negative

1. **Refactoring Effort**: 3-4 weeks of development time for
   high+medium priority items

2. **Risk of Regression**: Changes to core components could introduce
   bugs

3. **Breaking Changes**: Some internal APIs will change during
   refactoring

4. **Learning Curve**: Team needs to learn new abstractions and
   patterns

5. **Temporary Duplication**: During transition, some code may exist
   in both old and new forms

### Mitigations

1. **Incremental Refactoring**: Refactor one component at a time, test
   thoroughly before moving on

2. **Test-Driven Refactoring**: Write tests before refactoring to
   catch regressions

3. **Feature Freeze During Refactoring**: Avoid adding new features
   during high-priority refactoring sprints

4. **Code Review**: All refactoring changes require thorough review

5. **Documentation**: Update CLAUDE.md and architecture docs as
   patterns evolve

6. **Backward Compatibility**: Where possible, maintain old APIs
   temporarily with deprecation warnings

## Implementation Status

### ✅ Completed
- Code quality analysis (this document)
- Priority ranking of issues

### 🚧 Next Steps
- Sprint 1-2: High priority refactoring (CommandBar split, constants,
  dead code removal, security fixes, error handling)
- Sprint 3-4: Medium priority refactoring (duplication reduction,
  tests, abstractions, configuration)
- Sprint 5+: Low priority improvements (ongoing)
