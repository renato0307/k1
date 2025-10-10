package app

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/renato0307/k1/internal/commands"
	"github.com/renato0307/k1/internal/components"
	"github.com/renato0307/k1/internal/components/commandbar"
	"github.com/renato0307/k1/internal/k8s"
	"github.com/renato0307/k1/internal/screens"
	"github.com/renato0307/k1/internal/types"
	"github.com/renato0307/k1/internal/ui"
)

const (
	// StatusBarDisplayDuration is how long status messages (success, error,
	// info) are displayed before automatically clearing.
	StatusBarDisplayDuration = 5 * time.Second

	// MaxNavigationHistorySize limits the navigation history stack
	MaxNavigationHistorySize = 50

	// DisplayUpdateInterval is how often we update the display (spinner, refresh time)
	DisplayUpdateInterval = 100 * time.Millisecond
)

// displayTickMsg triggers display updates (spinner animation, refresh time)
type displayTickMsg time.Time

// NavigationState represents a point in navigation history
type NavigationState struct {
	ScreenID         string
	FilterContext    *types.FilterContext
	CommandBarFilter string // Fuzzy search filter from command bar
}

type Model struct {
	state             types.AppState
	registry          *types.ScreenRegistry
	currentScreen     types.Screen
	header            *components.Header
	layout            *components.Layout
	statusBar         *components.StatusBar
	commandBar        *commandbar.CommandBar
	fullScreen        *components.FullScreen
	fullScreenMode    bool
	navigationHistory []NavigationState
	repoPool          *k8s.RepositoryPool
	theme             *ui.Theme
}

func NewModel(pool *k8s.RepositoryPool, theme *ui.Theme) Model {
	registry := types.NewScreenRegistry()

	// Get active repository from pool
	repo := pool.GetActiveRepository()

	// Register all screens using config-driven approach
	// Tier 1: Critical (Pods)
	registry.Register(screens.NewConfigScreen(screens.GetPodsScreenConfig(), repo, theme))

	// Tier 2: Common resources
	registry.Register(screens.NewConfigScreen(screens.GetDeploymentsScreenConfig(), repo, theme))
	registry.Register(screens.NewConfigScreen(screens.GetServicesScreenConfig(), repo, theme))
	registry.Register(screens.NewConfigScreen(screens.GetConfigMapsScreenConfig(), repo, theme))
	registry.Register(screens.NewConfigScreen(screens.GetSecretsScreenConfig(), repo, theme))
	registry.Register(screens.NewConfigScreen(screens.GetNamespacesScreenConfig(), repo, theme))

	// Tier 3: Less common resources
	registry.Register(screens.NewConfigScreen(screens.GetStatefulSetsScreenConfig(), repo, theme))
	registry.Register(screens.NewConfigScreen(screens.GetDaemonSetsScreenConfig(), repo, theme))
	registry.Register(screens.NewConfigScreen(screens.GetJobsScreenConfig(), repo, theme))
	registry.Register(screens.NewConfigScreen(screens.GetCronJobsScreenConfig(), repo, theme))
	registry.Register(screens.NewConfigScreen(screens.GetNodesScreenConfig(), repo, theme))

	// Tier 1 (Phase 2): Additional high-value resources
	registry.Register(screens.NewConfigScreen(screens.GetReplicaSetsScreenConfig(), repo, theme))
	registry.Register(screens.NewConfigScreen(screens.GetPVCsScreenConfig(), repo, theme))
	registry.Register(screens.NewConfigScreen(screens.GetIngressesScreenConfig(), repo, theme))
	registry.Register(screens.NewConfigScreen(screens.GetEndpointsScreenConfig(), repo, theme))
	registry.Register(screens.NewConfigScreen(screens.GetHPAsScreenConfig(), repo, theme))

	// System screen
	registry.Register(screens.NewSystemScreen(repo, theme))

	// Contexts screen (special - uses pool directly)
	registry.Register(screens.NewConfigScreen(screens.GetContextsScreenConfig(), pool, theme))

	// Start with pods screen
	initialScreen, _ := registry.Get("pods")

	header := components.NewHeader("k1", theme)
	header.SetScreenTitle(initialScreen.Title())
	header.SetWidth(80)
	// Set initial refresh interval
	if configScreen, ok := initialScreen.(*screens.ConfigScreen); ok {
		header.SetRefreshInterval(configScreen.GetRefreshInterval())
	}

	cmdBar := commandbar.New(pool, theme)
	cmdBar.SetWidth(80)
	cmdBar.SetScreen("pods") // Set initial screen context

	statusBar := components.NewStatusBar(theme)
	statusBar.SetWidth(80)

	layout := components.NewLayout(80, 24, theme)
	layout.SetContext(pool.GetActiveContext()) // Set initial context on title line

	// Set initial size for the screen
	initialBodyHeight := layout.CalculateBodyHeightWithCommandBar(cmdBar.GetTotalHeight())
	if screenWithSize, ok := initialScreen.(interface{ SetSize(int, int) }); ok {
		screenWithSize.SetSize(80, initialBodyHeight)
	}

	return Model{
		state: types.AppState{
			CurrentScreen: "pods",
			Width:         80,
			Height:        24,
		},
		registry:          registry,
		currentScreen:     initialScreen,
		header:            header,
		layout:            layout,
		statusBar:         statusBar,
		commandBar:        cmdBar,
		navigationHistory: make([]NavigationState, 0, MaxNavigationHistorySize),
		repoPool:          pool,
		theme:             theme,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.currentScreen.Init(),
		tea.Tick(DisplayUpdateInterval, func(t time.Time) tea.Msg {
			return displayTickMsg(t)
		}),
	)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.state.Width = msg.Width
		m.state.Height = msg.Height
		m.layout.SetSize(msg.Width, msg.Height)
		m.header.SetWidth(msg.Width)
		m.statusBar.SetWidth(msg.Width)
		m.commandBar.SetWidth(msg.Width)

		bodyHeight := m.layout.CalculateBodyHeightWithCommandBar(m.commandBar.GetTotalHeight())
		if screenWithSize, ok := m.currentScreen.(interface{ SetSize(int, int) }); ok {
			screenWithSize.SetSize(msg.Width, bodyHeight)
		}

		return m, nil

	case tea.KeyMsg:
		// Handle global shortcuts
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "ctrl+r":
			// Global refresh - trigger current screen refresh
			if screen, ok := m.currentScreen.(interface{ Refresh() tea.Cmd }); ok {
				return m, screen.Refresh()
			}
			return m, nil
		case "ctrl+y":
			// Update selection context before executing command
			if screenWithSel, ok := m.currentScreen.(types.ScreenWithSelection); ok {
				m.commandBar.SetSelectedResource(screenWithSel.GetSelectedResource())
			}
			// Execute /yaml command
			updatedBar, barCmd := m.commandBar.ExecuteCommand("yaml", commands.CategoryAction)
			m.commandBar = updatedBar
			return m, barCmd
		case "ctrl+d":
			// Update selection context before executing command
			if screenWithSel, ok := m.currentScreen.(types.ScreenWithSelection); ok {
				m.commandBar.SetSelectedResource(screenWithSel.GetSelectedResource())
			}
			// Execute /describe command
			updatedBar, barCmd := m.commandBar.ExecuteCommand("describe", commands.CategoryAction)
			m.commandBar = updatedBar
			return m, barCmd
		case "ctrl+l":
			// Update selection context before executing command
			if screenWithSel, ok := m.currentScreen.(types.ScreenWithSelection); ok {
				m.commandBar.SetSelectedResource(screenWithSel.GetSelectedResource())
			}
			// Execute /logs command
			updatedBar, barCmd := m.commandBar.ExecuteCommand("logs", commands.CategoryAction)
			m.commandBar = updatedBar
			return m, barCmd
		case "ctrl+x":
			// Update selection context before executing command
			if screenWithSel, ok := m.currentScreen.(types.ScreenWithSelection); ok {
				m.commandBar.SetSelectedResource(screenWithSel.GetSelectedResource())
			}
			// Execute /delete command (will show confirmation)
			updatedBar, barCmd := m.commandBar.ExecuteCommand("delete", commands.CategoryAction)
			m.commandBar = updatedBar
			// Recalculate body height if command bar expanded for confirmation
			bodyHeight := m.layout.CalculateBodyHeightWithCommandBar(m.commandBar.GetTotalHeight())
			if screenWithSize, ok := m.currentScreen.(interface{ SetSize(int, int) }); ok {
				screenWithSize.SetSize(m.state.Width, bodyHeight)
			}
			return m, barCmd
		}

		// If in full-screen mode, handle ESC to return to list
		if m.fullScreenMode {
			if msg.String() == "esc" {
				return m.Update(types.ExitFullScreenMsg{})
			}
			// Forward to full-screen component
			updatedFS, fsCmd := m.fullScreen.Update(msg)
			m.fullScreen = updatedFS
			return m, fsCmd
		}

		// Handle ESC for back navigation (only when command bar is hidden)
		if msg.Type == tea.KeyEsc && m.commandBar.GetState() == commandbar.StateHidden {
			if len(m.navigationHistory) > 0 {
				return m, m.popNavigationHistory()
			}
		}

		// Update command bar with current selection context
		if screenWithSel, ok := m.currentScreen.(types.ScreenWithSelection); ok {
			m.commandBar.SetSelectedResource(screenWithSel.GetSelectedResource())
		}

		// Update command bar
		oldState := m.commandBar.GetState()
		updatedBar, barCmd := m.commandBar.Update(msg)
		m.commandBar = updatedBar

		// Recalculate body height if command bar height changed
		bodyHeight := m.layout.CalculateBodyHeightWithCommandBar(m.commandBar.GetTotalHeight())
		if screenWithSize, ok := m.currentScreen.(interface{ SetSize(int, int) }); ok {
			screenWithSize.SetSize(m.state.Width, bodyHeight)
		}

		// Forward to screen only if command bar is hidden or in filter mode
		// (In palette mode, arrows navigate palette not list)
		if oldState == commandbar.StateHidden || oldState == commandbar.StateFilter {
			model, screenCmd := m.currentScreen.Update(msg)
			m.currentScreen = model.(types.Screen)
			return m, tea.Batch(barCmd, screenCmd)
		}

		return m, barCmd

	case types.ScreenSwitchMsg:
		if screen, ok := m.registry.Get(msg.ScreenID); ok {
			// Push current state to history if this is contextual navigation
			// (FilterContext present and not a back navigation)
			if !msg.IsBackNav && msg.FilterContext != nil {
				m.pushNavigationHistory()
			}

			m.currentScreen = screen
			m.state.CurrentScreen = msg.ScreenID

			// Apply FilterContext (or clear it if nil)
			if configScreen, ok := screen.(*screens.ConfigScreen); ok {
				configScreen.ApplyFilterContext(msg.FilterContext)
			}

			// Update command bar with current screen context for command filtering
			m.commandBar.SetScreen(msg.ScreenID)

			// Update header with screen title
			m.header.SetScreenTitle(screen.Title())

			// Update header with refresh interval if screen is ConfigScreen
			if configScreen, ok := screen.(*screens.ConfigScreen); ok {
				m.header.SetRefreshInterval(configScreen.GetRefreshInterval())
			}

			// Update header with filter text if FilterContext is present
			if msg.FilterContext != nil {
				m.header.SetFilterText(msg.FilterContext.Description())
			} else {
				m.header.SetFilterText("")
			}

			// Restore command bar filter if this is back navigation
			var restoreFilterCmd tea.Cmd
			if msg.IsBackNav && msg.CommandBarFilter != "" {
				restoreFilterCmd = m.commandBar.RestoreFilter(msg.CommandBarFilter)
			}

			bodyHeight := m.layout.CalculateBodyHeightWithCommandBar(m.commandBar.GetTotalHeight())
			if screenWithSize, ok := m.currentScreen.(interface{ SetSize(int, int) }); ok {
				screenWithSize.SetSize(m.state.Width, bodyHeight)
			}

			return m, tea.Batch(screen.Init(), restoreFilterCmd)
		}

	case types.RefreshCompleteMsg:
		m.state.LastRefresh = time.Now()
		m.state.RefreshTime = msg.Duration
		m.header.SetLastRefresh(time.Now())
		return m, nil

	case types.StatusMsg:
		m.statusBar.SetMessage(msg.Message, msg.Type)
		return m, tea.Tick(StatusBarDisplayDuration, func(t time.Time) tea.Msg {
			return types.ClearStatusMsg{}
		})

	case types.ClearStatusMsg:
		m.statusBar.ClearMessage()
		return m, nil

	case displayTickMsg:
		// Update display elements (spinner, refresh time) without refreshing data
		m.header.AdvanceSpinner()

		// Update refresh time text (already formatted by GetRefreshTimeString)
		refreshTime := m.header.GetRefreshTimeString()
		m.header.SetRefreshText(refreshTime)

		// Schedule next display tick
		return m, tea.Tick(DisplayUpdateInterval, func(t time.Time) tea.Msg {
			return displayTickMsg(t)
		})

	case types.ContextLoadProgressMsg:
		// Check if loading is complete (Phase == 3 is PhaseComplete)
		if msg.Phase == 3 {
			// Clear loading state when complete
			m.header.SetContext(msg.Context)
		} else {
			// Show loading progress in header (spinner advances via displayTickMsg)
			m.header.SetContextLoading(msg.Context, msg.Message)
		}
		return m, nil

	case types.ContextLoadCompleteMsg:
		// Only clear loading spinner if this context was loading
		// Don't change the active context display - that only happens on ContextSwitchMsg
		currentContext := m.repoPool.GetActiveContext()
		if msg.Context == currentContext {
			// This is the active context finishing load - update display
			m.header.SetContext(msg.Context)
			m.layout.SetContext(msg.Context)
		}
		// Otherwise it's a background context - do nothing (keep current context displayed)
		return m, nil

	case types.ContextLoadFailedMsg:
		// Only show errors (critical feedback)
		m.statusBar.SetMessage(
			fmt.Sprintf("Failed to load context %s: %v", msg.Context, msg.Error),
			types.MessageTypeError,
		)
		return m, tea.Tick(StatusBarDisplayDuration, func(t time.Time) tea.Msg {
			return types.ClearStatusMsg{}
		})

	case types.ContextSwitchMsg:
		// Initiate context switch asynchronously
		return m, m.switchContextCmd(msg.ContextName)

	case types.ContextSwitchCompleteMsg:
		// Context switch completed - refresh current screen
		m.statusBar.SetMessage(
			fmt.Sprintf("Switched to context: %s", msg.NewContext),
			types.MessageTypeSuccess,
		)

		// Update header and layout with new context
		m.header.SetContext(msg.NewContext)
		m.layout.SetContext(msg.NewContext)

		// Special handling for contexts screen - navigate to pods after switching
		if m.currentScreen.ID() == "contexts" {
			// Re-register screens with new repository
			m.registry = types.NewScreenRegistry()
			m.initializeScreens()

			// Navigate to pods screen
			if screen, ok := m.registry.Get("pods"); ok {
				m.currentScreen = screen
				m.state.CurrentScreen = "pods"

				// Update command bar with pods screen context
				m.commandBar.SetScreen("pods")

				// Update header with screen title
				m.header.SetScreenTitle(screen.Title())
				m.header.SetFilterText("")

				// Update header with refresh interval
				if configScreen, ok := screen.(*screens.ConfigScreen); ok {
					m.header.SetRefreshInterval(configScreen.GetRefreshInterval())
				}

				// Trigger resize to fix table formatting
				bodyHeight := m.layout.CalculateBodyHeightWithCommandBar(m.commandBar.GetTotalHeight())
				if screenWithSize, ok := screen.(interface{ SetSize(int, int) }); ok {
					screenWithSize.SetSize(m.state.Width, bodyHeight)
				}

				return m, tea.Batch(
					screen.(interface{ Refresh() tea.Cmd }).Refresh(),
					tea.Tick(StatusBarDisplayDuration, func(t time.Time) tea.Msg {
						return types.ClearStatusMsg{}
					}),
				)
			}

			// Fallback if pods screen not found
			return m, tea.Tick(StatusBarDisplayDuration, func(t time.Time) tea.Msg {
				return types.ClearStatusMsg{}
			})
		}

		// Re-register screens with new repository
		m.registry = types.NewScreenRegistry()
		m.initializeScreens()

		// Switch to same screen type in new context
		if screen, ok := m.registry.Get(m.currentScreen.ID()); ok {
			m.currentScreen = screen
			m.header.SetScreenTitle(screen.Title())
		}

		bodyHeight := m.layout.CalculateBodyHeightWithCommandBar(m.commandBar.GetTotalHeight())
		if screenWithSize, ok := m.currentScreen.(interface{ SetSize(int, int) }); ok {
			screenWithSize.SetSize(m.state.Width, bodyHeight)
		}

		return m, tea.Batch(
			m.currentScreen.Init(),
			tea.Tick(StatusBarDisplayDuration, func(t time.Time) tea.Msg {
				return types.ClearStatusMsg{}
			}),
		)

	case types.ContextRetryMsg:
		// Retry failed context
		return m, m.retryContextCmd(msg.ContextName)

	case types.ShowFullScreenMsg:
		// Create full-screen view
		m.fullScreen = components.NewFullScreen(
			components.FullScreenViewType(msg.ViewType),
			msg.ResourceName,
			msg.Content,
			m.theme,
		)
		m.fullScreen.SetSize(m.state.Width, m.state.Height)
		m.fullScreenMode = true
		return m, nil

	case types.ExitFullScreenMsg:
		// Return to list view
		m.fullScreenMode = false
		m.fullScreen = nil
		return m, nil
	}

	// Forward messages to current screen
	var cmd tea.Cmd
	model, cmd := m.currentScreen.Update(msg)
	m.currentScreen = model.(types.Screen)
	return m, cmd
}

// pushNavigationHistory saves the current screen state to history
func (m *Model) pushNavigationHistory() {
	// Get current filter context if available
	var filterContext *types.FilterContext
	if configScreen, ok := m.currentScreen.(*screens.ConfigScreen); ok {
		filterContext = configScreen.GetFilterContext()
	}

	// Capture command bar filter (input may exist even if not in filter mode anymore,
	// since Enter key exits filter mode before we reach this code)
	commandBarFilter := m.commandBar.GetInput()

	state := NavigationState{
		ScreenID:         m.state.CurrentScreen,
		FilterContext:    filterContext,
		CommandBarFilter: commandBarFilter,
	}

	// Add to history, respecting max size
	if len(m.navigationHistory) >= MaxNavigationHistorySize {
		// Remove oldest entry
		m.navigationHistory = m.navigationHistory[1:]
	}
	m.navigationHistory = append(m.navigationHistory, state)
}

// popNavigationHistory returns a command to navigate to the previous screen
func (m *Model) popNavigationHistory() tea.Cmd {
	if len(m.navigationHistory) == 0 {
		return nil
	}

	// Pop the last state
	lastIdx := len(m.navigationHistory) - 1
	prevState := m.navigationHistory[lastIdx]
	m.navigationHistory = m.navigationHistory[:lastIdx]

	// Return navigation command with IsBackNav set to true
	return func() tea.Msg {
		return types.ScreenSwitchMsg{
			ScreenID:         prevState.ScreenID,
			FilterContext:    prevState.FilterContext,
			CommandBarFilter: prevState.CommandBarFilter,
			IsBackNav:        true,
		}
	}
}

func (m Model) View() string {
	// If in full-screen mode, show full-screen view instead of list
	if m.fullScreenMode && m.fullScreen != nil {
		return m.fullScreen.View()
	}

	// Build main layout
	header := m.header.View()
	body := m.currentScreen.View()
	statusBar := m.statusBar.View()
	commandBar := m.commandBar.View()
	paletteItems := m.commandBar.ViewPaletteItems()
	hints := m.commandBar.ViewHints()
	loadingText := m.header.GetLoadingText()

	baseView := m.layout.Render(header, body, statusBar, commandBar, paletteItems, hints, loadingText)

	// Return layout directly - it's already sized correctly via body height calculations
	return baseView
}

// switchContextCmd returns command to switch contexts asynchronously
func (m Model) switchContextCmd(contextName string) tea.Cmd {
	return func() tea.Msg {
		// Switch context (blocking operation, but runs in command goroutine)
		err := m.repoPool.SwitchContext(contextName, nil)

		if err != nil {
			return types.ContextLoadFailedMsg{
				Context: contextName,
				Error:   err,
			}
		}

		return types.ContextSwitchCompleteMsg{
			OldContext: m.repoPool.GetActiveContext(),
			NewContext: contextName,
		}
	}
}

// retryContextCmd returns command to retry failed context
func (m Model) retryContextCmd(contextName string) tea.Cmd {
	return func() tea.Msg {
		err := m.repoPool.RetryFailedContext(contextName, nil)

		if err != nil {
			return types.ContextLoadFailedMsg{
				Context: contextName,
				Error:   err,
			}
		}

		return types.ContextLoadCompleteMsg{
			Context: contextName,
		}
	}
}

// initializeScreens registers all screens with active repository
func (m *Model) initializeScreens() {
	repo := m.repoPool.GetActiveRepository()

	// Register all screens using config-driven approach
	// Tier 1: Critical (Pods)
	m.registry.Register(screens.NewConfigScreen(screens.GetPodsScreenConfig(), repo, m.theme))

	// Tier 2: Common resources
	m.registry.Register(screens.NewConfigScreen(screens.GetDeploymentsScreenConfig(), repo, m.theme))
	m.registry.Register(screens.NewConfigScreen(screens.GetServicesScreenConfig(), repo, m.theme))
	m.registry.Register(screens.NewConfigScreen(screens.GetConfigMapsScreenConfig(), repo, m.theme))
	m.registry.Register(screens.NewConfigScreen(screens.GetSecretsScreenConfig(), repo, m.theme))
	m.registry.Register(screens.NewConfigScreen(screens.GetNamespacesScreenConfig(), repo, m.theme))

	// Tier 3: Less common resources
	m.registry.Register(screens.NewConfigScreen(screens.GetStatefulSetsScreenConfig(), repo, m.theme))
	m.registry.Register(screens.NewConfigScreen(screens.GetDaemonSetsScreenConfig(), repo, m.theme))
	m.registry.Register(screens.NewConfigScreen(screens.GetJobsScreenConfig(), repo, m.theme))
	m.registry.Register(screens.NewConfigScreen(screens.GetCronJobsScreenConfig(), repo, m.theme))
	m.registry.Register(screens.NewConfigScreen(screens.GetNodesScreenConfig(), repo, m.theme))

	// Tier 1 (Phase 2): Additional high-value resources
	m.registry.Register(screens.NewConfigScreen(screens.GetReplicaSetsScreenConfig(), repo, m.theme))
	m.registry.Register(screens.NewConfigScreen(screens.GetPVCsScreenConfig(), repo, m.theme))
	m.registry.Register(screens.NewConfigScreen(screens.GetIngressesScreenConfig(), repo, m.theme))
	m.registry.Register(screens.NewConfigScreen(screens.GetEndpointsScreenConfig(), repo, m.theme))
	m.registry.Register(screens.NewConfigScreen(screens.GetHPAsScreenConfig(), repo, m.theme))

	// System screen
	m.registry.Register(screens.NewSystemScreen(repo, m.theme))

	// Contexts screen (special - uses pool directly)
	m.registry.Register(screens.NewConfigScreen(screens.GetContextsScreenConfig(), m.repoPool, m.theme))
}
