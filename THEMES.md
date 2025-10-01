# Theming in Bubble Tea Applications

Research findings on implementing themes in Bubble Tea TUI applications.

## Overview

Bubble Tea applications use [Lipgloss](https://github.com/charmbracelet/lipgloss) for styling. Themes are collections of lipgloss.Style objects organized into structs that can be applied to different components.

## Key Libraries

- **lipgloss**: Core styling library (`github.com/charmbracelet/lipgloss`)
- **lipgloss.AdaptiveColor**: Support light/dark terminal modes
- **colorprofile**: Detect terminal color capabilities (`github.com/charmbracelet/colorprofile`)
- **catppuccin/go**: Optional external color schemes (popular)

## Theme Structure Patterns

### Pattern 1: Component-Based (Huh Library)

Used by `charmbracelet/huh` - organized by component type with focused/blurred states:

```go
type Theme struct {
    Form           FormStyles
    Group          GroupStyles
    FieldSeparator lipgloss.Style
    Blurred        FieldStyles
    Focused        FieldStyles
    Help           help.Styles
}

type FieldStyles struct {
    Base           lipgloss.Style
    Title          lipgloss.Style
    Description    lipgloss.Style
    ErrorIndicator lipgloss.Style
    ErrorMessage   lipgloss.Style
    SelectSelector lipgloss.Style
    Option         lipgloss.Style
    // ... more fields
}
```

**Key Features:**
- Separation of focused vs blurred states
- Hierarchical organization (Form > Group > Field)
- Factory functions return fully-configured themes: `ThemeCharm()`, `ThemeDracula()`, `ThemeBase16()`, `ThemeCatppuccin()`
- Base theme provides defaults, specific themes inherit and override

### Pattern 2: Flat Styles Struct (Soft-Serve)

Used by `charmbracelet/soft-serve` - flat structure with nested component styles:

```go
type Styles struct {
    ActiveBorderColor   color.Color
    InactiveBorderColor color.Color

    App                  lipgloss.Style
    ServerName           lipgloss.Style
    TopLevelNormalTab    lipgloss.Style
    TopLevelActiveTab    lipgloss.Style

    RepoSelector struct {
        Normal struct {
            Base    lipgloss.Style
            Title   lipgloss.Style
            Desc    lipgloss.Style
        }
        Active struct {
            Base    lipgloss.Style
            Title   lipgloss.Style
            Desc    lipgloss.Style
        }
    }

    LogItem struct {
        Normal struct { /* ... */ }
        Active struct { /* ... */ }
    }

    StatusBar       lipgloss.Style
    StatusBarKey    lipgloss.Style
    StatusBarValue  lipgloss.Style
    // ... more fields
}
```

**Key Features:**
- Direct access to styles without nesting levels
- Color values stored separately from styles
- Single `DefaultStyles()` function creates theme
- Active/Normal states within component structs

## Adaptive Colors (Light/Dark Mode)

Lipgloss supports automatic light/dark adaptation:

```go
// Define colors that adapt to terminal background
normalFg := lipgloss.AdaptiveColor{Light: "235", Dark: "252"}
indigo   := lipgloss.AdaptiveColor{Light: "#5A56E0", Dark: "#7571F9"}
green    := lipgloss.AdaptiveColor{Light: "#02BA84", Dark: "#02BF87"}

// Apply to styles
style := lipgloss.NewStyle().Foreground(normalFg)
```

**Color Formats Supported:**
- ANSI 256 colors: `"240"`, `"57"`, `"229"`
- Hex colors: `"#5A56E0"`, `"#FF4672"`
- Adaptive colors: `AdaptiveColor{Light: "...", Dark: "..."}`

## Table Styling (Bubbles Table Component)

From `bubbles/table` package:

```go
type Styles struct {
    Header   lipgloss.Style
    Cell     lipgloss.Style
    Selected lipgloss.Style
}

// Example from proto-pods-tui
s := table.DefaultStyles()
s.Header = s.Header.
    BorderStyle(lipgloss.NormalBorder()).
    BorderForeground(lipgloss.Color("240")).
    BorderBottom(true).
    Bold(false)
s.Selected = s.Selected.
    Foreground(lipgloss.Color("229")).
    Background(lipgloss.Color("57")).
    Bold(false)
```

## Recommended Approach for Timoneiro

### 1. Define Theme Structure

Create `internal/ui/theme.go`:

```go
package ui

import "github.com/charmbracelet/lipgloss"

// Theme defines the color scheme and styles for the TUI
type Theme struct {
    Name string

    // Core colors
    Primary     lipgloss.AdaptiveColor
    Secondary   lipgloss.AdaptiveColor
    Accent      lipgloss.AdaptiveColor
    Background  lipgloss.AdaptiveColor
    Foreground  lipgloss.AdaptiveColor
    Muted       lipgloss.AdaptiveColor
    Error       lipgloss.AdaptiveColor
    Success     lipgloss.AdaptiveColor
    Warning     lipgloss.AdaptiveColor

    // Component styles
    Table       TableStyles
    Header      lipgloss.Style
    Footer      lipgloss.Style
    StatusBar   lipgloss.Style
    FilterInput lipgloss.Style
}

type TableStyles struct {
    Header        lipgloss.Style
    Cell          lipgloss.Style
    SelectedRow   lipgloss.Style
    Border        lipgloss.Style
    NamespaceCell lipgloss.Style // Custom for namespace column
    StatusRunning lipgloss.Style // Green for running
    StatusError   lipgloss.Style // Red for error
}
```

### 2. Create Theme Presets

```go
// ThemeCharm returns the default Charm theme
func ThemeCharm() *Theme {
    t := &Theme{Name: "charm"}

    // Define adaptive colors
    t.Primary = lipgloss.AdaptiveColor{Light: "#5A56E0", Dark: "#7571F9"}
    t.Success = lipgloss.AdaptiveColor{Light: "#02BA84", Dark: "#02BF87"}
    t.Error = lipgloss.AdaptiveColor{Light: "#FF4672", Dark: "#ED567A"}
    t.Foreground = lipgloss.AdaptiveColor{Light: "235", Dark: "252"}
    t.Muted = lipgloss.AdaptiveColor{Light: "243", Dark: "243"}

    // Build component styles
    t.Table.Header = lipgloss.NewStyle().
        BorderStyle(lipgloss.NormalBorder()).
        BorderForeground(t.Muted).
        BorderBottom(true).
        Foreground(t.Primary).
        Bold(true)

    t.Table.SelectedRow = lipgloss.NewStyle().
        Foreground(lipgloss.Color("229")).
        Background(lipgloss.Color("57"))

    t.Table.StatusRunning = lipgloss.NewStyle().Foreground(t.Success)
    t.Table.StatusError = lipgloss.NewStyle().Foreground(t.Error)

    return t
}

// ThemeDracula returns a Dracula-inspired theme
func ThemeDracula() *Theme {
    t := &Theme{Name: "dracula"}

    t.Primary = lipgloss.AdaptiveColor{Dark: "#bd93f9"}
    t.Success = lipgloss.AdaptiveColor{Dark: "#50fa7b"}
    t.Error = lipgloss.AdaptiveColor{Dark: "#ff5555"}
    // ... more colors

    return t
}

// ThemeCatppuccin returns Catppuccin theme
func ThemeCatppuccin() *Theme {
    // Use github.com/catppuccin/go for official colors
    t := &Theme{Name: "catppuccin"}
    // ... implementation
    return t
}
```

### 3. Apply Theme to Model

Update `model` struct:

```go
type model struct {
    pods   []Pod
    table  table.Model
    theme  *Theme  // Add theme field
    // ... other fields
}

func initialModel(lister v1listers.PodLister, themeName string) model {
    // Load theme
    var theme *Theme
    switch themeName {
    case "dracula":
        theme = ThemeDracula()
    case "catppuccin":
        theme = ThemeCatppuccin()
    default:
        theme = ThemeCharm()
    }

    // Create table with theme
    columns := []table.Column{/* ... */}
    t := table.New(
        table.WithColumns(columns),
        table.WithFocused(true),
        table.WithHeight(20),
    )

    // Apply theme styles
    s := table.Styles{
        Header:   theme.Table.Header,
        Cell:     theme.Table.Cell,
        Selected: theme.Table.SelectedRow,
    }
    t.SetStyles(s)

    return model{
        table: t,
        theme: theme,
        // ...
    }
}
```

### 4. Dynamic Styling with Theme

Use theme for conditional colors:

```go
func (m model) View() string {
    // Apply status-specific styling
    for i, pod := range m.filteredPods {
        statusStyle := m.theme.Table.Cell
        if strings.Contains(pod.Status, "Running") {
            statusStyle = m.theme.Table.StatusRunning
        } else if strings.Contains(pod.Status, "Error") {
            statusStyle = m.theme.Table.StatusError
        }
        // Apply style...
    }
}
```

### 5. Runtime Theme Switching

Add keybinding to switch themes:

```go
case "t":
    // Cycle through themes
    themes := []string{"charm", "dracula", "catppuccin"}
    currentIndex := 0
    for i, name := range themes {
        if m.theme.Name == name {
            currentIndex = i
            break
        }
    }
    nextIndex := (currentIndex + 1) % len(themes)
    m.theme = loadTheme(themes[nextIndex])
    m.applyThemeToTable()
    return m, nil
```

### 6. Configuration Persistence

Store theme preference in config file:

```go
// ~/.config/timoneiro/config.yaml
theme: "charm"
```

Load on startup:

```go
func loadConfig() (*Config, error) {
    // Read ~/.config/timoneiro/config.yaml
    // Return Config with theme preference
}
```

## Best Practices

1. **Use AdaptiveColor**: Always define Light and Dark variants for automatic terminal adaptation
2. **Provide Defaults**: Start with a base theme and allow overrides
3. **Semantic Naming**: Use names like `Primary`, `Error`, `Success` instead of colors like `Red`, `Blue`
4. **Factory Functions**: Create theme constructors like `ThemeCharm()` that return fully-configured themes
5. **Centralize Colors**: Define color palette first, then build styles from palette
6. **Test Both Modes**: Test themes in both light and dark terminal backgrounds
7. **Performance**: Theme application is cheap (just style objects), can switch at runtime

## Example Themes to Support

1. **Charm** (default): Charmbracelet's signature purple/pink theme
2. **Dracula**: Popular dark theme with purple/green accents
3. **Catppuccin**: Trendy pastel theme with light/dark variants
4. **Base16**: Classic 16-color terminal theme
5. **Gruvbox**: Popular warm theme (future consideration)
6. **Nord**: Cool blue theme (future consideration)

## External Color Scheme Libraries

- `github.com/catppuccin/go`: Official Catppuccin colors
- Terminal color profiles: Most are defined manually with AdaptiveColor

## Implementation Priority

For the prototype phase:

1. **Phase 1**: Implement basic Theme struct with Charm theme (current colors)
2. **Phase 2**: Add 1-2 alternative themes (Dracula, Catppuccin)
3. **Phase 3**: Add runtime theme switching with keybinding
4. **Phase 4**: Add config file support for persistence

## Modal Dimming and Overlays

### Current Implementation

Timoneiro uses `github.com/rmhubbert/bubbletea-overlay` for modal rendering:

```go
// From internal/app/app.go
if m.state.ShowScreenPicker {
    modalView := m.screenPicker.CenteredView(m.state.Width, m.state.Height)
    bg := &simpleModel{content: baseRendered}
    fg := &simpleModel{content: modalView}
    overlayModel := overlay.New(fg, bg, overlay.Center, overlay.Center, 0, 0)
    return overlayModel.View()
}
```

**How it works:**
- Overlay library composites foreground (modal) onto background (main view)
- Positioning: `overlay.Top`, `overlay.Right`, `overlay.Bottom`, `overlay.Left`, `overlay.Center`
- Offsets available for fine-tuning position
- Compositing algorithm preserves background text where modal doesn't cover

**Key limitation:** The overlay library does NOT provide built-in dimming - it simply overlays the modal on top of the existing view without modifying background colors.

### Dimming Techniques

#### Option 1: Unicode Box Drawing Characters (Current Best Option)

The most idiomatic approach in TUI applications is to **not dim the background** but instead use strong visual separation:

```go
// Modal with distinct background and border
modalStyle := lipgloss.NewStyle().
    Border(lipgloss.RoundedBorder()).
    BorderForeground(theme.Primary).
    Background(theme.Background).  // Solid background
    Padding(1, 2)
```

**Advantages:**
- Clean, terminal-native approach
- Works across all terminal types
- No color blending complexity
- Used by popular TUI apps (gum, huh, soft-serve)

**Best practices:**
- Use solid background color for modal (not transparent)
- Apply prominent border with theme color
- Add padding inside modal for breathing room
- Ensure modal background contrasts with foreground text

#### Option 2: ANSI 256-Color Dimming (Experimental)

For terminals supporting 256 colors, you can create a "dimmed" appearance by rendering background with darker/muted colors:

```go
// 1. Render background with muted/darker styles
dimmedStyle := lipgloss.NewStyle().
    Foreground(lipgloss.Color("240")).  // Gray out text
    Faint(true)  // ANSI faint attribute (not widely supported)

// 2. Re-render entire background with dimmed colors
dimmedBg := renderWithDimmedColors(baseView)

// 3. Composite modal on top
overlayModel := overlay.New(modal, dimmedBg, overlay.Center, overlay.Center, 0, 0)
```

**Challenges:**
- Requires re-rendering entire background with modified styles
- Performance cost (must traverse and re-style all content)
- `Faint()` ANSI attribute poorly supported across terminals
- No true transparency - must manually adjust colors

#### Option 3: Character Overlay Pattern (Advanced)

Render semi-transparent effect by overlaying characters in uncovered areas:

```go
// Render background with overlay characters
func dimBackground(bg string, width, height int, modalBounds Rect) string {
    lines := strings.Split(bg, "\n")
    dimChar := lipgloss.NewStyle().
        Foreground(lipgloss.Color("235")).
        Render("░")  // Light shade character

    for y := 0; y < len(lines); y++ {
        if y >= modalBounds.Y && y < modalBounds.Y+modalBounds.Height {
            continue  // Skip modal area
        }
        // Overlay dim pattern on line
        lines[y] = overlayDimPattern(lines[y], dimChar)
    }
    return strings.Join(lines, "\n")
}
```

**Challenges:**
- Complex implementation - must track modal bounds
- Can interfere with background text readability
- May look "busy" or noisy
- Not commonly used in production TUI apps

### Recommended Approach

**Use solid modal backgrounds with strong borders** (Option 1):

```go
// Add to Theme struct
type Theme struct {
    // ... existing fields

    Modal ModalStyles
}

type ModalStyles struct {
    Background  lipgloss.Style  // Solid modal background
    Border      lipgloss.Style  // Prominent border
    Title       lipgloss.Style  // Modal title
    Content     lipgloss.Style  // Modal content text
}

// In ThemeCharm()
t.Modal.Background = lipgloss.NewStyle().
    Background(lipgloss.Color("235")).  // Dark solid background
    Padding(1, 2).
    Border(lipgloss.RoundedBorder()).
    BorderForeground(t.Primary)

t.Modal.Title = lipgloss.NewStyle().
    Foreground(t.Primary).
    Bold(true).
    Padding(0, 0, 1, 0)

t.Modal.Content = lipgloss.NewStyle().
    Foreground(lipgloss.Color("252"))
```

**Apply to modals:**

```go
// In command_palette.go or screen_picker.go
func (m *Modal) View() string {
    title := m.theme.Modal.Title.Render("Command Palette")
    content := m.list.View()

    body := lipgloss.JoinVertical(lipgloss.Left, title, content)

    return m.theme.Modal.Background.Render(body)
}
```

### Why NOT True Dimming?

Terminal limitations make true dimming impractical:

1. **No Transparency**: Terminals don't support alpha/opacity - colors are always solid
2. **ANSI Limits**: `Faint` attribute poorly supported, no native dimming
3. **Performance**: Re-rendering background with modified colors is expensive
4. **Complexity**: Tracking which cells to dim vs. not dim is error-prone
5. **UX Convention**: TUI apps traditionally use solid backgrounds + borders for modals

### Examples from Popular TUI Apps

- **gum**: Solid modal backgrounds with borders, no dimming
- **huh**: Focused components have distinct backgrounds, no screen dimming
- **lazygit**: Panels have solid backgrounds, active panel highlighted
- **k9s**: Modals use bold borders and solid backgrounds

The TUI convention is clear visual separation, not dimming.

### Alternative: Focus Indicators

Instead of dimming, emphasize the modal:

```go
// Animate border or add shadow effect
modalWithShadow := lipgloss.NewStyle().
    Border(lipgloss.ThickBorder()).  // Thicker border
    BorderForeground(theme.Accent).  // Bright accent color
    Render(modalContent)
```

### Implementation Checklist

For production-ready modal styling:

- [ ] Add `Modal` styles to Theme struct
- [ ] Use solid backgrounds for all modals
- [ ] Apply prominent rounded/thick borders
- [ ] Ensure high contrast (modal bg vs. modal text)
- [ ] Add padding inside modals (1-2 chars)
- [ ] Consider box shadow effect with unicode characters (`▄▀`)
- [ ] Test in light and dark terminal modes
- [ ] Avoid attempting true transparency/dimming

## References

- Huh library themes: `.tmp/huh/theme.go`
- Soft-serve styles: `.tmp/soft-serve/pkg/ui/styles/styles.go`
- Lipgloss documentation: https://github.com/charmbracelet/lipgloss
- Bubble Tea examples: `.tmp/bubbletea/examples/`
- Overlay library: `github.com/rmhubbert/bubbletea-overlay`
