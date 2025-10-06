package commands

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"time"
)

// KubectlExecutor runs kubectl commands via subprocess
type KubectlExecutor struct {
	kubeconfig string
	context    string
}

// ExecuteOptions configures kubectl command execution
type ExecuteOptions struct {
	Timeout time.Duration // Command timeout (default: 30s)
}

// NewKubectlExecutor creates a new kubectl executor
func NewKubectlExecutor(kubeconfig, contextName string) *KubectlExecutor {
	return &KubectlExecutor{
		kubeconfig: kubeconfig,
		context:    contextName,
	}
}

// Execute runs a kubectl command and returns output
func (e *KubectlExecutor) Execute(args []string, opts ExecuteOptions) (string, error) {
	// Build command
	cmd := exec.Command("kubectl", args...)

	// Apply kubeconfig if set
	if e.kubeconfig != "" {
		cmd.Args = append(cmd.Args, "--kubeconfig", e.kubeconfig)
	}

	// Apply context if set
	if e.context != "" {
		cmd.Args = append(cmd.Args, "--context", e.context)
	}

	// Set up I/O
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Set timeout (default 30s)
	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	// Run command with timeout
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Start command
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("failed to start kubectl: %w", err)
	}

	// Wait for command to complete or timeout
	done := make(chan error)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case <-ctx.Done():
		// Timeout - kill process
		_ = cmd.Process.Kill()
		return "", fmt.Errorf("kubectl command timed out after %v", timeout)
	case err := <-done:
		// Command completed
		if err != nil {
			stderrStr := stderr.String()
			if stderrStr != "" {
				return "", fmt.Errorf("kubectl error: %s", stderrStr)
			}
			return "", fmt.Errorf("kubectl failed: %w", err)
		}
		return stdout.String(), nil
	}
}

// CheckAvailable checks if kubectl is available in PATH
func CheckAvailable() error {
	cmd := exec.Command("kubectl", "version", "--client", "--short")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("kubectl not found in PATH\nPlease install kubectl: https://kubernetes.io/docs/tasks/tools/")
	}
	return nil
}
