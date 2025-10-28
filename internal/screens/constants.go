package screens

import "time"

// Screen configuration constants
const (
	// RefreshInterval is how often the TUI refreshes resource lists to catch
	// changes that may have been missed by informer watches. 10 seconds provides
	// frequent updates without excessive CPU usage.
	RefreshInterval = 10 * time.Second

	// ContextsRefreshInterval is how often the contexts screen refreshes.
	// 30 seconds is sufficient since contexts don't change often.
	ContextsRefreshInterval = 30 * time.Second

	// ScreenPaddingLines is the additional padding added to screen layouts
	// for visual breathing room and consistent spacing.
	ScreenPaddingLines = 15
)
