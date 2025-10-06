package screens

import "time"

// Screen configuration constants
const (
	// RefreshInterval is how often the TUI refreshes resource lists to catch
	// changes that may have been missed by informer watches. 1 second provides
	// near-real-time updates without excessive CPU usage.
	RefreshInterval = 1 * time.Second

	// ScreenPaddingLines is the additional padding added to screen layouts
	// for visual breathing room and consistent spacing.
	ScreenPaddingLines = 15
)
