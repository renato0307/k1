# Implementation Plan: Kubernetes Informer-Based Repository

**Plan ID:** PLAN-01
**Date:** 2025-10-04
**Related Design:** DDR-03, DDR-04
**Status:** Not Started

## Overview

Replace `DummyRepository` with live Kubernetes data using client-go
informers. Focus on Pods only in Phase 1, with architecture supporting
future multi-resource expansion.

## Goals

- Connect to real Kubernetes cluster and display live pod data
- Achieve 1-2 second startup time
- Foundation for future multi-resource support (Phase 2+)

## TODO

- [ ] Phase 1: Core Repository Implementation
- [ ] Phase 2: Application Integration
- [ ] Phase 3: Automated Testing (envtest)
- [ ] Phase 4: Real-Time Updates
- [ ] Phase 5: Validation

## Major Phases

### 1. Core Repository Implementation
Build `InformerRepository` in `internal/k8s/informer_repository.go`:
- Use client-go SharedInformerFactory with protobuf encoding
- Implement pod lister with transformation logic (ready count, restarts, age)
- Add loading status tracking for future multi-resource support
- Reference POC implementation in `cmd/proto-pods-tui/main.go`

**Key Decision**: Use full pod informers (not metadata-only) to access
status fields needed for display.

### 2. Application Integration
Wire up the repository to main application:
- Add kubeconfig/context flags to `cmd/timoneiro/main.go`
- Block on initial pod sync before starting UI
- Pass live repository to screens instead of dummy data
- Implement graceful shutdown (close informers on exit)

### 3. Automated Testing (envtest)
Set up integration tests with real Kubernetes API:
- Use `sigs.k8s.io/controller-runtime/pkg/envtest`
- Test repository lifecycle (init, sync, query, shutdown)
- Test pod transformation logic (all states: running, pending, failed)
- Target >80% coverage, <10 second test suite runtime

**Key Decision**: Use envtest (real API server) instead of mocks for
higher confidence in client-go integration.

### 4. Real-Time Updates
Enable live data refresh in Pods screen:
- Add 1-second ticker for periodic GetPods() calls
- Implement smart cursor tracking (preserve selection by namespace/name)
- Handle errors gracefully (show message but keep running)

### 5. Validation
Manual testing with local cluster:
- Verify startup time (1-2 seconds target)
- Test error scenarios (bad config, no permissions, network issues)
- Confirm real-time updates work (create/delete pods)
- Verify filter/search with live data

## Critical Considerations

**Performance**: Protobuf encoding reduces network overhead. Informer
cache queries are microsecond-fast once synced.

**Error Handling**: Fail fast on startup errors (bad kubeconfig), but
degrade gracefully during runtime (network blips).

**Future Extensibility**: `LoadingStatus` and `ResourceType` types enable
future parallel loading of multiple resource types without breaking changes.

**Testing Strategy**: Envtest provides real Kubernetes API behavior without
requiring external cluster, enabling CI/CD integration.

## Success Criteria

- Application connects to cluster and displays real pods
- Startup completes in 1-2 seconds for hundreds of pods
- Live updates work (1-second refresh, smart cursor tracking)
- Graceful error handling (no crashes on network/auth failures)
- Automated tests pass with >80% coverage

## Future Work (Phase 2+)

- Parallel background loading for additional resources (Deployments,
  Services, etc.)
- Namespace filtering
- Loading indicators in UI
- Watch-based push updates

## References

- Design: `design/DDR-03.md` (Informer repository architecture)
- Design: `design/DDR-04.md` (Testing strategy with envtest)
- POC: `cmd/proto-pods-tui/main.go`
- Repository interface: `internal/k8s/repository.go`
