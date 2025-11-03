package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"k8s.io/klog/v2"

	"github.com/renato0307/k1/internal/app"
	"github.com/renato0307/k1/internal/k8s"
	"github.com/renato0307/k1/internal/logging"
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
	startTime := time.Now()

	// Suppress klog errors from client-go (RBAC permission errors during watch)
	klog.InitFlags(nil)
	flag.Set("logtostderr", "false")
	flag.Set("stderrthreshold", "FATAL") // Only show FATAL errors
	flag.Set("v", "0")                   // Minimum verbosity

	// Parse flags
	themeFlag := flag.String("theme", "charm", "Theme to use (dark: charm, dracula, catppuccin, nord, gruvbox, tokyo-night, solarized, monokai | light: catppuccin-latte, solarized-light, gruvbox-light)")
	kubeconfigFlag := flag.String("kubeconfig", "", "Path to kubeconfig file (default: $HOME/.kube/config)")
	maxContexts := flag.Int("max-contexts", 10, "Maximum number of contexts to keep loaded (1-20)")
	flag.Var(&contextFlags, "context", "Kubernetes context to use (can be specified multiple times)")

	// Logging flags
	logFile := flag.String("log-file", "", "Path to log file (empty = no logging)")
	logLevel := flag.String("log-level", "info", "Log level (debug, info, warn, error)")
	logFormat := flag.String("log-format", "text", "Log format (text, json)")
	logMaxSize := flag.Int("log-max-size", 100, "Maximum log file size in MB before rotation")
	logMaxBackups := flag.Int("log-max-backups", 3, "Maximum number of old log files to keep")

	flag.Parse()
	defer klog.Flush()

	// Initialize logger
	logConfig := logging.Config{
		FilePath:   *logFile,
		Level:      logging.ParseLevel(*logLevel),
		Format:     logging.ParseFormat(*logFormat),
		MaxSizeMB:  *logMaxSize,
		MaxBackups: *logMaxBackups,
	}
	if err := logging.Init(logConfig); err != nil {
		fmt.Printf("Error initializing logger: %v\n", err)
		os.Exit(1)
	}
	defer logging.Shutdown()

	logging.Info("Starting k1")

	// Validate max-contexts range
	if *maxContexts < 1 || *maxContexts > 20 {
		fmt.Println("Error: max-contexts must be between 1 and 20")
		os.Exit(1)
	}

	// Load theme
	theme := ui.GetTheme(*themeFlag)
	logging.Debug("Config loaded", "duration", time.Since(startTime).String(), "ms", time.Since(startTime).Milliseconds())

	// Check if kubectl is available (needed for resource commands)
	if err := checkKubectlAvailable(); err != nil {
		fmt.Printf("Warning: %v\n", err)
		fmt.Println("Some commands (delete, scale, etc.) will not work without kubectl.")
		fmt.Println("Continuing with read-only access…")
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

	logging.Info("Connecting to Kubernetes cluster", "context", contexts[0])
	fmt.Printf("Connecting to Kubernetes cluster (%s)...\n", contexts[0])
	fmt.Printf("Syncing cache... (config ready in %v)\n", time.Since(startTime))

	// Create repository pool
	poolStart := time.Now()
	pool, err := k8s.NewRepositoryPool(kubeconfig, *maxContexts)
	if err != nil {
		logging.Error("Failed to initialize repository pool", "error", err)
		fmt.Printf("Error initializing pool: %v\n", err)
		os.Exit(1)
	}
	defer pool.Close()
	poolDuration := time.Since(poolStart)
	logging.Debug("Repository pool created", "duration", poolDuration.String(), "ms", poolDuration.Milliseconds())
	fmt.Printf("Repository pool created (took %v)\n", poolDuration)

	// Load first context (BLOCKING - must complete before UI)
	contextLoadStart := time.Now()
	logging.Info("Loading context", "context", contexts[0])
	progressCh := make(chan k8s.ContextLoadProgress, 10)
	errCh := make(chan error, 1)

	go func() {
		err := pool.LoadContext(contexts[0], progressCh)
		errCh <- err
		close(progressCh)
	}()

	// Show progress for first context
	for progress := range progressCh {
		logging.Debug("Context load progress", "phase", progress.Phase, "message", progress.Message)
		fmt.Printf("  %s\n", progress.Message)
	}

	if err := <-errCh; err != nil {
		logging.Error("Failed to load context", "context", contexts[0], "error", err)
		fmt.Printf("Error connecting to context %s: %v\n", contexts[0], err)
		os.Exit(1)
	}
	contextLoadDuration := time.Since(contextLoadStart)
	logging.Info("Context loaded", "context", contexts[0], "duration", contextLoadDuration.String(), "ms", contextLoadDuration.Milliseconds())
	fmt.Printf("Context loaded (took %v)\n", contextLoadDuration)

	// Set first context as active
	if err := pool.SetActive(contexts[0]); err != nil {
		logging.Error("Failed to set active context", "context", contexts[0], "error", err)
		fmt.Printf("Error setting active context: %v\n", err)
		os.Exit(1)
	}

	totalStartupDuration := time.Since(startTime)
	logging.Info("Starting UI", "total_startup_duration", totalStartupDuration.String(), "ms", totalStartupDuration.Milliseconds())
	fmt.Printf("Cache synced! Starting UI... (took %v)\n", totalStartupDuration)

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
