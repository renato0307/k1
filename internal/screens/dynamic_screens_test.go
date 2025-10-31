package screens

import (
	"testing"

	"github.com/renato0307/k1/internal/k8s"
	"github.com/stretchr/testify/assert"
)

func TestGenerateScreenConfigForCR_Namespaced(t *testing.T) {
	crd := k8s.CustomResourceDefinition{
		Group:   "cert-manager.io",
		Version: "v1",
		Kind:    "Certificate",
		Plural:  "certificates",
		Scope:   "Namespaced",
	}

	config := GenerateScreenConfigForCR(crd)

	assert.Equal(t, "cert-manager.io/certificates", config.ID)
	assert.Equal(t, "Certificate", config.Title)

	// Should have 3 columns: Namespace, Name, Age
	assert.Len(t, config.Columns, 3)
	assert.Equal(t, "Namespace", config.Columns[0].Field)
	assert.Equal(t, "Name", config.Columns[1].Field)
	assert.Equal(t, "Age", config.Columns[2].Field)
}

func TestGenerateScreenConfigForCR_ClusterScoped(t *testing.T) {
	crd := k8s.CustomResourceDefinition{
		Group:   "stable.example.com",
		Version: "v1",
		Kind:    "ClusterWidget",
		Plural:  "clusterwidgets",
		Scope:   "Cluster",
	}

	config := GenerateScreenConfigForCR(crd)

	assert.Equal(t, "stable.example.com/clusterwidgets", config.ID)

	// Should have 2 columns: Name, Age (no Namespace)
	assert.Len(t, config.Columns, 2)
	assert.Equal(t, "Name", config.Columns[0].Field)
	assert.Equal(t, "Age", config.Columns[1].Field)
}

func TestGenerateScreenConfigForCR_WithAdditionalPrinterColumns_Namespaced(t *testing.T) {
	crd := k8s.CustomResourceDefinition{
		Group:   "cert-manager.io",
		Version: "v1",
		Kind:    "Certificate",
		Plural:  "certificates",
		Scope:   "Namespaced",
		Columns: []k8s.CRDColumn{
			{
				Name:        "Ready",
				Type:        "string",
				Description: "Certificate is ready",
				JSONPath:    ".status.conditions[?(@.type==\"Ready\")].status",
				Priority:    0,
			},
			{
				Name:        "Issuer",
				Type:        "string",
				Description: "Issuer name",
				JSONPath:    ".spec.issuerRef.name",
				Priority:    0,
			},
			{
				Name:        "Status",
				Type:        "string",
				Description: "Certificate status",
				JSONPath:    ".status.phase",
				Priority:    1,
			},
		},
	}

	config := GenerateScreenConfigForCR(crd)

	assert.Equal(t, "cert-manager.io/certificates", config.ID)
	assert.Equal(t, "Certificate", config.Title)

	// Should have 6 columns: Namespace, Name, Ready, Issuer, Status, Age
	assert.Len(t, config.Columns, 6)
	assert.Equal(t, "Namespace", config.Columns[0].Field)
	assert.Equal(t, "Name", config.Columns[1].Field)
	assert.Equal(t, "Fields.Ready", config.Columns[2].Field)
	assert.Equal(t, "Ready", config.Columns[2].Title)
	assert.Equal(t, "Fields.Issuer", config.Columns[3].Field)
	assert.Equal(t, "Issuer", config.Columns[3].Title)
	assert.Equal(t, "Fields.Status", config.Columns[4].Field)
	assert.Equal(t, "Status", config.Columns[4].Title)
	assert.Equal(t, "Age", config.Columns[5].Field)

	// Verify priorities are mapped correctly (CRD priority 0 -> our priority 1)
	assert.Equal(t, 1, config.Columns[2].Priority) // Ready (CRD priority 0)
	assert.Equal(t, 1, config.Columns[3].Priority) // Issuer (CRD priority 0)
	assert.Equal(t, 2, config.Columns[4].Priority) // Status (CRD priority 1 -> our 2)

	// Verify search fields include priority 0 columns
	assert.Contains(t, config.SearchFields, "Namespace")
	assert.Contains(t, config.SearchFields, "Name")
	assert.Contains(t, config.SearchFields, "Fields.Ready")
	assert.Contains(t, config.SearchFields, "Fields.Issuer")
}

func TestGenerateScreenConfigForCR_WithAdditionalPrinterColumns_ClusterScoped(t *testing.T) {
	crd := k8s.CustomResourceDefinition{
		Group:   "stable.example.com",
		Version: "v1",
		Kind:    "ClusterWidget",
		Plural:  "clusterwidgets",
		Scope:   "Cluster",
		Columns: []k8s.CRDColumn{
			{
				Name:     "Status",
				Type:     "string",
				JSONPath: ".status.phase",
				Priority: 0,
			},
			{
				Name:     "Count",
				Type:     "integer",
				JSONPath: ".status.count",
				Priority: 0,
			},
		},
	}

	config := GenerateScreenConfigForCR(crd)

	assert.Equal(t, "stable.example.com/clusterwidgets", config.ID)

	// Should have 4 columns: Name, Status, Count, Age (no Namespace for cluster-scoped)
	assert.Len(t, config.Columns, 4)
	assert.Equal(t, "Name", config.Columns[0].Field)
	assert.Equal(t, "Fields.Status", config.Columns[1].Field)
	assert.Equal(t, "Fields.Count", config.Columns[2].Field)
	assert.Equal(t, "Age", config.Columns[3].Field)

	// Verify no Namespace in search fields
	assert.NotContains(t, config.SearchFields, "Namespace")
	assert.Contains(t, config.SearchFields, "Name")
	assert.Contains(t, config.SearchFields, "Fields.Status")
	assert.Contains(t, config.SearchFields, "Fields.Count")
}

func TestGenerateScreenConfigForCR_WithDateColumn(t *testing.T) {
	crd := k8s.CustomResourceDefinition{
		Group:   "example.com",
		Version: "v1",
		Kind:    "Backup",
		Plural:  "backups",
		Scope:   "Namespaced",
		Columns: []k8s.CRDColumn{
			{
				Name:     "LastBackup",
				Type:     "date",
				JSONPath: ".status.lastBackupTime",
				Priority: 0,
			},
		},
	}

	config := GenerateScreenConfigForCR(crd)

	// Verify date column has appropriate width and format
	dateColumn := config.Columns[2] // After Namespace and Name
	assert.Equal(t, "Fields.LastBackup", dateColumn.Field)
	assert.Equal(t, 10, dateColumn.Width) // Date columns get 10 chars (e.g., "5d ago")
	assert.NotNil(t, dateColumn.Format)   // Should have FormatDate function
}
