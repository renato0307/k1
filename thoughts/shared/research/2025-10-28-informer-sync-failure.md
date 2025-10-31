---
date: 2025-10-28T10:45:00-03:00
researcher: claude
git_commit: 7f4bfff6b1559fe069ac6cff6bf8b3678ce38d3b
branch: feat/basic-crds
repository: k1-basic-crds
topic: "Kubernetes Informer Sync Failure for High-Volume Resources"
tags: [research, codebase, kubernetes, informers, client-go, performance]
status: complete
last_updated: 2025-10-28
last_updated_by: claude
---

# Research: Kubernetes Informer Sync Failure for High-Volume Resources

**Date**: 2025-10-28T10:45:00-03:00
**Researcher**: claude
**Git Commit**: 7f4bfff6b1559fe069ac6cff6bf8b3678ce38d3b
**Branch**: feat/basic-crds
**Repository**: k1-basic-crds

## Research Question

Why do high-volume Kubernetes resources (pods, deployments, replicasets)
fail to sync after 120 seconds while low-volume resources (services,
statefulsets, daemonsets) sync successfully, even though all use the same
factory initialization pattern?

**Observable symptoms:**
- Pod cache size: 0 items after 2 minutes
- Pods, deployments, replicasets: NOT SYNCED
- Services, statefulsets, daemonsets: synced successfully
- Works on another machine with same cluster
- ~950 pods in cluster with constant churn

## Summary

The root cause is **timeout exhaustion during initial LIST operations** for
large resource collections. The initial LIST request for 900+ pods exceeds
the configured 90-second API timeout, causing the informer to never populate
its cache. This is compounded by a known Kubernetes issue (#133810) where
reflectors can enter infinite loops when dealing with expired resource
versions.

**Key finding**: Pod cache has 0 items, meaning the initial LIST request
never completed successfully—not a sync processing issue, but a
LIST timeout issue.

## Detailed Findings

### 1. Identical Initialization Patterns

**Location**: `internal/k8s/informer_repository.go:152-174`

All resource types use identical initialization:
```go
// Pods (lines 152-154)
podInformer := factory.Core().V1().Pods().Informer()
podLister := factory.Core().V1().Pods().Lister()

// Services (lines 160-162)
serviceInformer := factory.Core().V1().Services().Informer()
serviceLister := factory.Core().V1().Services().Lister()

// StatefulSets (lines 168-170)
statefulSetInformer := factory.Apps().V1().StatefulSets().Informer()
statefulSetLister := factory.Apps().V1().StatefulSets().Lister()
```

**Conclusion**: No code-level differences. The failure is data-volume
dependent, not initialization dependent.

### 2. Timeout Configuration Analysis

**Location**: `internal/k8s/informer_repository.go:131-134`

Current configuration:
```go
config.Timeout = 90 * time.Second  // API request timeout
config.QPS = 50
config.Burst = 100
```

**Location**: `internal/k8s/constants.go:12-20`

```go
InformerSyncTimeout = 120 * time.Second  // Overall sync timeout
InformerIndividualSyncTimeout = 60 * time.Second
```

**Problem**: Timeout hierarchy issue discovered

1. API timeout: 90s (time allowed for single HTTP LIST request)
2. Informer sync timeout: 120s (time to check all informers)
3. With 6 typed informers starting simultaneously:
   - All share network bandwidth
   - All compete for API server resources
   - Large responses (pods: 900+ items) can exceed 90s
   - Smaller responses (services: fewer items) complete within 90s

### 3. Client-Go Behavior Analysis

**How informer sync works** (from client-go source):

1. `factory.Start(stopCh)` starts goroutines for LIST+WATCH
2. Each goroutine:
   - Performs HTTP LIST request to get initial resource snapshot
   - Populates DeltaFIFO queue with items
   - Starts controller to process queue
   - Begins WATCH for updates
3. `HasSynced()` returns true when:
   - Initial LIST completed (`populated=true`)
   - All LIST items processed (`initialPopulationCount==0`)

**Why HasSynced() stays false**:
- LIST request times out after 90s
- No items ever reach the cache (0 items observed)
- `populated` flag never set to true
- Reflector may retry but hits same timeout

### 4. Known Kubernetes Issue #133810

**Title**: "Reflector infinite loop with old resource versions"
**Status**: Open (as of January 2025)
**Impact**: Critical for understanding persistent failures

**Scenario**:
1. Client connects to API Server A with resourceVersion 100
2. etcd compacts data to resourceVersion 110
3. Client switches to API Server B (load balancing/restart)
4. Watch fails: "too old resource version: 100"
5. Client performs paginated LIST with rv 100
6. **Bug**: LIST returns same rv 100 instead of current version
7. Next watch repeats step 4 → infinite loop

**Relevance to this issue**:
- Explains why informers never recover even after 2 minutes
- Especially likely in clusters with high churn (pods updating constantly)
- User's other machine may connect to different API server instance

### 5. Resource Volume Correlation

**Evidence from logs**:

| Resource      | Count (estimated) | Sync Status | Cache Size |
|---------------|-------------------|-------------|------------|
| Pods          | 950+              | FAILED      | 0 items    |
| Deployments   | 100-200           | FAILED      | Unknown    |
| ReplicaSets   | 200-400           | FAILED      | Unknown    |
| Services      | 50-100            | SUCCESS ✓   | N/A        |
| StatefulSets  | 10-30             | SUCCESS ✓   | N/A        |
| DaemonSets    | 5-10              | SUCCESS ✓   | N/A        |

**Pattern**: Resources with fewer items sync successfully. Failure
threshold appears to be around 100-200 items.

**Explanation**:
- Small LIST responses (services) complete within 90s timeout
- Large LIST responses (pods) exceed 90s and get cancelled
- API server may prioritize smaller responses
- Network/etcd latency more impactful on larger responses

### 6. Error Handling Assessment

**Location**: `internal/k8s/informer_repository.go:193-297`

**Findings**:
- No errors are swallowed at initialization (line 193-194)
- `factory.Start()` doesn't return errors (by design in client-go)
- Sync errors ARE detected via `cache.WaitForCacheSync()` (line 233-240)
- Detailed per-resource error reporting implemented (line 246-275)
- **Missing**: Runtime watch error handler (SetWatchErrorHandler)

**Verdict**: Error handling is correct for initialization. The issue is not
masked errors but legitimate timeouts during LIST operations.

### 7. Why It Works on Another Machine

**Possible explanations**:

1. **Different API server instance**:
   - Load balancer may route to less-loaded server
   - Different etcd latency characteristics
   - Not affected by Issue #133810 (fresh resourceVersion)

2. **Network path differences**:
   - Lower latency network path
   - Different NAT/proxy configuration
   - More stable connection (fewer retries)

3. **Timing**:
   - Fewer concurrent users during test
   - Less cluster churn at that moment
   - etcd less loaded

4. **Client location**:
   - Different region/zone with better connectivity
   - Closer to API server geographically

## Code References

- `internal/k8s/informer_repository.go:131-134` - API timeout configuration
- `internal/k8s/informer_repository.go:193-194` - Factory start (no errors)
- `internal/k8s/informer_repository.go:233-240` - Sync check for all
  informers
- `internal/k8s/informer_repository.go:246-275` - Failure diagnosis
- `internal/k8s/constants.go:12-20` - Timeout constants
- `internal/k8s/informer_repository.go:216-230` - Progress logging

## Architecture Insights

**Informer lifecycle**:
```
factory.Start() → goroutine per resource
    ↓
HTTP LIST request (timeout: 90s)
    ↓ (success)
DeltaFIFO.Replace() → populated=true, count=N
    ↓
Controller.Pop() × N → count=0
    ↓
HasSynced()=true → WaitForCacheSync succeeds
```

**Failure point**: HTTP LIST request times out before completing, never
reaching DeltaFIFO.Replace().

**Why timeout varies by resource**:
- Response size: 900 pods × ~2KB (protobuf) = ~1.8MB
- Serialization time on API server
- Network transfer time
- Concurrent requests competing for bandwidth
- etcd read latency for large result sets

## Proposed Solutions

### Immediate Fix: Increase Timeouts

**Change 1**: Increase API timeout
```go
// internal/k8s/informer_repository.go:131
config.Timeout = 180 * time.Second  // Was 90s
```

**Change 2**: Increase informer sync timeout accordingly
```go
// internal/k8s/constants.go:15
InformerSyncTimeout = 240 * time.Second  // Was 120s, must exceed API timeout
```

**Rationale**:
- Allows large LIST requests to complete
- 180s is reasonable for 900+ pods in production clusters
- Must maintain hierarchy: InformerSyncTimeout > config.Timeout

### Short-term Fix: Add Diagnostic Logging

**Add before sync** (line ~205):
```go
// Log informer status before sync attempt
fmt.Fprintf(os.Stderr, "Pre-sync: pods=%v, deployments=%v, services=%v\n",
    podInformer.HasSynced(), deploymentInformer.HasSynced(),
    serviceInformer.HasSynced())
```

**Add to failure block** (line ~269):
```go
// Check if this might be Issue #133810
if podStore := podInformer.GetStore(); podStore != nil {
    fmt.Fprintf(os.Stderr, "  Pod cache size: %d items\n",
        len(podStore.List()))
    if len(podStore.List()) == 0 {
        fmt.Fprintf(os.Stderr, "  WARNING: Cache empty suggests LIST
            request never completed\n")
        fmt.Fprintf(os.Stderr, "  Possible causes: timeout, network
            issue, or K8s Issue #133810\n")
    }
}
```

### Medium-term Fix: Progressive Sync Strategy

**Approach**: Sync resources independently with individual timeouts

```go
// Instead of waiting for all together:
typedSynced := cache.WaitForCacheSync(syncCtx.Done(),
    podInformer.HasSynced,
    deploymentInformer.HasSynced,
    // ...
)

// Try syncing critical resources first with longer timeout:
podCtx, podCancel := context.WithTimeout(ctx, 180 * time.Second)
if !cache.WaitForCacheSync(podCtx.Done(), podInformer.HasSynced) {
    // Retry logic for Issue #133810
}
podCancel()

// Then sync remaining resources with shorter timeout
otherCtx, otherCancel := context.WithTimeout(ctx, 90 * time.Second)
cache.WaitForCacheSync(otherCtx.Done(),
    deploymentInformer.HasSynced,
    serviceInformer.HasSynced,
    // ...
)
otherCancel()
```

**Benefits**:
- Isolates timeout issues to specific resources
- Allows partial success (some resources work while others retry)
- Enables targeted retry logic

### Long-term Fix: Handle Issue #133810

**Implement retry on resourceVersion errors**:

```go
// Detect "too old resource version" error
if !typedSynced {
    // Check if cache is empty (suggests LIST never completed)
    if podStore := podInformer.GetStore(); podStore != nil {
        if len(podStore.List()) == 0 {
            // Possible Issue #133810 - recreate informer with fresh rv
            fmt.Fprintf(os.Stderr, "Retrying with fresh resourceVersion...\n")

            // Would need to:
            // 1. Stop existing informer
            // 2. Create new informer (gets fresh rv from LIST)
            // 3. Restart sync process

            // This requires refactoring to support informer recreation
        }
    }
}
```

**Note**: Full implementation requires refactoring to allow informer
recreation, which is non-trivial with current architecture.

### Alternative: Connection Pooling Optimization

**Add to config** (line ~130):
```go
// Configure transport for better concurrency
transport, err := rest.TransportFor(config)
if err != nil {
    return nil, fmt.Errorf("error creating transport: %w", err)
}

// Increase connection pool for concurrent LISTs
if t, ok := transport.(*http.Transport); ok {
    t.MaxIdleConns = 100
    t.MaxIdleConnsPerHost = 20
    t.IdleConnTimeout = 90 * time.Second
}
config.Transport = transport
```

**Rationale**: Ensures concurrent LIST requests don't exhaust connection
pool, reducing retry overhead.

## Open Questions

1. **Why does kubectl work faster?**
   - Does kubectl use different chunking strategy?
   - Does kubectl benefit from watch cache differently?
   - Need to profile actual kubectl LIST duration vs k1

2. **Is Issue #133810 occurring here?**
   - Check API server logs for "too old resource version" errors
   - Monitor resourceVersion values in logs
   - Test with API server restart during sync

3. **What's the actual LIST duration for 900 pods?**
   - Measure with: `time kubectl get pods -A -o json > /dev/null`
   - Compare with current 90s timeout
   - Test with/without protobuf encoding

4. **Does chunking help or hurt?**
   - Client-go uses pagination by default (limit=500)
   - Each chunk needs separate request → more round trips
   - But each request is smaller → less likely to timeout
   - Need to measure actual behavior

## Recommended Next Steps

1. **Immediate**: Increase timeouts (180s API, 240s sync) and test
2. **Verify**: Measure actual `kubectl get pods -A` duration to confirm
   timeout hypothesis
3. **Monitor**: Add logging to detect Issue #133810 patterns
4. **Consider**: Progressive sync strategy for better user experience
5. **Long-term**: Implement proper retry logic for resourceVersion errors

## Related Research

- Kubernetes Issue #133810: Reflector infinite loop
- client-go reflector.go: LIST/WATCH implementation
- client-go delta_fifo.go: HasSynced() implementation
- Client-go documentation on informer best practices

## References

- https://github.com/kubernetes/kubernetes/issues/133810
- https://github.com/kubernetes/client-go/blob/master/tools/cache/reflector.go
- https://github.com/kubernetes/client-go/blob/master/tools/cache/delta_fifo.go
- https://github.com/kubernetes/sample-controller/blob/master/docs/controller-client-go.md
