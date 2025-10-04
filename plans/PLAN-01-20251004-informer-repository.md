# Implementation Plan: Kubernetes Informer-Based Repository

**Plan ID:** PLAN-01
**Date:** 2025-10-04
**Related Design:** DDR-03
**Status:** In Progress

## Overview

Implement a production-ready Kubernetes repository using informers with local
caching, supporting tiered loading for fast UI startup. Phase 1 focuses on
Pods with priority loading, laying the foundation for multi-resource support.

## Goals

- Replace `DummyRepository` with live Kubernetes data
- Achieve 1-2 second startup time (Pods only)
- Enable future parallel loading of additional resources
- Provide loading status tracking for UI feedback

## Non-Goals (Phase 1)

- Multiple resource types (Deployments, Services, etc.) - Phase 2
- Parallel background loading - Phase 2
- Namespace filtering - Phase 4
- Watch-based push updates - Phase 4

## Prerequisites

- Go 1.24.0+
- Access to Kubernetes cluster (minikube, kind, or real cluster)
- Valid kubeconfig file
- Dependencies already in go.mod:
  - k8s.io/client-go v0.34.1
  - k8s.io/apimachinery v0.34.1

## TODO List

### 1. Repository Interface Updates

- [ ] Define `ResourceType` type in `internal/k8s/repository.go`
  - [ ] Add `ResourceTypePod` constant
  - [ ] Add constants for future resource types (commented for Phase 2)
- [ ] Define `LoadingStatus` struct in `internal/k8s/repository.go`
  - [ ] Add `Resource`, `Synced`, `Error` fields
- [ ] Update `Repository` interface in `internal/k8s/repository.go`
  - [ ] Add `GetLoadingStatus(ResourceType) LoadingStatus` method
  - [ ] Add `GetAllLoadingStatus() map[ResourceType]LoadingStatus` method
  - [ ] Add `IsResourceSynced(ResourceType) bool` method
  - [ ] Add `Stop()` method
- [ ] Keep existing `GetPods()`, `GetDeployments()`, `GetServices()` methods

### 2. InformerRepository Implementation

- [ ] Create `internal/k8s/informer_repository.go`
- [ ] Implement `InformerRepository` struct
  - [ ] Add `clientset *kubernetes.Clientset` field
  - [ ] Add `factory informers.SharedInformerFactory` field
  - [ ] Add `podLister v1listers.PodLister` field
  - [ ] Add `informers map[ResourceType]cache.SharedIndexInformer` field
  - [ ] Add `loadingStatus map[ResourceType]*LoadingStatus` field
  - [ ] Add `statusMu sync.RWMutex` for status protection
  - [ ] Add `stopCh chan struct{}` field
- [ ] Implement `NewInformerRepository(kubeconfig, context string)` function
  - [ ] Load kubeconfig using `clientcmd` (support default locations)
  - [ ] Apply context override if provided
  - [ ] Set protobuf content type: `config.ContentType =
    "application/vnd.kubernetes.protobuf"`
  - [ ] Create Kubernetes clientset
  - [ ] Create SharedInformerFactory (30 second resync)
  - [ ] Create Pod informer and lister
  - [ ] Store informer reference in map
  - [ ] Initialize loading status map
  - [ ] Initialize stopCh
  - [ ] Return repository instance
- [ ] Implement `StartPriority(ctx context.Context) error` method
  - [ ] Start the informer factory
  - [ ] Wait for Pod informer to sync using `cache.WaitForCacheSync`
  - [ ] Update loading status for Pods to synced=true
  - [ ] Return error if sync fails or times out
- [ ] Implement `Stop()` method
  - [ ] Close stopCh to stop informers
- [ ] Implement `GetLoadingStatus(ResourceType) LoadingStatus` method
  - [ ] Lock statusMu for reading
  - [ ] Return loading status for requested resource
  - [ ] Return "not synced" status if resource not tracked
- [ ] Implement `GetAllLoadingStatus() map[ResourceType]LoadingStatus` method
  - [ ] Lock statusMu for reading
  - [ ] Return copy of all loading statuses
- [ ] Implement `IsResourceSynced(ResourceType) bool` method
  - [ ] Return LoadingStatus.Synced for requested resource

### 3. GetPods Implementation

- [ ] Implement `GetPods() ([]Pod, error)` method in `InformerRepository`
  - [ ] Check if Pods are synced using loading status
  - [ ] Return error if not synced
  - [ ] List all pods using `podLister.List(labels.Everything())`
  - [ ] Transform each pod to internal `Pod` type (see next section)
  - [ ] Sort by age (newest first), then by name
  - [ ] Return pods slice
- [ ] Implement `transformPod(pod *corev1.Pod) Pod` helper function
  - [ ] Calculate age from CreationTimestamp
  - [ ] Calculate ready containers (readyCount/totalCount)
  - [ ] Calculate total restarts across all containers
  - [ ] Get pod phase as status
  - [ ] Extract node name and pod IP
  - [ ] Return internal Pod struct
  - [ ] Copy logic from `cmd/proto-pods-tui/main.go` lines 298-354

### 4. Main Application Integration

- [ ] Update `cmd/timoneiro/main.go`
  - [ ] Add kubeconfig flag (default: ~/.kube/config)
  - [ ] Add context flag (optional)
  - [ ] Create repository using `k8s.NewInformerRepository()`
  - [ ] Handle creation errors (log and exit)
  - [ ] Print "Syncing pods..." message
  - [ ] Call `repo.StartPriority(ctx)` (blocking)
  - [ ] Handle sync errors (log and exit)
  - [ ] Print "Pods synced! Starting UI..." message
  - [ ] Pass repository to `app.NewModel()`
  - [ ] Start Bubble Tea program
  - [ ] Add signal handling for graceful shutdown
  - [ ] Call `repo.Stop()` on shutdown

### 5. App Model Updates

- [ ] Update `internal/app/app.go`
  - [ ] Update `NewModel()` to accept `Repository` parameter
  - [ ] Store repository in model
  - [ ] Pass repository to screens during initialization
  - [ ] Remove `DummyRepository` instantiation

### 6. Pods Screen Updates

- [ ] Update `internal/screens/pods.go`
  - [ ] Store repository reference in model
  - [ ] Update `Init()` to trigger initial data load
  - [ ] Update `Update()` to handle periodic refresh (1 second tick)
  - [ ] Call `repo.GetPods()` on each tick
  - [ ] Handle errors gracefully (show error message, continue ticking)
  - [ ] Update table rows with new data
  - [ ] Maintain smart cursor positioning (track by namespace/name)
  - [ ] Copy cursor tracking logic from POC if needed

### 7. Update DummyRepository

- [ ] Update `internal/k8s/repository.go`
  - [ ] Implement new interface methods in `DummyRepository`
  - [ ] `GetLoadingStatus()` - return synced=true for all resources
  - [ ] `GetAllLoadingStatus()` - return map with dummy statuses
  - [ ] `IsResourceSynced()` - return true
  - [ ] `Stop()` - no-op

### 8. Testing

- [ ] Manual testing with local cluster
  - [ ] Test with minikube or kind cluster
  - [ ] Verify "Syncing pods..." message appears
  - [ ] Verify UI starts after 1-2 seconds
  - [ ] Verify pods are displayed correctly
  - [ ] Verify real-time updates (create/delete pods)
  - [ ] Verify cursor tracking works across updates
  - [ ] Verify filter/search works with real data
- [ ] Error scenario testing
  - [ ] Test with invalid kubeconfig path
  - [ ] Test with invalid context name
  - [ ] Test with no cluster access (connection refused)
  - [ ] Test with RBAC restrictions (no pod list permission)
- [ ] Performance testing
  - [ ] Measure sync time with 10, 100, 500 pods
  - [ ] Measure query time (should be <100μs)
  - [ ] Measure memory usage
  - [ ] Profile CPU usage during sync and refresh
- [ ] Verify graceful shutdown
  - [ ] Test ctrl+c handling
  - [ ] Verify informers stop cleanly
  - [ ] Verify no goroutine leaks

### 9. Documentation

- [ ] Update CLAUDE.md if needed
  - [ ] Document new command-line flags (--kubeconfig, --context)
  - [ ] Update development setup instructions
- [ ] Add code comments
  - [ ] Document InformerRepository struct and methods
  - [ ] Document transformPod function
  - [ ] Add package-level comments

### 10. Commit and Review

- [ ] Run `go mod tidy` to clean up dependencies
- [ ] Build and test: `go build ./cmd/timoneiro`
- [ ] Delete binary: `rm timoneiro`
- [ ] Create commit with message:
  ```
  feat: implement kubernetes informer-based repository for pods

  - Replace DummyRepository with InformerRepository
  - Use client-go informers with local caching
  - Support protobuf encoding for performance
  - Implement priority loading (Pods only in Phase 1)
  - Add loading status tracking for future multi-resource support
  - Achieve 1-2 second startup time

  Implements DDR-03 Phase 1
  ```
- [ ] Test final build one more time
- [ ] Mark DDR-03 status as "Partially Implemented"

## Success Criteria

- [ ] Application starts and connects to Kubernetes cluster
- [ ] Pods screen displays real pod data
- [ ] Startup time is 1-2 seconds for hundreds of pods
- [ ] Pod list updates in real-time (1-second refresh)
- [ ] Filter and search work with real data
- [ ] Application handles errors gracefully (connection, RBAC, etc.)
- [ ] No crashes or panics during normal operation
- [ ] Graceful shutdown works correctly

## Future Work (Phase 2+)

- Implement `StartBackground()` for parallel loading
- Add Deployments, Services, Namespaces, StatefulSets, DaemonSets
- Implement transformation functions for each resource type
- Add loading status indicator in UI header
- Update screens to check `IsResourceSynced()` before querying

## References

- Design Document: `design/DDR-03.md`
- POC Implementation: `cmd/proto-pods-tui/main.go`
- Current Repository: `internal/k8s/repository.go`
- Kubernetes client-go: https://pkg.go.dev/k8s.io/client-go
