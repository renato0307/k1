package k8s

import (
	"fmt"
	"sort"

	"k8s.io/client-go/tools/clientcmd"
)

// ContextInfo holds context metadata from kubeconfig
type ContextInfo struct {
	Name      string
	Cluster   string
	User      string
	Namespace string
}

// parseKubeconfig loads kubeconfig and extracts all contexts
func parseKubeconfig(kubeconfigPath string) ([]*ContextInfo, error) {
	// Load kubeconfig
	config, err := clientcmd.LoadFromFile(kubeconfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load kubeconfig: %w", err)
	}

	// Extract contexts
	contexts := make([]*ContextInfo, 0, len(config.Contexts))
	for name, ctx := range config.Contexts {
		contexts = append(contexts, &ContextInfo{
			Name:      name,
			Cluster:   ctx.Cluster,
			User:      ctx.AuthInfo,
			Namespace: ctx.Namespace,
		})
	}

	// Sort alphabetically by name to ensure stable order
	// This prevents context list position shifts caused by Go map iteration non-determinism
	sort.Slice(contexts, func(i, j int) bool {
		return contexts[i].Name < contexts[j].Name
	})

	return contexts, nil
}

// getCurrentContext returns the current context from kubeconfig
func getCurrentContext(kubeconfigPath string) (string, error) {
	config, err := clientcmd.LoadFromFile(kubeconfigPath)
	if err != nil {
		return "", err
	}
	return config.CurrentContext, nil
}

// GetCurrentContext returns the current context from kubeconfig (exported)
func GetCurrentContext(kubeconfigPath string) (string, error) {
	return getCurrentContext(kubeconfigPath)
}
