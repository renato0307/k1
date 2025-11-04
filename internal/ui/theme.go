package ui

import (
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/lipgloss"
)

// Theme defines the color scheme and styles for the TUI
type Theme struct {
	Name    string
	Variant string // "dark" or "light"

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

	// Visual depth (NEW - for layered themes like Catppuccin)
	BackgroundBase    lipgloss.AdaptiveColor // Base layer (deepest)
	BackgroundSurface lipgloss.AdaptiveColor // Raised surfaces

	// Interactive states (NEW - for hover/selection)
	Hover     lipgloss.AdaptiveColor // Hover state
	Selection lipgloss.AdaptiveColor // Selected items (distinct from table row)

	// Frame elements (NEW - for distinctive borders)
	FrameBorder      lipgloss.AdaptiveColor // Frame borders
	FrameFocusBorder lipgloss.AdaptiveColor // Focused frame

	// Enhanced semantics (NEW)
	Info lipgloss.AdaptiveColor // Info messages (distinct from Primary)

	// Message colors (Claude Code style)
	MessageSuccess lipgloss.AdaptiveColor // Green circle/text
	MessageError   lipgloss.AdaptiveColor // Red circle/text
	MessageInfo    lipgloss.AdaptiveColor // Blue circle/text
	MessageLoading lipgloss.AdaptiveColor // Orange spinner/text

	// Palette colors (for command palette overlay on terminal background)
	PaletteForeground         lipgloss.AdaptiveColor // Text color for unselected palette items
	PaletteBackground         lipgloss.AdaptiveColor // Background for unselected palette items
	PaletteSelectedForeground lipgloss.AdaptiveColor // Text color for selected palette item (on Subtle bg)
	PaletteShortcut           lipgloss.AdaptiveColor // Shortcut color for palette items

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
	t := &Theme{Name: "charm", Variant: "dark"}

	// Define adaptive colors
	t.Primary = lipgloss.AdaptiveColor{Light: "#5A56E0", Dark: "#7571F9"}
	t.Secondary = lipgloss.AdaptiveColor{Light: "#02BA84", Dark: "#02BF87"}
	t.Accent = lipgloss.AdaptiveColor{Light: "#F780E2", Dark: "#F780E2"}
	t.Foreground = lipgloss.AdaptiveColor{Light: "235", Dark: "255"} // Brightened from 252 to 255 (white)
	t.Muted = lipgloss.AdaptiveColor{Light: "243", Dark: "248"}
	t.Error = lipgloss.AdaptiveColor{Light: "#FF4672", Dark: "#ED567A"}
	t.Success = lipgloss.AdaptiveColor{Light: "#02BA84", Dark: "#02BF87"}
	t.Warning = lipgloss.AdaptiveColor{Light: "#FFAA00", Dark: "#FFAA00"}

	// UI element colors
	t.Border = lipgloss.AdaptiveColor{Light: "240", Dark: "240"}
	t.Dimmed = lipgloss.AdaptiveColor{Light: "243", Dark: "246"}
	t.Subtle = lipgloss.AdaptiveColor{Light: "241", Dark: "246"}
	t.Background = lipgloss.AdaptiveColor{Light: "254", Dark: "235"}

	// Visual depth
	t.BackgroundBase = lipgloss.AdaptiveColor{Light: "255", Dark: "234"}
	t.BackgroundSurface = lipgloss.AdaptiveColor{Light: "254", Dark: "235"}

	// Interactive states
	t.Hover = lipgloss.AdaptiveColor{Light: "#4A46C0", Dark: "#8682FF"}
	t.Selection = lipgloss.AdaptiveColor{Light: "#5A56E0", Dark: "#7571F9"}

	// Frame elements (balanced, not too prominent)
	t.FrameBorder = lipgloss.AdaptiveColor{Light: "243", Dark: "243"}
	t.FrameFocusBorder = t.Primary

	// Enhanced semantics
	t.Info = lipgloss.AdaptiveColor{Light: "#5A56E0", Dark: "#7aa2f7"}

	// Message colors (Claude Code style)
	t.MessageSuccess = t.Success
	t.MessageError = t.Error
	t.MessageInfo = t.Primary
	t.MessageLoading = lipgloss.AdaptiveColor{Light: "#FF8800", Dark: "#FF8800"}

	// Palette colors (overlay on terminal background) - DARK THEME
	t.PaletteForeground = lipgloss.AdaptiveColor{Light: "255", Dark: "255"}         // Bright for dark bg
	t.PaletteBackground = lipgloss.AdaptiveColor{Light: "235", Dark: "235"}         // Dark background
	t.PaletteSelectedForeground = lipgloss.AdaptiveColor{Light: "255", Dark: "255"} // Bright on Subtle
	t.PaletteShortcut = lipgloss.AdaptiveColor{Light: "250", Dark: "250"}           // Light gray

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
// Personality: Purple everywhere, vibrant, high-energy "night club" aesthetic
func ThemeDracula() *Theme {
	t := &Theme{Name: "dracula", Variant: "dark"}

	// Dracula color palette (official colors)
	t.Primary = lipgloss.AdaptiveColor{Light: "#bd93f9", Dark: "#bd93f9"}     // Purple (signature)
	t.Secondary = lipgloss.AdaptiveColor{Light: "#8be9fd", Dark: "#8be9fd"}   // Cyan
	t.Accent = lipgloss.AdaptiveColor{Light: "#ff79c6", Dark: "#ff79c6"}      // Pink
	t.Foreground = lipgloss.AdaptiveColor{Light: "#282a36", Dark: "#f8f8f2"}  // Foreground
	t.Muted = lipgloss.AdaptiveColor{Light: "#6272a4", Dark: "#8899c4"}       // Comment (brightened)
	t.Error = lipgloss.AdaptiveColor{Light: "#ff5555", Dark: "#ff5555"}       // Red
	t.Success = lipgloss.AdaptiveColor{Light: "#50fa7b", Dark: "#50fa7b"}     // Green
	t.Warning = lipgloss.AdaptiveColor{Light: "#f1fa8c", Dark: "#f1fa8c"}     // Yellow

	// UI element colors
	t.Border = lipgloss.AdaptiveColor{Light: "61", Dark: "61"}
	t.Dimmed = lipgloss.AdaptiveColor{Light: "#6272a4", Dark: "#7888b4"}
	t.Subtle = lipgloss.AdaptiveColor{Light: "#44475a", Dark: "#7888b4"}
	t.Background = lipgloss.AdaptiveColor{Light: "#f8f8f2", Dark: "#282a36"}

	// Visual depth (Dracula uses flat appearance)
	t.BackgroundBase = lipgloss.AdaptiveColor{Light: "#f8f8f2", Dark: "#282a36"}
	t.BackgroundSurface = lipgloss.AdaptiveColor{Light: "#eee8d5", Dark: "#44475a"} // Current Line

	// Interactive states (PURPLE EVERYWHERE - signature Dracula style)
	t.Hover = lipgloss.AdaptiveColor{Light: "#a07dcf", Dark: "#cba6f9"}       // Lighter purple
	t.Selection = lipgloss.AdaptiveColor{Light: "#bd93f9", Dark: "#bd93f9"}   // Purple selection

	// Frame elements (Purple for regular, Pink for focus - high energy)
	t.FrameBorder = lipgloss.AdaptiveColor{Light: "#bd93f9", Dark: "#bd93f9"}      // Purple borders
	t.FrameFocusBorder = lipgloss.AdaptiveColor{Light: "#ff79c6", Dark: "#ff79c6"} // Pink when focused

	// Enhanced semantics
	t.Info = lipgloss.AdaptiveColor{Light: "#8be9fd", Dark: "#8be9fd"} // Cyan for info

	// Message colors (Claude Code style)
	t.MessageSuccess = t.Success
	t.MessageError = t.Error
	t.MessageInfo = t.Primary
	t.MessageLoading = lipgloss.AdaptiveColor{Light: "#FF8800", Dark: "#ffb86c"} // Dracula orange

	// Palette colors (overlay on terminal background) - DARK THEME
	t.PaletteForeground = lipgloss.AdaptiveColor{Light: "255", Dark: "255"}         // Bright for dark bg
	t.PaletteBackground = lipgloss.AdaptiveColor{Light: "235", Dark: "235"}         // Dark background
	t.PaletteSelectedForeground = lipgloss.AdaptiveColor{Light: "255", Dark: "255"} // Bright on Subtle
	t.PaletteShortcut = lipgloss.AdaptiveColor{Light: "250", Dark: "250"}           // Light gray

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
// Personality: Soft pastel mauve, layered backgrounds, cozy "coffee shop" aesthetic
func ThemeCatppuccin() *Theme {
	t := &Theme{Name: "catppuccin", Variant: "dark"}

	// Catppuccin Mocha colors - official pastel palette
	t.Primary = lipgloss.AdaptiveColor{Light: "#8839ef", Dark: "#cba6f7"}   // Mauve (signature color)
	t.Secondary = lipgloss.AdaptiveColor{Light: "#179299", Dark: "#89dceb"} // Sky
	t.Accent = lipgloss.AdaptiveColor{Light: "#ea76cb", Dark: "#f5c2e7"}    // Pink
	t.Foreground = lipgloss.AdaptiveColor{Light: "#4c4f69", Dark: "#cdd6f4"} // Text
	t.Muted = lipgloss.AdaptiveColor{Light: "#9ca0b0", Dark: "#9399b2"}     // Overlay2 (better contrast)
	t.Error = lipgloss.AdaptiveColor{Light: "#d20f39", Dark: "#f38ba8"}     // Red
	t.Success = lipgloss.AdaptiveColor{Light: "#40a02b", Dark: "#a6e3a1"}   // Green
	t.Warning = lipgloss.AdaptiveColor{Light: "#df8e1d", Dark: "#f9e2af"}   // Yellow

	// UI element colors - softer borders
	t.Border = lipgloss.AdaptiveColor{Light: "#9ca0b0", Dark: "#585b70"} // Surface2 (better contrast)
	t.Dimmed = lipgloss.AdaptiveColor{Light: "#9ca0b0", Dark: "#9399b2"} // Overlay2
	t.Subtle = lipgloss.AdaptiveColor{Light: "#7c7f93", Dark: "#9399b2"}  // Overlay2
	t.Background = lipgloss.AdaptiveColor{Light: "#eff1f5", Dark: "#1e1e2e"} // Base

	// Visual depth (Catppuccin's layered aesthetic - base/mantle/surface/overlay)
	t.BackgroundBase = lipgloss.AdaptiveColor{Light: "#eff1f5", Dark: "#1e1e2e"}    // Base (deepest)
	t.BackgroundSurface = lipgloss.AdaptiveColor{Light: "#e6e9ef", Dark: "#313244"} // Surface0 (raised)

	// Interactive states (Soft mauve everywhere)
	t.Hover = lipgloss.AdaptiveColor{Light: "#7c3aed", Dark: "#d4bbf4"}       // Lighter mauve
	t.Selection = lipgloss.AdaptiveColor{Light: "#8839ef", Dark: "#cba6f7"}   // Mauve selection

	// Frame elements (Subtle Surface2 for borders, Mauve for focus - cozy feel)
	t.FrameBorder = lipgloss.AdaptiveColor{Light: "#9ca0b0", Dark: "#585b70"}      // Surface2 (subtle)
	t.FrameFocusBorder = lipgloss.AdaptiveColor{Light: "#8839ef", Dark: "#cba6f7"} // Mauve when focused

	// Enhanced semantics
	t.Info = lipgloss.AdaptiveColor{Light: "#04a5e5", Dark: "#89dceb"} // Sky for info

	// Message colors (Claude Code style)
	t.MessageSuccess = t.Success
	t.MessageError = t.Error
	t.MessageInfo = t.Primary
	t.MessageLoading = lipgloss.AdaptiveColor{Light: "#fe640b", Dark: "#fab387"} // Peach

	// Palette colors (overlay on terminal background) - DARK THEME
	t.PaletteForeground = lipgloss.AdaptiveColor{Light: "255", Dark: "255"}         // Bright for dark bg
	t.PaletteBackground = lipgloss.AdaptiveColor{Light: "235", Dark: "235"}         // Dark background
	t.PaletteSelectedForeground = lipgloss.AdaptiveColor{Light: "255", Dark: "255"} // Bright on Subtle
	t.PaletteShortcut = lipgloss.AdaptiveColor{Light: "250", Dark: "250"}           // Light gray

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
// Personality: Cool arctic blues, calm minimalism, "Scandinavian winter" aesthetic
func ThemeNord() *Theme {
	t := &Theme{Name: "nord", Variant: "dark"}

	// Nord color palette - official frost blues and polar night
	t.Primary = lipgloss.AdaptiveColor{Light: "#5e81ac", Dark: "#88c0d0"}   // Frost blue (nord8)
	t.Secondary = lipgloss.AdaptiveColor{Light: "#81a1c1", Dark: "#81a1c1"} // Frost lighter (nord9)
	t.Accent = lipgloss.AdaptiveColor{Light: "#b48ead", Dark: "#b48ead"}    // Aurora purple (nord15)
	t.Foreground = lipgloss.AdaptiveColor{Light: "#2e3440", Dark: "#eceff4"} // Snow Storm
	t.Muted = lipgloss.AdaptiveColor{Light: "#4c566a", Dark: "#b0bac8"}     // Brightened Polar Night
	t.Error = lipgloss.AdaptiveColor{Light: "#bf616a", Dark: "#bf616a"}     // Aurora red (nord11)
	t.Success = lipgloss.AdaptiveColor{Light: "#a3be8c", Dark: "#a3be8c"}   // Aurora green (nord14)
	t.Warning = lipgloss.AdaptiveColor{Light: "#ebcb8b", Dark: "#ebcb8b"}   // Aurora yellow (nord13)

	// UI element colors
	t.Border = lipgloss.AdaptiveColor{Light: "#d8dee9", Dark: "#4c566a"}
	t.Dimmed = lipgloss.AdaptiveColor{Light: "#4c566a", Dark: "#a0aac0"}
	t.Subtle = lipgloss.AdaptiveColor{Light: "#434c5e", Dark: "#a0aac0"}
	t.Background = lipgloss.AdaptiveColor{Light: "#eceff4", Dark: "#2e3440"} // Snow Storm / Polar Night

	// Visual depth (Nord's minimalist layering)
	t.BackgroundBase = lipgloss.AdaptiveColor{Light: "#eceff4", Dark: "#2e3440"}    // nord6/nord0
	t.BackgroundSurface = lipgloss.AdaptiveColor{Light: "#e5e9f0", Dark: "#3b4252"} // nord5/nord1

	// Interactive states (Cool frost blue accents only)
	t.Hover = lipgloss.AdaptiveColor{Light: "#5e81ac", Dark: "#8fbcbb"}       // Frost teal
	t.Selection = lipgloss.AdaptiveColor{Light: "#5e81ac", Dark: "#88c0d0"}   // Frost blue

	// Frame elements (Barely visible for minimalism, Frost blue when focused)
	t.FrameBorder = lipgloss.AdaptiveColor{Light: "#d8dee9", Dark: "#4c566a"}      // Polar Night 3
	t.FrameFocusBorder = lipgloss.AdaptiveColor{Light: "#5e81ac", Dark: "#88c0d0"} // Frost blue

	// Enhanced semantics
	t.Info = lipgloss.AdaptiveColor{Light: "#5e81ac", Dark: "#88c0d0"} // Frost blue

	// Message colors (Claude Code style)
	t.MessageSuccess = t.Success
	t.MessageError = t.Error
	t.MessageInfo = t.Primary
	t.MessageLoading = lipgloss.AdaptiveColor{Light: "#d08770", Dark: "#d08770"} // Aurora orange

	// Palette colors (overlay on terminal background) - DARK THEME
	t.PaletteForeground = lipgloss.AdaptiveColor{Light: "255", Dark: "255"}         // Bright for dark bg
	t.PaletteBackground = lipgloss.AdaptiveColor{Light: "235", Dark: "235"}         // Dark background
	t.PaletteSelectedForeground = lipgloss.AdaptiveColor{Light: "255", Dark: "255"} // Bright on Subtle
	t.PaletteShortcut = lipgloss.AdaptiveColor{Light: "250", Dark: "250"}           // Light gray

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
// Personality: Warm retro brown/orange, comfortable, "vintage notebook" aesthetic
func ThemeGruvbox() *Theme {
	t := &Theme{Name: "gruvbox", Variant: "dark"}

	// Gruvbox color palette - official warm retro colors
	t.Primary = lipgloss.AdaptiveColor{Light: "#af3a03", Dark: "#fe8019"}   // Orange (bright)
	t.Secondary = lipgloss.AdaptiveColor{Light: "#79740e", Dark: "#b8bb26"} // Green
	t.Accent = lipgloss.AdaptiveColor{Light: "#b16286", Dark: "#d3869b"}    // Purple
	t.Foreground = lipgloss.AdaptiveColor{Light: "#3c3836", Dark: "#ebdbb2"} // fg1
	t.Muted = lipgloss.AdaptiveColor{Light: "#7c6f64", Dark: "#a89984"}     // fg4 (brightened)
	t.Error = lipgloss.AdaptiveColor{Light: "#9d0006", Dark: "#fb4934"}     // Red
	t.Success = lipgloss.AdaptiveColor{Light: "#79740e", Dark: "#b8bb26"}   // Green
	t.Warning = lipgloss.AdaptiveColor{Light: "#b57614", Dark: "#fabd2f"}   // Yellow

	// UI element colors
	t.Border = lipgloss.AdaptiveColor{Light: "#d5c4a1", Dark: "#504945"}    // bg2
	t.Dimmed = lipgloss.AdaptiveColor{Light: "#7c6f64", Dark: "#a89984"}    // fg4
	t.Subtle = lipgloss.AdaptiveColor{Light: "#665c54", Dark: "#a89984"}    // fg4
	t.Background = lipgloss.AdaptiveColor{Light: "#fbf1c7", Dark: "#282828"} // bg

	// Visual depth (Gruvbox's warm layering)
	t.BackgroundBase = lipgloss.AdaptiveColor{Light: "#fbf1c7", Dark: "#282828"}    // bg
	t.BackgroundSurface = lipgloss.AdaptiveColor{Light: "#f2e5bc", Dark: "#3c3836"} // bg1

	// Interactive states (Warm orange everywhere)
	t.Hover = lipgloss.AdaptiveColor{Light: "#d65d0e", Dark: "#ff9f60"}       // Brighter orange
	t.Selection = lipgloss.AdaptiveColor{Light: "#af3a03", Dark: "#fe8019"}   // Orange selection

	// Frame elements (Brown borders, Orange focus - warm retro feel)
	t.FrameBorder = lipgloss.AdaptiveColor{Light: "#bdae93", Dark: "#504945"}      // bg2
	t.FrameFocusBorder = lipgloss.AdaptiveColor{Light: "#af3a03", Dark: "#fe8019"} // Orange when focused

	// Enhanced semantics
	t.Info = lipgloss.AdaptiveColor{Light: "#076678", Dark: "#83a598"} // Blue (aqua)

	// Message colors (Claude Code style)
	t.MessageSuccess = t.Success
	t.MessageError = t.Error
	t.MessageInfo = t.Primary
	t.MessageLoading = lipgloss.AdaptiveColor{Light: "#b57614", Dark: "#fabd2f"} // Yellow

	// Palette colors (overlay on terminal background) - DARK THEME
	t.PaletteForeground = lipgloss.AdaptiveColor{Light: "255", Dark: "255"}         // Bright for dark bg
	t.PaletteBackground = lipgloss.AdaptiveColor{Light: "235", Dark: "235"}         // Dark background
	t.PaletteSelectedForeground = lipgloss.AdaptiveColor{Light: "255", Dark: "255"} // Bright on Subtle
	t.PaletteShortcut = lipgloss.AdaptiveColor{Light: "250", Dark: "250"}           // Light gray

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
// Personality: Deep blue urban, modern sleek, "city at night" aesthetic
func ThemeTokyoNight() *Theme {
	t := &Theme{Name: "tokyo-night", Variant: "dark"}

	// Tokyo Night color palette - official nighttime Tokyo colors
	t.Primary = lipgloss.AdaptiveColor{Light: "#7aa2f7", Dark: "#7aa2f7"}   // Blue
	t.Secondary = lipgloss.AdaptiveColor{Light: "#2ac3de", Dark: "#7dcfff"} // Cyan
	t.Accent = lipgloss.AdaptiveColor{Light: "#bb9af7", Dark: "#bb9af7"}    // Purple
	t.Foreground = lipgloss.AdaptiveColor{Light: "#1a1b26", Dark: "#c0caf5"} // Foreground
	t.Muted = lipgloss.AdaptiveColor{Light: "#565f89", Dark: "#8890a8"}     // Comment (brightened)
	t.Error = lipgloss.AdaptiveColor{Light: "#f7768e", Dark: "#f7768e"}     // Red
	t.Success = lipgloss.AdaptiveColor{Light: "#9ece6a", Dark: "#9ece6a"}   // Green
	t.Warning = lipgloss.AdaptiveColor{Light: "#e0af68", Dark: "#e0af68"}   // Yellow

	// UI element colors
	t.Border = lipgloss.AdaptiveColor{Light: "#a9b1d6", Dark: "#565f89"}    // Storm
	t.Dimmed = lipgloss.AdaptiveColor{Light: "#565f89", Dark: "#787c99"}
	t.Subtle = lipgloss.AdaptiveColor{Light: "#414868", Dark: "#787c99"}
	t.Background = lipgloss.AdaptiveColor{Light: "#d5d6db", Dark: "#1a1b26"} // Night

	// Visual depth (Tokyo Night's layered urban aesthetic)
	t.BackgroundBase = lipgloss.AdaptiveColor{Light: "#d5d6db", Dark: "#1a1b26"}    // Night
	t.BackgroundSurface = lipgloss.AdaptiveColor{Light: "#c8c9ce", Dark: "#24283b"} // Storm

	// Interactive states (Bright blue accents - neon city lights)
	t.Hover = lipgloss.AdaptiveColor{Light: "#5a7bb0", Dark: "#9db4f7"}       // Lighter blue
	t.Selection = lipgloss.AdaptiveColor{Light: "#7aa2f7", Dark: "#7aa2f7"}   // Blue selection

	// Frame elements (Storm blue borders, Bright blue focus - urban sleek)
	t.FrameBorder = lipgloss.AdaptiveColor{Light: "#a9b1d6", Dark: "#565f89"}      // Storm
	t.FrameFocusBorder = lipgloss.AdaptiveColor{Light: "#7aa2f7", Dark: "#7aa2f7"} // Bright blue

	// Enhanced semantics
	t.Info = lipgloss.AdaptiveColor{Light: "#2ac3de", Dark: "#7dcfff"} // Cyan

	// Message colors (Claude Code style)
	t.MessageSuccess = t.Success
	t.MessageError = t.Error
	t.MessageInfo = t.Primary
	t.MessageLoading = lipgloss.AdaptiveColor{Light: "#ff9e64", Dark: "#ff9e64"} // Orange

	// Palette colors (overlay on terminal background) - DARK THEME
	t.PaletteForeground = lipgloss.AdaptiveColor{Light: "255", Dark: "255"}         // Bright for dark bg
	t.PaletteBackground = lipgloss.AdaptiveColor{Light: "235", Dark: "235"}         // Dark background
	t.PaletteSelectedForeground = lipgloss.AdaptiveColor{Light: "255", Dark: "255"} // Bright on Subtle
	t.PaletteShortcut = lipgloss.AdaptiveColor{Light: "250", Dark: "250"}           // Light gray

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
// Personality: Scientific precision, balanced, symmetric, "laboratory" aesthetic
func ThemeSolarized() *Theme {
	t := &Theme{Name: "solarized", Variant: "dark"}

	// Solarized Dark color palette - official scientifically calibrated colors
	t.Primary = lipgloss.AdaptiveColor{Light: "#268bd2", Dark: "#268bd2"}   // Blue
	t.Secondary = lipgloss.AdaptiveColor{Light: "#2aa198", Dark: "#2aa198"} // Cyan
	t.Accent = lipgloss.AdaptiveColor{Light: "#6c71c4", Dark: "#6c71c4"}    // Violet
	t.Foreground = lipgloss.AdaptiveColor{Light: "#002b36", Dark: "#839496"} // base0
	t.Muted = lipgloss.AdaptiveColor{Light: "#586e75", Dark: "#93a1a1"}     // base1 (brightened)
	t.Error = lipgloss.AdaptiveColor{Light: "#dc322f", Dark: "#dc322f"}     // Red
	t.Success = lipgloss.AdaptiveColor{Light: "#859900", Dark: "#859900"}   // Green
	t.Warning = lipgloss.AdaptiveColor{Light: "#cb4b16", Dark: "#cb4b16"}   // Orange

	// UI element colors
	t.Border = lipgloss.AdaptiveColor{Light: "#93a1a1", Dark: "#586e75"}    // base01
	t.Dimmed = lipgloss.AdaptiveColor{Light: "#586e75", Dark: "#839496"}    // base0
	t.Subtle = lipgloss.AdaptiveColor{Light: "#657b83", Dark: "#839496"}    // base0
	t.Background = lipgloss.AdaptiveColor{Light: "#fdf6e3", Dark: "#002b36"} // base3/base03

	// Visual depth (Solarized's precise symmetric layering)
	t.BackgroundBase = lipgloss.AdaptiveColor{Light: "#fdf6e3", Dark: "#002b36"}    // base3/base03
	t.BackgroundSurface = lipgloss.AdaptiveColor{Light: "#eee8d5", Dark: "#073642"} // base2/base02

	// Interactive states (Blue accents - scientific precision)
	t.Hover = lipgloss.AdaptiveColor{Light: "#2075b8", Dark: "#3fa5e8"}       // Lighter blue
	t.Selection = lipgloss.AdaptiveColor{Light: "#268bd2", Dark: "#268bd2"}   // Blue selection

	// Frame elements (Base01 borders, Blue focus - balanced)
	t.FrameBorder = lipgloss.AdaptiveColor{Light: "#93a1a1", Dark: "#586e75"}      // base01
	t.FrameFocusBorder = lipgloss.AdaptiveColor{Light: "#268bd2", Dark: "#268bd2"} // Blue

	// Enhanced semantics
	t.Info = lipgloss.AdaptiveColor{Light: "#2aa198", Dark: "#2aa198"} // Cyan

	// Message colors (Claude Code style)
	t.MessageSuccess = t.Success
	t.MessageError = t.Error
	t.MessageInfo = t.Primary
	t.MessageLoading = lipgloss.AdaptiveColor{Light: "#cb4b16", Dark: "#cb4b16"} // Orange

	// Palette colors (overlay on terminal background) - DARK THEME
	t.PaletteForeground = lipgloss.AdaptiveColor{Light: "255", Dark: "255"}         // Bright for dark bg
	t.PaletteBackground = lipgloss.AdaptiveColor{Light: "235", Dark: "235"}         // Dark background
	t.PaletteSelectedForeground = lipgloss.AdaptiveColor{Light: "255", Dark: "255"} // Bright on Subtle
	t.PaletteShortcut = lipgloss.AdaptiveColor{Light: "250", Dark: "250"}           // Light gray

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
// Personality: Neon on black, bold high-contrast, editor-focused "code terminal" aesthetic
func ThemeMonokai() *Theme {
	t := &Theme{Name: "monokai", Variant: "dark"}

	// Monokai color palette - official vibrant neon colors
	t.Primary = lipgloss.AdaptiveColor{Light: "#66d9ef", Dark: "#66d9ef"}   // Cyan (signature)
	t.Secondary = lipgloss.AdaptiveColor{Light: "#a6e22e", Dark: "#a6e22e"} // Green
	t.Accent = lipgloss.AdaptiveColor{Light: "#ae81ff", Dark: "#ae81ff"}    // Purple
	t.Foreground = lipgloss.AdaptiveColor{Light: "#272822", Dark: "#f8f8f2"} // Foreground
	t.Muted = lipgloss.AdaptiveColor{Light: "#75715e", Dark: "#a0a090"}     // Comment (brightened)
	t.Error = lipgloss.AdaptiveColor{Light: "#f92672", Dark: "#f92672"}     // Pink/Red
	t.Success = lipgloss.AdaptiveColor{Light: "#a6e22e", Dark: "#a6e22e"}   // Green
	t.Warning = lipgloss.AdaptiveColor{Light: "#e6db74", Dark: "#e6db74"}   // Yellow

	// UI element colors
	t.Border = lipgloss.AdaptiveColor{Light: "#464741", Dark: "#464741"}
	t.Dimmed = lipgloss.AdaptiveColor{Light: "#75715e", Dark: "#90907d"}
	t.Subtle = lipgloss.AdaptiveColor{Light: "#49483e", Dark: "#90907d"}
	t.Background = lipgloss.AdaptiveColor{Light: "#f8f8f2", Dark: "#272822"}

	// Visual depth (Monokai's flat black appearance)
	t.BackgroundBase = lipgloss.AdaptiveColor{Light: "#f8f8f2", Dark: "#272822"}    // Background
	t.BackgroundSurface = lipgloss.AdaptiveColor{Light: "#eee8d5", Dark: "#3e3d32"} // Slightly raised

	// Interactive states (Cyan everywhere - neon terminal)
	t.Hover = lipgloss.AdaptiveColor{Light: "#4fb8d9", Dark: "#7de9ff"}       // Brighter cyan
	t.Selection = lipgloss.AdaptiveColor{Light: "#66d9ef", Dark: "#66d9ef"}   // Cyan selection

	// Frame elements (Dark gray borders, Cyan focus - high contrast editor)
	t.FrameBorder = lipgloss.AdaptiveColor{Light: "#75715e", Dark: "#464741"}      // Dark gray
	t.FrameFocusBorder = lipgloss.AdaptiveColor{Light: "#66d9ef", Dark: "#66d9ef"} // Cyan when focused

	// Enhanced semantics
	t.Info = lipgloss.AdaptiveColor{Light: "#ae81ff", Dark: "#ae81ff"} // Purple for info

	// Message colors (Claude Code style)
	t.MessageSuccess = t.Success
	t.MessageError = t.Error
	t.MessageInfo = t.Primary
	t.MessageLoading = lipgloss.AdaptiveColor{Light: "#fd971f", Dark: "#fd971f"} // Orange

	// Palette colors (overlay on terminal background) - DARK THEME
	t.PaletteForeground = lipgloss.AdaptiveColor{Light: "255", Dark: "255"}         // Bright for dark bg
	t.PaletteBackground = lipgloss.AdaptiveColor{Light: "235", Dark: "235"}         // Dark background
	t.PaletteSelectedForeground = lipgloss.AdaptiveColor{Light: "255", Dark: "255"} // Bright on Subtle
	t.PaletteShortcut = lipgloss.AdaptiveColor{Light: "250", Dark: "250"}           // Light gray

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

// ThemeCatppuccinLatte returns a Catppuccin Latte light theme
// Personality: Soft pastel mauve on cream, cozy light theme for daytime use
// NOTE: Light themes use same colors for both Light/Dark terminal modes
func ThemeCatppuccinLatte() *Theme {
	t := &Theme{Name: "catppuccin-latte", Variant: "light"}

	// Catppuccin Latte colors - official light theme palette
	// Using string literal for both Light and Dark to force light theme colors
	t.Primary = lipgloss.AdaptiveColor{Light: "#8839ef", Dark: "#8839ef"}   // Mauve (signature)
	t.Secondary = lipgloss.AdaptiveColor{Light: "#179299", Dark: "#179299"} // Teal
	t.Accent = lipgloss.AdaptiveColor{Light: "#ea76cb", Dark: "#ea76cb"}    // Pink
	t.Foreground = lipgloss.AdaptiveColor{Light: "235", Dark: "235"}        // Dark text (ANSI 235)
	t.Muted = lipgloss.AdaptiveColor{Light: "240", Dark: "240"}             // Medium gray
	t.Error = lipgloss.AdaptiveColor{Light: "#d20f39", Dark: "#d20f39"}     // Red
	t.Success = lipgloss.AdaptiveColor{Light: "#40a02b", Dark: "#40a02b"}   // Green
	t.Warning = lipgloss.AdaptiveColor{Light: "#df8e1d", Dark: "#df8e1d"}   // Yellow

	// UI element colors
	t.Border = lipgloss.AdaptiveColor{Light: "#bcc0cc", Dark: "#bcc0cc"}    // Surface1
	t.Dimmed = lipgloss.AdaptiveColor{Light: "243", Dark: "243"}            // Medium-dark gray
	t.Subtle = lipgloss.AdaptiveColor{Light: "246", Dark: "246"}            // Light gray
	t.Background = lipgloss.AdaptiveColor{Light: "255", Dark: "255"}        // White background (ANSI 255)

	// Visual depth (Latte's layered light aesthetic)
	t.BackgroundBase = lipgloss.AdaptiveColor{Light: "255", Dark: "255"}    // White
	t.BackgroundSurface = lipgloss.AdaptiveColor{Light: "254", Dark: "254"} // Off-white

	// Interactive states (Soft mauve for light theme)
	t.Hover = lipgloss.AdaptiveColor{Light: "#7c3aed", Dark: "#7c3aed"}       // Darker mauve
	t.Selection = lipgloss.AdaptiveColor{Light: "#8839ef", Dark: "#8839ef"}   // Mauve selection

	// Frame elements (Subtle Surface1, Mauve focus)
	t.FrameBorder = lipgloss.AdaptiveColor{Light: "#ccd0da", Dark: "#ccd0da"}      // Surface0
	t.FrameFocusBorder = lipgloss.AdaptiveColor{Light: "#8839ef", Dark: "#8839ef"} // Mauve when focused

	// Enhanced semantics
	t.Info = lipgloss.AdaptiveColor{Light: "#04a5e5", Dark: "#04a5e5"} // Sky

	// Message colors
	t.MessageSuccess = t.Success
	t.MessageError = t.Error
	t.MessageInfo = t.Primary
	t.MessageLoading = lipgloss.AdaptiveColor{Light: "#fe640b", Dark: "#fe640b"} // Peach

	// Palette colors (overlay on terminal background) - LIGHT THEME
	t.PaletteForeground = lipgloss.AdaptiveColor{Light: "235", Dark: "235"}         // Dark for light bg
	t.PaletteBackground = lipgloss.AdaptiveColor{Light: "255", Dark: "255"}         // Light background
	t.PaletteSelectedForeground = lipgloss.AdaptiveColor{Light: "255", Dark: "255"} // Bright on Subtle
	t.PaletteShortcut = lipgloss.AdaptiveColor{Light: "243", Dark: "243"}           // Medium gray

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
		Foreground(lipgloss.Color("255")). // White text
		Background(lipgloss.Color("#8839ef")). // Mauve background
		Bold(false)

	t.Table.StatusRunning = lipgloss.NewStyle().Foreground(t.Success)
	t.Table.StatusError = lipgloss.NewStyle().Foreground(t.Error)
	t.Table.StatusWarning = lipgloss.NewStyle().Foreground(t.Warning)

	// AppTitle style
	t.AppTitle = lipgloss.NewStyle().
		Foreground(t.Primary).
		Background(lipgloss.Color("#e6e9ef")).
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

// ThemeSolarizedLight returns a Solarized Light theme
// Personality: Scientific precision, warm cream background, classic light theme
func ThemeSolarizedLight() *Theme {
	t := &Theme{Name: "solarized-light", Variant: "light"}

	// Solarized Light color palette - official scientifically calibrated colors
	t.Primary = lipgloss.AdaptiveColor{Light: "#268bd2", Dark: "#268bd2"}   // Blue
	t.Secondary = lipgloss.AdaptiveColor{Light: "#2aa198", Dark: "#2aa198"} // Cyan
	t.Accent = lipgloss.AdaptiveColor{Light: "#6c71c4", Dark: "#6c71c4"}    // Violet
	t.Foreground = lipgloss.AdaptiveColor{Light: "#002b36", Dark: "#002b36"} // base03
	t.Muted = lipgloss.AdaptiveColor{Light: "#586e75", Dark: "#586e75"}     // base01
	t.Error = lipgloss.AdaptiveColor{Light: "#dc322f", Dark: "#dc322f"}     // Red
	t.Success = lipgloss.AdaptiveColor{Light: "#859900", Dark: "#859900"}   // Green
	t.Warning = lipgloss.AdaptiveColor{Light: "#cb4b16", Dark: "#cb4b16"}   // Orange

	// UI element colors
	t.Border = lipgloss.AdaptiveColor{Light: "#93a1a1", Dark: "#93a1a1"}    // base1
	t.Dimmed = lipgloss.AdaptiveColor{Light: "#657b83", Dark: "#657b83"}    // base00
	t.Subtle = lipgloss.AdaptiveColor{Light: "#586e75", Dark: "#586e75"}    // base01
	t.Background = lipgloss.AdaptiveColor{Light: "#fdf6e3", Dark: "#fdf6e3"} // base3 (warm cream)

	// Visual depth (Solarized Light's precise layering)
	t.BackgroundBase = lipgloss.AdaptiveColor{Light: "#fdf6e3", Dark: "#fdf6e3"}    // base3
	t.BackgroundSurface = lipgloss.AdaptiveColor{Light: "#eee8d5", Dark: "#eee8d5"} // base2

	// Interactive states (Blue accents)
	t.Hover = lipgloss.AdaptiveColor{Light: "#2075b8", Dark: "#2075b8"}       // Darker blue
	t.Selection = lipgloss.AdaptiveColor{Light: "#268bd2", Dark: "#268bd2"}   // Blue selection

	// Frame elements (base1 borders, Blue focus)
	t.FrameBorder = lipgloss.AdaptiveColor{Light: "#93a1a1", Dark: "#93a1a1"}      // base1
	t.FrameFocusBorder = lipgloss.AdaptiveColor{Light: "#268bd2", Dark: "#268bd2"} // Blue

	// Enhanced semantics
	t.Info = lipgloss.AdaptiveColor{Light: "#2aa198", Dark: "#2aa198"} // Cyan

	// Message colors
	t.MessageSuccess = t.Success
	t.MessageError = t.Error
	t.MessageInfo = t.Primary
	t.MessageLoading = lipgloss.AdaptiveColor{Light: "#cb4b16", Dark: "#cb4b16"} // Orange

	// Palette colors (overlay on terminal background) - LIGHT THEME
	t.PaletteForeground = lipgloss.AdaptiveColor{Light: "235", Dark: "235"}         // Dark for light bg
	t.PaletteBackground = lipgloss.AdaptiveColor{Light: "255", Dark: "255"}         // Light background
	t.PaletteSelectedForeground = lipgloss.AdaptiveColor{Light: "255", Dark: "255"} // Bright on Subtle
	t.PaletteShortcut = lipgloss.AdaptiveColor{Light: "243", Dark: "243"}           // Medium gray

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
		Foreground(lipgloss.Color("#fdf6e3")). // Light text
		Background(lipgloss.Color("#268bd2")). // Blue background
		Bold(false)

	t.Table.StatusRunning = lipgloss.NewStyle().Foreground(t.Success)
	t.Table.StatusError = lipgloss.NewStyle().Foreground(t.Error)
	t.Table.StatusWarning = lipgloss.NewStyle().Foreground(t.Warning)

	// AppTitle style
	t.AppTitle = lipgloss.NewStyle().
		Foreground(t.Primary).
		Background(lipgloss.Color("#eee8d5")).
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

// ThemeGruvboxLight returns a Gruvbox Light theme
// Personality: Warm retro cream/brown, comfortable light theme
func ThemeGruvboxLight() *Theme {
	t := &Theme{Name: "gruvbox-light", Variant: "light"}

	// Gruvbox Light color palette - official light theme colors
	t.Primary = lipgloss.AdaptiveColor{Light: "#af3a03", Dark: "#af3a03"}   // Orange
	t.Secondary = lipgloss.AdaptiveColor{Light: "#79740e", Dark: "#79740e"} // Green
	t.Accent = lipgloss.AdaptiveColor{Light: "#b16286", Dark: "#b16286"}    // Purple
	t.Foreground = lipgloss.AdaptiveColor{Light: "#3c3836", Dark: "#3c3836"} // fg1
	t.Muted = lipgloss.AdaptiveColor{Light: "#7c6f64", Dark: "#7c6f64"}     // fg4
	t.Error = lipgloss.AdaptiveColor{Light: "#9d0006", Dark: "#9d0006"}     // Red
	t.Success = lipgloss.AdaptiveColor{Light: "#79740e", Dark: "#79740e"}   // Green
	t.Warning = lipgloss.AdaptiveColor{Light: "#b57614", Dark: "#b57614"}   // Yellow

	// UI element colors
	t.Border = lipgloss.AdaptiveColor{Light: "#d5c4a1", Dark: "#d5c4a1"}    // bg2
	t.Dimmed = lipgloss.AdaptiveColor{Light: "#665c54", Dark: "#665c54"}    // fg3
	t.Subtle = lipgloss.AdaptiveColor{Light: "#7c6f64", Dark: "#7c6f64"}    // fg4
	t.Background = lipgloss.AdaptiveColor{Light: "#fbf1c7", Dark: "#fbf1c7"} // bg (warm cream)

	// Visual depth (Gruvbox Light's warm layering)
	t.BackgroundBase = lipgloss.AdaptiveColor{Light: "#fbf1c7", Dark: "#fbf1c7"}    // bg
	t.BackgroundSurface = lipgloss.AdaptiveColor{Light: "#f2e5bc", Dark: "#f2e5bc"} // bg1

	// Interactive states (Warm orange)
	t.Hover = lipgloss.AdaptiveColor{Light: "#d65d0e", Dark: "#d65d0e"}       // Darker orange
	t.Selection = lipgloss.AdaptiveColor{Light: "#af3a03", Dark: "#af3a03"}   // Orange selection

	// Frame elements (Brown borders, Orange focus)
	t.FrameBorder = lipgloss.AdaptiveColor{Light: "#bdae93", Dark: "#bdae93"}      // bg3
	t.FrameFocusBorder = lipgloss.AdaptiveColor{Light: "#af3a03", Dark: "#af3a03"} // Orange

	// Enhanced semantics
	t.Info = lipgloss.AdaptiveColor{Light: "#076678", Dark: "#076678"} // Blue (aqua)

	// Message colors
	t.MessageSuccess = t.Success
	t.MessageError = t.Error
	t.MessageInfo = t.Primary
	t.MessageLoading = lipgloss.AdaptiveColor{Light: "#b57614", Dark: "#b57614"} // Yellow

	// Palette colors (overlay on terminal background) - LIGHT THEME
	t.PaletteForeground = lipgloss.AdaptiveColor{Light: "235", Dark: "235"}         // Dark for light bg
	t.PaletteBackground = lipgloss.AdaptiveColor{Light: "255", Dark: "255"}         // Light background
	t.PaletteSelectedForeground = lipgloss.AdaptiveColor{Light: "255", Dark: "255"} // Bright on Subtle
	t.PaletteShortcut = lipgloss.AdaptiveColor{Light: "243", Dark: "243"}           // Medium gray

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
		Foreground(lipgloss.Color("#fbf1c7")). // Light text
		Background(lipgloss.Color("#af3a03")). // Orange background
		Bold(false)

	t.Table.StatusRunning = lipgloss.NewStyle().Foreground(t.Success)
	t.Table.StatusError = lipgloss.NewStyle().Foreground(t.Error)
	t.Table.StatusWarning = lipgloss.NewStyle().Foreground(t.Warning)

	// AppTitle style
	t.AppTitle = lipgloss.NewStyle().
		Foreground(t.Primary).
		Background(lipgloss.Color("#f2e5bc")).
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
	// Dark themes
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
	// Light themes
	case "catppuccin-latte":
		return ThemeCatppuccinLatte()
	case "solarized-light":
		return ThemeSolarizedLight()
	case "gruvbox-light":
		return ThemeGruvboxLight()
	default:
		return ThemeCharm()
	}
}

// AvailableThemes returns a list of available theme names
func AvailableThemes() []string {
	return []string{
		// Dark themes (8)
		"charm", "dracula", "catppuccin", "nord", "gruvbox", "tokyo-night", "solarized", "monokai",
		// Light themes (3)
		"catppuccin-latte", "solarized-light", "gruvbox-light",
	}
}
