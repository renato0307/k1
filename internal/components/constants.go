package components

import "time"

// UI component constants
const (
	// MaxPaletteItems is the maximum number of items shown in the command
	// palette before scrolling is required. Set to 8 to fit comfortably on
	// most terminal sizes without overwhelming the user.
	MaxPaletteItems = 8

	// FullScreenReservedLines is the number of lines reserved for UI chrome
	// (header, command bar, borders) when showing full-screen views like YAML
	// or describe output. This ensures content doesn't overflow the terminal.
	FullScreenReservedLines = 3

	// StatusBarDisplayDuration is how long status messages (success, error,
	// info) are displayed before automatically clearing. 5 seconds provides
	// enough time to read without cluttering the UI.
	StatusBarDisplayDuration = 5 * time.Second
)
