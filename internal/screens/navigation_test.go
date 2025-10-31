package screens

import (
	"testing"

	"github.com/renato0307/k1/internal/k8s"
	"github.com/renato0307/k1/internal/types"
	"github.com/renato0307/k1/internal/ui"
	"github.com/stretchr/testify/assert"
)

func TestNavigateToCRInstances(t *testing.T) {
	tests := []struct {
		name           string
		resource       k8s.CustomResourceDefinition
		expectNil      bool
		expectedGroup  string
		expectedKind   string
		expectedPlural string
		columnCount    int
	}{
		{
			name: "valid CRD with columns",
			resource: k8s.CustomResourceDefinition{
				ResourceMetadata: k8s.ResourceMetadata{Name: "issuers.cert-manager.io"},
				Group:            "cert-manager.io",
				Version:          "v1",
				Kind:             "Issuer",
				Plural:           "issuers",
				Scope:            "Namespaced",
				Columns: []k8s.CRDColumn{
					{Name: "Ready", Type: "string", JSONPath: ".status.conditions[?(@.type==\"Ready\")].status"},
					{Name: "Status", Type: "string", JSONPath: ".status.conditions[?(@.type==\"Ready\")].message"},
					{Name: "Age", Type: "date", JSONPath: ".metadata.creationTimestamp"},
				},
			},
			expectNil:      false,
			expectedGroup:  "cert-manager.io",
			expectedKind:   "Issuer",
			expectedPlural: "issuers",
			columnCount:    3,
		},
		{
			name: "valid CRD without columns",
			resource: k8s.CustomResourceDefinition{
				ResourceMetadata: k8s.ResourceMetadata{Name: "myresources.example.com"},
				Group:            "example.com",
				Version:          "v1",
				Kind:             "MyResource",
				Plural:           "myresources",
				Scope:            "Cluster",
			},
			expectNil:      false,
			expectedGroup:  "example.com",
			expectedKind:   "MyResource",
			expectedPlural: "myresources",
			columnCount:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a mock screen with the resource
			screen := &ConfigScreen{
				config: ScreenConfig{
					ID:    "crds",
					Title: "Custom Resource Definitions",
				},
				filtered: []interface{}{tt.resource},
			}

			// Call the navigation handler
			handler := navigateToCRInstances()
			cmd := handler(screen)

			if tt.expectNil {
				assert.Nil(t, cmd)
				return
			}

			assert.NotNil(t, cmd)

			// Execute the command to get the message
			msg := cmd()

			// Type assert to DynamicScreenCreateMsg
			dynMsg, ok := msg.(types.DynamicScreenCreateMsg)
			assert.True(t, ok, "Expected DynamicScreenCreateMsg")

			// Type assert CRD to CustomResourceDefinition
			crd, ok := dynMsg.CRD.(k8s.CustomResourceDefinition)
			assert.True(t, ok, "Expected CRD to be CustomResourceDefinition")

			// Verify CRD fields
			if tt.expectedGroup != "" {
				assert.Equal(t, tt.expectedGroup, crd.Group)
			}
			if tt.expectedKind != "" {
				assert.Equal(t, tt.expectedKind, crd.Kind)
			}
			if tt.expectedPlural != "" {
				assert.Equal(t, tt.expectedPlural, crd.Plural)
			}

			// Verify columns
			assert.Equal(t, tt.columnCount, len(crd.Columns))
		})
	}
}

func TestNavigateToPodsForOwner(t *testing.T) {
	tests := []struct {
		name              string
		kind              string
		resource          interface{}
		expectNil         bool
		expectedNamespace string
		expectedOwner     string
	}{
		{
			name: "valid deployment resource",
			kind: "Deployment",
			resource: k8s.Deployment{
				ResourceMetadata: k8s.ResourceMetadata{
					Namespace: "default",
					Name:      "nginx-deployment",
				},
			},
			expectNil:         false,
			expectedNamespace: "default",
			expectedOwner:     "nginx-deployment",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			screen := &ConfigScreen{
				filtered: []interface{}{tt.resource},
			}

			handler := navigateToPodsForOwner(tt.kind)
			cmd := handler(screen)

			if tt.expectNil {
				assert.Nil(t, cmd)
				return
			}

			assert.NotNil(t, cmd)

			// Execute the command
			msg := cmd()

			// Type assert to ScreenSwitchMsg
			switchMsg, ok := msg.(types.ScreenSwitchMsg)
			assert.True(t, ok, "Expected ScreenSwitchMsg")

			// Verify message fields
			assert.Equal(t, "pods", switchMsg.ScreenID)
			assert.Equal(t, "owner", switchMsg.FilterContext.Field)
			assert.Equal(t, tt.expectedOwner, switchMsg.FilterContext.Value)
			assert.Equal(t, tt.expectedNamespace, switchMsg.FilterContext.Metadata["namespace"])
			assert.Equal(t, tt.kind, switchMsg.FilterContext.Metadata["kind"])
		})
	}
}

func TestNavigateToPodsForNode(t *testing.T) {
	screen := &ConfigScreen{
		filtered: []interface{}{
			k8s.Node{
				ResourceMetadata: k8s.ResourceMetadata{
					Name: "node-1",
				},
			},
		},
	}

	handler := navigateToPodsForNode()
	cmd := handler(screen)

	assert.NotNil(t, cmd)

	msg := cmd()
	switchMsg, ok := msg.(types.ScreenSwitchMsg)
	assert.True(t, ok)
	assert.Equal(t, "pods", switchMsg.ScreenID)
	assert.Equal(t, "node", switchMsg.FilterContext.Field)
	assert.Equal(t, "node-1", switchMsg.FilterContext.Value)
}

func TestNavigateToPodsForService(t *testing.T) {
	screen := &ConfigScreen{
		filtered: []interface{}{
			k8s.Service{
				ResourceMetadata: k8s.ResourceMetadata{
					Namespace: "kube-system",
					Name:      "kube-dns",
				},
			},
		},
	}

	handler := navigateToPodsForService()
	cmd := handler(screen)

	assert.NotNil(t, cmd)

	msg := cmd()
	switchMsg, ok := msg.(types.ScreenSwitchMsg)
	assert.True(t, ok)
	assert.Equal(t, "pods", switchMsg.ScreenID)
	assert.Equal(t, "selector", switchMsg.FilterContext.Field)
	assert.Equal(t, "kube-dns", switchMsg.FilterContext.Value)
}

func TestNavigateToContextSwitch(t *testing.T) {
	tests := []struct {
		name            string
		resource        k8s.Context
		expectNil       bool
		expectedContext string
	}{
		{
			name: "valid context",
			resource: k8s.Context{
				Name: "prod-cluster",
			},
			expectNil:       false,
			expectedContext: "prod-cluster",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			screen := &ConfigScreen{
				filtered: []interface{}{tt.resource},
			}

			handler := navigateToContextSwitch()
			cmd := handler(screen)

			if tt.expectNil {
				assert.Nil(t, cmd)
				return
			}

			assert.NotNil(t, cmd)

			msg := cmd()
			ctxMsg, ok := msg.(types.ContextSwitchMsg)
			assert.True(t, ok)
			assert.Equal(t, tt.expectedContext, ctxMsg.ContextName)
		})
	}
}

func TestNavigationFactories_ReturnFunctions(t *testing.T) {
	// Test that all navigation factory functions return non-nil functions
	repo := &k8s.DummyRepository{}
	theme := ui.ThemeCharm()

	factories := []struct {
		name    string
		factory NavigationFunc
	}{
		{"navigateToPodsForOwner", navigateToPodsForOwner("Deployment")},
		{"navigateToPodsForNode", navigateToPodsForNode()},
		{"navigateToPodsForService", navigateToPodsForService()},
		{"navigateToPodsForNamespace", navigateToPodsForNamespace()},
		{"navigateToPodsForVolumeSource", navigateToPodsForVolumeSource("ConfigMap")},
		{"navigateToJobsForCronJob", navigateToJobsForCronJob()},
		{"navigateToReplicaSetsForDeployment", navigateToReplicaSetsForDeployment()},
		{"navigateToPodsForPVC", navigateToPodsForPVC()},
		{"navigateToServicesForIngress", navigateToServicesForIngress()},
		{"navigateToPodsForEndpoints", navigateToPodsForEndpoints()},
		{"navigateToTargetForHPA", navigateToTargetForHPA()},
		{"navigateToContextSwitch", navigateToContextSwitch()},
		{"navigateToCRInstances", navigateToCRInstances()},
	}

	for _, tt := range factories {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotNil(t, tt.factory)

			// Create a mock screen
			screen := NewConfigScreen(ScreenConfig{
				ID:    "test",
				Title: "Test",
			}, repo, theme)

			// Call the factory function (should not panic)
			cmd := tt.factory(screen)
			// cmd can be nil if there's no selected resource, that's ok
			_ = cmd
		})
	}
}
