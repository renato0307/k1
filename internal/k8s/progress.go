package k8s

// ContextLoadProgress reports loading progress for a context
type ContextLoadProgress struct {
	Context string
	Message string
	Phase   LoadPhase
}

// LoadPhase represents the current loading phase
type LoadPhase int

const (
	PhaseConnecting LoadPhase = iota
	PhaseSyncingCore
	PhaseSyncingDynamic
	PhaseComplete
)

// ContextWithStatus combines context info with runtime status
type ContextWithStatus struct {
	*ContextInfo
	Status    RepositoryStatus
	Error     error
	IsCurrent bool
}
