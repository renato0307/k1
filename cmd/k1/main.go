package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"

	tea "github.com/charmbracelet/bubbletea"
	"k8s.io/klog/v2"

	"github.com/renato0307/k1/internal/app"
	"github.com/renato0307/k1/internal/k8s"
	"github.com/renato0307/k1/internal/ui"
)

func main() {
	// Suppress klog errors from client-go (RBAC permission errors during watch)
	klog.InitFlags(nil)
	flag.Set("logtostderr", "false")
	flag.Set("stderrthreshold", "FATAL") // Only show FATAL errors
	flag.Set("v", "0")                   // Minimum verbosity

	// Parse flags
	themeFlag := flag.String("theme", "charm", "Theme to use (charm, dracula, catppuccin, nord, gruvbox, tokyo-night, solarized, monokai)")
	kubeconfigFlag := flag.String("kubeconfig", "", "Path to kubeconfig file (default: $HOME/.kube/config)")
	contextFlag := flag.String("context", "", "Kubernetes context to use")
	dummyFlag := flag.Bool("dummy", false, "Use dummy data instead of connecting to cluster")
	flag.Parse()
	defer klog.Flush()

	// Load theme
	theme := ui.GetTheme(*themeFlag)

	// Initialize repository
	var repo k8s.Repository

	if *dummyFlag {
		// Use dummy repository for development
		repo = k8s.NewDummyRepository()
	} else {
		// Check if kubectl is available (needed for resource commands)
		if err := checkKubectlAvailable(); err != nil {
			fmt.Printf("Warning: %v\n", err)
			fmt.Println("Some commands (delete, scale, etc.) will not work without kubectl.")
			fmt.Println("Continuing with read-only access...")
		fmt.Println()
		}

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
	)

	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running program: %v\n", err)
		os.Exit(1)
	}
}

// checkKubectlAvailable checks if kubectl is available in PATH
func checkKubectlAvailable() error {
	cmd := exec.Command("kubectl", "version", "--client", "--short")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("kubectl not found in PATH\nInstall: https://kubernetes.io/docs/tasks/tools/")
	}
	return nil
}
