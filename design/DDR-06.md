# Resource Detail Commands: Describe and YAML

| Metadata | Value                                       |
|----------|---------------------------------------------|
| Date     | 2025-10-04                                  |
| Author   | @renato0307                                 |
| Status   | Implemented                                 |
| Tags     | commands, describe, yaml, kubectl, informer |

| Revision | Date       | Author      | Info                                           |
|----------|------------|-------------|------------------------------------------------|
| 1        | 2025-10-04 | @renato0307 | Initial design                                 |
| 2        | 2025-10-05 | @renato0307 | Use kubectl library for both yaml/describe    |
| 3        | 2025-10-05 | @renato0307 | Use on-demand event fetching (no informer)    |

## Context and Problem Statement

K1 needs to implement `/describe` and `/yaml` commands to show
detailed resource information. These commands are essential for
debugging and inspecting Kubernetes resources. The key challenge is
achieving fast performance (<10ms) while maintaining output parity with
kubectl.

**Key Questions:**
- Should we shell out to kubectl or use Go libraries?
- How can we leverage informer cache data for speed?
- What's the trade-off between implementation effort and performance?
- Is exact kubectl output parity required?

## Performance Baseline

Testing with a live cluster shows:

**kubectl subprocess:**
- `/yaml`: ~200ms (process spawn + kubeconfig + API call)
- `/describe`: ~100ms (same overhead)

**Performance breakdown:**
```
kubectl describe pod foo
├── Process spawn: ~20-30ms
├── Kubeconfig parsing: ~10-20ms
├── API client creation: ~10-20ms
├── Fetch pod from API: ~20-40ms
├── Fetch events from API: ~20-40ms
└── Format with describe logic: ~5-10ms
= Total: ~100ms
```

**Target performance:**
- `/yaml`: <1ms (marshal from cache)
- `/describe`: ~10ms (format from cache + events)

100-200ms is noticeable in a TUI. Users expect instant feedback when
viewing resource details.

## Options Considered

### Option 1: Subprocess to kubectl

**Implementation:**
```go
exec.Command("kubectl", "get", "pod", name, "-o", "yaml")
exec.Command("kubectl", "describe", "pod", name)
```

**Pros:**
- Zero implementation work
- Always matches kubectl behavior exactly
- No new dependencies

**Cons:**
- Slowest option (~100-200ms)
- Requires kubectl in PATH (portability issue)
- External process dependency

**Performance:** ~100-200ms per command

---

### Option 2: Use k8s.io/kubectl/pkg/describe (RECOMMENDED)

**Implementation:**
```go
import (
    "k8s.io/kubectl/pkg/describe"
    "k8s.io/kubectl/pkg/describe/versioned"
    "k8s.io/kubectl/pkg/printers"
)

// For describe
describer := versioned.NewHumanReadablePrinter(...)
describer.Describe(pod, events)

// For YAML
printer := printers.NewTypeSetter(scheme.Scheme).ToPrinter(
    &printers.YAMLPrinter{})
printer.PrintObj(pod, writer)
```

**Pros:**
- Fast (~1-10ms with informer cache)
- Same logic as kubectl (exact output match for both yaml and describe)
- Pure Go solution (no subprocess)
- Consistent approach for both commands
- Works for all resource types

**Cons:**
- Adds new dependency (`k8s.io/kubectl`)
- Larger binary size (~5-10MB increase)
- More complex to integrate than subprocess

**Performance:**
- `/yaml`: ~1-5ms (cache lookup + kubectl printer)
- `/describe`: ~5-10ms (cache lookup + formatting + events)

**Why faster than kubectl CLI?**
We skip process spawn, kubeconfig parsing, and API calls by using:
1. Informer cache for resource data (microsecond lookup)
2. Events informer for related events (microsecond lookup)
3. kubectl/pkg/describe and kubectl/pkg/printers for formatting only

**Decision:** Use this approach for both yaml and describe. The 1-10ms
performance is acceptable for TUI use case, and it provides consistency,
exact kubectl output parity, and simpler implementation than hybrid
approaches.

---

### Option 3: Marshal from informer cache (YAML only)

**Implementation:**
```go
import "sigs.k8s.io/yaml"

pod, err := r.podLister.Pods(namespace).Get(name)
yamlBytes, err := yaml.Marshal(pod)
```

**Pros:**
- BLAZING fast (<1ms, pure in-memory)
- Zero new dependencies (yaml already imported)
- Simple implementation (~10 lines)

**Cons:**
- Only works for YAML, not describe
- Output might differ slightly from kubectl:
  - Missing managed fields metadata
  - Status might be slightly stale between syncs
  - Different field ordering possible

**Performance:** <1ms (in-memory marshal)

---

### Option 4: Hybrid approach

**YAML:** Use Option 3 (in-memory marshal from cache)
**Describe:** Use Option 2 (kubectl/pkg/describe)

**Rationale:**
- YAML is the most common use case (viewing full resource spec)
- Sub-millisecond YAML is worth small output differences
- Describe is complex (~100s of lines per resource type)
- Worth the dependency to avoid reimplementing describe
- Both leverage informer cache (no API calls)

**Performance:**
- `/yaml`: <1ms
- `/describe`: ~10ms

**Trade-offs accepted:**
- Binary size increases ~5-10MB (currently ~20-30MB)
- YAML output may differ slightly from kubectl
- Need to add Events informer for describe

**Not chosen:** While this provides the best raw performance, the
additional complexity of maintaining two different approaches (custom
marshal vs kubectl library) is not justified. The 1-10ms vs <1ms
difference is negligible in practice.

## Design

### Architecture Overview

```
User Types: /yaml pod/nginx-abc123
     │
     ▼
┌─────────────────────────────────────────────────┐
│           Command Bar (commandbar.go)           │
│  - Parse command: /yaml pod/nginx-abc123        │
│  - Extract: resource=pod, name=nginx-abc123     │
└──────────────────┬──────────────────────────────┘
                   │ YamlCommandMsg
                   ▼
┌─────────────────────────────────────────────────┐
│          Current Screen (e.g., pods.go)         │
│  - Receive YamlCommandMsg                       │
│  - Call: repo.GetPodYAML(namespace, name)       │
└──────────────────┬──────────────────────────────┘
                   │
                   ▼
┌─────────────────────────────────────────────────┐
│       InformerRepository (informer.go)          │
│                                                  │
│  GetPodYAML(namespace, name):                   │
│    1. pod := podLister.Pods(ns).Get(name)       │
│       (~15-25μs - cache lookup)                 │
│    2. yamlBytes := yaml.Marshal(pod)            │
│       (~500μs - marshal)                        │
│    3. return string(yamlBytes)                  │
│                                                  │
│  Total: <1ms                                     │
└──────────────────┬──────────────────────────────┘
                   │ YAML string
                   ▼
┌─────────────────────────────────────────────────┐
│              YAML Modal (modal.go)              │
│  - Display YAML in scrollable full-screen view  │
│  - Syntax highlighting (future)                 │
│  - Copy to clipboard (future)                   │
└─────────────────────────────────────────────────┘
```

```
User Types: /describe pod/nginx-abc123
     │
     ▼
┌─────────────────────────────────────────────────┐
│           Command Bar (commandbar.go)           │
│  - Parse command: /describe pod/nginx-abc123    │
│  - Extract: resource=pod, name=nginx-abc123     │
└──────────────────┬──────────────────────────────┘
                   │ DescribeCommandMsg
                   ▼
┌─────────────────────────────────────────────────┐
│          Current Screen (e.g., pods.go)         │
│  - Receive DescribeCommandMsg                   │
│  - Call: repo.DescribePod(namespace, name)      │
└──────────────────┬──────────────────────────────┘
                   │
                   ▼
┌─────────────────────────────────────────────────┐
│       InformerRepository (informer.go)          │
│                                                  │
│  DescribePod(namespace, name):                  │
│    1. pod := podLister.Pods(ns).Get(name)       │
│       (~15-25μs - cache lookup)                 │
│    2. events := eventLister.Events(ns).List()   │
│       (~50-100μs - cache lookup, filter)        │
│    3. describer := describe.NewPodDescriber()   │
│    4. output := describer.Describe(pod, events) │
│       (~5-10ms - formatting)                    │
│    5. return output                             │
│                                                  │
│  Total: ~10ms                                    │
└──────────────────┬──────────────────────────────┘
                   │ Describe text
                   ▼
┌─────────────────────────────────────────────────┐
│           Describe Modal (modal.go)             │
│  - Display formatted text in scrollable view    │
│  - Same modal component as YAML (reusable)      │
└─────────────────────────────────────────────────┘
```

### Repository Interface Enhancement

Add new methods to `Repository` interface:

```go
type Repository interface {
    // ... existing methods (GetPods, GetDeployments, etc.)

    // YAML generation (fast path: <1ms)
    GetPodYAML(namespace, name string) (string, error)
    GetDeploymentYAML(namespace, name string) (string, error)
    GetServiceYAML(namespace, name string) (string, error)
    // ... other resource types

    // Describe generation (kubectl output: ~10ms)
    DescribePod(namespace, name string) (string, error)
    DescribeDeployment(namespace, name string) (string, error)
    DescribeService(namespace, name string) (string, error)
    // ... other resource types

    // Lifecycle
    Close()
}
```

### YAML Implementation

```go
import (
    "bytes"
    "fmt"
    "k8s.io/cli-runtime/pkg/printers"
    "k8s.io/client-go/kubernetes/scheme"
)

func (r *InformerRepository) GetPodYAML(
    namespace, name string) (string, error) {

    // Get pod from cache (microsecond operation)
    pod, err := r.podLister.Pods(namespace).Get(name)
    if err != nil {
        return "", fmt.Errorf("pod not found: %w", err)
    }

    // Create kubectl YAML printer
    printer := printers.NewTypeSetter(scheme.Scheme).ToPrinter(
        &printers.YAMLPrinter{})

    // Print to buffer (1-5ms)
    var buf bytes.Buffer
    if err := printer.PrintObj(pod, &buf); err != nil {
        return "", fmt.Errorf("failed to print YAML: %w", err)
    }

    return buf.String(), nil
}
```

**Benefits of using kubectl printers:**
- Exact output match with `kubectl get -o yaml`
- Proper type information and metadata formatting
- Handles all resource types consistently
- Same approach as describe (consistency)

### Describe Implementation

```go
import (
    "bytes"
    "fmt"
    "k8s.io/kubectl/pkg/describe"
    "k8s.io/kubectl/pkg/describe/versioned"
    corev1 "k8s.io/api/core/v1"
)

func (r *InformerRepository) DescribePod(
    namespace, name string) (string, error) {

    // Get pod from cache
    pod, err := r.podLister.Pods(namespace).Get(name)
    if err != nil {
        return "", fmt.Errorf("pod not found: %w", err)
    }

    // Get related events from cache
    events, err := r.eventLister.Events(namespace).List(
        labels.Everything())
    if err != nil {
        return "", fmt.Errorf("failed to list events: %w", err)
    }

    // Filter events related to this pod
    podEvents := filterEventsForObject(events, pod)

    // Create describer (kubectl package)
    describer := versioned.PodDescriber{
        Interface: r.clientset.CoreV1(),
    }

    // Generate describe output
    var buf bytes.Buffer
    err = describer.Describe(namespace, name, describe.DescriberSettings{
        ShowEvents: true,
    })
    if err != nil {
        return "", fmt.Errorf("describe failed: %w", err)
    }

    return buf.String(), nil
}

// Helper function to filter events for a specific object
func filterEventsForObject(
    events []*corev1.Event,
    obj runtime.Object) []*corev1.Event {

    // Filter events where InvolvedObject matches the target
    filtered := []*corev1.Event{}
    for _, event := range events {
        if matchesObject(event.InvolvedObject, obj) {
            filtered = append(filtered, event)
        }
    }
    return filtered
}
```

**Important:** This requires adding an **Events informer** to
`InformerRepository`. Events are essential for describe output (show
recent pod events, scheduling info, errors, etc.).

### Events: On-Demand Fetching (Revision 3)

**Decision (2025-10-05):** Use on-demand event fetching instead of
Events informer.

**Rationale:**
- Events are high-volume (100s-10,000s in busy clusters)
- Memory overhead: 1-20MB depending on cluster activity
- `/describe` is not performance-critical (user action, not hot path)
- 50-100ms latency for event fetching is acceptable for this use case

**Implementation:**
```go
func (r *InformerRepository) DescribeResource(...) (string, error) {
    // ... get resource from cache ...

    // Fetch events on-demand (not cached)
    events, err := r.fetchEventsForResource(namespace, name, uid)
    // Format and display events
}

func (r *InformerRepository) fetchEventsForResource(
    namespace, name, uid string) ([]corev1.Event, error) {

    fieldSelector := fmt.Sprintf(
        "involvedObject.name=%s,involvedObject.namespace=%s,involvedObject.uid=%s",
        name, namespace, uid)

    eventList, err := r.clientset.CoreV1().Events(namespace).List(
        ctx, metav1.ListOptions{
            FieldSelector: fieldSelector,
            Limit:         100,
        })
    return eventList.Items, err
}
```

**Performance:**
- YAML: <5ms (from cache, no events needed)
- Describe: 50-100ms (cache lookup + on-demand event fetch)

**Trade-offs:**
- ✅ Zero memory overhead for events
- ✅ Simple implementation (no informer lifecycle management)
- ✅ Works 100% of the time (direct API call)
- ❌ Slightly slower describe (~50-100ms vs ~10ms with cache)
- ✅ Acceptable for TUI use case (user-initiated action)

### ~~Events Informer Addition~~ (Superseded by Revision 3)

**Original approach (Revision 1-2):** Events should be loaded in
**Tier 2** (background) using informer.

**Superseded by:** On-demand fetching (Revision 3) for better memory
efficiency

```go
func (r *InformerRepository) DescribePod(
    namespace, name string) (string, error) {

    // Try to get events from cache
    var podEvents []*corev1.Event
    if r.IsResourceSynced(ResourceTypeEvent) {
        // Fast path: use cache (~10ms total)
        events, _ := r.eventLister.Events(namespace).List(
            labels.Everything())
        podEvents = filterEventsForObject(events, pod)
    } else {
        // Fallback: direct API call (~30-50ms)
        eventList, err := r.clientset.CoreV1().Events(
            namespace).List(ctx, metav1.ListOptions{
            FieldSelector: fmt.Sprintf(
                "involvedObject.name=%s", name),
        })
        if err == nil {
            podEvents = eventList.Items
        }
        // Continue even if events fetch fails
    }

    // Generate describe output with events
    // ...
}
```

This ensures:
- UI starts quickly (events not in Tier 1)
- Describe works before events are fully synced (slower fallback)
- Describe is fast once events are synced

### Message Types

Add new messages for commands:

```go
// YamlCommandMsg triggers YAML display for a resource
type YamlCommandMsg struct {
    ResourceType string // "pod", "deployment", "service"
    Namespace    string
    Name         string
}

// DescribeCommandMsg triggers describe display for a resource
type DescribeCommandMsg struct {
    ResourceType string
    Namespace    string
    Name         string
}

// ShowModalMsg displays a modal with content
type ShowModalMsg struct {
    Title   string // "YAML: pod/nginx-abc123"
    Content string // YAML or describe text
    Mode    string // "yaml" or "describe"
}
```

### Command Bar Integration

Command bar parses commands and sends messages:

```go
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        if m.state == StateInput {
            if msg.Type == tea.KeyEnter {
                cmd := m.input.Value()

                // Parse /yaml command
                if strings.HasPrefix(cmd, "/yaml ") {
                    return m, func() tea.Msg {
                        // Parse: /yaml pod/nginx-abc123
                        parts := parseResourceRef(cmd[6:])
                        return types.YamlCommandMsg{
                            ResourceType: parts.Type,
                            Namespace:    parts.Namespace,
                            Name:         parts.Name,
                        }
                    }
                }

                // Parse /describe command
                if strings.HasPrefix(cmd, "/describe ") {
                    return m, func() tea.Msg {
                        parts := parseResourceRef(cmd[10:])
                        return types.DescribeCommandMsg{
                            ResourceType: parts.Type,
                            Namespace:    parts.Namespace,
                            Name:         parts.Name,
                        }
                    }
                }
            }
        }
    }
    return m, nil
}

// Helper to parse resource references
// Formats: "pod/name", "name" (infer type from context),
//          "namespace/name"
func parseResourceRef(ref string) ResourceRef {
    // Implementation details...
}
```

### Modal Display

Reuse existing modal component for both YAML and describe:

```go
// In screen's Update()
case types.YamlCommandMsg:
    yaml, err := m.repo.GetPodYAML(msg.Namespace, msg.Name)
    if err != nil {
        return m, showError(err)
    }

    return m, func() tea.Msg {
        return types.ShowModalMsg{
            Title:   fmt.Sprintf("YAML: %s/%s",
                msg.Namespace, msg.Name),
            Content: yaml,
            Mode:    "yaml",
        }
    }

case types.DescribeCommandMsg:
    desc, err := m.repo.DescribePod(msg.Namespace, msg.Name)
    if err != nil {
        return m, showError(err)
    }

    return m, func() tea.Msg {
        return types.ShowModalMsg{
            Title:   fmt.Sprintf("Describe: %s/%s",
                msg.Namespace, msg.Name),
            Content: desc,
            Mode:    "describe",
        }
    }
```

Modal component provides:
- Full-screen scrollable view
- `ESC` to close
- `↑/↓` or `j/k` to scroll
- Page Up/Down support
- Search within content (future)
- Copy to clipboard (future)

## Testing Strategy

### Unit Tests

```go
func TestGetPodYAML(t *testing.T) {
    // Create test pod
    pod := &corev1.Pod{...}

    // Add to fake informer
    repo := newTestRepository(pod)

    // Get YAML
    yaml, err := repo.GetPodYAML("default", "test-pod")

    // Assert
    assert.NoError(t, err)
    assert.Contains(t, yaml, "kind: Pod")
    assert.Contains(t, yaml, "name: test-pod")
}

func TestDescribePod(t *testing.T) {
    // Create test pod and events
    pod := &corev1.Pod{...}
    events := []*corev1.Event{...}

    // Add to fake informers
    repo := newTestRepository(pod, events)

    // Get describe output
    desc, err := repo.DescribePod("default", "test-pod")

    // Assert
    assert.NoError(t, err)
    assert.Contains(t, desc, "Name:")
    assert.Contains(t, desc, "Namespace:")
    assert.Contains(t, desc, "Events:")
}
```

### Integration Tests

Test with live cluster (manual):
1. Run `/yaml` on various resources, compare with `kubectl get -o yaml`
2. Run `/describe` on various resources, compare with `kubectl describe`
3. Measure performance (should be <1ms for YAML, ~10ms for describe)
4. Test with resources that have many events (verify filtering)
5. Test with resources that don't exist (error handling)
6. Test with resources in different namespaces

### Performance Validation

```bash
# Benchmark YAML generation
go test -bench=BenchmarkGetPodYAML -benchtime=1000x

# Benchmark describe generation
go test -bench=BenchmarkDescribePod -benchtime=1000x

# Expected results:
# BenchmarkGetPodYAML    1000    ~500 μs/op    (0.5ms)
# BenchmarkDescribePod   1000    ~10 ms/op
```

Compare with kubectl subprocess:
```bash
# Time kubectl commands
time kubectl get pod nginx -o yaml
time kubectl describe pod nginx

# Expected: 100-200ms each
```

## Alternatives Considered (Detailed)

### Why not subprocess for everything?

**Performance:** 100-200ms is noticeable in a TUI. When user presses a
key to view details, they expect instant feedback (<50ms perceived as
instant). 100-200ms feels sluggish.

**Portability:** Requires kubectl in PATH. If user has custom kubectl
version or different binary name, it breaks. Binary distribution
becomes harder.

**Control:** Can't customize output, add syntax highlighting, or
integrate with UI components (modals, scrolling, search).

### Why not reimplement describe from scratch?

**Maintenance burden:** kubectl's describe logic is complex:
- Pod describe: ~300 lines (status, conditions, volumes, events)
- Deployment describe: ~200 lines (strategy, replicas, conditions)
- Service describe: ~150 lines (endpoints, ports, selectors)

Would need to maintain parity with kubectl as Kubernetes evolves.

**Output parity:** Users expect kubectl-style output. Any differences
would be confusing.

**Not worth it:** The kubectl/pkg/describe package exists and is stable.
5-10MB binary size increase is worth avoiding 1000+ lines of custom
formatting code.

### Why not use API calls instead of cache?

**Performance:** API calls are 20-40ms minimum, plus network latency.
Cannot achieve <10ms target.

**API server load:** Every describe/yaml command would hit the API.
With many users or rapid navigation, this creates unnecessary load.

**Offline capability:** Cache allows viewing resources during brief
network issues (informer reconnects automatically).

### Why hybrid (YAML from cache, describe from kubectl/pkg)?

**Best of both worlds:**
- YAML is trivial to generate (just marshal), so use cache (0.5ms)
- Describe is complex to generate, so use proven library (~10ms)

**Acceptable trade-offs:**
- YAML output differs slightly from kubectl (acceptable for TUI)
- Binary size increases (5-10MB, but still reasonable)
- Need Events informer (but useful for other features later)

## Consequences

### Positive

- **Fast response:** 1-10ms response time, feels instant in TUI
- **10-20x faster than kubectl CLI:** No process spawn or kubeconfig
  parsing overhead
- **No external dependencies:** Works without kubectl in PATH
- **Output parity:** Both yaml and describe match kubectl exactly
  (same code)
- **Leverages cache:** No API calls, no server load
- **Consistent UX:** Both commands feel instant in the TUI
- **Consistent implementation:** Same approach (kubectl libraries) for
  both commands
- **Future-proof:** Can add features (syntax highlighting, search, copy)

### Negative

- **Binary size:** Increases ~5-10MB due to kubectl packages import
  (from ~20-30MB to ~25-40MB)
- **Events required:** Describe needs Events informer, adds Tier 2
  loading complexity
- **Code complexity:** More complex than subprocess, but manageable
- **Testing effort:** Need integration tests to validate output parity

### Neutral

- **Unified approach:** Using kubectl libraries for both commands
  simplifies maintenance
- **Fallback strategy:** Events API call fallback adds ~30-50ms if not
  synced, but only temporary
- **kubectl dependency:** Adding k8s.io/kubectl and k8s.io/cli-runtime
  packages, but they're official and stable

## Implementation Notes

### Dependency Addition

Add to `go.mod`:
```bash
go get k8s.io/kubectl@v0.34.1
go get k8s.io/cli-runtime@v0.34.1
```

Ensure versions match existing k8s.io/client-go version (0.34.1).

Both packages are needed:
- `k8s.io/kubectl`: For describe formatters
- `k8s.io/cli-runtime`: For printers (YAML/JSON output)

### Events Informer Configuration

Events are high-volume (1000s in busy clusters). Consider:
- Add to Tier 2 (background loading)
- Use field selector to reduce memory (future optimization)
- Set shorter resync period (10s instead of 30s for fresher data)

```go
// In NewInformerRepository()
eventInformer := factory.Core().V1().Events().Informer()
eventLister := factory.Core().V1().Events().Lister()
```

### Resource Type Support

Start with common resources:
- Pods (most important)
- Deployments
- Services

Add more resource types incrementally:
- StatefulSets
- DaemonSets
- ConfigMaps
- Secrets (YAML only, redact sensitive data)

Each resource type needs:
1. `Get{Resource}YAML()` method (~10 lines)
2. `Describe{Resource}()` method (~30 lines)
3. Command bar parsing for resource type
4. Tests

### Error Handling

**Resource not found:**
```
Error: pod "nginx-abc123" not found in namespace "default"
```

**Events not synced (describe fallback):**
```
(loading events from API...)
```

**Marshal/describe errors:**
```
Error: failed to generate YAML: [error details]
```

Show errors in temporary banner or modal, not crash application.

## Future Enhancements

1. **Syntax highlighting:** Colorize YAML output (keywords, values, etc)
2. **Search in modal:** Press `/` to search within YAML/describe text
3. **Copy to clipboard:** Press `y` to copy content
4. **Export to file:** Save YAML/describe output to file
5. **Diff view:** Compare two resources side-by-side
6. **Edit YAML:** In-place editing with validation (advanced feature)
7. **JSON output:** Add `/json` command (trivial with json.Marshal)
8. **Watch mode:** Live-update YAML/describe view as resource changes

## References

- [k8s.io/kubectl/pkg/describe](
  https://pkg.go.dev/k8s.io/kubectl/pkg/describe)
- [sigs.k8s.io/yaml](https://pkg.go.dev/sigs.k8s.io/yaml)
- [kubectl source code](
  https://github.com/kubernetes/kubectl/tree/master/pkg/describe)
- DDR-03: Kubernetes Informer-Based Repository Implementation
- DDR-08: Pragmatic Command Implementation (explains why yaml/describe
  are exceptions requiring pure Go)
- PLAN-03: Command-Enhanced UI Implementation (Phase 3: Resource
  Commands)
