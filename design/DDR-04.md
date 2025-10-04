# Testing Strategy for Kubernetes Informer-Based Repository

| Metadata | Value                                                        |
|----------|--------------------------------------------------------------|
| Date     | 2025-10-04                                                   |
| Author   | @renato0307                                                  |
| Status   | Proposed                                                     |
| Tags     | testing, kubernetes, informers, envtest, integration-tests   |

| Revision | Date       | Author      | Info                         |
|----------|------------|-------------|------------------------------|
| 1        | 2025-10-04 | @renato0307 | Initial design               |
| 2        | 2025-10-04 | @renato0307 | Change to shared envtest     |

## Context and Problem Statement

The InformerRepository (DDR-03) uses Kubernetes client-go informers to
cache and query cluster resources. We need a testing strategy that:

1. Validates informer setup, cache sync, and query operations
2. Tests data transformation (k8s objects → internal types)
3. Tests error scenarios (connection failures, RBAC, timeouts)
4. Runs quickly (ideally under 2 seconds per test suite)
5. Minimizes mock usage (prefer real implementations when possible)
6. Works in CI/CD pipelines without external dependencies

**Key Question:** How do we test code that depends on Kubernetes API
server and informers without a full cluster?

## Prior Work

The project currently has:
- No existing tests
- Test dependencies already present (ginkgo, gomega, testify as
  transitive deps)
- POC implementation in `cmd/proto-pods-tui/main.go` showing informer
  usage patterns

Kubernetes ecosystem testing approaches:
1. **Fake Clientset** - In-memory fake implementation
2. **envtest** - Real API server + etcd binaries (controller-runtime)
3. **kind/k3s** - Full Kubernetes cluster in Docker
4. **Mocks** - Mock interfaces (gomock, testify/mock)

## Testing Options Analysis

### Option 1: Fake Clientset (`k8s.io/client-go/kubernetes/fake`)

**How it works:**
- In-memory fake implementation of Kubernetes clientset
- No real API server - objects stored in memory
- Supports CRUD operations and watches
- Works with informers and listers

**Pros:**
- ✅ Very fast: Tests run in milliseconds
- ✅ No external dependencies (no Docker, binaries, etc.)
- ✅ Works perfectly in CI/CD
- ✅ Predictable and deterministic
- ✅ Easy to set up specific test scenarios
- ✅ Real informer/lister code paths (not mocked)

**Cons:**
- ❌ Not a real API server (some edge cases may differ)
- ❌ No validation webhooks or admission controllers
- ❌ Limited RBAC simulation
- ❌ No API server-side pagination or field selectors

**Example:**
```go
import (
    "testing"
    "k8s.io/client-go/kubernetes/fake"
    "k8s.io/client-go/informers"
    corev1 "k8s.io/api/core/v1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestInformerRepository(t *testing.T) {
    // Create fake client with initial objects
    pod := &corev1.Pod{
        ObjectMeta: metav1.ObjectMeta{
            Name: "test-pod",
            Namespace: "default",
        },
        Status: corev1.PodStatus{
            Phase: corev1.PodRunning,
        },
    }

    client := fake.NewSimpleClientset(pod)
    factory := informers.NewSharedInformerFactory(client, 0)

    // Use real informer code paths
    podInformer := factory.Core().V1().Pods()
    podLister := podInformer.Lister()

    // Start informer and wait for sync
    stopCh := make(chan struct{})
    defer close(stopCh)
    factory.Start(stopCh)
    factory.WaitForCacheSync(stopCh)

    // Query from cache (same code as production)
    pods, err := podLister.List(labels.Everything())
    if err != nil {
        t.Fatal(err)
    }

    if len(pods) != 1 {
        t.Errorf("expected 1 pod, got %d", len(pods))
    }
}
```

### Option 2: envtest (controller-runtime)

**How it works:**
- Runs real etcd and kube-apiserver binaries
- No kubelet, scheduler, or controllers
- Downloads binaries on first run (~200MB)
- Startup time: 2-5 seconds per test suite

**Pros:**
- ✅ Real API server (most accurate behavior)
- ✅ Proper validation and defaulting
- ✅ Works with CRDs
- ✅ Realistic for controller testing

**Cons:**
- ❌ Slower: 2-5 seconds startup overhead
- ❌ Requires downloading binaries (~200MB first time)
- ❌ More complex setup
- ❌ Requires setup-envtest tool
- ❌ May require specific OS/arch binaries
- ❌ Overkill for simple repository testing

**When to use:**
- Integration tests (not unit tests)
- Testing custom controllers
- Testing CRD interactions
- Testing webhooks

### Option 3: kind/k3s (Full Cluster)

**How it works:**
- Full Kubernetes cluster running in Docker
- All components: API server, scheduler, kubelet, controllers
- Startup time: 30-60 seconds

**Pros:**
- ✅ Fully realistic cluster
- ✅ Tests against real kubelet
- ✅ Can test node interactions

**Cons:**
- ❌ Very slow: 30-60 seconds per cluster
- ❌ Requires Docker
- ❌ Too heavy for unit tests
- ❌ Complex cleanup and resource management
- ❌ May fail in some CI environments

**When to use:**
- End-to-end tests only
- Manual testing during development
- NOT suitable for automated unit tests

### Option 4: Mocks (gomock, testify/mock)

**How it works:**
- Mock the Repository interface or clientset interfaces
- Define expected method calls and return values

**Pros:**
- ✅ Very fast
- ✅ Complete control over behavior

**Cons:**
- ❌ High maintenance (mock setup code)
- ❌ Doesn't test real informer/lister logic
- ❌ Easy to create unrealistic test scenarios
- ❌ Brittle (breaks on interface changes)
- ❌ Doesn't catch integration issues

**When to use:**
- Testing code that USES the repository (screens, app logic)
- NOT for testing the repository implementation itself

## Decision

**Use envtest with Shared TestMain for Integration Tests**

We will use `sigs.k8s.io/controller-runtime/pkg/envtest` as the primary
testing approach with a **shared test environment** pattern:

1. **Real API Server**: Tests run against real kube-apiserver and etcd,
   providing maximum accuracy and catching real-world issues
2. **Fast with shared setup**: Using TestMain to start envtest once
   means 2-5s startup overhead for entire test suite (not per-test)
3. **Integration test confidence**: Tests validate actual Kubernetes
   behavior, not fake implementations
4. **Future-proof**: Works with CRDs, webhooks, admission controllers
   if we add them later
5. **Still CI-friendly**: Downloads binaries once (~200MB), then cached

**Key Insight:** With TestMain pattern, envtest becomes fast enough for
unit test workflow while providing real API server behavior.

**Fallback to fake clientset** only for:
- Pure transformation function tests (no API needed)
- Environments where binary downloads are blocked
- Tests that need extreme speed (<10ms)

**Never use kind/k3s for automated tests** - too slow (30-60s startup).

## Design

### Test Organization

```
internal/k8s/
├── repository.go
├── repository_test.go           # Integration tests with envtest
├── informer_repository.go
├── informer_repository_test.go  # Integration tests with envtest
├── suite_test.go                # TestMain - shared envtest setup
└── testdata/
    └── fixtures.go              # Shared test fixtures
```

### Dependencies

Add envtest to `go.mod`:
```bash
go get sigs.k8s.io/controller-runtime/pkg/envtest
go mod tidy
```

The package brings in:
- `sigs.k8s.io/controller-runtime/pkg/envtest` - Test environment
- Test binaries downloaded separately by `setup-envtest` tool

### Shared envtest Setup (TestMain Pattern)

**Key Pattern:** Start envtest once in TestMain, share across all tests

```go
// suite_test.go
package k8s

import (
    "os"
    "path/filepath"
    "testing"

    "k8s.io/client-go/kubernetes"
    "k8s.io/client-go/rest"
    "sigs.k8s.io/controller-runtime/pkg/envtest"
)

var (
    testEnv   *envtest.Environment
    cfg       *rest.Config
    clientset *kubernetes.Clientset
)

func TestMain(m *testing.M) {
    // Setup: Start envtest once for all tests
    testEnv = &envtest.Environment{
        CRDDirectoryPaths: []string{}, // Empty for core resources
    }

    var err error
    cfg, err = testEnv.Start()
    if err != nil {
        panic(err)
    }

    clientset, err = kubernetes.NewForConfig(cfg)
    if err != nil {
        panic(err)
    }

    // Run all tests
    code := m.Run()

    // Teardown: Stop envtest
    testEnv.Stop()

    os.Exit(code)
}
```

**Advantages:**
- ✅ 2-5s startup happens ONCE (not per test)
- ✅ All tests share same API server instance
- ✅ Tests still isolated (clean up objects between tests)
- ✅ Real API server behavior
- ✅ Fast individual test execution (~10-50ms per test)

### Test Layers

**Layer 1: Data Transformation Tests (No API Server)**
- Test `transformPod()` function in isolation
- Input: corev1.Pod object
- Output: internal Pod struct
- Fast, pure functions, no API needed
- Can use table-driven tests

**Layer 2: Repository Integration Tests (envtest)**
- Test `InformerRepository` with real API server
- Test cache sync, queries, loading status
- Test error scenarios (not found, RBAC simulation)
- Uses real informer/lister implementations
- Create/delete test objects between tests

**Layer 3: Screen Tests (Mock Repository)**
- Test screen Update() logic with mock repository interface
- Test periodic refresh
- Test filter/search integration
- Mock repository interface (not clientset or API server)

### envtest Test Pattern

**Test Setup (uses shared testEnv from TestMain):**
```go
func setupTestRepository(t *testing.T) *InformerRepository {
    // Use shared clientset from TestMain (already connected)
    factory := informers.NewSharedInformerFactory(clientset, 0)

    // Create informers
    podInformer := factory.Core().V1().Pods()
    podLister := podInformer.Lister()

    repo := &InformerRepository{
        clientset:     clientset,
        factory:       factory,
        podLister:     podLister,
        informers:     map[ResourceType]cache.SharedIndexInformer{
            ResourceTypePod: podInformer.Informer(),
        },
        loadingStatus: map[ResourceType]*LoadingStatus{},
        stopCh:        make(chan struct{}),
    }

    return repo
}

// Helper to clean up test objects after each test
func cleanupPods(t *testing.T, namespace string) {
    err := clientset.CoreV1().Pods(namespace).DeleteCollection(
        context.Background(),
        metav1.DeleteOptions{},
        metav1.ListOptions{},
    )
    if err != nil {
        t.Logf("Warning: failed to cleanup pods: %v", err)
    }
}
```

**Test Pattern with Real API Server:**
```go
func TestInformerRepository_GetPods(t *testing.T) {
    ctx := context.Background()
    namespace := "default"
    defer cleanupPods(t, namespace)

    // Arrange: Create real pod via API
    pod := &corev1.Pod{
        ObjectMeta: metav1.ObjectMeta{
            Name:      "nginx",
            Namespace: namespace,
        },
        Spec: corev1.PodSpec{
            Containers: []corev1.Container{
                {Name: "nginx", Image: "nginx:latest"},
            },
        },
    }

    _, err := clientset.CoreV1().Pods(namespace).Create(
        ctx, pod, metav1.CreateOptions{})
    if err != nil {
        t.Fatalf("failed to create pod: %v", err)
    }

    // Act: Setup repository and sync
    repo := setupTestRepository(t)
    defer close(repo.stopCh)

    if err := repo.StartPriority(ctx); err != nil {
        t.Fatalf("StartPriority failed: %v", err)
    }

    // Assert: Query pods from cache
    pods, err := repo.GetPods()
    if err != nil {
        t.Fatalf("GetPods failed: %v", err)
    }

    if len(pods) != 1 {
        t.Errorf("expected 1 pod, got %d", len(pods))
    }

    if pods[0].Name != "nginx" {
        t.Errorf("expected pod name 'nginx', got %s", pods[0].Name)
    }
}
```

**Test Isolation:** Each test creates and deletes its own objects.
The API server is shared, but objects are independent.

### Test Fixtures

Create reusable fixtures in `testdata/fixtures.go`:

```go
package k8s

import (
    corev1 "k8s.io/api/core/v1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "time"
)

// NewTestPod creates a minimal pod for testing
func NewTestPod(name, namespace string) *corev1.Pod {
    return &corev1.Pod{
        ObjectMeta: metav1.ObjectMeta{
            Name:      name,
            Namespace: namespace,
            CreationTimestamp: metav1.Time{
                Time: time.Now(),
            },
        },
        Status: corev1.PodStatus{
            Phase: corev1.PodRunning,
            ContainerStatuses: []corev1.ContainerStatus{
                {Ready: true, RestartCount: 0},
            },
        },
    }
}

// NewTestPodWithStatus creates a pod with custom status
func NewTestPodWithStatus(name, namespace string,
    phase corev1.PodPhase, ready bool) *corev1.Pod {

    pod := NewTestPod(name, namespace)
    pod.Status.Phase = phase
    pod.Status.ContainerStatuses[0].Ready = ready
    return pod
}
```

### Test Cases to Cover

**Repository Initialization:**
- ✅ Create repository with valid config
- ✅ Create repository with invalid config (error case)

**Priority Loading (StartPriority):**
- ✅ Sync succeeds with pods
- ✅ Sync succeeds with empty cache
- ✅ Loading status updated correctly
- ✅ IsResourceSynced returns true after sync

**Query Operations (GetPods):**
- ✅ Query returns all pods
- ✅ Query returns empty list when no pods
- ✅ Query fails before sync (error)
- ✅ Pods sorted by age (newest first)
- ✅ Pods sorted by name (secondary sort)

**Data Transformation:**
- ✅ Transform running pod correctly
- ✅ Transform pending pod correctly
- ✅ Transform failed pod correctly
- ✅ Calculate ready containers (1/2, 2/2, 0/1)
- ✅ Calculate total restarts
- ✅ Calculate age correctly
- ✅ Extract node name and IP

**Loading Status:**
- ✅ GetLoadingStatus before sync (synced=false)
- ✅ GetLoadingStatus after sync (synced=true)
- ✅ GetAllLoadingStatus returns all resources
- ✅ IsResourceSynced checks individual resources

**Background Loading (StartBackground - Future):**
- ✅ Parallel sync of multiple resources
- ✅ Loading status updates independently
- ✅ Partial failures (some resources fail)

**Lifecycle:**
- ✅ Stop closes stopCh
- ✅ Stop is idempotent (can call multiple times)

### Testing Tools

**Standard library `testing`:**
- Use for simple test cases
- Good for transformation function tests

**Optional: testify/assert (already a dependency):**
```go
import "github.com/stretchr/testify/assert"

func TestTransformPod(t *testing.T) {
    pod := NewTestPod("test", "default")
    result := transformPod(pod)

    assert.Equal(t, "test", result.Name)
    assert.Equal(t, "default", result.Namespace)
    assert.Equal(t, "Running", result.Status)
    assert.Equal(t, int32(0), result.Restarts)
}
```

**Optional: table-driven tests:**
```go
func TestTransformPod_Status(t *testing.T) {
    tests := []struct {
        name       string
        phase      corev1.PodPhase
        wantStatus string
    }{
        {"running", corev1.PodRunning, "Running"},
        {"pending", corev1.PodPending, "Pending"},
        {"failed", corev1.PodFailed, "Failed"},
        {"succeeded", corev1.PodSucceeded, "Succeeded"},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            pod := NewTestPodWithStatus("test", "default",
                tt.phase, true)
            result := transformPod(pod)

            if result.Status != tt.wantStatus {
                t.Errorf("expected status %s, got %s",
                    tt.wantStatus, result.Status)
            }
        })
    }
}
```

### Running Tests

**First-time setup:**
```bash
# Install setup-envtest (downloads API server binaries)
go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest

# Download test binaries (~200MB, cached)
setup-envtest use -p path

# Or let envtest download automatically on first test run
```

**Running tests:**
```bash
# Run all tests (envtest starts once via TestMain)
go test ./...

# Run with verbose output
go test -v ./internal/k8s/...

# Run specific test
go test -v -run TestInformerRepository_GetPods ./internal/k8s/

# Run with coverage
go test -cover ./...
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Run with race detector
go test -race ./...
```

**Performance:**
- First run: 2-5s (envtest startup in TestMain)
- Subsequent tests: 10-50ms per test
- Total suite time: 2-10s typically

### CI/CD Integration

**GitHub Actions example:**
```yaml
- name: Set up Go
  uses: actions/setup-go@v4
  with:
    go-version: '1.24'

- name: Install setup-envtest
  run: go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest

- name: Setup envtest binaries
  run: |
    ENVTEST_ASSETS=$(setup-envtest use -p path)
    echo "KUBEBUILDER_ASSETS=$ENVTEST_ASSETS" >> $GITHUB_ENV

- name: Run Tests
  run: |
    go test -v -race -coverprofile=coverage.out ./...
    go tool cover -func=coverage.out
```

**CI Considerations:**
- ✅ Binaries cached between runs (save 2-5s download time)
- ✅ No Docker required
- ✅ Fast execution (entire suite in <10s)
- ✅ Deterministic and reliable

## Consequences

### Positive
- ✅ **Real API server behavior**: Tests validate actual Kubernetes
  behavior, not fake implementations
- ✅ **Fast with TestMain pattern**: 2-5s startup once, then 10-50ms
  per test (enables TDD)
- ✅ **High confidence**: Integration tests catch real-world issues
  before production
- ✅ **Future-proof**: Works with CRDs, webhooks, admission controllers
  if added later
- ✅ **CI-friendly**: Binaries cached, no Docker, fast execution
- ✅ **Tests real informer/lister code**: Not mocked, actual client-go
  implementations

### Negative
- ❌ **Binary download required**: ~200MB first time (then cached)
- ❌ **Slightly slower than fake**: 2-5s startup vs <100ms for fake
  clientset
- ❌ **Platform-specific binaries**: Requires setup-envtest for correct
  OS/arch
- ❌ **More setup complexity**: Requires TestMain pattern and cleanup
  logic

### Neutral
- ⚖️ Trade-off accepted: Prefer real API server accuracy over extreme
  speed
- ⚖️ Transformation tests can still use pure functions (no API needed)
- ⚖️ Screen tests use mocked repository interface (separate layer)

## Alternatives Considered

### Alternative 1: Fake Clientset Only
**Pros:** Very fast (<100ms), no dependencies, simple setup
**Cons:** Not real API server, may miss edge cases, limited validation
**Rejected:** Prefer real API server accuracy for integration tests.
Fake clientset is still useful for pure transformation tests.

### Alternative 2: envtest Per-Test (No TestMain)
**Pros:** Complete test isolation
**Cons:** 2-5s startup PER test = very slow suite (minutes for 20 tests)
**Rejected:** Too slow. TestMain pattern solves this.

### Alternative 3: kind/k3s for Automated Tests
**Pros:** Full cluster with kubelet and controllers
**Cons:** 30-60s startup, requires Docker, heavy resource usage
**Rejected:** Way too slow for unit/integration tests. Only for manual
E2E testing.

### Alternative 4: Mocks Only (gomock/testify)
**Pros:** Very fast, complete control
**Cons:** Doesn't test real informer/lister logic, brittle, high
maintenance
**Rejected:** Doesn't provide integration test value. Only use mocks for
screen layer testing.

### Alternative 5: No Tests (rely on manual testing)
**Rejected:** Not sustainable. Repository logic with parallel loading
is complex and easy to break. Need automated tests.

## References

- [Kubernetes Fake Clientset](
  https://pkg.go.dev/k8s.io/client-go/kubernetes/fake)
- [Testing Kubernetes Controllers](
  https://book.kubebuilder.io/reference/testing.html)
- [envtest Documentation](
  https://pkg.go.dev/sigs.k8s.io/controller-runtime/pkg/envtest)
- Current Repository: `internal/k8s/repository.go`
- POC Implementation: `cmd/proto-pods-tui/main.go`
