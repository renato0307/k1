package components

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestOutputBuffer_Add(t *testing.T) {
	t.Run("basic add", func(t *testing.T) {
		buffer := NewOutputBuffer()
		entry := CommandOutput{
			Command:   "/scale deployment nginx 3",
			Output:    "Scaled to 3 replicas",
			Status:    "success",
			Context:   "prod-us",
			Timestamp: time.Now(),
		}

		buffer.Add(entry)

		assert.Equal(t, 1, buffer.Count())
		entries := buffer.GetAll()
		assert.Len(t, entries, 1)
		assert.Equal(t, entry.Command, entries[0].Command)
	})

	t.Run("bounded slice - oldest removed when exceeding max", func(t *testing.T) {
		buffer := NewOutputBuffer()

		// Add 101 entries (exceeds MaxOutputHistory of 100)
		for i := 0; i < 101; i++ {
			entry := CommandOutput{
				Command:   "/scale deployment nginx 3",
				Output:    "Scaled to 3 replicas",
				Status:    "success",
				Context:   "prod-us",
				Timestamp: time.Now().Add(time.Duration(i) * time.Second),
			}
			buffer.Add(entry)
		}

		// Should have exactly 100 entries (oldest removed)
		assert.Equal(t, MaxOutputHistory, buffer.Count())
		entries := buffer.GetAll()
		assert.Len(t, entries, MaxOutputHistory)

		// Newest entry should be first (reverse order for display)
		// The newest entry is the one added last (index 100 in loop)
		// It should have timestamp 100 seconds from the base
		assert.True(t, entries[0].Timestamp.After(entries[1].Timestamp))
	})

	t.Run("concurrent Add - race detector", func(t *testing.T) {
		buffer := NewOutputBuffer()
		var wg sync.WaitGroup

		// Spawn 10 goroutines adding entries concurrently
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				entry := CommandOutput{
					Command:   "/scale deployment nginx 3",
					Output:    "Scaled to 3 replicas",
					Status:    "success",
					Context:   "prod-us",
					Timestamp: time.Now(),
				}
				buffer.Add(entry)
			}(i)
		}

		wg.Wait()
		assert.Equal(t, 10, buffer.Count())
	})
}

func TestOutputBuffer_GetAll(t *testing.T) {
	t.Run("reverse order - newest first", func(t *testing.T) {
		buffer := NewOutputBuffer()

		// Add entries with distinct timestamps
		for i := 0; i < 5; i++ {
			entry := CommandOutput{
				Command:   "/scale deployment nginx 3",
				Output:    "Scaled to 3 replicas",
				Status:    "success",
				Context:   "prod-us",
				Timestamp: time.Now().Add(time.Duration(i) * time.Second),
			}
			buffer.Add(entry)
		}

		entries := buffer.GetAll()

		// Verify reverse order (newest first)
		for i := 0; i < len(entries)-1; i++ {
			assert.True(t, entries[i].Timestamp.After(entries[i+1].Timestamp),
				"Entry at index %d should have newer timestamp than entry at %d", i, i+1)
		}
	})

	t.Run("empty buffer", func(t *testing.T) {
		buffer := NewOutputBuffer()
		entries := buffer.GetAll()

		assert.NotNil(t, entries)
		assert.Len(t, entries, 0)
	})

	t.Run("concurrent GetAll while Add - race detector", func(t *testing.T) {
		buffer := NewOutputBuffer()
		var wg sync.WaitGroup

		// Spawn goroutines adding entries
		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				entry := CommandOutput{
					Command:   "/scale deployment nginx 3",
					Output:    "Scaled to 3 replicas",
					Status:    "success",
					Context:   "prod-us",
					Timestamp: time.Now(),
				}
				buffer.Add(entry)
			}()
		}

		// Spawn goroutines reading entries
		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_ = buffer.GetAll()
			}()
		}

		wg.Wait()
		// If we get here without deadlock or race, test passes
	})
}

func TestOutputBuffer_Clear(t *testing.T) {
	t.Run("clear removes all", func(t *testing.T) {
		buffer := NewOutputBuffer()

		// Add some entries
		for i := 0; i < 5; i++ {
			entry := CommandOutput{
				Command:   "/scale deployment nginx 3",
				Output:    "Scaled to 3 replicas",
				Status:    "success",
				Context:   "prod-us",
				Timestamp: time.Now(),
			}
			buffer.Add(entry)
		}

		assert.Equal(t, 5, buffer.Count())

		buffer.Clear()

		assert.Equal(t, 0, buffer.Count())
		entries := buffer.GetAll()
		assert.Len(t, entries, 0)
	})

	t.Run("concurrent Clear and Add - race detector", func(t *testing.T) {
		buffer := NewOutputBuffer()
		var wg sync.WaitGroup

		// Spawn goroutines adding entries
		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				entry := CommandOutput{
					Command:   "/scale deployment nginx 3",
					Output:    "Scaled to 3 replicas",
					Status:    "success",
					Context:   "prod-us",
					Timestamp: time.Now(),
				}
				buffer.Add(entry)
			}()
		}

		// Spawn goroutines clearing
		for i := 0; i < 2; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				buffer.Clear()
			}()
		}

		wg.Wait()
		// If we get here without deadlock or race, test passes
	})
}

func TestOutputBuffer_Count(t *testing.T) {
	buffer := NewOutputBuffer()

	assert.Equal(t, 0, buffer.Count())

	buffer.Add(CommandOutput{Command: "test1"})
	assert.Equal(t, 1, buffer.Count())

	buffer.Add(CommandOutput{Command: "test2"})
	assert.Equal(t, 2, buffer.Count())

	buffer.Clear()
	assert.Equal(t, 0, buffer.Count())
}
