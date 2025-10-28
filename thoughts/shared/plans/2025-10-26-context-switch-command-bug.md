# Context Switch Command Bug Fix - Implementation Plan

## Overview

After a context switch, commands that use kubectl subprocess execution (YAML,
describe, delete, scale, restart, cordon, drain, endpoints, logs, port-forward,
shell) fail because they execute against the WRONG Kubernetes context. The bug
is caused by command closures capturing repository references at command
registry creation time, instead of using the current active repository at
execution time.

## Current State Analysis

### Root Cause

**Command Registry Singleton Pattern:**
- Command registry is created ONCE when `commandbar.New(pool, theme)` is called
  during application startup (`app.go:113`)
- Registry creation calls `pool.GetActiveRepository()` ONCE to get the initial
  active repository (`registry.go:27`)
- This repository reference is captured in ALL command closures at creation
  time (e.g., `YamlCommand(repo)`, `ScaleCommand(repo)`)
- After context switch, command bar is NOT recreated, so commands still hold
  references to the OLD repository

**Context Switch Flow:**
1. User switches context (via contexts screen or `:context` command)
2. Pool updates `p.active` to point to new context's repository
3. App correctly recreates screen registry with new repository (`app.go:447,
   488`)
4. **BUG**: App does NOT recreate command bar, so command registry still has
   old repository

**Impact on Commands:**
- Commands call `repo.GetContext()` and `repo.GetKubeconfig()` to build kubectl
  commands
- These methods return the context name stored in the captured repository
- If captured repository is context-a, `GetContext()` always returns
  "context-a"
- kubectl executes with `--context context-a` even when viewing context-b data

### Key Discoveries

**Files with affected patterns:**

1. **Commands using NewKubectlExecutor (8 commands):**
   - `deployment.go:49` - ScaleCommand
   - `deployment.go:92` - RestartCommand
   - `node.go:36` - CordonCommand
   - `node.go:83` - DrainCommand
   - `service.go:35` - EndpointsCommand
   - `resource.go:145` - DeleteCommand

2. **Commands building kubectl manually (3 commands):**
   - `pod.go:63-69` - LogsCommand
   - `pod.go:130-136` - PortForwardCommand
   - `pod.go:191-197` - ShellCommand

3. **Commands calling repository methods (2 commands):**
   - `resource.go:54` - YamlCommand (calls `repo.GetResourceYAML()`)
   - `resource.go:98` - DescribeCommand (calls `repo.DescribeResource()`)

**Total affected commands: 13**

**Note:** YamlCommand and DescribeCommand delegate to repository methods which
use the repository's stored clientset. These clientsets are bound to specific
contexts at creation time, so they also fail after context switch.

## What We're NOT Doing

- NOT changing the RepositoryPool architecture (it's correct)
- NOT modifying how repositories store context (immutable design is correct)
- NOT changing how screens are re-initialized (already working correctly)
- NOT adding context parameters to every method (violates encapsulation)

## Implementation Approach

**Strategy:** Make command registry query the pool for the active repository at
EXECUTION time instead of capturing repository at CREATION time.

**Key insight:** RepositoryPool's `GetActiveRepository()` method ALWAYS returns
the correct current repository because it reads from `pool.repos[pool.active]`
on every call. We just need commands to call it at the right time.

**Three possible solutions:**

1. **Recreate command bar on context switch** (like screens)
   - Pro: Minimal code changes, mirrors screen pattern
   - Pro: Clean separation - new context = new command bar
   - Con: Recreates entire command bar infrastructure
   - Con: Loses command history on context switch

2. **Store pool reference in commands, call GetActiveRepository() at execution**
   - Pro: Commands always use correct repository
   - Pro: Preserves command history across switches
   - Con: Changes command signatures (repo → pool)
   - Con: Requires updating all 13 affected commands

3. **Add UpdateRegistry() method to CommandBar**
   - Pro: Preserves command bar state (history, UI state)
   - Pro: Only recreates registry (lighter weight)
   - Con: Adds complexity to CommandBar lifecycle
   - Con: Still requires updating all command signatures

**Recommended solution: Option 2** - Store pool reference in commands

Rationale:
- Most correct design - commands get current repository at execution time
- Aligns with Go idiom of "what you need, when you need it"
- Command history preservation is valuable for user workflow
- One-time cost to update command signatures, then maintainable

## Phase 1: Update Command Signatures

### Overview
Change all affected commands to accept `*k8s.RepositoryPool` instead of
`k8s.Repository`, and call `pool.GetActiveRepository()` at execution time.

### Changes Required

#### 1. Update Command Factory Functions

**File**: `internal/commands/deployment.go`

**Changes**:
- Line 19: Change `ScaleCommand(repo k8s.Repository)` to
  `ScaleCommand(pool *k8s.RepositoryPool)`
- Line 49: Change `NewKubectlExecutor(repo.GetKubeconfig(), repo.GetContext())`
  to:
  ```go
  repo := pool.GetActiveRepository()
  if repo == nil {
      return messages.ErrorCmd("No active repository")()
  }
  executor := NewKubectlExecutor(repo.GetKubeconfig(), repo.GetContext())
  ```
- Line 73: Change `RestartCommand(repo k8s.Repository)` to
  `RestartCommand(pool *k8s.RepositoryPool)`
- Line 92: Apply same pattern as line 49

**File**: `internal/commands/node.go`

**Changes**:
- Line 17: Change `CordonCommand(repo k8s.Repository)` to
  `CordonCommand(pool *k8s.RepositoryPool)`
- Line 36: Apply GetActiveRepository() pattern
- Line 59: Change `DrainCommand(repo k8s.Repository)` to
  `DrainCommand(pool *k8s.RepositoryPool)`
- Line 83: Apply GetActiveRepository() pattern

**File**: `internal/commands/service.go`

**Changes**:
- Line 17: Change `EndpointsCommand(repo k8s.Repository)` to
  `EndpointsCommand(pool *k8s.RepositoryPool)`
- Line 35: Apply GetActiveRepository() pattern

**File**: `internal/commands/resource.go`

**Changes**:
- Line 26: Change `YamlCommand(repo k8s.Repository)` to
  `YamlCommand(pool *k8s.RepositoryPool)`
- Line 54: Apply GetActiveRepository() pattern before calling
  `repo.GetResourceYAML()`
- Line 70: Change `DescribeCommand(repo k8s.Repository)` to
  `DescribeCommand(pool *k8s.RepositoryPool)`
- Line 98: Apply GetActiveRepository() pattern before calling
  `repo.DescribeResource()`
- Line 114: Change `DeleteCommand(repo k8s.Repository)` to
  `DeleteCommand(pool *k8s.RepositoryPool)`
- Line 145: Apply GetActiveRepository() pattern

**File**: `internal/commands/pod.go`

**Changes**:
- Line 27: Change `LogsCommand(repo k8s.Repository)` to
  `LogsCommand(pool *k8s.RepositoryPool)`
- Line 63-69: Add GetActiveRepository() call before building kubectl command
- Line 86: Change `PortForwardCommand(repo k8s.Repository)` to
  `PortForwardCommand(pool *k8s.RepositoryPool)`
- Line 130-136: Add GetActiveRepository() call
- Line 146: Change `ShellCommand(repo k8s.Repository)` to
  `ShellCommand(pool *k8s.RepositoryPool)`
- Line 191-197: Add GetActiveRepository() call

**File**: `internal/commands/pod_helpers.go`

**Changes**:
- Line 19: Change `JumpOwnerCommand(repo k8s.Repository)` to
  `JumpOwnerCommand(pool *k8s.RepositoryPool)`
- Add GetActiveRepository() call at execution time before using repo methods

**File**: `internal/commands/llm.go`

**Changes** (if any LLM commands use repo):
- Update signatures to use pool
- Add GetActiveRepository() calls

#### 2. Update Command Registry

**File**: `internal/commands/registry.go`

**Changes**:
- Line 26-28: Remove `repo := pool.GetActiveRepository()` call
- Update all command registrations to pass `pool` instead of `repo`:
  - Line 159: `YamlCommand(pool)`
  - Line 167: `DescribeCommand(pool)`
  - Line 176: `DeleteCommand(pool)`
  - Line 186: `LogsCommand(pool)`
  - Line 193: `LogsPreviousCommand(pool)`
  - Line 202: `PortForwardCommand(pool)`
  - Line 211: `ShellCommand(pool)`
  - Line 218: `JumpOwnerCommand(pool)`
  - Line 225: `ShowNodeCommand(pool)`
  - Line 234: `ScaleCommand(pool)`
  - Line 241: `CordonCommand(pool)`
  - Line 251: `DrainCommand(pool)`
  - Line 258: `EndpointsCommand(pool)`
  - Line 265: `RestartCommand(pool)`
  - Lines 274-301: Update all LLM commands if needed

### Success Criteria

#### Automated Verification:
- [x] All affected command files compile without errors: `go build ./...`
- [x] All unit tests pass: `make test`
- [x] Type checking passes: `go vet ./...`
- [ ] No linting errors: `golangci-lint run` (if configured)

#### Manual Verification:
- [x] Start app with context-a loaded
- [x] View a pod in context-a
- [x] Execute `/yaml` command - should show context-a pod YAML
- [x] Execute `/describe` command - should show context-a pod details
- [x] Switch to context-b via contexts screen
- [x] View a pod in context-b (different pod name)
- [x] Execute `/yaml` command - should show context-b pod YAML (not context-a)
- [x] Execute `/describe` command - should show context-b pod details (not
  context-a)
- [x] Test all other affected commands (scale, delete, logs, etc.) work in
  new context
- [x] Verify no errors in terminal output

**Implementation Note**: After completing this phase and all automated
verification passes, pause here for manual confirmation from the human that
manual testing was successful before proceeding to the next phase (if any).

---

## Phase 2: Update Tests

### Overview
Update all command tests to pass pool instead of mock repository.

### Changes Required

#### 1. Update Command Tests

**File**: `internal/commands/command_execution_test.go`

**Changes**:
- Update test setup to create mock RepositoryPool
- Pass pool to command factory functions instead of mock repo
- Verify commands call GetActiveRepository() at execution time

**File**: `internal/commands/deployment_test.go` (if exists)

**Changes**:
- Update ScaleCommand tests to use pool
- Update RestartCommand tests to use pool

**File**: `internal/commands/node_test.go` (if exists)

**Changes**:
- Update CordonCommand tests to use pool
- Update DrainCommand tests to use pool

**File**: `internal/commands/resource_test.go` (if exists)

**Changes**:
- Update YamlCommand tests to use pool
- Update DescribeCommand tests to use pool
- Update DeleteCommand tests to use pool

**File**: `internal/commands/pod_test.go` (if exists)

**Changes**:
- Update LogsCommand tests to use pool
- Update PortForwardCommand tests to use pool
- Update ShellCommand tests to use pool

#### 2. Create Test Utilities

**File**: `internal/k8s/test_helpers.go` (new file)

**Changes**:
```go
// NewTestRepositoryPool creates a pool with single test repository
func NewTestRepositoryPool(repo Repository) *RepositoryPool {
    pool := &RepositoryPool{
        repos:  make(map[string]*RepositoryEntry),
        active: "test-context",
    }
    pool.repos["test-context"] = &RepositoryEntry{
        Repo:   repo,
        Status: StatusLoaded,
    }
    return pool
}
```

### Success Criteria

#### Automated Verification:
- [ ] All unit tests pass: `make test`
- [ ] Test coverage remains >= 70%: `make test-coverage`
- [ ] No test failures in CI/CD pipeline (if configured)

#### Manual Verification:
- [ ] Review test output for any unexpected warnings
- [ ] Verify test coverage report shows affected commands are tested

**Implementation Note**: After completing this phase and all automated
verification passes, pause here for manual confirmation from the human that
manual testing was successful.

---

## Testing Strategy

### Unit Tests

**Command Factory Tests:**
- Test that commands call GetActiveRepository() at execution time
- Test that commands handle nil repository gracefully
- Test that commands use correct context from active repository

**Integration Tests:**
- Test context switch followed by command execution
- Test multiple context switches with interleaved command executions
- Test command history preservation across context switches

### Manual Testing Steps

1. **Initial Context Testing:**
   - Start app with default context
   - Execute all affected commands on a test resource
   - Verify commands work correctly

2. **Context Switch Testing:**
   - Switch to different context with different resources
   - Execute same commands on new context's resources
   - Verify commands operate on NEW context (not old)

3. **Multiple Switch Testing:**
   - Switch between 3+ contexts multiple times
   - Execute commands after each switch
   - Verify commands always use current context

4. **Edge Case Testing:**
   - Switch to context that failed to load
   - Verify commands show appropriate error messages
   - Switch back to working context
   - Verify commands work again

5. **Command History Testing:**
   - Execute several commands in context-a
   - Switch to context-b
   - Use up/down arrows to navigate history
   - Verify history is preserved
   - Execute historical command
   - Verify it runs in NEW context (context-b)

## Performance Considerations

**Pool GetActiveRepository() Call Overhead:**
- O(1) operation - map lookup with mutex lock
- Negligible overhead (nanoseconds)
- Called once per command execution (not in hot loop)

**No Performance Impact:**
- Commands already call repository methods which acquire locks
- Additional pool call is minimal compared to kubectl subprocess overhead
- No change to informer cache performance

## Migration Notes

**Backward Compatibility:**
- This is a bug fix, not a feature change
- No API changes visible to users
- No configuration changes required
- Existing kubeconfigs continue to work

**Deployment:**
- Single binary replacement
- No database migrations
- No data loss risk
- Safe to rollback by reverting binary

## References

- Issue: Context switching bug (commands use wrong context)
- Root cause: Command registry singleton pattern with captured repository
- Research: `thoughts/shared/plans/2025-10-09-kubernetes-context-management.md`
- Repository pool: `internal/k8s/repository_pool.go:149-159`
- Command registry: `internal/commands/registry.go:26-27`
- App context switch handler: `internal/app/app.go:433-507`
