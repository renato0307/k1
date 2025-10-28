package k8s

import "time"

// Kubernetes client constants
const (
	// InformerResyncPeriod is how often informers resync their full cache from
	// the Kubernetes API server. 30 seconds balances freshness with API server
	// load. Informers also receive real-time updates via watch connections.
	InformerResyncPeriod = 30 * time.Second

	// InformerSyncTimeout is the timeout for initial cache sync of all typed
	// informers (pods, deployments, services). 120 seconds allows time for very
	// large clusters with thousands of resources and must be longer than API timeout.
	InformerSyncTimeout = 120 * time.Second

	// InformerIndividualSyncTimeout is the timeout for each dynamic informer
	// to sync individually. 60 seconds per resource allows graceful handling of
	// RBAC permission errors and large resource lists without blocking startup.
	InformerIndividualSyncTimeout = 60 * time.Second
)
