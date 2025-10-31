package k8s

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/util/jsonpath"
)

// EvaluateJSONPath evaluates a JSONPath expression on an unstructured resource
// and returns the result as a string.
//
// JSONPath expressions used in CRD additionalPrinterColumns typically extract
// single values or simple lists. This function handles common patterns:
// - Simple field access: .status.phase
// - Nested fields: .status.conditions[0].status
// - Filtered arrays: .status.conditions[?(@.type=="Ready")].status
//
// Returns empty string if:
// - JSONPath expression is invalid
// - Field doesn't exist
// - Evaluation fails
func EvaluateJSONPath(u *unstructured.Unstructured, jsonPathExpr string) string {
	// Handle empty expression
	if jsonPathExpr == "" {
		return ""
	}

	// Kubernetes JSONPath expressions in additionalPrinterColumns start with "."
	// client-go's jsonpath parser expects expressions wrapped in {}
	// e.g., ".status.phase" becomes "{.status.phase}"
	if !strings.HasPrefix(jsonPathExpr, "{") {
		jsonPathExpr = "{" + jsonPathExpr + "}"
	}

	// Create and parse JSONPath
	jp := jsonpath.New("columnValue")
	jp.AllowMissingKeys(true) // Don't error on missing fields

	if err := jp.Parse(jsonPathExpr); err != nil {
		// Invalid JSONPath expression
		return ""
	}

	// Execute JSONPath on the resource
	results, err := jp.FindResults(u.Object)
	if err != nil {
		// Evaluation failed
		return ""
	}

	// No results found
	if len(results) == 0 || len(results[0]) == 0 {
		return ""
	}

	// Extract first result
	firstResult := results[0][0]
	if !firstResult.IsValid() || !firstResult.CanInterface() {
		return ""
	}

	value := firstResult.Interface()

	// Convert value to string based on type
	switch v := value.(type) {
	case string:
		return v
	case int, int32, int64:
		return fmt.Sprintf("%d", v)
	case float32, float64:
		return fmt.Sprintf("%v", v)
	case bool:
		if v {
			return "True"
		}
		return "False"
	case nil:
		return ""
	default:
		// For complex types (maps, slices), use string representation
		return fmt.Sprintf("%v", v)
	}
}

// EvaluateJSONPathOrDefault evaluates a JSONPath expression and returns the
// result or a default value if evaluation fails or returns empty.
func EvaluateJSONPathOrDefault(u *unstructured.Unstructured, jsonPathExpr, defaultValue string) string {
	result := EvaluateJSONPath(u, jsonPathExpr)
	if result == "" {
		return defaultValue
	}
	return result
}
