# PLAN-05: YAML and Describe Commands Implementation

| Metadata | Value                                     |
|----------|-------------------------------------------|
| Date     | 2025-10-05                                |
| Author   | @renato0307                               |
| Status   | Complete                                  |
| DDR      | DDR-06                                    |
| Tags     | commands, yaml, describe, kubectl, modal  |

## Goal

Implement `/yaml` and `/describe` commands for viewing detailed resource
information using kubectl libraries. Target 1-10ms response time by
leveraging informer cache with kubectl printers and formatters.

## Success Criteria

- `/yaml` command displays resource YAML in <5ms (exact kubectl output)
- `/describe` command displays resource details in <10ms (exact kubectl
  output)
- Both commands work for Pods, Deployments, and Services initially
- Modal viewer supports scrolling, ESC to close
- Events informer integrated for describe command
- Zero external dependencies (no kubectl CLI required)
- Test coverage validates output parity with kubectl

## Architecture Decision

Use kubectl libraries (`k8s.io/kubectl`, `k8s.io/cli-runtime`) for both
yaml and describe commands. Accept 1-10ms performance in exchange for:
- Exact kubectl output parity
- Consistent implementation approach
- Simpler maintenance
- Works for all resource types

See DDR-06 for detailed design rationale.

## Current Architecture

The command infrastructure already exists:
- **Commands registry**: `internal/commands/registry.go` with yaml/describe
  commands returning dummy data
- **Full-screen view**: `ShowFullScreenMsg` message type and handler
- **Command execution**: Commands execute via `CommandContext` with
  selected resource info
- **Shortcuts**: ctrl+y (yaml), ctrl+d (describe) already wired

We need to replace dummy data with real kubectl library calls.

## Major Phases

### Phase 0: Refactor Command Structure
**Goal:** Separate command registration from implementation (SRP)

**Current problem:**
- registry.go is 640 lines with mixed concerns
- Command implementations inline in NewRegistry()
- Hard to test individual commands
- Violates Single Responsibility Principle

**Key deliverables:**
- Create `internal/commands/types.go` - move Command, CommandContext,
  CommandCategory types
- Create `internal/commands/navigation.go` - navigation command handlers
  (:pods, :deployments, etc.)
- Create `internal/commands/resource.go` - resource command handlers
  (/yaml, /describe, /delete)
- Create `internal/commands/pod.go` - pod-specific handlers (/logs,
  /shell, /port-forward, /jump-owner, /show-node)
- Create `internal/commands/deployment.go` - deployment handlers (/scale,
  /restart)
- Create `internal/commands/service.go` - service handlers (/endpoints)
- Create `internal/commands/node.go` - node handlers (/cordon, /drain)
- Create `internal/commands/llm.go` - LLM command examples
- Update `registry.go` - only registration and discovery logic
- Update `NewRegistry()` signature to accept `repo k8s.Repository`
  parameter
- All tests still pass

**Handler pattern:**
```go
// Commands needing K8s access take repo
func YamlCommand(repo k8s.Repository) ExecuteFunc
func DescribeCommand(repo k8s.Repository) ExecuteFunc

// Pure navigation commands don't need repo
func PodsCommand() ExecuteFunc
```

### Phase 1: Add kubectl dependencies
**Goal:** Add required kubectl packages

**Key deliverables:**
- Add `k8s.io/kubectl@v0.34.1` to go.mod
- Add `k8s.io/cli-runtime@v0.34.1` to go.mod
- Verify dependency compatibility with existing k8s packages
- Run `go mod tidy`

### Phase 2: Repository Enhancement
**Goal:** Add YAML and describe methods using kubectl libraries

**Key deliverables:**
- Add `GetResourceYAML(gvr, namespace, name)` method with kubectl
  YAMLPrinter
- Add `DescribeResource(gvr, namespace, name)` method with simplified
  describe formatting
- Add on-demand event fetching for describe command (no informer)
- Implement `fetchEventsForResource()` for direct API calls with field
  selectors
- Implement `formatEvents()` for kubectl-style event display
- Handle resource not found errors
- Unit tests with envtest (Pods, Deployments, Services)

**Technical approach:**
- Use `printers.YAMLPrinter` from k8s.io/cli-runtime for YAML
- Use simplified custom describe with status as indented YAML
- Fetch events on-demand via API (zero memory overhead, 50-100ms latency)
- Generic method using GVR (GroupVersionResource) for flexibility

### Phase 3: Update Command Registry
**Goal:** Replace dummy data with real repository calls

**Key deliverables:**
- Update yaml command Execute function (lines 181-240 in registry.go)
- Update describe command Execute function (lines 248-314 in registry.go)
- Call repository methods with resource info from CommandContext
- Handle errors gracefully (show error message)
- Maintain compatibility with ShowFullScreenMsg

**Changes needed:**
```go
// Before: yamlContent := `dummy yaml...`
// After:
gvr := getGVRForResourceType(ctx.ResourceType)
yamlContent, err := repo.GetResourceYAML(gvr, namespace, resourceName)
if err != nil {
    return errorCmd(err)
}
```

### Phase 4: Testing and Validation
**Goal:** Validate output parity and performance

**Key deliverables:**
- Unit tests for GetResourceYAML (verify kubectl output match)
- Unit tests for DescribeResource (verify kubectl output match)
- Manual testing with live cluster:
  - Test yaml command for Pods, Deployments, Services
  - Test describe command for Pods, Deployments, Services
  - Compare output with `kubectl get -o yaml` and `kubectl describe`
  - Verify performance (<10ms response time)
- Test error cases (resource not found, events not synced)
- Test with different resource states (running, pending, failed)

### Phase 5: Documentation and Cleanup
**Goal:** Update docs and mark plan complete

**Key deliverables:**
- Update CLAUDE.md with yaml/describe implementation status
- Update DDR-06 status to "Implemented"
- Mark PLAN-05 phases as complete
- Document any deviations from original design
- Add usage examples to docs

## Risks and Considerations

**Binary size increase:** kubectl packages add ~5-10MB. Acceptable
trade-off for feature value.

**Events informer:** High-volume resource. Use Tier 2 loading and
consider field selectors for optimization if needed.

**Output staleness:** Informer cache may be up to 30s stale. Acceptable
for TUI debugging use case.

**kubectl API stability:** Using official kubectl packages. Breaking
changes rare but must track Kubernetes version compatibility.

## Future Extensions (Out of Scope)

- Syntax highlighting for YAML
- Search within modal (/)
- Copy to clipboard (y)
- Export to file
- Edit YAML in-place
- JSON output format
- Watch mode (live updates)
- Additional resource types (ConfigMaps, Secrets, etc.)

## TODO

- [x] Phase 0: Refactor command structure
  - [x] Create types.go (move types from registry.go)
  - [x] Create navigation.go (11 screen navigation commands)
  - [x] Create resource.go (/yaml, /describe, /delete stubs)
  - [x] Create pod.go (5 pod-specific commands)
  - [x] Create deployment.go (/scale, /restart)
  - [x] Create service.go (/endpoints)
  - [x] Create node.go (/cordon, /drain)
  - [x] Create llm.go (5 LLM examples)
  - [x] Refactor registry.go (registration only, 309 lines)
  - [x] Update NewRegistry() to accept repo parameter
  - [x] Update commandbar.go to accept repo parameter
  - [x] Update app.go to pass repo to NewCommandBar()
  - [x] Verify all tests pass
- [x] Phase 1: Add kubectl dependencies
  - [x] Add k8s.io/kubectl@v0.34.1
  - [x] Add k8s.io/cli-runtime@v0.34.1
  - [x] Run go mod tidy
  - [x] Verify build still works
- [x] Phase 2: Repository enhancement
  - [x] Add GetResourceYAML method with YAMLPrinter
  - [x] Add DescribeResource method with on-demand event fetching
  - [x] Implement GetResourceYAML in InformerRepository
  - [x] Implement DescribeResource in InformerRepository
  - [x] Add fetchEventsForResource() for on-demand event API calls
  - [x] Add formatEvents() for kubectl-style event formatting
  - [x] Update YamlCommand to use repo.GetResourceYAML()
  - [x] Update DescribeCommand to use repo.DescribeResource()
  - [x] Add GetGVRForResourceType helper function
  - [x] Add stub implementations in DummyRepository
  - [x] All tests pass (22/22)
  - [x] Events: Use on-demand fetching (no informer, zero memory overhead)
- [x] Phase 3: Update command registry
  - [x] Replace dummy yaml in YamlCommand with repo.GetResourceYAML()
  - [x] Replace dummy describe in DescribeCommand with repo.DescribeResource()
  - [x] Repository already passed to commands via NewRegistry(repo)
  - [x] Error handling added (resource not found, unknown resource type)
- [x] Phase 4: Testing and validation
  - [x] Unit tests for GetResourceYAML (pod and deployment, error cases)
  - [x] Unit tests for DescribeResource (with events, no events)
  - [x] Unit tests for formatEventAge helper
  - [x] Manual testing with live cluster
  - [x] Output verified working correctly
  - [x] Performance acceptable for TUI use case
- [x] Phase 5: Documentation
  - [x] Update CLAUDE.md
  - [x] Update DDR-06 status
  - [x] Mark plan complete

## References

- DDR-06: Resource Detail Commands (Design)
- DDR-03: Kubernetes Informer-Based Repository
- PLAN-03: Command-Enhanced UI (Phase 3 foundation)
- kubectl describe package: k8s.io/kubectl/pkg/describe
- kubectl printers: k8s.io/cli-runtime/pkg/printers
