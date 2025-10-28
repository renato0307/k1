# CRD Support Implementation Plan

## Overview

Add support for Custom Resource Definitions (CRDs) to k1, enabling users
to:
1. List and view CRDs in the cluster
2. Navigate to view instances of specific CRDs dynamically
3. Benefit from adaptive pre-loading based on usage patterns
4. Eventually view CRD-specific fields from OpenAPI schema

This extends k1's current 17 hardcoded resource types to support
arbitrary custom resources defined in the cluster.

## Current State Analysis

k1 uses a **config-driven registry pattern** for resources:

**Registry Pattern** (`internal/k8s/transforms.go:646-825`):
- Static map compiled at build time
- 17 hardcoded resource types (Pods, Deployments, Services, etc.)
- Each entry: GVR + name + scope + tier + transform function

**Dynamic Client** (`internal/k8s/informer_repository.go`):
- Already uses `dynamic.Interface` and unstructured resources
- Supports ANY Kubernetes resource type
- Limitation: hardcoded registry, not client capability

**Config-Driven Screens** (`internal/screens/screens.go`):
- All screens use shared `ConfigScreen` implementation
- ScreenConfig defines columns, operations, navigation
- No custom code needed per resource type

**Key Constraint**: All resource types must be known at compile time.

### Key Discoveries:
- Architecture intentionally designed for extensibility
  (research: `thoughts/shared/research/2025-10-08-scaling-to-71-api-resources.md`)
- CRD support identified as "open question" in prior research
- Dynamic client already supports unstructured resources
- Need runtime resource discovery, not architectural changes

## Desired End State

After completing all phases:

1. **CRDs listed as resource type**: Navigate to
   `:customresourcedefinitions` screen
2. **Dynamic CRD instance screens**: Press Enter on a CRD to view its
   instances
3. **Adaptive pre-loading**: Frequently-used CRDs load instantly after
   first use
4. **Schema-aware display**: CRD-specific fields shown in table (Phase
   4)

### Verification:
1. **Automated**: Tests pass for CRD listing, dynamic screens, usage
   tracking
2. **Manual**:
   - Navigate to CRDs screen, see all cluster CRDs
   - Press Enter on a CRD, see its instances
   - After first use, CRD instances load quickly on subsequent startups

## What We're NOT Doing

- **CRD CRUD operations**: No creating/editing CRDs themselves (only
  viewing)
- **CRD instance CRUD**: No creating/editing CR instances initially
  (view/describe/yaml only)
- **Multiple CRD versions**: Only show storage version, not all
  versions
- **CRD validation**: No schema validation when viewing CRs
- **CRD webhooks**: No webhook configuration viewing
- **Cross-cluster CRD sync**: Usage tracking is per-user, not
  per-cluster

## Implementation Approach

**Four-phase incremental approach**:

1. **Phase 1**: Treat CRDs as another resource type (2-4 hours)
   - Low risk, follows existing patterns exactly
   - Provides immediate value (can see what CRDs exist)

2. **Phase 2**: Dynamic CRD instance screens (2-3 days)
   - Medium complexity, new architectural components
   - On-demand screen creation when user navigates
   - Generic transform (namespace/name/age only)

3. **Phase 3**: Usage tracking and adaptive pre-loading (1 day)
   - Config file tracks access patterns
   - Pre-load top N frequently-used CRDs at startup
   - Learns from user behavior over time

4. **Phase 4**: Schema-aware transforms (future, 2-3 days)
   - Parse CRD OpenAPI schema
   - Extract important fields dynamically
   - Generate columns based on schema

**Each phase is independently valuable and testable.**

---

## Phase 1: List CRDs as Resource Type

### Overview
Add CRDs as the 18th resource type using the existing registry
pattern. Users can navigate to `:customresourcedefinitions` screen,
view all CRDs, search/filter, and view YAML/describe.

**Effort**: 2-4 hours | **Risk**: Low | **Value**: Medium

### Changes Required

#### 1. Add CRD Resource Type Constant
**File**: `internal/k8s/repository.go:13-31`

Add constant after line 30:
```go
ResourceTypeCRD ResourceType = "customresourcedefinitions"
```

Update `GetGVRForResourceType()` map (line 35-50):
```go
ResourceTypeCRD: {
    Group:    "apiextensions.k8s.io",
    Version:  "v1",
    Resource: "customresourcedefinitions",
},
```

#### 2. Add CRD Type Definition
**File**: `internal/k8s/repository_types.go`

Add after existing type definitions:
```go
// CustomResourceDefinition represents a CRD in the cluster
type CustomResourceDefinition struct {
    ResourceMetadata
    Group   string // e.g., "cert-manager.io"
    Version string // Storage version, e.g., "v1"
    Kind    string // e.g., "Certificate"
    Scope   string // "Namespaced" or "Cluster"
    Plural  string // e.g., "certificates"
}
```

#### 3. Add CRD Transform Function
**File**: `internal/k8s/transforms.go`

Add before `getResourceRegistry()` (around line 640):
```go
func transformCRD(u *unstructured.Unstructured,
                  common ResourceMetadata) (any, error) {
    // Extract CRD spec fields
    group, _, _ := unstructured.NestedString(u.Object,
                                              "spec", "group")
    kind, _, _ := unstructured.NestedString(u.Object,
                                             "spec", "names", "kind")
    plural, _, _ := unstructured.NestedString(u.Object,
                                               "spec", "names", "plural")
    scope, _, _ := unstructured.NestedString(u.Object,
                                              "spec", "scope")

    // Find storage version
    versions, _, _ := unstructured.NestedSlice(u.Object,
                                                "spec", "versions")
    version := ""
    for _, v := range versions {
        vMap, ok := v.(map[string]any)
        if !ok {
            continue
        }
        stored, _, _ := unstructured.NestedBool(vMap, "storage")
        if stored {
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

#### 4. Register in Resource Registry
**File**: `internal/k8s/transforms.go:646-825`

Add to `getResourceRegistry()` map (alphabetically after other
entries):
```go
ResourceTypeCRD: {
    GVR: schema.GroupVersionResource{
        Group:    "apiextensions.k8s.io",
        Version:  "v1",
        Resource: "customresourcedefinitions",
    },
    Name:       "Custom Resource Definitions",
    Namespaced: false, // CRDs are cluster-scoped
    Tier:       2,     // Background load
    Transform:  transformCRD,
},
```

#### 5. Create CRD Screen Config
**File**: `internal/screens/screens.go`

Add new config function (alphabetically with other configs):
```go
// GetCRDsScreenConfig returns the config for the CRDs screen
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
            {ID: "describe", Name: "Describe",
             Description: "Describe selected CRD", Shortcut: "d"},
            {ID: "yaml", Name: "View YAML",
             Description: "View CRD YAML", Shortcut: "y"},
        },
        // NavigationHandler will be added in Phase 2
    }
}
```

#### 6. Register CRD Screen in App
**File**: `internal/app/app.go:65-100`

Add to screen registration (in Tier 2 section, around line 77):
```go
registry.Register(screens.NewConfigScreen(
    screens.GetCRDsScreenConfig(), repo, theme))
```

#### 7. Add Navigation Command
**File**: `internal/commands/navigation.go`

Add to navigation registry map (around line 23):
```go
"customresourcedefinitions": "customresourcedefinitions",
"crds": "customresourcedefinitions", // Alias
```

#### 8. Create Tests for CRD Transform
**File**: `internal/k8s/transforms_test.go`

Add test cases for `transformCRD`:
```go
func TestTransformCRD(t *testing.T) {
    tests := []struct {
        name     string
        input    *unstructured.Unstructured
        expected CustomResourceDefinition
    }{
        {
            name: "cert-manager certificate CRD",
            input: &unstructured.Unstructured{
                Object: map[string]any{
                    "apiVersion": "apiextensions.k8s.io/v1",
                    "kind":       "CustomResourceDefinition",
                    "metadata": map[string]any{
                        "name": "certificates.cert-manager.io",
                        "creationTimestamp": "2024-01-15T10:30:00Z",
                    },
                    "spec": map[string]any{
                        "group": "cert-manager.io",
                        "names": map[string]any{
                            "kind":   "Certificate",
                            "plural": "certificates",
                        },
                        "scope": "Namespaced",
                        "versions": []any{
                            map[string]any{
                                "name":    "v1",
                                "storage": true,
                            },
                        },
                    },
                },
            },
            expected: CustomResourceDefinition{
                ResourceMetadata: ResourceMetadata{
                    Name: "certificates.cert-manager.io",
                    Age:  // calculated from creationTimestamp
                },
                Group:   "cert-manager.io",
                Version: "v1",
                Kind:    "Certificate",
                Scope:   "Namespaced",
                Plural:  "certificates",
            },
        },
        // Add test for cluster-scoped CRD
        // Add test for multiple versions (storage vs non-storage)
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            common := extractCommonFields(tt.input)
            result, err := transformCRD(tt.input, common)

            assert.NoError(t, err)
            crd, ok := result.(CustomResourceDefinition)
            assert.True(t, ok)
            assert.Equal(t, tt.expected.Group, crd.Group)
            assert.Equal(t, tt.expected.Version, crd.Version)
            assert.Equal(t, tt.expected.Kind, crd.Kind)
            assert.Equal(t, tt.expected.Scope, crd.Scope)
            assert.Equal(t, tt.expected.Plural, crd.Plural)
        })
    }
}
```

#### 9. Create Tests for CRD Screen Config
**File**: `internal/screens/screens_test.go`

Add test to verify CRD screen configuration:
```go
func TestGetCRDsScreenConfig(t *testing.T) {
    config := GetCRDsScreenConfig()

    assert.Equal(t, "customresourcedefinitions", config.ID)
    assert.Equal(t, "Custom Resource Definitions", config.Title)
    assert.Equal(t, k8s.ResourceTypeCRD, config.ResourceType)

    // Verify columns
    assert.Len(t, config.Columns, 6)
    assert.Equal(t, "Group", config.Columns[0].Field)
    assert.Equal(t, "Kind", config.Columns[2].Field)

    // Verify operations
    assert.Contains(t, config.Operations,
        OperationConfig{ID: "describe", Name: "Describe",
                        Description: "Describe selected CRD",
                        Shortcut: "d"})
    assert.Contains(t, config.Operations,
        OperationConfig{ID: "yaml", Name: "View YAML",
                        Description: "View CRD YAML", Shortcut: "y"})
}
```

### Success Criteria

#### Automated Verification:
- [x] All existing tests pass: `make test`
- [x] New CRD transform tests pass
- [x] New CRD screen config tests pass
- [x] Code compiles without errors: `make build`
- [ ] No linting errors: `golangci-lint run`

#### Manual Verification:
- [x] Application starts without errors: `./k1`
- [x] Type `:crds` or `:customresourcedefinitions` navigates to CRD
      screen
- [x] CRD screen shows all CRDs in cluster with correct columns
- [x] Filter works on CRD list (type to filter by group/kind/plural)
- [x] Press `d` on a CRD to view describe output
- [x] Press `y` on a CRD to view YAML output
- [x] CRD screen shows "No CRDs found" message if cluster has no CRDs

**Implementation Note**: After completing this phase and all automated
verification passes, pause here for manual confirmation from the human
that the manual testing was successful before proceeding to Phase 2.

**TODO - Technical Debt**: Navigation commands are currently registered
in two places (violates DRY and SRP):
- `internal/commands/navigation.go` (navigationRegistry map)
- `internal/commands/registry.go` (manual Command objects)

**Solution**: Refactor to auto-generate Command objects from
navigationRegistry map in NewRegistry(), eliminating duplication. This
would mean adding a new resource screen only requires updating one
location.

---

## Phase 2: Dynamic CRD Instance Screens

### Overview
Enable navigation from CRD list to view instances of that CRD. When
user presses Enter on a CRD row, dynamically create a screen showing
all instances of that custom resource. Informers are registered
on-demand (lazy loading).

**Effort**: 2-3 days | **Risk**: Medium | **Value**: High

### Architecture Simplification

Unlike the research document's proposal, we **do NOT need a
DynamicResourceManager**. This follows k1's existing patterns:

**How existing resources work**:
- Repository handles informers + data fetching
- Screen configs define display
- Transform functions convert data

**How CRD instances work (same pattern)**:
- Repository handles informers + data fetching (same)
- Screen configs generated at runtime (trivial helper function)
- Generic transform function (factory pattern)

**No manager needed**: Configs are cheap to generate, GVR is constructed
directly from CRD data, Repository already tracks informers in
`dynamicListers` map.

### Changes Required

#### 1. Add Generic Resource Type
**File**: `internal/k8s/repository_types.go`

Add generic type for unknown CRD schemas:
```go
// GenericResource represents a CR instance with unknown schema
type GenericResource struct {
    ResourceMetadata
    Kind string          // CRD Kind (e.g., "Certificate")
    Data map[string]any  // Raw unstructured data for describe/yaml
}
```

#### 2. Add Dynamic Screen Message Type
**File**: `internal/types/types.go`

Add new message type (around line 200):
```go
// DynamicScreenCreateMsg requests creation of dynamic screen for CRD
type DynamicScreenCreateMsg struct {
    CRD any // CustomResourceDefinition instance
}
```

#### 3. Add Generic Transform Factory
**File**: `internal/k8s/transforms.go`

Add factory function for creating generic transforms (around line 640):
```go
// CreateGenericTransform returns a transform function for unknown CRD
// schemas
func CreateGenericTransform(kind string) TransformFunc {
    return func(u *unstructured.Unstructured,
                common ResourceMetadata) (any, error) {
        return GenericResource{
            ResourceMetadata: common,
            Kind:             kind,
            Data:             u.Object,
        }, nil
    }
}
```

#### 4. Add Dynamic Informer Registration to Repository
**File**: `internal/k8s/informer_repository.go`

Add method to support runtime informer registration (around line 600):
```go
// EnsureCRInformer registers informer for CR on-demand if not already
// registered
func (r *InformerRepository) EnsureCRInformer(
    gvr schema.GroupVersionResource) error {

    r.mu.Lock()
    defer r.mu.Unlock()

    // Check if already registered
    if _, exists := r.dynamicListers[gvr]; exists {
        return nil // Already cached
    }

    // Create informer
    informer := r.dynamicFactory.ForResource(gvr)

    // Start factory (safe, idempotent)
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

// GetResourcesByGVR fetches resources using explicit GVR (for dynamic
// CRs)
func (r *InformerRepository) GetResourcesByGVR(
    gvr schema.GroupVersionResource,
    transform TransformFunc) ([]any, error) {

    r.mu.RLock()
    lister, exists := r.dynamicListers[gvr]
    r.mu.RUnlock()

    if !exists {
        return nil, fmt.Errorf("informer not registered for %v", gvr)
    }

    // List resources
    objList, err := lister.List(labels.Everything())
    if err != nil {
        return nil, fmt.Errorf("failed to list %v: %w", gvr, err)
    }

    // Transform to typed objects
    resources := make([]any, 0, len(objList))
    for _, obj := range objList {
        unstr, ok := obj.(*unstructured.Unstructured)
        if !ok {
            continue
        }

        common := extractCommonFields(unstr)
        transformed, err := transform(unstr, common)
        if err != nil {
            continue
        }

        resources = append(resources, transformed)
    }

    return resources, nil
}
```

Update Repository interface in `internal/k8s/repository.go` (around
line 60):
```go
type Repository interface {
    GetResources(resourceType ResourceType) ([]any, error)
    GetResourcesByGVR(gvr schema.GroupVersionResource,
                      transform TransformFunc) ([]any, error)  // New
    EnsureCRInformer(gvr schema.GroupVersionResource) error    // New
    // ... existing methods
}
```

#### 5. Create Dynamic Screen Generator
**File**: `internal/screens/dynamic_screens.go` (new file)

```go
package screens

import (
    "github.com/renato0307/k1/internal/k8s"
)

// GenerateScreenConfigForCR creates a ScreenConfig for CR instances
func GenerateScreenConfigForCR(
    crd k8s.CustomResourceDefinition) ScreenConfig {

    // Generate screen ID: group/plural (handles duplicates)
    screenID := crd.Plural
    if crd.Group != "" {
        screenID = crd.Group + "/" + crd.Plural
    }

    // Basic columns: namespace (if namespaced), name, age
    columns := []ColumnConfig{
        {Field: "Name", Title: "Name", Width: 50, Priority: 1},
        {Field: "Age", Title: "Age", Width: 10,
         Format: FormatDuration, Priority: 1},
    }

    // Add namespace column if CRD is namespaced
    if crd.Scope == "Namespaced" {
        columns = append([]ColumnConfig{
            {Field: "Namespace", Title: "Namespace", Width: 20,
             Priority: 2},
        }, columns...)
    }

    return ScreenConfig{
        ID:    screenID,
        Title: crd.Kind, // e.g., "Certificates"
        // ResourceType is dynamic, not from enum
        Columns:      columns,
        SearchFields: []string{"Namespace", "Name"},
        Operations: []OperationConfig{
            {ID: "describe", Name: "Describe",
             Description: "Describe selected " + crd.Kind,
             Shortcut: "d"},
            {ID: "yaml", Name: "View YAML",
             Description: "View " + crd.Kind + " YAML",
             Shortcut: "y"},
        },
    }
}
```

#### 6. Create Dynamic Screen Implementation
**File**: `internal/screens/dynamic_screen.go` (new file)

```go
package screens

import (
    tea "github.com/charmbracelet/bubbletea"
    "github.com/renato0307/k1/internal/k8s"
    "github.com/renato0307/k1/internal/types"
    "github.com/renato0307/k1/internal/ui"
    "k8s.io/apimachinery/pkg/runtime/schema"
)

// DynamicScreen wraps ConfigScreen for dynamic CRD instances
type DynamicScreen struct {
    *ConfigScreen
    gvr       schema.GroupVersionResource
    transform k8s.TransformFunc
}

// NewDynamicScreen creates a screen for CR instances
func NewDynamicScreen(
    config ScreenConfig,
    gvr schema.GroupVersionResource,
    transform k8s.TransformFunc,
    repo k8s.Repository,
    theme *ui.Theme) *DynamicScreen {

    baseScreen := NewConfigScreen(config, repo, theme)

    return &DynamicScreen{
        ConfigScreen: baseScreen,
        gvr:          gvr,
        transform:    transform,
    }
}

// Init ensures informer is registered before loading data
func (s *DynamicScreen) Init() tea.Cmd {
    return tea.Batch(
        func() tea.Msg {
            // Register informer on-demand
            if err := s.repo.EnsureCRInformer(s.gvr); err != nil {
                return types.ErrorStatusMsg{
                    Message: "Failed to load " + s.config.Title +
                             ": " + err.Error(),
                }
            }
            return types.InfoStatusMsg{
                Message: "Loading " + s.config.Title + "...",
            }
        },
        s.Refresh(),
    )
}

// Refresh fetches CR instances using GVR
func (s *DynamicScreen) Refresh() tea.Cmd {
    return func() tea.Msg {
        resources, err := s.repo.GetResourcesByGVR(s.gvr, s.transform)
        if err != nil {
            return types.ErrorStatusMsg{
                Message: "Failed to refresh " + s.config.Title +
                         ": " + err.Error(),
            }
        }

        return types.RefreshCompleteMsg{
            Resources: resources,
        }
    }
}
```

#### 7. Add Navigation Handler to CRD Screen
**File**: `internal/screens/navigation.go`

Add navigation handler factory (around line 350):
```go
// navigateToCRInstances creates navigation handler for CRD -> CR
// instances
func navigateToCRInstances() NavigationFunc {
    return func(s *ConfigScreen) tea.Cmd {
        resource := s.GetSelectedResource()
        if resource == nil {
            return nil
        }

        crd, ok := resource.(k8s.CustomResourceDefinition)
        if !ok {
            return nil
        }

        // Trigger dynamic screen creation
        return func() tea.Msg {
            return types.DynamicScreenCreateMsg{
                CRD: crd,
            }
        }
    }
}
```

Update CRD screen config in `internal/screens/screens.go`:
```go
func GetCRDsScreenConfig() ScreenConfig {
    return ScreenConfig{
        // ... existing fields
        NavigationHandler: navigateToCRInstances(), // Add this line
    }
}
```

#### 8. Handle Dynamic Screen Creation in App
**File**: `internal/app/app.go`

Handle `DynamicScreenCreateMsg` in Update() (around line 200):
```go
case types.DynamicScreenCreateMsg:
    // Extract CRD
    crd, ok := msg.CRD.(k8s.CustomResourceDefinition)
    if !ok {
        return m, messages.ErrorCmd("Invalid CRD type")
    }

    // Construct GVR directly from CRD
    gvr := schema.GroupVersionResource{
        Group:    crd.Group,
        Version:  crd.Version,
        Resource: crd.Plural,
    }

    // Generate screen config
    screenConfig := screens.GenerateScreenConfigForCR(crd)

    // Check if screen already registered
    screenID := screenConfig.ID
    if existingScreen, exists := m.registry.Get(screenID); exists {
        // Already exists, just switch to it
        m.currentScreen = existingScreen
        m.header.SetScreenTitle(existingScreen.Title())
        return m, existingScreen.Init()
    }

    // Create generic transform
    transform := k8s.CreateGenericTransform(crd.Kind)

    // Create dynamic screen
    dynamicScreen := screens.NewDynamicScreen(
        screenConfig,
        gvr,
        transform,
        m.repo,
        m.theme,
    )

    // Register new screen
    m.registry.Register(dynamicScreen)

    // Switch to it
    m.currentScreen = dynamicScreen
    m.header.SetScreenTitle(dynamicScreen.Title())

    return m, dynamicScreen.Init()
```

**Note**: Import `k8s.io/apimachinery/pkg/runtime/schema` at top of file.

#### 9. Create Tests for Generic Transform
**File**: `internal/k8s/transforms_test.go`

Add test for generic transform factory:
```go
func TestCreateGenericTransform(t *testing.T) {
    transform := CreateGenericTransform("Certificate")

    input := &unstructured.Unstructured{
        Object: map[string]any{
            "apiVersion": "cert-manager.io/v1",
            "kind":       "Certificate",
            "metadata": map[string]any{
                "name":              "test-cert",
                "namespace":         "default",
                "creationTimestamp": "2024-01-15T10:30:00Z",
            },
            "spec": map[string]any{
                "secretName": "test-secret",
                "issuerRef": map[string]any{
                    "name": "letsencrypt",
                },
            },
        },
    }

    common := extractCommonFields(input)
    result, err := transform(input, common)

    assert.NoError(t, err)
    generic, ok := result.(GenericResource)
    assert.True(t, ok)
    assert.Equal(t, "Certificate", generic.Kind)
    assert.Equal(t, "test-cert", generic.Name)
    assert.NotNil(t, generic.Data)
}
```

#### 10. Create Tests for Dynamic Screen Generator
**File**: `internal/screens/dynamic_screens_test.go` (new file)

```go
package screens

import (
    "testing"

    "github.com/renato0307/k1/internal/k8s"
    "github.com/stretchr/testify/assert"
)

func TestGenerateScreenConfigForCR_Namespaced(t *testing.T) {
    crd := k8s.CustomResourceDefinition{
        Group:   "cert-manager.io",
        Version: "v1",
        Kind:    "Certificate",
        Plural:  "certificates",
        Scope:   "Namespaced",
    }

    config := GenerateScreenConfigForCR(crd)

    assert.Equal(t, "cert-manager.io/certificates", config.ID)
    assert.Equal(t, "Certificate", config.Title)

    // Should have 3 columns: Namespace, Name, Age
    assert.Len(t, config.Columns, 3)
    assert.Equal(t, "Namespace", config.Columns[0].Field)
    assert.Equal(t, "Name", config.Columns[1].Field)
    assert.Equal(t, "Age", config.Columns[2].Field)
}

func TestGenerateScreenConfigForCR_ClusterScoped(t *testing.T) {
    crd := k8s.CustomResourceDefinition{
        Group:   "stable.example.com",
        Version: "v1",
        Kind:    "ClusterWidget",
        Plural:  "clusterwidgets",
        Scope:   "Cluster",
    }

    config := GenerateScreenConfigForCR(crd)

    assert.Equal(t, "stable.example.com/clusterwidgets", config.ID)

    // Should have 2 columns: Name, Age (no Namespace)
    assert.Len(t, config.Columns, 2)
    assert.Equal(t, "Name", config.Columns[0].Field)
    assert.Equal(t, "Age", config.Columns[1].Field)
}
```

#### 11. Create Integration Tests
**File**: `internal/k8s/informer_repository_integration_test.go`

Add test for dynamic informer registration:
```go
func TestInformerRepository_EnsureCRInformer(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test")
    }

    // Setup envtest with CRD
    // ... (use existing envtest setup)

    // Create a test CRD in cluster
    testCRD := &apiextensionsv1.CustomResourceDefinition{
        ObjectMeta: metav1.ObjectMeta{
            Name: "testresources.test.example.com",
        },
        Spec: apiextensionsv1.CustomResourceDefinitionSpec{
            Group: "test.example.com",
            Names: apiextensionsv1.CustomResourceDefinitionNames{
                Kind:   "TestResource",
                Plural: "testresources",
            },
            Scope: apiextensionsv1.NamespaceScoped,
            Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
                {
                    Name:    "v1",
                    Served:  true,
                    Storage: true,
                    Schema: &apiextensionsv1.CustomResourceValidation{
                        OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{
                            Type: "object",
                        },
                    },
                },
            },
        },
    }

    // Install CRD
    _, err := apiExtClient.CustomResourceDefinitions().Create(
        ctx, testCRD, metav1.CreateOptions{})
    assert.NoError(t, err)

    // Wait for CRD to be established
    // ...

    // Test dynamic informer registration
    gvr := schema.GroupVersionResource{
        Group:    "test.example.com",
        Version:  "v1",
        Resource: "testresources",
    }

    err = repo.EnsureCRInformer(gvr)
    assert.NoError(t, err)

    // Verify can list resources (even if empty)
    resources, err := repo.GetResourcesByGVR(gvr,
        func(u *unstructured.Unstructured,
             common ResourceMetadata) (any, error) {
            return GenericResource{
                ResourceMetadata: common,
                Kind:             "TestResource",
                Data:             u.Object,
            }, nil
        })
    assert.NoError(t, err)
    assert.NotNil(t, resources)
}
```

### Success Criteria

#### Automated Verification:
- [ ] All existing tests pass: `make test`
- [ ] New generic transform tests pass
- [ ] New dynamic screen generator tests pass
- [ ] Integration tests pass for dynamic informer registration
- [ ] Code compiles without errors: `make build`
- [ ] No linting errors: `golangci-lint run`

#### Manual Verification:
- [ ] Navigate to `:crds` screen
- [ ] Select a CRD and press Enter
- [ ] Dynamic screen appears showing CR instances
- [ ] Screen title shows CRD Kind (e.g., "Certificates")
- [ ] Table shows Namespace (if applicable), Name, and Age columns
- [ ] Can filter CR instances by name
- [ ] Press `d` to describe a CR instance
- [ ] Press `y` to view CR instance YAML
- [ ] Navigate to different CRD, press Enter, see its instances
- [ ] Return to first CRD instances - should be instant (already
      cached)
- [ ] Loading message appears during initial 10s informer sync

**Implementation Note**: After completing this phase and all automated
verification passes, pause here for manual confirmation from the human
that the manual testing was successful before proceeding to Phase 3.

---

## Phase 3: Usage Tracking & Adaptive Pre-loading

### Overview
Track which CRD instances users access most frequently and pre-load
their informers at startup. Config file stored at
`~/.config/k1/crd-usage.yaml` tracks access patterns. Top N (default
10) CRDs are pre-loaded on subsequent startups for instant access.

**Effort**: 1 day | **Risk**: Low | **Value**: High

### Changes Required

#### 1. Create CRD Usage Config Package
**File**: `internal/config/crd_usage.go` (new file)

```go
package config

import (
    "os"
    "path/filepath"
    "sort"
    "time"

    "gopkg.in/yaml.v3"
    "k8s.io/apimachinery/pkg/runtime/schema"
)

// CRDUsageConfig tracks CRD access patterns
type CRDUsageConfig struct {
    Version     int              `yaml:"version"`
    LastUpdated time.Time        `yaml:"last_updated"`
    MaxPreload  int              `yaml:"max_preload"`
    CRDUsage    []CRDUsageEntry  `yaml:"crd_usage"`
}

// CRDUsageEntry records access stats for one CRD
type CRDUsageEntry struct {
    Group        string    `yaml:"group"`
    Version      string    `yaml:"version"`
    Resource     string    `yaml:"resource"`
    AccessCount  int       `yaml:"access_count"`
    LastAccessed time.Time `yaml:"last_accessed"`
}

// ToGVR converts entry to GroupVersionResource
func (e *CRDUsageEntry) ToGVR() schema.GroupVersionResource {
    return schema.GroupVersionResource{
        Group:    e.Group,
        Version:  e.Version,
        Resource: e.Resource,
    }
}

// GetConfigPath returns path to usage config file
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

// LoadCRDUsage reads usage config from disk
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

// Save writes usage config to disk
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

// RecordAccess updates access count for GVR
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

// GetTopN returns N most-accessed CRDs
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
        // Keep if accessed recently OR frequently
        if entry.LastAccessed.After(cutoff) || entry.AccessCount >= 5 {
            filtered = append(filtered, entry)
        }
    }

    c.CRDUsage = filtered
}
```

#### 2. Integrate Usage Tracking into Repository
**File**: `internal/k8s/informer_repository.go`

Add fields to InformerRepository (around line 47):
```go
type InformerRepository struct {
    // ... existing fields
    crdUsageConfig *config.CRDUsageConfig
    crdUsageMutex  sync.Mutex
}
```

Update initialization in `NewInformerRepositoryWithProgress()` (around
line 140):
```go
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
if err := repo.preloadTopCRDs(progress); err != nil {
    // Log error but don't fail startup
}
```

Add pre-loading method (around line 300):
```go
// preloadTopCRDs loads frequently-used CRDs at startup
func (r *InformerRepository) preloadTopCRDs(
    progress chan<- ProgressUpdate) error {

    // Get top N GVRs from config
    topGVRs := r.crdUsageConfig.GetTopN(r.crdUsageConfig.MaxPreload)

    if len(topGVRs) == 0 {
        return nil // First run, nothing to pre-load
    }

    // Get CRDs from registry (must be loaded first)
    crdConfig, exists := getResourceRegistry()[ResourceTypeCRD]
    if !exists {
        return nil // CRDs not registered yet
    }

    // List available CRDs in cluster
    existingCRDs, err := r.GetResources(ResourceTypeCRD)
    if err != nil {
        return err // Can't verify, skip pre-loading
    }

    // Build map of existing GVRs
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
    for i, gvr := range topGVRs {
        if existingGVRs[gvr] {
            progress <- ProgressUpdate{
                Message: fmt.Sprintf("Pre-loading CR: %s/%s",
                                     gvr.Group, gvr.Resource),
                Current: i + 1,
                Total:   len(topGVRs),
            }

            if err := r.registerCRInformerNoTracking(gvr); err != nil {
                // Log but continue - don't fail for one CRD
                continue
            }
        }
    }

    return nil
}

// registerCRInformerNoTracking registers without usage tracking (for
// pre-loading)
func (r *InformerRepository) registerCRInformerNoTracking(
    gvr schema.GroupVersionResource) error {

    r.mu.Lock()
    defer r.mu.Unlock()

    // Check if already registered
    if _, exists := r.dynamicListers[gvr]; exists {
        return nil
    }

    // Create informer
    informer := r.dynamicFactory.ForResource(gvr)

    // Start factory (safe, idempotent)
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
```

Update `EnsureCRInformer()` to track usage (around line 610):
```go
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

// recordCRDAccess updates usage tracking
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
```

#### 3. Add Command-Line Flag for Max Preload
**File**: `cmd/k1/main.go`

Add flag (around line 30):
```go
var (
    // ... existing flags
    crdPreloadMax = flag.Int("crd-preload-max", 10,
        "Maximum CRDs to pre-load at startup (0 to disable)")
)
```

Use flag when creating repository (around line 80):
```go
// Override max preload if flag provided
if *crdPreloadMax >= 0 {
    // Will be applied when loading config
}
```

Update config loading in repository initialization to respect flag.

#### 4. Create Tests for Usage Config
**File**: `internal/config/crd_usage_test.go` (new file)

```go
package config

import (
    "os"
    "path/filepath"
    "testing"
    "time"

    "github.com/stretchr/testify/assert"
    "k8s.io/apimachinery/pkg/runtime/schema"
)

func TestCRDUsageConfig_RecordAccess(t *testing.T) {
    config := &CRDUsageConfig{
        Version:    1,
        MaxPreload: 10,
        CRDUsage:   []CRDUsageEntry{},
    }

    gvr := schema.GroupVersionResource{
        Group:    "cert-manager.io",
        Version:  "v1",
        Resource: "certificates",
    }

    // First access
    config.RecordAccess(gvr)
    assert.Len(t, config.CRDUsage, 1)
    assert.Equal(t, 1, config.CRDUsage[0].AccessCount)

    // Second access
    config.RecordAccess(gvr)
    assert.Len(t, config.CRDUsage, 1)
    assert.Equal(t, 2, config.CRDUsage[0].AccessCount)
}

func TestCRDUsageConfig_GetTopN(t *testing.T) {
    config := &CRDUsageConfig{
        CRDUsage: []CRDUsageEntry{
            {Group: "a.io", Resource: "as", AccessCount: 10},
            {Group: "b.io", Resource: "bs", AccessCount: 5},
            {Group: "c.io", Resource: "cs", AccessCount: 15},
        },
    }

    top2 := config.GetTopN(2)

    assert.Len(t, top2, 2)
    assert.Equal(t, "c.io", top2[0].Group)    // 15 accesses
    assert.Equal(t, "a.io", top2[1].Group)    // 10 accesses
}

func TestCRDUsageConfig_SaveAndLoad(t *testing.T) {
    // Create temp config dir
    tmpDir := t.TempDir()
    os.Setenv("HOME", tmpDir)

    config := &CRDUsageConfig{
        Version:    1,
        MaxPreload: 5,
        CRDUsage: []CRDUsageEntry{
            {
                Group:        "test.io",
                Resource:     "tests",
                AccessCount:  3,
                LastAccessed: time.Now(),
            },
        },
    }

    // Save
    err := config.Save()
    assert.NoError(t, err)

    // Verify file exists
    path := filepath.Join(tmpDir, ".config", "k1", "crd-usage.yaml")
    _, err = os.Stat(path)
    assert.NoError(t, err)

    // Load
    loaded, err := LoadCRDUsage()
    assert.NoError(t, err)
    assert.Equal(t, 1, loaded.Version)
    assert.Equal(t, 5, loaded.MaxPreload)
    assert.Len(t, loaded.CRDUsage, 1)
    assert.Equal(t, "test.io", loaded.CRDUsage[0].Group)
}

func TestCRDUsageConfig_Cleanup(t *testing.T) {
    now := time.Now()
    old := now.Add(-100 * 24 * time.Hour) // 100 days ago

    config := &CRDUsageConfig{
        CRDUsage: []CRDUsageEntry{
            // Keep: recent access
            {Group: "a.io", AccessCount: 1, LastAccessed: now},
            // Keep: high access count
            {Group: "b.io", AccessCount: 10, LastAccessed: old},
            // Remove: old and low access
            {Group: "c.io", AccessCount: 2, LastAccessed: old},
        },
    }

    config.Cleanup()

    assert.Len(t, config.CRDUsage, 2)
    assert.Equal(t, "a.io", config.CRDUsage[0].Group)
    assert.Equal(t, "b.io", config.CRDUsage[1].Group)
}
```

#### 5. Add Documentation
**File**: `README.md`

Add section on CRD usage tracking:
```markdown
### CRD Usage Tracking

k1 learns which CRDs you access most frequently and pre-loads them at
startup:

- **First startup**: All CRDs load on-demand (10s delay)
- **Subsequent startups**: Top 10 frequently-used CRDs pre-load
  instantly
- **Config file**: `~/.config/k1/crd-usage.yaml`

Customize max pre-loaded CRDs:
```bash
k1 --crd-preload-max 20  # Pre-load top 20
k1 --crd-preload-max 0   # Disable pre-loading (pure lazy-loading)
```

Config file format:
```yaml
version: 1
max_preload: 10
crd_usage:
  - group: cert-manager.io
    version: v1
    resource: certificates
    access_count: 45
    last_accessed: 2025-10-28T10:30:00Z
```

Old entries (>90 days, <5 accesses) are automatically cleaned up.
```

### Success Criteria

#### Automated Verification:
- [ ] All existing tests pass: `make test`
- [ ] New config package tests pass
- [ ] Config save/load works correctly
- [ ] GetTopN sorting works correctly
- [ ] Cleanup removes old entries
- [ ] Code compiles without errors: `make build`
- [ ] No linting errors: `golangci-lint run`

#### Manual Verification:
- [ ] First startup with new cluster: no config file exists
- [ ] Navigate to a CRD instances screen (10s load time)
- [ ] Check config file exists at `~/.config/k1/crd-usage.yaml`
- [ ] Config file has 1 entry with access_count=1
- [ ] Navigate to same CRD again: instant (already cached)
- [ ] Restart k1: pre-loading message shows for that CRD
- [ ] Second startup: CRD instances load instantly (<1s)
- [ ] Navigate to 5 different CRDs during session
- [ ] Restart k1: all 5 CRDs pre-load at startup
- [ ] Test with `--crd-preload-max 0`: no pre-loading occurs
- [ ] Test with `--crd-preload-max 2`: only top 2 pre-load

**Implementation Note**: After completing this phase and all automated
verification passes, pause here for manual confirmation from the human
that the manual testing was successful before proceeding to Phase 4.

---

## Phase 4: Schema-Aware Transforms (Future Enhancement)

### Overview
Parse CRD OpenAPI v3 schema to extract important fields and generate
meaningful table columns dynamically. Instead of only showing
namespace/name/age, display CRD-specific fields like spec.replicas,
status.phase, etc.

**Effort**: 2-3 days | **Risk**: Medium | **Value**: High

**Note**: This phase is marked as future work. Implement only when
users request richer CRD instance displays.

### Proposed Approach

1. **Schema Extraction**: Parse `spec.versions[].schema.openAPIV3Schema`
   from CRD
2. **Field Prioritization**: Identify important fields (common
   patterns: `spec.replicas`, `status.phase`, `status.conditions`)
3. **Dynamic Column Generation**: Generate ColumnConfig based on schema
   fields
4. **Type-Aware Formatting**: Format columns based on OpenAPI types
   (number, boolean, etc.)

### Changes Required (High-Level)

#### 1. Schema Parser
**File**: `internal/k8s/schema_parser.go` (new file)

- Parse OpenAPI v3 schema from CRD
- Extract top-level spec/status fields
- Identify field types and descriptions
- Prioritize fields (heuristics: replicas, phase, conditions, ready)

#### 2. Enhanced Dynamic Transform
Update `internal/k8s/dynamic_resources.go`:

- Extract schema-identified fields from unstructured object
- Create typed struct with dynamic fields
- Handle nested fields (e.g., `status.conditions[0].status`)

#### 3. Enhanced Screen Generator
Update `internal/screens/dynamic_screens.go`:

- Generate columns from schema fields
- Add appropriate formatters (FormatNumber, FormatBool, etc.)
- Include field descriptions in help text

### Success Criteria

#### Automated Verification:
- [ ] Schema parser tests pass
- [ ] Field extraction tests pass
- [ ] Dynamic column generation tests pass

#### Manual Verification:
- [ ] Navigate to Certificate CR instances
- [ ] Table shows: Namespace, Name, Ready, Issuer, Status, Age
- [ ] Navigate to VirtualService CR instances
- [ ] Table shows: Namespace, Name, Hosts, Gateways, Age
- [ ] Complex nested fields display correctly
- [ ] Fallback to basic columns if schema parsing fails

**Implementation Note**: This phase should be tackled only when basic
CRD support is stable and users request more detailed views.

---

## Testing Strategy

### Unit Tests
- Transform functions for each phase
- Config-driven screen generation
- Usage tracking logic
- Schema parsing (Phase 4)

### Integration Tests
- Dynamic informer registration with envtest
- CRD creation and instance listing
- Usage config persistence
- Pre-loading verification

### Manual Testing Steps
1. **Phase 1**:
   - Install k1 in cluster with CRDs (e.g., cert-manager)
   - Verify CRDs screen shows all CRDs
   - Test filtering, describe, YAML viewing

2. **Phase 2**:
   - Navigate to Certificate CRD
   - Press Enter to view instances
   - Verify table shows namespaced resources correctly
   - Test with cluster-scoped CRD

3. **Phase 3**:
   - Navigate to 5 different CRD instance screens
   - Check usage config file has 5 entries
   - Restart k1, verify top 5 pre-load
   - Test with different --crd-preload-max values

4. **Phase 4**:
   - Verify Certificate instances show issuer, status fields
   - Compare with kubectl output for accuracy

### Test Coverage Targets
- **Phase 1**: 75% coverage (transforms, screen config)
- **Phase 2**: 70% coverage (dynamic screens, manager)
- **Phase 3**: 80% coverage (config logic, ranking)
- **Phase 4**: 70% coverage (schema parsing)

## Performance Considerations

### Memory Usage
- Each informer caches all resources in memory
- Pre-loading 10 CRDs × 100 instances each = ~1MB memory
- Large clusters (50+ CRDs): use lower max_preload value
- Future: LRU cache with eviction if memory becomes concern

### Startup Time
- Tier 2 CRD resource: loads in background (~2s)
- Pre-loading 10 CRDs: adds 5-10s to startup (parallelized)
- Adaptive: only frequently-used CRDs pre-load

### Runtime Performance
- On-demand informer registration: 10s first access
- Cached informers: instant subsequent access
- Usage config save: async, non-blocking

## Migration Notes

### From No CRD Support
- Existing installations: no migration needed
- Usage config created on first CRD access
- No breaking changes to existing resource types

### Config File
- First run: empty config auto-created
- Config file location: `~/.config/k1/crd-usage.yaml`
- Safe to delete: will recreate on next run

### Backwards Compatibility
- All phases maintain existing resource support
- No changes to existing screens or navigation
- Command-line flags are optional

## References

- Original research:
  `thoughts/shared/research/2025-10-28-crd-support-research.md`
- Related extensibility research:
  `thoughts/shared/research/2025-10-08-scaling-to-71-api-resources.md`
- Registry pattern implementation:
  `thoughts/shared/plans/2025-10-08-issue-3-scale-to-31-resources.md`
- Config-driven navigation:
  `thoughts/shared/research/2025-10-07-contextual-navigation.md`
- Kubernetes dynamic client docs:
  https://pkg.go.dev/k8s.io/client-go/dynamic
- CRD API reference:
  https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#customresourcedefinition-v1-apiextensions-k8s-io
