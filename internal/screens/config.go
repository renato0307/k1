package screens

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/renato0307/k1/internal/k8s"
	"github.com/renato0307/k1/internal/types"
	"github.com/renato0307/k1/internal/ui"
	"github.com/sahilm/fuzzy"
)

// tickMsg triggers periodic refresh
type tickMsg time.Time

// ColumnConfig defines a column in the resource list table
type ColumnConfig struct {
	Field  string                   // Field name in resource struct
	Title  string                   // Column display title
	Width  int                      // 0 = dynamic (fills remaining space)
	Format func(interface{}) string // Optional custom formatter
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
		config: cfg,
		repo:   repo,
		table:  t,
		theme:  theme,
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
	cmds := []tea.Cmd{s.Refresh()}

	// If periodic refresh is enabled, start the tick cycle
	if s.config.EnablePeriodicRefresh {
		cmds = append(cmds, tea.Tick(s.config.RefreshInterval, func(t time.Time) tea.Msg {
			return tickMsg(t)
		}))
	}

	return tea.Batch(cmds...)
}

func (s *ConfigScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Check for custom update handler
	if s.config.CustomUpdate != nil {
		return s.config.CustomUpdate(s, msg)
	}

	return s.DefaultUpdate(msg)
}

func (s *ConfigScreen) DefaultUpdate(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case types.RefreshCompleteMsg:
		if s.config.TrackSelection {
			s.restoreCursorPosition()
		}
		return s, nil

	case types.FilterUpdateMsg:
		s.SetFilter(msg.Filter)
		return s, nil

	case types.ClearFilterMsg:
		s.SetFilter("")
		return s, nil

	case tea.KeyMsg:
		// Intercept Enter for contextual navigation
		if msg.Type == tea.KeyEnter {
			if cmd := s.handleEnterKey(); cmd != nil {
				return s, cmd
			}
		}

		var cmd tea.Cmd
		s.table, cmd = s.table.Update(msg)
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

	// Calculate dynamic column widths
	fixedTotal := 0
	dynamicCount := 0

	for _, col := range s.config.Columns {
		if col.Width > 0 {
			fixedTotal += col.Width
		} else {
			dynamicCount++
		}
	}

	// Account for cell padding: numColumns * 2
	padding := len(s.config.Columns) * 2
	dynamicWidth := (width - fixedTotal - padding) / dynamicCount
	if dynamicWidth < 20 {
		dynamicWidth = 20
	}

	columns := make([]table.Column, len(s.config.Columns))
	for i, col := range s.config.Columns {
		w := col.Width
		if w == 0 {
			w = dynamicWidth
		}
		columns[i] = table.Column{
			Title: col.Title,
			Width: w,
		}
	}

	s.table.SetColumns(columns)
	s.table.SetWidth(width)
}

// Refresh fetches resources and updates the table
func (s *ConfigScreen) Refresh() tea.Cmd {
	if s.config.CustomRefresh != nil {
		return s.config.CustomRefresh(s)
	}

	return func() tea.Msg {
		start := time.Now()

		var items []interface{}
		var err error

		// Use filtered repository methods if FilterContext is set
		if s.filterContext != nil {
			items, err = s.refreshWithFilterContext()
		} else {
			items, err = s.repo.GetResources(s.config.ResourceType)
		}

		if err != nil {
			return types.ErrorStatusMsg(fmt.Sprintf("Failed to fetch %s: %v", s.config.Title, err))
		}

		s.items = items
		s.applyFilter()

		return types.RefreshCompleteMsg{Duration: time.Since(start)}
	}
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

// SetFilter applies a filter to the resource list
func (s *ConfigScreen) SetFilter(filter string) {
	s.filter = filter

	if s.config.CustomFilter != nil {
		s.config.CustomFilter(s, filter)
		return
	}

	s.applyFilter()
}

// applyFilter filters items based on fuzzy search
func (s *ConfigScreen) applyFilter() {
	if s.filter == "" {
		s.filtered = s.items
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
			// Normal fuzzy search
			matches := fuzzy.Find(s.filter, searchStrings)
			s.filtered = make([]interface{}, len(matches))
			for i, m := range matches {
				s.filtered[i] = s.items[m.Index]
			}
		}
	}

	s.updateTable()
}

// updateTable rebuilds table rows from filtered items
func (s *ConfigScreen) updateTable() {
	rows := make([]table.Row, len(s.filtered))

	for i, item := range s.filtered {
		row := make(table.Row, len(s.config.Columns))
		for j, col := range s.config.Columns {
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

	// Ensure cursor is at a valid position
	// If we have rows and cursor is out of bounds, move to first row
	if len(rows) > 0 {
		cursor := s.table.Cursor()
		if cursor < 0 || cursor >= len(rows) {
			s.table.SetCursor(0)
		}
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
func getFieldValue(obj interface{}, fieldName string) interface{} {
	v := reflect.ValueOf(obj)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

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
