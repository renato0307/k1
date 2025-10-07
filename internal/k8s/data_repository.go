package k8s

import (
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
)

// DataRepository provides access to Kubernetes resource data
type DataRepository struct {
	manager   *InformerManager
	resources map[ResourceType]ResourceConfig
}

// NewDataRepository creates a new data repository
func NewDataRepository(manager *InformerManager) *DataRepository {
	return &DataRepository{
		manager:   manager,
		resources: getResourceRegistry(),
	}
}

// GetPods returns all pods from the informer cache
func (d *DataRepository) GetPods() ([]Pod, error) {
	podList, err := d.manager.GetPodLister().List(labels.Everything())
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
	sortByCreationTime(pods, func(p Pod) time.Time { return p.CreatedAt }, func(p Pod) string { return p.Name })

	return pods, nil
}

// GetDeployments returns all deployments from the informer cache
func (d *DataRepository) GetDeployments() ([]Deployment, error) {
	deploymentList, err := d.manager.GetDeploymentLister().List(labels.Everything())
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
	sortByCreationTime(deployments, func(d Deployment) time.Time { return d.CreatedAt }, func(d Deployment) string { return d.Name })

	return deployments, nil
}

// GetServices returns all services from the informer cache
func (d *DataRepository) GetServices() ([]Service, error) {
	serviceList, err := d.manager.GetServiceLister().List(labels.Everything())
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
			// Show all external IPs comma-separated
			externalIP = svc.Spec.ExternalIPs[0]
			for i := 1; i < len(svc.Spec.ExternalIPs); i++ {
				externalIP += "," + svc.Spec.ExternalIPs[i]
			}
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
		portsStr := fmt.Sprintf("%v", ports)
		if len(ports) == 0 {
			portsStr = "<none>"
		} else if len(ports) == 1 {
			portsStr = ports[0]
		} else {
			portsStr = ports[0]
			for i := 1; i < len(ports); i++ {
				portsStr += "," + ports[i]
			}
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
	sortByCreationTime(services, func(s Service) time.Time { return s.CreatedAt }, func(s Service) string { return s.Name })

	return services, nil
}

// GetResources returns all resources of the specified type using dynamic informers
func (d *DataRepository) GetResources(resourceType ResourceType) ([]any, error) {
	// Get resource config
	config, ok := d.resources[resourceType]
	if !ok {
		return nil, fmt.Errorf("unknown resource type: %s", resourceType)
	}

	// Get dynamic lister for this resource
	lister, ok := d.manager.GetDynamicLister(config.GVR)
	if !ok {
		return nil, fmt.Errorf("informer not initialized for resource type: %s", resourceType)
	}

	// List resources from cache
	objList, err := lister.List(labels.Everything())
	if err != nil {
		return nil, fmt.Errorf("failed to list %s: %w", resourceType, err)
	}

	// Transform unstructured objects to typed structs
	results := make([]any, 0, len(objList))
	for _, obj := range objList {
		unstr, ok := obj.(*unstructured.Unstructured)
		if !ok {
			continue
		}

		// Extract common fields once per resource (optimization)
		common := extractCommonFields(unstr)

		transformed, err := config.Transform(unstr, common)
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
