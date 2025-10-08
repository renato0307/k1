# Add Spec Section to Describe Output

## Overview

Add the Spec section to the `/describe` command output to match kubectl
describe format. Currently, the describe output only shows metadata, status,
and events - it's missing the spec field which contains crucial resource
configuration details like replicas, selectors, templates, and strategies.

## Current State Analysis

**Current describe output** (`internal/k8s/informer_repository.go:711-799`):
1. Metadata: Name, Namespace, Kind, API Version, Labels, Created (lines
   748-770)
2. Status: Formatted as YAML with 2-space indent (lines 773-785)
3. Events: Fetched on-demand and formatted as table (lines 787-796)

**Missing**: Spec section between metadata and status

**Why it's missing**: The `DescribeResource()` method never extracts the spec
field from the unstructured object, even though the data is available via
`unstructured.NestedFieldCopy(obj.Object, "spec")`.

### Key Discoveries:
- Spec data is fully accessible from `unstructured.Unstructured` objects
  (`informer_repository.go:773`)
- Status section already uses the exact pattern we need
  (`unstructured.NestedFieldCopy` + YAML marshal + 2-space indent)
- All 11 resource types have spec fields (Pods, Deployments, Services,
  ConfigMaps, Secrets, Namespaces, StatefulSets, DaemonSets, Jobs, CronJobs,
  Nodes)
- Some resources might not have spec field - code should gracefully skip if
  not found

## Desired End State

After implementation, `/describe` output should match kubectl describe format
with all three sections in correct order:

```
Name:         nats-proxy
Namespace:    kube-gloo-system
Kind:         Deployment
API Version:  apps/v1
Labels:       gateway-proxy-id=nats-proxy
              gloo=gateway-proxy
Created:      2022-06-06 17:59:03 +0100 WEST

Spec:
  replicas: 3
  selector:
    matchLabels:
      app: nats-proxy
  template:
    metadata:
      labels:
        app: nats-proxy
    spec:
      containers:
      - name: proxy
        image: nats:latest
  strategy:
    type: RollingUpdate

Status:
  availableReplicas: 3
  conditions:
  - lastTransitionTime: "2025-10-02T13:39:58Z"
    message: ReplicaSet "nats-proxy-7b9bbb4bc4" has successfully progressed.
    reason: NewReplicaSetAvailable
    status: "True"
    type: Progressing

Events:
  <none>
```

**Verification**: Run `/describe` (or `ctrl+d`) on any resource and confirm
spec section appears between metadata and status with proper formatting.

## What We're NOT Doing

- NOT implementing resource-specific describers (kubectl has PodDescriber,
  DeploymentDescriber, etc.) - keeping generic implementation
- NOT adding spec field filtering or prettification - show raw YAML as-is
- NOT caching spec data separately - use existing unstructured objects
- NOT changing status or events sections - only adding spec
- NOT modifying YAML output (`/yaml` command) - that already shows full
  resource

## Implementation Approach

**Single-file change**: Add spec extraction to `DescribeResource()` method
in `internal/k8s/informer_repository.go`.

**Pattern reuse**: Copy the exact pattern used for status extraction (lines
773-785) and apply it to spec, inserting between the Created timestamp and
Status section.

**Error handling**: If spec field not found or marshal fails, silently skip
the Spec section (same as status handling).

1: Add Spec Section to Describe Output

### Overview
Modify the `DescribeResource()` method to extract and display the spec field
using the same YAML formatting pattern as the status section.

### Changes Required:

#### 1. Modify DescribeResource Method
**File**: `internal/k8s/informer_repository.go`
**Location**: Lines 772-786 (insert after Created, before Status)

**Add spec extraction code**:

```go
// Add spec if present, formatted as YAML (insert at line ~772)
spec, found, err := unstructured.NestedFieldCopy(obj.Object, "spec")
if found && err == nil {
    specYAML, err := yaml.Marshal(spec)
    if err == nil {
        buf.WriteString("\nSpec:\n")
        // Indent spec YAML by 2 spaces
        for _, line := range strings.Split(string(specYAML), "\n") {
            if line != "" {
                buf.WriteString("  " + line + "\n")
            }
        }
    }
}
```

**Insertion point**: After line 770 (Created timestamp), before line 773
(Status section).

**Pattern**: Exact copy of status extraction pattern (lines 773-785), just
replace "status" with "spec".

#### 2. Update Tests
**File**: `internal/k8s/informer_repository_test.go`
**Location**: Lines 1112-1202 (`TestInformerRepository_DescribeResource`)

**Modify test expectations**:

```go
// Update assertions to check for Spec section
assert.Contains(t, desc, "Spec:")
assert.Contains(t, desc, "containers:")
assert.Contains(t, desc, "name: nginx")
assert.Contains(t, desc, "image: nginx:latest")

// Verify order: Spec appears before Status
specIdx := strings.Index(desc, "Spec:")
statusIdx := strings.Index(desc, "Status:")
assert.True(t, specIdx > 0, "Spec section should be present")
assert.True(t, statusIdx > specIdx, "Status should appear after Spec")
```

**Test resource**: The test already creates a pod with spec (containers,
restartPolicy) at lines 1121-1154, so spec content will be available.

### Success Criteria:

#### Automated Verification:
- [ ] All unit tests pass: `make test`
- [ ] Describe test verifies spec presence and ordering
- [ ] No linting errors: `make check`
- [ ] Code compiles without errors: `go build cmd/k1/main.go`

#### Manual Verification:
- [ ] Run `/describe` on a Deployment and verify spec shows replicas,
      selector, template, strategy
- [ ] Run `/describe` on a Pod and verify spec shows containers,
      restartPolicy, volumes
- [ ] Run `/describe` on a Service and verify spec shows ports, selector,
      type, clusterIP
- [ ] Verify spec appears between Created and Status sections
- [ ] Verify 2-space indentation matches status formatting
- [ ] Test on all 11 resource types to ensure no errors

**Implementation Note**: After completing this phase and all automated
verification passes, pause here for manual confirmation from the human that
the manual testing was successful before marking as complete.

---

## Testing Strategy

### Unit Tests:
**File**: `internal/k8s/informer_repository_test.go:1112-1202`

**What to test**:
- Spec section appears in describe output
- Spec contains expected fields (containers, replicas, etc.)
- Spec appears after Created timestamp
- Spec appears before Status section
- 2-space indentation applied correctly
- Resources without spec don't cause errors

**Existing test infrastructure**:
- Uses envtest with real Kubernetes API server
- Creates test pod with full spec (containers, volumes, restartPolicy)
- Test namespace isolation prevents conflicts
- Table-driven test structure for multiple scenarios

### Manual Testing Steps:
1. Build and run k1: `make build && ./k1 -dummy`
2. Navigate to Deployments screen (`:deployments`)
3. Select a deployment and press `ctrl+d`
4. Verify spec section shows:
   - replicas: 3
   - selector with matchLabels
   - template with pod spec
   - strategy with RollingUpdate
5. Press ESC to exit, navigate to Pods (`:pods`)
6. Select a pod and press `ctrl+d`
7. Verify spec section shows:
   - containers with name, image, ports
   - restartPolicy
   - volumes (if any)
8. Repeat for Services, StatefulSets, Nodes
9. Verify formatting consistency across all resource types

## Performance Considerations

**No performance impact**:
- Spec data already loaded in memory (part of unstructured object from
  informer cache)
- No additional API calls required
- YAML marshaling is fast (same as status section)
- Only affects describe command (not hot path)

## Migration Notes

Not applicable - this is a purely additive change with no data migration or
breaking changes.

## References

- Original ticket: `thoughts/shared/tickets/issue_2.md`
- Describe implementation: `internal/k8s/informer_repository.go:711-799`
- Status extraction pattern: `internal/k8s/informer_repository.go:773-785`
- Existing tests: `internal/k8s/informer_repository_test.go:1111-1282`
- Unstructured field access examples: `internal/k8s/transforms.go:46-419`
