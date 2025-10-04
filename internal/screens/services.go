package screens

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/sahilm/fuzzy"

	"github.com/renato0307/k1/internal/k8s"
	"github.com/renato0307/k1/internal/types"
	"github.com/renato0307/k1/internal/ui"
)

type ServicesScreen struct {
	repo     k8s.Repository
	table    table.Model
	services []k8s.Service
	filtered []k8s.Service
	filter   string
	theme    *ui.Theme
}

func NewServicesScreen(repo k8s.Repository, theme *ui.Theme) *ServicesScreen {
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

	t.SetStyles(theme.ToTableStyles())

	return &ServicesScreen{
		repo:  repo,
		table: t,
		theme: theme,
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
	switch msg := msg.(type) {
	case types.RefreshCompleteMsg:
		// Already handled in app.go
		return s, nil
	case types.ErrorMsg:
		// Already handled in app.go
		return s, nil
	case types.FilterUpdateMsg:
		s.SetFilter(msg.Filter)
		return s, nil
	case types.ClearFilterMsg:
		s.SetFilter("")
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

	// Calculate dynamic column widths
	// Note: bubble tea table handles column spacing automatically via cell padding

	namespaceWidth := 20
	typeWidth := 15
	clusterIPWidth := 15
	externalIPWidth := 15
	portsWidth := 20
	ageWidth := 10

	fixedTotal := namespaceWidth + typeWidth + clusterIPWidth + externalIPWidth + portsWidth + ageWidth

	// Name column gets remaining space
	// Account for cell padding: 7 columns * 2 = 14 chars
	nameWidth := width - fixedTotal - 14
	if nameWidth < 25 {
		nameWidth = 25
	}

	columns := []table.Column{
		{Title: "Namespace", Width: namespaceWidth},
		{Title: "Name", Width: nameWidth},
		{Title: "Type", Width: typeWidth},
		{Title: "Cluster-IP", Width: clusterIPWidth},
		{Title: "External-IP", Width: externalIPWidth},
		{Title: "Ports", Width: portsWidth},
		{Title: "Age", Width: ageWidth},
	}

	s.table.SetColumns(columns)
	s.table.SetWidth(width)
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
		if strings.HasPrefix(s.filter, "!") {
			// Negation: exclude matches
			negatePattern := strings.TrimPrefix(s.filter, "!")
			s.filtered = make([]k8s.Service, 0)

			searchStrings := make([]string, len(s.services))
			for i, svc := range s.services {
				searchStrings[i] = fmt.Sprintf("%s %s %s %s", svc.Namespace, svc.Name, svc.Type, svc.ClusterIP)
			}

			matches := fuzzy.Find(negatePattern, searchStrings)
			matchSet := make(map[int]bool)
			for _, m := range matches {
				matchSet[m.Index] = true
			}

			for i, svc := range s.services {
				if !matchSet[i] {
					s.filtered = append(s.filtered, svc)
				}
			}
		} else {
			// Normal fuzzy search
			searchStrings := make([]string, len(s.services))
			for i, svc := range s.services {
				searchStrings[i] = fmt.Sprintf("%s %s %s %s", svc.Namespace, svc.Name, svc.Type, svc.ClusterIP)
			}

			matches := fuzzy.Find(s.filter, searchStrings)
			s.filtered = make([]k8s.Service, len(matches))
			for i, m := range matches {
				s.filtered[i] = s.services[m.Index]
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
