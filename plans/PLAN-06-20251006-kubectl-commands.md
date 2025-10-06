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
- ✅ All commands work with inline positional args (power users)
- ✅ Core commands (delete, scale, restart) work end-to-end
- ✅ Node commands (cordon, drain) work with confirmation
- ✅ Service commands (endpoints) work
- ✅ Complex commands (shell, logs, port-forward) copy command to clipboard
- ✅ Error messages from kubectl are properly surfaced to users
- ✅ Form support added incrementally after inline args work

**Clipboard mode for complex commands**: /shell, /logs, /port-forward
generate kubectl command and copy to clipboard instead of executing
directly. User can run in separate terminal. Avoids complex TUI suspend,
streaming, and background process management.

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

**Phased delivery**: Implement inline args first for all commands (deliver
working app for power users), then add forms incrementally (improve UX for
new users). Enables faster iteration and early feedback.

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

**Strategy**: Implement inline args first (Phases 1-4), then add form
support (Phases 5-6). This delivers a working app faster and enables early
testing with real usage.

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

### Phase 2: Core Commands with Inline Args Only (2-3 hours)

**Goal**: Implement delete, scale, restart commands using inline positional
args only. No forms yet - deliver working commands for power users first.

**Tasks**:
1. Implement /delete command
   - No args needed (operates on selected resource)
   - Confirmation already works
   - Simple kubectl delete execution
   - Return success/error message

2. Implement /scale command with inline args
   - Define ScaleArgs struct (as documented in Phase 3 example)
   - Implement ParseInlineArgs for "/scale 5" format
   - ArgPattern: " <replicas>" (show in palette)
   - Build kubectl scale command
   - Apply to deployments and statefulsets
   - Test: `/scale 5` scales to 5 replicas

3. Implement /restart command
   - No args needed
   - Use kubectl rollout restart
   - Show progress message

4. Wire up inline args parsing in CommandBar
   - Parse command string: "/scale 5" → cmd="scale", args="5"
   - If command has ArgsType → call ParseInlineArgs(argsType, args)
   - Pass parsed struct to Execute function via CommandContext
   - Execute command with type-safe args

5. Update palette to show ArgPattern
   - Display "/scale <replicas>" in command list
   - Use <arg> for required args

**Deliverable**: delete, scale, restart work with inline args only. App is
functional for power users who can type `/scale 5` directly.

**Note**: Forms deferred to Phase 5. Focus on working implementation first.

### Phase 3: Node & Service Commands with Inline Args (1-2 hours)

**Goal**: Implement node and service commands with inline args only.

**Tasks**:
1. Implement /cordon command
   - No args needed
   - kubectl cordon <node>
   - No confirmation (non-destructive)
   - Test on nodes screen

2. Implement /drain command with inline args
   - Define DrainArgs struct:
   ```go
   type DrainArgs struct {
       GracePeriod      int  `form:"grace" title:"Grace Period (seconds)" default:"30"`
       Force            bool `form:"force" title:"Force Drain" default:"false"`
       IgnoreDaemonsets bool `form:"ignore-daemonsets" title:"Ignore DaemonSets" default:"true"`
   }
   ```
   - ArgPattern: " [grace-period] [force] [ignore-daemonsets]"
   - Parse inline: "/drain 60 true true" → grace=60, force=true, ignore=true
   - Optional args use defaults: "/drain" uses all defaults
   - Confirmation already works (NeedsConfirmation: true)

3. Implement /endpoints command
   - No args needed
   - kubectl get endpoints <service>
   - Show in full-screen view

**Deliverable**: Node and service commands work with inline args

### Phase 4: Clipboard Commands (1-2 hours)

**Goal**: Implement complex commands in "clipboard mode" - generate kubectl
command and copy to clipboard for user to run in separate terminal.

**Pattern**: For commands that would require complex implementation (TTY,
streaming, background processes), generate the kubectl command string and
copy to clipboard. Show message: "Command copied to clipboard: kubectl
exec -it pod-name -- /bin/sh"

**Tasks**:
1. Add clipboard library
   - Use: github.com/atotto/clipboard (cross-platform)
   - Implement CopyToClipboard(text string) helper

2. Implement /shell command in clipboard mode
   - Define ShellArgs struct:
   ```go
   type ShellArgs struct {
       Container string `form:"container" title:"Container" optional:"true"`
       Shell     string `form:"shell" title:"Shell" default:"/bin/sh"`
   }
   ```
   - ArgPattern: " [container] [shell]"
   - Parse inline: "/shell nginx /bin/bash" or "/shell" (defaults)
   - Generate command: "kubectl exec -it <pod> -c <container> -- <shell>"
   - Copy to clipboard and show message

3. Implement /logs command in clipboard mode
   - Define LogsArgs struct:
   ```go
   type LogsArgs struct {
       Container string `form:"container" title:"Container" optional:"true"`
       Tail      int    `form:"tail" title:"Tail Lines" default:"100" validate:"min=0"`
       Follow    bool   `form:"follow" title:"Follow" default:"false"`
   }
   ```
   - ArgPattern: " [container] [tail] [follow]"
   - Generate: "kubectl logs <pod> -c <container> --tail=100 -f"
   - Copy to clipboard

4. Implement /port-forward command in clipboard mode
   - Define PortForwardArgs struct:
   ```go
   type PortForwardArgs struct {
       Ports string `form:"ports" title:"Port Mapping" validate:"portmap"`
   }
   ```
   - ArgPattern: " <local:remote>"
   - Generate: "kubectl port-forward <pod> 8080:80"
   - Copy to clipboard

**Deliverable**: /shell, /logs, /port-forward generate kubectl commands and
copy to clipboard. User can run in separate terminal.

**Future pattern**: Use clipboard mode for any command that would require
complex implementation (keep k1 simple, leverage kubectl).

### Phase 5: Add huh Form Support (2-3 hours)

**Goal**: Add form UI infrastructure so commands can show forms when no
inline args provided. Improves UX for new users.

**Tasks**:
1. Add huh dependency
   - Run: go get github.com/charmbracelet/huh

2. Implement form generation from struct tags
   - Update GenerateInputFields() to create huh-compatible fields
   - Implement BuildHuhForm(argsStruct) → huh.Form
   - Map struct tags to huh field types (Input, Select, Confirm)
   - Wire up default values and validation

3. Add StateInputForm to CommandBar
   - Add StateInputForm constant
   - Add huh.Form field to CommandBar struct
   - Calculate form height dynamically

4. Integrate huh into command bar
   - Add viewInputForm() that renders huh.Form.View()
   - Add handleInputFormState() that passes keys to huh.Form.Update()
   - Handle form completion (huh.Form.State == huh.StateCompleted)
   - Extract values using ParseFormArgs() and execute command

5. Route execution through inline args or form
   - If inline args provided → ParseInlineArgs() and execute
   - If no args and ArgsType exists → BuildHuhForm() and expand command bar
   - Show "Press Enter to show form" hint if needed

**Deliverable**: Form infrastructure works, commands can show forms when
invoked without args (e.g., `/scale` shows form, `/scale 5` executes
directly)

### Phase 6: Incremental Form Addition (1-2 hours)

**Goal**: Add form support to commands one by one, test each thoroughly.

**Tasks**:
1. Test /scale with both inline and form
   - Test: `/scale 5` → executes immediately
   - Test: `/scale` → shows form with replica input
   - Verify form validation (0-100 range)
   - Verify form defaults work

2. Test /drain with both inline and form
   - Test: `/drain 60 true` → executes with args
   - Test: `/drain` → shows form with 3 fields
   - Verify optional args work
   - Verify boolean checkboxes work

3. Test /shell with form
   - Test: `/shell nginx /bin/bash` → inline args
   - Test: `/shell` → shows form with container select + shell input
   - Verify container list populates dynamically from pod spec

4. Document form patterns
   - Add examples to CLAUDE.md
   - Document struct tag options
   - Show inline vs form usage for each command

**Deliverable**: All commands support both inline args and forms, well
tested and documented

### Phase 7: Testing & Polish (1-2 hours)

**Goal**: Ensure reliability and good error handling.

**Tasks**:
1. Test error scenarios
   - kubectl not installed
   - Invalid kubeconfig/context
   - RBAC permission denied
   - Resource not found
   - Invalid inline args (validation errors)

2. Add performance monitoring
   - Log command execution times
   - Track which commands are used most
   - Identify candidates for pure Go migration

3. Test all commands end-to-end
   - Manual testing on real cluster
   - Verify confirmations work
   - Verify error messages are clear

4. Update documentation
   - Update CLAUDE.md with command usage patterns
   - Document struct tags approach
   - Add troubleshooting section

**Deliverable**: Robust, well-tested command execution with good error
handling

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

### Phase 2: Core Commands with Inline Args Only (2-3 hours)
- [ ] Implement /delete command (no args)
- [ ] Implement /scale command with ScaleArgs struct
- [ ] Implement ParseInlineArgs for "/scale 5" format
- [ ] Implement /restart command (no args)
- [ ] Wire up inline args parsing in CommandBar
- [ ] Update palette to show ArgPattern

### Phase 3: Node & Service Commands with Inline Args (1-2 hours)
- [ ] Implement /cordon command (no args)
- [ ] Implement /drain command with DrainArgs struct
- [ ] Parse inline: "/drain 60 true" with optional args
- [ ] Implement /endpoints command (no args)

### Phase 4: Clipboard Commands (1-2 hours)
- [ ] Add clipboard library (github.com/atotto/clipboard)
- [ ] Implement CopyToClipboard helper
- [ ] Implement /shell in clipboard mode (generate kubectl exec command)
- [ ] Implement /logs in clipboard mode (generate kubectl logs command)
- [ ] Implement /port-forward in clipboard mode (generate kubectl port-forward)
- [ ] Test clipboard functionality on macOS/Linux

### Phase 5: Add huh Form Support (2-3 hours)
- [ ] Add huh dependency (go get github.com/charmbracelet/huh)
- [ ] Implement BuildHuhForm(argsStruct) → huh.Form
- [ ] Add StateInputForm to CommandBar
- [ ] Integrate huh rendering and updates in command bar
- [ ] Route execution through inline args or form

### Phase 6: Incremental Form Addition (1-2 hours)
- [ ] Test /scale with both inline and form
- [ ] Test /drain with both inline and form
- [ ] Test /shell with form (dynamic container list)
- [ ] Document form patterns and struct tags

### Phase 7: Testing & Polish (1-2 hours)
- [ ] Test error scenarios (kubectl missing, RBAC, etc)
- [ ] Add performance monitoring
- [ ] Test all commands end-to-end on real cluster
- [ ] Update documentation

## Notes

- Do NOT run tests during implementation (per CLAUDE.md prototype
  guidelines)
- Keep commits atomic per phase
- Create branch: feat/plan-06-kubectl-commands
- Each phase should compile and be testable independently
- Avoid over-engineering - focus on pragmatic delivery
- **Phased delivery strategy**: Implement inline args first (Phases 1-4,
  9-13 hours), then add forms (Phases 5-6, 3-5 hours), then polish (Phase
  7, 1-2 hours). Delivers working app faster.
- **Total plan**: 13-20 hours
  - Phase 1 (Foundation): 3-4 hours
  - Phase 2 (Core commands): 2-3 hours
  - Phase 3 (Node/Service): 1-2 hours
  - Phase 4 (Clipboard commands): 1-2 hours
  - Phase 5 (huh forms): 2-3 hours
  - Phase 6 (Form testing): 1-2 hours
  - Phase 7 (Polish): 1-2 hours
- **Command bar expansion**: Forms appear in expanded command bar (5-8
  lines), NOT overlays/modals. Follows existing confirmation pattern.
- **Struct tags pattern**: Each command defines typed args struct with form
  tags. Reflection helpers auto-generate forms and parse values. Type-safe,
  self-documenting, no magic strings.
- **Clipboard pattern**: /shell, /logs, /port-forward use clipboard mode
  (generate kubectl command, copy to clipboard, show message). Avoids
  complex TUI suspend, streaming, and background process management. Good
  fallback pattern for future complex commands.
