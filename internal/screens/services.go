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

type ServicesScreen struct {
	repo     k8s.Repository
	table    table.Model
	services []k8s.Service
	filtered []k8s.Service
	filter   string
}

func NewServicesScreen(repo k8s.Repository) *ServicesScreen {
	columns := []table.Column{
		{Title: "Namespace", Width: 20},
		{Title: "Name", Width: 30},
		{Title: "Type", Width: 15},
		{Title: "Cluster-IP", Width: 15},
		{Title: "External-IP", Width: 15},
		{Title: "Ports", Width: 20},
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

	return &ServicesScreen{
		repo:  repo,
		table: t,
	}
}

func (s *ServicesScreen) ID() string {
	return "services"
}

func (s *ServicesScreen) Title() string {
	return "Services"
}

func (s *ServicesScreen) HelpText() string {
	return "↑/↓: navigate • /: filter • ctrl+s: screens • ctrl+p: commands • ctrl+c: quit"
}

func (s *ServicesScreen) Operations() []types.Operation {
	return []types.Operation{
		{
			ID:          "describe",
			Name:        "Describe",
			Description: "Describe selected service",
			Shortcut:    "d",
			Execute:     func() tea.Cmd { return nil },
		},
		{
			ID:          "endpoints",
			Name:        "Show Endpoints",
			Description: "Show endpoints for selected service",
			Shortcut:    "e",
			Execute:     func() tea.Cmd { return nil },
		},
		{
			ID:          "delete",
			Name:        "Delete",
			Description: "Delete selected service",
			Shortcut:    "x",
			Execute:     func() tea.Cmd { return nil },
		},
	}
}

func (s *ServicesScreen) Init() tea.Cmd {
	return s.refresh()
}

func (s *ServicesScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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

func (s *ServicesScreen) View() string {
	return s.table.View()
}

func (s *ServicesScreen) SetSize(width, height int) {
	s.table.SetHeight(height)
	// TODO: Adjust column widths based on width
}

func (s *ServicesScreen) SetFilter(filter string) {
	s.filter = filter
	s.applyFilter()
}

func (s *ServicesScreen) refresh() tea.Cmd {
	return func() tea.Msg {
		start := time.Now()
		services, err := s.repo.GetServices()
		if err != nil {
			return types.ErrorMsg{Error: fmt.Sprintf("Failed to fetch services: %v", err)}
		}

		s.services = services
		s.applyFilter()

		return types.RefreshCompleteMsg{Duration: time.Since(start)}
	}
}

func (s *ServicesScreen) applyFilter() {
	if s.filter == "" {
		s.filtered = s.services
	} else {
		s.filtered = make([]k8s.Service, 0)
		lowerFilter := strings.ToLower(s.filter)
		for _, svc := range s.services {
			searchText := strings.ToLower(fmt.Sprintf("%s %s %s %s",
				svc.Namespace, svc.Name, svc.Type, svc.ClusterIP))
			if strings.Contains(searchText, lowerFilter) {
				s.filtered = append(s.filtered, svc)
			}
		}
	}

	s.updateTable()
}

func (s *ServicesScreen) updateTable() {
	rows := make([]table.Row, len(s.filtered))
	for i, svc := range s.filtered {
		rows[i] = table.Row{
			svc.Namespace,
			svc.Name,
			svc.Type,
			svc.ClusterIP,
			svc.ExternalIP,
			svc.Ports,
			formatDuration(svc.Age),
		}
	}
	s.table.SetRows(rows)
}
