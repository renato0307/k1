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
	// to sync in parallel. 30 seconds allows handling RBAC errors and large
	// resource lists. Since informers sync in parallel, total startup time is
	// max(all individual syncs), not sum(all individual syncs).
	InformerIndividualSyncTimeout = 30 * time.Second
)
