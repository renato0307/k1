package k8s

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

// TestParseKubeconfig tests parseKubeconfig function
func TestParseKubeconfig(t *testing.T) {
	tests := []struct {
		name        string
		setupFunc   func(t *testing.T) string // Returns kubeconfig path
		expectError bool
		validate    func(t *testing.T, contexts []*ContextInfo)
	}{
		{
			name: "valid kubeconfig with multiple contexts",
			setupFunc: func(t *testing.T) string {
				config := clientcmdapi.NewConfig()
				config.Clusters["cluster1"] = &clientcmdapi.Cluster{
					Server: "https://cluster1.example.com",
				}
				config.Clusters["cluster2"] = &clientcmdapi.Cluster{
					Server: "https://cluster2.example.com",
				}
				config.AuthInfos["user1"] = &clientcmdapi.AuthInfo{
					Token: "token1",
				}
				config.AuthInfos["user2"] = &clientcmdapi.AuthInfo{
					Token: "token2",
				}
				config.Contexts["ctx-alpha"] = &clientcmdapi.Context{
					Cluster:   "cluster1",
					AuthInfo:  "user1",
					Namespace: "default",
				}
				config.Contexts["ctx-beta"] = &clientcmdapi.Context{
					Cluster:   "cluster2",
					AuthInfo:  "user2",
					Namespace: "kube-system",
				}
				config.Contexts["ctx-gamma"] = &clientcmdapi.Context{
					Cluster:   "cluster1",
					AuthInfo:  "user1",
					Namespace: "",
				}
				config.CurrentContext = "ctx-beta"

				tmpDir := t.TempDir()
				kubeconfigPath := filepath.Join(tmpDir, "kubeconfig")
				err := clientcmd.WriteToFile(*config, kubeconfigPath)
				require.NoError(t, err)
				return kubeconfigPath
			},
			expectError: false,
			validate: func(t *testing.T, contexts []*ContextInfo) {
				require.Len(t, contexts, 3)

				// Verify alphabetical sorting
				assert.Equal(t, "ctx-alpha", contexts[0].Name)
				assert.Equal(t, "ctx-beta", contexts[1].Name)
				assert.Equal(t, "ctx-gamma", contexts[2].Name)

				// Verify first context fields
				assert.Equal(t, "cluster1", contexts[0].Cluster)
				assert.Equal(t, "user1", contexts[0].User)
				assert.Equal(t, "default", contexts[0].Namespace)

				// Verify second context fields
				assert.Equal(t, "cluster2", contexts[1].Cluster)
				assert.Equal(t, "user2", contexts[1].User)
				assert.Equal(t, "kube-system", contexts[1].Namespace)

				// Verify empty namespace handling
				assert.Equal(t, "", contexts[2].Namespace)
			},
		},
		{
			name: "invalid kubeconfig path",
			setupFunc: func(t *testing.T) string {
				return "/nonexistent/path/kubeconfig"
			},
			expectError: true,
			validate:    nil,
		},
		{
			name: "empty kubeconfig",
			setupFunc: func(t *testing.T) string {
				config := clientcmdapi.NewConfig()
				tmpDir := t.TempDir()
				kubeconfigPath := filepath.Join(tmpDir, "kubeconfig")
				err := clientcmd.WriteToFile(*config, kubeconfigPath)
				require.NoError(t, err)
				return kubeconfigPath
			},
			expectError: false,
			validate: func(t *testing.T, contexts []*ContextInfo) {
				assert.Len(t, contexts, 0)
			},
		},
		{
			name: "single context",
			setupFunc: func(t *testing.T) string {
				config := clientcmdapi.NewConfig()
				config.Clusters["cluster"] = &clientcmdapi.Cluster{
					Server: "https://cluster.example.com",
				}
				config.AuthInfos["user"] = &clientcmdapi.AuthInfo{
					Token: "token",
				}
				config.Contexts["only-context"] = &clientcmdapi.Context{
					Cluster:   "cluster",
					AuthInfo:  "user",
					Namespace: "default",
				}
				config.CurrentContext = "only-context"

				tmpDir := t.TempDir()
				kubeconfigPath := filepath.Join(tmpDir, "kubeconfig")
				err := clientcmd.WriteToFile(*config, kubeconfigPath)
				require.NoError(t, err)
				return kubeconfigPath
			},
			expectError: false,
			validate: func(t *testing.T, contexts []*ContextInfo) {
				require.Len(t, contexts, 1)
				assert.Equal(t, "only-context", contexts[0].Name)
				assert.Equal(t, "cluster", contexts[0].Cluster)
				assert.Equal(t, "user", contexts[0].User)
				assert.Equal(t, "default", contexts[0].Namespace)
			},
		},
		{
			name: "sorting verification with reverse alphabetical names",
			setupFunc: func(t *testing.T) string {
				config := clientcmdapi.NewConfig()
				config.Clusters["cluster"] = &clientcmdapi.Cluster{
					Server: "https://cluster.example.com",
				}
				config.AuthInfos["user"] = &clientcmdapi.AuthInfo{
					Token: "token",
				}
				// Add contexts in reverse alphabetical order
				for _, name := range []string{"zulu", "yankee", "xray", "alpha", "bravo"} {
					config.Contexts[name] = &clientcmdapi.Context{
						Cluster:   "cluster",
						AuthInfo:  "user",
						Namespace: "default",
					}
				}
				config.CurrentContext = "zulu"

				tmpDir := t.TempDir()
				kubeconfigPath := filepath.Join(tmpDir, "kubeconfig")
				err := clientcmd.WriteToFile(*config, kubeconfigPath)
				require.NoError(t, err)
				return kubeconfigPath
			},
			expectError: false,
			validate: func(t *testing.T, contexts []*ContextInfo) {
				require.Len(t, contexts, 5)
				// Verify sorted alphabetically
				assert.Equal(t, "alpha", contexts[0].Name)
				assert.Equal(t, "bravo", contexts[1].Name)
				assert.Equal(t, "xray", contexts[2].Name)
				assert.Equal(t, "yankee", contexts[3].Name)
				assert.Equal(t, "zulu", contexts[4].Name)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kubeconfigPath := tt.setupFunc(t)

			contexts, err := parseKubeconfig(kubeconfigPath)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, contexts)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, contexts)
				if tt.validate != nil {
					tt.validate(t, contexts)
				}
			}
		})
	}
}

// TestGetCurrentContext tests getCurrentContext and GetCurrentContext functions
func TestGetCurrentContext(t *testing.T) {
	tests := []struct {
		name           string
		setupFunc      func(t *testing.T) string
		expectedResult string
		expectError    bool
	}{
		{
			name: "valid kubeconfig returns current context",
			setupFunc: func(t *testing.T) string {
				config := clientcmdapi.NewConfig()
				config.Clusters["cluster"] = &clientcmdapi.Cluster{
					Server: "https://cluster.example.com",
				}
				config.AuthInfos["user"] = &clientcmdapi.AuthInfo{
					Token: "token",
				}
				config.Contexts["context1"] = &clientcmdapi.Context{
					Cluster:   "cluster",
					AuthInfo:  "user",
					Namespace: "default",
				}
				config.Contexts["context2"] = &clientcmdapi.Context{
					Cluster:   "cluster",
					AuthInfo:  "user",
					Namespace: "kube-system",
				}
				config.CurrentContext = "context2"

				tmpDir := t.TempDir()
				kubeconfigPath := filepath.Join(tmpDir, "kubeconfig")
				err := clientcmd.WriteToFile(*config, kubeconfigPath)
				require.NoError(t, err)
				return kubeconfigPath
			},
			expectedResult: "context2",
			expectError:    false,
		},
		{
			name: "invalid kubeconfig path returns error",
			setupFunc: func(t *testing.T) string {
				return "/nonexistent/path/kubeconfig"
			},
			expectedResult: "",
			expectError:    true,
		},
		{
			name: "no current context set returns empty string",
			setupFunc: func(t *testing.T) string {
				config := clientcmdapi.NewConfig()
				config.Clusters["cluster"] = &clientcmdapi.Cluster{
					Server: "https://cluster.example.com",
				}
				config.AuthInfos["user"] = &clientcmdapi.AuthInfo{
					Token: "token",
				}
				config.Contexts["context1"] = &clientcmdapi.Context{
					Cluster:   "cluster",
					AuthInfo:  "user",
					Namespace: "default",
				}
				// CurrentContext not set

				tmpDir := t.TempDir()
				kubeconfigPath := filepath.Join(tmpDir, "kubeconfig")
				err := clientcmd.WriteToFile(*config, kubeconfigPath)
				require.NoError(t, err)
				return kubeconfigPath
			},
			expectedResult: "",
			expectError:    false,
		},
		{
			name: "corrupted kubeconfig file",
			setupFunc: func(t *testing.T) string {
				tmpDir := t.TempDir()
				kubeconfigPath := filepath.Join(tmpDir, "kubeconfig")
				// Write invalid YAML
				err := os.WriteFile(kubeconfigPath, []byte("invalid: yaml: content: ["), 0644)
				require.NoError(t, err)
				return kubeconfigPath
			},
			expectedResult: "",
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kubeconfigPath := tt.setupFunc(t)

			// Test private function
			result1, err1 := getCurrentContext(kubeconfigPath)

			if tt.expectError {
				assert.Error(t, err1)
				assert.Equal(t, tt.expectedResult, result1)
			} else {
				assert.NoError(t, err1)
				assert.Equal(t, tt.expectedResult, result1)
			}

			// Test public function (should have same behavior)
			result2, err2 := GetCurrentContext(kubeconfigPath)

			if tt.expectError {
				assert.Error(t, err2)
				assert.Equal(t, tt.expectedResult, result2)
			} else {
				assert.NoError(t, err2)
				assert.Equal(t, tt.expectedResult, result2)
			}

			// Verify both functions return same results
			assert.Equal(t, result1, result2)
			if err1 != nil || err2 != nil {
				assert.Equal(t, err1 != nil, err2 != nil)
			}
		})
	}
}
