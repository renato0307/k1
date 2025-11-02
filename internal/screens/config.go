package screens

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/renato0307/k1/internal/k8s"
	"github.com/renato0307/k1/internal/logging"
	"github.com/renato0307/k1/internal/types"
	"github.com/renato0307/k1/internal/ui"
	"github.com/sahilm/fuzzy"
)

// tickMsg triggers periodic refresh for a specific screen
type tickMsg struct {
	screenID string
	time     time.Time
}

// startConfigLoadingMsg triggers the loading sequence for ConfigScreen
type startConfigLoadingMsg struct{}

// ColumnConfig defines a column in the resource list table
type ColumnConfig struct {
	Field    string                   // Field name in resource struct
	Title    string                   // Column display title

	// Width fields (backward compatible):
	// NEW: Set MinWidth/MaxWidth/Weight for weighted distribution
	// OLD: Set Width for fixed width (backward compatible during migration)
	Width    int     // DEPRECATED: 0 = dynamic, >0 = fixed
	MinWidth int     // NEW: Minimum width (readability)
	MaxWidth int     // NEW: Maximum width (prevent domination)
	Weight   float64 // NEW: Growth weight (higher = more space)

	Format   func(interface{}) string // Optional custom formatter
	Priority int                      // 1=critical, 2=important, 3=optional
}

// OperationConfig defines an operation that can be executed
type OperationConfig struct {
	ID          string
	Name        string
	Description string
	Shortcut    string
}

// NavigationFunc defines a function that handles Enter key navigation for a screen
type NavigationFunc func(screen *ConfigScreen) tea.Cmd

// ScreenConfig defines configuration for a generic resource screen
type ScreenConfig struct {
	ID           string
	Title        string
	ResourceType k8s.ResourceType
	Columns      []ColumnConfig
	SearchFields []string
	Operations   []OperationConfig

	// Optional behavior flags
	EnablePeriodicRefresh bool
	RefreshInterval       time.Duration
	TrackSelection        bool

	// Optional navigation handler (contextual navigation on Enter key)
	NavigationHandler NavigationFunc

	// Optional custom overrides (Level 2 customization)
	CustomRefresh func(*ConfigScreen) tea.Cmd
	CustomFilter  func(*ConfigScreen, string)
	CustomUpdate  func(*ConfigScreen, tea.Msg) (tea.Model, tea.Cmd)
	CustomView    func(*ConfigScreen) string
}

// ConfigScreen is a generic screen implementation driven by ScreenConfig
type ConfigScreen struct {
	config   ScreenConfig
	repo     k8s.Repository
	table    table.Model
	items    []interface{}
	filtered []interface{}
	filter   string
	theme    *ui.Theme
	width    int
	height   int

	// For selection tracking (if enabled)
	selectedKey string

	// For contextual navigation filtering
	filterContext *types.FilterContext

	// Column visibility tracking (Phase 2: responsive display)
	visibleColumns []ColumnConfig // Columns currently visible
	hiddenCount    int            // Number of hidden columns

	// Track initialization for loading messages and periodic refresh
	initialized bool
}

// NewConfigScreen creates a new config-driven screen
func NewConfigScreen(cfg ScreenConfig, repo k8s.Repository, theme *ui.Theme) *ConfigScreen {
	// Build table columns from config
	columns := make([]table.Column, len(cfg.Columns))
	for i, col := range cfg.Columns {
		columns[i] = table.Column{
			Title: col.Title,
			Width: col.Width,
		}
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithFocused(true),
		table.WithHeight(10),
	)
	t.SetStyles(theme.ToTableStyles())

	return &ConfigScreen{
		config:         cfg,
		repo:           repo,
		table:          t,
		theme:          theme,
		visibleColumns: cfg.Columns, // Initialize with all columns
		hiddenCount:    0,
	}
}

// Implement Screen interface

func (s *ConfigScreen) ID() string {
	return s.config.ID
}

func (s *ConfigScreen) Title() string {
	return s.config.Title
}

func (s *ConfigScreen) HelpText() string {
	return "↑/↓: navigate • type: filter • esc: clear filter • q: quit"
}

func (s *ConfigScreen) Operations() []types.Operation {
	ops := make([]types.Operation, len(s.config.Operations))
	for i, opCfg := range s.config.Operations {
		ops[i] = types.Operation{
			ID:          opCfg.ID,
			Name:        opCfg.Name,
			Description: opCfg.Description,
			Shortcut:    opCfg.Shortcut,
			Execute:     s.makeOperationHandler(opCfg),
		}
	}
	return ops
}

func (s *ConfigScreen) Init() tea.Cmd {
	// Reset initialized flag to allow fresh tick scheduling
	// This prevents multiple concurrent ticks from previous screen visits
	s.initialized = false
	// Always refresh, but Refresh() will handle loading state
	return s.Refresh()
}

// needsInitialLoad checks if this screen needs to show loading on first visit
func (s *ConfigScreen) needsInitialLoad() bool {
	// Safety check for tests
	if s.repo == nil {
		return false
	}

	// Check if this resource uses typed informers (Pods, Deployments, Services, StatefulSets, DaemonSets)
	usesTypedInformer := s.config.ResourceType == k8s.ResourceTypePod ||
		s.config.ResourceType == k8s.ResourceTypeDeployment ||
		s.config.ResourceType == k8s.ResourceTypeService ||
		s.config.ResourceType == k8s.ResourceTypeStatefulSet ||
		s.config.ResourceType == k8s.ResourceTypeDaemonSet

	if usesTypedInformer {
		// Check if typed informers are ready (they sync in background now)
		return !s.repo.AreTypedInformersReady()
	}

	// For dynamic informers, check tier
	config, exists := k8s.GetResourceConfig(s.config.ResourceType)
	if !exists {
		return false
	}

	// Only Tier 0 (on-demand) resources need loading messages
	// Tier 1/2/3 dynamic resources are already loaded at startup
	if config.Tier != 0 {
		return false
	}

	// Check if informer is already synced (reuse config from above)
	return !s.repo.IsInformerSynced(config.GVR)
}

func (s *ConfigScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Log all messages for debugging
	logging.Debug("ConfigScreen.Update received message", "screen", s.config.Title, "msg_type", fmt.Sprintf("%T", msg))

	// Handle loading message first (before custom update)
	switch msg.(type) {
	case startConfigLoadingMsg:
		// Show loading message and start refresh
		return s, tea.Batch(
			func() tea.Msg {
				return types.LoadingMsg("Loading " + s.config.Title + "…")
			},
			s.Refresh(),
		)
	}

	// Check for custom update handler
	if s.config.CustomUpdate != nil {
		logging.Debug("Calling CustomUpdate", "screen", s.config.Title)
		return s.config.CustomUpdate(s, msg)
	}

	logging.Debug("Calling DefaultUpdate", "screen", s.config.Title)
	return s.DefaultUpdate(msg)
}

func (s *ConfigScreen) DefaultUpdate(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case types.RefreshCompleteMsg:
		// Don't restore cursor when filter is active - cursor should be at position 0
		if s.config.TrackSelection && s.filter == "" {
			s.restoreCursorPosition()
		}
		return s, nil

	case types.FilterUpdateMsg:
		s.SetFilter(msg.Filter)
		// Return RefreshCompleteMsg to trigger item count update
		return s, func() tea.Msg {
			return types.RefreshCompleteMsg{Duration: 0}
		}

	case types.ClearFilterMsg:
		s.SetFilter("")
		// Reset cursor to first row when clearing filter
		if len(s.filtered) > 0 {
			s.table.SetCursor(0)
			// Update tracked selection to row 0 so restoreCursorPosition doesn't undo this
			if s.config.TrackSelection {
				s.updateSelectedKey()
			}
		}
		// Return RefreshCompleteMsg to trigger item count update
		return s, func() tea.Msg {
			return types.RefreshCompleteMsg{Duration: 0}
		}

	case tea.KeyMsg:
		// Intercept Enter for contextual navigation
		if msg.Type == tea.KeyEnter {
			if cmd := s.handleEnterKey(); cmd != nil {
				return s, cmd
			}
		}

		// Track if this is a page navigation key
		isPageNav := msg.Type == tea.KeyPgUp || msg.Type == tea.KeyPgDown

		var cmd tea.Cmd
		s.table, cmd = s.table.Update(msg)

		// After page navigation, ensure cursor is visible
		if isPageNav {
			s.ensureCursorVisible()
		}

		if s.config.TrackSelection {
			s.updateSelectedKey()
		}
		return s, cmd

	case tea.WindowSizeMsg:
		s.SetSize(msg.Width, msg.Height)
		return s, nil
	}

	var cmd tea.Cmd
	s.table, cmd = s.table.Update(msg)
	return s, cmd
}

func (s *ConfigScreen) View() string {
	if s.config.CustomView != nil {
		return s.config.CustomView(s)
	}

	// Check if we have a filter context and no results
	if s.filterContext != nil && len(s.table.Rows()) == 0 {
		return s.renderEmptyFilteredView()
	}

	return s.table.View()
}

// renderEmptyFilteredView shows a helpful message when filter returns no results
func (s *ConfigScreen) renderEmptyFilteredView() string {
	// Create styled message
	titleStyle := lipgloss.NewStyle().
		Foreground(s.theme.Muted).
		Bold(true).
		Align(lipgloss.Center)

	hintStyle := lipgloss.NewStyle().
		Foreground(s.theme.Muted).
		Align(lipgloss.Center)

	title := titleStyle.Render("No resources found")
	hint := hintStyle.Render("Press ESC to go back")

	// Center content vertically
	content := lipgloss.JoinVertical(
		lipgloss.Center,
		"",
		title,
		"",
		hint,
	)

	// Center horizontally and vertically in available space
	return lipgloss.Place(
		s.width,
		s.height,
		lipgloss.Center,
		lipgloss.Center,
		content,
	)
}

// SetSize updates dimensions and recalculates dynamic column widths
func (s *ConfigScreen) SetSize(width, height int) {
	s.width = width
	s.height = height
	s.table.SetHeight(height)

	// 1. Determine visible columns (use existing Priority logic)
	visibleColumns := s.calculateVisibleColumns(width)
	s.visibleColumns = visibleColumns
	s.hiddenCount = len(s.config.Columns) - len(visibleColumns)

	// 2. Calculate widths using weighted distribution
	widths := s.calculateWeightedWidths(visibleColumns, width)

	// 3. Build table columns
	columns := make([]table.Column, len(visibleColumns))
	for i, col := range visibleColumns {
		columns[i] = table.Column{
			Title: col.Title,
			Width: widths[i],
		}
	}

	// 4. Update table
	// Clear rows BEFORE setting columns to prevent panic
	// SetColumns() internally triggers rendering, so we need empty rows first
	s.table.SetRows([]table.Row{})
	s.table.SetColumns(columns)
	s.table.SetWidth(width)

	// Now rebuild rows with correct number of columns
	s.updateTable()
}

// calculateWeightedWidths implements weighted proportional distribution
// with min/max constraints. Returns final widths for each visible column.
func (s *ConfigScreen) calculateWeightedWidths(
	visible []ColumnConfig,
	terminalWidth int,
) []int {
	// 1. Check for backward compatibility (old Width-based configs)
	usesWeights := false
	for _, col := range visible {
		if col.Weight > 0 {
			usesWeights = true
			break
		}
	}

	// Fall back to old algorithm if no weights specified
	if !usesWeights {
		return s.calculateLegacyWidths(visible, terminalWidth)
	}

	// 2. Calculate padding
	padding := len(visible) * 2
	available := terminalWidth - padding
	if available <= 0 {
		// Fallback: equal distribution
		minWidths := make([]int, len(visible))
		for i := range visible {
			minWidths[i] = 10 // Emergency minimum
		}
		return minWidths
	}

	// 3. Allocate minimums
	totalMin := 0
	for _, col := range visible {
		totalMin += col.MinWidth
	}

	remaining := available - totalMin
	if remaining < 0 {
		// Not enough space for minimums, return minimums anyway
		widths := make([]int, len(visible))
		for i, col := range visible {
			widths[i] = col.MinWidth
		}
		return widths
	}

	// 4. Distribute remaining space using Largest Remainder Method
	// This guarantees full width usage by handling integer truncation properly
	widths := make([]int, len(visible))
	for i, col := range visible {
		widths[i] = col.MinWidth
	}

	if remaining <= 0 {
		return widths
	}

	// Calculate total weight
	totalWeight := 0.0
	for _, col := range visible {
		totalWeight += col.Weight
	}

	if totalWeight == 0 {
		// No weights specified, distribute equally
		perColumn := remaining / len(visible)
		extra := remaining % len(visible)
		for i := range widths {
			widths[i] += perColumn
			if i < extra {
				widths[i] += 1
			}
		}
		return widths
	}

	// Phase 1: Calculate integer shares and track fractional remainders
	type allocation struct {
		index     int
		allocated int
		remainder float64
	}

	allocations := make([]allocation, len(visible))
	totalAllocated := 0

	for i, col := range visible {
		ideal := float64(remaining) * (col.Weight / totalWeight)
		integer := int(ideal)
		remainder := ideal - float64(integer)

		allocations[i] = allocation{
			index:     i,
			allocated: integer,
			remainder: remainder,
		}
		totalAllocated += integer
	}

	// Phase 2: Distribute leftover pixels to columns with largest remainders
	leftover := remaining - totalAllocated

	// Sort by remainder (descending) to prioritize columns that lost most to truncation
	sort.Slice(allocations, func(i, j int) bool {
		return allocations[i].remainder > allocations[j].remainder
	})

	// Give 1 pixel to top N columns (where N = leftover)
	for i := 0; i < leftover && i < len(allocations); i++ {
		allocations[i].allocated += 1
	}

	// Apply allocations to widths
	for _, alloc := range allocations {
		widths[alloc.index] += alloc.allocated
	}

	return widths
}

// calculateLegacyWidths implements old Width-based algorithm for backward
// compatibility. DEPRECATED: Will be removed after all screens migrate.
func (s *ConfigScreen) calculateLegacyWidths(
	visible []ColumnConfig,
	terminalWidth int,
) []int {
	// Replicate old algorithm from SetSize() (lines 358-377)
	fixedTotal := 0
	dynamicCount := 0

	for _, col := range visible {
		if col.Width > 0 {
			fixedTotal += col.Width
		} else {
			dynamicCount++
		}
	}

	// Recalculate padding for visible columns only
	visiblePadding := len(visible) * 2
	dynamicWidth := 20 // Default minimum
	if dynamicCount > 0 {
		dynamicWidth = (terminalWidth - fixedTotal - visiblePadding) / dynamicCount
		if dynamicWidth < 20 {
			dynamicWidth = 20
		}
	}

	widths := make([]int, len(visible))
	for i, col := range visible {
		if col.Width > 0 {
			widths[i] = col.Width
		} else {
			widths[i] = dynamicWidth
		}
	}

	return widths
}

// calculateVisibleColumns determines which columns fit based on Priority.
// Reuses existing logic from SetSize() but extracted for clarity.
func (s *ConfigScreen) calculateVisibleColumns(
	terminalWidth int,
) []ColumnConfig {
	// Calculate padding
	padding := len(s.config.Columns) * 2
	availableWidth := terminalWidth - padding

	// Sort columns by priority (1 first, then 2, then 3)
	sorted := make([]ColumnConfig, len(s.config.Columns))
	copy(sorted, s.config.Columns)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].Priority < sorted[j].Priority
	})

	// Calculate which columns fit
	visibleColumns := []ColumnConfig{}
	usedWidth := 0

	for _, col := range sorted {
		// Use MinWidth if available, otherwise estimate
		estimatedWidth := col.MinWidth
		if estimatedWidth == 0 {
			if col.Width > 0 {
				estimatedWidth = col.Width
			} else {
				estimatedWidth = 20 // Legacy estimate for dynamic
			}
		}

		exclude := s.shouldExcludeColumn(col, availableWidth, usedWidth, estimatedWidth)
		if exclude {
			continue
		}

		visibleColumns = append(visibleColumns, col)
		usedWidth += estimatedWidth
	}

	// Restore original column order
	return s.restoreColumnOrder(visibleColumns)
}

// shouldExcludeColumn determines if a column should be hidden based on
// priority and available width. Now accepts estimatedWidth parameter.
func (s *ConfigScreen) shouldExcludeColumn(
	col ColumnConfig,
	availableWidth int,
	usedWidth int,
	estimatedWidth int,
) bool {
	// Priority 1 (critical) always shows, even if squished
	if col.Priority == 1 {
		return false
	}

	// Priority 2 and 3 only show if they fit
	return usedWidth+estimatedWidth > availableWidth
}

// restoreColumnOrder restores the original column order after sorting by
// priority. This ensures columns appear in the same order as defined in
// screen config, not sorted by priority.
func (s *ConfigScreen) restoreColumnOrder(visible []ColumnConfig) []ColumnConfig {
	result := []ColumnConfig{}

	// Iterate original config order
	for _, original := range s.config.Columns {
		// Check if this column is in visible list
		for _, v := range visible {
			if v.Field == original.Field {
				result = append(result, v)
				break
			}
		}
	}

	return result
}

// Refresh fetches resources and updates the table
func (s *ConfigScreen) Refresh() tea.Cmd {
	if s.config.CustomRefresh != nil {
		return s.config.CustomRefresh(s)
	}

	return func() tea.Msg {
		start := time.Now()

		// Check if this is a Tier 0 (on-demand) resource
		config, exists := k8s.GetResourceConfig(s.config.ResourceType)
		isTier0 := exists && config.Tier == 0

		// For Tier 0 resources, trigger loading if not started
		if isTier0 {
			if err := s.repo.EnsureResourceTypeInformer(s.config.ResourceType); err != nil {
				logging.Error("EnsureResourceTypeInformer failed", "screen", s.config.Title, "error", err)
				return types.ErrorStatusMsg(fmt.Sprintf("Failed to load %s informer: %v", s.config.Title, err))
			}
		}

		// Check if informer is synced (non-blocking check)
		ready := s.isInformerReady()
		logging.Debug("Screen refresh", "screen", s.config.Title, "ready", ready)

		if !ready {
			// Check if sync failed with an error
			if syncErr := s.getInformerSyncError(); syncErr != nil {
				// Sync failed - show error to user
				logging.Error("Informer sync failed", "screen", s.config.Title, "error", syncErr)
				return types.ErrorStatusMsg(fmt.Sprintf("Cluster connection error: %v", syncErr))
			}

			// Informers not synced yet - show loading message
			// Periodic refresh will retry automatically
			logging.Debug("Still loading", "screen", s.config.Title)
			return types.LoadingMsg(fmt.Sprintf("Loading %s…", s.config.Title))
		}

		var items []interface{}
		var err error

		// Use filtered repository methods if FilterContext is set
		if s.filterContext != nil {
			items, err = s.refreshWithFilterContext()
		} else {
			items, err = s.repo.GetResources(s.config.ResourceType)
		}

		if err != nil {
			logging.Error("GetResources failed", "screen", s.config.Title, "error", err)
			return types.ErrorStatusMsg(fmt.Sprintf("Failed to fetch %s: %v", s.config.Title, err))
		}

		logging.Debug("Screen refresh complete", "screen", s.config.Title, "items", len(items), "duration", time.Since(start))
		s.items = items
		s.applyFilter()
		logging.Debug("After filter", "screen", s.config.Title, "filtered_items", len(s.filtered))

		return types.RefreshCompleteMsg{Duration: time.Since(start)}
	}
}

// waitForInformers blocks until the informers for this screen are synced
func (s *ConfigScreen) waitForInformers() error {
	// Check if this resource uses typed informers
	usesTypedInformer := s.config.ResourceType == k8s.ResourceTypePod ||
		s.config.ResourceType == k8s.ResourceTypeDeployment ||
		s.config.ResourceType == k8s.ResourceTypeService ||
		s.config.ResourceType == k8s.ResourceTypeStatefulSet ||
		s.config.ResourceType == k8s.ResourceTypeDaemonSet

	if usesTypedInformer {
		// Wait for typed informers with timeout
		timeout := time.After(30 * time.Second)
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-timeout:
				return fmt.Errorf("timeout waiting for %s informer to sync", s.config.Title)
			case <-ticker.C:
				if s.repo.AreTypedInformersReady() {
					return nil
				}
			}
		}
	}

	// For dynamic informers, use existing EnsureResourceTypeInformer
	// (it already handles the wait)
	return nil
}

// isInformerReady checks if the informer for this screen's resource type is synced
func (s *ConfigScreen) isInformerReady() bool {
	// Safety check for tests
	if s.repo == nil {
		return true
	}

	// Check if this resource uses typed informers (Pods, Deployments, Services, StatefulSets, DaemonSets)
	usesTypedInformer := s.config.ResourceType == k8s.ResourceTypePod ||
		s.config.ResourceType == k8s.ResourceTypeDeployment ||
		s.config.ResourceType == k8s.ResourceTypeService ||
		s.config.ResourceType == k8s.ResourceTypeStatefulSet ||
		s.config.ResourceType == k8s.ResourceTypeDaemonSet

	if usesTypedInformer {
		// Check typed informer sync status
		return s.repo.AreTypedInformersReady()
	}

	// For dynamic informers, check if lister is registered (means sync completed)
	config, ok := k8s.GetResourceConfig(s.config.ResourceType)
	if !ok {
		return true // Unknown resource, let it try and fail gracefully
	}

	return s.repo.IsInformerSynced(config.GVR)
}

// getInformerSyncError checks if the informer failed to sync and returns the error
func (s *ConfigScreen) getInformerSyncError() error {
	// Safety check for tests
	if s.repo == nil {
		return nil
	}

	// Check if this resource uses typed informers (Pods, Deployments, Services, StatefulSets, DaemonSets)
	usesTypedInformer := s.config.ResourceType == k8s.ResourceTypePod ||
		s.config.ResourceType == k8s.ResourceTypeDeployment ||
		s.config.ResourceType == k8s.ResourceTypeService ||
		s.config.ResourceType == k8s.ResourceTypeStatefulSet ||
		s.config.ResourceType == k8s.ResourceTypeDaemonSet

	if usesTypedInformer {
		// Check typed informer sync error
		return s.repo.GetTypedInformersSyncError()
	}

	// For dynamic informers, check if GVR has sync error
	config, ok := k8s.GetResourceConfig(s.config.ResourceType)
	if !ok {
		return nil
	}

	return s.repo.GetDynamicInformerSyncError(config.GVR)
}

// refreshWithFilterContext fetches resources using filtered repository methods
func (s *ConfigScreen) refreshWithFilterContext() ([]interface{}, error) {
	// Handle CronJob → Jobs navigation (target is jobs, not pods)
	if s.config.ResourceType == k8s.ResourceTypeJob && s.filterContext.Field == "owner" {
		namespace := s.filterContext.Metadata["namespace"]
		jobs, err := s.repo.GetJobsForCronJob(namespace, s.filterContext.Value)
		if err != nil {
			return nil, err
		}
		// Convert []Job to []interface{}
		items := make([]interface{}, len(jobs))
		for i, job := range jobs {
			items[i] = job
		}
		return items, nil
	}

	// Handle Deployment → ReplicaSets navigation (target is replicasets, not pods)
	if s.config.ResourceType == k8s.ResourceTypeReplicaSet && s.filterContext.Field == "owner" {
		namespace := s.filterContext.Metadata["namespace"]
		replicaSets, err := s.repo.GetReplicaSetsForDeployment(namespace, s.filterContext.Value)
		if err != nil {
			return nil, err
		}
		// Convert []ReplicaSet to []interface{}
		items := make([]interface{}, len(replicaSets))
		for i, rs := range replicaSets {
			items[i] = rs
		}
		return items, nil
	}

	// All other filtering targets pods
	if s.config.ResourceType != k8s.ResourceTypePod {
		return s.repo.GetResources(s.config.ResourceType)
	}

	var pods []k8s.Pod
	var err error
	namespace := s.filterContext.Metadata["namespace"]
	kind := s.filterContext.Metadata["kind"]

	switch s.filterContext.Field {
	case "owner":
		// Deployment/StatefulSet/DaemonSet/Job/ReplicaSet → Pods
		switch kind {
		case "Deployment":
			pods, err = s.repo.GetPodsForDeployment(namespace, s.filterContext.Value)
		case "StatefulSet":
			pods, err = s.repo.GetPodsForStatefulSet(namespace, s.filterContext.Value)
		case "DaemonSet":
			pods, err = s.repo.GetPodsForDaemonSet(namespace, s.filterContext.Value)
		case "Job":
			pods, err = s.repo.GetPodsForJob(namespace, s.filterContext.Value)
		case "ReplicaSet":
			pods, err = s.repo.GetPodsForReplicaSet(namespace, s.filterContext.Value)
		default:
			return s.repo.GetResources(s.config.ResourceType)
		}
	case "node":
		// Node → Pods
		pods, err = s.repo.GetPodsOnNode(s.filterContext.Value)
	case "selector":
		// Service → Pods
		pods, err = s.repo.GetPodsForService(namespace, s.filterContext.Value)
	case "namespace":
		// Namespace → Pods
		pods, err = s.repo.GetPodsForNamespace(s.filterContext.Value)
	case "configmap":
		// ConfigMap → Pods
		pods, err = s.repo.GetPodsUsingConfigMap(namespace, s.filterContext.Value)
	case "secret":
		// Secret → Pods
		pods, err = s.repo.GetPodsUsingSecret(namespace, s.filterContext.Value)
	case "pvc":
		// PVC → Pods
		pods, err = s.repo.GetPodsForPVC(namespace, s.filterContext.Value)
	case "endpoints":
		// Endpoints → Pods (same as service selector)
		pods, err = s.repo.GetPodsForService(namespace, s.filterContext.Value)
	default:
		return s.repo.GetResources(s.config.ResourceType)
	}

	if err != nil {
		return nil, err
	}

	// Convert []Pod to []interface{}
	items := make([]interface{}, len(pods))
	for i, pod := range pods {
		items[i] = pod
	}

	return items, nil
}

// ApplyFilterContext sets the filter context for this screen
func (s *ConfigScreen) ApplyFilterContext(ctx *types.FilterContext) {
	s.filterContext = ctx
}

// GetFilterContext returns the current filter context
func (s *ConfigScreen) GetFilterContext() *types.FilterContext {
	return s.filterContext
}

// GetRefreshInterval returns the screen's refresh interval
func (s *ConfigScreen) GetRefreshInterval() time.Duration {
	return s.config.RefreshInterval
}

// GetItemCount returns the number of filtered items currently displayed
func (s *ConfigScreen) GetItemCount() int {
	return len(s.filtered)
}

// SetFilter applies a filter to the resource list
func (s *ConfigScreen) SetFilter(filter string) {
	s.filter = filter

	if s.config.CustomFilter != nil {
		s.config.CustomFilter(s, filter)
		return
	}

	s.applyFilter()

	// When filter is active, always select first row
	// This ONLY happens when user types (SetFilter), not on refresh
	if s.filter != "" && len(s.filtered) > 0 {
		s.table.SetCursor(0)
	}
}

// applyFilter filters items based on fuzzy search
func (s *ConfigScreen) applyFilter() {
	if s.filter == "" {
		s.filtered = s.items
		// Unfiltered list: keep original order from repository (already sorted by age)
	} else {
		// Build search strings using reflection on configured fields
		searchStrings := make([]string, len(s.items))
		for i, item := range s.items {
			fields := []string{}
			for _, fieldName := range s.config.SearchFields {
				val := getFieldValue(item, fieldName)
				fields = append(fields, fmt.Sprint(val))
			}
			searchStrings[i] = strings.ToLower(strings.Join(fields, " "))
		}

		// Handle negation
		if strings.HasPrefix(s.filter, "!") {
			negatePattern := strings.TrimPrefix(s.filter, "!")
			matches := fuzzy.Find(negatePattern, searchStrings)
			matchSet := make(map[int]bool)
			for _, m := range matches {
				matchSet[m.Index] = true
			}

			s.filtered = make([]interface{}, 0)
			for i, item := range s.items {
				if !matchSet[i] {
					s.filtered = append(s.filtered, item)
				}
			}
		} else {
			// Normal fuzzy search for all screens
			matches := fuzzy.Find(s.filter, searchStrings)

			// Sort matches: score (desc) → age (desc) → name (asc)
			sort.Slice(matches, func(i, j int) bool {
				// If scores are different, sort by score (higher score first)
				if matches[i].Score != matches[j].Score {
					return matches[i].Score > matches[j].Score
				}

				itemI := s.items[matches[i].Index]
				itemJ := s.items[matches[j].Index]

				// Same score: sort by age (newest first)
				ageI := getFieldValue(itemI, "Age")
				ageJ := getFieldValue(itemJ, "Age")

				// Age is time.Time, compare timestamps
				timeI, okI := ageI.(time.Time)
				timeJ, okJ := ageJ.(time.Time)

				// Both have valid times: compare them
				if okI && okJ && !timeI.IsZero() && !timeJ.IsZero() {
					if !timeI.Equal(timeJ) {
						return timeI.After(timeJ) // Newer items first
					}
				}

				// Same score and age (or age not available): sort alphabetically by name
				nameI := getFieldValue(itemI, "Name")
				nameJ := getFieldValue(itemJ, "Name")
				return strings.ToLower(fmt.Sprint(nameI)) < strings.ToLower(fmt.Sprint(nameJ))
			})

			s.filtered = make([]interface{}, len(matches))
			for i, m := range matches {
				s.filtered[i] = s.items[m.Index]
			}
		}

		// Sort negation results by age, then name
		if strings.HasPrefix(s.filter, "!") {
			sort.Slice(s.filtered, func(i, j int) bool {
				// Sort by age (newest first)
				ageI := getFieldValue(s.filtered[i], "Age")
				ageJ := getFieldValue(s.filtered[j], "Age")

				timeI, okI := ageI.(time.Time)
				timeJ, okJ := ageJ.(time.Time)

				// Both have valid times: compare them
				if okI && okJ && !timeI.IsZero() && !timeJ.IsZero() {
					if !timeI.Equal(timeJ) {
						return timeI.After(timeJ) // Newer items first
					}
				}

				// Same age or age not available: sort alphabetically by name
				nameI := getFieldValue(s.filtered[i], "Name")
				nameJ := getFieldValue(s.filtered[j], "Name")
				return strings.ToLower(fmt.Sprint(nameI)) < strings.ToLower(fmt.Sprint(nameJ))
			})
		}
	}

	s.updateTable()
}

// updateTable rebuilds table rows from filtered items
func (s *ConfigScreen) updateTable() {
	rows := make([]table.Row, len(s.filtered))

	for i, item := range s.filtered {
		// Use visibleColumns instead of s.config.Columns
		row := make(table.Row, len(s.visibleColumns))
		for j, col := range s.visibleColumns {
			val := getFieldValue(item, col.Field)

			// Apply custom formatter if provided
			if col.Format != nil {
				row[j] = col.Format(val)
			} else {
				row[j] = fmt.Sprint(val)
			}
		}
		rows[i] = row
	}

	s.table.SetRows(rows)

	// Ensure cursor is at a valid position (bounds checking only)
	if len(rows) > 0 {
		cursor := s.table.Cursor()
		if cursor < 0 || cursor >= len(rows) {
			s.table.SetCursor(0)
		}
	}

	// Ensure cursor is visible in viewport after table rebuild
	s.ensureCursorVisible()
}

// ensureCursorVisible ensures the cursor is at a valid position within the
// filtered items. The Bubble Tea table handles viewport scrolling automatically,
// so we just need to ensure the cursor is within bounds and at position 0
// if the list was filtered/updated.
func (s *ConfigScreen) ensureCursorVisible() {
	rowCount := len(s.filtered)
	if rowCount == 0 {
		return
	}

	cursor := s.table.Cursor()

	// Ensure cursor is within bounds
	if cursor < 0 {
		s.table.SetCursor(0)
	} else if cursor >= rowCount {
		s.table.SetCursor(rowCount - 1)
	}
}

// GetSelectedResource returns the currently selected resource as a map
func (s *ConfigScreen) GetSelectedResource() map[string]interface{} {
	cursor := s.table.Cursor()
	if cursor < 0 || cursor >= len(s.filtered) {
		return nil
	}

	// Convert to map using reflection
	item := s.filtered[cursor]
	result := make(map[string]interface{})

	v := reflect.ValueOf(item)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)
		fieldName := field.Name
		fieldValue := v.Field(i)

		// Handle embedded structs (like ResourceMetadata)
		if field.Anonymous && fieldValue.Kind() == reflect.Struct {
			// Flatten embedded struct fields into result map
			embeddedType := fieldValue.Type()
			for j := 0; j < fieldValue.NumField(); j++ {
				embeddedFieldName := embeddedType.Field(j).Name
				embeddedFieldValue := fieldValue.Field(j).Interface()
				result[strings.ToLower(embeddedFieldName)] = embeddedFieldValue
			}
		} else {
			// Normal field
			result[strings.ToLower(fieldName)] = fieldValue.Interface()
		}
	}

	return result
}

// handleEnterKey handles contextual navigation when Enter is pressed
func (s *ConfigScreen) handleEnterKey() tea.Cmd {
	// Delegate to configured navigation handler
	if s.config.NavigationHandler != nil {
		return s.config.NavigationHandler(s)
	}
	return nil
}

// Helper functions

// getFieldValue extracts a field value from an interface{} using reflection
// Supports dot notation for nested field access (e.g., "Fields.Ready" accesses the Fields map)
func getFieldValue(obj interface{}, fieldName string) interface{} {
	v := reflect.ValueOf(obj)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	// Handle dot notation for nested fields (e.g., "Fields.Ready")
	if strings.Contains(fieldName, ".") {
		parts := strings.SplitN(fieldName, ".", 2)
		parentField := parts[0]
		childKey := parts[1]

		// Get the parent field (e.g., "Fields")
		parent := v.FieldByName(parentField)
		if !parent.IsValid() {
			return ""
		}

		// If parent is a map, access the key
		if parent.Kind() == reflect.Map {
			mapValue := parent.MapIndex(reflect.ValueOf(childKey))
			if !mapValue.IsValid() {
				return ""
			}
			return mapValue.Interface()
		}

		// If parent is a struct, recurse
		if parent.Kind() == reflect.Struct {
			return getFieldValue(parent.Interface(), childKey)
		}

		return ""
	}

	// Direct field access for simple field names
	field := v.FieldByName(fieldName)
	if !field.IsValid() {
		return ""
	}

	return field.Interface()
}

// makeOperationHandler creates an Execute function for an operation
func (s *ConfigScreen) makeOperationHandler(_ OperationConfig) func() tea.Cmd {
	return func() tea.Cmd {
		// Default: no-op
		// Custom operation handlers can be added when needed
		return nil
	}
}

// updateSelectedKey tracks the selected resource (for cursor restoration)
func (s *ConfigScreen) updateSelectedKey() {
	cursor := s.table.Cursor()
	if cursor >= 0 && cursor < len(s.filtered) {
		item := s.filtered[cursor]
		s.selectedKey = getResourceKey(item)
	}
}

// restoreCursorPosition restores cursor to previously selected resource
func (s *ConfigScreen) restoreCursorPosition() {
	if s.selectedKey == "" {
		return
	}

	for i, item := range s.filtered {
		if getResourceKey(item) == s.selectedKey {
			s.table.SetCursor(i)
			return
		}
	}
}

// getResourceKey generates a unique key for a resource (namespace/name)
func getResourceKey(item interface{}) string {
	namespace := fmt.Sprint(getFieldValue(item, "Namespace"))
	name := fmt.Sprint(getFieldValue(item, "Name"))
	return fmt.Sprintf("%s/%s", namespace, name)
}

// FormatDuration formats a time.Duration as a human-readable string
func FormatDuration(val interface{}) string {
	d, ok := val.(time.Duration)
	if !ok {
		return fmt.Sprint(val)
	}

	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dd", int(d.Hours()/24))
}

// FormatDate formats a date/time value as a human-readable string
// Handles string values (like those from JSONPath evaluation) or time.Time
func FormatDate(val interface{}) string {
	switch v := val.(type) {
	case string:
		// If already a string (from JSONPath), try to parse it
		if v == "" {
			return "<none>"
		}
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			// If parsing fails, return as-is (might already be formatted)
			return v
		}
		// Format as relative time
		return FormatDuration(time.Since(t)) + " ago"
	case time.Time:
		if v.IsZero() {
			return "<none>"
		}
		return FormatDuration(time.Since(v)) + " ago"
	default:
		return fmt.Sprint(val)
	}
}
