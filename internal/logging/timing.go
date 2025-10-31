package logging

import (
	"time"
)

// TimingContext holds timing information for manual Start/End tracking
type TimingContext struct {
	name      string
	startTime time.Time
}

// Time executes the given function and logs its execution time.
// This is useful for wrapping existing code blocks with timing measurement.
//
// Example:
//
//	logging.Time("sync informers", func() {
//	    // ... sync logic here ...
//	})
func Time(name string, fn func()) {
	if !IsEnabled() {
		fn()
		return
	}

	start := time.Now()
	fn()
	duration := time.Since(start)

	Get().Debug(name,
		"duration", duration.String(),
		"ms", duration.Milliseconds(),
	)
}

// TimeWithResult executes the given function and logs its execution time,
// returning the function's result.
//
// Example:
//
//	result, err := logging.TimeWithResult("fetch resources", func() ([]Pod, error) {
//	    return repo.GetPods()
//	})
func TimeWithResult[T any](name string, fn func() T) T {
	if !IsEnabled() {
		return fn()
	}

	start := time.Now()
	result := fn()
	duration := time.Since(start)

	Get().Debug(name,
		"duration", duration.String(),
		"ms", duration.Milliseconds(),
	)

	return result
}

// Start begins a timing measurement for manual control.
// Must be paired with End() to log the duration.
//
// Example:
//
//	ctx := logging.Start("operation name")
//	// ... do work ...
//	logging.End(ctx)
func Start(name string) TimingContext {
	return TimingContext{
		name:      name,
		startTime: time.Now(),
	}
}

// End completes a timing measurement started with Start() and logs the duration.
//
// Example:
//
//	ctx := logging.Start("operation name")
//	// ... do work ...
//	logging.End(ctx)
func End(ctx TimingContext) {
	if !IsEnabled() {
		return
	}

	duration := time.Since(ctx.startTime)
	Get().Debug(ctx.name,
		"duration", duration.String(),
		"ms", duration.Milliseconds(),
	)
}

// EndWithCount completes a timing measurement and logs the duration with an item count.
// This is useful for operations that process multiple items.
//
// Example:
//
//	ctx := logging.Start("sync pods")
//	pods := fetchPods()
//	logging.EndWithCount(ctx, len(pods))
func EndWithCount(ctx TimingContext, count int) {
	if !IsEnabled() {
		return
	}

	duration := time.Since(ctx.startTime)
	Get().Debug(ctx.name,
		"duration", duration.String(),
		"ms", duration.Milliseconds(),
		"count", count,
	)
}

// TimeMethod is a helper for timing methods with the logger instance.
// Use this when you want to use a specific logger instead of the global one.
//
// Example:
//
//	logger := logging.Get().With("component", "repository")
//	logger.Time("sync informers", func() {
//	    // ... sync logic ...
//	})
func (l *Logger) Time(name string, fn func()) {
	if !l.IsEnabled() {
		fn()
		return
	}

	start := time.Now()
	fn()
	duration := time.Since(start)

	l.Debug(name,
		"duration", duration.String(),
		"ms", duration.Milliseconds(),
	)
}
