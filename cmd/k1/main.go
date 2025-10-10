package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"k8s.io/klog/v2"

	"github.com/renato0307/k1/internal/app"
	"github.com/renato0307/k1/internal/k8s"
	"github.com/renato0307/k1/internal/types"
	"github.com/renato0307/k1/internal/ui"
)

// contextList is a custom flag type to support multiple -context flags
type contextList []string

func (c *contextList) String() string {
	return fmt.Sprintf("%v", *c)
}

func (c *contextList) Set(value string) error {
	*c = append(*c, value)
	return nil
}

var (
	contextFlags contextList
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
	maxContexts := flag.Int("max-contexts", 10, "Maximum number of contexts to keep loaded (1-20)")
	flag.Var(&contextFlags, "context", "Kubernetes context to use (can be specified multiple times)")
	flag.Parse()
	defer klog.Flush()

	// Validate max-contexts range
	if *maxContexts < 1 || *maxContexts > 20 {
		fmt.Println("Error: max-contexts must be between 1 and 20")
		os.Exit(1)
	}

	// Load theme
	theme := ui.GetTheme(*themeFlag)

	// Check if kubectl is available (needed for resource commands)
	if err := checkKubectlAvailable(); err != nil {
		fmt.Printf("Warning: %v\n", err)
		fmt.Println("Some commands (delete, scale, etc.) will not work without kubectl.")
		fmt.Println("Continuing with read-only access...")
		fmt.Println()
	}

	// Determine kubeconfig path
	kubeconfig := *kubeconfigFlag
	if kubeconfig == "" {
		if home := os.Getenv("HOME"); home != "" {
			kubeconfig = filepath.Join(home, ".kube", "config")
		}
	}

	// Determine context list
	contexts := []string(contextFlags)
	if len(contexts) == 0 {
		// No contexts specified - use current from kubeconfig
		currentCtx, err := k8s.GetCurrentContext(kubeconfig)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		contexts = []string{currentCtx}
	}

	fmt.Printf("Connecting to Kubernetes cluster (%s)...\n", contexts[0])
	fmt.Println("Syncing cache...")

	// Create repository pool
	pool, err := k8s.NewRepositoryPool(kubeconfig, *maxContexts)
	if err != nil {
		fmt.Printf("Error initializing pool: %v\n", err)
		os.Exit(1)
	}
	defer pool.Close()

	// Load first context (BLOCKING - must complete before UI)
	progressCh := make(chan k8s.ContextLoadProgress, 10)
	errCh := make(chan error, 1)

	go func() {
		err := pool.LoadContext(contexts[0], progressCh)
		errCh <- err
		close(progressCh)
	}()

	// Show progress for first context
	for progress := range progressCh {
		fmt.Printf("  %s\n", progress.Message)
	}

	if err := <-errCh; err != nil {
		fmt.Printf("Error connecting to context %s: %v\n", contexts[0], err)
		os.Exit(1)
	}

	// Set first context as active
	if err := pool.SetActive(contexts[0]); err != nil {
		fmt.Printf("Error setting active context: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Cache synced! Starting UI...")

	// Create the app model with theme
	model := app.NewModel(pool, theme)

	// Start the Bubble Tea program
	p := tea.NewProgram(
		model,
		tea.WithAltScreen(),
	)

	// Load remaining contexts in background (non-blocking)
	if len(contexts) > 1 {
		go loadBackgroundContexts(pool, contexts[1:], p)
	}

	// Run UI
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running program: %v\n", err)
		os.Exit(1)
	}
}

// loadBackgroundContexts loads contexts after UI starts
func loadBackgroundContexts(pool *k8s.RepositoryPool, contexts []string, program *tea.Program) {
	for _, ctx := range contexts {
		progressCh := make(chan k8s.ContextLoadProgress, 10)

		go func(contextName string) {
			// Send progress messages to UI
			for progress := range progressCh {
				program.Send(types.ContextLoadProgressMsg{
					Context: progress.Context,
					Message: progress.Message,
					Phase:   int(progress.Phase),
				})
			}
		}(ctx)

		err := pool.LoadContext(ctx, progressCh)
		close(progressCh)

		if err != nil {
			program.Send(types.ContextLoadFailedMsg{
				Context: ctx,
				Error:   err,
			})
		} else {
			program.Send(types.ContextLoadCompleteMsg{
				Context: ctx,
			})
		}
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
