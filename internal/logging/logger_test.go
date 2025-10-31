package logging

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

func TestInit(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "valid config with file",
			config: Config{
				FilePath:   filepath.Join(t.TempDir(), "test.log"),
				Level:      slog.LevelInfo,
				Format:     FormatText,
				MaxSizeMB:  10,
				MaxBackups: 2,
			},
			wantErr: false,
		},
		{
			name: "empty filepath creates noop logger",
			config: Config{
				FilePath: "",
				Level:    slog.LevelInfo,
				Format:   FormatText,
			},
			wantErr: false,
		},
		{
			name: "json format",
			config: Config{
				FilePath:   filepath.Join(t.TempDir(), "test.log"),
				Level:      slog.LevelDebug,
				Format:     FormatJSON,
				MaxSizeMB:  10,
				MaxBackups: 2,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Init(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("Init() error = %v, wantErr %v", err, tt.wantErr)
			}

			// Verify logger is accessible
			logger := Get()
			if logger == nil {
				t.Error("Get() returned nil logger")
			}

			// Test logging doesn't panic
			logger.Info("test message")
			logger.Debug("test debug")
			logger.Warn("test warning")
			logger.Error("test error")

			// Clean up
			Shutdown()
		})
	}
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected slog.Level
	}{
		{"debug", slog.LevelDebug},
		{"info", slog.LevelInfo},
		{"warn", slog.LevelWarn},
		{"error", slog.LevelError},
		{"invalid", slog.LevelInfo}, // defaults to info
		{"", slog.LevelInfo},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := ParseLevel(tt.input)
			if result != tt.expected {
				t.Errorf("ParseLevel(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestParseFormat(t *testing.T) {
	tests := []struct {
		input    string
		expected LogFormat
	}{
		{"text", FormatText},
		{"json", FormatJSON},
		{"invalid", FormatText}, // defaults to text
		{"", FormatText},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := ParseFormat(tt.input)
			if result != tt.expected {
				t.Errorf("ParseFormat(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestIsEnabled(t *testing.T) {
	// Test with empty filepath (noop logger)
	err := Init(Config{FilePath: ""})
	if err != nil {
		t.Fatalf("Init() failed: %v", err)
	}
	if IsEnabled() {
		t.Error("IsEnabled() should return false with empty filepath")
	}

	// Test with valid filepath
	logFile := filepath.Join(t.TempDir(), "test.log")
	err = Init(Config{
		FilePath:   logFile,
		Level:      slog.LevelInfo,
		Format:     FormatText,
		MaxSizeMB:  10,
		MaxBackups: 2,
	})
	if err != nil {
		t.Fatalf("Init() failed: %v", err)
	}
	if !IsEnabled() {
		t.Error("IsEnabled() should return true with valid filepath")
	}

	// Clean up
	Shutdown()
	os.Remove(logFile)
}

func TestLoggerWith(t *testing.T) {
	logFile := filepath.Join(t.TempDir(), "test.log")
	err := Init(Config{
		FilePath:   logFile,
		Level:      slog.LevelInfo,
		Format:     FormatText,
		MaxSizeMB:  10,
		MaxBackups: 2,
	})
	if err != nil {
		t.Fatalf("Init() failed: %v", err)
	}
	defer Shutdown()

	// Test With() method
	logger := Get().With("component", "test", "version", 1)
	if logger == nil {
		t.Error("With() returned nil logger")
	}

	// Should not panic
	logger.Info("test message with context")
}

func TestPackageLevelFunctions(t *testing.T) {
	logFile := filepath.Join(t.TempDir(), "test.log")
	err := Init(Config{
		FilePath:   logFile,
		Level:      slog.LevelDebug,
		Format:     FormatText,
		MaxSizeMB:  10,
		MaxBackups: 2,
	})
	if err != nil {
		t.Fatalf("Init() failed: %v", err)
	}
	defer Shutdown()

	// Test package-level convenience functions
	Debug("debug message", "key", "value")
	Info("info message", "key", "value")
	Warn("warn message", "key", "value")
	Error("error message", "key", "value")

	// Should not panic and should write to file
	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	if len(content) == 0 {
		t.Error("Log file is empty, expected log messages")
	}
}
