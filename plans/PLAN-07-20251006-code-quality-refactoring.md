# PLAN-07: Code Quality Refactoring (Sprint 1-2)

| Metadata | Value                                     |
|----------|-------------------------------------------|
| Date     | 2025-10-06                                |
| Author   | @renato0307                               |
| Status   | Not Started                               |
| DDR      | DDR-14                                    |
| Tags     | refactoring, quality, security, testing   |

## Goal

Address high-priority code quality issues identified in DDR-14 code audit.
Focus on security vulnerabilities, maintainability, and reducing technical
debt in preparation for future feature development.

## Success Criteria

- Zero command injection vulnerabilities
- CommandBar component split into <400 lines per file
- All magic numbers replaced with named constants
- Consistent error handling patterns across all layers
- Dead code removed (prototypes archived, LLM stubs handled)
- Test coverage for refactored components maintains 70%+
- No regressions in existing functionality

## Timeline

**Sprint 1-2**: 2 weeks (prioritized for immediate impact)

## Major Phases

### Phase 1: Security Hardening (SKIPPED)
**Goal:** Eliminate command injection vulnerabilities

**Status: SKIPPED - Not a real vulnerability in this context**

**Rationale:**
- Resource names come from Kubernetes API which enforces validation
  (`[a-z0-9]([-a-z0-9]*[a-z0-9])?`)
- Kubernetes API server prevents special shell characters in resource names
- Clipboard commands are reviewed by user before execution (not auto-run)
- This is a local TUI operated by cluster owner, not a web app with
  untrusted input
- Defense-in-depth shell quoting would add complexity with no real security
  benefit

**DDR-14 overestimated this risk.** Moving directly to Phase 2.

### Phase 2: Extract Magic Numbers (NOW FIRST PRIORITY)
**Goal:** Replace all magic numbers with named constants

**Why first:** Foundation for other refactoring. Makes CommandBar split
easier by having clear semantic names.

**Key changes:**
- Create `internal/constants/ui.go` for UI-related constants:
  - `MaxPaletteItems = 8`
  - `FullScreenReservedLines = 3`
  - `StatusBarDisplayDuration = 5 * time.Second`
- Create `internal/constants/k8s.go` for Kubernetes constants:
  - `DefaultTimeout = 30 * time.Second`
  - `ResyncPeriod = 30 * time.Second`
  - `RefreshInterval = 1 * time.Second`
- Replace all hardcoded numbers with constants
- Document why each constant has its value (comments)

**Files affected:**
- `internal/components/commandbar.go` (palette limits, heights)
- `internal/components/fullscreen.go` (height calculations)
- `internal/screens/screens.go` (padding calculations)
- `internal/k8s/informer_repository.go` (timeouts, resync)
- `internal/commands/executor.go` (command timeout)

**Success metric:** Zero magic numbers in production code, all values use
semantic constants

### Phase 3: Standardize Error Handling
**Goal:** Consistent error patterns across all layers

**Why third:** Needed before CommandBar split. Defines how split components
communicate errors.

**Key decisions:**
- Repository layer: Return `error` types
- Command layer: Return `tea.Cmd` with `types.ErrorStatusMsg`
- UI layer: Display via StatusBar component (already exists)
- Infrastructure layer: Log errors to stderr, continue with partial data
  (RBAC)

**Key changes:**
- Document error handling patterns in `internal/errors/doc.go`
- Create error helper functions (e.g., `commands.errorCmd(err)`)
- Audit all error handling in codebase
- Replace inconsistent patterns (fmt.Fprintf to stderr, etc.)
- Add structured error types for common failures (NotFound,
  PermissionDenied, Timeout)

**Files affected:**
- All files in `internal/commands/`
- `internal/k8s/informer_repository.go` (RBAC errors)
- `internal/components/commandbar.go` (error display)

**Success metric:** All errors follow documented patterns, error experience
is predictable

### Phase 4: Split CommandBar Component
**Goal:** Decompose God Object into focused components

**Why fourth:** Depends on constants (Phase 2) and error patterns (Phase 3).
Largest impact on maintainability.

**Target architecture:**
```
internal/components/commandbar/
  commandbar.go       - Coordinator (state machine, <200 lines)
  input.go            - Input handling (<150 lines)
  palette.go          - Palette rendering and filtering (<200 lines)
  executor.go         - Command execution with confirmation (<150 lines)
  history.go          - Command history tracking (<100 lines)
```

**Responsibilities:**
- `commandbar.go`: State machine transitions, component coordination
- `input.go`: Keystroke handling, input buffer management
- `palette.go`: Command filtering (fuzzy), rendering, navigation
- `executor.go`: Confirmation flow, command execution
- `history.go`: Command history storage and retrieval

**Key principles:**
- Each component has single responsibility
- Components communicate via messages (Bubble Tea pattern)
- Shared state in coordinator, components are stateless where possible
- Test each component in isolation (mock dependencies)

**Migration strategy:**
- Extract one component at a time (start with history, simplest)
- Maintain old code alongside new until fully migrated
- Add tests for new components before removing old code
- No UI changes - refactor only, behavior identical

**Files affected:**
- `internal/components/commandbar.go` (split into package)
- Tests: Create `internal/components/commandbar/*_test.go`

**Success metric:** CommandBar package <800 lines total, all components
<200 lines, 70%+ test coverage

### Phase 5: Remove Dead Code
**Goal:** Clean up unused code and stubs

**Why last:** Doesn't impact functionality. Safe to defer but improves
maintainability.

**Key changes:**
- Archive prototypes to `examples/` directory:
  - `cmd/proto-k8s-informers/` → `examples/proto-k8s-informers/`
  - `cmd/proto-bubbletea/` → `examples/proto-bubbletea/`
  - `cmd/proto-pods-tui/` → `examples/proto-pods-tui/`
- Handle LLM stubs in `internal/commands/llm.go`:
  - Option A: Implement minimal versions (return "not implemented" message)
  - Option B: Remove from command registry until DDR-12 implemented
  - **Recommendation:** Option A (avoids breaking command bar)
- Remove unused struct fields:
  - `InputField.Options` (never used)
  - `ScreenConfig.CustomOperations` (never used)
- Update documentation to reference examples instead of prototypes

**Files affected:**
- `cmd/proto-*` (move to examples/)
- `internal/commands/llm.go` (implement stubs or remove)
- `internal/types/types.go` (remove unused fields)
- `CLAUDE.md` (update prototype references)

**Success metric:** Zero unused code in production paths, prototypes
available as examples

## Risks and Mitigations

**Risk:** Regressions from refactoring
**Mitigation:** Write tests before refactoring, test-driven refactoring
approach

**Risk:** CommandBar split introduces bugs
**Mitigation:** Extract one component at a time, maintain old code until
migration complete

**Risk:** Security fixes break existing commands
**Mitigation:** Add security tests first, validate all commands still work

**Risk:** Constants change behavior unintentionally
**Mitigation:** Ensure constant values match existing magic numbers exactly

**Risk:** Effort exceeds 2-week estimate
**Mitigation:** Phase 5 (dead code) is optional, can defer if needed

## Testing Strategy

**Security testing:**
- Create test suite with malicious inputs (resource names with special
  chars)
- Validate sanitization works correctly
- Ensure no command injection possible

**Refactoring testing:**
- Write tests for existing behavior before refactoring
- Maintain test coverage >70% for refactored components
- Run full test suite after each phase
- Manual smoke testing on live cluster

**Regression prevention:**
- Test all commands end-to-end after changes
- Verify UI behavior identical (no visual changes)
- Check performance (no degradation)

## Out of Scope (Medium/Low Priority)

Deferred to future sprints per DDR-14:
- Duplication reduction (transform functions, navigation commands)
- Missing tests (increase coverage to 80%+)
- Resource abstractions (typed contexts)
- Configuration management (central config, user overrides)
- Performance optimizations (caching, debouncing)
- State machine library evaluation
- Test architecture improvements

## TODO

### Phase 1: Security Hardening ⏭️ SKIPPED
- [x] Audit revealed no real vulnerability (Kubernetes validates resource names)
- [x] Threat model analysis: clipboard pattern + local TUI = low risk
- [x] Decision: Skip this phase, DDR-14 overestimated the risk

### Phase 2: Extract Magic Numbers ✅ COMPLETE
- [x] Create constants in internal/components/constants.go (UI constants)
- [x] Create constants in internal/k8s/constants.go (Kubernetes constants)
- [x] Create constants in internal/commands/constants.go (Command constants)
- [x] Create constants in internal/screens/constants.go (Screen constants)
- [x] Replace magic numbers in commandbar.go (MaxPaletteItems: 4 occurrences)
- [x] Replace magic numbers in fullscreen.go (FullScreenReservedLines: 4 occurrences)
- [x] Replace magic numbers in screens.go (RefreshInterval: 12 occurrences)
- [x] Replace magic numbers in informer_repository.go (InformerResyncPeriod: 2 occurrences)
- [x] Replace magic numbers in executor.go (DefaultKubectlTimeout: 1 occurrence)
- [x] Replace magic number in app.go (StatusBarDisplayDuration: 1 occurrence)
- [x] Add comments documenting constant rationale
- [x] Verify compilation successful

### Phase 3: Standardize Error Handling
- [x] Document message patterns in internal/messages/doc.go
- [x] Refactored errors package → messages package (better semantics)
- [ ] Create error helper functions
- [ ] Define structured error types
- [ ] Audit and fix command layer errors
- [ ] Audit and fix repository layer errors
- [ ] Audit and fix UI layer error display
- [ ] Test error scenarios end-to-end

### Phase 4: Split CommandBar Component
- [ ] Design component interfaces and message flow
- [ ] Create internal/components/commandbar/ package structure
- [ ] Extract history.go (simplest, test extraction process)
- [ ] Extract palette.go (rendering and filtering)
- [ ] Extract input.go (keystroke handling)
- [ ] Extract executor.go (confirmation and execution)
- [ ] Refactor commandbar.go to coordinator
- [ ] Write tests for all components (70%+ coverage)
- [ ] Remove old code after migration complete
- [ ] Verify no regressions with manual testing

### Phase 5: Remove Dead Code
- [ ] Create examples/ directory
- [ ] Move cmd/proto-* to examples/
- [ ] Update CLAUDE.md references
- [ ] Handle llm.go stubs (implement minimal or remove)
- [ ] Remove unused struct fields
- [ ] Update documentation

## References

- DDR-14: Code Quality Analysis and Refactoring Strategy
- PROCESS-IMPROVEMENTS.md: Quality gates and process guidelines
- DDR-08: Pragmatic command implementation (executor patterns)
- DDR-05: Command bar architecture
