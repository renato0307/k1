# Prometheus Metrics Collection and Remote Push for k1

| Metadata | Value                                |
|----------|--------------------------------------|
| Date     | 2025-10-06                           |
| Author   | @renato0307                          |
| Status   | Proposed                             |
| Tags     | metrics, observability, telemetry    |

| Revision | Date       | Author      | Info           |
|----------|------------|-------------|----------------|
| 1        | 2025-10-06 | @renato0307 | Initial design |

## Context and Problem Statement

k1 is a high-performance TUI for Kubernetes cluster management. To ensure
it delivers on its "supersonic" promise and to understand user behavior,
we need observability into performance characteristics and operation
execution patterns.

Use cases requiring metrics collection:

1. **Development/debugging**: Analyze performance trends over time during
   local development sessions
2. **Performance benchmarking**: Compare metrics across different runs,
   configurations, or code versions
3. **Production analytics** (opt-in): Understand usage patterns from real
   users to guide feature prioritization
4. **Aggregate monitoring**: Collect metrics from multiple k1 instances to
   identify common performance issues

These use cases require:
- Days/weeks of historical data retention
- On-demand push to remote systems (Push Gateway, Grafana Cloud)
- Privacy-conscious opt-in model (explicit user consent)
- Ephemeral buffer semantics (delete after successful push)

## Prior Work

### Time-Series Storage Formats

**Prometheus Text Format**:
```
# HELP k1_screen_views_total Screen view count
# TYPE k1_screen_views_total counter
k1_screen_views_total{screen="pods"} 42 1728201234567
k1_screen_views_total{screen="deployments"} 18 1728201234567
```
- Human-readable, easy to debug
- Append-friendly (write new lines with timestamps)
- Supported by Push Gateway and Grafana Cloud

**JSON Lines (JSONL)**:
```json
{"metric":"k1_screen_views_total","labels":{"screen":"pods"},"value":42,"ts":1728201234567}
```
- Structured, easy to parse
- Better for custom tooling
- Not directly compatible with Prometheus ecosystem

**Protobuf** (Prometheus Remote Write):
```
Binary format used by Prometheus remote_write protocol
```
- Most compact format (~50% size reduction)
- Requires encoding/decoding libraries
- Standard for Prometheus remote_write API

### Push Protocols

**Push Gateway HTTP API**:
```bash
echo "k1_metric 42" | curl --data-binary @- \
  http://pushgateway:9091/metrics/job/k1/instance/user123
```
- Simple HTTP POST
- Accepts Prometheus text format
- Idempotent (same labels overwrite)

**Grafana Cloud API** (and Self-Hosted Grafana):
```bash
curl -u $GRAFANA_USER:$GRAFANA_TOKEN \
  -H "Content-Type: application/x-protobuf" \
  --data-binary @metrics.pb \
  https://prometheus-us-central1.grafana.net/api/prom/push
```
- Uses Prometheus remote_write protocol (protobuf)
- Requires authentication (API key or basic auth)
- Batching support for efficiency
- Self-hosted Grafana exposes same `/api/prom/push` endpoint

**AWS CloudWatch**:
```go
cloudwatch.PutMetricData(&cloudwatch.PutMetricDataInput{
    Namespace: "k1",
    MetricData: [...],
})
```
- Custom API (not Prometheus-compatible)
- Different metric model (dimensions vs labels)
- Not recommended (adds AWS SDK dependency)

## Design

### Architecture Overview

k1 collects metrics to local time-series files and supports on-demand
push to remote destinations:

```
┌─────────────────────────────────────────────────────┐
│ k1 Process                                          │
│                                                     │
│  ┌─────────────────────────────────────────────┐  │
│  │ Prometheus Metrics Registry                 │  │
│  │ (in-memory counters, histograms, gauges)    │  │
│  └────────────┬────────────────────────────────┘  │
│               │                                    │
│               ▼                                    │
│  ┌────────────────────────────────────────────┐  │
│  │ Time-Series Writer                         │  │
│  │                                            │  │
│  │ Every 5 minutes:                           │  │
│  │ ~/.k1/metrics/2025-10-06.prom              │  │
│  └────────────┬───────────────────────────────┘  │
│               │                                    │
└───────────────┼────────────────────────────────────┘
                │
       ┌────────▼─────────┐
       │ k1 upload-metrics│
       │ --to=grafana     │
       └────────┬─────────┘
                │
┌───────────────┼───────────────────┐
│               │                   │
▼               ▼                   ▼
┌──────────┐  ┌────────┐  ┌────────────────┐
│ Push     │  │Grafana │  │ Grafana Cloud  │
│ Gateway  │  │ (self) │  │                │
└──────────┘  └────────┘  └────────────────┘
```

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

### File Storage Strategy

**Directory Structure**:
```
~/.k1/
  ├── config.yaml                (configuration)
  └── metrics/
      ├── 2025-10-06.prom        (time-series: append-only)
      ├── 2025-10-05.prom
      ├── 2025-10-04.prom
      └── .uploaded              (tracks push status)
```

**File Format**: Prometheus text format with millisecond timestamps
```
# Written: 2025-10-06T14:30:00Z
k1_screen_views_total{screen="pods",user="anonymized"} 5 1728218400000
k1_informer_sync_duration_seconds_bucket{resource="pods",le="0.5"} 10 1728218400000
k1_informer_sync_duration_seconds_bucket{resource="pods",le="1.0"} 15 1728218400000
k1_informer_sync_duration_seconds_sum{resource="pods"} 8.5 1728218400000
k1_informer_sync_duration_seconds_count{resource="pods"} 15 1728218400000
```

**Rotation Strategy**:
- Daily file rotation (midnight local time)
- Append-only within each day
- Write every 5 minutes (balance between data loss and I/O overhead)
- Atomic writes with temp file + rename

**Retention Policy**:
```yaml
# ~/.k1/config.yaml
metrics:
  enabled: false               # Opt-in (privacy-first)
  retention_days: 7            # Keep 7 days locally
  delete_after_push: true      # Ephemeral buffer (delete on success)
  write_interval: 5m           # Write to disk every 5 minutes
```

### Privacy and Opt-In Model

**Key Principles**:
1. **Disabled by default**: No metrics collection without explicit opt-in
2. **Clear documentation**: Explain what data is collected and why
3. **User anonymization**: Hash user identifiers (hostname → SHA256)
4. **No sensitive data**: Never collect resource names, namespaces, or
   cluster identifiers

**Labels to Include**:
```
user: <sha256-of-hostname>       # Anonymized user identifier
version: v0.1.0                  # k1 version for correlation
os: darwin                       # Operating system
arch: arm64                      # Architecture
```

**Labels to EXCLUDE**:
```
❌ cluster_name                  # Sensitive cluster identifier
❌ namespace                     # May contain sensitive names
❌ resource_name                 # May contain sensitive names
❌ node_name                     # Infrastructure details
❌ context                       # Kubeconfig context name
```

**Opt-In Flow**:
```bash
# First run (metrics disabled by default)
$ k1
# ... TUI runs normally, no metrics collected

# Enable via config
$ cat ~/.k1/config.yaml
metrics:
  enabled: true                  # User explicitly enables
  destinations:
    - name: grafana
      type: grafana-cloud
      url: https://prometheus-us-central1.grafana.net/api/prom/push
      auth:
        username: 123456
        password_cmd: "pass show k1/grafana-token"

# Or enable via flag (one-time sessions)
$ k1 --enable-metrics-collection
```

### Upload Command

**CLI Interface**:
```bash
# Upload to default destination (from config)
k1 upload-metrics

# Upload to specific destination
k1 upload-metrics --to=grafana

# Upload specific date range
k1 upload-metrics --from=2025-10-01 --to=2025-10-06

# Dry-run (show what would be uploaded)
k1 upload-metrics --dry-run

# Force upload (even if already uploaded)
k1 upload-metrics --force

# Upload and delete local files
k1 upload-metrics --cleanup
```

**Upload Tracking**:
```
~/.k1/metrics/.uploaded (JSON)
{
  "2025-10-06.prom": {
    "destinations": {
      "grafana": {
        "uploaded_at": "2025-10-06T20:00:00Z",
        "status": "success",
        "metrics_count": 1234
      }
    }
  }
}
```

### Destination Configurations

**Push Gateway** (self-hosted or remote):
```yaml
destinations:
  - name: company-pushgateway
    type: pushgateway
    url: https://pushgateway.example.com
    auth:
      type: basic
      username: k1-user
      password_cmd: "pass show k1/pushgateway"
    job_name: k1
    grouping_keys:
      user: "${USER_HASH}"      # Anonymized user
      version: "${K1_VERSION}"
```

**Grafana Cloud**:
```yaml
destinations:
  - name: grafana-cloud
    type: grafana-cloud
    url: https://prometheus-us-central1.grafana.net/api/prom/push
    auth:
      type: basic
      username: "${GRAFANA_USER}"
      password_cmd: "pass show k1/grafana-token"
    # Uses Prometheus remote_write protocol (protobuf)
```

**Self-Hosted Grafana** (via Prometheus remote_write):
```yaml
destinations:
  - name: grafana-self
    type: grafana-cloud              # Same type, different URL
    url: https://grafana.company.com/api/prom/push
    auth:
      type: basic                    # Or bearer token
      username: k1-metrics
      password_cmd: "pass show k1/grafana-self"
    # Self-hosted Grafana uses same protocol as Cloud
```

**Custom HTTP Endpoint**:
```yaml
destinations:
  - name: custom
    type: http
    url: https://metrics.example.com/ingest
    auth:
      type: bearer
      token_cmd: "aws sts get-session-token --query 'Credentials.SessionToken' --output text"
    format: prometheus-text      # or 'json'
    headers:
      X-Source: k1
```

### Implementation Details

**Time-Series Writer** (internal/metrics/tswriter.go):
```go
package metrics

import (
    "os"
    "path/filepath"
    "time"
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/common/expfmt"
)

type TSWriter struct {
    dir      string
    registry *prometheus.Registry
    interval time.Duration
    enabled  bool
}

func (w *TSWriter) Start(ctx context.Context) error {
    if !w.enabled {
        return nil
    }

    ticker := time.NewTicker(w.interval)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            w.flush() // Write final metrics before exit
            return nil
        case <-ticker.C:
            if err := w.flush(); err != nil {
                // Log error but don't fail (metrics are non-critical)
                log.Warn("Failed to write metrics", "error", err)
            }
        }
    }
}

func (w *TSWriter) flush() error {
    // Daily file: ~/.k1/metrics/2025-10-06.prom
    filename := filepath.Join(w.dir,
        time.Now().Format("2006-01-02")+".prom")

    // Atomic write: temp file + rename
    tmpFile := filename + ".tmp"
    f, err := os.OpenFile(tmpFile,
        os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
    if err != nil {
        return err
    }
    defer f.Close()

    // Gather metrics from registry
    metrics, err := w.registry.Gather()
    if err != nil {
        return err
    }

    // Write in Prometheus text format with timestamps
    for _, mf := range metrics {
        if _, err := expfmt.MetricFamilyToText(f, mf); err != nil {
            return err
        }
    }

    // Atomic rename
    return os.Rename(tmpFile, filename)
}
```

**Upload Command** (internal/metrics/upload.go):
```go
package metrics

import (
    "bytes"
    "fmt"
    "io"
    "net/http"
    "os"
    "path/filepath"
)

type Uploader struct {
    config *Config
}

func (u *Uploader) Upload(destination string, opts UploadOpts) error {
    dest := u.config.GetDestination(destination)
    if dest == nil {
        return fmt.Errorf("destination %q not found", destination)
    }

    files, err := u.getFilesToUpload(opts)
    if err != nil {
        return err
    }

    for _, file := range files {
        if err := u.uploadFile(dest, file, opts); err != nil {
            return fmt.Errorf("upload %s: %w", file, err)
        }
    }

    if opts.Cleanup {
        return u.cleanupFiles(files)
    }

    return nil
}

func (u *Uploader) uploadFile(dest *Destination, file string,
    opts UploadOpts) error {

    data, err := os.ReadFile(file)
    if err != nil {
        return err
    }

    switch dest.Type {
    case "pushgateway":
        return u.uploadToPushGateway(dest, data)
    case "grafana-cloud":
        return u.uploadToGrafanaCloud(dest, data)
    case "http":
        return u.uploadToHTTP(dest, data)
    default:
        return fmt.Errorf("unknown destination type: %s", dest.Type)
    }
}

func (u *Uploader) uploadToPushGateway(dest *Destination,
    data []byte) error {

    // Push Gateway expects: POST /metrics/job/<job>/instance/<instance>
    url := fmt.Sprintf("%s/metrics/job/%s/instance/%s",
        dest.URL, dest.JobName, dest.GroupingKeys["user"])

    req, err := http.NewRequest("POST", url, bytes.NewReader(data))
    if err != nil {
        return err
    }

    if dest.Auth.Type == "basic" {
        req.SetBasicAuth(dest.Auth.Username, dest.Auth.Password)
    }

    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        body, _ := io.ReadAll(resp.Body)
        return fmt.Errorf("push failed: %s (status %d)",
            body, resp.StatusCode)
    }

    return nil
}
```

**Rotation and Cleanup** (internal/metrics/rotation.go):
```go
package metrics

import (
    "os"
    "path/filepath"
    "time"
)

func (w *TSWriter) cleanupOldFiles() error {
    if w.retentionDays <= 0 {
        return nil
    }

    cutoff := time.Now().AddDate(0, 0, -w.retentionDays)

    return filepath.Walk(w.dir, func(path string, info os.FileInfo,
        err error) error {

        if err != nil {
            return err
        }

        if info.IsDir() {
            return nil
        }

        // Skip non-.prom files
        if filepath.Ext(path) != ".prom" {
            return nil
        }

        // Parse date from filename: 2025-10-06.prom
        basename := filepath.Base(path)
        date, err := time.Parse("2006-01-02.prom", basename)
        if err != nil {
            return nil // Skip files with non-standard names
        }

        if date.Before(cutoff) {
            if err := os.Remove(path); err != nil {
                return fmt.Errorf("remove %s: %w", path, err)
            }
        }

        return nil
    })
}
```

### Configuration Schema

**Full Example** (~/.k1/config.yaml):
```yaml
metrics:
  enabled: false                 # Opt-in (privacy-first)
  dir: ~/.k1/metrics
  retention_days: 7
  delete_after_push: true
  write_interval: 5m

  # Privacy settings
  anonymize_user: true           # Hash hostname with SHA256
  exclude_cluster_info: true     # Don't include cluster/context names

  # Destinations
  destinations:
    - name: grafana-cloud
      type: grafana-cloud
      url: https://prometheus-us-central1.grafana.net/api/prom/push
      auth:
        type: basic
        username: "123456"
        password_cmd: "pass show k1/grafana-token"

    - name: grafana-self
      type: grafana-cloud          # Same protocol as cloud
      url: https://grafana.company.com/api/prom/push
      auth:
        type: basic
        username: k1-metrics
        password_cmd: "pass show k1/grafana-self"

    - name: company-gateway
      type: pushgateway
      url: https://pushgateway.company.com
      auth:
        type: basic
        username: k1-metrics
        password_cmd: "pass show k1/company-gateway"
      job_name: k1
      grouping_keys:
        team: platform
```

### Integration with Existing Code

**Main Application** (cmd/k1/main.go):
```go
func main() {
    // ... flag parsing, config loading

    // Initialize metrics system
    registry := prometheus.NewRegistry()

    // Time-series writer (opt-in)
    if cfg.Metrics.Enabled {
        tsWriter := metrics.NewTSWriter(
            cfg.Metrics.Dir,
            registry,
            cfg.Metrics.WriteInterval,
        )
        go tsWriter.Start(ctx)

        // Cleanup old files daily
        go tsWriter.StartCleanupRoutine(ctx, 24*time.Hour)
    }

    // ... rest of application
}
```

**Upload Subcommand** (cmd/k1/main.go):
```go
func main() {
    if len(os.Args) > 1 && os.Args[1] == "upload-metrics" {
        handleUploadMetrics()
        return
    }

    // ... normal TUI flow
}

func handleUploadMetrics() {
    // Parse flags: --to, --from, --to-date, --dry-run, --force, --cleanup
    opts := parseUploadFlags()

    cfg, err := config.Load()
    if err != nil {
        fatal(err)
    }

    uploader := metrics.NewUploader(cfg)
    if err := uploader.Upload(opts.Destination, opts); err != nil {
        fatal(err)
    }

    fmt.Println("✓ Metrics uploaded successfully")
}
```

**Metric Collection Points** (examples):

1. **Informer sync** (internal/k8s/repository.go):
   ```go
   timer := prometheus.NewTimer(
       metrics.InformerSyncDuration.WithLabelValues(resource))
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

## Decision

Implement **historical metrics collection** with on-demand remote push:

- Time-series metrics stored locally in daily files
- **Disabled by default** (opt-in via config or flag)
- Write to `~/.k1/metrics/YYYY-MM-DD.prom` every 5 minutes
- Daily file rotation with configurable retention (7 days default)
- Upload command: `k1 upload-metrics --to=<destination>`
- Support Push Gateway, Grafana Cloud, and self-hosted Grafana
- Privacy-first: anonymize user data, exclude cluster info
- Ephemeral buffer: delete after successful push (optional)

**File Format**: Prometheus text format (human-readable, ecosystem-
compatible)

**Privacy Model**: Explicit opt-in with clear documentation about
collected data

**Retention**: 7 days default, delete after successful push

## Consequences

### Positive

**Development Experience**:
- Analyze performance trends over development sessions
- Compare before/after metrics when testing optimizations
- Debug performance regressions with historical context

**Performance Benchmarking**:
- Compare metrics across different runs
- Track performance improvements over time
- Identify regressions early

**Production Analytics** (opt-in):
- Understand real-world usage patterns (which screens, commands used)
- Identify common performance bottlenecks across users
- Guide feature prioritization based on actual usage data

**Privacy-First**:
- Disabled by default (no surprise telemetry)
- Clear documentation of collected data
- User anonymization (no PII)
- No sensitive cluster information

**Flexibility**:
- Multiple destination support (Push Gateway, Grafana Cloud, self-
  hosted Grafana, custom HTTP)
- Configurable retention and cleanup policies
- On-demand push (user controls when/where data goes)

### Negative

**Disk Space Usage**:
- ~1-5MB per day of metrics (depends on activity level)
- 7-day retention = ~7-35MB disk usage
- Mitigated by: configurable retention, auto-cleanup, ephemeral buffer

**Implementation Complexity**:
- Upload command with multi-destination support
- Configuration validation and auth credential handling
- File rotation and cleanup logic
- Upload tracking state management

**User Education**:
- Configuration complexity (destinations, auth, privacy settings)
- Must understand opt-in model and implications
- Need clear documentation of collected metrics

**Privacy Concerns**:
- Even anonymized metrics may be sensitive in some environments
- Users must trust that we're not collecting sensitive data
- Opt-in model requires proactive user action (low adoption?)

### Operational

**Development Workflow**:
```bash
# Day-to-day development (metrics disabled)
k1

# Benchmark testing (enable metrics)
k1 --enable-metrics-collection
# ... run benchmark tests
k1 upload-metrics --to=grafana --cleanup
```

**Production Monitoring** (opt-in users):
```bash
# Configure once
cat > ~/.k1/config.yaml <<EOF
metrics:
  enabled: true
  destinations:
    - name: company
      type: pushgateway
      url: https://pushgateway.company.com
EOF

# Normal usage (metrics collected in background)
k1

# Upload weekly (cron job or manual)
k1 upload-metrics --to=company --cleanup
```

**Disk Management**:
- Auto-cleanup after 7 days (configurable)
- Delete after push (ephemeral buffer mode)
- Warning if disk space low (future enhancement)

### Performance Impact

**Time-Series Writer**:
- 5-20ms every 5 minutes (append to file)
- Additional 1-2ms per metric family
- Negligible CPU/memory overhead for TUI application

**Upload Command**:
- Run on-demand (zero runtime overhead)
- Network-bound (depends on file size and bandwidth)
- Typical: 1-10MB upload in 1-5 seconds

### Migration Path

**Phase 1**: Implement time-series writer
- File rotation, retention, cleanup
- In-memory metrics registry

**Phase 2**: Implement Push Gateway destination
- Basic HTTP POST with auth
- Upload command skeleton

**Phase 3**: Add Grafana Cloud support
- Protobuf encoding (remote_write protocol)
- Auth credential management

**Phase 4**: Privacy and opt-in UX
- Configuration validation
- Clear documentation
- Opt-in prompts (future: first-run wizard)

**Phase 5**: Custom HTTP destinations
- Pluggable destination system
- JSON format support

**Future**: Auto-upload option
- Background upload (instead of manual command)
- Requires careful privacy considerations
