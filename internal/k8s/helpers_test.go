package k8s

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestExtractAge(t *testing.T) {
	tests := []struct {
		name     string
		item     interface{}
		expected time.Duration
	}{
		{
			name:     "Pod with age",
			item:     Pod{Age: 5 * time.Minute},
			expected: 5 * time.Minute,
		},
		{
			name:     "Deployment with age",
			item:     Deployment{Age: 10 * time.Hour},
			expected: 10 * time.Hour,
		},
		{
			name:     "Service with age",
			item:     Service{Age: 2 * time.Hour},
			expected: 2 * time.Hour,
		},
		{
			name:     "ConfigMap with age",
			item:     ConfigMap{Age: 1 * time.Hour},
			expected: 1 * time.Hour,
		},
		{
			name:     "Secret with age",
			item:     Secret{Age: 3 * time.Hour},
			expected: 3 * time.Hour,
		},
		{
			name:     "Namespace with age",
			item:     Namespace{Age: 24 * time.Hour},
			expected: 24 * time.Hour,
		},
		{
			name:     "StatefulSet with age",
			item:     StatefulSet{Age: 6 * time.Hour},
			expected: 6 * time.Hour,
		},
		{
			name:     "DaemonSet with age",
			item:     DaemonSet{Age: 12 * time.Hour},
			expected: 12 * time.Hour,
		},
		{
			name:     "Job with age",
			item:     Job{Age: 30 * time.Minute},
			expected: 30 * time.Minute,
		},
		{
			name:     "CronJob with age",
			item:     CronJob{Age: 48 * time.Hour},
			expected: 48 * time.Hour,
		},
		{
			name:     "Node with age",
			item:     Node{Age: 168 * time.Hour},
			expected: 168 * time.Hour,
		},
		{
			name:     "Unknown type",
			item:     "unknown",
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractAge(tt.item)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractName(t *testing.T) {
	tests := []struct {
		name     string
		item     interface{}
		expected string
	}{
		{
			name:     "Pod with name",
			item:     Pod{Name: "pod-1"},
			expected: "pod-1",
		},
		{
			name:     "Deployment with name",
			item:     Deployment{Name: "deploy-1"},
			expected: "deploy-1",
		},
		{
			name:     "Service with name",
			item:     Service{Name: "svc-1"},
			expected: "svc-1",
		},
		{
			name:     "ConfigMap with name",
			item:     ConfigMap{Name: "cm-1"},
			expected: "cm-1",
		},
		{
			name:     "Secret with name",
			item:     Secret{Name: "secret-1"},
			expected: "secret-1",
		},
		{
			name:     "Namespace with name",
			item:     Namespace{Name: "ns-1"},
			expected: "ns-1",
		},
		{
			name:     "StatefulSet with name",
			item:     StatefulSet{Name: "sts-1"},
			expected: "sts-1",
		},
		{
			name:     "DaemonSet with name",
			item:     DaemonSet{Name: "ds-1"},
			expected: "ds-1",
		},
		{
			name:     "Job with name",
			item:     Job{Name: "job-1"},
			expected: "job-1",
		},
		{
			name:     "CronJob with name",
			item:     CronJob{Name: "cron-1"},
			expected: "cron-1",
		},
		{
			name:     "Node with name",
			item:     Node{Name: "node-1"},
			expected: "node-1",
		},
		{
			name:     "Unknown type",
			item:     "unknown",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractName(tt.item)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSortByAge(t *testing.T) {
	now := time.Now()
	items := []interface{}{
		Pod{Name: "old-pod", CreatedAt: now.Add(-10 * time.Hour)},
		Pod{Name: "new-pod", CreatedAt: now.Add(-1 * time.Hour)},
		Pod{Name: "medium-pod", CreatedAt: now.Add(-5 * time.Hour)},
		Pod{Name: "ancient-pod", CreatedAt: now.Add(-24 * time.Hour)},
	}

	sortByAge(items)

	// Should be sorted by age (newest first)
	assert.Equal(t, "new-pod", extractName(items[0]))
	assert.Equal(t, "medium-pod", extractName(items[1]))
	assert.Equal(t, "old-pod", extractName(items[2]))
	assert.Equal(t, "ancient-pod", extractName(items[3]))
}

func TestSortByAge_SameAge(t *testing.T) {
	now := time.Now()
	sameTime := now.Add(-5 * time.Hour)
	items := []interface{}{
		Pod{Name: "pod-c", CreatedAt: sameTime},
		Pod{Name: "pod-a", CreatedAt: sameTime},
		Pod{Name: "pod-b", CreatedAt: sameTime},
	}

	sortByAge(items)

	// With same age, should be sorted by name alphabetically
	assert.Equal(t, "pod-a", extractName(items[0]))
	assert.Equal(t, "pod-b", extractName(items[1]))
	assert.Equal(t, "pod-c", extractName(items[2]))
}

func TestSortByAge_MixedTypes(t *testing.T) {
	now := time.Now()
	items := []interface{}{
		Deployment{Name: "deploy-1", CreatedAt: now.Add(-10 * time.Hour)},
		Service{Name: "svc-1", CreatedAt: now.Add(-2 * time.Hour)},
		Pod{Name: "pod-1", CreatedAt: now.Add(-5 * time.Hour)},
	}

	sortByAge(items)

	// Should be sorted by age (newest first) regardless of type
	assert.Equal(t, "svc-1", extractName(items[0]))
	assert.Equal(t, "pod-1", extractName(items[1]))
	assert.Equal(t, "deploy-1", extractName(items[2]))
}
