# Kubernetes Informer-Based Repository Implementation

| Metadata | Value                                      |
|----------|--------------------------------------------|
| Date     | 2025-10-04                                 |
| Author   | @renato0307                                |
| Status   | Proposed                                   |
| Tags     | kubernetes, informers, repository, caching |

| Revision | Date       | Author      | Info           |
|----------|------------|-------------|----------------|
| 1        | 2025-10-04 | @renato0307 | Initial design |

## Context and Problem Statement

Timoneiro currently uses a `DummyRepository` that returns fake data. We need
to implement a production-ready Kubernetes repository that provides real-time
data from a Kubernetes cluster with minimal latency. The POC in
`cmd/proto-pods-tui/main.go` demonstrated that Kubernetes informers with local
caching can provide microsecond-level query performance after initial sync.

**Key Questions:**
- How do we structure the repository to leverage informer caching?
- How do we handle the informer lifecycle (initialization, sync, updates)?
- How do we integrate real-time updates with the Bubble Tea event loop?
- How do we support filtering/searching efficiently?

## Prior Work

The POC implementation (`cmd/proto-pods-tui/main.go`) validated several key
approaches:

1. **Full Informers with Protobuf** - Originally planned metadata-only, but
   full pod data is needed for display (Ready, Status, Restarts, Node, IP).
   Using protobuf encoding (`application/vnd.kubernetes.protobuf`) reduces
   network payload vs JSON while still getting full objects.

2. **Performance Characteristics:**
   - Initial cache sync: ~1-2 seconds for hundreds of pods
   - Cache query time: 15-25μs (microseconds)
   - Search time: 1-5ms for 100s of pods using fuzzy matching

3. **Informer Pattern:**
   - Use `SharedInformerFactory` to share informers across resources
   - Use `Lister` interface for thread-safe cache queries
   - Periodic refresh from cache (POC used 1-second polling)

4. **Fuzzy Search:**
   - Library: `github.com/sahilm/fuzzy`
   - Search across: Namespace, Name, Status, Node, IP
   - Negation support: `!pattern` excludes matches

## Design Challenges

### Multi-Resource Scaling Problem

Kubernetes clusters have 100+ resource types (see `kubectl api-resources`),
including core resources (Pods, Services), workload controllers (Deployments,
StatefulSets), config (ConfigMaps, Secrets), and many custom resources.

**Challenge:** Cannot load all resources at startup because:
- Too many resource types (100+) would take 10-30 seconds to sync
- Most resources are rarely viewed (80/20 rule)
- Excessive memory usage for unused data
- Poor user experience waiting for everything

**Solution: Tiered Loading Strategy**
1. **Priority Tier (Tier 1):** Load Pods immediately (block UI)
   - Pods are the most frequently viewed resource
   - UI can start as soon as Pods are ready (~1-2s)

2. **Background Tiers (Tier 2-3):** Load common resources in parallel
   - Deployments, Services, StatefulSets, etc.
   - Load while UI is already responsive
   - Show loading status in UI

3. **Deferred Tier (Tier 4):** Load on-demand or later
   - Less common resources (Ingresses, PVs, etc.)
   - Future enhancement

**Key Insight:** User wants to see Pods immediately. Loading everything else
can happen in the background without blocking the main workflow.

## Design

### Architecture Overview

```
┌─────────────────────────────────────────────────────────────┐
│                      Kubernetes Cluster                      │
└─────────────────────┬───────────────────────────────────────┘
                      │ Watch API (protobuf)
                      │
┌─────────────────────▼───────────────────────────────────────┐
│              SharedInformerFactory                           │
│  ┌────────────────┐  ┌────────────────┐  ┌────────────────┐│
│  │ Pod Informer   │  │Deploy Informer │  │Service Informer││
│  │   + Cache      │  │   + Cache      │  │   + Cache      ││
│  └────────────────┘  └────────────────┘  └────────────────┘│
└─────────────────────┬───────────────────────────────────────┘
                      │
┌─────────────────────▼───────────────────────────────────────┐
│            InformerRepository (implements Repository)        │
│  - PodLister                                                 │
│  - DeploymentLister                                          │
│  - ServiceLister                                             │
│  - Transform K8s objects → internal types                    │
│  - Search/filter logic                                       │
└─────────────────────┬───────────────────────────────────────┘
                      │
┌─────────────────────▼───────────────────────────────────────┐
│                  Bubble Tea Screens                          │
│  - Periodic refresh (tickCmd every 1s)                      │
│  - Filter/search on cached data                             │
└─────────────────────────────────────────────────────────────┘
```

### Supported Resources (Tiered Loading)

Kubernetes has 100+ resource types. We'll support only the most commonly used
resources, loaded in tiers:

**Tier 1 (Critical - Block UI):**
- Pods - Main resource, most frequently viewed

**Tier 2 (Important - Load in Parallel with UI):**
- Deployments - Most common workload controller
- Services - Service discovery/networking
- Namespaces - Organization/filtering
- StatefulSets - Stateful workloads
- DaemonSets - Node-level workloads

**Tier 3 (Secondary - Load in Background):**
- ReplicaSets - Usually viewed through Deployments
- Jobs - Batch workloads
- CronJobs - Scheduled workloads
- ConfigMaps - Configuration
- Secrets - Sensitive data (metadata only for security)

**Tier 4 (Optional - Load on Demand or Later):**
- PersistentVolumeClaims (PVCs) - Storage
- PersistentVolumes (PVs) - Storage
- Ingresses - External access
- Nodes - Cluster infrastructure
- Events - Debugging (high volume, special handling)
- ServiceAccounts - RBAC
- HorizontalPodAutoscalers (HPAs) - Scaling

**Strategy:**
- UI waits only for Pods (Tier 1) to be synced
- Tiers 2-3 load in parallel after UI starts
- Show loading progress in UI (e.g., "Loading Deployments... 7/10 synced")
- Tier 4 resources loaded on-demand or deferred to future releases

### Repository Interface Enhancement

Updated interface with loading status support:

```go
// ResourceType identifies different Kubernetes resources
type ResourceType string

const (
    ResourceTypePod         ResourceType = "pods"
    ResourceTypeDeployment  ResourceType = "deployments"
    ResourceTypeService     ResourceType = "services"
    ResourceTypeNamespace   ResourceType = "namespaces"
    ResourceTypeStatefulSet ResourceType = "statefulsets"
    ResourceTypeDaemonSet   ResourceType = "daemonsets"
    // ... other resource types
)

// LoadingStatus represents the sync status of a resource type
type LoadingStatus struct {
    Resource ResourceType
    Synced   bool
    Error    error
}

type Repository interface {
    // Resource accessors
    GetPods() ([]Pod, error)
    GetDeployments() ([]Deployment, error)
    GetServices() ([]Service, error)
    GetNamespaces() ([]Namespace, error)
    GetStatefulSets() ([]StatefulSet, error)
    GetDaemonSets() ([]DaemonSet, error)
    // ... other Get methods

    // Loading status
    GetLoadingStatus(resource ResourceType) LoadingStatus
    GetAllLoadingStatus() map[ResourceType]LoadingStatus
    IsResourceSynced(resource ResourceType) bool

    // Lifecycle
    Stop()
}
```

Future enhancements (out of scope for initial implementation):
- `GetPod(namespace, name string) (*Pod, error)` - Get single pod
- `SearchPods(query string) ([]Pod, error)` - Fuzzy search
- `WatchPods(ctx context.Context) (<-chan PodEvent, error)` - Real-time
  updates
- Namespace filtering parameter

### InformerRepository Implementation

```go
// InformerRepository uses Kubernetes informers for local caching
type InformerRepository struct {
    // Kubernetes client
    clientset *kubernetes.Clientset

    // Shared informer factory (creates and manages informers)
    factory informers.SharedInformerFactory

    // Listers for thread-safe cache access
    podLister         v1listers.PodLister
    deploymentLister  appsv1listers.DeploymentLister
    serviceLister     v1listers.ServiceLister
    namespaceLister   v1listers.NamespaceLister
    statefulSetLister appsv1listers.StatefulSetLister
    daemonSetLister   appsv1listers.DaemonSetLister
    // ... other listers

    // Informer references for status checking
    informers map[ResourceType]cache.SharedIndexInformer

    // Loading status tracking
    loadingStatus map[ResourceType]*LoadingStatus
    statusMu      sync.RWMutex

    // Synchronization
    stopCh chan struct{}
}

// NewInformerRepository creates a new repository with informers
// Only creates informers, does not start syncing yet
func NewInformerRepository(kubeconfig string, context string) (
    *InformerRepository, error)

// StartPriority starts the priority (Tier 1) informers and blocks until
// they are synced. Returns error if sync fails.
// This ensures Pods are ready before the UI starts.
func (r *InformerRepository) StartPriority(ctx context.Context) error

// StartBackground starts the remaining (Tier 2+) informers in parallel
// and returns immediately. Sync status can be checked via GetLoadingStatus.
// This allows the UI to start while other resources load.
func (r *InformerRepository) StartBackground(ctx context.Context)

// GetLoadingStatus returns the sync status of a specific resource
func (r *InformerRepository) GetLoadingStatus(
    resource ResourceType) LoadingStatus

// GetAllLoadingStatus returns sync status of all resources
func (r *InformerRepository) GetAllLoadingStatus()
    map[ResourceType]LoadingStatus

// IsResourceSynced checks if a specific resource is synced
func (r *InformerRepository) IsResourceSynced(
    resource ResourceType) bool

// Stop gracefully stops all informers
func (r *InformerRepository) Stop()
```

### Initialization Lifecycle

**Phase 1: Repository Creation**
```go
// App initialization
repo, err := k8s.NewInformerRepository(kubeconfigPath, contextName)
if err != nil {
    return fmt.Errorf("failed to create repository: %w", err)
}
```

**Phase 2: Priority Resources Sync (Blocking)**
```go
// Start priority informers (Pods only) and wait for cache sync
ctx := context.Background()
fmt.Println("Syncing pods...")
if err := repo.StartPriority(ctx); err != nil {
    return fmt.Errorf("failed to start priority informers: %w", err)
}
// At this point, Pods are synced and UI can start
fmt.Println("Pods synced! Starting UI...")
```

**Phase 3: Background Resources Sync (Non-blocking)**
```go
// Start remaining informers in background (returns immediately)
go repo.StartBackground(ctx)

// UI starts immediately, can show loading status for other resources
app := app.NewModel(repo)
tea.NewProgram(app).Run()
```

**Phase 4: Query Phase**
```go
// Pods are ready immediately
pods, err := repo.GetPods()

// Other resources may still be loading
if repo.IsResourceSynced(k8s.ResourceTypeDeployment) {
    deployments, err := repo.GetDeployments()
} else {
    // Show "Loading..." in UI
    status := repo.GetLoadingStatus(k8s.ResourceTypeDeployment)
    // status.Synced == false
}
```

**Parallel Loading Implementation:**
```go
func (r *InformerRepository) StartBackground(ctx context.Context) {
    // Start factory (begins watching all registered informers)
    r.factory.Start(r.stopCh)

    // Launch goroutines to wait for each resource to sync
    resources := []ResourceType{
        ResourceTypeDeployment,
        ResourceTypeService,
        ResourceTypeNamespace,
        ResourceTypeStatefulSet,
        ResourceTypeDaemonSet,
        // ... other Tier 2+ resources
    }

    var wg sync.WaitGroup
    for _, res := range resources {
        wg.Add(1)
        go func(resource ResourceType) {
            defer wg.Done()

            informer := r.informers[resource]
            if cache.WaitForCacheSync(
                ctx.Done(), informer.HasSynced) {
                // Mark as synced
                r.updateLoadingStatus(resource, true, nil)
            } else {
                // Mark as failed
                r.updateLoadingStatus(
                    resource, false,
                    fmt.Errorf("cache sync timeout"))
            }
        }(res)
    }

    // Wait for all to complete (in background goroutine)
    wg.Wait()
}
```

This approach ensures:
- User sees "Syncing pods..." message (1-2 seconds)
- UI starts as soon as Pods are ready
- Other resources load in parallel while UI is responsive
- UI can show loading progress (e.g., in header or status bar)
- No blocking on less critical resources

### Data Transformation

Transform Kubernetes objects to internal types:

```go
func transformPod(pod *corev1.Pod) k8s.Pod {
    now := time.Now()
    age := now.Sub(pod.CreationTimestamp.Time)

    // Calculate ready containers
    readyCount := 0
    totalCount := len(pod.Status.ContainerStatuses)
    for _, cs := range pod.Status.ContainerStatuses {
        if cs.Ready {
            readyCount++
        }
    }

    // Calculate total restarts
    restarts := int32(0)
    for _, cs := range pod.Status.ContainerStatuses {
        restarts += cs.RestartCount
    }

    return k8s.Pod{
        Namespace: pod.Namespace,
        Name:      pod.Name,
        Ready:     fmt.Sprintf("%d/%d", readyCount, totalCount),
        Status:    string(pod.Status.Phase),
        Restarts:  restarts,
        Age:       age,
        Node:      pod.Spec.NodeName,
        IP:        pod.Status.PodIP,
    }
}
```

### GetPods Implementation

```go
func (r *InformerRepository) GetPods() ([]Pod, error) {
    r.mu.RLock()
    defer r.mu.RUnlock()

    if !r.synced {
        return nil, fmt.Errorf("cache not synced yet")
    }

    // List all pods from cache (microsecond operation)
    podList, err := r.podLister.List(labels.Everything())
    if err != nil {
        return nil, fmt.Errorf("failed to list pods: %w", err)
    }

    // Transform to internal type
    pods := make([]Pod, 0, len(podList))
    for _, pod := range podList {
        pods = append(pods, transformPod(pod))
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

### Search Implementation

Search will be implemented at the screen level (not in repository) because:
1. Search is a UI concern with UI state (filter text, mode)
2. Cache queries are so fast (microseconds) that filtering in-memory is
   negligible
3. Keeps repository interface simple

Screens will:
```go
// Fetch all pods from cache
pods, err := repo.GetPods()

// Apply fuzzy search (1-5ms for 100s of pods)
filtered := fuzzySearchPods(pods, filterText)
```

### Bubble Tea Integration

**Initialization:**
```go
// main.go or app initialization
repo, err := k8s.NewInformerRepository(kubeconfig, context)
if err != nil {
    log.Fatal(err)
}

// Start priority resources (blocks until Pods are synced)
fmt.Println("Syncing pods...")
if err := repo.StartPriority(ctx); err != nil {
    log.Fatal(err)
}
fmt.Println("Pods synced! Starting UI...")

// Start background resources (returns immediately)
go repo.StartBackground(ctx)

// Pass repository to app
app := app.NewModel(repo)
tea.NewProgram(app).Run()
```

**Periodic Refresh with Loading Status:**
```go
// Screen implements periodic refresh
func tickCmd() tea.Cmd {
    return tea.Tick(1*time.Second, func(t time.Time) tea.Msg {
        return tickMsg(t)
    })
}

// In Update()
case tickMsg:
    // Pods are always ready (synced before UI started)
    pods, err := m.repo.GetPods()
    if err != nil {
        return m, tickCmd()
    }
    m.pods = pods
    m.filteredPods = filterPods(m.pods, m.filterText)

    // Update loading status for other resources
    m.loadingStatus = m.repo.GetAllLoadingStatus()

    return m, tickCmd()
```

**UI Display with Loading Progress:**
```go
// In header component or status bar
func (m model) View() string {
    // Build loading status indicator
    loadingIndicator := ""
    status := m.repo.GetAllLoadingStatus()

    loading := []string{}
    for resource, s := range status {
        if !s.Synced && s.Error == nil {
            loading = append(loading, string(resource))
        }
    }

    if len(loading) > 0 {
        loadingIndicator = fmt.Sprintf(
            " | Loading: %s", strings.Join(loading, ", "))
    }

    header := fmt.Sprintf("Timoneiro - Pods (%d)%s",
        len(m.pods), loadingIndicator)

    // ... rest of UI
}
```

**Example Header Display:**
```
Timoneiro - Pods (147) | Loading: deployments, services, namespaces

┌──────────────────────────────────────────────────────────┐
│ Namespace   │ Name                    │ Ready  │ Status  │
├──────────────────────────────────────────────────────────┤
│ default     │ nginx-7d64f8d9c8-abc12  │ 1/1    │ Running │
│ ...         │ ...                     │ ...    │ ...     │
└──────────────────────────────────────────────────────────┘
```

After a few seconds, when all resources are loaded:
```
Timoneiro - Pods (147)

┌──────────────────────────────────────────────────────────┐
│ Namespace   │ Name                    │ Ready  │ Status  │
├──────────────────────────────────────────────────────────┤
│ default     │ nginx-7d64f8d9c8-abc12  │ 1/1    │ Running │
│ ...         │ ...                     │ ...    │ ...     │
└──────────────────────────────────────────────────────────┘
```

### Configuration

**Protobuf Encoding:**
```go
config.ContentType = "application/vnd.kubernetes.protobuf"
```

**Informer Resync Period:**
```go
// 30 seconds resync period (standard value)
factory := informers.NewSharedInformerFactory(
    clientset, 30*time.Second)
```

The resync period triggers periodic full list operations to ensure cache
consistency. 30 seconds is a good balance between API server load and data
freshness.

**Kubeconfig Loading:**
```go
// Support standard kubeconfig locations
// - $KUBECONFIG environment variable
// - --kubeconfig flag
// - ~/.kube/config (default)

loadingRules := &clientcmd.ClientConfigLoadingRules{
    ExplicitPath: kubeconfigPath,
}
configOverrides := &clientcmd.ConfigOverrides{
    CurrentContext: contextName, // Optional context override
}
config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
    loadingRules,
    configOverrides,
).ClientConfig()
```

### Error Handling

**Cache Sync Errors:**
```go
if !cache.WaitForCacheSync(ctx.Done(), podInformer.HasSynced) {
    return fmt.Errorf("cache sync timeout or failed")
}
```

**Query Errors:**
- Return errors to caller (screen)
- Screen displays error message in UI
- Continue periodic refresh (errors may be transient)

**Connection Errors:**
- Initial connection failure: Fatal error, exit with message
- Runtime disconnection: Informer will reconnect automatically
- Show warning in UI if data is stale (future enhancement)

### Thread Safety

- Informer cache is thread-safe (client-go guarantees)
- Lister methods are thread-safe
- Use `sync.RWMutex` to protect repository state (synced flag)
- Read-heavy workload: RLock for queries, Lock only for state changes

### Memory Considerations

**Full vs Metadata-Only:**
- Full pod informers: ~2-5KB per pod (with protobuf)
- Metadata-only: ~500 bytes per pod
- For typical clusters (100-1000 pods): 200KB - 5MB memory
- Trade-off accepted: Full data needed for display, memory cost acceptable

**Namespace Filtering (Future):**
- Watch only specific namespaces to reduce memory
- `factory := informers.NewSharedInformerFactoryWithOptions(clientset,
  resync, informers.WithNamespace(namespace))`

## Testing Strategy

**Manual Testing:**
1. Test with local minikube/kind cluster
2. Test with real clusters (small, medium, large)
3. Test cache sync time for various cluster sizes
4. Test filter performance with many pods
5. Test parallel loading (verify UI starts before all resources sync)
6. Test loading status display in UI
7. Test switching screens while resources are still loading

**Error Scenarios:**
1. Invalid kubeconfig
2. Network disconnection during priority sync (Pods)
3. Network disconnection during background sync
4. Network disconnection during runtime
5. Context switch while running
6. Permission errors (RBAC) - some resources accessible, others not
7. Timeout during background loading (some resources fail to sync)

**Performance Validation:**
1. Measure priority sync time (Pods only, should be ~1-2s)
2. Measure background sync time (all Tier 2-3 resources)
3. Measure query time (should be <100μs per resource)
4. Measure memory usage with all resources loaded
5. Profile CPU usage during parallel loading
6. Verify UI responsiveness during background loading

**Expected Performance (Medium Cluster: 100-500 pods, ~50 other resources):**
- Priority sync (Pods): 1-2 seconds
- UI start time: 1-2 seconds (same as priority sync)
- Background sync (Tier 2): 2-4 seconds (parallel)
- Background sync (Tier 3): 3-5 seconds (parallel)
- Total memory: 5-20MB (all resources)
- Query latency: <100μs per resource type

## Alternatives Considered

### Alternative 1: Direct API Calls (No Cache)
**Pros:** Simple, no cache management
**Cons:** 100-500ms latency per query, API server load, rate limiting
**Rejected:** Too slow for real-time UI

### Alternative 2: Metadata-Only Informers
**Pros:** 70-90% less memory
**Cons:** Need full pod data for display (Ready, Status, Node, IP)
**Rejected:** Would require additional API calls, defeating the purpose

### Alternative 3: Server-Side Filtering
**Pros:** Less data transferred
**Cons:** Slower (network round-trip), complex API calls, limits fuzzy
search
**Rejected:** Local filtering is fast enough (<5ms)

### Alternative 4: Watch-Based Push Updates
**Pros:** More efficient than polling, real-time updates
**Cons:** More complex implementation, need event reconciliation
**Deferred:** Start with polling, add watch later if needed

### Alternative 5: Load All Resources Before Starting UI
**Pros:** Simpler implementation, all data available immediately
**Cons:** Slow startup (5-10 seconds), poor user experience, wasted time for
unused resources
**Rejected:** User wants to see Pods immediately, not wait for all resources

### Alternative 6: Support All 100+ Kubernetes Resources
**Pros:** Complete coverage, handles custom resources
**Cons:** Excessive memory, complex UI, maintenance burden, most resources
rarely used
**Rejected:** Focus on common resources (80/20 rule), add more later if needed

## Consequences

### Positive
- **Performance:** Microsecond-level query latency after sync
- **Fast Startup:** UI starts in 1-2 seconds (Pods only), not 5-10s (all
  resources)
- **Responsive UI:** Background loading doesn't block user interaction
- **Transparency:** Loading status visible to user, no mystery about what's
  happening
- **Simplicity:** Proven pattern (kubectl uses same approach)
- **Real-time:** Watch connections provide live updates
- **Scalability:** Works well for clusters up to 10k+ pods
- **Offline-capable:** Cache allows continued operation during brief network
  issues
- **Parallel Efficiency:** Multiple resources sync simultaneously (faster
  than sequential)

### Negative
- **Initial Delay:** 1-2 second cache sync for Pods before UI starts
  (acceptable trade-off)
- **Memory:** Full objects require more memory than metadata-only
- **Complexity:** Informer lifecycle management and parallel loading add code
  complexity
- **Partial State:** Screens may show "Loading..." for a few seconds after
  startup
- **Resource Limitation:** Only ~15-20 resource types supported initially
  (not all 100+)

### Neutral
- **Polling vs Push:** Starting with 1-second polling (can optimize later)
- **Namespace Filtering:** Not implemented initially (add when needed)
- **Tier Selection:** Resource tiers based on common usage, may need
  adjustment

## References

- [Kubernetes client-go Informers](
  https://pkg.go.dev/k8s.io/client-go/informers)
- [Kubernetes client-go Listers](
  https://pkg.go.dev/k8s.io/client-go/listers)
- [Kubernetes client-go Cache](
  https://pkg.go.dev/k8s.io/client-go/tools/cache)
- POC Implementation: `cmd/proto-pods-tui/main.go`
- Current Repository Interface: `internal/k8s/repository.go`
