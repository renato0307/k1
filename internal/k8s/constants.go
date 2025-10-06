package k8s

import "time"

// Kubernetes client constants
const (
	// InformerResyncPeriod is how often informers resync their full cache from
	// the Kubernetes API server. 30 seconds balances freshness with API server
	// load. Informers also receive real-time updates via watch connections.
	InformerResyncPeriod = 30 * time.Second
)
