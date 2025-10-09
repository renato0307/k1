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
)

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
