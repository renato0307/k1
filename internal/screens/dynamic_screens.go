package screens

import (
	"github.com/renato0307/k1/internal/k8s"
)

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
			columns = append(columns, ColumnConfig{
				Field: "Namespace", Title: "Namespace", Width: 25, Priority: 1,
			})
			searchFields = append(searchFields, "Namespace")
		}

		// Add name column (always present)
		columns = append(columns, ColumnConfig{
			Field: "Name", Title: "Name", Width: 40, Priority: 1,
		})
		searchFields = append(searchFields, "Name")

		// Add CRD-specific columns from additionalPrinterColumns
		hasAgeColumn := false
		numColumns := len(crd.Columns)

		for i, col := range crd.Columns {
			// Track if CRD already defines Age
			if col.Name == "Age" {
				hasAgeColumn = true
			}

			// Map CRD column types to formatting functions
			var format func(interface{}) string
			var width int

			switch col.Type {
			case "date":
				format = FormatDate
				width = 10 // Dates are compact (e.g., "5d ago")
			case "integer", "number":
				width = 12 // Numbers are usually short
			default:
				// String type - use heuristic based on position
				// Strategy: make second-to-last string column dynamic to fill space
				// (usually Status/Message fields, while last is often Age/date)
				isSecondToLast := (i == numColumns-2)
				isLastAndNotDate := (i == numColumns-1 && col.Type != "date")

				if isSecondToLast || isLastAndNotDate {
					width = 0 // Dynamic - fills remaining space
				} else {
					width = 15 // Compact fixed width for other strings
				}
			}

			// Use Fields map for dynamic column values
			fieldKey := "Fields." + col.Name

			columns = append(columns, ColumnConfig{
				Field:    fieldKey,
				Title:    col.Name,
				Width:    width,
				Priority: int(col.Priority) + 1, // CRD priority 0 = our priority 1
				Format:   format,
			})

			// Add to search fields if priority 0 (always visible)
			if col.Priority == 0 {
				searchFields = append(searchFields, fieldKey)
			}
		}

		// Only add Age column if CRD didn't define it
		if !hasAgeColumn {
			columns = append(columns, ColumnConfig{
				Field: "Age", Title: "Age", Width: 10, Format: FormatDuration, Priority: 1,
			})
		}

	} else {
		// Strategy 2: Fallback to generic columns (namespace, name, age)
		if crd.Scope == "Namespaced" {
			columns = append(columns, ColumnConfig{
				Field: "Namespace", Title: "Namespace", Width: 20, Priority: 1,
			})
			searchFields = append(searchFields, "Namespace")
		}

		columns = append(columns,
			ColumnConfig{Field: "Name", Title: "Name", Width: 0, Priority: 1},
			ColumnConfig{Field: "Age", Title: "Age", Width: 10, Format: FormatDuration, Priority: 1},
		)
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
