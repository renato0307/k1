# PLAN-04: Config-Driven Multi-Resource Architecture

**Status:** Complete - All 5 Phases Implemented and Tested
**Created:** 2025-10-05
**Design:** DDR-07
**Author:** @renato0307

## Goal

Refactor k1 to use a config-driven architecture that reduces code
duplication by 67% and enables rapid addition of new Kubernetes
resources. Replace typed informers with dynamic client and replace
per-resource screen implementations with a generic ConfigScreen driven
by declarative configuration.

## Success Criteria

- [x] Repository supports generic `GetResources(resourceType)` method
- [x] Dynamic informers replace typed informers for all resources
- [x] ConfigScreen handles 90% of list views with zero custom code
- [x] All 3 existing screens migrated to config-driven approach
- [x] At least 3 new resources added (9 total: ConfigMaps, Secrets,
      Namespaces, StatefulSets, DaemonSets, Jobs, CronJobs, Nodes)
- [x] Code reduction: Removed 817 lines, added config-driven screens
- [x] Adding new resource requires ≤60 lines of code (config + transform)
- [x] All tests passing with envtest integration (22/22 k8s, 7/7 screens)
- [x] Performance: resource list queries remain <1ms from cache

## Key Architectural Decisions

1. **Dynamic Client Strategy**
   - Use `k8s.io/client-go/dynamic` for unstructured resource access
   - Keep typed operations available via transform functions
   - Protobuf encoding maintained for performance

2. **Three-Level Customization Model**
   - Level 1 (Pure Config): Standard list views, zero code
   - Level 2 (Config + Overrides): Custom behaviors via function
     pointers
   - Level 3 (Fully Custom): Implement Screen interface directly for
     unique UIs

3. **Tiered Resource Loading**
   - Tier 1 (Pods): Block UI startup, critical for first view
   - Tier 2 (Deployments, Services, etc.): Load in background
   - Tier 3+ (Optional resources): Load on-demand or lazily

4. **Reflection Trade-offs**
   - Accept reflection overhead for config-driven field access
   - Scope limited to filter/display logic, not hot paths
   - Type safety preserved at repository layer via transforms

## Major Phases

### Phase 1: Dynamic Repository Layer
**Goal:** Add dynamic client support alongside existing typed informers

**Outcomes:**
- `ResourceType` enum and `ResourceConfig` registry established
- Dynamic client and informer factory initialized
- Generic `GetResources(resourceType)` method implemented
- Transform functions for Pods, Deployments, Services
- Typed convenience methods preserved (GetPods, etc.)
- Existing screens continue working unchanged

**Key Files:** `internal/k8s/repository.go`,
`internal/k8s/informer_repository.go`

### Phase 2: ConfigScreen Foundation
**Goal:** Create generic screen implementation driven by configuration

**Outcomes:**
- `ScreenConfig` struct defines screen behavior declaratively
- `ConfigScreen` struct implements Screen interface generically
- Reflection-based field access for columns and filtering
- Support for custom overrides via function pointers
- Dynamic column width calculation maintained
- Fuzzy search and negation filtering working

**Key Files:** `internal/screens/config_screen.go`,
`internal/types/types.go`

### Phase 3: Migrate Existing Screens
**Goal:** Replace hand-coded screens with config-driven equivalents

**Outcomes:**
- Deployments migrated to pure config (Level 1)
- Services migrated to config with custom operations (Level 2)
- Pods migrated to config with periodic refresh (Level 2)
- All existing functionality preserved (cursor tracking, refresh, etc.)
- Old screen implementations removed
- Screen registry supports both config and custom screens

**Key Files:** `internal/screens/screen_configs.go`,
`internal/app/app.go`

### Phase 4: Add New Resources
**Goal:** Demonstrate scalability by adding 3+ new resource types

**Outcomes:**
- ConfigMaps (namespaced, Tier 2)
- Secrets (namespaced, Tier 2, sensitive data handling)
- Namespaces (cluster-scoped, Tier 2)
- Each new resource: ~30 line config + ~30 line transform = 60 lines
- Navigation palette includes all new screens
- All resources use tiered loading strategy

**Key Files:** `internal/k8s/informer_repository.go`,
`internal/screens/screen_configs.go`

### Phase 5: Testing and Validation
**Goal:** Ensure reliability and performance with expanded architecture

**Outcomes:**
- Unit tests for transform functions (all resources)
- Unit tests for ConfigScreen (filter, custom overrides)
- Integration tests for dynamic repository (envtest)
- Performance validation: <1ms list queries, <2s initial sync
- Memory footprint measured with 6+ resources loaded
- All existing tests updated and passing

**Key Files:** `internal/k8s/*_test.go`, `internal/screens/*_test.go`

## Risks and Mitigations

**Risk:** Reflection overhead impacts performance
**Mitigation:** Profile hot paths, use reflection only for display
logic, keep cache queries typed

**Risk:** Loss of type safety introduces runtime errors
**Mitigation:** Transform functions provide typed conversion,
comprehensive tests catch field access errors early

**Risk:** ConfigScreen complexity becomes unmaintainable
**Mitigation:** Keep Level 3 escape hatch for truly custom UIs, prefer
composition over flags

**Risk:** Migration breaks existing functionality
**Mitigation:** Maintain typed wrapper methods during transition,
migrate one screen at a time with feature parity validation

## Non-Goals

- CRD discovery (future enhancement)
- Auto-generating configs from OpenAPI schemas (future)
- Plugin system for external screens (future)
- Supporting resources beyond Tier 1-3 in this plan

## Dependencies

- DDR-07 design approved and documented
- Current test suite passing (DDR-04 envtest setup)
- No breaking changes to external APIs (command bar, themes, etc.)

## TODO Progress Tracking

### Phase 1: Dynamic Repository Layer
- [x] Add ResourceType enum and ResourceConfig struct
- [x] Initialize dynamic client alongside typed client
- [x] Implement generic GetResources() method
- [x] Write transform functions (Pods, Deployments, Services)
- [x] Add unit tests for transforms
- [x] Verify existing screens still work (all tests passing)

### Phase 2: ConfigScreen Foundation
- [x] Create ScreenConfig struct definition
- [x] Implement ConfigScreen with core list logic
- [x] Add reflection-based field access (columns, filtering)
- [x] Support custom override function pointers
- [x] Test with sample config (all tests passing)

### Phase 3: Migrate Existing Screens
- [x] Migrate Deployments to pure config (Level 1)
- [x] Migrate Services to pure config (Level 1)
- [x] Migrate Pods to config + periodic refresh (Level 2)
- [x] Remove old screen implementations (-817 lines!)
- [x] Update app to use ConfigScreen with configs

### Phase 4: Add New Resources
- [x] Add ConfigMaps (config + transform)
- [x] Add Secrets (config + transform + sensitive handling)
- [x] Add Namespaces (config + transform + cluster-scoped)
- [x] Add StatefulSets (config + transform)
- [x] Add DaemonSets (config + transform)
- [x] Add Jobs (config + transform)
- [x] Add CronJobs (config + transform)
- [x] Add Nodes (config + transform + cluster-scoped, Tier 3)
- [x] Update navigation palette with all 8 new commands
- [x] Implement tiered loading (Tier 1: Pods, Tier 2: Common, Tier 3:
      Less common)

### Phase 5: Testing and Validation
- [x] Write unit tests for all transforms (GetResources tests cover all
      resources)
- [x] Write unit tests for ConfigScreen (7 tests passing: constructor,
      refresh, filter, negation, selection, duration formatting,
      operations)
- [x] Update integration tests for dynamic repository (fixed
      cluster-scoped resource test isolation)
- [x] Performance testing (informer cache queries remain microsecond-fast)
- [x] All tests passing (22/22 k8s, 7/7 screens)

---

**Note:** Update this TODO list as phases complete. Mark items [x] when
done. Add discovered tasks as implementation progresses.
