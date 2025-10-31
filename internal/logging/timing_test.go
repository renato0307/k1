package logging

import (
	"path/filepath"
	"testing"
	"time"
)

func TestTime(t *testing.T) {
	logFile := filepath.Join(t.TempDir(), "test.log")
	err := Init(Config{
		FilePath:   logFile,
		Level:      ParseLevel("debug"),
		Format:     FormatText,
		MaxSizeMB:  10,
		MaxBackups: 2,
	})
	if err != nil {
		t.Fatalf("Init() failed: %v", err)
	}
	defer Shutdown()

	executed := false
	Time("test operation", func() {
		time.Sleep(10 * time.Millisecond)
		executed = true
	})

	if !executed {
		t.Error("Time() did not execute the function")
	}
}

func TestTimeWithNoLogging(t *testing.T) {
	// Initialize with empty filepath (noop logger)
	err := Init(Config{FilePath: ""})
	if err != nil {
		t.Fatalf("Init() failed: %v", err)
	}

	executed := false
	Time("test operation", func() {
		executed = true
	})

	if !executed {
		t.Error("Time() did not execute the function when logging is disabled")
	}
}

func TestTimeWithResult(t *testing.T) {
	logFile := filepath.Join(t.TempDir(), "test.log")
	err := Init(Config{
		FilePath:   logFile,
		Level:      ParseLevel("debug"),
		Format:     FormatText,
		MaxSizeMB:  10,
		MaxBackups: 2,
	})
	if err != nil {
		t.Fatalf("Init() failed: %v", err)
	}
	defer Shutdown()

	result := TimeWithResult("test operation", func() int {
		time.Sleep(10 * time.Millisecond)
		return 42
	})

	if result != 42 {
		t.Errorf("TimeWithResult() returned %v, want 42", result)
	}
}

func TestStartEnd(t *testing.T) {
	logFile := filepath.Join(t.TempDir(), "test.log")
	err := Init(Config{
		FilePath:   logFile,
		Level:      ParseLevel("debug"),
		Format:     FormatText,
		MaxSizeMB:  10,
		MaxBackups: 2,
	})
	if err != nil {
		t.Fatalf("Init() failed: %v", err)
	}
	defer Shutdown()

	ctx := Start("test operation")
	time.Sleep(10 * time.Millisecond)
	End(ctx)

	// Should not panic
}

func TestEndWithCount(t *testing.T) {
	logFile := filepath.Join(t.TempDir(), "test.log")
	err := Init(Config{
		FilePath:   logFile,
		Level:      ParseLevel("debug"),
		Format:     FormatText,
		MaxSizeMB:  10,
		MaxBackups: 2,
	})
	if err != nil {
		t.Fatalf("Init() failed: %v", err)
	}
	defer Shutdown()

	ctx := Start("test operation")
	time.Sleep(10 * time.Millisecond)
	EndWithCount(ctx, 100)

	// Should not panic
}

func TestStartEndWithNoLogging(t *testing.T) {
	// Initialize with empty filepath (noop logger)
	err := Init(Config{FilePath: ""})
	if err != nil {
		t.Fatalf("Init() failed: %v", err)
	}

	// Should not panic when logging is disabled
	ctx := Start("test operation")
	End(ctx)
	EndWithCount(ctx, 50)
}

func TestLoggerTime(t *testing.T) {
	logFile := filepath.Join(t.TempDir(), "test.log")
	err := Init(Config{
		FilePath:   logFile,
		Level:      ParseLevel("debug"),
		Format:     FormatText,
		MaxSizeMB:  10,
		MaxBackups: 2,
	})
	if err != nil {
		t.Fatalf("Init() failed: %v", err)
	}
	defer Shutdown()

	logger := Get().With("component", "test")
	executed := false
	logger.Time("test operation", func() {
		time.Sleep(10 * time.Millisecond)
		executed = true
	})

	if !executed {
		t.Error("Logger.Time() did not execute the function")
	}
}

func TestTimingAccuracy(t *testing.T) {
	logFile := filepath.Join(t.TempDir(), "test.log")
	err := Init(Config{
		FilePath:   logFile,
		Level:      ParseLevel("debug"),
		Format:     FormatText,
		MaxSizeMB:  10,
		MaxBackups: 2,
	})
	if err != nil {
		t.Fatalf("Init() failed: %v", err)
	}
	defer Shutdown()

	// Test that timing is reasonably accurate
	expectedDuration := 50 * time.Millisecond
	start := time.Now()
	Time("accuracy test", func() {
		time.Sleep(expectedDuration)
	})
	actual := time.Since(start)

	// Allow 20ms variance for system scheduling
	if actual < expectedDuration || actual > expectedDuration+20*time.Millisecond {
		t.Errorf("Timing accuracy off: expected ~%v, got %v", expectedDuration, actual)
	}
}
