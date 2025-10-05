# Prometheus Metrics Instrumentation for k1

| Metadata | Value                   |
|----------|-------------------------|
| Date     | 2025-10-05              |
| Author   | @renato0307             |
| Status   | Proposed                |
| Tags     | metrics, observability  |

| Revision | Date       | Author      | Info           |
|----------|------------|-------------|----------------|
| 1        | 2025-10-05 | @renato0307 | Initial design |

## Context and Problem Statement

k1 is a high-performance TUI for Kubernetes cluster management. To ensure
it delivers on its "supersonic" promise and to understand user behavior,
we need observability into performance characteristics and operation
execution patterns.

Key questions:
- How long do Kubernetes informers take to sync?
- What is the query latency from informer caches?
- How long does rendering take for different screen sizes?
- Which screens and commands are used most frequently?
- Are there performance regressions over time?

We need to expose these metrics in a way that can be collected by
Prometheus for monitoring, alerting, and analysis.

## Prior Work

Prometheus is the de-facto standard for metrics in the Kubernetes
ecosystem. Three common patterns exist:

1. **HTTP /metrics endpoint (pull)**: Prometheus scrapes metrics
   - Standard for long-running services
   - Requires network port and service discovery

2. **Push Gateway (push)**: Application pushes metrics to gateway
   - Designed for short-lived batch jobs
   - Adds operational complexity (gateway deployment)

3. **Textfile collector (file-based)**: Write metrics to file, Node
   Exporter reads them
   - Zero network overhead
   - Simple file I/O operations
   - Standard pattern for node-level metrics

## Design

### Metrics to Track

**Performance Metrics** (Histograms):
```
k1_informer_sync_duration_seconds{resource="pods"}
k1_informer_query_duration_seconds{resource="pods",operation="list"}
k1_screen_render_duration_seconds{screen="pods"}
k1_filter_operation_duration_seconds{type="fuzzy"}
k1_command_execution_duration_seconds{command="describe"}
```

**Operation Counters**:
```
k1_screen_views_total{screen="pods"}
k1_commands_executed_total{command="describe"}
k1_filter_operations_total{type="fuzzy"}
k1_refresh_operations_total{screen="pods"}
k1_errors_total{type="informer_sync",resource="pods"}
```

**Resource Metrics** (Gauges):
```
k1_informer_objects_cached{resource="pods"}
k1_terminal_width
k1_terminal_height
```

### Architecture: Three-Approach Evaluation

#### Option 1: HTTP /metrics Endpoint (Pull)

**Implementation**:
```go
// Start metrics server in goroutine
import "github.com/prometheus/client_golang/prometheus/promhttp"

func startMetricsServer(port int) {
    http.Handle("/metrics", promhttp.Handler())
    go http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
}
```

**Pros**:
- Standard Prometheus pattern
- Rich client library (`prometheus/client_golang`)
- Automatic aggregation and cardinality management
- Works well for long-running processes

**Cons**:
- Requires network port (conflicts, permissions)
- Requires service discovery (how does Prometheus find k1 instances?)
- Not suitable for ephemeral desktop TUIs
- Port conflicts if multiple users run k1 on same machine

**Use Case**: Better for server-side components, not desktop TUIs

#### Option 2: Push Gateway (Push)

**Implementation**:
```go
import "github.com/prometheus/client_golang/prometheus/push"

pusher := push.New(gatewayURL, "k1").
    Collector(metricCollector)
pusher.Push() // periodically or on exit
```

**Pros**:
- Works for short-lived processes
- No port management needed
- Centralized collection point

**Cons**:
- Requires Push Gateway deployment and management
- Network dependency (TUI may run offline)
- Gateway becomes SPOF
- Timestamp semantics: push time vs metric time
- Gateway persistence: metrics survive job restarts (good/bad?)

**Use Case**: Designed for batch jobs, CI/CD pipelines - overkill for k1

#### Option 3: Textfile Collector (File-based)

**Implementation**:
```go
// Write metrics to file in Prometheus text format
func writeMetrics(path string, metrics map[string]interface{}) {
    f, _ := os.Create(path)
    defer f.Close()

    // Format: metric_name{labels} value timestamp
    fmt.Fprintf(f, "# HELP k1_screen_views_total Screen view count\n")
    fmt.Fprintf(f, "# TYPE k1_screen_views_total counter\n")
    fmt.Fprintf(f, "k1_screen_views_total{screen=\"pods\"} %d %d\n",
        count, time.Now().UnixMilli())
}
```

**Collection via Node Exporter**:
```yaml
# Node exporter reads from --collector.textfile.directory
# Files must end in .prom
/var/lib/node_exporter/textfile_collector/k1.prom
```

**Pros**:
- Zero network overhead
- No port conflicts
- Works offline (write locally, sync later)
- Simple implementation (just file I/O)
- Standard pattern for node-level metrics
- No external dependencies (Push Gateway)
- Can write metrics on k1 exit (graceful shutdown)

**Cons**:
- Requires Node Exporter deployment
- File path coordination (where to write?)
- Stale metrics if k1 crashes without cleanup
- Manual metric lifetime management

**Use Case**: Perfect for desktop tools, system utilities, node-level
metrics

### Recommended Approach

**Primary: Textfile Collector**
- Best fit for TUI application lifecycle
- Matches k1's usage pattern (user-initiated, terminal-bound)
- Minimal operational overhead
- Standard practice for node-level metrics

**Secondary: HTTP Endpoint (Optional)**
- Enable via flag `--metrics-port 9090` for development/debugging
- Disabled by default (avoid port conflicts)
- Useful for live inspection during development

**Not Recommended: Push Gateway**
- Operational complexity doesn't match benefit
- Network dependency contradicts TUI simplicity
- Better suited for batch jobs

### Implementation Details

#### Metric Collection

```go
// internal/metrics/metrics.go
package metrics

import (
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
)

var (
    InformerSyncDuration = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "k1_informer_sync_duration_seconds",
            Help: "Informer initial sync duration",
            Buckets: []float64{.1, .5, 1, 2, 5, 10},
        },
        []string{"resource"},
    )

    ScreenViews = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "k1_screen_views_total",
            Help: "Total screen view count",
        },
        []string{"screen"},
    )
)

// Usage in code:
timer := prometheus.NewTimer(InformerSyncDuration.WithLabelValues("pods"))
defer timer.ObserveDuration()
```

#### File Writer

```go
// internal/metrics/writer.go
type FileWriter struct {
    path     string
    registry *prometheus.Registry
}

func (w *FileWriter) Write() error {
    metrics, _ := w.registry.Gather()

    f, err := os.Create(w.path + ".tmp")
    if err != nil {
        return err
    }
    defer f.Close()

    // Write Prometheus text format
    for _, mf := range metrics {
        expfmt.MetricFamilyToText(f, mf)
    }

    // Atomic rename
    return os.Rename(w.path+".tmp", w.path)
}
```

#### Integration Points

1. **Informer sync** (internal/k8s/repository.go):
   ```go
   timer := prometheus.NewTimer(metrics.InformerSyncDuration.WithLabelValues(resource))
   cache.WaitForCacheSync(...)
   timer.ObserveDuration()
   ```

2. **Screen rendering** (internal/screens/*.go):
   ```go
   func (m Model) View() string {
       start := time.Now()
       defer func() {
           metrics.RenderDuration.WithLabelValues(m.ID()).Observe(
               time.Since(start).Seconds())
       }()
       // ... render logic
   }
   ```

3. **Command execution** (internal/components/commandbar.go):
   ```go
   metrics.CommandsExecuted.WithLabelValues(cmd.Name).Inc()
   ```

4. **Graceful shutdown** (cmd/k1/main.go):
   ```go
   defer func() {
       if metricsWriter != nil {
           metricsWriter.Write()
       }
   }()
   ```

### Configuration

```go
// Flags
var (
    metricsFile = flag.String("metrics-file",
        "~/.k1/metrics.prom",
        "Path to write Prometheus metrics")
    metricsPort = flag.Int("metrics-port", 0,
        "HTTP port for /metrics endpoint (0=disabled)")
    metricsInterval = flag.Duration("metrics-interval", 30*time.Second,
        "Interval to write metrics file")
)
```

### Node Exporter Setup (User Documentation)

```bash
# Install Node Exporter
brew install node_exporter  # macOS
apt install prometheus-node-exporter  # Linux

# Configure textfile directory
node_exporter --collector.textfile.directory=/var/lib/node_exporter

# Configure k1 to write to that directory
k1 --metrics-file=/var/lib/node_exporter/k1.prom
```

## Decision

**Primary**: Implement textfile collector pattern with file-based metrics
- Write metrics to `~/.k1/metrics.prom` every 30 seconds and on exit
- Use Prometheus client library for metric collection
- Atomic file writes (tmp + rename) to prevent partial reads
- Document Node Exporter setup for users

**Optional**: HTTP endpoint for development
- Enable via `--metrics-port` flag
- Disabled by default
- Share same metric registry as file writer

**Not Implementing**: Push Gateway integration
- Operational complexity outweighs benefits for TUI application
- Can be added later if deployment patterns change

## Consequences

**Positive**:
- Visibility into performance characteristics (informer sync, query
  latency, render time)
- Data-driven optimization decisions (which operations are slow?)
- Usage analytics (which screens/commands are popular?)
- Regression detection in CI/CD (compare metrics across versions)
- Standard Prometheus integration (no custom tooling)

**Negative**:
- File I/O overhead (mitigated by 30s write interval)
- Disk space usage (metrics file ~10-50KB)
- Requires Node Exporter deployment for collection
- Metrics may be stale if k1 crashes (no graceful shutdown)

**Operational**:
- Users who want metrics must run Node Exporter
- CI/CD can enable HTTP endpoint for testing (`--metrics-port`)
- File path must be writable (default: ~/.k1/ directory)

**Performance Impact**:
- Metric collection: negligible (in-memory counters/histograms)
- File write: ~1-5ms every 30 seconds (non-blocking)
- HTTP endpoint: only if explicitly enabled

**Migration Path**:
- Phase 1: Implement file writer + core metrics
- Phase 2: Add optional HTTP endpoint for development
- Phase 3: Evaluate Push Gateway if deployment patterns change (e.g.,
  k1-as-a-service)
