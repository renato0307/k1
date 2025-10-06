package commandbar

// History manages command history with deduplication and size limits.
// It provides stateless helper functions for history management.
type History struct {
	entries []string
	index   int // Current position in history (-1 means not navigating)
}

// NewHistory creates a new history manager.
func NewHistory() *History {
	return &History{
		entries: []string{},
		index:   -1,
	}
}

// Add adds a command to history, avoiding duplicates of most recent entry.
func (h *History) Add(cmd string) {
	// Don't add empty commands
	if len(cmd) == 0 {
		return
	}

	// Don't add if it's the same as the most recent command
	if len(h.entries) > 0 && h.entries[len(h.entries)-1] == cmd {
		return
	}

	// Add to history
	h.entries = append(h.entries, cmd)

	// Keep max 100 entries
	const maxHistory = 100
	if len(h.entries) > maxHistory {
		h.entries = h.entries[len(h.entries)-maxHistory:]
	}

	// Reset index
	h.index = -1
}

// NavigateUp navigates backwards in history (older commands).
// Returns the command at the new position and whether navigation succeeded.
func (h *History) NavigateUp() (string, bool) {
	if len(h.entries) == 0 {
		return "", false
	}

	if h.index == -1 {
		// Start from most recent
		h.index = len(h.entries) - 1
	} else if h.index > 0 {
		h.index--
	}

	return h.entries[h.index], true
}

// NavigateDown navigates forwards in history (newer commands).
// Returns the command at the new position and whether navigation succeeded.
// Returns empty string and false when reaching the end (most recent).
func (h *History) NavigateDown() (string, bool) {
	if len(h.entries) == 0 || h.index == -1 {
		return "", false
	}

	if h.index < len(h.entries)-1 {
		h.index++
		return h.entries[h.index], true
	}

	// At most recent, clear input
	h.index = -1
	return "", false
}

// Reset resets the history navigation index.
func (h *History) Reset() {
	h.index = -1
}

// IsEmpty returns true if history is empty.
func (h *History) IsEmpty() bool {
	return len(h.entries) == 0
}

// Size returns the number of entries in history.
func (h *History) Size() int {
	return len(h.entries)
}
