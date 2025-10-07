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

// Formatter provides resource output formatting
type Formatter struct {
	manager *InformerManager
}

// NewResourceFormatter creates a new resource formatter
func NewResourceFormatter(manager *InformerManager) *Formatter {
	return &Formatter{
		manager: manager,
	}
}

// GetResourceYAML returns YAML representation of a resource using kubectl YAMLPrinter
func (f *Formatter) GetResourceYAML(gvr schema.GroupVersionResource, namespace, name string) (string, error) {
	// Get resource from dynamic informer cache
	lister, ok := f.manager.GetDynamicLister(gvr)
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
func (f *Formatter) DescribeResource(gvr schema.GroupVersionResource, namespace, name string) (string, error) {
	// Get resource from dynamic informer cache
	lister, ok := f.manager.GetDynamicLister(gvr)
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
	events, err := f.fetchEventsForResource(namespace, name, string(obj.GetUID()))
	if err != nil {
		buf.WriteString(fmt.Sprintf("  Failed to fetch events: %v\n", err))
	} else if len(events) == 0 {
		buf.WriteString("  <none>\n")
	} else {
		buf.WriteString(f.formatEvents(events))
	}

	return buf.String(), nil
}

// fetchEventsForResource fetches events related to a specific resource on-demand
func (f *Formatter) fetchEventsForResource(namespace, name, uid string) ([]corev1.Event, error) {
	// Use field selector to filter events for this specific resource
	fieldSelector := fmt.Sprintf("involvedObject.name=%s,involvedObject.namespace=%s", name, namespace)
	if uid != "" {
		fieldSelector += fmt.Sprintf(",involvedObject.uid=%s", uid)
	}

	eventList, err := f.manager.GetClientset().CoreV1().Events(namespace).List(
		f.manager.GetCtx(),
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
func (f *Formatter) formatEvents(events []corev1.Event) string {
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
