package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sahilm/fuzzy"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	v1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
)

// Pod represents a simplified pod for display
type Pod struct {
	Name      string
	Namespace string
	Ready     string
	Status    string
	Restarts  int
	Age       time.Duration
	Node      string
	IP        string
}

// Model represents the application state
type model struct {
	pods           []Pod
	filteredPods   []Pod
	table          table.Model
	selectedPodKey string // namespace/name to track across sorts
	ready          bool
	err            error
	quitting       bool
	windowHeight   int
	windowWidth    int

	// Filter
	filterMode     bool
	filterText     string
	filterActive   bool
	lastSearchTime time.Duration

	// Informer
	lister v1listers.PodLister
}

// Messages
type podsLoadedMsg struct {
	pods []Pod
}

type tickMsg time.Time

type errMsg struct {
	err error
}

func initialModel(lister v1listers.PodLister) model {
	// Initial columns with placeholder widths (will be updated on window resize)
	columns := []table.Column{
		{Title: "Namespace", Width: 20},
		{Title: "Name", Width: 30},
		{Title: "Ready", Width: 8},
		{Title: "Status", Width: 12},
		{Title: "Restarts", Width: 10},
		{Title: "Age", Width: 10},
		{Title: "Node", Width: 40},
		{Title: "IP", Width: 15},
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithFocused(true),
		table.WithHeight(20),
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

	return model{
		pods:           []Pod{},
		filteredPods:   []Pod{},
		table:          t,
		selectedPodKey: "",
		ready:          false,
		windowHeight:   0,
		windowWidth:    0,
		filterMode:     false,
		filterText:     "",
		filterActive:   false,
		lister:         lister,
	}
}

// Calculate table height based on window height and filter state
func (m model) calculateTableHeight() int {
	if m.windowHeight == 0 {
		return 20 // Default
	}

	// Reserve space for:
	// - Header (2 lines: title + blank line)
	// - Footer (2 lines: blank line + help text)
	// - Filter line (1 line if active)
	height := m.windowHeight - 6
	if m.filterMode || m.filterActive {
		height-- // Reserve extra line for filter
	}
	if height < 5 {
		height = 5 // Minimum table height
	}
	return height
}

// Calculate dynamic column widths based on window width
func (m model) calculateColumnWidths() []table.Column {
	if m.windowWidth == 0 {
		// Default widths if no window size yet
		return []table.Column{
			{Title: "Namespace", Width: 36},
			{Title: "Name", Width: 30},
			{Title: "Ready", Width: 8},
			{Title: "Status", Width: 12},
			{Title: "Restarts", Width: 10},
			{Title: "Age", Width: 10},
			{Title: "Node", Width: 40},
			{Title: "IP", Width: 15},
		}
	}

	// Fixed widths for most columns
	namespaceWidth := 36
	readyWidth := 8
	statusWidth := 12
	restartsWidth := 10
	ageWidth := 10
	nodeWidth := 40
	ipWidth := 15

	// Calculate remaining space for Name column
	// Account for borders and spacing (approximately 15 chars)
	fixedWidth := namespaceWidth + readyWidth + statusWidth + restartsWidth + ageWidth + nodeWidth + ipWidth + 15
	nameWidth := m.windowWidth - fixedWidth

	// Ensure minimum width for Name column
	if nameWidth < 20 {
		nameWidth = 20
	}

	return []table.Column{
		{Title: "Namespace", Width: namespaceWidth},
		{Title: "Name", Width: nameWidth},
		{Title: "Ready", Width: readyWidth},
		{Title: "Status", Width: statusWidth},
		{Title: "Restarts", Width: restartsWidth},
		{Title: "Age", Width: ageWidth},
		{Title: "Node", Width: nodeWidth},
		{Title: "IP", Width: ipWidth},
	}
}

// Helper to get pod key
func podKey(p Pod) string {
	return p.Namespace + "/" + p.Name
}

// podSearchString creates a searchable string for fuzzy matching
type podSearchString struct {
	pod Pod
	str string
}

func (p podSearchString) String() string {
	return p.str
}

// Filter pods by text using fuzzy search
func filterPods(pods []Pod, filterText string) []Pod {
	if filterText == "" {
		return pods
	}

	// Check for negation
	negate := false
	searchText := filterText
	if len(filterText) > 0 && filterText[0] == '!' {
		negate = true
		searchText = filterText[1:]
	}

	// Don't filter if only "!" is typed
	if searchText == "" {
		return pods
	}

	// Build searchable strings for fuzzy matching
	searchables := make([]podSearchString, len(pods))
	for i, pod := range pods {
		// Combine all searchable fields into one lowercase string
		searchStr := strings.ToLower(strings.Join([]string{
			pod.Namespace,
			pod.Name,
			pod.Status,
			pod.Node,
			pod.IP,
		}, " "))
		searchables[i] = podSearchString{pod: pod, str: searchStr}
	}

	// Perform fuzzy search
	matches := fuzzy.FindFrom(strings.ToLower(searchText), fuzzySource(searchables))

	// Build filtered result
	filtered := make([]Pod, 0)

	if negate {
		// For negation, include pods that didn't match
		matchedIndices := make(map[int]bool)
		for _, match := range matches {
			matchedIndices[match.Index] = true
		}
		for i, s := range searchables {
			if !matchedIndices[i] {
				filtered = append(filtered, s.pod)
			}
		}
	} else {
		// Include matched pods, sorted by match score
		for _, match := range matches {
			filtered = append(filtered, searchables[match.Index].pod)
		}
	}

	return filtered
}

// fuzzySource wraps podSearchString slice for fuzzy.Find
type fuzzySource []podSearchString

func (f fuzzySource) Len() int {
	return len(f)
}

func (f fuzzySource) String(i int) string {
	return f[i].str
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		waitForPodsCmd(m.lister),
		tickCmd(),
	)
}

// Wait for pods to load from informer
func waitForPodsCmd(lister v1listers.PodLister) tea.Cmd {
	return func() tea.Msg {
		// Poll until we have pods
		for {
			pods, err := fetchPods(lister)
			if err == nil && len(pods) > 0 {
				return podsLoadedMsg{pods: pods}
			}
			time.Sleep(100 * time.Millisecond)
		}
	}
}

// Periodic tick to refresh pod list
func tickCmd() tea.Cmd {
	return tea.Tick(1*time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// Fetch pods from informer cache
func fetchPods(lister v1listers.PodLister) ([]Pod, error) {
	podList, err := lister.List(labels.Everything())
	if err != nil {
		return nil, err
	}

	pods := make([]Pod, 0, len(podList))
	now := time.Now()

	for _, pod := range podList {
		age := now.Sub(pod.CreationTimestamp.Time)

		// Calculate ready containers
		readyContainers := 0
		totalContainers := len(pod.Status.ContainerStatuses)
		for _, cs := range pod.Status.ContainerStatuses {
			if cs.Ready {
				readyContainers++
			}
		}
		readyStatus := fmt.Sprintf("%d/%d", readyContainers, totalContainers)

		// Calculate total restarts
		totalRestarts := 0
		for _, cs := range pod.Status.ContainerStatuses {
			totalRestarts += int(cs.RestartCount)
		}

		// Get pod status
		status := string(pod.Status.Phase)

		// Get node and IP
		node := pod.Spec.NodeName
		ip := pod.Status.PodIP

		pods = append(pods, Pod{
			Name:      pod.Name,
			Namespace: pod.Namespace,
			Ready:     readyStatus,
			Status:    status,
			Restarts:  totalRestarts,
			Age:       age,
			Node:      node,
			IP:        ip,
		})
	}

	// Sort by age (newest first), then by name
	sort.Slice(pods, func(i, j int) bool {
		if pods[i].Age != pods[j].Age {
			return pods[i].Age < pods[j].Age
		}
		return pods[i].Name < pods[j].Name
	})

	return pods, nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {

	case tea.KeyMsg:
		// Handle filter mode separately
		if m.filterMode {
			// Handle paste
			if msg.Paste {
				m.filterText += string(msg.Runes)
				start := time.Now()
				m.filteredPods = filterPods(m.pods, m.filterText)
				m.lastSearchTime = time.Since(start)
				m = updateTableRows(m)
				m = smartSetCursor(m)
				return m, nil
			}

			switch msg.String() {
			case "enter":
				// Stop filtering but keep the filter active
				m.filterMode = false
				m.filterActive = m.filterText != ""
				m.table.SetHeight(m.calculateTableHeight())
				return m, nil

			case "esc":
				// Cancel filter mode and clear filter
				m.filterMode = false
				m.filterText = ""
				m.filterActive = false
				m.filteredPods = m.pods
				m = updateTableRows(m)
				m.table.SetHeight(m.calculateTableHeight())
				return m, nil

			case "backspace":
				// Remove last character
				if len(m.filterText) > 0 {
					m.filterText = m.filterText[:len(m.filterText)-1]
					start := time.Now()
					m.filteredPods = filterPods(m.pods, m.filterText)
					m.lastSearchTime = time.Since(start)
					m = updateTableRows(m)
					m = smartSetCursor(m)
				}
				return m, nil

			default:
				// Add character to filter
				if len(msg.String()) == 1 {
					m.filterText += msg.String()
					start := time.Now()
					m.filteredPods = filterPods(m.pods, m.filterText)
					m.lastSearchTime = time.Since(start)
					m = updateTableRows(m)
					m = smartSetCursor(m)
				}
				return m, nil
			}
		}

		// Normal mode key handling
		switch msg.String() {
		case "ctrl+c", "q":
			m.quitting = true
			return m, tea.Quit

		case "/":
			// Enter filter mode
			m.filterMode = true
			m.table.SetHeight(m.calculateTableHeight())
			return m, nil

		case "esc":
			// Clear filter if active
			if m.filterActive {
				m.filterText = ""
				m.filterActive = false
				m.filteredPods = m.pods
				m = updateTableRows(m)
				m.table.SetHeight(m.calculateTableHeight())
			}
			return m, nil
		}

		// Let table handle navigation
		m.table, cmd = m.table.Update(msg)

		// Update selected pod key after navigation
		if m.table.Cursor() >= 0 && m.table.Cursor() < len(m.filteredPods) {
			m.selectedPodKey = podKey(m.filteredPods[m.table.Cursor()])
		}

		return m, cmd

	case podsLoadedMsg:
		m.pods = msg.pods
		m.filteredPods = filterPods(m.pods, m.filterText)
		m.ready = true
		m = updateTableRows(m)
		// Set initial selection
		if len(m.filteredPods) > 0 {
			m.selectedPodKey = podKey(m.filteredPods[0])
		}
		return m, tickCmd()

	case tickMsg:
		// Refresh pods from cache
		pods, err := fetchPods(m.lister)
		if err != nil {
			return m, tickCmd()
		}

		// Update pods and apply filter
		m.pods = pods
		m.filteredPods = filterPods(m.pods, m.filterText)

		// Find the previously selected pod in the new sorted list
		newCursor := 0
		if m.selectedPodKey != "" {
			found := false
			for i, pod := range m.filteredPods {
				if podKey(pod) == m.selectedPodKey {
					newCursor = i
					found = true
					break
				}
			}
			// If previously selected pod is gone, keep nearest position
			if !found && len(m.filteredPods) > 0 {
				if m.table.Cursor() >= len(m.filteredPods) {
					newCursor = len(m.filteredPods) - 1
				} else {
					newCursor = m.table.Cursor()
				}
				m.selectedPodKey = podKey(m.filteredPods[newCursor])
			}
		}

		m = updateTableRows(m)
		m.table.SetCursor(newCursor)

		return m, tickCmd()

	case tea.WindowSizeMsg:
		m.windowHeight = msg.Height
		m.windowWidth = msg.Width
		m.table.SetHeight(m.calculateTableHeight())
		m.table.SetColumns(m.calculateColumnWidths())
		m = updateTableRows(m) // Rerender rows with new column widths
		m.table, cmd = m.table.Update(msg)
		return m, cmd

	case errMsg:
		m.err = msg.err
		return m, nil
	}

	return m, nil
}

// Update table rows from filtered pods
func updateTableRows(m model) model {
	// Get dynamic column widths
	columns := m.calculateColumnWidths()
	nameWidth := columns[1].Width // Name column

	rows := make([]table.Row, len(m.filteredPods))
	for i, pod := range m.filteredPods {
		// Truncate name if it exceeds column width
		name := pod.Name
		if len(name) > nameWidth {
			if nameWidth > 3 {
				name = name[:nameWidth-3] + "..."
			} else {
				name = name[:nameWidth]
			}
		}

		rows[i] = table.Row{
			pod.Namespace,
			name,
			pod.Ready,
			pod.Status,
			fmt.Sprintf("%d", pod.Restarts),
			formatDuration(pod.Age),
			pod.Node,
			pod.IP,
		}
	}
	m.table.SetRows(rows)
	return m
}

// Smart cursor positioning: keep selection on same pod if it matches filter, otherwise go to first
func smartSetCursor(m model) model {
	// If we have a selected pod key, try to find it in filtered results
	if m.selectedPodKey != "" {
		for i, pod := range m.filteredPods {
			if podKey(pod) == m.selectedPodKey {
				// Found it! Keep selection on this pod
				m.table.SetCursor(i)
				return m
			}
		}
	}

	// Selected pod doesn't match filter (or no selection), go to first row
	if len(m.filteredPods) > 0 {
		m.table.SetCursor(0)
		m.selectedPodKey = podKey(m.filteredPods[0])
	}

	return m
}

// Immediate mode view - rebuild from scratch each frame
func (m model) View() string {
	if m.quitting {
		return "Goodbye!\n"
	}

	if m.err != nil {
		return fmt.Sprintf("Error: %v\n", m.err)
	}

	if !m.ready {
		return "Loading pods...\n"
	}

	var s string

	// Header
	title := fmt.Sprintf("K1 - Pods (%d)", len(m.pods))
	if (m.filterActive || m.filterMode) && m.lastSearchTime > 0 {
		title += fmt.Sprintf(" | Search: %v", m.lastSearchTime)
	}
	s += lipgloss.NewStyle().Bold(true).Render(title)
	s += "\n\n"

	// Table
	s += m.table.View()
	s += "\n"

	// Filter line (if active or in filter mode)
	if m.filterMode || m.filterActive {
		s += "\n"
		if m.filterMode {
			s += "🔍 " + m.filterText + "█" // Show cursor
		} else if m.filterActive {
			s += "🔍 " + m.filterText
		}
	}

	s += "\n"

	// Footer
	if m.filterMode {
		s += "enter: apply filter | esc: cancel"
	} else if m.filterActive {
		s += fmt.Sprintf("↑/↓ or j/k: navigate | esc: clear filter | q: quit | Filtered: %d/%d", len(m.filteredPods), len(m.pods))
	} else {
		s += "↑/↓ or j/k: navigate | /: filter | q: quit"
	}

	return s
}

func formatDuration(d time.Duration) string {
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

func main() {
	var kubeconfig *string
	if home := os.Getenv("HOME"); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}
	contextFlag := flag.String("context", "", "(optional) kubeconfig context to use")
	flag.Parse()

	// Build config
	loadingRules := &clientcmd.ClientConfigLoadingRules{ExplicitPath: *kubeconfig}
	configOverrides := &clientcmd.ConfigOverrides{}
	if *contextFlag != "" {
		configOverrides.CurrentContext = *contextFlag
	}

	config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		loadingRules,
		configOverrides,
	).ClientConfig()
	if err != nil {
		fmt.Printf("Error building kubeconfig: %v\n", err)
		os.Exit(1)
	}

	// Use protobuf for better performance
	config.ContentType = "application/vnd.kubernetes.protobuf"

	// Create clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		fmt.Printf("Error creating clientset: %v\n", err)
		os.Exit(1)
	}

	// Create shared informer factory
	factory := informers.NewSharedInformerFactory(clientset, 30*time.Second)

	// Create pod informer
	podInformer := factory.Core().V1().Pods().Informer()
	lister := factory.Core().V1().Pods().Lister()

	// Start informer in background
	ctx := context.Background()
	factory.Start(ctx.Done())

	// Wait for cache to sync before starting UI
	fmt.Println("Syncing cache...")
	if !cache.WaitForCacheSync(ctx.Done(), podInformer.HasSynced) {
		fmt.Println("Failed to sync cache")
		os.Exit(1)
	}
	fmt.Println("Cache synced! Starting UI...")

	// Create and run Bubble Tea program in full screen mode
	m := initialModel(lister)
	p := tea.NewProgram(m, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running program: %v\n", err)
		os.Exit(1)
	}
}
