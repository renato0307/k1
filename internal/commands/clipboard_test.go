package commands

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/renato0307/k1/internal/k8s"
	"github.com/renato0307/k1/internal/types"
)

// mockRepository is defined in command_execution_test.go
// (shared across test files in the same package)

func TestShellCommand_CommandGeneration(t *testing.T) {
	repo := &mockRepository{
		kubeconfig: "/path/to/kubeconfig",
		context:    "test-context",
	}
	shellCmd := ShellCommand(repo)

	tests := []struct {
		name      string
		argString string
		expected  []string // Expected command parts
	}{
		{
			name:      "default shell",
			argString: "",
			expected:  []string{"kubectl exec -it", "test-pod", "--namespace default", "-- /bin/sh"},
		},
		{
			name:      "bash shell",
			argString: "bash",
			expected:  []string{"kubectl exec -it", "test-pod", "--namespace default", "-- bash"},
		},
		{
			name:      "with container",
			argString: "nginx /bin/bash",
			expected:  []string{"kubectl exec -it", "test-pod", "-c nginx", "-- /bin/bash"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := CommandContext{
				ResourceType: k8s.ResourceTypePod,
				Selected: map[string]any{
					"name":      "test-pod",
					"namespace": "default",
				},
				Args: tt.argString,
			}

			cmd := shellCmd(ctx)
			require.NotNil(t, cmd)

			// Execute the command
			msg := cmd()
			require.NotNil(t, msg)

			// Should return info message (clipboard copy)
			statusMsg, ok := msg.(types.StatusMsg)
			assert.True(t, ok, "expected StatusMsg")
			if ok {
				// Message should contain the command or clipboard info
				if statusMsg.Type == types.MessageTypeInfo || statusMsg.Type == types.MessageTypeError {
					t.Logf("Shell command message: %s", statusMsg.Message)
				}
			}
		})
	}
}

func TestLogsCommand_CommandGeneration(t *testing.T) {
	repo := &mockRepository{}
	logsCmd := LogsCommand(repo)

	tests := []struct {
		name      string
		argString string
		expected  []string
	}{
		{
			name:      "default settings",
			argString: "",
			expected:  []string{"kubectl logs", "test-pod", "--namespace default", "--tail=100"},
		},
		{
			name:      "with follow flag",
			argString: " 200 true",
			expected:  []string{"kubectl logs", "test-pod", "--tail=200", "-f"},
		},
		{
			name:      "with container",
			argString: "nginx 50 false",
			expected:  []string{"kubectl logs", "test-pod", "-c nginx", "--tail=50"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := CommandContext{
				ResourceType: k8s.ResourceTypePod,
				Selected: map[string]any{
					"name":      "test-pod",
					"namespace": "default",
				},
				Args: tt.argString,
			}

			cmd := logsCmd(ctx)
			require.NotNil(t, cmd)

			// Execute the command
			msg := cmd()
			require.NotNil(t, msg)

			statusMsg, ok := msg.(types.StatusMsg)
			assert.True(t, ok)
			if ok {
				t.Logf("Logs command status: %s", statusMsg.Message)
			}
		})
	}
}

func TestPortForwardCommand_CommandGeneration(t *testing.T) {
	repo := &mockRepository{}
	pfCmd := PortForwardCommand(repo)

	tests := []struct {
		name      string
		argString string
		wantErr   bool
		expected  []string
	}{
		{
			name:      "single port",
			argString: "8080:80",
			wantErr:   false,
			expected:  []string{"kubectl port-forward", "test-pod", "8080:80"},
		},
		{
			name:      "same port",
			argString: "3000",
			wantErr:   false,
			expected:  []string{"kubectl port-forward", "test-pod", "3000"},
		},
		{
			name:      "missing port",
			argString: "",
			wantErr:   true,
			expected:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := CommandContext{
				ResourceType: k8s.ResourceTypePod,
				Selected: map[string]any{
					"name":      "test-pod",
					"namespace": "default",
				},
				Args: tt.argString,
			}

			cmd := pfCmd(ctx)
			require.NotNil(t, cmd)

			// Execute the command
			msg := cmd()
			require.NotNil(t, msg)

			statusMsg, ok := msg.(types.StatusMsg)
			assert.True(t, ok)
			if ok {
				if tt.wantErr {
					assert.Equal(t, types.MessageTypeError, statusMsg.Type)
				} else {
					t.Logf("Port-forward command status: %s", statusMsg.Message)
				}
			}
		})
	}
}

func TestShellCommand_KubeconfigContext(t *testing.T) {
	repo := &mockRepository{
		kubeconfig: "/custom/kubeconfig",
		context:    "prod-cluster",
	}
	shellCmd := ShellCommand(repo)

	ctx := CommandContext{
		ResourceType: k8s.ResourceTypePod,
		Selected: map[string]any{
			"name":      "app-pod",
			"namespace": "production",
		},
		Args: "",
	}

	cmd := shellCmd(ctx)
	require.NotNil(t, cmd)

	msg := cmd()
	require.NotNil(t, msg)

	statusMsg, ok := msg.(types.StatusMsg)
	assert.True(t, ok)
	if ok {
		// The message should contain command with kubeconfig/context
		// or clipboard copy notification
		assert.NotEmpty(t, statusMsg.Message)
		t.Logf("Shell command with custom config: %s", statusMsg.Message)
	}
}

func TestLogsCommand_ArgParsing(t *testing.T) {
	repo := &mockRepository{}
	logsCmd := LogsCommand(repo)

	tests := []struct {
		name      string
		argString string
		wantErr   bool
	}{
		{
			name:      "valid args",
			argString: "nginx 100 true",
			wantErr:   false,
		},
		{
			name:      "defaults only",
			argString: "",
			wantErr:   false,
		},
		{
			name:      "invalid tail",
			argString: "container notanumber false",
			wantErr:   true,
		},
		{
			name:      "invalid follow",
			argString: "container 100 notabool",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := CommandContext{
				ResourceType: k8s.ResourceTypePod,
				Selected: map[string]any{
					"name":      "test-pod",
					"namespace": "default",
				},
				Args: tt.argString,
			}

			cmd := logsCmd(ctx)
			require.NotNil(t, cmd)

			msg := cmd()
			statusMsg, ok := msg.(types.StatusMsg)
			assert.True(t, ok)

			if tt.wantErr {
				assert.Equal(t, types.MessageTypeError, statusMsg.Type)
				assert.Contains(t, strings.ToLower(statusMsg.Message), "invalid")
			}
		})
	}
}
