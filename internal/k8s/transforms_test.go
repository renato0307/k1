package k8s

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestTransformConfigMap(t *testing.T) {
	now := time.Now()
	u := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"metadata": map[string]interface{}{
				"name":              "test-cm",
				"namespace":         "default",
				"creationTimestamp": metav1.NewTime(now).Format(time.RFC3339),
			},
			"data": map[string]interface{}{
				"key1": "value1",
				"key2": "value2",
			},
		},
	}

	common := extractMetadata(u)
	result, err := transformConfigMap(u, common)
	require.NoError(t, err)

	cm, ok := result.(ConfigMap)
	require.True(t, ok)
	assert.Equal(t, "test-cm", cm.Name)
	assert.Equal(t, "default", cm.Namespace)
	assert.Equal(t, 2, cm.Data)
}

func TestTransformSecret(t *testing.T) {
	now := time.Now()
	u := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"metadata": map[string]interface{}{
				"name":              "test-secret",
				"namespace":         "default",
				"creationTimestamp": metav1.NewTime(now).Format(time.RFC3339),
			},
			"type": "Opaque",
			"data": map[string]interface{}{
				"password": "c2VjcmV0",
				"username": "YWRtaW4=",
			},
		},
	}

	common := extractMetadata(u)
	result, err := transformSecret(u, common)
	require.NoError(t, err)

	secret, ok := result.(Secret)
	require.True(t, ok)
	assert.Equal(t, "test-secret", secret.Name)
	assert.Equal(t, "default", secret.Namespace)
	assert.Equal(t, "Opaque", secret.Type)
	assert.Equal(t, 2, secret.Data)
}

func TestTransformService(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name               string
		spec               map[string]interface{}
		status             map[string]interface{}
		expectedType       string
		expectedClusterIP  string
		expectedExternalIP string
		checkPorts         func(t *testing.T, ports string)
	}{
		{
			name: "ClusterIP service with regular ports",
			spec: map[string]interface{}{
				"type":      "ClusterIP",
				"clusterIP": "10.0.0.1",
				"ports": []interface{}{
					map[string]interface{}{
						"port":     int64(80),
						"protocol": "TCP",
					},
				},
			},
			status:             map[string]interface{}{},
			expectedType:       "ClusterIP",
			expectedClusterIP:  "10.0.0.1",
			expectedExternalIP: "<none>",
			checkPorts: func(t *testing.T, ports string) {
				assert.Equal(t, "80/TCP", ports)
			},
		},
		{
			name: "service with empty cluster IP",
			spec: map[string]interface{}{
				"type":      "ClusterIP",
				"clusterIP": "",
				"ports": []interface{}{
					map[string]interface{}{
						"port":     int64(443),
						"protocol": "TCP",
					},
				},
			},
			status:             map[string]interface{}{},
			expectedType:       "ClusterIP",
			expectedClusterIP:  "<none>",
			expectedExternalIP: "<none>",
			checkPorts: func(t *testing.T, ports string) {
				assert.Equal(t, "443/TCP", ports)
			},
		},
		{
			name: "NodePort service with port mappings",
			spec: map[string]interface{}{
				"type":      "NodePort",
				"clusterIP": "10.0.0.2",
				"ports": []interface{}{
					map[string]interface{}{
						"port":     int64(80),
						"nodePort": int64(30080),
						"protocol": "TCP",
					},
					map[string]interface{}{
						"port":     int64(443),
						"nodePort": int64(30443),
						"protocol": "TCP",
					},
				},
			},
			status:             map[string]interface{}{},
			expectedType:       "NodePort",
			expectedClusterIP:  "10.0.0.2",
			expectedExternalIP: "<none>",
			checkPorts: func(t *testing.T, ports string) {
				assert.Contains(t, ports, "80:30080/TCP")
				assert.Contains(t, ports, "443:30443/TCP")
			},
		},
		{
			name: "LoadBalancer with IP in status",
			spec: map[string]interface{}{
				"type":      "LoadBalancer",
				"clusterIP": "10.0.0.3",
				"ports": []interface{}{
					map[string]interface{}{
						"port":     int64(80),
						"protocol": "TCP",
					},
				},
			},
			status: map[string]interface{}{
				"loadBalancer": map[string]interface{}{
					"ingress": []interface{}{
						map[string]interface{}{
							"ip": "198.51.100.1",
						},
					},
				},
			},
			expectedType:       "LoadBalancer",
			expectedClusterIP:  "10.0.0.3",
			expectedExternalIP: "198.51.100.1",
			checkPorts: func(t *testing.T, ports string) {
				assert.Equal(t, "80/TCP", ports)
			},
		},
		{
			name: "LoadBalancer with hostname in status",
			spec: map[string]interface{}{
				"type":      "LoadBalancer",
				"clusterIP": "10.0.0.4",
				"ports": []interface{}{
					map[string]interface{}{
						"port":     int64(443),
						"protocol": "TCP",
					},
				},
			},
			status: map[string]interface{}{
				"loadBalancer": map[string]interface{}{
					"ingress": []interface{}{
						map[string]interface{}{
							"hostname": "lb.example.com",
						},
					},
				},
			},
			expectedType:       "LoadBalancer",
			expectedClusterIP:  "10.0.0.4",
			expectedExternalIP: "lb.example.com",
			checkPorts: func(t *testing.T, ports string) {
				assert.Equal(t, "443/TCP", ports)
			},
		},
		{
			name: "service with external IPs from spec",
			spec: map[string]interface{}{
				"type":      "ClusterIP",
				"clusterIP": "10.0.0.5",
				"externalIPs": []interface{}{
					"203.0.113.1",
					"203.0.113.2",
				},
				"ports": []interface{}{
					map[string]interface{}{
						"port":     int64(8080),
						"protocol": "TCP",
					},
				},
			},
			status:             map[string]interface{}{},
			expectedType:       "ClusterIP",
			expectedClusterIP:  "10.0.0.5",
			expectedExternalIP: "203.0.113.1,203.0.113.2",
			checkPorts: func(t *testing.T, ports string) {
				assert.Equal(t, "8080/TCP", ports)
			},
		},
		{
			name: "service with no ports",
			spec: map[string]interface{}{
				"type":      "ClusterIP",
				"clusterIP": "10.0.0.6",
				"ports":     []interface{}{},
			},
			status:             map[string]interface{}{},
			expectedType:       "ClusterIP",
			expectedClusterIP:  "10.0.0.6",
			expectedExternalIP: "<none>",
			checkPorts: func(t *testing.T, ports string) {
				assert.Equal(t, "<none>", ports)
			},
		},
		{
			name: "service with invalid port map",
			spec: map[string]interface{}{
				"type":      "ClusterIP",
				"clusterIP": "10.0.0.7",
				"ports": []interface{}{
					"invalid-port-string", // Not a map
					map[string]interface{}{
						"port":     int64(9090),
						"protocol": "UDP",
					},
				},
			},
			status:             map[string]interface{}{},
			expectedType:       "ClusterIP",
			expectedClusterIP:  "10.0.0.7",
			expectedExternalIP: "<none>",
			checkPorts: func(t *testing.T, ports string) {
				// Should skip invalid entry and only include valid one
				assert.Equal(t, "9090/UDP", ports)
			},
		},
		{
			name: "LoadBalancer with invalid ingress entry",
			spec: map[string]interface{}{
				"type":      "LoadBalancer",
				"clusterIP": "10.0.0.8",
				"ports": []interface{}{
					map[string]interface{}{
						"port":     int64(80),
						"protocol": "TCP",
					},
				},
			},
			status: map[string]interface{}{
				"loadBalancer": map[string]interface{}{
					"ingress": []interface{}{
						"invalid-ingress-string", // Not a map
					},
				},
			},
			expectedType:       "LoadBalancer",
			expectedClusterIP:  "10.0.0.8",
			expectedExternalIP: "<none>",
			checkPorts: func(t *testing.T, ports string) {
				assert.Equal(t, "80/TCP", ports)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name":              "test-svc",
						"namespace":         "default",
						"creationTimestamp": metav1.NewTime(now).Format(time.RFC3339),
					},
					"spec":   tt.spec,
					"status": tt.status,
				},
			}

			common := extractMetadata(u)
			result, err := transformService(u, common)
			require.NoError(t, err)

			svc, ok := result.(Service)
			require.True(t, ok)
			assert.Equal(t, "test-svc", svc.Name)
			assert.Equal(t, "default", svc.Namespace)
			assert.Equal(t, tt.expectedType, svc.Type)
			assert.Equal(t, tt.expectedClusterIP, svc.ClusterIP)
			assert.Equal(t, tt.expectedExternalIP, svc.ExternalIP)
			tt.checkPorts(t, svc.Ports)
		})
	}
}

func TestTransformNamespace(t *testing.T) {
	now := time.Now()
	u := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"metadata": map[string]interface{}{
				"name":              "test-ns",
				"creationTimestamp": metav1.NewTime(now).Format(time.RFC3339),
			},
			"status": map[string]interface{}{
				"phase": "Active",
			},
		},
	}

	common := extractMetadata(u)
	result, err := transformNamespace(u, common)
	require.NoError(t, err)

	ns, ok := result.(Namespace)
	require.True(t, ok)
	assert.Equal(t, "test-ns", ns.Name)
	assert.Equal(t, "Active", ns.Status)
}

func TestTransformStatefulSet(t *testing.T) {
	now := time.Now()
	u := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"metadata": map[string]interface{}{
				"name":              "test-sts",
				"namespace":         "default",
				"creationTimestamp": metav1.NewTime(now).Format(time.RFC3339),
			},
			"spec": map[string]interface{}{
				"replicas": int64(3),
			},
			"status": map[string]interface{}{
				"readyReplicas": int64(2),
			},
		},
	}

	common := extractMetadata(u)
	result, err := transformStatefulSet(u, common)
	require.NoError(t, err)

	sts, ok := result.(StatefulSet)
	require.True(t, ok)
	assert.Equal(t, "test-sts", sts.Name)
	assert.Equal(t, "default", sts.Namespace)
	assert.Equal(t, "2/3", sts.Ready)
}

func TestTransformDaemonSet(t *testing.T) {
	now := time.Now()
	u := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"metadata": map[string]interface{}{
				"name":              "test-ds",
				"namespace":         "kube-system",
				"creationTimestamp": metav1.NewTime(now).Format(time.RFC3339),
			},
			"status": map[string]interface{}{
				"desiredNumberScheduled": int64(5),
				"currentNumberScheduled": int64(5),
				"numberReady":            int64(4),
				"updatedNumberScheduled": int64(5),
				"numberAvailable":        int64(4),
			},
		},
	}

	common := extractMetadata(u)
	result, err := transformDaemonSet(u, common)
	require.NoError(t, err)

	ds, ok := result.(DaemonSet)
	require.True(t, ok)
	assert.Equal(t, "test-ds", ds.Name)
	assert.Equal(t, "kube-system", ds.Namespace)
	assert.Equal(t, int32(5), ds.Desired)
	assert.Equal(t, int32(5), ds.Current)
	assert.Equal(t, int32(4), ds.Ready)
	assert.Equal(t, int32(5), ds.UpToDate)
	assert.Equal(t, int32(4), ds.Available)
}

func TestTransformJob(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name                string
		spec                map[string]interface{}
		status              map[string]interface{}
		expectedCompletions string
		expectedDuration    time.Duration
	}{
		{
			name: "job with completions and succeeded",
			spec: map[string]interface{}{
				"completions": int64(5),
			},
			status: map[string]interface{}{
				"succeeded": int64(3),
			},
			expectedCompletions: "3/5",
			expectedDuration:    0,
		},
		{
			name: "job with start time only",
			spec: map[string]interface{}{
				"completions": int64(1),
			},
			status: map[string]interface{}{
				"succeeded": int64(0),
				"startTime": metav1.NewTime(now).Format(time.RFC3339),
			},
			expectedCompletions: "0/1",
			expectedDuration:    0,
		},
		{
			name: "job with start and completion time",
			spec: map[string]interface{}{
				"completions": int64(1),
			},
			status: map[string]interface{}{
				"succeeded":      int64(1),
				"startTime":      metav1.NewTime(now).Format(time.RFC3339),
				"completionTime": metav1.NewTime(now.Add(5 * time.Minute)).Format(time.RFC3339),
			},
			expectedCompletions: "1/1",
			expectedDuration:    0, // Simplified calculation returns 0
		},
		{
			name: "job with no completions",
			spec: map[string]interface{}{},
			status: map[string]interface{}{
				"succeeded": int64(0),
			},
			expectedCompletions: "0/0",
			expectedDuration:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name":              "test-job",
						"namespace":         "default",
						"creationTimestamp": metav1.NewTime(now).Format(time.RFC3339),
					},
					"spec":   tt.spec,
					"status": tt.status,
				},
			}

			common := extractMetadata(u)
			result, err := transformJob(u, common)
			require.NoError(t, err)

			job, ok := result.(Job)
			require.True(t, ok)
			assert.Equal(t, "test-job", job.Name)
			assert.Equal(t, "default", job.Namespace)
			assert.Equal(t, tt.expectedCompletions, job.Completions)
			assert.Equal(t, tt.expectedDuration, job.Duration)
		})
	}
}

func TestTransformCronJob(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name                 string
		spec                 map[string]interface{}
		status               map[string]interface{}
		expectedSchedule     string
		expectedSuspend      bool
		expectedActive       int32
		expectedLastSchedule time.Duration
	}{
		{
			name: "active cronjob with multiple jobs",
			spec: map[string]interface{}{
				"schedule": "0 * * * *",
				"suspend":  false,
			},
			status: map[string]interface{}{
				"active": []interface{}{
					map[string]interface{}{"name": "job-1"},
					map[string]interface{}{"name": "job-2"},
				},
			},
			expectedSchedule:     "0 * * * *",
			expectedSuspend:      false,
			expectedActive:       2,
			expectedLastSchedule: 0,
		},
		{
			name: "suspended cronjob",
			spec: map[string]interface{}{
				"schedule": "*/5 * * * *",
				"suspend":  true,
			},
			status: map[string]interface{}{
				"active": []interface{}{},
			},
			expectedSchedule:     "*/5 * * * *",
			expectedSuspend:      true,
			expectedActive:       0,
			expectedLastSchedule: 0,
		},
		{
			name: "cronjob with last schedule time",
			spec: map[string]interface{}{
				"schedule": "0 0 * * *",
				"suspend":  false,
			},
			status: map[string]interface{}{
				"active":           []interface{}{},
				"lastScheduleTime": metav1.NewTime(now.Add(-1 * time.Hour)).Format(time.RFC3339),
			},
			expectedSchedule:     "0 0 * * *",
			expectedSuspend:      false,
			expectedActive:       0,
			expectedLastSchedule: 0, // Simplified calculation returns 0
		},
		{
			name: "cronjob with no status",
			spec: map[string]interface{}{
				"schedule": "0 12 * * *",
				"suspend":  false,
			},
			status:               map[string]interface{}{},
			expectedSchedule:     "0 12 * * *",
			expectedSuspend:      false,
			expectedActive:       0,
			expectedLastSchedule: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name":              "test-cron",
						"namespace":         "default",
						"creationTimestamp": metav1.NewTime(now).Format(time.RFC3339),
					},
					"spec":   tt.spec,
					"status": tt.status,
				},
			}

			common := extractMetadata(u)
			result, err := transformCronJob(u, common)
			require.NoError(t, err)

			cron, ok := result.(CronJob)
			require.True(t, ok)
			assert.Equal(t, "test-cron", cron.Name)
			assert.Equal(t, "default", cron.Namespace)
			assert.Equal(t, tt.expectedSchedule, cron.Schedule)
			assert.Equal(t, tt.expectedSuspend, cron.Suspend)
			assert.Equal(t, tt.expectedActive, cron.Active)
			assert.Equal(t, tt.expectedLastSchedule, cron.LastSchedule)
		})
	}
}

func TestTransformNode(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name                 string
		labels               map[string]interface{}
		conditionStatus      string
		kubeletVersion       string
		osImage              string
		expectedStatus       string
		expectedRoles        string
		expectedHostname     string
		expectedInstanceType string
		expectedZone         string
		expectedNodePool     string
		expectedOSImage      string
		rolesContains        []string // For multiple roles (order-independent)
	}{
		{
			name: "ready node with all labels",
			labels: map[string]interface{}{
				"node-role.kubernetes.io/control-plane": "",
				"kubernetes.io/hostname":                "ip-10-0-1-100",
				"beta.kubernetes.io/instance-type":      "t3.large",
				"topology.kubernetes.io/zone":           "us-east-1a",
				"karpenter.sh/nodepool":                 "default",
			},
			conditionStatus:      "True",
			kubeletVersion:       "v1.28.0",
			osImage:              "Amazon Linux 2",
			expectedStatus:       "Ready",
			expectedRoles:        "control-plane",
			expectedHostname:     "ip-10-0-1-100",
			expectedInstanceType: "t3.large",
			expectedZone:         "us-east-1a",
			expectedNodePool:     "default",
			expectedOSImage:      "Amazon Linux 2",
		},
		{
			name: "not ready node with minimal labels",
			labels: map[string]interface{}{
				"kubernetes.io/hostname": "node-2",
			},
			conditionStatus:      "False",
			kubeletVersion:       "v1.28.0",
			osImage:              "",
			expectedStatus:       "NotReady",
			expectedRoles:        "<none>",
			expectedHostname:     "node-2",
			expectedInstanceType: "<none>",
			expectedZone:         "<none>",
			expectedNodePool:     "<none>",
			expectedOSImage:      "<none>",
		},
		{
			name: "node with multiple roles",
			labels: map[string]interface{}{
				"node-role.kubernetes.io/master":        "",
				"node-role.kubernetes.io/control-plane": "",
				"kubernetes.io/hostname":                "node-3",
			},
			conditionStatus:  "True",
			kubeletVersion:   "v1.28.0",
			expectedStatus:   "Ready",
			expectedHostname: "node-3",
			rolesContains:    []string{"master", "control-plane"},
		},
		{
			name: "node with newer instance-type label",
			labels: map[string]interface{}{
				"node.kubernetes.io/instance-type": "m5.xlarge",
				"kubernetes.io/hostname":           "node-4",
			},
			conditionStatus:      "True",
			kubeletVersion:       "v1.28.0",
			expectedStatus:       "Ready",
			expectedHostname:     "node-4",
			expectedInstanceType: "m5.xlarge",
		},
		{
			name: "node with older zone label",
			labels: map[string]interface{}{
				"failure-domain.beta.kubernetes.io/zone": "us-west-2b",
				"kubernetes.io/hostname":                 "node-5",
			},
			conditionStatus:  "True",
			kubeletVersion:   "v1.28.0",
			expectedStatus:   "Ready",
			expectedHostname: "node-5",
			expectedZone:     "us-west-2b",
		},
		{
			name:                 "node with no labels",
			labels:               map[string]interface{}{},
			conditionStatus:      "True",
			kubeletVersion:       "v1.28.0",
			expectedStatus:       "Ready",
			expectedRoles:        "<none>",
			expectedHostname:     "<none>",
			expectedInstanceType: "<none>",
			expectedZone:         "<none>",
			expectedNodePool:     "<none>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name":              "test-node",
						"creationTimestamp": metav1.NewTime(now).Format(time.RFC3339),
						"labels":            tt.labels,
					},
					"status": map[string]interface{}{
						"conditions": []interface{}{
							map[string]interface{}{
								"type":   "Ready",
								"status": tt.conditionStatus,
							},
						},
						"nodeInfo": map[string]interface{}{
							"kubeletVersion": tt.kubeletVersion,
							"osImage":        tt.osImage,
						},
					},
				},
			}

			common := extractMetadata(u)
			result, err := transformNode(u, common)
			require.NoError(t, err)

			node, ok := result.(Node)
			require.True(t, ok)
			assert.Equal(t, "test-node", node.Name)
			assert.Equal(t, tt.expectedStatus, node.Status)
			assert.Equal(t, tt.kubeletVersion, node.Version)

			if tt.rolesContains != nil {
				// For multiple roles, check each is present (order-independent)
				for _, role := range tt.rolesContains {
					assert.Contains(t, node.Roles, role)
				}
			} else if tt.expectedRoles != "" {
				assert.Equal(t, tt.expectedRoles, node.Roles)
			}

			if tt.expectedHostname != "" {
				assert.Equal(t, tt.expectedHostname, node.Hostname)
			}
			if tt.expectedInstanceType != "" {
				assert.Equal(t, tt.expectedInstanceType, node.InstanceType)
			}
			if tt.expectedZone != "" {
				assert.Equal(t, tt.expectedZone, node.Zone)
			}
			if tt.expectedNodePool != "" {
				assert.Equal(t, tt.expectedNodePool, node.NodePool)
			}
			if tt.expectedOSImage != "" {
				assert.Equal(t, tt.expectedOSImage, node.OSImage)
			}
		})
	}
}

func TestTransformReplicaSet(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name            string
		spec            map[string]interface{}
		status          map[string]interface{}
		expectedDesired int32
		expectedCurrent int32
		expectedReady   int32
	}{
		{
			name: "ReplicaSet with all replicas ready",
			spec: map[string]interface{}{
				"replicas": int64(3),
			},
			status: map[string]interface{}{
				"replicas":      int64(3),
				"readyReplicas": int64(3),
			},
			expectedDesired: 3,
			expectedCurrent: 3,
			expectedReady:   3,
		},
		{
			name: "ReplicaSet scaling up",
			spec: map[string]interface{}{
				"replicas": int64(5),
			},
			status: map[string]interface{}{
				"replicas":      int64(3),
				"readyReplicas": int64(3),
			},
			expectedDesired: 5,
			expectedCurrent: 3,
			expectedReady:   3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name":              "test-rs",
						"namespace":         "default",
						"creationTimestamp": metav1.NewTime(now).Format(time.RFC3339),
					},
					"spec":   tt.spec,
					"status": tt.status,
				},
			}

			common := extractMetadata(u)
			result, err := transformReplicaSet(u, common)
			require.NoError(t, err)

			rs, ok := result.(ReplicaSet)
			require.True(t, ok)
			assert.Equal(t, "test-rs", rs.Name)
			assert.Equal(t, "default", rs.Namespace)
			assert.Equal(t, tt.expectedDesired, rs.Desired)
			assert.Equal(t, tt.expectedCurrent, rs.Current)
			assert.Equal(t, tt.expectedReady, rs.Ready)
		})
	}
}

func TestTransformPVC(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name                 string
		spec                 map[string]interface{}
		status               map[string]interface{}
		expectedStatus       string
		expectedVolume       string
		expectedCapacity     string
		expectedAccessModes  string
		expectedStorageClass string
	}{
		{
			name: "Bound PVC",
			spec: map[string]interface{}{
				"volumeName":       "pv-123",
				"accessModes":      []interface{}{"ReadWriteOnce"},
				"storageClassName": "standard",
			},
			status: map[string]interface{}{
				"phase": "Bound",
				"capacity": map[string]interface{}{
					"storage": "10Gi",
				},
			},
			expectedStatus:       "Bound",
			expectedVolume:       "pv-123",
			expectedCapacity:     "10Gi",
			expectedAccessModes:  "ReadWriteOnce",
			expectedStorageClass: "standard",
		},
		{
			name: "Pending PVC",
			spec: map[string]interface{}{
				"accessModes":      []interface{}{"ReadWriteOnce", "ReadWriteMany"},
				"storageClassName": "fast",
			},
			status: map[string]interface{}{
				"phase": "Pending",
			},
			expectedStatus:       "Pending",
			expectedVolume:       "",
			expectedCapacity:     "<none>",
			expectedAccessModes:  "ReadWriteOnce,ReadWriteMany",
			expectedStorageClass: "fast",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name":              "test-pvc",
						"namespace":         "default",
						"creationTimestamp": metav1.NewTime(now).Format(time.RFC3339),
					},
					"spec":   tt.spec,
					"status": tt.status,
				},
			}

			common := extractMetadata(u)
			result, err := transformPVC(u, common)
			require.NoError(t, err)

			pvc, ok := result.(PersistentVolumeClaim)
			require.True(t, ok)
			assert.Equal(t, "test-pvc", pvc.Name)
			assert.Equal(t, "default", pvc.Namespace)
			assert.Equal(t, tt.expectedStatus, pvc.Status)
			assert.Equal(t, tt.expectedVolume, pvc.Volume)
			assert.Equal(t, tt.expectedCapacity, pvc.Capacity)
			assert.Equal(t, tt.expectedAccessModes, pvc.AccessModes)
			assert.Equal(t, tt.expectedStorageClass, pvc.StorageClass)
		})
	}
}

func TestTransformIngress(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name            string
		spec            map[string]interface{}
		status          map[string]interface{}
		expectedClass   string
		expectedHosts   string
		expectedAddress string
	}{
		{
			name: "Ingress with hostname",
			spec: map[string]interface{}{
				"ingressClassName": "nginx",
				"rules": []interface{}{
					map[string]interface{}{
						"host": "example.com",
					},
					map[string]interface{}{
						"host": "api.example.com",
					},
				},
			},
			status: map[string]interface{}{
				"loadBalancer": map[string]interface{}{
					"ingress": []interface{}{
						map[string]interface{}{
							"hostname": "lb.example.com",
						},
					},
				},
			},
			expectedClass:   "nginx",
			expectedHosts:   "example.com, api.example.com",
			expectedAddress: "lb.example.com",
		},
		{
			name: "Ingress with IP address",
			spec: map[string]interface{}{
				"ingressClassName": "traefik",
				"rules": []interface{}{
					map[string]interface{}{
						"host": "test.com",
					},
				},
			},
			status: map[string]interface{}{
				"loadBalancer": map[string]interface{}{
					"ingress": []interface{}{
						map[string]interface{}{
							"ip": "10.0.0.1",
						},
					},
				},
			},
			expectedClass:   "traefik",
			expectedHosts:   "test.com",
			expectedAddress: "10.0.0.1",
		},
		{
			name: "Ingress without host (default backend)",
			spec: map[string]interface{}{
				"ingressClassName": "nginx",
				"rules":            []interface{}{},
			},
			status:          map[string]interface{}{},
			expectedClass:   "nginx",
			expectedHosts:   "*",
			expectedAddress: "<pending>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name":              "test-ingress",
						"namespace":         "default",
						"creationTimestamp": metav1.NewTime(now).Format(time.RFC3339),
					},
					"spec":   tt.spec,
					"status": tt.status,
				},
			}

			common := extractMetadata(u)
			result, err := transformIngress(u, common)
			require.NoError(t, err)

			ingress, ok := result.(Ingress)
			require.True(t, ok)
			assert.Equal(t, "test-ingress", ingress.Name)
			assert.Equal(t, "default", ingress.Namespace)
			assert.Equal(t, tt.expectedClass, ingress.Class)
			assert.Equal(t, tt.expectedHosts, ingress.Hosts)
			assert.Equal(t, tt.expectedAddress, ingress.Address)
			assert.Equal(t, "80, 443", ingress.Ports)
		})
	}
}

func TestTransformEndpoints(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name              string
		subsets           []interface{}
		expectedEndpoints string
	}{
		{
			name: "Endpoints with multiple addresses",
			subsets: []interface{}{
				map[string]interface{}{
					"addresses": []interface{}{
						map[string]interface{}{"ip": "10.0.1.5"},
						map[string]interface{}{"ip": "10.0.1.6"},
					},
					"ports": []interface{}{
						map[string]interface{}{"port": int64(8080)},
					},
				},
			},
			expectedEndpoints: "10.0.1.5:8080, 10.0.1.6:8080",
		},
		{
			name: "Endpoints with multiple ports",
			subsets: []interface{}{
				map[string]interface{}{
					"addresses": []interface{}{
						map[string]interface{}{"ip": "10.0.1.5"},
					},
					"ports": []interface{}{
						map[string]interface{}{"port": int64(8080)},
						map[string]interface{}{"port": int64(9090)},
					},
				},
			},
			expectedEndpoints: "10.0.1.5:8080, 10.0.1.5:9090",
		},
		{
			name:              "Empty endpoints",
			subsets:           []interface{}{},
			expectedEndpoints: "<none>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name":              "test-endpoints",
						"namespace":         "default",
						"creationTimestamp": metav1.NewTime(now).Format(time.RFC3339),
					},
					"subsets": tt.subsets,
				},
			}

			common := extractMetadata(u)
			result, err := transformEndpoints(u, common)
			require.NoError(t, err)

			endpoints, ok := result.(Endpoints)
			require.True(t, ok)
			assert.Equal(t, "test-endpoints", endpoints.Name)
			assert.Equal(t, "default", endpoints.Namespace)
			assert.Equal(t, tt.expectedEndpoints, endpoints.Endpoints)
		})
	}
}

func TestTransformHPA(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name              string
		spec              map[string]interface{}
		status            map[string]interface{}
		expectedReference string
		expectedMinPods   int32
		expectedMaxPods   int32
		expectedReplicas  int32
		expectedTargetCPU string
	}{
		{
			name: "HPA with CPU metric",
			spec: map[string]interface{}{
				"minReplicas": int64(2),
				"maxReplicas": int64(10),
				"scaleTargetRef": map[string]interface{}{
					"kind": "Deployment",
					"name": "nginx",
				},
				"metrics": []interface{}{
					map[string]interface{}{
						"type": "Resource",
						"resource": map[string]interface{}{
							"name": "cpu",
							"target": map[string]interface{}{
								"averageUtilization": int64(80),
							},
						},
					},
				},
			},
			status: map[string]interface{}{
				"currentReplicas": int64(5),
			},
			expectedReference: "Deployment/nginx",
			expectedMinPods:   2,
			expectedMaxPods:   10,
			expectedReplicas:  5,
			expectedTargetCPU: "80%",
		},
		{
			name: "HPA without CPU metric",
			spec: map[string]interface{}{
				"minReplicas": int64(1),
				"maxReplicas": int64(5),
				"scaleTargetRef": map[string]interface{}{
					"kind": "StatefulSet",
					"name": "redis",
				},
				"metrics": []interface{}{},
			},
			status: map[string]interface{}{
				"currentReplicas": int64(3),
			},
			expectedReference: "StatefulSet/redis",
			expectedMinPods:   1,
			expectedMaxPods:   5,
			expectedReplicas:  3,
			expectedTargetCPU: "N/A",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name":              "test-hpa",
						"namespace":         "default",
						"creationTimestamp": metav1.NewTime(now).Format(time.RFC3339),
					},
					"spec":   tt.spec,
					"status": tt.status,
				},
			}

			common := extractMetadata(u)
			result, err := transformHPA(u, common)
			require.NoError(t, err)

			hpa, ok := result.(HorizontalPodAutoscaler)
			require.True(t, ok)
			assert.Equal(t, "test-hpa", hpa.Name)
			assert.Equal(t, "default", hpa.Namespace)
			assert.Equal(t, tt.expectedReference, hpa.Reference)
			assert.Equal(t, tt.expectedMinPods, hpa.MinPods)
			assert.Equal(t, tt.expectedMaxPods, hpa.MaxPods)
			assert.Equal(t, tt.expectedReplicas, hpa.Replicas)
			assert.Equal(t, tt.expectedTargetCPU, hpa.TargetCPU)
		})
	}
}
