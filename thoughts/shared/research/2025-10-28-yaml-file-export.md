---
date: 2025-10-28T07:03:46+00:00
researcher: Claude
git_commit: 7206c20452f397727d32d009cc2183212f045b13
branch: feat/kubernetes-context-management
repository: k1
topic: "Copy YAML screen contents to file with namespace-name-type naming"
tags: [research, codebase, yaml, export, file-operations, fullscreen]
status: complete
last_updated: 2025-10-28
last_updated_by: Claude
---

# Research: Copy YAML Screen Contents to File with
Namespace-Name-Type Naming

**Date**: 2025-10-28T07:03:46+00:00
**Researcher**: Claude
**Git Commit**: 7206c20452f397727d32d009cc2183212f045b13
**Branch**: feat/kubernetes-context-management
**Repository**: k1

## Research Question

How to implement saving YAML screen contents to a file in the current
k1 folder with a filename based on namespace, name, and resource type
(e.g., `production-nginx-123-pod.yaml`)?

## Summary

The codebase has all the building blocks needed for implementing YAML
file export:

1. **YAML content** is already available via `GetResourceYAML()` in the
   FullScreen component
2. **Resource metadata** (namespace, name, type) flows through
   `CommandContext` to commands
3. **File operations** follow established patterns using `os.WriteFile()`
   with `0644` permissions
4. **Export functionality** is documented in future plans but not yet
   implemented

**Implementation approach**: Add a `/save` command to the FullScreen
component that constructs filename from metadata and writes YAML content
to disk.

## Detailed Findings

### 1. YAML Content Source

**FullScreen Component**
(`internal/components/fullscreen.go:26-34`):
```go
type FullScreen struct {
    viewType     FullScreenViewType  // 0=YAML, 1=Describe, 2=Logs
    resourceName string              // Display name: "namespace/name"
    content      string              // Raw YAML content
    width        int
    height       int
    theme        *ui.Theme
    scrollOffset int
}
```

**Key insight**: The `content` field already contains the full YAML
string that needs to be saved.

**Content retrieval**
(`internal/k8s/resource_formatters.go:19-53`):
- `GetResourceYAML()` uses kubectl's `YAMLPrinter` for authentic output
- Content is identical to `kubectl get <resource> -o yaml`
- Already validated and working in production

### 2. Resource Metadata Access

**Complete data flow** from screen selection to command execution:

**Step 1**: Screen converts struct to map
(`internal/screens/config.go:640-677`):
```go
func (s *ConfigScreen) GetSelectedResource() map[string]interface{} {
    cursor := s.table.Cursor()
    item := s.filtered[cursor]
    result := make(map[string]interface{})

    // Uses reflection to flatten embedded ResourceMetadata
    // Keys are lowercase: "namespace", "name", "age", etc.
    // ...

    return result
}
```

**Step 2**: CommandBar stores selection context
(`internal/components/commandbar/commandbar.go:24-70`):
```go
type CommandBar struct {
    screenID         string              // e.g., "pods"
    selectedResource map[string]any      // From GetSelectedResource()
    // ...
}
```

**Step 3**: Executor builds CommandContext
(`internal/components/commandbar/executor.go:41-48`):
```go
ctx := commands.CommandContext{
    ResourceType: k8s.ResourceType(screenID),  // "pods" -> ResourceTypePod
    Selected:     selectedResource,             // map with metadata
    Args:         args,
}
```

**Step 4**: Commands extract metadata
(`internal/commands/resource.go:28-45`):
```go
resourceName := "unknown"
namespace := ""

if name, ok := ctx.Selected["name"].(string); ok {
    resourceName = name
}

if !isClusterScoped(ctx.ResourceType) {
    namespace = "default"
    if ns, ok := ctx.Selected["namespace"].(string); ok {
        namespace = ns
    }
}
```

**Key patterns**:
- Map keys are **lowercase** (`"namespace"`, `"name"`)
- Cluster-scoped resources (nodes, namespaces) have no namespace
- Default to `"default"` namespace if missing

### 3. File Operation Patterns

**Pattern from test code**
(`internal/app/app_test.go:22,40`):
```go
// Construct path
kubeconfigPath := filepath.Join(t.TempDir(), "kubeconfig")

// Write file
err := os.WriteFile(kubeconfigPath, []byte(kubeconfigContent), 0600)
require.NoError(t, err, "Failed to create test kubeconfig")
```

**Pattern from cross-platform paths**
(`cmd/k1/main.go:44`):
```go
defaultKubeconfig := filepath.Join(os.Getenv("HOME"), ".kube", "config")
```

**Established conventions**:
- Always use `filepath.Join()` for path construction
- Use `os.WriteFile()` for simple file writes
- Permissions: `0600` for sensitive, `0644` for general config
- Error handling: `fmt.Errorf("failed to <action>: %w", err)`

**For YAML export**: Use `0644` permissions (read all, write owner)
since resource YAML is not sensitive data.

### 4. Filename Construction

**Proposed pattern**:
```
<namespace>-<name>-<type>.yaml
```

**Examples**:
- Namespaced pod: `production-nginx-123-pod.yaml`
- Deployment: `default-myapp-deployment.yaml`
- Cluster-scoped node: `node-worker-01-node.yaml` (no namespace prefix)

**Implementation considerations**:

1. **Sanitization needed**: Kubernetes names can contain characters
   invalid in filenames (e.g., `/`, `:`, spaces)
   - Replace `/` with `-`
   - Keep alphanumeric, `-`, `_`, `.`
   - Example: `my/special:pod` → `my-special-pod`

2. **Cluster-scoped resources**: Omit namespace prefix for nodes and
   namespaces
   - Node: `worker-01-node.yaml`
   - Namespace: `production-namespace.yaml`

3. **Collision handling**: If file exists, append counter
   - `production-nginx-123-pod.yaml`
   - `production-nginx-123-pod-2.yaml`
   - Or prompt user for confirmation to overwrite

4. **Resource type suffix**: Use lowercase resource type
   - `ctx.ResourceType` provides canonical name ("pod", "deployment")

### 5. Current Working Directory

**Application runs from**: Current shell directory where `k1` is executed
- User requirement: "file should stay in the currents k1 folder"
- This means: Save to current working directory (`./`)

**Implementation**:
```go
cwd, err := os.Getwd()
if err != nil {
    return messages.ErrorCmd("Failed to get current directory: %v", err)
}

filename := constructFilename(namespace, name, resourceType)
filepath := filepath.Join(cwd, filename)
```

**Alternative**: Let user specify path via command argument
- `/save` - Save to current directory
- `/save /path/to/dir` - Save to specific directory

## Code References

**YAML content source**:
- `internal/components/fullscreen.go:26-34` - FullScreen struct with
  content field
- `internal/k8s/resource_formatters.go:19-53` - GetResourceYAML()
  implementation

**Metadata extraction examples**:
- `internal/commands/resource.go:28-45` - YamlCommand metadata extraction
- `internal/commands/pod.go:42-47` - ShellCommand metadata extraction
- `internal/screens/navigation.go:21-25` - Navigation handler metadata
  extraction

**File operation patterns**:
- `internal/app/app_test.go:22,40` - os.WriteFile() usage
- `cmd/k1/main.go:44` - filepath.Join() for cross-platform paths
- `internal/k8s/kubeconfig_parser_test.go:55-59` - Temporary file creation

**Command implementation patterns**:
- `internal/commands/resource.go:26-73` - YamlCommand structure
- `internal/commands/types.go:24-40` - CommandContext and ExecuteFunc
- `internal/messages/messages.go` - Error/success message helpers

## Architecture Insights

### Message Flow for FullScreen Commands

Currently, FullScreen component only handles navigation keys (scroll,
exit). Adding a save command requires:

1. **New message type** in `internal/types/types.go`:
   ```go
   type SaveYAMLMsg struct {
       Filepath string
       Content  string
   }
   ```

2. **FullScreen Update method**
   (`internal/components/fullscreen.go:56-108`):
   - Add case for `/save` or `ctrl+s` keybinding
   - Extract metadata from `resourceName` field (format: "namespace/name")
   - Construct filename
   - Return command that writes file

3. **App Update method** (`internal/app/app.go:241-247`):
   - Handle `SaveYAMLMsg` to display success/error status

### Alternative: Integrate with Command System

Instead of FullScreen-specific logic, integrate with existing command
system:

**Advantage**: Reuses command bar UI, confirmation flow, error handling
**Disadvantage**: Command bar not currently accessible in full-screen mode

**Implementation would require**:
1. Make command bar available in full-screen mode
2. Add `/save` command to resource commands category
3. Pass FullScreen content and metadata to command execution

**Simpler approach**: Direct keybinding in FullScreen (e.g., `ctrl+s`)
that triggers save without command bar.

### Filename Sanitization Function

**Recommended implementation**:
```go
func sanitizeFilename(s string) string {
    // Replace invalid characters with dash
    s = strings.ReplaceAll(s, "/", "-")
    s = strings.ReplaceAll(s, ":", "-")
    s = strings.ReplaceAll(s, " ", "-")

    // Keep only alphanumeric, dash, underscore, dot
    var result strings.Builder
    for _, r := range s {
        if unicode.IsLetter(r) || unicode.IsNumber(r) ||
           r == '-' || r == '_' || r == '.' {
            result.WriteRune(r)
        }
    }

    return strings.ToLower(result.String())
}
```

**Usage**:
```go
filename := fmt.Sprintf("%s-%s-%s.yaml",
    sanitizeFilename(namespace),
    sanitizeFilename(name),
    sanitizeFilename(string(resourceType)))
```

### Collision Handling Strategies

**Strategy 1: Overwrite silently**
- Simplest implementation
- Risk: User loses existing file without warning

**Strategy 2: Append counter**
- Check if file exists with `os.Stat()`
- Increment counter until unique filename found
- Safe but can create many files

**Strategy 3: Prompt for confirmation**
- Check if file exists
- Use existing confirmation flow (see clipboard commands)
- Best UX but requires command bar integration

**Recommended**: Start with Strategy 2 (append counter), add Strategy 3
later with command bar integration.

## Historical Context (from thoughts/)

### Future Export Plans

**Research document**
(`thoughts/shared/research/2025-10-09-yaml-describe-search-feature.md`):
- Phase 4 includes export features (not yet implemented):
  - `/copy` - Copy entire YAML/describe content to clipboard
  - `/export-json` - Convert YAML to JSON and copy to clipboard
- Uses `atotto/clipboard` library for clipboard operations
- File export not explicitly mentioned in plans

**Log streaming research**
(`thoughts/shared/research/2025-10-26-log-streaming-tui-implementation.md`):
- Open question: "Should we support saving logs to file?"
- Lists "log export/save" as future research item
- Suggests similar patterns could apply to YAML/describe views

**Clipboard implementation**
(`internal/commands/clipboard.go`):
- Already implements `/copy` for table rows
- Pattern can be adapted for full-screen content
- Uses `atotto/clipboard` library

### Related Issues

**Issue #2**
(`thoughts/shared/tickets/issue_2.md`,
`thoughts/shared/plans/2025-10-08-issue-2-add-spec-to-describe.md`):
- Describe output missing spec section (only shows status)
- Uses `unstructured.NestedFieldCopy` pattern for extraction
- Relevant to improving resource detail views

**Container navigation**
(`thoughts/shared/research/2025-10-09-container-navigation.md`):
- Discusses screen vs modal vs full-screen overlay approaches
- Full-screen overlay already used for YAML/describe views
- Navigation patterns and FilterContext design

## Related Research

- `thoughts/shared/research/2025-10-09-yaml-describe-search-feature.md` -
  YAML/describe search and export features (Phase 4 future work)
- `thoughts/shared/research/2025-10-26-log-streaming-tui-implementation.md`
  - Log streaming with potential export functionality
- `thoughts/shared/research/2025-10-09-container-navigation.md` - Full-
  screen component patterns and navigation

## Implementation Recommendations

### Recommended Approach

**Phase 1: Simple file save** (minimum viable feature)
1. Add `ctrl+s` keybinding to FullScreen component
2. Extract namespace/name from `resourceName` field
3. Construct filename: `<namespace>-<name>-<type>.yaml`
4. Write to current working directory with counter for collisions
5. Show success/error message in app status bar

**Phase 2: Command integration** (better UX)
1. Add `/save` command to resource commands
2. Support optional path argument: `/save [directory]`
3. Add confirmation prompt for existing files
4. Integrate with command bar's confirmation flow

**Phase 3: Advanced features** (future)
1. Export to JSON format
2. Save multiple resources (batch selection)
3. User-configurable default export directory
4. Template-based filename patterns

### Code Changes Required

**Minimal implementation** (Phase 1):

1. **Add keybinding** to `internal/components/fullscreen.go:56-108`:
   ```go
   case "ctrl+s":
       return f, f.saveToFile()
   ```

2. **Add save method** to `internal/components/fullscreen.go`:
   ```go
   func (f *FullScreen) saveToFile() tea.Cmd {
       // Parse resourceName (format: "namespace/name" or "name")
       parts := strings.Split(f.resourceName, "/")
       var namespace, name string
       if len(parts) == 2 {
           namespace, name = parts[0], parts[1]
       } else {
           name = parts[0]
       }

       // Construct filename
       filename := constructFilename(namespace, name, f.viewType)

       // Write file
       return func() tea.Msg {
           cwd, err := os.Getwd()
           if err != nil {
               return types.ErrorStatusMsg{Message:
                   fmt.Sprintf("Failed to get directory: %v", err)}
           }

           filepath := filepath.Join(cwd, filename)
           err = os.WriteFile(filepath, []byte(f.content), 0644)
           if err != nil {
               return types.ErrorStatusMsg{Message:
                   fmt.Sprintf("Failed to save file: %v", err)}
           }

           return types.SuccessStatusMsg{Message:
               fmt.Sprintf("Saved to %s", filename)}
       }
   }
   ```

3. **Add helper function**:
   ```go
   func constructFilename(namespace, name string,
                          viewType FullScreenViewType) string {
       // Get resource type string
       resourceType := ""
       switch viewType {
       case FullScreenYAML:
           resourceType = "yaml"
       case FullScreenDescribe:
           resourceType = "describe"
       }

       // Sanitize components
       namespace = sanitizeFilename(namespace)
       name = sanitizeFilename(name)

       // Construct filename
       if namespace != "" {
           return fmt.Sprintf("%s-%s-%s.yaml",
               namespace, name, resourceType)
       }
       return fmt.Sprintf("%s-%s.yaml", name, resourceType)
   }
   ```

4. **Update help text** in `internal/components/fullscreen.go:139`:
   ```go
   helpText := "↑/↓: scroll • PgUp/PgDn: page • g/G: top/bottom • " +
               "ctrl+s: save • esc: close"
   ```

### Testing Considerations

**Unit tests needed**:
1. `sanitizeFilename()` - Test invalid characters, edge cases
2. `constructFilename()` - Test namespaced vs cluster-scoped
3. File write with collision detection
4. Error handling (permission denied, disk full)

**Integration tests**:
1. End-to-end: View YAML → press ctrl+s → verify file created
2. Verify file content matches displayed YAML
3. Test collision handling (save same resource twice)

**Test pattern** (from existing code):
```go
func TestSaveYAML(t *testing.T) {
    tests := []struct {
        name      string
        namespace string
        resource  string
        viewType  FullScreenViewType
        expected  string
    }{
        {"namespaced pod", "production", "nginx-123",
         FullScreenYAML, "production-nginx-123-yaml.yaml"},
        {"cluster node", "", "worker-01",
         FullScreenYAML, "worker-01-yaml.yaml"},
        {"special chars", "my-ns", "app:v1/test",
         FullScreenYAML, "my-ns-app-v1-test-yaml.yaml"},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := constructFilename(tt.namespace, tt.resource,
                                       tt.viewType)
            assert.Equal(t, tt.expected, result)
        })
    }
}
```

## Open Questions

1. **Filename pattern**: Should we include resource type in filename?
   - Current proposal: `production-nginx-123-pod.yaml`
   - Alternative: `production-nginx-123.yaml` (type obvious from content)
   - User preference?

2. **Resource type extraction**: FullScreen component doesn't currently
   store resource type
   - Could add field to FullScreen struct
   - Or infer from viewType (but YAML could be any resource)
   - Need to pass resource type when creating FullScreen

3. **Directory choice**: Always save to current working directory?
   - Some users might want `~/Downloads` or `~/.k1/exports`
   - Add configuration option later?

4. **Collision strategy**: Overwrite, append counter, or prompt?
   - Phase 1: Append counter (safest, no UI changes needed)
   - Phase 2: Add confirmation prompt (better UX)

5. **Describe format**: Save describe output as `.txt` or `.yaml`?
   - Describe output is human-readable text, not valid YAML
   - Should use `.txt` extension
   - Update `constructFilename()` to check viewType

6. **Status message**: Where to display save confirmation?
   - App already has status message system
     (`types.SuccessStatusMsg`, `types.ErrorStatusMsg`)
   - But full-screen mode might not show app status bar
   - Consider inline message in FullScreen component

## Next Steps

**Immediate implementation** (1-2 hours):
1. Add resource type field to FullScreen struct
2. Implement `sanitizeFilename()` helper with tests
3. Implement `constructFilename()` helper with tests
4. Add `ctrl+s` keybinding to FullScreen Update
5. Implement `saveToFile()` method with collision detection
6. Update help text to show ctrl+s shortcut
7. Manual testing with various resource types

**Future enhancements**:
1. Add `/save [path]` command with command bar integration
2. Add confirmation prompt for overwrites
3. Export to JSON format
4. User-configurable export directory
5. Batch export (save multiple selected resources)

**Documentation updates**:
1. Update CLAUDE.md with new feature
2. Add keyboard shortcut to README.md
3. Update help text in application
