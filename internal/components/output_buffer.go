package components

import (
	"sync"
	"time"
)

const MaxOutputHistory = 100

// CommandOutput represents a single command execution in history
type CommandOutput struct {
	Command        string
	KubectlCommand string
	Output         string
	Status         string // "success", "error", "info"
	Context        string
	ResourceType   string
	ResourceName   string
	Namespace      string
	Timestamp      time.Time
	Duration       time.Duration
}

// OutputBuffer manages command output history
type OutputBuffer struct {
	mu      sync.RWMutex
	entries []CommandOutput
}

// NewOutputBuffer creates a new output buffer
func NewOutputBuffer() *OutputBuffer {
	return &OutputBuffer{
		entries: make([]CommandOutput, 0, MaxOutputHistory),
	}
}

// Add appends entry to history (bounded slice pattern)
func (b *OutputBuffer) Add(entry CommandOutput) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.entries = append(b.entries, entry)

	// Truncate to max size (follows commandbar/history.go:34 pattern)
	if len(b.entries) > MaxOutputHistory {
		b.entries = b.entries[len(b.entries)-MaxOutputHistory:]
	}
}

// GetAll returns all entries (newest first for display)
func (b *OutputBuffer) GetAll() []CommandOutput {
	b.mu.RLock()
	defer b.mu.RUnlock()

	// Reverse order for display (newest first)
	result := make([]CommandOutput, len(b.entries))
	for i, entry := range b.entries {
		result[len(b.entries)-1-i] = entry
	}
	return result
}

// Clear removes all entries
func (b *OutputBuffer) Clear() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.entries = make([]CommandOutput, 0, MaxOutputHistory)
}

// Count returns number of entries
func (b *OutputBuffer) Count() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.entries)
}
