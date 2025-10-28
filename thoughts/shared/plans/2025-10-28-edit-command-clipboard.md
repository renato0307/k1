# Edit Command Implementation Plan

## Overview

Implement a `/edit` command that generates a `kubectl edit` command and
copies it to the clipboard, similar to existing clipboard-based commands like
`/logs`, `/shell`, and `/port-forward`. The command will be available for all
resource screens and will work for both namespaced and cluster-scoped
resources.

## Current State Analysis

The codebase already has two distinct patterns for executing kubectl commands:

1. **Direct execution** (e.g., `/delete`, `/cordon`, `/drain`, `/scale`,
   `/restart`):
   - Uses `KubectlExecutor` to run commands via subprocess
   - Returns success/error messages to the UI
   - Example: `internal/commands/resource.go:DeleteCommand()`

2. **Clipboard copy** (e.g., `/logs`, `/shell`, `/port-forward`):
   - Builds kubectl command string
   - Copies to clipboard using `CopyToClipboard()`
   - Returns success message with the command
   - Example: `internal/commands/pod.go:LogsCommand()`

The edit command needs to follow pattern #2 (clipboard copy) because:
- `kubectl edit` opens an interactive editor (vim/nano)
- Interactive commands cannot be executed in subprocess
- User needs to run it in their own terminal

### Key Discoveries:

- **Clipboard helper**: `internal/commands/clipboard.go:CopyToClipboard()`
  handles copying and returns user-friendly messages
- **Kubectl command builder**: Commands like `LogsCommand()` show how to
  build kubectl commands with kubeconfig/context flags
- **Cluster-scoped detection**: `internal/commands/resource.go:isClusterScoped()`
  identifies resources without namespaces
- **Resource registry**: All 16 resource types defined in
  `internal/k8s/repository.go:10-30`
- **Command registry**: `internal/commands/registry.go` shows how to register
  commands with `ResourceTypes: []k8s.ResourceType{}` for all resources

## Desired End State

After implementation:
1. User selects a resource in any screen (pods, deployments, services, etc.)
2. User types `/edit` in command palette
3. Command builds: `kubectl edit <resource-type> <name> [--namespace <ns>]
   [--kubeconfig <path>] [--context <ctx>]`
4. Command is copied to clipboard
5. Success message shows the command that was copied
6. User can paste and run the command in their terminal
7. User can edit the resource in their preferred editor

### Verification:
- Command appears in palette for all resource screens (16 total)
- Keyboard shortcut ctrl+e works on all resource screens
- Generated command is valid for both namespaced and cluster-scoped resources
- Command includes kubeconfig/context flags when configured
- Clipboard copy works on all platforms (Linux, macOS, Windows)
- Success message clearly shows what was copied

## What We're NOT Doing

- Executing `kubectl edit` directly (not possible due to interactive editor)
- Supporting inline args (edit has no args, just resource selection)
- Creating custom TUI editor (use system kubectl editor config)
- Supporting batch edit of multiple resources

## Implementation Approach

Follow existing clipboard command pattern from `pod.go:LogsCommand()`:
1. Create `EditCommand()` in `internal/commands/resource.go`
2. Register in `internal/commands/registry.go` with `ResourceTypes:
   []k8s.ResourceType{}` (empty = all resources)
3. Build kubectl command string with proper namespace/context handling
4. Copy to clipboard and return success message

## Phase 1: Implement Edit Command

### Overview
Create the core edit command implementation following the clipboard command
pattern.

### Changes Required:

#### 1. Add EditCommand function
**File**: `internal/commands/resource.go`
**Changes**: Add EditCommand function after DeleteCommand

```go
// EditCommand returns execute function for editing a resource (clipboard)
func EditCommand(pool *k8s.RepositoryPool) ExecuteFunc {
	return func(ctx CommandContext) tea.Cmd {
		// Get resource info
		resourceName := "unknown"
		namespace := ""
		if name, ok := ctx.Selected["name"].(string); ok {
			resourceName = name
		}

		// Only set namespace for namespaced resources
		if !isClusterScoped(ctx.ResourceType) {
			namespace = "default"
			if ns, ok := ctx.Selected["namespace"].(string); ok {
				namespace = ns
			}
		}

		// Validate we have active repository first
		repo := pool.GetActiveRepository()
		if repo == nil {
			return messages.ErrorCmd("No active repository")
		}

		// Build kubectl edit command
		var kubectlCmd strings.Builder
		kubectlCmd.WriteString("kubectl edit ")
		kubectlCmd.WriteString(string(ctx.ResourceType))
		kubectlCmd.WriteString(" ")
		kubectlCmd.WriteString(resourceName)

		// Add namespace flag only for namespaced resources
		if namespace != "" {
			kubectlCmd.WriteString(" --namespace ")
			kubectlCmd.WriteString(namespace)
		}

		// Add kubeconfig/context if set
		if repo.GetKubeconfig() != "" {
			kubectlCmd.WriteString(" --kubeconfig ")
			kubectlCmd.WriteString(repo.GetKubeconfig())
		}
		if repo.GetContext() != "" {
			kubectlCmd.WriteString(" --context ")
			kubectlCmd.WriteString(repo.GetContext())
		}

		command := kubectlCmd.String()

		return func() tea.Msg {
			msg, err := CopyToClipboard(command)
			if err != nil {
				return messages.ErrorCmd("Copy failed: %v", err)()
			}
			return messages.InfoCmd("%s", msg)()
		}
	}
}
```

#### 2. Register Edit Command
**File**: `internal/commands/registry.go`
**Changes**: Add edit command to registry after delete command (around line
175)

```go
{
	Name:          "edit",
	Description:   "Edit resource (clipboard)",
	Category:      CategoryAction,
	ResourceTypes: []k8s.ResourceType{}, // Applies to all resource types
	Shortcut:      "ctrl+e",
	Execute:       EditCommand(pool),
},
```

### Success Criteria:

#### Automated Verification:
- [x] Code compiles: `go build ./...`
- [ ] No linting errors: `make lint` (if available)
- [ ] Command registry tests pass: `go test
  ./internal/commands/registry_test.go`

#### Manual Verification:
- [x] Run application: `make run` or `go run cmd/k1/main.go`
- [x] Navigate to Pods screen
- [x] Select a pod
- [x] Type `/edit` in command palette
- [x] Command appears in suggestions
- [x] Press Enter to execute
- [x] Success message shows: "Command copied to clipboard: kubectl edit pods
  <pod-name> --namespace <ns> ..."
- [x] Paste in terminal and verify command is valid (do not execute, just
  verify format)
- [x] Test keyboard shortcut: Press ctrl+e directly
- [x] Verify same success message appears
- [x] Test with Nodes screen (cluster-scoped resource):
  - Select a node
  - Type `/edit`
  - Verify command does NOT include `--namespace` flag
  - Command should be: "kubectl edit nodes <node-name> ..."
- [x] Test with custom kubeconfig/context:
  - Run with flags: `go run cmd/k1/main.go -kubeconfig ~/.kube/config
    -context my-context`
  - Execute `/edit` on any resource
  - Verify command includes `--kubeconfig` and `--context` flags

**Implementation Note**: After completing this phase and all automated
verification passes, pause here for manual confirmation from the human that
the manual testing was successful before considering the implementation
complete.

---

## Testing Strategy

### Unit Tests:
- **Not required for Phase 1**: Edit command is simple clipboard copy
- If needed in future: Test command string building logic
- Mock clipboard operations for deterministic testing

### Integration Tests:
- **Not required**: Clipboard operations are hard to test in CI
- Manual testing is more appropriate for this feature

### Manual Testing Steps:
1. **Basic functionality** (namespaced resource):
   - Run k1 with default config
   - Navigate to Pods screen
   - Select any pod
   - Type `/edit`
   - Press Enter
   - Verify success message shows correct kubectl command
   - Paste command in terminal (do not execute)
   - Verify command format is correct

2. **Cluster-scoped resource** (no namespace):
   - Navigate to Nodes screen
   - Select any node
   - Type `/edit`
   - Press Enter
   - Verify command does NOT include `--namespace` flag
   - Verify command format: `kubectl edit nodes <name>`

3. **With custom kubeconfig/context**:
   - Run: `go run cmd/k1/main.go -kubeconfig ~/.kube/config -context
     my-context`
   - Execute `/edit` on any resource
   - Verify command includes `--kubeconfig ~/.kube/config --context
     my-context`

4. **All resource screens**:
   - Test `/edit` command on at least 5 different resource types:
     - Pods (namespaced)
     - Deployments (namespaced)
     - Services (namespaced)
     - ConfigMaps (namespaced)
     - Nodes (cluster-scoped)
   - Verify command format is correct for each

5. **Error handling**:
   - Try `/edit` with no resource selected (should show error or no-op)
   - Verify clipboard errors are handled gracefully

## Performance Considerations

- **Zero performance impact**: Simple string building and clipboard copy
- **No network calls**: Command is generated client-side
- **No blocking operations**: Clipboard copy is synchronous but fast (<1ms)

## Migration Notes

Not applicable - this is a new feature with no existing data or behavior to
migrate.

## References

- Existing clipboard commands: `internal/commands/pod.go:55-81` (ShellCommand)
- Existing clipboard commands: `internal/commands/pod.go:119-152`
  (LogsCommand)
- Clipboard helper: `internal/commands/clipboard.go:9-15`
  (CopyToClipboard)
- Cluster-scoped detection: `internal/commands/resource.go:17-23`
  (isClusterScoped)
- Delete command pattern: `internal/commands/resource.go:125-174`
  (DeleteCommand)
- Command registration: `internal/commands/registry.go:26-332`
  (NewRegistry)
- Resource types: `internal/k8s/repository.go:10-30` (ResourceType constants)
