package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/renato0307/k1/internal/app"
	"github.com/renato0307/k1/internal/k8s"
	"github.com/renato0307/k1/internal/ui"
)

func main() {
	// Parse flags
	themeFlag := flag.String("theme", "charm", "Theme to use (charm, dracula, catppuccin, nord, gruvbox, tokyo-night, solarized, monokai)")
	kubeconfigFlag := flag.String("kubeconfig", "", "Path to kubeconfig file (default: $HOME/.kube/config)")
	contextFlag := flag.String("context", "", "Kubernetes context to use")
	dummyFlag := flag.Bool("dummy", false, "Use dummy data instead of connecting to cluster")
	flag.Parse()

	// Load theme
	theme := ui.GetTheme(*themeFlag)

	// Initialize repository
	var repo k8s.Repository

	if *dummyFlag {
		// Use dummy repository for development
		repo = k8s.NewDummyRepository()
	} else {
		// Connect to Kubernetes cluster
		fmt.Println("Connecting to Kubernetes cluster...")
		fmt.Println("Syncing cache...")

		var err error
		repo, err = k8s.NewInformerRepository(*kubeconfigFlag, *contextFlag)
		if err != nil {
			fmt.Printf("Error initializing Kubernetes connection: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("Cache synced! Starting UI...")
	}

	// Ensure cleanup on exit
	defer repo.Close()

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
