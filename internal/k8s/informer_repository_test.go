package k8s

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
)

// createTestNamespace creates a unique namespace for test isolation
func createTestNamespace(t *testing.T) string {
	t.Helper()

	// Create unique namespace name based on test name and timestamp
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "test-",
		},
	}

	created, err := testClient.CoreV1().Namespaces().Create(
		context.Background(), ns, metav1.CreateOptions{})
	require.NoError(t, err, "Failed to create test namespace")

	// Cleanup namespace when test completes
	t.Cleanup(func() {
		_ = testClient.CoreV1().Namespaces().Delete(
			context.Background(),
			created.Name,
			metav1.DeleteOptions{},
		)
	})

	return created.Name
}

func TestInformerRepository_Init(t *testing.T) {
	// Verify shared test environment is initialized
	assert.NotNil(t, testCfg, "testCfg should be initialized by TestMain")
	assert.NotNil(t, testClient, "testClient should be initialized by TestMain")

	t.Log("Shared envtest environment initialized successfully")
}

func TestInformerRepository_GetPods_EmptyCluster(t *testing.T) {
	// Create unique namespace for test isolation
	ns := createTestNamespace(t)

	// Create a repository using the shared test config
	repo := createTestRepository(t, ns)
	defer repo.Close()

	// Query pods - should be empty (no pods in this namespace)
	pods, err := repo.GetPods()
	require.NoError(t, err, "GetPods failed")
	assert.Empty(t, pods, "Expected no pods in empty namespace")
}

func TestInformerRepository_GetPods_PodStates(t *testing.T) {
	tests := []struct {
		name            string
		podName         string
		containerName   string
		phase           corev1.PodPhase
		containerReady  bool
		restartCount    int32
		nodeIP          string
		expectedStatus  string
		expectedReady   string
		expectedRestarts int32
		expectedNode    string
		expectedIP      string
	}{
		{
			name:            "running pod with ready container",
			podName:         "test-pod",
			containerName:   "nginx",
			phase:           corev1.PodRunning,
			containerReady:  true,
			restartCount:    0,
			nodeIP:          "10.0.0.1",
			expectedStatus:  "Running",
			expectedReady:   "1/1",
			expectedRestarts: 0,
			expectedNode:    "test-node",
			expectedIP:      "10.0.0.1",
		},
		{
			name:            "crash loop pod with high restarts",
			podName:         "crash-pod",
			containerName:   "failing-app",
			phase:           corev1.PodRunning,
			containerReady:  false,
			restartCount:    15,
			nodeIP:          "10.0.0.2",
			expectedStatus:  "Running",
			expectedReady:   "0/1",
			expectedRestarts: 15,
			expectedNode:    "test-node",
			expectedIP:      "10.0.0.2",
		},
		{
			name:            "pending pod",
			podName:         "pending-pod",
			containerName:   "app",
			phase:           corev1.PodPending,
			containerReady:  false,
			restartCount:    0,
			nodeIP:          "",
			expectedStatus:  "Pending",
			expectedReady:   "0/1",
			expectedRestarts: 0,
			expectedNode:    "test-node",
			expectedIP:      "",
		},
		{
			name:            "pod with multiple restarts",
			podName:         "restart-pod",
			containerName:   "app",
			phase:           corev1.PodRunning,
			containerReady:  true,
			restartCount:    3,
			nodeIP:          "10.0.0.3",
			expectedStatus:  "Running",
			expectedReady:   "1/1",
			expectedRestarts: 3,
			expectedNode:    "test-node",
			expectedIP:      "10.0.0.3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create unique namespace for test isolation
			ns := createTestNamespace(t)

			// Create pod spec
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      tt.podName,
					Namespace: ns,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  tt.containerName,
							Image: "test:latest",
						},
					},
					NodeName: "test-node",
				},
			}

			createdPod, err := testClient.CoreV1().Pods(ns).Create(context.Background(), pod, metav1.CreateOptions{})
			require.NoError(t, err, "Failed to create pod")

			// Update pod status (envtest doesn't have controllers)
			createdPod.Status = corev1.PodStatus{
				Phase: tt.phase,
				PodIP: tt.nodeIP,
				ContainerStatuses: []corev1.ContainerStatus{
					{
						Name:         tt.containerName,
						Ready:        tt.containerReady,
						RestartCount: tt.restartCount,
					},
				},
			}

			_, err = testClient.CoreV1().Pods(ns).UpdateStatus(context.Background(), createdPod, metav1.UpdateOptions{})
			require.NoError(t, err, "Failed to update pod status")

			// Create repository and wait for sync
			repo := createTestRepository(t, ns)
			defer repo.Close()

			time.Sleep(100 * time.Millisecond)

			// Query pods
			pods, err := repo.GetPods()
			require.NoError(t, err, "GetPods failed")
			require.Len(t, pods, 1, "Expected 1 pod")

			// Verify pod fields
			gotPod := pods[0]
			assert.Equal(t, tt.podName, gotPod.Name, "pod name")
			assert.Equal(t, ns, gotPod.Namespace, "namespace")
			assert.Equal(t, tt.expectedStatus, gotPod.Status, "status")
			assert.Equal(t, tt.expectedReady, gotPod.Ready, "ready")
			assert.Equal(t, tt.expectedRestarts, gotPod.Restarts, "restarts")
			assert.Equal(t, tt.expectedNode, gotPod.Node, "node")
			assert.Equal(t, tt.expectedIP, gotPod.IP, "IP")
		})
	}
}

func TestInformerRepository_GetPods_SortByAge(t *testing.T) {
	// Create unique namespace for test isolation
	ns := createTestNamespace(t)

	now := time.Now()

	// Create pods with different ages
	pods := []*corev1.Pod{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "old-pod",
				Namespace:         ns,
				CreationTimestamp: metav1.NewTime(now.Add(-1 * time.Hour)),
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{{Name: "c1", Image: "img"}},
			},
			Status: corev1.PodStatus{Phase: corev1.PodRunning},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "new-pod",
				Namespace:         ns,
				CreationTimestamp: metav1.NewTime(now.Add(-10 * time.Minute)),
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{{Name: "c1", Image: "img"}},
			},
			Status: corev1.PodStatus{Phase: corev1.PodRunning},
		},
	}

	for _, pod := range pods {
		_, err := testClient.CoreV1().Pods(ns).Create(context.Background(), pod, metav1.CreateOptions{})
		require.NoError(t, err, "Failed to create pod")
	}

	repo := createTestRepository(t, ns)
	defer repo.Close()

	time.Sleep(100 * time.Millisecond)

	retrievedPods, err := repo.GetPods()
	require.NoError(t, err, "GetPods failed")
	require.Len(t, retrievedPods, 2, "Expected 2 pods")

	// Should be sorted by age (newest first)
	assert.Equal(t, "new-pod", retrievedPods[0].Name, "First pod should be newest")
	assert.Equal(t, "old-pod", retrievedPods[1].Name, "Second pod should be oldest")
}

func TestInformerRepository_GetDeployments_EmptyCluster(t *testing.T) {
	ns := createTestNamespace(t)
	repo := createTestRepository(t, ns)
	defer repo.Close()

	deployments, err := repo.GetDeployments()
	require.NoError(t, err)
	assert.Empty(t, deployments)
}

func TestInformerRepository_GetDeployments_WithData(t *testing.T) {
	ns := createTestNamespace(t)
	now := time.Now()
	replicas := int32(3)

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "test-deploy",
			Namespace:         ns,
			CreationTimestamp: metav1.NewTime(now.Add(-1 * time.Hour)),
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "test"}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "test"}},
				Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "c", Image: "img"}}},
			},
		},
	}

	created, err := testClient.AppsV1().Deployments(ns).Create(context.Background(), deployment, metav1.CreateOptions{})
	require.NoError(t, err)

	created.Status = appsv1.DeploymentStatus{
		Replicas:          3,
		ReadyReplicas:     2,
		UpdatedReplicas:   3,
		AvailableReplicas: 2,
	}
	_, err = testClient.AppsV1().Deployments(ns).UpdateStatus(context.Background(), created, metav1.UpdateOptions{})
	require.NoError(t, err)

	repo := createTestRepository(t, ns)
	defer repo.Close()
	time.Sleep(100 * time.Millisecond)

	deployments, err := repo.GetDeployments()
	require.NoError(t, err)
	require.Len(t, deployments, 1)

	assert.Equal(t, "test-deploy", deployments[0].Name)
	assert.Equal(t, "2/3", deployments[0].Ready)
	assert.Equal(t, int32(3), deployments[0].UpToDate)
	assert.Equal(t, int32(2), deployments[0].Available)
}

func TestInformerRepository_GetServices_EmptyCluster(t *testing.T) {
	ns := createTestNamespace(t)
	repo := createTestRepository(t, ns)
	defer repo.Close()

	services, err := repo.GetServices()
	require.NoError(t, err)
	assert.Empty(t, services)
}

func TestInformerRepository_GetServices_ServiceTypes(t *testing.T) {
	tests := []struct {
		name              string
		serviceType       corev1.ServiceType
		ports             []corev1.ServicePort
		externalIPs       []string
		loadBalancerIP    string
		loadBalancerHost  string
		expectedType      string
		expectedExtIP     string
		checkPorts        func(t *testing.T, ports string)
	}{
		{
			name:        "ClusterIP service with regular ports",
			serviceType: corev1.ServiceTypeClusterIP,
			ports: []corev1.ServicePort{
				{Name: "http", Port: 80, Protocol: corev1.ProtocolTCP},
			},
			expectedType:  "ClusterIP",
			expectedExtIP: "<none>",
			checkPorts: func(t *testing.T, ports string) {
				assert.Equal(t, "80/TCP", ports)
			},
		},
		{
			name:        "NodePort service with NodePort mapping",
			serviceType: corev1.ServiceTypeNodePort,
			ports: []corev1.ServicePort{
				{Name: "http", Port: 80, NodePort: 30080, Protocol: corev1.ProtocolTCP},
				{Name: "https", Port: 443, NodePort: 30443, Protocol: corev1.ProtocolTCP},
			},
			expectedType:  "NodePort",
			expectedExtIP: "<none>",
			checkPorts: func(t *testing.T, ports string) {
				assert.Contains(t, ports, "80:30080/TCP")
				assert.Contains(t, ports, "443:30443/TCP")
			},
		},
		{
			name:           "Service with external IPs from spec",
			serviceType:    corev1.ServiceTypeClusterIP,
			ports:          []corev1.ServicePort{{Port: 443, Protocol: corev1.ProtocolTCP}},
			externalIPs:    []string{"203.0.113.1", "203.0.113.2"},
			expectedType:   "ClusterIP",
			expectedExtIP:  "203.0.113.1,203.0.113.2",
			checkPorts:     func(t *testing.T, ports string) { assert.Equal(t, "443/TCP", ports) },
		},
		{
			name:           "LoadBalancer with IP in status",
			serviceType:    corev1.ServiceTypeLoadBalancer,
			ports:          []corev1.ServicePort{{Port: 80, Protocol: corev1.ProtocolTCP}},
			loadBalancerIP: "198.51.100.1",
			expectedType:   "LoadBalancer",
			expectedExtIP:  "198.51.100.1",
			checkPorts:     func(t *testing.T, ports string) { assert.Contains(t, ports, "80:") },
		},
		{
			name:             "LoadBalancer with hostname in status",
			serviceType:      corev1.ServiceTypeLoadBalancer,
			ports:            []corev1.ServicePort{{Port: 80, Protocol: corev1.ProtocolTCP}},
			loadBalancerHost: "lb.example.com",
			expectedType:     "LoadBalancer",
			expectedExtIP:    "lb.example.com",
			checkPorts:       func(t *testing.T, ports string) { assert.Contains(t, ports, "80:") },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ns := createTestNamespace(t)

			service := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-svc",
					Namespace: ns,
				},
				Spec: corev1.ServiceSpec{
					Type:        tt.serviceType,
					Ports:       tt.ports,
					ExternalIPs: tt.externalIPs,
					Selector:    map[string]string{"app": "test"},
				},
			}

			created, err := testClient.CoreV1().Services(ns).Create(context.Background(), service, metav1.CreateOptions{})
			require.NoError(t, err)

			// Update status if LoadBalancer IP/Hostname specified
			if tt.loadBalancerIP != "" || tt.loadBalancerHost != "" {
				created.Status.LoadBalancer.Ingress = []corev1.LoadBalancerIngress{
					{IP: tt.loadBalancerIP, Hostname: tt.loadBalancerHost},
				}
				_, err = testClient.CoreV1().Services(ns).UpdateStatus(context.Background(), created, metav1.UpdateOptions{})
				require.NoError(t, err)
			}

			repo := createTestRepository(t, ns)
			defer repo.Close()
			time.Sleep(100 * time.Millisecond)

			services, err := repo.GetServices()
			require.NoError(t, err)
			require.Len(t, services, 1)

			assert.Equal(t, "test-svc", services[0].Name)
			assert.Equal(t, tt.expectedType, services[0].Type)
			assert.NotEmpty(t, services[0].ClusterIP)
			assert.Equal(t, tt.expectedExtIP, services[0].ExternalIP)
			tt.checkPorts(t, services[0].Ports)
		})
	}
}

func TestInformerRepository_GetServices_SortByAge(t *testing.T) {
	ns := createTestNamespace(t)
	now := time.Now()

	// Create services with different ages
	services := []*corev1.Service{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "old-svc",
				Namespace:         ns,
				CreationTimestamp: metav1.NewTime(now.Add(-3 * time.Hour)),
			},
			Spec: corev1.ServiceSpec{
				Type:  corev1.ServiceTypeClusterIP,
				Ports: []corev1.ServicePort{{Port: 80, Protocol: corev1.ProtocolTCP}},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "new-svc",
				Namespace:         ns,
				CreationTimestamp: metav1.NewTime(now.Add(-15 * time.Minute)),
			},
			Spec: corev1.ServiceSpec{
				Type:  corev1.ServiceTypeClusterIP,
				Ports: []corev1.ServicePort{{Port: 8080, Protocol: corev1.ProtocolTCP}},
			},
		},
	}

	for _, svc := range services {
		_, err := testClient.CoreV1().Services(ns).Create(context.Background(), svc, metav1.CreateOptions{})
		require.NoError(t, err)
	}

	repo := createTestRepository(t, ns)
	defer repo.Close()
	time.Sleep(100 * time.Millisecond)

	retrievedServices, err := repo.GetServices()
	require.NoError(t, err)
	require.Len(t, retrievedServices, 2)

	// Should be sorted by age (newest first)
	assert.Equal(t, "new-svc", retrievedServices[0].Name)
	assert.Equal(t, "old-svc", retrievedServices[1].Name)
}

func TestInformerRepository_Close(t *testing.T) {
	// Create unique namespace for test isolation
	ns := createTestNamespace(t)

	repo := createTestRepository(t, ns)

	// Close should not panic
	assert.NotPanics(t, func() {
		repo.Close()
	}, "First Close should not panic")

	// Calling Close again should be safe
	assert.NotPanics(t, func() {
		repo.Close()
	}, "Second Close should not panic")
}

func TestInformerRepository_GetResources_Pods(t *testing.T) {
	ns := createTestNamespace(t)

	// Create a test pod
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: ns,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "nginx", Image: "nginx:latest"}},
			NodeName:   "test-node",
		},
	}

	createdPod, err := testClient.CoreV1().Pods(ns).Create(context.Background(), pod, metav1.CreateOptions{})
	require.NoError(t, err)

	createdPod.Status = corev1.PodStatus{
		Phase: corev1.PodRunning,
		PodIP: "10.0.0.1",
		ContainerStatuses: []corev1.ContainerStatus{
			{Name: "nginx", Ready: true, RestartCount: 0},
		},
	}
	_, err = testClient.CoreV1().Pods(ns).UpdateStatus(context.Background(), createdPod, metav1.UpdateOptions{})
	require.NoError(t, err)

	repo := createTestRepository(t, ns)
	defer repo.Close()
	time.Sleep(100 * time.Millisecond)

	// Call GetResources with ResourceTypePod
	resources, err := repo.GetResources(ResourceTypePod)
	require.NoError(t, err)
	require.Len(t, resources, 1)

	// Type assert to Pod
	pod1, ok := resources[0].(Pod)
	require.True(t, ok, "Resource should be of type Pod")

	assert.Equal(t, "test-pod", pod1.Name)
	assert.Equal(t, ns, pod1.Namespace)
	assert.Equal(t, "Running", pod1.Status)
	assert.Equal(t, "1/1", pod1.Ready)
	assert.Equal(t, int32(0), pod1.Restarts)
}

func TestInformerRepository_GetResources_Deployments(t *testing.T) {
	ns := createTestNamespace(t)
	replicas := int32(2)

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-deploy",
			Namespace: ns,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "test"}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "test"}},
				Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "c", Image: "img"}}},
			},
		},
	}

	created, err := testClient.AppsV1().Deployments(ns).Create(context.Background(), deployment, metav1.CreateOptions{})
	require.NoError(t, err)

	created.Status = appsv1.DeploymentStatus{
		Replicas:          2,
		ReadyReplicas:     1,
		UpdatedReplicas:   2,
		AvailableReplicas: 1,
	}
	_, err = testClient.AppsV1().Deployments(ns).UpdateStatus(context.Background(), created, metav1.UpdateOptions{})
	require.NoError(t, err)

	repo := createTestRepository(t, ns)
	defer repo.Close()
	time.Sleep(100 * time.Millisecond)

	resources, err := repo.GetResources(ResourceTypeDeployment)
	require.NoError(t, err)
	require.Len(t, resources, 1)

	deploy, ok := resources[0].(Deployment)
	require.True(t, ok, "Resource should be of type Deployment")

	assert.Equal(t, "test-deploy", deploy.Name)
	assert.Equal(t, "1/2", deploy.Ready)
	assert.Equal(t, int32(2), deploy.UpToDate)
	assert.Equal(t, int32(1), deploy.Available)
}

func TestInformerRepository_GetResources_Services(t *testing.T) {
	ns := createTestNamespace(t)

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-svc",
			Namespace: ns,
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeClusterIP,
			Ports: []corev1.ServicePort{
				{Port: 80, Protocol: corev1.ProtocolTCP},
			},
			Selector: map[string]string{"app": "test"},
		},
	}

	_, err := testClient.CoreV1().Services(ns).Create(context.Background(), service, metav1.CreateOptions{})
	require.NoError(t, err)

	repo := createTestRepository(t, ns)
	defer repo.Close()
	time.Sleep(100 * time.Millisecond)

	resources, err := repo.GetResources(ResourceTypeService)
	require.NoError(t, err)
	require.Len(t, resources, 1)

	svc, ok := resources[0].(Service)
	require.True(t, ok, "Resource should be of type Service")

	assert.Equal(t, "test-svc", svc.Name)
	assert.Equal(t, "ClusterIP", svc.Type)
	assert.NotEmpty(t, svc.ClusterIP)
	assert.Contains(t, svc.Ports, "80/TCP")
}

func TestInformerRepository_GetResources_UnknownType(t *testing.T) {
	ns := createTestNamespace(t)
	repo := createTestRepository(t, ns)
	defer repo.Close()

	_, err := repo.GetResources("unknown-type")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown resource type")
}

// TestResourceInterface verifies that all resource types implement Resource interface
func TestResourceInterface(t *testing.T) {
	now := time.Now()

	// Verify all types implement Resource interface
	var _ Resource = Pod{}
	var _ Resource = Deployment{}
	var _ Resource = Service{}
	var _ Resource = ConfigMap{}
	var _ Resource = Secret{}
	var _ Resource = Namespace{}
	var _ Resource = StatefulSet{}
	var _ Resource = DaemonSet{}
	var _ Resource = Job{}
	var _ Resource = CronJob{}
	var _ Resource = Node{}
	var _ Resource = ReplicaSet{}
	var _ Resource = PersistentVolumeClaim{}
	var _ Resource = Ingress{}
	var _ Resource = Endpoints{}
	var _ Resource = HorizontalPodAutoscaler{}

	// Test that interface methods work correctly
	pod := Pod{
		ResourceMetadata: ResourceMetadata{
			Namespace: "default",
			Name:      "test-pod",
			Age:       5 * time.Minute,
			CreatedAt: now,
		},
	}

	assert.Equal(t, "default", pod.GetNamespace())
	assert.Equal(t, "test-pod", pod.GetName())
	assert.Equal(t, 5*time.Minute, pod.GetAge())
	assert.Equal(t, now, pod.GetCreatedAt())

	// Test cluster-scoped resources return empty namespace
	node := Node{
		ResourceMetadata: ResourceMetadata{
			Name:      "test-node",
			Age:       1 * time.Hour,
			CreatedAt: now,
		},
	}

	assert.Equal(t, "", node.GetNamespace())
	assert.Equal(t, "test-node", node.GetName())
}

func TestSortByAge_CreatedAtOrder(t *testing.T) {
	now := time.Now()
	items := []Resource{
		Pod{ResourceMetadata: ResourceMetadata{Name: "old-pod", CreatedAt: now.Add(-10 * time.Hour)}},
		Pod{ResourceMetadata: ResourceMetadata{Name: "new-pod", CreatedAt: now.Add(-1 * time.Hour)}},
		Pod{ResourceMetadata: ResourceMetadata{Name: "medium-pod", CreatedAt: now.Add(-5 * time.Hour)}},
		Pod{ResourceMetadata: ResourceMetadata{Name: "ancient-pod", CreatedAt: now.Add(-24 * time.Hour)}},
	}

	sortByAge(items)

	// Should be sorted by age (newest first)
	assert.Equal(t, "new-pod", items[0].GetName())
	assert.Equal(t, "medium-pod", items[1].GetName())
	assert.Equal(t, "old-pod", items[2].GetName())
	assert.Equal(t, "ancient-pod", items[3].GetName())
}

func TestSortByAge_SameAge(t *testing.T) {
	now := time.Now()
	sameTime := now.Add(-5 * time.Hour)
	items := []Resource{
		Pod{ResourceMetadata: ResourceMetadata{Name: "pod-c", CreatedAt: sameTime}},
		Pod{ResourceMetadata: ResourceMetadata{Name: "pod-a", CreatedAt: sameTime}},
		Pod{ResourceMetadata: ResourceMetadata{Name: "pod-b", CreatedAt: sameTime}},
	}

	sortByAge(items)

	// With same age, should be sorted by name alphabetically
	assert.Equal(t, "pod-a", items[0].GetName())
	assert.Equal(t, "pod-b", items[1].GetName())
	assert.Equal(t, "pod-c", items[2].GetName())
}

func TestSortByAge_MixedTypes(t *testing.T) {
	now := time.Now()
	items := []Resource{
		Deployment{ResourceMetadata: ResourceMetadata{Name: "deploy-1", CreatedAt: now.Add(-10 * time.Hour)}},
		Service{ResourceMetadata: ResourceMetadata{Name: "svc-1", CreatedAt: now.Add(-2 * time.Hour)}},
		Pod{ResourceMetadata: ResourceMetadata{Name: "pod-1", CreatedAt: now.Add(-5 * time.Hour)}},
	}

	sortByAge(items)

	// Should be sorted by age (newest first) regardless of type
	assert.Equal(t, "svc-1", items[0].GetName())
	assert.Equal(t, "pod-1", items[1].GetName())
	assert.Equal(t, "deploy-1", items[2].GetName())
}

func TestSortByAge_UsesAgeField(t *testing.T) {
	now := time.Now()

	// Create resources where Age ≠ CreatedAt
	// This can happen when resources are restarted or rescheduled
	items := []Resource{
		// Old CreatedAt but recent Age (e.g., restarted pod)
		Pod{
			ResourceMetadata: ResourceMetadata{
				Name:      "restarted-pod",
				CreatedAt: now.Add(-10 * time.Hour), // Created 10h ago
				Age:       5 * time.Minute,           // But Age shows 5m (restarted)
			},
		},
		// Recent CreatedAt and Age match
		Pod{
			ResourceMetadata: ResourceMetadata{
				Name:      "normal-pod",
				CreatedAt: now.Add(-1 * time.Hour),
				Age:       1 * time.Hour,
			},
		},
		// Very old both CreatedAt and Age
		Pod{
			ResourceMetadata: ResourceMetadata{
				Name:      "ancient-pod",
				CreatedAt: now.Add(-24 * time.Hour),
				Age:       24 * time.Hour,
			},
		},
	}

	sortByAge(items)

	// Should sort by CreatedAt (newest first), not Age field
	// Because sortByAge uses GetCreatedAt() for sorting
	assert.Equal(t, "normal-pod", items[0].GetName(), "Should sort by CreatedAt (newest first)")
	assert.Equal(t, "restarted-pod", items[1].GetName(), "CreatedAt determines order, not Age")
	assert.Equal(t, "ancient-pod", items[2].GetName(), "Oldest CreatedAt should be last")
}

// createTestRepository creates an InformerRepository using the shared test config
// and scoped to a specific namespace for test isolation
func createTestRepository(t *testing.T, namespace string) *InformerRepository {
	t.Helper()

	// Create dynamic client
	dynamicClient, err := dynamic.NewForConfig(testCfg)
	require.NoError(t, err, "Failed to create dynamic client")

	// Create namespace-scoped informer factory for test isolation
	factory := informers.NewSharedInformerFactoryWithOptions(
		testClient,
		30*time.Second,
		informers.WithNamespace(namespace), // Scope to test namespace
	)

	// Create namespace-scoped dynamic informer factory
	dynamicFactory := dynamicinformer.NewFilteredDynamicSharedInformerFactory(
		dynamicClient,
		30*time.Second,
		namespace,
		nil,
	)

	// Create pod informer
	podInformer := factory.Core().V1().Pods().Informer()
	podLister := factory.Core().V1().Pods().Lister()

	// Create deployment informer
	deploymentInformer := factory.Apps().V1().Deployments().Informer()
	deploymentLister := factory.Apps().V1().Deployments().Lister()

	// Create service informer
	serviceInformer := factory.Core().V1().Services().Informer()
	serviceLister := factory.Core().V1().Services().Lister()

	// Create statefulset informer
	statefulSetInformer := factory.Apps().V1().StatefulSets().Informer()
	statefulSetLister := factory.Apps().V1().StatefulSets().Lister()

	// Create daemonset informer
	daemonSetInformer := factory.Apps().V1().DaemonSets().Informer()
	daemonSetLister := factory.Apps().V1().DaemonSets().Lister()

	// Initialize resource registry
	resourceRegistry := getResourceRegistry()

	// Create dynamic informers for all registered resources
	// Skip cluster-scoped resources when using namespace-scoped factory
	dynamicListers := make(map[schema.GroupVersionResource]cache.GenericLister)
	dynamicInformers := []cache.SharedIndexInformer{}

	for _, resCfg := range resourceRegistry {
		// Skip cluster-scoped resources (nodes, namespaces) in namespace-scoped tests
		if !resCfg.Namespaced {
			continue
		}
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
		statefulSetInformer.HasSynced,
		daemonSetInformer.HasSynced,
	}
	for _, inf := range dynamicInformers {
		allInformers = append(allInformers, inf.HasSynced)
	}

	synced := cache.WaitForCacheSync(ctx.Done(), allInformers...)
	if !synced {
		cancel()
	}
	require.True(t, synced, "Failed to sync caches")

	// Create repository with initialized indexes
	repo := &InformerRepository{
		clientset:         testClient,
		factory:           factory,
		podLister:         podLister,
		deploymentLister:  deploymentLister,
		serviceLister:     serviceLister,
		statefulSetLister: statefulSetLister,
		daemonSetLister:   daemonSetLister,
		dynamicClient:     dynamicClient,
		dynamicFactory:    dynamicFactory,
		resources:         resourceRegistry,
		dynamicListers:    dynamicListers,
		podsByNode:        make(map[string][]*corev1.Pod),
		podsByNamespace:   make(map[string][]*corev1.Pod),
		podsByOwnerUID:    make(map[string][]*corev1.Pod),
		podsByConfigMap:   make(map[string]map[string][]*corev1.Pod),
		podsBySecret:      make(map[string]map[string][]*corev1.Pod),
		jobsByOwnerUID:    make(map[string][]string),
		jobsByNamespace:   make(map[string][]string),
		ctx:               ctx,
		cancel:            cancel,
	}

	// Setup pod indexes with event handlers
	repo.setupPodIndexes()
	repo.setupJobIndexes()

	return repo
}

// TestInformerRepository_GetResourceYAML tests YAML generation using kubectl YAMLPrinter
func TestInformerRepository_GetResourceYAML(t *testing.T) {
	tests := []struct {
		name      string
		setupFunc func(t *testing.T, ns string) *corev1.Pod
		gvr       schema.GroupVersionResource
		validate  func(t *testing.T, yaml string)
	}{
		{
			name: "pod yaml with kubectl printer",
			setupFunc: func(t *testing.T, ns string) *corev1.Pod {
				pod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-pod",
						Namespace: ns,
						Labels: map[string]string{
							"app": "test",
						},
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  "nginx",
								Image: "nginx:latest",
							},
						},
					},
				}
				created, err := testClient.CoreV1().Pods(ns).Create(
					context.Background(), pod, metav1.CreateOptions{})
				require.NoError(t, err)
				return created
			},
			gvr: schema.GroupVersionResource{
				Group:    "",
				Version:  "v1",
				Resource: "pods",
			},
			validate: func(t *testing.T, yaml string) {
				assert.Contains(t, yaml, "kind: Pod")
				assert.Contains(t, yaml, "name: test-pod")
				assert.Contains(t, yaml, "app: test")
				assert.Contains(t, yaml, "image: nginx:latest")
				assert.Contains(t, yaml, "apiVersion: v1")
			},
		},
		{
			name: "deployment yaml with kubectl printer",
			setupFunc: func(t *testing.T, ns string) *corev1.Pod {
				replicas := int32(3)
				deployment := &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-deployment",
						Namespace: ns,
					},
					Spec: appsv1.DeploymentSpec{
						Replicas: &replicas,
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"app": "test"},
						},
						Template: corev1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Labels: map[string]string{"app": "test"},
							},
							Spec: corev1.PodSpec{
								Containers: []corev1.Container{
									{Name: "nginx", Image: "nginx:latest"},
								},
							},
						},
					},
				}
				_, err := testClient.AppsV1().Deployments(ns).Create(
					context.Background(), deployment, metav1.CreateOptions{})
				require.NoError(t, err)
				return nil // Not returning pod for deployment test
			},
			gvr: schema.GroupVersionResource{
				Group:    "apps",
				Version:  "v1",
				Resource: "deployments",
			},
			validate: func(t *testing.T, yaml string) {
				assert.Contains(t, yaml, "kind: Deployment")
				assert.Contains(t, yaml, "name: test-deployment")
				assert.Contains(t, yaml, "replicas: 3")
				assert.Contains(t, yaml, "apiVersion: apps/v1")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create unique namespace for test isolation
			ns := createTestNamespace(t)

			// Setup test resource
			tt.setupFunc(t, ns)

			// Create test repository
			repo := createTestRepository(t, ns)
			defer repo.Close()

			// Wait for cache sync
			time.Sleep(100 * time.Millisecond)

			// Get resource name based on GVR
			resourceName := "test-pod"
			if tt.gvr.Resource == "deployments" {
				resourceName = "test-deployment"
			}

			// Call GetResourceYAML
			yaml, err := repo.GetResourceYAML(tt.gvr, ns, resourceName)

			// Validate
			require.NoError(t, err)
			assert.NotEmpty(t, yaml)
			tt.validate(t, yaml)
		})
	}
}

// TestInformerRepository_GetResourceYAML_NotFound tests error handling
func TestInformerRepository_GetResourceYAML_NotFound(t *testing.T) {
	ns := createTestNamespace(t)
	repo := createTestRepository(t, ns)
	defer repo.Close()

	gvr := schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "pods",
	}

	yaml, err := repo.GetResourceYAML(gvr, ns, "nonexistent-pod")

	assert.Error(t, err)
	assert.Empty(t, yaml)
	assert.Contains(t, err.Error(), "not found")
}

// TestInformerRepository_DescribeResource tests describe output with on-demand events
func TestInformerRepository_DescribeResource(t *testing.T) {
	ns := createTestNamespace(t)

	// Create test pod
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: ns,
			Labels: map[string]string{
				"app":  "test",
				"tier": "backend",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "nginx",
					Image: "nginx:latest",
				},
			},
		},
	}
	created, err := testClient.CoreV1().Pods(ns).Create(
		context.Background(), pod, metav1.CreateOptions{})
	require.NoError(t, err)

	// Create test event for this pod
	event := &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-event",
			Namespace: ns,
		},
		InvolvedObject: corev1.ObjectReference{
			Kind:      "Pod",
			Name:      created.Name,
			Namespace: ns,
			UID:       created.UID,
		},
		Reason:  "Created",
		Message: "Pod created successfully",
		Type:    "Normal",
		LastTimestamp: metav1.Time{
			Time: time.Now().Add(-5 * time.Minute),
		},
	}
	_, err = testClient.CoreV1().Events(ns).Create(
		context.Background(), event, metav1.CreateOptions{})
	require.NoError(t, err)

	// Create test repository
	repo := createTestRepository(t, ns)
	defer repo.Close()

	// Wait for cache sync
	time.Sleep(100 * time.Millisecond)

	// Get GVR for pods
	gvr := schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "pods",
	}

	// Call DescribeResource
	describe, err := repo.DescribeResource(gvr, ns, "test-pod")

	// Validate basic describe structure
	require.NoError(t, err)
	assert.NotEmpty(t, describe)

	// Validate required fields
	assert.Contains(t, describe, "Name:         test-pod")
	assert.Contains(t, describe, "Namespace:    "+ns)
	assert.Contains(t, describe, "Kind:         Pod")
	assert.Contains(t, describe, "API Version:  v1")

	// Validate labels section
	assert.Contains(t, describe, "Labels:")
	assert.Contains(t, describe, "app=test")
	assert.Contains(t, describe, "tier=backend")

	// Validate status section
	assert.Contains(t, describe, "Status:")

	// Validate events section
	assert.Contains(t, describe, "Events:")
	assert.Contains(t, describe, "Type")
	assert.Contains(t, describe, "Reason")
	assert.Contains(t, describe, "Created")
	assert.Contains(t, describe, "Pod created successfully")
}

// TestInformerRepository_DescribeResource_NoEvents tests describe when no events exist
func TestInformerRepository_DescribeResource_NoEvents(t *testing.T) {
	ns := createTestNamespace(t)

	// Create test pod without any events
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod-no-events",
			Namespace: ns,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "nginx", Image: "nginx:latest"},
			},
		},
	}
	_, err := testClient.CoreV1().Pods(ns).Create(
		context.Background(), pod, metav1.CreateOptions{})
	require.NoError(t, err)

	// Create test repository
	repo := createTestRepository(t, ns)
	defer repo.Close()

	// Wait for cache sync
	time.Sleep(100 * time.Millisecond)

	// Get GVR for pods
	gvr := schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "pods",
	}

	// Call DescribeResource
	describe, err := repo.DescribeResource(gvr, ns, "test-pod-no-events")

	// Validate
	require.NoError(t, err)
	assert.Contains(t, describe, "Events:")
	assert.Contains(t, describe, "<none>")
}

// TestInformerRepository_formatEventAge tests event age formatting
func TestInformerRepository_formatEventAge(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		expected string
	}{
		{
			name:     "seconds",
			duration: 30 * time.Second,
			expected: "30s",
		},
		{
			name:     "minutes",
			duration: 5 * time.Minute,
			expected: "5m",
		},
		{
			name:     "hours",
			duration: 3 * time.Hour,
			expected: "3h",
		},
		{
			name:     "days",
			duration: 2 * 24 * time.Hour,
			expected: "2d",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatEventAge(tt.duration)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestInformerRepository_GetPodsForDeployment(t *testing.T) {
	t.Skip("Skipping: requires full cluster with deployment controller - envtest doesn't run controllers")
	ns := createTestNamespace(t)

	// Create deployment
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-deployment",
			Namespace: ns,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: int32Ptr(2),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "test"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "test"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "nginx", Image: "nginx:latest"}},
				},
			},
		},
	}
	createdDeployment, err := testClient.AppsV1().Deployments(ns).Create(context.Background(), deployment, metav1.CreateOptions{})
	require.NoError(t, err)

	// Create ReplicaSet owned by deployment
	rs := &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-rs",
			Namespace: ns,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
					Name:       createdDeployment.Name,
					UID:        createdDeployment.UID,
				},
			},
		},
		Spec: appsv1.ReplicaSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "test"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "test"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "nginx", Image: "nginx:latest"}},
				},
			},
		},
	}
	createdRS, err := testClient.AppsV1().ReplicaSets(ns).Create(context.Background(), rs, metav1.CreateOptions{})
	require.NoError(t, err)

	// Create pods owned by ReplicaSet
	for i := 1; i <= 2; i++ {
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("test-pod-%d", i),
				Namespace: ns,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: "apps/v1",
						Kind:       "ReplicaSet",
						Name:       createdRS.Name,
						UID:        createdRS.UID,
					},
				},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{{Name: "nginx", Image: "nginx:latest"}},
				NodeName:   "test-node",
			},
		}
		createdPod, err := testClient.CoreV1().Pods(ns).Create(context.Background(), pod, metav1.CreateOptions{})
		require.NoError(t, err)

		createdPod.Status = corev1.PodStatus{
			Phase: corev1.PodRunning,
			ContainerStatuses: []corev1.ContainerStatus{
				{Name: "nginx", Ready: true, RestartCount: 0},
			},
		}
		_, err = testClient.CoreV1().Pods(ns).UpdateStatus(context.Background(), createdPod, metav1.UpdateOptions{})
		require.NoError(t, err)
	}

	// Create unrelated pod
	unrelatedPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "unrelated-pod",
			Namespace: ns,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "nginx", Image: "nginx:latest"}},
		},
	}
	_, err = testClient.CoreV1().Pods(ns).Create(context.Background(), unrelatedPod, metav1.CreateOptions{})
	require.NoError(t, err)

	repo := createTestRepository(t, ns)
	defer repo.Close()
	// Wait longer for deployment → replicaset → pods chain to sync
	time.Sleep(2 * time.Second)

	// Get pods for deployment
	pods, err := repo.GetPodsForDeployment(ns, "test-deployment")
	require.NoError(t, err)
	require.Len(t, pods, 2, "should return 2 pods owned by deployment")

	// Verify pod names
	podNames := []string{pods[0].Name, pods[1].Name}
	assert.Contains(t, podNames, "test-pod-1")
	assert.Contains(t, podNames, "test-pod-2")
	assert.NotContains(t, podNames, "unrelated-pod")
}

func TestInformerRepository_GetPodsOnNode(t *testing.T) {
	ns := createTestNamespace(t)

	// Create pods on different nodes
	node1Pods := []string{"pod-on-node1-a", "pod-on-node1-b"}
	node2Pods := []string{"pod-on-node2-a"}

	for _, podName := range node1Pods {
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      podName,
				Namespace: ns,
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{{Name: "nginx", Image: "nginx:latest"}},
				NodeName:   "node-1",
			},
		}
		createdPod, err := testClient.CoreV1().Pods(ns).Create(context.Background(), pod, metav1.CreateOptions{})
		require.NoError(t, err)

		createdPod.Status = corev1.PodStatus{
			Phase: corev1.PodRunning,
			ContainerStatuses: []corev1.ContainerStatus{
				{Name: "nginx", Ready: true, RestartCount: 0},
			},
		}
		_, err = testClient.CoreV1().Pods(ns).UpdateStatus(context.Background(), createdPod, metav1.UpdateOptions{})
		require.NoError(t, err)
	}

	for _, podName := range node2Pods {
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      podName,
				Namespace: ns,
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{{Name: "nginx", Image: "nginx:latest"}},
				NodeName:   "node-2",
			},
		}
		_, err := testClient.CoreV1().Pods(ns).Create(context.Background(), pod, metav1.CreateOptions{})
		require.NoError(t, err)
	}

	repo := createTestRepository(t, ns)
	defer repo.Close()
	time.Sleep(1 * time.Second)

	// Get pods on node-1
	pods, err := repo.GetPodsOnNode("node-1")
	require.NoError(t, err)
	assert.Len(t, pods, 2, "should return 2 pods on node-1")

	// Verify pod names
	podNames := []string{pods[0].Name, pods[1].Name}
	assert.Contains(t, podNames, "pod-on-node1-a")
	assert.Contains(t, podNames, "pod-on-node1-b")
	assert.NotContains(t, podNames, "pod-on-node2-a")

	// Get pods on node-2
	pods, err = repo.GetPodsOnNode("node-2")
	require.NoError(t, err)
	assert.Len(t, pods, 1, "should return 1 pod on node-2")
	assert.Equal(t, "pod-on-node2-a", pods[0].Name)

	// Get pods on non-existent node
	pods, err = repo.GetPodsOnNode("non-existent-node")
	require.NoError(t, err)
	assert.Empty(t, pods, "should return empty list for non-existent node")
}

func TestInformerRepository_GetPodsForService(t *testing.T) {
	ns := createTestNamespace(t)

	// Create service with selector
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-service",
			Namespace: ns,
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"app":  "test",
				"tier": "frontend",
			},
			Ports: []corev1.ServicePort{
				{Port: 80, Protocol: corev1.ProtocolTCP},
			},
		},
	}
	_, err := testClient.CoreV1().Services(ns).Create(context.Background(), service, metav1.CreateOptions{})
	require.NoError(t, err)

	// Create pods with matching labels
	matchingPods := []string{"matching-pod-1", "matching-pod-2"}
	for _, podName := range matchingPods {
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      podName,
				Namespace: ns,
				Labels: map[string]string{
					"app":  "test",
					"tier": "frontend",
				},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{{Name: "nginx", Image: "nginx:latest"}},
			},
		}
		createdPod, err := testClient.CoreV1().Pods(ns).Create(context.Background(), pod, metav1.CreateOptions{})
		require.NoError(t, err)

		createdPod.Status = corev1.PodStatus{
			Phase: corev1.PodRunning,
			ContainerStatuses: []corev1.ContainerStatus{
				{Name: "nginx", Ready: true, RestartCount: 0},
			},
		}
		_, err = testClient.CoreV1().Pods(ns).UpdateStatus(context.Background(), createdPod, metav1.UpdateOptions{})
		require.NoError(t, err)
	}

	// Create pod with non-matching labels
	nonMatchingPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "non-matching-pod",
			Namespace: ns,
			Labels: map[string]string{
				"app": "other",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "nginx", Image: "nginx:latest"}},
		},
	}
	_, err = testClient.CoreV1().Pods(ns).Create(context.Background(), nonMatchingPod, metav1.CreateOptions{})
	require.NoError(t, err)

	repo := createTestRepository(t, ns)
	defer repo.Close()
	time.Sleep(1 * time.Second)

	// Get pods for service
	pods, err := repo.GetPodsForService(ns, "test-service")
	require.NoError(t, err)
	assert.Len(t, pods, 2, "should return 2 pods matching service selector")

	// Verify pod names
	podNames := []string{pods[0].Name, pods[1].Name}
	assert.Contains(t, podNames, "matching-pod-1")
	assert.Contains(t, podNames, "matching-pod-2")
	assert.NotContains(t, podNames, "non-matching-pod")
}

// Helper function for int32 pointer
func int32Ptr(i int32) *int32 {
	return &i
}

// TestInformerRepository_IndexMaintenance_Add tests that indexes are updated on pod add
func TestInformerRepository_IndexMaintenance_Add(t *testing.T) {
	ns := createTestNamespace(t)

	// Create repository first so event handlers are registered
	repo := createTestRepository(t, ns)
	defer repo.Close()

	// Create pod AFTER repository so ADD event fires with handler registered
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: ns,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "nginx", Image: "nginx:latest"}},
			NodeName:   "test-node",
		},
	}
	createdPod, err := testClient.CoreV1().Pods(ns).Create(context.Background(), pod, metav1.CreateOptions{})
	require.NoError(t, err)
	time.Sleep(200 * time.Millisecond)

	// Verify pod is in node index
	repo.mu.RLock()
	podsOnNode := repo.podsByNode["test-node"]
	repo.mu.RUnlock()
	assert.Len(t, podsOnNode, 1, "pod should be in node index")
	assert.Equal(t, createdPod.UID, podsOnNode[0].UID)

	// Verify pod is in namespace index
	repo.mu.RLock()
	podsInNs := repo.podsByNamespace[ns]
	repo.mu.RUnlock()
	assert.Len(t, podsInNs, 1, "pod should be in namespace index")
	assert.Equal(t, createdPod.UID, podsInNs[0].UID)
}

// TestInformerRepository_IndexMaintenance_Delete tests that indexes are cleaned up on pod delete
func TestInformerRepository_IndexMaintenance_Delete(t *testing.T) {
	t.Skip("Skipping: informer delete events have inconsistent timing in envtest")
	ns := createTestNamespace(t)

	// Create repository first so event handlers are registered
	repo := createTestRepository(t, ns)
	defer repo.Close()

	// Create pod AFTER repository so ADD event fires with handler registered
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: ns,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "nginx", Image: "nginx:latest"}},
			NodeName:   "test-node",
		},
	}
	createdPod, err := testClient.CoreV1().Pods(ns).Create(context.Background(), pod, metav1.CreateOptions{})
	require.NoError(t, err)
	time.Sleep(200 * time.Millisecond)

	// Verify pod is in indexes
	repo.mu.RLock()
	podsOnNode := repo.podsByNode["test-node"]
	podsInNs := repo.podsByNamespace[ns]
	repo.mu.RUnlock()
	assert.Len(t, podsOnNode, 1)
	assert.Len(t, podsInNs, 1)

	// Delete pod
	err = testClient.CoreV1().Pods(ns).Delete(context.Background(), createdPod.Name, metav1.DeleteOptions{})
	require.NoError(t, err)
	time.Sleep(500 * time.Millisecond)

	// Verify pod is removed from indexes
	repo.mu.RLock()
	podsOnNodeAfter := repo.podsByNode["test-node"]
	podsInNsAfter := repo.podsByNamespace[ns]
	repo.mu.RUnlock()
	assert.Empty(t, podsOnNodeAfter, "pod should be removed from node index")
	assert.Empty(t, podsInNsAfter, "pod should be removed from namespace index")
}

// TestInformerRepository_IndexMaintenance_OwnerReferences tests owner UID index
func TestInformerRepository_IndexMaintenance_OwnerReferences(t *testing.T) {
	ns := createTestNamespace(t)

	// Create ReplicaSet
	rs := &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-rs",
			Namespace: ns,
		},
		Spec: appsv1.ReplicaSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "test"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "test"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "nginx", Image: "nginx:latest"}},
				},
			},
		},
	}
	createdRS, err := testClient.AppsV1().ReplicaSets(ns).Create(context.Background(), rs, metav1.CreateOptions{})
	require.NoError(t, err)

	// Create pod owned by ReplicaSet
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: ns,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "apps/v1",
					Kind:       "ReplicaSet",
					Name:       createdRS.Name,
					UID:        createdRS.UID,
				},
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "nginx", Image: "nginx:latest"}},
			NodeName:   "test-node",
		},
	}
	createdPod, err := testClient.CoreV1().Pods(ns).Create(context.Background(), pod, metav1.CreateOptions{})
	require.NoError(t, err)

	repo := createTestRepository(t, ns)
	defer repo.Close()
	time.Sleep(100 * time.Millisecond)

	// Verify pod is in owner index
	repo.mu.RLock()
	podsForOwner := repo.podsByOwnerUID[string(createdRS.UID)]
	repo.mu.RUnlock()
	assert.Len(t, podsForOwner, 1, "pod should be in owner index")
	assert.Equal(t, createdPod.UID, podsForOwner[0].UID)
}

// TestInformerRepository_IndexMaintenance_MultiplePods tests multiple pods in same index
func TestInformerRepository_IndexMaintenance_MultiplePods(t *testing.T) {
	ns := createTestNamespace(t)

	// Create multiple pods on same node
	for i := 1; i <= 3; i++ {
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("test-pod-%d", i),
				Namespace: ns,
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{{Name: "nginx", Image: "nginx:latest"}},
				NodeName:   "shared-node",
			},
		}
		_, err := testClient.CoreV1().Pods(ns).Create(context.Background(), pod, metav1.CreateOptions{})
		require.NoError(t, err)
	}

	repo := createTestRepository(t, ns)
	defer repo.Close()
	time.Sleep(100 * time.Millisecond)

	// Verify all pods are in node index
	repo.mu.RLock()
	podsOnNode := repo.podsByNode["shared-node"]
	repo.mu.RUnlock()
	assert.Len(t, podsOnNode, 3, "all 3 pods should be in node index")
}

// TestInformerRepository_IndexedQuery_Performance tests that indexed queries are faster
func TestInformerRepository_IndexedQuery_Performance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	ns := createTestNamespace(t)

	// Create 100 pods across 10 nodes
	nodesCount := 10
	podsPerNode := 10
	for i := 0; i < nodesCount; i++ {
		nodeName := fmt.Sprintf("node-%d", i)
		for j := 0; j < podsPerNode; j++ {
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("pod-%d-%d", i, j),
					Namespace: ns,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "nginx", Image: "nginx:latest"}},
					NodeName:   nodeName,
				},
			}
			_, err := testClient.CoreV1().Pods(ns).Create(context.Background(), pod, metav1.CreateOptions{})
			require.NoError(t, err)
		}
	}

	repo := createTestRepository(t, ns)
	defer repo.Close()
	time.Sleep(500 * time.Millisecond)

	// Time indexed query
	start := time.Now()
	pods, err := repo.GetPodsOnNode("node-5")
	indexedDuration := time.Since(start)

	require.NoError(t, err)
	assert.Len(t, pods, podsPerNode, "should return correct number of pods")

	t.Logf("Indexed query took: %v", indexedDuration)

	// Indexed query should be fast (< 10ms)
	assert.Less(t, indexedDuration, 10*time.Millisecond, "indexed query should be fast")
}

// Phase 4 tests: New navigation relationships

func TestInformerRepository_GetPodsForStatefulSet(t *testing.T) {
	ns := createTestNamespace(t)

	// Create statefulset
	statefulSet := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-statefulset",
			Namespace: ns,
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: int32Ptr(2),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "stateful"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "stateful"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "nginx", Image: "nginx:latest"}},
				},
			},
		},
	}
	createdSS, err := testClient.AppsV1().StatefulSets(ns).Create(context.Background(), statefulSet, metav1.CreateOptions{})
	require.NoError(t, err)

	// Create pods owned by StatefulSet
	for i := 0; i < 2; i++ {
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("test-statefulset-%d", i),
				Namespace: ns,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: "apps/v1",
						Kind:       "StatefulSet",
						Name:       createdSS.Name,
						UID:        createdSS.UID,
					},
				},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{{Name: "nginx", Image: "nginx:latest"}},
				NodeName:   "test-node",
			},
		}
		createdPod, err := testClient.CoreV1().Pods(ns).Create(context.Background(), pod, metav1.CreateOptions{})
		require.NoError(t, err)

		createdPod.Status = corev1.PodStatus{
			Phase: corev1.PodRunning,
			ContainerStatuses: []corev1.ContainerStatus{
				{Name: "nginx", Ready: true, RestartCount: 0},
			},
		}
		_, err = testClient.CoreV1().Pods(ns).UpdateStatus(context.Background(), createdPod, metav1.UpdateOptions{})
		require.NoError(t, err)
	}

	// Create unrelated pod
	unrelatedPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "unrelated-pod",
			Namespace: ns,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "nginx", Image: "nginx:latest"}},
		},
	}
	_, err = testClient.CoreV1().Pods(ns).Create(context.Background(), unrelatedPod, metav1.CreateOptions{})
	require.NoError(t, err)

	repo := createTestRepository(t, ns)
	defer repo.Close()
	time.Sleep(1 * time.Second)

	// Get pods for statefulset
	pods, err := repo.GetPodsForStatefulSet(ns, "test-statefulset")
	require.NoError(t, err)
	assert.Len(t, pods, 2, "should return 2 pods owned by statefulset")

	// Verify pod names
	podNames := []string{pods[0].Name, pods[1].Name}
	assert.Contains(t, podNames, "test-statefulset-0")
	assert.Contains(t, podNames, "test-statefulset-1")
	assert.NotContains(t, podNames, "unrelated-pod")
}

func TestInformerRepository_GetPodsForDaemonSet(t *testing.T) {
	ns := createTestNamespace(t)

	// Create daemonset
	daemonSet := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-daemonset",
			Namespace: ns,
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "daemon"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "daemon"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "nginx", Image: "nginx:latest"}},
				},
			},
		},
	}
	createdDS, err := testClient.AppsV1().DaemonSets(ns).Create(context.Background(), daemonSet, metav1.CreateOptions{})
	require.NoError(t, err)

	// Create pods owned by DaemonSet
	for i := 0; i < 2; i++ {
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("test-daemonset-pod-%d", i),
				Namespace: ns,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: "apps/v1",
						Kind:       "DaemonSet",
						Name:       createdDS.Name,
						UID:        createdDS.UID,
					},
				},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{{Name: "nginx", Image: "nginx:latest"}},
				NodeName:   "test-node",
			},
		}
		createdPod, err := testClient.CoreV1().Pods(ns).Create(context.Background(), pod, metav1.CreateOptions{})
		require.NoError(t, err)

		createdPod.Status = corev1.PodStatus{
			Phase: corev1.PodRunning,
			ContainerStatuses: []corev1.ContainerStatus{
				{Name: "nginx", Ready: true, RestartCount: 0},
			},
		}
		_, err = testClient.CoreV1().Pods(ns).UpdateStatus(context.Background(), createdPod, metav1.UpdateOptions{})
		require.NoError(t, err)
	}

	repo := createTestRepository(t, ns)
	defer repo.Close()
	time.Sleep(1 * time.Second)

	// Get pods for daemonset
	pods, err := repo.GetPodsForDaemonSet(ns, "test-daemonset")
	require.NoError(t, err)
	assert.Len(t, pods, 2, "should return 2 pods owned by daemonset")

	// Verify pods have correct owner
	for _, pod := range pods {
		assert.Contains(t, pod.Name, "test-daemonset")
	}
}

func TestInformerRepository_GetPodsForNamespace(t *testing.T) {
	ns1 := createTestNamespace(t)
	ns2 := createTestNamespace(t)

	// Create pods in namespace 1
	for i := 0; i < 3; i++ {
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("ns1-pod-%d", i),
				Namespace: ns1,
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{{Name: "nginx", Image: "nginx:latest"}},
				NodeName:   "test-node",
			},
		}
		createdPod, err := testClient.CoreV1().Pods(ns1).Create(context.Background(), pod, metav1.CreateOptions{})
		require.NoError(t, err)

		createdPod.Status = corev1.PodStatus{
			Phase: corev1.PodRunning,
		}
		_, err = testClient.CoreV1().Pods(ns1).UpdateStatus(context.Background(), createdPod, metav1.UpdateOptions{})
		require.NoError(t, err)
	}

	// Create pods in namespace 2
	for i := 0; i < 2; i++ {
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("ns2-pod-%d", i),
				Namespace: ns2,
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{{Name: "nginx", Image: "nginx:latest"}},
				NodeName:   "test-node",
			},
		}
		_, err := testClient.CoreV1().Pods(ns2).Create(context.Background(), pod, metav1.CreateOptions{})
		require.NoError(t, err)
	}

	repo := createTestRepository(t, ns1)
	defer repo.Close()
	time.Sleep(1 * time.Second)

	// Get pods for namespace 1
	pods, err := repo.GetPodsForNamespace(ns1)
	require.NoError(t, err)
	assert.Len(t, pods, 3, "should return 3 pods in namespace 1")

	// Verify all pods are from namespace 1
	for _, pod := range pods {
		assert.Equal(t, ns1, pod.Namespace)
		assert.Contains(t, pod.Name, "ns1-pod")
	}
}

func TestInformerRepository_GetPodsUsingConfigMap(t *testing.T) {
	ns := createTestNamespace(t)

	// Create configmap
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-config",
			Namespace: ns,
		},
		Data: map[string]string{
			"key": "value",
		},
	}
	_, err := testClient.CoreV1().ConfigMaps(ns).Create(context.Background(), cm, metav1.CreateOptions{})
	require.NoError(t, err)

	// Create pod using configmap as volume
	podWithCM := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod-with-configmap",
			Namespace: ns,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "nginx", Image: "nginx:latest"}},
			Volumes: []corev1.Volume{
				{
					Name: "config-volume",
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "test-config",
							},
						},
					},
				},
			},
			NodeName: "test-node",
		},
	}
	createdPod, err := testClient.CoreV1().Pods(ns).Create(context.Background(), podWithCM, metav1.CreateOptions{})
	require.NoError(t, err)

	createdPod.Status = corev1.PodStatus{Phase: corev1.PodRunning}
	_, err = testClient.CoreV1().Pods(ns).UpdateStatus(context.Background(), createdPod, metav1.UpdateOptions{})
	require.NoError(t, err)

	// Create pod without configmap
	podWithoutCM := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod-without-configmap",
			Namespace: ns,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "nginx", Image: "nginx:latest"}},
		},
	}
	_, err = testClient.CoreV1().Pods(ns).Create(context.Background(), podWithoutCM, metav1.CreateOptions{})
	require.NoError(t, err)

	repo := createTestRepository(t, ns)
	defer repo.Close()
	time.Sleep(1 * time.Second)

	// Get pods using configmap
	pods, err := repo.GetPodsUsingConfigMap(ns, "test-config")
	require.NoError(t, err)
	assert.Len(t, pods, 1, "should return 1 pod using configmap")
	assert.Equal(t, "pod-with-configmap", pods[0].Name)
}

func TestInformerRepository_GetPodsUsingSecret(t *testing.T) {
	ns := createTestNamespace(t)

	// Create secret
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-secret",
			Namespace: ns,
		},
		Data: map[string][]byte{
			"password": []byte("secret"),
		},
	}
	_, err := testClient.CoreV1().Secrets(ns).Create(context.Background(), secret, metav1.CreateOptions{})
	require.NoError(t, err)

	// Create pod using secret as volume
	podWithSecret := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod-with-secret",
			Namespace: ns,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "nginx", Image: "nginx:latest"}},
			Volumes: []corev1.Volume{
				{
					Name: "secret-volume",
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName: "test-secret",
						},
					},
				},
			},
			NodeName: "test-node",
		},
	}
	createdPod, err := testClient.CoreV1().Pods(ns).Create(context.Background(), podWithSecret, metav1.CreateOptions{})
	require.NoError(t, err)

	createdPod.Status = corev1.PodStatus{Phase: corev1.PodRunning}
	_, err = testClient.CoreV1().Pods(ns).UpdateStatus(context.Background(), createdPod, metav1.UpdateOptions{})
	require.NoError(t, err)

	// Create pod without secret
	podWithoutSecret := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod-without-secret",
			Namespace: ns,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "nginx", Image: "nginx:latest"}},
		},
	}
	_, err = testClient.CoreV1().Pods(ns).Create(context.Background(), podWithoutSecret, metav1.CreateOptions{})
	require.NoError(t, err)

	repo := createTestRepository(t, ns)
	defer repo.Close()
	time.Sleep(1 * time.Second)

	// Get pods using secret
	pods, err := repo.GetPodsUsingSecret(ns, "test-secret")
	require.NoError(t, err)
	assert.Len(t, pods, 1, "should return 1 pod using secret")
	assert.Equal(t, "pod-with-secret", pods[0].Name)
}

func TestInformerRepository_GetPodsForJob(t *testing.T) {
	ns := createTestNamespace(t)

	// Create job
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-job",
			Namespace: ns,
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers:    []corev1.Container{{Name: "busybox", Image: "busybox:latest", Command: []string{"echo", "hello"}}},
					RestartPolicy: corev1.RestartPolicyNever,
				},
			},
		},
	}
	createdJob, err := testClient.BatchV1().Jobs(ns).Create(context.Background(), job, metav1.CreateOptions{})
	require.NoError(t, err)

	// Create pods owned by Job
	for i := 0; i < 2; i++ {
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("test-job-pod-%d", i),
				Namespace: ns,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: "batch/v1",
						Kind:       "Job",
						Name:       createdJob.Name,
						UID:        createdJob.UID,
					},
				},
			},
			Spec: corev1.PodSpec{
				Containers:    []corev1.Container{{Name: "busybox", Image: "busybox:latest"}},
				RestartPolicy: corev1.RestartPolicyNever,
				NodeName:      "test-node",
			},
		}
		createdPod, err := testClient.CoreV1().Pods(ns).Create(context.Background(), pod, metav1.CreateOptions{})
		require.NoError(t, err)

		createdPod.Status = corev1.PodStatus{
			Phase: corev1.PodSucceeded,
		}
		_, err = testClient.CoreV1().Pods(ns).UpdateStatus(context.Background(), createdPod, metav1.UpdateOptions{})
		require.NoError(t, err)
	}

	repo := createTestRepository(t, ns)
	defer repo.Close()
	time.Sleep(2 * time.Second) // Jobs need more time to sync via dynamic client

	// Get pods for job
	pods, err := repo.GetPodsForJob(ns, "test-job")
	require.NoError(t, err)
	assert.Len(t, pods, 2, "should return 2 pods owned by job")

	// Verify pods have correct owner
	for _, pod := range pods {
		assert.Contains(t, pod.Name, "test-job")
	}
}

func TestInformerRepository_GetJobsForCronJob(t *testing.T) {
	ns := createTestNamespace(t)

	// Create cronjob
	cronJob := &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cronjob",
			Namespace: ns,
		},
		Spec: batchv1.CronJobSpec{
			Schedule: "*/5 * * * *",
			JobTemplate: batchv1.JobTemplateSpec{
				Spec: batchv1.JobSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers:    []corev1.Container{{Name: "busybox", Image: "busybox:latest", Command: []string{"echo", "hello"}}},
							RestartPolicy: corev1.RestartPolicyNever,
						},
					},
				},
			},
		},
	}
	createdCronJob, err := testClient.BatchV1().CronJobs(ns).Create(context.Background(), cronJob, metav1.CreateOptions{})
	require.NoError(t, err)

	// Create jobs owned by CronJob
	createdJobs := []*batchv1.Job{}
	for i := 0; i < 2; i++ {
		job := &batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("test-cronjob-%d", i),
				Namespace: ns,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: "batch/v1",
						Kind:       "CronJob",
						Name:       createdCronJob.Name,
						UID:        createdCronJob.UID,
					},
				},
			},
			Spec: batchv1.JobSpec{
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers:    []corev1.Container{{Name: "busybox", Image: "busybox:latest", Command: []string{"echo", "hello"}}},
						RestartPolicy: corev1.RestartPolicyNever,
					},
				},
			},
		}
		createdJob, err := testClient.BatchV1().Jobs(ns).Create(context.Background(), job, metav1.CreateOptions{})
		require.NoError(t, err)
		createdJobs = append(createdJobs, createdJob)
	}

	repo := createTestRepository(t, ns)
	defer repo.Close()
	time.Sleep(2 * time.Second) // CronJobs and Jobs need more time to sync via dynamic client

	// Get jobs for cronjob
	jobs, err := repo.GetJobsForCronJob(ns, "test-cronjob")
	require.NoError(t, err)
	assert.Len(t, jobs, 2, "should return 2 jobs owned by cronjob")

	// Verify job names
	jobNames := []string{jobs[0].Name, jobs[1].Name}
	assert.Contains(t, jobNames, "test-cronjob-0")
	assert.Contains(t, jobNames, "test-cronjob-1")
}
