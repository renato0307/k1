package screens

import (
	"github.com/renato0307/k1/internal/k8s"
)

// GenerateScreenConfigForCR creates a ScreenConfig for CR instances
func GenerateScreenConfigForCR(crd k8s.CustomResourceDefinition) ScreenConfig {
	// Generate screen ID: group/plural (handles duplicates)
	screenID := crd.Plural
	if crd.Group != "" {
		screenID = crd.Group + "/" + crd.Plural
	}

	// Build columns: namespace (if namespaced), name, age
	columns := []ColumnConfig{}

	// Add namespace column first if CRD is namespaced
	// Fixed width (20 chars) - most namespaces fit in this
	if crd.Scope == "Namespaced" {
		columns = append(columns, ColumnConfig{
			Field: "Namespace", Title: "Namespace", Width: 20, Priority: 1,
		})
	}

	// Name gets dynamic width (0) to fill remaining space - it's the most important field
	// Age gets fixed width (10 chars)
	columns = append(columns,
		ColumnConfig{Field: "Name", Title: "Name", Width: 0, Priority: 1},
		ColumnConfig{Field: "Age", Title: "Age", Width: 10, Format: FormatDuration, Priority: 1},
	)

	return ScreenConfig{
		ID:           screenID,
		Title:        crd.Kind,
		Columns:      columns,
		SearchFields: []string{"Namespace", "Name"},
		Operations: []OperationConfig{
			{ID: "describe", Name: "Describe",
				Description: "Describe selected " + crd.Kind,
				Shortcut:    "d"},
			{ID: "yaml", Name: "View YAML",
				Description: "View " + crd.Kind + " YAML",
				Shortcut:    "y"},
		},
	}
}
