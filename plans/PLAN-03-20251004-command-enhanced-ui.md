# Implementation Plan: Command-Enhanced List Browser UI Prototype

**Plan ID:** PLAN-03
**Date:** 2025-10-04
**Related Design:** DDR-05
**Status:** In Progress (Phase 4 complete - ready for Phase 5)

## Overview

Prototype the command-enhanced list browser UI from DDR-05, including
simulated LLM commands. Build incrementally with dummy data to test the
interaction model: expandable command bar, inline command palette,
confirmation flows, LLM preview, and full-screen views. Uses static mock
responses for LLM simulation (no real API calls).

## Goals

- Validate command bar UX (expandable states, inline palette)
- Test keyboard-driven command discovery and execution
- Prove LLM preview flow with static mock responses
- Prove full-screen view pattern works with list preservation
- Build foundation for future command system

## TODO

### Phase 0: Cleanup and Layout Preparation
- [x] Remove existing modal system (ScreenPickerModal, CommandPaletteModal)
- [x] Remove current `/` filter mode from app state and global keybindings
- [x] Update Layout component to reserve space for command bar at bottom
- [x] Remove ctrl+s and ctrl+p keybindings from app.go
- [x] Simplify app.go Update() to remove modal routing logic
- [x] Test that app still runs with simplified layout

### Phase 1: Command Bar Component
- [x] Create `internal/components/commandbar.go` with component structure
- [x] Implement state machine (Hidden/Filter/SuggestionPalette/Input/Confirmation/LLMPreview/Result)
- [x] Add basic filter mode (no prefix, filters list in real-time)
- [x] Add expansion/contraction behavior with list area coordination
- [x] Integrate command bar into app.go layout rendering
- [x] Add basic `:` and `/` keystroke detection to trigger suggestion palette
- [x] Test smooth transitions between states and heights
- [x] Test filter mode with fuzzy matching on list items
- [x] Fix title/header visibility when palette expands
- [x] Add horizontal separator line above command bar
- [x] Implement fuzzy search with negation support (!pattern)
- [x] Prevent arrow keys from affecting table when palette is active

### Phase 2: Inline Suggestion Palette
- [x] Create `internal/commands/registry.go` for command registry
- [x] Implement palette rendering with fuzzy search integration
- [x] Add keyboard navigation and context-aware filtering
- [x] Wire up `:` for navigation (screens) and `/` for resource commands
- [x] Test palette + command bar unified bottom component
- [x] Add `/ai` option to palette for AI commands
- [x] Implement smooth transition between palette and input modes with backspace
- [x] Refactor duplicated palette logic into helper methods

### Phase 3: Basic Predefined Commands
- [x] Implement navigation commands (:pods, :deployments, :services)
- [x] Add LLM commands to registry with mock translations (internal/commands/llm_mock.go)
- [x] Wire up `/ai` prefix for natural language AI commands
- [x] Implement LLM preview expansion (4-6 lines) with action buttons
- [x] Implement namespace filtering command (:ns) - placeholder for now
- [x] Implement resource commands (/yaml, /describe, /delete, /logs) - return placeholder messages
- [x] Wire up delete command with confirmation flow (NeedsConfirmation: true)
- [x] Verify filter mode with negation (!pattern) - already implemented in applyFilter()
- [x] Add context-aware command filtering (ResourceTypes field on Command struct)
- [x] Implement /scale command for deployments only
- [x] Wire up screen context tracking in command bar (SetScreen method)
- [x] Add CommandContext struct with resource type and selected resource data
- [x] Update ExecuteFunc signature to accept CommandContext instead of string
- [x] Implement ScreenWithSelection interface for screens that provide selected resource info
- [x] Wire up app.go to collect and pass selected resource context to commands
- [x] Implement GetSelectedResource() on PodsScreen
- [x] Update all command Execute functions to use CommandContext (shows selected resource in messages)

### Phase 4: Full-Screen Views
- [x] Create `internal/components/fullscreen.go` component
- [x] Implement YAML viewer with syntax highlighting
- [x] Implement describe viewer with formatted output
- [x] Add list state preservation (selection, filter, scroll position)
- [x] Wire up ShowFullScreenMsg and ExitFullScreenMsg messages
- [x] Update /yaml and /describe commands to show full-screen views
- [x] Add scrolling support (↑↓/jk, PgUp/PgDn, g/G)

### Phase 5: Add shortcuts for commands
- [ ] Implement global keybindings in app.go
- [ ] Keybindings must show in the command palette
- [ ] Add `ctrl+y` shortcut for `/yaml` command
- [ ] Add `ctrl+d` shortcut for `/describe` command
- [ ] Add `ctrl+l` shortcut for `/logs` command
- [ ] Add `ctrl+x` shortcut for `/delete` command


### Phase 6: Command History
- [ ] Add in-memory history storage
- [ ] Implement arrow key navigation (↑/↓)
- [ ] Test history across different command types

### Small Fixes (To Handle Later)
- [ ] When search text is small the list is returning empty and the list on the screen is being cleared
- [x] Typing after `:` and `/` does not filter the command/screen list - FIXED with refactoring
- [x] Add margins on the table columns (when text is larger than column width, the ... is too close to the text) - FIXED by adding PaddingLeft(1) and PaddingRight(1) to Cell styles
- [ ] Uses themes all over the place (lipgloss styles)
- [ ] Tables are not occupying full width of the screen
- [ ] Column widths are not dynamic (hardcoded values)

## Major Phases

### 0. Cleanup and Layout Preparation
Prepare the codebase for the new command bar approach:
- **Remove modal system**: Delete `internal/modals/screenpicker.go` and
  `internal/modals/commandpalette.go`
- **Remove filter mode**: Current `/` filter lives in app state with
  global handling - will be replaced by command bar filter mode (no
  prefix)
- **Simplify app.go**: Remove `ctrl+s`/`ctrl+p` keybindings, remove
  modal overlay rendering, remove `/` filter mode state handling
- **Update Layout component**: Modify `internal/components/layout.go` to
  reserve bottom space for command bar (initially 1 line)
- **Preserve screens**: Keep all screen implementations unchanged for now

**Key Decision**: Keep screens and data layer intact. Only touch UI
routing and layout. This allows testing the new structure without
breaking existing functionality.

### 1. Command Bar Component
Build expandable command bar at bottom of screen:
- Component in `internal/components/commandbar.go`
- States: Hidden (1 line), Filter (1 line), SuggestionPalette (2-10
  lines), Input (1 line), Confirmation (3-5 lines), LLMPreview (4-6
  lines), Result (2-3 lines)
- **Filter mode**: Just start typing (no prefix) filters list in
  real-time with fuzzy matching. No palette expansion. Arrow keys navigate
  list (list stays interactive). ESC clears filter.
- **Expansion behavior**: Command bar grows upward (visually downward
  from list perspective), list shrinks proportionally
- Confirmation state expands to show: prompt text, warning/context,
  action hints
- LLMPreview state expands to show: original prompt, generated command
  (syntax highlighted), action buttons
- Basic input handling: text entry, cursor, ESC to cancel
- Dynamic height (1-10 lines max)
- Visual hints: `[: screens  / commands]` when hidden, `[Enter] Confirm
  [ESC] Cancel` when active

**Key Decision**: Start with filter mode (simplest) and minimal states,
add complexity incrementally. Test expansion/contraction smoothly
adjusts list area. Integrate into app.go early to validate layout
coordination.

### 2. Inline Suggestion Palette
Implement command/screen discovery UI:
- Command registry in `internal/commands/registry.go` (supports both
  navigation and resource command types)
- **Downward expansion**: Palette appears on `:` or `/` press, expands
  upward from command bar (looks like it grows downward from list)
- `:` shows navigation options (screens, namespace)
- `/` shows resource commands (context-aware for selected resource type)
- **Arrow keys navigate palette items** (list becomes non-interactive)
- Palette is part of command bar area (command bar + palette together
  occupy bottom space)
- Two-column layout: option name | description
- Arrow key navigation, fuzzy filtering as user types
- Dynamic sizing: 1-8 items shown, palette shrinks as matches narrow
- List area shrinks proportionally when palette expands

**Key Decision**: Use existing fuzzy library (`sahilm/fuzzy`) from pods
screen. Palette and command bar form unified bottom component. Single
palette component serves both `:` and `/` modes with different item
sources. Arrow keys have different behavior: in filter mode, they
navigate the list (shared focus); in palette mode, they navigate palette
items (palette has focus).

### 3. Basic Predefined Commands
Implement commands to test different flows:

**Filter mode** (no prefix):
- Just start typing to filter current list
- Real-time fuzzy matching on list items
- Arrow keys navigate the filtered list (list stays interactive)
- Backspace edits filter text in command bar
- Support negation: `!pattern` excludes matches
- ESC clears filter, Enter applies and exits filter mode

**Navigation commands** (`:` prefix):
- `:pods` - Switch to Pods screen
- `:deployments` - Switch to Deployments screen
- `:services` - Switch to Services screen
- `:ns <namespace>` - Switch namespace filter

**Resource commands** (`/` prefix):
- `/yaml` - Show resource YAML (prepare for full-screen in Phase 4)
- `/describe` - Show kubectl describe output
- `/delete` - Delete with confirmation flow (tests confirmation expansion)
  - Command bar expands to 3-5 lines showing multi-line confirmation
  - Display: resource details, warning message, action buttons
  - Tests upward expansion and list area adjustment

**Simulated LLM commands** (`/ai` prefix):
- `/ai <natural language>` - Simulate LLM translation to kubectl
- Static mock responses (map common phrases to kubectl commands):
  - "delete failing pods" → `kubectl delete pods --field-selector status.phase=Failed`
  - "scale nginx to 3" → `kubectl scale deployment nginx --replicas=3`
  - "get pod logs" → `kubectl logs <selected-pod>`
- LLMPreview state shows: prompt + generated command + action buttons
- Buttons: `[Enter] Execute  [e] Edit  [ESC] Cancel`
- Tests LLM preview expansion (4-6 lines)

**Key Decision**: Start with filter mode (most common action). Use
static command mapping for LLM simulation (no actual API calls). Focus
on UX flow, not real translation. `/delete` and `/ai` are critical for
testing different expansion behaviors.

### 4. Full-Screen Views
Implement pattern for full-screen content:
- Full-screen component in `internal/components/fullscreen.go`
- YAML viewer (syntax highlighting with lipgloss)
- Describe viewer (formatted output)
- Header with resource name + ESC hint
- List state preservation (selection, filter, scroll position)
- ESC returns to list view

**Key Decision**: Test with dummy YAML/describe data, no actual kubectl
calls in prototype.

### 5. Command History
Add command recall with arrow keys:
- In-memory history (no persistence in prototype)
- `↑`/`↓` navigate through previous commands
- Works in both direct input and palette modes
- History shared across all command types

## Critical Considerations

**Incremental Testing**: After each phase, manually test interactions
before moving forward. Validate keyboard shortcuts don't conflict.

**Expansion Behavior**: Core UX pattern to validate. Command bar expands
upward (from list perspective, grows downward), list area shrinks
proportionally. Test with different terminal sizes to ensure smooth
transitions.

**List Integration**: Command bar must coordinate with existing Pods screen
(or create simplified test screen). List needs to shrink/expand when
command bar changes height. Calculate available list height dynamically.
**Important**: Arrow keys in filter mode navigate the list (list stays
interactive), but in palette mode navigate palette items (list becomes
non-interactive).

**State Management**: Command bar state machine needs clean transitions.
Document valid state changes upfront. Each state has specific height
requirements.

**Dummy Data**: Use mock YAML/describe output to avoid kubectl dependencies
in prototype. Focus on UX, not real operations.

## Success Criteria

- Can start typing (no prefix) and see list filter in real-time with fuzzy matching
- Filter mode shows input in command bar, ESC clears filter
- Can use `!pattern` to exclude matches (negation)
- Can press `:` and see inline navigation palette (screens, namespace)
- Can press `/` and see inline resource command palette (yaml, logs, delete, etc.)
- List area shrinks smoothly when palette/confirmation/LLM preview expands
- Can navigate palette with arrows, select option with Enter
- Can type commands directly (`:pods`, `/yaml`) and execute
- `:pods`, `:deployments`, `:services` successfully switch screens
- Full-screen views display and ESC returns to list correctly
- `/delete` command expands command bar to 3-5 lines showing multi-line confirmation
- Confirmation can be executed or cancelled, command bar contracts afterward
- `/ai` commands show LLM preview state (4-6 lines) with prompt + generated command
- LLM preview can be executed, edited, or cancelled
- Command history recalls previous inputs with `↑`/`↓`
- No janky rendering (smooth transitions between states and heights)

## Out of Scope (Prototype)

- Real LLM API calls (use static mock responses instead)
- LLM cache persistence (no need for cache with static responses)
- Persistent command history (in-memory only)
- Real kubectl execution (dummy/mock operations)
- Multi-select operations
- Custom command aliases
- Live Kubernetes data (use dummy repository)
- Edit mode for generated commands (just show the button, don't implement editor)

## Future Work (Post-Prototype)

If prototype validates the UX:
- Replace dummy data with live repository
- Replace static LLM responses with real API integration (OpenAI, Anthropic, etc.)
- Implement LLM cache persistence and management
- Add full command set from DDR-05
- Implement persistent command history
- Implement edit mode for generated commands
- Add real kubectl execution
- Extend to all screen types (deployments, services, etc.)

## References

- Design: `design/DDR-05.md` (Full command-enhanced UI specification)
- Existing filter logic: `internal/screens/pods.go`
- Fuzzy library: `github.com/sahilm/fuzzy`
- Overlay example: `internal/modals/screenpicker.go`
