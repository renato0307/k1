package k8s

import (
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/cache"
)

// GetPodsForDeployment returns pods owned by a specific deployment (uses indexed lookups)
func (r *InformerRepository) GetPodsForDeployment(namespace, name string) ([]Pod, error) {
	// Get deployment to find its UID
	deployment, err := r.deploymentLister.Deployments(namespace).Get(name)
	if err != nil {
		return nil, fmt.Errorf("failed to get deployment: %w", err)
	}

	// Get ReplicaSets owned by this deployment
	allReplicaSets, err := r.replicaSetLister.ReplicaSets(namespace).List(labels.Everything())
	if err != nil {
		return nil, fmt.Errorf("failed to list replicasets: %w", err)
	}

	// Find ReplicaSets owned by this deployment and collect their pods using index
	r.mu.RLock()
	var allPods []*corev1.Pod
	for _, rs := range allReplicaSets {
		for _, owner := range rs.OwnerReferences {
			if owner.UID == deployment.UID {
				// Use indexed lookup for pods owned by this ReplicaSet
				pods := r.podsByOwnerUID[string(rs.UID)]
				allPods = append(allPods, pods...)
				break
			}
		}
	}
	r.mu.RUnlock()

	return r.transformPods(allPods)
}

// GetPodsOnNode returns pods running on a specific node (O(1) indexed lookup)
func (r *InformerRepository) GetPodsOnNode(nodeName string) ([]Pod, error) {
	r.mu.RLock()
	pods := r.podsByNode[nodeName]
	r.mu.RUnlock()

	return r.transformPods(pods)
}

// GetPodsForService returns pods matching a service's selector
func (r *InformerRepository) GetPodsForService(namespace, name string) ([]Pod, error) {
	// Get service to find its selector
	service, err := r.serviceLister.Services(namespace).Get(name)
	if err != nil {
		return nil, fmt.Errorf("failed to get service: %w", err)
	}

	// If service has no selector, return empty list
	if len(service.Spec.Selector) == 0 {
		return []Pod{}, nil
	}

	// Convert service selector to label selector
	selector := labels.SelectorFromSet(service.Spec.Selector)

	// List pods in the same namespace matching the selector
	podList, err := r.podLister.Pods(namespace).List(selector)
	if err != nil {
		return nil, fmt.Errorf("error listing pods: %w", err)
	}

	// Convert to our Pod type
	pods := make([]Pod, 0, len(podList))
	now := time.Now()

	for _, pod := range podList {
		// Calculate age
		age := now.Sub(pod.CreationTimestamp.Time)

		// Calculate ready containers
		readyContainers := 0
		totalContainers := len(pod.Status.ContainerStatuses)
		for _, cs := range pod.Status.ContainerStatuses {
			if cs.Ready {
				readyContainers++
			}
		}
		readyStatus := fmt.Sprintf("%d/%d", readyContainers, totalContainers)

		// Calculate total restarts
		totalRestarts := int32(0)
		for _, cs := range pod.Status.ContainerStatuses {
			totalRestarts += cs.RestartCount
		}

		pods = append(pods, Pod{
			ResourceMetadata: ResourceMetadata{
				Namespace: pod.Namespace,
				Name:      pod.Name,
				Age:       age,
				CreatedAt: pod.CreationTimestamp.Time,
			},
			Ready:    readyStatus,
			Status:   string(pod.Status.Phase),
			Restarts: totalRestarts,
			Node:     pod.Spec.NodeName,
			IP:       pod.Status.PodIP,
		})
	}

	// Sort by creation time (newest first), then by name for stable sort
	sortByCreationTime(pods, func(p Pod) time.Time { return p.CreatedAt }, func(p Pod) string { return p.Name })

	return pods, nil
}

// GetPodsForStatefulSet returns pods owned by a specific statefulset (uses indexed lookups)
func (r *InformerRepository) GetPodsForStatefulSet(namespace, name string) ([]Pod, error) {
	// Get statefulset to find its UID
	statefulSet, err := r.statefulSetLister.StatefulSets(namespace).Get(name)
	if err != nil {
		return nil, fmt.Errorf("failed to get statefulset: %w", err)
	}

	// Use indexed lookup for pods owned by this StatefulSet
	r.mu.RLock()
	pods := r.podsByOwnerUID[string(statefulSet.UID)]
	r.mu.RUnlock()

	return r.transformPods(pods)
}

// GetPodsForDaemonSet returns pods owned by a specific daemonset (uses indexed lookups)
func (r *InformerRepository) GetPodsForDaemonSet(namespace, name string) ([]Pod, error) {
	// Get daemonset to find its UID
	daemonSet, err := r.daemonSetLister.DaemonSets(namespace).Get(name)
	if err != nil {
		return nil, fmt.Errorf("failed to get daemonset: %w", err)
	}

	// Use indexed lookup for pods owned by this DaemonSet
	r.mu.RLock()
	pods := r.podsByOwnerUID[string(daemonSet.UID)]
	r.mu.RUnlock()

	return r.transformPods(pods)
}

// GetPodsForJob returns pods owned by a specific job (uses indexed lookups)
func (r *InformerRepository) GetPodsForJob(namespace, name string) ([]Pod, error) {
	// Get job from dynamic lister to find its UID
	lister, ok := r.dynamicListers[jobGVR]
	if !ok {
		return nil, fmt.Errorf("job informer not initialized")
	}

	obj, err := lister.ByNamespace(namespace).Get(name)
	if err != nil {
		return nil, fmt.Errorf("failed to get job: %w", err)
	}

	unstr, ok := obj.(*unstructured.Unstructured)
	if !ok {
		return nil, fmt.Errorf("unexpected object type: %T", obj)
	}

	// Use indexed lookup for pods owned by this Job
	r.mu.RLock()
	pods := r.podsByOwnerUID[string(unstr.GetUID())]
	r.mu.RUnlock()

	return r.transformPods(pods)
}

// GetJobsForCronJob returns jobs owned by a specific cronjob (uses indexed lookups)
func (r *InformerRepository) GetJobsForCronJob(namespace, name string) ([]Job, error) {
	// Get cronjob from dynamic lister to find its UID
	cronJobGVR := schema.GroupVersionResource{Group: "batch", Version: "v1", Resource: "cronjobs"}
	lister, ok := r.dynamicListers[cronJobGVR]
	if !ok {
		return nil, fmt.Errorf("cronjob informer not initialized")
	}

	obj, err := lister.ByNamespace(namespace).Get(name)
	if err != nil {
		return nil, fmt.Errorf("failed to get cronjob: %w", err)
	}

	unstr, ok := obj.(*unstructured.Unstructured)
	if !ok {
		return nil, fmt.Errorf("unexpected object type: %T", obj)
	}

	// Use indexed lookup for jobs owned by this CronJob
	r.mu.RLock()
	jobKeys := r.jobsByOwnerUID[string(unstr.GetUID())]
	r.mu.RUnlock()

	// Fetch jobs from dynamic lister
	jobLister, ok := r.dynamicListers[jobGVR]
	if !ok {
		return nil, fmt.Errorf("job informer not initialized")
	}

	jobs := make([]Job, 0, len(jobKeys))

	for _, jobKey := range jobKeys {
		// Parse namespace/name from key
		parts := strings.Split(jobKey, "/")
		if len(parts) != 2 {
			continue
		}
		jobNamespace, jobName := parts[0], parts[1]

		obj, err := jobLister.ByNamespace(jobNamespace).Get(jobName)
		if err != nil {
			// Job may have been deleted, skip
			continue
		}

		jobUnstr, ok := obj.(*unstructured.Unstructured)
		if !ok {
			continue
		}

		// Extract common fields
		common := extractMetadata(jobUnstr)

		// Transform to Job type
		transformed, err := transformJob(jobUnstr, common)
		if err != nil {
			continue
		}

		job, ok := transformed.(Job)
		if !ok {
			continue
		}

		jobs = append(jobs, job)
	}

	// Sort by creation time (newest first)
	sortByCreationTime(jobs, func(j Job) time.Time { return j.CreatedAt }, func(j Job) string { return j.Name })

	return jobs, nil
}

// GetPodsForNamespace returns all pods in a specific namespace (uses indexed lookups)
func (r *InformerRepository) GetPodsForNamespace(namespace string) ([]Pod, error) {
	r.mu.RLock()
	pods := r.podsByNamespace[namespace]
	r.mu.RUnlock()

	return r.transformPods(pods)
}

// GetPodsUsingConfigMap returns pods that use a specific ConfigMap (uses indexed lookups)
func (r *InformerRepository) GetPodsUsingConfigMap(namespace, name string) ([]Pod, error) {
	r.mu.RLock()
	var pods []*corev1.Pod
	if r.podsByConfigMap[namespace] != nil {
		pods = r.podsByConfigMap[namespace][name]
	}
	r.mu.RUnlock()

	return r.transformPods(pods)
}

// GetPodsUsingSecret returns pods that use a specific Secret (uses indexed lookups)
func (r *InformerRepository) GetPodsUsingSecret(namespace, name string) ([]Pod, error) {
	r.mu.RLock()
	var pods []*corev1.Pod
	if r.podsBySecret[namespace] != nil {
		pods = r.podsBySecret[namespace][name]
	}
	r.mu.RUnlock()

	return r.transformPods(pods)
}

// GetPodsForReplicaSet returns pods owned by a specific ReplicaSet (uses indexed lookups)
func (r *InformerRepository) GetPodsForReplicaSet(namespace, name string) ([]Pod, error) {
	// Get ReplicaSet to extract UID
	rsGVR := schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "replicasets"}
	rsLister, ok := r.dynamicListers[rsGVR]
	if !ok {
		return nil, fmt.Errorf("replicaset informer not initialized")
	}

	rsObj, err := rsLister.ByNamespace(namespace).Get(name)
	if err != nil {
		return nil, fmt.Errorf("replicaset not found: %w", err)
	}

	rsUnstr, ok := rsObj.(*unstructured.Unstructured)
	if !ok {
		return nil, fmt.Errorf("invalid replicaset object")
	}

	// Use existing podsByOwnerUID index
	r.mu.RLock()
	pods := r.podsByOwnerUID[string(rsUnstr.GetUID())]
	r.mu.RUnlock()

	return r.transformPods(pods)
}

// GetReplicaSetsForDeployment returns ReplicaSets owned by a specific Deployment (uses indexed lookups)
func (r *InformerRepository) GetReplicaSetsForDeployment(namespace, name string) ([]ReplicaSet, error) {
	// Get Deployment to extract UID
	deployGVR := schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}
	deployLister, ok := r.dynamicListers[deployGVR]
	if !ok {
		return nil, fmt.Errorf("deployment informer not initialized")
	}

	deployObj, err := deployLister.ByNamespace(namespace).Get(name)
	if err != nil {
		return nil, fmt.Errorf("deployment not found: %w", err)
	}

	deployUnstr, ok := deployObj.(*unstructured.Unstructured)
	if !ok {
		return nil, fmt.Errorf("invalid deployment object")
	}

	// Use replicaSetsByOwnerUID index
	r.mu.RLock()
	rsKeys := r.replicaSetsByOwnerUID[string(deployUnstr.GetUID())]
	r.mu.RUnlock()

	// Fetch ReplicaSets by keys
	rsGVR := schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "replicasets"}
	rsLister, ok := r.dynamicListers[rsGVR]
	if !ok {
		return nil, fmt.Errorf("replicaset informer not initialized")
	}

	results := make([]ReplicaSet, 0, len(rsKeys))
	for _, key := range rsKeys {
		keyNamespace, keyName, err := cache.SplitMetaNamespaceKey(key)
		if err != nil {
			continue
		}

		rsObj, err := rsLister.ByNamespace(keyNamespace).Get(keyName)
		if err != nil {
			continue
		}

		rsUnstr, ok := rsObj.(*unstructured.Unstructured)
		if !ok {
			continue
		}

		common := extractMetadata(rsUnstr)
		transformed, err := transformReplicaSet(rsUnstr, common)
		if err != nil {
			continue
		}

		rs, ok := transformed.(ReplicaSet)
		if !ok {
			continue
		}

		results = append(results, rs)
	}

	// Sort by creation time (newest first)
	sortByCreationTime(results, func(r ReplicaSet) time.Time { return r.CreatedAt }, func(r ReplicaSet) string { return r.Name })

	return results, nil
}

// GetPodsForPVC returns pods that use a specific PersistentVolumeClaim (uses indexed lookups)
func (r *InformerRepository) GetPodsForPVC(namespace, name string) ([]Pod, error) {
	key := namespace + "/" + name

	r.mu.RLock()
	pods := r.podsByPVC[key]
	r.mu.RUnlock()

	return r.transformPods(pods)
}

// transformPods converts []*corev1.Pod to []Pod (our app type)
func (r *InformerRepository) transformPods(podList []*corev1.Pod) ([]Pod, error) {
	pods := make([]Pod, 0, len(podList))
	now := time.Now()

	for _, pod := range podList {
		// Calculate age
		age := now.Sub(pod.CreationTimestamp.Time)

		// Calculate ready containers
		readyContainers := 0
		totalContainers := len(pod.Status.ContainerStatuses)
		for _, cs := range pod.Status.ContainerStatuses {
			if cs.Ready {
				readyContainers++
			}
		}
		readyStatus := fmt.Sprintf("%d/%d", readyContainers, totalContainers)

		// Calculate total restarts
		totalRestarts := int32(0)
		for _, cs := range pod.Status.ContainerStatuses {
			totalRestarts += cs.RestartCount
		}

		pods = append(pods, Pod{
			ResourceMetadata: ResourceMetadata{
				Namespace: pod.Namespace,
				Name:      pod.Name,
				Age:       age,
				CreatedAt: pod.CreationTimestamp.Time,
			},
			Ready:    readyStatus,
			Status:   string(pod.Status.Phase),
			Restarts: totalRestarts,
			Node:     pod.Spec.NodeName,
			IP:       pod.Status.PodIP,
		})
	}

	// Sort by creation time (newest first), then by name for stable sort
	sortByCreationTime(pods, func(p Pod) time.Time { return p.CreatedAt }, func(p Pod) string { return p.Name })

	return pods, nil
}
