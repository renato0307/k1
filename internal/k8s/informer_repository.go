package k8s

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

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

// InformerRepository implements Repository using Kubernetes informers
type InformerRepository struct {
	// Typed client and informers (legacy, preserved for compatibility)
	clientset        *kubernetes.Clientset
	factory          informers.SharedInformerFactory
	podLister        v1listers.PodLister
	deploymentLister appsv1listers.DeploymentLister
	serviceLister    v1listers.ServiceLister

	// Dynamic client and informers (config-driven approach)
	dynamicClient  dynamic.Interface
	dynamicFactory dynamicinformer.DynamicSharedInformerFactory
	resources      map[ResourceType]ResourceConfig
	dynamicListers map[schema.GroupVersionResource]cache.GenericLister

	ctx    context.Context
	cancel context.CancelFunc
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

	// Create shared informer factories with 30 second resync period
	factory := informers.NewSharedInformerFactory(clientset, 30*time.Second)
	dynamicFactory := dynamicinformer.NewDynamicSharedInformerFactory(dynamicClient, 30*time.Second)

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
	dynamicInformers := []cache.SharedIndexInformer{}

	for _, resCfg := range resourceRegistry {
		informer := dynamicFactory.ForResource(resCfg.GVR).Informer()
		dynamicListers[resCfg.GVR] = dynamicFactory.ForResource(resCfg.GVR).Lister()
		dynamicInformers = append(dynamicInformers, informer)
	}

	// Create context for lifecycle management
	ctx, cancel := context.WithCancel(context.Background())

	// Start informers in background
	factory.Start(ctx.Done())
	dynamicFactory.Start(ctx.Done())

	// Wait for all caches to sync (both typed and dynamic)
	allInformers := []cache.InformerSynced{
		podInformer.HasSynced,
		deploymentInformer.HasSynced,
		serviceInformer.HasSynced,
	}
	for _, inf := range dynamicInformers {
		allInformers = append(allInformers, inf.HasSynced)
	}

	if !cache.WaitForCacheSync(ctx.Done(), allInformers...) {
		cancel()
		return nil, fmt.Errorf("failed to sync caches")
	}

	return &InformerRepository{
		clientset:        clientset,
		factory:          factory,
		podLister:        podLister,
		deploymentLister: deploymentLister,
		serviceLister:    serviceLister,
		dynamicClient:    dynamicClient,
		dynamicFactory:   dynamicFactory,
		resources:        resourceRegistry,
		dynamicListers:   dynamicListers,
		ctx:              ctx,
		cancel:           cancel,
	}, nil
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
			Namespace: pod.Namespace,
			Name:      pod.Name,
			Ready:     readyStatus,
			Status:    status,
			Restarts:  totalRestarts,
			Age:       age,
			Node:      node,
			IP:        ip,
		})
	}

	// Sort by age (newest first), then by name for stable sort
	sort.Slice(pods, func(i, j int) bool {
		if pods[i].Age != pods[j].Age {
			return pods[i].Age < pods[j].Age
		}
		return pods[i].Name < pods[j].Name
	})

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
			Namespace: deploy.Namespace,
			Name:      deploy.Name,
			Ready:     readyStatus,
			UpToDate:  upToDate,
			Available: available,
			Age:       age,
		})
	}

	// Sort by age (newest first), then by name for stable sort
	sort.Slice(deployments, func(i, j int) bool {
		if deployments[i].Age != deployments[j].Age {
			return deployments[i].Age < deployments[j].Age
		}
		return deployments[i].Name < deployments[j].Name
	})

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
			Namespace:  svc.Namespace,
			Name:       svc.Name,
			Type:       string(svc.Spec.Type),
			ClusterIP:  clusterIP,
			ExternalIP: externalIP,
			Ports:      portsStr,
			Age:        age,
		})
	}

	// Sort by age (newest first), then by name for stable sort
	sort.Slice(services, func(i, j int) bool {
		if services[i].Age != services[j].Age {
			return services[i].Age < services[j].Age
		}
		return services[i].Name < services[j].Name
	})

	return services, nil
}

// Close stops the informers and cleans up resources
func (r *InformerRepository) Close() {
	if r.cancel != nil {
		r.cancel()
	}
}

// GetResources returns all resources of the specified type using dynamic informers
func (r *InformerRepository) GetResources(resourceType ResourceType) ([]interface{}, error) {
	// Get resource config
	config, ok := r.resources[resourceType]
	if !ok {
		return nil, fmt.Errorf("unknown resource type: %s", resourceType)
	}

	// Get dynamic lister for this resource
	lister, ok := r.dynamicListers[config.GVR]
	if !ok {
		return nil, fmt.Errorf("informer not initialized for resource type: %s", resourceType)
	}

	// List resources from cache
	objList, err := lister.List(labels.Everything())
	if err != nil {
		return nil, fmt.Errorf("failed to list %s: %w", resourceType, err)
	}

	// Transform unstructured objects to typed structs
	results := make([]interface{}, 0, len(objList))
	for _, obj := range objList {
		unstr, ok := obj.(*unstructured.Unstructured)
		if !ok {
			continue
		}

		transformed, err := config.Transform(unstr)
		if err != nil {
			// Log error but continue (partial results better than nothing)
			continue
		}
		results = append(results, transformed)
	}

	// Sort results by age (newest first) if they have Age field
	sortByAge(results)

	return results, nil
}

// sortByAge sorts resources by Age field if present
func sortByAge(items []interface{}) {
	sort.Slice(items, func(i, j int) bool {
		// Try to extract Age from both items
		ageI := extractAge(items[i])
		ageJ := extractAge(items[j])

		if ageI != ageJ {
			return ageI < ageJ
		}

		// Fall back to name comparison
		nameI := extractName(items[i])
		nameJ := extractName(items[j])
		return nameI < nameJ
	})
}

// extractAge tries to extract Age field from an interface{}
func extractAge(item interface{}) time.Duration {
	switch v := item.(type) {
	case Pod:
		return v.Age
	case Deployment:
		return v.Age
	case Service:
		return v.Age
	case ConfigMap:
		return v.Age
	case Secret:
		return v.Age
	case Namespace:
		return v.Age
	case StatefulSet:
		return v.Age
	case DaemonSet:
		return v.Age
	case Job:
		return v.Age
	case CronJob:
		return v.Age
	case Node:
		return v.Age
	default:
		return 0
	}
}

// extractName tries to extract Name field from an interface{}
func extractName(item interface{}) string {
	switch v := item.(type) {
	case Pod:
		return v.Name
	case Deployment:
		return v.Name
	case Service:
		return v.Name
	case ConfigMap:
		return v.Name
	case Secret:
		return v.Name
	case Namespace:
		return v.Name
	case StatefulSet:
		return v.Name
	case DaemonSet:
		return v.Name
	case Job:
		return v.Name
	case CronJob:
		return v.Name
	case Node:
		return v.Name
	default:
		return ""
	}
}
