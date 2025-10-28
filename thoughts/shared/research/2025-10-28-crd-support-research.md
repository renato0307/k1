---
date: 2025-10-28T07:12:20+00:00
researcher: claude
git_commit: 7206c20452f397727d32d009cc2183212f045b13
branch: feat/kubernetes-context-management
repository: k1
topic: "Supporting Custom Resource Definitions (CRDs)"
tags: [research, codebase, crds, extensibility, dynamic-resources,
       kubernetes-api]
status: complete
last_updated: 2025-10-28
last_updated_by: claude
last_updated_note: "Added adaptive hybrid caching strategy (usage-based
                    pre-loading)"
---

# Research: Supporting Custom Resource Definitions (CRDs)

**Date**: 2025-10-28T07:12:20+00:00
**Researcher**: claude
**Git Commit**: 7206c20452f397727d32d009cc2183212f045b13
**Branch**: feat/kubernetes-context-management
**Repository**: k1

## Research Question

How can k1 support Custom Resource Definitions (CRDs)?
1. List CRDs (CustomResourceDefinition resources)
2. From the list of CRDs, dynamically list instances of specific CRDs

## Summary

k1 currently has **no CRD support** but has an excellent architectural
foundation for adding it. The codebase uses a **config-driven registry
pattern** where resources are defined through `ResourceConfig` entries
with GVR (GroupVersionResource), transform functions, and display
configurations.

**Key findings**:
- Current resource registry is **hardcoded** at compile time (17 types)
- Architecture supports adding CRD listing as just another resource type
- Dynamic CRD instance listing requires **runtime resource discovery**
- Would need to query cluster for available CRDs and generate configs
  dynamically
- All necessary building blocks exist: dynamic client, unstructured
  transforms, config-driven screens

## Detailed Findings

### Current Resource Architecture

k1 uses a 3-layer architecture for resources:

1. **Backend Layer** (`internal/k8s/`): Resource registry with GVR
   mappings and transform functions
2. **Screen Layer** (`internal/screens/`): Config-driven screens with
   column definitions
3. **App Layer** (`internal/app/`): Screen registration and routing

#### Resource Registry Pattern

**Location**: `internal/k8s/transforms.go:646-825`

Resources are registered in `getResourceRegistry()` map:

```go
func getResourceRegistry() map[ResourceType]ResourceConfig {
    return map[ResourceType]ResourceConfig{
        ResourceTypePod: {
            GVR: schema.GroupVersionResource{
                Group:    "",
                Version:  "v1",
                Resource: "pods",
            },
            Name:       "Pods",
            Namespaced: true,
            Tier:       1, // Critical - block UI startup
            Transform:  transformPod,
        },
        // ... 16 more built-in resource types
    }
}
```

**Key characteristics**:
- Static map compiled into binary
- Each entry: GVR + display name + scope + tier + transform function
- Adding new built-in resource: add registry entry + transform function
- No runtime discovery of available resources

#### Dynamic Client with Unstructured Resources

**Location**: `internal/k8s/informer_repository.go:90-316`

k1 uses Kubernetes dynamic client for extensibility:

```go
// Dynamic client fields (line 47-50)
dynamicClient dynamic.Interface
dynamicFactory dynamicinformer.DynamicSharedInformerFactory
dynamicListers map[schema.GroupVersionResource]cache.GenericLister
```

**How it works**:
1. Creates dynamic informer factory at startup (line 143)
2. Registers informer for each GVR in registry (lines 176-180)
3. Waits for cache sync with timeout (lines 227-237)
4. Lists resources via `GetResources()` using GVR lookup (line 551)
5. Transforms unstructured objects using config.Transform (line 574)

**Key insight**: The dynamic client already supports **any Kubernetes
resource** via unstructured objects. The limitation is the hardcoded
registry, not the client capability.

#### Transform Functions (Unstructured → Typed)

**Location**: `internal/k8s/transforms.go:47-643`

Each resource type has a transform function:

```go
func transformPod(u *unstructured.Unstructured,
                  common ResourceMetadata) (any, error) {
    // Extract fields using unstructured helpers
    status, _, _ := unstructured.NestedString(u.Object, "status",
                                               "phase")

    // Return typed struct
    return Pod{
        ResourceMetadata: common,
        Status:           status,
        // ... other fields
    }, nil
}
```

**Pattern used across all 17 resources**:
- Receives unstructured object + pre-extracted metadata
- Uses `unstructured.NestedString/Int64/Bool/Slice/Map` helpers
- Extracts relevant fields for table display
- Returns typed struct matching screen columns

**For CRDs**: Would need **generic transform** that extracts common
fields (name, namespace, age) plus configurable custom fields based on
CRD schema.

#### Config-Driven Screens

**Location**: `internal/screens/screens.go:11-493`

Screens defined entirely through `ScreenConfig` structs:

```go
func GetPodsScreenConfig() ScreenConfig {
    return ScreenConfig{
        ID:           "pods",
        Title:        "Pods",
        ResourceType: k8s.ResourceTypePod,
        Columns: []ColumnConfig{
            {Field: "Name", Title: "Name", Width: 50, Priority: 1},
            {Field: "Status", Title: "Status", Width: 15, Priority: 1},
            // ... more columns
        },
        SearchFields: []string{"Namespace", "Name", "Status"},
        Operations: []OperationConfig{
            {ID: "logs", Name: "View Logs", Shortcut: "l"},
            // ... more operations
        },
    }
}
```

**Registered in app** (`internal/app/app.go:65-100`):
```go
registry.Register(screens.NewConfigScreen(
    screens.GetPodsScreenConfig(), repo, theme))
```

**All screens share `ConfigScreen` implementation** - no custom code
needed per resource type.

**For CRDs**: Would need to generate `ScreenConfig` dynamically at
runtime based on CRD schema.

### No Existing CRD Support

**Search results**: No mentions of:
- "CRD"
- "CustomResourceDefinition"
- "apiextensions"
- "custom resources"
- "dynamic resource types"

The codebase **only supports the 17 built-in resource types** currently
registered.

### Historical Context: Extensibility Research

**Related research documents**:

#### `thoughts/shared/research/2025-10-08-scaling-to-71-api-resources.md`

Key insights:
- Registry pattern was designed for extensibility
- Adding resources requires no changes to core components
- **Open question noted**: "CRD support - registry currently hardcoded
  to built-in types, no dynamic GVR discovery"
- Identified this exact limitation we're researching

#### `thoughts/shared/plans/2025-10-08-issue-3-scale-to-31-resources.md`

Implementation plan for scaling to 31 Kubernetes resources:
- Uses same registry pattern
- Demonstrates how to add resources (add entry + transform)
- Does not address dynamic/runtime resource discovery

#### `thoughts/shared/research/2025-10-08-issue-3-implementation-challenges.md`

Analysis of extensibility patterns:
- Replaced type switches with interface pattern for better extensibility
- Config-driven architecture enables Open/Closed Principle
- Current architecture prepared for expansion

**Conclusion from historical research**: The architecture was
intentionally designed for extensibility, but CRD support was deferred
as an "open question."

## Architecture for CRD Support

### Phase 1: List CRDs (Simpler)

**Treat CRDs as a resource type like any other.**

#### Add CRD Resource Type

**Location**: `internal/k8s/repository.go:13-30`

Add constant:
```go
const ResourceTypeCRD ResourceType = "customresourcedefinitions"
```

#### Add CRD Type Definition

**Location**: `internal/k8s/repository_types.go`

```go
type CustomResourceDefinition struct {
    ResourceMetadata
    Group    string  // e.g., "example.com"
    Version  string  // e.g., "v1"
    Kind     string  // e.g., "MyResource"
    Scope    string  // "Namespaced" or "Cluster"
    Plural   string  // e.g., "myresources"
}
```

#### Add Transform Function

**Location**: `internal/k8s/transforms.go`

```go
func transformCRD(u *unstructured.Unstructured,
                  common ResourceMetadata) (any, error) {
    group, _, _ := unstructured.NestedString(u.Object,
                                              "spec", "group")
    kind, _, _ := unstructured.NestedString(u.Object,
                                             "spec", "names", "kind")
    plural, _, _ := unstructured.NestedString(u.Object,
                                               "spec", "names",
                                               "plural")
    scope, _, _ := unstructured.NestedString(u.Object,
                                              "spec", "scope")

    // Extract preferred version
    versions, _, _ := unstructured.NestedSlice(u.Object,
                                                "spec", "versions")
    version := ""
    for _, v := range versions {
        vMap, ok := v.(map[string]any)
        if !ok {
            continue
        }
        if stored, _, _ := unstructured.NestedBool(vMap, "storage");
           stored {
            version, _, _ = unstructured.NestedString(vMap, "name")
            break
        }
    }

    return CustomResourceDefinition{
        ResourceMetadata: common,
        Group:            group,
        Version:          version,
        Kind:             kind,
        Scope:            scope,
        Plural:           plural,
    }, nil
}
```

#### Register in Resource Registry

**Location**: `internal/k8s/transforms.go:646-825`

Add to `getResourceRegistry()`:
```go
ResourceTypeCRD: {
    GVR: schema.GroupVersionResource{
        Group:    "apiextensions.k8s.io",
        Version:  "v1",
        Resource: "customresourcedefinitions",
    },
    Name:       "Custom Resource Definitions",
    Namespaced: false,  // CRDs are cluster-scoped
    Tier:       2,
    Transform:  transformCRD,
},
```

#### Create Screen Config

**Location**: `internal/screens/screens.go`

```go
func GetCRDsScreenConfig() ScreenConfig {
    return ScreenConfig{
        ID:           "customresourcedefinitions",
        Title:        "Custom Resource Definitions",
        ResourceType: k8s.ResourceTypeCRD,
        Columns: []ColumnConfig{
            {Field: "Group", Title: "Group", Width: 30, Priority: 1},
            {Field: "Version", Title: "Version", Width: 10,
             Priority: 2},
            {Field: "Kind", Title: "Kind", Width: 25, Priority: 1},
            {Field: "Plural", Title: "Plural", Width: 25,
             Priority: 1},
            {Field: "Scope", Title: "Scope", Width: 12, Priority: 2},
            {Field: "Age", Title: "Age", Width: 10,
             Format: FormatDuration, Priority: 1},
        },
        SearchFields: []string{"Group", "Kind", "Plural"},
        Operations: []OperationConfig{
            {ID: "describe", Name: "Describe", Shortcut: "d"},
            {ID: "yaml", Name: "View YAML", Shortcut: "y"},
        },
        NavigationHandler: navigateToCRInstances(),  // Phase 2!
    }
}
```

#### Register in App

**Location**: `internal/app/app.go:65-100`

Add to screen registration:
```go
registry.Register(screens.NewConfigScreen(
    screens.GetCRDsScreenConfig(), repo, theme))
```

**Result**: CRDs listed like any other resource. User can navigate to
`:customresourcedefinitions` screen, search/filter CRDs, view YAML,
describe.

**Complexity**: Low - follows existing patterns exactly.

### Phase 2: List CRD Instances (Harder)

**Dynamically create screens for CRD instances at runtime.**

#### Challenge: Dynamic Resource Discovery

Current architecture assumes:
- All resource types known at compile time
- Transform functions are code
- Screen configs are code

For CRD instances:
- Resource types discovered at **runtime** (from CRD list)
- Transform must be **generic** (don't know fields ahead of time)
- Screen config must be **generated** (don't know columns ahead of time)

#### Proposed Architecture

**1. Dynamic Resource Manager** (new component)

**Location**: `internal/k8s/dynamic_resources.go` (new file)

```go
type DynamicResourceManager struct {
    repo            Repository
    crdConfigs      map[string]*DynamicResourceConfig
    registeredGVRs  map[schema.GroupVersionResource]bool
}

type DynamicResourceConfig struct {
    CRD            CustomResourceDefinition
    GVR            schema.GroupVersionResource
    DisplayColumns []string  // extracted from CRD spec
    Transform      TransformFunc
}

// Discover CRDs and create resource configs
func (m *DynamicResourceManager) DiscoverCRDs() error {
    crds, err := m.repo.GetResources(ResourceTypeCRD)
    if err != nil {
        return err
    }

    for _, crd := range crds {
        crdTyped := crd.(CustomResourceDefinition)
        config := m.createConfigFromCRD(crdTyped)
        m.crdConfigs[crdTyped.Plural] = config

        // Register dynamic informer for this GVR
        m.registerDynamicInformer(config.GVR)
    }

    return nil
}

func (m *DynamicResourceManager) createConfigFromCRD(
    crd CustomResourceDefinition) *DynamicResourceConfig {

    gvr := schema.GroupVersionResource{
        Group:    crd.Group,
        Version:  crd.Version,
        Resource: crd.Plural,
    }

    // Generate generic transform that extracts common fields only
    transform := func(u *unstructured.Unstructured,
                      common ResourceMetadata) (any, error) {
        return GenericResource{
            ResourceMetadata: common,
            Kind:             crd.Kind,
            Data:             u.Object,  // Keep raw data for viewing
        }, nil
    }

    return &DynamicResourceConfig{
        CRD:       crd,
        GVR:       gvr,
        Transform: transform,
    }
}
```

**2. Generic Resource Type**

**Location**: `internal/k8s/repository_types.go`

```go
// GenericResource for CRD instances where schema is unknown
type GenericResource struct {
    ResourceMetadata
    Kind string
    Data map[string]any  // Raw unstructured data
}
```

**3. Dynamic Screen Config Generation**

**Location**: `internal/screens/dynamic_screens.go` (new file)

```go
func GenerateScreenConfigForCR(
    crd CustomResourceDefinition,
    manager *DynamicResourceManager) ScreenConfig {

    return ScreenConfig{
        ID:           crd.Plural,  // e.g., "myresources"
        Title:        crd.Kind,    // e.g., "MyResource"
        ResourceType: ResourceType(crd.Plural),
        Columns: []ColumnConfig{
            {Field: "Namespace", Title: "Namespace", Width: 20,
             Priority: 2},
            {Field: "Name", Title: "Name", Width: 40, Priority: 1},
            {Field: "Age", Title: "Age", Width: 10,
             Format: FormatDuration, Priority: 1},
            // Could extract more from CRD schema if needed
        },
        SearchFields: []string{"Namespace", "Name"},
        Operations: []OperationConfig{
            {ID: "describe", Name: "Describe", Shortcut: "d"},
            {ID: "yaml", Name: "View YAML", Shortcut: "y"},
            {ID: "delete", Name: "Delete", Shortcut: "x"},
        },
    }
}
```

**4. Navigation from CRD → Instances**

**Location**: `internal/screens/navigation.go`

```go
func navigateToCRInstances() NavigationFunc {
    return func(s *ConfigScreen) tea.Cmd {
        resource := s.GetSelectedResource()
        if resource == nil {
            return nil
        }

        crd, ok := resource.(CustomResourceDefinition)
        if !ok {
            return nil
        }

        return func() tea.Msg {
            // Trigger dynamic screen creation
            return types.DynamicScreenCreateMsg{
                CRD: crd,
            }
        }
    }
}
```

**5. App Integration**

**Location**: `internal/app/app.go`

Needs to handle `DynamicScreenCreateMsg`:
```go
case types.DynamicScreenCreateMsg:
    // Generate screen config
    config := screens.GenerateScreenConfigForCR(msg.CRD,
                                                 m.dynamicMgr)

    // Create and register screen
    screen := screens.NewConfigScreen(config, m.repo, m.theme)
    m.registry.Register(screen)

    // Switch to it
    m.currentScreen = screen
    return m, m.currentScreen.Init()
```

#### Implementation Challenges

**Challenge 1: Runtime Informer Registration**

Current `InformerRepository` registers all informers at startup. For
CRDs, need to:
- Discover CRDs first
- Register informers dynamically
- Handle informer lifecycle (add/remove as CRDs change)

**Solution**: Add `RegisterDynamicInformer(gvr)` method to repository
that creates and syncs an informer on-demand.

**Challenge 2: Generic Transform vs Schema-Aware**

Two approaches:

**Option A: Generic (simpler)**
- Transform returns `GenericResource` with raw `map[string]any`
- Screen shows only common fields (namespace, name, age)
- YAML/describe view shows full object

**Option B: Schema-Aware (complex)**
- Parse CRD OpenAPI schema
- Extract important fields (e.g., spec.replicas, status.phase)
- Generate columns dynamically
- More useful display but much more complex

**Recommendation**: Start with Option A (generic), add Option B later
if needed.

**Challenge 3: CRD Changes**

What happens when:
- New CRD installed → need to discover and register
- CRD updated → need to refresh config
- CRD deleted → need to unregister screen and informer

**Solution**: Watch CRDs with informer, handle Add/Update/Delete events
to manage dynamic screens.

**Challenge 4: Screen Registry Naming**

Current screen IDs are simple strings ("pods", "deployments"). For CRDs:
- Multiple CRDs could have same plural name in different groups
- Need to namespace screen IDs: `{group}/{plural}` or
  `{group}_{version}_{plural}`

**Solution**: Use `{group}/{plural}` as screen ID for CRD instances.

### Implementation Complexity Comparison

| Phase | Complexity | Effort | Risk | Value |
|-------|-----------|--------|------|-------|
| Phase 1: List CRDs | Low | 2-4 hours | Low | Medium |
| Phase 2: CRD Instances | High | 2-3 days | Medium | High |

**Phase 1** follows existing patterns exactly - just another resource
type.

**Phase 2** requires new architectural components:
- Dynamic resource manager (new)
- Runtime informer registration (modify repository)
- Dynamic screen creation (new message type + handling)
- Screen lifecycle management (new)
- CRD watching for auto-discovery (new)

## Code References

### Key Files for Implementation

**Resource Registry**:
- `internal/k8s/transforms.go:646-825` - Resource registry map
- `internal/k8s/repository.go:10-51` - ResourceType constants and GVR
  mapping
- `internal/k8s/repository_types.go:29-37` - Type definitions

**Transform Functions**:
- `internal/k8s/transforms.go:47-643` - Transform implementations
- `internal/k8s/transforms.go:37-45` - Common metadata extraction

**Dynamic Client**:
- `internal/k8s/informer_repository.go:90-316` - Repository
  initialization
- `internal/k8s/informer_repository.go:47-50` - Dynamic client fields
- `internal/k8s/informer_repository.go:542-599` - GetResources()
  implementation

**Screen Configuration**:
- `internal/screens/screens.go:11-493` - Screen config definitions
- `internal/screens/config.go:43-65` - ScreenConfig struct
- `internal/screens/config.go:90-115` - NewConfigScreen constructor

**Screen Registration**:
- `internal/app/app.go:65-100` - Screen registration at startup
- `internal/types/types.go:35-46` - ScreenRegistry

**Navigation**:
- `internal/screens/navigation.go:12-357` - Navigation handler
  factories
- `internal/types/types.go:189-202` - ScreenSwitchMsg and FilterContext

## Architecture Insights

### Design Strengths for CRD Support

1. **Config-Driven Architecture**: Screens don't contain resource-
   specific logic. Can generate ScreenConfig at runtime.

2. **Dynamic Client**: Already uses unstructured resources. No changes
   needed to support arbitrary resource types.

3. **Transform Pattern**: Clear separation between data fetching and
   transformation. Can create generic transforms for unknown schemas.

4. **Registry Pattern**: Centralized resource configuration. Can extend
   to support runtime registration.

5. **Interface-Based**: Resource interface allows polymorphic handling.
   GenericResource can implement same interface.

### Design Limitations for CRD Support

1. **Static Registration**: Registry populated at compile time.
   Refactor needed for runtime discovery.

2. **No Informer Lifecycle Management**: All informers started at
   startup. Need on-demand creation/destruction.

3. **Hardcoded Navigation**: Navigation palette has fixed list of
   screens. Need to support dynamic screen list.

4. **No Schema Parsing**: Transform functions manually extract fields.
   For CRDs, would need to parse OpenAPI schema or use generic approach.

### Patterns to Follow

When implementing CRD support, follow these established patterns:

1. **Resource Type Definition**: Create CRD type with ResourceMetadata
   embedding
2. **Transform Function**: Extract fields with unstructured helpers,
   return typed struct
3. **Registry Entry**: Add to getResourceRegistry() with GVR +
   transform
4. **Screen Config**: Define columns, search fields, operations
5. **Screen Registration**: Use NewConfigScreen with config
6. **Navigation Handler**: Factory function that returns NavigationFunc

For CRD instances, these patterns need to be **generated at runtime**
instead of written as code.

## Historical Context (from thoughts/)

The k1 project has been progressively improving extensibility:

**2025-10-07**: Implemented contextual navigation with config-driven
approach
- `thoughts/shared/research/2025-10-07-contextual-navigation.md`
- Established pattern of function pointers in configs

**2025-10-08**: Scaled from 3 resources to 17 resources using registry
pattern
- `thoughts/shared/plans/2025-10-08-issue-3-scale-to-31-resources.md`
- Proved registry pattern works for extensibility
- Identified CRD support as open question

**2025-10-08**: Refactored type switches to interface pattern
- `thoughts/shared/plans/2025-10-08-refactor-type-switches-to-interfaces.md`
- Eliminated god file anti-pattern
- Improved Open/Closed Principle compliance

**2025-10-08**: Researched scaling to all 71 Kubernetes API resources
- `thoughts/shared/research/2025-10-08-scaling-to-71-api-resources.md`
- Documented extensibility via registry entries
- Noted CRD support limitation: "registry currently hardcoded to
  built-in types, no dynamic GVR discovery"

**Trend**: Consistent focus on extensibility and config-driven design.
CRD support is the next logical evolution.

## Related Research

- `thoughts/shared/research/2025-10-08-scaling-to-71-api-resources.md`
  - Registry pattern for resource extensibility
- `thoughts/shared/plans/2025-10-08-issue-3-scale-to-31-resources.md`
  - Multi-tier resource loading strategy
- `thoughts/shared/research/2025-10-08-issue-3-implementation-challenges.md`
  - Type switches vs interfaces for extensibility

## Recommended CR Caching Strategy: Adaptive Hybrid

### Strategy D: Adaptive Hybrid (Usage-Based Pre-loading)

**Start with lazy loading, learn from user behavior, pre-load
frequently-used CRDs on subsequent startups.**

#### Architecture Components

**1. CRD Usage Tracking Config**

**Location**: `~/.config/k1/crd-usage.yaml`

```yaml
version: 1
last_updated: 2025-10-28T10:30:00Z
max_preload: 10  # Maximum CRDs to pre-load at startup

crd_usage:
  - group: cert-manager.io
    version: v1
    resource: certificates
    access_count: 45
    last_accessed: 2025-10-28T10:30:00Z

  - group: networking.istio.io
    version: v1beta1
    resource: virtualservices
    access_count: 23
    last_accessed: 2025-10-27T14:20:00Z

  - group: external-secrets.io
    version: v1beta1
    resource: externalsecrets
    access_count: 12
    last_accessed: 2025-10-26T09:15:00Z
```

**2. Config Manager**

**Location**: `internal/config/crd_usage.go` (new file)

```go
package config

import (
    "os"
    "path/filepath"
    "time"

    "gopkg.in/yaml.v3"
    "k8s.io/apimachinery/pkg/runtime/schema"
)

type CRDUsageConfig struct {
    Version     int                `yaml:"version"`
    LastUpdated time.Time          `yaml:"last_updated"`
    MaxPreload  int                `yaml:"max_preload"`
    CRDUsage    []CRDUsageEntry    `yaml:"crd_usage"`
}

type CRDUsageEntry struct {
    Group        string    `yaml:"group"`
    Version      string    `yaml:"version"`
    Resource     string    `yaml:"resource"`
    AccessCount  int       `yaml:"access_count"`
    LastAccessed time.Time `yaml:"last_accessed"`
}

func (e *CRDUsageEntry) ToGVR() schema.GroupVersionResource {
    return schema.GroupVersionResource{
        Group:    e.Group,
        Version:  e.Version,
        Resource: e.Resource,
    }
}

// GetConfigPath returns the config file path
func GetConfigPath() (string, error) {
    home, err := os.UserHomeDir()
    if err != nil {
        return "", err
    }

    configDir := filepath.Join(home, ".config", "k1")
    if err := os.MkdirAll(configDir, 0755); err != nil {
        return "", err
    }

    return filepath.Join(configDir, "crd-usage.yaml"), nil
}

// LoadCRDUsage reads config from disk
func LoadCRDUsage() (*CRDUsageConfig, error) {
    path, err := GetConfigPath()
    if err != nil {
        return nil, err
    }

    data, err := os.ReadFile(path)
    if err != nil {
        if os.IsNotExist(err) {
            // First run - return empty config
            return &CRDUsageConfig{
                Version:    1,
                MaxPreload: 10,
                CRDUsage:   []CRDUsageEntry{},
            }, nil
        }
        return nil, err
    }

    var config CRDUsageConfig
    if err := yaml.Unmarshal(data, &config); err != nil {
        return nil, err
    }

    return &config, nil
}

// SaveCRDUsage writes config to disk
func (c *CRDUsageConfig) Save() error {
    c.LastUpdated = time.Now()

    data, err := yaml.Marshal(c)
    if err != nil {
        return err
    }

    path, err := GetConfigPath()
    if err != nil {
        return err
    }

    return os.WriteFile(path, data, 0644)
}

// RecordAccess updates usage for a GVR
func (c *CRDUsageConfig) RecordAccess(gvr schema.GroupVersionResource) {
    // Find existing entry
    for i := range c.CRDUsage {
        entry := &c.CRDUsage[i]
        if entry.Group == gvr.Group &&
           entry.Version == gvr.Version &&
           entry.Resource == gvr.Resource {
            entry.AccessCount++
            entry.LastAccessed = time.Now()
            return
        }
    }

    // New entry
    c.CRDUsage = append(c.CRDUsage, CRDUsageEntry{
        Group:        gvr.Group,
        Version:      gvr.Version,
        Resource:     gvr.Resource,
        AccessCount:  1,
        LastAccessed: time.Now(),
    })
}

// GetTopN returns the N most-accessed CRDs
func (c *CRDUsageConfig) GetTopN(n int) []schema.GroupVersionResource {
    if n > len(c.CRDUsage) {
        n = len(c.CRDUsage)
    }

    // Sort by access count (descending)
    sorted := make([]CRDUsageEntry, len(c.CRDUsage))
    copy(sorted, c.CRDUsage)

    sort.Slice(sorted, func(i, j int) bool {
        // Primary: access count (higher first)
        if sorted[i].AccessCount != sorted[j].AccessCount {
            return sorted[i].AccessCount > sorted[j].AccessCount
        }
        // Secondary: last accessed (more recent first)
        return sorted[i].LastAccessed.After(sorted[j].LastAccessed)
    })

    result := make([]schema.GroupVersionResource, n)
    for i := 0; i < n; i++ {
        result[i] = sorted[i].ToGVR()
    }

    return result
}

// Cleanup removes entries older than 90 days with low usage
func (c *CRDUsageConfig) Cleanup() {
    cutoff := time.Now().Add(-90 * 24 * time.Hour)
    filtered := []CRDUsageEntry{}

    for _, entry := range c.CRDUsage {
        // Keep if accessed recently OR accessed frequently
        if entry.LastAccessed.After(cutoff) || entry.AccessCount >= 5 {
            filtered = append(filtered, entry)
        }
    }

    c.CRDUsage = filtered
}
```

**3. Repository Integration**

**Location**: `internal/k8s/informer_repository.go` (additions)

```go
// Add to InformerRepository struct
type InformerRepository struct {
    // ... existing fields

    crdUsageConfig *config.CRDUsageConfig
    crdUsageMutex  sync.Mutex
}

// Modified initialization to pre-load top N CRDs
func NewInformerRepositoryWithProgress(...) (*InformerRepository, error) {
    // ... existing initialization code

    // Load CRD usage config
    crdUsageConfig, err := config.LoadCRDUsage()
    if err != nil {
        // Log but don't fail - just skip pre-loading
        crdUsageConfig = &config.CRDUsageConfig{MaxPreload: 10}
    }

    repo := &InformerRepository{
        // ... existing fields
        crdUsageConfig: crdUsageConfig,
    }

    // Pre-load top N CRDs if they exist in cluster
    if err := repo.preloadTopCRDs(); err != nil {
        // Log error but don't fail startup
    }

    return repo, nil
}

// Pre-load frequently-used CRDs at startup
func (r *InformerRepository) preloadTopCRDs() error {
    // Get top N GVRs from config
    topGVRs := r.crdUsageConfig.GetTopN(
        r.crdUsageConfig.MaxPreload)

    if len(topGVRs) == 0 {
        return nil  // First run, nothing to pre-load
    }

    // Verify these CRDs still exist in cluster
    existingCRDs, err := r.GetResources(ResourceTypeCRD)
    if err != nil {
        return err
    }

    existingGVRs := make(map[schema.GroupVersionResource]bool)
    for _, crd := range existingCRDs {
        crdTyped := crd.(CustomResourceDefinition)
        gvr := schema.GroupVersionResource{
            Group:    crdTyped.Group,
            Version:  crdTyped.Version,
            Resource: crdTyped.Plural,
        }
        existingGVRs[gvr] = true
    }

    // Register informers for top CRDs that still exist
    for _, gvr := range topGVRs {
        if existingGVRs[gvr] {
            if err := r.registerCRInformerNoTracking(gvr); err != nil {
                // Log but continue - don't fail for one CRD
                continue
            }
        }
    }

    return nil
}

// EnsureCRInformer registers informer on-demand and tracks usage
func (r *InformerRepository) EnsureCRInformer(
    gvr schema.GroupVersionResource) error {

    r.mu.Lock()
    defer r.mu.Unlock()

    // Check if already registered
    if _, exists := r.dynamicListers[gvr]; exists {
        // Already cached - just record access
        r.recordCRDAccess(gvr)
        return nil
    }

    // Register new informer
    if err := r.registerCRInformerNoTracking(gvr); err != nil {
        return err
    }

    // Record access
    r.recordCRDAccess(gvr)

    return nil
}

// Internal registration without usage tracking (for pre-loading)
func (r *InformerRepository) registerCRInformerNoTracking(
    gvr schema.GroupVersionResource) error {

    // Create informer
    informer := r.dynamicFactory.ForResource(gvr)

    // Start factory again (safe, idempotent)
    r.dynamicFactory.Start(r.ctx.Done())

    // Wait for cache sync with timeout
    syncCtx, cancel := context.WithTimeout(r.ctx, 10*time.Second)
    defer cancel()

    if !cache.WaitForCacheSync(syncCtx.Done(),
                                informer.Informer().HasSynced) {
        return fmt.Errorf("failed to sync cache for %v", gvr)
    }

    // Store lister
    r.dynamicListers[gvr] = r.dynamicFactory.ForResource(gvr).Lister()

    return nil
}

// Record CRD access and persist to config
func (r *InformerRepository) recordCRDAccess(
    gvr schema.GroupVersionResource) {

    r.crdUsageMutex.Lock()
    defer r.crdUsageMutex.Unlock()

    r.crdUsageConfig.RecordAccess(gvr)

    // Save async to avoid blocking
    go func() {
        if err := r.crdUsageConfig.Save(); err != nil {
            // Log error but don't fail
        }
    }()
}

// Cleanup old usage data periodically
func (r *InformerRepository) cleanupCRDUsage() {
    r.crdUsageMutex.Lock()
    defer r.crdUsageMutex.Unlock()

    r.crdUsageConfig.Cleanup()

    go func() {
        if err := r.crdUsageConfig.Save(); err != nil {
            // Log error
        }
    }()
}
```

**4. Screen Integration**

**Location**: `internal/screens/cr_screen.go` (new for CR instances)

```go
func (s *CRScreen) Init() tea.Cmd {
    return tea.Batch(
        s.showLoadingState(),
        func() tea.Msg {
            // Ensure informer exists (registers + tracks usage)
            if err := s.repo.EnsureCRInformer(s.gvr); err != nil {
                return types.ErrorStatusMsg(
                    fmt.Sprintf("Failed to load %s: %v",
                                s.config.Title, err))
            }

            // Refresh data
            return s.Refresh()()
        },
    )
}
```

#### Behavior Over Time

**First Startup** (no config file):
- No CRDs pre-loaded
- User navigates to CRD → 10s loading delay
- Access recorded: `certificates.cert-manager.io: count=1`
- Config saved to `~/.config/k1/crd-usage.yaml`

**After 1 Week** (user regularly views 3 CRDs):
```yaml
crd_usage:
  - {group: cert-manager.io, resource: certificates, access_count: 45}
  - {group: networking.istio.io, resource: virtualservices,
     access_count: 23}
  - {group: external-secrets.io, resource: externalsecrets,
     access_count: 12}
```

**Second Startup**:
- Pre-loads top 3 CRDs at startup (5-10s delay)
- User navigates to pre-loaded CRD → instant (already cached)
- User navigates to new CRD → 10s loading delay
- New CRD tracked for future pre-loading

**After 1 Month** (user views 15 different CRDs):
- Config contains 15 entries
- Top 10 by usage pre-loaded at startup
- Bottom 5 lazy-loaded on-demand

**After 3 Months**:
- Cleanup removes entries >90 days old with access_count < 5
- Config stays lean (only frequently-used CRDs tracked)

#### Configuration Options

Users can customize via config file:

```yaml
max_preload: 20  # Increase to pre-load more CRDs (higher memory)
# Set to 0 to disable pre-loading entirely (pure lazy-loading)
```

Or via command-line flag:
```bash
k1 --crd-preload-max 20
k1 --crd-preload-max 0  # Disable pre-loading
```

#### Pros

- ✅ Fast initial startup (nothing pre-loaded first run)
- ✅ Learns from user behavior (adaptive)
- ✅ Eventually fast for frequently-used CRDs
- ✅ Automatic optimization (no manual configuration)
- ✅ Memory-efficient (bounded by max_preload)
- ✅ Works across different clusters (per-user, not per-cluster)
- ✅ Graceful handling of CRD changes (verifies existence at startup)

#### Cons

- ❌ Complex implementation (config management + usage tracking)
- ❌ First access always slow (inherent to lazy-loading)
- ❌ Config file needs maintenance (cleanup logic)
- ❌ Usage data not shared across users (each user builds own
  profile)

#### Implementation Effort

- **Config Manager**: 2-3 hours (yaml read/write, sorting, cleanup)
- **Repository Integration**: 2-3 hours (pre-loading, usage tracking)
- **Testing**: 2-3 hours (mock config, test ranking, test persistence)
- **Total**: ~1 day

#### Alternative: LRU Cache with Eviction

If memory is a concern with many CRDs, add LRU eviction:

```go
// Add to config
max_cached_crs: 20  # Maximum CR types to keep in memory

// When registering new CRD informer:
if len(r.dynamicListers) >= r.config.MaxCachedCRs {
    // Evict least-recently-used CR informer
    lru := r.findLRUCR()
    r.evictCRInformer(lru)
}
```

This keeps memory bounded even with 100+ CRD types in cluster.

## Open Questions

### For Phase 1 (List CRDs)

**Q1**: Should CRD screen be tier 2 (background) or tier 3 (deferred)?
- **Recommendation**: Tier 2 - Users interested in CRDs will want them
  available quickly, but not critical for basic cluster navigation.

**Q2**: What operations should CRD screen support?
- **Recommendation**: Start with read-only (describe, yaml). Add delete
  in later iteration.

### For Phase 2 (CRD Instances)

**Q3**: When should CRD instance screens be created?
- **Option A**: On-demand when user navigates from CRD screen
- **Option B**: Automatically discover and create at startup
- **Recommendation**: Option A - don't slow startup, create on-demand

**Q4**: How to handle CRD version changes?
- CRDs can have multiple versions (v1alpha1, v1beta1, v1)
- Should we show all versions or just the storage version?
- **Recommendation**: Use storage version (marked in CRD spec), show
  version in screen title

**Q5**: Should generic transforms try to parse spec/status?
- **Option A**: Only show metadata (namespace, name, age)
- **Option B**: Try to extract common patterns (spec.replicas,
  status.phase)
- **Recommendation**: Start with A, iterate to B if users request it

**Q6**: How to handle cluster-scoped vs namespaced CRDs?
- Some CRDs are namespaced, others are cluster-scoped
- **Recommendation**: Use CRD's scope field to determine, filter
  namespace column accordingly

**Q7**: Should we watch CRDs for changes?
- **Recommendation**: Yes - add CRD informer event handler that
  registers/unregisters dynamic screens as CRDs are added/removed

**Q8**: How to test dynamic screen creation?
- Need fake CRD definitions for tests
- Need to test screen generation logic
- **Recommendation**: Create test fixtures with sample CRDs, test
  config generation in isolation

### Performance Considerations

**Q9**: Memory overhead of many CRD instance informers?
- Each informer caches all resources in memory
- Cluster with 50 CRDs × 100 instances each = 5000 objects cached
- **Recommendation**: Implement lazy loading - only create informer when
  user views screen first time, potentially with eviction of unused
  informers after timeout

**Q10**: Startup time impact?
- Don't want to block startup waiting for all CRD instance informers
- **Recommendation**: Make all CRD instance loading tier 3 (deferred),
  load on-demand

## Follow-up Research [2025-10-28T07:15:00Z]

### Question: How to cache CRs from each CRD?

**Answer**: Use adaptive hybrid strategy (Strategy D above).

**Key insight from research**: Kubernetes `DynamicSharedInformerFactory`
supports dynamic informer registration:
- Can call `factory.ForResource(gvr)` AFTER `factory.Start()`
- Calling `Start()` multiple times is safe (idempotent)
- New informers start by calling `Start()` again
- Must call `cache.WaitForCacheSync()` for new informers

**Implementation approach**:
1. **First startup**: Pure lazy-loading (no CRDs pre-loaded)
2. **Track usage**: Record each CR access in
   `~/.config/k1/crd-usage.yaml`
3. **Subsequent startups**: Pre-load top N (e.g., 10) most-accessed CRDs
4. **Memory bounded**: Config file limits max_preload to prevent
   excessive memory usage
5. **Adaptive**: Learns from user behavior over time

**Benefits**:
- Fast initial startup (no pre-loading)
- Eventually fast for frequently-used CRDs
- Automatic optimization (no manual config)
- Memory-efficient (bounded by max_preload)

**Code structure**:
- `internal/config/crd_usage.go` - Config management
- `internal/k8s/informer_repository.go` - Add `EnsureCRInformer()`,
  `preloadTopCRDs()`
- `~/.config/k1/crd-usage.yaml` - Persistent usage tracking

See "Strategy D: Adaptive Hybrid" section above for complete
implementation details.
