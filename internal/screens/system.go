package screens

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/renato0307/k1/internal/k8s"
	"github.com/renato0307/k1/internal/types"
	"github.com/renato0307/k1/internal/ui"
)

// SystemScreen displays system resource statistics
type SystemScreen struct {
	repo        k8s.Repository
	theme       *ui.Theme
	table       table.Model
	width       int
	height      int
	lastRefresh time.Time
}

// NewSystemScreen creates a new system resources screen
func NewSystemScreen(repo k8s.Repository, theme *ui.Theme) *SystemScreen {
	columns := []table.Column{
		{Title: "Resource Type", Width: 30},
		{Title: "Count", Width: 10},
		{Title: "Memory", Width: 12},
		{Title: "Synced", Width: 10},
		{Title: "Adds", Width: 10},
		{Title: "Updates", Width: 10},
		{Title: "Deletes", Width: 10},
		{Title: "Last Update", Width: 20},
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithFocused(true),
		table.WithHeight(20),
	)

	s := theme.ToTableStyles()
	t.SetStyles(s)

	return &SystemScreen{
		repo:  repo,
		theme: theme,
		table: t,
	}
}

func (s *SystemScreen) Init() tea.Cmd {
	return tea.Batch(
		s.refresh(),
		tea.Tick(time.Second, func(t time.Time) tea.Msg {
			return tickMsg{screenID: s.ID(), time: t}
		}),
	)
}

func (s *SystemScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s.width = msg.Width
		s.height = msg.Height
		return s, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return s, tea.Quit
		case "esc":
			return s, func() tea.Msg {
				return types.ScreenSwitchMsg{ScreenID: "pods"}
			}
		}

	case types.RefreshCompleteMsg:
		s.lastRefresh = time.Now()
		return s, nil

	case tickMsg:
		// Ignore ticks from other screens (prevents multiple concurrent ticks)
		if msg.screenID != s.ID() {
			return s, nil
		}
		return s, tea.Batch(
			s.refresh(),
			tea.Tick(time.Second, func(t time.Time) tea.Msg {
				return tickMsg{screenID: s.ID(), time: t}
			}),
		)
	}

	var cmd tea.Cmd
	s.table, cmd = s.table.Update(msg)
	return s, cmd
}

func (s *SystemScreen) View() string {
	return s.table.View()
}

func (s *SystemScreen) refresh() tea.Cmd {
	return func() tea.Msg {
		start := time.Now()

		stats := s.repo.GetResourceStats()

		// Calculate totals
		var totalCount int
		var totalMemory int64
		var totalAdds int64
		var totalUpdates int64
		var totalDeletes int64
		syncedCount := 0

		rows := make([]table.Row, 0, len(stats)+2) // +2 for separator and totals
		for _, stat := range stats {
			syncedStr := "Yes"
			if !stat.Synced {
				syncedStr = "No"
			} else {
				syncedCount++
			}

			memoryMB := fmt.Sprintf("%.2f MB", float64(stat.MemoryBytes)/1024/1024)

			lastUpdateStr := "Never"
			if !stat.LastUpdate.IsZero() {
				lastUpdateStr = stat.LastUpdate.Format("15:04:05")
			}

			rows = append(rows, table.Row{
				string(stat.ResourceType),
				fmt.Sprintf("%d", stat.Count),
				memoryMB,
				syncedStr,
				fmt.Sprintf("%d", stat.AddEvents),
				fmt.Sprintf("%d", stat.UpdateEvents),
				fmt.Sprintf("%d", stat.DeleteEvents),
				lastUpdateStr,
			})

			// Accumulate totals
			totalCount += stat.Count
			totalMemory += stat.MemoryBytes
			totalAdds += stat.AddEvents
			totalUpdates += stat.UpdateEvents
			totalDeletes += stat.DeleteEvents
		}

		// Add separator row
		rows = append(rows, table.Row{
			"─────────────────────────────",
			"──────────",
			"────────────",
			"──────────",
			"──────────",
			"──────────",
			"──────────",
			"────────────────────",
		})

		// Add totals row
		totalMemoryMB := fmt.Sprintf("%.2f MB", float64(totalMemory)/1024/1024)
		syncedSummary := fmt.Sprintf("%d/%d", syncedCount, len(stats))
		rows = append(rows, table.Row{
			"TOTAL",
			fmt.Sprintf("%d", totalCount),
			totalMemoryMB,
			syncedSummary,
			fmt.Sprintf("%d", totalAdds),
			fmt.Sprintf("%d", totalUpdates),
			fmt.Sprintf("%d", totalDeletes),
			"",
		})

		s.table.SetRows(rows)

		return types.RefreshCompleteMsg{Duration: time.Since(start)}
	}
}

func (s *SystemScreen) ID() string {
	return "system-resources"
}

func (s *SystemScreen) Title() string {
	return "System Resources"
}

func (s *SystemScreen) HelpText() string {
	return "↑/↓: Navigate | esc: Back to Pods | q: Quit"
}

func (s *SystemScreen) Operations() []types.Operation {
	return []types.Operation{}
}

func (s *SystemScreen) SetSize(width, height int) {
	s.width = width
	s.height = height
	s.table.SetHeight(height - 5)
}

func (s *SystemScreen) GetSelectedResource() map[string]any {
	return nil
}

func (s *SystemScreen) ApplyFilterContext(ctx *types.FilterContext) {
	// No-op for system screen
}

func (s *SystemScreen) GetFilterContext() *types.FilterContext {
	return nil
}
