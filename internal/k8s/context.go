package k8s

import "time"

// Context represents a Kubernetes context for display
type Context struct {
	Name      string
	Cluster   string
	User      string
	Namespace string
	Status    string // "Loaded", "Loading", "Failed", "Not Loaded"
	Current   string // "✓" if current, "" otherwise
	Error     string // Error message if failed
	LoadedAt  time.Time
}
