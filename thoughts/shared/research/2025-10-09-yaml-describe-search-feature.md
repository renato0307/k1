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
status: complete
last_updated: 2025-10-09
last_updated_by: claude
last_updated_note: "Added YAMLPath research findings"
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
