---
date: 2025-10-09T00:00:00Z
researcher: claude
git_commit: b22fe8a0c005295e708d66e728e4b1cafbc4e16a
branch: docs/designs
repository: renato0307/k1
topic: "Search functionality for YAML and describe screens with highlighting
and YAMLPath support"
tags: [research, codebase, fullscreen, yaml, describe, search, yamlpath,
highlighting]
status: in-progress
last_updated: 2025-10-28
last_updated_by: claude
last_updated_note: "Added follow-up review: command bar integration status,
implementation gaps, and context management additions"
---

# Research: Search Functionality for YAML and Describe Screens

**Date**: 2025-10-09
**Researcher**: Claude
**Git Commit**: b22fe8a0c005295e708d66e728e4b1cafbc4e16a
**Branch**: docs/designs
**Repository**: renato0307/k1

## Research Question

How can we implement search functionality for YAML and describe screens
with match highlighting, and add YAMLPath query capability as a second
search mode?

## Summary

The research reveals that implementing search in YAML/describe screens
requires three main components:

1. **Current Architecture**: YAML/describe content displayed via custom
   `FullScreen` component with manual scrolling (no viewport), content
   fetched from repository layer, already has YAML syntax highlighting

2. **Search Infrastructure**: Existing fuzzy search pattern
   (github.com/sahilm/fuzzy) can be adapted, but simple text search may be
   more appropriate for YAML content; highlighting requires character-level
   styling using Lipgloss patterns already present in codebase

3. **YAMLPath Integration**: Use goccy/go-yaml for structured YAML queries:
   - Query YAML directly without JSON conversion
   - Preserves comments and formatting
   - Provides native line numbers for highlighting
   - No position mapping required

Key implementation considerations:
- Extend FullScreen component with search state machine
- Add text matching with position tracking for highlighting
- Implement two search modes: simple text search and YAMLPath queries
- Leverage existing Lipgloss styling patterns for match highlighting
- YAMLPath works directly on YAML AST with native position information

## Detailed Findings

### Current YAML/Describe Implementation

#### Display Architecture

**Component**: `internal/components/fullscreen.go`
- Custom scrolling implementation (no Bubble Tea viewport)
- Manual line-by-line rendering with scroll offset tracking
- Keyboard navigation: ↑↓, jk, PgUp/PgDn, g/G for scrolling
- Scroll indicator shows position (e.g., "1-20 of 150")

**Key methods**:
- `NewFullScreen()` (lines 37-47): Constructor
- `Update()` (line 56): Handles keyboard input
- `View()` (line 110): Renders with header, content, scroll indicator
- `highlightYAML()` (lines 183-216): Already implements YAML syntax
  highlighting

**Current features**:
- Three view types: YAML (0), Describe (1), Logs (2)
- Theme-aware styling (Primary for keys, Success for values, Muted for
  comments)
- Line-based rendering from scrollOffset to scrollOffset+visibleLines

#### Command Flow

**Commands**: `internal/commands/resource.go`
- `YamlCommand()` (lines 26-67): Fetches YAML, creates ShowFullScreenMsg
- `DescribeCommand()` (lines 70-111): Fetches describe output, creates
  ShowFullScreenMsg

**Repository**: `internal/k8s/informer_repository.go`
- `GetResourceYAML()` (lines 837-871): Uses kubectl's YAMLPrinter
- `DescribeResource()` (lines 874-950): Custom describe formatter with
  on-demand events

**Message types**: `internal/types/types.go`
- `ShowFullScreenMsg` (lines 161-165): Triggers full-screen display
- `ExitFullScreenMsg` (line 168): Exits full-screen (ESC key)

**Application integration**: `internal/app/app.go`
- Lines 291-301: Creates FullScreen component on ShowFullScreenMsg
- Lines 303-308: Exits full-screen on ExitFullScreenMsg
- Lines 189-197: Forwards keys to FullScreen when active

### Existing Search/Filter Patterns

#### Command Bar Filter Mode

**Location**: `internal/components/commandbar/commandbar.go`

**Entry points**:
- Lines 162-196: `handleHiddenState()` - Typing activates filter mode
- Lines 198-244: `handleFilterState()` - Processes filter input

**Message flow**:
1. Character typed → StateFilter activated
2. FilterUpdateMsg sent with current filter string
3. Screen receives message, applies fuzzy search
4. Table/list updates with filtered results

**Negation support**: Filter strings starting with `!` perform inverse
matching

#### Screen-Level Filtering

**Location**: `internal/screens/config.go`

**Key methods**:
- Lines 167-173: `DefaultUpdate()` receives FilterUpdateMsg/ClearFilterMsg
- Lines 426-435: `SetFilter()` applies filter string
- Lines 437-479: `applyFilter()` performs fuzzy matching

**Search mechanism**:
- Builds search strings from configured fields (e.g., Name, Namespace,
  Status)
- Uses `github.com/sahilm/fuzzy` v0.1.1 for matching
- Returns ranked results sorted by match quality
- Supports negation with `!` prefix

**Fuzzy library data**:
```go
type Match struct {
    Index           int      // Position in input array
    Str             string   // Matched string
    MatchedIndexes  []int    // Character positions (AVAILABLE but UNUSED)
}
```

**Current limitation**: MatchedIndexes available but not used for
highlighting

#### Palette Filtering

**Location**: `internal/components/commandbar/palette.go`

**Filter method** (lines 38-77):
- Fuzzy matches command names
- Returns ranked results
- No visual highlighting of matched characters

**Rendering** (lines 127-213):
- Selection shown with ▶ indicator and background color
- No character-level match highlighting

### Text Highlighting and Styling Patterns

#### Pattern 1: YAML Syntax Highlighting (Already Implemented)

**Location**: `internal/components/fullscreen.go:183-216`

```go
func (fs *FullScreen) highlightYAML(yaml string) string {
    lines := strings.Split(yaml, "\n")

    keyStyle := lipgloss.NewStyle().Foreground(fs.theme.Primary)
    valueStyle := lipgloss.NewStyle().Foreground(fs.theme.Success)
    commentStyle := lipgloss.NewStyle().Foreground(fs.theme.Muted)

    var highlighted []string
    for _, line := range lines {
        // Color keys, values, comments differently
        // ...
    }
    return strings.Join(highlighted, "\n")
}
```

**Key aspects**:
- Line-by-line processing
- Different colors for syntax elements
- Styled fragments concatenated together

#### Pattern 2: Status Message Highlighting

**Location**: `internal/components/statusbar.go:58-91`

```go
messageStyle := baseStyle.Copy().
    Background(sb.theme.Success).  // Or Error, Primary
    Foreground(sb.theme.Background).
    Bold(true)
return messageStyle.Render(prefix + sb.message)
```

**Key aspects**:
- Background + Foreground for high contrast
- Bold for emphasis
- Semantic colors (Success, Error, Primary)

#### Pattern 3: Selection Highlighting

**Palette selection** (`palette.go:177-206`):
```go
if i == p.index {
    selectedStyle := lipgloss.NewStyle().
        Foreground(p.theme.Foreground).
        Background(p.theme.Subtle).
        Bold(true)
    line = selectedStyle.Render("▶ " + itemContent)
}
```

**Table selection** (`theme.go:87-90`):
```go
t.Table.SelectedRow = lipgloss.NewStyle().
    Foreground(lipgloss.Color("229")).
    Background(lipgloss.Color("57")).
    Bold(false)
```

#### Pattern 4: Dimmed/Muted Text

**Location**: `palette.go:170-173`

```go
shortcutStyle := lipgloss.NewStyle().Foreground(p.theme.Dimmed)
styledShortcut := shortcutStyle.Render(cmd.Shortcut)
```

**Used for**: Secondary information, hints, shortcuts

#### Available Theme Colors

**Location**: `internal/ui/theme.go:9-33`

Semantic colors available:
- `Primary` - Main accent (headers, keys)
- `Success` - Green (success states, YAML values)
- `Error` - Red (error states)
- `Warning` - Yellow/Orange (warnings)
- `Muted` - Grey (secondary info)
- `Dimmed` - Very subtle grey (hints)
- `Subtle` - Medium grey (selection backgrounds)
- `Accent` - Alternative highlight color

**Recommendation for match highlighting**: Use `Accent` or `Warning` color
with Bold style to distinguish from syntax highlighting

### YAMLPath for Go

#### Why YAMLPath Instead of JSONPath?

**YAMLPath** operates directly on YAML AST (Abstract Syntax Tree):
- Preserves YAML-specific features:
  - Comments (inline and block)
  - Anchors (`&anchor`) and aliases (`*alias`)
  - YAML tags (`!!str`, `!!int`)
  - Multi-document streams (`---` separators)
  - Original formatting and indentation
- Provides **native line number and column position information**
- Can modify YAML while preserving structure
- No conversion overhead

**Critical for k1**: We need to highlight matches in the original YAML
display. YAMLPath provides native position tracking, eliminating complex
YAML→JSON→YAML position mapping.

#### Recommendation: goccy/go-yaml

**GitHub**: https://github.com/goccy/go-yaml
**Import**: `github.com/goccy/go-yaml`
**Status**: Actively maintained (2024), ~1.1k+ stars

**Advantages for k1**:
1. **Native YAML path querying** with position information
2. **Comment preservation** - Won't lose YAML comments
3. **Line/column tracking** - Direct highlighting in original YAML
4. **Fast parsing** - 2-3x faster than standard go-yaml
5. **Anchor/alias support** - Handles YAML-specific features
6. **AST access** - Can inspect structure for advanced queries

**API Example**:
```go
import (
    "github.com/goccy/go-yaml"
    "github.com/goccy/go-yaml/parser"
)

// Parse YAML to get AST with positions
file, err := parser.ParseBytes(yamlBytes, 0)
if err != nil {
    return nil, err
}

// Query using path (JSONPath-like syntax with $ notation)
path, err := yaml.PathString("$.spec.containers[*].name")
if err != nil {
    return nil, err
}

// Find matching nodes in AST
matches := []MatchPosition{}
for _, doc := range file.Docs {
    nodes := path.FilterNode(doc.Body)
    for _, node := range nodes {
        token := node.GetToken()
        matches = append(matches, MatchPosition{
            Line:   token.Position.Line,      // Native line number!
            Column: token.Position.Column,    // Native column!
            Value:  node.String(),
        })
    }
}

// Use matches to highlight in YAML viewer - no mapping needed!
```

**Example Kubernetes queries**:
- All container images: `$.spec.containers[*].image`
- Container names: `$.spec.containers[*].name`
- Pod labels: `$.metadata.labels`
- Specific label value: `$.metadata.labels.app`

**Limitations**:
- Path syntax is not standard YAMLPath (uses JSONPath-like `$` notation)
- Less documentation than mainstream libraries
- Smaller ecosystem than Python's yamlpath

**Why YAMLPath is ideal for this use case**:
```
YAMLPath approach (goccy/go-yaml):
YAML → AST → Query → Results with native line numbers ✅
                     ↑ Line numbers built into AST nodes

No conversion, no position mapping, no complexity!
```

### Design Documents and Historical Context

**Issue #2**: `thoughts/shared/tickets/issue_2.md`
- Enhance describe with spec section (Future)

#### Related Research

**Contextual Navigation**:
`thoughts/shared/research/2025-10-07-contextual-navigation.md`
- Message-based communication patterns
- Config-driven screens
- Navigation between related resources

## Architecture Insights

### Search Mode State Machine

**Proposed approach: Integrate Command Bar into FullScreen**

Instead of using `/` or `:` directly (which conflicts with command/screen
navigation), **add the command bar component to YAML/describe screens**:

**States**:

1. **Normal** - Default viewing mode (current state)
   - Arrow keys scroll content
   - Typing activates filter mode (fuzzy search on visible content)
   - `/` opens command palette
   - ESC exits full-screen

2. **FilterMode** - Real-time fuzzy search (like list screens)
   - Type characters to filter visible content
   - Highlights all matches in real-time
   - ESC clears filter and returns to Normal
   - Reuses existing command bar filter pattern

3. **CommandMode** - Command palette (triggered by `/`)
   - Shows available commands:
     - `/search` - Text search with n/N navigation
     - `/yamlpath` - YAMLPath structured queries
     - `/copy` - Copy to clipboard
     - `/export-json` - Export as JSON
     - `/edit` - Edit YAML (future)
   - ESC returns to Normal

4. **SearchActive** - After executing `/search` command
   - Input field for search term
   - n/N navigate between matches
   - Highlights current match differently
   - ESC exits search, returns to Normal

5. **YAMLPathActive** - After executing `/yamlpath` command
   - Input field for YAMLPath query
   - Shows matching nodes with line numbers
   - Highlights matching sections
   - ESC exits query mode, returns to Normal

### Message Flow Extension

**New messages needed**:

```go
// types.go additions
type EnterSearchMsg struct {
    Mode SearchMode  // TextSearch or YAMLPath
}

type SearchQueryUpdateMsg struct {
    Query string
}

type SearchResultsMsg struct {
    Matches []SearchMatch
}

type SearchMatch struct {
    LineNum   int
    ColStart  int
    ColEnd    int
    MatchText string
    Context   string  // Surrounding lines
}

type ExitSearchMsg struct{}
```

### Highlighting Strategy

**For text search**:

1. Find all occurrences of search term in content
2. Track line numbers and column positions
3. When rendering visible lines:
   - Split line at match positions
   - Apply highlight style to matched text
   - Concatenate styled fragments

**Example**:
```go
// Line: "  name: nginx-deployment"
// Search: "nginx"
// Result: "  name: " + highlight("nginx") + "-deployment"

normalStyle := lipgloss.NewStyle().Foreground(theme.Foreground)
highlightStyle := lipgloss.NewStyle().
    Foreground(theme.Background).
    Background(theme.Warning).
    Bold(true)

parts := []string{
    normalStyle.Render("  name: "),
    highlightStyle.Render("nginx"),
    normalStyle.Render("-deployment"),
}
line := strings.Join(parts, "")
```

**For YAMLPath results**:

1. Parse YAML to AST using goccy/go-yaml
2. Execute YAMLPath query on AST
3. Get result nodes with native line/column positions
4. Highlight matching lines directly (no mapping needed!)

**Advantage**: goccy/go-yaml provides native position tracking:
```go
token := node.GetToken()
line := token.Position.Line     // Direct line number
column := token.Position.Column // Direct column
```

### Integration Points

**FullScreen component** (`fullscreen.go`):
- **Add command bar instance** (reuse existing CommandBar component)
- Add search state fields (mode, query, matches, currentMatch)
- Extend Update() to route messages to command bar
- Forward filter messages to search logic
- Handle command execution (ExecuteSearch, ExecuteYAMLPath)
- Modify View() to include command bar at bottom
- Extend content rendering to apply highlighting

**Command registry** (`commands/registry.go`):
- Register YAML/describe-specific commands:
  - `/search` - Text search command
  - `/yamlpath` - YAMLPath query command
  - `/copy` - Copy content to clipboard
  - `/export-json` - Export as JSON
- Commands only available when in full-screen mode

**CommandBar component** (reuse existing):
- No changes needed - already supports filter and command modes
- FilterUpdateMsg already exists
- Command execution pattern already implemented

**Repository layer** (no changes):
- Search operates on already-fetched content
- No additional Kubernetes API calls needed

## Implementation Recommendations

### Phase 0: Add Command Bar to FullScreen

**Scope**: Integrate command bar component into YAML/describe screens

**Changes**:
1. Add CommandBar instance to FullScreen:
   ```go
   type FullScreen struct {
       // ... existing fields
       commandBar *commandbar.CommandBar
   }
   ```

2. Initialize command bar in NewFullScreen():
   ```go
   commandBar := commandbar.NewCommandBar(theme, width)
   ```

3. Route messages to command bar in Update():
   ```go
   func (fs *FullScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
       switch msg := msg.(type) {
       case tea.KeyMsg:
           // Check if command bar should handle
           if fs.commandBar.IsActive() {
               // Route to command bar
           }
           // Otherwise handle scrolling
       case types.FilterUpdateMsg:
           // Apply filter to content
       }
   }
   ```

4. Render command bar in View():
   ```go
   content := fs.renderContent()
   commandBarView := fs.commandBar.View()
   return lipgloss.JoinVertical(lipgloss.Left, content, commandBarView)
   ```

5. Register full-screen commands in command registry

**Estimated complexity**: Low (1 day)

**Dependencies**: Existing CommandBar component

### Phase 1: Fuzzy Filter on Content

**Scope**: Add real-time fuzzy search (typing activates filter)

**Changes**:
1. Handle FilterUpdateMsg in FullScreen:
   ```go
   case types.FilterUpdateMsg:
       fs.filterQuery = msg.Filter
       fs.applyFilter()
   ```

2. Implement fuzzy matching on visible lines:
   ```go
   func (fs *FullScreen) applyFilter() {
       lines := strings.Split(fs.content, "\n")
       matches := fuzzy.Find(fs.filterQuery, lines)
       // Store matched line numbers for highlighting
   }
   ```

3. Highlight matching lines in rendering:
   - Apply subtle background to matched lines
   - Or highlight matched characters within lines

**Estimated complexity**: Low-Medium (1-2 days)

**Dependencies**: Existing fuzzy library (github.com/sahilm/fuzzy)

### Phase 2: Text Search Command (/search)

**Scope**: Add persistent search with n/N navigation

**Changes**:
1. Register `/search` command:
   ```go
   func SearchCommand() ExecuteFunc {
       return func(ctx CommandContext) tea.Cmd {
           // Transition to search input mode
           return func() tea.Msg {
               return types.EnterSearchModeMsg{Mode: "text"}
           }
       }
   }
   ```

2. Extend FullScreen with search state:
   ```go
   type FullScreen struct {
       // ... existing fields
       searchMode    SearchMode  // None, Text, YAMLPath
       searchQuery   string
       searchMatches []SearchMatch
       currentMatch  int
   }
   ```

3. Handle search navigation (n/N):
   ```go
   func (fs *FullScreen) nextMatch() {
       fs.currentMatch = (fs.currentMatch + 1) % len(fs.searchMatches)
       fs.scrollToMatch()
   }
   ```

4. Highlight current match differently than other matches

**Estimated complexity**: Medium (2-3 days)

**Dependencies**: Phase 0 complete

### Phase 3: YAMLPath Query Command (/yamlpath)

**Scope**: Add structured queries for Kubernetes resources

**Changes**:
1. Add dependency: `go get github.com/goccy/go-yaml`

2. Register `/yamlpath` command (executes from command palette)

3. Implement query execution:
   ```go
   func (fs *FullScreen) executeYAMLPath(query string) ([]SearchMatch, error)
   ```

4. Parse YAML and execute query:
   ```go
   import (
       "github.com/goccy/go-yaml"
       "github.com/goccy/go-yaml/parser"
   )

   // Parse YAML to AST
   file, _ := parser.ParseBytes([]byte(fs.content), 0)

   // Execute YAMLPath query
   path, _ := yaml.PathString(query)
   nodes := path.FilterNode(file.Docs[0].Body)

   // Extract positions from nodes (native support!)
   for _, node := range nodes {
       token := node.GetToken()
       matches = append(matches, SearchMatch{
           LineNum:  token.Position.Line,
           ColStart: token.Position.Column,
           // ...
       })
   }
   ```

5. Display results with context (matched value + surrounding lines)

**Estimated complexity**: Medium (3-4 days, native position support!)

**Dependencies**: Phase 0 complete, goccy/go-yaml library

### Phase 4: Additional Commands

**Scope**: Add utility commands for YAML/describe screens

**Commands to implement**:
1. `/copy` - Copy entire content to clipboard
2. `/copy-selection` - Copy current match/selection
3. `/export-json` - Convert YAML to JSON and copy
4. `/goto-line` - Jump to specific line number

**Estimated complexity**: Low-Medium (2-3 days)

**Dependencies**: Clipboard library (e.g., atotto/clipboard)

### Phase 5: Advanced Features

**Optional enhancements**:
- Search history (remember recent queries)
- Case-sensitive toggle
- Regular expression support for text search
- Save/load favorite YAMLPath queries
- Export search results
- Search result count in status bar

### Testing Strategy

**Unit tests**:
- Text matching with various patterns
- YAMLPath query execution with goccy/go-yaml
- Position tracking and highlighting
- Search navigation (n/N)
- AST parsing and node position extraction

**Integration tests**:
- Full search flow with envtest Kubernetes objects
- YAML syntax highlighting + search highlighting (combined)
- Edge cases: empty results, malformed queries
- Multi-document YAML streams

**Manual testing**:
- Search in large YAML files (1000+ lines)
- Complex YAMLPath queries on real pods/deployments
- Verify highlighting doesn't break YAML syntax coloring
- Test with YAML comments and anchors/aliases

## Code References

**Current implementation**:
- `internal/components/fullscreen.go:26-216` - FullScreen component
- `internal/commands/resource.go:26-111` - YAML/Describe commands
- `internal/k8s/informer_repository.go:837-950` - Data fetching
- `internal/components/commandbar/commandbar.go:162-244` - Filter mode
  pattern
- `internal/screens/config.go:437-479` - Fuzzy search implementation

**Styling patterns**:
- `internal/ui/theme.go:9-52` - Theme colors and styles
- `internal/components/statusbar.go:58-91` - Message highlighting
- `internal/components/commandbar/palette.go:127-213` - Selection rendering

**Related messages**:
- `internal/types/types.go:154-168` - Filter and screen switch messages

## Open Questions

1. **Command bar integration**: How to handle command bar state?
   - Should FullScreen own command bar instance?
   - Or should app manage command bar and route to FullScreen?
   - Recommend: FullScreen owns instance for encapsulation

2. **Filter vs Search distinction**:
   - Filter (typing): Real-time fuzzy match, shows all matches
   - Search (/search): Persistent, navigate with n/N, shows current match
   - Is this distinction clear to users?
   - Recommend: Show different indicators for filter vs search mode

3. **Search scope**: Should search work across:
   - Only visible lines (current viewport)?
   - All content (scroll to matches)?
   - Recommend: All content with auto-scroll to first match

4. **Highlighting conflicts**: How to combine YAML syntax highlighting with
   search match highlighting?
   - Syntax highlighting uses Primary/Success/Muted
   - Search highlighting should use Warning/Accent + Bold
   - Apply search highlighting on top of syntax highlighting

5. **Performance**: For very large YAML files (10,000+ lines), should we:
   - Parse AST on full-screen entry (one-time cost)?
   - Lazy parse only when /yamlpath command executed?
   - Show progress indicator for slow queries?
   - Recommend: Parse on /yamlpath execution, cache AST

6. **Command availability**: Should full-screen commands show in main
   command palette?
   - Show all commands always (with context indicators)?
   - Only show full-screen commands when in full-screen?
   - Recommend: Context-aware command filtering in registry

7. **YAMLPath syntax**: goccy/go-yaml uses JSONPath-like `$` notation:
   - Should we display syntax examples/help?
   - Provide common query templates (e.g., containers, labels)?
   - Show query validation errors inline?
   - Recommend: Show examples in command description

## Related Research

- **Contextual Navigation Research**
  (`thoughts/shared/research/2025-10-07-contextual-navigation.md`):
  Message-based patterns

## Next Steps

1. **Create design document** for search feature architecture
2. **Prototype text search**: Implement Phase 1 in feature branch
3. **Evaluate YAMLPath**: Test goccy/go-yaml with real Kubernetes YAML
4. **Test position tracking**: Verify line/column accuracy for highlighting
5. **User feedback**: Test search UX with realistic workflows
6. **Documentation**: Update CLAUDE.md with search usage patterns

---

## Follow-up Review [2025-10-28T07:01:58+00:00]

**Date**: 2025-10-28
**Git Commit**: 7206c20452f397727d32d009cc2183212f045b13
**Branch**: feat/kubernetes-context-management

### Review Purpose

Review the codebase to identify what has changed since the original research
(October 9, 2025) and assess the current implementation status of the
proposed search features.

### Key Findings

#### 1. No Search Implementation Yet

**Finding**: None of the proposed search features have been implemented.

**Details**:
- FullScreen component remains unchanged (`fullscreen.go:26-247`)
- No search state machine additions
- No text matching or position tracking
- No YAMLPath integration
- `goccy/go-yaml` dependency NOT added to `go.mod`

**Proposed messages NOT implemented**:
- `EnterSearchMsg` - Does not exist in `internal/types/types.go`
- `SearchQueryUpdateMsg` - Does not exist
- `SearchResultsMsg` - Does not exist
- `ExitSearchMsg` - Does not exist

**Status**: Research was complete, but no implementation work has started.

#### 2. Command Bar NOT Integrated with FullScreen

**Finding**: FullScreen and command bar operate independently without
integration.

**Details** (`app.go:240-249`):
- When `fullScreenMode = true`, command bar is completely bypassed
- ESC key handled specially to exit full-screen (`app.go:242-244`)
- All input forwarded directly to FullScreen component
- Command bar not rendered during full-screen mode (`app.go:589-591`)

**Implication**: Phase 0 (Add Command Bar to FullScreen) from original
research is NOT implemented. The proposed integration pattern would require
significant refactoring:
- FullScreen would need to own a CommandBar instance
- Message routing would need to be added
- View rendering would need to include command bar
- Height coordination would need implementation

**Current architecture**: FullScreen is a standalone modal that replaces the
entire UI (header, body, command bar).

**Proposed architecture**: FullScreen would become a complex component with
embedded command bar, similar to how screens work.

#### 3. Command Registry is Well-Structured

**Finding**: Command registry exists with sophisticated filtering, but no
search commands registered.

**Current commands** (`internal/commands/registry.go`):
- Resource commands: `/yaml`, `/describe`, `/delete`, `/logs`
- Deployment commands: `/scale`, `/restart`
- Node commands: `/cordon`, `/drain`
- Service commands: `/endpoints`
- Navigation commands: `:pods`, `:deployments`, `:services`, etc.
- Context commands: `:contexts`, `:next`, `:prev`
- LLM commands: `/ai <prompt>`

**Missing commands**:
- `/search` - NOT registered
- `/yamlpath` - NOT registered
- `/copy` - NOT registered
- `/export-json` - NOT registered
- `/goto-line` - NOT registered

**Registry capabilities** (`registry.go:351-376`):
- Fuzzy search filtering via `github.com/sahilm/fuzzy`
- Category-based filtering (Resource vs Action)
- Resource-type filtering (context-aware commands)
- Could easily support new commands when implemented

#### 4. Filter Patterns Work Well for Lists

**Finding**: Existing filter infrastructure is mature and could be adapted
for FullScreen search.

**Current implementation** (`screens/config.go:553-605`):
- Real-time fuzzy search on table rows
- Negation support with `!` prefix
- `fuzzy.Match.MatchedIndexes` available but UNUSED for highlighting
- Strict prefix matching for contexts screen

**Command bar integration** (`commandbar/commandbar.go:162-244`):
- Any character activates filter mode
- `FilterUpdateMsg` sent on every keystroke
- ESC clears filter and returns to hidden state
- Works seamlessly with screens

**Adaptation opportunity**: The filter pattern could be adapted to FullScreen
content, but would require:
- Line-based filtering instead of row-based
- Match position tracking for highlighting
- Scroll-to-match functionality
- Visual indicator of filtered vs all content

#### 5. Command Bar State Machine is Sophisticated

**Finding**: Command bar has a complex 7-state state machine that could
support search modes.

**Current states** (`types.go:16-23`):
- `StateHidden` - Inactive
- `StateFilter` - Real-time fuzzy search
- `StateSuggestionPalette` - Command palette (`:` or `/`)
- `StateInput` - Direct command input with args
- `StateConfirmation` - Destructive operation confirmation
- `StateLLMPreview` - AI command preview
- `StateResult` - Command execution result display

**Potential new states for search**:
- `StateSearchActive` - Text search with n/N navigation
- `StateYAMLPathActive` - YAMLPath query mode

**Height coordination** (`commandbar.go:73-91`, `app.go:269`):
- Dynamic height (1-6 lines) based on state
- App recalculates body height on every state change
- Layout system supports variable command bar height

**Challenge**: FullScreen currently takes entire terminal (`app.go:521`). To
integrate command bar, would need to:
- Reserve space for command bar at bottom
- Coordinate height between FullScreen and CommandBar
- Handle state transitions and message routing
- Render both components in full-screen mode

#### 6. Major Feature Addition: Context Management

**Finding**: Kubernetes context switching with loading states was added since
October 9 research.

**New messages** (`types.go:170-204`):
- `ContextSwitchMsg` - Initiates context switch
- `ContextLoadProgressMsg` - Reports loading progress
- `ContextLoadCompleteMsg` - Signals successful load
- `ContextLoadFailedMsg` - Signals failed load
- `ContextSwitchCompleteMsg` - Signals successful switch
- `ContextRetryMsg` - Requests retry of failed context

**Implementation** (`app.go:394-646`):
- Background context loading via RepositoryPool
- Loading UI with spinner and progress messages
- Re-registration of all screens with new repository
- History preservation across context switches
- Error handling and retry mechanism

**Impact on search feature**: None directly, but demonstrates the project's
ability to add complex multi-message workflows.

#### 7. FullScreen Component Unchanged

**Finding**: FullScreen component implementation matches October 9 research
exactly.

**Current features** (`fullscreen.go:26-247`):
- Three view types: YAML (0), Describe (1), Logs (2)
- Manual scrolling: ↑↓, jk, PgUp/PgDn, g/G
- YAML syntax highlighting (simple pattern-based)
- Scroll position indicator
- Content stored as single string
- Reserved lines constant: 3

**No additions**:
- No search state
- No match tracking
- No highlighting infrastructure
- No command bar integration
- No viewport component

**Logs view type**: Still not implemented (reserved but unused)

**Reference**: Log streaming research exists at
`thoughts/shared/research/2025-10-26-log-streaming-tui-implementation.md`

#### 8. Highlighting Infrastructure is Ready

**Finding**: Theme colors and styling patterns exist for implementing match
highlighting.

**Available theme colors** (`ui/theme.go`):
- `Primary` - Main accent (currently used for YAML keys)
- `Success` - Green (currently used for YAML values)
- `Error` - Red (for error states)
- `Warning` - Yellow/Orange (available for match highlighting)
- `Muted` - Grey (currently used for comments)
- `Dimmed` - Very subtle grey (for hints)
- `Subtle` - Medium grey (for backgrounds)
- `Accent` - Alternative highlight (available for match highlighting)

**Existing highlighting patterns**:
- YAML syntax highlighting: line-by-line with key/value/comment colors
  (`fullscreen.go:184-216`)
- Selection highlighting: background + bold + indicator
  (`palette.go:177-206`)
- Status messages: high-contrast background + foreground + bold
  (`statusbar.go:58-91`)

**Recommendation from original research**: Use `Accent` or `Warning` with
Bold for search matches to distinguish from syntax highlighting.

**Ready for implementation**: The styling infrastructure is mature and could
support layered highlighting (syntax + search matches).

### Implementation Gaps Analysis

#### Phase 0: Add Command Bar to FullScreen (NOT DONE)

**Original estimate**: Low complexity (1 day)

**Current reality**: More complex than originally estimated due to:
1. FullScreen currently replaces entire UI (modal pattern)
2. Command bar bypass in full-screen mode is intentional
3. Would require architectural change from modal to embedded pattern
4. Height coordination more complex than anticipated

**Required changes**:
- Add CommandBar field to FullScreen struct
- Modify View() to render command bar at bottom
- Update Update() to route messages to command bar
- Reserve space in height calculations
- Handle state transitions (filter, palette, input)
- Coordinate scrolling when command bar expands

**Revised estimate**: Medium complexity (2-3 days)

#### Phase 1: Fuzzy Filter on Content (NOT DONE)

**Original estimate**: Low-Medium complexity (1-2 days)

**Readiness**: High - existing filter infrastructure could be adapted

**Required changes**:
- Handle `FilterUpdateMsg` in FullScreen
- Split content into lines, apply fuzzy search
- Track matched line numbers
- Highlight matched lines or characters
- Scroll to first match on filter activation
- Show match count in UI

**Dependencies**: Phase 0 must be complete

**Consideration**: Should filter hide non-matching lines or just highlight
them? Original research doesn't specify.

#### Phase 2: Text Search Command (/search) (NOT DONE)

**Original estimate**: Medium complexity (2-3 days)

**Missing pieces**:
- `/search` command not registered in registry
- Search state not added to FullScreen
- No n/N navigation logic
- No match position tracking
- No current match highlighting

**Required changes**:
- Register `/search` command in registry
- Add search fields to FullScreen: searchMode, searchQuery, searchMatches,
  currentMatch
- Implement text matching with position tracking
- Add n/N key handlers for navigation
- Highlight current match differently than other matches
- Auto-scroll to current match

**Dependencies**: Phase 0 must be complete

**Consideration**: Distinguish filter (immediate, fuzzy) from search
(persistent, navigate with n/N).

#### Phase 3: YAMLPath Query Command (/yamlpath) (NOT DONE)

**Original estimate**: Medium complexity (3-4 days with native position
support)

**Missing pieces**:
- `goccy/go-yaml` dependency not added to `go.mod`
- `/yamlpath` command not registered
- No AST parsing logic
- No YAMLPath query execution
- No result rendering with context

**Required changes**:
- Add dependency: `go get github.com/goccy/go-yaml`
- Register `/yamlpath` command
- Implement YAMLPath query execution with AST parsing
- Extract native line/column positions from AST nodes
- Display results with surrounding context
- Handle query validation errors
- Provide query syntax examples

**Dependencies**: Phase 0 must be complete

**Advantage**: goccy/go-yaml provides native position tracking, eliminating
need for complex mapping.

#### Phase 4: Additional Commands (NOT DONE)

**Original estimate**: Low-Medium complexity (2-3 days)

**Commands to implement**:
- `/copy` - Copy entire content to clipboard
- `/copy-selection` - Copy current match/selection
- `/export-json` - Convert YAML to JSON and copy
- `/goto-line` - Jump to specific line number

**Missing pieces**:
- No clipboard integration (would need `atotto/clipboard` or similar)
- No commands registered
- No execution logic

**Status**: Not started

#### Phase 5: Advanced Features (NOT DONE)

**Original estimate**: Optional enhancements

**Features proposed**:
- Search history (remember recent queries)
- Case-sensitive toggle
- Regular expression support
- Save/load favorite YAMLPath queries
- Export search results
- Search result count in status bar

**Status**: Not started, still optional

### Revised Implementation Strategy

#### Challenge 1: Modal vs Embedded Pattern

**Current**: FullScreen is a modal that replaces entire UI

**Required**: FullScreen needs embedded command bar like screens

**Options**:

**Option A: Full Integration (Original Plan)**
- Embed CommandBar in FullScreen
- FullScreen owns command bar instance
- Coordinate height dynamically
- Message routing in FullScreen.Update()

**Pros**: Clean separation, FullScreen self-contained
**Cons**: Complex, breaks modal pattern, height coordination tricky

**Option B: Parallel Components**
- Keep FullScreen as modal for content
- Keep CommandBar separate at app level
- App coordinates messages between them
- Command bar stays visible in full-screen mode

**Pros**: Simpler, maintains current architecture
**Cons**: App becomes more complex coordinator, less encapsulation

**Option C: Hybrid Approach**
- FullScreen remains modal for YAML/describe views (no search)
- Add new "SearchableFullScreen" component with embedded command bar
- Migrate to SearchableFullScreen only when search feature implemented

**Pros**: Gradual migration, doesn't break existing functionality
**Cons**: Two full-screen components, more code

**Recommendation**: Option B (Parallel Components) for Phase 0, then
refactor to Option A if needed. Simpler to start, less risk.

#### Challenge 2: Filter vs Search Distinction

**Original research question** (Open Question #2):
"Is the distinction between filter and search clear to users?"

**Current filter behavior** (command bar + screens):
- Type character → immediate fuzzy filtering
- Real-time updates on every keystroke
- ESC clears filter
- Shows all matches at once

**Proposed search behavior** (Phase 2):
- Execute `/search` command → enter search mode
- Type search term → find matches
- n/N navigate between matches
- Shows current match with highlighting
- Persistent until explicitly exited

**Distinction**:
- **Filter**: Immediate, fuzzy, shows all matches
- **Search**: Persistent, exact, navigate matches

**User signal**: Filter for quick narrowing, search for precise navigation

**Implementation approach**: Implement filter first (Phase 1), defer search
(Phase 2) until user feedback confirms need for both.

#### Challenge 3: Highlighting Conflicts

**Original research question** (Open Question #4):
"How to combine YAML syntax highlighting with search match highlighting?"

**Current YAML highlighting** (`fullscreen.go:184-216`):
- Line-by-line processing
- Keys: `theme.Primary`
- Values: `theme.Success`
- Comments: `theme.Muted`

**Proposed match highlighting**:
- Background: `theme.Warning` or `theme.Accent`
- Foreground: `theme.Background` (high contrast)
- Bold: true

**Implementation strategy**:
1. Apply YAML syntax highlighting first (current behavior)
2. Identify match positions within styled content
3. Re-style matched characters/words with search highlight
4. Concatenate styled fragments

**Example**:
```
Original:  "  name: nginx-deployment"
Syntax:    "  " + Primary("name:") + " " + Success("nginx-deployment")
Search:    "  " + Primary("name:") + " " + Success("nginx") +
Warning("-deployment")
```

**Challenge**: Need character-level styling, not line-level. Current
`highlightYAML()` doesn't support this.

**Solution**: Refactor to build styled string character-by-character or
segment-by-segment.

### Updated Recommendations

#### Short-Term (Immediate Next Steps)

1. **Prototype parallel components approach** (Option B)
   - Keep FullScreen as modal
   - Make command bar visible in full-screen mode
   - Route messages at app level
   - Test with simple filter mode

2. **Implement Phase 1: Fuzzy Filter**
   - Reuse existing filter message types
   - Apply fuzzy search to FullScreen content
   - Highlight matched lines (simple background color)
   - Defer character-level highlighting

3. **User feedback on filter behavior**
   - Is filter mode sufficient for most use cases?
   - Do users need persistent search with n/N navigation?
   - Is the filter vs search distinction necessary?

#### Medium-Term (After Feedback)

4. **Decide on search implementation**
   - If filter is sufficient, stop at Phase 1
   - If persistent search needed, implement Phase 2
   - Re-evaluate architectural pattern (parallel vs embedded)

5. **Evaluate YAMLPath demand**
   - Survey users: Would they use YAMLPath queries?
   - Identify common query patterns (containers, labels, etc.)
   - Prototype with `goccy/go-yaml` if demand exists

6. **Implement utility commands**
   - `/copy` (highest value, easiest)
   - `/goto-line` (if large YAML files common)
   - `/export-json` (if conversion needed)

#### Long-Term (Future Enhancements)

7. **Advanced highlighting**
   - Character-level match highlighting
   - Multiple match colors (current vs others)
   - Context lines around matches

8. **Search history and templates**
   - Remember recent queries
   - Favorite YAMLPath templates
   - Query validation and syntax help

9. **Performance optimization**
   - Lazy AST parsing for YAMLPath
   - Progress indicators for slow queries
   - Caching for repeated queries

### Testing Strategy Updates

#### Unit Tests (Currently Missing)

**Required for search implementation**:
- `fullscreen_test.go` - DOES NOT EXIST (add before search work)
- Test scrolling, view rendering, size calculations
- Test content processing and highlighting

**Search-specific tests** (add when implementing):
- Text matching with various patterns
- Position tracking and match navigation
- Filter vs search behavior
- Highlighting with YAML syntax

#### Integration Tests

**Required**:
- Full search flow with envtest Kubernetes objects
- Command bar coordination in full-screen mode
- Height recalculation when command bar changes state
- Message routing between app, command bar, full-screen

**YAMLPath-specific** (if implemented):
- AST parsing with goccy/go-yaml
- Query execution with real Kubernetes YAML
- Position extraction from AST nodes
- Multi-document YAML support

#### Manual Testing Checklist

**Filter mode**:
- [ ] Type characters to filter content
- [ ] ESC clears filter
- [ ] Matched lines highlighted
- [ ] Scroll position preserved or reset?
- [ ] Large files (1000+ lines) performance

**Search mode** (if implemented):
- [ ] `/search` command activation
- [ ] n/N navigation between matches
- [ ] Current match highlighted differently
- [ ] Auto-scroll to current match
- [ ] Search persists across scrolling

**YAMLPath mode** (if implemented):
- [ ] `/yamlpath` command activation
- [ ] Query syntax validation
- [ ] Result highlighting with context
- [ ] Complex queries (nested fields, arrays)
- [ ] YAML comments preserved

### Open Questions (Updated)

#### Resolved Since Original Research

**Question 1**: Command bar integration approach
**Answer**: Parallel components approach recommended for Phase 0

**Question 3**: Search scope
**Answer**: All content with auto-scroll (confirmed as correct approach)

#### Still Open

**Question 2**: Filter vs Search distinction
**Status**: Implement filter first, defer search until user feedback

**Question 4**: Highlighting conflicts
**Status**: Refactor to character-level styling required

**Question 5**: Performance for large files
**Status**: Implement basic version, optimize if slow

**Question 6**: Command availability in full-screen
**Status**: Commands should be context-aware (only show when applicable)

**Question 7**: YAMLPath syntax help
**Status**: Provide examples if YAMLPath implemented

#### New Questions

**Question 8**: Should filter hide non-matching lines or just highlight them?
**Recommendation**: Highlight only (preserves context)

**Question 9**: Should search work across view types (YAML, describe, logs)?
**Recommendation**: Yes, search should be view-agnostic

**Question 10**: How to handle search in streaming logs (future)?
**Recommendation**: Defer until log streaming implemented, may need
different approach

### Conclusion

**Implementation status**: None of the proposed search features have been
implemented since the October 9 research.

**Key blockers**:
1. Architectural decision needed: modal vs embedded pattern
2. Command bar integration more complex than estimated
3. No clear user demand signal for search vs filter

**Recommended path forward**:
1. Start with Phase 0: parallel components approach (simpler)
2. Implement Phase 1: basic filter mode (reuse existing patterns)
3. Gather user feedback before investing in full search (Phase 2-5)
4. Re-evaluate YAMLPath based on actual user needs

**Effort estimate** (revised):
- Phase 0: 2-3 days (was 1 day)
- Phase 1: 2-3 days (was 1-2 days)
- Total for basic filter: 4-6 days

**Risk**: Medium - architectural changes to app-level coordination

**Value**: High - improves YAML/describe navigation significantly
