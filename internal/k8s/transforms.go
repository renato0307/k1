package k8s

import (
	"fmt"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// Transform functions convert unstructured Kubernetes resources to typed structs.
//
// Design Decision: Why not use reflection?
//
// We use explicit transform functions instead of reflection-based generic
// transformers for several reasons:
//
// 1. Performance: Reflection is 10-100x slower than direct field access.
//    This matters for large clusters (1000+ resources) where transforms run
//    on every list operation.
//
// 2. Complexity trade-off: Reflection adds implicit behavior that's harder to
//    debug. Explicit transforms are immediately understandable.
//
// 3. Already optimized: The extractMetadata helper eliminates most
//    duplication (namespace, name, age, createdAt) without reflection overhead.
//    It extracts once per resource and passes ResourceMetadata to transforms.
//
// 4. Type safety: Explicit transforms fail fast at compile time, while
//    reflection-based approaches defer errors to runtime.
//
// The current approach balances maintainability (DRY via extractMetadata)
// with performance (no reflection overhead) and debuggability (explicit code).

// extractMetadata extracts common fields from unstructured resource
func extractMetadata(u *unstructured.Unstructured) ResourceMetadata {
	createdAt := u.GetCreationTimestamp().Time
	return ResourceMetadata{
		Namespace: u.GetNamespace(),
		Name:      u.GetName(),
		Age:       time.Since(createdAt),
		CreatedAt: createdAt,
	}
}

// transformPod converts an unstructured pod to a typed Pod
func transformPod(u *unstructured.Unstructured, common ResourceMetadata) (any, error) {

	// Extract container status for ready count
	containerStatuses, _, _ := unstructured.NestedSlice(u.Object, "status", "containerStatuses")
	readyContainers := 0
	totalContainers := len(containerStatuses)
	totalRestarts := int32(0)

	for _, cs := range containerStatuses {
		csMap, ok := cs.(map[string]any)
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
		ResourceMetadata: ResourceMetadata{
			Namespace: common.Namespace,
			Name:      common.Name,
			Age:       common.Age,
			CreatedAt: common.CreatedAt,
		},
		Ready:    readyStatus,
		Status:   status,
		Restarts: totalRestarts,
		Node:     node,
		IP:       ip,
	}, nil
}

// transformDeployment converts an unstructured deployment to a typed Deployment
func transformDeployment(u *unstructured.Unstructured, common ResourceMetadata) (any, error) {

	// Extract replica counts
	ready, _, _ := unstructured.NestedInt64(u.Object, "status", "readyReplicas")
	desired, _, _ := unstructured.NestedInt64(u.Object, "spec", "replicas")
	upToDate, _, _ := unstructured.NestedInt64(u.Object, "status", "updatedReplicas")
	available, _, _ := unstructured.NestedInt64(u.Object, "status", "availableReplicas")

	readyStatus := fmt.Sprintf("%d/%d", ready, desired)

	return Deployment{
		ResourceMetadata: ResourceMetadata{
			Namespace: common.Namespace,
			Name:      common.Name,
			Age:       common.Age,
			CreatedAt: common.CreatedAt,
		},
		Ready:     readyStatus,
		UpToDate:  int32(upToDate),
		Available: int32(available),
	}, nil
}

// transformService converts an unstructured service to a typed Service
func transformService(u *unstructured.Unstructured, common ResourceMetadata) (any, error) {

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
		ingressMap, ok := lbIngress[0].(map[string]any)
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
		portMap, ok := p.(map[string]any)
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
		ResourceMetadata: ResourceMetadata{
			Namespace: common.Namespace,
			Name:      common.Name,
			Age:       common.Age,
			CreatedAt: common.CreatedAt,
		},
		Type:       svcType,
		ClusterIP:  clusterIP,
		ExternalIP: externalIP,
		Ports:      portsStr,
	}, nil
}

// transformConfigMap converts an unstructured configmap to a typed ConfigMap
func transformConfigMap(u *unstructured.Unstructured, common ResourceMetadata) (any, error) {

	// Count data items
	data, _, _ := unstructured.NestedMap(u.Object, "data")
	dataCount := len(data)

	return ConfigMap{
		ResourceMetadata: ResourceMetadata{
			Namespace: common.Namespace,
			Name:      common.Name,
			Age:       common.Age,
			CreatedAt: common.CreatedAt,
		},
		Data: dataCount,
	}, nil
}

// transformSecret converts an unstructured secret to a typed Secret
func transformSecret(u *unstructured.Unstructured, common ResourceMetadata) (any, error) {

	// Extract type
	secretType, _, _ := unstructured.NestedString(u.Object, "type")

	// Count data items
	data, _, _ := unstructured.NestedMap(u.Object, "data")
	dataCount := len(data)

	return Secret{
		ResourceMetadata: ResourceMetadata{
			Namespace: common.Namespace,
			Name:      common.Name,
			Age:       common.Age,
			CreatedAt: common.CreatedAt,
		},
		Type: secretType,
		Data: dataCount,
	}, nil
}

// transformNamespace converts an unstructured namespace to a typed Namespace
func transformNamespace(u *unstructured.Unstructured, common ResourceMetadata) (any, error) {

	// Extract status
	status, _, _ := unstructured.NestedString(u.Object, "status", "phase")

	return Namespace{
		ResourceMetadata: ResourceMetadata{
			Namespace: common.Namespace,
			Name:      common.Name,
			Age:       common.Age,
			CreatedAt: common.CreatedAt,
		},
		Status: status,
	}, nil
}

// transformStatefulSet converts an unstructured statefulset to a typed StatefulSet
func transformStatefulSet(u *unstructured.Unstructured, common ResourceMetadata) (any, error) {

	// Extract replica counts
	ready, _, _ := unstructured.NestedInt64(u.Object, "status", "readyReplicas")
	desired, _, _ := unstructured.NestedInt64(u.Object, "spec", "replicas")

	readyStatus := fmt.Sprintf("%d/%d", ready, desired)

	return StatefulSet{
		ResourceMetadata: ResourceMetadata{
			Namespace: common.Namespace,
			Name:      common.Name,
			Age:       common.Age,
			CreatedAt: common.CreatedAt,
		},
		Ready: readyStatus,
	}, nil
}

// transformDaemonSet converts an unstructured daemonset to a typed DaemonSet
func transformDaemonSet(u *unstructured.Unstructured, common ResourceMetadata) (any, error) {

	// Extract counts
	desired, _, _ := unstructured.NestedInt64(u.Object, "status", "desiredNumberScheduled")
	current, _, _ := unstructured.NestedInt64(u.Object, "status", "currentNumberScheduled")
	ready, _, _ := unstructured.NestedInt64(u.Object, "status", "numberReady")
	upToDate, _, _ := unstructured.NestedInt64(u.Object, "status", "updatedNumberScheduled")
	available, _, _ := unstructured.NestedInt64(u.Object, "status", "numberAvailable")

	return DaemonSet{
		ResourceMetadata: ResourceMetadata{
			Namespace: common.Namespace,
			Name:      common.Name,
			Age:       common.Age,
			CreatedAt: common.CreatedAt,
		},
		Desired:   int32(desired),
		Current:   int32(current),
		Ready:     int32(ready),
		UpToDate:  int32(upToDate),
		Available: int32(available),
	}, nil
}

// transformJob converts an unstructured job to a typed Job
func transformJob(u *unstructured.Unstructured, common ResourceMetadata) (any, error) {

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
		ResourceMetadata: ResourceMetadata{
			Namespace: common.Namespace,
			Name:      common.Name,
			Age:       common.Age,
			CreatedAt: common.CreatedAt,
		},
		Completions: completionsStr,
		Duration:    duration,
	}, nil
}

// transformCronJob converts an unstructured cronjob to a typed CronJob
func transformCronJob(u *unstructured.Unstructured, common ResourceMetadata) (any, error) {

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
		ResourceMetadata: ResourceMetadata{
			Namespace: common.Namespace,
			Name:      common.Name,
			Age:       common.Age,
			CreatedAt: common.CreatedAt,
		},
		Schedule:     schedule,
		Suspend:      suspend,
		Active:       active,
		LastSchedule: lastSchedule,
	}, nil
}

// transformNode converts an unstructured node to a typed Node
func transformNode(u *unstructured.Unstructured, common ResourceMetadata) (any, error) {

	// Extract status
	conditions, _, _ := unstructured.NestedSlice(u.Object, "status", "conditions")
	status := "Unknown"
	for _, c := range conditions {
		condMap, ok := c.(map[string]any)
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
		ResourceMetadata: ResourceMetadata{
			Namespace: common.Namespace,
			Name:      common.Name,
			Age:       common.Age,
			CreatedAt: common.CreatedAt,
		},
		Status:       status,
		Roles:        rolesStr,
		Version:      version,
		Hostname:     hostname,
		InstanceType: instanceType,
		Zone:         zone,
		NodePool:     nodePool,
		OSImage:      osImage,
	}, nil
}

// transformReplicaSet converts an unstructured replicaset to a typed ReplicaSet
func transformReplicaSet(u *unstructured.Unstructured, common ResourceMetadata) (any, error) {
	desired, _, _ := unstructured.NestedInt64(u.Object, "spec", "replicas")
	current, _, _ := unstructured.NestedInt64(u.Object, "status", "replicas")
	ready, _, _ := unstructured.NestedInt64(u.Object, "status", "readyReplicas")

	return ReplicaSet{
		ResourceMetadata: ResourceMetadata{
			Namespace: common.Namespace,
			Name:      common.Name,
			Age:       common.Age,
			CreatedAt: common.CreatedAt,
		},
		Desired: int32(desired),
		Current: int32(current),
		Ready:   int32(ready),
	}, nil
}

// transformPVC converts an unstructured PVC to a typed PersistentVolumeClaim
func transformPVC(u *unstructured.Unstructured, common ResourceMetadata) (any, error) {
	phase, _, _ := unstructured.NestedString(u.Object, "status", "phase")
	volumeName, _, _ := unstructured.NestedString(u.Object, "spec", "volumeName")

	// Extract capacity
	capacity := "<none>"
	if phase == "Bound" {
		capacityMap, found, _ := unstructured.NestedMap(u.Object, "status", "capacity")
		if found {
			if storage, ok := capacityMap["storage"].(string); ok {
				capacity = storage
			}
		}
	}

	// Extract access modes
	accessModes, _, _ := unstructured.NestedStringSlice(u.Object, "spec", "accessModes")
	accessModesStr := strings.Join(accessModes, ",")

	storageClass, _, _ := unstructured.NestedString(u.Object, "spec", "storageClassName")

	return PersistentVolumeClaim{
		ResourceMetadata: ResourceMetadata{
			Namespace: common.Namespace,
			Name:      common.Name,
			Age:       common.Age,
			CreatedAt: common.CreatedAt,
		},
		Status:       phase,
		Volume:       volumeName,
		Capacity:     capacity,
		AccessModes:  accessModesStr,
		StorageClass: storageClass,
	}, nil
}

// transformIngress converts an unstructured ingress to a typed Ingress
func transformIngress(u *unstructured.Unstructured, common ResourceMetadata) (any, error) {
	ingressClass, _, _ := unstructured.NestedString(u.Object, "spec", "ingressClassName")

	// Extract hosts from rules
	rules, _, _ := unstructured.NestedSlice(u.Object, "spec", "rules")
	hosts := []string{}
	for _, rule := range rules {
		ruleMap, ok := rule.(map[string]any)
		if !ok {
			continue
		}
		if host, _, _ := unstructured.NestedString(ruleMap, "host"); host != "" {
			hosts = append(hosts, host)
		}
	}
	hostsStr := strings.Join(hosts, ", ")
	if hostsStr == "" {
		hostsStr = "*"
	}

	// Extract load balancer address
	address := "<pending>"
	lbIngress, _, _ := unstructured.NestedSlice(u.Object, "status", "loadBalancer", "ingress")
	if len(lbIngress) > 0 {
		if lbMap, ok := lbIngress[0].(map[string]any); ok {
			if ip, _, _ := unstructured.NestedString(lbMap, "ip"); ip != "" {
				address = ip
			} else if hostname, _, _ := unstructured.NestedString(lbMap, "hostname"); hostname != "" {
				address = hostname
			}
		}
	}

	return Ingress{
		ResourceMetadata: ResourceMetadata{
			Namespace: common.Namespace,
			Name:      common.Name,
			Age:       common.Age,
			CreatedAt: common.CreatedAt,
		},
		Class:   ingressClass,
		Hosts:   hostsStr,
		Address: address,
		Ports:   "80, 443", // Simplified - most ingresses use these
	}, nil
}

// transformEndpoints converts an unstructured endpoints to a typed Endpoints
func transformEndpoints(u *unstructured.Unstructured, common ResourceMetadata) (any, error) {
	// Parse subsets to extract endpoints (IP:port pairs)
	subsets, _, _ := unstructured.NestedSlice(u.Object, "subsets")
	endpoints := []string{}

	for _, subset := range subsets {
		subsetMap, ok := subset.(map[string]any)
		if !ok {
			continue
		}

		addresses, _, _ := unstructured.NestedSlice(subsetMap, "addresses")
		ports, _, _ := unstructured.NestedSlice(subsetMap, "ports")

		for _, addr := range addresses {
			addrMap, ok := addr.(map[string]any)
			if !ok {
				continue
			}
			ip, _, _ := unstructured.NestedString(addrMap, "ip")

			for _, port := range ports {
				portMap, ok := port.(map[string]any)
				if !ok {
					continue
				}
				portNum, _, _ := unstructured.NestedInt64(portMap, "port")
				endpoints = append(endpoints, fmt.Sprintf("%s:%d", ip, portNum))
			}
		}
	}

	endpointsStr := strings.Join(endpoints, ", ")
	if endpointsStr == "" {
		endpointsStr = "<none>"
	}

	return Endpoints{
		ResourceMetadata: ResourceMetadata{
			Namespace: common.Namespace,
			Name:      common.Name,
			Age:       common.Age,
			CreatedAt: common.CreatedAt,
		},
		Endpoints: endpointsStr,
	}, nil
}

// transformHPA converts an unstructured HPA to a typed HorizontalPodAutoscaler
func transformHPA(u *unstructured.Unstructured, common ResourceMetadata) (any, error) {
	minReplicas, _, _ := unstructured.NestedInt64(u.Object, "spec", "minReplicas")
	maxReplicas, _, _ := unstructured.NestedInt64(u.Object, "spec", "maxReplicas")
	currentReplicas, _, _ := unstructured.NestedInt64(u.Object, "status", "currentReplicas")

	// Extract scale target reference
	refKind, _, _ := unstructured.NestedString(u.Object, "spec", "scaleTargetRef", "kind")
	refName, _, _ := unstructured.NestedString(u.Object, "spec", "scaleTargetRef", "name")
	reference := fmt.Sprintf("%s/%s", refKind, refName)

	// Extract target CPU utilization (v2 API)
	targetCPU := "N/A"
	metrics, _, _ := unstructured.NestedSlice(u.Object, "spec", "metrics")
	for _, metric := range metrics {
		metricMap, ok := metric.(map[string]any)
		if !ok {
			continue
		}
		metricType, _, _ := unstructured.NestedString(metricMap, "type")
		if metricType == "Resource" {
			resource, _, _ := unstructured.NestedMap(metricMap, "resource")
			name, _, _ := unstructured.NestedString(resource, "name")
			if name == "cpu" {
				if target, _, _ := unstructured.NestedInt64(resource, "target", "averageUtilization"); target > 0 {
					targetCPU = fmt.Sprintf("%d%%", target)
				}
			}
		}
	}

	return HorizontalPodAutoscaler{
		ResourceMetadata: ResourceMetadata{
			Namespace: common.Namespace,
			Name:      common.Name,
			Age:       common.Age,
			CreatedAt: common.CreatedAt,
		},
		Reference: reference,
		MinPods:   int32(minReplicas),
		MaxPods:   int32(maxReplicas),
		Replicas:  int32(currentReplicas),
		TargetCPU: targetCPU,
	}, nil
}

// transformCRD converts an unstructured CRD to a typed CustomResourceDefinition
func transformCRD(u *unstructured.Unstructured, common ResourceMetadata) (any, error) {
	// Extract CRD spec fields
	group, _, _ := unstructured.NestedString(u.Object, "spec", "group")
	kind, _, _ := unstructured.NestedString(u.Object, "spec", "names", "kind")
	plural, _, _ := unstructured.NestedString(u.Object, "spec", "names", "plural")
	scope, _, _ := unstructured.NestedString(u.Object, "spec", "scope")

	// Find storage version
	versions, _, _ := unstructured.NestedSlice(u.Object, "spec", "versions")
	version := ""
	for _, v := range versions {
		vMap, ok := v.(map[string]any)
		if !ok {
			continue
		}
		stored, _, _ := unstructured.NestedBool(vMap, "storage")
		if stored {
			version, _, _ = unstructured.NestedString(vMap, "name")
			break
		}
	}

	return CustomResourceDefinition{
		ResourceMetadata: ResourceMetadata{
			Namespace: common.Namespace,
			Name:      common.Name,
			Age:       common.Age,
			CreatedAt: common.CreatedAt,
		},
		Group:   group,
		Version: version,
		Kind:    kind,
		Scope:   scope,
		Plural:  plural,
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
		ResourceTypeReplicaSet: {
			GVR: schema.GroupVersionResource{
				Group:    "apps",
				Version:  "v1",
				Resource: "replicasets",
			},
			Name:       "ReplicaSets",
			Namespaced: true,
			Tier:       1,
			Transform:  transformReplicaSet,
		},
		ResourceTypePersistentVolumeClaim: {
			GVR: schema.GroupVersionResource{
				Group:    "",
				Version:  "v1",
				Resource: "persistentvolumeclaims",
			},
			Name:       "PersistentVolumeClaims",
			Namespaced: true,
			Tier:       1,
			Transform:  transformPVC,
		},
		ResourceTypeIngress: {
			GVR: schema.GroupVersionResource{
				Group:    "networking.k8s.io",
				Version:  "v1",
				Resource: "ingresses",
			},
			Name:       "Ingresses",
			Namespaced: true,
			Tier:       1,
			Transform:  transformIngress,
		},
		ResourceTypeEndpoints: {
			GVR: schema.GroupVersionResource{
				Group:    "",
				Version:  "v1",
				Resource: "endpoints",
			},
			Name:       "Endpoints",
			Namespaced: true,
			Tier:       1,
			Transform:  transformEndpoints,
		},
		ResourceTypeHPA: {
			GVR: schema.GroupVersionResource{
				Group:    "autoscaling",
				Version:  "v2",
				Resource: "horizontalpodautoscalers",
			},
			Name:       "HorizontalPodAutoscalers",
			Namespaced: true,
			Tier:       1,
			Transform:  transformHPA,
		},
		ResourceTypeCRD: {
			GVR: schema.GroupVersionResource{
				Group:    "apiextensions.k8s.io",
				Version:  "v1",
				Resource: "customresourcedefinitions",
			},
			Name:       "Custom Resource Definitions",
			Namespaced: false, // CRDs are cluster-scoped
			Tier:       2,     // Background load
			Transform:  transformCRD,
		},
	}
}
