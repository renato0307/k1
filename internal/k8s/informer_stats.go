package k8s

import (
	"sort"

	"k8s.io/apimachinery/pkg/labels"
)

// updateMemoryStats calculates approximate memory usage for all resource types
// No locks needed - this is called from GetResourceStats which reads from the
// statsUpdater goroutine-owned map. Slight data races are acceptable since
// stats are approximate.
func (r *InformerRepository) updateMemoryStats() {
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

	// For informers that failed to sync (not in dynamicListers), ensure stats reflect 0
	for gvr, stats := range r.resourceStats {
		if _, exists := r.dynamicListers[gvr]; !exists {
			stats.Count = 0
			stats.MemoryBytes = 0
		}
	}
}

// GetResourceStats returns statistics for all resource types
// No locks needed - accepts slightly stale data for better performance
func (r *InformerRepository) GetResourceStats() []ResourceStats {
	r.updateMemoryStats() // Refresh counts and memory

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
