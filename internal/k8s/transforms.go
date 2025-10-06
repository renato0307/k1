package k8s

import (
	"fmt"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// transformPod converts an unstructured pod to a typed Pod
func transformPod(u *unstructured.Unstructured) (interface{}, error) {
	namespace := u.GetNamespace()
	name := u.GetName()
	age := time.Since(u.GetCreationTimestamp().Time)

	// Extract container status for ready count
	containerStatuses, _, _ := unstructured.NestedSlice(u.Object, "status", "containerStatuses")
	readyContainers := 0
	totalContainers := len(containerStatuses)
	totalRestarts := int32(0)

	for _, cs := range containerStatuses {
		csMap, ok := cs.(map[string]interface{})
		if !ok {
			continue
		}
		if ready, _, _ := unstructured.NestedBool(csMap, "ready"); ready {
			readyContainers++
		}
		if restartCount, _, _ := unstructured.NestedInt64(csMap, "restartCount"); restartCount > 0 {
			totalRestarts += int32(restartCount)
		}
	}

	readyStatus := fmt.Sprintf("%d/%d", readyContainers, totalContainers)

	// Extract pod status
	status, _, _ := unstructured.NestedString(u.Object, "status", "phase")

	// Extract node and IP
	node, _, _ := unstructured.NestedString(u.Object, "spec", "nodeName")
	ip, _, _ := unstructured.NestedString(u.Object, "status", "podIP")

	return Pod{
		Namespace: namespace,
		Name:      name,
		Ready:     readyStatus,
		Status:    status,
		Restarts:  totalRestarts,
		Age:       age,
		CreatedAt: u.GetCreationTimestamp().Time,
		Node:      node,
		IP:        ip,
	}, nil
}

// transformDeployment converts an unstructured deployment to a typed Deployment
func transformDeployment(u *unstructured.Unstructured) (interface{}, error) {
	namespace := u.GetNamespace()
	name := u.GetName()
	age := time.Since(u.GetCreationTimestamp().Time)

	// Extract replica counts
	ready, _, _ := unstructured.NestedInt64(u.Object, "status", "readyReplicas")
	desired, _, _ := unstructured.NestedInt64(u.Object, "spec", "replicas")
	upToDate, _, _ := unstructured.NestedInt64(u.Object, "status", "updatedReplicas")
	available, _, _ := unstructured.NestedInt64(u.Object, "status", "availableReplicas")

	readyStatus := fmt.Sprintf("%d/%d", ready, desired)

	return Deployment{
		Namespace: namespace,
		Name:      name,
		Ready:     readyStatus,
		UpToDate:  int32(upToDate),
		Available: int32(available),
		Age:       age,
		CreatedAt: u.GetCreationTimestamp().Time,
	}, nil
}

// transformService converts an unstructured service to a typed Service
func transformService(u *unstructured.Unstructured) (interface{}, error) {
	namespace := u.GetNamespace()
	name := u.GetName()
	age := time.Since(u.GetCreationTimestamp().Time)

	// Extract service type
	svcType, _, _ := unstructured.NestedString(u.Object, "spec", "type")

	// Extract cluster IP
	clusterIP, _, _ := unstructured.NestedString(u.Object, "spec", "clusterIP")
	if clusterIP == "" {
		clusterIP = "<none>"
	}

	// Extract external IP
	externalIP := "<none>"

	// Check load balancer ingress
	lbIngress, _, _ := unstructured.NestedSlice(u.Object, "status", "loadBalancer", "ingress")
	if len(lbIngress) > 0 {
		ingressMap, ok := lbIngress[0].(map[string]interface{})
		if ok {
			if ip, _, _ := unstructured.NestedString(ingressMap, "ip"); ip != "" {
				externalIP = ip
			} else if hostname, _, _ := unstructured.NestedString(ingressMap, "hostname"); hostname != "" {
				externalIP = hostname
			}
		}
	}

	// Check spec external IPs
	if externalIP == "<none>" {
		externalIPs, _, _ := unstructured.NestedStringSlice(u.Object, "spec", "externalIPs")
		if len(externalIPs) > 0 {
			externalIP = strings.Join(externalIPs, ",")
		}
	}

	// Format ports
	portsSlice, _, _ := unstructured.NestedSlice(u.Object, "spec", "ports")
	ports := []string{}
	for _, p := range portsSlice {
		portMap, ok := p.(map[string]interface{})
		if !ok {
			continue
		}
		port, _, _ := unstructured.NestedInt64(portMap, "port")
		nodePort, found, _ := unstructured.NestedInt64(portMap, "nodePort")
		protocol, _, _ := unstructured.NestedString(portMap, "protocol")

		portStr := fmt.Sprintf("%d", port)
		if found && nodePort != 0 {
			portStr = fmt.Sprintf("%d:%d", port, nodePort)
		}
		portStr = fmt.Sprintf("%s/%s", portStr, protocol)
		ports = append(ports, portStr)
	}

	portsStr := strings.Join(ports, ",")
	if portsStr == "" {
		portsStr = "<none>"
	}

	return Service{
		Namespace:  namespace,
		Name:       name,
		Type:       svcType,
		ClusterIP:  clusterIP,
		ExternalIP: externalIP,
		Ports:      portsStr,
		Age:        age,
		CreatedAt:  u.GetCreationTimestamp().Time,
	}, nil
}

// transformConfigMap converts an unstructured configmap to a typed ConfigMap
func transformConfigMap(u *unstructured.Unstructured) (interface{}, error) {
	namespace := u.GetNamespace()
	name := u.GetName()
	age := time.Since(u.GetCreationTimestamp().Time)

	// Count data items
	data, _, _ := unstructured.NestedMap(u.Object, "data")
	dataCount := len(data)

	return ConfigMap{
		Namespace: namespace,
		Name:      name,
		Data:      dataCount,
		Age:       age,
		CreatedAt: u.GetCreationTimestamp().Time,
	}, nil
}

// transformSecret converts an unstructured secret to a typed Secret
func transformSecret(u *unstructured.Unstructured) (interface{}, error) {
	namespace := u.GetNamespace()
	name := u.GetName()
	age := time.Since(u.GetCreationTimestamp().Time)

	// Extract type
	secretType, _, _ := unstructured.NestedString(u.Object, "type")

	// Count data items
	data, _, _ := unstructured.NestedMap(u.Object, "data")
	dataCount := len(data)

	return Secret{
		Namespace: namespace,
		Name:      name,
		Type:      secretType,
		Data:      dataCount,
		Age:       age,
		CreatedAt: u.GetCreationTimestamp().Time,
	}, nil
}

// transformNamespace converts an unstructured namespace to a typed Namespace
func transformNamespace(u *unstructured.Unstructured) (interface{}, error) {
	name := u.GetName()
	age := time.Since(u.GetCreationTimestamp().Time)

	// Extract status
	status, _, _ := unstructured.NestedString(u.Object, "status", "phase")

	return Namespace{
		Name:      name,
		Status:    status,
		Age:       age,
		CreatedAt: u.GetCreationTimestamp().Time,
	}, nil
}

// transformStatefulSet converts an unstructured statefulset to a typed StatefulSet
func transformStatefulSet(u *unstructured.Unstructured) (interface{}, error) {
	namespace := u.GetNamespace()
	name := u.GetName()
	age := time.Since(u.GetCreationTimestamp().Time)

	// Extract replica counts
	ready, _, _ := unstructured.NestedInt64(u.Object, "status", "readyReplicas")
	desired, _, _ := unstructured.NestedInt64(u.Object, "spec", "replicas")

	readyStatus := fmt.Sprintf("%d/%d", ready, desired)

	return StatefulSet{
		Namespace: namespace,
		Name:      name,
		Ready:     readyStatus,
		Age:       age,
		CreatedAt: u.GetCreationTimestamp().Time,
	}, nil
}

// transformDaemonSet converts an unstructured daemonset to a typed DaemonSet
func transformDaemonSet(u *unstructured.Unstructured) (interface{}, error) {
	namespace := u.GetNamespace()
	name := u.GetName()
	age := time.Since(u.GetCreationTimestamp().Time)

	// Extract counts
	desired, _, _ := unstructured.NestedInt64(u.Object, "status", "desiredNumberScheduled")
	current, _, _ := unstructured.NestedInt64(u.Object, "status", "currentNumberScheduled")
	ready, _, _ := unstructured.NestedInt64(u.Object, "status", "numberReady")
	upToDate, _, _ := unstructured.NestedInt64(u.Object, "status", "updatedNumberScheduled")
	available, _, _ := unstructured.NestedInt64(u.Object, "status", "numberAvailable")

	return DaemonSet{
		Namespace: namespace,
		Name:      name,
		Desired:   int32(desired),
		Current:   int32(current),
		Ready:     int32(ready),
		UpToDate:  int32(upToDate),
		Available: int32(available),
		Age:       age,
		CreatedAt: u.GetCreationTimestamp().Time,
	}, nil
}

// transformJob converts an unstructured job to a typed Job
func transformJob(u *unstructured.Unstructured) (interface{}, error) {
	namespace := u.GetNamespace()
	name := u.GetName()
	age := time.Since(u.GetCreationTimestamp().Time)

	// Extract completions
	completions, _, _ := unstructured.NestedInt64(u.Object, "spec", "completions")
	succeeded, _, _ := unstructured.NestedInt64(u.Object, "status", "succeeded")
	completionsStr := fmt.Sprintf("%d/%d", succeeded, completions)

	// Calculate duration
	var duration time.Duration
	if startTime, found, _ := unstructured.NestedString(u.Object, "status", "startTime"); found && startTime != "" {
		if completionTime, found, _ := unstructured.NestedString(u.Object, "status", "completionTime"); found && completionTime != "" {
			// Parse times and calculate duration
			duration = 0 // Simplified for now
		}
	}

	return Job{
		Namespace:   namespace,
		Name:        name,
		Completions: completionsStr,
		Duration:    duration,
		Age:         age,
		CreatedAt:   u.GetCreationTimestamp().Time,
	}, nil
}

// transformCronJob converts an unstructured cronjob to a typed CronJob
func transformCronJob(u *unstructured.Unstructured) (interface{}, error) {
	namespace := u.GetNamespace()
	name := u.GetName()
	age := time.Since(u.GetCreationTimestamp().Time)

	// Extract schedule
	schedule, _, _ := unstructured.NestedString(u.Object, "spec", "schedule")

	// Extract suspend flag
	suspend, _, _ := unstructured.NestedBool(u.Object, "spec", "suspend")

	// Count active jobs
	activeJobs, _, _ := unstructured.NestedSlice(u.Object, "status", "active")
	active := int32(len(activeJobs))

	// Get last schedule time
	var lastSchedule time.Duration
	if lastScheduleTime, found, _ := unstructured.NestedString(u.Object, "status", "lastScheduleTime"); found && lastScheduleTime != "" {
		// Parse time and calculate duration - simplified for now
		lastSchedule = 0
	}

	return CronJob{
		Namespace:    namespace,
		Name:         name,
		Schedule:     schedule,
		Suspend:      suspend,
		Active:       active,
		LastSchedule: lastSchedule,
		Age:          age,
		CreatedAt:    u.GetCreationTimestamp().Time,
	}, nil
}

// transformNode converts an unstructured node to a typed Node
func transformNode(u *unstructured.Unstructured) (interface{}, error) {
	name := u.GetName()
	age := time.Since(u.GetCreationTimestamp().Time)

	// Extract status
	conditions, _, _ := unstructured.NestedSlice(u.Object, "status", "conditions")
	status := "Unknown"
	for _, c := range conditions {
		condMap, ok := c.(map[string]interface{})
		if !ok {
			continue
		}
		if condType, _, _ := unstructured.NestedString(condMap, "type"); condType == "Ready" {
			if condStatus, _, _ := unstructured.NestedString(condMap, "status"); condStatus == "True" {
				status = "Ready"
			} else {
				status = "NotReady"
			}
			break
		}
	}

	// Extract roles from labels
	labels := u.GetLabels()
	roles := []string{}
	for key := range labels {
		if strings.HasPrefix(key, "node-role.kubernetes.io/") {
			role := strings.TrimPrefix(key, "node-role.kubernetes.io/")
			if role != "" {
				roles = append(roles, role)
			}
		}
	}
	rolesStr := strings.Join(roles, ",")
	if rolesStr == "" {
		rolesStr = "<none>"
	}

	// Extract label-based metadata
	hostname := labels["kubernetes.io/hostname"]
	if hostname == "" {
		hostname = "<none>"
	}

	instanceType := labels["beta.kubernetes.io/instance-type"]
	if instanceType == "" {
		instanceType = labels["node.kubernetes.io/instance-type"] // Try newer label
	}
	if instanceType == "" {
		instanceType = "<none>"
	}

	zone := labels["topology.kubernetes.io/zone"]
	if zone == "" {
		zone = labels["failure-domain.beta.kubernetes.io/zone"] // Try older label
	}
	if zone == "" {
		zone = "<none>"
	}

	nodePool := labels["karpenter.sh/nodepool"]
	if nodePool == "" {
		nodePool = "<none>"
	}

	// Extract version and OS image from nodeInfo
	version, _, _ := unstructured.NestedString(u.Object, "status", "nodeInfo", "kubeletVersion")
	osImage, _, _ := unstructured.NestedString(u.Object, "status", "nodeInfo", "osImage")
	if osImage == "" {
		osImage = "<none>"
	}

	return Node{
		Name:         name,
		Status:       status,
		Roles:        rolesStr,
		Age:          age,
		CreatedAt:    u.GetCreationTimestamp().Time,
		Version:      version,
		Hostname:     hostname,
		InstanceType: instanceType,
		Zone:         zone,
		NodePool:     nodePool,
		OSImage:      osImage,
	}, nil
}

// getResourceRegistry returns the registry of all supported resources
func getResourceRegistry() map[ResourceType]ResourceConfig {
	return map[ResourceType]ResourceConfig{
		ResourceTypePod: {
			GVR: schema.GroupVersionResource{
				Group:    "",
				Version:  "v1",
				Resource: "pods",
			},
			Name:       "Pods",
			Namespaced: true,
			Tier:       1, // Critical - block UI startup
			Transform:  transformPod,
		},
		ResourceTypeDeployment: {
			GVR: schema.GroupVersionResource{
				Group:    "apps",
				Version:  "v1",
				Resource: "deployments",
			},
			Name:       "Deployments",
			Namespaced: true,
			Tier:       2, // Background load
			Transform:  transformDeployment,
		},
		ResourceTypeService: {
			GVR: schema.GroupVersionResource{
				Group:    "",
				Version:  "v1",
				Resource: "services",
			},
			Name:       "Services",
			Namespaced: true,
			Tier:       2, // Background load
			Transform:  transformService,
		},
		ResourceTypeConfigMap: {
			GVR: schema.GroupVersionResource{
				Group:    "",
				Version:  "v1",
				Resource: "configmaps",
			},
			Name:       "ConfigMaps",
			Namespaced: true,
			Tier:       2,
			Transform:  transformConfigMap,
		},
		ResourceTypeSecret: {
			GVR: schema.GroupVersionResource{
				Group:    "",
				Version:  "v1",
				Resource: "secrets",
			},
			Name:       "Secrets",
			Namespaced: true,
			Tier:       2,
			Transform:  transformSecret,
		},
		ResourceTypeNamespace: {
			GVR: schema.GroupVersionResource{
				Group:    "",
				Version:  "v1",
				Resource: "namespaces",
			},
			Name:       "Namespaces",
			Namespaced: false, // Cluster-scoped
			Tier:       2,
			Transform:  transformNamespace,
		},
		ResourceTypeStatefulSet: {
			GVR: schema.GroupVersionResource{
				Group:    "apps",
				Version:  "v1",
				Resource: "statefulsets",
			},
			Name:       "StatefulSets",
			Namespaced: true,
			Tier:       3,
			Transform:  transformStatefulSet,
		},
		ResourceTypeDaemonSet: {
			GVR: schema.GroupVersionResource{
				Group:    "apps",
				Version:  "v1",
				Resource: "daemonsets",
			},
			Name:       "DaemonSets",
			Namespaced: true,
			Tier:       3,
			Transform:  transformDaemonSet,
		},
		ResourceTypeJob: {
			GVR: schema.GroupVersionResource{
				Group:    "batch",
				Version:  "v1",
				Resource: "jobs",
			},
			Name:       "Jobs",
			Namespaced: true,
			Tier:       3,
			Transform:  transformJob,
		},
		ResourceTypeCronJob: {
			GVR: schema.GroupVersionResource{
				Group:    "batch",
				Version:  "v1",
				Resource: "cronjobs",
			},
			Name:       "CronJobs",
			Namespaced: true,
			Tier:       3,
			Transform:  transformCronJob,
		},
		ResourceTypeNode: {
			GVR: schema.GroupVersionResource{
				Group:    "",
				Version:  "v1",
				Resource: "nodes",
			},
			Name:       "Nodes",
			Namespaced: false, // Cluster-scoped
			Tier:       3,
			Transform:  transformNode,
		},
	}
}
