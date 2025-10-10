package k8s

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	appsv1listers "k8s.io/client-go/listers/apps/v1"
	v1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
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

// NewInformerRepositoryWithProgress creates a new informer-based repository with progress reporting
func NewInformerRepositoryWithProgress(kubeconfig, contextName string, progress chan<- ContextLoadProgress) (*InformerRepository, error) {
	// Report connection phase
	if progress != nil {
		progress <- ContextLoadProgress{
			Context: contextName,
			Message: "Connecting to API server...",
			Phase:   PhaseConnecting,
		}
	}


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

	// Report core sync phase
	if progress != nil {
		progress <- ContextLoadProgress{
			Context: contextName,
			Message: "Syncing core resources...",
			Phase:   PhaseSyncingCore,
		}
	}

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
	// Note: If typedSynced is false, we'll return an error below after checking pods

	// Report dynamic sync phase
	if progress != nil {
		progress <- ContextLoadProgress{
			Context: contextName,
			Message: "Syncing dynamic resources...",
			Phase:   PhaseSyncingDynamic,
		}
	}

	// Try each dynamic informer individually
	for gvr, informer := range dynamicInformers {
		informerCtx, informerCancel := context.WithTimeout(ctx, InformerIndividualSyncTimeout)
		if cache.WaitForCacheSync(informerCtx.Done(), informer.HasSynced) {
			syncedInformers[gvr] = true
		} else {
			// Informer failed to sync (likely RBAC), remove from listers
			delete(dynamicListers, gvr)
			// Silently skip - RBAC failures are expected in some clusters
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

	// Report completion
	if progress != nil {
		progress <- ContextLoadProgress{
			Context: contextName,
			Message: "Context loaded successfully",
			Phase:   PhaseComplete,
		}
	}

	return repo, nil
}

// NewInformerRepository creates a new informer-based repository (backward compatible)
func NewInformerRepository(kubeconfig, contextName string) (*InformerRepository, error) {
	return NewInformerRepositoryWithProgress(kubeconfig, contextName, nil)
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

// GetKubeconfig returns the kubeconfig path
func (r *InformerRepository) GetKubeconfig() string {
	return r.kubeconfig
}

// GetContext returns the context name
func (r *InformerRepository) GetContext() string {
	return r.contextName
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

// Context management methods (not supported by single repository, use RepositoryPool)

// SwitchContext is not supported by InformerRepository (use RepositoryPool)
func (r *InformerRepository) SwitchContext(contextName string, progress chan<- ContextLoadProgress) error {
	return fmt.Errorf("context switching not supported by InformerRepository, use RepositoryPool")
}

// GetAllContexts is not supported by InformerRepository (use RepositoryPool)
func (r *InformerRepository) GetAllContexts() []ContextWithStatus {
	return []ContextWithStatus{}
}

// GetActiveContext returns the current context name
func (r *InformerRepository) GetActiveContext() string {
	return r.contextName
}

// RetryFailedContext is not supported by InformerRepository (use RepositoryPool)
func (r *InformerRepository) RetryFailedContext(contextName string, progress chan<- ContextLoadProgress) error {
	return fmt.Errorf("retry failed context not supported by InformerRepository, use RepositoryPool")
}

// GetContexts is not supported by InformerRepository (use RepositoryPool)
func (r *InformerRepository) GetContexts() ([]Context, error) {
	return []Context{}, fmt.Errorf("get contexts not supported by InformerRepository, use RepositoryPool")
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
