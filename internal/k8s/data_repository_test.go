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
)

// TestDataRepository_GetPods tests pod data access
func TestDataRepository_GetPods(t *testing.T) {
	if testCfg == nil {
		t.Skip("Skipping test - envtest not initialized")
	}

	ns := createTestNamespace(t)
	comp := createTestRepository(t, ns)
	defer comp.manager.Close()

	// Initially empty
	pods, err := comp.dataRepo.GetPods()
	require.NoError(t, err)
	assert.Empty(t, pods)

	// Create a pod
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: ns,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "nginx", Image: "nginx:latest"},
			},
		},
	}
	_, err = testClient.CoreV1().Pods(ns).Create(context.Background(), pod, metav1.CreateOptions{})
	require.NoError(t, err)

	// Wait for cache sync
	time.Sleep(100 * time.Millisecond)

	// Should now have 1 pod
	pods, err = comp.dataRepo.GetPods()
	require.NoError(t, err)
	require.Len(t, pods, 1)
	assert.Equal(t, "test-pod", pods[0].Name)
	assert.Equal(t, ns, pods[0].Namespace)
}

// TestDataRepository_GetDeployments tests deployment data access
func TestDataRepository_GetDeployments(t *testing.T) {
	if testCfg == nil {
		t.Skip("Skipping test - envtest not initialized")
	}

	ns := createTestNamespace(t)
	comp := createTestRepository(t, ns)
	defer comp.manager.Close()

	// Initially empty
	deployments, err := comp.dataRepo.GetDeployments()
	require.NoError(t, err)
	assert.Empty(t, deployments)

	// Create a deployment
	replicas := int32(3)
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-deploy",
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
	_, err = testClient.AppsV1().Deployments(ns).Create(context.Background(), deployment, metav1.CreateOptions{})
	require.NoError(t, err)

	// Wait for cache sync
	time.Sleep(100 * time.Millisecond)

	// Should now have 1 deployment
	deployments, err = comp.dataRepo.GetDeployments()
	require.NoError(t, err)
	require.Len(t, deployments, 1)
	assert.Equal(t, "test-deploy", deployments[0].Name)
	assert.Equal(t, ns, deployments[0].Namespace)
}

// TestDataRepository_GetServices tests service data access
func TestDataRepository_GetServices(t *testing.T) {
	if testCfg == nil {
		t.Skip("Skipping test - envtest not initialized")
	}

	ns := createTestNamespace(t)
	comp := createTestRepository(t, ns)
	defer comp.manager.Close()

	// Initially empty
	services, err := comp.dataRepo.GetServices()
	require.NoError(t, err)
	assert.Empty(t, services)

	// Create a service
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
	_, err = testClient.CoreV1().Services(ns).Create(context.Background(), service, metav1.CreateOptions{})
	require.NoError(t, err)

	// Wait for cache sync
	time.Sleep(100 * time.Millisecond)

	// Should now have 1 service
	services, err = comp.dataRepo.GetServices()
	require.NoError(t, err)
	require.Len(t, services, 1)
	assert.Equal(t, "test-svc", services[0].Name)
	assert.Equal(t, ns, services[0].Namespace)
}

// TestDataRepository_GetResources tests generic resource access
func TestDataRepository_GetResources(t *testing.T) {
	if testCfg == nil {
		t.Skip("Skipping test - envtest not initialized")
	}

	tests := []struct {
		name         string
		resourceType ResourceType
		createFunc   func(t *testing.T, ns string)
		expectCount  int
	}{
		{
			name:         "pods",
			resourceType: ResourceTypePod,
			createFunc: func(t *testing.T, ns string) {
				pod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-pod",
						Namespace: ns,
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{Name: "nginx", Image: "nginx:latest"},
						},
					},
				}
				_, err := testClient.CoreV1().Pods(ns).Create(context.Background(), pod, metav1.CreateOptions{})
				require.NoError(t, err)
			},
			expectCount: 1,
		},
		{
			name:         "deployments",
			resourceType: ResourceTypeDeployment,
			createFunc: func(t *testing.T, ns string) {
				replicas := int32(1)
				deployment := &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-deploy",
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
				_, err := testClient.AppsV1().Deployments(ns).Create(context.Background(), deployment, metav1.CreateOptions{})
				require.NoError(t, err)
			},
			expectCount: 1,
		},
		{
			name:         "services",
			resourceType: ResourceTypeService,
			createFunc: func(t *testing.T, ns string) {
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
			},
			expectCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ns := createTestNamespace(t)
			comp := createTestRepository(t, ns)
			defer comp.manager.Close()

			// Create resource
			tt.createFunc(t, ns)

			// Wait for cache sync
			time.Sleep(100 * time.Millisecond)

			// Get resources
			resources, err := comp.dataRepo.GetResources(tt.resourceType)
			require.NoError(t, err)
			assert.Len(t, resources, tt.expectCount)
		})
	}
}

// TestDataRepository_GetResources_UnknownType tests error handling
func TestDataRepository_GetResources_UnknownType(t *testing.T) {
	if testCfg == nil {
		t.Skip("Skipping test - envtest not initialized")
	}

	ns := createTestNamespace(t)
	comp := createTestRepository(t, ns)
	defer comp.manager.Close()

	_, err := comp.dataRepo.GetResources("unknown-type")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown resource type")
}

// TestDataRepository_SortByAge tests that results are sorted correctly
func TestDataRepository_SortByAge(t *testing.T) {
	if testCfg == nil {
		t.Skip("Skipping test - envtest not initialized")
	}

	ns := createTestNamespace(t)
	comp := createTestRepository(t, ns)
	defer comp.manager.Close()

	// Create pods with different creation times
	now := time.Now()
	pods := []*corev1.Pod{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "old-pod",
				Namespace:         ns,
				CreationTimestamp: metav1.NewTime(now.Add(-2 * time.Hour)),
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{Name: "nginx", Image: "nginx:latest"},
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "new-pod",
				Namespace:         ns,
				CreationTimestamp: metav1.NewTime(now.Add(-1 * time.Hour)),
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{Name: "nginx", Image: "nginx:latest"},
				},
			},
		},
	}

	for _, pod := range pods {
		_, err := testClient.CoreV1().Pods(ns).Create(context.Background(), pod, metav1.CreateOptions{})
		require.NoError(t, err)
	}

	// Wait for cache sync
	time.Sleep(100 * time.Millisecond)

	// Get pods
	retrievedPods, err := comp.dataRepo.GetPods()
	require.NoError(t, err)
	require.Len(t, retrievedPods, 2)

	// Should be sorted by age (newest first)
	assert.Equal(t, "new-pod", retrievedPods[0].Name)
	assert.Equal(t, "old-pod", retrievedPods[1].Name)
}
