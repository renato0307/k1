# Keyboard Shortcuts: k9s Alignment Research

**Date:** 2025-11-04
**Author:** @renato0307
**Status:** Research Complete - Ready for Implementation

## Context

The k1 team is heavy users of k9s and struggling to transition to k1's
current keyboard shortcuts. This research explores aligning k1's shortcuts
with k9s conventions to reduce friction and improve muscle memory
compatibility.

## Problem Statement

**Current k1 approach:**
- Type any character → Start filtering/searching
- `:` → Navigate to screens/resources
- `/` → Command palette (actions)
- `ctrl+p`/`ctrl+n` → Context switching
- Resource operations use `ctrl+` modifiers (ctrl+y, ctrl+d, ctrl+x, ctrl+e)

**k9s approach:**
- `/` → Search/filter (explicit activation)
- `:` → Command/resource navigation
- Single-key shortcuts for operations (d, e, l, y)
- `[` / `]` → History navigation

**Key pain points:**
1. `/` and `:` semantics are swapped between k9s and k1
2. k1's "type to filter" conflicts with k9s explicit `/` search
3. Resource operations require `ctrl+` modifiers (less ergonomic)
4. No vim-style navigation (j/k/g/G)
5. No help overlay (`?`)

## Proposed Solution

### Final Keyboard Scheme

#### Core Navigation
- **`/`** → Search/filter (k9s, vim style)
- **`:`** → Resource/screen navigation (k9s style) [ALREADY CORRECT]
- **`ctrl+p`** → Command palette (VS Code style)
- **`?`** → Help overlay (vim, k9s style)

**Rationale:**
- `/` for search matches k9s, vim, less, man pages (universal)
- `:` already correct in k1
- `ctrl+p` matches VS Code spirit (ctrl+shift+p in GUI, but terminals
  only support ctrl modifiers)
- `?` is standard help key in vim, k9s, many TUIs

#### Context Switching
- **`[`** → Previous context (was ctrl+p)
- **`]`** → Next context (was ctrl+n)

**Rationale:**
- Frees up `ctrl+p` for command palette
- Brackets suggest directional navigation (left/right)
- Single-key shortcuts more ergonomic
- Matches k9s history navigation pattern

#### Resource Operations (k9s alignment)
- **`d`** → Describe (move from ctrl+d)
- **`e`** → Edit (move from ctrl+e)
- **`l`** → Logs (move from ctrl+l)
- **`y`** → YAML (move from ctrl+y)
- **`ctrl+x`** or **`ctrl+k`** → Delete (keep one, consider k9s ctrl+d)
- **`n`** → Namespace filter (NEW)
- **`c`** → Copy to clipboard (NEW - future)

**Rationale:**
- Single keys are more ergonomic than ctrl+ modifiers
- Direct k9s compatibility for muscle memory
- Mnemonic: d=describe, e=edit, l=logs, y=yaml, n=namespace

#### Vim-style Navigation (k9s compatible)
- **`j`** → Down (supplement arrow keys)
- **`k`** → Up (supplement arrow keys)
- **`g`** → Jump to top
- **`G`** (shift+g) → Jump to bottom

**Rationale:**
- Standard vim navigation
- k9s supports these
- Supplements arrow keys (doesn't replace them for accessibility)
- j/k for line-by-line, g/G for jumping

#### Global Operations (Keep as-is)
- **`:q`** → Quit (command only, no single-key quit)
- **`ctrl+c`** → Quit (standard terminal interrupt)
- **`ctrl+r`** → Refresh
- **`esc`** → Back/clear filter

**Rationale:**
- Remove single `q` key to prevent accidental quits
- `:q` matches vim/k9s command style
- `ctrl+c` is universal terminal quit convention
- `ctrl+r` is good addition (manual refresh)
- `esc` universally understood as "back/cancel"

### Rejected Alternatives

#### Alternative 1: Match k9s exactly
- **`/`** → Search
- **`:`** → Command palette
- **Shift+Colon** → Resource navigation

**Rejected because:**
- User likes `:` for resources (cleaner mental model)
- Two colons (shifted vs unshifted) adds confusion
- Command palette can use different key

#### Alternative 2: Keep k1's approach but make search explicit
- **`ctrl+f` or `s`** → Activate search
- **`:`** → Resources
- **`/`** → Commands

**Rejected because:**
- `/` for search is too universal to ignore (vim, k9s, less, man)
- Team muscle memory expects `/` to be search
- Would still feel different from k9s

#### Alternative 3: Modal approach with indicator
- Add vim-like mode indicator (NORMAL/SEARCH)
- Different key meanings in different modes

**Rejected because:**
- Adds complexity
- Overkill for a TUI
- k9s doesn't have modes

#### Alternative 4: Use `ctrl+r` for search
- User's original suggestion
- **Rejected because:** Conflicts with bash/zsh reverse history search
- **However:** Acceptable in TUI context (not a shell)
- Chose `ctrl+f` as more universal "find" convention

## k9s to k1 Detailed Mapping

### Resource Operations

| k9s Key | k9s Action | k1 Current | Status | Decision |
|---------|-----------|------------|--------|----------|
| **0-9** | View by index | ❌ None | SKIP | Low priority - arrow keys work |
| **a** | Attach | ❌ None | DEFER | Already has `/shell` command |
| **c** | Copy | ❌ None | FUTURE | HIGH priority for Phase 3 |
| **n** | Namespace | ❌ None | ADD | Phase 2 - namespace filter |
| **ctrl+d** | Delete | `ctrl+d`=Describe | CONFLICT | Keep `ctrl+x` or add `ctrl+k` |
| **d** | Describe | `ctrl+d` | ADD | Move describe to `d` |
| **e** | Edit | `ctrl+e` | ADD | Move edit to `e` |
| **shift+j** | Auto-scroll | ❌ None | SKIP | No streaming logs yet |
| **ctrl+k** | Kill/Delete | `ctrl+x` | CONSIDER | Alias for delete? |
| **l** | Logs | `ctrl+l` | ADD | Move logs to `l` |
| **p** | Previous logs | ❌ Command | DEFER | Phase 3 - `shift+p` |

### General Operations

| k9s Key | k9s Action | k1 Current | Status | Decision |
|---------|-----------|------------|--------|----------|
| **ctrl+a** | All resources | ❌ None | SKIP | Has `:` palette |
| **esc** | Back/Clear | ✅ `esc` | KEEP | Already aligned |
| **ctrl+u** | Clear filter | ❌ None | SKIP | `esc` already clears |
| **:cmd** | Command mode | ✅ `:` | KEEP | Already aligned |
| **tab/backtab** | Next/prev | ❌ None | SKIP | Single-pane focus |
| **/term** | Filter | Type to filter | CHANGE | Make `/` explicit |
| **?** | Help | ❌ None | ADD | Phase 2 - help overlay |
| **space** | Mark | ❌ None | DEFER | Phase 3 - batch ops |
| **ctrl+\\** | Wide cols | ❌ None | SKIP | Auto-sizing works |
| **ctrl+space** | Mark all | ❌ None | DEFER | Requires mark first |
| **:q** | Quit | `q`, `ctrl+c` | CHANGE | Remove `q`, keep `:q` and `ctrl+c` only |

### Navigation

| k9s Key | k9s Action | k1 Current | Status | Decision |
|---------|-----------|------------|--------|----------|
| **j** | Down | Arrow down | ADD | Phase 2 - vim nav |
| **k** | Up | Arrow up | ADD | Phase 2 - vim nav |
| **shift+g** | Bottom | ❌ None | ADD | Phase 2 - `G` jump |
| **g** | Top | ❌ None | ADD | Phase 2 - `g` jump |
| **[** | Back | `esc` | CONSIDER | Already have `esc` |
| **]** | Forward | ❌ None | ADD | Phase 1 - next context |
| **-** | Toggle fold | ❌ None | N/A | Full-screen views |
| **h** | YAML | `ctrl+y` | CONSIDER | Prefer `y` |
| **ctrl+f** | Page down | PgDn | KEEP | Already works |
| **ctrl+b** | Page up | PgUp | KEEP | Already works |
| **l** | Right | ❌ None | N/A | No horizontal nav |

### Context Switching

| k9s Key | k9s Action | k1 Current | Status | Decision |
|---------|-----------|------------|--------|----------|
| **ctrl+n** | Next context | ✅ `ctrl+n` | CHANGE | Move to `]` |
| **ctrl+p** | Prev context | ✅ `ctrl+p` | CHANGE | Move to `[` |

**Reason for change:** Free up `ctrl+p` for command palette

## Implementation Phases

### Phase 1: Core Shortcuts (Breaking Changes)

**Goal:** Align core k9s muscle memory

**Changes:**
1. Swap command bar activation:
   - `/` → Filter/search (was command palette)
   - `ctrl+p` → Command palette (new)
   - Keep `:` as-is (already correct)

2. Move context switching:
   - `[` → Previous context (was ctrl+p)
   - `]` → Next context (was ctrl+n)

3. Move resource operations to single keys:
   - `d` → Describe (was ctrl+d)
   - `e` → Edit (was ctrl+e)
   - `l` → Logs (was ctrl+l)
   - `y` → YAML (was ctrl+y)
   - Keep `ctrl+x` for delete

4. Remove "type to filter" behavior:
   - Only `/` activates filter
   - Prevents accidental filtering

5. Update command bar default hint:
   - Update default hint text: `:` screens  `/` search  `ctrl+p` palette  `?` help
   - Shows when no rotating tip is displayed
   - Keep rotating tips system for additional feature discovery

**Files to modify:**
- `internal/app/app.go` - Global key handling
- `internal/components/commandbar/commandbar.go` - Activation keys, default hint
- `internal/commands/registry.go` - Command shortcuts
- `internal/components/commandbar/palette.go` - Shortcut display

**Testing:**
- Build: `make build`
- Manual test all shortcuts
- User confirms k9s muscle memory works

### Phase 2: Navigation & Help

**Goal:** Add vim navigation and help overlay

**Changes:**
1. Add vim navigation:
   - `j`/`k` → Up/down (supplement arrows)
   - `g` → Jump to top
   - `G` → Jump to bottom

2. Add help overlay:
   - `?` → Show all shortcuts
   - Organized by category (Navigation, Resources, Context, Global)
   - `esc` to dismiss

3. Add namespace filter:
   - `n` → Quick namespace filter

**Files to modify:**
- `internal/app/app.go` - Add vim keys and `?`
- `internal/screens/help.go` (NEW) - Help screen
- Table component wrapper - vim navigation handlers

**Testing:**
- Vim navigation works in all screens
- Help overlay shows complete shortcut reference
- Namespace filter opens palette

### Phase 3: Enhanced Features (Future)

**Goal:** Additional k9s features

**Changes:**
1. Copy functionality (`c` key)
2. Mark/select items (`space` key)
3. Previous logs shortcut (`shift+p`)
4. Attach/shell shortcut (`a` key)

**Status:** Deferred to future work

## Technical Implementation Details

### Command Bar State Machine Changes

Current states:
1. StateHidden
2. StateFilter
3. StateSuggestionPalette
4. StateInput
5. StateConfirmation
6. StateLLMPreview
7. StateResult

**Changes needed:**
- `/` triggers StateFilter (currently triggers StateSuggestionPalette)
- `ctrl+p` triggers StateSuggestionPalette (new)
- Remove auto-filter on typing (only activate on `/`)

### Key Routing Changes

**Before (app.go lines 207-250):**
```go
case "ctrl+p": // Previous context
case "ctrl+n": // Next context
// Default: Any key → start filter
```

**After:**
```go
case "[": // Previous context
case "]": // Next context
case "/": // Activate filter
case "ctrl+p": // Command palette
case "d", "e", "l", "y": // Resource ops (delegate to commands)
case "j", "k", "g", "G": // Vim navigation
case "?": // Help overlay
case "n": // Namespace filter
```

### Command Registry Changes

**Before (registry.go):**
```go
{Name: "yaml", Shortcut: "ctrl+y", ...}
{Name: "describe", Shortcut: "ctrl+d", ...}
{Name: "edit", Shortcut: "ctrl+e", ...}
{Name: "logs", Shortcut: "ctrl+l", ...}
```

**After:**
```go
{Name: "yaml", Shortcut: "y", ...}
{Name: "describe", Shortcut: "d", ...}
{Name: "edit", Shortcut: "e", ...}
{Name: "logs", Shortcut: "l", ...}
```

### Help Screen Content

**Categories:**
1. **Core Navigation**
   - `/` - Search/filter current list
   - `:` - Navigate to resource/screen
   - `ctrl+p` - Open command palette
   - `esc` - Back/clear filter

2. **Resource Operations**
   - `d` - Describe selected resource
   - `e` - Edit resource (copies to clipboard)
   - `l` - View logs (pods only)
   - `y` - View YAML
   - `ctrl+x` - Delete resource
   - `n` - Filter by namespace

3. **Context Switching**
   - `[` - Previous Kubernetes context
   - `]` - Next Kubernetes context

4. **List Navigation**
   - `↑`/`↓` or `j`/`k` - Move selection
   - `g` - Jump to top
   - `G` - Jump to bottom
   - `PgUp`/`PgDn` or `ctrl+b`/`ctrl+f` - Page up/down

5. **Global**
   - `:q` or `ctrl+c` - Quit application
   - `ctrl+r` - Refresh data
   - `?` - Show this help

6. **Command Palette** (when active with `ctrl+p`)
   - `↑`/`↓` - Navigate suggestions
   - `Enter` - Execute command
   - `Tab` - Auto-complete
   - `esc` - Cancel

## Breaking Changes & Migration

### User Impact

**Breaking changes:**
1. Command palette moved from `/` to `ctrl+p`
2. Filter now requires `/` (no auto-filter on typing)
3. Context switching from `ctrl+p`/`ctrl+n` to `[`/`]`
4. Resource operations from `ctrl+key` to single key

### Migration Strategy

**Option A: Hard cutover (Recommended)**
- Implement all changes in Phase 1
- Update documentation
- Show new shortcuts in rotating tips
- Users learn new muscle memory

**Option B: Grace period**
- Keep old shortcuts working for 1-2 releases
- Show deprecation warnings in tips
- Remove in future release

**Recommendation:** Option A (hard cutover)
- Clean break, no confusion
- Team already learning new tool
- Better to learn correct way immediately

### Documentation Updates

1. **README.md:**
   - Update keyboard shortcuts section
   - Highlight k9s compatibility

2. **CLAUDE.md:**
   - Update shortcuts reference
   - Document Phase 1/2/3 completion status

3. **Command bar default hint:**
   - Update default hint (shown when no rotating tip is displayed)
   - Display: `:` screens  `/` search  `ctrl+p` palette  `?` help
   - Provides immediate discoverability for core shortcuts

4. **Rotating usage tips:**
   - Keep rotating tips system (for additional feature discovery)
   - Update all tip text with new shortcuts
   - Add tips about help overlay (`?`)
   - Rotates with the updated default hint

## Success Criteria

### Phase 1 Success
- [ ] `/` opens filter mode
- [ ] `ctrl+p` opens command palette
- [ ] `[` / `]` switch contexts
- [ ] `d`, `e`, `l`, `y` perform operations
- [ ] No auto-filter on typing
- [ ] User confirms k9s muscle memory works

### Phase 2 Success
- [ ] `j`/`k` navigate lists
- [ ] `g`/`G` jump to top/bottom
- [ ] `?` shows help screen
- [ ] `n` triggers namespace filter
- [ ] Help screen shows all shortcuts

### Phase 3 Success
- [ ] `c` copies resource details
- [ ] `space` marks items
- [ ] Batch operations work on marked items

## Risks & Mitigations

**Risk:** Breaking changes confuse users
**Mitigation:**
- Clear documentation
- Help overlay (`?`) always available
- Rotating tips show new shortcuts

**Risk:** Accidental quits
**Mitigation:**
- Remove single `q` key (prevents accidental quits)
- Require `:q` command or `ctrl+c` to quit
- Both require intentional action

**Risk:** Single-key shortcuts trigger accidentally
**Mitigation:**
- Only active when command bar hidden
- Command bar and filter capture keys first
- Unlikely to type `d`/`e`/`l`/`y` accidentally in list view

**Risk:** Missing features from k9s
**Mitigation:**
- Phase 3 adds copy, mark/select
- Users can request features via issues
- Core operations covered in Phase 1

## Related Research

- `2025-10-09-k8s-context-management.md` - Context switching implementation
- `2025-11-02-command-output-history-design.md` - Command bar states
- `2025-11-02-usage-tips-display-alternatives.md` - Tip rotation system

## Next Steps

1. ✅ Research complete
2. User reviews and approves plan
3. Create implementation branch: `feat/k9s-keyboard-shortcuts`
4. Implement Phase 1 (core shortcuts)
5. Build and user test
6. Commit Phase 1 after approval
7. Implement Phase 2 (navigation + help)
8. Build and user test
9. Commit Phase 2 after approval
10. Defer Phase 3 to future work
11. Update all documentation
12. Close keyboard shortcuts alignment issue (if exists)

## Conclusion

Aligning k1's keyboard shortcuts with k9s conventions will significantly
reduce friction for the team's transition. The proposed scheme:
- Keeps universal conventions (`/` search, `:` navigation)
- Adds familiar command palette (`ctrl+p`)
- Improves ergonomics (single keys for operations)
- Maintains k1's unique features (command bar, AI commands)
- Provides clear migration path (help overlay)

The three-phase approach balances immediate impact (Phase 1) with
incremental enhancements (Phases 2-3), allowing user feedback to guide
future development.
