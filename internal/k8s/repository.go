package k8s

import (
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// ResourceType identifies a Kubernetes resource type
type ResourceType string

const (
	ResourceTypePod         ResourceType = "pods"
	ResourceTypeDeployment  ResourceType = "deployments"
	ResourceTypeService     ResourceType = "services"
	ResourceTypeConfigMap   ResourceType = "configmaps"
	ResourceTypeSecret      ResourceType = "secrets"
	ResourceTypeNamespace   ResourceType = "namespaces"
	ResourceTypeStatefulSet ResourceType = "statefulsets"
	ResourceTypeDaemonSet   ResourceType = "daemonsets"
	ResourceTypeJob         ResourceType = "jobs"
	ResourceTypeCronJob     ResourceType = "cronjobs"
	ResourceTypeNode        ResourceType = "nodes"
)

// GetGVRForResourceType returns the GroupVersionResource for a resource type string
func GetGVRForResourceType(resourceType string) (schema.GroupVersionResource, bool) {
	gvrMap := map[string]schema.GroupVersionResource{
		"pods":         {Group: "", Version: "v1", Resource: "pods"},
		"deployments":  {Group: "apps", Version: "v1", Resource: "deployments"},
		"services":     {Group: "", Version: "v1", Resource: "services"},
		"configmaps":   {Group: "", Version: "v1", Resource: "configmaps"},
		"secrets":      {Group: "", Version: "v1", Resource: "secrets"},
		"namespaces":   {Group: "", Version: "v1", Resource: "namespaces"},
		"statefulsets": {Group: "apps", Version: "v1", Resource: "statefulsets"},
		"daemonsets":   {Group: "apps", Version: "v1", Resource: "daemonsets"},
		"jobs":         {Group: "batch", Version: "v1", Resource: "jobs"},
		"cronjobs":     {Group: "batch", Version: "v1", Resource: "cronjobs"},
		"nodes":        {Group: "", Version: "v1", Resource: "nodes"},
	}

	gvr, ok := gvrMap[resourceType]
	return gvr, ok
}

// ResourceConfig defines configuration for a resource type
type ResourceConfig struct {
	GVR        schema.GroupVersionResource
	Name       string
	Namespaced bool
	Tier       int // 1=critical (block UI), 2=background, 3=deferred
	Transform  TransformFunc
}

// TransformFunc converts an unstructured resource to a typed struct
type TransformFunc func(*unstructured.Unstructured) (interface{}, error)

// Repository provides access to Kubernetes resources
type Repository interface {
	// Generic resource access (config-driven)
	GetResources(resourceType ResourceType) ([]interface{}, error)

	// Typed convenience methods (preserved for compatibility)
	GetPods() ([]Pod, error)
	GetDeployments() ([]Deployment, error)
	GetServices() ([]Service, error)

	// Resource detail commands (using kubectl libraries)
	GetResourceYAML(gvr schema.GroupVersionResource, namespace, name string) (string, error)
	DescribeResource(gvr schema.GroupVersionResource, namespace, name string) (string, error)

	// Kubeconfig and context (for kubectl subprocess commands)
	GetKubeconfig() string
	GetContext() string

	Close()
}

// Pod represents a Kubernetes pod
type Pod struct {
	Namespace string
	Name      string
	Ready     string
	Status    string
	Restarts  int32
	Age       time.Duration
	CreatedAt time.Time // Stable creation timestamp for sorting
	Node      string
	IP        string
}

// Deployment represents a Kubernetes deployment
type Deployment struct {
	Namespace string
	Name      string
	Ready     string
	UpToDate  int32
	Available int32
	Age       time.Duration
	CreatedAt time.Time // Stable creation timestamp for sorting
}

// Service represents a Kubernetes service
type Service struct {
	Namespace  string
	Name       string
	Type       string
	ClusterIP  string
	ExternalIP string
	Ports      string
	Age        time.Duration
	CreatedAt  time.Time // Stable creation timestamp for sorting
}

// ConfigMap represents a Kubernetes configmap
type ConfigMap struct {
	Namespace string
	Name      string
	Data      int // Number of data items
	Age       time.Duration
	CreatedAt time.Time // Stable creation timestamp for sorting
}

// Secret represents a Kubernetes secret
type Secret struct {
	Namespace string
	Name      string
	Type      string
	Data      int // Number of data items
	Age       time.Duration
	CreatedAt time.Time // Stable creation timestamp for sorting
}

// Namespace represents a Kubernetes namespace
type Namespace struct {
	Name      string
	Status    string
	Age       time.Duration
	CreatedAt time.Time // Stable creation timestamp for sorting
}

// StatefulSet represents a Kubernetes statefulset
type StatefulSet struct {
	Namespace string
	Name      string
	Ready     string
	Age       time.Duration
	CreatedAt time.Time // Stable creation timestamp for sorting
}

// DaemonSet represents a Kubernetes daemonset
type DaemonSet struct {
	Namespace string
	Name      string
	Desired   int32
	Current   int32
	Ready     int32
	UpToDate  int32
	Available int32
	Age       time.Duration
	CreatedAt time.Time // Stable creation timestamp for sorting
}

// Job represents a Kubernetes job
type Job struct {
	Namespace   string
	Name        string
	Completions string
	Duration    time.Duration
	Age         time.Duration
	CreatedAt   time.Time // Stable creation timestamp for sorting
}

// CronJob represents a Kubernetes cronjob
type CronJob struct {
	Namespace    string
	Name         string
	Schedule     string
	Suspend      bool
	Active       int32
	LastSchedule time.Duration
	Age          time.Duration
	CreatedAt    time.Time // Stable creation timestamp for sorting
}

// Node represents a Kubernetes node
type Node struct {
	Name         string
	Status       string
	Roles        string
	Age          time.Duration
	CreatedAt    time.Time // Stable creation timestamp for sorting
	Version      string
	Hostname     string
	InstanceType string
	Zone         string
	NodePool     string
	OSImage      string
}

// DummyRepository provides fake data for prototyping
type DummyRepository struct{}

func NewDummyRepository() *DummyRepository {
	return &DummyRepository{}
}

func (r *DummyRepository) GetPods() ([]Pod, error) {
	return []Pod{
		{
			Namespace: "default",
			Name:      "nginx-deployment-7d64f8d9c8-abc12",
			Ready:     "1/1",
			Status:    "Running",
			Restarts:  0,
			Age:       24 * time.Hour,
			Node:      "node-1",
			IP:        "10.244.1.5",
		},
		{
			Namespace: "default",
			Name:      "nginx-deployment-7d64f8d9c8-def34",
			Ready:     "1/1",
			Status:    "Running",
			Restarts:  2,
			Age:       24 * time.Hour,
			Node:      "node-2",
			IP:        "10.244.2.3",
		},
		{
			Namespace: "kube-system",
			Name:      "coredns-5d78c9869d-xyz89",
			Ready:     "1/1",
			Status:    "Running",
			Restarts:  0,
			Age:       168 * time.Hour,
			Node:      "node-1",
			IP:        "10.244.1.2",
		},
		{
			Namespace: "production",
			Name:      "api-server-6b9f8c7d5e-qwert",
			Ready:     "0/1",
			Status:    "CrashLoopBackOff",
			Restarts:  15,
			Age:       2 * time.Hour,
			Node:      "node-3",
			IP:        "10.244.3.7",
		},
	}, nil
}

func (r *DummyRepository) GetDeployments() ([]Deployment, error) {
	return []Deployment{
		{
			Namespace: "default",
			Name:      "nginx-deployment",
			Ready:     "2/2",
			UpToDate:  2,
			Available: 2,
			Age:       24 * time.Hour,
		},
		{
			Namespace: "kube-system",
			Name:      "coredns",
			Ready:     "2/2",
			UpToDate:  2,
			Available: 2,
			Age:       168 * time.Hour,
		},
		{
			Namespace: "production",
			Name:      "api-server",
			Ready:     "1/3",
			UpToDate:  1,
			Available: 1,
			Age:       48 * time.Hour,
		},
	}, nil
}

func (r *DummyRepository) GetServices() ([]Service, error) {
	return []Service{
		{
			Namespace:  "default",
			Name:       "kubernetes",
			Type:       "ClusterIP",
			ClusterIP:  "10.96.0.1",
			ExternalIP: "<none>",
			Ports:      "443/TCP",
			Age:        168 * time.Hour,
		},
		{
			Namespace:  "default",
			Name:       "nginx-service",
			Type:       "LoadBalancer",
			ClusterIP:  "10.96.10.5",
			ExternalIP: "203.0.113.45",
			Ports:      "80/TCP,443/TCP",
			Age:        24 * time.Hour,
		},
		{
			Namespace:  "production",
			Name:       "api-service",
			Type:       "ClusterIP",
			ClusterIP:  "10.96.20.10",
			ExternalIP: "<none>",
			Ports:      "8080/TCP",
			Age:        48 * time.Hour,
		},
	}, nil
}

func (r *DummyRepository) GetResourceYAML(gvr schema.GroupVersionResource, namespace, name string) (string, error) {
	// Return dummy YAML for development
	return `apiVersion: v1
kind: Pod
metadata:
  name: ` + name + `
  namespace: ` + namespace + `
status:
  phase: Running`, nil
}

func (r *DummyRepository) DescribeResource(gvr schema.GroupVersionResource, namespace, name string) (string, error) {
	// Return dummy describe output for development
	return `Name:         ` + name + `
Namespace:    ` + namespace + `
Status:       Running
(Dummy data - connect to real cluster for actual describe output)`, nil
}

func (r *DummyRepository) Close() {
	// No-op for dummy repository
}

func (r *DummyRepository) GetKubeconfig() string {
	return ""
}

func (r *DummyRepository) GetContext() string {
	return ""
}

func (r *DummyRepository) GetResources(resourceType ResourceType) ([]interface{}, error) {
	switch resourceType {
	case ResourceTypePod:
		pods, err := r.GetPods()
		if err != nil {
			return nil, err
		}
		result := make([]interface{}, len(pods))
		for i, p := range pods {
			result[i] = p
		}
		return result, nil
	case ResourceTypeDeployment:
		deployments, err := r.GetDeployments()
		if err != nil {
			return nil, err
		}
		result := make([]interface{}, len(deployments))
		for i, d := range deployments {
			result[i] = d
		}
		return result, nil
	case ResourceTypeService:
		services, err := r.GetServices()
		if err != nil {
			return nil, err
		}
		result := make([]interface{}, len(services))
		for i, s := range services {
			result[i] = s
		}
		return result, nil
	default:
		return []interface{}{}, nil
	}
}
