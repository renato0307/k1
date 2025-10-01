package screens

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"timoneiro/internal/k8s"
	"timoneiro/internal/types"
)

type DeploymentsScreen struct {
	repo        k8s.Repository
	table       table.Model
	deployments []k8s.Deployment
	filtered    []k8s.Deployment
	filter      string
}

func NewDeploymentsScreen(repo k8s.Repository) *DeploymentsScreen {
	columns := []table.Column{
		{Title: "Namespace", Width: 20},
		{Title: "Name", Width: 40},
		{Title: "Ready", Width: 10},
		{Title: "Up-to-date", Width: 12},
		{Title: "Available", Width: 12},
		{Title: "Age", Width: 10},
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithFocused(true),
		table.WithHeight(10),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(false)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(false)
	t.SetStyles(s)

	return &DeploymentsScreen{
		repo:  repo,
		table: t,
	}
}

func (s *DeploymentsScreen) ID() string {
	return "deployments"
}

func (s *DeploymentsScreen) Title() string {
	return "Deployments"
}

func (s *DeploymentsScreen) HelpText() string {
	return "↑/↓: navigate • /: filter • ctrl+s: screens • ctrl+p: commands • ctrl+c: quit"
}

func (s *DeploymentsScreen) Operations() []types.Operation {
	return []types.Operation{
		{
			ID:          "scale",
			Name:        "Scale",
			Description: "Scale selected deployment",
			Shortcut:    "s",
			Execute:     func() tea.Cmd { return nil },
		},
		{
			ID:          "restart",
			Name:        "Restart",
			Description: "Restart selected deployment",
			Shortcut:    "r",
			Execute:     func() tea.Cmd { return nil },
		},
		{
			ID:          "describe",
			Name:        "Describe",
			Description: "Describe selected deployment",
			Shortcut:    "d",
			Execute:     func() tea.Cmd { return nil },
		},
	}
}

func (s *DeploymentsScreen) Init() tea.Cmd {
	return s.refresh()
}

func (s *DeploymentsScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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

func (s *DeploymentsScreen) View() string {
	return s.table.View()
}

func (s *DeploymentsScreen) SetSize(width, height int) {
	s.table.SetHeight(height)
	// TODO: Adjust column widths based on width
}

func (s *DeploymentsScreen) SetFilter(filter string) {
	s.filter = filter
	s.applyFilter()
}

func (s *DeploymentsScreen) refresh() tea.Cmd {
	return func() tea.Msg {
		start := time.Now()
		deployments, err := s.repo.GetDeployments()
		if err != nil {
			return types.ErrorMsg{Error: fmt.Sprintf("Failed to fetch deployments: %v", err)}
		}

		s.deployments = deployments
		s.applyFilter()

		return types.RefreshCompleteMsg{Duration: time.Since(start)}
	}
}

func (s *DeploymentsScreen) applyFilter() {
	if s.filter == "" {
		s.filtered = s.deployments
	} else {
		s.filtered = make([]k8s.Deployment, 0)
		lowerFilter := strings.ToLower(s.filter)
		for _, dep := range s.deployments {
			searchText := strings.ToLower(fmt.Sprintf("%s %s",
				dep.Namespace, dep.Name))
			if strings.Contains(searchText, lowerFilter) {
				s.filtered = append(s.filtered, dep)
			}
		}
	}

	s.updateTable()
}

func (s *DeploymentsScreen) updateTable() {
	rows := make([]table.Row, len(s.filtered))
	for i, dep := range s.filtered {
		rows[i] = table.Row{
			dep.Namespace,
			dep.Name,
			dep.Ready,
			fmt.Sprintf("%d", dep.UpToDate),
			fmt.Sprintf("%d", dep.Available),
			formatDuration(dep.Age),
		}
	}
	s.table.SetRows(rows)
}
