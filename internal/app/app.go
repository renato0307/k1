package app

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/renato0307/k1/internal/commands"
	"github.com/renato0307/k1/internal/components"
	"github.com/renato0307/k1/internal/k8s"
	"github.com/renato0307/k1/internal/screens"
	"github.com/renato0307/k1/internal/types"
	"github.com/renato0307/k1/internal/ui"
)

type Model struct {
	state          types.AppState
	registry       *types.ScreenRegistry
	currentScreen  types.Screen
	header         *components.Header
	layout         *components.Layout
	statusBar      *components.StatusBar
	commandBar     *components.CommandBar
	fullScreen     *components.FullScreen
	fullScreenMode bool
	repo           k8s.Repository
	theme          *ui.Theme
}

func NewModel(repo k8s.Repository, theme *ui.Theme) Model {
	registry := types.NewScreenRegistry()

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

	// Start with pods screen
	initialScreen, _ := registry.Get("pods")

	header := components.NewHeader("k1", theme)
	header.SetScreenTitle(initialScreen.Title())
	header.SetWidth(80)

	commandBar := components.NewCommandBar(repo, theme)
	commandBar.SetWidth(80)
	commandBar.SetScreen("pods") // Set initial screen context

	statusBar := components.NewStatusBar(theme)
	statusBar.SetWidth(80)

	layout := components.NewLayout(80, 24, theme)

	// Set initial size for the screen
	initialBodyHeight := layout.CalculateBodyHeightWithCommandBar(commandBar.GetTotalHeight())
	if screenWithSize, ok := initialScreen.(interface{ SetSize(int, int) }); ok {
		screenWithSize.SetSize(80, initialBodyHeight)
	}

	return Model{
		state: types.AppState{
			CurrentScreen: "pods",
			Width:         80,
			Height:        24,
		},
		registry:      registry,
		currentScreen: initialScreen,
		header:        header,
		layout:        layout,
		statusBar:     statusBar,
		commandBar:    commandBar,
		repo:          repo,
		theme:         theme,
	}
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
		case "ctrl+c", "q":
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
		if oldState == 0 || oldState == 1 { // StateHidden or StateFilter
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
		return m, tea.Tick(components.StatusBarDisplayDuration, func(t time.Time) tea.Msg {
			return types.ClearStatusMsg{}
		})

	case types.ClearStatusMsg:
		m.statusBar.ClearMessage()
		return m, nil

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
