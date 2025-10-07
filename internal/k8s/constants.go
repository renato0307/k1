package k8s

import "time"

// Kubernetes client constants
const (
	// InformerResyncPeriod is how often informers resync their full cache from
	// the Kubernetes API server. 30 seconds balances freshness with API server
	// load. Informers also receive real-time updates via watch connections.
	InformerResyncPeriod = 30 * time.Second

	// InformerSyncTimeout is the timeout for initial cache sync of all typed
	// informers (pods, deployments, services). 10 seconds allows time for the
	// API server to respond while preventing excessive startup delays.
	InformerSyncTimeout = 10 * time.Second

	// InformerIndividualSyncTimeout is the timeout for each dynamic informer
	// to sync individually. 5 seconds per resource allows graceful handling of
	// RBAC permission errors without blocking startup.
	InformerIndividualSyncTimeout = 5 * time.Second
)
