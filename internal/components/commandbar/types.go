package commandbar

import "time"

// CommandType represents the type of command being entered.
type CommandType int

const (
	CommandTypeFilter    CommandType = iota // no prefix
	CommandTypeResource                     // : prefix
	CommandTypeAction                       // / prefix
	CommandTypeLLMAction                    // /ai prefix
)

// CommandBarState represents the current state of the command bar.
type CommandBarState int

const (
	StateHidden            CommandBarState = iota
	StateFilter                            // No prefix, filtering list
	StateSuggestionPalette                 // : or / pressed, showing suggestions
	StateInput                             // Direct command input
	StateConfirmation                      // Destructive operation confirmation
	StateLLMPreview                        // /ai command preview
	StateResult                            // Success/error message
)

// tipRotationMsg triggers rotation to next tip
type tipRotationMsg time.Time
