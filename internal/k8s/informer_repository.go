package k8s

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/printers"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	appsv1listers "k8s.io/client-go/listers/apps/v1"
	v1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/yaml"
)

// Common GVRs used for statistics tracking
var (
	podGVR = schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}
	jobGVR = schema.GroupVersionResource{Group: "batch", Version: "v1", Resource: "jobs"}
)

// InformerRepository implements Repository using Kubernetes informers
type InformerRepository struct {
	// Typed client and informers (legacy, preserved for compatibility)
	clientset         *kubernetes.Clientset
	factory           informers.SharedInformerFactory
	podLister         v1listers.PodLister
	deploymentLister  appsv1listers.DeploymentLister
	serviceLister     v1listers.ServiceLister
	replicaSetLister  appsv1listers.ReplicaSetLister
	statefulSetLister appsv1listers.StatefulSetLister
	daemonSetLister   appsv1listers.DaemonSetLister

	// Dynamic client and informers (config-driven approach)
	dynamicClient  dynamic.Interface
	dynamicFactory dynamicinformer.DynamicSharedInformerFactory
	resources      map[ResourceType]ResourceConfig
	dynamicListers map[schema.GroupVersionResource]cache.GenericLister

	// Kubeconfig and context (for kubectl subprocess commands)
	kubeconfig  string
	contextName string

	// Performance indexes (built on informer updates)
	mu                    sync.RWMutex
	podsByNode            map[string][]*corev1.Pod            // nodeName → pods
	podsByNamespace       map[string][]*corev1.Pod            // namespace → pods
	podsByOwnerUID        map[string][]*corev1.Pod            // ownerUID → pods
	podsByConfigMap       map[string]map[string][]*corev1.Pod // namespace/configMapName → pods
	podsBySecret          map[string]map[string][]*corev1.Pod // namespace/secretName → pods
	jobsByOwnerUID        map[string][]string                 // ownerUID → job namespaced names
	jobsByNamespace       map[string][]string                 // namespace → job names
	replicaSetsByOwnerUID map[string][]string                 // deploymentUID → RS keys
	podsByPVC             map[string][]*corev1.Pod            // ns/pvcName → pods

	// Statistics tracking (channel-based, no locks needed)
	resourceStats map[schema.GroupVersionResource]*ResourceStats
	statsUpdateCh chan statsUpdateMsg

	ctx    context.Context
	cancel context.CancelFunc
}

// Event type constants for statistics tracking
const (
	eventTypeAdd    = "add"
	eventTypeUpdate = "update"
	eventTypeDelete = "delete"
)

// statsUpdateMsg is an internal message for statistics updates
type statsUpdateMsg struct {
	gvr       schema.GroupVersionResource
	eventType string
}

// NewInformerRepository creates a new informer-based repository
func NewInformerRepository(kubeconfig, contextName string) (*InformerRepository, error) {
	// Build kubeconfig path
	if kubeconfig == "" {
		if home := os.Getenv("HOME"); home != "" {
			kubeconfig = filepath.Join(home, ".kube", "config")
		} else {
			return nil, fmt.Errorf("HOME environment variable not set and no kubeconfig provided")
		}
	}

	// Build config
	loadingRules := &clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfig}
	configOverrides := &clientcmd.ConfigOverrides{}
	if contextName != "" {
		configOverrides.CurrentContext = contextName
	}

	config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		loadingRules,
		configOverrides,
	).ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("error building kubeconfig: %w", err)
	}

	// Use protobuf for better performance
	config.ContentType = "application/vnd.kubernetes.protobuf"

	// Create clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("error creating clientset: %w", err)
	}

	// Create dynamic client
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("error creating dynamic client: %w", err)
	}

	// Create shared informer factories with resync period
	factory := informers.NewSharedInformerFactory(clientset, InformerResyncPeriod)
	dynamicFactory := dynamicinformer.NewDynamicSharedInformerFactory(dynamicClient, InformerResyncPeriod)

	// Create pod informer and lister
	podInformer := factory.Core().V1().Pods().Informer()
	podLister := factory.Core().V1().Pods().Lister()

	// Create deployment informer and lister
	deploymentInformer := factory.Apps().V1().Deployments().Informer()
	deploymentLister := factory.Apps().V1().Deployments().Lister()

	// Create service informer and lister
	serviceInformer := factory.Core().V1().Services().Informer()
	serviceLister := factory.Core().V1().Services().Lister()

	// Create replicaset informer and lister (needed for deployment → pods filtering)
	replicaSetInformer := factory.Apps().V1().ReplicaSets().Informer()
	replicaSetLister := factory.Apps().V1().ReplicaSets().Lister()

	// Create statefulset informer and lister
	statefulSetInformer := factory.Apps().V1().StatefulSets().Informer()
	statefulSetLister := factory.Apps().V1().StatefulSets().Lister()

	// Create daemonset informer and lister
	daemonSetInformer := factory.Apps().V1().DaemonSets().Informer()
	daemonSetLister := factory.Apps().V1().DaemonSets().Lister()

	// Initialize resource registry
	resourceRegistry := getResourceRegistry()

	// Create dynamic informers for all registered resources
	dynamicListers := make(map[schema.GroupVersionResource]cache.GenericLister)
	dynamicInformers := make(map[schema.GroupVersionResource]cache.SharedIndexInformer)

	for _, resCfg := range resourceRegistry {
		informer := dynamicFactory.ForResource(resCfg.GVR).Informer()
		dynamicListers[resCfg.GVR] = dynamicFactory.ForResource(resCfg.GVR).Lister()
		dynamicInformers[resCfg.GVR] = informer
	}

	// Create context for lifecycle management
	ctx, cancel := context.WithCancel(context.Background())

	// Start informers in background
	factory.Start(ctx.Done())
	dynamicFactory.Start(ctx.Done())

	// Wait for caches to sync with timeout (graceful handling of RBAC errors)
	// Try to sync each informer individually, continue on failures
	syncCtx, syncCancel := context.WithTimeout(ctx, InformerSyncTimeout)
	defer syncCancel()

	// Track which informers synced successfully
	syncedInformers := make(map[schema.GroupVersionResource]bool)

	// Try typed informers first (pods, deployments, services, replicasets, statefulsets, daemonsets)
	typedSynced := cache.WaitForCacheSync(syncCtx.Done(),
		podInformer.HasSynced,
		deploymentInformer.HasSynced,
		serviceInformer.HasSynced,
		replicaSetInformer.HasSynced,
		statefulSetInformer.HasSynced,
		daemonSetInformer.HasSynced,
	)
	if !typedSynced {
		fmt.Fprintf(os.Stderr, "Warning: Some core resources (pods/deployments/services) failed to sync - may have permission issues\n")
	}

	// Try each dynamic informer individually
	for gvr, informer := range dynamicInformers {
		informerCtx, informerCancel := context.WithTimeout(ctx, InformerIndividualSyncTimeout)
		if cache.WaitForCacheSync(informerCtx.Done(), informer.HasSynced) {
			syncedInformers[gvr] = true
		} else {
			// Informer failed to sync (likely RBAC), remove from listers
			delete(dynamicListers, gvr)
			fmt.Fprintf(os.Stderr, "Warning: Resource %s/%s failed to sync - you may not have permission to watch this resource\n",
				gvr.Group, gvr.Resource)
		}
		informerCancel()
	}

	// Check if at least pods synced (critical resource)
	if !typedSynced {
		cancel()
		return nil, fmt.Errorf("failed to sync critical resources (pods/deployments/services) - check RBAC permissions")
	}

	// Initialize resource statistics
	resourceStats := make(map[schema.GroupVersionResource]*ResourceStats)
	for _, resCfg := range resourceRegistry {
		synced := syncedInformers[resCfg.GVR]
		resourceStats[resCfg.GVR] = &ResourceStats{
			ResourceType: ResourceType(resCfg.GVR.Resource),
			Count:        0,
			LastUpdate:   time.Time{},
			AddEvents:    0,
			UpdateEvents: 0,
			DeleteEvents: 0,
			Synced:       synced,
			MemoryBytes:  0,
		}
	}

	// Create repository with initialized indexes
	repo := &InformerRepository{
		clientset:             clientset,
		factory:               factory,
		podLister:             podLister,
		deploymentLister:      deploymentLister,
		serviceLister:         serviceLister,
		replicaSetLister:      replicaSetLister,
		statefulSetLister:     statefulSetLister,
		daemonSetLister:       daemonSetLister,
		dynamicClient:         dynamicClient,
		dynamicFactory:        dynamicFactory,
		resources:             resourceRegistry,
		dynamicListers:        dynamicListers,
		kubeconfig:            kubeconfig,
		contextName:           contextName,
		podsByNode:            make(map[string][]*corev1.Pod),
		podsByNamespace:       make(map[string][]*corev1.Pod),
		podsByOwnerUID:        make(map[string][]*corev1.Pod),
		podsByConfigMap:       make(map[string]map[string][]*corev1.Pod),
		podsBySecret:          make(map[string]map[string][]*corev1.Pod),
		jobsByOwnerUID:        make(map[string][]string),
		jobsByNamespace:       make(map[string][]string),
		replicaSetsByOwnerUID: make(map[string][]string),
		podsByPVC:             make(map[string][]*corev1.Pod),
		resourceStats:         resourceStats,
		statsUpdateCh:         make(chan statsUpdateMsg, 1000), // Buffered channel for high-frequency events
		ctx:                   ctx,
		cancel:                cancel,
	}

	// Start statistics updater goroutine
	go repo.statsUpdater()

	// Setup pod indexes with event handlers
	repo.setupPodIndexes()
	repo.setupJobIndexes()
	repo.setupReplicaSetIndexes()
	repo.setupDynamicInformersEventTracking(dynamicInformers)

	return repo, nil
}

// GetPods returns all pods from the informer cache
func (r *InformerRepository) GetPods() ([]Pod, error) {
	podList, err := r.podLister.List(labels.Everything())
	if err != nil {
		return nil, fmt.Errorf("error listing pods: %w", err)
	}

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

		// Get pod status
		status := string(pod.Status.Phase)

		// Get node and IP
		node := pod.Spec.NodeName
		ip := pod.Status.PodIP

		pods = append(pods, Pod{
			ResourceMetadata: ResourceMetadata{
				Namespace: pod.Namespace,
				Name:      pod.Name,
				Age:       age,
				CreatedAt: pod.CreationTimestamp.Time,
			},
			Ready:    readyStatus,
			Status:   status,
			Restarts: totalRestarts,
			Node:     node,
			IP:       ip,
		})
	}

	// Sort by creation time (newest first), then by name for stable sort
	sortByCreationTime(pods, func(p Pod) time.Time { return p.CreatedAt }, func(p Pod) string { return p.Name })

	return pods, nil
}

// GetDeployments returns all deployments from the informer cache
func (r *InformerRepository) GetDeployments() ([]Deployment, error) {
	deploymentList, err := r.deploymentLister.List(labels.Everything())
	if err != nil {
		return nil, fmt.Errorf("error listing deployments: %w", err)
	}

	deployments := make([]Deployment, 0, len(deploymentList))
	now := time.Now()

	for _, deploy := range deploymentList {
		// Calculate age
		age := now.Sub(deploy.CreationTimestamp.Time)

		// Get replica counts
		ready := int32(0)
		if deploy.Status.ReadyReplicas > 0 {
			ready = deploy.Status.ReadyReplicas
		}
		desired := int32(0)
		if deploy.Spec.Replicas != nil {
			desired = *deploy.Spec.Replicas
		}
		readyStatus := fmt.Sprintf("%d/%d", ready, desired)

		// Get up-to-date and available counts
		upToDate := deploy.Status.UpdatedReplicas
		available := deploy.Status.AvailableReplicas

		deployments = append(deployments, Deployment{
			ResourceMetadata: ResourceMetadata{
				Namespace: deploy.Namespace,
				Name:      deploy.Name,
				Age:       age,
				CreatedAt: deploy.CreationTimestamp.Time,
			},
			Ready:     readyStatus,
			UpToDate:  upToDate,
			Available: available,
		})
	}

	// Sort by creation time (newest first), then by name for stable sort
	sortByCreationTime(deployments, func(d Deployment) time.Time { return d.CreatedAt }, func(d Deployment) string { return d.Name })

	return deployments, nil
}

// GetServices returns all services from the informer cache
func (r *InformerRepository) GetServices() ([]Service, error) {
	serviceList, err := r.serviceLister.List(labels.Everything())
	if err != nil {
		return nil, fmt.Errorf("error listing services: %w", err)
	}

	services := make([]Service, 0, len(serviceList))
	now := time.Now()

	for _, svc := range serviceList {
		// Calculate age
		age := now.Sub(svc.CreationTimestamp.Time)

		// Get cluster IP
		clusterIP := svc.Spec.ClusterIP
		if clusterIP == "" {
			clusterIP = "<none>"
		}

		// Get external IP
		externalIP := "<none>"
		if len(svc.Status.LoadBalancer.Ingress) > 0 {
			if svc.Status.LoadBalancer.Ingress[0].IP != "" {
				externalIP = svc.Status.LoadBalancer.Ingress[0].IP
			} else if svc.Status.LoadBalancer.Ingress[0].Hostname != "" {
				externalIP = svc.Status.LoadBalancer.Ingress[0].Hostname
			}
		} else if len(svc.Spec.ExternalIPs) > 0 {
			externalIP = strings.Join(svc.Spec.ExternalIPs, ",")
		}

		// Format ports
		ports := make([]string, 0, len(svc.Spec.Ports))
		for _, port := range svc.Spec.Ports {
			portStr := fmt.Sprintf("%d", port.Port)
			if port.NodePort != 0 {
				portStr = fmt.Sprintf("%d:%d", port.Port, port.NodePort)
			}
			portStr = fmt.Sprintf("%s/%s", portStr, port.Protocol)
			ports = append(ports, portStr)
		}
		portsStr := strings.Join(ports, ",")
		if portsStr == "" {
			portsStr = "<none>"
		}

		services = append(services, Service{
			ResourceMetadata: ResourceMetadata{
				Namespace: svc.Namespace,
				Name:      svc.Name,
				Age:       age,
				CreatedAt: svc.CreationTimestamp.Time,
			},
			Type:       string(svc.Spec.Type),
			ClusterIP:  clusterIP,
			ExternalIP: externalIP,
			Ports:      portsStr,
		})
	}

	// Sort by creation time (newest first), then by name for stable sort
	sortByCreationTime(services, func(s Service) time.Time { return s.CreatedAt }, func(s Service) string { return s.Name })

	return services, nil
}

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

// GetResourceYAML returns YAML representation of a resource using kubectl YAMLPrinter
func (r *InformerRepository) GetResourceYAML(gvr schema.GroupVersionResource, namespace, name string) (string, error) {
	// Get resource from dynamic informer cache
	lister, ok := r.dynamicListers[gvr]
	if !ok {
		return "", fmt.Errorf("informer not initialized for resource %s", gvr)
	}

	var runtimeObj any
	var err error

	// Handle namespaced vs cluster-scoped resources
	if namespace != "" {
		runtimeObj, err = lister.ByNamespace(namespace).Get(name)
	} else {
		runtimeObj, err = lister.Get(name)
	}

	if err != nil {
		return "", fmt.Errorf("resource not found: %w", err)
	}

	// Type assert to unstructured
	obj, ok := runtimeObj.(*unstructured.Unstructured)
	if !ok {
		return "", fmt.Errorf("unexpected object type: %T", runtimeObj)
	}

	// Use kubectl YAML printer for exact kubectl output match
	printer := printers.NewTypeSetter(scheme.Scheme).ToPrinter(&printers.YAMLPrinter{})

	var buf bytes.Buffer
	if err := printer.PrintObj(obj, &buf); err != nil {
		return "", fmt.Errorf("failed to print YAML: %w", err)
	}

	return buf.String(), nil
}

// DescribeResource returns kubectl describe output for a resource
func (r *InformerRepository) DescribeResource(gvr schema.GroupVersionResource, namespace, name string) (string, error) {
	// For now, use a simplified describe implementation
	// TODO: Implement full kubectl describe formatters with Events support

	// Get resource from dynamic informer cache
	lister, ok := r.dynamicListers[gvr]
	if !ok {
		return "", fmt.Errorf("informer not initialized for resource %s", gvr)
	}

	var runtimeObj any
	var err error

	// Handle namespaced vs cluster-scoped resources
	if namespace != "" {
		runtimeObj, err = lister.ByNamespace(namespace).Get(name)
	} else {
		runtimeObj, err = lister.Get(name)
	}

	if err != nil {
		return "", fmt.Errorf("resource not found: %w", err)
	}

	// Type assert to unstructured
	obj, ok := runtimeObj.(*unstructured.Unstructured)
	if !ok {
		return "", fmt.Errorf("unexpected object type: %T", runtimeObj)
	}

	// Create a basic describe output using the resource's fields
	// This is a simplified version - full kubectl describe would require:
	// 1. Events informer
	// 2. Resource-specific describers (PodDescriber, DeploymentDescriber, etc.)

	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("Name:         %s\n", name))
	if namespace != "" {
		buf.WriteString(fmt.Sprintf("Namespace:    %s\n", namespace))
	}
	buf.WriteString(fmt.Sprintf("Kind:         %s\n", obj.GetKind()))
	buf.WriteString(fmt.Sprintf("API Version:  %s\n", obj.GetAPIVersion()))

	// Add labels if present
	labels := obj.GetLabels()
	if len(labels) > 0 {
		buf.WriteString("Labels:       ")
		first := true
		for k, v := range labels {
			if !first {
				buf.WriteString("              ")
			}
			buf.WriteString(fmt.Sprintf("%s=%s\n", k, v))
			first = false
		}
	}

	// Add creation timestamp
	buf.WriteString(fmt.Sprintf("Created:      %s\n", obj.GetCreationTimestamp().String()))

	// Add status if present, formatted as YAML
	status, found, err := unstructured.NestedFieldCopy(obj.Object, "status")
	if found && err == nil {
		statusYAML, err := yaml.Marshal(status)
		if err == nil {
			buf.WriteString("\nStatus:\n")
			// Indent status YAML by 2 spaces
			for _, line := range strings.Split(string(statusYAML), "\n") {
				if line != "" {
					buf.WriteString("  " + line + "\n")
				}
			}
		}
	}

	// Fetch events on-demand (not cached) to avoid memory overhead
	buf.WriteString("\nEvents:\n")
	events, err := r.fetchEventsForResource(namespace, name, string(obj.GetUID()))
	if err != nil {
		buf.WriteString(fmt.Sprintf("  Failed to fetch events: %v\n", err))
	} else if len(events) == 0 {
		buf.WriteString("  <none>\n")
	} else {
		buf.WriteString(r.formatEvents(events))
	}

	return buf.String(), nil
}

// fetchEventsForResource fetches events related to a specific resource on-demand
func (r *InformerRepository) fetchEventsForResource(namespace, name, uid string) ([]corev1.Event, error) {
	// Use field selector to filter events for this specific resource
	fieldSelector := fmt.Sprintf("involvedObject.name=%s,involvedObject.namespace=%s", name, namespace)
	if uid != "" {
		fieldSelector += fmt.Sprintf(",involvedObject.uid=%s", uid)
	}

	eventList, err := r.clientset.CoreV1().Events(namespace).List(
		r.ctx,
		metav1.ListOptions{
			FieldSelector: fieldSelector,
			Limit:         100, // Limit to most recent 100 events
		},
	)
	if err != nil {
		return nil, err
	}

	return eventList.Items, nil
}

// formatEvents formats events in kubectl describe style
func (r *InformerRepository) formatEvents(events []corev1.Event) string {
	if len(events) == 0 {
		return "  <none>\n"
	}

	// Sort events by timestamp (newest first)
	sort.Slice(events, func(i, j int) bool {
		return events[i].LastTimestamp.After(events[j].LastTimestamp.Time)
	})

	var buf bytes.Buffer
	buf.WriteString("  Type    Reason    Age                    Message\n")
	buf.WriteString("  ----    ------    ---                    -------\n")

	now := time.Now()
	for _, event := range events {
		eventType := event.Type
		reason := event.Reason
		message := event.Message

		// Calculate age
		var age string
		if !event.LastTimestamp.IsZero() {
			duration := now.Sub(event.LastTimestamp.Time)
			age = formatEventAge(duration)
		} else if !event.EventTime.IsZero() {
			duration := now.Sub(event.EventTime.Time)
			age = formatEventAge(duration)
		} else {
			age = "<unknown>"
		}

		// Truncate message if too long
		if len(message) > 80 {
			message = message[:77] + "..."
		}

		buf.WriteString(fmt.Sprintf("  %-7s %-9s %-22s %s\n", eventType, reason, age, message))
	}

	return buf.String()
}

// formatEventAge formats event age in kubectl style (e.g., "5m", "2h", "3d")
func formatEventAge(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	} else if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	} else if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	} else {
		return fmt.Sprintf("%dd", int(d.Hours()/24))
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

// Close stops the informers and cleans up resources
func (r *InformerRepository) Close() {
	if r.cancel != nil {
		r.cancel()
	}
	if r.statsUpdateCh != nil {
		close(r.statsUpdateCh) // Goroutine will exit when channel is drained
	}
}

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
			pod := obj.(*corev1.Pod)
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

// GetKubeconfig returns the kubeconfig path
func (r *InformerRepository) GetKubeconfig() string {
	return r.kubeconfig
}

// GetContext returns the context name
func (r *InformerRepository) GetContext() string {
	return r.contextName
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
			unstr, ok := obj.(*unstructured.Unstructured)
			if !ok {
				return
			}
			r.removeJobFromIndexes(unstr)
			r.trackStats(jobGVR, eventTypeDelete)
		},
	})
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
			unstr, ok := obj.(*unstructured.Unstructured)
			if !ok {
				return
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

// GetResources returns all resources of the specified type using dynamic informers
func (r *InformerRepository) GetResources(resourceType ResourceType) ([]any, error) {
	// Get resource config
	config, ok := r.resources[resourceType]
	if !ok {
		return nil, fmt.Errorf("unknown resource type: %s", resourceType)
	}

	// Get dynamic lister for this resource
	lister, ok := r.dynamicListers[config.GVR]
	if !ok {
		// Informer failed to sync (likely RBAC issue) - return explicit error
		return nil, fmt.Errorf("cannot access %s: informer failed to sync (check RBAC permissions)", resourceType)
	}

	// List resources from cache
	objList, err := lister.List(labels.Everything())
	if err != nil {
		return nil, fmt.Errorf("failed to list %s: %w", resourceType, err)
	}

	// Transform unstructured objects to typed structs
	resources := make([]Resource, 0, len(objList))
	for _, obj := range objList {
		unstr, ok := obj.(*unstructured.Unstructured)
		if !ok {
			continue
		}

		// Extract common fields once per resource (optimization)
		common := extractMetadata(unstr)

		transformed, err := config.Transform(unstr, common)
		if err != nil {
			// Log error but continue (partial results better than nothing)
			continue
		}

		// Type assert to Resource interface for sorting
		resource, ok := transformed.(Resource)
		if !ok {
			// Skip non-Resource types (shouldn't happen)
			continue
		}
		resources = append(resources, resource)
	}

	// Sort resources by age using Resource interface (newest first)
	sortByAge(resources)

	// Convert back to []any for existing API compatibility
	results := make([]any, len(resources))
	for i, r := range resources {
		results[i] = r
	}

	return results, nil
}

// sortByAge sorts resources by CreatedAt field using Resource interface (newest first)
// Note: Despite the name "sortByAge", we sort by CreatedAt (stable timestamp) not Age (recalculated each time)
// This ensures stable sorting - Age field changes every second which causes list instability
// Uses SliceStable with name as secondary sort key for deterministic ordering
func sortByAge(items []Resource) {
	sort.SliceStable(items, func(i, j int) bool {
		createdI := items[i].GetCreatedAt()
		createdJ := items[j].GetCreatedAt()

		// Primary sort: by creation time (newest first)
		if !createdI.Equal(createdJ) {
			return createdI.After(createdJ)
		}

		// Secondary sort: by name (alphabetically) for deterministic ordering
		nameI := items[i].GetName()
		nameJ := items[j].GetName()
		if nameI != nameJ {
			return nameI < nameJ
		}

		// Tertiary sort: by namespace for cluster-wide views with same names
		return items[i].GetNamespace() < items[j].GetNamespace()
	})
}

// sortByCreationTime is a generic helper for sorting typed slices by CreatedAt (newest first)
type resourceWithTimestamp interface {
	Pod | Deployment | Service | ConfigMap | Secret | Namespace | StatefulSet | DaemonSet | Job | CronJob | Node | ReplicaSet | PersistentVolumeClaim | Ingress | Endpoints | HorizontalPodAutoscaler
}

func sortByCreationTime[T resourceWithTimestamp](items []T, getCreatedAt func(T) time.Time, getName func(T) string) {
	sort.Slice(items, func(i, j int) bool {
		createdI := getCreatedAt(items[i])
		createdJ := getCreatedAt(items[j])

		if !createdI.Equal(createdJ) {
			return createdI.After(createdJ) // Newer first
		}

		// Fall back to name comparison for stable sort
		return getName(items[i]) < getName(items[j])
	})
}
