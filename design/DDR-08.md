# Pragmatic Command Implementation: kubectl First, Optimize Later

| Metadata | Value                                                |
|----------|------------------------------------------------------|
| Date     | 2025-10-05                                           |
| Author   | @renato0307                                          |
| Status   | Proposed                                             |
| Tags     | commands, kubectl, performance, pragmatism, strategy |

| Revision | Date       | Author      | Info                             |
|----------|------------|-------------|----------------------------------|
| 1        | 2025-10-05 | @renato0307 | Initial design                   |
| 2        | 2025-10-05 | @renato0307 | Add input forms for commands     |

## Context and Problem Statement

k1 needs to implement many resource commands (delete, scale, restart,
cordon, drain, etc.) to provide full cluster management capabilities.
While pure Go implementations using client-go would be fastest, they
require significant development time. How can we deliver features
quickly while maintaining a path to optimal performance?

**Key constraints:**
- AI commands (`/ai`) will generate kubectl commands from natural
  language, requiring kubectl subprocess execution anyway
- Users expect feature completeness before micro-optimizations
- Pure Go implementations require deep client-go knowledge for each
  resource type
- Performance matters, but not as much as having the features

**Key questions:**
- Should all commands use pure Go from the start?
- Can we leverage kubectl CLI initially?
- What's the migration path to optimal performance?
- Which commands need immediate optimization?

## Context

### Current Architecture

**Implemented (Pure Go):**
- `/yaml` - Uses kubectl printers library (~1-5ms, DDR-06)
- `/describe` - Uses kubectl describe library (~5-10ms, DDR-06)

**Planned (Need Implementation):**
- Navigation commands: `:pods`, `:deployments`, etc. (pure Go)
- Resource commands: `/delete`, `/scale`, `/restart`, etc.
- Pod commands: `/logs`, `/shell`, `/port-forward`
- Node commands: `/cordon`, `/drain`
- Service commands: `/endpoints`
- AI commands: `/ai <natural language prompt>`

**AI Command Flow:**
```
User: /ai restart all failing pods in namespace foo
  ↓
LLM: Generates kubectl commands
  ↓
k1: Executes via subprocess (with confirmation)
  ↓
kubectl delete pod <pod-name> --namespace foo
```

Since AI commands require kubectl subprocess capability, we already
need the infrastructure.

### Performance Baselines

**kubectl subprocess:**
- Simple commands: 50-150ms (delete, scale, cordon)
- Complex commands: 100-300ms (drain, port-forward setup)
- Process spawn overhead: ~20-30ms
- Kubeconfig parsing: ~10-20ms
- Network latency: ~20-100ms (varies)

**Pure Go client-go:**
- Simple commands: 20-50ms (API call only)
- Complex commands: 50-150ms (multiple API calls)
- No process overhead
- Shared client and informer cache
- Better error handling and progress reporting

**Trade-off:** kubectl is 2-5x slower, but implements all edge cases
correctly (RBAC, dry-run, output formats, error messages).

## Decision

**Use kubectl subprocess for all resource commands initially, except
yaml and describe. Migrate to pure Go implementations incrementally
based on usage patterns and performance needs.**

### Implementation Strategy

**Phase 1: Quick Delivery (kubectl subprocess)**
- Implement all resource commands using `exec.Command("kubectl", ...)`
- Leverage kubectl's mature error handling and output formatting
- Reuse subprocess infrastructure built for AI commands
- Accept 50-300ms latency for initial release

**Phase 2: Incremental Optimization (pure Go)**
- Profile actual usage to identify hot paths
- Migrate high-frequency commands to pure Go first
- Keep kubectl fallback for edge cases
- Measure performance improvements

### Command Categories

**Category A: Pure Go from Start**
- `/yaml` - Already pure Go (kubectl printers library, DDR-06)
- `/describe` - Already pure Go (kubectl describe library, DDR-06)
- Navigation commands - Pure Go (no kubectl needed, instant)

**Rationale:** These need <10ms response for good UX, and pure Go
solutions already exist.

**Category B: kubectl subprocess Initially**
- `/delete` - Delete resources (can be migrated to client-go)
- `/scale` - Scale deployments/statefulsets
- `/restart` - Restart deployments
- `/cordon` - Mark node unschedulable
- `/drain` - Evict pods from node
- `/endpoints` - Show service endpoints

**Rationale:** Mature kubectl implementations handle all edge cases
(dry-run, force, grace periods, RBAC errors, output formats). We get
100% compatibility for free.

**Category C: kubectl subprocess Preferred**
- `/logs` - Stream pod logs (kubectl handles follow, timestamps, etc.)
- `/shell` - Interactive shell (kubectl handles TTY, resize, etc.)
- `/port-forward` - Port forwarding (kubectl handles reconnects)
- `/ai` - AI-generated commands (requires kubectl by design)

**Rationale:** These involve complex protocols (streaming, TTY,
tunneling) that kubectl handles expertly. Pure Go implementations
would be high-risk with little benefit.

### Command Input Requirements

Many commands require user input parameters. These can be captured
inline via command args (e.g., `/scale 5`) or via a form UI when the
command is invoked without args.

**Commands requiring input:**

| Command         | Input Required       | Type    | Default | Notes                 |
|-----------------|----------------------|---------|---------|---------------------- |
| `/scale`        | Replica count        | Number  | Current | 0-100 validation      |
| `/port-forward` | Port mapping         | String  | None    | Format: local:remote  |
| `/logs`         | Container (optional) | Select  | First   | If multi-container    |
|                 | Tail lines           | Number  | 100     | 0 = all               |
|                 | Follow flag          | Boolean | false   | Stream logs           |
| `/shell`        | Container (optional) | Select  | First   | If multi-container    |
|                 | Shell path           | String  | /bin/sh | Or /bin/bash          |
| `/ai`           | Natural language     | Text    | None    | Required, multi-line  |
| `/delete`       | Confirmation only    | N/A     | N/A     | No inputs needed      |
| `/drain`        | Grace period         | Number  | 30      | Seconds               |
|                 | Force flag           | Boolean | false   | Ignore errors         |
|                 | Ignore daemonsets    | Boolean | true    | Common default        |

**Commands without input:**
- `/yaml` - No input (operates on selected resource)
- `/describe` - No input (operates on selected resource)
- `/restart` - No input (restarts selected deployment)
- `/cordon` - No input (cordons selected node)
- `/endpoints` - No input (shows endpoints for selected service)
- Navigation (`:pods`, etc.) - No input

**Input strategies:**

**Strategy 1: Inline positional args (power users)**

Use positional arguments (not flags) for concise, fast command entry.
Each command defines its arg pattern shown in the palette.

```
/scale 5                         # Scale to 5 replicas
/port-forward 8080:80            # Forward local 8080 to pod 80
/logs nginx 100 true             # Container, tail lines, follow
/drain 30 true                   # Grace period, force
/ai restart all failing pods     # Natural language (rest of line)
```

**Optional args use defaults when omitted:**
```
/logs                            # All defaults (first container, 100 lines, no follow)
/logs nginx                      # Specific container, other defaults
/logs nginx 50                   # Container + tail, follow=false
/logs nginx 50 true              # All specified
```

**Strategy 2: Form UI (default, friendly)**

When command invoked without args, show interactive form:
```
User types: /scale [Enter]
  ↓
Command bar expands to show form:
┌──────────────────────────────────────────┐
│ Scale Deployment                         │
│                                          │
│ Replicas: [___5___]                     │
│                                          │
│ [Enter] Confirm  [ESC] Cancel           │
└──────────────────────────────────────────┘
```

**Hybrid approach:** Support both inline positional args and form UI.
If args provided, parse and execute directly. If args missing, show
form. Power users get speed, new users get guidance.

## Design

### Subprocess Command Interface

```go
// In internal/commands/executor.go

// KubectlExecutor runs kubectl commands via subprocess
type KubectlExecutor struct {
    kubeconfig string
    context    string
}

// Execute runs a kubectl command and returns output
func (e *KubectlExecutor) Execute(
    args []string,
    opts ExecuteOptions,
) (string, error) {

    cmd := exec.Command("kubectl", args...)

    // Apply kubeconfig and context
    if e.kubeconfig != "" {
        cmd.Args = append(cmd.Args,
            "--kubeconfig", e.kubeconfig)
    }
    if e.context != "" {
        cmd.Args = append(cmd.Args,
            "--context", e.context)
    }

    // Set up I/O
    var stdout, stderr bytes.Buffer
    cmd.Stdout = &stdout
    cmd.Stderr = &stderr

    // Run command with timeout
    ctx, cancel := context.WithTimeout(
        context.Background(), opts.Timeout)
    defer cancel()

    if err := cmd.Run(); err != nil {
        return "", fmt.Errorf("kubectl failed: %s: %w",
            stderr.String(), err)
    }

    return stdout.String(), nil
}
```

### Command Implementation Example

```go
// In internal/commands/resource.go

func DeleteCommand(repo k8s.Repository) ExecuteFunc {
    return func(ctx CommandContext) tea.Cmd {
        // Get resource info from context
        namespace := ctx.SelectedNamespace
        name := ctx.SelectedResource
        resourceType := ctx.ResourceType

        // Build kubectl command
        args := []string{
            "delete",
            resourceType,
            name,
            "--namespace", namespace,
            "--output", "json", // Get structured output
        }

        // Execute via subprocess
        executor := NewKubectlExecutor(
            repo.GetKubeconfig(),
            repo.GetContext(),
        )

        output, err := executor.Execute(args, ExecuteOptions{
            Timeout: 30 * time.Second,
        })

        if err != nil {
            return errorCmd(err)
        }

        return successCmd(
            fmt.Sprintf("Deleted %s/%s", resourceType, name))
    }
}
```

### Migration Path Example

When performance becomes critical:

```go
// Before: kubectl subprocess (Category B)
func ScaleCommand(repo k8s.Repository) ExecuteFunc {
    return func(ctx CommandContext) tea.Cmd {
        args := []string{"scale", "deployment", name,
            "--replicas", fmt.Sprint(replicas)}
        output, err := executor.Execute(args, opts)
        // ...
    }
}

// After: pure Go client-go (migrated)
func ScaleCommand(repo k8s.Repository) ExecuteFunc {
    return func(ctx CommandContext) tea.Cmd {
        // Use client-go directly
        deployment, err := repo.GetDeployment(namespace, name)
        if err != nil {
            return errorCmd(err)
        }

        // Update replica count
        deployment.Spec.Replicas = &replicas

        // Apply update via API
        err = repo.UpdateDeployment(deployment)
        if err != nil {
            return errorCmd(err)
        }

        return successCmd(
            fmt.Sprintf("Scaled to %d replicas", replicas))
    }
}
```

**Key insight:** Command interface stays the same. Only implementation
changes. Users see no difference except better performance.

### Portability Strategy

**kubectl availability check:**
```go
func (e *KubectlExecutor) CheckAvailable() error {
    cmd := exec.Command("kubectl", "version", "--client")
    if err := cmd.Run(); err != nil {
        return fmt.Errorf(
            "kubectl not found in PATH: %w\n" +
            "Install: https://kubernetes.io/docs/tasks/tools/",
            err)
    }
    return nil
}
```

Show clear error on startup if kubectl missing. Most k8s users already
have kubectl installed.

### Form UI for Command Input

Commands requiring parameters show a form in the command bar,
expanding the bar height as needed (similar to confirmation state).

#### Command Input Field Definition

```go
// In internal/commands/types.go

// InputFieldType defines the type of input field
type InputFieldType int

const (
    InputTypeText    InputFieldType = iota // Free text input
    InputTypeNumber                        // Integer input with validation
    InputTypeBoolean                       // Checkbox/toggle
    InputTypeSelect                        // Dropdown selection
)

// InputField defines a parameter for a command
type InputField struct {
    Name        string         // Field name (used in Args map)
    Label       string         // Display label
    Type        InputFieldType // Input type
    Required    bool           // Whether field is required
    Default     interface{}    // Default value (string, int, bool, []string)
    Placeholder string         // Placeholder text
    Validation  func(string) error // Custom validation function
    Options     []string       // For Select type only
}

// CommandContext enhancement (update existing type)
type CommandContext struct {
    ResourceType string                 // Type of resource (pods, deployments, etc.)
    Selected     map[string]interface{} // Selected resource data (name, namespace, etc.)
    Args         map[string]string      // UPDATED: Command arguments from form or inline args
}

// Command struct enhancement
type Command struct {
    Name              string          // Short command name
    Description       string          // Human-readable description
    Category          CommandCategory // Command category
    NeedsConfirmation bool            // Confirmation before execution
    InputFields       []InputField    // Input parameters (if any)
    ArgPattern        string          // Display pattern in palette (e.g., "<replicas>")
    Execute           ExecuteFunc     // Execution function
    ResourceTypes     []string        // Applicable resource types
    Shortcut          string          // Keyboard shortcut
}

// Example ArgPatterns:
//   "/scale <replicas>"
//   "/port-forward <local:remote>"
//   "/logs [container] [tail] [follow]"
//   "/drain [grace-period] [force]"
//
// Convention:
//   <arg>  = required positional argument
//   [arg]  = optional positional argument
```

#### Command Bar State Enhancement

```go
// In internal/components/commandbar.go

const (
    StateHidden            CommandBarState = iota
    StateFilter
    StateSuggestionPalette
    StateInput
    StateInputForm         // NEW: Show input form for command params
    StateConfirmation
    StateLLMPreview
    StateResult
)

type CommandBar struct {
    state     CommandBarState
    input     string
    inputType CommandType
    // ... existing fields ...

    // Form state (for StateInputForm)
    formFields     []commands.InputField // Fields to display
    formValues     map[string]string     // Current field values
    formFocusIdx   int                   // Currently focused field
    formValidation map[string]error      // Validation errors per field
}
```

#### Palette Display with ArgPattern

Commands with input fields show their ArgPattern in the palette:

```
┌────────────────────────────────────────────────────────┐
│ /                                                      │
├────────────────────────────────────────────────────────┤
│ > /scale <replicas>              Scale Deployment     │
│   /restart                        Restart Deployment  │
│   /port-forward <local:remote>   Port Forward         │
│   /logs [container] [tail] [follow]  Pod Logs        │
│   /delete                         Delete Resource     │
│   /drain [grace] [force]         Drain Node           │
└────────────────────────────────────────────────────────┘
```

**Rendering logic:**
```go
// In internal/components/commandbar.go

func (cb *CommandBar) renderPaletteItem(cmd Command, selected bool) string {
    // Build display name with arg pattern
    displayName := "/" + cmd.Name
    if cmd.ArgPattern != "" {
        // Strip command name from pattern (avoid "/scale /scale <replicas>")
        pattern := strings.TrimPrefix(cmd.ArgPattern, "/"+cmd.Name)
        displayName += pattern
    }

    // Align description
    nameWidth := 30
    paddedName := lipgloss.NewStyle().
        Width(nameWidth).
        Render(displayName)

    line := paddedName + "  " + cmd.Description

    if selected {
        return selectedStyle.Render(line)
    }
    return normalStyle.Render(line)
}
```

#### Form UI Flow

```
User types: /scale [Enter]
  ↓
Check if command has InputFields
  ↓
YES: Check if args provided after command name
  ↓
NO ARGS: Transition to StateInputForm
  ↓
Display form in command bar (5-10 lines depending on fields)
  ↓
User fills fields (Tab/Shift+Tab to navigate)
  ↓
User presses Enter
  ↓
Validate all fields
  ↓
If valid: Execute command with formValues as Args
If invalid: Show validation errors, stay in form

Alternative path (inline args):
  ↓
HAS ARGS: Parse positional args
  ↓
Validate parsed args
  ↓
If valid: Execute directly (skip form)
If invalid: Show error, stay in input mode
```

#### Form View Example

```
┌──────────────────────────────────────────────────────────┐
│ Scale Deployment: nginx-deployment                       │
│                                                           │
│ Replicas: [___5___]                     ← focused field  │
│                                                           │
│ [Tab] Next field  [Enter] Confirm  [ESC] Cancel         │
└──────────────────────────────────────────────────────────┘
```

**Multi-field example** (`/logs`):

```
┌──────────────────────────────────────────────────────────┐
│ Pod Logs: nginx-abc123                                   │
│                                                           │
│ Container: [nginx        ▾]          (if multi-container) │
│ Tail lines: [100_______]                                 │
│ Follow: [✓]                                              │
│                                                           │
│ [Tab] Next  [Space] Toggle  [Enter] OK  [ESC] Cancel    │
└──────────────────────────────────────────────────────────┘
```

#### Form View Implementation

```go
// In internal/components/commandbar.go

func (cb *CommandBar) viewInputForm() string {
    if cb.pendingCommand == nil || len(cb.formFields) == 0 {
        return ""
    }

    // Styles
    titleStyle := lipgloss.NewStyle().
        Foreground(cb.theme.Primary).
        Bold(true).
        Width(cb.width).
        Padding(0, 1)

    labelStyle := lipgloss.NewStyle().
        Foreground(cb.theme.Foreground).
        Width(15)

    inputStyle := lipgloss.NewStyle().
        Foreground(cb.theme.Primary).
        Background(cb.theme.Background)

    focusedInputStyle := inputStyle.Copy().
        Background(cb.theme.Selection)

    errorStyle := lipgloss.NewStyle().
        Foreground(cb.theme.Error).
        Width(cb.width).
        Padding(0, 1)

    hintStyle := lipgloss.NewStyle().
        Foreground(cb.theme.Subtle).
        Width(cb.width).
        Padding(0, 1)

    // Build form
    lines := []string{}

    // Title
    title := fmt.Sprintf("%s: %s",
        cb.pendingCommand.Description,
        getResourceName(cb.selectedResource))
    lines = append(lines, titleStyle.Render(title))
    lines = append(lines, "")

    // Render each field
    for i, field := range cb.formFields {
        focused := (i == cb.formFocusIdx)
        value := cb.formValues[field.Name]

        // Render based on field type
        switch field.Type {
        case commands.InputTypeText, commands.InputTypeNumber:
            label := labelStyle.Render(field.Label + ":")
            inputStr := value
            if inputStr == "" && field.Placeholder != "" {
                inputStr = field.Placeholder
            }

            style := inputStyle
            if focused {
                style = focusedInputStyle
                inputStr = "[" + inputStr + "_]" // Show cursor
            } else {
                inputStr = "[" + inputStr + "]"
            }

            line := label + " " + style.Render(inputStr)
            lines = append(lines, line)

        case commands.InputTypeBoolean:
            label := labelStyle.Render(field.Label + ":")
            checked := (value == "true")
            checkbox := "[ ]"
            if checked {
                checkbox = "[✓]"
            }

            style := inputStyle
            if focused {
                style = focusedInputStyle
            }

            line := label + " " + style.Render(checkbox)
            lines = append(lines, line)

        case commands.InputTypeSelect:
            label := labelStyle.Render(field.Label + ":")
            displayValue := value
            if displayValue == "" && len(field.Options) > 0 {
                displayValue = field.Options[0]
            }

            style := inputStyle
            if focused {
                style = focusedInputStyle
                displayValue = displayValue + " ▾" // Dropdown indicator
            }

            line := label + " " + style.Render("["+displayValue+"]")
            lines = append(lines, line)
        }

        // Show validation error if exists
        if err, ok := cb.formValidation[field.Name]; ok && err != nil {
            lines = append(lines,
                errorStyle.Render("  ✗ "+err.Error()))
        }
    }

    // Hints
    lines = append(lines, "")
    hints := "[Tab] Next  [Enter] Confirm  [ESC] Cancel"
    lines = append(lines, hintStyle.Render(hints))

    return lipgloss.JoinVertical(lipgloss.Left, lines...)
}
```

#### Form State Handling

```go
// In internal/components/commandbar.go

func (cb *CommandBar) handleInputFormState(msg tea.KeyMsg) (
    *CommandBar, tea.Cmd) {

    switch msg.Type {
    case tea.KeyEsc:
        // Cancel form
        cb.reset()
        return cb, nil

    case tea.KeyTab:
        // Navigate to next field
        cb.formFocusIdx = (cb.formFocusIdx + 1) % len(cb.formFields)
        return cb, nil

    case tea.KeyShiftTab:
        // Navigate to previous field
        cb.formFocusIdx--
        if cb.formFocusIdx < 0 {
            cb.formFocusIdx = len(cb.formFields) - 1
        }
        return cb, nil

    case tea.KeyEnter:
        // Validate and execute
        valid := true
        cb.formValidation = make(map[string]error)

        for _, field := range cb.formFields {
            value := cb.formValues[field.Name]

            // Check required fields
            if field.Required && value == "" {
                cb.formValidation[field.Name] =
                    fmt.Errorf("required")
                valid = false
                continue
            }

            // Run custom validation
            if field.Validation != nil {
                if err := field.Validation(value); err != nil {
                    cb.formValidation[field.Name] = err
                    valid = false
                }
            }
        }

        if !valid {
            return cb, nil // Stay in form, show errors
        }

        // Execute command with form values
        return cb.executeCommandWithForm()

    case tea.KeySpace:
        // Toggle boolean fields
        field := cb.formFields[cb.formFocusIdx]
        if field.Type == commands.InputTypeBoolean {
            current := cb.formValues[field.Name]
            if current == "true" {
                cb.formValues[field.Name] = "false"
            } else {
                cb.formValues[field.Name] = "true"
            }
        }
        return cb, nil

    default:
        // Handle text input for current field
        field := cb.formFields[cb.formFocusIdx]
        if field.Type == commands.InputTypeText ||
            field.Type == commands.InputTypeNumber {

            current := cb.formValues[field.Name]

            switch msg.Type {
            case tea.KeyBackspace:
                if len(current) > 0 {
                    cb.formValues[field.Name] = current[:len(current)-1]
                }
            case tea.KeyRunes:
                cb.formValues[field.Name] = current + string(msg.Runes)
            }
        }
        return cb, nil
    }
}
```

#### Command Definition Examples

```go
// In internal/commands/deployment.go

func ScaleCommand(repo k8s.Repository) *Command {
    return &Command{
        Name:        "scale",
        Description: "Scale Deployment",
        Category:    CategoryAction,
        ArgPattern:  " <replicas>", // Shown in palette: "/scale <replicas>"
        InputFields: []InputField{
            {
                Name:        "replicas",
                Label:       "Replicas",
                Type:        InputTypeNumber,
                Required:    true,
                Default:     nil, // Will be set to current replica count
                Placeholder: "0",
                Validation: func(value string) error {
                    n, err := strconv.Atoi(value)
                    if err != nil {
                        return fmt.Errorf("must be a number")
                    }
                    if n < 0 || n > 100 {
                        return fmt.Errorf("must be 0-100")
                    }
                    return nil
                },
            },
        },
        Execute: func(ctx CommandContext) tea.Cmd {
            replicas := ctx.Args["replicas"] // From form or inline arg
            // Build kubectl command
            args := []string{
                "scale", ctx.ResourceType, ctx.Selected["name"],
                "--namespace", ctx.Selected["namespace"],
                "--replicas", replicas,
            }
            // ... execute kubectl scale ...
        },
        ResourceTypes: []string{"deployments", "statefulsets"},
    }
}

// In internal/commands/pod.go

func PortForwardCommand(repo k8s.Repository) *Command {
    return &Command{
        Name:        "port-forward",
        Description: "Port Forward",
        Category:    CategoryAction,
        ArgPattern:  " <local:remote>", // "/port-forward <local:remote>"
        InputFields: []InputField{
            {
                Name:        "ports",
                Label:       "Port Mapping",
                Type:        InputTypeText,
                Required:    true,
                Placeholder: "8080:80",
                Validation: func(value string) error {
                    // Validate format: local:remote
                    parts := strings.Split(value, ":")
                    if len(parts) != 2 {
                        return fmt.Errorf("format: local:remote")
                    }
                    // Validate port numbers
                    for _, p := range parts {
                        if _, err := strconv.Atoi(p); err != nil {
                            return fmt.Errorf("invalid port number")
                        }
                    }
                    return nil
                },
            },
        },
        Execute: func(ctx CommandContext) tea.Cmd {
            ports := ctx.Args["ports"] // "8080:80" from form or inline
            // Build kubectl command
            args := []string{
                "port-forward",
                fmt.Sprintf("pod/%s", ctx.Selected["name"]),
                "--namespace", ctx.Selected["namespace"],
                ports,
            }
            // ... execute kubectl port-forward ...
        },
        ResourceTypes: []string{"pods"},
    }
}

// In internal/commands/pod.go

func LogsCommand(repo k8s.Repository) *Command {
    return &Command{
        Name:        "logs",
        Description: "Pod Logs",
        Category:    CategoryAction,
        ArgPattern:  " [container] [tail] [follow]", // "/logs [container] [tail] [follow]"
        InputFields: []InputField{
            {
                Name:    "container",
                Label:   "Container",
                Type:    InputTypeSelect,
                Required: false, // Optional, defaults to first container
                Options: nil, // Populated dynamically from pod spec
            },
            {
                Name:        "tail",
                Label:       "Tail Lines",
                Type:        InputTypeNumber,
                Required:    false,
                Default:     100,
                Placeholder: "100",
            },
            {
                Name:     "follow",
                Label:    "Follow",
                Type:     InputTypeBoolean,
                Required: false,
                Default:  false,
            },
        },
        Execute: func(ctx CommandContext) tea.Cmd {
            container := ctx.Args["container"]
            tail := ctx.Args["tail"]
            follow := ctx.Args["follow"] == "true"

            // Build kubectl command
            args := []string{
                "logs",
                ctx.Selected["name"],
                "--namespace", ctx.Selected["namespace"],
            }
            if container != "" {
                args = append(args, "-c", container)
            }
            if tail != "" {
                args = append(args, "--tail", tail)
            }
            if follow {
                args = append(args, "-f")
            }

            // ... execute kubectl logs ...
        },
        ResourceTypes: []string{"pods"},
    }
}
```

// In internal/commands/node.go

func DrainCommand(repo k8s.Repository) *Command {
    return &Command{
        Name:        "drain",
        Description: "Drain Node",
        Category:    CategoryAction,
        ArgPattern:  " [grace-period] [force] [ignore-daemonsets]",
        NeedsConfirmation: true, // Destructive operation
        InputFields: []InputField{
            {
                Name:        "grace",
                Label:       "Grace Period (seconds)",
                Type:        InputTypeNumber,
                Required:    false,
                Default:     30,
                Placeholder: "30",
            },
            {
                Name:     "force",
                Label:    "Force Drain",
                Type:     InputTypeBoolean,
                Required: false,
                Default:  false,
            },
            {
                Name:     "ignore-daemonsets",
                Label:    "Ignore DaemonSets",
                Type:     InputTypeBoolean,
                Required: false,
                Default:  true, // Common safe default
            },
        },
        Execute: func(ctx CommandContext) tea.Cmd {
            grace := ctx.Args["grace"]
            force := ctx.Args["force"] == "true"
            ignoreDaemonsets := ctx.Args["ignore-daemonsets"] == "true"

            // Build kubectl command
            args := []string{
                "drain", ctx.Selected["name"],
                "--grace-period", grace,
            }
            if force {
                args = append(args, "--force")
            }
            if ignoreDaemonsets {
                args = append(args, "--ignore-daemonsets")
            }

            // ... execute kubectl drain ...
        },
        ResourceTypes: []string{"nodes"},
    }
}
```

**Key benefits of positional args approach:**

1. **Concise**: `/scale 5` vs `/scale --replicas 5` (saves keystrokes)
2. **Discoverable**: ArgPattern visible in palette shows expected args
3. **Flexible**: Optional args with sensible defaults
4. **Validating**: Parse-time validation catches errors before execution
5. **Form fallback**: New users get guided forms, power users skip them
6. **TUI-optimized**: Faster than typing flag names in a terminal UI

**Usage patterns:**
```
/scale 5                         # Inline: instant execution
/scale                           # Form: guided input
/logs nginx 100 true             # Multiple args: all specified
/logs nginx                      # Partial args: rest use defaults
/logs                            # No args: all defaults, show form
```

#### Dynamic Field Population

Some fields need dynamic values (e.g., container list from pod spec):

```go
// In internal/components/commandbar.go

func (cb *CommandBar) showInputForm(cmd *commands.Command) tea.Cmd {
    cb.pendingCommand = cmd
    cb.formFields = cmd.InputFields
    cb.formValues = make(map[string]string)
    cb.formFocusIdx = 0
    cb.formValidation = make(map[string]error)

    // Populate defaults and dynamic options
    for i, field := range cb.formFields {
        // Set default value
        if field.Default != nil {
            cb.formValues[field.Name] = fmt.Sprint(field.Default)
        }

        // Populate dynamic options (e.g., container names for pods)
        if field.Type == commands.InputTypeSelect &&
            len(field.Options) == 0 {

            switch field.Name {
            case "container":
                // Extract container names from selected pod
                if containers := getContainerNames(
                    cb.selectedResource); len(containers) > 0 {

                    cb.formFields[i].Options = containers
                    // Set first as default
                    cb.formValues[field.Name] = containers[0]
                }
            }
        }
    }

    // Calculate height based on number of fields
    // Each field: 1 line, plus title (2), blank (1), hints (2) = 5 base
    cb.height = 5 + len(cb.formFields)
    cb.state = StateInputForm

    return nil
}
```

#### Inline Args Support (Power Users)

Support inline args to skip form for experienced users:

```go
// In internal/components/commandbar.go

func (cb *CommandBar) executeCommand(cmdStr string) tea.Cmd {
    // Parse command: "/scale 5" or "/port-forward 8080:80"
    parts := strings.Fields(cmdStr)
    cmdName := strings.TrimPrefix(parts[0], "/")
    args := parts[1:]

    cmd := cb.registry.GetByName(cmdName)
    if cmd == nil {
        return errorCmd("command not found")
    }

    // Check if command needs input
    if len(cmd.InputFields) > 0 {
        // Check if inline args provided
        if len(args) > 0 {
            // Parse inline args and execute directly
            formValues, err := parseInlineArgs(cmd.InputFields, args)
            if err != nil {
                // Validation failed, show error
                return errorCmd(fmt.Sprintf("Invalid args: %v", err))
            }
            // Execute with parsed args
            ctx := buildContext(cb)
            ctx.Args = formValues
            return cmd.Execute(ctx)
        } else {
            // No inline args, show form
            return cb.showInputForm(cmd)
        }
    }

    // No input needed, execute immediately
    return cmd.Execute(buildContext(cb))
}

func parseInlineArgs(
    fields []commands.InputField,
    args []string,
) (map[string]string, error) {

    values := make(map[string]string)

    // Positional mapping: args[0] -> fields[0], args[1] -> fields[1], etc.
    for i, field := range fields {
        if i < len(args) {
            // User provided this arg
            values[field.Name] = args[i]
        } else {
            // User didn't provide, use default
            if field.Required {
                return nil, fmt.Errorf("missing required arg: %s", field.Label)
            }
            if field.Default != nil {
                values[field.Name] = fmt.Sprint(field.Default)
            } else {
                values[field.Name] = "" // Empty for optional without default
            }
        }
    }

    // Validate each field
    for _, field := range fields {
        value := values[field.Name]

        // Run custom validation if present
        if field.Validation != nil {
            if err := field.Validation(value); err != nil {
                return nil, fmt.Errorf("%s: %w", field.Label, err)
            }
        }
    }

    return values, nil
}

// Example usage:
//
// Command: /scale <replicas>
// User types: /scale 5
// Args: ["5"]
// Result: {"replicas": "5"}
//
// Command: /logs [container] [tail] [follow]
// User types: /logs nginx 50
// Args: ["nginx", "50"]
// Result: {"container": "nginx", "tail": "50", "follow": "false"} (default)
//
// Command: /logs [container] [tail] [follow]
// User types: /logs
// Args: []
// Result: {"container": "<first>", "tail": "100", "follow": "false"} (all defaults)
```

**Example usage:**
```
/scale 5              → Execute directly (skip form)
/scale                → Show form
/port-forward 8080:80 → Execute directly
/port-forward         → Show form
```

### Summary: Positional Args Design

**Core principles:**

1. **Positional over flags**: `/scale 5` not `/scale --replicas 5`
   - Shorter, faster typing in TUI context
   - More natural for simple commands

2. **ArgPattern in palette**: Show expected args in command list
   - `<arg>` = required
   - `[arg]` = optional
   - Example: `/logs [container] [tail] [follow]`

3. **Hybrid execution**: Args provided = instant, no args = form
   - Power users: fast inline execution
   - New users: guided form UI
   - No mode switching needed

4. **Partial args support**: Optional args use defaults
   - `/logs` = all defaults
   - `/logs nginx` = container specified, rest default
   - `/logs nginx 50 true` = all specified

5. **Validation everywhere**: Parse-time and form-time
   - Catches errors before kubectl execution
   - Consistent validation logic for both paths

**Implementation checklist:**
- [ ] Add `ArgPattern` field to Command struct
- [ ] Add `InputFields` array to Command struct
- [ ] Update CommandContext.Args to `map[string]string`
- [ ] Add StateInputForm to CommandBar
- [ ] Implement parseInlineArgs() with validation
- [ ] Implement viewInputForm() rendering
- [ ] Implement handleInputFormState() interaction
- [ ] Update renderPaletteItem() to show ArgPattern
- [ ] Update executeCommand() to route args vs form
- [ ] Add command definitions with InputFields

## Consequences

### Positive

- **Fast feature delivery:** Implement 20+ commands in days instead of
  weeks
- **Proven correctness:** kubectl handles all edge cases (RBAC,
  validation, output formats, dry-run, etc.)
- **Consistent with AI commands:** Same subprocess infrastructure
- **Clear migration path:** Can optimize hot paths later with real
  usage data
- **Risk mitigation:** Start simple, optimize based on evidence
- **User value first:** Features > micro-optimizations initially
- **Friendly UX:** Form UI makes commands accessible to new users,
  inline args support power users
- **Validation built-in:** Input forms prevent invalid commands before
  execution

### Negative

- **Performance:** Commands are 2-5x slower than pure Go (50-300ms vs
  20-150ms)
- **External dependency:** Requires kubectl in PATH (but most k8s users
  have it)
- **Process overhead:** ~20-30ms spawn cost per command
- **Less control:** Can't customize error messages or progress
  reporting
- **Binary size:** No benefit from Go's single-binary distribution
- **Form UI complexity:** Adds new command bar state and input handling
  logic (but reuses confirmation pattern)

### Neutral

- **Migration cost:** Each command needs rewrite for pure Go, but can
  be done incrementally
- **Testing complexity:** Need to test both subprocess and pure Go
  implementations during migration
- **Error handling:** kubectl errors are strings, harder to parse than
  Go API errors

## Implementation Notes

### Priority for Migration

Based on expected usage frequency:

**High priority (migrate soon):**
1. `/delete` - Used constantly for cleanup
2. `/scale` - Frequent during debugging
3. `/restart` - Common troubleshooting action

**Medium priority (migrate when needed):**
4. `/cordon` - Occasional maintenance
5. `/drain` - Occasional maintenance
6. `/endpoints` - Debugging network issues

**Low priority (keep kubectl):**
7. `/logs` - Streaming is complex
8. `/shell` - TTY handling is complex
9. `/port-forward` - Tunneling is complex
10. `/ai` - Generates kubectl by design

### Performance Monitoring

Track command execution times:
```go
type CommandMetrics struct {
    CommandName string
    Duration    time.Duration
    Success     bool
}

// Log metrics for analysis
func (e *KubectlExecutor) Execute(...) {
    start := time.Now()
    defer func() {
        metrics.Record(CommandMetrics{
            CommandName: args[0],
            Duration:    time.Since(start),
            Success:     err == nil,
        })
    }()
    // ...
}
```

After 1-2 weeks of real usage, review metrics to prioritize
optimizations.

### kubectl Version Compatibility

Support kubectl 1.28+ (last 3 versions):
- Most stable output formats
- JSON output for parsing
- Common error messages

Check version on startup:
```go
output := exec.Command("kubectl", "version", "--client", "-o", "json")
// Parse and validate version >= 1.28
```

### Testing Strategy

**Subprocess commands:**
- Mock `exec.Command` for unit tests
- Integration tests with real kubectl (local cluster)
- Test error handling (RBAC, not found, invalid input)

**Pure Go commands:**
- Use envtest (real API server)
- Test against informer cache
- Compare output with kubectl (validation)

## Alternative Considered

### Alternative 1: Pure Go from Start

**Approach:** Implement all commands using client-go directly.

**Pros:**
- Best performance (20-150ms)
- No external dependencies
- Full control over errors and output
- Single binary distribution

**Cons:**
- Months of development time
- High risk of bugs and edge cases
- Delayed feature delivery to users
- Requires deep client-go expertise for all resource types

**Why rejected:** Perfect is the enemy of good. Users need features now,
not months from now. Performance can be optimized incrementally based
on real usage data.

### Alternative 2: kubectl for Everything (Including yaml/describe)

**Approach:** Use kubectl subprocess for all commands, no exceptions.

**Pros:**
- Maximally consistent approach
- Minimal code complexity
- Zero risk of output differences

**Cons:**
- yaml and describe are too slow (100-200ms)
- Poor TUI experience (noticeable lag)
- Binary size advantage of Go wasted

**Why rejected:** yaml and describe are used constantly (view details,
debug). 100-200ms feels sluggish in a TUI. Users expect instant
feedback (<50ms). Pure Go achieves 1-10ms with kubectl libraries
(DDR-06).

### Alternative 3: Hybrid with Batching

**Approach:** Use kubectl subprocess but batch multiple commands.

**Example:**
```bash
kubectl delete pod pod1 pod2 pod3 --namespace default
```

**Pros:**
- Amortizes process spawn cost
- Faster bulk operations

**Cons:**
- Complex UX (how to select multiple resources?)
- Error handling harder (which one failed?)
- Not applicable to most commands

**Why rejected:** Adds complexity without solving the core trade-off.
Still need pure Go for best performance. Batching can be added later
if needed.

## Future Enhancements

1. **Command proxy mode:** Run kubectl as persistent subprocess, send
   commands via stdin/stdout. Eliminates spawn overhead.

2. **Gradual migration:** Build migration checklist, track progress
   per command.

3. **Fallback strategy:** If pure Go fails (rare API changes), fallback
   to kubectl subprocess automatically.

4. **Performance dashboard:** Show command execution time distribution,
   identify optimization targets.

5. **Custom error messages:** When migrating to pure Go, improve error
   messages beyond kubectl defaults.

## References

- DDR-05: Command-Enhanced List Browser (command bar architecture,
  state machine, confirmation pattern)
- DDR-06: Resource Detail Commands (yaml/describe use kubectl libraries,
  exceptions to kubectl subprocess strategy)
- [kubectl source code](https://github.com/kubernetes/kubectl)
- [client-go examples](https://github.com/kubernetes/client-go/tree/master/examples)
- PLAN-05: YAML and Describe Commands Implementation
