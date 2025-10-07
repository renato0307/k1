---
date: 2025-10-07T21:54:24+01:00
researcher: Claude
git_commit: 15f51878ed8b4c4fda8a58f596b486ab19d2684d
branch: contextual-navigation-2
repository: k1
topic: "Contextual Navigation with Enter Key"
tags: [research, codebase, navigation, screens, keybindings, resource-relationships, performance, indexed-lookups]
status: complete
last_updated: 2025-10-07
last_updated_by: Claude
last_updated_note: "Added finalized requirements: all screens support navigation, ESC back button, persistent filters, indexed lookups for 10K+ pods"
---

# Research: Contextual Navigation with Enter Key

**Date**: 2025-10-07T21:54:24+01:00
**Researcher**: Claude
**Git Commit**: 15f51878ed8b4c4fda8a58f596b486ab19d2684d
**Branch**: contextual-navigation-2
**Repository**: k1

## Research Question

How to implement contextual navigation where pressing Enter on a resource navigates to related resources:
- Deployment → Pods (owned by that deployment)
- Node → Pods (running on that node)
- Service → Pods (matching service selector)
- Other relationship patterns

## Summary

The k1 codebase uses a config-driven screen architecture with message-based navigation. Currently:

1. **Enter key is handled by Bubble Tea table component** - no custom handling exists
2. **Navigation is stateless** - ScreenSwitchMsg only contains target screenID
3. **No filtered repository queries** - all methods return cluster-wide unfiltered lists
4. **Resource relationships exist but unused** - Pod.Node field is populated but never used for filtering

To implement contextual navigation, we need to:
1. Add filtered query methods to repository layer
2. Extend ScreenSwitchMsg to include filter context
3. Intercept Enter key in ConfigScreen
4. Apply filters in target screens

## Detailed Findings

### Current Enter Key Handling

**Location**: `internal/screens/config.go:165-171`

The ConfigScreen delegates ALL key events directly to the Bubble Tea table component:

```go
case tea.KeyMsg:
    var cmd tea.Cmd
    s.table, cmd = s.table.Update(msg)  // ← Forwards to table
    if s.config.TrackSelection {
        s.updateSelectedKey()
    }
    return s, cmd
```

**Key observations**:
- No custom Enter handling exists at screen level
- Table component handles Enter for row selection/focus only
- Command bar intercepts Enter in palette mode but NOT in hidden/filter mode
- Screens receive Enter key when command bar is in StateHidden or StateFilter

**Message flow for Enter key**:
1. App layer updates selection context: `app.go:178-180`
2. App forwards to command bar: `app.go:184`
3. Command bar processes (returns to hidden if in filter mode): `commandbar.go:221-224`
4. If old state was StateHidden or StateFilter, forward to screen: `app.go:195-198`
5. Screen forwards to table component: `config.go:167`

### Navigation Architecture

**Current pattern** (`internal/commands/navigation.go:25-32`):

```go
func NavigationCommand(screenID string) ExecuteFunc {
    return func(ctx CommandContext) tea.Cmd {
        return func() tea.Msg {
            return types.ScreenSwitchMsg{ScreenID: screenID}
        }
    }
}
```

**ScreenSwitchMsg definition** (`internal/types/types.go:77-79`):

```go
type ScreenSwitchMsg struct {
    ScreenID string
}
```

**Message handling** (`internal/app/app.go:203-220`):

```go
case types.ScreenSwitchMsg:
    if screen, ok := m.registry.Get(msg.ScreenID); ok {
        m.currentScreen = screen
        m.state.CurrentScreen = msg.ScreenID
        m.commandBar.SetScreen(msg.ScreenID)
        m.header.SetScreenTitle(screen.Title())
        // ... size calculations
        return m, screen.Init()  // ← Fresh screen init
    }
```

**Key observations**:
- Navigation is completely stateless - no context passed between screens
- Screen.Init() is called on every switch - screens start fresh
- ScreenSwitchMsg can be extended with additional fields without breaking existing code

### Resource Relationships

**Repository interface** (`internal/k8s/repository.go:71-83`):

Current methods are **unfiltered cluster-wide listings**:

```go
type Repository interface {
    GetResources(resourceType ResourceType) ([]any, error)
    GetPods() ([]Pod, error)
    GetDeployments() ([]Deployment, error)
    GetServices() ([]Service, error)
    // ... etc
}
```

**No filtered methods exist** such as:
- `GetPodsForDeployment(namespace, name)`
- `GetPodsOnNode(nodeName)`
- `GetPodsForService(namespace, name)`

**Implementation patterns from existing code**:

All repository methods use `lister.List(labels.Everything())` with no filtering:

```go
// internal/k8s/informer_repository.go:186-240
func (r *InformerRepository) GetPods() ([]Pod, error) {
    podList, err := r.podLister.List(labels.Everything())  // ← All pods
    // ... transform to Pod structs
}
```

**Resource relationship data**:

The Pod struct contains relationship hints (`internal/k8s/repository.go:91-102`):

```go
type Pod struct {
    Namespace string
    Name      string
    Ready     string
    Status    string
    Restarts  int32
    Age       time.Duration
    CreatedAt time.Time
    Node      string    // ← From spec.nodeName
    IP        string    // ← From status.podIP
}
```

The `Node` field is extracted during transform (`internal/k8s/transforms.go:74`):

```go
node, _, _ := unstructured.NestedString(u.Object, "spec", "nodeName")
```

**However**: This field is never used for filtering - all filtering happens at the UI layer after retrieving complete lists.

### Client-Go Lister Capabilities

The repository uses typed listers from `k8s.io/client-go/listers`:

**PodLister interface**:
```go
type PodLister interface {
    List(selector labels.Selector) ([]*v1.Pod, error)
    Pods(namespace string) PodNamespaceLister
}

type PodNamespaceLister interface {
    List(selector labels.Selector) ([]*v1.Pod, error)
    Get(name string) (*v1.Pod, error)
}
```

**Capabilities**:
- ✅ **Label selector filtering**: `List(labels.Selector)` - can filter by labels
- ✅ **Namespace scoping**: `Pods(namespace).List()` - can scope to namespace
- ❌ **Field selector filtering**: NOT supported - requires manual filtering
- ❌ **Owner reference filtering**: NOT supported - requires manual filtering

### Selection Context

**GetSelectedResource method** (`internal/screens/config.go:330-354`):

```go
func (s *ConfigScreen) GetSelectedResource() map[string]interface{} {
    cursor := s.table.Cursor()
    if cursor < 0 || cursor >= len(s.filtered) {
        return nil
    }

    // Convert to map using reflection
    item := s.filtered[cursor]
    result := make(map[string]interface{})

    v := reflect.ValueOf(item)
    if v.Kind() == reflect.Ptr {
        v = v.Elem()
    }

    t := v.Type()
    for i := 0; i < v.NumField(); i++ {
        fieldName := t.Field(i).Name
        fieldValue := v.Field(i).Interface()
        result[strings.ToLower(fieldName)] = fieldValue
    }

    return result
}
```

**Key aspects**:
- Returns selected resource as `map[string]interface{}`
- Field names are **lowercased** for consistent access
- Pod selection includes `"node"` field (from Pod.Node)
- Deployment selection includes `"name"`, `"namespace"`, etc.

**Selection synchronization** (`internal/app/app.go:178-180`):

```go
// Update command bar with current selection context
if screenWithSel, ok := m.currentScreen.(types.ScreenWithSelection); ok {
    m.commandBar.SetSelectedResource(screenWithSel.GetSelectedResource())
}
```

This happens on **every key press** before forwarding to command bar, ensuring context is always up-to-date.

## Architecture Insights

### Message-Based Communication

The app uses typed messages for all state changes:
- `ScreenSwitchMsg` - Navigation between screens
- `FilterUpdateMsg` / `ClearFilterMsg` - Filter management
- `RefreshCompleteMsg` - Data updates
- `StatusMsg` - User feedback
- `ShowFullScreenMsg` / `ExitFullScreenMsg` - View state

This pattern allows adding new message types without modifying existing handlers.

### Config-Driven Screens

All 11 resource screens use a single `ConfigScreen` implementation (`internal/screens/config.go`):
- No per-screen custom code
- Behavior defined by `ScreenConfig` struct
- Custom behaviors via function overrides (CustomUpdate, CustomRefresh, CustomFilter, CustomView)

**Opportunity**: Can add contextual navigation as a config option or as a new CustomUpdate override.

### Informer-Based Data Access

Repository layer uses Kubernetes informers for efficient data access:
- Data cached in memory via informers
- Sub-second query times (no API calls)
- Listers provide filtered access via label selectors

**Opportunity**: Filtered queries can leverage informer cache without additional API calls.

### Event Fetching Pattern (Relevant Example)

The repository implements on-demand event fetching using **direct API calls** (`internal/k8s/informer_repository.go:482-502`):

```go
func (r *InformerRepository) fetchEventsForResource(namespace, name, uid string) ([]corev1.Event, error) {
    fieldSelector := fmt.Sprintf("involvedObject.name=%s,involvedObject.namespace=%s", name, namespace)
    if uid != "" {
        fieldSelector += fmt.Sprintf(",involvedObject.uid=%s", uid)
    }

    eventList, err := r.clientset.CoreV1().Events(namespace).List(
        r.ctx,
        metav1.ListOptions{
            FieldSelector: fieldSelector,
            Limit:         100,
        },
    )
    // ...
}
```

**Why relevant**: Shows pattern for resource-specific filtering (events are NOT cached, fetched on-demand only).

## Implementation Recommendations

### 1. Extend ScreenSwitchMsg with Filter Context

**Add to** `internal/types/types.go`:

```go
// FilterContext defines filtering to apply on screen switch
type FilterContext struct {
    Field    string  // Field to filter on ("owner", "node", "selector")
    Value    string  // Filter value (deployment name, node name, etc.)
    Metadata map[string]string  // Additional context (namespace, etc.)
}

type ScreenSwitchMsg struct {
    ScreenID      string
    FilterContext *FilterContext  // Optional filter to apply
}
```

**Backward compatible**: Existing navigation commands continue to work (FilterContext is nil).

### 2. Add Filtered Repository Methods

**Add to** `internal/k8s/repository.go` interface:

```go
type Repository interface {
    // Existing methods...

    // Relationship-based filtering
    GetPodsForDeployment(namespace, name string) ([]Pod, error)
    GetPodsOnNode(nodeName string) ([]Pod, error)
    GetPodsForService(namespace, name string) ([]Pod, error)
}
```

**Implementation approaches**:

**A. Pods by owner reference** (Deployment → Pods):
1. Get deployment to find ReplicaSet name
2. List all pods in namespace
3. Manually filter by owner reference matching ReplicaSet

**B. Pods by node name** (Node → Pods):
1. List all pods (from cache)
2. Manually filter where `pod.Spec.NodeName == nodeName`

**C. Pods by label selector** (Service → Pods):
1. Get service to extract `spec.selector`
2. Convert to `labels.Selector` using `labels.SelectorFromSet()`
3. Use lister: `podLister.Pods(namespace).List(selector)`

### 3. Intercept Enter in ConfigScreen

**Modify** `internal/screens/config.go` DefaultUpdate method:

```go
case tea.KeyMsg:
    // Check for Enter key before forwarding to table
    if msg.String() == "enter" {
        if cmd := s.handleEnterKey(); cmd != nil {
            return s, cmd
        }
    }

    // Existing logic: forward to table
    var cmd tea.Cmd
    s.table, cmd = s.table.Update(msg)
    if s.config.TrackSelection {
        s.updateSelectedKey()
    }
    return s, cmd
```

**Add new method**:

```go
// handleEnterKey checks for contextual navigation based on selected resource
func (s *ConfigScreen) handleEnterKey() tea.Cmd {
    selected := s.GetSelectedResource()
    if selected == nil {
        return nil
    }

    // Determine navigation based on current screen
    switch s.config.ID {
    case "deployments":
        return s.navigateToPodsForDeployment(selected)
    case "nodes":
        return s.navigateToPodsOnNode(selected)
    case "services":
        return s.navigateToPodsForService(selected)
    // Add more relationships as needed
    }

    return nil  // No contextual navigation for this screen
}
```

**Helper methods**:

```go
func (s *ConfigScreen) navigateToPodsForDeployment(selected map[string]interface{}) tea.Cmd {
    name, _ := selected["name"].(string)
    namespace, _ := selected["namespace"].(string)

    return func() tea.Msg {
        return types.ScreenSwitchMsg{
            ScreenID: "pods",
            FilterContext: &types.FilterContext{
                Field: "owner",
                Value: name,
                Metadata: map[string]string{
                    "namespace": namespace,
                    "kind":      "Deployment",
                },
            },
        }
    }
}

func (s *ConfigScreen) navigateToPodsOnNode(selected map[string]interface{}) tea.Cmd {
    nodeName, _ := selected["name"].(string)

    return func() tea.Msg {
        return types.ScreenSwitchMsg{
            ScreenID: "pods",
            FilterContext: &types.FilterContext{
                Field: "node",
                Value: nodeName,
            },
        }
    }
}

func (s *ConfigScreen) navigateToPodsForService(selected map[string]interface{}) tea.Cmd {
    name, _ := selected["name"].(string)
    namespace, _ := selected["namespace"].(string)

    return func() tea.Msg {
        return types.ScreenSwitchMsg{
            ScreenID: "pods",
            FilterContext: &types.FilterContext{
                Field: "selector",
                Value: name,
                Metadata: map[string]string{
                    "namespace": namespace,
                },
            },
        }
    }
}
```

### 4. Apply Filter in Target Screen

**Modify** `internal/app/app.go` ScreenSwitchMsg handler:

```go
case types.ScreenSwitchMsg:
    if screen, ok := m.registry.Get(msg.ScreenID); ok {
        m.currentScreen = screen
        m.state.CurrentScreen = msg.ScreenID
        m.commandBar.SetScreen(msg.ScreenID)
        m.header.SetScreenTitle(screen.Title())

        // Apply filter context if provided
        if msg.FilterContext != nil {
            if screenWithFilter, ok := screen.(interface{ ApplyFilterContext(*types.FilterContext) }); ok {
                screenWithFilter.ApplyFilterContext(msg.FilterContext)
            }
        }

        bodyHeight := m.layout.CalculateBodyHeightWithCommandBar(m.commandBar.GetTotalHeight())
        if screenWithSize, ok := m.currentScreen.(interface{ SetSize(int, int) }); ok {
            screenWithSize.SetSize(m.state.Width, bodyHeight)
        }

        return m, screen.Init()
    }
```

**Add method to ConfigScreen**:

```go
// ApplyFilterContext applies filtering based on context from navigation
func (s *ConfigScreen) ApplyFilterContext(ctx *types.FilterContext) {
    if ctx == nil {
        return
    }

    // Store filter context for use during Refresh()
    s.filterContext = ctx
}
```

**Modify ConfigScreen struct**:

```go
type ConfigScreen struct {
    config        ScreenConfig
    repo          k8s.Repository
    table         table.Model
    items         []interface{}
    filtered      []interface{}
    filter        string
    filterContext *types.FilterContext  // ← Add this
    theme         *ui.Theme
    width         int
    height        int
    selectedKey   string
}
```

**Modify Refresh() method**:

```go
func (s *ConfigScreen) Refresh() tea.Cmd {
    return func() tea.Msg {
        start := time.Now()

        var items []interface{}
        var err error

        // Check for filter context (relationship-based filtering)
        if s.filterContext != nil {
            items, err = s.refreshWithFilterContext()
        } else if s.config.CustomRefresh != nil {
            items, err = s.config.CustomRefresh(s.repo)
        } else {
            items, err = s.repo.GetResources(s.config.ResourceType)
        }

        // ... existing logic
    }
}

func (s *ConfigScreen) refreshWithFilterContext() ([]interface{}, error) {
    ctx := s.filterContext

    switch ctx.Field {
    case "owner":
        namespace := ctx.Metadata["namespace"]
        deploymentName := ctx.Value
        return s.repo.GetPodsForDeployment(namespace, deploymentName)

    case "node":
        nodeName := ctx.Value
        return s.repo.GetPodsOnNode(nodeName)

    case "selector":
        namespace := ctx.Metadata["namespace"]
        serviceName := ctx.Value
        return s.repo.GetPodsForService(namespace, serviceName)
    }

    // Fallback to unfiltered
    return s.repo.GetResources(s.config.ResourceType)
}
```

### 5. Visual Feedback for Filtered View

**Add filter indicator to header** when FilterContext is active:

```go
// In ConfigScreen.View()
if s.filterContext != nil {
    filterText := s.getFilterContextDescription()
    header = lipgloss.JoinHorizontal(lipgloss.Left, header, filterText)
}

func (s *ConfigScreen) getFilterContextDescription() string {
    ctx := s.filterContext

    switch ctx.Field {
    case "owner":
        return fmt.Sprintf(" (filtered by deployment: %s/%s)",
            ctx.Metadata["namespace"], ctx.Value)
    case "node":
        return fmt.Sprintf(" (filtered by node: %s)", ctx.Value)
    case "selector":
        return fmt.Sprintf(" (filtered by service: %s/%s)",
            ctx.Metadata["namespace"], ctx.Value)
    }

    return ""
}
```

**Add keybinding to clear filter context** (e.g., ESC or backspace):

```go
case tea.KeyMsg:
    if msg.String() == "esc" && s.filterContext != nil {
        // Return to unfiltered pods view
        return s, func() tea.Msg {
            return types.ScreenSwitchMsg{
                ScreenID: "pods",
                FilterContext: nil,
            }
        }
    }

    // ... existing Enter handling
```

## Testing Strategy

### Unit Tests for Filtered Repository Methods

**Create** `internal/k8s/informer_repository_relationships_test.go`:

```go
func TestInformerRepository_GetPodsForDeployment(t *testing.T) {
    repo := setupTestRepository(t)

    // Create deployment and pods with owner references
    // ...

    pods, err := repo.GetPodsForDeployment("default", "nginx")
    assert.NoError(t, err)
    assert.Len(t, pods, 3)  // Expected pod count
}

func TestInformerRepository_GetPodsOnNode(t *testing.T) {
    repo := setupTestRepository(t)

    // Create pods on different nodes
    // ...

    pods, err := repo.GetPodsOnNode("node-1")
    assert.NoError(t, err)
    assert.Len(t, pods, 2)  // Pods on node-1
}
```

### Integration Tests for Contextual Navigation

**Add to** `internal/screens/config_test.go`:

```go
func TestConfigScreen_ContextualNavigation(t *testing.T) {
    tests := []struct {
        name          string
        screenID      string
        selectedItem  map[string]interface{}
        expectedMsg   types.ScreenSwitchMsg
    }{
        {
            name:     "deployment to pods",
            screenID: "deployments",
            selectedItem: map[string]interface{}{
                "name":      "nginx",
                "namespace": "default",
            },
            expectedMsg: types.ScreenSwitchMsg{
                ScreenID: "pods",
                FilterContext: &types.FilterContext{
                    Field: "owner",
                    Value: "nginx",
                    Metadata: map[string]string{
                        "namespace": "default",
                        "kind":      "Deployment",
                    },
                },
            },
        },
        // ... more test cases
    }

    // ... test implementation
}
```

## Alternative Approaches

### Approach A: Screen-Level Filtering (Chosen Above)

**Pros**:
- Leverages informer cache (fast)
- Repository methods are reusable
- Clean separation of concerns

**Cons**:
- Requires repository changes
- More complex initial implementation

### Approach B: UI-Layer Filtering Only

Apply filters after getting complete pod list in ConfigScreen:

**Pros**:
- No repository changes needed
- Simpler initial implementation

**Cons**:
- Less efficient (filters entire cluster's pods)
- Filtering logic in UI layer
- Not reusable for API/CLI

**Verdict**: Approach A is better for scalability and reusability.

### Approach C: New Message Type (ContextualNavigateMsg)

Create specialized message instead of extending ScreenSwitchMsg:

**Pros**:
- Clearer intent
- Separate from normal navigation

**Cons**:
- More message types to maintain
- Duplicates navigation logic

**Verdict**: Extending ScreenSwitchMsg is simpler and sufficient.

## Code References

### Key Files

- `internal/types/types.go:77-79` - ScreenSwitchMsg definition
- `internal/screens/config.go:165-171` - Enter key handling
- `internal/screens/config.go:330-354` - GetSelectedResource method
- `internal/k8s/repository.go:71-83` - Repository interface
- `internal/k8s/informer_repository.go:186-240` - GetPods implementation
- `internal/k8s/transforms.go:74` - Node field extraction
- `internal/app/app.go:203-220` - ScreenSwitchMsg handler
- `internal/app/app.go:178-180` - Selection context sync
- `internal/commands/navigation.go:25-32` - NavigationCommand pattern

### Related Components

- `internal/components/commandbar/commandbar.go:221-224` - Enter in filter mode
- `internal/components/commandbar/commandbar.go:293-294` - Enter in palette mode
- `internal/app/app.go:195-198` - Message routing to screens
- `k8s.io/client-go/listers/core/v1` - PodLister interface
- `k8s.io/apimachinery/pkg/labels` - Label selector utilities

## Finalized Requirements

The following decisions have been made based on user requirements:

1. **All screens support contextual navigation** ✅
   - Implement relationships for all 11 resource screens
   - See "Comprehensive Relationship Mappings" section below

2. **ESC for back navigation** ✅
   - Track navigation history in app state (stack-based)
   - ESC returns to previous screen with previous filter context
   - Navigation history cleared on explicit screen switch (`:pods`, etc.)

3. **Filter context persists across screen switches** ✅
   - Filter stays active when switching screens
   - Only cleared by ESC (back navigation) or explicit clear action
   - Enables exploring different views while maintaining context

4. **Empty filtered results handling** ✅
   - Show empty list with helpful message
   - Display filter context prominently in header for clarity
   - Provide visual indication of active filter

5. **Performance critical: Indexed lookups required** ✅
   - Expect clusters with 10K+ pods
   - Manual O(n) filtering is too slow - need O(1) indexed lookups
   - Add index structures to InformerRepository (see Performance Optimization section)
   - Target: Sub-second response for all filtered queries

## Comprehensive Relationship Mappings

All 11 resource screens will support contextual navigation via Enter key:

### Tier 1: Pod Owners (Owner Reference Filtering)
- **Deployments → Pods**: Filter pods by owner reference (via ReplicaSet)
- **StatefulSets → Pods**: Filter pods by owner reference (direct)
- **DaemonSets → Pods**: Filter pods by owner reference (direct)
- **Jobs → Pods**: Filter pods by owner reference (direct)

### Tier 2: Infrastructure
- **Nodes → Pods**: Filter pods by `spec.nodeName` field
- **Namespaces → Pods**: Filter pods by namespace

### Tier 3: Service Discovery
- **Services → Pods**: Filter pods by label selector matching `spec.selector`

### Tier 4: Configuration (Volume References)
- **ConfigMaps → Pods**: Filter pods that reference this ConfigMap in volumes
- **Secrets → Pods**: Filter pods that reference this Secret in volumes

### Tier 5: Scheduled Jobs (Multi-Level)
- **CronJobs → Jobs**: Filter jobs by owner reference
- **Jobs → Pods**: Already in Tier 1 (can chain: CronJob → Job → Pods)

### Filter Context Patterns

**Pattern 1: Owner Reference** (Deployments, StatefulSets, DaemonSets, Jobs, CronJobs)
```go
FilterContext{
    Field: "owner",
    Value: "<resource-name>",
    Metadata: map[string]string{
        "namespace": "<namespace>",
        "kind":      "Deployment|StatefulSet|DaemonSet|Job|CronJob",
    },
}
```

**Pattern 2: Field Match** (Nodes, Namespaces)
```go
// Nodes → Pods (by spec.nodeName)
FilterContext{
    Field: "node",
    Value: "<node-name>",
}

// Namespaces → Pods (by metadata.namespace)
FilterContext{
    Field: "namespace",
    Value: "<namespace-name>",
}
```

**Pattern 3: Label Selector** (Services)
```go
FilterContext{
    Field: "selector",
    Value: "<service-name>",
    Metadata: map[string]string{
        "namespace": "<namespace>",
    },
}
```

**Pattern 4: Volume Reference** (ConfigMaps, Secrets)
```go
FilterContext{
    Field: "volumeRef",
    Value: "<configmap-or-secret-name>",
    Metadata: map[string]string{
        "namespace": "<namespace>",
        "kind":      "ConfigMap|Secret",
    },
}
```

## Performance Optimization: Indexed Lookups

For clusters with 10K+ pods, manual filtering is too slow. Add index structures to InformerRepository:

### Index Structure Design

**Add to** `internal/k8s/informer_repository.go`:

```go
type InformerRepository struct {
    // Existing fields...
    clientset        *kubernetes.Clientset
    factory          informers.SharedInformerFactory
    podLister        v1listers.PodLister
    // ... other listers

    // Performance indexes (built on informer updates)
    mu               sync.RWMutex
    podsByNode       map[string][]*corev1.Pod     // nodeName → pods
    podsByNamespace  map[string][]*corev1.Pod     // namespace → pods
    podsByOwnerUID   map[string][]*corev1.Pod     // ownerUID → pods
    podsByConfigMap  map[string][]*corev1.Pod     // namespace/name → pods
    podsBySecret     map[string][]*corev1.Pod     // namespace/name → pods

    // Label selector hash → pods (for service matching)
    podsByLabelHash  map[string][]*corev1.Pod     // hash(labels) → pods
}
```

### Index Building Strategy

**Approach**: Use informer event handlers to maintain indexes:

```go
func (r *InformerRepository) setupPodIndexes() {
    // Add event handlers to pod informer
    podInformer := r.factory.Core().V1().Pods().Informer()

    podInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
        AddFunc: func(obj interface{}) {
            pod := obj.(*corev1.Pod)
            r.updatePodIndexes(pod, nil)
        },
        UpdateFunc: func(oldObj, newObj interface{}) {
            oldPod := oldObj.(*corev1.Pod)
            newPod := newObj.(*corev1.Pod)
            r.updatePodIndexes(newPod, oldPod)
        },
        DeleteFunc: func(obj interface{}) {
            pod := obj.(*corev1.Pod)
            r.removePodFromIndexes(pod)
        },
    })
}

func (r *InformerRepository) updatePodIndexes(newPod, oldPod *corev1.Pod) {
    r.mu.Lock()
    defer r.mu.Unlock()

    // Remove old pod from indexes if updating
    if oldPod != nil {
        r.removePodFromIndexesLocked(oldPod)
    }

    // Add to node index
    if newPod.Spec.NodeName != "" {
        r.podsByNode[newPod.Spec.NodeName] = append(
            r.podsByNode[newPod.Spec.NodeName],
            newPod,
        )
    }

    // Add to namespace index
    r.podsByNamespace[newPod.Namespace] = append(
        r.podsByNamespace[newPod.Namespace],
        newPod,
    )

    // Add to owner index
    for _, ownerRef := range newPod.OwnerReferences {
        r.podsByOwnerUID[string(ownerRef.UID)] = append(
            r.podsByOwnerUID[string(ownerRef.UID)],
            newPod,
        )
    }

    // Add to ConfigMap/Secret indexes
    r.indexPodVolumes(newPod)

    // Add to label hash index
    labelHash := computeLabelHash(newPod.Labels)
    r.podsByLabelHash[labelHash] = append(
        r.podsByLabelHash[labelHash],
        newPod,
    )
}
```

### Indexed Query Methods

**Add to** `internal/k8s/informer_repository.go`:

```go
// GetPodsOnNode returns pods running on a specific node (O(1) lookup)
func (r *InformerRepository) GetPodsOnNode(nodeName string) ([]Pod, error) {
    r.mu.RLock()
    pods := r.podsByNode[nodeName]
    r.mu.RUnlock()

    return r.transformPods(pods)
}

// GetPodsInNamespace returns all pods in a namespace (O(1) lookup)
func (r *InformerRepository) GetPodsInNamespace(namespace string) ([]Pod, error) {
    r.mu.RLock()
    pods := r.podsByNamespace[namespace]
    r.mu.RUnlock()

    return r.transformPods(pods)
}

// GetPodsForOwner returns pods owned by a resource (O(1) lookup)
func (r *InformerRepository) GetPodsForOwner(ownerUID string) ([]Pod, error) {
    r.mu.RLock()
    pods := r.podsByOwnerUID[ownerUID]
    r.mu.RUnlock()

    return r.transformPods(pods)
}

// GetPodsForDeployment returns pods owned by deployment (2 lookups: deployment→RS, RS→pods)
func (r *InformerRepository) GetPodsForDeployment(namespace, name string) ([]Pod, error) {
    // Get deployment to find ReplicaSets
    deployment, err := r.deploymentLister.Deployments(namespace).Get(name)
    if err != nil {
        return nil, fmt.Errorf("deployment not found: %w", err)
    }

    // Find owned ReplicaSets
    replicaSets, err := r.factory.Apps().V1().ReplicaSets().Lister().
        ReplicaSets(namespace).List(labels.Everything())
    if err != nil {
        return nil, err
    }

    var allPods []*corev1.Pod
    for _, rs := range replicaSets {
        // Check if ReplicaSet is owned by this deployment
        if isOwnedBy(rs, deployment.UID) {
            // Use indexed lookup for pods
            r.mu.RLock()
            pods := r.podsByOwnerUID[string(rs.UID)]
            r.mu.RUnlock()
            allPods = append(allPods, pods...)
        }
    }

    return r.transformPods(allPods)
}
```

### Memory Overhead Estimation

For a cluster with 10,000 pods:
- **podsByNode**: ~100 nodes × 100 pods/node = ~10K pointers = ~80KB
- **podsByNamespace**: ~50 namespaces × 200 pods/ns = ~10K pointers = ~80KB
- **podsByOwnerUID**: ~500 owners × 20 pods/owner = ~10K pointers = ~80KB
- **podsByConfigMap/Secret**: ~1000 configs × 10 pods = ~10K pointers = ~80KB
- **podsByLabelHash**: ~2000 unique label sets × 5 pods = ~10K pointers = ~80KB

**Total memory overhead**: ~400KB for indexes (negligible compared to pod data itself)

**Performance gain**: O(n) → O(1) for filtered queries, ~1000x faster for large clusters

## Navigation History Implementation

**Add to** `internal/app/app.go`:

```go
type NavigationState struct {
    ScreenID      string
    FilterContext *types.FilterContext
}

type Model struct {
    // Existing fields...
    currentScreen     types.Screen
    commandBar        *commandbar.CommandBar

    // Navigation history stack
    navigationHistory []NavigationState
    maxHistorySize    int  // Default: 50
}

// Push current state before navigating
func (m *Model) pushNavigationHistory() {
    if m.currentScreen == nil {
        return
    }

    // Get current filter context from screen
    var filterCtx *types.FilterContext
    if screenWithFilter, ok := m.currentScreen.(interface{ GetFilterContext() *types.FilterContext }); ok {
        filterCtx = screenWithFilter.GetFilterContext()
    }

    state := NavigationState{
        ScreenID:      m.state.CurrentScreen,
        FilterContext: filterCtx,
    }

    m.navigationHistory = append(m.navigationHistory, state)

    // Limit history size
    if len(m.navigationHistory) > m.maxHistorySize {
        m.navigationHistory = m.navigationHistory[1:]
    }
}

// Pop and restore previous state
func (m *Model) popNavigationHistory() tea.Cmd {
    if len(m.navigationHistory) == 0 {
        return nil
    }

    // Pop last state
    lastIdx := len(m.navigationHistory) - 1
    prevState := m.navigationHistory[lastIdx]
    m.navigationHistory = m.navigationHistory[:lastIdx]

    // Navigate to previous screen with context
    return func() tea.Msg {
        return types.ScreenSwitchMsg{
            ScreenID:      prevState.ScreenID,
            FilterContext: prevState.FilterContext,
            IsBackNav:     true,  // Don't push to history again
        }
    }
}
```

**Modify ScreenSwitchMsg**:

```go
type ScreenSwitchMsg struct {
    ScreenID      string
    FilterContext *FilterContext
    IsBackNav     bool  // True when navigating back (don't push history)
}
```

**Update ScreenSwitchMsg handler**:

```go
case types.ScreenSwitchMsg:
    // Push current state to history (unless back navigation)
    if !msg.IsBackNav && msg.FilterContext != nil {
        m.pushNavigationHistory()
    }

    // Existing navigation logic...
```

**Handle ESC in app layer**:

```go
case tea.KeyMsg:
    // Check for ESC with navigation history
    if msg.String() == "esc" && len(m.navigationHistory) > 0 {
        return m, m.popNavigationHistory()
    }

    // Existing key handling...
```

## Next Steps

1. **Phase 1: Repository Layer**
   - Add filtered query methods to Repository interface
   - Implement GetPodsForDeployment, GetPodsOnNode, GetPodsForService
   - Write unit tests with envtest

2. **Phase 2: Message Extension**
   - Add FilterContext to types.go
   - Extend ScreenSwitchMsg

3. **Phase 3: Screen Enter Handling**
   - Intercept Enter in ConfigScreen
   - Implement relationship-specific navigation methods
   - Add tests for navigation logic

4. **Phase 4: Filter Application**
   - Modify ScreenSwitchMsg handler in app.go
   - Add ApplyFilterContext to ConfigScreen
   - Update Refresh() to use filtered queries

5. **Phase 5: User Feedback**
   - Add filter context display in header
   - Add ESC to clear filter
   - Test with real cluster data

## Success Criteria

### Functional Requirements
- ✅ All 11 screens support contextual navigation via Enter key
- ✅ Deployments/StatefulSets/DaemonSets/Jobs → Pods (owner reference)
- ✅ Nodes → Pods (by node name)
- ✅ Services → Pods (by label selector)
- ✅ Namespaces → Pods (by namespace filter)
- ✅ ConfigMaps/Secrets → Pods (by volume reference)
- ✅ CronJobs → Jobs (owner reference)
- ✅ Filtered view shows clear indication of active filter in header
- ✅ Empty filtered results show helpful message

### Navigation & UX
- ✅ ESC performs back navigation (returns to previous screen with previous filter)
- ✅ Navigation history maintained (stack-based, max 50 entries)
- ✅ Filter context persists when switching screens (until ESC or explicit clear)
- ✅ Explicit navigation (`:pods`, `:deployments`) clears history stack
- ✅ No breaking changes to existing navigation patterns

### Performance Requirements (Critical)
- ✅ Sub-second response for all filtered queries on 10K+ pod clusters
- ✅ Indexed lookups implemented (O(1) instead of O(n))
- ✅ Memory overhead acceptable (<1MB for 10K pods)
- ✅ Index maintenance via informer event handlers (no manual refresh)

### Testing & Quality
- ✅ All tests pass with >70% coverage
- ✅ Unit tests for indexed repository methods
- ✅ Integration tests for contextual navigation flows
- ✅ Performance tests with simulated large clusters
