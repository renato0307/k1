package app

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/rmhubbert/bubbletea-overlay"

	"timoneiro/internal/components"
	"timoneiro/internal/k8s"
	"timoneiro/internal/modals"
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
	screenPicker       *modals.ScreenPickerModal
	commandPalette     *modals.CommandPaletteModal
	repo               k8s.Repository
	filterInput        string
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

	return Model{
		state: types.AppState{
			CurrentScreen: "pods",
			Width:         80,
			Height:        24,
		},
		registry:       registry,
		currentScreen:  initialScreen,
		header:         header,
		layout:         components.NewLayout(80, 24),
		screenPicker:   modals.NewScreenPickerModal(registry.All()),
		commandPalette: modals.NewCommandPaletteModal(initialScreen.Operations()),
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

		bodyHeight := m.layout.CalculateBodyHeight()
		if screenWithSize, ok := m.currentScreen.(interface{ SetSize(int, int) }); ok {
			screenWithSize.SetSize(msg.Width, bodyHeight)
		}

		return m, nil

	case tea.KeyMsg:
		// Handle global shortcuts
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit

		case "ctrl+p":
			m.state.ShowCommandPalette = !m.state.ShowCommandPalette
			if m.state.ShowCommandPalette {
				m.commandPalette.UpdateOperations(m.currentScreen.Operations())
			}
			return m, nil

		case "ctrl+s":
			m.state.ShowScreenPicker = !m.state.ShowScreenPicker
			return m, nil

		case "/":
			if !m.state.FilterMode && !m.state.ShowCommandPalette && !m.state.ShowScreenPicker {
				m.state.FilterMode = true
				m.state.FilterText = ""
				return m, nil
			}

		case "esc":
			// Clear filter and exit filter mode
			if m.state.FilterMode || m.state.FilterText != "" {
				m.state.FilterMode = false
				m.state.FilterText = ""
				if screenWithFilter, ok := m.currentScreen.(interface{ SetFilter(string) }); ok {
					screenWithFilter.SetFilter("")
				}
				return m, nil
			}
			m.state.ShowCommandPalette = false
			m.state.ShowScreenPicker = false
			return m, nil

		case "enter":
			if m.state.FilterMode {
				m.state.FilterMode = false
				return m, nil
			}

		case "backspace":
			if m.state.FilterMode && len(m.state.FilterText) > 0 {
				m.state.FilterText = m.state.FilterText[:len(m.state.FilterText)-1]
				if screenWithFilter, ok := m.currentScreen.(interface{ SetFilter(string) }); ok {
					screenWithFilter.SetFilter(m.state.FilterText)
				}
				return m, nil
			}

		default:
			// Handle filter text input
			if m.state.FilterMode && len(msg.String()) == 1 {
				m.state.FilterText += msg.String()
				if screenWithFilter, ok := m.currentScreen.(interface{ SetFilter(string) }); ok {
					screenWithFilter.SetFilter(m.state.FilterText)
				}
				return m, nil
			}
		}

	case types.ScreenSwitchMsg:
		if screen, ok := m.registry.Get(msg.ScreenID); ok {
			m.currentScreen = screen
			m.state.CurrentScreen = msg.ScreenID
			m.state.ShowScreenPicker = false
			m.commandPalette.UpdateOperations(screen.Operations())

			// Update header with screen title
			m.header.SetScreenTitle(screen.Title())

			bodyHeight := m.layout.CalculateBodyHeight()
			if screenWithSize, ok := m.currentScreen.(interface{ SetSize(int, int) }); ok {
				screenWithSize.SetSize(m.state.Width, bodyHeight)
			}

			return m, screen.Init()
		}

	case types.RefreshCompleteMsg:
		m.state.LastRefresh = time.Now()
		m.state.RefreshTime = msg.Duration
		m.header.SetRefreshTime(msg.Duration)
		return m, nil

	case types.ErrorMsg:
		m.state.ErrorMessage = msg.Error
		return m, tea.Tick(3*time.Second, func(t time.Time) tea.Msg {
			return types.ClearErrorMsg{}
		})

	case types.ClearErrorMsg:
		m.state.ErrorMessage = ""
		return m, nil

	case types.ToggleScreenPickerMsg:
		m.state.ShowScreenPicker = !m.state.ShowScreenPicker
		// Reset and focus filter when opening
		if m.state.ShowScreenPicker {
			m.screenPicker.Init()
		}
		return m, nil

	case types.ToggleCommandPaletteMsg:
		m.state.ShowCommandPalette = !m.state.ShowCommandPalette
		// Reset and focus filter when opening
		if m.state.ShowCommandPalette {
			m.commandPalette.Init()
		}
		return m, nil
	}

	// Forward messages to modals or current screen
	if m.state.ShowScreenPicker {
		model, cmd := m.screenPicker.Update(msg)
		m.screenPicker = model.(*modals.ScreenPickerModal)
		return m, cmd
	}

	if m.state.ShowCommandPalette {
		model, cmd := m.commandPalette.Update(msg)
		m.commandPalette = model.(*modals.CommandPaletteModal)
		return m, cmd
	}

	var cmd tea.Cmd
	model, cmd := m.currentScreen.Update(msg)
	m.currentScreen = model.(types.Screen)
	return m, cmd
}

func (m Model) View() string {
	// Build main layout
	header := m.header.View()

	title := m.currentScreen.Title()

	// Show filter in title area with ESC hint
	var filterDisplay string
	if m.state.FilterMode {
		filterDisplay = "Filter: " + m.state.FilterText + "_ (ESC to clear)"
	} else if m.state.FilterText != "" {
		filterDisplay = "Filter: " + m.state.FilterText + " (ESC to clear)"
	}

	body := m.currentScreen.View()
	help := m.currentScreen.HelpText()
	message := m.state.ErrorMessage

	baseView := m.layout.Render(header, title, body, help, message, filterDisplay)

	// Render base view as full screen
	baseRendered := lipgloss.NewStyle().
		Width(m.state.Width).
		Height(m.state.Height).
		Render(baseView)

	// Overlay modals on top using overlay library
	if m.state.ShowScreenPicker {
		modalView := m.screenPicker.CenteredView(m.state.Width, m.state.Height)
		bg := &simpleModel{content: baseRendered}
		fg := &simpleModel{content: modalView}
		overlayModel := overlay.New(fg, bg, overlay.Center, overlay.Center, 0, 0)
		return overlayModel.View()
	}

	if m.state.ShowCommandPalette {
		modalView := m.commandPalette.CenteredView(m.state.Width, m.state.Height)
		bg := &simpleModel{content: baseRendered}
		fg := &simpleModel{content: modalView}
		overlayModel := overlay.New(fg, bg, overlay.Center, overlay.Center, 0, 0)
		return overlayModel.View()
	}

	return baseRendered
}

// simpleModel is a minimal tea.Model wrapper for static content
type simpleModel struct {
	content string
}

func (m *simpleModel) Init() tea.Cmd {
	return nil
}

func (m *simpleModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return m, nil
}

func (m *simpleModel) View() string {
	return m.content
}
