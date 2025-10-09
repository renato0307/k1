---
date: 2025-10-09T21:24:18+0000
researcher: Claude
git_commit: 131eb33487f06a1d90dfdf3db3eb0dda4e7ccf2f
branch: docs/designs
repository: k1-designs
topic: "Container Navigation: Pressing Enter on Pod to View Containers"
tags: [research, codebase, containers, navigation, pods, enter-key]
status: complete
last_updated: 2025-10-09
last_updated_by: Claude
---

# Research: Container Navigation - Pressing Enter on Pod to View Containers

**Date**: 2025-10-09T21:24:18+0000
**Researcher**: Claude
**Git Commit**: 131eb33487f06a1d90dfdf3db3eb0dda4e7ccf2f
**Branch**: docs/designs
**Repository**: k1-designs

## Research Question

How can users press Enter on a Pod to view and interact with its
containers?

## Summary

**Current State**: k1 has a robust Enter key navigation system for
resource-to-resource navigation (e.g., Deployment→Pods, Service→Pods)
using a config-driven `NavigationHandler` pattern with `FilterContext`.
However, Pods currently show only **aggregate container metrics**
(Ready: "2/3", Restarts: 15) and have **no NavigationHandler**
configured. There is NO existing implementation or documentation for
navigating from a Pod to view its individual containers.

**Infrastructure Available**:
- ✅ NavigationHandler function pointer pattern
  (`internal/screens/config.go:38`)
- ✅ ScreenSwitchMsg with FilterContext
  (`internal/types/types.go:110-115`)
- ✅ Enter key interception (`config.go:175-181`)
- ✅ Full-screen detail view component
  (`internal/components/fullscreen.go`)
- ✅ Container data in pod specs (name, image, ports, resources, etc.)

**What Does NOT Exist**:
- ❌ Pod→Container navigation handler
- ❌ Container list screen or detail view
- ❌ Container selection for multi-container pods
- ❌ Container-specific commands integration (/logs, /shell with auto
     container selection)

**Recommendation**: Extend the existing navigation pattern to add a
NavigationHandler for Pods that navigates to a new "containers" screen
showing individual containers with their specs and statuses.

## Detailed Findings

### 1. Current Enter Key Navigation System

**Entry Point**:
`internal/screens/config.go:175-181` (DefaultUpdate method)

When user presses Enter on any resource row:
```go
case key.Matches(msg, keys.Enter):
    if cmd := s.handleEnterKey(); cmd != nil {
        return s, cmd
    }
```

**Handler Delegation**:
`internal/screens/config.go:552-559` (handleEnterKey method)

```go
func (s *ConfigScreen) handleEnterKey() tea.Cmd {
    if s.config.NavigationHandler != nil {
        return s.config.NavigationHandler(s)
    }
    return nil  // No navigation configured
}
```

**Pods Screen Configuration**:
`internal/screens/screens.go:11-36` (GetPodsScreenConfig)

```go
func GetPodsScreenConfig() ScreenConfig {
    return ScreenConfig{
        // ...
        NavigationHandler: nil,  // ❌ No navigation configured!
        // ...
    }
}
```

**Finding**: Pods are intentionally configured as **leaf nodes** in the
navigation graph - they have NO NavigationHandler, so pressing Enter
does nothing.

### 2. Navigation Pattern Examples

**Deployment → Pods**
(`internal/screens/navigation.go:12-41`,
`internal/screens/screens.go:72`)

Factory function creates navigation handler:
```go
func navigateToPodsForOwner(kind string) NavigationFunc {
    return func(s *ConfigScreen) tea.Cmd {
        resource := s.GetSelectedResource()  // Get selected row
        namespace, _ := resource["namespace"].(string)
        name, _ := resource["name"].(string)

        return func() tea.Msg {
            return types.ScreenSwitchMsg{
                ScreenID: "pods",
                FilterContext: &types.FilterContext{
                    Field: "owner",
                    Value: name,
                    Metadata: map[string]string{
                        "namespace": namespace,
                        "kind":      kind,
                    },
                },
            }
        }
    }
}
```

Screen config assigns it:
```go
GetDeploymentsScreenConfig() ScreenConfig {
    NavigationHandler: navigateToPodsForOwner("Deployment"),
    // ...
}
```

**Service → Pods**
(`internal/screens/navigation.go:101-129`)

Different filter type for selector-based navigation:
```go
FilterContext{
    Field: "selector",           // Not "owner"
    Value: "web",                // Service name
    Metadata: {"kind": "Service"},
}
```

**Finding**: Navigation pattern is **flexible and extensible** - can
support Pod→Container navigation with a new factory function.

### 3. Container Data in Codebase

**Pod Type Structure**:
`internal/k8s/repository_types.go:30-37`

```go
type Pod struct {
    Namespace   string
    Name        string
    Ready       string  // "2/3" - aggregate container status
    Status      string  // "Running" - pod phase
    Restarts    int     // Sum of all container restarts
    Age         string
    IP          string
    Node        string
}
```

**Finding**: Pod struct stores only **aggregate container metrics**, not
individual container details (names, images, individual statuses).

**Container Data Extraction**:
`internal/k8s/transforms.go:47-90` (transformPod)

```go
containerStatuses, _, _ := unstructured.NestedSlice(obj.Object,
    "status", "containerStatuses")
readyContainers := 0
totalContainers := len(containerStatuses)
totalRestarts := 0

for _, cs := range containerStatuses {
    csMap := cs.(map[string]interface{})
    if ready, ok := csMap["ready"].(bool); ok && ready {
        readyContainers++
    }
    if restartCount, ok := csMap["restartCount"].(int64); ok {
        totalRestarts += int(restartCount)
    }
}
```

**Finding**: Container data **exists in Kubernetes API**
(pod.status.containerStatuses) but is currently **aggregated** before
display. Individual container information is discarded.

**Raw Container Data Available**:
From Kubernetes API, each pod has:
- `spec.containers[]`: name, image, ports, resources, env, volumeMounts
- `status.containerStatuses[]`: name, ready, restartCount, state
  (running/waiting/terminated), imageID

### 4. Container Commands

**Logs Command**:
`internal/commands/pod.go:56-60`

```go
func LogsCommand(repo k8s.Repository) ExecuteFunc {
    return func(ctx CommandContext) tea.Cmd {
        // ...
        if args.Container != "" {
            flags = append(flags, "-c", args.Container)
        }
        // Runs: kubectl logs <pod> -c <container>
    }
}
```

**Shell Command**:
`internal/commands/pod.go:114-118`

```go
func ShellCommand(repo k8s.Repository) ExecuteFunc {
    // ...
    if args.Container != "" {
        flags = append(flags, "-c", args.Container)
    }
    // Runs: kubectl exec <pod> -c <container> -- <shell>
}
```

**Finding**: Container-specific commands **already support** the
`-c <container>` flag, but users must **manually type the container
name**. No UI for selecting containers from the current pod.

### 5. Full-Screen Detail View Component

**Component Implementation**:
`internal/components/fullscreen.go:26-181`

Existing full-screen overlay for YAML/describe views:
- Keyboard navigation (↑↓, PgUp/PgDn, g/G)
- Scrolling with offset tracking
- Scroll indicator "X-Y of Z"
- ESC to exit

**Triggering Pattern**:
`internal/app/app.go:291-301`

```go
case types.ShowFullScreenMsg:
    content := strings.Split(msg.Content, "\n")
    m.fullScreen = fullscreen.NewFullScreen(
        msg.ViewType,
        msg.ResourceName,
        content,
    )
    m.fullScreen.SetSize(m.width, m.height)
    m.fullScreenMode = true
```

**Finding**: Could reuse FullScreen component for container details, or
create a dedicated container list screen (ConfigScreen with container
rows).

### 6. No Existing Container Navigation

**Search in thoughts/** found:
- ✅ Contextual navigation plan for resource-to-resource
  (`thoughts/shared/plans/2025-10-07-contextual-navigation.md`)
- ✅ Container operations in AI command research
  (`thoughts/shared/research/2025-10-09-ai-command-implementation.md` -
  lines 715, 720, 944)
- ❌ NO plans or designs for Pod→Container navigation
- ❌ NO discussion of container list views or selection UI

**Finding**: This would be a **new feature** requiring new design and
implementation.

## Code References

### Navigation Infrastructure
- `internal/screens/config.go:38` - NavigationFunc type definition
- `internal/screens/config.go:552-559` - handleEnterKey() delegation
- `internal/screens/config.go:175-181` - Enter key interception in
  DefaultUpdate()
- `internal/screens/navigation.go:12-379` - Navigation factory functions
- `internal/screens/screens.go:11-36` - Pods screen config (no
  NavigationHandler)
- `internal/types/types.go:110-115` - ScreenSwitchMsg definition
- `internal/types/types.go:77-82` - FilterContext definition

### Container Data
- `internal/k8s/repository_types.go:30-37` - Pod struct (aggregate
  metrics only)
- `internal/k8s/transforms.go:47-90` - Container status extraction
- `internal/k8s/informer_repository.go:296-328` - GetPods() with
  container calculations
- `internal/k8s/informer_repository_test.go:75-202` - Test cases with
  container scenarios

### Container Commands
- `internal/commands/pod.go:14-20` - LogsArgs and ShellArgs with
  Container field
- `internal/commands/pod.go:56-60` - LogsCommand with -c flag
- `internal/commands/pod.go:114-118` - ShellCommand with -c flag
- `internal/commands/registry.go:174,199` - Command definitions with
  [container] arg

### Detail Views
- `internal/components/fullscreen.go:26-181` - Full-screen overlay
  component
- `internal/app/app.go:291-301` - ShowFullScreenMsg handling
- `internal/commands/resource.go:26-111` - YamlCommand and
  DescribeCommand

## Architecture Insights

### 1. Config-Driven Navigation is Extensible

The current architecture uses **function pointers** in ScreenConfig to
avoid coupling the core ConfigScreen to specific resource types. This
pattern makes it **trivial to add Pod→Container navigation**:

```go
// Just add this to GetPodsScreenConfig():
NavigationHandler: navigateToContainersForPod(),
```

No modification to ConfigScreen.handleEnterKey() required.

### 2. Two Possible Approaches

**Option A: New Container Screen (Recommended)**
- Create new "containers" screen using ConfigScreen pattern
- Show table with columns: Name, Image, Ready, Restarts, State
- Each row represents one container in the selected pod
- Supports filtering, sorting, commands (/logs, /shell)
- Fits existing UI patterns

**Option B: Full-Screen Container Detail**
- Press Enter on pod → show full-screen view with all container specs
- Similar to /describe command output
- Less interactive, harder to select specific container for commands

**Recommendation**: Option A (new screen) provides better UX and
consistency with existing patterns.

### 3. FilterContext Design Pattern

Current FilterContext supports various filter types:
- `owner`: Filter by owner reference (Deployment/StatefulSet/etc)
- `node`: Filter by node name
- `selector`: Filter by label selector
- `namespace`: Filter by namespace

**New filter type needed**:
```go
FilterContext{
    Field: "pod",              // New filter type
    Value: "nginx-abc123",     // Pod name
    Metadata: {
        "namespace": "default",
    },
}
```

Repository would implement:
```go
func (r *Repository) GetContainersForPod(namespace, name string)
    ([]Container, error)
```

### 4. Container as First-Class Resource

Currently containers are **not** first-class resources in k1's
architecture:
- No Container type in `repository_types.go`
- No container screen in screen registry
- No container commands (except as args to pod commands)

**Architectural shift required**: Elevate containers to first-class
resources with:
- `Container` type struct
- Container screen config
- Container-specific operations (logs, shell, attach, port-forward)
- Transform function for container data

### 5. Multi-Container Pod Complexity

Many pods have **multiple containers** (app + sidecar):
- Init containers (run before main containers)
- App containers (main application)
- Sidecar containers (logging, monitoring, proxies)

Container screen should:
- Group by container type (init vs regular)
- Show container lifecycle (Waiting, Running, Terminated)
- Highlight failed containers
- Support operations on specific containers

## Historical Context (from thoughts/)

### Contextual Navigation Implementation
`thoughts/shared/plans/2025-10-07-contextual-navigation.md`

Original plan that established Enter key navigation pattern:
- Introduced NavigationHandler function pointer
- Created FilterContext struct
- Implemented Deployment→Pods, Node→Pods, Service→Pods
- **Explicitly made Pods the "leaf node"** (no further navigation)

**Quote**: "Pods screen (NO NavigationHandler - end of navigation
chain)"

This was a **deliberate design decision** at the time, not an oversight.
The feature was scoped to resource-to-resource navigation, not
sub-resource navigation.

### Container Operations in AI Command
`thoughts/shared/research/2025-10-09-ai-command-implementation.md`

Research shows containers are important for AI command validation:
- Line 715: kubectl logs command
- Line 720: Container name parameter
- Line 944: Log retrieval for pods
- Line 1005: kubectl exec for shell access
- Line 1207-1209: Multi-container handling

This suggests **users will want container-level operations** via natural
language, which would benefit from UI-based container selection.

### Testing Infrastructure
`thoughts/shared/research/2025-10-09-bubble-tea-tui-testing.md`

Line 354: "Enter key interception for navigation" is marked as
**untestable** in Bubble Tea TUI tests.

**Implication**: Container navigation would need **integration tests**
at the repository/screen level, not pure TUI tests.

## Related Research

- `thoughts/shared/research/2025-10-07-contextual-navigation.md` -
  Original navigation research
- `thoughts/shared/research/2025-10-09-yaml-describe-search-feature.md`
  - Detail view patterns
- `thoughts/shared/research/2025-10-09-ai-command-implementation.md` -
  Container operations

## Open Questions

### 1. Screen vs Modal vs Full-Screen?

**Should container view be**:
- A) New dedicated screen in screen registry (like pods, deployments)
- B) Modal overlay on pods screen
- C) Full-screen detail view (like /describe)

**Trade-offs**:
- Screen: Most consistent, supports all operations, requires screen
  registry changes
- Modal: Lighter weight, stays "within" pods screen, but overlays have
  issues
- Full-screen: Quickest to implement, but less interactive

### 2. Container Type Definition?

**What fields should Container struct have**:
```go
type Container struct {
    // Identity
    PodName       string
    PodNamespace  string
    ContainerName string

    // Spec
    Image         string
    ImagePullPolicy string
    Ports         []ContainerPort  // or string representation?

    // Status
    Ready         bool
    RestartCount  int
    State         string  // "Running", "Waiting", "Terminated"
    StateReason   string  // Why waiting/terminated

    // Resources
    CPURequest    string
    MemoryRequest string
    CPULimit      string
    MemoryLimit   string

    // Lifecycle
    Type          string  // "init", "app", "ephemeral"
}
```

### 3. Init Containers?

Pods can have **init containers** that run before main containers.
Should they:
- Be shown in the same list with a "Type" column?
- Be shown in a separate section?
- Be hidden unless user expands?

### 4. Navigation Back Behavior?

When user presses ESC from container screen:
- Should return to pods screen
- Should restore previous pod selection cursor position
- Should clear the pod filter context?

Current back-navigation already handles this via
`navigationHistory` stack in `app.go:317-357`.

### 5. Container Commands Integration?

Currently `/logs` and `/shell` accept optional `[container]` argument:
```
/logs nginx  (specify container name)
/logs        (defaults to first container)
```

After implementing container screen:
- Should commands pre-fill container name from selected row?
- Should `/logs` on pods screen show container selection if multiple?
- Should shortcuts (Ctrl+L?) work differently in containers screen?

### 6. Performance Considerations?

Container data is already fetched (it's in pod.status.containerStatuses):
- **No additional API calls needed** for basic container list
- Container screen would just **unfold** existing data
- Transform function would extract container details from pod object

**Memory impact**: Minimal - just storing flattened container objects
instead of aggregating them.

### 7. Should All Screens Support Enter?

Currently only some screens have NavigationHandler:
- Deployments → Pods ✅
- Services → Pods ✅
- Nodes → Pods ✅
- Pods → ??? ❌

If Pods get Enter key support:
- Should ConfigMaps/Secrets also navigate (to pods using them)?
- Should Namespaces navigate (to all pods in namespace)?

**Answer**: They already do! See `internal/screens/screens.go`:
- ConfigMaps: navigateToPodsForVolumeSource("ConfigMap")
- Secrets: navigateToPodsForVolumeSource("Secret")
- Namespaces: navigateToPodsForNamespace()

So Pods are currently the **only major resource** without navigation.

## Implementation Checklist (If Proceeding)

**Phase 1: Foundation**
- [ ] Define Container type in `repository_types.go`
- [ ] Add transformContainer() function in `transforms.go`
- [ ] Add GetContainersForPod() method to Repository interface
- [ ] Implement GetContainersForPod() in InformerRepository
- [ ] Add dummy implementation for testing

**Phase 2: Screen**
- [ ] Create GetContainersScreenConfig() in `screens.go`
- [ ] Define container columns (Name, Image, Ready, Restarts, State)
- [ ] Add "containers" to screen registry in `app.go`
- [ ] Create navigation factory navigateToContainersForPod() in
      `navigation.go`
- [ ] Update GetPodsScreenConfig() to use new NavigationHandler

**Phase 3: Navigation**
- [ ] Add "pod" filter type to FilterContext dispatch logic in
      `config.go:refreshWithFilterContext()`
- [ ] Test Enter key on pod row navigates to containers screen
- [ ] Test ESC key returns to pods screen with restored state
- [ ] Update header to show "Containers for pod: nginx-abc123"

**Phase 4: Commands**
- [ ] Update /logs command to pre-fill container name from selected row
- [ ] Update /shell command to pre-fill container name from selected row
- [ ] Consider /restart command for individual containers
- [ ] Add container-specific operations to Operations list

**Phase 5: Testing**
- [ ] Add Container type tests in `repository_types_test.go`
- [ ] Add transformContainer tests in `transforms_test.go`
- [ ] Add GetContainersForPod tests in `informer_repository_test.go`
- [ ] Add container screen navigation tests in `navigation_test.go`
- [ ] Add container screen config tests in `screens_test.go`

**Phase 6: Documentation**
- [ ] Update CLAUDE.md with container navigation pattern
- [ ] Update README.md with Enter key behavior for pods
- [ ] Create design document (DDR-XX) for container navigation
- [ ] Add examples to help text

## Conclusion

**The infrastructure exists to support Pod→Container navigation** via
the NavigationHandler pattern. The main work involves:
1. Elevating containers to first-class resources with their own type and
   screen
2. Creating a navigation factory function for Pod→Container
3. Implementing repository method to extract container data from pods
4. Integrating with existing commands (/logs, /shell)

The architecture is **well-positioned** for this feature due to the
config-driven navigation design established in the contextual navigation
implementation.

**Estimated effort**: Medium (2-3 days)
- Day 1: Repository layer (Container type, transform, repository method)
- Day 2: Screen and navigation (screen config, factory function,
  registration)
- Day 3: Testing and polish (tests, documentation, command integration)

**User value**: High - provides visibility into multi-container pods and
streamlines container-specific operations (logs, shell, debugging).
