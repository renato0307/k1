package screens

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/renato0307/k1/internal/k8s"
	"github.com/renato0307/k1/internal/types"
	"github.com/renato0307/k1/internal/ui"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// DynamicScreen wraps ConfigScreen for dynamic CRD instances
type DynamicScreen struct {
	*ConfigScreen
	gvr       schema.GroupVersionResource
	transform k8s.TransformFunc
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

// Init ensures informer is registered before loading data
func (s *DynamicScreen) Init() tea.Cmd {
	return tea.Batch(
		func() tea.Msg {
			// Register informer on-demand
			if err := s.repo.EnsureCRInformer(s.gvr); err != nil {
				return types.ErrorStatusMsg("Failed to load " + s.config.Title + ": " + err.Error())
			}
			return types.InfoMsg("Loading " + s.config.Title + "...")
		},
		s.Refresh(),
	)
}

// Refresh fetches CR instances using GVR
func (s *DynamicScreen) Refresh() tea.Cmd {
	return func() tea.Msg {
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
