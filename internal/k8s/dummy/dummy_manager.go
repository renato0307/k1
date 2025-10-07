package dummy

// Manager provides fake kubeconfig for development
type Manager struct{}

func NewManager() *Manager {
	return &Manager{}
}

func (m *Manager) GetKubeconfig() string {
	return ""
}

func (m *Manager) GetContext() string {
	return ""
}

func (m *Manager) Close() {
	// No-op for dummy manager
}
