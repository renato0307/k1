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

type DeploymentsScreen struct {
	repo        k8s.Repository
	table       table.Model
	deployments []k8s.Deployment
	filtered    []k8s.Deployment
	filter      string
	theme       *ui.Theme
}

func NewDeploymentsScreen(repo k8s.Repository, theme *ui.Theme) *DeploymentsScreen {
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

	t.SetStyles(theme.ToTableStyles())

	return &DeploymentsScreen{
		repo:  repo,
		table: t,
		theme: theme,
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

func (s *DeploymentsScreen) View() string {
	return s.table.View()
}

func (s *DeploymentsScreen) SetSize(width, height int) {
	s.table.SetHeight(height)

	// Calculate dynamic column widths
	// Note: bubble tea table handles column spacing automatically via cell padding

	namespaceWidth := 20
	readyWidth := 10
	upToDateWidth := 12
	availableWidth := 12
	ageWidth := 10

	fixedTotal := namespaceWidth + readyWidth + upToDateWidth + availableWidth + ageWidth

	// Name column gets remaining space
	// Account for cell padding: 6 columns * 2 = 12 chars
	nameWidth := width - fixedTotal - 12
	if nameWidth < 30 {
		nameWidth = 30
	}

	columns := []table.Column{
		{Title: "Namespace", Width: namespaceWidth},
		{Title: "Name", Width: nameWidth},
		{Title: "Ready", Width: readyWidth},
		{Title: "Up-to-date", Width: upToDateWidth},
		{Title: "Available", Width: availableWidth},
		{Title: "Age", Width: ageWidth},
	}

	s.table.SetColumns(columns)
	s.table.SetWidth(width)
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
		if strings.HasPrefix(s.filter, "!") {
			// Negation: exclude matches
			negatePattern := strings.TrimPrefix(s.filter, "!")
			s.filtered = make([]k8s.Deployment, 0)

			searchStrings := make([]string, len(s.deployments))
			for i, dep := range s.deployments {
				searchStrings[i] = fmt.Sprintf("%s %s", dep.Namespace, dep.Name)
			}

			matches := fuzzy.Find(negatePattern, searchStrings)
			matchSet := make(map[int]bool)
			for _, m := range matches {
				matchSet[m.Index] = true
			}

			for i, dep := range s.deployments {
				if !matchSet[i] {
					s.filtered = append(s.filtered, dep)
				}
			}
		} else {
			// Normal fuzzy search
			searchStrings := make([]string, len(s.deployments))
			for i, dep := range s.deployments {
				searchStrings[i] = fmt.Sprintf("%s %s", dep.Namespace, dep.Name)
			}

			matches := fuzzy.Find(s.filter, searchStrings)
			s.filtered = make([]k8s.Deployment, len(matches))
			for i, m := range matches {
				s.filtered[i] = s.deployments[m.Index]
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
