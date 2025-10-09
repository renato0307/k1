package k8s

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/printers"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/yaml"
)

// GetResourceYAML returns YAML representation of a resource using kubectl YAMLPrinter
func (r *InformerRepository) GetResourceYAML(gvr schema.GroupVersionResource, namespace, name string) (string, error) {
	// Get resource from dynamic informer cache
	lister, ok := r.dynamicListers[gvr]
	if !ok {
		return "", fmt.Errorf("informer not initialized for resource %s", gvr)
	}

	var runtimeObj any
	var err error

	// Handle namespaced vs cluster-scoped resources
	if namespace != "" {
		runtimeObj, err = lister.ByNamespace(namespace).Get(name)
	} else {
		runtimeObj, err = lister.Get(name)
	}

	if err != nil {
		return "", fmt.Errorf("resource not found: %w", err)
	}

	// Type assert to unstructured
	obj, ok := runtimeObj.(*unstructured.Unstructured)
	if !ok {
		return "", fmt.Errorf("unexpected object type: %T", runtimeObj)
	}

	// Use kubectl YAML printer for exact kubectl output match
	printer := printers.NewTypeSetter(scheme.Scheme).ToPrinter(&printers.YAMLPrinter{})

	var buf bytes.Buffer
	if err := printer.PrintObj(obj, &buf); err != nil {
		return "", fmt.Errorf("failed to print YAML: %w", err)
	}

	return buf.String(), nil
}

// DescribeResource returns kubectl describe output for a resource
func (r *InformerRepository) DescribeResource(gvr schema.GroupVersionResource, namespace, name string) (string, error) {
	// For now, use a simplified describe implementation
	// TODO: Implement full kubectl describe formatters with Events support

	// Get resource from dynamic informer cache
	lister, ok := r.dynamicListers[gvr]
	if !ok {
		return "", fmt.Errorf("informer not initialized for resource %s", gvr)
	}

	var runtimeObj any
	var err error

	// Handle namespaced vs cluster-scoped resources
	if namespace != "" {
		runtimeObj, err = lister.ByNamespace(namespace).Get(name)
	} else {
		runtimeObj, err = lister.Get(name)
	}

	if err != nil {
		return "", fmt.Errorf("resource not found: %w", err)
	}

	// Type assert to unstructured
	obj, ok := runtimeObj.(*unstructured.Unstructured)
	if !ok {
		return "", fmt.Errorf("unexpected object type: %T", runtimeObj)
	}

	// Create a basic describe output using the resource's fields
	// This is a simplified version - full kubectl describe would require:
	// 1. Events informer
	// 2. Resource-specific describers (PodDescriber, DeploymentDescriber, etc.)

	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("Name:         %s\n", name))
	if namespace != "" {
		buf.WriteString(fmt.Sprintf("Namespace:    %s\n", namespace))
	}
	buf.WriteString(fmt.Sprintf("Kind:         %s\n", obj.GetKind()))
	buf.WriteString(fmt.Sprintf("API Version:  %s\n", obj.GetAPIVersion()))

	// Add labels if present
	labels := obj.GetLabels()
	if len(labels) > 0 {
		buf.WriteString("Labels:       ")
		first := true
		for k, v := range labels {
			if !first {
				buf.WriteString("              ")
			}
			buf.WriteString(fmt.Sprintf("%s=%s\n", k, v))
			first = false
		}
	}

	// Add creation timestamp
	buf.WriteString(fmt.Sprintf("Created:      %s\n", obj.GetCreationTimestamp().String()))

	// Add status if present, formatted as YAML
	status, found, err := unstructured.NestedFieldCopy(obj.Object, "status")
	if found && err == nil {
		statusYAML, err := yaml.Marshal(status)
		if err == nil {
			buf.WriteString("\nStatus:\n")
			// Indent status YAML by 2 spaces
			for _, line := range strings.Split(string(statusYAML), "\n") {
				if line != "" {
					buf.WriteString("  " + line + "\n")
				}
			}
		}
	}

	// Fetch events on-demand (not cached) to avoid memory overhead
	buf.WriteString("\nEvents:\n")
	events, err := r.fetchEventsForResource(namespace, name, string(obj.GetUID()))
	if err != nil {
		buf.WriteString(fmt.Sprintf("  Failed to fetch events: %v\n", err))
	} else if len(events) == 0 {
		buf.WriteString("  <none>\n")
	} else {
		buf.WriteString(r.formatEvents(events))
	}

	return buf.String(), nil
}

// fetchEventsForResource fetches events related to a specific resource on-demand
func (r *InformerRepository) fetchEventsForResource(namespace, name, uid string) ([]corev1.Event, error) {
	// Use field selector to filter events for this specific resource
	fieldSelector := fmt.Sprintf("involvedObject.name=%s,involvedObject.namespace=%s", name, namespace)
	if uid != "" {
		fieldSelector += fmt.Sprintf(",involvedObject.uid=%s", uid)
	}

	eventList, err := r.clientset.CoreV1().Events(namespace).List(
		r.ctx,
		metav1.ListOptions{
			FieldSelector: fieldSelector,
			Limit:         100, // Limit to most recent 100 events
		},
	)
	if err != nil {
		return nil, err
	}

	return eventList.Items, nil
}

// formatEvents formats events in kubectl describe style
func (r *InformerRepository) formatEvents(events []corev1.Event) string {
	if len(events) == 0 {
		return "  <none>\n"
	}

	// Sort events by timestamp (newest first)
	sort.Slice(events, func(i, j int) bool {
		return events[i].LastTimestamp.After(events[j].LastTimestamp.Time)
	})

	var buf bytes.Buffer
	buf.WriteString("  Type    Reason    Age                    Message\n")
	buf.WriteString("  ----    ------    ---                    -------\n")

	now := time.Now()
	for _, event := range events {
		eventType := event.Type
		reason := event.Reason
		message := event.Message

		// Calculate age
		var age string
		if !event.LastTimestamp.IsZero() {
			duration := now.Sub(event.LastTimestamp.Time)
			age = formatEventAge(duration)
		} else if !event.EventTime.IsZero() {
			duration := now.Sub(event.EventTime.Time)
			age = formatEventAge(duration)
		} else {
			age = "<unknown>"
		}

		// Truncate message if too long
		if len(message) > 80 {
			message = message[:77] + "..."
		}

		buf.WriteString(fmt.Sprintf("  %-7s %-9s %-22s %s\n", eventType, reason, age, message))
	}

	return buf.String()
}

// formatEventAge formats event age in kubectl style (e.g., "5m", "2h", "3d")
func formatEventAge(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	} else if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	} else if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	} else {
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}
