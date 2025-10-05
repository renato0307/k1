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
	}
}
