package k8s

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

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

// InformerManager manages Kubernetes informer lifecycle
type InformerManager struct {
	// Clients
	clientset     *kubernetes.Clientset
	dynamicClient dynamic.Interface

	// Factories
	factory        informers.SharedInformerFactory
	dynamicFactory dynamicinformer.DynamicSharedInformerFactory

	// Listers (exposed for DataRepository/ResourceFormatter)
	podLister        v1listers.PodLister
	deploymentLister appsv1listers.DeploymentLister
	serviceLister    v1listers.ServiceLister
	dynamicListers   map[schema.GroupVersionResource]cache.GenericLister

	// Lifecycle
	ctx    context.Context
	cancel context.CancelFunc

	// Config
	kubeconfig  string
	contextName string
}

// NewInformerManager creates a new informer manager and starts all informers
func NewInformerManager(kubeconfig, contextName string) (*InformerManager, error) {
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

	// Try typed informers first (pods, deployments, services)
	typedSynced := cache.WaitForCacheSync(syncCtx.Done(),
		podInformer.HasSynced,
		deploymentInformer.HasSynced,
		serviceInformer.HasSynced,
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

	return &InformerManager{
		clientset:        clientset,
		dynamicClient:    dynamicClient,
		factory:          factory,
		dynamicFactory:   dynamicFactory,
		podLister:        podLister,
		deploymentLister: deploymentLister,
		serviceLister:    serviceLister,
		dynamicListers:   dynamicListers,
		ctx:              ctx,
		cancel:           cancel,
		kubeconfig:       kubeconfig,
		contextName:      contextName,
	}, nil
}

// Close stops the informers and cleans up resources
func (m *InformerManager) Close() {
	if m.cancel != nil {
		m.cancel()
	}
}

// GetKubeconfig returns the kubeconfig path
func (m *InformerManager) GetKubeconfig() string {
	return m.kubeconfig
}

// GetContext returns the context name
func (m *InformerManager) GetContext() string {
	return m.contextName
}

// GetPodLister returns the pod lister
func (m *InformerManager) GetPodLister() v1listers.PodLister {
	return m.podLister
}

// GetDeploymentLister returns the deployment lister
func (m *InformerManager) GetDeploymentLister() appsv1listers.DeploymentLister {
	return m.deploymentLister
}

// GetServiceLister returns the service lister
func (m *InformerManager) GetServiceLister() v1listers.ServiceLister {
	return m.serviceLister
}

// GetDynamicLister returns the dynamic lister for a given GVR
func (m *InformerManager) GetDynamicLister(gvr schema.GroupVersionResource) (cache.GenericLister, bool) {
	lister, ok := m.dynamicListers[gvr]
	return lister, ok
}

// GetDynamicClient returns the dynamic client
func (m *InformerManager) GetDynamicClient() dynamic.Interface {
	return m.dynamicClient
}

// GetClientset returns the clientset
func (m *InformerManager) GetClientset() *kubernetes.Clientset {
	return m.clientset
}

// GetContext returns the context
func (m *InformerManager) GetCtx() context.Context {
	return m.ctx
}
