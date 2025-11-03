package commandbar

import "time"

// MaxPaletteItems is the maximum number of items shown in the command palette.
// Set to 8 to fit comfortably on most terminal sizes without overwhelming the user.
const MaxPaletteItems = 8

// TipRotationInterval is how often tips rotate in the hints line
const TipRotationInterval = 15 * time.Second
