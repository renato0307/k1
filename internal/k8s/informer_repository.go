package k8s

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	appsv1listers "k8s.io/client-go/listers/apps/v1"
	v1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
)

// InformerRepository implements Repository using Kubernetes informers
type InformerRepository struct {
	clientset        *kubernetes.Clientset
	factory          informers.SharedInformerFactory
	podLister        v1listers.PodLister
	deploymentLister appsv1listers.DeploymentLister
	serviceLister    v1listers.ServiceLister
	ctx              context.Context
	cancel           context.CancelFunc
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

	// Create shared informer factory with 30 second resync period
	factory := informers.NewSharedInformerFactory(clientset, 30*time.Second)

	// Create pod informer and lister
	podInformer := factory.Core().V1().Pods().Informer()
	podLister := factory.Core().V1().Pods().Lister()

	// Create deployment informer and lister
	deploymentInformer := factory.Apps().V1().Deployments().Informer()
	deploymentLister := factory.Apps().V1().Deployments().Lister()

	// Create service informer and lister
	serviceInformer := factory.Core().V1().Services().Informer()
	serviceLister := factory.Core().V1().Services().Lister()

	// Create context for lifecycle management
	ctx, cancel := context.WithCancel(context.Background())

	// Start informers in background
	factory.Start(ctx.Done())

	// Wait for all caches to sync
	if !cache.WaitForCacheSync(ctx.Done(),
		podInformer.HasSynced,
		deploymentInformer.HasSynced,
		serviceInformer.HasSynced) {
		cancel()
		return nil, fmt.Errorf("failed to sync caches")
	}

	return &InformerRepository{
		clientset:        clientset,
		factory:          factory,
		podLister:        podLister,
		deploymentLister: deploymentLister,
		serviceLister:    serviceLister,
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
