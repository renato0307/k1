package k8s

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
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

func TestExtractAge(t *testing.T) {
	tests := []struct {
		name     string
		item     interface{}
		expected time.Duration
	}{
		{
			name:     "Pod with age",
			item:     Pod{Age: 5 * time.Minute},
			expected: 5 * time.Minute,
		},
		{
			name:     "Deployment with age",
			item:     Deployment{Age: 10 * time.Hour},
			expected: 10 * time.Hour,
		},
		{
			name:     "Service with age",
			item:     Service{Age: 2 * time.Hour},
			expected: 2 * time.Hour,
		},
		{
			name:     "ConfigMap with age",
			item:     ConfigMap{Age: 1 * time.Hour},
			expected: 1 * time.Hour,
		},
		{
			name:     "Secret with age",
			item:     Secret{Age: 3 * time.Hour},
			expected: 3 * time.Hour,
		},
		{
			name:     "Namespace with age",
			item:     Namespace{Age: 24 * time.Hour},
			expected: 24 * time.Hour,
		},
		{
			name:     "StatefulSet with age",
			item:     StatefulSet{Age: 6 * time.Hour},
			expected: 6 * time.Hour,
		},
		{
			name:     "DaemonSet with age",
			item:     DaemonSet{Age: 12 * time.Hour},
			expected: 12 * time.Hour,
		},
		{
			name:     "Job with age",
			item:     Job{Age: 30 * time.Minute},
			expected: 30 * time.Minute,
		},
		{
			name:     "CronJob with age",
			item:     CronJob{Age: 48 * time.Hour},
			expected: 48 * time.Hour,
		},
		{
			name:     "Node with age",
			item:     Node{Age: 168 * time.Hour},
			expected: 168 * time.Hour,
		},
		{
			name:     "Unknown type",
			item:     "unknown",
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractAge(tt.item)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractName(t *testing.T) {
	tests := []struct {
		name     string
		item     interface{}
		expected string
	}{
		{
			name:     "Pod with name",
			item:     Pod{Name: "pod-1"},
			expected: "pod-1",
		},
		{
			name:     "Deployment with name",
			item:     Deployment{Name: "deploy-1"},
			expected: "deploy-1",
		},
		{
			name:     "Service with name",
			item:     Service{Name: "svc-1"},
			expected: "svc-1",
		},
		{
			name:     "ConfigMap with name",
			item:     ConfigMap{Name: "cm-1"},
			expected: "cm-1",
		},
		{
			name:     "Secret with name",
			item:     Secret{Name: "secret-1"},
			expected: "secret-1",
		},
		{
			name:     "Namespace with name",
			item:     Namespace{Name: "ns-1"},
			expected: "ns-1",
		},
		{
			name:     "StatefulSet with name",
			item:     StatefulSet{Name: "sts-1"},
			expected: "sts-1",
		},
		{
			name:     "DaemonSet with name",
			item:     DaemonSet{Name: "ds-1"},
			expected: "ds-1",
		},
		{
			name:     "Job with name",
			item:     Job{Name: "job-1"},
			expected: "job-1",
		},
		{
			name:     "CronJob with name",
			item:     CronJob{Name: "cron-1"},
			expected: "cron-1",
		},
		{
			name:     "Node with name",
			item:     Node{Name: "node-1"},
			expected: "node-1",
		},
		{
			name:     "Unknown type",
			item:     "unknown",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractName(tt.item)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSortByAge(t *testing.T) {
	items := []interface{}{
		Pod{Name: "old-pod", Age: 10 * time.Hour},
		Pod{Name: "new-pod", Age: 1 * time.Hour},
		Pod{Name: "medium-pod", Age: 5 * time.Hour},
		Pod{Name: "ancient-pod", Age: 24 * time.Hour},
	}

	sortByAge(items)

	// Should be sorted by age (newest first)
	assert.Equal(t, "new-pod", extractName(items[0]))
	assert.Equal(t, "medium-pod", extractName(items[1]))
	assert.Equal(t, "old-pod", extractName(items[2]))
	assert.Equal(t, "ancient-pod", extractName(items[3]))
}

func TestSortByAge_SameAge(t *testing.T) {
	items := []interface{}{
		Pod{Name: "pod-c", Age: 5 * time.Hour},
		Pod{Name: "pod-a", Age: 5 * time.Hour},
		Pod{Name: "pod-b", Age: 5 * time.Hour},
	}

	sortByAge(items)

	// With same age, should be sorted by name alphabetically
	assert.Equal(t, "pod-a", extractName(items[0]))
	assert.Equal(t, "pod-b", extractName(items[1]))
	assert.Equal(t, "pod-c", extractName(items[2]))
}

func TestSortByAge_MixedTypes(t *testing.T) {
	items := []interface{}{
		Deployment{Name: "deploy-1", Age: 10 * time.Hour},
		Service{Name: "svc-1", Age: 2 * time.Hour},
		Pod{Name: "pod-1", Age: 5 * time.Hour},
	}

	sortByAge(items)

	// Should be sorted by age (newest first) regardless of type
	assert.Equal(t, "svc-1", extractName(items[0]))
	assert.Equal(t, "pod-1", extractName(items[1]))
	assert.Equal(t, "deploy-1", extractName(items[2]))
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
	}
	for _, inf := range dynamicInformers {
		allInformers = append(allInformers, inf.HasSynced)
	}

	synced := cache.WaitForCacheSync(ctx.Done(), allInformers...)
	if !synced {
		cancel()
	}
	require.True(t, synced, "Failed to sync caches")

	return &InformerRepository{
		clientset:        testClient,
		factory:          factory,
		podLister:        podLister,
		deploymentLister: deploymentLister,
		serviceLister:    serviceLister,
		dynamicClient:    dynamicClient,
		dynamicFactory:   dynamicFactory,
		resources:        resourceRegistry,
		dynamicListers:   dynamicListers,
		ctx:              ctx,
		cancel:           cancel,
	}
}
