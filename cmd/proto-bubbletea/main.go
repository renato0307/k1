package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

// model represents the application state
type model struct {
	cursor   int
	selected map[int]struct{}
	items    []string
}

// initialModel creates the initial application state
func initialModel() model {
	return model{
		cursor:   0,
		selected: make(map[int]struct{}),
		items: []string{
			"Pod: nginx-deployment-7d4f8c9b5d-abc12",
			"Pod: postgres-statefulset-0",
			"Pod: redis-master-6f8d9c7b5a-xyz34",
			"Service: nginx-service",
			"Service: postgres-service",
			"Deployment: nginx-deployment",
			"StatefulSet: postgres-statefulset",
		},
	}
}

// Init initializes the application
func (m model) Init() tea.Cmd {
	return nil
}

// Update handles messages and updates the model
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}

		case "down", "j":
			if m.cursor < len(m.items)-1 {
				m.cursor++
			}

		case "enter", " ":
			_, ok := m.selected[m.cursor]
			if ok {
				delete(m.selected, m.cursor)
			} else {
				m.selected[m.cursor] = struct{}{}
			}
		}
	}

	return m, nil
}

// View renders the UI
func (m model) View() string {
	s := "K1 - Kubernetes Resource Browser (Prototype)\n\n"
	s += "Use ↑/↓ or j/k to navigate, space/enter to select, q to quit\n\n"

	for i, item := range m.items {
		cursor := " "
		if m.cursor == i {
			cursor = ">"
		}

		checked := " "
		if _, ok := m.selected[i]; ok {
			checked = "✓"
		}

		s += fmt.Sprintf("%s [%s] %s\n", cursor, checked, item)
	}

	s += "\n"
	return s
}

func main() {
	p := tea.NewProgram(initialModel())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running program: %v\n", err)
		os.Exit(1)
	}
}
