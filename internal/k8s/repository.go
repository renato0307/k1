package k8s

import (
	"time"
)

// Repository provides access to Kubernetes resources
type Repository interface {
	GetPods() ([]Pod, error)
	GetDeployments() ([]Deployment, error)
	GetServices() ([]Service, error)
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
}

// Service represents a Kubernetes service
type Service struct {
	Namespace   string
	Name        string
	Type        string
	ClusterIP   string
	ExternalIP  string
	Ports       string
	Age         time.Duration
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

func (r *DummyRepository) Close() {
	// No-op for dummy repository
}
