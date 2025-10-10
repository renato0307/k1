package ui

import (
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/lipgloss"
)

// Theme defines the color scheme and styles for the TUI
type Theme struct {
	Name string

	// Core colors
	Primary    lipgloss.AdaptiveColor
	Secondary  lipgloss.AdaptiveColor
	Accent     lipgloss.AdaptiveColor
	Foreground lipgloss.AdaptiveColor
	Muted      lipgloss.AdaptiveColor
	Error      lipgloss.AdaptiveColor
	Success    lipgloss.AdaptiveColor
	Warning    lipgloss.AdaptiveColor

	// UI element colors
	Border     lipgloss.AdaptiveColor // Separator lines, borders
	Dimmed     lipgloss.AdaptiveColor // Very subtle text (shortcuts)
	Subtle     lipgloss.AdaptiveColor // Subtle UI elements
	Background lipgloss.AdaptiveColor // Background for overlays

	// Component styles
	Table     TableStyles
	AppTitle  lipgloss.Style // App title with background
	Header    lipgloss.Style
	StatusBar lipgloss.Style
}

// TableStyles defines styles for table components
type TableStyles struct {
	Header        lipgloss.Style
	Cell          lipgloss.Style
	SelectedRow   lipgloss.Style
	StatusRunning lipgloss.Style
	StatusError   lipgloss.Style
	StatusWarning lipgloss.Style
}

// ToTableStyles converts Theme.Table to bubbles table.Styles
func (t *Theme) ToTableStyles() table.Styles {
	return table.Styles{
		Header:   t.Table.Header,
		Cell:     t.Table.Cell,
		Selected: t.Table.SelectedRow,
	}
}

// ThemeCharm returns the default Charm theme
func ThemeCharm() *Theme {
	t := &Theme{Name: "charm"}

	// Define adaptive colors
	t.Primary = lipgloss.AdaptiveColor{Light: "#5A56E0", Dark: "#7571F9"}
	t.Secondary = lipgloss.AdaptiveColor{Light: "#02BA84", Dark: "#02BF87"}
	t.Accent = lipgloss.AdaptiveColor{Light: "#F780E2", Dark: "#F780E2"}
	t.Foreground = lipgloss.AdaptiveColor{Light: "235", Dark: "252"}
	t.Muted = lipgloss.AdaptiveColor{Light: "243", Dark: "243"}
	t.Error = lipgloss.AdaptiveColor{Light: "#FF4672", Dark: "#ED567A"}
	t.Success = lipgloss.AdaptiveColor{Light: "#02BA84", Dark: "#02BF87"}
	t.Warning = lipgloss.AdaptiveColor{Light: "#FFAA00", Dark: "#FFAA00"}

	// UI element colors
	t.Border = lipgloss.AdaptiveColor{Light: "240", Dark: "240"}
	t.Dimmed = lipgloss.AdaptiveColor{Light: "243", Dark: "243"}
	t.Subtle = lipgloss.AdaptiveColor{Light: "241", Dark: "241"}
	t.Background = lipgloss.AdaptiveColor{Light: "254", Dark: "235"}

	// Table styles
	t.Table.Header = lipgloss.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(t.Border).
		BorderBottom(true).
		Bold(false).
		PaddingLeft(1).
		PaddingRight(1)

	t.Table.Cell = lipgloss.NewStyle().
		PaddingLeft(1).
		PaddingRight(1)

	t.Table.SelectedRow = lipgloss.NewStyle().
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(false)

	t.Table.StatusRunning = lipgloss.NewStyle().Foreground(t.Success)
	t.Table.StatusError = lipgloss.NewStyle().Foreground(t.Error)
	t.Table.StatusWarning = lipgloss.NewStyle().Foreground(t.Warning)

	// AppTitle style with background (using darker gray for better contrast)
	t.AppTitle = lipgloss.NewStyle().
		Foreground(t.Primary).
		Background(lipgloss.Color("235")).
		Bold(true)

	// Header style
	t.Header = lipgloss.NewStyle().
		Foreground(t.Primary).
		Bold(true)

	// StatusBar style
	t.StatusBar = lipgloss.NewStyle().
		Foreground(t.Muted)

	return t
}

// ThemeDracula returns a Dracula-inspired theme
func ThemeDracula() *Theme {
	t := &Theme{Name: "dracula"}

	// Dracula color palette
	t.Primary = lipgloss.AdaptiveColor{Light: "#bd93f9", Dark: "#bd93f9"}
	t.Secondary = lipgloss.AdaptiveColor{Light: "#8be9fd", Dark: "#8be9fd"}
	t.Accent = lipgloss.AdaptiveColor{Light: "#ff79c6", Dark: "#ff79c6"}
	t.Foreground = lipgloss.AdaptiveColor{Light: "#282a36", Dark: "#f8f8f2"}
	t.Muted = lipgloss.AdaptiveColor{Light: "#6272a4", Dark: "#6272a4"}
	t.Error = lipgloss.AdaptiveColor{Light: "#ff5555", Dark: "#ff5555"}
	t.Success = lipgloss.AdaptiveColor{Light: "#50fa7b", Dark: "#50fa7b"}
	t.Warning = lipgloss.AdaptiveColor{Light: "#f1fa8c", Dark: "#f1fa8c"}

	// UI element colors
	t.Border = lipgloss.AdaptiveColor{Light: "61", Dark: "61"}
	t.Dimmed = lipgloss.AdaptiveColor{Light: "#6272a4", Dark: "#6272a4"}
	t.Subtle = lipgloss.AdaptiveColor{Light: "#44475a", Dark: "#44475a"}
	t.Background = lipgloss.AdaptiveColor{Light: "#f8f8f2", Dark: "#282a36"}

	// Table styles
	t.Table.Header = lipgloss.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(t.Border).
		BorderBottom(true).
		Foreground(t.Primary).
		Bold(true).
		PaddingLeft(1).
		PaddingRight(1)

	t.Table.Cell = lipgloss.NewStyle().
		PaddingLeft(1).
		PaddingRight(1)

	t.Table.SelectedRow = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#282a36")).
		Background(lipgloss.Color("#bd93f9")).
		Bold(false)

	t.Table.StatusRunning = lipgloss.NewStyle().Foreground(t.Success)
	t.Table.StatusError = lipgloss.NewStyle().Foreground(t.Error)
	t.Table.StatusWarning = lipgloss.NewStyle().Foreground(t.Warning)

	// AppTitle style with background (using subtle dark color)
	t.AppTitle = lipgloss.NewStyle().
		Foreground(t.Primary).
		Background(lipgloss.Color("#44475a")).
		Bold(true)

	// Header style
	t.Header = lipgloss.NewStyle().
		Foreground(t.Primary).
		Bold(true)

	// StatusBar style
	t.StatusBar = lipgloss.NewStyle().
		Foreground(t.Muted)

	return t
}

// ThemeCatppuccin returns a Catppuccin-inspired theme (Mocha variant)
// Softer, more pastel colors than Dracula
func ThemeCatppuccin() *Theme {
	t := &Theme{Name: "catppuccin"}

	// Catppuccin Mocha colors - softer pastel palette
	t.Primary = lipgloss.AdaptiveColor{Light: "#8839ef", Dark: "#cba6f7"}   // Mauve (softer than Dracula purple)
	t.Secondary = lipgloss.AdaptiveColor{Light: "#179299", Dark: "#89dceb"} // Sky
	t.Accent = lipgloss.AdaptiveColor{Light: "#ea76cb", Dark: "#f5c2e7"}    // Pink
	t.Foreground = lipgloss.AdaptiveColor{Light: "#4c4f69", Dark: "#cdd6f4"}
	t.Muted = lipgloss.AdaptiveColor{Light: "#9ca0b0", Dark: "#7f849c"} // Overlay1 (more muted)
	t.Error = lipgloss.AdaptiveColor{Light: "#d20f39", Dark: "#f38ba8"}
	t.Success = lipgloss.AdaptiveColor{Light: "#40a02b", Dark: "#a6e3a1"}
	t.Warning = lipgloss.AdaptiveColor{Light: "#df8e1d", Dark: "#f9e2af"}

	// UI element colors - softer borders
	t.Border = lipgloss.AdaptiveColor{Light: "#9ca0b0", Dark: "#45475a"} // Surface1 (softer)
	t.Dimmed = lipgloss.AdaptiveColor{Light: "#9ca0b0", Dark: "#7f849c"}
	t.Subtle = lipgloss.AdaptiveColor{Light: "#7c7f93", Dark: "#585b70"}
	t.Background = lipgloss.AdaptiveColor{Light: "#eff1f5", Dark: "#1e1e2e"}

	// Table styles - softer, more pastel feel
	t.Table.Header = lipgloss.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(t.Border).
		BorderBottom(true).
		Foreground(t.Primary).
		Bold(true).
		PaddingLeft(1).
		PaddingRight(1)

	t.Table.Cell = lipgloss.NewStyle().
		PaddingLeft(1).
		PaddingRight(1)

	t.Table.SelectedRow = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#1e1e2e")).
		Background(lipgloss.Color("#cba6f7")). // Softer mauve
		Bold(false)

	t.Table.StatusRunning = lipgloss.NewStyle().Foreground(t.Success)
	t.Table.StatusError = lipgloss.NewStyle().Foreground(t.Error)
	t.Table.StatusWarning = lipgloss.NewStyle().Foreground(t.Warning)

	// AppTitle style with background (using subtle dark color)
	t.AppTitle = lipgloss.NewStyle().
		Foreground(t.Primary).
		Background(lipgloss.Color("#313244")).
		Bold(true)

	// Header style
	t.Header = lipgloss.NewStyle().
		Foreground(t.Primary).
		Bold(true)

	// StatusBar style
	t.StatusBar = lipgloss.NewStyle().
		Foreground(t.Muted)

	return t
}

// ThemeNord returns a Nord-inspired theme
// Cool, muted blues and grays with Scandinavian aesthetics
func ThemeNord() *Theme {
	t := &Theme{Name: "nord"}

	// Nord color palette - cool blues and grays
	t.Primary = lipgloss.AdaptiveColor{Light: "#5e81ac", Dark: "#88c0d0"}   // Frost blue
	t.Secondary = lipgloss.AdaptiveColor{Light: "#81a1c1", Dark: "#81a1c1"} // Frost lighter blue
	t.Accent = lipgloss.AdaptiveColor{Light: "#b48ead", Dark: "#b48ead"}    // Aurora purple
	t.Foreground = lipgloss.AdaptiveColor{Light: "#2e3440", Dark: "#eceff4"}
	t.Muted = lipgloss.AdaptiveColor{Light: "#4c566a", Dark: "#4c566a"}
	t.Error = lipgloss.AdaptiveColor{Light: "#bf616a", Dark: "#bf616a"}
	t.Success = lipgloss.AdaptiveColor{Light: "#a3be8c", Dark: "#a3be8c"}
	t.Warning = lipgloss.AdaptiveColor{Light: "#ebcb8b", Dark: "#ebcb8b"}

	// UI element colors
	t.Border = lipgloss.AdaptiveColor{Light: "#d8dee9", Dark: "#3b4252"}
	t.Dimmed = lipgloss.AdaptiveColor{Light: "#4c566a", Dark: "#4c566a"}
	t.Subtle = lipgloss.AdaptiveColor{Light: "#434c5e", Dark: "#434c5e"}
	t.Background = lipgloss.AdaptiveColor{Light: "#eceff4", Dark: "#2e3440"}

	// Table styles
	t.Table.Header = lipgloss.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(t.Border).
		BorderBottom(true).
		Foreground(t.Primary).
		Bold(true).
		PaddingLeft(1).
		PaddingRight(1)

	t.Table.Cell = lipgloss.NewStyle().
		PaddingLeft(1).
		PaddingRight(1)

	t.Table.SelectedRow = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#2e3440")).
		Background(lipgloss.Color("#88c0d0")).
		Bold(false)

	t.Table.StatusRunning = lipgloss.NewStyle().Foreground(t.Success)
	t.Table.StatusError = lipgloss.NewStyle().Foreground(t.Error)
	t.Table.StatusWarning = lipgloss.NewStyle().Foreground(t.Warning)

	// AppTitle style with background (using subtle dark color)
	t.AppTitle = lipgloss.NewStyle().
		Foreground(t.Primary).
		Background(lipgloss.Color("#3b4252")).
		Bold(true)

	// Header style
	t.Header = lipgloss.NewStyle().
		Foreground(t.Primary).
		Bold(true)

	// StatusBar style
	t.StatusBar = lipgloss.NewStyle().
		Foreground(t.Muted)

	return t
}

// ThemeGruvbox returns a Gruvbox-inspired theme
// Warm, retro colors with brown/orange/yellow palette
func ThemeGruvbox() *Theme {
	t := &Theme{Name: "gruvbox"}

	// Gruvbox color palette - warm retro colors
	t.Primary = lipgloss.AdaptiveColor{Light: "#af3a03", Dark: "#fe8019"}   // Orange
	t.Secondary = lipgloss.AdaptiveColor{Light: "#79740e", Dark: "#b8bb26"} // Green
	t.Accent = lipgloss.AdaptiveColor{Light: "#b16286", Dark: "#d3869b"}    // Purple
	t.Foreground = lipgloss.AdaptiveColor{Light: "#3c3836", Dark: "#ebdbb2"}
	t.Muted = lipgloss.AdaptiveColor{Light: "#7c6f64", Dark: "#928374"}
	t.Error = lipgloss.AdaptiveColor{Light: "#9d0006", Dark: "#fb4934"}
	t.Success = lipgloss.AdaptiveColor{Light: "#79740e", Dark: "#b8bb26"}
	t.Warning = lipgloss.AdaptiveColor{Light: "#b57614", Dark: "#fabd2f"}

	// UI element colors
	t.Border = lipgloss.AdaptiveColor{Light: "#d5c4a1", Dark: "#504945"}
	t.Dimmed = lipgloss.AdaptiveColor{Light: "#7c6f64", Dark: "#928374"}
	t.Subtle = lipgloss.AdaptiveColor{Light: "#665c54", Dark: "#665c54"}
	t.Background = lipgloss.AdaptiveColor{Light: "#fbf1c7", Dark: "#282828"}

	// Table styles
	t.Table.Header = lipgloss.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(t.Border).
		BorderBottom(true).
		Foreground(t.Primary).
		Bold(true).
		PaddingLeft(1).
		PaddingRight(1)

	t.Table.Cell = lipgloss.NewStyle().
		PaddingLeft(1).
		PaddingRight(1)

	t.Table.SelectedRow = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#282828")).
		Background(lipgloss.Color("#fe8019")).
		Bold(false)

	t.Table.StatusRunning = lipgloss.NewStyle().Foreground(t.Success)
	t.Table.StatusError = lipgloss.NewStyle().Foreground(t.Error)
	t.Table.StatusWarning = lipgloss.NewStyle().Foreground(t.Warning)

	// AppTitle style with background (using subtle dark color)
	t.AppTitle = lipgloss.NewStyle().
		Foreground(t.Primary).
		Background(lipgloss.Color("#3c3836")).
		Bold(true)

	// Header style
	t.Header = lipgloss.NewStyle().
		Foreground(t.Primary).
		Bold(true)

	// StatusBar style
	t.StatusBar = lipgloss.NewStyle().
		Foreground(t.Muted)

	return t
}

// ThemeTokyoNight returns a Tokyo Night-inspired theme
// Modern deep blue theme with bright accents
func ThemeTokyoNight() *Theme {
	t := &Theme{Name: "tokyo-night"}

	// Tokyo Night color palette - deep blue background
	t.Primary = lipgloss.AdaptiveColor{Light: "#7aa2f7", Dark: "#7aa2f7"}   // Blue
	t.Secondary = lipgloss.AdaptiveColor{Light: "#2ac3de", Dark: "#2ac3de"} // Cyan
	t.Accent = lipgloss.AdaptiveColor{Light: "#bb9af7", Dark: "#bb9af7"}    // Purple
	t.Foreground = lipgloss.AdaptiveColor{Light: "#1a1b26", Dark: "#c0caf5"}
	t.Muted = lipgloss.AdaptiveColor{Light: "#565f89", Dark: "#565f89"}
	t.Error = lipgloss.AdaptiveColor{Light: "#f7768e", Dark: "#f7768e"}
	t.Success = lipgloss.AdaptiveColor{Light: "#9ece6a", Dark: "#9ece6a"}
	t.Warning = lipgloss.AdaptiveColor{Light: "#e0af68", Dark: "#e0af68"}

	// UI element colors
	t.Border = lipgloss.AdaptiveColor{Light: "#a9b1d6", Dark: "#292e42"}
	t.Dimmed = lipgloss.AdaptiveColor{Light: "#565f89", Dark: "#565f89"}
	t.Subtle = lipgloss.AdaptiveColor{Light: "#414868", Dark: "#414868"}
	t.Background = lipgloss.AdaptiveColor{Light: "#d5d6db", Dark: "#1a1b26"}

	// Table styles
	t.Table.Header = lipgloss.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(t.Border).
		BorderBottom(true).
		Foreground(t.Primary).
		Bold(true).
		PaddingLeft(1).
		PaddingRight(1)

	t.Table.Cell = lipgloss.NewStyle().
		PaddingLeft(1).
		PaddingRight(1)

	t.Table.SelectedRow = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#1a1b26")).
		Background(lipgloss.Color("#7aa2f7")).
		Bold(false)

	t.Table.StatusRunning = lipgloss.NewStyle().Foreground(t.Success)
	t.Table.StatusError = lipgloss.NewStyle().Foreground(t.Error)
	t.Table.StatusWarning = lipgloss.NewStyle().Foreground(t.Warning)

	// AppTitle style with background (using subtle dark color)
	t.AppTitle = lipgloss.NewStyle().
		Foreground(t.Primary).
		Background(lipgloss.Color("#24283b")).
		Bold(true)

	// Header style
	t.Header = lipgloss.NewStyle().
		Foreground(t.Primary).
		Bold(true)

	// StatusBar style
	t.StatusBar = lipgloss.NewStyle().
		Foreground(t.Muted)

	return t
}

// ThemeSolarized returns a Solarized Dark theme
// Classic theme with distinctive blue/cyan/orange palette
func ThemeSolarized() *Theme {
	t := &Theme{Name: "solarized"}

	// Solarized Dark color palette
	t.Primary = lipgloss.AdaptiveColor{Light: "#268bd2", Dark: "#268bd2"}   // Blue
	t.Secondary = lipgloss.AdaptiveColor{Light: "#2aa198", Dark: "#2aa198"} // Cyan
	t.Accent = lipgloss.AdaptiveColor{Light: "#6c71c4", Dark: "#6c71c4"}    // Violet
	t.Foreground = lipgloss.AdaptiveColor{Light: "#002b36", Dark: "#839496"}
	t.Muted = lipgloss.AdaptiveColor{Light: "#586e75", Dark: "#586e75"}
	t.Error = lipgloss.AdaptiveColor{Light: "#dc322f", Dark: "#dc322f"}
	t.Success = lipgloss.AdaptiveColor{Light: "#859900", Dark: "#859900"}
	t.Warning = lipgloss.AdaptiveColor{Light: "#cb4b16", Dark: "#cb4b16"}

	// UI element colors
	t.Border = lipgloss.AdaptiveColor{Light: "#93a1a1", Dark: "#073642"}
	t.Dimmed = lipgloss.AdaptiveColor{Light: "#586e75", Dark: "#586e75"}
	t.Subtle = lipgloss.AdaptiveColor{Light: "#657b83", Dark: "#657b83"}
	t.Background = lipgloss.AdaptiveColor{Light: "#fdf6e3", Dark: "#002b36"}

	// Table styles
	t.Table.Header = lipgloss.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(t.Border).
		BorderBottom(true).
		Foreground(t.Primary).
		Bold(true).
		PaddingLeft(1).
		PaddingRight(1)

	t.Table.Cell = lipgloss.NewStyle().
		PaddingLeft(1).
		PaddingRight(1)

	t.Table.SelectedRow = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#002b36")).
		Background(lipgloss.Color("#268bd2")).
		Bold(false)

	t.Table.StatusRunning = lipgloss.NewStyle().Foreground(t.Success)
	t.Table.StatusError = lipgloss.NewStyle().Foreground(t.Error)
	t.Table.StatusWarning = lipgloss.NewStyle().Foreground(t.Warning)

	// AppTitle style with background (using subtle dark color)
	t.AppTitle = lipgloss.NewStyle().
		Foreground(t.Primary).
		Background(lipgloss.Color("#073642")).
		Bold(true)

	// Header style
	t.Header = lipgloss.NewStyle().
		Foreground(t.Primary).
		Bold(true)

	// StatusBar style
	t.StatusBar = lipgloss.NewStyle().
		Foreground(t.Muted)

	return t
}

// ThemeMonokai returns a Monokai-inspired theme
// Vibrant editor theme with black background and bright colors
func ThemeMonokai() *Theme {
	t := &Theme{Name: "monokai"}

	// Monokai color palette - vibrant colors
	t.Primary = lipgloss.AdaptiveColor{Light: "#66d9ef", Dark: "#66d9ef"}   // Cyan
	t.Secondary = lipgloss.AdaptiveColor{Light: "#a6e22e", Dark: "#a6e22e"} // Green
	t.Accent = lipgloss.AdaptiveColor{Light: "#ae81ff", Dark: "#ae81ff"}    // Purple
	t.Foreground = lipgloss.AdaptiveColor{Light: "#272822", Dark: "#f8f8f2"}
	t.Muted = lipgloss.AdaptiveColor{Light: "#75715e", Dark: "#75715e"}
	t.Error = lipgloss.AdaptiveColor{Light: "#f92672", Dark: "#f92672"}
	t.Success = lipgloss.AdaptiveColor{Light: "#a6e22e", Dark: "#a6e22e"}
	t.Warning = lipgloss.AdaptiveColor{Light: "#e6db74", Dark: "#e6db74"}

	// UI element colors
	t.Border = lipgloss.AdaptiveColor{Light: "#464741", Dark: "#464741"}
	t.Dimmed = lipgloss.AdaptiveColor{Light: "#75715e", Dark: "#75715e"}
	t.Subtle = lipgloss.AdaptiveColor{Light: "#49483e", Dark: "#49483e"}
	t.Background = lipgloss.AdaptiveColor{Light: "#f8f8f2", Dark: "#272822"}

	// Table styles
	t.Table.Header = lipgloss.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(t.Border).
		BorderBottom(true).
		Foreground(t.Primary).
		Bold(true).
		PaddingLeft(1).
		PaddingRight(1)

	t.Table.Cell = lipgloss.NewStyle().
		PaddingLeft(1).
		PaddingRight(1)

	t.Table.SelectedRow = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#272822")).
		Background(lipgloss.Color("#66d9ef")).
		Bold(false)

	t.Table.StatusRunning = lipgloss.NewStyle().Foreground(t.Success)
	t.Table.StatusError = lipgloss.NewStyle().Foreground(t.Error)
	t.Table.StatusWarning = lipgloss.NewStyle().Foreground(t.Warning)

	// AppTitle style with background (using subtle dark color)
	t.AppTitle = lipgloss.NewStyle().
		Foreground(t.Primary).
		Background(lipgloss.Color("#3e3d32")).
		Bold(true)

	// Header style
	t.Header = lipgloss.NewStyle().
		Foreground(t.Primary).
		Bold(true)

	// StatusBar style
	t.StatusBar = lipgloss.NewStyle().
		Foreground(t.Muted)

	return t
}

// GetTheme returns a theme by name, defaulting to Charm
func GetTheme(name string) *Theme {
	switch name {
	case "dracula":
		return ThemeDracula()
	case "catppuccin":
		return ThemeCatppuccin()
	case "nord":
		return ThemeNord()
	case "gruvbox":
		return ThemeGruvbox()
	case "tokyo-night":
		return ThemeTokyoNight()
	case "solarized":
		return ThemeSolarized()
	case "monokai":
		return ThemeMonokai()
	default:
		return ThemeCharm()
	}
}

// AvailableThemes returns a list of available theme names
func AvailableThemes() []string {
	return []string{"charm", "dracula", "catppuccin", "nord", "gruvbox", "tokyo-night", "solarized", "monokai"}
}
