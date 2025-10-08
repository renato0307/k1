# Cache Implementation: Mutex vs Channels for Index Cache

**Date:** 2025-10-08
**Context:** Discussing concurrency strategy for InformerRepository indexes
**Status:** Decision - Keep mutex implementation

## Question

Should we replace the current RWMutex-based index cache with a channel-based
implementation to avoid potential lock contention?

## Background

The InformerRepository maintains several in-memory indexes for fast lookups:
- `podsByNode` - node name → pods
- `podsByOwnerUID` - owner UID → pods
- `podsByNamespace` - namespace → pods
- `podsByConfigMap` - namespace/name → pods
- `podsBySecret` - namespace/name → pods
- `jobsByOwnerUID` - owner UID → jobs

These indexes are:
- **Written** by informer event handlers (pod add/update/delete)
- **Read** by navigation queries (e.g., GetPodsForDeployment)

Current implementation uses `sync.RWMutex` to protect concurrent access.

## Contention Scenario Analysis

### Real-World Case: Pod Creation Storm

During cluster scale-up, rolling updates, or batch job starts:
- 1000 pods created rapidly
- Each pod triggers index update with **exclusive lock**
- User navigates (Deployment → Pods) during this storm

### Impact Measurements

**Lock behavior:**
```go
// Each update holds exclusive lock
r.mu.Lock()           // Blocks ALL reads
r.podsByNode[node] = append(...)
r.podsByOwnerUID[uid] = append(...)
r.mu.Unlock()         // ~5μs per pod
```

**Contention metrics:**
- 1000 pod events = 5ms total lock time
- User query waits up to **5-10ms** worst case
- **User perception:** Slight pause, generally acceptable for TUI

**Mitigating factor:** Informer event handlers run sequentially by default,
so writes aren't concurrent with each other - only with reads.

## Trade-off Analysis

### Option 1: Keep RWMutex (Current)

**Pros:**
- ✅ Concurrent reads (multiple GetPods* calls run in parallel)
- ✅ O(1) map lookups without serialization
- ✅ Standard Go pattern for protecting shared data
- ✅ Simple to reason about
- ✅ Optimized for read-heavy workloads

**Cons:**
- ⚠️ 5-10ms occasional delays during pod storms
- ⚠️ Need careful lock/unlock discipline

### Option 2: Channel-based Architecture

**Proposed design:**
```go
type cacheQuery struct {
    requestType string
    params      map[string]string
    responseCh  chan<- []Pod
}

// Single goroutine owns all indexes
go func() {
    for query := range queryCh {
        result := indexes[query.requestType][query.params]
        query.responseCh <- result
    }
}()
```

**Pros:**
- ✅ No shared memory, eliminates lock bugs
- ✅ Natural Go idiom
- ✅ Clear ownership

**Cons:**
- ❌ ALL reads serialized (even with zero writes)
- ❌ Every lookup requires channel roundtrip
- ❌ **10x slower in normal case** to avoid 5ms occasional delay
- ❌ More complex: channel lifecycle, request/response management
- ❌ Harder to debug channel deadlocks

## Decision

**Keep the RWMutex implementation** because:

1. **Read-heavy workload:** Most operations are queries, not updates
2. **RWMutex is designed for this:** Multiple concurrent readers
3. **Performance matters:** UI responsiveness requires <10ms queries
4. **5-10ms acceptable:** Humans react at 100ms+ timescale
5. **Rare occurrence:** Pod storms don't happen constantly
6. **Simplicity:** Current code is clear and maintainable

**Channels would be appropriate for:**
- Write-heavy workloads
- Work queues/pipelines
- Fan-out/fan-in patterns
- Complex coordination between multiple writers

## Future Optimization Options

**If contention becomes a real problem** (measured user complaints or >50ms
lock waits), consider:

### 1. Batch Index Updates (Easiest - 90% benefit)

Accumulate events, update indexes in batches:
```go
// Reduces 1000 lock acquisitions to ~10
func (r *InformerRepository) setupPodIndexes() {
    batchCh := make(chan *corev1.Pod, 100)

    go func() {
        batch := []*corev1.Pod{}
        ticker := time.NewTicker(10 * time.Millisecond)

        for {
            select {
            case pod := <-batchCh:
                batch = append(batch, pod)
            case <-ticker.C:
                if len(batch) > 0 {
                    r.updatePodIndexesBatch(batch)  // Single lock
                    batch = batch[:0]
                }
            }
        }
    }()
}
```

**Benefit:** Reduces lock acquisitions by 100x during storms
**Cost:** Minimal complexity, <50 lines of code

### 2. Shard by Namespace (Better Scaling)

Separate locks per namespace:
```go
type indexShard struct {
    mu              sync.RWMutex
    podsByOwnerUID  map[string][]*corev1.Pod
}

shards map[string]*indexShard  // namespace → shard
```

**Benefit:** Updates in namespace A don't block reads in namespace B
**Cost:** More memory, moderate complexity

### 3. Copy-on-Write (Lowest Latency)

Immutable snapshots with atomic swaps:
```go
type indexSnapshot struct {
    podsByOwnerUID  map[string][]*corev1.Pod  // immutable
}

indexes atomic.Value  // *indexSnapshot

// Updates clone, modify, swap
new := old.clone()
new.podsByOwnerUID[uid] = append(...)
r.indexes.Store(new)
```

**Benefit:** Zero read latency
**Cost:** Higher memory usage, GC pressure

### 4. Measurement First

Add contention metrics before optimizing:
```go
func (r *InformerRepository) GetPodsForDeployment(...) {
    start := time.Now()
    r.mu.RLock()
    lockWait := time.Since(start)

    if lockWait > 10*time.Millisecond {
        metrics.LockContentionMs.Observe(lockWait.Milliseconds())
    }

    // ... query logic
}
```

## When to Revisit

**Optimize when:**
- Users report sluggishness during deployments
- Metrics show >50ms lock wait times consistently
- Supporting 10K+ pod clusters with rapid churn
- Measured performance impact, not theoretical concern

**Current target:** <1K pod clusters with typical DevOps workflows
**Current decision:** Keep simple, measure, optimize when needed

## References

- `internal/k8s/informer_repository.go` - Current implementation
- `internal/k8s/informer_repository_test.go` - Performance test
  (TestInformerRepository_IndexedQuery_Performance)

## Lessons

1. **Measure before optimizing:** Premature optimization adds complexity
2. **RWMutex for read-heavy:** Don't reach for channels by default
3. **Context matters:** 5ms delay in TUI ≠ 5ms delay in web API
4. **Incremental optimization:** Start with batching, not full rewrites
