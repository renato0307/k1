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
func ThemeCatppuccin() *Theme {
	t := &Theme{Name: "catppuccin"}

	// Catppuccin Mocha colors (simplified)
	t.Primary = lipgloss.AdaptiveColor{Light: "#8839ef", Dark: "#cba6f7"}
	t.Secondary = lipgloss.AdaptiveColor{Light: "#179299", Dark: "#89dceb"}
	t.Accent = lipgloss.AdaptiveColor{Light: "#ea76cb", Dark: "#f5c2e7"}
	t.Foreground = lipgloss.AdaptiveColor{Light: "#4c4f69", Dark: "#cdd6f4"}
	t.Muted = lipgloss.AdaptiveColor{Light: "#9ca0b0", Dark: "#6c7086"}
	t.Error = lipgloss.AdaptiveColor{Light: "#d20f39", Dark: "#f38ba8"}
	t.Success = lipgloss.AdaptiveColor{Light: "#40a02b", Dark: "#a6e3a1"}
	t.Warning = lipgloss.AdaptiveColor{Light: "#df8e1d", Dark: "#f9e2af"}

	// UI element colors
	t.Border = lipgloss.AdaptiveColor{Light: "#9ca0b0", Dark: "#6c7086"}
	t.Dimmed = lipgloss.AdaptiveColor{Light: "#9ca0b0", Dark: "#6c7086"}
	t.Subtle = lipgloss.AdaptiveColor{Light: "#7c7f93", Dark: "#585b70"}
	t.Background = lipgloss.AdaptiveColor{Light: "#eff1f5", Dark: "#1e1e2e"}

	// Table styles
	t.Table.Header = lipgloss.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(t.Muted).
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
		Background(t.Primary).
		Bold(false)

	t.Table.StatusRunning = lipgloss.NewStyle().Foreground(t.Success)
	t.Table.StatusError = lipgloss.NewStyle().Foreground(t.Error)
	t.Table.StatusWarning = lipgloss.NewStyle().Foreground(t.Warning)

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
	default:
		return ThemeCharm()
	}
}

// AvailableThemes returns a list of available theme names
func AvailableThemes() []string {
	return []string{"charm", "dracula", "catppuccin"}
}
