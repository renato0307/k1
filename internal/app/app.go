package app

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"timoneiro/internal/components"
	"timoneiro/internal/k8s"
	"timoneiro/internal/screens"
	"timoneiro/internal/types"
	"timoneiro/internal/ui"
)

type Model struct {
	state              types.AppState
	registry           *types.ScreenRegistry
	currentScreen      types.Screen
	header             *components.Header
	layout             *components.Layout
	commandBar         *components.CommandBar
	repo               k8s.Repository
	theme              *ui.Theme
}

func NewModel(repo k8s.Repository, theme *ui.Theme) Model {
	registry := types.NewScreenRegistry()

	// Register all screens with theme
	registry.Register(screens.NewPodsScreen(repo, theme))
	registry.Register(screens.NewDeploymentsScreen(repo, theme))
	registry.Register(screens.NewServicesScreen(repo, theme))

	// Start with pods screen
	initialScreen, _ := registry.Get("pods")

	header := components.NewHeader("Timoneiro", theme)
	header.SetScreenTitle(initialScreen.Title())
	header.SetWidth(80)

	commandBar := components.NewCommandBar(theme)
	commandBar.SetWidth(80)
	commandBar.SetScreen("pods") // Set initial screen context

	layout := components.NewLayout(80, 24)

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
		registry:       registry,
		currentScreen:  initialScreen,
		header:         header,
		layout:         layout,
		commandBar:     commandBar,
		repo:           repo,
		theme:          theme,
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

	case types.ErrorMsg:
		m.state.ErrorMessage = msg.Error
		return m, tea.Tick(3*time.Second, func(t time.Time) tea.Msg {
			return types.ClearErrorMsg{}
		})

	case types.ClearErrorMsg:
		m.state.ErrorMessage = ""
		return m, nil
	}

	// Forward messages to current screen
	var cmd tea.Cmd
	model, cmd := m.currentScreen.Update(msg)
	m.currentScreen = model.(types.Screen)
	return m, cmd
}

func (m Model) View() string {
	// Build main layout
	header := m.header.View()
	body := m.currentScreen.View()
	message := m.state.ErrorMessage
	commandBar := m.commandBar.View()
	paletteItems := m.commandBar.ViewPaletteItems()
	hints := m.commandBar.ViewHints()

	baseView := m.layout.Render(header, body, message, commandBar, paletteItems, hints)

	// Return layout directly - it's already sized correctly via body height calculations
	return baseView
}
