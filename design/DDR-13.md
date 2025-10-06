# Contextual Navigation with Enter Key

| Metadata | Value              |
|----------|--------------------|
| Date     | 2025-10-06         |
| Author   | @renato0307        |
| Status   | `Proposed`         |
| Tags     | navigation, ux     |

| Revision | Date       | Author      | Info           |
|----------|------------|-------------|----------------|
| 1        | 2025-10-06 | @renato0307 | Initial design |

## Context and Problem Statement

Currently, k1 supports browsing resource lists with filtering, sorting,
and commands (/describe, /yaml, etc.). However, there's no quick way to
"drill down" from a parent resource to its related child resources. For
example, viewing a Deployment should allow instant navigation to its
managed Pods, or viewing a Service should show which Pods it routes to.

The Enter key is underutilized and represents the most intuitive
interaction for "go deeper" or "show details" in a hierarchical
interface. This design proposes consistent Enter key behavior across all
11 resource screens.

## Prior Work

Similar tools implement drill-down navigation:
- **k9s**: Enter key navigates to related resources (Deployment → Pods)
- **kubectl**: Manual filtering required (`kubectl get pods -l app=foo`)
- **Lens**: Click-based navigation through resource relationships
- **VS Code**: Enter opens files, expands trees

## Design

### Core Principle

The Enter key should navigate to the **most useful related resource** or
**detailed view** based on the current context. The behavior should be:
- **Predictable**: Users learn the pattern once, apply everywhere
- **Contextual**: Action matches the resource's natural relationships
- **Reversible**: Easy to navigate back (ESC or q to previous screen)
- **Fast**: No intermediate confirmations or modals

### Enter Key Behavior by Resource

#### 1. Pods → Container List
**Action**: Show list of containers within the selected pod

**Why**: Pods are multi-container units. Users need to:
- Select which container for logs/shell commands
- View per-container resource limits/requests
- See container status (waiting, running, terminated)

**Implementation**: New "Containers" screen showing:
- Container Name | Image | Ready | State | Restarts | CPU | Memory
- Pre-filtered to selected pod (namespace/name)
- Operations: /logs, /shell, /describe

**Navigation**: Enter on pod → Containers screen (filtered to that pod)

---

#### 2. Deployments → Pod List
**Action**: Navigate to Pods screen filtered by deployment's label
selector

**Why**: Deployments manage ReplicaSets which manage Pods. Users need to:
- Verify all replicas are running
- Check pod status distribution across nodes
- Troubleshoot individual pod failures

**Implementation**:
- Extract Deployment's `.spec.selector.matchLabels`
- Navigate to existing Pods screen with pre-applied filter
- Filter format: `namespace=X AND labels match deployment selector`
- Show breadcrumb: "Pods (Deployment: my-app)"

**Navigation**: Enter on deployment → Pods screen (filtered)

---

#### 3. Services → Pod List (via Endpoints)
**Action**: Navigate to Pods screen showing pods backing the service

**Why**: Services route traffic to pods via endpoints. Users need to:
- Verify which pods are receiving traffic
- Troubleshoot routing issues (no endpoints = no pods match selector)
- Check health of backend pods

**Implementation**:
- Query service's `.spec.selector` labels
- Navigate to Pods screen with matching label filter
- Show breadcrumb: "Pods (Service: my-svc)"
- Note: Empty list indicates no pods match selector (common issue)

**Navigation**: Enter on service → Pods screen (filtered by selector)

---

#### 4. ConfigMaps → Detail View
**Action**: Show full ConfigMap contents in a scrollable detail modal

**Why**: ConfigMaps contain configuration data. Users need to:
- View all key-value pairs
- Copy specific configuration values
- Verify mounted config content

**Implementation**:
- New modal or full-screen detail view
- Show all data keys with values (YAML or table format)
- Operations: /yaml, /copy-key, ESC to close
- Support large configs (>1MB) with pagination

**Navigation**: Enter on configmap → Detail modal

---

#### 5. Secrets → Detail View (Masked)
**Action**: Show Secret metadata and keys in a detail modal

**Why**: Secrets contain sensitive data. Users need to:
- View available keys (not values by default)
- Optionally reveal specific values with confirmation
- Copy decoded values for debugging

**Implementation**:
- Show keys with masked values (••••••••)
- Operation: /reveal-key (with confirmation prompt)
- Operation: /copy-key (copies decoded value to clipboard)
- Show type and size metadata

**Navigation**: Enter on secret → Detail modal (values masked)

---

#### 6. Namespaces → Pod List (Namespace Context)
**Action**: Navigate to Pods screen filtered to selected namespace

**Why**: Namespaces are logical groupings. Users need to:
- Explore what's running in a namespace
- Verify namespace is empty before deletion
- Quickly assess namespace resource usage

**Implementation**:
- Navigate to Pods screen with namespace filter
- Show breadcrumb: "Pods (Namespace: my-ns)"
- Alternative: Show resource summary screen (pods, services, etc.)

**Navigation**: Enter on namespace → Pods screen (filtered to namespace)

---

#### 7. StatefulSets → Pod List
**Action**: Navigate to Pods screen filtered by statefulset's selector

**Why**: StatefulSets manage stateful pods with stable identities. Users
need to:
- Verify pod naming sequence (app-0, app-1, app-2)
- Check which pods are running vs pending
- Troubleshoot volume attachment issues

**Implementation**:
- Extract StatefulSet's `.spec.selector.matchLabels`
- Navigate to Pods screen with label filter
- Show breadcrumb: "Pods (StatefulSet: my-db)"

**Navigation**: Enter on statefulset → Pods screen (filtered)

---

#### 8. DaemonSets → Pod List
**Action**: Navigate to Pods screen filtered by daemonset's selector

**Why**: DaemonSets run one pod per node. Users need to:
- Verify pod distribution across all nodes
- Check which nodes are missing pods
- Troubleshoot node-specific issues

**Implementation**:
- Extract DaemonSet's `.spec.selector.matchLabels`
- Navigate to Pods screen with label filter
- Show breadcrumb: "Pods (DaemonSet: my-agent)"
- Optionally: Show node coverage summary

**Navigation**: Enter on daemonset → Pods screen (filtered)

---

#### 9. Jobs → Pod List
**Action**: Navigate to Pods screen filtered by job's pods

**Why**: Jobs create pods to run batch workloads. Users need to:
- View all pods created by the job (succeeded, failed, running)
- Troubleshoot failures by checking pod logs
- Verify job completion status

**Implementation**:
- Extract Job's `.spec.selector.matchLabels` or use controller-uid
- Navigate to Pods screen with label filter
- Show breadcrumb: "Pods (Job: my-import)"
- Include completed pods (not filtered out)

**Navigation**: Enter on job → Pods screen (filtered)

---

#### 10. CronJobs → Job List
**Action**: Navigate to Jobs screen filtered by cronjob's created jobs

**Why**: CronJobs create jobs on schedule. Users need to:
- View job execution history
- Check success/failure rates
- Troubleshoot missed or failed runs

**Implementation**:
- Filter Jobs by ownerReference pointing to the CronJob
- Navigate to Jobs screen with filter
- Show breadcrumb: "Jobs (CronJob: my-backup)"
- Note: May need new Jobs screen (currently exists per configs)

**Navigation**: Enter on cronjob → Jobs screen (filtered)

---

#### 11. Nodes → Pod List (Scheduled on Node)
**Action**: Navigate to Pods screen showing all pods on selected node

**Why**: Nodes run pods. Users need to:
- See what's running on a specific node
- Troubleshoot node resource pressure (too many pods)
- Verify pod distribution before draining

**Implementation**:
- Filter Pods by `.spec.nodeName` field
- Navigate to Pods screen with filter
- Show breadcrumb: "Pods (Node: my-node-xyz)"

**Navigation**: Enter on node → Pods screen (filtered)

---

### Navigation State Management

**Breadcrumb Trail**:
- Show navigation path in header: "Pods (Deployment: my-app)"
- ESC or 'q' returns to previous screen
- Maintain filter/sort state when navigating back

**Filter Handling**:
- Pre-apply filter when navigating to child screen
- Show filter in command bar (read-only, with "Clear" option)
- Allow additional filtering on top (AND logic)

**Selection Persistence**:
- Remember last selected row when returning to parent screen
- Scroll to previously selected item

### UI Indicators

**Visual Cues**:
- Show "(Press Enter for X)" hint in help bar when row is selected
- Example: "Pods | ↑↓: navigate  enter: show containers  /: command"
- Highlight Enter key capability in quick-start tutorial

**Keyboard Shortcuts Summary**:
```
enter     - Drill down to related resources
esc/q     - Return to previous screen
/         - Open command palette
:         - Open navigation palette
```

## Consequences

### Positive

1. **Improved Discoverability**: Users learn relationships by
   navigation (Deployment manages Pods, Service routes to Pods)

2. **Faster Workflows**: No need to manually type filters or navigate
   menus

3. **Consistent Mental Model**: "Enter = go deeper" works everywhere

4. **Reduced Cognitive Load**: Less memorization of label selectors

5. **Enhanced Troubleshooting**: Quick path from parent to child
   resources

### Negative

1. **New Screen Required**: Containers screen doesn't exist yet (for
   Pods → Containers)

2. **Navigation Stack**: Need to implement back/forward history (ESC
   already works)

3. **Filter Complexity**: Displaying pre-applied filters clearly
   without clutter

4. **Performance**: Filtering large pod lists by labels (should be fast
   with informer cache)

5. **Ambiguity**: Some resources could drill down to multiple targets
   (design chooses most common)

### Alternatives Considered

**Option 1: Modal Detail View (Rejected)**
- Show resource details in modal instead of navigating
- Rejected: Less useful than seeing related resources

**Option 2: Submenu (Rejected)**
- Enter opens submenu: "View Pods | View YAML | Describe"
- Rejected: Adds extra step, less direct

**Option 3: Shift+Enter for Detail (Alternative)**
- Enter = drill down to related resources
- Shift+Enter = show full YAML detail
- Keep as future enhancement (conflicts with current /yaml command)

## Implementation Notes

1. **Phase 1**: Implement for high-value screens first:
   - Deployments → Pods
   - Services → Pods
   - Nodes → Pods

2. **Phase 2**: Implement Containers screen and Pods → Containers

3. **Phase 3**: Complete remaining screens (Jobs, CronJobs, etc.)

4. **Phase 4**: Add breadcrumb navigation and back button

5. **Testing**: Verify label selector filtering matches kubectl behavior

## References

- k9s drill-down navigation: https://k9scli.io/topics/navigation/
- Kubernetes label selectors:
  https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/
