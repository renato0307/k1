# Implementation Plan: Deployments and Services Support

**Plan ID:** PLAN-02
**Date:** 2025-10-04
**Related Design:** DDR-03, DDR-04
**Status:** Completed

## Overview

Extend the InformerRepository to support Deployments and Services. This
follows the established pattern from Pods (PLAN-01) with full informer
syncing at startup for simplicity.

## Goals

- Add real-time Deployments and Services data to existing screens
- Sync all resources together at startup (simplified vs tiered loading)
- Maintain <2 second startup time for typical clusters
- Achieve comprehensive test coverage for new resources

## TODO

- [x] Phase 1: Repository Enhancement
- [x] Phase 2: Transformation Logic
- [x] Phase 3: Automated Testing
- [x] Phase 4: Validation

## Major Phases

### 1. Repository Enhancement
Extend `InformerRepository` with Deployment and Service informers:
- Add deployment and service listers alongside existing pod lister
- Sync all three resource types together at startup
- Update WaitForCacheSync to include all three informers

**Key Decision**: All resources sync together at startup (not tiered).
Simpler implementation, still fast enough for typical clusters.

### 2. Transformation Logic
Implement GetDeployments() and GetServices() methods:
- Transform k8s Deployment objects to internal Deployment type
- Transform k8s Service objects to internal Service type
- Extract relevant fields (ready count, replicas, ports, IPs)
- Apply same sorting pattern as Pods (age-based, name secondary)

**Key Insight**: Both resources need full objects (not metadata-only)
for display fields like Ready status, replica counts, and port
configurations.

### 3. Automated Testing
Extend envtest suite to cover new resources:
- Test deployment informer lifecycle and transformation
- Test service informer lifecycle and transformation
- Test different deployment states (fully ready, partial, scaled down)
- Test different service types (ClusterIP, LoadBalancer, NodePort)
- Target >80% coverage across all repository methods

**Key Approach**: Follow established envtest patterns from PLAN-01.
Use shared TestMain, namespace isolation, table-driven tests.

### 4. Validation
Manual testing with real clusters:
- Verify Deployments screen displays real cluster data
- Verify Services screen displays real cluster data
- Test startup time remains <2 seconds
- Validate transformation logic (ready counts, ports formatting)

## Critical Considerations

**Simplicity Over Tiering**: Unlike DDR-03's tiered loading design, this
implementation syncs all resources together for simplicity. Trade-off
accepted: slightly slower startup (still <2s) for less code complexity.

**Transformation Complexity**: Deployments require replica calculation
(ready/desired). Services require port list formatting (80:30080/TCP).

**Testing Realism**: Envtest provides real API server behavior, ensuring
transformations work correctly with actual k8s objects.

## Success Criteria

- Deployments screen displays real cluster data
- Services screen displays real cluster data
- All resources sync at startup in <2 seconds for typical clusters
- Tests pass with >80% coverage on new methods
- Memory usage acceptable (<20MB for typical clusters)

## Future Work (Phase 3+)

- Optimize with tiered loading if startup time becomes issue
- Support additional resources (StatefulSets, DaemonSets)
- Namespace filtering to reduce memory

## References

- Design: `design/DDR-03.md` (Informer repository architecture)
- Design: `design/DDR-04.md` (Testing with envtest)
- Previous Plan: `plans/PLAN-01-20251004-informer-repository.md`
- Repository Interface: `internal/k8s/repository.go`
- Current Implementation: `internal/k8s/informer_repository.go`
