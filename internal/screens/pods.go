package screens

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/sahilm/fuzzy"

	"timoneiro/internal/k8s"
	"timoneiro/internal/types"
	"timoneiro/internal/ui"
)

// tickMsg triggers periodic refresh
type tickMsg time.Time

type PodsScreen struct {
	repo           k8s.Repository
	table          table.Model
	pods           []k8s.Pod
	filtered       []k8s.Pod
	filter         string
	theme          *ui.Theme
	selectedPodKey string // namespace/name to track cursor across refreshes
}

func NewPodsScreen(repo k8s.Repository, theme *ui.Theme) *PodsScreen {
	columns := []table.Column{
		{Title: "Namespace", Width: 20},
		{Title: "Name", Width: 40},
		{Title: "Ready", Width: 8},
		{Title: "Status", Width: 15},
		{Title: "Restarts", Width: 10},
		{Title: "Age", Width: 10},
		{Title: "Node", Width: 30},
		{Title: "IP", Width: 16},
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

// GetSelectedResource returns information about the currently selected pod
func (s *PodsScreen) GetSelectedResource() map[string]interface{} {
	cursor := s.table.Cursor()
	if cursor < 0 || cursor >= len(s.filtered) {
		return map[string]interface{}{}
	}

	pod := s.filtered[cursor]
	return map[string]interface{}{
		"name":      pod.Name,
		"namespace": pod.Namespace,
		"status":    pod.Status,
		"ip":        pod.IP,
		"node":      pod.Node,
	}
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
	return tea.Batch(
		s.refresh(),
		s.tickCmd(), // Start periodic refresh
	)
}

func (s *PodsScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tickMsg:
		// Periodic refresh
		return s, tea.Batch(
			s.refresh(),
			s.tickCmd(), // Schedule next tick
		)

	case types.RefreshCompleteMsg:
		// Restore cursor position after refresh
		s.restoreCursorPosition()
		return s, nil

	case types.ErrorMsg:
		// Error already handled in app.go, continue ticking
		return s, nil

	case types.FilterUpdateMsg:
		// Update filter and reapply
		s.SetFilter(msg.Filter)
		return s, nil

	case types.ClearFilterMsg:
		// Clear filter
		s.SetFilter("")
		return s, nil

	case tea.KeyMsg:
		// Track cursor position on navigation
		var cmd tea.Cmd
		s.table, cmd = s.table.Update(msg)
		s.updateSelectedPodKey()
		return s, cmd
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

	// Calculate dynamic column widths
	// Note: bubble tea table handles column spacing automatically via cell padding

	// Fixed width columns
	namespaceWidth := 20
	readyWidth := 8
	statusWidth := 15
	restartsWidth := 10
	ageWidth := 10
	nodeWidth := 30
	ipWidth := 16

	fixedTotal := namespaceWidth + readyWidth + statusWidth + restartsWidth + ageWidth + nodeWidth + ipWidth

	// Name column gets remaining space (with minimum width)
	// Account for cell padding: each column has PaddingLeft(1) + PaddingRight(1) = 2 extra chars
	// Total padding: 8 columns * 2 = 16 chars
	nameWidth := width - fixedTotal - 16
	if nameWidth < 30 {
		nameWidth = 30
	}

	columns := []table.Column{
		{Title: "Namespace", Width: namespaceWidth},
		{Title: "Name", Width: nameWidth},
		{Title: "Ready", Width: readyWidth},
		{Title: "Status", Width: statusWidth},
		{Title: "Restarts", Width: restartsWidth},
		{Title: "Age", Width: ageWidth},
		{Title: "Node", Width: nodeWidth},
		{Title: "IP", Width: ipWidth},
	}

	s.table.SetColumns(columns)
	s.table.SetWidth(width)
}

func (s *PodsScreen) SetFilter(filter string) {
	s.filter = filter
	s.applyFilter()
}

// tickCmd returns a command that sends a tick message after 1 second
func (s *PodsScreen) tickCmd() tea.Cmd {
	return tea.Tick(1*time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
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

// updateSelectedPodKey stores the currently selected pod's key
func (s *PodsScreen) updateSelectedPodKey() {
	cursor := s.table.Cursor()
	if cursor >= 0 && cursor < len(s.filtered) {
		pod := s.filtered[cursor]
		s.selectedPodKey = fmt.Sprintf("%s/%s", pod.Namespace, pod.Name)
	}
}

// restoreCursorPosition restores cursor to the previously selected pod
func (s *PodsScreen) restoreCursorPosition() {
	if s.selectedPodKey == "" {
		return
	}

	// Find the pod in the filtered list
	for i, pod := range s.filtered {
		podKey := fmt.Sprintf("%s/%s", pod.Namespace, pod.Name)
		if podKey == s.selectedPodKey {
			s.table.SetCursor(i)
			return
		}
	}

	// If pod not found (deleted or filtered out), keep cursor at safe position
	if len(s.filtered) > 0 {
		cursor := s.table.Cursor()
		if cursor >= len(s.filtered) {
			s.table.SetCursor(len(s.filtered) - 1)
		}
	}
}

func (s *PodsScreen) applyFilter() {
	if s.filter == "" {
		s.filtered = s.pods
	} else {
		// Check for negation filter
		if strings.HasPrefix(s.filter, "!") {
			// Negation: exclude matches
			negatePattern := strings.TrimPrefix(s.filter, "!")
			s.filtered = make([]k8s.Pod, 0)

			// Build search strings
			searchStrings := make([]string, len(s.pods))
			for i, pod := range s.pods {
				searchStrings[i] = fmt.Sprintf("%s %s %s %s %s",
					pod.Namespace, pod.Name, pod.Status, pod.Node, pod.IP)
			}

			// Find matches to exclude
			matches := fuzzy.Find(negatePattern, searchStrings)
			matchSet := make(map[int]bool)
			for _, m := range matches {
				matchSet[m.Index] = true
			}

			// Include only non-matches
			for i, pod := range s.pods {
				if !matchSet[i] {
					s.filtered = append(s.filtered, pod)
				}
			}
		} else {
			// Normal fuzzy search
			// Build search strings
			searchStrings := make([]string, len(s.pods))
			for i, pod := range s.pods {
				searchStrings[i] = fmt.Sprintf("%s %s %s %s %s",
					pod.Namespace, pod.Name, pod.Status, pod.Node, pod.IP)
			}

			// Perform fuzzy search
			matches := fuzzy.Find(s.filter, searchStrings)

			// Build filtered list from matches (already sorted by score)
			s.filtered = make([]k8s.Pod, len(matches))
			for i, m := range matches {
				s.filtered[i] = s.pods[m.Index]
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

	// Set initial selected pod key if not set
	if s.selectedPodKey == "" && len(s.filtered) > 0 {
		pod := s.filtered[0]
		s.selectedPodKey = fmt.Sprintf("%s/%s", pod.Namespace, pod.Name)
	}
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
