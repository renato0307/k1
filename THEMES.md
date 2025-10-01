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

## References

- Huh library themes: `.tmp/huh/theme.go`
- Soft-serve styles: `.tmp/soft-serve/pkg/ui/styles/styles.go`
- Lipgloss documentation: https://github.com/charmbracelet/lipgloss
- Bubble Tea examples: `.tmp/bubbletea/examples/`
