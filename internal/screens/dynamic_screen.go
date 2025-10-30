package screens

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/renato0307/k1/internal/k8s"
	"github.com/renato0307/k1/internal/types"
	"github.com/renato0307/k1/internal/ui"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// startLoadingMsg triggers the loading sequence
type startLoadingMsg struct{}

// DynamicScreen wraps ConfigScreen for dynamic CRD instances
type DynamicScreen struct {
	*ConfigScreen
	gvr         schema.GroupVersionResource
	transform   k8s.TransformFunc
	initialized bool // Track if Init() has been called before
}

// NewDynamicScreen creates a screen for CR instances
func NewDynamicScreen(
	config ScreenConfig,
	gvr schema.GroupVersionResource,
	transform k8s.TransformFunc,
	repo k8s.Repository,
	theme *ui.Theme) *DynamicScreen {

	baseScreen := NewConfigScreen(config, repo, theme)

	return &DynamicScreen{
		ConfigScreen: baseScreen,
		gvr:          gvr,
		transform:    transform,
	}
}

// Init starts the data refresh, showing loading message only on first call
func (s *DynamicScreen) Init() tea.Cmd {
	// If already initialized before (cached screen being revisited), just refresh
	if s.initialized {
		return s.Refresh()
	}

	// First initialization
	s.initialized = true

	// Check if informer needs syncing
	if s.repo.IsInformerSynced(s.gvr) {
		// Already synced (shouldn't happen on first init, but handle it)
		return s.Refresh()
	}

	// Informer needs syncing - show loading message before blocking
	return func() tea.Msg {
		return startLoadingMsg{}
	}
}

// Update handles messages for DynamicScreen
func (s *DynamicScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg.(type) {
	case startLoadingMsg:
		// Show loading message and start sync
		return s, tea.Batch(
			func() tea.Msg {
				return types.InfoMsg("Loading " + s.config.Title + "...")
			},
			s.Refresh(),
		)
	case types.RefreshCompleteMsg:
		// Let ConfigScreen handle the refresh complete
		// App will clear loading message automatically
		return s.ConfigScreen.Update(msg)
	}

	// Delegate to ConfigScreen for all other messages
	return s.ConfigScreen.Update(msg)
}

// Refresh fetches CR instances using GVR
func (s *DynamicScreen) Refresh() tea.Cmd {
	return func() tea.Msg {
		// Ensure informer is registered on-demand (may take 10-30s first time)
		if err := s.repo.EnsureCRInformer(s.gvr); err != nil {
			return types.ErrorStatusMsg("Failed to load " + s.config.Title + ": " + err.Error())
		}

		resources, err := s.repo.GetResourcesByGVR(s.gvr, s.transform)
		if err != nil {
			return types.ErrorStatusMsg("Failed to refresh " + s.config.Title + ": " + err.Error())
		}

		// Update items directly (following ConfigScreen pattern)
		s.items = resources
		s.applyFilter()

		return types.RefreshCompleteMsg{}
	}
}
