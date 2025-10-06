# Implementation Plan: kubectl Subprocess Commands (DDR-08)

| Metadata | Value                                          |
|----------|------------------------------------------------|
| Date     | 2025-10-06                                     |
| Author   | @renato0307                                    |
| Status   | Planning                                       |
| DDR      | DDR-08                                         |

## Goal

Implement resource commands (delete, scale, restart, cordon, drain, etc.)
using kubectl subprocess for rapid feature delivery, with a clear path to
optimize hot paths with pure Go later.

## Success Criteria

- ✅ kubectl subprocess executor infrastructure works reliably
- ✅ Input form system allows users to enter command parameters
- ✅ Core commands (delete, scale, restart) work end-to-end
- ✅ Complex commands (logs, port-forward, shell) handle streaming/TTY
- ✅ Node commands (cordon, drain) work with confirmation
- ✅ Inline positional args work for power users
- ✅ Error messages from kubectl are properly surfaced to users

## Key Decisions

**Pragmatic approach**: Start with kubectl subprocess for all resource
commands except yaml/describe (already pure Go). Migrate to client-go
incrementally based on usage data.

**Positional args over flags**: `/scale 5` not `/scale --replicas 5` for
faster typing in TUI context.

**Hybrid UX**: Support both inline args (power users) and form UI
(discoverability for new users).

**Use huh for forms**: Leverage github.com/charmbracelet/huh for form UI
instead of building custom forms. Reduces code, better UX, maintained by
Charmbracelet team.

## Code Issues to Address

### 1. CommandContext.Args Needs Better Abstraction
**Current**: `Args string` - simple but not maintainable for complex forms
**Needed**: Struct tags approach (like JSON parsing)

**New approach**: Each command defines typed args struct with tags:
```go
type ScaleArgs struct {
    Replicas int `form:"replicas" title:"Replicas" validate:"min=0,max=100" default:"1"`
}

func ScaleCommand(repo k8s.Repository) ExecuteFunc {
    return func(ctx CommandContext) tea.Cmd {
        var args ScaleArgs
        if err := ctx.ParseArgs(&args); err != nil {
            return errorCmd(err)
        }
        replicas := args.Replicas  // Type-safe int, not string!
    }
}
```

**Benefits**:
- Type-safe (no string parsing errors)
- Self-documenting (struct is source of truth)
- No magic strings scattered in code
- IDE autocomplete for field names
- Validation in tags (declarative)
- Auto-generates InputFields from struct tags

**Need to implement**:
- `internal/commands/args.go` - Reflection helpers
- `GenerateInputFields(argsStruct)` - Reads tags → InputFields
- `ParseInlineArgs(argsStruct, string)` - Positional args → struct
- `ParseFormArgs(argsStruct, form)` - huh.Form values → struct
- `CommandContext.ParseArgs(dest)` - Convenience wrapper

**Supported struct tags**:
```go
type Args struct {
    Field type `form:"name" title:"Display" type:"input|select|confirm" validate:"rules" default:"val" optional:"true"`
}
```
- `form:"name"` - Form field identifier (required)
- `title:"..."` - Display label in form (required)
- `type:"..."` - Field type: input, select, confirm (auto-inferred from Go type)
- `validate:"..."` - Validation rules: `min=X,max=Y,required,portmap,etc`
- `default:"..."` - Default value
- `optional:"true"` - Field is optional for inline args (allows partial args)

### 2. Repository Doesn't Store Kubeconfig/Context
**Issue**: InformerRepository receives kubeconfig/context but doesn't store
them. KubectlExecutor needs these to pass to kubectl subprocess.

**Solutions**:
- Option A: Store in Repository, add GetKubeconfig()/GetContext() methods
- Option B: Store at app level, pass to KubectlExecutor separately
- **Recommendation**: Option A - cleaner, Repository knows its config

### 3. No Form Integration Yet
**Missing**:
- StateInputForm constant in CommandBar (for expanded form state)
- huh.Form field in CommandBar struct
- Bridge function to convert InputFields → huh.Form
- Form rendering in command bar (viewInputForm with huh)
- Form height calculation for command bar expansion (like confirmation)
- Form completion handling and result extraction

### 4. No kubectl Availability Check
**Need**: Check kubectl exists at startup, show helpful error if missing.

## Implementation Phases

### Phase 1: Foundation (3-4 hours)

**Goal**: Build kubectl executor and struct tags args infrastructure.

**Tasks**:
1. Implement struct tags args system
   - Create internal/commands/args.go with reflection helpers
   - Implement GenerateInputFields(argsStruct) - reads tags → InputFields
   - Implement ParseInlineArgs(argsStruct, string) - positional → struct
   - Implement ParseFormArgs(argsStruct, form) - huh values → struct
   - Add CommandContext.ParseArgs(dest) convenience method
   - Define supported tags: form, title, type, validate, default, optional

2. Update Command struct
   - Add ArgsType interface{} field (pointer to args struct)
   - Keep ArgPattern string for palette display
   - Command can generate InputFields from ArgsType on-demand

3. Store kubeconfig/context in Repository
   - Add fields to InformerRepository struct
   - Add GetKubeconfig() and GetContext() to Repository interface
   - Update DummyRepository too

4. Implement KubectlExecutor
   - Create internal/commands/executor.go
   - Implement Execute() with timeout, kubeconfig, context
   - Add CheckAvailable() for kubectl detection
   - Handle stdout/stderr properly

5. Add kubectl availability check to main.go
   - Check at startup if not using -dummy
   - Show helpful error message with install link

**Deliverable**: Struct tags work, KubectlExecutor works, can run simple
kubectl commands

### Phase 2: Input Forms with huh (1-2 hours)

**Goal**: Enable users to enter command parameters via form UI in expanded
command bar.

**Tasks**:
1. Add huh dependency
   - Run: go get github.com/charmbracelet/huh

2. Add StateInputForm to CommandBar
   - Add StateInputForm constant
   - Add huh.Form field to CommandBar struct
   - Implement buildHuhForm() to convert InputFields → huh.Form
   - Calculate form height dynamically (huh forms are variable height)

3. Integrate huh into command bar
   - Add viewInputForm() that renders huh.Form.View()
   - Add handleInputFormState() that passes keys to huh.Form.Update()
   - Handle form completion (huh.Form.State == huh.StateCompleted)
   - Extract values using ParseFormArgs() and execute command

4. Wire up struct tags to command execution
   - When command has ArgsType: generate InputFields via GenerateInputFields()
   - Create huh.Form from generated InputFields
   - On inline args: call ParseInlineArgs(argsType, args string)
   - On form complete: call ParseFormArgs(argsType, huh.Form)

5. Update palette rendering for ArgPattern
   - Show ArgPattern in command list (e.g., "/scale <replicas>")
   - Use <arg> for required, [arg] for optional

6. Route command execution through form or inline
   - Check if command has InputFields
   - If inline args provided -> parse and execute
   - If no args -> create huh.Form and expand command bar

**Deliverable**: Users can enter params via huh form (expanded command bar)
or inline args

**Key insight**: huh.Form is a tea.Model, so it integrates naturally into
command bar. Command bar expands (like confirmation state) to show the
form, user fills it, form completes, command bar extracts values and
executes.

**UI Flow**:
```
User types: /scale [Enter]
  ↓
CommandBar checks: command has InputFields but no inline args
  ↓
CommandBar.state = StateInputForm
CommandBar.height expands (like confirmation: 5-8 lines)
CommandBar.huhForm = buildHuhForm(command.InputFields)
  ↓
viewInputForm() renders huhForm.View() in expanded command bar
  ↓
User fills form (huh handles all input)
  ↓
User presses Enter on last field
  ↓
huhForm.State() == huh.StateCompleted
  ↓
Extract values from huhForm, build CommandContext.Args
  ↓
Execute command with args
  ↓
CommandBar.state = StateHidden (collapse)
```

### Phase 3: Core Commands (2-3 hours)

**Goal**: Implement high-priority resource commands with struct tags.

**Tasks**:
1. Implement /delete command
   - No args needed (operates on selected resource)
   - Confirmation already works
   - Simple kubectl delete execution
   - Return success/error message

2. Implement /scale command with struct tags
   - Define ScaleArgs struct:
   ```go
   type ScaleArgs struct {
       Replicas int `form:"replicas" title:"Replicas" validate:"min=0,max=100" default:"1"`
   }
   ```
   - Use ctx.ParseArgs(&args) in Execute function
   - ArgPattern: " <replicas>"
   - Build kubectl scale command with args.Replicas
   - Apply to deployments and statefulsets

3. Implement /restart command
   - No args needed (operates on selected deployment)
   - Use kubectl rollout restart
   - Show progress message

4. Update command definitions in registry
   - Add ArgsType to commands that need arguments
   - Add ArgPattern to all commands for palette display

**Example pattern**:
```go
// In internal/commands/deployment.go
type ScaleArgs struct {
    Replicas int `form:"replicas" title:"Replicas" validate:"min=0,max=100" default:"1"`
}

func ScaleCommand(repo k8s.Repository) *Command {
    return &Command{
        Name:        "scale",
        Description: "Scale Deployment",
        Category:    CategoryAction,
        ArgsType:    &ScaleArgs{},  // Struct pointer
        ArgPattern:  " <replicas>",
        Execute: func(ctx CommandContext) tea.Cmd {
            var args ScaleArgs
            if err := ctx.ParseArgs(&args); err != nil {
                return errorCmd(err)
            }

            // Type-safe: args.Replicas is int
            kubectlArgs := []string{
                "scale", ctx.ResourceType,
                ctx.Selected["name"].(string),
                "--replicas", strconv.Itoa(args.Replicas),
            }
            // ... execute kubectl
        },
    }
}
```

**Deliverable**: delete, scale, restart work end-to-end with type-safe args

### Phase 4: Complex Commands (3-4 hours)

**Goal**: Handle streaming and interactive commands with struct tags.

**Tasks**:
1. Implement /logs command with struct tags
   - Define LogsArgs struct:
   ```go
   type LogsArgs struct {
       Container string `form:"container" title:"Container" type:"select" optional:"true"`
       Tail      int    `form:"tail" title:"Tail Lines" default:"100" validate:"min=0"`
       Follow    bool   `form:"follow" title:"Follow" default:"false"`
   }
   ```
   - ArgPattern: " [container] [tail] [follow]"
   - Dynamic field population (container names from pod spec)
   - Handle follow mode (streaming output)
   - Show in full-screen view

2. Implement /shell command with struct tags
   - Define ShellArgs struct:
   ```go
   type ShellArgs struct {
       Container string `form:"container" title:"Container" type:"select" optional:"true"`
       Shell     string `form:"shell" title:"Shell" default:"/bin/sh"`
   }
   ```
   - Exit k1 TUI temporarily
   - Run kubectl exec -it with proper TTY
   - Return to k1 after shell exits

3. Implement /port-forward command with struct tags
   - Define PortForwardArgs struct:
   ```go
   type PortForwardArgs struct {
       Ports string `form:"ports" title:"Port Mapping" validate:"portmap"`
   }
   ```
   - Custom validation for "local:remote" format
   - Run in background, show connection status
   - Keep running until user cancels

**Deliverable**: logs, shell, port-forward work with streaming/TTY and
type-safe args

### Phase 5: Node & Service Commands (1-2 hours)

**Goal**: Complete remaining resource-specific commands with struct tags.

**Tasks**:
1. Implement /cordon command
   - No args needed
   - kubectl cordon <node>
   - No confirmation needed (non-destructive)

2. Implement /drain command with struct tags
   - Define DrainArgs struct:
   ```go
   type DrainArgs struct {
       GracePeriod       int  `form:"grace" title:"Grace Period (seconds)" default:"30"`
       Force             bool `form:"force" title:"Force Drain" default:"false"`
       IgnoreDaemonsets  bool `form:"ignore-daemonsets" title:"Ignore DaemonSets" default:"true"`
   }
   ```
   - ArgPattern: " [grace-period] [force] [ignore-daemonsets]"
   - Confirmation already works (NeedsConfirmation: true)
   - Show progress during drain

3. Implement /endpoints command
   - No args needed
   - kubectl get endpoints <service>
   - Show in full-screen view or command bar result

**Deliverable**: All node and service commands work with type-safe args

### Phase 6: Testing & Polish (2-3 hours)

**Goal**: Ensure reliability and good error handling.

**Tasks**:
1. Add executor tests
   - Mock exec.Command for unit tests
   - Test error handling (command not found, timeout, RBAC errors)
   - Test kubeconfig/context passing

2. Test error scenarios
   - kubectl not installed
   - Invalid kubeconfig/context
   - RBAC permission denied
   - Resource not found
   - Validation errors in form

3. Add performance monitoring
   - Log command execution times
   - Track which commands are used most
   - Identify candidates for pure Go migration

4. Update documentation
   - Update CLAUDE.md with new command usage
   - Document positional args format
   - Add troubleshooting section

**Deliverable**: Robust, well-tested command execution

## Migration Strategy

After 1-2 weeks of usage, review metrics to identify hot paths:

**High-frequency commands** (>10 uses/day):
- Migrate to pure Go client-go for best performance
- Example: /delete, /scale

**Medium-frequency commands** (1-10 uses/day):
- Keep kubectl subprocess, optimize if needed
- Example: /restart, /cordon

**Low-frequency commands** (<1 use/day):
- Keep kubectl subprocess indefinitely
- Example: /drain, /port-forward

**Keep kubectl forever**:
- /logs, /shell (complex streaming/TTY)
- /ai (generates kubectl by design)

## Risks & Mitigations

**Risk**: kubectl not installed
**Mitigation**: Check at startup, show clear error with install link

**Risk**: Command timeout/hang
**Mitigation**: Set 30s timeout for all commands, allow cancel

**Risk**: Poor error messages from kubectl
**Mitigation**: Parse stderr, provide context in error display

**Risk**: Breaking change to CommandContext.Args
**Mitigation**: Update all command implementations in single commit

**Risk**: Form UI complexity
**Mitigation**: Reuse confirmation pattern, keep form simple

## TODO

### Phase 1: Foundation (3-4 hours)
- [ ] Implement struct tags args system (internal/commands/args.go)
- [ ] Implement GenerateInputFields(argsStruct) reflection helper
- [ ] Implement ParseInlineArgs(argsStruct, string) helper
- [ ] Implement ParseFormArgs(argsStruct, form) helper
- [ ] Add CommandContext.ParseArgs(dest) convenience method
- [ ] Update Command struct to include ArgsType field
- [ ] Store kubeconfig/context in Repository (add GetKubeconfig/GetContext)
- [ ] Implement KubectlExecutor (internal/commands/executor.go)
- [ ] Add kubectl availability check to main.go

### Phase 2: Input Forms with huh
- [ ] Add huh dependency (go get github.com/charmbracelet/huh)
- [ ] Add StateInputForm and huh.Form to CommandBar
- [ ] Implement buildHuhForm() to convert InputFields → huh.Form
- [ ] Integrate huh rendering and updates in command bar
- [ ] Handle form completion and value extraction
- [ ] Implement inline args parsing
- [ ] Update palette rendering for ArgPattern
- [ ] Route execution through form or inline

### Phase 3: Core Commands
- [ ] Implement /delete command
- [ ] Implement /scale command with InputField
- [ ] Implement /restart command
- [ ] Update command registry with InputFields/ArgPattern

### Phase 4: Complex Commands
- [ ] Implement /logs command with streaming
- [ ] Implement /shell command with TTY
- [ ] Implement /port-forward command

### Phase 5: Node & Service Commands
- [ ] Implement /cordon command
- [ ] Implement /drain command with options
- [ ] Implement /endpoints command

### Phase 6: Testing & Polish
- [ ] Add executor tests
- [ ] Test error scenarios
- [ ] Add performance monitoring
- [ ] Update documentation

## Notes

- Do NOT run tests during implementation (per CLAUDE.md prototype
  guidelines)
- Keep commits atomic per phase
- Create branch: feat/plan-06-kubectl-commands
- Each phase should compile and be testable independently
- Avoid over-engineering - focus on pragmatic delivery
- **huh saves time**: Phase 2 reduced from 2-3 hours to 1-2 hours by using
  huh instead of custom forms.
- **Struct tags add time but huge value**: Phase 1 increases from 2-3 to
  3-4 hours for reflection helpers, but eliminates magic strings and
  provides type safety across all commands.
- **Total plan**: 12-18 hours (Phase 1: 3-4h, Phase 2: 1-2h, Phase 3: 2-3h,
  Phase 4: 3-4h, Phase 5: 1-2h, Phase 6: 2-3h)
- **Command bar expansion**: Forms appear in expanded command bar (5-8
  lines), NOT overlays/modals. Follows existing confirmation pattern.
- **Struct tags pattern**: Each command defines typed args struct with form
  tags. Reflection helpers auto-generate forms and parse values. Type-safe,
  self-documenting, no magic strings.
