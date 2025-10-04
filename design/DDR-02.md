# Theming System for TUI

| Metadata | Value                    |
|----------|--------------------------|
| Date     | 2025-10-04               |
| Author   | @renato0307              |
| Status   | `Implemented`            |
| Tags     | ui, theming, lipgloss    |
| Updates  | -                        |

| Revision | Date       | Author      | Info           |
|----------|------------|-------------|----------------|
| 1        | 2025-10-04 | @renato0307 | Initial design |

## Context and Problem Statement

Timoneiro needs a flexible theming system that allows users to customize the appearance of the TUI. The system should support multiple color schemes (Charm, Dracula, Catppuccin), adapt to light/dark terminal backgrounds, and enable runtime theme switching. How should we structure themes to be both maintainable and extensible?

## References

Research based on:
- **Lipgloss**: Core styling library (github.com/charmbracelet/lipgloss)
- **Huh library themes**: Component-based theme structure with focused/blurred states
- **Soft-Serve styles**: Flat structure with nested component styles
- **Catppuccin Go**: Official Catppuccin colors (github.com/catppuccin/go)
- **Overlay library**: Modal rendering (github.com/rmhubbert/bubbletea-overlay)

## Design

### Theme Structure

```go
type Theme struct {
    Name string

    // Core colors (semantic naming)
    Primary     lipgloss.Color
    Secondary   lipgloss.Color
    Accent      lipgloss.Color
    Background  lipgloss.Color
    Foreground  lipgloss.Color
    Muted       lipgloss.Color
    Error       lipgloss.Color
    Success     lipgloss.Color
    Warning     lipgloss.Color

    // Pre-built component styles
    Table       TableStyles
    Header      lipgloss.Style
    Footer      lipgloss.Style
    StatusBar   StatusBarStyles
    Modal       ModalStyles
}
```

### Component-Specific Styles

```go
type TableStyles struct {
    Header        lipgloss.Style
    Cell          lipgloss.Style
    SelectedRow   lipgloss.Style
    Border        lipgloss.Style
}

type StatusBarStyles struct {
    Info    lipgloss.Style
    Success lipgloss.Style
    Warning lipgloss.Style
    Error   lipgloss.Style
}

type ModalStyles struct {
    Background  lipgloss.Style  // Solid modal background
    Border      lipgloss.Style  // Prominent border
    Title       lipgloss.Style  // Modal title
    Content     lipgloss.Style  // Modal content text
}
```

### Adaptive Colors for Light/Dark Mode

Lipgloss supports automatic light/dark adaptation:

```go
// Define colors that adapt to terminal background
normalFg := lipgloss.AdaptiveColor{Light: "235", Dark: "252"}
accent   := lipgloss.AdaptiveColor{Light: "#5A56E0", Dark: "#7571F9"}

// Apply to styles
style := lipgloss.NewStyle().Foreground(normalFg)
```

**Color Formats Supported**:
- ANSI 256 colors: `"240"`, `"57"`, `"229"`
- Hex colors: `"#5A56E0"`, `"#FF4672"`
- Adaptive colors: `AdaptiveColor{Light: "...", Dark: "..."}`

### Factory Functions for Themes

```go
// ThemeCharm returns the default Charm theme
func ThemeCharm() *Theme {
    t := &Theme{Name: "charm"}

    // Define colors
    t.Primary = lipgloss.Color("63")
    t.Success = lipgloss.Color("42")
    t.Error = lipgloss.Color("196")

    // Build component styles from colors
    t.Table.Header = lipgloss.NewStyle().
        BorderStyle(lipgloss.NormalBorder()).
        BorderForeground(t.Primary).
        Foreground(t.Primary).
        Bold(true)

    return t
}

// ThemeDracula returns Dracula-inspired theme
func ThemeDracula() *Theme {
    t := &Theme{Name: "dracula"}
    t.Primary = lipgloss.Color("#bd93f9")
    t.Success = lipgloss.Color("#50fa7b")
    t.Error = lipgloss.Color("#ff5555")
    // ... more colors
    return t
}

// ThemeCatppuccin returns Catppuccin theme
func ThemeCatppuccin() *Theme {
    t := &Theme{Name: "catppuccin"}
    // Use github.com/catppuccin/go for official colors
    return t
}
```

### Application in Models

```go
type model struct {
    theme *Theme
    table table.Model
}

func initialModel(themeName string) model {
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

    // Apply to components
    t := table.New()
    s := table.Styles{
        Header:   theme.Table.Header,
        Selected: theme.Table.SelectedRow,
    }
    t.SetStyles(s)

    return model{theme: theme, table: t}
}
```

### Factory Pattern for Component Styles

Theme provides factory methods to generate component-specific styles:

```go
// In internal/ui/theme.go
func (t *Theme) ToTableStyles() table.Styles {
    return table.Styles{
        Header:   t.Table.Header,
        Cell:     t.Table.Cell,
        Selected: t.Table.SelectedRow,
    }
}

// Usage in screens
s := m.theme.ToTableStyles()
m.table.SetStyles(s)
```

### Modal Styling (No Dimming)

Terminal limitations make true background dimming impractical:
- No transparency support (colors are always solid)
- ANSI `Faint` attribute poorly supported
- Re-rendering background with modified colors is expensive

**Recommended approach**: Use solid modal backgrounds with strong borders.

```go
t.Modal.Background = lipgloss.NewStyle().
    Background(lipgloss.Color("235")).  // Dark solid background
    Padding(1, 2).
    Border(lipgloss.RoundedBorder()).
    BorderForeground(t.Primary)
```

This follows TUI conventions (gum, huh, lazygit, k9s) which use clear visual separation rather than dimming.

### Runtime Theme Switching

```go
case "t":
    // Cycle through themes
    themes := []string{"charm", "dracula", "catppuccin"}
    currentIndex := indexOf(themes, m.theme.Name)
    nextIndex := (currentIndex + 1) % len(themes)

    m.theme = loadTheme(themes[nextIndex])
    m.applyThemeToComponents()
    return m, nil
```

### Configuration Persistence

Store theme preference in config file:

```yaml
# ~/.config/timoneiro/config.yaml
theme: "charm"
```

## Decision

Implement a component-based theming system with the following characteristics:

1. **Semantic Color Naming**: Use names like `Primary`, `Error`, `Success` instead of color names
2. **Factory Functions**: Create theme constructors (`ThemeCharm()`, `ThemeDracula()`) that return fully-configured themes
3. **Component Styles**: Provide pre-built styles for tables, modals, status bar, etc.
4. **Adaptive Colors**: Use `AdaptiveColor` where appropriate for light/dark terminal adaptation
5. **Solid Modal Backgrounds**: Use strong visual separation (borders + solid backgrounds) instead of attempting dimming
6. **Runtime Switching**: Support changing themes without restart
7. **Initial Themes**: Charm (default), Dracula, Catppuccin

Factory methods on Theme (like `ToTableStyles()`) convert theme colors to component-specific styles.

## Consequences

### Positive

1. **User Customization**: Users can choose their preferred color scheme
2. **Accessibility**: Light/dark adaptation improves readability
3. **Maintainability**: Centralized theme definitions make changes easy
4. **Consistency**: All components use colors from same theme
5. **Extensibility**: Adding new themes requires only implementing factory function
6. **Performance**: Theme switching is instant (just style objects)

### Negative

1. **Limited Color Space**: Terminal 256-color palette constrains design choices
2. **No True Transparency**: Cannot dim backgrounds like GUI applications
3. **Terminal Variance**: Color rendering varies across terminal emulators
4. **Theme Testing Overhead**: Must test each theme in light/dark modes

### Mitigations

- Focus on high-contrast, accessible color combinations
- Test themes in popular terminal emulators (iTerm2, Alacritty, Windows Terminal)
- Provide screenshots of themes in documentation
- Use semantic naming to clarify intent (e.g., "Error" not "Red")

## Implementation Status

### ✅ Implemented
- Theme struct with semantic color definitions
- Three themes: Charm (default), Dracula, Catppuccin
- Factory functions for theme creation
- Factory methods for component styles (ToTableStyles)
- Theme passed to screens at initialization
- Modal styling with solid backgrounds and borders

### 🚧 To Do
- Runtime theme switching (keybinding + command palette integration)
- Configuration file support (~/.config/timoneiro/config.yaml)
- Additional themes (Base16, Gruvbox, Nord)
- Light mode variants for existing themes
- Theme preview command

## Best Practices

1. **Use AdaptiveColor**: Always define Light and Dark variants for automatic terminal adaptation
2. **Provide Defaults**: Start with a base theme and allow overrides
3. **Test Both Modes**: Test themes in both light and dark terminal backgrounds
4. **Centralize Colors**: Define color palette first, then build styles from palette
5. **Semantic Naming**: Use semantic names for colors and styles
6. **Factory Pattern**: Use factory functions to generate themes, factory methods for component styles
7. **No Dimming**: Accept TUI limitations; use solid backgrounds + borders for modals
