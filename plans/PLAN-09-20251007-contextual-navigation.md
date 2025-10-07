# PLAN-09: Contextual Navigation with Enter Key

**Status**: Proposed
**Created**: 2025-10-07
**DDR Reference**: DDR-13 (Contextual Navigation with Enter Key)
**Branch**: TBD (suggest: `feat/contextual-navigation`)

## Overview

Implement intuitive drill-down navigation using the Enter key to navigate from
parent resources to their related child resources. This enables users to
quickly explore resource relationships (e.g., Deployment → Pods, Service →
Pods, Pod → Containers) without manually typing filters or label selectors.

**Scope**: All 11 resource screens plus new Containers screen
**Out of Scope**: Advanced multi-hop navigation, graph visualization

## Goals

1. **Consistent UX**: Enter key navigates to the most useful related resource
   for every resource type
2. **Fast Workflows**: Eliminate manual filter typing for common navigation
   patterns
3. **Reversible Navigation**: ESC/q returns to previous screen with state
   preserved
4. **Visual Feedback**: Clear breadcrumb trail showing current navigation path
5. **New Capability**: Containers screen for multi-container pod inspection

## Phases

### Phase 1: Core Navigation Infrastructure (~2-3 days)

**Key components**:
- Navigation stack for back/forward history
- Breadcrumb component showing navigation trail (e.g., "Pods (Deployment:
  my-app)")
- Pre-applied filter state when navigating to filtered screens
- ESC/q handler to pop navigation stack and restore previous screen

**Key decisions**:
- Navigation stack stored in root app.Model
- Breadcrumbs display in header component (compact format)
- Filters are read-only with visual "Clear" option
- Selection persistence when navigating back

**Success criteria**: Can navigate forward (Enter) and back (ESC) with state
preservation

### Phase 2: High-Value Navigations (~2-3 days)

**Implement Enter key for**:
- Deployments → Pods (filter by `.spec.selector.matchLabels`)
- StatefulSets → Pods (filter by `.spec.selector.matchLabels`)
- DaemonSets → Pods (filter by `.spec.selector.matchLabels`)
- Services → Pods (filter by `.spec.selector` labels)
- Nodes → Pods (filter by `.spec.nodeName`)

**Key decisions**:
- Extract label selector helper for consistent filtering
- Reuse existing Pods screen with pre-applied filters
- Show "No pods found" hint if selector doesn't match any pods

**Success criteria**: 5 most-used navigation patterns working end-to-end

### Phase 3: Containers Screen (~3-4 days)

**New screen showing**:
- Container Name | Image | Ready | State | Restarts | CPU | Memory
- Pre-filtered to selected pod (namespace/name)
- Container-level operations: /logs, /shell, /describe

**Key decisions**:
- Use existing config-driven screen pattern (DDR-07)
- Container state parsing (Waiting/Running/Terminated with reasons)
- Resource limits/requests display (from container spec)
- Support init containers and ephemeral containers

**Success criteria**: Pods → Containers navigation fully functional

### Phase 4: Remaining Resources + Polish (~2-3 days)

**Complete Enter key for**:
- Jobs → Pods (filter by controller-uid or selector)
- CronJobs → Jobs (filter by ownerReference)
- Namespaces → Pods (filter by namespace)
- ConfigMaps → Detail view (modal with all key-value pairs)
- Secrets → Detail view (modal with masked values, /reveal-key command)

**Polish items**:
- Help text updates showing "(Press Enter for X)" hints
- Keyboard shortcuts reference in help
- Visual indicator when row is selected
- Performance testing with large clusters (1000+ pods)

**Success criteria**: All 11 resources have working Enter key navigation

## Architectural Decisions

**Navigation Stack vs Routing**:
- Use simple stack (push/pop) instead of full router
- Rationale: Simpler implementation, sufficient for linear drill-down

**Filter Representation**:
- Pre-applied filters stored as CommandContext state
- Displayed in command bar as read-only chips
- Rationale: Clear visibility without cluttering the main view

**Breadcrumb Format**:
- Compact: "Pods (Deployment: my-app)" in header
- Alternative considered: Full path "Deployments > my-app > Pods" (too long)
- Rationale: Screen space is precious in TUI

**Containers as New Screen vs Modal**:
- Decision: New screen (implements types.Screen interface)
- Rationale: Containers have operations (/logs, /shell) that benefit from
  command bar integration
- Alternative considered: Modal view (rejected - less flexible)

## Risks and Mitigations

**Risk**: Navigation stack grows unbounded with deep drill-downs
**Mitigation**: Limit stack depth to 10 levels, show warning if exceeded

**Risk**: Label selector filtering performance on large clusters
**Mitigation**: Informer cache should make this fast (<50ms); add benchmarks
to verify

**Risk**: Breadcrumb text too long for narrow terminals
**Mitigation**: Truncate middle of resource names ("my-very-lon...app-name")

**Risk**: Containers screen requires new data fetching patterns
**Mitigation**: Containers are already available in pod.Spec, no new API calls
needed

**Risk**: User confusion about Enter vs /describe vs /yaml
**Mitigation**: Help text clearly distinguishes: Enter (navigate), /describe
(view), /yaml (raw)

## Success Criteria

- [ ] Navigation stack with forward/back (Enter/ESC) working
- [ ] Breadcrumb trail shows current navigation path
- [ ] 8 resource types navigate to Pods with correct filters
- [ ] Containers screen shows all containers for selected pod
- [ ] ConfigMaps/Secrets show detail modal on Enter
- [ ] CronJobs → Jobs navigation working
- [ ] Selection and scroll position preserved on back navigation
- [ ] Help text updated with Enter key hints
- [ ] Performance: navigation <100ms on 1000-pod clusters
- [ ] Zero regressions in existing navigation (:/colon palette)

## TODO

### Phase 1: Navigation Infrastructure
- [ ] Add navigation stack to app.Model (max depth 10)
- [ ] Implement pushNavigation/popNavigation helpers
- [ ] Create breadcrumb component for header
- [ ] Add pre-applied filter state to Screen interface
- [ ] Update ESC handler to check navigation stack before quitting
- [ ] Test: forward/back navigation preserves state

### Phase 2: High-Value Navigations
- [x] Extract getLabelSelector() helper for Deployment/StatefulSet/DaemonSet
- [x] Implement Deployments → Pods navigation with filter
- [x] Implement Services → Pods navigation (via selector)
- [x] Implement Nodes → Pods navigation (via nodeName)
- [x] Implement StatefulSets → Pods navigation
- [x] Implement DaemonSets → Pods navigation
- [x] Implement Jobs → Pods navigation
- [ ] Add "No pods found" hint for empty filtered results (deferred)
- [ ] Test: label selector matching behaves like kubectl (needs actual filtering)

**Note**: Phase 2 implemented navigation infrastructure and Enter key handlers. The actual filtering of pods based on navigation context (label selectors, node name) is deferred and will require additional work to access the underlying unstructured resource data and apply proper Kubernetes label matching logic.

### Phase 3: Containers Screen
- [ ] Create internal/screens/containers.go (config-driven)
- [ ] Add container transform function (extracting state, resources, etc.)
- [ ] Implement Pods → Containers navigation (filtered by pod name)
- [ ] Add container operations: /logs, /shell, /describe
- [ ] Support init containers and ephemeral containers
- [ ] Test: multi-container pods show all containers
- [ ] Test: container state parsing (Waiting/Running/Terminated)

### Phase 4: Remaining Resources + Polish
- [ ] Implement Jobs → Pods navigation (via controller-uid)
- [ ] Implement CronJobs → Jobs navigation (via ownerReference)
- [ ] Implement Namespaces → Pods navigation (via namespace filter)
- [ ] Create detail modal for ConfigMaps (show all key-value pairs)
- [ ] Create detail modal for Secrets (masked values, /reveal-key)
- [ ] Update help text: "(Press Enter for X)" hints
- [ ] Add visual indicator for selected row
- [ ] Performance benchmark: 1000-pod cluster navigation
- [ ] Update README.md and CLAUDE.md with new navigation patterns

## Notes

- Follow existing patterns: config-driven screens (DDR-07), message helpers
  (internal/messages)
- Keep Enter key behavior predictable: always navigates to "most useful"
  related resource
- Breadcrumbs should be compact but informative (truncate if needed)
- Consider adding Shift+Enter for alternative actions in future (out of scope
  for this plan)
- Navigation stack enables future features: back/forward buttons, navigation
  history command
