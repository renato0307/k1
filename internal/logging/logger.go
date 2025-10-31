// Package logging provides a structured logging wrapper around Go's log/slog
// with support for file output, log rotation, and execution timing helpers.
package logging

import (
	"context"
	"io"
	"log/slog"
	"time"

	"gopkg.in/natefinch/lumberjack.v2"
)

// Logger wraps slog.Logger with convenience methods for k1
type Logger struct {
	logger *slog.Logger
}

// LogFormat represents the output format for logs
type LogFormat string

const (
	// FormatText outputs human-readable text logs
	FormatText LogFormat = "text"
	// FormatJSON outputs structured JSON logs
	FormatJSON LogFormat = "json"
)

// Config holds configuration for logger initialization
type Config struct {
	// FilePath is the path to the log file (empty = no logging)
	FilePath string
	// Level is the minimum log level (debug, info, warn, error)
	Level slog.Level
	// Format is the output format (text or json)
	Format LogFormat
	// MaxSizeMB is the maximum size in MB before rotation
	MaxSizeMB int
	// MaxBackups is the maximum number of old log files to keep
	MaxBackups int
}

var (
	// globalLogger is the package-level logger instance
	globalLogger *Logger
	// noopLogger is used when logging is disabled
	noopLogger = &Logger{logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
)

// Init initializes the global logger with the given configuration.
// If config.FilePath is empty, logging is disabled (noop logger).
func Init(config Config) error {
	if config.FilePath == "" {
		globalLogger = noopLogger
		return nil
	}

	// Set up log rotation with lumberjack
	writer := &lumberjack.Logger{
		Filename:   config.FilePath,
		MaxSize:    config.MaxSizeMB,
		MaxBackups: config.MaxBackups,
		Compress:   true,
	}

	// Create handler based on format
	var handler slog.Handler
	opts := &slog.HandlerOptions{
		Level: config.Level,
	}

	switch config.Format {
	case FormatJSON:
		handler = slog.NewJSONHandler(writer, opts)
	default:
		handler = slog.NewTextHandler(writer, opts)
	}

	globalLogger = &Logger{
		logger: slog.New(handler),
	}

	return nil
}

// Get returns the global logger instance.
// Returns a noop logger if Init was not called or logging is disabled.
func Get() *Logger {
	if globalLogger == nil {
		return noopLogger
	}
	return globalLogger
}

// Debug logs a debug message
func (l *Logger) Debug(msg string, args ...any) {
	l.logger.Debug(msg, args...)
}

// Info logs an info message
func (l *Logger) Info(msg string, args ...any) {
	l.logger.Info(msg, args...)
}

// Warn logs a warning message
func (l *Logger) Warn(msg string, args ...any) {
	l.logger.Warn(msg, args...)
}

// Error logs an error message
func (l *Logger) Error(msg string, args ...any) {
	l.logger.Error(msg, args...)
}

// With returns a new Logger with the given key-value pairs added as context
func (l *Logger) With(args ...any) *Logger {
	return &Logger{
		logger: l.logger.With(args...),
	}
}

// WithContext returns a new Logger with context
func (l *Logger) WithContext(ctx context.Context) *Logger {
	return &Logger{
		logger: l.logger.With(slog.Any("context", ctx)),
	}
}

// IsEnabled returns true if logging is enabled (not noop)
func (l *Logger) IsEnabled() bool {
	return l != noopLogger
}

// Package-level convenience functions

// Debug logs a debug message using the global logger
func Debug(msg string, args ...any) {
	Get().Debug(msg, args...)
}

// Info logs an info message using the global logger
func Info(msg string, args ...any) {
	Get().Info(msg, args...)
}

// Warn logs a warning message using the global logger
func Warn(msg string, args ...any) {
	Get().Warn(msg, args...)
}

// Error logs an error message using the global logger
func Error(msg string, args ...any) {
	Get().Error(msg, args...)
}

// IsEnabled returns true if logging is enabled globally
func IsEnabled() bool {
	return Get().IsEnabled()
}

// ParseLevel converts a string to slog.Level
func ParseLevel(level string) slog.Level {
	switch level {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// ParseFormat converts a string to LogFormat
func ParseFormat(format string) LogFormat {
	switch format {
	case "json":
		return FormatJSON
	case "text":
		return FormatText
	default:
		return FormatText
	}
}

// Shutdown closes the logger and flushes any buffered logs
func Shutdown() error {
	// lumberjack doesn't need explicit shutdown, but we can close the handler
	// For now, this is a noop, but we include it for future extensibility
	return nil
}

// timeTracker holds timing information for Time() helper
type timeTracker struct {
	name      string
	startTime time.Time
	logger    *Logger
}

// done completes the timing measurement and logs the duration
func (t *timeTracker) done() {
	duration := time.Since(t.startTime)
	t.logger.Debug(t.name, "duration", duration.String(), "ms", duration.Milliseconds())
}
