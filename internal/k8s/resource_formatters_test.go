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
)

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

	// Validate spec section
	assert.Contains(t, describe, "Spec:")
	assert.Contains(t, describe, "nginx") // Container name from spec

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
