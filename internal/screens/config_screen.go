package screens

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/renato0307/k1/internal/k8s"
	"github.com/renato0307/k1/internal/types"
	"github.com/renato0307/k1/internal/ui"
	"github.com/sahilm/fuzzy"
)

// tickMsg triggers periodic refresh
type tickMsg time.Time

// ColumnConfig defines a column in the resource list table
type ColumnConfig struct {
	Field  string                      // Field name in resource struct
	Title  string                      // Column display title
	Width  int                         // 0 = dynamic (fills remaining space)
	Format func(interface{}) string   // Optional custom formatter
}

// OperationConfig defines an operation that can be executed
type OperationConfig struct {
	ID          string
	Name        string
	Description string
	Shortcut    string
}

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

	// Optional custom overrides (Level 2 customization)
	CustomRefresh    func(*ConfigScreen) tea.Cmd
	CustomFilter     func(*ConfigScreen, string)
	CustomUpdate     func(*ConfigScreen, tea.Msg) (tea.Model, tea.Cmd)
	CustomView       func(*ConfigScreen) string
	CustomOperations map[string]func(*ConfigScreen) tea.Cmd
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
	return s.table.View()
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

		items, err := s.repo.GetResources(s.config.ResourceType)
		if err != nil {
			return types.ErrorMsg{
				Error: fmt.Sprintf("Failed to fetch %s: %v", s.config.Title, err),
			}
		}

		s.items = items
		s.applyFilter()

		return types.RefreshCompleteMsg{Duration: time.Since(start)}
	}
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
		fieldName := t.Field(i).Name
		fieldValue := v.Field(i).Interface()
		result[strings.ToLower(fieldName)] = fieldValue
	}

	return result
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
func (s *ConfigScreen) makeOperationHandler(opCfg OperationConfig) func() tea.Cmd {
	return func() tea.Cmd {
		// Check for custom operation handler
		if customHandler, ok := s.config.CustomOperations[opCfg.ID]; ok {
			return customHandler(s)
		}

		// Default: no-op
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
