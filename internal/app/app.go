package app

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/renato0307/k1/internal/commands"
	"github.com/renato0307/k1/internal/components"
	"github.com/renato0307/k1/internal/components/commandbar"
	"github.com/renato0307/k1/internal/screens"
	"github.com/renato0307/k1/internal/types"
)

const (
	// StatusBarDisplayDuration is how long status messages (success, error,
	// info) are displayed before automatically clearing.
	StatusBarDisplayDuration = 5 * time.Second

	// MaxNavigationDepth is the maximum depth of the navigation stack
	MaxNavigationDepth = 10
)

type Model struct {
	state           types.AppState
	registry        *types.ScreenRegistry
	currentScreen   types.Screen
	header          *components.Header
	layout          *components.Layout
	statusBar       *components.StatusBar
	commandBar      *commandbar.CommandBar
	fullScreen      *components.FullScreen
	fullScreenMode  bool
	ctx             *types.AppContext
	navigationStack []types.NavigationStackEntry
	navContext      *types.NavigationContext // Current navigation context
}

func NewModel(ctx *types.AppContext) Model {
	registry := types.NewScreenRegistry()

	// Register all screens using config-driven approach
	// Tier 1: Critical (Pods)
	registry.Register(screens.NewConfigScreen(ctx, screens.GetPodsScreenConfig()))

	// Tier 2: Common resources
	registry.Register(screens.NewConfigScreen(ctx, screens.GetDeploymentsScreenConfig()))
	registry.Register(screens.NewConfigScreen(ctx, screens.GetServicesScreenConfig()))
	registry.Register(screens.NewConfigScreen(ctx, screens.GetConfigMapsScreenConfig()))
	registry.Register(screens.NewConfigScreen(ctx, screens.GetSecretsScreenConfig()))
	registry.Register(screens.NewConfigScreen(ctx, screens.GetNamespacesScreenConfig()))

	// Tier 3: Less common resources
	registry.Register(screens.NewConfigScreen(ctx, screens.GetStatefulSetsScreenConfig()))
	registry.Register(screens.NewConfigScreen(ctx, screens.GetDaemonSetsScreenConfig()))
	registry.Register(screens.NewConfigScreen(ctx, screens.GetJobsScreenConfig()))
	registry.Register(screens.NewConfigScreen(ctx, screens.GetCronJobsScreenConfig()))
	registry.Register(screens.NewConfigScreen(ctx, screens.GetNodesScreenConfig()))

	// Start with pods screen
	initialScreen, _ := registry.Get("pods")

	header := components.NewHeader(ctx, "k1")
	header.SetScreenTitle(initialScreen.Title())
	header.SetWidth(80)

	cmdBar := commandbar.New(ctx)
	cmdBar.SetWidth(80)
	cmdBar.SetScreen("pods") // Set initial screen context

	statusBar := components.NewStatusBar(ctx)
	statusBar.SetWidth(80)

	layout := components.NewLayout(ctx, 80, 24)

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
		registry:        registry,
		currentScreen:   initialScreen,
		header:          header,
		layout:          layout,
		statusBar:       statusBar,
		commandBar:      cmdBar,
		ctx:             ctx,
		navigationStack: []types.NavigationStackEntry{},
		navContext:      nil,
	}
}

// pushNavigation adds a new entry to the navigation stack
func (m *Model) pushNavigation(screenID string, context *types.NavigationContext, scrollPos int) {
	// Check max depth
	if len(m.navigationStack) >= MaxNavigationDepth {
		// Remove oldest entry
		m.navigationStack = m.navigationStack[1:]
	}

	entry := types.NavigationStackEntry{
		ScreenID:  screenID,
		Context:   context,
		ScrollPos: scrollPos,
	}
	m.navigationStack = append(m.navigationStack, entry)
}

// popNavigation removes the last entry from the navigation stack
func (m *Model) popNavigation() *types.NavigationStackEntry {
	if len(m.navigationStack) == 0 {
		return nil
	}

	lastIdx := len(m.navigationStack) - 1
	entry := m.navigationStack[lastIdx]
	m.navigationStack = m.navigationStack[:lastIdx]
	return &entry
}

// hasNavigationHistory returns true if there's navigation history
func (m *Model) hasNavigationHistory() bool {
	return len(m.navigationStack) > 0
}

func (m Model) Init() tea.Cmd {
	return m.currentScreen.Init()
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
		case "ctrl+c":
			return m, tea.Quit
		case "q":
			// Check if we have navigation history, if so pop the stack
			if m.hasNavigationHistory() {
				return m.Update(types.NavigateBackMsg{})
			}
			return m, tea.Quit
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

		// Handle ESC to navigate back (if not handled by command bar)
		if msg.String() == "esc" && m.hasNavigationHistory() {
			// Check if command bar is visible - if so, let it handle ESC
			if m.commandBar.GetState() != commandbar.StateHidden {
				// Let command bar handle ESC (it will dismiss itself)
			} else {
				// Command bar is hidden, navigate back
				return m.Update(types.NavigateBackMsg{})
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
			m.currentScreen = screen
			m.state.CurrentScreen = msg.ScreenID

			// Update command bar with current screen context for command filtering
			m.commandBar.SetScreen(msg.ScreenID)

			// Update header with screen title
			m.header.SetScreenTitle(screen.Title())

			bodyHeight := m.layout.CalculateBodyHeightWithCommandBar(m.commandBar.GetTotalHeight())
			if screenWithSize, ok := m.currentScreen.(interface{ SetSize(int, int) }); ok {
				screenWithSize.SetSize(m.state.Width, bodyHeight)
			}

			return m, screen.Init()
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

	case types.ShowFullScreenMsg:
		// Create full-screen view
		m.fullScreen = components.NewFullScreen(
			m.ctx,
			components.FullScreenViewType(msg.ViewType),
			msg.ResourceName,
			msg.Content,
		)
		m.fullScreen.SetSize(m.state.Width, m.state.Height)
		m.fullScreenMode = true
		return m, nil

	case types.ExitFullScreenMsg:
		// Return to list view
		m.fullScreenMode = false
		m.fullScreen = nil
		return m, nil

	case types.NavigateMsg:
		// Save current screen state to navigation stack
		scrollPos := 0
		if screenWithSel, ok := m.currentScreen.(types.ScreenWithSelection); ok {
			// Try to get scroll position if available
			if screenWithScroll, ok := screenWithSel.(interface{ GetScrollPosition() int }); ok {
				scrollPos = screenWithScroll.GetScrollPosition()
			}
		}
		m.pushNavigation(m.state.CurrentScreen, m.navContext, scrollPos)

		// Navigate to new screen with context
		if screen, ok := m.registry.Get(msg.ScreenID); ok {
			m.currentScreen = screen
			m.state.CurrentScreen = msg.ScreenID
			m.navContext = &msg.Context

			// Update command bar with current screen context for command filtering
			m.commandBar.SetScreen(msg.ScreenID)

			// Update header with screen title and navigation context
			m.header.SetScreenTitle(screen.Title())
			m.header.SetNavigationContext(m.navContext)

			bodyHeight := m.layout.CalculateBodyHeightWithCommandBar(m.commandBar.GetTotalHeight())
			if screenWithSize, ok := m.currentScreen.(interface{ SetSize(int, int) }); ok {
				screenWithSize.SetSize(m.state.Width, bodyHeight)
			}

			return m, screen.Init()
		}

	case types.NavigateBackMsg:
		// Pop navigation stack and restore previous screen
		entry := m.popNavigation()
		if entry != nil {
			if screen, ok := m.registry.Get(entry.ScreenID); ok {
				m.currentScreen = screen
				m.state.CurrentScreen = entry.ScreenID
				m.navContext = entry.Context

				// Update command bar with current screen context
				m.commandBar.SetScreen(entry.ScreenID)

				// Update header with screen title and navigation context
				m.header.SetScreenTitle(screen.Title())
				m.header.SetNavigationContext(m.navContext)

				bodyHeight := m.layout.CalculateBodyHeightWithCommandBar(m.commandBar.GetTotalHeight())
				if screenWithSize, ok := m.currentScreen.(interface{ SetSize(int, int) }); ok {
					screenWithSize.SetSize(m.state.Width, bodyHeight)
				}

				// TODO: Restore scroll position if screen supports it
				// if screenWithScroll, ok := screen.(interface{ SetScrollPosition(int) }); ok {
				//     screenWithScroll.SetScrollPosition(entry.ScrollPos)
				// }

				return m, screen.Init()
			}
		}
		// If no history, just return
		return m, nil
	}

	// Forward messages to current screen
	var cmd tea.Cmd
	model, cmd := m.currentScreen.Update(msg)
	m.currentScreen = model.(types.Screen)
	return m, cmd
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

	baseView := m.layout.Render(header, body, statusBar, commandBar, paletteItems, hints)

	// Return layout directly - it's already sized correctly via body height calculations
	return baseView
}
