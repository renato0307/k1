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

	// Common column width constants
	// Only defined for the most frequently used columns that appear across
	// almost every screen (Name, Namespace, Age, Status)

	// Name column (primary identifier)
	NameMinWidth = 20
	NameMaxWidth = 50
	NameWeight   = 3

	// Namespace column (critical context)
	NamespaceMinWidth = 10
	NamespaceMaxWidth = 30
	NamespaceWeight   = 2

	// Status column (state/phase)
	StatusMinWidth = 10
	StatusMaxWidth = 15
	StatusWeight   = 1

	// Age column (timestamp display)
	AgeMinWidth = 8
	AgeMaxWidth = 12
	AgeWeight   = 1
)
