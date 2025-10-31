package k8s

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestEvaluateJSONPath(t *testing.T) {
	tests := []struct {
		name     string
		object   map[string]any
		jsonPath string
		expected string
	}{
		{
			name: "simple field access",
			object: map[string]any{
				"status": map[string]any{
					"phase": "Running",
				},
			},
			jsonPath: ".status.phase",
			expected: "Running",
		},
		{
			name: "nested field access",
			object: map[string]any{
				"status": map[string]any{
					"conditions": []any{
						map[string]any{
							"type":   "Ready",
							"status": "True",
						},
					},
				},
			},
			jsonPath: ".status.conditions[0].status",
			expected: "True",
		},
		{
			name: "boolean value",
			object: map[string]any{
				"spec": map[string]any{
					"suspend": true,
				},
			},
			jsonPath: ".spec.suspend",
			expected: "True",
		},
		{
			name: "integer value",
			object: map[string]any{
				"spec": map[string]any{
					"replicas": 3,
				},
			},
			jsonPath: ".spec.replicas",
			expected: "3",
		},
		{
			name: "missing field",
			object: map[string]any{
				"status": map[string]any{},
			},
			jsonPath: ".status.nonexistent",
			expected: "",
		},
		{
			name: "empty JSONPath",
			object: map[string]any{
				"status": map[string]any{
					"phase": "Running",
				},
			},
			jsonPath: "",
			expected: "",
		},
		{
			name: "JSONPath with braces already",
			object: map[string]any{
				"status": map[string]any{
					"phase": "Running",
				},
			},
			jsonPath: "{.status.phase}",
			expected: "Running",
		},
		{
			name: "nil value",
			object: map[string]any{
				"status": map[string]any{
					"phase": nil,
				},
			},
			jsonPath: ".status.phase",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := &unstructured.Unstructured{Object: tt.object}
			result := EvaluateJSONPath(u, tt.jsonPath)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestEvaluateJSONPathOrDefault(t *testing.T) {
	tests := []struct {
		name         string
		object       map[string]any
		jsonPath     string
		defaultValue string
		expected     string
	}{
		{
			name: "value exists",
			object: map[string]any{
				"status": map[string]any{
					"phase": "Running",
				},
			},
			jsonPath:     ".status.phase",
			defaultValue: "Unknown",
			expected:     "Running",
		},
		{
			name: "value missing - use default",
			object: map[string]any{
				"status": map[string]any{},
			},
			jsonPath:     ".status.phase",
			defaultValue: "Unknown",
			expected:     "Unknown",
		},
		{
			name:         "empty result - use default",
			object:       map[string]any{},
			jsonPath:     ".status.phase",
			defaultValue: "N/A",
			expected:     "N/A",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := &unstructured.Unstructured{Object: tt.object}
			result := EvaluateJSONPathOrDefault(u, tt.jsonPath, tt.defaultValue)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestEvaluateJSONPath_ComplexTypes(t *testing.T) {
	// Test with complex nested structures (like cert-manager Certificate)
	u := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "cert-manager.io/v1",
			"kind":       "Certificate",
			"metadata": map[string]any{
				"name":      "test-cert",
				"namespace": "default",
			},
			"spec": map[string]any{
				"secretName": "test-secret",
				"issuerRef": map[string]any{
					"name": "letsencrypt",
					"kind": "ClusterIssuer",
				},
			},
			"status": map[string]any{
				"conditions": []any{
					map[string]any{
						"type":   "Ready",
						"status": "True",
					},
				},
			},
		},
	}

	// Test extracting issuer name
	issuer := EvaluateJSONPath(u, ".spec.issuerRef.name")
	assert.Equal(t, "letsencrypt", issuer)

	// Test extracting ready status
	ready := EvaluateJSONPath(u, ".status.conditions[0].status")
	assert.Equal(t, "True", ready)
}
