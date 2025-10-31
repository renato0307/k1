package k8s

import "time"

// Resource represents any Kubernetes resource with common fields
// All resource types must implement this interface for sorting and polymorphic operations
type Resource interface {
	GetNamespace() string // "" for cluster-scoped resources
	GetName() string
	GetAge() time.Duration
	GetCreatedAt() time.Time
}

// ResourceMetadata contains common fields shared by all Kubernetes resources
// Embed this in resource structs to automatically implement Resource interface
type ResourceMetadata struct {
	Namespace string
	Name      string
	Age       time.Duration
	CreatedAt time.Time
}

// ResourceMetadata implements Resource interface
func (r ResourceMetadata) GetNamespace() string    { return r.Namespace }
func (r ResourceMetadata) GetName() string         { return r.Name }
func (r ResourceMetadata) GetAge() time.Duration   { return r.Age }
func (r ResourceMetadata) GetCreatedAt() time.Time { return r.CreatedAt }

// Pod represents a Kubernetes pod
type Pod struct {
	ResourceMetadata
	Ready    string
	Status   string
	Restarts int32
	Node     string
	IP       string
}

// Deployment represents a Kubernetes deployment
type Deployment struct {
	ResourceMetadata
	Ready     string
	UpToDate  int32
	Available int32
}

// Service represents a Kubernetes service
type Service struct {
	ResourceMetadata
	Type       string
	ClusterIP  string
	ExternalIP string
	Ports      string
}

// ConfigMap represents a Kubernetes configmap
type ConfigMap struct {
	ResourceMetadata
	Data int // Number of data items
}

// Secret represents a Kubernetes secret
type Secret struct {
	ResourceMetadata
	Type string
	Data int // Number of data items
}

// Namespace represents a Kubernetes namespace
type Namespace struct {
	ResourceMetadata
	Status string
}

// StatefulSet represents a Kubernetes statefulset
type StatefulSet struct {
	ResourceMetadata
	Ready string
}

// DaemonSet represents a Kubernetes daemonset
type DaemonSet struct {
	ResourceMetadata
	Desired   int32
	Current   int32
	Ready     int32
	UpToDate  int32
	Available int32
}

// Job represents a Kubernetes job
type Job struct {
	ResourceMetadata
	Completions string
	Duration    time.Duration
}

// CronJob represents a Kubernetes cronjob
type CronJob struct {
	ResourceMetadata
	Schedule     string
	Suspend      bool
	Active       int32
	LastSchedule time.Duration
}

// Node represents a Kubernetes node
type Node struct {
	ResourceMetadata
	Status       string
	Roles        string
	Version      string
	Hostname     string
	InstanceType string
	Zone         string
	NodePool     string
	OSImage      string
}

// ReplicaSet represents a Kubernetes replicaset
type ReplicaSet struct {
	ResourceMetadata
	Desired int32
	Current int32
	Ready   int32
}

// PersistentVolumeClaim represents a Kubernetes PVC
type PersistentVolumeClaim struct {
	ResourceMetadata
	Status       string // Bound, Pending, Lost
	Volume       string // PV name
	Capacity     string // "10Gi"
	AccessModes  string // "RWO", "RWX", "ROX"
	StorageClass string
}

// Ingress represents a Kubernetes ingress
type Ingress struct {
	ResourceMetadata
	Class   string // IngressClass name
	Hosts   string // Comma-separated
	Address string // LoadBalancer IP/hostname
	Ports   string // "80, 443"
}

// Endpoints represents a Kubernetes endpoints
type Endpoints struct {
	ResourceMetadata
	Endpoints string // "10.0.1.5:8080, 10.0.1.6:8080" (comma-separated)
}

// HorizontalPodAutoscaler represents a Kubernetes HPA
type HorizontalPodAutoscaler struct {
	ResourceMetadata
	Reference string // "Deployment/nginx"
	MinPods   int32
	MaxPods   int32
	Replicas  int32  // Current
	TargetCPU string // "80%" or "N/A"
}

// CRDColumn represents a column defined in CRD additionalPrinterColumns
type CRDColumn struct {
	Name        string // Column name (e.g., "Ready", "Status")
	Type        string // Column type (e.g., "string", "integer", "boolean", "date")
	Description string // Human-readable description
	JSONPath    string // JSONPath expression (e.g., ".status.conditions[?(@.type==\"Ready\")].status")
	Priority    int32  // Display priority (0=always show, >0=hide by default)
}

// CustomResourceDefinition represents a CRD in the cluster
type CustomResourceDefinition struct {
	ResourceMetadata
	Group   string      // e.g., "cert-manager.io"
	Version string      // Storage version, e.g., "v1"
	Kind    string      // e.g., "Certificate"
	Scope   string      // "Namespaced" or "Cluster"
	Plural  string      // e.g., "certificates"
	Columns []CRDColumn // additionalPrinterColumns from storage version
}

// GenericResource represents a CR instance with unknown schema
type GenericResource struct {
	ResourceMetadata
	Kind   string            // CRD Kind (e.g., "Certificate")
	Data   map[string]any    // Raw unstructured data for describe/yaml
	Fields map[string]string // Dynamic column values extracted via JSONPath
}

// GenericResource represents a CR instance with unknown schema
type GenericResource struct {
	ResourceMetadata
	Kind   string            // CRD Kind (e.g., "Certificate")
	Data   map[string]any    // Raw unstructured data for describe/yaml
	Fields map[string]string // Dynamic column values extracted via JSONPath
}
