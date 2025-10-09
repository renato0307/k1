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

**Proposed states for FullScreen component**:

1. **Normal** - Default viewing mode (current state)
   - Arrow keys scroll content
   - ESC exits full-screen

2. **TextSearch** - Simple text search mode
   - `/` enters search mode
   - Type query to search
   - n/N navigate matches
   - ESC returns to Normal
   - Highlights all matches in visible content

3. **YAMLPathSearch** - Structured query mode
   - `:` enters YAMLPath mode
   - Type YAMLPath query (e.g., `$.spec.containers[*].image`)
   - Shows matching values with context
   - Highlights matching paths in YAML
   - ESC returns to Normal

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
- Add search state fields (mode, query, matches, currentMatch)
- Extend Update() to handle search keys (/, :, n, N, ESC)
- Modify View() to render search input and match count
- Extend content rendering to apply highlighting

**Command bar pattern** (optional):
- Could reuse command bar component for search input
- But simpler to embed search input in FullScreen header

**Repository layer** (no changes):
- Search operates on already-fetched content
- No additional Kubernetes API calls needed

## Implementation Recommendations

### Phase 1: Text Search with Highlighting

**Scope**: Add simple text search to YAML/describe screens

**Changes**:
1. Extend FullScreen struct with search state:
   ```go
   type FullScreen struct {
       // ... existing fields
       searchMode    SearchMode  // None, Text, YAMLPath
       searchQuery   string
       searchMatches []SearchMatch
       currentMatch  int
   }
   ```

2. Add search key handlers in Update():
   - `/` enters text search mode
   - Characters update query
   - `n` / `N` navigate matches
   - ESC exits search

3. Implement text matching:
   ```go
   func (fs *FullScreen) findTextMatches(query string) []SearchMatch
   ```

4. Modify rendering to highlight matches:
   ```go
   func (fs *FullScreen) renderLineWithHighlights(
       line string,
       lineNum int
   ) string
   ```

5. Add search input display to header

**Estimated complexity**: Medium (2-3 days)

**Dependencies**: None (uses existing patterns)

### Phase 2: YAMLPath Query Support

**Scope**: Add structured queries for Kubernetes resources

**Changes**:
1. Add dependency: `go get github.com/goccy/go-yaml`

2. Add YAMLPath mode triggered by `:`

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

**Dependencies**: goccy/go-yaml library

### Phase 3: Advanced Features

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

1. **Search mode activation**: Use different triggers for different modes:
   - `/` for text search (vim-like)
   - `:` for YAMLPath queries (structured)
   - Recommend: Keep them separate for clear mental model

2. **Search scope**: Should search work across:
   - Only visible lines (current viewport)?
   - All content (scroll to matches)?
   - Recommend: All content with auto-scroll to first match

3. **Highlighting conflicts**: How to combine YAML syntax highlighting with
   search match highlighting?
   - Syntax highlighting uses Primary/Success/Muted
   - Search highlighting should use Warning/Accent + Bold
   - Apply search highlighting on top of syntax highlighting

4. **Performance**: For very large YAML files (10,000+ lines), should we:
   - Parse AST on full-screen entry (one-time cost)?
   - Lazy parse only when YAMLPath mode activated?
   - Show progress indicator for slow queries?
   - Recommend: Parse on YAMLPath activation, cache AST

5. **User experience**: Should search input be:
   - In header (like current filter in command bar)?
   - As overlay (like command palette)?
   - Inline at bottom (like vim search)?
   - Recommend: Header input similar to command bar pattern

6. **YAMLPath syntax**: goccy/go-yaml uses JSONPath-like `$` notation:
   - Should we display syntax examples/help?
   - Provide common query templates?
   - Show query validation errors?
   - Recommend: Display example queries in help text

## Related Research

- **Contextual Navigation Research**
  (`thoughts/shared/research/2025-10-07-contextual-navigation.md`):
  Message-based patterns

## Next Steps

1. **Create design document**: DDR-09 for search feature architecture
2. **Prototype text search**: Implement Phase 1 in feature branch
3. **Evaluate YAMLPath**: Test goccy/go-yaml with real Kubernetes YAML
4. **Test position tracking**: Verify line/column accuracy for highlighting
5. **User feedback**: Test search UX with realistic workflows
6. **Documentation**: Update CLAUDE.md with search usage patterns
