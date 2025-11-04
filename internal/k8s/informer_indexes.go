package k8s

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/cache"
)

// setupPodIndexes registers event handlers to maintain pod indexes
func (r *InformerRepository) setupPodIndexes() {
	podInformer := r.factory.Core().V1().Pods().Informer()

	podInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			pod := obj.(*corev1.Pod)
			r.updatePodIndexes(pod, nil)
			r.trackStats(podGVR, eventTypeAdd)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldPod := oldObj.(*corev1.Pod)
			newPod := newObj.(*corev1.Pod)
			r.updatePodIndexes(newPod, oldPod)
			r.trackStats(podGVR, eventTypeUpdate)
		},
		DeleteFunc: func(obj interface{}) {
			// Handle DeletedFinalStateUnknown wrapper
			pod, ok := obj.(*corev1.Pod)
			if !ok {
				tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
				if !ok {
					// Unable to get object from tombstone
					return
				}
				pod, ok = tombstone.Obj.(*corev1.Pod)
				if !ok {
					// Tombstone contained object that is not a Pod
					return
				}
			}
			r.removePodFromIndexes(pod)
			r.trackStats(podGVR, eventTypeDelete)
		},
	})
}

// updatePodIndexes updates all indexes for a pod (call with nil oldPod for adds)
func (r *InformerRepository) updatePodIndexes(newPod, oldPod *corev1.Pod) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Remove old pod from indexes if updating
	if oldPod != nil {
		r.removePodFromIndexesLocked(oldPod)
	}

	// Add to node index
	if newPod.Spec.NodeName != "" {
		r.podsByNode[newPod.Spec.NodeName] = append(
			r.podsByNode[newPod.Spec.NodeName],
			newPod,
		)
	}

	// Add to namespace index
	r.podsByNamespace[newPod.Namespace] = append(
		r.podsByNamespace[newPod.Namespace],
		newPod,
	)

	// Add to owner index
	for _, ownerRef := range newPod.OwnerReferences {
		r.podsByOwnerUID[string(ownerRef.UID)] = append(
			r.podsByOwnerUID[string(ownerRef.UID)],
			newPod,
		)
	}

	// Add to ConfigMap index (inspect volumes)
	for _, volume := range newPod.Spec.Volumes {
		if volume.ConfigMap != nil {
			cmName := volume.ConfigMap.Name
			ns := newPod.Namespace
			if r.podsByConfigMap[ns] == nil {
				r.podsByConfigMap[ns] = make(map[string][]*corev1.Pod)
			}
			r.podsByConfigMap[ns][cmName] = append(r.podsByConfigMap[ns][cmName], newPod)
		}
	}

	// Add to Secret index (inspect volumes)
	for _, volume := range newPod.Spec.Volumes {
		if volume.Secret != nil {
			secretName := volume.Secret.SecretName
			ns := newPod.Namespace
			if r.podsBySecret[ns] == nil {
				r.podsBySecret[ns] = make(map[string][]*corev1.Pod)
			}
			r.podsBySecret[ns][secretName] = append(r.podsBySecret[ns][secretName], newPod)
		}
	}

	// Add to PVC index (inspect volumes)
	for _, volume := range newPod.Spec.Volumes {
		if volume.PersistentVolumeClaim != nil {
			pvcKey := newPod.Namespace + "/" + volume.PersistentVolumeClaim.ClaimName
			r.podsByPVC[pvcKey] = append(r.podsByPVC[pvcKey], newPod)
		}
	}
}

// removePodFromIndexes removes a pod from all indexes (acquires lock)
func (r *InformerRepository) removePodFromIndexes(pod *corev1.Pod) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.removePodFromIndexesLocked(pod)
}

// removePodFromIndexesLocked removes a pod from all indexes (assumes lock held)
func (r *InformerRepository) removePodFromIndexesLocked(pod *corev1.Pod) {
	// Remove from node index
	if pod.Spec.NodeName != "" {
		r.podsByNode[pod.Spec.NodeName] = removePodFromSlice(
			r.podsByNode[pod.Spec.NodeName],
			pod,
		)
		if len(r.podsByNode[pod.Spec.NodeName]) == 0 {
			delete(r.podsByNode, pod.Spec.NodeName)
		}
	}

	// Remove from namespace index
	r.podsByNamespace[pod.Namespace] = removePodFromSlice(
		r.podsByNamespace[pod.Namespace],
		pod,
	)
	if len(r.podsByNamespace[pod.Namespace]) == 0 {
		delete(r.podsByNamespace, pod.Namespace)
	}

	// Remove from owner index
	for _, ownerRef := range pod.OwnerReferences {
		ownerUID := string(ownerRef.UID)
		r.podsByOwnerUID[ownerUID] = removePodFromSlice(
			r.podsByOwnerUID[ownerUID],
			pod,
		)
		if len(r.podsByOwnerUID[ownerUID]) == 0 {
			delete(r.podsByOwnerUID, ownerUID)
		}
	}

	// Remove from ConfigMap index
	for _, volume := range pod.Spec.Volumes {
		if volume.ConfigMap != nil {
			cmName := volume.ConfigMap.Name
			ns := pod.Namespace
			if r.podsByConfigMap[ns] != nil {
				r.podsByConfigMap[ns][cmName] = removePodFromSlice(
					r.podsByConfigMap[ns][cmName],
					pod,
				)
				if len(r.podsByConfigMap[ns][cmName]) == 0 {
					delete(r.podsByConfigMap[ns], cmName)
				}
				if len(r.podsByConfigMap[ns]) == 0 {
					delete(r.podsByConfigMap, ns)
				}
			}
		}
	}

	// Remove from Secret index
	for _, volume := range pod.Spec.Volumes {
		if volume.Secret != nil {
			secretName := volume.Secret.SecretName
			ns := pod.Namespace
			if r.podsBySecret[ns] != nil {
				r.podsBySecret[ns][secretName] = removePodFromSlice(
					r.podsBySecret[ns][secretName],
					pod,
				)
				if len(r.podsBySecret[ns][secretName]) == 0 {
					delete(r.podsBySecret[ns], secretName)
				}
				if len(r.podsBySecret[ns]) == 0 {
					delete(r.podsBySecret, ns)
				}
			}
		}
	}

	// Remove from PVC index
	for _, volume := range pod.Spec.Volumes {
		if volume.PersistentVolumeClaim != nil {
			pvcKey := pod.Namespace + "/" + volume.PersistentVolumeClaim.ClaimName
			r.podsByPVC[pvcKey] = removePodFromSlice(r.podsByPVC[pvcKey], pod)
			if len(r.podsByPVC[pvcKey]) == 0 {
				delete(r.podsByPVC, pvcKey)
			}
		}
	}
}

// removePodFromSlice removes a pod from a slice by comparing UIDs
func removePodFromSlice(pods []*corev1.Pod, target *corev1.Pod) []*corev1.Pod {
	result := make([]*corev1.Pod, 0, len(pods))
	for _, p := range pods {
		if p.UID != target.UID {
			result = append(result, p)
		}
	}
	return result
}

// setupJobIndexes registers event handlers to maintain job indexes for CronJob → Jobs navigation
func (r *InformerRepository) setupJobIndexes() {
	// Get job informer from dynamic factory
	jobInformer := r.dynamicFactory.ForResource(jobGVR).Informer()

	jobInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			unstr, ok := obj.(*unstructured.Unstructured)
			if !ok {
				return
			}
			r.updateJobIndexes(unstr, nil)
			r.trackStats(jobGVR, eventTypeAdd)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldUnstr, _ := oldObj.(*unstructured.Unstructured)
			newUnstr, ok := newObj.(*unstructured.Unstructured)
			if !ok {
				return
			}
			r.updateJobIndexes(newUnstr, oldUnstr)
			r.trackStats(jobGVR, eventTypeUpdate)
		},
		DeleteFunc: func(obj interface{}) {
			// Handle DeletedFinalStateUnknown wrapper
			unstr, ok := obj.(*unstructured.Unstructured)
			if !ok {
				tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
				if !ok {
					return
				}
				unstr, ok = tombstone.Obj.(*unstructured.Unstructured)
				if !ok {
					return
				}
			}
			r.removeJobFromIndexes(unstr)
			r.trackStats(jobGVR, eventTypeDelete)
		},
	})
}

// updateJobIndexes updates all indexes for a job
func (r *InformerRepository) updateJobIndexes(newJob, oldJob *unstructured.Unstructured) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Remove old job from indexes if updating
	if oldJob != nil {
		r.removeJobFromIndexesLocked(oldJob)
	}

	// Create job namespaced name
	namespace := newJob.GetNamespace()
	name := newJob.GetName()
	jobKey := namespace + "/" + name

	// Add to namespace index
	r.jobsByNamespace[namespace] = append(r.jobsByNamespace[namespace], jobKey)

	// Add to owner index
	for _, ownerRef := range newJob.GetOwnerReferences() {
		ownerUID := string(ownerRef.UID)
		r.jobsByOwnerUID[ownerUID] = append(r.jobsByOwnerUID[ownerUID], jobKey)
	}
}

// removeJobFromIndexes removes a job from all indexes (acquires lock)
func (r *InformerRepository) removeJobFromIndexes(job *unstructured.Unstructured) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.removeJobFromIndexesLocked(job)
}

// removeJobFromIndexesLocked removes a job from all indexes (assumes lock held)
func (r *InformerRepository) removeJobFromIndexesLocked(job *unstructured.Unstructured) {
	namespace := job.GetNamespace()
	name := job.GetName()
	jobKey := namespace + "/" + name

	// Remove from namespace index
	r.jobsByNamespace[namespace] = removeStringFromSlice(r.jobsByNamespace[namespace], jobKey)
	if len(r.jobsByNamespace[namespace]) == 0 {
		delete(r.jobsByNamespace, namespace)
	}

	// Remove from owner index
	for _, ownerRef := range job.GetOwnerReferences() {
		ownerUID := string(ownerRef.UID)
		r.jobsByOwnerUID[ownerUID] = removeStringFromSlice(r.jobsByOwnerUID[ownerUID], jobKey)
		if len(r.jobsByOwnerUID[ownerUID]) == 0 {
			delete(r.jobsByOwnerUID, ownerUID)
		}
	}
}

// setupReplicaSetIndexes sets up event handlers for ReplicaSet index maintenance
func (r *InformerRepository) setupReplicaSetIndexes() {
	// Get ReplicaSet informer from dynamic factory
	rsGVR := schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "replicasets"}
	rsInformer := r.dynamicFactory.ForResource(rsGVR).Informer()

	rsInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			unstr, ok := obj.(*unstructured.Unstructured)
			if !ok {
				return
			}
			r.updateReplicaSetIndexes(unstr, nil)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			// Owner references are immutable, no update needed for indexes
			// But we still track the event for statistics
		},
		DeleteFunc: func(obj interface{}) {
			// Handle DeletedFinalStateUnknown wrapper
			unstr, ok := obj.(*unstructured.Unstructured)
			if !ok {
				tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
				if !ok {
					return
				}
				unstr, ok = tombstone.Obj.(*unstructured.Unstructured)
				if !ok {
					return
				}
			}
			r.removeReplicaSetFromIndexes(unstr)
		},
	})
}

// updateReplicaSetIndexes updates all indexes for a ReplicaSet
func (r *InformerRepository) updateReplicaSetIndexes(newRS, oldRS *unstructured.Unstructured) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Remove old RS from indexes if updating
	if oldRS != nil {
		r.removeReplicaSetFromIndexesLocked(oldRS)
	}

	// Create RS namespaced name key
	rsKey, err := cache.MetaNamespaceKeyFunc(newRS)
	if err != nil {
		return
	}

	// Add to owner index (Deployment → ReplicaSet)
	for _, ownerRef := range newRS.GetOwnerReferences() {
		if ownerRef.Kind == "Deployment" {
			ownerUID := string(ownerRef.UID)
			r.replicaSetsByOwnerUID[ownerUID] = append(r.replicaSetsByOwnerUID[ownerUID], rsKey)
		}
	}
}

// removeReplicaSetFromIndexes removes a ReplicaSet from all indexes (acquires lock)
func (r *InformerRepository) removeReplicaSetFromIndexes(rs *unstructured.Unstructured) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.removeReplicaSetFromIndexesLocked(rs)
}

// removeReplicaSetFromIndexesLocked removes a ReplicaSet from all indexes (assumes lock held)
func (r *InformerRepository) removeReplicaSetFromIndexesLocked(rs *unstructured.Unstructured) {
	rsKey, err := cache.MetaNamespaceKeyFunc(rs)
	if err != nil {
		return
	}

	// Remove from owner index
	for _, ownerRef := range rs.GetOwnerReferences() {
		if ownerRef.Kind == "Deployment" {
			ownerUID := string(ownerRef.UID)
			r.replicaSetsByOwnerUID[ownerUID] = removeStringFromSlice(r.replicaSetsByOwnerUID[ownerUID], rsKey)
			if len(r.replicaSetsByOwnerUID[ownerUID]) == 0 {
				delete(r.replicaSetsByOwnerUID, ownerUID)
			}
		}
	}
}

// removeStringFromSlice removes a string from a slice
func removeStringFromSlice(slice []string, target string) []string {
	result := make([]string, 0, len(slice))
	for _, s := range slice {
		if s != target {
			result = append(result, s)
		}
	}
	return result
}
