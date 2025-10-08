# Scale k1 to 31 Kubernetes Resources Implementation Plan

## Overview

Add support for 20 new Kubernetes resources (bringing total from 11 to 31)
and create a system resources monitoring screen to track resource counts,
memory usage, and update activity. This implements Issue #3 with guidance
from the scaling research document.

## Current State Analysis

**Supported Resources (11)**:
- Pods, Deployments, StatefulSets, DaemonSets, Jobs, CronJobs
- Services, ConfigMaps, Secrets
- Namespaces, Nodes

**Architecture Strengths**:
- Config-driven multi-resource system eliminates duplication
- Single `ConfigScreen` handles all resource types
- Navigation factory functions support owner/selector/volume relationships
- Repository pattern with informer-based caching
- Dynamic client with unstructured resources for extensibility

**Missing Infrastructure**:
- No memory usage tracking (no runtime.MemStats monitoring)
- No resource statistics tracking (counts, update timestamps)
- Informer sync status discarded after initialization
- No system metrics dashboard screen

**Key Constraint**: Research shows memory is NOT a constraint. 71
informers on 1000-pod cluster would use <5MB (vs 8GB etcd limit). The
challenge is organization and user value, not technical limits.

## Desired End State

### Resource Coverage
- **31 total resource screens** (current 11 + new 20)
- **Tier 1** (16 total): Current 11 + ReplicaSets, PVCs, Ingresses,
  Endpoints, HPAs
- **Tier 2** (23 total): Tier 1 + ServiceAccounts, NetworkPolicies, PVs,
  StorageClasses, PDBs, Roles, RoleBindings
- **Tier 3** (31 total): Tier 2 + MutatingWebhookConfigs,
  ValidatingWebhookConfigs, ReplicationControllers, ClusterRoles,
  ClusterRoleBindings, IngressClasses, VolumeAttachments, VPAs

### System Resources Screen
- Accessible via `:system-resources` command
- Shows table with columns:
  - Resource Type (name)
  - Count (number of objects in cluster)
  - Memory (approximate MB per resource type)
  - Synced (yes/no, when informer last synced)
  - Updates (count of add/update/delete events since app started)
  - Last Update (timestamp of most recent event)
- Real-time statistics (1-second refresh)
- Helps users understand cluster scale and app health

### Verification
1. Navigate to all 31 resource screens successfully
2. Enter key navigation works for all resources with relationships
3. System resources screen shows accurate statistics
4. Memory usage stays under 10MB even on large clusters
5. All screens respond in <100ms on 1000+ resource clusters

## What We're NOT Doing

- Custom Resource Definitions (CRDs) support - hardcoded built-in types
  only
- Lazy loading or tier-based initialization - all informers start eagerly
- Metrics-server integration for real-time pod/node metrics
- Historical statistics or time-series data
- Alerting or threshold-based warnings
- Multi-cluster support (single kubeconfig context only)
- Webhook admission control testing features
- VPA recommendations UI (just list VPAs)

## Implementation Approach

**Phased rollout starting with monitoring infrastructure**:

**Phase 1 (System Screen)**: Monitoring infrastructure first
- Add statistics tracking to repository layer
- Implement system resources screen for current 11 resources
- Provides immediate value: visibility into cluster state
- Foundation for tracking new resources as they're added
- Validates statistics infrastructure before expansion

**Phase 2 (Tier 1)**: Add 5 most valuable resources
- Validates transform function patterns
- Establishes navigation for new relationship types (RS→Deployment,
  PVC→Pod, etc.)
- Tests performance with 16 total resources
- Delivers 52% user-facing resource coverage
- New resources automatically appear in system screen

**Phase 3 (Tier 2)**: Add RBAC and advanced networking
- More complex transforms (RBAC relationships, policy matching)
- Multi-target navigation (ServiceAccount→Pods OR RoleBindings)
- Brings coverage to 74%

**Phase 4 (Tier 3)**: Completeness and cluster admin features
- Rare/legacy resources (ReplicationControllers, VPA)
- Cluster-scoped RBAC (ClusterRoles, ClusterRoleBindings)
- Webhooks for admission control auditing
- 100% coverage of user-facing resources

This ordering delivers immediate monitoring value and ensures the
statistics infrastructure is battle-tested before scaling to 31
resources. Each new resource added in later phases automatically
appears in the system screen.

---

## Phase 2: Tier 1 Resources (5 New Resources)

### Overview
Add the 5 highest-priority resources (ReplicaSets, PVCs, Ingresses,
Endpoints, HPAs) bringing total from 11 to 16 resources. These cover
90% of daily troubleshooting workflows.

### Changes Required

#### 1. Add Resource Type Constants
**File**: `internal/k8s/repository.go`
**Changes**: Add 5 new constants after line 25

```go
ResourceTypeReplicaSet            ResourceType = "replicasets"
ResourceTypePersistentVolumeClaim ResourceType = "persistentvolumeclaims"
ResourceTypeIngress               ResourceType = "ingresses"
ResourceTypeEndpoints             ResourceType = "endpoints"
ResourceTypeHPA                   ResourceType = "horizontalpodautoscalers"
```

#### 2. Define Typed Structs
**File**: `internal/k8s/types.go`
**Changes**: Add 5 new structs after Node struct (line 223)

```go
type ReplicaSet struct {
    Namespace   string
    Name        string
    Desired     int32
    Current     int32
    Ready       int32
    Age         time.Duration
    CreatedAt   time.Time
}

type PersistentVolumeClaim struct {
    Namespace    string
    Name         string
    Status       string  // Bound, Pending, Lost
    Volume       string  // PV name
    Capacity     string  // "10Gi"
    AccessModes  string  // "RWO", "RWX", "ROX"
    StorageClass string
    Age          time.Duration
    CreatedAt    time.Time
}

type Ingress struct {
    Namespace   string
    Name        string
    Class       string    // IngressClass name
    Hosts       string    // Comma-separated
    Address     string    // LoadBalancer IP/hostname
    Ports       string    // "80, 443"
    Age         time.Duration
    CreatedAt   time.Time
}

type Endpoints struct {
    Namespace   string
    Name        string
    Endpoints   string    // "10.0.1.5:8080, 10.0.1.6:8080" (comma-separated)
    Age         time.Duration
    CreatedAt   time.Time
}

type HorizontalPodAutoscaler struct {
    Namespace   string
    Name        string
    Reference   string    // "Deployment/nginx"
    MinPods     int32
    MaxPods     int32
    Replicas    int32     // Current
    TargetCPU   string    // "80%" or "N/A"
    Age         time.Duration
    CreatedAt   time.Time
}
```

#### 3. Write Transform Functions
**File**: `internal/k8s/transforms.go`
**Changes**: Add 5 transform functions at end of file (after line 546)

```go
func transformReplicaSet(u *unstructured.Unstructured, common
    commonFields) (any, error) {

    desired, _, _ := unstructured.NestedInt64(u.Object, "spec",
        "replicas")
    current, _, _ := unstructured.NestedInt64(u.Object, "status",
        "replicas")
    ready, _, _ := unstructured.NestedInt64(u.Object, "status",
        "readyReplicas")

    return ReplicaSet{
        Namespace: common.Namespace,
        Name:      common.Name,
        Desired:   int32(desired),
        Current:   int32(current),
        Ready:     int32(ready),
        Age:       common.Age,
        CreatedAt: common.CreatedAt,
    }, nil
}

func transformPVC(u *unstructured.Unstructured, common commonFields)
    (any, error) {

    phase, _, _ := unstructured.NestedString(u.Object, "status",
        "phase")
    volumeName, _, _ := unstructured.NestedString(u.Object, "spec",
        "volumeName")

    // Extract capacity
    capacity := "<none>"
    if phase == "Bound" {
        capacityMap, found, _ := unstructured.NestedMap(u.Object,
            "status", "capacity")
        if found {
            if storage, ok := capacityMap["storage"].(string); ok {
                capacity = storage
            }
        }
    }

    // Extract access modes
    accessModes, _, _ := unstructured.NestedStringSlice(u.Object,
        "spec", "accessModes")
    accessModesStr := strings.Join(accessModes, ",")

    storageClass, _, _ := unstructured.NestedString(u.Object, "spec",
        "storageClassName")

    return PersistentVolumeClaim{
        Namespace:    common.Namespace,
        Name:         common.Name,
        Status:       phase,
        Volume:       volumeName,
        Capacity:     capacity,
        AccessModes:  accessModesStr,
        StorageClass: storageClass,
        Age:          common.Age,
        CreatedAt:    common.CreatedAt,
    }, nil
}

func transformIngress(u *unstructured.Unstructured, common commonFields)
    (any, error) {

    ingressClass, _, _ := unstructured.NestedString(u.Object, "spec",
        "ingressClassName")

    // Extract hosts from rules
    rules, _, _ := unstructured.NestedSlice(u.Object, "spec", "rules")
    hosts := []string{}
    for _, rule := range rules {
        ruleMap, ok := rule.(map[string]any)
        if !ok {
            continue
        }
        if host, _, _ := unstructured.NestedString(ruleMap, "host");
            host != "" {
            hosts = append(hosts, host)
        }
    }
    hostsStr := strings.Join(hosts, ", ")
    if hostsStr == "" {
        hostsStr = "*"
    }

    // Extract load balancer address
    address := "<pending>"
    lbIngress, _, _ := unstructured.NestedSlice(u.Object, "status",
        "loadBalancer", "ingress")
    if len(lbIngress) > 0 {
        if lbMap, ok := lbIngress[0].(map[string]any); ok {
            if ip, _, _ := unstructured.NestedString(lbMap, "ip");
                ip != "" {
                address = ip
            } else if hostname, _, _ := unstructured.NestedString(lbMap,
                "hostname"); hostname != "" {
                address = hostname
            }
        }
    }

    return Ingress{
        Namespace: common.Namespace,
        Name:      common.Name,
        Class:     ingressClass,
        Hosts:     hostsStr,
        Address:   address,
        Ports:     "80, 443", // Simplified - most ingresses use these
        Age:       common.Age,
        CreatedAt: common.CreatedAt,
    }, nil
}

func transformEndpoints(u *unstructured.Unstructured, common
    commonFields) (any, error) {

    // Parse subsets to extract endpoints (IP:port pairs)
    subsets, _, _ := unstructured.NestedSlice(u.Object, "subsets")
    endpoints := []string{}

    for _, subset := range subsets {
        subsetMap, ok := subset.(map[string]any)
        if !ok {
            continue
        }

        addresses, _, _ := unstructured.NestedSlice(subsetMap,
            "addresses")
        ports, _, _ := unstructured.NestedSlice(subsetMap, "ports")

        for _, addr := range addresses {
            addrMap, ok := addr.(map[string]any)
            if !ok {
                continue
            }
            ip, _, _ := unstructured.NestedString(addrMap, "ip")

            for _, port := range ports {
                portMap, ok := port.(map[string]any)
                if !ok {
                    continue
                }
                portNum, _, _ := unstructured.NestedInt64(portMap,
                    "port")
                endpoints = append(endpoints,
                    fmt.Sprintf("%s:%d", ip, portNum))
            }
        }
    }

    endpointsStr := strings.Join(endpoints, ", ")
    if endpointsStr == "" {
        endpointsStr = "<none>"
    }

    return Endpoints{
        Namespace: common.Namespace,
        Name:      common.Name,
        Endpoints: endpointsStr,
        Age:       common.Age,
        CreatedAt: common.CreatedAt,
    }, nil
}

func transformHPA(u *unstructured.Unstructured, common commonFields)
    (any, error) {

    minReplicas, _, _ := unstructured.NestedInt64(u.Object, "spec",
        "minReplicas")
    maxReplicas, _, _ := unstructured.NestedInt64(u.Object, "spec",
        "maxReplicas")
    currentReplicas, _, _ := unstructured.NestedInt64(u.Object, "status",
        "currentReplicas")

    // Extract scale target reference
    refKind, _, _ := unstructured.NestedString(u.Object, "spec",
        "scaleTargetRef", "kind")
    refName, _, _ := unstructured.NestedString(u.Object, "spec",
        "scaleTargetRef", "name")
    reference := fmt.Sprintf("%s/%s", refKind, refName)

    // Extract target CPU utilization (v2 API)
    targetCPU := "N/A"
    metrics, _, _ := unstructured.NestedSlice(u.Object, "spec",
        "metrics")
    for _, metric := range metrics {
        metricMap, ok := metric.(map[string]any)
        if !ok {
            continue
        }
        metricType, _, _ := unstructured.NestedString(metricMap, "type")
        if metricType == "Resource" {
            resource, _, _ := unstructured.NestedMap(metricMap,
                "resource")
            name, _, _ := unstructured.NestedString(resource, "name")
            if name == "cpu" {
                if target, _, _ := unstructured.NestedInt64(resource,
                    "target", "averageUtilization"); target > 0 {
                    targetCPU = fmt.Sprintf("%d%%", target)
                }
            }
        }
    }

    return HorizontalPodAutoscaler{
        Namespace: common.Namespace,
        Name:      common.Name,
        Reference: reference,
        MinPods:   int32(minReplicas),
        MaxPods:   int32(maxReplicas),
        Replicas:  int32(currentReplicas),
        TargetCPU: targetCPU,
        Age:       common.Age,
        CreatedAt: common.CreatedAt,
    }, nil
}
```

#### 4. Add Registry Entries
**File**: `internal/k8s/transforms.go`
**Changes**: Add 5 entries to `getResourceRegistry()` map at line 546

```go
ResourceTypeReplicaSet: {
    GVR: schema.GroupVersionResource{
        Group: "apps", Version: "v1", Resource: "replicasets"},
    Name:       "ReplicaSets",
    Namespaced: true,
    Tier:       1,
    Transform:  transformReplicaSet,
},
ResourceTypePersistentVolumeClaim: {
    GVR: schema.GroupVersionResource{
        Group: "", Version: "v1", Resource: "persistentvolumeclaims"},
    Name:       "PersistentVolumeClaims",
    Namespaced: true,
    Tier:       1,
    Transform:  transformPVC,
},
ResourceTypeIngress: {
    GVR: schema.GroupVersionResource{
        Group: "networking.k8s.io", Version: "v1",
        Resource: "ingresses"},
    Name:       "Ingresses",
    Namespaced: true,
    Tier:       1,
    Transform:  transformIngress,
},
ResourceTypeEndpoints: {
    GVR: schema.GroupVersionResource{
        Group: "", Version: "v1", Resource: "endpoints"},
    Name:       "Endpoints",
    Namespaced: true,
    Tier:       1,
    Transform:  transformEndpoints,
},
ResourceTypeHPA: {
    GVR: schema.GroupVersionResource{
        Group: "autoscaling", Version: "v2",
        Resource: "horizontalpodautoscalers"},
    Name:       "HorizontalPodAutoscalers",
    Namespaced: true,
    Tier:       1,
    Transform:  transformHPA,
},
```

#### 5. Add Navigation Methods to Repository Interface
**File**: `internal/k8s/repository.go`
**Changes**: Add 3 new methods after line 90

```go
GetPodsForReplicaSet(namespace, name string) ([]Pod, error)
GetReplicaSetsForDeployment(namespace, name string) ([]ReplicaSet,
    error)
GetPodsForPVC(namespace, name string) ([]Pod, error)
```

**Rationale**: ReplicaSet→Pods, Deployment→ReplicaSets (reverse
relationship), PVC→Pods need specialized queries with indexes.
Ingress, Endpoints, HPA use simpler navigation patterns not requiring
new repository methods.

#### 6. Implement Navigation Methods
**File**: `internal/k8s/informer_repository.go`
**Changes**: Add implementations after `GetJobsForCronJob` (around
line 670)

```go
func (r *InformerRepository) GetPodsForReplicaSet(namespace, name
    string) ([]Pod, error) {

    // Get ReplicaSet to extract UID
    rsLister := r.dynamicListers[schema.GroupVersionResource{
        Group: "apps", Version: "v1", Resource: "replicasets"}]
    if rsLister == nil {
        return nil, fmt.Errorf("replicaset informer not initialized")
    }

    rsObj, err := rsLister.ByNamespace(namespace).Get(name)
    if err != nil {
        return nil, fmt.Errorf("replicaset not found: %w", err)
    }

    rsUnstr, ok := rsObj.(*unstructured.Unstructured)
    if !ok {
        return nil, fmt.Errorf("invalid replicaset object")
    }

    // Use existing podsByOwnerUID index
    r.mu.RLock()
    pods := r.podsByOwnerUID[string(rsUnstr.GetUID())]
    r.mu.RUnlock()

    return convertPodsToSlice(pods), nil
}

func (r *InformerRepository) GetReplicaSetsForDeployment(namespace,
    name string) ([]ReplicaSet, error) {

    // Get Deployment to extract UID
    deployLister := r.dynamicListers[schema.GroupVersionResource{
        Group: "apps", Version: "v1", Resource: "deployments"}]
    if deployLister == nil {
        return nil, fmt.Errorf("deployment informer not initialized")
    }

    deployObj, err := deployLister.ByNamespace(namespace).Get(name)
    if err != nil {
        return nil, fmt.Errorf("deployment not found: %w", err)
    }

    deployUnstr, ok := deployObj.(*unstructured.Unstructured)
    if !ok {
        return nil, fmt.Errorf("invalid deployment object")
    }

    // NEW INDEX REQUIRED: replicaSetsByOwnerUID
    r.mu.RLock()
    rsKeys := r.replicaSetsByOwnerUID[string(deployUnstr.GetUID())]
    r.mu.RUnlock()

    // Fetch ReplicaSets by keys
    rsLister := r.dynamicListers[schema.GroupVersionResource{
        Group: "apps", Version: "v1", Resource: "replicasets"}]

    results := []ReplicaSet{}
    for _, key := range rsKeys {
        namespace, name, err := cache.SplitMetaNamespaceKey(key)
        if err != nil {
            continue
        }

        rsObj, err := rsLister.ByNamespace(namespace).Get(name)
        if err != nil {
            continue
        }

        rsUnstr, ok := rsObj.(*unstructured.Unstructured)
        if !ok {
            continue
        }

        common := extractCommonFields(rsUnstr)
        transformed, err := transformReplicaSet(rsUnstr, common)
        if err != nil {
            continue
        }

        results = append(results, transformed.(ReplicaSet))
    }

    return results, nil
}

func (r *InformerRepository) GetPodsForPVC(namespace, name string)
    ([]Pod, error) {

    // NEW INDEX REQUIRED: podsByPVC (namespace/pvcName → pods)
    key := namespace + "/" + name

    r.mu.RLock()
    pods := r.podsByPVC[key]
    r.mu.RUnlock()

    return convertPodsToSlice(pods), nil
}
```

#### 7. Add New Indexes to Repository Struct
**File**: `internal/k8s/informer_repository.go`
**Changes**: Add fields after line 62

```go
replicaSetsByOwnerUID map[string][]string  // deploymentUID → RS keys
podsByPVC             map[string][]*corev1.Pod  // ns/pvcName → pods
```

#### 8. Initialize New Indexes
**File**: `internal/k8s/informer_repository.go`
**Changes**: Initialize in `NewInformerRepository` after line 107

```go
replicaSetsByOwnerUID: make(map[string][]string),
podsByPVC:             make(map[string][]*corev1.Pod),
```

#### 9. Add Index Maintenance Event Handlers
**File**: `internal/k8s/informer_repository.go`
**Changes**: Add after job event handlers (around line 1142)

**ReplicaSet Index Maintenance**:
```go
// ReplicaSet event handlers for replicaSetsByOwnerUID index
rsInformer, ok := dynamicInformers[schema.GroupVersionResource{
    Group: "apps", Version: "v1", Resource: "replicasets"}]
if ok {
    rsInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
        AddFunc: func(obj interface{}) {
            rs, ok := obj.(*unstructured.Unstructured)
            if !ok {
                return
            }

            owners := rs.GetOwnerReferences()
            for _, owner := range owners {
                if owner.Kind == "Deployment" {
                    r.mu.Lock()
                    key, _ := cache.MetaNamespaceKeyFunc(rs)
                    r.replicaSetsByOwnerUID[string(owner.UID)] =
                        append(r.replicaSetsByOwnerUID[string(owner.UID)],
                        key)
                    r.mu.Unlock()
                }
            }
        },
        UpdateFunc: func(oldObj, newObj interface{}) {
            // Owner references are immutable, no update needed
        },
        DeleteFunc: func(obj interface{}) {
            rs, ok := obj.(*unstructured.Unstructured)
            if !ok {
                return
            }

            owners := rs.GetOwnerReferences()
            for _, owner := range owners {
                if owner.Kind == "Deployment" {
                    r.mu.Lock()
                    key, _ := cache.MetaNamespaceKeyFunc(rs)
                    r.replicaSetsByOwnerUID[string(owner.UID)] =
                        removeString(
                            r.replicaSetsByOwnerUID[string(owner.UID)],
                            key)
                    r.mu.Unlock()
                }
            }
        },
    })
}
```

**PVC Index Maintenance** (add to existing pod event handler):
```go
// Modify existing AddFunc in pod event handler (line 887)
// Add PVC volume tracking after existing index updates

// Track PVC volumes
volumes := pod.Spec.Volumes
for _, vol := range volumes {
    if vol.PersistentVolumeClaim != nil {
        pvcKey := pod.Namespace + "/" + vol.PersistentVolumeClaim.ClaimName
        r.podsByPVC[pvcKey] = append(r.podsByPVC[pvcKey], pod)
    }
}
```

```go
// Modify existing DeleteFunc in pod event handler
// Add PVC volume cleanup

volumes := pod.Spec.Volumes
for _, vol := range volumes {
    if vol.PersistentVolumeClaim != nil {
        pvcKey := pod.Namespace + "/" + vol.PersistentVolumeClaim.ClaimName
        r.podsByPVC[pvcKey] = removePod(r.podsByPVC[pvcKey], pod)
    }
}
```

#### 10. Add Navigation Factory Functions
**File**: `internal/screens/navigation.go`
**Changes**: Add after existing factories (line 191)

```go
// navigateToReplicaSetsForDeployment creates handler for Deployment →
// ReplicaSets
func navigateToReplicaSetsForDeployment() NavigationFunc {
    return func(s *ConfigScreen) tea.Cmd {
        resource := s.GetSelectedResource()
        if resource == nil {
            return nil
        }

        namespace, _ := resource["namespace"].(string)
        name, _ := resource["name"].(string)
        if namespace == "" || name == "" {
            return nil
        }

        return func() tea.Msg {
            return types.ScreenSwitchMsg{
                ScreenID: "replicasets",
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
}

// navigateToPodsForPVC creates handler for PVC → Pods
func navigateToPodsForPVC() NavigationFunc {
    return func(s *ConfigScreen) tea.Cmd {
        resource := s.GetSelectedResource()
        if resource == nil {
            return nil
        }

        namespace, _ := resource["namespace"].(string)
        name, _ := resource["name"].(string)
        if namespace == "" || name == "" {
            return nil
        }

        return func() tea.Msg {
            return types.ScreenSwitchMsg{
                ScreenID: "pods",
                FilterContext: &types.FilterContext{
                    Field: "pvc",
                    Value: name,
                    Metadata: map[string]string{
                        "namespace": namespace,
                        "kind":      "PersistentVolumeClaim",
                    },
                },
            }
        }
    }
}

// navigateToServicesForIngress creates handler for Ingress → Services
func navigateToServicesForIngress() NavigationFunc {
    return func(s *ConfigScreen) tea.Cmd {
        resource := s.GetSelectedResource()
        if resource == nil {
            return nil
        }

        namespace, _ := resource["namespace"].(string)
        name, _ := resource["name"].(string)
        if namespace == "" || name == "" {
            return nil
        }

        return func() tea.Msg {
            return types.ScreenSwitchMsg{
                ScreenID: "services",
                FilterContext: &types.FilterContext{
                    Field: "ingress",
                    Value: name,
                    Metadata: map[string]string{
                        "namespace": namespace,
                        "kind":      "Ingress",
                    },
                },
            }
        }
    }
}

// navigateToPodsForEndpoints creates handler for Endpoints → Pods
func navigateToPodsForEndpoints() NavigationFunc {
    return func(s *ConfigScreen) tea.Cmd {
        resource := s.GetSelectedResource()
        if resource == nil {
            return nil
        }

        namespace, _ := resource["namespace"].(string)
        name, _ := resource["name"].(string)
        if namespace == "" || name == "" {
            return nil
        }

        return func() tea.Msg {
            return types.ScreenSwitchMsg{
                ScreenID: "pods",
                FilterContext: &types.FilterContext{
                    Field: "endpoints",
                    Value: name,
                    Metadata: map[string]string{
                        "namespace": namespace,
                        "kind":      "Endpoints",
                    },
                },
            }
        }
    }
}

// navigateToTargetForHPA creates handler for HPA → Deployment/StatefulSet
func navigateToTargetForHPA() NavigationFunc {
    return func(s *ConfigScreen) tea.Cmd {
        resource := s.GetSelectedResource()
        if resource == nil {
            return nil
        }

        namespace, _ := resource["namespace"].(string)
        reference, _ := resource["reference"].(string)
        if namespace == "" || reference == "" {
            return nil
        }

        // Parse "Deployment/nginx" format
        parts := strings.Split(reference, "/")
        if len(parts) != 2 {
            return nil
        }

        kind := parts[0]
        name := parts[1]

        // Map kind to screen ID
        screenID := strings.ToLower(kind) + "s"

        return func() tea.Msg {
            return types.ScreenSwitchMsg{
                ScreenID: screenID,
                FilterContext: &types.FilterContext{
                    Field: "name",
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

#### 11. Create Screen Configs
**File**: `internal/screens/screens.go`
**Changes**: Add 5 screen config functions at end (after line 321)

```go
func GetReplicaSetsScreenConfig() ScreenConfig {
    return ScreenConfig{
        ID:           "replicasets",
        Title:        "ReplicaSets",
        ResourceType: k8s.ResourceTypeReplicaSet,
        Columns: []ColumnConfig{
            {Field: "Namespace", Title: "Namespace", Width: 40},
            {Field: "Name", Title: "Name", Width: 0},
            {Field: "Desired", Title: "Desired", Width: 10},
            {Field: "Current", Title: "Current", Width: 10},
            {Field: "Ready", Title: "Ready", Width: 10},
            {Field: "Age", Title: "Age", Width: 10,
                Format: FormatDuration},
        },
        SearchFields: []string{"Namespace", "Name"},
        Operations: []OperationConfig{
            {ID: "describe", Name: "Describe",
                Description: "Describe selected ReplicaSet",
                Shortcut: "d"},
            {ID: "yaml", Name: "YAML",
                Description: "View YAML",
                Shortcut: "y"},
        },
        NavigationHandler:     navigateToPodsForOwner("ReplicaSet"),
        EnablePeriodicRefresh: true,
        RefreshInterval:       RefreshInterval,
        TrackSelection:        true,
    }
}

func GetPVCsScreenConfig() ScreenConfig {
    return ScreenConfig{
        ID:           "persistentvolumeclaims",
        Title:        "PersistentVolumeClaims",
        ResourceType: k8s.ResourceTypePersistentVolumeClaim,
        Columns: []ColumnConfig{
            {Field: "Namespace", Title: "Namespace", Width: 40},
            {Field: "Name", Title: "Name", Width: 0},
            {Field: "Status", Title: "Status", Width: 12},
            {Field: "Volume", Title: "Volume", Width: 30},
            {Field: "Capacity", Title: "Capacity", Width: 12},
            {Field: "AccessModes", Title: "Access", Width: 12},
            {Field: "StorageClass", Title: "StorageClass", Width: 20},
            {Field: "Age", Title: "Age", Width: 10,
                Format: FormatDuration},
        },
        SearchFields: []string{"Namespace", "Name", "Status",
            "StorageClass"},
        Operations: []OperationConfig{
            {ID: "describe", Name: "Describe",
                Description: "Describe selected PVC",
                Shortcut: "d"},
            {ID: "yaml", Name: "YAML",
                Description: "View YAML",
                Shortcut: "y"},
        },
        NavigationHandler:     navigateToPodsForPVC(),
        EnablePeriodicRefresh: true,
        RefreshInterval:       RefreshInterval,
        TrackSelection:        true,
    }
}

func GetIngressesScreenConfig() ScreenConfig {
    return ScreenConfig{
        ID:           "ingresses",
        Title:        "Ingresses",
        ResourceType: k8s.ResourceTypeIngress,
        Columns: []ColumnConfig{
            {Field: "Namespace", Title: "Namespace", Width: 40},
            {Field: "Name", Title: "Name", Width: 0},
            {Field: "Class", Title: "Class", Width: 20},
            {Field: "Hosts", Title: "Hosts", Width: 40},
            {Field: "Address", Title: "Address", Width: 30},
            {Field: "Ports", Title: "Ports", Width: 12},
            {Field: "Age", Title: "Age", Width: 10,
                Format: FormatDuration},
        },
        SearchFields: []string{"Namespace", "Name", "Hosts", "Address"},
        Operations: []OperationConfig{
            {ID: "describe", Name: "Describe",
                Description: "Describe selected Ingress",
                Shortcut: "d"},
            {ID: "yaml", Name: "YAML",
                Description: "View YAML",
                Shortcut: "y"},
        },
        NavigationHandler:     navigateToServicesForIngress(),
        EnablePeriodicRefresh: true,
        RefreshInterval:       RefreshInterval,
        TrackSelection:        true,
    }
}

func GetEndpointsScreenConfig() ScreenConfig {
    return ScreenConfig{
        ID:           "endpoints",
        Title:        "Endpoints",
        ResourceType: k8s.ResourceTypeEndpoints,
        Columns: []ColumnConfig{
            {Field: "Namespace", Title: "Namespace", Width: 40},
            {Field: "Name", Title: "Name", Width: 30},
            {Field: "Endpoints", Title: "Endpoints", Width: 0},
            {Field: "Age", Title: "Age", Width: 10,
                Format: FormatDuration},
        },
        SearchFields: []string{"Namespace", "Name", "Endpoints"},
        Operations: []OperationConfig{
            {ID: "describe", Name: "Describe",
                Description: "Describe selected Endpoints",
                Shortcut: "d"},
            {ID: "yaml", Name: "YAML",
                Description: "View YAML",
                Shortcut: "y"},
        },
        NavigationHandler:     navigateToPodsForEndpoints(),
        EnablePeriodicRefresh: true,
        RefreshInterval:       RefreshInterval,
        TrackSelection:        true,
    }
}

func GetHPAsScreenConfig() ScreenConfig {
    return ScreenConfig{
        ID:           "horizontalpodautoscalers",
        Title:        "HorizontalPodAutoscalers",
        ResourceType: k8s.ResourceTypeHPA,
        Columns: []ColumnConfig{
            {Field: "Namespace", Title: "Namespace", Width: 40},
            {Field: "Name", Title: "Name", Width: 0},
            {Field: "Reference", Title: "Reference", Width: 35},
            {Field: "MinPods", Title: "Min", Width: 8},
            {Field: "MaxPods", Title: "Max", Width: 8},
            {Field: "Replicas", Title: "Current", Width: 10},
            {Field: "TargetCPU", Title: "Target", Width: 12},
            {Field: "Age", Title: "Age", Width: 10,
                Format: FormatDuration},
        },
        SearchFields: []string{"Namespace", "Name", "Reference"},
        Operations: []OperationConfig{
            {ID: "describe", Name: "Describe",
                Description: "Describe selected HPA",
                Shortcut: "d"},
            {ID: "yaml", Name: "YAML",
                Description: "View YAML",
                Shortcut: "y"},
        },
        NavigationHandler:     navigateToTargetForHPA(),
        EnablePeriodicRefresh: true,
        RefreshInterval:       RefreshInterval,
        TrackSelection:        true,
    }
}
```

#### 12. Register Screens in App
**File**: `internal/app/app.go`
**Changes**: Add 5 registrations after line 67

```go
registry.Register(screens.NewConfigScreen(
    screens.GetReplicaSetsScreenConfig(), repo, theme))
registry.Register(screens.NewConfigScreen(
    screens.GetPVCsScreenConfig(), repo, theme))
registry.Register(screens.NewConfigScreen(
    screens.GetIngressesScreenConfig(), repo, theme))
registry.Register(screens.NewConfigScreen(
    screens.GetEndpointsScreenConfig(), repo, theme))
registry.Register(screens.NewConfigScreen(
    screens.GetHPAsScreenConfig(), repo, theme))
```

#### 13. Add Dummy Repository Support
**File**: `internal/k8s/dummy_repository.go`
**Changes**: Add dummy implementations for new methods

```go
func (r *DummyRepository) GetPodsForReplicaSet(namespace, name string)
    ([]Pod, error) {
    return []Pod{
        {Namespace: namespace, Name: "nginx-abc123-xyz789",
            Status: "Running"},
    }, nil
}

func (r *DummyRepository) GetReplicaSetsForDeployment(namespace, name
    string) ([]ReplicaSet, error) {
    return []ReplicaSet{
        {Namespace: namespace, Name: name + "-abc123", Desired: 3,
            Current: 3, Ready: 3},
    }, nil
}

func (r *DummyRepository) GetPodsForPVC(namespace, name string)
    ([]Pod, error) {
    return []Pod{
        {Namespace: namespace, Name: "app-xyz789", Status: "Running"},
    }, nil
}
```

#### 14. Update FilterContext Handling
**File**: `internal/screens/config.go`
**Changes**: Add new cases to `refreshWithFilterContext()` around
line 390

```go
case "pvc":
    // PVC → Pods
    pods, err = s.repo.GetPodsForPVC(namespace, s.filterContext.Value)
case "ingress":
    // Ingress → Services (no-op, handled by service screen filter)
    return s.repo.GetResources(s.config.ResourceType)
case "endpoints":
    // Endpoints → Pods (same as service selector)
    // Implementation depends on endpoint subset parsing
    pods, err = s.repo.GetPodsForService(namespace,
        s.filterContext.Value)
case "name":
    // HPA → Deployment/StatefulSet (filter by name)
    // Just apply fuzzy filter, no special repository method needed
    return s.repo.GetResources(s.config.ResourceType)
```

#### 15. Add Tests for New Resources
**File**: `internal/screens/screens_test.go`
**Changes**: Add test cases for 5 new screens (table-driven pattern)

```go
// Add to existing TestGetScreenConfigs test
{
    name:   "replicasets screen",
    config: GetReplicaSetsScreenConfig(),
    expectedFields: []string{"ID", "Title", "ResourceType", "Columns",
        "NavigationHandler"},
},
{
    name:   "pvcs screen",
    config: GetPVCsScreenConfig(),
    expectedFields: []string{"ID", "Title", "ResourceType", "Columns",
        "NavigationHandler"},
},
// ... 3 more test cases
```

**File**: `internal/screens/navigation_test.go`
**Changes**: Add tests for new navigation factories

```go
func TestNavigateToReplicaSetsForDeployment(t *testing.T) {
    // Table-driven tests for Deployment → ReplicaSets navigation
}

func TestNavigateToPodsForPVC(t *testing.T) {
    // Table-driven tests for PVC → Pods navigation
}

// ... 3 more test functions
```

**File**: `internal/k8s/transforms_test.go`
**Changes**: Add transform tests for 5 new resource types

```go
func TestTransformReplicaSet(t *testing.T) {
    // Table-driven test with various replica states
}

func TestTransformPVC(t *testing.T) {
    // Test Bound, Pending, Lost states
}

// ... 3 more test functions
```

### Success Criteria

#### Automated Verification:
- [ ] Build succeeds: `make build`
- [ ] All tests pass: `make test`
- [ ] Linting passes: `golangci-lint run`
- [ ] Integration tests pass with envtest

#### Manual Verification:
- [ ] Navigate to all 5 new screens via `:` palette
- [ ] Verify data displays correctly for each resource
- [ ] Test Enter key navigation for each resource type:
  - ReplicaSet → Pods
  - Deployment → ReplicaSets (new relationship)
  - PVC → Pods
  - Ingress → Services
  - Endpoints → Pods
  - HPA → Deployment/StatefulSet
- [ ] Verify fuzzy search works on all 5 screens
- [ ] Test `/yaml` and `/describe` commands on each resource
- [ ] Confirm 1-second refresh updates data in real-time
- [ ] No regressions in existing 11 screens

**Implementation Note**: After completing this phase and all automated
verification passes, pause for manual testing confirmation before
proceeding to Phase 3.

---

## Phase 3: Tier 2 Resources (7 New Resources)

### Overview
Add RBAC and advanced networking resources (ServiceAccounts,
NetworkPolicies, PVs, StorageClasses, PDBs, Roles, RoleBindings)
bringing total from 16 to 23 resources. These enable security auditing
and advanced troubleshooting.

### Changes Required

[Follow same pattern as Phase 2 for each of 7 resources]:

#### Resources to Add:
1. **ServiceAccounts** - Identity for pods, links to RBAC
   - Navigation: ServiceAccount → Pods (default) | RoleBindings
     (Shift+Enter - future)
   - Columns: Namespace, Name, Secrets, Age

2. **NetworkPolicies** - Network isolation rules
   - Navigation: NetworkPolicy → Pods (selector-based)
   - Columns: Namespace, Name, PodSelector, Ingress, Egress, Age

3. **PersistentVolumes** - Cluster-wide storage resources
   - Navigation: PV → PVCs
   - Columns: Name, Capacity, AccessModes, ReclaimPolicy, Status,
     Claim, StorageClass, Age

4. **StorageClasses** - Storage provisioner templates
   - Navigation: StorageClass → PVCs
   - Columns: Name, Provisioner, ReclaimPolicy, VolumeBindingMode, Age

5. **PodDisruptionBudgets** - Availability constraints
   - Navigation: PDB → Pods (selector-based)
   - Columns: Namespace, Name, MinAvailable, MaxUnavailable,
     AllowedDisruptions, CurrentHealthy, DesiredHealthy, Age

6. **Roles** - Namespace-scoped RBAC permissions
   - Navigation: Role → RoleBindings
   - Columns: Namespace, Name, Age

7. **RoleBindings** - Namespace-scoped RBAC assignments
   - Navigation: RoleBinding → Role (default) | ServiceAccount
     (Shift+Enter - future)
   - Columns: Namespace, Name, Role, Subjects, Age

### New Indexes Required:
- `pvcsByPV` - pvName → PVCs
- `pvcsByStorageClass` - storageClassName → PVCs
- `roleBindingsByRole` - namespace/roleName → RoleBindings

### Success Criteria

#### Automated Verification:
- [ ] Build succeeds: `make build`
- [ ] All tests pass: `make test`
- [ ] Test coverage remains >70%

#### Manual Verification:
- [ ] All 7 new screens accessible and functional
- [ ] RBAC screens show correct relationships (Role ↔ RoleBinding)
- [ ] ServiceAccount navigation to Pods works
- [ ] Storage hierarchy navigation (StorageClass → PV → PVC → Pods)
- [ ] Network policies show correct pod selectors
- [ ] PDB shows current vs desired healthy pods

**Implementation Note**: Pause for manual testing after this phase.

---

## Phase 4: Tier 3 Resources (8 New Resources)

### Overview
Add completeness resources (webhooks, legacy, cluster admin features)
bringing total from 23 to 31 resources. This achieves 100% user-facing
resource coverage.

### Changes Required

#### Resources to Add:
1. **MutatingWebhookConfigurations** - Admission control mutations
   - Navigation: None (terminal resource)
   - Columns: Name, Webhooks, Age

2. **ValidatingWebhookConfigurations** - Admission control validation
   - Navigation: None (terminal resource)
   - Columns: Name, Webhooks, Age

3. **ReplicationControllers** - Legacy workload controller
   - Navigation: RC → Pods (same as Deployment)
   - Columns: Namespace, Name, Desired, Current, Ready, Age

4. **ClusterRoles** - Cluster-scoped RBAC permissions
   - Navigation: ClusterRole → ClusterRoleBindings
   - Columns: Name, Age

5. **ClusterRoleBindings** - Cluster-scoped RBAC assignments
   - Navigation: ClusterRoleBinding → ClusterRole (default) |
     ServiceAccount (Shift+Enter - future)
   - Columns: Name, Role, Subjects, Age

6. **IngressClasses** - Ingress controller types
   - Navigation: IngressClass → Ingresses
   - Columns: Name, Controller, Age

7. **VolumeAttachments** - CSI volume attachment details
   - Navigation: VolumeAttachment → PV → PVC → Pods (multi-hop)
   - Columns: Name, Attacher, PV, Node, Attached, Age

8. **VerticalPodAutoscalers** - Resource recommendation
   - Navigation: VPA → Deployment/StatefulSet
   - Columns: Namespace, Name, Mode, Reference, Age

### New Indexes Required:
- `clusterRoleBindingsByRole` - roleName → ClusterRoleBindings
- `ingressesByClass` - className → Ingresses

### Success Criteria

#### Automated Verification:
- [ ] Build succeeds: `make build`
- [ ] All tests pass: `make test`
- [ ] Full 31 resource types registered

#### Manual Verification:
- [ ] All 31 screens accessible via `:` navigation palette
- [ ] Cluster-scoped resources (ClusterRoles, ClusterRoleBindings,
  IngressClasses) work correctly
- [ ] Webhooks display webhook count and age
- [ ] VPA screen shows mode and target reference
- [ ] Legacy ReplicationControllers work same as Deployments
- [ ] Navigation works for all resources with relationships

**Implementation Note**: After this phase, we have complete resource
coverage with all 31 resources supported. They will all appear in the
system resources screen implemented in Phase 1.

---

## Phase 1: System Resources Monitoring Screen

### Overview
Add monitoring infrastructure and create `:system-resources` screen to
track resource counts, memory usage, sync status, and update activity
for current 11 resource types (automatically expanding to 31 as new
resources are added in later phases).

### Changes Required

#### 1. Add Statistics Tracking to Repository
**File**: `internal/k8s/informer_repository.go`
**Changes**: Add statistics fields to struct after line 62

```go
// Statistics tracking
statsLock     sync.RWMutex
resourceStats map[schema.GroupVersionResource]*ResourceStats
```

**Add ResourceStats struct**:
```go
type ResourceStats struct {
    ResourceType  k8s.ResourceType
    Count         int
    LastUpdate    time.Time
    AddEvents     int64
    UpdateEvents  int64
    DeleteEvents  int64
    Synced        bool
    MemoryBytes   int64  // Approximate
}
```

#### 2. Initialize Statistics Map
**File**: `internal/k8s/informer_repository.go`
**Changes**: In `NewInformerRepository` after line 107

```go
resourceStats: make(map[schema.GroupVersionResource]*ResourceStats),
```

#### 3. Track Events in Event Handlers
**File**: `internal/k8s/informer_repository.go`
**Changes**: Modify all informer AddEventHandler calls to track stats

```go
// Add to each AddFunc
r.statsLock.Lock()
if stats, ok := r.resourceStats[gvr]; ok {
    stats.AddEvents++
    stats.LastUpdate = time.Now()
}
r.statsLock.Unlock()

// Add to each UpdateFunc
r.statsLock.Lock()
if stats, ok := r.resourceStats[gvr]; ok {
    stats.UpdateEvents++
    stats.LastUpdate = time.Now()
}
r.statsLock.Unlock()

// Add to each DeleteFunc
r.statsLock.Lock()
if stats, ok := r.resourceStats[gvr]; ok {
    stats.DeleteEvents++
    stats.LastUpdate = time.Now()
}
r.statsLock.Unlock()
```

#### 4. Add Memory Tracking
**File**: `internal/k8s/informer_repository.go`
**Changes**: Add method to calculate approximate memory usage

```go
func (r *InformerRepository) updateMemoryStats() {
    r.statsLock.Lock()
    defer r.statsLock.Unlock()

    for gvr, lister := range r.dynamicListers {
        objs, err := lister.List(labels.Everything())
        if err != nil {
            continue
        }

        stats, ok := r.resourceStats[gvr]
        if !ok {
            continue
        }

        // Approximate: 1KB per resource (conservative estimate)
        stats.Count = len(objs)
        stats.MemoryBytes = int64(len(objs) * 1024)
    }
}
```

#### 5. Add Repository Method for Statistics
**File**: `internal/k8s/repository.go`
**Changes**: Add to Repository interface

```go
GetResourceStats() []ResourceStats
```

**File**: `internal/k8s/informer_repository.go`
**Changes**: Implement method

```go
func (r *InformerRepository) GetResourceStats() []ResourceStats {
    r.updateMemoryStats()  // Refresh counts and memory

    r.statsLock.RLock()
    defer r.statsLock.RUnlock()

    result := make([]ResourceStats, 0, len(r.resourceStats))
    for _, stats := range r.resourceStats {
        result = append(result, *stats)
    }

    // Sort by resource type name
    sort.Slice(result, func(i, j int) bool {
        return result[i].ResourceType < result[j].ResourceType
    })

    return result
}
```

#### 6. Create System Resources Screen Config
**File**: `internal/screens/system.go` (new file)
**Changes**: Create custom screen (not ConfigScreen) for system metrics

```go
package screens

import (
    "fmt"
    "time"

    tea "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/bubbles/table"
    "github.com/renato0307/k1/internal/k8s"
    "github.com/renato0307/k1/internal/types"
    "github.com/renato0307/k1/internal/ui"
)

type SystemScreen struct {
    repo         k8s.Repository
    theme        *ui.Theme
    table        table.Model
    width        int
    height       int
    lastRefresh  time.Time
}

func NewSystemScreen(repo k8s.Repository, theme *ui.Theme)
    *SystemScreen {

    columns := []table.Column{
        {Title: "Resource Type", Width: 30},
        {Title: "Count", Width: 10},
        {Title: "Memory", Width: 12},
        {Title: "Synced", Width: 10},
        {Title: "Adds", Width: 10},
        {Title: "Updates", Width: 10},
        {Title: "Deletes", Width: 10},
        {Title: "Last Update", Width: 20},
    }

    t := table.New(
        table.WithColumns(columns),
        table.WithFocused(true),
        table.WithHeight(20),
    )

    s := theme.ToTableStyles()
    t.SetStyles(s)

    return &SystemScreen{
        repo:  repo,
        theme: theme,
        table: t,
    }
}

func (s *SystemScreen) Init() tea.Cmd {
    return tea.Batch(
        s.Refresh(),
        tea.Tick(time.Second, func(t time.Time) tea.Msg {
            return tickMsg(t)
        }),
    )
}

func (s *SystemScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.WindowSizeMsg:
        s.width = msg.Width
        s.height = msg.Height
        return s, nil

    case tea.KeyMsg:
        switch msg.String() {
        case "q", "ctrl+c":
            return s, tea.Quit
        case "esc":
            return s, func() tea.Msg {
                return types.ScreenSwitchMsg{ScreenID: "pods"}
            }
        }

    case types.RefreshCompleteMsg:
        s.lastRefresh = time.Now()
        return s, nil

    case tickMsg:
        return s, tea.Batch(
            s.Refresh(),
            tea.Tick(time.Second, func(t time.Time) tea.Msg {
                return tickMsg(t)
            }),
        )
    }

    var cmd tea.Cmd
    s.table, cmd = s.table.Update(msg)
    return s, cmd
}

func (s *SystemScreen) View() string {
    return s.table.View()
}

func (s *SystemScreen) Refresh() tea.Cmd {
    return func() tea.Msg {
        start := time.Now()

        stats := s.repo.GetResourceStats()

        rows := make([]table.Row, 0, len(stats))
        for _, stat := range stats {
            syncedStr := "Yes"
            if !stat.Synced {
                syncedStr = "No"
            }

            memoryMB := fmt.Sprintf("%.2f MB",
                float64(stat.MemoryBytes)/1024/1024)

            lastUpdateStr := "Never"
            if !stat.LastUpdate.IsZero() {
                lastUpdateStr = stat.LastUpdate.Format("15:04:05")
            }

            rows = append(rows, table.Row{
                string(stat.ResourceType),
                fmt.Sprintf("%d", stat.Count),
                memoryMB,
                syncedStr,
                fmt.Sprintf("%d", stat.AddEvents),
                fmt.Sprintf("%d", stat.UpdateEvents),
                fmt.Sprintf("%d", stat.DeleteEvents),
                lastUpdateStr,
            })
        }

        s.table.SetRows(rows)

        return types.RefreshCompleteMsg{Duration: time.Since(start)}
    }
}

func (s *SystemScreen) ID() string {
    return "system-resources"
}

func (s *SystemScreen) Title() string {
    return "System Resources"
}

func (s *SystemScreen) HelpText() string {
    return "↑/↓: Navigate | esc: Back to Pods | q: Quit"
}

func (s *SystemScreen) Operations() []types.Operation {
    return []types.Operation{}
}

func (s *SystemScreen) SetSize(width, height int) {
    s.width = width
    s.height = height
    s.table.SetHeight(height - 5)
}

func (s *SystemScreen) GetSelectedResource() map[string]any {
    return nil
}

func (s *SystemScreen) ApplyFilterContext(ctx *types.FilterContext) {
    // No-op for system screen
}

func (s *SystemScreen) GetFilterContext() *types.FilterContext {
    return nil
}
```

#### 7. Register System Screen
**File**: `internal/app/app.go`
**Changes**: Add system screen registration after line 67

```go
registry.Register(screens.NewSystemScreen(repo, theme))
```

#### 8. Add Navigation Command
**File**: `internal/commands/registry.go`
**Changes**: Add system-resources navigation command

```go
{
    Name:        "system-resources",
    Description: "View system resource statistics",
    Category:    CategoryNavigation,
    Execute:     NavigationCommand("system-resources"),
},
```

#### 9. Add Dummy Repository Support
**File**: `internal/k8s/dummy_repository.go`
**Changes**: Add method implementation

```go
func (r *DummyRepository) GetResourceStats() []ResourceStats {
    return []ResourceStats{
        {
            ResourceType: ResourceTypePod,
            Count:        150,
            LastUpdate:   time.Now().Add(-5 * time.Second),
            AddEvents:    10,
            UpdateEvents: 50,
            DeleteEvents: 3,
            Synced:       true,
            MemoryBytes:  153600, // ~150KB
        },
        {
            ResourceType: ResourceTypeDeployment,
            Count:        25,
            LastUpdate:   time.Now().Add(-2 * time.Second),
            AddEvents:    5,
            UpdateEvents: 12,
            DeleteEvents: 1,
            Synced:       true,
            MemoryBytes:  25600,
        },
        // ... more dummy stats for all 31 resources
    }
}
```

#### 10. Add Tests
**File**: `internal/screens/system_test.go` (new file)
**Changes**: Add comprehensive tests

```go
func TestSystemScreenRefresh(t *testing.T) {
    repo := k8s.NewDummyRepository()
    theme := ui.GetTheme("charm")
    screen := NewSystemScreen(repo, theme)

    cmd := screen.Refresh()
    require.NotNil(t, cmd)

    msg := cmd()
    refreshMsg, ok := msg.(types.RefreshCompleteMsg)
    require.True(t, ok)

    assert.Greater(t, refreshMsg.Duration, time.Duration(0))
}

func TestSystemScreenUpdate(t *testing.T) {
    // Test window size, key handling, tick messages
}
```

**File**: `internal/k8s/informer_repository_test.go`
**Changes**: Add statistics tracking tests

```go
func TestResourceStatsTracking(t *testing.T) {
    // Test that stats are updated on add/update/delete events
}
```

### Success Criteria

#### Automated Verification:
- [x] Build succeeds: `make build`
- [x] All tests pass: `make test`
- [x] Test coverage for system screen >70%

#### Manual Verification:
- [ ] Navigate to `:system-resources` screen
- [ ] Verify current 11 resource types listed
- [ ] Check resource counts match actual cluster state
- [ ] Memory usage shows reasonable values (<2MB for 11 resources)
- [ ] Synced column shows "Yes" for all initialized informers
- [ ] Event counts increment in real-time as resources change
- [ ] Last Update timestamps update when resources change
- [ ] Screen refreshes every 1 second automatically
- [ ] ESC key navigates back to Pods screen

**Implementation Note**: This completes the monitoring infrastructure.
As new resources are added in Phases 2-4, they will automatically
appear in this screen. Pause for manual testing before proceeding to
Phase 2.

---

## Testing Strategy

### Unit Tests

**Transform Functions** (`internal/k8s/transforms_test.go`):
- Test each of 20 new transform functions
- Table-driven tests with multiple field combinations
- Edge cases: missing fields, nil values, empty arrays
- Invalid data handling

**Navigation Functions** (`internal/screens/navigation_test.go`):
- Test all new navigation factory functions
- Verify FilterContext fields set correctly
- Test nil resource handling
- Test missing namespace/name handling

**Screen Configs** (`internal/screens/screens_test.go`):
- Verify all 31 screen configs have required fields
- Test that ResourceType matches transform function
- Validate column configurations
- Verify navigation handler wiring

**Repository Methods** (`internal/k8s/informer_repository_test.go`):
- Test new filtered query methods
- Verify index lookups work correctly
- Test event handler index maintenance
- Test statistics tracking updates

**System Screen** (`internal/screens/system_test.go`):
- Test refresh updates statistics
- Test table rendering with various data
- Test key handling and navigation
- Test tick-based auto-refresh

### Integration Tests

**With envtest**:
- Create all 31 resource types in test cluster
- Verify informers sync correctly
- Test filtered queries return correct results
- Verify navigation relationships work end-to-end
- Test statistics accurately reflect cluster state

### Manual Testing Steps

**Phase 1 Testing (System Screen)**:
1. Start k1 with live cluster connection
2. Navigate to `:system-resources`
3. Verify current 11 resources listed with correct counts
4. Create/delete resources and watch counts update
5. Verify memory usage column shows reasonable values
6. Check that event counts increment in real-time
7. Verify Last Update timestamps change when resources modified
8. ESC key returns to pods screen

**Phase 2 Testing (Tier 1 Resources)**:
1. Navigate to each new screen (`:replicasets`, `:pvcs`, `:ingresses`,
   `:endpoints`, `:hpas`)
2. Verify data displays correctly
3. Test Enter key navigation:
   - ReplicaSet → Pods
   - Deployment → ReplicaSets (new)
   - PVC → Pods
   - Ingress → Services
   - Endpoints → Pods
   - HPA → Deployment
4. Test `/yaml` and `/describe` commands
5. Verify fuzzy search works
6. Verify new resources appear in `:system-resources` screen

**Phase 3 Testing (Tier 2 Resources)**:
1. Test RBAC screens (Roles, RoleBindings, ServiceAccounts)
2. Verify Role ↔ RoleBinding relationships
3. Test ServiceAccount → Pods navigation
4. Verify storage hierarchy (StorageClass → PV → PVC → Pods)
5. Test NetworkPolicy and PDB screens
6. Verify new resources appear in `:system-resources` screen

**Phase 4 Testing (Tier 3 Resources)**:
1. Test all cluster-scoped resources (ClusterRole, ClusterRoleBinding,
   IngressClass)
2. Verify webhook screens show webhook count
3. Test VPA and ReplicationController screens
4. Verify all 31 screens accessible via `:` palette
5. Verify all 31 resources appear in `:system-resources` screen

## Performance Considerations

**Informer Memory**:
- Research shows 71 informers on 1000-pod cluster = <5MB
- Current 31 resources should use <3MB total
- Memory NOT a constraint

**Startup Time**:
- Each informer has 5-second individual sync timeout
- Informers sync in parallel, not sequential
- Expected startup: 10-15 seconds for 31 resources
- Acceptable for TUI application

**Query Performance**:
- All queries use in-memory caches (no API calls)
- Indexed lookups are O(1) for owner/node/namespace/volume queries
- Label selector queries are O(n) but on cached data (fast)
- Target: <100ms for all queries on 1000+ resource clusters

**Index Memory Overhead**:
- New indexes: replicaSetsByOwnerUID, podsByPVC, pvcsByPV,
  pvcsByStorageClass, roleBindingsByRole, clusterRoleBindingsByRole
- Each index entry is ~40 bytes (pointer + map overhead)
- Total overhead: ~50KB for 1000-pod cluster (negligible)

**Statistics Tracking**:
- Statistics map: 31 entries × 64 bytes = ~2KB
- Event counters use atomic operations (no mutex contention)
- Memory update runs on-demand (when screen refreshed)
- No background goroutines for tracking

## Migration Notes

**No Migration Required**:
- New resources are additive, no breaking changes
- Existing 11 resources unchanged
- Registry pattern supports arbitrary resource counts
- Backward compatible with existing kubeconfigs

**Graceful Degradation**:
- RBAC errors for new resources won't crash app
- Failed informers removed from dynamicListers map
- Warning printed to stderr, app continues
- System screen shows Synced=No for failed informers

**Configuration**:
- No new CLI flags required
- All resources start eagerly (no lazy loading)
- Same refresh interval (1 second) for all screens
- No persistent storage or configuration files

## References

- Original ticket: `thoughts/shared/tickets/issue_3.md`
- Scaling research:
  `thoughts/shared/research/2025-10-08-scaling-to-71-api-resources.md`
- Config-driven architecture: `design/DDR-07.md`
- Testing architecture: `design/DDR-04.md`
- Navigation patterns: `thoughts/shared/plans/2025-10-07-contextual-navigation.md`
- Existing 11 resource implementations:
  - `internal/k8s/transforms.go:422-546` (registry)
  - `internal/screens/screens.go:10-321` (screen configs)
  - `internal/screens/navigation.go:1-192` (navigation factories)
