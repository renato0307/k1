# Plan: Update Message Styling to Match Claude Code

**Date**: 2025-10-31
**Status**: TODO
**Author**: @renato0307

## Goal

Update k1's message display to match Claude Code's visual style:
- **Loading**: Orange spinner (asterisk-style) + orange text
- **Error**: Red circle (⏺) + white text
- **Info**: Blue circle (⏺) + white text
- **Success**: Green circle (⏺) + white text

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

**File**: `internal/components/statusbar.go`

Current: Bubble Tea's `spinner.Dot` style
Target: Asterisk-style rotating animation (match Claude Code's "✢")

Options to explore:
1. Try Bubble Tea's built-in spinner styles (Points, Globe, etc.)
2. Create custom spinner with rotating characters: `✢ ✣ ✤ ✥` or similar
3. Use existing Braille spinner from header.go: `⠋ ⠙ ⠹ ⠸ ⠼ ⠴ ⠦ ⠧`

Apply orange color (theme.MessageLoading) to spinner characters.

Test animation smoothness and visual appearance.

### Phase 4: Testing & Verification

Test all message types:
1. Success message (green circle, white text, auto-clear after 5s)
2. Error message (red circle, white text, persists)
3. Info message (blue circle, white text, persists)
4. Loading message (orange spinner, orange text, clears on complete)

Verify across all 8 themes:
- White text readable on all theme backgrounds
- Circle colors consistent with theme palette
- Spinner animation smooth and visible
- No background color artifacts

Test message lifecycle:
- Success auto-clears after 5 seconds
- Errors persist until explicitly cleared
- Loading clears on RefreshCompleteMsg

## Files Modified

- `internal/ui/theme.go` - Add 5 new color fields to all themes
- `internal/components/statusbar.go` - Update rendering logic and spinner
- No changes to message creation (`internal/messages/helpers.go`)
- No changes to message types (`internal/types/types.go`)

## Success Criteria

- [ ] All 8 themes have new message color fields defined
- [ ] Messages render with colored circle + white text (no background)
- [ ] Spinner uses asterisk-style animation with orange color
- [ ] Visual appearance matches Claude Code style
- [ ] Message lifecycle behavior unchanged (auto-clear, persistence)
- [ ] White text readable across all themes and terminal backgrounds
- [ ] All existing tests pass

## Notes

- Keep existing message helpers and types unchanged (backward compatible)
- Maintain spinner update mechanism via Update() method
- Consider terminal background contrast (white text on light backgrounds)
- Document new theme color fields in theme.go
- No breaking changes to message API
