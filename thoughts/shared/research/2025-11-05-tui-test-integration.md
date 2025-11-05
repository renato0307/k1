---
date: 2025-11-05T00:00:00Z
researcher: @renato0307
git_commit: cb7ed35c75ffa7dbd0fe6a53987f67685664ac2b
branch: chore/tui-tests
repository: k1-tui-tests
topic: "How to use microsoft/tui-test for k1 testing"
tags: [research, testing, tui-test, e2e, typescript, kind, ctlptl]
status: complete
last_updated: 2025-11-05
last_updated_by: @renato0307
last_updated_note: "Added dummy mode removal, kind cluster setup with ctlptl,
and detailed tui-test vs VHS comparison"
---

# Research: How to use microsoft/tui-test for k1 testing

**Date**: 2025-11-05
**Researcher**: @renato0307
**Git Commit**: cb7ed35c75ffa7dbd0fe6a53987f67685664ac2b
**Branch**: chore/tui-tests
**Repository**: k1-tui-tests

## Research Question

How can we use https://github.com/microsoft/tui-test to add E2E testing
capabilities to the k1 Kubernetes TUI?

## Summary

**tui-test** is a Microsoft end-to-end testing framework for terminal
applications, built on xterm.js (the same engine powering VSCode). It
provides automated testing of CLI/TUI programs through terminal
interaction, visual snapshots, and output assertions.

**Critical Findings**:

1. **tui-test** is a **TypeScript/JavaScript** framework, while k1 is
   **Go**. Direct integration is not possible. However, tui-test can be
   used for **black-box E2E testing** by launching the k1 binary.

2. **Dummy mode is broken and will be removed** from k1. The `-dummy`
   flag exists in Makefile but is not implemented in `cmd/k1/main.go`.
   Tests need a **real Kubernetes cluster**.

3. **ctlptl + kind** is the recommended approach for creating test
   clusters with predictable resources for E2E testing.

**Testing Strategy**:
- **Current**: Unit/integration tests with envtest (76.7% coverage)
- **New**: E2E tests with tui-test against kind cluster
- **Prerequisite**: kind cluster with test resources (via ctlptl)

## Detailed Findings

### What is tui-test?

**Repository**: https://github.com/microsoft/tui-test

**Description**: An automated testing framework for terminal
applications that:
- Launches terminal programs in isolated contexts
- Simulates keyboard input and user interactions
- Captures and asserts on terminal output
- Takes terminal screenshots for visual regression testing
- Supports multiple platforms (macOS, Linux, Windows)
- Works with various shells (bash, zsh, fish, PowerShell, cmd)

**Key Features**:
- Auto-wait functionality (waits for terminal readiness)
- Built-in retry strategies for flaky test elimination
- Terminal snapshots and detailed tracing
- Fast test isolation (milliseconds per context)
- Framework-agnostic (tests any terminal program)

### Installation and Setup

#### Prerequisites
- Node.js 20.x, 18.x, or 16.6.0+
- The program under test (k1 binary)

#### Installation

```bash
# In project root, create package.json if needed
npm init -y

# Install tui-test as dev dependency
npm install -D @microsoft/tui-test

# Or with yarn
yarn add --dev @microsoft/tui-test

# Or with pnpm
pnpm add -D @microsoft/tui-test
```

#### Configuration

Create `tui-test.config.ts` in project root:

```typescript
import { defineConfig } from "@microsoft/tui-test";

export default defineConfig({
  retries: 3,      // Retry flaky tests
  trace: true      // Enable detailed tracing
});
```

### API Reference

#### Core Methods

**Terminal Interaction**:
```typescript
terminal.write("text")           // Type text
terminal.submit("command")       // Execute command
terminal.getByText("string")     // Find text in output
terminal.getByText(/regex/g)     // Find text by pattern
```

**Assertions**:
```typescript
await expect(terminal.getByText("foo")).toBeVisible()
await expect(terminal).toMatchSnapshot()
```

**Configuration**:
```typescript
test.use({
  program: {
    file: "./k1",           // Path to binary
    args: ["-dummy"]        // Command arguments
  }
})
```

### Example Test for k1

Here's how a tui-test for k1 might look:

```typescript
// tests/e2e/k1-basic.test.ts
import { test, expect } from "@microsoft/tui-test";

// Configure k1 binary path
test.use({
  program: {
    file: "./k1",
    args: ["-dummy"]  // Use dummy mode for predictable data
  }
});

test("k1 shows pods screen on startup", async ({ terminal }) => {
  // Wait for initial screen render
  await expect(
    terminal.getByText("Pods", { full: true })
  ).toBeVisible();

  // Verify table headers are present
  await expect(terminal.getByText("NAME")).toBeVisible();
  await expect(terminal.getByText("NAMESPACE")).toBeVisible();
  await expect(terminal.getByText("STATUS")).toBeVisible();
});

test("k1 navigation with keyboard", async ({ terminal }) => {
  // Wait for pods screen
  await expect(terminal.getByText("Pods")).toBeVisible();

  // Press 'd' to switch to deployments
  terminal.write("d");

  // Verify deployments screen loads
  await expect(terminal.getByText("Deployments")).toBeVisible();

  // Take snapshot for visual regression
  await expect(terminal).toMatchSnapshot();
});

test("k1 filter functionality", async ({ terminal }) => {
  await expect(terminal.getByText("Pods")).toBeVisible();

  // Open filter with '/'
  terminal.write("/");

  // Type filter text
  terminal.write("nginx");
  terminal.submit("nginx");  // Press enter

  // Verify filtered results
  await expect(terminal.getByText(/nginx/)).toBeVisible();
});

test("k1 command palette", async ({ terminal }) => {
  await expect(terminal.getByText("Pods")).toBeVisible();

  // Open command palette with ':'
  terminal.write(":");

  // Verify palette appears
  await expect(
    terminal.getByText("Available Commands")
  ).toBeVisible();
});
```

### Integration Strategy for k1

#### Recommended Approach

**1. Create E2E Test Directory**
```
k1-tui-tests/
├── e2e/
│   ├── tests/
│   │   ├── navigation.test.ts
│   │   ├── filtering.test.ts
│   │   ├── commands.test.ts
│   │   └── themes.test.ts
│   ├── fixtures/
│   │   └── test-kubeconfig.yaml
│   └── __snapshots__/
├── package.json
├── tui-test.config.ts
└── tsconfig.json
```

**2. Add to Makefile**
```makefile
# Install Node.js dependencies
setup-e2e:
	npm install

# Run E2E tests
test-e2e:
	go build -o k1 cmd/k1/main.go
	npx @microsoft/tui-test
	rm k1

# Run all tests (unit + E2E)
test-all: test test-e2e
```

**3. Test Scenarios to Cover**

| Category | Tests |
|----------|-------|
| **Navigation** | Screen switching (p/d/s/n), back nav (Esc) |
| **Filtering** | Open filter (/), apply filter, clear filter |
| **Commands** | Palette (:), execute command, confirmation |
| **Keyboard** | Arrow keys, Enter, tab navigation |
| **Themes** | Switch theme (-theme flag), visual regression |
| **Resource Views** | All 11 resource types render correctly |
| **Error Handling** | Invalid input, missing kubeconfig |

**4. Use kind Cluster with Test Data**

**IMPORTANT**: Dummy mode (`-dummy` flag) is not implemented and will be
removed. Tests must use a real Kubernetes cluster.

**Setup kind cluster with ctlptl** (see "Test Cluster Setup" section):
```bash
make setup-test-cluster  # Creates kind cluster + test resources
```

**Configure tests to use kind cluster**:
```typescript
test.use({
  program: {
    file: "./k1",
    // No args - uses default kubeconfig context (kind-k1-test)
  }
});
```

This ensures:
- Realistic testing with real K8s API
- Predictable test data via fixtures
- Consistent across all environments

**5. Visual Regression Testing**
```typescript
test("deployments screen matches snapshot", async ({ terminal }) => {
  terminal.write("d");  // Navigate to deployments
  await expect(terminal.getByText("Deployments")).toBeVisible();
  await expect(terminal).toMatchSnapshot();
});
```

Snapshots stored in `e2e/__snapshots__/` for comparison.

### Comparison: Current k1 Testing vs tui-test

| Aspect | Current (Go Tests) | tui-test (E2E) |
|--------|-------------------|----------------|
| **Language** | Go | TypeScript |
| **Scope** | Unit/integration | End-to-end |
| **Method** | White-box (direct API) | Black-box (terminal I/O) |
| **Speed** | Fast (~5-10s total) | Slower (terminal startup) |
| **Coverage** | 76.7% code | User workflows |
| **Cluster** | envtest (fake K8s) | kind cluster (real K8s) |
| **CI** | ✅ Integrated | ⚠️ Requires Node.js |
| **Visual** | ❌ No UI validation | ✅ Screenshots |
| **Keyboard** | ❌ Not tested | ✅ Full interaction |

### Architecture Insights

**Current k1 Testing Strategy**
(from `thoughts/shared/research/2025-10-09-bubble-tea-tui-testing.md`):

1. **Repository Layer** (`internal/k8s/*`)
   - Uses envtest (real K8s API server locally)
   - Shared TestMain for fast suite execution
   - Namespace isolation per test
   - 76.7% coverage

2. **Screen Layer** (`internal/screens/*`)
   - Uses DummyRepository (mock data)
   - Tests configuration, transforms, operations
   - 71.0% coverage

3. **Command Layer** (`internal/commands/*`)
   - Uses inline mock repositories
   - Tests validation, messages, success paths
   - ~60% coverage

**Gaps tui-test Would Fill**:
- ✅ No current E2E tests of full TUI
- ✅ No keyboard interaction testing
- ✅ No visual regression testing
- ✅ No screen transition verification
- ✅ No user workflow validation

### Implementation Challenges

#### 1. Language Barrier (TypeScript vs Go)

**Challenge**: tui-test is TypeScript-only, k1 is Go.

**Solution**: Use tui-test for E2E only, keep Go tests for unit/integration.

**Trade-off**:
- ✅ Adds E2E coverage
- ⚠️ Requires Node.js in dev environment and CI
- ⚠️ Two test suites to maintain

#### 2. CI Integration

**Challenge**: GitHub Actions needs both Go and Node.js.

**Solution**: Add Node.js setup to CI workflow.

```yaml
# .github/workflows/test.yml
- name: Setup Node.js
  uses: actions/setup-node@v3
  with:
    node-version: '20'

- name: Install E2E dependencies
  run: npm ci

- name: Run E2E tests
  run: make test-e2e
```

#### 3. Snapshot Management

**Challenge**: Terminal snapshots may differ across environments.

**Solution**:
- Pin terminal dimensions in tests
- Use consistent font/theme for snapshots
- Consider platform-specific snapshots if needed

#### 4. Test Speed

**Challenge**: Terminal-based E2E tests slower than unit tests.

**Solution**:
- Run in parallel with `test.concurrent`
- Use persistent kind cluster (fast startup ~200ms)
- Reserve for critical user flows only (~10-20 tests)

### Recommended Next Steps

#### Phase 1: Proof of Concept (1-2 days)

1. **Setup**
   ```bash
   npm init -y
   npm install -D @microsoft/tui-test typescript
   npx tsc --init
   ```

2. **Write 2-3 Basic Tests**
   - Startup and pods screen render
   - Navigation between screens
   - Filter open/close

3. **Validate Approach**
   - Does it work with k1's Bubble Tea TUI?
   - Are tests reliable/deterministic?
   - What's the performance impact?

#### Phase 2: Core Workflows (3-5 days)

4. **Add Test Coverage**
   - All 11 resource screens
   - Command palette interactions
   - Filter and search
   - Theme switching

5. **Visual Regression**
   - Snapshot each screen
   - Verify layout consistency

#### Phase 3: CI Integration (1-2 days)

6. **Update CI Pipeline**
   - Add Node.js setup
   - Run E2E tests after build
   - Upload snapshots as artifacts

7. **Documentation**
   - Update CLAUDE.md with E2E testing section
   - Add README for e2e/ directory
   - Document snapshot update process

## Follow-up Research: Dummy Mode Removal and Test Cluster Setup

### Dummy Mode Status and Removal

**Current State**:
- **DummyRepository** exists (`internal/k8s/dummy_repository.go`, 664
  lines) and is actively used in unit tests
- **`-dummy` CLI flag** was removed from `cmd/k1/main.go` but
  references remain in:
  - `Makefile:36-37` - `run-dummy` target
  - `CLAUDE.md:44,48,94` - Documentation
  - `internal/components/commandbar/commandbar.go:34` - UI hints

**Issues**:
- User reports dummy mode is broken
- `-dummy` flag never implemented in main.go (only documented)
- DummyRepository provides static data unsuitable for E2E testing
- 50+ test files depend on DummyRepository for unit testing

**Recommended Actions**:

1. **Keep DummyRepository for unit tests** (important for fast
   feedback)
2. **Remove outdated references**:
   - Delete `run-dummy` target from Makefile
   - Remove `-dummy` mentions from CLAUDE.md
   - Remove "use -dummy" hint from commandbar.go
3. **Use kind cluster for E2E tests** (see next section)

**Files to Update**:
- `Makefile:36-37` - Remove `run-dummy` target
- `CLAUDE.md:44,48,94` - Remove `-dummy` flag documentation
- `internal/components/commandbar/commandbar.go:34` - Remove hint
- `thoughts/shared/research/2025-10-31-manual-tui-testing-guide.md` -
  Update to reflect removal

### Test Cluster Setup with ctlptl and kind

#### What is ctlptl?

**ctlptl** ("cattle patrol") is a declarative tool for managing local
Kubernetes development clusters. Created by Tilt team, it solves the
problem of manually configuring Docker Desktop, kind, Minikube, or k3d
clusters.

**Key Features**:
- Declarative YAML configuration (like kubectl for clusters)
- Automatic registry setup for local image testing
- Idempotent operations (safe to run multiple times)
- Remote Docker host support (useful for CI)
- Works with kind, k3d, Docker Desktop, Minikube

#### Installation

```bash
# Homebrew (Mac/Linux)
brew install tilt-dev/tap/ctlptl

# Scoop (Windows)
scoop bucket add tilt-dev https://github.com/tilt-dev/scoop-bucket
scoop install ctlptl

# Go install
go install github.com/tilt-dev/ctlptl/cmd/ctlptl@latest
```

#### kind Cluster Configuration for k1 E2E Tests

Create `e2e/kind-cluster.yaml`:

```yaml
apiVersion: ctlptl.dev/v1alpha1
kind: Cluster
product: kind
name: k1-test
registry: k1-registry
minCPUs: 2
kubernetesVersion: v1.31.0
```

**Why this configuration?**
- `name: k1-test` → Creates cluster named "kind-k1-test" in kubeconfig
- `registry: k1-registry` → Local registry for fast image loading
- `minCPUs: 2` → Adequate for test workloads
- `kubernetesVersion: v1.31.0` → Current stable version

#### Test Fixtures for Predictable Data

Create `e2e/fixtures/test-resources.yaml`:

```yaml
---
apiVersion: v1
kind: Namespace
metadata:
  name: test-app
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx-deployment
  namespace: test-app
spec:
  replicas: 3
  selector:
    matchLabels:
      app: nginx
  template:
    metadata:
      labels:
        app: nginx
    spec:
      containers:
      - name: nginx
        image: nginx:1.27
        ports:
        - containerPort: 80
---
apiVersion: v1
kind: Service
metadata:
  name: nginx-service
  namespace: test-app
spec:
  selector:
    app: nginx
  ports:
  - protocol: TCP
    port: 80
    targetPort: 80
  type: ClusterIP
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: test-config
  namespace: test-app
data:
  key1: value1
  key2: value2
---
apiVersion: v1
kind: Secret
metadata:
  name: test-secret
  namespace: test-app
type: Opaque
stringData:
  username: admin
  password: secret123
---
apiVersion: v1
kind: Pod
metadata:
  name: standalone-pod
  namespace: test-app
spec:
  containers:
  - name: busybox
    image: busybox:1.37
    command: ["sleep", "3600"]
---
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: web
  namespace: test-app
spec:
  serviceName: "nginx"
  replicas: 2
  selector:
    matchLabels:
      app: web
  template:
    metadata:
      labels:
        app: web
    spec:
      containers:
      - name: nginx
        image: nginx:1.27
        ports:
        - containerPort: 80
---
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: fluentd
  namespace: test-app
spec:
  selector:
    matchLabels:
      name: fluentd
  template:
    metadata:
      labels:
        name: fluentd
    spec:
      containers:
      - name: fluentd
        image: fluent/fluentd:v1.17
---
apiVersion: batch/v1
kind: Job
metadata:
  name: pi-job
  namespace: test-app
spec:
  template:
    spec:
      containers:
      - name: pi
        image: perl:5.40
        command: ["perl", "-Mbignum=bpi", "-wle", "print bpi(2000)"]
      restartPolicy: Never
  backoffLimit: 4
---
apiVersion: batch/v1
kind: CronJob
metadata:
  name: hello-cron
  namespace: test-app
spec:
  schedule: "*/5 * * * *"
  jobTemplate:
    spec:
      template:
        spec:
          containers:
          - name: hello
            image: busybox:1.37
            command: ["/bin/sh", "-c", "echo 'Hello from CronJob'"]
          restartPolicy: OnFailure
```

**Coverage**: This fixture creates resources for 9 of k1's 11 resource
screens:
- ✅ Pods (2 types: deployment-managed + standalone)
- ✅ Deployments (1)
- ✅ Services (1)
- ✅ ConfigMaps (1)
- ✅ Secrets (1)
- ✅ StatefulSets (1)
- ✅ DaemonSets (1)
- ✅ Jobs (1)
- ✅ CronJobs (1)
- ❌ Namespaces (created but not fixture-specific)
- ❌ Nodes (kind cluster nodes)

#### Makefile Integration

Add to `Makefile`:

```makefile
# E2E Test Cluster Management
.PHONY: setup-test-cluster teardown-test-cluster test-e2e

# Install ctlptl if not available
install-ctlptl:
	@which ctlptl > /dev/null || \
	(echo "Installing ctlptl..." && \
	brew install tilt-dev/tap/ctlptl)

# Create kind cluster with test resources
setup-test-cluster: install-ctlptl
	@echo "Creating kind cluster for E2E tests..."
	ctlptl apply -f e2e/kind-cluster.yaml
	@echo "Waiting for cluster to be ready..."
	kubectl wait --for=condition=Ready nodes --all --timeout=60s
	@echo "Applying test fixtures..."
	kubectl apply -f e2e/fixtures/test-resources.yaml
	@echo "Waiting for test resources to be ready..."
	kubectl wait --for=condition=Available \
	  deployment/nginx-deployment -n test-app --timeout=60s
	@echo "Test cluster ready!"
	@kubectl get pods -n test-app

# Teardown test cluster
teardown-test-cluster:
	ctlptl delete -f e2e/kind-cluster.yaml

# Run E2E tests (assumes cluster exists)
test-e2e: build
	@echo "Running E2E tests..."
	npx @microsoft/tui-test
	@echo "E2E tests complete!"

# Full E2E workflow: setup + test + teardown
test-e2e-full: setup-test-cluster test-e2e teardown-test-cluster
```

#### Usage Workflow

**One-time setup** (developers/CI):
```bash
make setup-test-cluster    # Creates cluster + fixtures
# ... run E2E tests multiple times ...
make teardown-test-cluster # Cleanup when done
```

**Daily development**:
```bash
# Cluster persists between sessions
make test-e2e              # Just run tests
```

**CI Pipeline**:
```yaml
# .github/workflows/test.yml
- name: Setup kind cluster
  run: make setup-test-cluster

- name: Run E2E tests
  run: make test-e2e

- name: Cleanup
  if: always()
  run: make teardown-test-cluster
```

#### Updated E2E Test Examples

With kind cluster, tests are more realistic:

```typescript
// e2e/tests/navigation.test.ts
import { test, expect } from "@microsoft/tui-test";

test.use({
  program: {
    file: "./k1",
    // Uses default kubeconfig context (kind-k1-test)
  }
});

test("navigate to test-app namespace", async ({ terminal }) => {
  // Wait for initial load
  await expect(terminal.getByText("Pods")).toBeVisible();

  // Open namespace filter
  terminal.write("/");
  await expect(terminal.getByText("Filter:")).toBeVisible();

  // Filter to test-app
  terminal.write("test-app");
  terminal.submit();

  // Should show nginx pods
  await expect(terminal.getByText(/nginx-deployment/)).toBeVisible();
});

test("deployment shows 3 replicas", async ({ terminal }) => {
  await expect(terminal.getByText("Pods")).toBeVisible();

  // Switch to deployments
  terminal.write("d");
  await expect(terminal.getByText("Deployments")).toBeVisible();

  // Find nginx-deployment with 3 replicas
  await expect(
    terminal.getByText(/nginx-deployment.*3\/3/)
  ).toBeVisible();
});

test("navigate from deployment to pods", async ({ terminal }) => {
  terminal.write("d");
  await expect(terminal.getByText("Deployments")).toBeVisible();

  // Select first deployment (arrow keys + enter)
  terminal.write("\r");  // Enter key

  // Should navigate to pods screen filtered by deployment
  await expect(terminal.getByText("Pods")).toBeVisible();
  await expect(
    terminal.getByText(/nginx-deployment-.*-.*/)
  ).toBeVisible();
});
```

#### Benefits of ctlptl + kind Approach

| Aspect | Benefit |
|--------|---------|
| **Declarative** | Cluster config in version control |
| **Idempotent** | Safe to run `setup-test-cluster` repeatedly |
| **Fast** | kind cluster starts in ~10-20s, reused between runs |
| **Realistic** | Real K8s API, real networking, real controllers |
| **CI-Friendly** | Same setup locally and in GitHub Actions |
| **Predictable** | Fixtures ensure consistent test data |
| **No Dummy Mode** | Tests against real k1 behavior |

### Alternative: VHS (Existing Approach)

**Note**: k1 already has research on VHS testing approach
(`thoughts/shared/research/2025-10-31-manual-tui-testing-guide.md`).

**VHS** (Video Hardware Store):
- Creates terminal recordings from .tape scripts
- Generates GIF/PNG/MP4 outputs
- Requires manual recording of interactions
- Claude can verify via screenshots
- Primarily for documentation/demos

#### Detailed Comparison: tui-test vs VHS

| Aspect | tui-test | VHS |
|--------|----------|-----|
| **Primary Purpose** | Automated E2E testing | Documentation & demos |
| **Automation Level** | ✅ Fully automated | ⚠️ Semi-automated (scripts) |
| **CI Integration** | ✅ Native test runner | ⚠️ Possible but manual |
| **Language** | TypeScript | .tape DSL |
| **Setup** | Node.js + npm | `go install` |
| **Test Assertions** | ✅ Programmatic (expect API) | ❌ Visual inspection only |
| **Debugging** | ✅ Traces, snapshots, logs | ✅ Visual output (GIF/PNG) |
| **Flaky Test Handling** | ✅ Auto-retry, auto-wait | ⚠️ Manual timing adjustments |
| **Test Speed** | Fast (parallel execution) | Slow (sequential recording) |
| **Output** | Pass/fail + snapshots | GIF/PNG/MP4 videos |
| **Coverage Measurement** | ❌ No | ❌ No |
| **Cluster Requirement** | ✅ kind cluster | ✅ kind cluster (same) |

#### Pros and Cons

**tui-test Pros**:
- ✅ **True E2E testing**: Validates actual behavior with assertions
- ✅ **Developer-friendly**: TypeScript with full IDE support
- ✅ **CI-native**: Integrates seamlessly with test pipelines
- ✅ **Fast feedback**: Runs in seconds, parallelizable
- ✅ **Regression detection**: Snapshots catch visual changes
- ✅ **Auto-wait**: No manual timing adjustments needed
- ✅ **Debugging tools**: Traces and detailed error messages

**tui-test Cons**:
- ❌ **Node.js dependency**: Adds language to Go project
- ❌ **Learning curve**: TypeScript for Go developers
- ❌ **Maintenance**: Test code needs updates with UI changes
- ⚠️ **Snapshot drift**: Cross-platform terminal differences

**VHS Pros**:
- ✅ **Visual output**: Beautiful GIFs for documentation
- ✅ **Simple scripts**: .tape files are easy to read/write
- ✅ **Go ecosystem**: Installed via `go install`
- ✅ **No dependencies**: Standalone binary
- ✅ **Demo-ready**: Output perfect for README/docs

**VHS Cons**:
- ❌ **No assertions**: Can't fail tests, only visual inspection
- ❌ **Manual verification**: Human must review each recording
- ❌ **Slow**: Sequential recording, not parallelizable
- ❌ **Not for CI**: Requires post-recording manual review
- ⚠️ **Timing-dependent**: Sleep commands need manual tuning
- ⚠️ **Brittle**: UI changes break recordings

#### When to Use Each

**Use tui-test when**:
- ✅ You need automated regression testing
- ✅ Tests must run in CI/CD pipeline
- ✅ Validating user workflows end-to-end
- ✅ Catching bugs before releases
- ✅ Testing keyboard interactions and navigation
- ✅ Ensuring UI consistency across versions

**Use VHS when**:
- ✅ Creating README/docs demonstrations
- ✅ Recording feature walkthroughs
- ✅ Showing off UI capabilities
- ✅ Marketing materials (GIFs for blog posts)
- ⚠️ Manual exploratory testing (with Claude verification)

#### Hybrid Approach (Recommended)

**Best of both worlds**:

1. **tui-test for testing** (10-20 tests)
   - Critical user workflows
   - Navigation flows
   - Filter/search functionality
   - Command palette interactions
   - Run in CI on every PR

2. **VHS for documentation** (2-5 recordings)
   - README hero GIF (quick demo)
   - Feature walkthroughs (blog posts)
   - Release announcement videos
   - Update manually when major UI changes

**Example workflow**:
```bash
# Development
make test-e2e              # Run automated tests

# Pre-release
make test-e2e-full         # Full test suite
./scripts/record-demos.sh  # Update VHS recordings

# Release
# - CI runs tui-test automatically
# - Include updated VHS GIFs in release notes
```

#### Example: Same Test in Both Frameworks

**tui-test version** (automated testing):
```typescript
test("filter pods by namespace", async ({ terminal }) => {
  await expect(terminal.getByText("Pods")).toBeVisible();
  terminal.write("/");
  await expect(terminal.getByText("Filter:")).toBeVisible();
  terminal.write("test-app");
  terminal.submit();
  await expect(terminal.getByText(/nginx-deployment/)).toBeVisible();
});
```

**VHS version** (documentation):
```tape
# Filter pods by namespace
Output demos/filter-pods.gif
Set FontSize 14
Set Width 1200
Set Height 600

Type "k1"
Sleep 2s
Type "/"
Sleep 500ms
Type "test-app"
Sleep 500ms
Enter
Sleep 2s
```

**Key differences**:
- tui-test: **Asserts** nginx-deployment appears
- VHS: **Records** whatever happens (no validation)

#### Cost-Benefit Analysis

**tui-test**:
- **Initial cost**: 2-3 days setup + 1 day per 10 tests
- **Ongoing cost**: Update tests when UI changes
- **Benefit**: Catch bugs before production, confidence in releases
- **ROI**: High for active development, prevents regressions

**VHS**:
- **Initial cost**: 1-2 hours setup + 30min per recording
- **Ongoing cost**: Re-record when UI changes (infrequent)
- **Benefit**: Beautiful demos, marketing materials
- **ROI**: High for documentation, low for testing

#### Recommendation

**For k1 project**:

1. **Adopt tui-test** for E2E testing
   - Focus on 10-15 critical workflows
   - Run in CI to catch regressions
   - Provides confidence in releases

2. **Keep VHS** for documentation
   - 2-3 key demos (README, features)
   - Update quarterly or for major releases
   - Great for showing off k1 capabilities

3. **Don't mix purposes**
   - tui-test = testing (automated)
   - VHS = documentation (manual)
   - Trying to use VHS for testing will be frustrating

## Code References

**Current Testing Infrastructure**:
- `internal/k8s/suite_test.go:19-55` - Shared envtest setup
- `internal/k8s/informer_repository_test.go:55-204` - Repository tests
- `internal/screens/screens_test.go:12-232` - Screen config tests
- `internal/commands/command_execution_test.go:14-96` - Command tests
- `internal/k8s/dummy_repository.go:1-664` - Mock data (unit tests only)

**Dummy Mode References** (to be removed):
- `cmd/k1/main.go` - No `-dummy` flag implementation (already removed)
- `Makefile:36-37` - `run-dummy` target (remove)
- `CLAUDE.md:44,48,94` - Documentation (remove)
- `internal/components/commandbar/commandbar.go:34` - Hint (remove)

**Build Configuration**:
- `Makefile:1-42` - Current targets (test, build, run)

## Related Research

- `thoughts/shared/research/2025-10-09-bubble-tea-tui-testing.md` -
  Current k1 testing strategy (envtest, 71-76% coverage)
- `thoughts/shared/research/2025-10-31-manual-tui-testing-guide.md` -
  VHS-based manual testing with Claude verification

## Open Questions

1. **Should k1 adopt tui-test?**
   - Pro: Fills E2E testing gap, validates user workflows
   - Con: Adds Node.js dependency, slower than unit tests
   - **Decision**: YES - Recommended for 10-15 critical workflows

2. **How many E2E tests are enough?**
   - **Answer**: 10-20 tests covering main user journeys
   - Keep unit tests (76.7% coverage) as primary mechanism
   - Focus E2E on integration and user workflows

3. **Snapshot management strategy?**
   - Pin terminal dimensions in tests (80x24 standard)
   - Use consistent theme for snapshots (charm default)
   - Platform-specific snapshots if needed (unlikely)
   - Update command: `npx @microsoft/tui-test --update-snapshots`

4. **Integration with existing Makefile?**
   - **Answer**: Separate `make test-e2e` target
   - Don't include in `make test` (too slow for TDD)
   - Optional in CI (can run on schedule vs every PR)

5. **Performance impact?**
   - Need to measure once implemented
   - Estimate: ~30-60s for 15 tests (2-4s each)
   - Compare: unit tests ~5-10s
   - Acceptable for CI, not for TDD loop

6. **Cluster lifecycle in CI?**
   - **Answer**: Create → Test → Teardown (ephemeral)
   - Locally: Keep cluster running between test runs
   - Total CI time: ~90-120s (cluster + tests + teardown)

## Conclusion

**tui-test is strongly recommended for k1 E2E testing** with the
following implementation plan:

### Final Recommendations

**1. Adopt tui-test + kind cluster approach**:
- ✅ Fills critical E2E testing gap
- ✅ Tests real user workflows
- ✅ Validates keyboard navigation
- ✅ Catches regressions before releases
- ⚠️ Requires Node.js (acceptable trade-off)

**2. Remove dummy mode completely**:
- Keep DummyRepository for unit tests (important!)
- Remove `-dummy` flag references from docs/Makefile
- Document that E2E tests use kind cluster

**3. Use ctlptl + kind for test infrastructure**:
- Declarative cluster configuration
- Predictable test fixtures
- Fast cluster reuse (~200ms startup when cached)
- Same setup locally and CI

**4. Hybrid testing strategy**:
- **Unit tests** (76.7%): Fast feedback, TDD, code coverage
- **E2E tests** (10-20): User workflows, integration, regression
- **VHS** (2-3): Documentation, demos, marketing

**5. Implementation phases**:

**Phase 1** (2-3 days):
- Setup ctlptl + kind cluster
- Create test fixtures (e2e/fixtures/)
- Install tui-test + write 2-3 proof-of-concept tests
- Validate approach

**Phase 2** (3-5 days):
- Write 10-15 E2E tests (navigation, filtering, commands)
- Add visual regression snapshots
- Document testing strategy
- Update CLAUDE.md

**Phase 3** (1-2 days):
- Integrate with CI (GitHub Actions)
- Remove dummy mode references
- Final cleanup and documentation

**Total effort**: 6-10 days for complete E2E testing infrastructure

### Key Decisions Made

| Question | Decision | Rationale |
|----------|----------|-----------|
| tui-test vs VHS? | Both (different purposes) | tui-test for testing, VHS for docs |
| Dummy mode? | Remove | Broken, not realistic, kind is better |
| Test cluster? | ctlptl + kind | Declarative, fast, CI-friendly |
| How many tests? | 10-20 | Focus on critical workflows |
| CI integration? | Yes (separate target) | Don't slow down TDD loop |
| Node.js dependency? | Accept | Worth it for E2E capabilities |

### Success Metrics

After implementation, k1 will have:
- ✅ 76.7% unit test coverage (existing)
- ✅ 10-20 E2E tests (new)
- ✅ Visual regression snapshots (new)
- ✅ CI automation (new)
- ✅ Realistic test environment (kind, not dummy)
- ✅ Documentation demos (VHS, existing)

## Can Claude Code Use tui-test to Verify Features?

**Short answer**: No, not directly. But Claude Code can help set up and
debug tests.

### What Claude Code CAN Do

**1. Setup and Implementation**:
- ✅ Write tui-test TypeScript test files
- ✅ Create kind cluster configurations
- ✅ Write test fixtures (YAML)
- ✅ Add Makefile targets
- ✅ Configure tui-test.config.ts
- ✅ Generate test scenarios based on requirements

**2. Indirect Testing**:
- ✅ Build k1 binary: `make build`
- ✅ Run tui-test suite: `npx @microsoft/tui-test`
- ✅ Read test output (pass/fail)
- ✅ Analyze terminal snapshots (PNG images)
- ✅ Debug test failures from logs
- ✅ Compare before/after snapshots

**3. Alternative Manual Testing**:
- ✅ Build and run k1 in background: `./k1 &`
- ❌ Cannot interact directly with TUI (no stdin control)
- ✅ Can view screenshots if you provide them
- ✅ Can analyze logs with `-log-file` flag

### What Claude Code CANNOT Do

**Direct TUI Interaction**:
- ❌ Cannot run interactive TUI programs (no PTY access)
- ❌ Cannot send keyboard input to running TUI
- ❌ Cannot see live TUI output
- ❌ Cannot verify UI appearance in real-time

**Reason**: Claude Code's Bash tool doesn't support interactive
programs requiring stdin/tty. Running `./k1` would hang waiting for
keyboard input that Claude cannot provide.

### How Claude Code Helps with tui-test

**Workflow Claude Can Automate**:

```bash
# 1. Claude builds k1
make build

# 2. Claude runs E2E tests
npx @microsoft/tui-test

# 3. Claude reads test results
# Output shows:
# ✓ k1 shows pods screen on startup (2.1s)
# ✓ k1 navigation with keyboard (1.8s)
# ✗ k1 filter functionality (failed)

# 4. Claude analyzes failures
# Reads error logs, stack traces
# Suggests fixes based on test output

# 5. Claude can view snapshots
# You share: e2e/__snapshots__/navigation.png
# Claude verifies: "Yes, I see the Deployments screen
# rendered correctly"
```

### Recommended Testing Workflow with Claude

**Option A: Automated (Best for Claude)**:

1. **You define requirements**: "I want to test navigation from
   deployments to pods"

2. **Claude writes test**:
   ```typescript
   test("navigate from deployment to pods", async ({ terminal }) => {
     terminal.write("d");
     await expect(terminal.getByText("Deployments")).toBeVisible();
     terminal.write("\r");
     await expect(terminal.getByText("Pods")).toBeVisible();
   });
   ```

3. **Claude runs test**: `npx @microsoft/tui-test`

4. **Claude reports results**: "Test passed! Snapshot saved to
   __snapshots__/navigation.png"

5. **You verify manually** (first time): Check PNG looks correct

6. **Future runs**: Claude detects regressions automatically

**Option B: Manual Testing + Claude Verification**:

1. **You run k1**: `./k1` (in your terminal)

2. **You take screenshot**: Screenshot the TUI

3. **You share with Claude**: Provide image path

4. **Claude analyzes**: "I can see the Pods screen with 3 nginx pods
   in the test-app namespace. The table shows NAME, NAMESPACE, STATUS,
   AGE columns correctly."

**Option C: VHS + Claude (Existing)**:

1. **You create VHS tape**: `vhs demos/navigation.tape`

2. **VHS generates PNG**: `demos/navigation.png`

3. **Claude reviews**: Analyzes the PNG for correctness

### Example: Claude's Role in E2E Testing

**Scenario**: Verify filter functionality works

**What Claude Does**:

```bash
# 1. Write the test
cat > e2e/tests/filter.test.ts << 'EOF'
import { test, expect } from "@microsoft/tui-test";

test.use({ program: { file: "./k1" } });

test("filter pods by namespace", async ({ terminal }) => {
  await expect(terminal.getByText("Pods")).toBeVisible();
  terminal.write("/");
  terminal.write("test-app");
  terminal.submit();
  await expect(terminal.getByText(/nginx/)).toBeVisible();
});
EOF

# 2. Run test
npx @microsoft/tui-test e2e/tests/filter.test.ts

# 3. Read output and report to you
# "Test passed in 2.3s"
# or
# "Test failed: Expected 'nginx' to be visible but found 'No results'"
```

**What You Do**:
- Review Claude's test code
- Confirm test results match expectations
- Investigate failures Claude can't see directly

### tui-test Advantages for Claude Code

**Why tui-test is better than manual testing for Claude**:

1. **Automated execution**: Claude can run tests without you
2. **Clear pass/fail**: No ambiguity in results
3. **Regression detection**: Snapshots catch visual changes
4. **Reproducible**: Same test runs the same way
5. **Fast feedback**: Claude gets results in seconds

**What Claude cannot do with manual TUI**:
- Cannot send keyboard input to `./k1`
- Cannot see what's rendered in your terminal
- Cannot verify UI state without screenshots from you

### Best Practices for Claude + tui-test

**1. Test-Driven Development with Claude**:
```
You: "Add filter by namespace feature"
Claude: "Let me write a test first..."
Claude: *writes failing test*
Claude: *implements feature*
Claude: *runs test*
Claude: "Test passes! Feature ready."
```

**2. Regression Prevention**:
```
You: "Refactor the table component"
Claude: *refactors code*
Claude: *runs E2E tests*
Claude: "All 15 tests pass, including visual snapshots. No
regressions."
```

**3. Bug Investigation**:
```
You: "Navigation is broken"
Claude: "Let me run the navigation tests..."
Claude: *runs tests*
Claude: "Test 'navigate from deployment to pods' failed. Expected
'Pods' screen but got 'Error loading resources'. Let me check the
logs..."
```

### Limitations to Remember

**Claude Code is not a manual tester**:
- ✗ Cannot click through UI manually
- ✗ Cannot "use the app" like a human
- ✗ Cannot verify "feel" or UX
- ✓ Can verify behavior through automated tests
- ✓ Can analyze screenshots you provide
- ✓ Can read test results and suggest fixes

**Think of Claude as**:
- A test engineer (writes/runs automated tests)
- A code reviewer (analyzes snapshots/logs)
- Not a QA tester (manual exploratory testing)

### Summary

**Can Claude Code verify k1 features?**

| Method | Claude Capability | Requires You |
|--------|------------------|--------------|
| **tui-test (automated)** | ✅ Write, run, analyze | Initial approval |
| **Manual TUI testing** | ❌ Cannot interact | Full manual testing |
| **Screenshot review** | ✅ Analyze provided images | Take screenshots |
| **VHS recordings** | ✅ Analyze PNG output | Create .tape files |
| **Log analysis** | ✅ Read k1 -log-file output | Trigger the issue |

**Best approach**: Combine automated tui-test (Claude runs) with
occasional manual testing (you verify UX).
