package k8s

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
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

	// Kubeconfig and context (for kubectl subprocess commands)
	kubeconfig  string
	contextName string

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
	syncTimeout := 10 * time.Second
	syncCtx, syncCancel := context.WithTimeout(ctx, syncTimeout)
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
		informerCtx, informerCancel := context.WithTimeout(ctx, 5*time.Second)
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
		kubeconfig:       kubeconfig,
		contextName:      contextName,
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
			CreatedAt: pod.CreationTimestamp.Time,
			Node:      node,
			IP:        ip,
		})
	}

	// Sort by creation time (newest first), then by name for stable sort
	sort.Slice(pods, func(i, j int) bool {
		if !pods[i].CreatedAt.Equal(pods[j].CreatedAt) {
			return pods[i].CreatedAt.After(pods[j].CreatedAt) // Newer first
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
			CreatedAt: deploy.CreationTimestamp.Time,
		})
	}

	// Sort by creation time (newest first), then by name for stable sort
	sort.Slice(deployments, func(i, j int) bool {
		if !deployments[i].CreatedAt.Equal(deployments[j].CreatedAt) {
			return deployments[i].CreatedAt.After(deployments[j].CreatedAt) // Newer first
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
			CreatedAt:  svc.CreationTimestamp.Time,
		})
	}

	// Sort by creation time (newest first), then by name for stable sort
	sort.Slice(services, func(i, j int) bool {
		if !services[i].CreatedAt.Equal(services[j].CreatedAt) {
			return services[i].CreatedAt.After(services[j].CreatedAt) // Newer first
		}
		return services[i].Name < services[j].Name
	})

	return services, nil
}

// GetResourceYAML returns YAML representation of a resource using kubectl YAMLPrinter
func (r *InformerRepository) GetResourceYAML(gvr schema.GroupVersionResource, namespace, name string) (string, error) {
	// Get resource from dynamic informer cache
	lister, ok := r.dynamicListers[gvr]
	if !ok {
		return "", fmt.Errorf("informer not initialized for resource %s", gvr)
	}

	var runtimeObj interface{}
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

	var runtimeObj interface{}
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

// Close stops the informers and cleans up resources
func (r *InformerRepository) Close() {
	if r.cancel != nil {
		r.cancel()
	}
}

// GetKubeconfig returns the kubeconfig path
func (r *InformerRepository) GetKubeconfig() string {
	return r.kubeconfig
}

// GetContext returns the context name
func (r *InformerRepository) GetContext() string {
	return r.contextName
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

// sortByAge sorts resources by CreatedAt field if present (newest first)
func sortByAge(items []interface{}) {
	sort.Slice(items, func(i, j int) bool {
		// Try to extract CreatedAt from both items
		createdI := extractCreatedAt(items[i])
		createdJ := extractCreatedAt(items[j])

		if !createdI.Equal(createdJ) {
			return createdI.After(createdJ) // Newer first
		}

		// Fall back to name comparison
		nameI := extractName(items[i])
		nameJ := extractName(items[j])
		return nameI < nameJ
	})
}

// extractCreatedAt tries to extract CreatedAt field from an interface{}
func extractCreatedAt(item interface{}) time.Time {
	switch v := item.(type) {
	case Pod:
		return v.CreatedAt
	case Deployment:
		return v.CreatedAt
	case Service:
		return v.CreatedAt
	case ConfigMap:
		return v.CreatedAt
	case Secret:
		return v.CreatedAt
	case Namespace:
		return v.CreatedAt
	case StatefulSet:
		return v.CreatedAt
	case DaemonSet:
		return v.CreatedAt
	case Job:
		return v.CreatedAt
	case CronJob:
		return v.CreatedAt
	case Node:
		return v.CreatedAt
	default:
		return time.Time{} // Zero time
	}
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
