package k8s

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	v1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
)

// InformerRepository implements Repository using Kubernetes informers
type InformerRepository struct {
	clientset *kubernetes.Clientset
	factory   informers.SharedInformerFactory
	podLister v1listers.PodLister
	ctx       context.Context
	cancel    context.CancelFunc
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

	// Create context for lifecycle management
	ctx, cancel := context.WithCancel(context.Background())

	// Start informers in background
	factory.Start(ctx.Done())

	// Wait for cache to sync
	if !cache.WaitForCacheSync(ctx.Done(), podInformer.HasSynced) {
		cancel()
		return nil, fmt.Errorf("failed to sync pod cache")
	}

	return &InformerRepository{
		clientset: clientset,
		factory:   factory,
		podLister: podLister,
		ctx:       ctx,
		cancel:    cancel,
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

// GetDeployments is not yet implemented (Phase 2+)
func (r *InformerRepository) GetDeployments() ([]Deployment, error) {
	return []Deployment{}, nil
}

// GetServices is not yet implemented (Phase 2+)
func (r *InformerRepository) GetServices() ([]Service, error) {
	return []Service{}, nil
}

// Close stops the informers and cleans up resources
func (r *InformerRepository) Close() {
	if r.cancel != nil {
		r.cancel()
	}
}
