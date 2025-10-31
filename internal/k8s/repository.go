package k8s

import (
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// ResourceType identifies a Kubernetes resource type
type ResourceType string

const (
	ResourceTypePod                   ResourceType = "pods"
	ResourceTypeDeployment            ResourceType = "deployments"
	ResourceTypeService               ResourceType = "services"
	ResourceTypeConfigMap             ResourceType = "configmaps"
	ResourceTypeSecret                ResourceType = "secrets"
	ResourceTypeNamespace             ResourceType = "namespaces"
	ResourceTypeStatefulSet           ResourceType = "statefulsets"
	ResourceTypeDaemonSet             ResourceType = "daemonsets"
	ResourceTypeJob                   ResourceType = "jobs"
	ResourceTypeCronJob               ResourceType = "cronjobs"
	ResourceTypeNode                  ResourceType = "nodes"
	ResourceTypeReplicaSet            ResourceType = "replicasets"
	ResourceTypePersistentVolumeClaim ResourceType = "persistentvolumeclaims"
	ResourceTypeIngress               ResourceType = "ingresses"
	ResourceTypeEndpoints             ResourceType = "endpoints"
	ResourceTypeHPA                   ResourceType = "horizontalpodautoscalers"
	ResourceTypeCRD                   ResourceType = "customresourcedefinitions"
	ResourceTypeContext               ResourceType = "contexts"
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
		ResourceTypeCRD:         {Group: "apiextensions.k8s.io", Version: "v1", Resource: "customresourcedefinitions"},
	}

	gvr, ok := gvrMap[resourceType]
	return gvr, ok
}

// ResourceConfig defines configuration for a resource type
type ResourceConfig struct {
	GVR        schema.GroupVersionResource
	Name       string
	Namespaced bool
	Tier       int // 0=on-demand only, 1=critical (block UI), 2=background, 3=deferred
	Transform  TransformFunc
}

// TransformFunc converts an unstructured resource to a typed struct
// The ResourceMetadata parameter contains pre-extracted common fields (name, namespace, age, createdAt)
// to avoid redundant field extraction in every transform function
type TransformFunc func(*unstructured.Unstructured, ResourceMetadata) (any, error)

// ResourceStats holds statistics for a resource type
type ResourceStats struct {
	ResourceType ResourceType
	Count        int
	LastUpdate   time.Time
	AddEvents    int64
	UpdateEvents int64
	DeleteEvents int64
	Synced       bool
	MemoryBytes  int64 // Approximate
}

// Repository provides access to Kubernetes resources
type Repository interface {
	// Generic resource access (config-driven)
	GetResources(resourceType ResourceType) ([]any, error)

	// Dynamic CRD instance access (for on-demand informers)
	GetResourcesByGVR(gvr schema.GroupVersionResource, transform TransformFunc) ([]any, error)
	EnsureCRInformer(gvr schema.GroupVersionResource) error
	IsInformerSynced(gvr schema.GroupVersionResource) bool
	AreTypedInformersReady() bool // Check if typed informers (pods, deployments, services, etc.) are synced
	GetTypedInformersSyncError() error // Get error if typed informers failed to sync
	GetDynamicInformerSyncError(gvr schema.GroupVersionResource) error // Get error if dynamic informer failed to sync
	// Ensure informer for resource type is loaded (for on-demand Tier 0 resources)
	EnsureResourceTypeInformer(resourceType ResourceType) error

	// Typed convenience methods (preserved for compatibility)
	GetPods() ([]Pod, error)
	GetDeployments() ([]Deployment, error)
	GetServices() ([]Service, error)

	// Filtered queries for contextual navigation
	GetPodsForDeployment(namespace, name string) ([]Pod, error)
	GetPodsOnNode(nodeName string) ([]Pod, error)
	GetPodsForService(namespace, name string) ([]Pod, error)
	GetPodsForStatefulSet(namespace, name string) ([]Pod, error)
	GetPodsForDaemonSet(namespace, name string) ([]Pod, error)
	GetPodsForJob(namespace, name string) ([]Pod, error)
	GetJobsForCronJob(namespace, name string) ([]Job, error)
	GetPodsForNamespace(namespace string) ([]Pod, error)
	GetPodsUsingConfigMap(namespace, name string) ([]Pod, error)
	GetPodsUsingSecret(namespace, name string) ([]Pod, error)
	GetPodsForReplicaSet(namespace, name string) ([]Pod, error)
	GetReplicaSetsForDeployment(namespace, name string) ([]ReplicaSet, error)
	GetPodsForPVC(namespace, name string) ([]Pod, error)

	// Resource detail commands (using kubectl libraries)
	GetResourceYAML(gvr schema.GroupVersionResource, namespace, name string) (string, error)
	DescribeResource(gvr schema.GroupVersionResource, namespace, name string) (string, error)

	// Kubeconfig and context (for kubectl subprocess commands)
	GetKubeconfig() string
	GetContext() string

	// Statistics (for system resources screen)
	GetResourceStats() []ResourceStats

	// Context management (for repository pool)
	SwitchContext(contextName string, progress chan<- ContextLoadProgress) error
	GetAllContexts() []ContextWithStatus
	GetActiveContext() string
	RetryFailedContext(contextName string, progress chan<- ContextLoadProgress) error
	GetContexts() ([]Context, error)

	Close()
}

// Resource types are now defined in repository_types.go
