// Package messages defines message handling patterns and conventions for the k1
// application. This includes error, success, and info messages. Consistent
// messaging across layers improves code quality, debuggability, and user
// experience.
//
// # Message Handling Patterns by Layer
//
// ## Repository Layer (internal/k8s)
//
// Return standard Go errors. The repository is a pure data access layer and
// should not depend on UI concerns.
//
// Pattern:
//
//	func (r *InformerRepository) GetPods(namespace string) ([]Pod, error) {
//	    pods, err := r.podLister.Pods(namespace).List(labels.Everything())
//	    if err != nil {
//	        return nil, fmt.Errorf("failed to list pods: %w", err)
//	    }
//	    return pods, nil
//	}
//
// Use fmt.Errorf with %w to wrap errors and preserve the error chain for
// debugging. Always provide context about what operation failed.
//
// For new errors (not wrapping), use fmt.Errorf directly:
//
//	if resourceName == "" {
//	    return nil, fmt.Errorf("resource name cannot be empty")
//	}
//
// Helper available: messages.WrapError(err, "context") as a clearer alternative
// to fmt.Errorf("context: %w", err).
//
// ## Command Layer (internal/commands)
//
// Return tea.Cmd that produces a StatusMsg. Commands execute in response to
// user actions and need to communicate results back through the Bubble Tea
// message system.
//
// Pattern:
//
//	func ScaleCommand(repo k8s.Repository) ExecuteFunc {
//	    return func(ctx CommandContext) tea.Cmd {
//	        return func() tea.Msg {
//	            // Execute operation
//	            err := doSomething()
//	            if err != nil {
//	                return types.ErrorStatusMsg(fmt.Sprintf("Scale failed: %v", err))
//	            }
//	            return types.SuccessMsg("Scaled successfully")
//	        }
//	    }
//	}
//
// Use types.ErrorStatusMsg for errors, types.SuccessMsg for success, and
// types.InfoMsg for informational messages. Keep messages concise and
// user-friendly (avoid technical jargon when possible).
//
// ## UI Layer (internal/app, internal/components, internal/screens)
//
// Display errors via the StatusBar component. UI components should not format
// error messages - they receive pre-formatted StatusMsg from commands.
//
// Pattern:
//
//	case types.StatusMsg:
//	    m.statusBar.SetMessage(msg.Message, msg.Type)
//	    return m, tea.Tick(components.StatusBarDisplayDuration, func(t time.Time) tea.Msg {
//	        return types.ClearStatusMsg{}
//	    })
//
// The status bar automatically clears after StatusBarDisplayDuration (5s).
// Status messages are color-coded: green for success, red for errors, blue
// for info.
//
// ## Infrastructure Layer (informer setup, resource fetching)
//
// Log warnings to stderr and continue with partial data when appropriate.
// This prevents the application from failing completely due to RBAC or
// network issues.
//
// Pattern:
//
//	if err := informer.AddEventHandler(handler); err != nil {
//	    fmt.Fprintf(os.Stderr, "Warning: Failed to add event handler: %v\n", err)
//	    // Continue - application can still function with reduced features
//	}
//
// Use this pattern for non-critical failures where degraded functionality is
// acceptable (e.g., RBAC denying access to certain resource types).
//
// # Error Types
//
// Currently we use string-based error messages. Future enhancement could add
// structured error types:
//
//	type NotFoundError struct {
//	    ResourceType string
//	    Name         string
//	    Namespace    string
//	}
//
//	type PermissionDeniedError struct {
//	    Operation    string
//	    ResourceType string
//	}
//
//	type TimeoutError struct {
//	    Operation string
//	    Duration  time.Duration
//	}
//
// Structured errors would enable type-based error handling and better error
// messages. This is a low-priority enhancement.
//
// # Error Message Guidelines
//
// 1. Be specific: "Failed to scale deployment/nginx" not "Operation failed"
// 2. Include context: What operation failed, on what resource
// 3. User-friendly: Avoid stack traces or technical details in UI messages
// 4. Actionable when possible: Suggest what the user should check/fix
// 5. Consistent format: Start with verb describing what failed
//
// Good examples:
//   - "Scale failed: deployment/nginx not found"
//   - "Cordon failed: insufficient permissions for nodes"
//   - "kubectl command timed out after 30s"
//
// Bad examples:
//   - "Error" (too vague)
//   - "panic: runtime error: invalid memory address" (too technical)
//   - "The system encountered an unexpected error" (no actionable info)
//
// # Testing Error Handling
//
// When testing components that handle errors:
//
// 1. Test happy path (no errors)
// 2. Test expected errors (not found, permission denied, etc.)
// 3. Test unexpected errors (network failures, timeouts)
// 4. Verify error messages are user-friendly
// 5. Verify error messages contain sufficient context for debugging
//
// Example test:
//
//	func TestScaleCommand_NotFound(t *testing.T) {
//	    repo := &mockRepository{err: errors.New("not found")}
//	    cmd := ScaleCommand(repo)
//	    msg := cmd(ctx)()
//	    errorMsg, ok := msg.(types.StatusMsg)
//	    assert.True(t, ok)
//	    assert.Equal(t, types.MessageTypeError, errorMsg.Type)
//	    assert.Contains(t, errorMsg.Message, "Scale failed")
//	}
package messages
