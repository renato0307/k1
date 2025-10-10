package k8s

import (
	"container/list"
	"fmt"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

// RepositoryStatus represents the state of a repository in the pool
type RepositoryStatus string

const (
	StatusNotLoaded RepositoryStatus = "Not Loaded"
	StatusLoading   RepositoryStatus = "Loading"
	StatusLoaded    RepositoryStatus = "Loaded"
	StatusFailed    RepositoryStatus = "Failed"
)

// RepositoryEntry wraps a repository with metadata
type RepositoryEntry struct {
	Repo       Repository // Changed from *InformerRepository to support testing
	Status     RepositoryStatus
	Error      error
	LoadedAt   time.Time
	ContextObj *ContextInfo // Parsed from kubeconfig
}

// RepositoryPool manages multiple Kubernetes contexts
type RepositoryPool struct {
	mu         sync.RWMutex
	repos      map[string]*RepositoryEntry
	active     string          // Current context name
	maxSize    int             // Pool size limit
	lru        *list.List      // LRU eviction order
	kubeconfig string
	contexts   []*ContextInfo  // All contexts from kubeconfig
}

// NewRepositoryPool creates a new repository pool
func NewRepositoryPool(kubeconfig string, maxSize int) (*RepositoryPool, error) {
	if maxSize <= 0 {
		maxSize = 10 // Default limit
	}

	// Parse kubeconfig to get all contexts
	contexts, err := parseKubeconfig(kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to parse kubeconfig: %w", err)
	}

	return &RepositoryPool{
		repos:      make(map[string]*RepositoryEntry),
		lru:        list.New(),
		maxSize:    maxSize,
		kubeconfig: kubeconfig,
		contexts:   contexts,
	}, nil
}

// LoadContext loads a context into the pool (blocking operation)
func (p *RepositoryPool) LoadContext(contextName string, progress chan<- ContextLoadProgress) error {
	p.mu.Lock()
	// Mark as loading
	if _, exists := p.repos[contextName]; !exists {
		p.repos[contextName] = &RepositoryEntry{
			Status: StatusLoading,
		}
	}
	p.mu.Unlock()

	// Report progress
	if progress != nil {
		progress <- ContextLoadProgress{
			Context: contextName,
			Message: "Connecting to API server...",
		}
	}

	// Create repository (5-15s operation)
	repo, err := NewInformerRepositoryWithProgress(p.kubeconfig, contextName, progress)

	p.mu.Lock()
	defer p.mu.Unlock()

	if err != nil {
		// Update existing entry with error
		if entry, ok := p.repos[contextName]; ok {
			entry.Status = StatusFailed
			entry.Error = err
		}
		return err
	}

	// Check pool size and evict if needed
	if len(p.repos) >= p.maxSize {
		p.evictLRU()
	}

	// Update existing entry with loaded repository
	if entry, ok := p.repos[contextName]; ok {
		entry.Repo = repo
		entry.Status = StatusLoaded
		entry.LoadedAt = time.Now()
		entry.Error = nil
	} else {
		// Create new entry if it doesn't exist (shouldn't happen, but defensive)
		p.repos[contextName] = &RepositoryEntry{
			Repo:     repo,
			Status:   StatusLoaded,
			LoadedAt: time.Now(),
		}
	}
	p.lru.PushFront(contextName)

	return nil
}

// GetActiveRepository returns the currently active repository
func (p *RepositoryPool) GetActiveRepository() Repository {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if entry, ok := p.repos[p.active]; ok && entry.Status == StatusLoaded {
		return entry.Repo
	}

	return nil // Should never happen if pool is initialized correctly
}

// GetActiveContext returns the name of the currently active context
func (p *RepositoryPool) GetActiveContext() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.active
}

// SwitchContext switches to a different context
func (p *RepositoryPool) SwitchContext(contextName string, progress chan<- ContextLoadProgress) error {
	p.mu.RLock()
	entry, exists := p.repos[contextName]
	p.mu.RUnlock()

	// Context already loaded - instant switch
	if exists && entry.Status == StatusLoaded {
		p.mu.Lock()
		p.active = contextName
		p.markUsed(contextName)
		p.mu.Unlock()
		return nil
	}

	// Load new context (blocking operation)
	if err := p.LoadContext(contextName, progress); err != nil {
		return err
	}

	// Switch to newly loaded context
	p.mu.Lock()
	p.active = contextName
	p.mu.Unlock()

	return nil
}

// GetAllContexts returns all contexts from kubeconfig with status
func (p *RepositoryPool) GetAllContexts() []ContextWithStatus {
	p.mu.RLock()
	defer p.mu.RUnlock()

	result := make([]ContextWithStatus, 0, len(p.contexts))
	for _, ctx := range p.contexts {
		status := StatusNotLoaded
		var err error

		if entry, ok := p.repos[ctx.Name]; ok {
			status = entry.Status
			err = entry.Error
		}

		result = append(result, ContextWithStatus{
			ContextInfo: ctx,
			Status:      status,
			Error:       err,
			IsCurrent:   ctx.Name == p.active,
		})
	}

	return result
}

// RetryFailedContext retries loading a failed context
func (p *RepositoryPool) RetryFailedContext(contextName string, progress chan<- ContextLoadProgress) error {
	p.mu.Lock()
	if entry, ok := p.repos[contextName]; ok {
		if entry.Status != StatusFailed {
			p.mu.Unlock()
			return fmt.Errorf("context %s is not in failed state", contextName)
		}
		delete(p.repos, contextName) // Remove failed entry
	}
	p.mu.Unlock()

	return p.LoadContext(contextName, progress)
}

// SetActive sets the active context without loading
func (p *RepositoryPool) SetActive(contextName string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if _, ok := p.repos[contextName]; !ok {
		return fmt.Errorf("context %s not loaded", contextName)
	}

	p.active = contextName
	p.markUsed(contextName)
	return nil
}

// Close closes all repositories in the pool
func (p *RepositoryPool) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, entry := range p.repos {
		if entry.Repo != nil {
			entry.Repo.Close()
		}
	}
}

// Repository interface delegation methods (delegate to active repository)

// GetResources delegates to active repository, except for contexts which are handled by the pool
func (p *RepositoryPool) GetResources(resourceType ResourceType) ([]any, error) {
	// Special handling for contexts - they come from the pool, not Kubernetes API
	if resourceType == ResourceTypeContext {
		contexts, err := p.GetContexts()
		if err != nil {
			return nil, err
		}
		result := make([]any, len(contexts))
		for i, ctx := range contexts {
			result[i] = ctx
		}
		return result, nil
	}

	// All other resources delegate to active repository
	repo := p.GetActiveRepository()
	if repo == nil {
		return nil, fmt.Errorf("no active repository")
	}
	return repo.GetResources(resourceType)
}

// GetPods delegates to active repository
func (p *RepositoryPool) GetPods() ([]Pod, error) {
	repo := p.GetActiveRepository()
	if repo == nil {
		return nil, fmt.Errorf("no active repository")
	}
	return repo.GetPods()
}

// GetDeployments delegates to active repository
func (p *RepositoryPool) GetDeployments() ([]Deployment, error) {
	repo := p.GetActiveRepository()
	if repo == nil {
		return nil, fmt.Errorf("no active repository")
	}
	return repo.GetDeployments()
}

// GetServices delegates to active repository
func (p *RepositoryPool) GetServices() ([]Service, error) {
	repo := p.GetActiveRepository()
	if repo == nil {
		return nil, fmt.Errorf("no active repository")
	}
	return repo.GetServices()
}

// GetPodsForDeployment delegates to active repository
func (p *RepositoryPool) GetPodsForDeployment(namespace, name string) ([]Pod, error) {
	repo := p.GetActiveRepository()
	if repo == nil {
		return nil, fmt.Errorf("no active repository")
	}
	return repo.GetPodsForDeployment(namespace, name)
}

// GetPodsOnNode delegates to active repository
func (p *RepositoryPool) GetPodsOnNode(nodeName string) ([]Pod, error) {
	repo := p.GetActiveRepository()
	if repo == nil {
		return nil, fmt.Errorf("no active repository")
	}
	return repo.GetPodsOnNode(nodeName)
}

// GetPodsForService delegates to active repository
func (p *RepositoryPool) GetPodsForService(namespace, name string) ([]Pod, error) {
	repo := p.GetActiveRepository()
	if repo == nil {
		return nil, fmt.Errorf("no active repository")
	}
	return repo.GetPodsForService(namespace, name)
}

// GetPodsForStatefulSet delegates to active repository
func (p *RepositoryPool) GetPodsForStatefulSet(namespace, name string) ([]Pod, error) {
	repo := p.GetActiveRepository()
	if repo == nil {
		return nil, fmt.Errorf("no active repository")
	}
	return repo.GetPodsForStatefulSet(namespace, name)
}

// GetPodsForDaemonSet delegates to active repository
func (p *RepositoryPool) GetPodsForDaemonSet(namespace, name string) ([]Pod, error) {
	repo := p.GetActiveRepository()
	if repo == nil {
		return nil, fmt.Errorf("no active repository")
	}
	return repo.GetPodsForDaemonSet(namespace, name)
}

// GetPodsForJob delegates to active repository
func (p *RepositoryPool) GetPodsForJob(namespace, name string) ([]Pod, error) {
	repo := p.GetActiveRepository()
	if repo == nil {
		return nil, fmt.Errorf("no active repository")
	}
	return repo.GetPodsForJob(namespace, name)
}

// GetJobsForCronJob delegates to active repository
func (p *RepositoryPool) GetJobsForCronJob(namespace, name string) ([]Job, error) {
	repo := p.GetActiveRepository()
	if repo == nil {
		return nil, fmt.Errorf("no active repository")
	}
	return repo.GetJobsForCronJob(namespace, name)
}

// GetPodsForNamespace delegates to active repository
func (p *RepositoryPool) GetPodsForNamespace(namespace string) ([]Pod, error) {
	repo := p.GetActiveRepository()
	if repo == nil {
		return nil, fmt.Errorf("no active repository")
	}
	return repo.GetPodsForNamespace(namespace)
}

// GetPodsUsingConfigMap delegates to active repository
func (p *RepositoryPool) GetPodsUsingConfigMap(namespace, name string) ([]Pod, error) {
	repo := p.GetActiveRepository()
	if repo == nil {
		return nil, fmt.Errorf("no active repository")
	}
	return repo.GetPodsUsingConfigMap(namespace, name)
}

// GetPodsUsingSecret delegates to active repository
func (p *RepositoryPool) GetPodsUsingSecret(namespace, name string) ([]Pod, error) {
	repo := p.GetActiveRepository()
	if repo == nil {
		return nil, fmt.Errorf("no active repository")
	}
	return repo.GetPodsUsingSecret(namespace, name)
}

// GetPodsForReplicaSet delegates to active repository
func (p *RepositoryPool) GetPodsForReplicaSet(namespace, name string) ([]Pod, error) {
	repo := p.GetActiveRepository()
	if repo == nil {
		return nil, fmt.Errorf("no active repository")
	}
	return repo.GetPodsForReplicaSet(namespace, name)
}

// GetReplicaSetsForDeployment delegates to active repository
func (p *RepositoryPool) GetReplicaSetsForDeployment(namespace, name string) ([]ReplicaSet, error) {
	repo := p.GetActiveRepository()
	if repo == nil {
		return nil, fmt.Errorf("no active repository")
	}
	return repo.GetReplicaSetsForDeployment(namespace, name)
}

// GetPodsForPVC delegates to active repository
func (p *RepositoryPool) GetPodsForPVC(namespace, name string) ([]Pod, error) {
	repo := p.GetActiveRepository()
	if repo == nil {
		return nil, fmt.Errorf("no active repository")
	}
	return repo.GetPodsForPVC(namespace, name)
}

// GetResourceYAML delegates to active repository
func (p *RepositoryPool) GetResourceYAML(gvr schema.GroupVersionResource, namespace, name string) (string, error) {
	repo := p.GetActiveRepository()
	if repo == nil {
		return "", fmt.Errorf("no active repository")
	}
	return repo.GetResourceYAML(gvr, namespace, name)
}

// DescribeResource delegates to active repository
func (p *RepositoryPool) DescribeResource(gvr schema.GroupVersionResource, namespace, name string) (string, error) {
	repo := p.GetActiveRepository()
	if repo == nil {
		return "", fmt.Errorf("no active repository")
	}
	return repo.DescribeResource(gvr, namespace, name)
}

// GetKubeconfig returns the kubeconfig path
func (p *RepositoryPool) GetKubeconfig() string {
	return p.kubeconfig
}

// GetContext returns the current context name (alias for GetActiveContext)
func (p *RepositoryPool) GetContext() string {
	return p.GetActiveContext()
}

// GetResourceStats delegates to active repository
func (p *RepositoryPool) GetResourceStats() []ResourceStats {
	repo := p.GetActiveRepository()
	if repo == nil {
		return []ResourceStats{}
	}
	return repo.GetResourceStats()
}

// GetContexts returns all contexts for display, sorted with loaded contexts first
func (p *RepositoryPool) GetContexts() ([]Context, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	allContexts := p.GetAllContexts()

	// Build result list with all contexts, sorted alphabetically for stable positions
	result := make([]Context, 0, len(allContexts))

	for _, ctx := range allContexts {
		current := ""
		if ctx.IsCurrent {
			current = "✓"
		}

		status := string(ctx.Status)
		errorMsg := ""
		if ctx.Error != nil {
			errorMsg = ctx.Error.Error()
		}

		var loadedAt time.Time
		if entry, ok := p.repos[ctx.Name]; ok {
			loadedAt = entry.LoadedAt
		}

		context := Context{
			Name:      ctx.Name,
			Cluster:   ctx.Cluster,
			User:      ctx.User,
			Namespace: ctx.Namespace,
			Status:    status,
			Current:   current,
			Error:     errorMsg,
			LoadedAt:  loadedAt,
		}

		result = append(result, context)
	}

	// Sort all contexts alphabetically - status changes won't affect position
	sortContextsByName(result)

	return result, nil
}

// sortContextsByName sorts contexts alphabetically by name
func sortContextsByName(contexts []Context) {
	for i := 0; i < len(contexts); i++ {
		for j := i + 1; j < len(contexts); j++ {
			if contexts[i].Name > contexts[j].Name {
				contexts[i], contexts[j] = contexts[j], contexts[i]
			}
		}
	}
}

// SetTestRepository manually sets a repository for testing (bypasses LoadContext)
// For tests only - allows injecting a dummy repository without connecting to API server
func (p *RepositoryPool) SetTestRepository(contextName string, repo Repository) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.repos[contextName] = &RepositoryEntry{
		Repo:     repo,
		LoadedAt: time.Now(),
		Status:   StatusLoaded,
	}
	p.active = contextName
	p.lru.PushFront(contextName)
}

// Private helper methods

// markUsed moves a context to the front of the LRU list
// Must be called with p.mu held
func (p *RepositoryPool) markUsed(contextName string) {
	// Move to front of LRU list
	for e := p.lru.Front(); e != nil; e = e.Next() {
		if e.Value.(string) == contextName {
			p.lru.MoveToFront(e)
			return
		}
	}
}

// evictLRU evicts the least recently used context
// Must be called with p.mu held
func (p *RepositoryPool) evictLRU() {
	if p.lru.Len() == 0 {
		return
	}

	// Get least recently used context
	back := p.lru.Back()
	if back == nil {
		return
	}

	contextName := back.Value.(string)

	// Don't evict active context
	if contextName == p.active {
		return
	}

	// Close and remove
	if entry, ok := p.repos[contextName]; ok {
		if entry.Repo != nil {
			entry.Repo.Close()
		}
		delete(p.repos, contextName)
	}
	p.lru.Remove(back)
}
