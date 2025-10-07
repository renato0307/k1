# PLAN-08: Medium Priority Code Quality Improvements

**Status**: Phase 1 Complete, Phase 2 Complete, Phase 4 Complete (Phase 3 deferred)
**Created**: 2025-10-06
**DDR Reference**: DDR-14 (Medium Priority items 6-8, partial 9)
**Branch**: `refactor/medium-priority-improvements`

## Overview

Reduce code duplication, improve test coverage, and create resource
abstractions from DDR-14 medium priority items. This plan focuses on
maintainability improvements without major architectural changes.

**Scope**: Items 6-8 + partial item 9 (config defaults only)
**Out of Scope**: Items 9 (user config file, theme context), 10 (docs)

## Goals

1. **Reduce Duplication**: Extract 30%+ duplicate code using shared helpers
2. **Improve Testability**: Reach 70%+ coverage for commands/clipboard
3. **Type Safety**: Replace primitives with domain types for resources
4. **Config Foundation**: Establish config package with sensible defaults

## Phases

### Phase 1: Extract Duplicate Code (~2-3 days)

**Duplication targets**:
- Sorting logic: `sortByAge` helper (3 instances → 1)
- Transform pattern: `TransformFunc` abstraction (11 instances)
- Navigation commands: table-driven registry (11 functions → 1)

**Key decisions**:
- Transform abstraction should use generics for type safety
- Navigation registry uses map[string]string (command → screenID)
- Keep existing public APIs stable during extraction

**Success criteria**: Reduced LoC by ~500, zero functional regressions

### Phase 2: Resource Type Safety (~1-2 days) ✅ COMPLETE

**Create domain types**:
```go
type ResourceInfo struct {
    Name      string
    Namespace string
    Kind      ResourceType
}

type ResourceType string
const (
    ResourceTypePod ResourceType = "pods"
    // ...11 resource types
)
```

**Key decisions**:
- Minimal interface: commands only need ResourceInfo, not full repository
- Add `GetResourceInfo()` helper to CommandContext
- Migrate commands incrementally (one type at a time)

**Success criteria**: Zero string-based resource types in commands layer

### Phase 3: Add Missing Tests (~2-3 days)

**Test coverage targets**:
- Command execution logic: 70%+ (currently 0%)
- Clipboard operations: 70%+ (currently 0%)
- Overall project: 50%+ (currently 30%)

**Test strategy**:
- Use table-driven tests for command execution
- Mock Repository with minimal interface (KubeconfigProvider)
- Test clipboard operations with in-memory buffer

**Success criteria**: `make test-coverage` shows 50%+ overall, commands 70%+

### Phase 4: Config Package Foundations (~1 day) ✅ COMPLETE

**Note**: Using per-package constants pattern (CLAUDE.md) instead of central config.

**Completed work**:
```go
package config

const (
    DefaultTimeout = 30 * time.Second
    DefaultGracePeriod = 30 * time.Second
    MaxPaletteItems = 8
    FullScreenReservedLines = 3
)
```

**Key decisions**:
- Central constants package: `internal/config/defaults.go`
- No user config file yet (deferred to future work)
- No context-based theme (deferred to future work)
- Migrate magic numbers incrementally

**Success criteria**: All magic numbers replaced with named constants

## Risks and Mitigations

**Risk**: Transform abstraction adds complexity without benefit
**Mitigation**: Start with simplest abstraction (shared field extraction),
only add generics if type safety proves valuable

**Risk**: Test coverage goal slows down implementation
**Mitigation**: Focus on critical paths first (command execution), defer
edge cases if needed

**Risk**: Config package creates import cycles
**Mitigation**: Keep config as leaf package with zero internal dependencies

## Success Criteria

- [ ] LoC reduced by ~500 lines (duplication elimination)
- [ ] Overall test coverage ≥50%, commands/clipboard ≥70%
- [ ] Zero string-based resource types in commands layer
- [ ] All magic numbers replaced with named constants
- [ ] Zero functional regressions (all existing tests pass)
- [ ] Build time unchanged or improved

## TODO

### Phase 1: Extract Duplicate Code
- [x] Extract `sortByAge` helper function (generic sortByCreationTime)
- [x] Create `TransformFunc` abstraction for 11 transform functions
- [x] Implement table-driven navigation command registry
- [x] Run tests and verify zero regressions
- [x] Optimize transform performance (extract commonFields once in caller)
- [x] Replace interface{} with any (Go 1.18+ modernization)
- [x] Extract DummyRepository to separate file
- [x] Document patterns in CLAUDE.md

### Phase 2: Resource Type Safety
- [x] Define `ResourceType` string type with 11 constants
- [x] Create `ResourceInfo` struct with domain types
- [x] Add `GetResourceInfo()` helper to CommandContext
- [x] Migrate commands to use ResourceInfo (8 command files)
- [x] Update CommandContext tests
- [x] Modernize min/max patterns (Go 1.21+)

### Phase 3: Add Missing Tests (DEFERRED)
- [ ] Write command execution tests (pod, deployment, node, service)
- [ ] Write clipboard operation tests (shell, logs, port-forward)
- [ ] Run `make test-coverage` and verify 50%+ overall, 70%+ commands
- [ ] Fix any coverage gaps in critical paths

### Phase 4: Config Package Foundations
- [x] Remove duplicate MaxPaletteItems constant from commandbar/constants.go
- [x] Extract InformerSyncTimeout (10s) to k8s/constants.go
- [x] Extract InformerIndividualSyncTimeout (5s) to k8s/constants.go
- [x] Update all references to use constants from parent package
- [x] Run full test suite and verify zero regressions
- [x] Verified per-package constants pattern (no central config needed)

## Notes

- Avoid over-engineering: prefer simple solutions over perfect abstractions
- Keep existing public APIs stable to minimize merge conflicts
- Update CLAUDE.md patterns section if new idioms emerge
- Run `make test-coverage` after each phase to catch regressions early
