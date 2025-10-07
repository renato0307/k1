package k8s

import (
	"fmt"
	"sort"
	"time"
)

// sortByAge sorts resources by CreatedAt field if present (newest first)
func sortByAge(items []any) {
	sort.Slice(items, func(i, j int) bool {
		// Try to extract CreatedAt from both items
		createdI := extractCreatedAt(items[i])
		createdJ := extractCreatedAt(items[j])

		if !createdI.Equal(createdJ) {
			return createdI.After(createdJ) // Newer first
		}

		// Fall back to name comparison
		nameI := extractName(items[i])
		nameJ := extractName(items[j])
		return nameI < nameJ
	})
}

// sortByCreationTime is a generic helper for sorting typed slices by CreatedAt (newest first)
type resourceWithTimestamp interface {
	Pod | Deployment | Service | ConfigMap | Secret | Namespace | StatefulSet | DaemonSet | Job | CronJob | Node
}

func sortByCreationTime[T resourceWithTimestamp](items []T, getCreatedAt func(T) time.Time, getName func(T) string) {
	sort.Slice(items, func(i, j int) bool {
		createdI := getCreatedAt(items[i])
		createdJ := getCreatedAt(items[j])

		if !createdI.Equal(createdJ) {
			return createdI.After(createdJ) // Newer first
		}

		// Fall back to name comparison for stable sort
		return getName(items[i]) < getName(items[j])
	})
}

// extractCreatedAt tries to extract CreatedAt field from any
func extractCreatedAt(item any) time.Time {
	switch v := item.(type) {
	case Pod:
		return v.CreatedAt
	case Deployment:
		return v.CreatedAt
	case Service:
		return v.CreatedAt
	case ConfigMap:
		return v.CreatedAt
	case Secret:
		return v.CreatedAt
	case Namespace:
		return v.CreatedAt
	case StatefulSet:
		return v.CreatedAt
	case DaemonSet:
		return v.CreatedAt
	case Job:
		return v.CreatedAt
	case CronJob:
		return v.CreatedAt
	case Node:
		return v.CreatedAt
	default:
		return time.Time{} // Zero time
	}
}

// extractAge tries to extract Age field from any
func extractAge(item any) time.Duration {
	switch v := item.(type) {
	case Pod:
		return v.Age
	case Deployment:
		return v.Age
	case Service:
		return v.Age
	case ConfigMap:
		return v.Age
	case Secret:
		return v.Age
	case Namespace:
		return v.Age
	case StatefulSet:
		return v.Age
	case DaemonSet:
		return v.Age
	case Job:
		return v.Age
	case CronJob:
		return v.Age
	case Node:
		return v.Age
	default:
		return 0
	}
}

// extractName tries to extract Name field from any
func extractName(item any) string {
	switch v := item.(type) {
	case Pod:
		return v.Name
	case Deployment:
		return v.Name
	case Service:
		return v.Name
	case ConfigMap:
		return v.Name
	case Secret:
		return v.Name
	case Namespace:
		return v.Name
	case StatefulSet:
		return v.Name
	case DaemonSet:
		return v.Name
	case Job:
		return v.Name
	case CronJob:
		return v.Name
	case Node:
		return v.Name
	default:
		return ""
	}
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
