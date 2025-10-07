package components

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/renato0307/k1/internal/types"
	"github.com/renato0307/k1/internal/ui"
)

type Header struct {
	appName      string
	screenTitle  string
	namespace    string
	itemCount    int
	lastRefresh  time.Time
	width        int
	theme        *ui.Theme
	navContext   *types.NavigationContext
}

func NewHeader(ctx *types.AppContext, appName string) *Header {
	return &Header{
		appName: appName,
		theme:   ctx.Theme,
	}
}

func (h *Header) SetScreenTitle(title string) {
	h.screenTitle = title
}

func (h *Header) SetNamespace(namespace string) {
	h.namespace = namespace
}

func (h *Header) SetItemCount(count int) {
	h.itemCount = count
}

func (h *Header) SetLastRefresh(t time.Time) {
	h.lastRefresh = t
}

func (h *Header) SetWidth(width int) {
	h.width = width
}

func (h *Header) SetNavigationContext(ctx *types.NavigationContext) {
	h.navContext = ctx
}

func (h *Header) View() string {
	// Build header style from theme
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(h.theme.Primary)

	timingStyle := lipgloss.NewStyle().
		Foreground(h.theme.Muted).
		Padding(0, 1)

	// Build left side: "Pods • namespace: default • 47 items"
	// If navigation context exists, show breadcrumb: "Pods (Deployment: my-app)"
	leftParts := []string{}
	if h.screenTitle != "" {
		title := h.screenTitle
		// Add breadcrumb if navigation context exists
		if h.navContext != nil && h.navContext.FilterLabel != "" {
			// Truncate resource name if too long
			filterLabel := h.navContext.FilterLabel
			if len(filterLabel) > 40 {
				filterLabel = filterLabel[:37] + "..."
			}
			title = fmt.Sprintf("%s (%s)", h.screenTitle, filterLabel)
		}
		leftParts = append(leftParts, title)
	}

	if h.namespace != "" {
		leftParts = append(leftParts, fmt.Sprintf("namespace: %s", h.namespace))
	}

	if h.itemCount > 0 {
		leftParts = append(leftParts, fmt.Sprintf("%d items", h.itemCount))
	}

	leftText := strings.Join(leftParts, " • ")
	if leftText == "" {
		leftText = h.appName
	}
	left := headerStyle.Render(leftText)

	// Build right side: "Last refresh: 2s ago"
	var right string
	if !h.lastRefresh.IsZero() {
		elapsed := time.Since(h.lastRefresh)
		var timeStr string
		if elapsed < time.Minute {
			timeStr = fmt.Sprintf("%ds ago", int(elapsed.Seconds()))
		} else if elapsed < time.Hour {
			timeStr = fmt.Sprintf("%dm ago", int(elapsed.Minutes()))
		} else {
			timeStr = fmt.Sprintf("%dh ago", int(elapsed.Hours()))
		}
		right = timingStyle.Render(fmt.Sprintf("Last refresh: %s", timeStr))
	}

	// Calculate spacing to push timing to the right
	leftWidth := lipgloss.Width(left)
	rightWidth := lipgloss.Width(right)
	spacing := h.width - leftWidth - rightWidth
	if spacing < 0 {
		spacing = 0
	}

	spacer := lipgloss.NewStyle().
		Width(spacing).
		Render("")

	return lipgloss.JoinHorizontal(lipgloss.Top, left, spacer, right)
}
