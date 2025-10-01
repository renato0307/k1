package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"timoneiro/internal/app"
	"timoneiro/internal/k8s"
	"timoneiro/internal/ui"
)

func main() {
	// Parse flags
	themeFlag := flag.String("theme", "charm", "Theme to use (charm, dracula, catppuccin)")
	flag.Parse()

	// Load theme
	theme := ui.GetTheme(*themeFlag)

	// Use dummy repository for now
	repo := k8s.NewDummyRepository()

	// Create the app model with theme
	model := app.NewModel(repo, theme)

	// Start the Bubble Tea program
	p := tea.NewProgram(
		model,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running program: %v\n", err)
		os.Exit(1)
	}
}
