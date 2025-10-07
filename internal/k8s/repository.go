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

// GetGVRForResourceType returns the GroupVersionResource for a resource type
func GetGVRForResourceType(resourceType ResourceType) (schema.GroupVersionResource, bool) {
	gvrMap := map[ResourceType]schema.GroupVersionResource{
		ResourceTypePod:         {Group: "", Version: "v1", Resource: "pods"},
		ResourceTypeDeployment:  {Group: "apps", Version: "v1", Resource: "deployments"},
		ResourceTypeService:     {Group: "", Version: "v1", Resource: "services"},
		ResourceTypeConfigMap:   {Group: "", Version: "v1", Resource: "configmaps"},
		ResourceTypeSecret:      {Group: "", Version: "v1", Resource: "secrets"},
		ResourceTypeNamespace:   {Group: "", Version: "v1", Resource: "namespaces"},
		ResourceTypeStatefulSet: {Group: "apps", Version: "v1", Resource: "statefulsets"},
		ResourceTypeDaemonSet:   {Group: "apps", Version: "v1", Resource: "daemonsets"},
		ResourceTypeJob:         {Group: "batch", Version: "v1", Resource: "jobs"},
		ResourceTypeCronJob:     {Group: "batch", Version: "v1", Resource: "cronjobs"},
		ResourceTypeNode:        {Group: "", Version: "v1", Resource: "nodes"},
	}

	gvr, ok := gvrMap[resourceType]
	return gvr, ok
}

// commonFields holds fields that are common to all resource types
// This is extracted once per resource to avoid redundant field extraction
type commonFields struct {
	Namespace string
	Name      string
	Age       time.Duration
	CreatedAt time.Time
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
// The commonFields parameter contains pre-extracted common fields (name, namespace, age, createdAt)
// to avoid redundant field extraction in every transform function
type TransformFunc func(*unstructured.Unstructured, commonFields) (any, error)

// Repository provides access to Kubernetes resources
type Repository interface {
	// Generic resource access (config-driven)
	GetResources(resourceType ResourceType) ([]any, error)

	// Typed convenience methods (preserved for compatibility)
	GetPods() ([]Pod, error)
	GetDeployments() ([]Deployment, error)
	GetServices() ([]Service, error)

	// Filtered queries for contextual navigation
	GetPodsForDeployment(namespace, name string) ([]Pod, error)
	GetPodsOnNode(nodeName string) ([]Pod, error)
	GetPodsForService(namespace, name string) ([]Pod, error)

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
