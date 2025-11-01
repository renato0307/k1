package k8s

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/renato0307/k1/internal/logging"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

	// Informer sync status tracking
	typedInformersReady     atomic.Bool
	typedInformersSyncError atomic.Value // stores error
	dynamicInformerErrors   map[schema.GroupVersionResource]error

	closed atomic.Bool // Atomic flag for safe close detection
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
	totalStart := logging.Start("NewInformerRepositoryWithProgress")
	defer logging.End(totalStart)

	logging.Info("Creating informer repository", "context", contextName)

	// Report connection phase
	if progress != nil {
		progress <- ContextLoadProgress{
			Context: contextName,
			Message: "Connecting to API server…",
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
	logging.Debug("Using protobuf content type", "content_type", config.ContentType)

	// Increase timeout and configure for large clusters
	config.Timeout = 90 * time.Second
	config.QPS = 50    // Allow more requests per second
	config.Burst = 100 // Allow bursts for initial sync
	logging.Debug("API config", "timeout", config.Timeout, "qps", config.QPS, "burst", config.Burst)

	// Create clientset
	clientsetStart := logging.Start("create typed clientset")
	clientset, err := kubernetes.NewForConfig(config)
	logging.End(clientsetStart)
	if err != nil {
		logging.Error("Failed to create clientset", "error", err)
		return nil, fmt.Errorf("error creating clientset: %w", err)
	}
	logging.Debug("Typed clientset created")

	// Early auth check - fail fast instead of waiting 120s for informer timeout
	authCheckStart := logging.Start("auth check")
	authCtx, authCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer authCancel()
	_, err = clientset.CoreV1().Namespaces().List(authCtx, metav1.ListOptions{Limit: 1})
	logging.End(authCheckStart)
	if err != nil {
		logging.Error("Auth check failed", "error", err)
		return nil, fmt.Errorf("failed to connect to cluster: %w", err)
	}
	logging.Debug("Auth check passed")

	// Create dynamic client
	dynamicStart := logging.Start("create dynamic client")
	dynamicClient, err := dynamic.NewForConfig(config)
	logging.End(dynamicStart)
	if err != nil {
		logging.Error("Failed to create dynamic client", "error", err)
		return nil, fmt.Errorf("error creating dynamic client: %w", err)
	}
	logging.Debug("Dynamic client created")

	// Create shared informer factories with resync period
	factoryStart := logging.Start("create informer factories")
	factory := informers.NewSharedInformerFactory(clientset, InformerResyncPeriod)
	dynamicFactory := dynamicinformer.NewDynamicSharedInformerFactory(dynamicClient, InformerResyncPeriod)
	logging.End(factoryStart)
	logging.Debug("Informer factories created")

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

	// Create dynamic informers for startup resources only (Tier > 0)
	// Tier 0 resources are loaded on-demand
	// NOTE: Listers are NOT added yet - they'll be added after sync completes
	dynamicListers := make(map[schema.GroupVersionResource]cache.GenericLister)
	dynamicInformers := make(map[schema.GroupVersionResource]cache.SharedIndexInformer)

	for _, resCfg := range resourceRegistry {
		if resCfg.Tier == 0 {
			// Skip on-demand resources at startup
			continue
		}
		informer := dynamicFactory.ForResource(resCfg.GVR).Informer()
		// Don't add lister yet - will be added by background goroutine after sync
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
			Message: "Syncing core resources…",
			Phase:   PhaseSyncingCore,
		}
	}

	logging.Info("Starting informer sync (non-blocking)")

	// Try ReplicaSets separately (non-blocking)
	go func() {
		rsCtx, rsCancel := context.WithTimeout(ctx, InformerSyncTimeout)
		defer rsCancel()
		if !cache.WaitForCacheSync(rsCtx.Done(), replicaSetInformer.HasSynced) {
			logging.Warn("ReplicaSet informer did not sync (timeout)")
		} else {
			logging.Debug("ReplicaSet informer synced")
		}
	}()

	// Report dynamic sync phase
	if progress != nil {
		progress <- ContextLoadProgress{
			Context: contextName,
			Message: "Syncing dynamic resources…",
			Phase:   PhaseSyncingDynamic,
		}
	}

	logging.Info("Starting dynamic informer sync (non-blocking)", "resource_count", len(dynamicInformers))

	// Initialize resource statistics
	// All informers are syncing in background, so Synced will be false initially
	// The background goroutines will update the synced status as they complete
	resourceStats := make(map[schema.GroupVersionResource]*ResourceStats)
	for _, resCfg := range resourceRegistry {
		resourceStats[resCfg.GVR] = &ResourceStats{
			ResourceType: ResourceType(resCfg.GVR.Resource),
			Count:        0,
			LastUpdate:   time.Time{},
			AddEvents:    0,
			UpdateEvents: 0,
			DeleteEvents: 0,
			Synced:       false, // Will be updated by background sync goroutines
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
		dynamicInformerErrors: make(map[schema.GroupVersionResource]error),
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

	// Start background sync for typed informers (non-blocking)
	// Launched after repo creation so it can store errors
	go func() {
		typedSyncStart := logging.Start("sync typed informers (pods, deployments, services, statefulsets, daemonsets)")
		syncCtx, syncCancel := context.WithTimeout(ctx, InformerSyncTimeout)
		defer syncCancel()

		// Try typed informers together (they sync in parallel)
		// Note: ReplicaSets excluded from critical check - they're used internally by
		// deployments but can hit load balancer timeouts on large clusters
		typedSynced := cache.WaitForCacheSync(syncCtx.Done(),
			podInformer.HasSynced,
			deploymentInformer.HasSynced,
			serviceInformer.HasSynced,
			statefulSetInformer.HasSynced,
			daemonSetInformer.HasSynced,
		)
		logging.End(typedSyncStart)

		if typedSynced {
			// Mark as ready
			repo.typedInformersReady.Store(true)

			// Log individual resource counts
			podCount := len(podInformer.GetStore().List())
			deploymentCount := len(deploymentInformer.GetStore().List())
			serviceCount := len(serviceInformer.GetStore().List())
			statefulsetCount := len(statefulSetInformer.GetStore().List())
			daemonsetCount := len(daemonSetInformer.GetStore().List())

			logging.Info("Core informers synced",
				"pods", podCount,
				"deployments", deploymentCount,
				"services", serviceCount,
				"statefulsets", statefulsetCount,
				"daemonsets", daemonsetCount,
			)
		} else {
			// Sync failed - try to determine why by making a test API call
			errMsg := "Informers failed to sync within timeout"

			// Try a simple API call to get a better error message
			testCtx, testCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer testCancel()
			_, err := clientset.CoreV1().Namespaces().List(testCtx, metav1.ListOptions{Limit: 1})
			if err != nil {
				errMsg = fmt.Sprintf("Failed to connect to cluster: %v", err)
				logging.Error("Cluster connection failed", "error", err)
			} else {
				// API call succeeded but sync timed out - likely slow cluster
				errMsg = fmt.Sprintf("Informers did not sync within %s - cluster may be slow or have many resources", InformerSyncTimeout.String())
				logging.Warn("Informer sync timeout", "timeout", InformerSyncTimeout.String())
			}

			// Store the error so screens can display it
			repo.typedInformersSyncError.Store(fmt.Errorf("%s", errMsg))
		}
	}()

	// Start background sync for dynamic informers (non-blocking)
	// Goroutines launched after repo creation so they can use repo.mu
	for gvr, informer := range dynamicInformers {
		resCfg := resourceRegistry[ResourceType(gvr.Resource)]

		// Launch sync goroutine for each resource (runs in background)
		go func(gvr schema.GroupVersionResource, informer cache.SharedIndexInformer, tier int) {
			resourceSyncStart := logging.Start(fmt.Sprintf("sync %s (tier %d)", gvr.Resource, tier))
			informerCtx, informerCancel := context.WithTimeout(ctx, InformerIndividualSyncTimeout)
			defer informerCancel()

			if cache.WaitForCacheSync(informerCtx.Done(), informer.HasSynced) {
				count := len(informer.GetStore().List())
				logging.EndWithCount(resourceSyncStart, count)
				logging.Debug("Dynamic informer synced", "resource", gvr.Resource, "tier", tier, "count", count)

				// Add lister now that sync is complete (use repo.mu for consistency)
				repo.mu.Lock()
				repo.dynamicListers[gvr] = dynamicFactory.ForResource(gvr).Lister()
				repo.mu.Unlock()

				// Update stats
				if stats, ok := repo.resourceStats[gvr]; ok {
					stats.Synced = true
					stats.Count = count
					stats.LastUpdate = time.Now()
				}
			} else {
				logging.End(resourceSyncStart)
				// Informer failed to sync - determine why
				errMsg := fmt.Sprintf("Failed to sync %s within %s", gvr.Resource, InformerIndividualSyncTimeout.String())

				// Try a test API call to get a better error message
				testCtx, testCancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer testCancel()

				// Try to list this specific resource
				_, err := repo.dynamicClient.Resource(gvr).List(testCtx, metav1.ListOptions{Limit: 1})
				if err != nil {
					// Specific API error (likely RBAC)
					errMsg = fmt.Sprintf("Cannot access %s: %v", gvr.Resource, err)
					logging.Warn("Dynamic informer sync failed", "resource", gvr.Resource, "tier", tier, "error", err)
				} else {
					// API call succeeded but sync timed out
					errMsg = fmt.Sprintf("%s informer timed out (cluster may be slow)", gvr.Resource)
					logging.Warn("Dynamic informer sync timeout", "resource", gvr.Resource, "tier", tier, "timeout", InformerIndividualSyncTimeout.String())
				}

				// Store error for screen to display
				repo.mu.Lock()
				repo.dynamicInformerErrors[gvr] = fmt.Errorf("%s", errMsg)
				repo.mu.Unlock()
			}
		}(gvr, informer, resCfg.Tier)
	}

	// Report completion (informers syncing in background)
	if progress != nil {
		progress <- ContextLoadProgress{
			Context: contextName,
			Message: "Starting informers in background",
			Phase:   PhaseComplete,
		}
	}

	logging.Info("Repository created, informers syncing in background")

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
	r.closed.Store(true) // Set flag BEFORE closing channel

	if r.cancel != nil {
		r.cancel()
	}
	if r.statsUpdateCh != nil {
		close(r.statsUpdateCh)
	}
	// Wait briefly for goroutine to exit (defensive)
	time.Sleep(10 * time.Millisecond)
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

// IsInformerSynced checks if informer for GVR is already registered and synced
func (r *InformerRepository) IsInformerSynced(gvr schema.GroupVersionResource) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, exists := r.dynamicListers[gvr]
	return exists
}

// AreTypedInformersReady checks if typed informers (pods, deployments, services, etc.) are synced
func (r *InformerRepository) AreTypedInformersReady() bool {
	return r.typedInformersReady.Load()
}

// GetTypedInformersSyncError returns the error if typed informers failed to sync
func (r *InformerRepository) GetTypedInformersSyncError() error {
	if errVal := r.typedInformersSyncError.Load(); errVal != nil {
		if err, ok := errVal.(error); ok {
			return err
		}
	}
	return nil
}

// GetDynamicInformerSyncError returns the error if a dynamic informer failed to sync
func (r *InformerRepository) GetDynamicInformerSyncError(gvr schema.GroupVersionResource) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.dynamicInformerErrors[gvr]
}

// EnsureCRInformer registers informer for CR on-demand if not already registered
func (r *InformerRepository) EnsureCRInformer(gvr schema.GroupVersionResource) error {
	r.mu.RLock()
	_, exists := r.dynamicListers[gvr]
	r.mu.RUnlock()

	if exists {
		return nil // Already registered
	}

	// Get informer (safe, idempotent - returns same informer if called multiple times)
	informer := r.dynamicFactory.ForResource(gvr).Informer()

	// Check if already synced (might have been loaded by another goroutine)
	if informer.HasSynced() {
		r.mu.Lock()
		r.dynamicListers[gvr] = r.dynamicFactory.ForResource(gvr).Lister()
		r.mu.Unlock()
		return nil
	}

	// Start factory (safe, idempotent)
	r.dynamicFactory.Start(r.ctx.Done())

	// Start background sync (non-blocking)
	// Note: Multiple calls are safe - informer is shared, sync happens once
	go func() {
		syncCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if cache.WaitForCacheSync(syncCtx.Done(), informer.HasSynced) {
			// Sync succeeded - register lister
			r.mu.Lock()
			r.dynamicListers[gvr] = r.dynamicFactory.ForResource(gvr).Lister()
			r.mu.Unlock()
			logging.Debug("On-demand resource synced", "resource", gvr.Resource)
		} else {
			// Sync failed or timed out
			logging.Warn("On-demand resource sync failed", "resource", gvr.Resource)
		}
	}()

	return nil
}

// EnsureResourceTypeInformer registers informer for resource type on-demand if not already registered
func (r *InformerRepository) EnsureResourceTypeInformer(resourceType ResourceType) error {
	// Get resource config
	config, exists := r.resources[resourceType]
	if !exists {
		return fmt.Errorf("unknown resource type: %v", resourceType)
	}

	// Check if already registered
	r.mu.RLock()
	_, alreadyExists := r.dynamicListers[config.GVR]
	r.mu.RUnlock()

	if alreadyExists {
		return nil // Already loaded
	}

	// Use EnsureCRInformer to register (it handles the locking and sync)
	return r.EnsureCRInformer(config.GVR)
}

// GetResourcesByGVR fetches resources using explicit GVR (for dynamic CRs)
func (r *InformerRepository) GetResourcesByGVR(
	gvr schema.GroupVersionResource,
	transform TransformFunc) ([]any, error) {

	r.mu.RLock()
	lister, exists := r.dynamicListers[gvr]
	r.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("informer not registered for %v", gvr)
	}

	// List resources
	objList, err := lister.List(labels.Everything())
	if err != nil {
		return nil, fmt.Errorf("failed to list %v: %w", gvr, err)
	}

	// Transform to typed objects (sort by Resource interface)
	resourceList := make([]Resource, 0, len(objList))
	for _, obj := range objList {
		unstr, ok := obj.(*unstructured.Unstructured)
		if !ok {
			continue
		}

		common := extractMetadata(unstr)
		transformed, err := transform(unstr, common)
		if err != nil {
			continue
		}

		// Type assert to Resource interface for sorting
		resource, ok := transformed.(Resource)
		if !ok {
			// Skip non-Resource types (shouldn't happen for CRs)
			continue
		}
		resourceList = append(resourceList, resource)
	}

	// Sort by age using Resource interface (newest first)
	sortByAge(resourceList)

	// Convert back to []any for API compatibility
	resources := make([]any, len(resourceList))
	for i, r := range resourceList {
		resources[i] = r
	}

	return resources, nil
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
