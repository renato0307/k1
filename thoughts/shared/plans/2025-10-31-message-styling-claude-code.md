# Plan: Update Message Styling to Match Claude Code

**Date**: 2025-10-31
**Status**: COMPLETE
**Author**: @renato0307

## Goal

Update k1's message display to match Claude Code's visual style:
- **Loading**: Orange spinner (asterisk-style) + orange text
- **Error**: Red circle (⏺) + red text
- **Info**: Blue circle (⏺) + blue text
- **Success**: Green circle (⏺) + green text

**Note**: Text color changed to match circle color for consistency.

## Current Implementation

### Message Display (StatusBar)
- Location: `internal/components/statusbar.go`
- Current style: Full-width colored background + dark text + symbols (✓/✗/ℹ)
- Colors defined: theme.Success, theme.Error, theme.Primary, theme.Accent
- Text color: theme.Background (dark)
- Spinner: Bubble Tea's `spinner.Dot` style

### Message Types
- Location: `internal/types/types.go`
- Types: MessageTypeInfo, MessageTypeSuccess, MessageTypeError,
  MessageTypeLoading
- Helpers: InfoMsg(), SuccessMsg(), ErrorStatusMsg(), LoadingMsg()

### Theme System
- Location: `internal/ui/theme.go`
- Defines colors for 8 themes (charm, dracula, catppuccin, nord, gruvbox,
  tokyo-night, solarized, monokai)
- Current colors: Primary, Secondary, Accent, Success, Error, Warning, etc.

## Changes Required

### Phase 1: Add Message Colors to Theme System

**File**: `internal/ui/theme.go`

Add new color fields to Theme struct:
```go
type Theme struct {
    // ... existing fields ...

    // Message colors (Claude Code style)
    MessageText     lipgloss.AdaptiveColor // White text for messages
    MessageSuccess  lipgloss.AdaptiveColor // Green circle
    MessageError    lipgloss.AdaptiveColor // Red circle
    MessageInfo     lipgloss.AdaptiveColor // Blue circle
    MessageLoading  lipgloss.AdaptiveColor // Orange spinner/text
}
```

Update all 8 theme definitions with new colors:
- MessageText: White (#FFFFFF) across all themes
- MessageSuccess: Green tones (match existing theme style)
- MessageError: Red tones (match existing theme style)
- MessageInfo: Blue tones (match existing theme style)
- MessageLoading: Orange tones (#FF8800 or similar)

Ensure colors are consistent with each theme's palette but follow Claude
Code's visual pattern (colored circle + white text).

### Phase 2: Update StatusBar Rendering

**File**: `internal/components/statusbar.go`

Changes to message rendering:
1. Replace symbol prefixes (✓/✗/ℹ) with colored circle (⏺) character
2. Use theme.MessageText (white) for all message text
3. Remove background color fills (transparent background)
4. Use new theme colors for circle bullets:
   - Success: theme.MessageSuccess + "⏺ "
   - Error: theme.MessageError + "⏺ "
   - Info: theme.MessageInfo + "⏺ "
   - Loading: theme.MessageLoading + spinner
5. Remove bold styling for cleaner look

Example rendering logic:
```go
var circleColor lipgloss.AdaptiveColor
var prefix string

switch sb.messageType {
case types.MessageTypeSuccess:
    circleColor = sb.theme.MessageSuccess
    prefix = "⏺ "
case types.MessageTypeError:
    circleColor = sb.theme.MessageError
    prefix = "⏺ "
case types.MessageTypeInfo:
    circleColor = sb.theme.MessageInfo
    prefix = "⏺ "
case types.MessageTypeLoading:
    circleColor = sb.theme.MessageLoading
    prefix = sb.spinner.View() + " "
}

circleStyle := lipgloss.NewStyle().Foreground(circleColor)
textStyle := lipgloss.NewStyle().Foreground(sb.theme.MessageText)

rendered := circleStyle.Render(prefix) + textStyle.Render(sb.message)
```

### Phase 3: Update Spinner Style

**File**: `internal/components/usermessage.go` (renamed from statusbar.go)

**Implemented**: Custom spinner with exact Claude Code symbols
- Symbols: ✽ ✻ ✶ · ✢
- FPS: 6 frames per second (slower animation)
- Color: theme.MessageLoading (orange)

Applied to loading messages only, rendered via spinnerView parameter.

### Phase 4: Architectural Refactoring (Added During Implementation)

**Purpose**: Eliminate code duplication and follow Single Responsibility
Principle

**Changes**:
1. Created `internal/ui/message.go` - Pure rendering function
   - `RenderMessage()` - Shared rendering logic for all message types
   - Stateless function accepting text, type, theme, spinnerView
   - Single source of truth for message styling

2. Renamed `internal/components/statusbar.go` → `usermessage.go`
   - Component name: StatusBar → UserMessage
   - Better semantic meaning (it's user-facing messages, not just status)
   - Delegates rendering to ui.RenderMessage()

3. Updated `internal/components/commandbar/executor.go`
   - ViewResult() now uses ui.RenderMessage()
   - Eliminated duplicate rendering logic
   - Consistent styling between command bar and status area

4. Updated `internal/app/app.go`
   - Renamed all statusBar references → userMessage
   - Added messageID tracking for timer anti-race pattern
   - Fixed bug: messages disappearing prematurely due to timer conflicts
   - Increment messageID on new message, only clear if ID matches

**Architecture**:
```
ui.RenderMessage() (pure function, DRY)
    ↑
    ├── UserMessage.View() (main status display)
    └── Executor.ViewResult() (command bar results)
```

### Phase 5: Message Truncation (Added During Implementation)

**Purpose**: Prevent long messages from pushing title line off screen

**Changes**:
1. Updated `ui.RenderMessage()` to accept terminal width parameter
2. Truncates messages to `width - 7` (prefix + margins)
3. Adds "…" ellipsis when truncated
4. Minimum length of 20 characters for small terminals

**Callers updated**:
- `UserMessage.View()` passes `um.width`
- `Executor.ViewResult()` passes `e.width`

### Phase 6: Error Logging (Added During Implementation)

**Purpose**: Preserve full error messages for debugging after truncation

**Changes**:
1. Added error logging in `internal/app/app.go`
2. Logs full error message before truncation
3. Available in log file when running with `-log-file` flag
4. Format: `level=ERROR msg="User error message" message="..."`

### Phase 7: Testing & Verification

Test all message types:
1. Success message (green circle, green text, auto-clear after 5s)
2. Error message (red circle, red text, persists)
3. Info message (blue circle, blue text, persists)
4. Loading message (orange spinner, orange text, clears on complete)

Verify across all 8 themes:
- Text color matches circle color for all themes
- Circle colors consistent with theme palette
- Spinner animation smooth and visible
- No background color artifacts

Test message lifecycle:
- Success auto-clears after 5 seconds
- Errors persist until explicitly cleared
- Loading clears on RefreshCompleteMsg

Test message truncation:
- Long messages truncated to fit terminal width
- Ellipsis added when truncated
- Title line never pushed off screen
- Full error available in log file

## Files Modified

- `internal/ui/theme.go` - Added 4 message color fields to all 8 themes
- `internal/ui/message.go` - NEW: Pure rendering function with truncation
- `internal/components/statusbar.go` → `usermessage.go` - RENAMED
- `internal/components/commandbar/executor.go` - Uses shared renderer
- `internal/components/commandbar/executor_test.go` - Updated expectations
- `internal/app/app.go` - Renamed references, messageID tracking, error logging
- `internal/types/types.go` - Added MessageID to ClearStatusMsg
- `internal/commands/executor.go` - Trimmed kubectl stderr output

## Success Criteria

- [x] All 8 themes have new message color fields defined
- [x] Messages render with colored circle + colored text (no background)
- [x] Text color matches circle color for consistency
- [x] Spinner uses exact Claude Code symbols (✽ ✻ ✶ · ✢)
- [x] Visual appearance matches Claude Code style
- [x] Message timer bugs fixed (messageID anti-race pattern)
- [x] Code duplication eliminated (DRY with ui.RenderMessage)
- [x] Single Responsibility Principle followed (UserMessage component)
- [x] Long messages truncated based on terminal width
- [x] Full error messages logged for debugging
- [x] Title line never pushed off screen
- [x] All existing tests pass

## Notes

### Implementation Changes
- Changed text color from white to match circle color (user feedback)
- Renamed StatusBar → UserMessage for better semantics
- Created ui.RenderMessage() pure function for DRY
- Fixed message timer race condition with messageID tracking

### Backward Compatibility
- Message helpers and types unchanged (backward compatible)
- Spinner update mechanism maintained via Update() method
- No breaking changes to message API

### Key Learnings
- UTF-8 encoding: Used bash heredoc for circle bullet character (⏺)
- Spinner symbols: Exact match to Claude Code (✽ ✻ ✶ · ✢)
- Timer coordination: messageID prevents old timers from clearing new
  messages
- Architecture: SRP separation of rendering (pure function) from state
  management (component)
- Message truncation: Must use terminal width, not hardcoded values
- Logging: Preserve full error messages before truncation for debugging
- Bubble Tea layout: Long messages can push content off screen if not
  truncated
