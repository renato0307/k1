package k8s

import (
	"container/list"
	"fmt"
	"sync"
	"time"

	"github.com/renato0307/k1/internal/logging"
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

// loadingState tracks in-progress context loading operations
type loadingState struct {
	done chan struct{}
	err  error
}

// RepositoryPool manages multiple Kubernetes contexts
type RepositoryPool struct {
	mu         sync.RWMutex
	repos      map[string]*RepositoryEntry
	active     string     // Current context name
	maxSize    int        // Pool size limit
	lru        *list.List // LRU eviction order
	kubeconfig string
	contexts   []*ContextInfo // All contexts from kubeconfig
	loading    sync.Map       // map[string]*loadingState - coordinate concurrent loads
}

// NewRepositoryPool creates a new repository pool
func NewRepositoryPool(kubeconfig string, maxSize int) (*RepositoryPool, error) {
	timingCtx := logging.Start("NewRepositoryPool")
	defer logging.End(timingCtx)

	if maxSize <= 0 {
		maxSize = 10 // Default limit
	}

	// Parse kubeconfig to get all contexts
	kubeconfigStart := logging.Start("parse kubeconfig")
	contexts, err := parseKubeconfig(kubeconfig)
	logging.End(kubeconfigStart)

	if err != nil {
		logging.Error("Failed to parse kubeconfig", "error", err)
		return nil, fmt.Errorf("failed to parse kubeconfig: %w", err)
	}

	logging.Debug("Repository pool initialized", "context_count", len(contexts), "max_size", maxSize)

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
	loadCtx := logging.Start("LoadContext")
	defer logging.End(loadCtx)

	logging.Info("Loading context", "context", contextName)

	// Try to start loading - use LoadOrStore to coordinate concurrent attempts
	state := &loadingState{done: make(chan struct{})}
	actual, loaded := p.loading.LoadOrStore(contextName, state)

	if loaded {
		logging.Debug("Context already loading, waiting", "context", contextName)
		// Another goroutine is loading this context, wait for it
		<-actual.(*loadingState).done
		return actual.(*loadingState).err
	}

	// We're the loader - ensure cleanup
	defer func() {
		close(state.done)
		p.loading.Delete(contextName)
	}()

	// Mark as loading in repos map
	p.mu.Lock()
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
			Message: "Connecting to API server…",
		}
	}

	// Create repository (5-15s operation, no lock held)
	repoStart := logging.Start("NewInformerRepositoryWithProgress")
	repo, err := NewInformerRepositoryWithProgress(p.kubeconfig, contextName, progress)
	logging.End(repoStart)

	p.mu.Lock()
	defer p.mu.Unlock()

	if err != nil {
		logging.Error("Failed to create repository", "context", contextName, "error", err)
		// Cleanup partial repository if it exists
		if repo != nil {
			repo.Close()
		}
		// Update entry with error
		if entry, ok := p.repos[contextName]; ok {
			entry.Status = StatusFailed
			entry.Error = err
		}
		state.err = err
		return err
	}

	// Check pool size and evict if needed
	for len(p.repos) > p.maxSize {
		p.evictLRU()
	}

	// Update existing entry with loaded repository
	if entry, ok := p.repos[contextName]; ok {
		entry.Repo = repo
		entry.Status = StatusLoaded
		entry.LoadedAt = time.Now()
		entry.Error = nil
	} else {
		// Create new entry (defensive)
		p.repos[contextName] = &RepositoryEntry{
			Repo:     repo,
			Status:   StatusLoaded,
			LoadedAt: time.Now(),
		}
	}
	p.lru.PushFront(contextName)

	logging.Info("Context loaded successfully", "context", contextName)

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
	p.mu.Lock()
	entry, exists := p.repos[contextName]

	// Context already loaded - instant switch
	if exists && entry.Status == StatusLoaded {
		p.active = contextName
		p.markUsed(contextName)
		p.mu.Unlock()
		return nil
	}

	// Context not loaded or in error state
	p.mu.Unlock()

	// Load new context (blocking operation, no lock held)
	if err := p.LoadContext(contextName, progress); err != nil {
		return err
	}

	// Switch to newly loaded context
	p.mu.Lock()
	p.active = contextName
	p.markUsed(contextName) // Update LRU
	p.mu.Unlock()

	return nil
}

// MarkAsLoading marks a context as loading (for UI feedback)
func (p *RepositoryPool) MarkAsLoading(contextName string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if _, exists := p.repos[contextName]; !exists {
		p.repos[contextName] = &RepositoryEntry{
			Status: StatusLoading,
		}
	}
}

// GetAllContexts returns all contexts from kubeconfig with status
func (p *RepositoryPool) GetAllContexts() []ContextWithStatus {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.getAllContextsLocked()
}

// getAllContextsLocked returns all contexts (must be called with lock held)
func (p *RepositoryPool) getAllContextsLocked() []ContextWithStatus {
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

	// Close all repositories
	for _, entry := range p.repos {
		if entry.Repo != nil {
			entry.Repo.Close()
		}
	}

	// Clear all state
	p.repos = make(map[string]*RepositoryEntry)
	p.lru = list.New()
	p.active = ""
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

// EnsureCRInformer delegates to active repository
func (p *RepositoryPool) IsInformerSynced(gvr schema.GroupVersionResource) bool {
	repo := p.GetActiveRepository()
	if repo == nil {
		return false
	}
	return repo.IsInformerSynced(gvr)
}

func (p *RepositoryPool) AreTypedInformersReady() bool {
	repo := p.GetActiveRepository()
	if repo == nil {
		return false
	}
	return repo.AreTypedInformersReady()
}

func (p *RepositoryPool) GetTypedInformersSyncError() error {
	repo := p.GetActiveRepository()
	if repo == nil {
		return nil
	}
	return repo.GetTypedInformersSyncError()
}

func (p *RepositoryPool) GetDynamicInformerSyncError(gvr schema.GroupVersionResource) error {
	repo := p.GetActiveRepository()
	if repo == nil {
		return nil
	}
	return repo.GetDynamicInformerSyncError(gvr)
}

func (p *RepositoryPool) EnsureCRInformer(gvr schema.GroupVersionResource) error {
	repo := p.GetActiveRepository()
	if repo == nil {
		return fmt.Errorf("no active repository")
	}
	return repo.EnsureCRInformer(gvr)
}

func (p *RepositoryPool) EnsureResourceTypeInformer(resourceType ResourceType) error {
	repo := p.GetActiveRepository()
	if repo == nil {
		return fmt.Errorf("no active repository")
	}
	return repo.EnsureResourceTypeInformer(resourceType)
}

// GetResourcesByGVR delegates to active repository
func (p *RepositoryPool) GetResourcesByGVR(gvr schema.GroupVersionResource, transform TransformFunc) ([]any, error) {
	repo := p.GetActiveRepository()
	if repo == nil {
		return nil, fmt.Errorf("no active repository")
	}
	return repo.GetResourcesByGVR(gvr, transform)
}

// GetContexts returns all contexts for display, sorted with loaded contexts first
func (p *RepositoryPool) GetContexts() ([]Context, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	allContexts := p.getAllContextsLocked()

	// Build result list with all contexts
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

	// Sort: loaded/loading/failed first (alphabetically), then not-loaded (alphabetically)
	sortContextsByStatusThenName(result)

	return result, nil
}

// sortContextsByStatusThenName sorts contexts with loaded/loading/failed first (alphabetically within each group), then not-loaded (alphabetical)
func sortContextsByStatusThenName(contexts []Context) {
	// Define status priority: Loaded=0, Loading=1, Failed=2, NotLoaded=3
	statusPriority := map[string]int{
		string(StatusLoaded):    0,
		string(StatusLoading):   1,
		string(StatusFailed):    2,
		string(StatusNotLoaded): 3,
	}

	for i := 0; i < len(contexts); i++ {
		for j := i + 1; j < len(contexts); j++ {
			priorityI := statusPriority[contexts[i].Status]
			priorityJ := statusPriority[contexts[j].Status]

			// Sort by priority first
			if priorityI > priorityJ {
				contexts[i], contexts[j] = contexts[j], contexts[i]
			} else if priorityI == priorityJ {
				// Within same priority, sort alphabetically by name
				if contexts[i].Name > contexts[j].Name {
					contexts[i], contexts[j] = contexts[j], contexts[i]
				}
			}
		}
	}
}

// SetTestRepository manually sets a repository for testing (bypasses LoadContext)
// For tests only - allows injecting a dummy repository without connecting to API server
func (p *RepositoryPool) SetTestRepository(contextName string, repo Repository) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Initialize fields if not already done (for test use)
	if p.repos == nil {
		p.repos = make(map[string]*RepositoryEntry)
	}
	if p.lru == nil {
		p.lru = list.New()
	}

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
	// Not found - add to front (defensive repair for LRU corruption)
	p.lru.PushFront(contextName)
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
