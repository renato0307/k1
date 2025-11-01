package components

import (
	"fmt"
	"strings"
	"time"

	"github.com/renato0307/k1/internal/ui"
)

type Header struct {
	appName         string
	screenTitle     string
	namespace       string
	itemCount       int
	filterText      string // Contextual navigation filter
	refreshText     string // Last refresh text (e.g., "refreshing in 5s")
	lastRefresh     time.Time
	refreshInterval time.Duration // How often the screen refreshes
	width           int
	theme           *ui.Theme
	context         string // Current Kubernetes context
	contextLoading  bool   // Whether context is loading
	loadingMessage  string // Loading progress message
	loadingSpinner  int    // Spinner frame index (0-7)
}

func NewHeader(appName string, theme *ui.Theme) *Header {
	return &Header{
		appName: appName,
		theme:   theme,
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

func (h *Header) SetFilterText(text string) {
	h.filterText = text
}

func (h *Header) SetRefreshText(text string) {
	h.refreshText = text
}

func (h *Header) SetRefreshInterval(interval time.Duration) {
	h.refreshInterval = interval
}

func (h *Header) SetWidth(width int) {
	h.width = width
}

// SetContext sets the current context name
func (h *Header) SetContext(context string) {
	h.context = context
	h.contextLoading = false
}

// SetContextLoading sets context loading state with message
func (h *Header) SetContextLoading(context, message string) {
	h.context = context
	h.contextLoading = true
	h.loadingMessage = message
}

// AdvanceSpinner advances the loading spinner animation
func (h *Header) AdvanceSpinner() {
	h.loadingSpinner = (h.loadingSpinner + 1) % 8
}

// GetLoadingText returns loading spinner text if context is loading
func (h *Header) GetLoadingText() string {
	if !h.contextLoading {
		return ""
	}
	spinner := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧"}
	// Lowercase first letter for display
	message := h.loadingMessage
	if len(message) > 0 {
		message = strings.ToLower(message[:1]) + message[1:]
	}
	return fmt.Sprintf("%s %s", message, spinner[h.loadingSpinner])
}

// GetRefreshTimeString returns formatted refresh countdown string
func (h *Header) GetRefreshTimeString() string {
	if h.lastRefresh.IsZero() || h.refreshInterval == 0 {
		return ""
	}

	elapsed := time.Since(h.lastRefresh)
	remaining := h.refreshInterval - elapsed

	if remaining <= 0 {
		return "refreshing…"
	}

	seconds := int(remaining.Seconds())
	return fmt.Sprintf("refreshing in %ds", seconds)
}

func (h *Header) View() string {
	// Use theme's Header style (no background)
	headerStyle := h.theme.Header

	// Build left side: "Pods • refreshing in 10s"
	leftParts := []string{}

	if h.screenTitle != "" {
		leftParts = append(leftParts, h.screenTitle)
	}

	// Add filter text if present
	if h.filterText != "" {
		leftParts = append(leftParts, h.filterText)
	}

	if h.namespace != "" {
		leftParts = append(leftParts, fmt.Sprintf("namespace: %s", h.namespace))
	}

	if h.itemCount > 0 {
		leftParts = append(leftParts, fmt.Sprintf("%d items", h.itemCount))
	}

	// Add refresh text at the end if present
	if h.refreshText != "" {
		leftParts = append(leftParts, h.refreshText)
	}

	leftText := strings.Join(leftParts, " • ")
	if leftText == "" {
		leftText = h.appName
	}

	return headerStyle.Render(leftText)
}
