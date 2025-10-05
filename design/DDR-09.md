# Logging System for TUI Troubleshooting

| Metadata | Value                                          |
|----------|------------------------------------------------|
| Date     | 2025-10-05                                     |
| Author   | @renato0307                                    |
| Status   | Proposed                                       |
| Tags     | logging, troubleshooting, diagnostics, slog    |

| Revision | Date       | Author      | Info           |
|----------|------------|-------------|----------------|
| 1        | 2025-10-05 | @renato0307 | Initial design |

## Context and Problem Statement

k1 is a TUI (Terminal User Interface) application that manages
Kubernetes clusters. When issues occur (crashes, performance problems,
unexpected behavior), troubleshooting is difficult without diagnostic
information. However, traditional logging to stdout/stderr interferes
with the TUI rendering.

**Key challenges:**
- TUI applications own stdout/stderr for rendering
- Users need troubleshooting capabilities without disrupting UX
- Performance issues need metrics (informer sync, API latency, command
  execution)
- Kubernetes API interactions may fail silently
- Errors need context (which screen, which resource, which operation)

**Questions:**
- Where should logs be written?
- What log levels and verbosity should be supported?
- What information should be logged?
- How to prevent logs from filling disk space?
- How to enable debug logging when needed?

## References

**Prior art:**
- **k9s**: Logs to `~/.config/k9s/k9s.log` with rotation
- **kubectl**: Uses `--v` flag for verbosity (0-10 scale), logs to
  stderr
- **Bubble Tea apps**: Typically use file-based logging or debug mode
  with log output to separate file

**Go logging libraries:**
- **log/slog** (Go 1.21+): Standard library, structured logging,
  performant
- **zerolog**: Fast structured logging, zero allocation
- **zap**: Uber's logger, fast but more complex API
- **logrus**: Popular but older, slower than slog/zerolog

## Design

### Core Principles

1. **Non-intrusive**: Logging never interferes with TUI rendering
2. **File-based**: All logs written to files, never stdout/stderr
3. **Structured**: Use structured logging for better parsing and
   filtering
4. **Configurable**: Users control verbosity via CLI flags
5. **Rotation**: Automatic log rotation to prevent disk filling
6. **Performance**: Logging overhead should be negligible (<1ms per
   call)

### Logging Library: log/slog

Use Go's standard library `log/slog` (introduced in Go 1.21):

**Rationale:**
- Part of standard library (no external dependency)
- Structured logging with key-value pairs
- Multiple handlers (JSON, text, custom)
- Context-aware logging
- Good performance (comparable to zerolog/zap)
- Simple API, easy to use correctly

**Performance:**
- ~100-200ns per log call with buffered writer
- Async writing prevents blocking TUI
- Negligible impact on application performance

### Log Location and Files

**Directory:** `~/.config/k1/logs/`

**Files:**
- `k1.log` - Current log file (rotated when full)
- `k1.log.1` - Previous log (oldest entry first)
- `k1.log.2` - Second oldest log
- Maximum 5 rotated logs kept

**Rotation policy:**
- Rotate when file exceeds 10MB
- Keep last 5 files (50MB total max)
- Delete oldest when limit reached

**Initialization:**
```go
// On startup, create log directory if needed
logDir := filepath.Join(os.UserHomeDir(), ".config", "k1", "logs")
os.MkdirAll(logDir, 0755)
```

### Log Levels

Support standard slog levels:

| Level | Value | Usage                                        |
|-------|-------|----------------------------------------------|
| DEBUG | -4    | Verbose details (API calls, state changes)   |
| INFO  | 0     | Normal operations (startup, screen switches) |
| WARN  | 4     | Recoverable issues (slow API, retries)       |
| ERROR | 8     | Errors requiring attention (API failures)    |

**Default level:** INFO (production)
**Debug level:** DEBUG (when `-debug` flag used)

### CLI Flags

```bash
# Default: INFO level logging
k1

# Enable debug logging
k1 -debug

# Disable logging entirely
k1 -no-log

# Custom log file location
k1 -log-file /tmp/k1-debug.log
```

**Flag definitions:**
```go
var (
    debugFlag  = flag.Bool("debug", false,
        "Enable debug logging")
    noLogFlag  = flag.Bool("no-log", false,
        "Disable logging entirely")
    logFileFlag = flag.String("log-file", "",
        "Custom log file path (default: ~/.config/k1/logs/k1.log)")
)
```

### Logger Initialization

```go
// In main.go or internal/logging/logger.go

package logging

import (
    "log/slog"
    "os"
    "path/filepath"
)

var Logger *slog.Logger

// InitLogger sets up the global logger
func InitLogger(debug bool, noLog bool, customPath string) error {
    if noLog {
        // Discard all logs
        Logger = slog.New(slog.NewTextHandler(
            io.Discard, nil))
        return nil
    }

    // Determine log file path
    logPath := customPath
    if logPath == "" {
        home, _ := os.UserHomeDir()
        logDir := filepath.Join(home, ".config", "k1", "logs")
        os.MkdirAll(logDir, 0755)
        logPath = filepath.Join(logDir, "k1.log")
    }

    // Open log file with append mode
    logFile, err := os.OpenFile(logPath,
        os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
    if err != nil {
        return fmt.Errorf("failed to open log file: %w", err)
    }

    // Create rotating writer
    writer := NewRotatingWriter(logFile, 10*1024*1024, 5)

    // Determine log level
    level := slog.LevelInfo
    if debug {
        level = slog.LevelDebug
    }

    // Create handler options
    opts := &slog.HandlerOptions{
        Level: level,
        AddSource: debug, // Include file:line in debug mode
    }

    // Use JSON handler for structured logs
    handler := slog.NewJSONHandler(writer, opts)

    // Set global logger
    Logger = slog.New(handler)
    slog.SetDefault(Logger)

    Logger.Info("k1 started",
        "version", Version,
        "log_level", level.String(),
    )

    return nil
}
```

### Rotating Writer

Implement custom writer with rotation logic:

```go
// In internal/logging/rotate.go

type RotatingWriter struct {
    file       *os.File
    maxSize    int64  // Max file size before rotation
    maxBackups int    // Max number of old files to keep
    mu         sync.Mutex
}

func NewRotatingWriter(
    file *os.File,
    maxSize int64,
    maxBackups int,
) *RotatingWriter {
    return &RotatingWriter{
        file:       file,
        maxSize:    maxSize,
        maxBackups: maxBackups,
    }
}

func (w *RotatingWriter) Write(p []byte) (n int, err error) {
    w.mu.Lock()
    defer w.mu.Unlock()

    // Check if rotation needed
    stat, err := w.file.Stat()
    if err == nil && stat.Size()+int64(len(p)) > w.maxSize {
        w.rotate()
    }

    return w.file.Write(p)
}

func (w *RotatingWriter) rotate() error {
    // Close current file
    w.file.Close()

    // Rotate existing backups
    basePath := w.file.Name()
    for i := w.maxBackups - 1; i >= 1; i-- {
        oldPath := fmt.Sprintf("%s.%d", basePath, i)
        newPath := fmt.Sprintf("%s.%d", basePath, i+1)
        os.Rename(oldPath, newPath)
    }

    // Move current to .1
    os.Rename(basePath, basePath+".1")

    // Open new file
    newFile, err := os.OpenFile(basePath,
        os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
    if err != nil {
        return err
    }
    w.file = newFile

    return nil
}

func (w *RotatingWriter) Close() error {
    w.mu.Lock()
    defer w.mu.Unlock()
    return w.file.Close()
}
```

### What to Log

#### Application Lifecycle

```go
// Startup
Logger.Info("k1 started",
    "version", Version,
    "kubeconfig", kubeconfigPath,
    "context", contextName,
    "theme", themeName,
)

// Shutdown
Logger.Info("k1 shutdown", "reason", "user_exit")
```

#### Kubernetes Operations

```go
// Informer sync
Logger.Info("informer syncing",
    "resource", "pods",
    "tier", "priority",
)
Logger.Info("informer synced",
    "resource", "pods",
    "duration_ms", elapsed.Milliseconds(),
    "count", len(pods),
)

// API calls (debug only)
Logger.Debug("kubernetes API call",
    "method", "GET",
    "resource", "pods",
    "namespace", namespace,
    "latency_ms", latency.Milliseconds(),
)

// Errors
Logger.Error("failed to sync informer",
    "resource", "deployments",
    "error", err.Error(),
)
```

#### Command Execution

```go
// Command start
Logger.Info("executing command",
    "command", "/scale",
    "resource_type", "deployment",
    "resource_name", "nginx",
    "args", args,
)

// Command success
Logger.Info("command completed",
    "command", "/scale",
    "duration_ms", elapsed.Milliseconds(),
)

// Command error
Logger.Error("command failed",
    "command", "/delete",
    "resource", "pod/nginx",
    "error", err.Error(),
)
```

#### Performance Metrics

```go
// Screen render time (debug only)
Logger.Debug("screen rendered",
    "screen", "pods",
    "duration_ms", elapsed.Milliseconds(),
    "rows", rowCount,
)

// Search/filter performance
Logger.Debug("filter applied",
    "screen", "pods",
    "query", filterText,
    "matches", matchCount,
    "duration_ms", elapsed.Milliseconds(),
)
```

#### User Actions

```go
// Screen navigation
Logger.Info("screen changed",
    "from", "pods",
    "to", "deployments",
)

// Filter changes
Logger.Debug("filter updated",
    "screen", "pods",
    "filter", filterText,
    "matches", matchCount,
)
```

#### Errors and Warnings

```go
// Recoverable errors (warnings)
Logger.Warn("slow API response",
    "operation", "ListPods",
    "latency_ms", latency.Milliseconds(),
    "threshold_ms", 1000,
)

// Application errors
Logger.Error("failed to create repository",
    "error", err.Error(),
    "kubeconfig", kubeconfigPath,
)

// Panics (via recover)
defer func() {
    if r := recover(); r != nil {
        Logger.Error("panic recovered",
            "panic", r,
            "stack", string(debug.Stack()),
        )
    }
}()
```

### Context-Aware Logging

Use context to pass logger with additional attributes:

```go
// Add screen context
ctx := context.WithValue(ctx, "screen", "pods")
Logger.InfoContext(ctx, "screen loaded",
    "row_count", len(pods),
)

// Add request ID for tracing
ctx = context.WithValue(ctx, "request_id", uuid.New())
Logger.DebugContext(ctx, "processing request")
```

### Testing and Development

**Development mode:**
```bash
# Run with debug logging and custom path
k1 -debug -log-file ./k1-dev.log
```

**Watch logs in real-time:**
```bash
tail -f ~/.config/k1/logs/k1.log
```

**Parse JSON logs:**
```bash
# Filter errors only
jq 'select(.level == "ERROR")' ~/.config/k1/logs/k1.log

# Show command execution times
jq 'select(.msg == "command completed") |
    {command, duration_ms}' ~/.config/k1/logs/k1.log

# Count log levels
jq -r '.level' ~/.config/k1/logs/k1.log | sort | uniq -c
```

### Error Reporting to User

**Principle:** Logs are for developers, not end users.

When errors occur:
1. Log full details (error, context, stack trace)
2. Show user-friendly message in TUI (command bar result state)
3. Suggest checking logs for details

**Example:**
```go
// In command execution
if err != nil {
    Logger.Error("command failed",
        "command", cmd.Name,
        "resource", resourceName,
        "error", err.Error(),
        "context", ctx,
    )

    return types.ErrorMsg{
        Title: "Command Failed",
        Message: "Failed to scale deployment. " +
            "Check logs at ~/.config/k1/logs/k1.log",
        Details: err.Error(), // Brief error for UI
    }
}
```

## Consequences

### Positive

- **Non-intrusive:** Logging never interferes with TUI rendering
- **Structured:** JSON logs easy to parse, filter, analyze
- **Configurable:** Users control verbosity via `-debug` flag
- **Rotation:** Automatic cleanup prevents disk filling (max 50MB)
- **Performance:** slog is fast, negligible overhead (<1ms)
- **Standard:** Using stdlib reduces dependencies
- **Troubleshooting:** Rich context for debugging issues
- **Metrics:** Performance data available for optimization
- **Production-ready:** Sensible defaults (INFO level, auto-rotation)

### Negative

- **File management:** Users need to know where logs are stored
- **Disk usage:** Up to 50MB of logs (acceptable trade-off)
- **No real-time UI:** Cannot show logs in TUI without complexity
- **JSON verbosity:** JSON logs are less readable than text (but
  parseable)
- **Setup complexity:** Rotating writer adds code vs simple file write

### Neutral

- **Log location:** `~/.config/k1/logs/` follows XDG spec, may need
  documentation
- **Debug flag:** Users need to enable debug mode for verbose logs
- **Retention:** 5 files (50MB) may be too much or too little depending
  on usage

## Alternatives Considered

### Alternative 1: Log to stderr

**Approach:** Use stderr for logging, redirect in TUI mode.

**Pros:**
- Simple, no file management
- Standard Unix pattern

**Cons:**
- Requires shell redirection (`k1 2>log.txt`)
- Interferes with TUI if not redirected
- Lost on exit if not captured
- No rotation

**Rejected:** Too error-prone for TUI applications.

### Alternative 2: Separate debug binary

**Approach:** Build two binaries: `k1` (no logging) and `k1-debug`
(verbose).

**Pros:**
- Zero overhead in production
- Clear separation

**Cons:**
- Maintenance burden (two builds)
- Cannot enable debug on-demand
- Users need to know about debug binary

**Rejected:** Flag-based control is simpler and more flexible.

### Alternative 3: Remote logging (syslog, telemetry)

**Approach:** Send logs to external service (syslog, cloud logging).

**Pros:**
- Centralized logging
- No local disk usage
- Can aggregate across users

**Cons:**
- Privacy concerns
- Network dependency
- Setup complexity
- Overkill for single-user TUI app

**Rejected:** Local file logging is sufficient. Remote logging can be
added later if needed.

### Alternative 4: Text logs instead of JSON

**Approach:** Use `slog.NewTextHandler` for human-readable logs.

**Pros:**
- Easier to read with `tail -f`
- Simpler format

**Cons:**
- Harder to parse programmatically
- No structured querying (jq, etc.)
- Less information density

**Rejected:** JSON's parsability outweighs readability. Users can use
`jq` for formatting if needed.

### Alternative 5: Embedded log viewer in TUI

**Approach:** Add `:logs` screen to view logs in TUI.

**Pros:**
- No need to open terminal/editor
- Integrated experience

**Cons:**
- Complex UI (log viewer with filtering, search)
- Logs are for troubleshooting crashes (TUI may not be running)
- Debugging TUI rendering requires external logs

**Deferred:** Start with file logging, consider log viewer later if
users request it.

## Future Enhancements

1. **Log viewer screen:** Add `:logs` navigation command to view logs
   in TUI
2. **Export diagnostics:** Single command to collect logs + system info
   for bug reports
3. **Performance profiling:** Add pprof integration for CPU/memory
   profiling
4. **Remote logging:** Optional telemetry for aggregate error tracking
5. **Log compression:** Gzip old log files to save space
6. **Custom formatters:** Allow plugins to format logs (e.g., colored
   text for development)

## References

- [Go log/slog documentation](https://pkg.go.dev/log/slog)
- [Bubble Tea logging best practices](
  https://github.com/charmbracelet/bubbletea/tree/master/examples)
- k9s logging implementation:
  `~/.config/k9s/k9s.log`
- XDG Base Directory Specification:
  `~/.config/` for user configuration
