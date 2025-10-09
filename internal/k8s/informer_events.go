package k8s

import (
	"time"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/cache"
)

// setupDynamicInformersEventTracking registers event handlers for statistics tracking on all dynamic informers
func (r *InformerRepository) setupDynamicInformersEventTracking(dynamicInformers map[schema.GroupVersionResource]cache.SharedIndexInformer) {
	for gvr, informer := range dynamicInformers {
		// Skip job informer (already has tracking in setupJobIndexes)
		if gvr.Group == "batch" && gvr.Resource == "jobs" {
			continue
		}

		// Capture gvr in closure
		gvrCopy := gvr

		informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				r.trackStats(gvrCopy, eventTypeAdd)
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				r.trackStats(gvrCopy, eventTypeUpdate)
			},
			DeleteFunc: func(obj interface{}) {
				r.trackStats(gvrCopy, eventTypeDelete)
			},
		})
	}
}

// trackStats sends a statistics update to the channel (non-blocking)
// If the channel is full, the update is skipped since stats are approximate
func (r *InformerRepository) trackStats(gvr schema.GroupVersionResource, eventType string) {
	select {
	case r.statsUpdateCh <- statsUpdateMsg{gvr: gvr, eventType: eventType}:
	default:
		// Channel full, skip this update (stats are approximate anyway)
	}
}

// statsUpdater is a goroutine that owns the resourceStats map and processes updates
// This eliminates lock contention from high-frequency event handlers
func (r *InformerRepository) statsUpdater() {
	for msg := range r.statsUpdateCh {
		stats, ok := r.resourceStats[msg.gvr]
		if !ok {
			continue
		}

		switch msg.eventType {
		case eventTypeAdd:
			stats.AddEvents++
		case eventTypeUpdate:
			stats.UpdateEvents++
		case eventTypeDelete:
			stats.DeleteEvents++
		}
		stats.LastUpdate = time.Now()
	}
}
