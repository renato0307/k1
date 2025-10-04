# Scalable Multi-Resource Architecture with Config-Driven Design

| Metadata | Value                                                |
|----------|------------------------------------------------------|
| Date     | 2025-10-04                                           |
| Author   | @renato0307                                          |
| Status   | Proposed                                             |
| Tags     | architecture, repository, screens, DRY, generics     |

| Revision | Date       | Author      | Info           |
|----------|------------|-------------|----------------|
| 1        | 2025-10-04 | @renato0307 | Initial design |

## Context and Problem Statement

Timoneiro currently supports 3 resources (Pods, Deployments, Services)
with 812 lines of screen code and 292 lines of repository code. Each
new resource requires ~270 lines of screen code and ~80 lines of
repository code, leading to massive duplication.

**Scaling problem:**
- Supporting 10 resources: ~2700 screen lines + ~800 repository lines
- Supporting 17 resources: ~4500 screen lines + ~1400 repository lines
- Kubernetes has 80+ built-in resources plus unlimited CRDs

**Current duplication:**
- Repository: Each resource needs informer setup, lister field, sync
  check, Get method
- Screens: Each resource needs identical Init/Update/View/SetFilter
  logic, fuzzy search, refresh pattern

**Key Questions:**
- How do we support 10-20 resources without massive code duplication?
- How do we keep type safety while reducing boilerplate?
- How do we allow customization when needed?
- Can we support CRDs (Custom Resource Definitions) automatically?

## Top 10-17 Most Common Resources

Based on kubectl api-resources and common usage patterns:

**Tier 1 (Critical - Block UI startup):**
1. Pods - Most viewed resource, primary debugging target

**Tier 2 (Very Common - Background load):**
2. Deployments - Most common workload controller
3. Services - Networking/discovery
4. Namespaces - Organization (cluster-scoped)
5. ConfigMaps - Configuration
6. Secrets - Sensitive data

**Tier 3 (Common - Background load):**
7. StatefulSets - Stateful workloads
8. DaemonSets - Node-level workloads
9. Jobs - Batch workloads
10. CronJobs - Scheduled workloads

**Tier 4 (Optional - On-demand or later):**
11. ReplicaSets - Usually viewed through Deployments
12. Ingresses - External access
13. PersistentVolumeClaims (PVCs) - Storage
14. Nodes - Cluster infrastructure (cluster-scoped)
15. ServiceAccounts - RBAC
16. HorizontalPodAutoscalers (HPAs) - Scaling
17. Events - Debugging (high volume, special handling)

**Future:**
- Custom Resource Definitions (CRDs) - Dynamic discovery
- Additional core resources (NetworkPolicies, PVs, etc.)

## Design Approach: Config-Driven with Escape Hatches

### Three-Level Customization Model

**Level 1: Pure Config (90% of resources)**
Define screens entirely in configuration. No code needed.

**Level 2: Config + Custom Methods (8% of resources)**
Use config for structure, override specific methods for custom behavior.

**Level 3: Fully Custom Screen (2% of resources)**
Implement Screen interface directly for unique UIs.

This provides:
- Zero-code path for standard list views
- Progressive customization as complexity grows
- Full escape hatch when config isn't enough

## Design: Repository Layer

### Dynamic Informer-Based Repository

**Replace typed informers with dynamic client:**

```go
import (
    "k8s.io/client-go/dynamic"
    "k8s.io/client-go/dynamic/dynamicinformer"
    "k8s.io/apimachinery/pkg/runtime/schema"
)

// Resource configuration
type ResourceConfig struct {
    GVR          schema.GroupVersionResource
    Name         string
    Namespaced   bool
    Tier         int // 1=critical, 2=background, 3=deferred
    Transform    TransformFunc
}

type TransformFunc func(*unstructured.Unstructured) (interface{}, error)

// InformerRepository with dynamic client
type InformerRepository struct {
    clientset      *kubernetes.Clientset // Keep for typed operations
    dynamicClient  dynamic.Interface
    dynamicFactory dynamicinformer.DynamicSharedInformerFactory

    // Resource registry
    resources map[ResourceType]ResourceConfig
    informers map[schema.GroupVersionResource]informers.GenericInformer

    // Loading status
    loadingStatus map[ResourceType]*LoadingStatus
    statusMu      sync.RWMutex

    ctx    context.Context
    cancel context.CancelFunc
}

// Resource type enum (extensible)
type ResourceType string

const (
    ResourceTypePod                ResourceType = "pods"
    ResourceTypeDeployment         ResourceType = "deployments"
    ResourceTypeService            ResourceType = "services"
    ResourceTypeNamespace          ResourceType = "namespaces"
    ResourceTypeConfigMap          ResourceType = "configmaps"
    ResourceTypeSecret             ResourceType = "secrets"
    ResourceTypeStatefulSet        ResourceType = "statefulsets"
    ResourceTypeDaemonSet          ResourceType = "daemonsets"
    ResourceTypeJob                ResourceType = "jobs"
    ResourceTypeCronJob            ResourceType = "cronjobs"
    ResourceTypeReplicaSet         ResourceType = "replicasets"
    ResourceTypeIngress            ResourceType = "ingresses"
    ResourceTypePVC                ResourceType = "pvcs"
    ResourceTypeNode               ResourceType = "nodes"
    ResourceTypeServiceAccount     ResourceType = "serviceaccounts"
    ResourceTypeHPA                ResourceType = "hpas"
    ResourceTypeEvent              ResourceType = "events"
)

// Resource registry (defines all supported resources)
var resourceRegistry = map[ResourceType]ResourceConfig{
    ResourceTypePod: {
        GVR:        schema.GroupVersionResource{Group: "", Version: "v1",
                    Resource: "pods"},
        Name:       "Pods",
        Namespaced: true,
        Tier:       1, // Critical - block UI
        Transform:  transformPod,
    },
    ResourceTypeDeployment: {
        GVR:        schema.GroupVersionResource{Group: "apps",
                    Version: "v1", Resource: "deployments"},
        Name:       "Deployments",
        Namespaced: true,
        Tier:       2, // Background
        Transform:  transformDeployment,
    },
    ResourceTypeService: {
        GVR:        schema.GroupVersionResource{Group: "", Version: "v1",
                    Resource: "services"},
        Name:       "Services",
        Namespaced: true,
        Tier:       2,
        Transform:  transformService,
    },
    // ... register other resources
}
```

### Repository Interface

```go
type Repository interface {
    // Generic resource access (dynamic)
    GetResources(resourceType ResourceType) ([]interface{}, error)
    GetResource(resourceType ResourceType, namespace, name string)
        (interface{}, error)

    // Typed convenience wrappers (optional, for common resources)
    GetPods() ([]Pod, error)
    GetDeployments() ([]Deployment, error)
    GetServices() ([]Service, error)

    // YAML and Describe (DDR-06)
    GetResourceYAML(resourceType ResourceType, namespace, name string)
        (string, error)
    DescribeResource(resourceType ResourceType, namespace, name string)
        (string, error)

    // Loading status
    GetLoadingStatus(resourceType ResourceType) LoadingStatus
    IsResourceSynced(resourceType ResourceType) bool

    // Lifecycle
    Close()
}
```

### Initialization with Tiered Loading

```go
func NewInformerRepository(kubeconfig, contextName string)
    (*InformerRepository, error) {

    // Build config and clients
    config := buildKubeConfig(kubeconfig, contextName)
    config.ContentType = "application/vnd.kubernetes.protobuf"

    clientset := kubernetes.NewForConfig(config)
    dynamicClient := dynamic.NewForConfig(config)
    dynamicFactory := dynamicinformer.NewDynamicSharedInformerFactory(
        dynamicClient, 30*time.Second)

    repo := &InformerRepository{
        clientset:      clientset,
        dynamicClient:  dynamicClient,
        dynamicFactory: dynamicFactory,
        resources:      resourceRegistry,
        informers:      make(map[schema.GroupVersionResource]
                            informers.GenericInformer),
        loadingStatus:  make(map[ResourceType]*LoadingStatus),
    }

    return repo, nil
}

// StartPriority blocks until Tier 1 resources are synced
func (r *InformerRepository) StartPriority(ctx context.Context) error {
    tier1Resources := r.getResourcesByTier(1)

    // Create informers for Tier 1
    informersToSync := []cache.SharedIndexInformer{}
    for _, res := range tier1Resources {
        informer := r.dynamicFactory.ForResource(res.GVR).Informer()
        r.informers[res.GVR] = informer
        informersToSync = append(informersToSync, informer)
    }

    // Start factory
    r.dynamicFactory.Start(ctx.Done())

    // Wait for Tier 1 sync
    if !cache.WaitForCacheSync(ctx.Done(), toHasSyncedSlice(
        informersToSync)...) {
        return fmt.Errorf("failed to sync Tier 1 caches")
    }

    // Mark Tier 1 as synced
    for _, res := range tier1Resources {
        r.updateLoadingStatus(res.Name, true, nil)
    }

    return nil
}

// StartBackground starts Tier 2+ resources in parallel (non-blocking)
func (r *InformerRepository) StartBackground(ctx context.Context) {
    tier2PlusResources := r.getResourcesByTier(2, 3)

    var wg sync.WaitGroup
    for _, res := range tier2PlusResources {
        wg.Add(1)
        go func(resource ResourceConfig) {
            defer wg.Done()

            // Create informer
            informer := r.dynamicFactory.ForResource(resource.GVR).
                Informer()
            r.informers[resource.GVR] = informer

            // Start factory (idempotent)
            r.dynamicFactory.Start(ctx.Done())

            // Wait for sync
            if cache.WaitForCacheSync(ctx.Done(), informer.HasSynced) {
                r.updateLoadingStatus(resource.Name, true, nil)
            } else {
                r.updateLoadingStatus(resource.Name, false,
                    fmt.Errorf("cache sync timeout"))
            }
        }(res)
    }

    // Wait in background (doesn't block UI)
    wg.Wait()
}
```

### Generic GetResources Implementation

```go
func (r *InformerRepository) GetResources(resourceType ResourceType)
    ([]interface{}, error) {

    // Get resource config
    config, ok := r.resources[resourceType]
    if !ok {
        return nil, fmt.Errorf("unknown resource type: %s",
            resourceType)
    }

    // Get informer
    informer, ok := r.informers[config.GVR]
    if !ok {
        return nil, fmt.Errorf("informer not initialized for %s",
            resourceType)
    }

    // List from cache
    objList, err := informer.Lister().List(labels.Everything())
    if err != nil {
        return nil, fmt.Errorf("failed to list %s: %w",
            resourceType, err)
    }

    // Transform unstructured objects to typed objects
    results := make([]interface{}, 0, len(objList))
    for _, obj := range objList {
        unstr := obj.(*unstructured.Unstructured)
        transformed, err := config.Transform(unstr)
        if err != nil {
            // Log error but continue (partial results better than
            // nothing)
            continue
        }
        results = append(results, transformed)
    }

    return results, nil
}
```

### Transform Functions

```go
// Transform unstructured to typed Pod
func transformPod(u *unstructured.Unstructured) (interface{}, error) {
    namespace := u.GetNamespace()
    name := u.GetName()

    // Extract fields using unstructured helpers
    readyContainers, totalContainers := getContainerStatus(u)
    restarts := getRestartCount(u)
    status, _ := unstructured.NestedString(u.Object, "status", "phase")
    node, _ := unstructured.NestedString(u.Object, "spec", "nodeName")
    ip, _ := unstructured.NestedString(u.Object, "status", "podIP")

    age := time.Since(u.GetCreationTimestamp().Time)

    return k8s.Pod{
        Namespace: namespace,
        Name:      name,
        Ready:     fmt.Sprintf("%d/%d", readyContainers,
                      totalContainers),
        Status:    status,
        Restarts:  restarts,
        Age:       age,
        Node:      node,
        IP:        ip,
    }, nil
}

// Transform unstructured to typed Deployment
func transformDeployment(u *unstructured.Unstructured)
    (interface{}, error) {

    namespace := u.GetNamespace()
    name := u.GetName()

    // Extract replica counts
    ready, _, _ := unstructured.NestedInt64(u.Object, "status",
        "readyReplicas")
    desired, _, _ := unstructured.NestedInt64(u.Object, "spec",
        "replicas")
    upToDate, _, _ := unstructured.NestedInt64(u.Object, "status",
        "updatedReplicas")
    available, _, _ := unstructured.NestedInt64(u.Object, "status",
        "availableReplicas")

    age := time.Since(u.GetCreationTimestamp().Time)

    return k8s.Deployment{
        Namespace: namespace,
        Name:      name,
        Ready:     fmt.Sprintf("%d/%d", ready, desired),
        UpToDate:  int32(upToDate),
        Available: int32(available),
        Age:       age,
    }, nil
}

// Similar transforms for other resources...
```

### Typed Convenience Wrappers

```go
// Convenience methods for common resources (optional)
func (r *InformerRepository) GetPods() ([]Pod, error) {
    items, err := r.GetResources(ResourceTypePod)
    if err != nil {
        return nil, err
    }

    pods := make([]Pod, len(items))
    for i, item := range items {
        pods[i] = item.(Pod)
    }

    // Sort by age (newest first), then by name
    sort.Slice(pods, func(i, j int) bool {
        if pods[i].Age != pods[j].Age {
            return pods[i].Age < pods[j].Age
        }
        return pods[i].Name < pods[j].Name
    })

    return pods, nil
}
```

## Design: Screen Layer

### Three-Level Screen Architecture

#### Level 1: Pure Config Screen

**Zero code needed, just configuration:**

```go
// ScreenConfig defines a resource list screen
type ScreenConfig struct {
    ID              string
    Title           string
    ResourceType    k8s.ResourceType
    Columns         []ColumnConfig
    SearchFields    []string
    Operations      []OperationConfig

    // Optional behavior flags
    EnablePeriodicRefresh bool
    RefreshInterval       time.Duration
    TrackSelection        bool

    // Optional custom overrides (Level 2)
    CustomRefresh   func(*ConfigScreen) tea.Cmd
    CustomFilter    func(*ConfigScreen, string)
    CustomUpdate    func(*ConfigScreen, tea.Msg) (tea.Model, tea.Cmd)
    CustomView      func(*ConfigScreen) string
    CustomOperations map[string]func(*ConfigScreen) tea.Cmd
}

type ColumnConfig struct {
    Field   string // Field name in resource struct
    Title   string
    Width   int    // 0 = dynamic (fills remaining space)
    Format  func(interface{}) string // Optional custom formatter
}

type OperationConfig struct {
    ID          string
    Name        string
    Description string
    Shortcut    string
}

// Example: Deployments screen (zero code, pure config)
var deploymentsConfig = ScreenConfig{
    ID:           "deployments",
    Title:        "Deployments",
    ResourceType: k8s.ResourceTypeDeployment,
    Columns: []ColumnConfig{
        {Field: "Namespace", Title: "Namespace", Width: 20},
        {Field: "Name", Title: "Name", Width: 0}, // Dynamic width
        {Field: "Ready", Title: "Ready", Width: 10},
        {Field: "UpToDate", Title: "Up-to-date", Width: 12},
        {Field: "Available", Title: "Available", Width: 12},
        {Field: "Age", Title: "Age", Width: 10, Format: formatDuration},
    },
    SearchFields: []string{"Namespace", "Name"},
    Operations: []OperationConfig{
        {ID: "scale", Name: "Scale", Shortcut: "s"},
        {ID: "restart", Name: "Restart", Shortcut: "r"},
        {ID: "describe", Name: "Describe", Shortcut: "d"},
    },
}

// 8 more similar configs for other standard list views...
// ConfigMaps, Secrets, StatefulSets, DaemonSets, Jobs, CronJobs,
// ReplicaSets, Ingresses
```

#### Level 2: Config + Custom Methods

**Use config for structure, override specific behavior:**

```go
// Pods screen with custom periodic refresh
var podsConfig = ScreenConfig{
    ID:           "pods",
    Title:        "Pods",
    ResourceType: k8s.ResourceTypePod,
    Columns: []ColumnConfig{
        {Field: "Namespace", Title: "Namespace", Width: 20},
        {Field: "Name", Title: "Name", Width: 0},
        {Field: "Ready", Title: "Ready", Width: 8},
        {Field: "Status", Title: "Status", Width: 15},
        {Field: "Restarts", Title: "Restarts", Width: 10},
        {Field: "Age", Title: "Age", Width: 10, Format: formatDuration},
        {Field: "Node", Title: "Node", Width: 30},
        {Field: "IP", Title: "IP", Width: 16},
    },
    SearchFields: []string{"Namespace", "Name", "Status", "Node", "IP"},
    Operations: []OperationConfig{
        {ID: "logs", Name: "View Logs", Shortcut: "l"},
        {ID: "describe", Name: "Describe", Shortcut: "d"},
        {ID: "delete", Name: "Delete", Shortcut: "x"},
    },

    // CUSTOM: Enable features
    EnablePeriodicRefresh: true,
    RefreshInterval:       1 * time.Second,
    TrackSelection:        true,

    // CUSTOM: Override Update to handle tick messages
    CustomUpdate: func(s *ConfigScreen, msg tea.Msg) (tea.Model,
        tea.Cmd) {
        switch msg := msg.(type) {
        case tickMsg:
            return s, tea.Batch(s.Refresh(), s.TickCmd())
        default:
            return s.DefaultUpdate(msg)
        }
    },
}

// Services screen with custom endpoint viewer
var servicesConfig = ScreenConfig{
    ID:           "services",
    Title:        "Services",
    ResourceType: k8s.ResourceTypeService,
    Columns: []ColumnConfig{
        {Field: "Namespace", Title: "Namespace", Width: 20},
        {Field: "Name", Title: "Name", Width: 0},
        {Field: "Type", Title: "Type", Width: 15},
        {Field: "ClusterIP", Title: "Cluster-IP", Width: 15},
        {Field: "ExternalIP", Title: "External-IP", Width: 15},
        {Field: "Ports", Title: "Ports", Width: 20},
        {Field: "Age", Title: "Age", Width: 10, Format: formatDuration},
    },
    SearchFields: []string{"Namespace", "Name", "Type"},
    Operations: []OperationConfig{
        {ID: "describe", Name: "Describe", Shortcut: "d"},
        {ID: "endpoints", Name: "Show Endpoints", Shortcut: "e"},
        {ID: "delete", Name: "Delete", Shortcut: "x"},
    },

    // CUSTOM: Add operation handler
    CustomOperations: map[string]func(*ConfigScreen) tea.Cmd{
        "endpoints": func(s *ConfigScreen) tea.Cmd {
            return showEndpointsModal(s.GetSelectedResource())
        },
    },
}
```

#### Level 3: Fully Custom Screen

**Implement Screen interface directly for unique UIs:**

```go
// Example: Log viewer with streaming (not a list view)
type LogViewerScreen struct {
    repo        k8s.Repository
    podName     string
    namespace   string
    logs        []string
    theme       *ui.Theme
    scrollPos   int
    autoScroll  bool
    streaming   bool
}

func NewLogViewerScreen(repo k8s.Repository, theme *ui.Theme,
    namespace, podName string) *LogViewerScreen {
    return &LogViewerScreen{
        repo:       repo,
        namespace:  namespace,
        podName:    podName,
        theme:      theme,
        autoScroll: true,
    }
}

// Implement Screen interface
func (s *LogViewerScreen) ID() string { return "logs" }
func (s *LogViewerScreen) Title() string {
    return fmt.Sprintf("Logs: %s/%s", s.namespace, s.podName)
}
func (s *LogViewerScreen) HelpText() string {
    return "↑/↓: scroll • a: toggle auto-scroll • q: back"
}
func (s *LogViewerScreen) Operations() []types.Operation { return nil }

func (s *LogViewerScreen) Init() tea.Cmd {
    return s.streamLogs() // Start streaming
}

func (s *LogViewerScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    // Fully custom update logic
    switch msg := msg.(type) {
    case logLineMsg:
        s.logs = append(s.logs, msg.line)
        if s.autoScroll {
            s.scrollPos = len(s.logs) - 1
        }
        return s, s.streamLogs() // Continue streaming
    case tea.KeyMsg:
        switch msg.String() {
        case "a":
            s.autoScroll = !s.autoScroll
            return s, nil
        case "q":
            return s, func() tea.Msg {
                return types.ScreenSwitchMsg{ScreenID: "pods"}
            }
        }
    }
    return s, nil
}

func (s *LogViewerScreen) View() string {
    // Custom rendering with viewport
    return s.renderLogs()
}

func (s *LogViewerScreen) SetSize(width, height int) {
    // Custom sizing
}
```

### ConfigScreen Implementation

```go
// ConfigScreen is the generic screen driven by ScreenConfig
type ConfigScreen struct {
    config   ScreenConfig
    repo     k8s.Repository
    table    table.Model
    items    []interface{}
    filtered []interface{}
    filter   string
    theme    *ui.Theme

    // For selection tracking (if enabled)
    selectedKey string
}

func NewConfigScreen(cfg ScreenConfig, repo k8s.Repository,
    theme *ui.Theme) *ConfigScreen {

    // Build table from config
    columns := make([]table.Column, len(cfg.Columns))
    for i, col := range cfg.Columns {
        columns[i] = table.Column{
            Title: col.Title,
            Width: col.Width,
        }
    }

    t := table.New(
        table.WithColumns(columns),
        table.WithFocused(true),
        table.WithHeight(10),
    )
    t.SetStyles(theme.ToTableStyles())

    return &ConfigScreen{
        config: cfg,
        repo:   repo,
        table:  t,
        theme:  theme,
    }
}

// Implement Screen interface
func (s *ConfigScreen) ID() string { return s.config.ID }
func (s *ConfigScreen) Title() string { return s.config.Title }
func (s *ConfigScreen) HelpText() string {
    return "↑/↓: navigate • /: filter • ctrl+s: screens • " +
           "ctrl+p: commands • ctrl+c: quit"
}

func (s *ConfigScreen) Operations() []types.Operation {
    ops := make([]types.Operation, len(s.config.Operations))
    for i, opCfg := range s.config.Operations {
        ops[i] = types.Operation{
            ID:          opCfg.ID,
            Name:        opCfg.Name,
            Description: opCfg.Description,
            Shortcut:    opCfg.Shortcut,
            Execute:     s.makeOperationHandler(opCfg),
        }
    }
    return ops
}

func (s *ConfigScreen) Init() tea.Cmd {
    cmds := []tea.Cmd{s.Refresh()}

    if s.config.EnablePeriodicRefresh {
        cmds = append(cmds, s.TickCmd())
    }

    return tea.Batch(cmds...)
}

func (s *ConfigScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    // Check for custom update handler
    if s.config.CustomUpdate != nil {
        return s.config.CustomUpdate(s, msg)
    }

    return s.DefaultUpdate(msg)
}

func (s *ConfigScreen) DefaultUpdate(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case types.RefreshCompleteMsg:
        if s.config.TrackSelection {
            s.restoreCursorPosition()
        }
        return s, nil
    case types.FilterUpdateMsg:
        s.SetFilter(msg.Filter)
        return s, nil
    case types.ClearFilterMsg:
        s.SetFilter("")
        return s, nil
    case tea.KeyMsg:
        var cmd tea.Cmd
        s.table, cmd = s.table.Update(msg)
        if s.config.TrackSelection {
            s.updateSelectedKey()
        }
        return s, cmd
    }

    var cmd tea.Cmd
    s.table, cmd = s.table.Update(msg)
    return s, cmd
}

func (s *ConfigScreen) View() string {
    if s.config.CustomView != nil {
        return s.config.CustomView(s)
    }
    return s.table.View()
}

func (s *ConfigScreen) SetSize(width, height int) {
    s.table.SetHeight(height)

    // Calculate dynamic column widths
    fixedTotal := 0
    dynamicCount := 0

    for _, col := range s.config.Columns {
        if col.Width > 0 {
            fixedTotal += col.Width
        } else {
            dynamicCount++
        }
    }

    // Account for cell padding: numColumns * 2
    padding := len(s.config.Columns) * 2
    dynamicWidth := (width - fixedTotal - padding) / dynamicCount
    if dynamicWidth < 20 {
        dynamicWidth = 20
    }

    columns := make([]table.Column, len(s.config.Columns))
    for i, col := range s.config.Columns {
        width := col.Width
        if width == 0 {
            width = dynamicWidth
        }
        columns[i] = table.Column{
            Title: col.Title,
            Width: width,
        }
    }

    s.table.SetColumns(columns)
    s.table.SetWidth(width)
}

func (s *ConfigScreen) Refresh() tea.Cmd {
    if s.config.CustomRefresh != nil {
        return s.config.CustomRefresh(s)
    }

    return func() tea.Msg {
        start := time.Now()

        items, err := s.repo.GetResources(s.config.ResourceType)
        if err != nil {
            return types.ErrorMsg{Error: fmt.Sprintf(
                "Failed to fetch %s: %v", s.config.Title, err)}
        }

        s.items = items
        s.applyFilter()

        return types.RefreshCompleteMsg{Duration: time.Since(start)}
    }
}

func (s *ConfigScreen) SetFilter(filter string) {
    s.filter = filter

    if s.config.CustomFilter != nil {
        s.config.CustomFilter(s, filter)
        return
    }

    s.applyFilter()
}

func (s *ConfigScreen) applyFilter() {
    if s.filter == "" {
        s.filtered = s.items
    } else {
        // Build search strings using reflection on configured fields
        searchStrings := make([]string, len(s.items))
        for i, item := range s.items {
            fields := []string{}
            for _, fieldName := range s.config.SearchFields {
                val := getFieldValue(item, fieldName)
                fields = append(fields, fmt.Sprint(val))
            }
            searchStrings[i] = strings.Join(fields, " ")
        }

        // Handle negation
        if strings.HasPrefix(s.filter, "!") {
            negatePattern := strings.TrimPrefix(s.filter, "!")
            matches := fuzzy.Find(negatePattern, searchStrings)
            matchSet := make(map[int]bool)
            for _, m := range matches {
                matchSet[m.Index] = true
            }

            s.filtered = make([]interface{}, 0)
            for i, item := range s.items {
                if !matchSet[i] {
                    s.filtered = append(s.filtered, item)
                }
            }
        } else {
            // Normal fuzzy search
            matches := fuzzy.Find(s.filter, searchStrings)
            s.filtered = make([]interface{}, len(matches))
            for i, m := range matches {
                s.filtered[i] = s.items[m.Index]
            }
        }
    }

    s.updateTable()
}

func (s *ConfigScreen) updateTable() {
    rows := make([]table.Row, len(s.filtered))

    for i, item := range s.filtered {
        row := make(table.Row, len(s.config.Columns))
        for j, col := range s.config.Columns {
            val := getFieldValue(item, col.Field)

            // Apply custom formatter if provided
            if col.Format != nil {
                row[j] = col.Format(val)
            } else {
                row[j] = fmt.Sprint(val)
            }
        }
        rows[i] = row
    }

    s.table.SetRows(rows)
}

// Helper: Get field value using reflection
func getFieldValue(obj interface{}, fieldName string) interface{} {
    v := reflect.ValueOf(obj)
    if v.Kind() == reflect.Ptr {
        v = v.Elem()
    }
    field := v.FieldByName(fieldName)
    if !field.IsValid() {
        return ""
    }
    return field.Interface()
}

// Helper: Get selected resource as map
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

### Screen Registry

```go
// Enhanced registry supporting both config and custom screens
type ScreenRegistry struct {
    configs map[string]ScreenConfig
    custom  map[string]Screen
    order   []string
}

func NewScreenRegistry() *ScreenRegistry {
    return &ScreenRegistry{
        configs: make(map[string]ScreenConfig),
        custom:  make(map[string]Screen),
        order:   []string{},
    }
}

func (r *ScreenRegistry) RegisterConfig(cfg ScreenConfig) {
    r.configs[cfg.ID] = cfg
    r.order = append(r.order, cfg.ID)
}

func (r *ScreenRegistry) RegisterCustom(screen Screen) {
    r.custom[screen.ID()] = screen
    r.order = append(r.order, screen.ID())
}

func (r *ScreenRegistry) Get(id string, repo k8s.Repository,
    theme *ui.Theme) Screen {

    // Check custom screens first
    if custom, ok := r.custom[id]; ok {
        return custom
    }

    // Check config screens
    if cfg, ok := r.configs[id]; ok {
        return NewConfigScreen(cfg, repo, theme)
    }

    return nil
}

func (r *ScreenRegistry) All(repo k8s.Repository, theme *ui.Theme)
    []Screen {

    screens := make([]Screen, 0, len(r.order))
    for _, id := range r.order {
        screen := r.Get(id, repo, theme)
        if screen != nil {
            screens = append(screens, screen)
        }
    }
    return screens
}
```

### Main Initialization

```go
// main.go or app initialization
func initializeScreens(repo k8s.Repository, theme *ui.Theme)
    *types.ScreenRegistry {

    registry := types.NewScreenRegistry()

    // Level 1: Register pure config screens (no code)
    registry.RegisterConfig(deploymentsConfig)
    registry.RegisterConfig(configMapsConfig)
    registry.RegisterConfig(secretsConfig)
    registry.RegisterConfig(statefulSetsConfig)
    registry.RegisterConfig(daemonSetsConfig)
    registry.RegisterConfig(jobsConfig)
    registry.RegisterConfig(cronJobsConfig)
    registry.RegisterConfig(replicaSetsConfig)
    registry.RegisterConfig(ingressesConfig)

    // Level 2: Register config with custom methods
    registry.RegisterConfig(podsConfig)      // Periodic refresh
    registry.RegisterConfig(servicesConfig)  // Custom operations

    // Level 3: Register fully custom screens
    // (Instantiate later when needed, not at startup)

    return registry
}
```

## Code Reduction Analysis

### Before (Current):

**Repository:**
- 3 resources = 292 lines
- 10 resources ≈ 800 lines
- 17 resources ≈ 1400 lines

**Screens:**
- 3 screens = 812 lines
- 10 screens ≈ 2700 lines
- 17 screens ≈ 4500 lines

**Total for 17 resources: ~5900 lines**

### After (Config-Driven):

**Repository:**
- Core implementation: ~400 lines
- Transform functions: 17 × ~30 lines = ~510 lines
- **Total: ~910 lines** (vs 1400)

**Screens:**
- ConfigScreen implementation: ~500 lines
- Screen configs: 15 × ~30 lines = ~450 lines
- Custom overrides: 2 × ~50 lines = ~100 lines
- **Total: ~1050 lines** (vs 4500)

**Total for 17 resources: ~1960 lines** (vs 5900)
**Reduction: 67% less code**

### Adding 18th Resource:

**Before:** +80 repository lines + 270 screen lines = 350 lines
**After:** +30 transform lines + 30 config lines = 60 lines
**83% less code per new resource**

## Progressive Adoption Path

### Phase 1: Refactor Repository (Week 1)

1. Add dynamic client alongside existing typed informers
2. Implement generic GetResources() method
3. Add transform functions for Pods, Deployments, Services
4. Create typed wrapper methods (GetPods, etc.)
5. Test parity with existing implementation
6. Switch screens to use new methods

### Phase 2: Introduce ConfigScreen (Week 2)

1. Implement ConfigScreen with core functionality
2. Migrate Deployments screen to pure config
3. Migrate Services screen to config + custom operations
4. Test thoroughly, ensure feature parity
5. Document config pattern

### Phase 3: Migrate Pods Screen (Week 2)

1. Migrate Pods to config + custom update
2. Verify periodic refresh works
3. Verify cursor tracking works
4. Remove old PodsScreen implementation

### Phase 4: Add New Resources (Week 3+)

1. Add ConfigMaps (pure config)
2. Add Secrets (pure config with sensitive data handling)
3. Add StatefulSets (pure config)
4. Add DaemonSets (pure config)
5. Add Jobs (pure config)
6. Add CronJobs (pure config)
7. Add remaining Tier 3 resources

Each new resource = ~60 lines (transform + config)

### Phase 5: Custom Screens (Week 4+)

1. Implement LogViewerScreen (Level 3 custom)
2. Implement NamespacePickerScreen (Level 3 custom)
3. Other unique UIs as needed

## Testing Strategy

### Unit Tests

**Repository layer:**
```go
func TestGetResourcesWithDynamicClient(t *testing.T) {
    // Create test environment with envtest
    // Add test objects
    // Call GetResources()
    // Verify transform works correctly
}

func TestTransformPod(t *testing.T) {
    // Create unstructured pod
    // Call transformPod()
    // Verify all fields mapped correctly
}
```

**Screen layer:**
```go
func TestConfigScreenInit(t *testing.T) {
    // Create ConfigScreen with test config
    // Verify table setup
    // Verify operations registered
}

func TestConfigScreenFilter(t *testing.T) {
    // Create ConfigScreen with test data
    // Apply filter
    // Verify fuzzy search works
    // Verify negation works
}

func TestCustomUpdateOverride(t *testing.T) {
    // Create config with CustomUpdate
    // Send message
    // Verify custom handler called
}
```

### Integration Tests

1. Test all 17 resources load correctly
2. Test tiered loading (Tier 1 blocks, Tier 2+ background)
3. Test screen switching between config and custom screens
4. Test filter/search across different resource types
5. Test periodic refresh for Pods screen
6. Test custom operations (endpoints viewer)

### Performance Validation

1. Measure initial load time (Tier 1 sync)
2. Measure background load time (Tier 2-3 sync)
3. Measure GetResources() latency (should be <1ms)
4. Measure transform overhead (should be negligible)
5. Measure memory usage with all 17 resources loaded
6. Compare with current typed implementation

## Alternatives Considered

### Alternative 1: Keep Typed Informers for Everything

**Approach:** Continue current pattern for all resources

**Pros:**
- Type-safe
- Simple, no reflection
- Fast

**Cons:**
- 5900+ lines of code for 17 resources
- Cannot support CRDs
- Maintenance nightmare

**Rejected:** Doesn't scale

### Alternative 2: Pure Reflection/Config (No Custom Screens)

**Approach:** Everything defined in config, no code customization

**Pros:**
- Minimal code (~1000 lines total)
- Very DRY

**Cons:**
- Cannot handle unique UIs (logs, graphs, etc.)
- Heavy reflection overhead
- Loses type safety completely
- Hard to customize behavior

**Rejected:** Too rigid, loses flexibility

### Alternative 3: Go Generics for Everything

**Approach:** Use generics for repository and screens

**Pros:**
- Type-safe
- Less code than current

**Cons:**
- Complex generic constraints
- Still requires code per resource type
- Harder to understand
- Doesn't help with screens much

**Rejected:** Doesn't reduce screen code enough

### Alternative 4: Code Generation

**Approach:** Generate repository and screen code from schemas

**Pros:**
- Type-safe generated code
- Can regenerate anytime

**Cons:**
- Adds build complexity
- Generated code still huge (5900 lines)
- Hard to customize generated code
- Doesn't solve maintenance issue

**Rejected:** Trades runtime flexibility for build complexity

## Consequences

### Positive

- **67% less code** for 17 resources (1960 vs 5900 lines)
- **83% less code** per new resource (60 vs 350 lines)
- **CRD support:** Dynamic client works with any resource
- **Progressive customization:** Start with config, add code only when
  needed
- **Type safety where it matters:** Transform functions are typed
- **Flexibility:** Three levels (config, config+custom, fully custom)
- **Scalability:** Adding resources is trivial (~30 line config)
- **Maintainability:** Common logic in one place
- **Testing:** Test generic logic once, not 17 times

### Negative

- **Reflection overhead:** Some use of reflection for field access
  (minimal performance impact)
- **Initial complexity:** ConfigScreen is more complex than single
  screen (~500 lines)
- **Learning curve:** Developers need to understand config pattern
- **Debugging:** Reflection-based errors harder to trace
- **Type safety loss:** Interface{} in some places, runtime panics
  possible
- **Migration effort:** Need to refactor existing screens

### Neutral

- **Two patterns:** Config screens vs custom screens (acceptable
  trade-off)
- **Transform boilerplate:** Still need transform function per resource
  (~30 lines each)
- **Reflection dependency:** Acceptable for config-driven approach

## Future Enhancements

### CRD Discovery

Automatically discover and display CRDs:

```go
// Discover all CRDs in cluster
func (r *InformerRepository) DiscoverCRDs() ([]ResourceConfig, error) {
    crdClient := r.clientset.ApiextensionsV1().CustomResourceDefinitions()
    crdList, err := crdClient.List(context.TODO(), metav1.ListOptions{})

    configs := []ResourceConfig{}
    for _, crd := range crdList.Items {
        gvr := schema.GroupVersionResource{
            Group:    crd.Spec.Group,
            Version:  crd.Spec.Versions[0].Name,
            Resource: crd.Spec.Names.Plural,
        }

        configs = append(configs, ResourceConfig{
            GVR:        gvr,
            Name:       crd.Spec.Names.Kind,
            Namespaced: crd.Spec.Scope == "Namespaced",
            Tier:       4, // On-demand
            Transform:  makeGenericTransform(crd), // Auto-generate
        })
    }

    return configs, nil
}
```

### Auto-Generated Screen Configs

Generate screen configs from OpenAPI schemas:

```go
// Generate screen config from discovered CRD
func makeScreenConfigFromCRD(crd ResourceConfig) ScreenConfig {
    return ScreenConfig{
        ID:           string(crd.Name),
        Title:        crd.Name,
        ResourceType: k8s.ResourceType(crd.Name),
        Columns:      autoDetectColumns(crd),
        SearchFields: []string{"Namespace", "Name"},
        Operations:   defaultOperations,
    }
}
```

### Config Persistence

Save user customizations (column widths, sort order):

```go
// ~/.config/timoneiro/screens.yaml
screens:
  pods:
    columns:
      - field: Namespace
        width: 25  # User adjusted
      - field: Name
        width: 50
    sort: Age desc
    refresh: 2s  # User preference
```

### Plugin System

Allow external plugins to add screens:

```go
// Plugin interface
type ScreenPlugin interface {
    ID() string
    CreateScreen(repo k8s.Repository, theme *ui.Theme) Screen
}

// Load plugins
func LoadPlugins(pluginDir string) []ScreenPlugin {
    // Discover .so files
    // Load with plugin.Open()
    // Return screens
}
```

## References

- [kubernetes/client-go dynamic client](
  https://pkg.go.dev/k8s.io/client-go/dynamic)
- [kubernetes/client-go dynamic informer](
  https://pkg.go.dev/k8s.io/client-go/dynamic/dynamicinformer)
- [kubernetes/apimachinery unstructured](
  https://pkg.go.dev/k8s.io/apimachinery/pkg/apis/meta/v1/unstructured)
- DDR-03: Kubernetes Informer-Based Repository Implementation
- DDR-06: Resource Detail Commands (Describe and YAML)
- Current implementations: internal/k8s/informer_repository.go,
  internal/screens/*.go
