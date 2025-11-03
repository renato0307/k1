# Roadmap for Next Features

## Urgent

1. [ ] Search on yaml
2. [ ] Save yaml
3. [ ] Basic edit using kubectl command generation
4. [ ] Sticky namespaces
5. [X] Basic CRD support
6. [ ] :ns not implemented (ns goes default ns <name> goes to that namespace), with auto-suggest/auto-complete

## Important

1. [ ] Log streaming support
2. [ ] Support more resource types
3. [ ] Shortcut to expand/collapse a column with copy+paste of the value
3. [ ] Configuration file (e.g. ~/.k1s/config.yml) - ENABLER

## UX

1. [ ] Show something on the pod list if it is the first screen
2. [ ] Allow user to say which screen to show first
3. [ ] Search: support "nginx !default"

## Nice to have

1. [ ] AI assistant for generating kubectl commands
2. [ ] Screens by configuration

## Refactor/Tech Debt

1. [ ] app.go contains too much logic, specially about contexts; we should refactor it into smaller components, needs a research on how to best do it

2. [ ]  Lack of encapsulation: The height calculation contract between app.go and screens is implicit, not explicit. Each screen
  reimplements sizing logic independently, leading to:

  1. ❌ Inconsistent behavior (Pods fills space, Output doesn't)
  2. ❌ Easy to make mistakes (double subtraction)
  3. ❌ Hard to change (must update all screens)
  4. ❌ No compile-time guarantees

  This is a classic case where composition over inheritance would help, but Go requires more explicit patterns to achieve it.

3. [ ] Context Deadline Exceeded

  1. Context Deadline Exceeded (Main Issue)
  Lines 216-273 show widespread timeout errors across all resource types:
  time=2025-11-02T18:59:24.007Z level=WARN msg="Dynamic informer sync failed"
  resource=endpoints tier=1
  error="Get \"https://.../api/v1/endpoints?limit=1&timeout=1m30s\": context deadline exceeded"

  This affects: endpoints, pods, deployments, services, jobs, configmaps, secrets, nodes, replicasets,
  horizontalpodautoscalers, daemonsets, cronjobs

  Root Causes:
  1. Large cluster - Line 18 shows ~1008 pods, 798 deployments, 642 services
  2. Network latency - EKS clusters in us-east-1, slow API responses
  3. Parallel informer sync - Line 7: "resource_count=16" - all syncing simultaneously
  4. Short timeout - 90-second timeout (timeout=1m30s) insufficient for initial list operations

  2. Permission Errors (Expected)
  Lines 20, 21, 70, 72, 104, 105, 138, 139, 196, 197, 257:
  "secrets is forbidden: User \"readonly-users\" cannot list resource \"secrets\""
  This is expected for read-only users.

  3. ReplicaSet Informer Timeouts
  Lines 76, 175-176, 237-238: ReplicaSet informers not syncing within timeout window.

  This is NOT related to Phase 3 changes

  My Phase 3 work only touched:
  - CommandContext struct (added OriginalCommand field)
  - Command implementations (scale, restart, describe)
  - BuildContext call sites

  None of these affect informer timeouts or API client configuration.

  The Real Problem

  The dynamic informer implementation in internal/k8s/ has aggressive timeouts that don't scale well for:
  - Large clusters (1000+ pods)
  - High network latency (remote EKS clusters)
  - Parallel resource syncing (16 resources at once)

  This is a pre-existing infrastructure issue, not caused by the command output history feature.

4. [ ] All screens list based must use ConfigScreen

## Bugs

[X] Columns with bad sizing on configmaps (Fixed: Phase 1)
[X] Columns with bad sizing on daemonsets (Fixed: Phase 1)
[X] Columns with bad sizing on cronjobs (Fixed: Phase 1)
[X] Columns with bad sizing on crds (Fixed: Phase 1)
[X] Pageup/down hides the selected row (Fixed: Phase 2)
[X] When filtering, the selected row is not always visible (Fixed: Phase 2)
[X] Show the number of items e.g. Pods (50) (Fixed: Phase 1)
[X] The filtered search is not sorted (Fixed: Phase 3)
[X] HPAs cannot do yaml (Fixed: Phase 5 - also fixed ReplicaSets, PVCs, Ingresses, Endpoints)
[X] Missing Spec field in describe output (Fixed: Phase 5)
[X] Failed to refresh Consumer: informer not registered for jetstream.nats.io/v1beta2, Resource=consumers
[X] CRDs are not sorted

### Context related

[ ] Better error handling with multiple -context flags if we fail to connect to one of the clusters
[ ] If cannot connect to cluster, the connecting to API Server is showing spinning
[ ] If context load fails the error is not shown

### UI related

[ ] When list is filtered, page up/page down makes the selected row disappear
[ ] Start app, go to :nodes when pods are still loading, the nodes shows up but the header is gone, navigating makes it show again