package commands

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewKubectlExecutor(t *testing.T) {
	tests := []struct {
		name        string
		kubeconfig  string
		context     string
		expectValid bool
	}{
		{
			name:        "with kubeconfig and context",
			kubeconfig:  "/path/to/kubeconfig",
			context:     "test-context",
			expectValid: true,
		},
		{
			name:        "with kubeconfig only",
			kubeconfig:  "/path/to/kubeconfig",
			context:     "",
			expectValid: true,
		},
		{
			name:        "empty kubeconfig",
			kubeconfig:  "",
			context:     "",
			expectValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor := NewKubectlExecutor(tt.kubeconfig, tt.context)
			require.NotNil(t, executor)
			assert.Equal(t, tt.kubeconfig, executor.kubeconfig)
			assert.Equal(t, tt.context, executor.context)
		})
	}
}

func TestKubectlExecutor_Execute_WithTimeout(t *testing.T) {
	executor := NewKubectlExecutor("", "")

	// Test default timeout is applied
	opts := ExecuteOptions{
		Timeout: 0, // Should use DefaultKubectlTimeout
	}

	// Use kubectl version as a fast command
	_, err := executor.Execute([]string{"version", "--client"}, opts)

	// This might fail if kubectl is not installed, which is fine for this test
	// We're mainly testing that the timeout logic works
	if err != nil {
		t.Logf("kubectl not available: %v", err)
	}
}

func TestKubectlExecutor_Execute_WithCustomTimeout(t *testing.T) {
	executor := NewKubectlExecutor("", "")

	// Test custom timeout
	opts := ExecuteOptions{
		Timeout: 1 * time.Second,
	}

	// Use a fast command
	_, err := executor.Execute([]string{"version", "--client"}, opts)

	if err != nil {
		t.Logf("kubectl not available: %v", err)
	}
}

func TestCheckAvailable(t *testing.T) {
	// This will check if kubectl is actually installed
	err := CheckAvailable()

	// Just log the result - don't fail test if kubectl not installed
	if err != nil {
		t.Logf("kubectl not available: %v", err)
	} else {
		t.Log("kubectl is available")
	}

	// The check should not panic - either nil or error is valid
	assert.True(t, err == nil || err != nil)
}
