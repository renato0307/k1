package screens

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"

	"timoneiro/internal/k8s"
	"timoneiro/internal/types"
	"timoneiro/internal/ui"
)

type PodsScreen struct {
	repo     k8s.Repository
	table    table.Model
	pods     []k8s.Pod
	filtered []k8s.Pod
	filter   string
	theme    *ui.Theme
}

func NewPodsScreen(repo k8s.Repository, theme *ui.Theme) *PodsScreen {
	columns := []table.Column{
		{Title: "Namespace", Width: 20},
		{Title: "Name", Width: 40},
		{Title: "Ready", Width: 8},
		{Title: "Status", Width: 15},
		{Title: "Restarts", Width: 10},
		{Title: "Age", Width: 10},
		{Title: "Node", Width: 20},
		{Title: "IP", Width: 15},
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithFocused(true),
		table.WithHeight(10),
	)

	// Apply theme styles
	t.SetStyles(theme.ToTableStyles())

	return &PodsScreen{
		repo:  repo,
		table: t,
		theme: theme,
	}
}

func (s *PodsScreen) ID() string {
	return "pods"
}

func (s *PodsScreen) Title() string {
	return "Pods"
}

func (s *PodsScreen) HelpText() string {
	return "↑/↓: navigate • /: filter • ctrl+s: screens • ctrl+p: commands • ctrl+c: quit"
}

func (s *PodsScreen) Operations() []types.Operation {
	return []types.Operation{
		{
			ID:          "logs",
			Name:        "View Logs",
			Description: "View logs for selected pod",
			Shortcut:    "l",
			Execute:     func() tea.Cmd { return nil },
		},
		{
			ID:          "describe",
			Name:        "Describe",
			Description: "Describe selected pod",
			Shortcut:    "d",
			Execute:     func() tea.Cmd { return nil },
		},
		{
			ID:          "delete",
			Name:        "Delete",
			Description: "Delete selected pod",
			Shortcut:    "x",
			Execute:     func() tea.Cmd { return nil },
		},
	}
}

func (s *PodsScreen) Init() tea.Cmd {
	return s.refresh()
}

func (s *PodsScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg.(type) {
	case types.RefreshCompleteMsg:
		// Already handled in app.go
		return s, nil
	case types.ErrorMsg:
		// Already handled in app.go
		return s, nil
	}

	var cmd tea.Cmd
	s.table, cmd = s.table.Update(msg)
	return s, cmd
}

func (s *PodsScreen) View() string {
	return s.table.View()
}

func (s *PodsScreen) SetSize(width, height int) {
	s.table.SetHeight(height)
	// TODO: Adjust column widths based on width
}

func (s *PodsScreen) SetFilter(filter string) {
	s.filter = filter
	s.applyFilter()
}

func (s *PodsScreen) refresh() tea.Cmd {
	return func() tea.Msg {
		start := time.Now()
		pods, err := s.repo.GetPods()
		if err != nil {
			return types.ErrorMsg{Error: fmt.Sprintf("Failed to fetch pods: %v", err)}
		}

		s.pods = pods
		s.applyFilter()

		return types.RefreshCompleteMsg{Duration: time.Since(start)}
	}
}

func (s *PodsScreen) applyFilter() {
	if s.filter == "" {
		s.filtered = s.pods
	} else {
		s.filtered = make([]k8s.Pod, 0)
		lowerFilter := strings.ToLower(s.filter)
		for _, pod := range s.pods {
			searchText := strings.ToLower(fmt.Sprintf("%s %s %s %s %s",
				pod.Namespace, pod.Name, pod.Status, pod.Node, pod.IP))
			if strings.Contains(searchText, lowerFilter) {
				s.filtered = append(s.filtered, pod)
			}
		}
	}

	s.updateTable()
}

func (s *PodsScreen) updateTable() {
	rows := make([]table.Row, len(s.filtered))
	for i, pod := range s.filtered {
		rows[i] = table.Row{
			pod.Namespace,
			pod.Name,
			pod.Ready,
			pod.Status,
			fmt.Sprintf("%d", pod.Restarts),
			formatDuration(pod.Age),
			pod.Node,
			pod.IP,
		}
	}
	s.table.SetRows(rows)
}

func formatDuration(d time.Duration) string {
	days := int(d.Hours() / 24)
	hours := int(d.Hours()) % 24

	if days > 0 {
		return fmt.Sprintf("%dd%dh", days, hours)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh", hours)
	}
	return fmt.Sprintf("%dm", int(d.Minutes()))
}
