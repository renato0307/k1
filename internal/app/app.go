package app

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/renato0307/k1/internal/commands"
	"github.com/renato0307/k1/internal/components"
	"github.com/renato0307/k1/internal/components/commandbar"
	"github.com/renato0307/k1/internal/k8s"
	"github.com/renato0307/k1/internal/keyboard"
	"github.com/renato0307/k1/internal/logging"
	"github.com/renato0307/k1/internal/messages"
	"github.com/renato0307/k1/internal/screens"
	"github.com/renato0307/k1/internal/types"
	"github.com/renato0307/k1/internal/ui"
)

const (
	// StatusBarDisplayDuration is how long success messages are displayed
	// before automatically clearing.
	StatusBarDisplayDuration = 5 * time.Second

	// ErrorMessageDisplayDuration is how long error messages are displayed
	// before automatically clearing (longer than success for readability).
	ErrorMessageDisplayDuration = 10 * time.Second

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
	userMessage       *components.UserMessage
	commandBar        *commandbar.CommandBar
	fullScreen        *components.FullScreen
	fullScreenMode    bool
	navigationHistory []NavigationState
	repoPool          *k8s.RepositoryPool
	theme             *ui.Theme
	messageID         int // Track current message to prevent old timers from clearing new messages
	outputBuffer      *components.OutputBuffer
	keys              *keyboard.Keys // Keyboard configuration
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
	registry.Register(screens.NewConfigScreen(screens.GetCRDsScreenConfig(), repo, theme))

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

	// Help screen
	registry.Register(screens.NewConfigScreen(screens.GetHelpScreenConfig(), repo, theme))

	// Output screen (special - uses outputBuffer)
	outputBuffer := components.NewOutputBuffer()
	registry.Register(screens.NewConfigScreen(screens.GetOutputScreenConfig(outputBuffer), pool, theme))

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

	keys := keyboard.GetKeys()

	cmdBar := commandbar.New(pool, theme, keys)
	cmdBar.SetWidth(80)
	cmdBar.SetScreen("pods") // Set initial screen context

	userMessage := components.NewUserMessage(theme)
	userMessage.SetWidth(80)

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
		userMessage:       userMessage,
		commandBar:        cmdBar,
		navigationHistory: make([]NavigationState, 0, MaxNavigationHistorySize),
		repoPool:          pool,
		theme:             theme,
		outputBuffer:      outputBuffer,
		keys:              keys,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.currentScreen.Init(),
		m.commandBar.Init(), // Start tip rotation
		tea.Tick(DisplayUpdateInterval, func(t time.Time) tea.Msg {
			return displayTickMsg(t)
		}),
	)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	// Route all messages to command bar for timer handling (including tipRotationMsg)
	// Key messages will be handled in their respective cases below (no double processing)
	if _, isKeyMsg := msg.(tea.KeyMsg); !isKeyMsg {
		updatedBar, barCmd := m.commandBar.Update(msg)
		m.commandBar = updatedBar
		if barCmd != nil {
			logging.Debug("App.Update: CommandBar returned cmd for non-key message",
				"msgType", fmt.Sprintf("%T", msg),
				"cmdsBefore", len(cmds))
			cmds = append(cmds, barCmd)
			logging.Debug("App.Update: Added cmd to batch",
				"cmdsAfter", len(cmds))
		}
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.state.Width = msg.Width
		m.state.Height = msg.Height
		m.layout.SetSize(msg.Width, msg.Height)
		m.header.SetWidth(msg.Width)
		m.userMessage.SetWidth(msg.Width)
		m.commandBar.SetWidth(msg.Width)

		bodyHeight := m.layout.CalculateBodyHeightWithCommandBar(m.commandBar.GetTotalHeight())
		if screenWithSize, ok := m.currentScreen.(interface{ SetSize(int, int) }); ok {
			screenWithSize.SetSize(msg.Width, bodyHeight)
		}

		return m, tea.Batch(cmds...)

	case tea.KeyMsg:
		logging.Debug("Key pressed", "key", msg.String(), "type", msg.Type, "currentScreen", m.currentScreen.ID())
		// If command bar is active (filter/palette mode), let it handle all keys first
		// This prevents global shortcuts from interfering with typing in filter mode
		if !m.commandBar.IsActive() {
			// Handle global shortcuts only when command bar is hidden
			switch msg.String() {
			case m.keys.Quit:
				return m, tea.Quit

			case m.keys.PrevContext:
				// Previous context ([)
				updatedBar, barCmd := m.commandBar.ExecuteCommand("prev-context", commands.CategoryResource)
				m.commandBar = updatedBar
				return m, barCmd

			case m.keys.NextContext:
				// Next context (])
				updatedBar, barCmd := m.commandBar.ExecuteCommand("next-context", commands.CategoryResource)
				m.commandBar = updatedBar
				return m, barCmd

			case m.keys.Refresh:
				// Global refresh - trigger current screen refresh
				if screen, ok := m.currentScreen.(interface{ Refresh() tea.Cmd }); ok {
					return m, screen.Refresh()
				}
				return m, nil

			case m.keys.NamespaceFilter:
				// Namespace filter (n)
				updatedBar, barCmd := m.commandBar.ExecuteCommand("ns", commands.CategoryResource)
				m.commandBar = updatedBar
				return m, barCmd

			case m.keys.Help:
				// Show help screen (?) - ignore if already on help screen
				if m.currentScreen.ID() != screens.HelpScreenID {
					return m.Update(types.ScreenSwitchMsg{
						ScreenID:    screens.HelpScreenID,
						PushHistory: true,
					})
				}
				return m, nil

			case m.keys.Down:
				// Vim navigation: j -> down arrow
				return m.Update(tea.KeyMsg{Type: tea.KeyDown})

			case m.keys.Up:
				// Vim navigation: k -> up arrow
				return m.Update(tea.KeyMsg{Type: tea.KeyUp})

			case m.keys.JumpTop:
				// Vim navigation: g -> jump to top
				model, cmd := m.currentScreen.Update(tea.KeyMsg{
					Type:  tea.KeyRunes,
					Runes: []rune{'g'},
				})
				m.currentScreen = model.(types.Screen)
				return m, cmd

			case m.keys.JumpBottom:
				// Vim navigation: G -> jump to bottom
				model, cmd := m.currentScreen.Update(tea.KeyMsg{
					Type:  tea.KeyRunes,
					Runes: []rune{'G'},
				})
				m.currentScreen = model.(types.Screen)
				return m, cmd
			}

			// Try to find command by shortcut dynamically (only when command bar is hidden)
			if cmd := m.commandBar.GetCommandByShortcut(msg.String()); cmd != nil {
				// Check if command is applicable to current screen's resource type
				if !m.isCommandApplicable(cmd) {
					// Don't execute command if not applicable to this resource type
					// Let the key pass through to the screen (for navigation, etc.)
				} else {
					// Update selection context before executing command
					if screenWithSel, ok := m.currentScreen.(types.ScreenWithSelection); ok {
						m.commandBar.SetSelectedResource(screenWithSel.GetSelectedResource())
					}

					// Execute command
					updatedBar, barCmd := m.commandBar.ExecuteCommand(cmd.Name, cmd.Category)
					m.commandBar = updatedBar

					// Recalculate body height if command bar expanded (e.g., for confirmation)
					bodyHeight := m.layout.CalculateBodyHeightWithCommandBar(m.commandBar.GetTotalHeight())
					if screenWithSize, ok := m.currentScreen.(interface{ SetSize(int, int) }); ok {
						screenWithSize.SetSize(m.state.Width, bodyHeight)
					}

					return m, barCmd
				}
			}
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

	case types.DynamicScreenCreateMsg:
		// Extract CRD
		crd, ok := msg.CRD.(k8s.CustomResourceDefinition)
		if !ok {
			return m, messages.ErrorCmd("Invalid CRD type")
		}

		// Generate screen config
		screenConfig := screens.GenerateScreenConfigForCR(crd)
		screenID := screenConfig.ID

		// Check if screen already registered
		if _, exists := m.registry.Get(screenID); !exists {
			// Construct GVR from CRD
			gvr := schema.GroupVersionResource{
				Group:    crd.Group,
				Version:  crd.Version,
				Resource: crd.Plural,
			}

			// Create generic transform with CRD columns for JSONPath evaluation
			transform := k8s.CreateGenericTransform(crd.Kind, crd.Columns)

			// Create and register dynamic screen
			dynamicScreen := screens.NewDynamicScreen(
				screenConfig,
				gvr,
				transform,
				m.repoPool.GetActiveRepository(),
				m.theme,
			)
			m.registry.Register(dynamicScreen)
		}

		// Use ScreenSwitchMsg to handle all the switching logic
		// PushHistory=true so ESC goes back to CRD list
		return m.Update(types.ScreenSwitchMsg{
			ScreenID:    screenID,
			PushHistory: true,
		})

	case types.ScreenSwitchMsg:
		if screen, ok := m.registry.Get(msg.ScreenID); ok {
			// Push current state to history if:
			// 1. PushHistory flag is set (explicit request), OR
			// 2. FilterContext present and not a back navigation (contextual nav)
			if (!msg.IsBackNav && msg.FilterContext != nil) || msg.PushHistory {
				m.pushNavigationHistory()
			}

			// Clear any sticky error/status messages when switching screens
			m.userMessage.ClearMessage()

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

			cmds = append(cmds, screen.Init())
			if restoreFilterCmd != nil {
				cmds = append(cmds, restoreFilterCmd)
			}
			return m, tea.Batch(cmds...)
		}

	case types.RefreshCompleteMsg:
		m.state.LastRefresh = time.Now()
		m.state.RefreshTime = msg.Duration
		m.header.SetLastRefresh(time.Now())
		// Update item count in header if screen is ConfigScreen
		if configScreen, ok := m.currentScreen.(*screens.ConfigScreen); ok {
			m.header.SetItemCount(configScreen.GetItemCount())
		}
		// Forward to screen so it can schedule first tick for periodic refresh
		// Only clear loading messages (preserve success/error/info messages)
		if m.userMessage.IsLoadingMessage() {
			m.messageID++ // Invalidate any loading message timer
			m.userMessage.ClearMessage()
		}
		model, cmd := m.currentScreen.Update(msg)
		m.currentScreen = model.(types.Screen)
		cmds = append(cmds, cmd)
		return m, tea.Batch(cmds...)

	case types.StatusMsg:
		// Capture command output for history (always, even for silent messages)
		if msg.TrackInHistory && msg.HistoryMetadata != nil {
			entry := components.CommandOutput{
				Command:        msg.HistoryMetadata.Command,
				KubectlCommand: msg.HistoryMetadata.KubectlCommand,
				Output:         msg.Message, // Full message (before truncation)
				Status:         messageTypeToStatus(msg.Type),
				Context:        msg.HistoryMetadata.Context,
				ResourceType:   string(msg.HistoryMetadata.ResourceType),
				ResourceName:   msg.HistoryMetadata.ResourceName,
				Namespace:      msg.HistoryMetadata.Namespace,
				Timestamp:      msg.HistoryMetadata.Timestamp,
				Duration:       msg.HistoryMetadata.Duration,
			}
			m.outputBuffer.Add(entry)

			// Debug logging to verify history tracking (remove after Phase 2)
			logging.Info("History entry added",
				"command", entry.Command,
				"context", entry.Context,
				"status", entry.Status,
				"duration", entry.Duration.String(),
				"buffer_count", m.outputBuffer.Count(),
			)
		}

		// If silent, skip showing message to user
		if msg.Silent {
			// Forward to screen so it can schedule periodic refresh if needed
			model, cmd := m.currentScreen.Update(msg)
			m.currentScreen = model.(types.Screen)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
			return m, tea.Batch(cmds...)
		}

		// Show message to user
		m.messageID++ // Increment to invalidate any pending clear timers
		currentID := m.messageID

		// Log error messages for debugging (preserves full message before truncation)
		if msg.Type == types.MessageTypeError {
			logging.Error("User error message", "message", msg.Message)
		}

		m.userMessage.SetMessage(msg.Message, msg.Type)

		// Forward to screen so it can schedule periodic refresh if needed
		model, cmd := m.currentScreen.Update(msg)
		m.currentScreen = model.(types.Screen)

		// Start spinner for loading messages
		spinnerCmd := m.userMessage.GetSpinnerCmd()

		// Auto-clear success and error messages with appropriate durations
		// Loading messages persist until RefreshCompleteMsg clears them
		// Info messages persist until user takes action
		if msg.Type == types.MessageTypeSuccess || msg.Type == types.MessageTypeError {
			duration := StatusBarDisplayDuration // 5s for success
			if msg.Type == types.MessageTypeError {
				duration = ErrorMessageDisplayDuration // 10s for errors
			}
			statusCmd := tea.Tick(duration, func(t time.Time) tea.Msg {
				return types.ClearStatusMsg{MessageID: currentID}
			})
			cmds = append(cmds, cmd, statusCmd, spinnerCmd)
			return m, tea.Batch(cmds...)
		}
		cmds = append(cmds, cmd, spinnerCmd)
		return m, tea.Batch(cmds...)

	case types.ClearStatusMsg:
		// Only clear if this timer belongs to the current message
		if msg.MessageID == m.messageID {
			m.userMessage.ClearMessage()
		}
		return m, tea.Batch(cmds...)

	case displayTickMsg:
		// Update display elements (refresh time) without refreshing data
		m.header.AdvanceSpinner()

		// Update refresh time text (already formatted by GetRefreshTimeString)
		refreshTime := m.header.GetRefreshTimeString()
		m.header.SetRefreshText(refreshTime)

		// Schedule next display tick
		tickCmd := tea.Tick(DisplayUpdateInterval, func(t time.Time) tea.Msg {
			return displayTickMsg(t)
		})
		cmds = append(cmds, tickCmd)
		return m, tea.Batch(cmds...)

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
		m.messageID++
		currentID := m.messageID
		m.userMessage.SetMessage(
			fmt.Sprintf("Failed to load context %s: %v", msg.Context, msg.Error),
			types.MessageTypeError,
		)
		return m, tea.Tick(StatusBarDisplayDuration, func(t time.Time) tea.Msg {
			return types.ClearStatusMsg{MessageID: currentID}
		})

	case types.ContextSwitchMsg:
		// Check if context is already loaded
		contexts, err := m.repoPool.GetContexts()
		if err != nil {
			return m, messages.ErrorCmd("Failed to get contexts: %v", err)
		}

		// Find the target context
		var needsLoading bool
		for _, ctx := range contexts {
			if ctx.Name == msg.ContextName {
				needsLoading = (ctx.Status != string(k8s.StatusLoaded))
				break
			}
		}

		// If needs loading, show immediate feedback
		if needsLoading {
			// Mark context as loading BEFORE refresh so UI shows it
			m.repoPool.MarkAsLoading(msg.ContextName)

			// Show info message (doesn't auto-clear)
			infoCmd := messages.InfoCmd("Loading context %s…", msg.ContextName)
			// Refresh current screen to show updated status
			if m.currentScreen.ID() == "contexts" {
				refreshCmd := m.currentScreen.(*screens.ConfigScreen).Refresh()
				return m, tea.Batch(infoCmd, refreshCmd, m.switchContextCmd(msg.ContextName))
			}
			// Initiate context switch asynchronously with info message
			return m, tea.Batch(infoCmd, m.switchContextCmd(msg.ContextName))
		}

		// Initiate context switch asynchronously
		return m, m.switchContextCmd(msg.ContextName)

	case types.ContextSwitchCompleteMsg:
		// Context switch completed
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

				return m, screen.(interface{ Refresh() tea.Cmd }).Refresh()
			}

			// Fallback if pods screen not found
			return m, nil
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

		return m, m.currentScreen.Init()

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

	// Forward messages to status bar for spinner tick handling
	updatedUserMessage, userMessageCmd := m.userMessage.Update(msg)
	m.userMessage = updatedUserMessage
	if userMessageCmd != nil {
		cmds = append(cmds, userMessageCmd)
	}

	// Forward messages to current screen
	var screenCmd tea.Cmd
	model, screenCmd := m.currentScreen.Update(msg)
	m.currentScreen = model.(types.Screen)
	if screenCmd != nil {
		cmds = append(cmds, screenCmd)
	}

	logging.Debug("App.Update: End of function, returning batch",
		"totalCmds", len(cmds))

	return m, tea.Batch(cmds...)
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
	logging.Debug("App.View rendering", "screen", m.currentScreen.ID(), "headerLen", len(header), "bodyLen", len(body))
	userMessage := m.userMessage.View()
	commandBar := m.commandBar.View()
	paletteItems := m.commandBar.ViewPaletteItems()
	hints := m.commandBar.ViewHints()
	loadingText := m.header.GetLoadingText()

	baseView := m.layout.Render(header, body, userMessage, commandBar, paletteItems, hints, loadingText)

	// Return layout directly - it's already sized correctly via body height calculations
	return baseView
}

// switchContextCmd returns command to switch contexts asynchronously
func (m Model) switchContextCmd(contextName string) tea.Cmd {
	return func() tea.Msg {
		start := time.Now() // Track start time for history
		oldContext := m.repoPool.GetActiveContext() // Capture BEFORE switch
		err := m.repoPool.SwitchContext(contextName, nil)

		// Build history metadata
		metadata := &types.CommandMetadata{
			Command:        fmt.Sprintf("Load context %s", contextName),
			KubectlCommand: "", // Context switching doesn't use kubectl
			Context:        contextName,
			Duration:       time.Since(start),
			Timestamp:      time.Now(),
		}

		if err != nil {
			// Return error message with history tracking
			errMsg := messages.ErrorCmd("Failed to load context %s: %v", contextName, err)
			return messages.WithHistory(errMsg, metadata)()
		}

		// Success - return both ContextSwitchCompleteMsg and success message with history
		switchComplete := types.ContextSwitchCompleteMsg{
			OldContext: oldContext,
			NewContext: contextName,
		}
		successMsg := messages.SuccessCmd("Loaded context %s", contextName)

		return tea.Batch(
			func() tea.Msg { return switchComplete },
			messages.WithHistory(successMsg, metadata),
		)()
	}
}

// retryContextCmd returns command to retry failed context
func (m Model) retryContextCmd(contextName string) tea.Cmd {
	return func() tea.Msg {
		start := time.Now() // Track start time for history
		oldContext := m.repoPool.GetActiveContext() // Capture BEFORE retry
		err := m.repoPool.RetryFailedContext(contextName, nil)

		// Build history metadata
		metadata := &types.CommandMetadata{
			Command:        fmt.Sprintf("Retry context %s", contextName),
			KubectlCommand: "", // Context switching doesn't use kubectl
			Context:        contextName,
			Duration:       time.Since(start),
			Timestamp:      time.Now(),
		}

		if err != nil {
			// Return error message with history tracking
			errMsg := messages.ErrorCmd("Failed to retry context %s: %v", contextName, err)
			return messages.WithHistory(errMsg, metadata)()
		}

		// Success - return both ContextSwitchCompleteMsg and success message with history
		switchComplete := types.ContextSwitchCompleteMsg{
			OldContext: oldContext,
			NewContext: contextName,
		}
		successMsg := messages.SuccessCmd("Retried context %s", contextName)

		return tea.Batch(
			func() tea.Msg { return switchComplete },
			messages.WithHistory(successMsg, metadata),
		)()
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
	m.registry.Register(screens.NewConfigScreen(screens.GetCRDsScreenConfig(), repo, m.theme))

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

	// Help screen
	m.registry.Register(screens.NewConfigScreen(screens.GetHelpScreenConfig(), repo, m.theme))

	// Output screen (special - uses outputBuffer from model)
	m.registry.Register(screens.NewConfigScreen(screens.GetOutputScreenConfig(m.outputBuffer), m.repoPool, m.theme))

	// Contexts screen (special - uses pool directly)
	m.registry.Register(screens.NewConfigScreen(screens.GetContextsScreenConfig(), m.repoPool, m.theme))
}

// isCommandApplicable checks if a command is applicable to the current screen's resource type
func (m Model) isCommandApplicable(cmd *commands.Command) bool {
	// Non-action commands (navigation, etc.) are always applicable
	if cmd.Category != commands.CategoryAction {
		return true
	}

	// Get current screen's resource type
	var currentResourceType k8s.ResourceType
	if screenWithResourceType, ok := m.currentScreen.(interface{ GetResourceType() k8s.ResourceType }); ok {
		currentResourceType = screenWithResourceType.GetResourceType()
	} else {
		// Screen doesn't have a resource type (e.g., SystemScreen) - not applicable
		return false
	}

	// Empty ResourceTypes means command applies to all resource types
	if len(cmd.ResourceTypes) == 0 {
		// But still need to check if this is a real K8s resource, not help/system/output/contexts
		nonK8sResources := map[k8s.ResourceType]bool{
			k8s.ResourceType("help"):     true,
			k8s.ResourceType("system"):   true,
			k8s.ResourceType("output"):   true,
			k8s.ResourceType("contexts"): true,
		}
		return !nonK8sResources[currentResourceType]
	}

	// Check if current resource type is in the command's ResourceTypes list
	for _, rt := range cmd.ResourceTypes {
		if rt == currentResourceType {
			return true
		}
	}

	return false
}

// messageTypeToStatus converts MessageType to status string for history
func messageTypeToStatus(t types.MessageType) string {
	switch t {
	case types.MessageTypeSuccess:
		return "success"
	case types.MessageTypeError:
		return "error"
	case types.MessageTypeInfo:
		return "info"
	case types.MessageTypeLoading:
		return "loading"
	default:
		return "unknown"
	}
}
