package screens

import (
	"strings"

	"github.com/renato0307/k1/internal/k8s"
)

// inferColumnConfig determines appropriate MinWidth/MaxWidth/Weight for a column
// based on heuristics (column name pattern matching).
func inferColumnConfig(columnName string, colType string) (minWidth, maxWidth int, weight float64, priority int) {
	// Normalize column name for matching
	normalized := strings.ToLower(columnName)

	// Match common column types by name
	switch {
	case normalized == "name" || normalized == "NAME":
		return NameMinWidth, NameMaxWidth, NameWeight, 1

	case normalized == "namespace" || normalized == "NAMESPACE":
		return NamespaceMinWidth, NamespaceMaxWidth, 3, 2 // Weight=3 per user preference

	case strings.Contains(normalized, "status") || strings.Contains(normalized, "state") || strings.Contains(normalized, "phase"):
		return StatusMinWidth, StatusMaxWidth, StatusWeight, 1

	case strings.Contains(normalized, "age"):
		return AgeMinWidth, AgeMaxWidth, AgeWeight, 1

	case colType == "date":
		// Date columns (usually Age-like)
		return AgeMinWidth, AgeMaxWidth, AgeWeight, 1

	case colType == "integer" || colType == "number":
		// Numeric columns (count, replicas, etc.)
		return 6, 12, 1, 1

	case strings.Contains(normalized, "message") || strings.Contains(normalized, "reason") || strings.Contains(normalized, "description"):
		// Long text fields - allow more growth
		return 15, 50, 2, 2

	default:
		// Generic string column
		return 10, 30, 2, 3
	}
}

// GenerateScreenConfigForCR creates a ScreenConfig for CR instances.
// Uses additionalPrinterColumns from CRD if available, otherwise falls back
// to generic columns (namespace, name, age).
func GenerateScreenConfigForCR(crd k8s.CustomResourceDefinition) ScreenConfig {

	// Generate screen ID: group/plural (handles duplicates)
	screenID := crd.Plural
	if crd.Group != "" {
		screenID = crd.Group + "/" + crd.Plural
	}

	// Build columns
	columns := []ColumnConfig{}
	searchFields := []string{}

	// Strategy 1: Use additionalPrinterColumns if available (Priority 1 approach)
	if len(crd.Columns) > 0 {
		// Add namespace column first if CRD is namespaced
		if crd.Scope == "Namespaced" {
			minW, maxW, weight, priority := inferColumnConfig("Namespace", "string")
			columns = append(columns, ColumnConfig{
				Field:    "Namespace",
				Title:    "Namespace",
				MinWidth: minW,
				MaxWidth: maxW,
				Weight:   weight,
				Priority: priority,
			})
			searchFields = append(searchFields, "Namespace")
		}

		// Add name column (always present)
		minW, maxW, weight, priority := inferColumnConfig("Name", "string")
		columns = append(columns, ColumnConfig{
			Field:    "Name",
			Title:    "Name",
			MinWidth: minW,
			MaxWidth: maxW,
			Weight:   weight,
			Priority: priority,
		})
		searchFields = append(searchFields, "Name")

		// Add CRD-specific columns from additionalPrinterColumns
		hasAgeColumn := false

		for _, col := range crd.Columns {
			// Track if CRD already defines Age
			if col.Name == "Age" {
				hasAgeColumn = true
			}

			// Infer sizing configuration based on column name and type
			minW, maxW, weight, inferredPriority := inferColumnConfig(col.Name, col.Type)

			// Map CRD column types to formatting functions
			var format func(interface{}) string
			switch col.Type {
			case "date":
				format = FormatDate
			}

			// Use Fields map for dynamic column values
			fieldKey := "Fields." + col.Name

			// Use CRD priority if specified, otherwise use inferred priority
			// CRD priority 0 = always visible (our priority 1)
			// CRD priority 1+ = less important (our priority 2+)
			priority := int(col.Priority) + 1
			if col.Priority > 10 {
				// If CRD doesn't specify priority (uses default large value),
				// fall back to inferred priority
				priority = inferredPriority
			}

			columns = append(columns, ColumnConfig{
				Field:    fieldKey,
				Title:    col.Name,
				MinWidth: minW,
				MaxWidth: maxW,
				Weight:   weight,
				Priority: priority,
				Format:   format,
			})

			// Add to search fields if priority 0 (always visible)
			if col.Priority == 0 {
				searchFields = append(searchFields, fieldKey)
			}
		}

		// Only add Age column if CRD didn't define it
		if !hasAgeColumn {
			minW, maxW, weight, priority := inferColumnConfig("Age", "date")
			columns = append(columns, ColumnConfig{
				Field:    "Age",
				Title:    "Age",
				MinWidth: minW,
				MaxWidth: maxW,
				Weight:   weight,
				Priority: priority,
				Format:   FormatDuration,
			})
		}

	} else {
		// Strategy 2: Fallback to generic columns (namespace, name, age)
		if crd.Scope == "Namespaced" {
			minW, maxW, weight, priority := inferColumnConfig("Namespace", "string")
			columns = append(columns, ColumnConfig{
				Field:    "Namespace",
				Title:    "Namespace",
				MinWidth: minW,
				MaxWidth: maxW,
				Weight:   weight,
				Priority: priority,
			})
			searchFields = append(searchFields, "Namespace")
		}

		// Name column
		minW, maxW, weight, priority := inferColumnConfig("Name", "string")
		columns = append(columns, ColumnConfig{
			Field:    "Name",
			Title:    "Name",
			MinWidth: minW,
			MaxWidth: maxW,
			Weight:   weight,
			Priority: priority,
		})

		// Age column
		minW2, maxW2, weight2, priority2 := inferColumnConfig("Age", "date")
		columns = append(columns, ColumnConfig{
			Field:    "Age",
			Title:    "Age",
			MinWidth: minW2,
			MaxWidth: maxW2,
			Weight:   weight2,
			Priority: priority2,
			Format:   FormatDuration,
		})

		searchFields = append(searchFields, "Name")
	}

	return ScreenConfig{
		ID:           screenID,
		Title:        crd.Kind,
		Columns:      columns,
		SearchFields: searchFields,
		Operations: []OperationConfig{
			{ID: "describe", Name: "Describe",
				Description: "Describe selected " + crd.Kind,
				Shortcut:    "d"},
			{ID: "yaml", Name: "View YAML",
				Description: "View " + crd.Kind + " YAML",
				Shortcut:    "y"},
		},
		EnablePeriodicRefresh: true,
		RefreshInterval:       RefreshInterval,
		TrackSelection:        true,
		CustomUpdate:          getPeriodicRefreshUpdate(),
	}
}
