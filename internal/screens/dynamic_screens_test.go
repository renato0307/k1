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
