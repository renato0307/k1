package commandbar

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/renato0307/k1/internal/commands"
	"github.com/renato0307/k1/internal/k8s"
	"github.com/renato0307/k1/internal/types"
	"github.com/renato0307/k1/internal/ui"
)

// Palette manages command palette filtering, rendering, and navigation.
type Palette struct {
	items    []commands.Command
	index    int
	registry *commands.Registry
	theme    *ui.Theme
	width    int
}

// NewPalette creates a new palette manager.
func NewPalette(ctx *types.AppContext, registry *commands.Registry, width int) *Palette {
	return &Palette{
		items:    []commands.Command{},
		index:    0,
		registry: registry,
		theme:    ctx.Theme,
		width:    width,
	}
}

// SetWidth updates the palette width.
func (p *Palette) SetWidth(width int) {
	p.width = width
}

// Filter filters commands by query and command type.
// Handles special case of /ai for LLM commands.
// Filters resource commands by current screen (resource type).
func (p *Palette) Filter(query string, cmdType CommandType, screenID string) {
	var items []commands.Command

	switch cmdType {
	case CommandTypeResource:
		category := commands.CategoryResource
		if query == "" {
			items = p.registry.GetByCategory(category)
		} else {
			items = p.registry.Filter(query, category)
		}

	case CommandTypeAction:
		category := commands.CategoryAction
		if query == "" {
			items = p.registry.GetByCategory(category)
		} else {
			items = p.registry.Filter(query, category)
		}

		// Filter by current screen (resource type)
		items = p.registry.FilterByResourceType(items, k8s.ResourceType(screenID))

		// Add /ai option if it matches the query
		if strings.HasPrefix("ai", strings.ToLower(query)) || query == "" {
			items = append(items, commands.Command{
				Name:        "ai",
				Description: "Natural language AI commands",
				Category:    commands.CategoryLLMAction,
				Execute:     nil,
			})
		}
	}

	p.items = items
	p.index = 0
}

// NavigateUp moves selection up in palette.
func (p *Palette) NavigateUp() {
	if p.index > 0 {
		p.index--
	}
}

// NavigateDown moves selection down in palette.
func (p *Palette) NavigateDown() {
	if p.index < len(p.items)-1 {
		p.index++
	}
}

// GetSelected returns the currently selected command, or nil if empty.
func (p *Palette) GetSelected() *commands.Command {
	if p.index >= 0 && p.index < len(p.items) {
		return &p.items[p.index]
	}
	return nil
}

// IsEmpty returns true if palette has no items.
func (p *Palette) IsEmpty() bool {
	return len(p.items) == 0
}

// Size returns the number of items in palette.
func (p *Palette) Size() int {
	return len(p.items)
}

// Reset clears the palette.
func (p *Palette) Reset() {
	p.items = []commands.Command{}
	p.index = 0
}

// GetHeight returns the height needed to display the palette.
// Returns 0 if palette is empty.
func (p *Palette) GetHeight() int {
	if p.IsEmpty() {
		return 0
	}

	return min(len(p.items), MaxPaletteItems)
}

// View renders the palette items with selection indicator.
func (p *Palette) View(prefix string) string {
	if p.IsEmpty() {
		return ""
	}

	sections := []string{}

	// Show up to MaxPaletteItems
	maxItems := min(MaxPaletteItems, len(p.items))

	// First pass: find longest description to align shortcuts
	longestMainText := 0
	for i := 0; i < maxItems; i++ {
		cmd := p.items[i]
		mainText := prefix + cmd.Name
		if cmd.ArgPattern != "" {
			mainText += cmd.ArgPattern
		}
		mainText += " - " + cmd.Description
		if len(mainText) > longestMainText {
			longestMainText = len(mainText)
		}
	}

	// Add 10 spaces for separation
	shortcutColumn := longestMainText + 10

	// Second pass: render items with aligned shortcuts
	for i := 0; i < maxItems; i++ {
		cmd := p.items[i]
		mainText := prefix + cmd.Name
		if cmd.ArgPattern != "" {
			mainText += cmd.ArgPattern
		}
		mainText += " - " + cmd.Description

		var line string
		if cmd.Shortcut != "" {
			// Pad to shortcut column position (minimum 2 spaces)
			padding := max(shortcutColumn-len(mainText), 2)
			spacer := strings.Repeat(" ", padding)

			// Style shortcut with dimmed color
			shortcutStyle := lipgloss.NewStyle().
				Foreground(p.theme.Dimmed)
			styledShortcut := shortcutStyle.Render(cmd.Shortcut)

			itemContent := mainText + spacer + styledShortcut

			if i == p.index {
				selectedStyle := lipgloss.NewStyle().
					Foreground(p.theme.Foreground).
					Background(p.theme.Subtle).
					Width(p.width).
					Padding(0, 1).
					Bold(true)
				line = selectedStyle.Render("▶ " + itemContent)
			} else {
				paletteStyle := lipgloss.NewStyle().
					Width(p.width).
					Padding(0, 1)
				line = paletteStyle.Render("  " + itemContent)
			}
		} else {
			// No shortcut, simple rendering
			if i == p.index {
				selectedStyle := lipgloss.NewStyle().
					Foreground(p.theme.Foreground).
					Background(p.theme.Subtle).
					Width(p.width).
					Padding(0, 1).
					Bold(true)
				line = selectedStyle.Render("▶ " + mainText)
			} else {
				paletteStyle := lipgloss.NewStyle().
					Width(p.width).
					Padding(0, 1)
				line = paletteStyle.Render("  " + mainText)
			}
		}

		sections = append(sections, line)
	}

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}
