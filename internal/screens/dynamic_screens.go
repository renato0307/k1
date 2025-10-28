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

	// Basic columns: name, age
	columns := []ColumnConfig{
		{Field: "Name", Title: "Name", Width: 0, Priority: 1},
		{Field: "Age", Title: "Age", Width: 10, Format: FormatDuration, Priority: 1},
	}

	// Add namespace column if CRD is namespaced
	if crd.Scope == "Namespaced" {
		columns = append([]ColumnConfig{
			{Field: "Namespace", Title: "Namespace", Width: 20, Priority: 2},
		}, columns...)
	}

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
