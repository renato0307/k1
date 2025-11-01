package screens

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/renato0307/k1/internal/k8s"
	"github.com/renato0307/k1/internal/logging"
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
	switch msg := msg.(type) {
	case startLoadingMsg:
		// Show loading message and start sync
		return s, tea.Batch(
			func() tea.Msg {
				return types.InfoMsg("Loading " + s.config.Title + "…")
			},
			s.Refresh(),
		)
	case tickMsg:
		// Ignore ticks from other screens (prevents multiple concurrent ticks)
		if msg.screenID != s.config.ID {
			logging.Debug("Ignoring tick from different screen", "tick_screen", msg.screenID, "current_screen", s.config.ID)
			return s, nil
		}
		// Handle periodic refresh - call DynamicScreen's Refresh(), not ConfigScreen's
		logging.Debug("Tick received, triggering refresh", "screen", s.config.Title)
		nextTick := tea.Tick(s.config.RefreshInterval, func(t time.Time) tea.Msg {
			return tickMsg{screenID: s.config.ID, time: t}
		})
		return s, tea.Batch(s.Refresh(), nextTick)
	case types.RefreshCompleteMsg:
		// After first refresh completes, schedule the first tick
		if !s.initialized {
			logging.Debug("First RefreshComplete, scheduling tick", "screen", s.config.Title, "interval", s.config.RefreshInterval)
			s.initialized = true
			nextTick := tea.Tick(s.config.RefreshInterval, func(t time.Time) tea.Msg {
				return tickMsg{screenID: s.config.ID, time: t}
			})
			// Let ConfigScreen handle the RefreshCompleteMsg, then schedule tick
			model, cmd := s.ConfigScreen.Update(msg)
			// Restore DynamicScreen wrapper
			s.ConfigScreen = model.(*ConfigScreen)
			return s, tea.Batch(cmd, nextTick)
		}
		// Non-first refresh complete - just delegate
		model, cmd := s.ConfigScreen.Update(msg)
		s.ConfigScreen = model.(*ConfigScreen)
		return s, cmd
	case types.StatusMsg:
		// Status message (loading, error, etc.) - schedule tick to retry/continue
		if !s.initialized {
			logging.Debug("First StatusMsg, scheduling tick", "screen", s.config.Title, "interval", s.config.RefreshInterval, "msg_type", msg.Type)
			s.initialized = true
			nextTick := tea.Tick(s.config.RefreshInterval, func(t time.Time) tea.Msg {
				return tickMsg{screenID: s.config.ID, time: t}
			})
			// Let ConfigScreen handle the StatusMsg, then schedule first tick
			model, cmd := s.ConfigScreen.Update(msg)
			// Restore DynamicScreen wrapper
			s.ConfigScreen = model.(*ConfigScreen)
			return s, tea.Batch(cmd, nextTick)
		}
		// Non-first status message - just delegate
		model, cmd := s.ConfigScreen.Update(msg)
		s.ConfigScreen = model.(*ConfigScreen)
		return s, cmd
	}

	// Delegate to ConfigScreen for all other messages
	model, cmd := s.ConfigScreen.Update(msg)
	// Restore DynamicScreen wrapper after delegation
	s.ConfigScreen = model.(*ConfigScreen)
	return s, cmd
}

// Refresh fetches CR instances using GVR
func (s *DynamicScreen) Refresh() tea.Cmd {
	return func() tea.Msg {
		// Ensure informer is registered on-demand (starts background sync if needed)
		if err := s.repo.EnsureCRInformer(s.gvr); err != nil {
			return types.ErrorStatusMsg("Failed to load " + s.config.Title + ": " + err.Error())
		}

		// Check if informer has synced yet (non-blocking check)
		if !s.repo.IsInformerSynced(s.gvr) {
			// Check if sync failed with an error
			if syncErr := s.repo.GetDynamicInformerSyncError(s.gvr); syncErr != nil {
				// Sync failed - show error to user
				return types.ErrorStatusMsg(syncErr.Error())
			}

			// Still syncing - show loading message (periodic refresh will retry)
			return types.LoadingMsg("Loading " + s.config.Title + "…")
		}

		// Informer ready - fetch data
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

// GetSelectedResource returns the selected resource with GVR metadata included
// This allows commands (like /yaml, /describe) to work with dynamic CRD instances
func (s *DynamicScreen) GetSelectedResource() map[string]interface{} {
	// Get base resource from ConfigScreen
	resource := s.ConfigScreen.GetSelectedResource()
	if resource == nil {
		return nil
	}

	// Add GVR metadata for command execution
	resource["__gvr_group"] = s.gvr.Group
	resource["__gvr_version"] = s.gvr.Version
	resource["__gvr_resource"] = s.gvr.Resource

	return resource
}
