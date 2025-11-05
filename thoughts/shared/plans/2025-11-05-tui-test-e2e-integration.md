# TUI-Test E2E Integration Implementation Plan

## Overview

Implement end-to-end testing infrastructure for k1 using Microsoft's
tui-test framework, enabling automated testing of user workflows,
keyboard navigation, and visual regression detection. This addresses the
current testing gap where unit tests (76.7% coverage) validate code
behavior but don't verify the actual TUI user experience.

## Current State Analysis

### Existing Test Infrastructure
- **Unit/Integration Tests**: 31 test files with 76.7% (k8s) and 71.0%
  (screens) coverage
- **Test Strategy**: envtest for k8s layer, DummyRepository for
  screens/commands
- **Execution Time**: ~5-10 seconds for full test suite
- **CI**: Only release workflow exists (`.github/workflows/release.yml`)
- **E2E Testing**: None - this is the gap we're filling

### Key Discoveries

**Dummy Mode is Broken and Will Be Removed**:
- `Makefile:36-37` references `run-dummy` target with `-dummy` flag
- `cmd/k1/main.go:37-59` shows no `-dummy` flag implementation
- `internal/k8s/dummy_repository.go:1-664` exists but for unit tests
  only
- Research document (lines 42-43) confirms removal plan
- **Action**: Keep DummyRepository for unit tests, remove `-dummy`
  references

**No Node.js Infrastructure**:
- No `package.json` exists in project root
- No TypeScript configuration files
- **Action**: Bootstrap Node.js/TypeScript setup from scratch

**Research is Complete**:
- `thoughts/shared/research/2025-11-05-tui-test-integration.md`
  (1356 lines)
- Provides complete implementation guide with examples
- Recommends ctlptl + kind for test clusters
- Suggests hybrid testing: tui-test (testing) + VHS (docs)

## Desired End State

After implementation, k1 will have:

1. **E2E Test Infrastructure**:
   - TypeScript-based tui-test setup with 15-20 tests
   - Tests covering navigation, filtering, command palette, context
     switching
   - Visual regression snapshots for all screens
   - Hybrid cluster management (ephemeral + persistent modes)

2. **Test Cluster Setup**:
   - ctlptl + kind configuration for reproducible test clusters
   - Test fixtures with predictable K8s resources
   - Makefile targets for cluster lifecycle management

3. **Clean Dummy Mode Removal**:
   - All `-dummy` references removed from docs and Makefile
   - DummyRepository retained for fast unit tests
   - Documentation updated to reflect E2E testing approach

4. **Developer Experience**:
   - `make setup-test-cluster` - One-command cluster setup
   - `make test-e2e` - Fast test execution (assumes cluster exists)
   - `make test-e2e-full` - Full ephemeral test run
   - Clear documentation in CLAUDE.md

### Verification

**Automated Verification**:
- [ ] `npm install` succeeds with no errors
- [ ] `npx tsc --noEmit` type-checks successfully
- [ ] `make setup-test-cluster` creates kind cluster + resources
- [ ] `kubectl get pods -n test-app` shows 4+ test pods
- [ ] `make build` compiles k1 binary successfully
- [ ] `make test-e2e` runs and reports pass/fail
- [ ] All 15+ E2E tests pass
- [ ] Snapshots generated in `e2e/__snapshots__/`

**Manual Verification**:
- [ ] kind cluster has predictable test resources (nginx deployment,
      services, etc.)
- [ ] E2E tests correctly navigate between screens (visual inspection)
- [ ] Filter tests show correct filtering behavior
- [ ] Command palette tests execute commands correctly
- [ ] Context switching tests handle multiple kubeconfigs
- [ ] Visual snapshots match expected screen layouts
- [ ] `make teardown-test-cluster` cleans up successfully
- [ ] Documentation is clear and complete

## What We're NOT Doing

**Out of Scope for This Plan**:
- CI/CD integration (GitHub Actions) - Future work per user request
- VHS-based documentation demos - Separate effort
- Performance/load testing - Not the goal of E2E tests
- Unit test migration - Keep existing 76.7% coverage as-is
- Modifying existing Go test infrastructure - E2E is additive
- Testing every edge case - Focus on critical user workflows (15-20
  tests)
- Cross-platform snapshot testing - Start with macOS/Linux only

## Implementation Approach

We'll implement E2E testing in three phases:

1. **Infrastructure Setup** (Phase 1): Bootstrap Node.js/TypeScript,
   create kind cluster config, set up test fixtures
2. **Core E2E Tests** (Phase 2): Write 15-20 tests covering critical
   workflows
3. **Cleanup & Documentation** (Phase 3): Remove dummy mode references,
   update docs

The hybrid cluster approach (ephemeral + persistent) ensures fast local
development while supporting future CI integration.

---

## Phase 1: Infrastructure Setup and Proof of Concept

### Overview
Bootstrap the E2E testing infrastructure: Node.js/TypeScript setup,
ctlptl + kind cluster configuration, test fixtures, and 2-3 proof of
concept tests to validate the approach.

### Changes Required

#### 1. Node.js and TypeScript Setup
**Files**: `package.json`, `tsconfig.json`, `tui-test.config.ts` (all
new)

**Changes**: Create Node.js project with tui-test dependencies

**`package.json`**:
```json
{
  "name": "k1-e2e-tests",
  "version": "1.0.0",
  "description": "E2E tests for k1 Kubernetes TUI",
  "private": true,
  "scripts": {
    "test": "tui-test",
    "test:debug": "tui-test --debug",
    "test:update-snapshots": "tui-test --update-snapshots",
    "typecheck": "tsc --noEmit"
  },
  "devDependencies": {
    "@microsoft/tui-test": "^0.2.0",
    "@types/node": "^20.10.0",
    "typescript": "^5.3.0"
  },
  "engines": {
    "node": ">=18.0.0"
  }
}
```

**`tsconfig.json`**:
```json
{
  "compilerOptions": {
    "target": "ES2022",
    "module": "commonjs",
    "lib": ["ES2022"],
    "outDir": "./dist",
    "rootDir": "./e2e",
    "strict": true,
    "esModuleInterop": true,
    "skipLibCheck": true,
    "forceConsistentCasingInFileNames": true,
    "resolveJsonModule": true,
    "moduleResolution": "node"
  },
  "include": ["e2e/**/*"],
  "exclude": ["node_modules", "dist"]
}
```

**`tui-test.config.ts`**:
```typescript
import { defineConfig } from "@microsoft/tui-test";

export default defineConfig({
  testDir: "./e2e/tests",
  timeout: 30000,
  retries: 2,
  workers: 4,
  use: {
    trace: "on-first-retry",
    screenshot: "only-on-failure",
  },
});
```

#### 2. kind Cluster Configuration
**File**: `e2e/kind-cluster.yaml` (new)

**Changes**: Create ctlptl configuration for reproducible test cluster

```yaml
apiVersion: ctlptl.dev/v1alpha1
kind: Cluster
product: kind
name: k1-test
registry: k1-registry
minCPUs: 2
kubernetesVersion: v1.31.0
```

**Why this configuration?**:
- `name: k1-test` creates cluster "kind-k1-test" in kubeconfig
- `registry: k1-registry` enables local image testing (future use)
- `minCPUs: 2` adequate for test workloads
- `kubernetesVersion: v1.31.0` current stable release

#### 3. Test Fixtures
**File**: `e2e/fixtures/test-resources.yaml` (new)

**Changes**: Create predictable K8s resources for testing

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

**Coverage**: Creates resources for 9 of 11 k1 screens (Pods,
Deployments, Services, ConfigMaps, Secrets, StatefulSets, DaemonSets,
Jobs, CronJobs)

#### 4. Makefile Integration
**File**: `Makefile`

**Changes**: Add E2E test targets (append to existing Makefile)

```makefile
# E2E Test Cluster Management
.PHONY: setup-test-cluster teardown-test-cluster test-e2e test-e2e-full

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
	@echo "Deleting kind cluster..."
	ctlptl delete -f e2e/kind-cluster.yaml
	@echo "Cluster deleted."

# Install Node.js dependencies
setup-e2e:
	@echo "Installing Node.js dependencies..."
	npm install
	@echo "Dependencies installed."

# Run E2E tests (assumes cluster exists)
test-e2e: build
	@echo "Running E2E tests..."
	npm test
	@echo "E2E tests complete!"

# Full E2E workflow: setup + test + teardown (ephemeral)
test-e2e-full: setup-test-cluster test-e2e teardown-test-cluster
	@echo "Full E2E test cycle complete!"
```

**Remove broken dummy mode target**:
```makefile
# DELETE THESE LINES (36-37):
run-dummy:
	@go run cmd/k1/main.go -dummy
```

#### 5. Proof of Concept Tests
**Directory**: `e2e/tests/` (new directory)

**File**: `e2e/tests/poc.test.ts` (new)

**Changes**: Write 3 basic tests to validate approach

```typescript
import { test, expect } from "@microsoft/tui-test";

// Configure k1 binary path (built by 'make build')
test.use({
  program: {
    file: "./k1",
    // No args - uses default kubeconfig context (kind-k1-test)
  },
});

test.describe("Proof of Concept - Basic Functionality", () => {
  test("k1 starts and shows Pods screen", async ({ terminal }) => {
    // Wait for initial screen render
    await expect(
      terminal.getByText("Pods", { full: true })
    ).toBeVisible({ timeout: 5000 });

    // Verify table headers are present
    await expect(terminal.getByText("NAMESPACE")).toBeVisible();
    await expect(terminal.getByText("NAME")).toBeVisible();
    await expect(terminal.getByText("READY")).toBeVisible();
    await expect(terminal.getByText("STATUS")).toBeVisible();
    await expect(terminal.getByText("AGE")).toBeVisible();
  });

  test("navigation to Deployments screen", async ({ terminal }) => {
    // Wait for initial pods screen
    await expect(terminal.getByText("Pods")).toBeVisible();

    // Press 'd' to switch to deployments
    terminal.write("d");

    // Verify deployments screen loads
    await expect(terminal.getByText("Deployments")).toBeVisible({
      timeout: 3000,
    });

    // Should see nginx-deployment from fixtures
    await expect(terminal.getByText(/nginx-deployment/)).toBeVisible();
  });

  test("filter functionality opens and accepts input", async ({
    terminal,
  }) => {
    await expect(terminal.getByText("Pods")).toBeVisible();

    // Open filter with '/'
    terminal.write("/");

    // Should show "Filter:" prompt
    await expect(terminal.getByText("Filter:")).toBeVisible({
      timeout: 2000,
    });

    // Type filter text
    terminal.write("test-app");

    // Should show typed text (basic assertion - detailed filtering in
    // Phase 2)
    await expect(terminal.getByText(/test-app/)).toBeVisible();

    // Close filter with Esc
    terminal.write("\x1b"); // ESC key
  });
});
```

#### 6. .gitignore Updates
**File**: `.gitignore`

**Changes**: Add Node.js and E2E artifacts

```gitignore
# Node.js
node_modules/
package-lock.json
npm-debug.log*

# E2E artifacts
e2e/__snapshots__/
e2e/test-results/
dist/
```

### Success Criteria

#### Automated Verification:
- [ ] `npm install` completes without errors
- [ ] `npx tsc --noEmit` type-checks successfully
- [ ] `make install-ctlptl` installs ctlptl (or confirms it's installed)
- [ ] `make setup-test-cluster` creates kind cluster without errors
- [ ] `kubectl config current-context` returns "kind-k1-test"
- [ ] `kubectl get pods -n test-app` shows 4 pods (nginx-deployment x3,
      standalone-pod)
- [ ] `make build` successfully compiles k1 binary (file exists: ./k1)
- [ ] `make test-e2e` runs and reports 3/3 tests passed
- [ ] `e2e/__snapshots__/` directory created with snapshots

#### Manual Verification:
- [ ] kind cluster has expected resources (test-app namespace,
      nginx-deployment with 3 replicas, services, configmaps, secrets)
- [ ] k1 binary runs correctly against kind-k1-test cluster manually
      (`./k1`)
- [ ] Pods screen shows test-app pods in terminal
- [ ] Deployments screen shows nginx-deployment when pressing 'd'
- [ ] Filter opens when pressing '/' and accepts input
- [ ] E2E test output is readable and shows clear pass/fail
- [ ] Snapshots in `e2e/__snapshots__/` are PNG files showing k1 screens
- [ ] `make teardown-test-cluster` successfully removes cluster
- [ ] Cluster can be recreated with `make setup-test-cluster`
      (idempotent)

**Implementation Note**: After completing this phase and all automated
verification passes, pause here for manual confirmation that the POC
tests work as expected before proceeding to Phase 2.

---

## Phase 2: Core E2E Test Coverage

### Overview
Expand test coverage to 15-20 tests covering all critical user
workflows: navigation across all 11 screens, filtering, command palette,
and context switching.

### Changes Required

#### 1. Navigation Tests
**File**: `e2e/tests/navigation.test.ts` (new)

**Changes**: Test keyboard navigation across all 11 resource screens

```typescript
import { test, expect } from "@microsoft/tui-test";

test.use({
  program: {
    file: "./k1",
  },
});

test.describe("Screen Navigation", () => {
  test("navigate through all resource screens", async ({ terminal }) => {
    // Start on Pods screen
    await expect(terminal.getByText("Pods")).toBeVisible();

    // Navigate to Deployments (d)
    terminal.write("d");
    await expect(terminal.getByText("Deployments")).toBeVisible();
    await expect(terminal.getByText(/nginx-deployment/)).toBeVisible();

    // Navigate to Services (s)
    terminal.write("s");
    await expect(terminal.getByText("Services")).toBeVisible();
    await expect(terminal.getByText(/nginx-service/)).toBeVisible();

    // Navigate to ConfigMaps (c)
    terminal.write("c");
    await expect(terminal.getByText("ConfigMaps")).toBeVisible();
    await expect(terminal.getByText(/test-config/)).toBeVisible();

    // Navigate to Secrets (shift+s via 'S')
    terminal.write("S");
    await expect(terminal.getByText("Secrets")).toBeVisible();
    await expect(terminal.getByText(/test-secret/)).toBeVisible();

    // Navigate to Namespaces (n)
    terminal.write("n");
    await expect(terminal.getByText("Namespaces")).toBeVisible();
    await expect(terminal.getByText(/test-app/)).toBeVisible();

    // Navigate to StatefulSets (shift+t via 'T')
    terminal.write("T");
    await expect(terminal.getByText("StatefulSets")).toBeVisible();
    await expect(terminal.getByText(/web/)).toBeVisible();

    // Navigate to DaemonSets (shift+d via 'D')
    terminal.write("D");
    await expect(terminal.getByText("DaemonSets")).toBeVisible();
    await expect(terminal.getByText(/fluentd/)).toBeVisible();

    // Navigate to Jobs (j)
    terminal.write("j");
    await expect(terminal.getByText("Jobs")).toBeVisible();
    await expect(terminal.getByText(/pi-job/)).toBeVisible();

    // Navigate to CronJobs (shift+j via 'J')
    terminal.write("J");
    await expect(terminal.getByText("CronJobs")).toBeVisible();
    await expect(terminal.getByText(/hello-cron/)).toBeVisible();

    // Navigate to Nodes (shift+n via 'N')
    terminal.write("N");
    await expect(terminal.getByText("Nodes")).toBeVisible();
    await expect(terminal.getByText(/control-plane/)).toBeVisible();

    // Navigate back to Pods (p)
    terminal.write("p");
    await expect(terminal.getByText("Pods")).toBeVisible();
  });

  test("back navigation from Deployments to Pods", async ({
    terminal,
  }) => {
    await expect(terminal.getByText("Pods")).toBeVisible();

    // Navigate to Deployments
    terminal.write("d");
    await expect(terminal.getByText("Deployments")).toBeVisible();

    // Select first deployment (assume cursor starts at first row)
    terminal.write("\r"); // Enter key

    // Should navigate to Pods screen filtered by deployment
    await expect(terminal.getByText("Pods")).toBeVisible({ timeout:
      3000 });

    // Should show nginx-deployment pods (generated names with hash)
    await expect(
      terminal.getByText(/nginx-deployment-[a-z0-9]+-[a-z0-9]+/)
    ).toBeVisible();
  });

  test("visual regression - all screens match snapshots", async ({
    terminal,
  }) => {
    const screens = [
      { key: "p", name: "Pods" },
      { key: "d", name: "Deployments" },
      { key: "s", name: "Services" },
      { key: "c", name: "ConfigMaps" },
      { key: "S", name: "Secrets" },
      { key: "n", name: "Namespaces" },
      { key: "T", name: "StatefulSets" },
      { key: "D", name: "DaemonSets" },
      { key: "j", name: "Jobs" },
      { key: "J", name: "CronJobs" },
      { key: "N", name: "Nodes" },
    ];

    for (const screen of screens) {
      terminal.write(screen.key);
      await expect(terminal.getByText(screen.name)).toBeVisible();
      await expect(terminal).toMatchSnapshot(`${screen.name.toLowerCase()}-screen.png`);
    }
  });
});
```

#### 2. Filtering and Search Tests
**File**: `e2e/tests/filtering.test.ts` (new)

**Changes**: Test filter activation, text input, and filtering behavior

```typescript
import { test, expect } from "@microsoft/tui-test";

test.use({
  program: {
    file: "./k1",
  },
});

test.describe("Filtering and Search", () => {
  test("filter pods by namespace", async ({ terminal }) => {
    await expect(terminal.getByText("Pods")).toBeVisible();

    // Open filter
    terminal.write("/");
    await expect(terminal.getByText("Filter:")).toBeVisible();

    // Type namespace filter
    terminal.write("test-app");
    terminal.submit(); // Press Enter

    // Should show only test-app namespace pods
    await expect(terminal.getByText(/test-app/)).toBeVisible();

    // Should show nginx pods from test-app
    await expect(
      terminal.getByText(/nginx-deployment|standalone-pod/)
    ).toBeVisible();
  });

  test("filter deployments by name", async ({ terminal }) => {
    // Navigate to deployments
    terminal.write("d");
    await expect(terminal.getByText("Deployments")).toBeVisible();

    // Open filter
    terminal.write("/");
    await expect(terminal.getByText("Filter:")).toBeVisible();

    // Filter for nginx
    terminal.write("nginx");
    terminal.submit();

    // Should show nginx-deployment
    await expect(terminal.getByText(/nginx-deployment/)).toBeVisible();
  });

  test("clear filter with Esc", async ({ terminal }) => {
    await expect(terminal.getByText("Pods")).toBeVisible();

    // Apply filter
    terminal.write("/");
    await expect(terminal.getByText("Filter:")).toBeVisible();
    terminal.write("test-app");
    terminal.submit();

    // Filter should be active (shows filtered results)
    await expect(terminal.getByText(/test-app/)).toBeVisible();

    // Clear filter by opening and pressing Esc
    terminal.write("/");
    terminal.write("\x1b"); // ESC key

    // Filter should be cleared (may show pods from other namespaces)
    // Note: Exact assertion depends on what other namespaces exist
    await expect(terminal.getByText("Pods")).toBeVisible();
  });

  test("filter persists across screen refresh", async ({ terminal }) => {
    await expect(terminal.getByText("Pods")).toBeVisible();

    // Apply filter
    terminal.write("/");
    terminal.write("test-app");
    terminal.submit();

    // Verify filter applied
    await expect(terminal.getByText(/test-app/)).toBeVisible();

    // Trigger refresh (implementation-specific - may need to press 'r'
    // or Ctrl+R)
    // For now, assume filter persists by default
    // TODO: Verify filter persistence behavior with actual k1
  });
});
```

#### 3. Command Palette Tests
**File**: `e2e/tests/command-palette.test.ts` (new)

**Changes**: Test command palette activation and command execution

```typescript
import { test, expect } from "@microsoft/tui-test";

test.use({
  program: {
    file: "./k1",
  },
});

test.describe("Command Palette", () => {
  test("open command palette with colon", async ({ terminal }) => {
    await expect(terminal.getByText("Pods")).toBeVisible();

    // Open command palette
    terminal.write(":");

    // Should show command prompt or palette
    await expect(
      terminal.getByText(/Command|Available Commands|:/)
    ).toBeVisible({ timeout: 2000 });
  });

  test("close command palette with Esc", async ({ terminal }) => {
    await expect(terminal.getByText("Pods")).toBeVisible();

    // Open command palette
    terminal.write(":");
    await expect(
      terminal.getByText(/Command|Available Commands|:/)
    ).toBeVisible();

    // Close with Esc
    terminal.write("\x1b"); // ESC key

    // Should return to normal screen (command palette closed)
    await expect(terminal.getByText("Pods")).toBeVisible();
  });

  test("command palette shows available commands", async ({ terminal
  }) => {
    await expect(terminal.getByText("Pods")).toBeVisible();

    // Open command palette
    terminal.write(":");
    await expect(
      terminal.getByText(/Command|Available Commands/)
    ).toBeVisible();

    // Should list some commands (exact commands depend on k1
    // implementation)
    // For now, take snapshot to verify visually
    await expect(terminal).toMatchSnapshot("command-palette.png");
  });

  test("execute describe command via palette", async ({ terminal }) => {
    // Navigate to pods screen
    await expect(terminal.getByText("Pods")).toBeVisible();

    // Select first pod (cursor should start at first row)
    // Depending on implementation, may need arrow keys to select

    // Open command palette
    terminal.write(":");

    // Type describe command
    terminal.write("describe");
    terminal.submit();

    // Should show describe output (YAML or formatted text)
    // Exact assertion depends on describe output format
    await expect(
      terminal.getByText(/Name:|Namespace:|Labels:/)
    ).toBeVisible({ timeout: 3000 });
  });
});
```

#### 4. Context Switching Tests
**File**: `e2e/tests/context-switching.test.ts` (new)

**Changes**: Test switching between multiple Kubernetes contexts

```typescript
import { test, expect } from "@microsoft/tui-test";
import * as fs from "fs";
import * as path from "path";
import * as os from "os";

// Helper to create multi-context kubeconfig
function createMultiContextKubeconfig(): string {
  const testKubeconfigPath = path.join(
    os.tmpdir(),
    `k1-test-kubeconfig-${Date.now()}.yaml`
  );

  // Read the default kind-k1-test kubeconfig
  const homeDir = os.homedir();
  const defaultKubeconfig = path.join(homeDir, ".kube", "config");
  const kubeconfigContent = fs.readFileSync(defaultKubeconfig, "utf8");

  // For POC, we'll use the same cluster but create a second context
  // pointing to it
  // In a real scenario, you'd have multiple clusters
  const multiContextConfig = kubeconfigContent.replace(
    /current-context: kind-k1-test/,
    `current-context: kind-k1-test
contexts:
- context:
    cluster: kind-k1-test
    user: kind-k1-test
  name: kind-k1-test
- context:
    cluster: kind-k1-test
    user: kind-k1-test
  name: kind-k1-test-secondary`
  );

  fs.writeFileSync(testKubeconfigPath, multiContextConfig);
  return testKubeconfigPath;
}

test.describe("Context Switching", () => {
  let testKubeconfigPath: string;

  test.beforeAll(() => {
    testKubeconfigPath = createMultiContextKubeconfig();
  });

  test.afterAll(() => {
    if (testKubeconfigPath && fs.existsSync(testKubeconfigPath)) {
      fs.unlinkSync(testKubeconfigPath);
    }
  });

  test("show current context in UI", async ({ terminal }) => {
    // Launch k1 with custom kubeconfig
    test.use({
      program: {
        file: "./k1",
        args: ["--kubeconfig", testKubeconfigPath],
      },
    });

    await expect(terminal.getByText("Pods")).toBeVisible();

    // Should show current context somewhere in UI (status line, title,
    // etc.)
    await expect(terminal.getByText(/kind-k1-test/)).toBeVisible();
  });

  test("switch context via command", async ({ terminal }) => {
    test.use({
      program: {
        file: "./k1",
        args: [
          "--kubeconfig",
          testKubeconfigPath,
          "--context",
          "kind-k1-test",
          "--context",
          "kind-k1-test-secondary",
        ],
      },
    });

    await expect(terminal.getByText("Pods")).toBeVisible();

    // Open command palette
    terminal.write(":");

    // Execute context switch command (exact command depends on k1
    // implementation)
    terminal.write("context kind-k1-test-secondary");
    terminal.submit();

    // Should show context switched message or update UI
    await expect(
      terminal.getByText(/kind-k1-test-secondary|Context switched/)
    ).toBeVisible({ timeout: 3000 });
  });
});
```

#### 5. Test Utilities
**File**: `e2e/helpers/test-utils.ts` (new)

**Changes**: Shared utilities for tests

```typescript
import { Terminal } from "@microsoft/tui-test";

/**
 * Wait for k1 to fully load (pods screen visible)
 */
export async function waitForK1Ready(
  terminal: Terminal,
  timeout: number = 5000
): Promise<void> {
  await terminal.waitForText("Pods", { timeout });
}

/**
 * Navigate to a specific screen by key
 */
export async function navigateToScreen(
  terminal: Terminal,
  key: string,
  screenName: string
): Promise<void> {
  terminal.write(key);
  await terminal.waitForText(screenName, { timeout: 3000 });
}

/**
 * Open filter and type query
 */
export async function applyFilter(
  terminal: Terminal,
  query: string
): Promise<void> {
  terminal.write("/");
  await terminal.waitForText("Filter:", { timeout: 2000 });
  terminal.write(query);
  terminal.submit();
}

/**
 * Clear filter
 */
export async function clearFilter(terminal: Terminal): Promise<void> {
  terminal.write("/");
  terminal.write("\x1b"); // ESC
}

/**
 * Open command palette
 */
export async function openCommandPalette(terminal: Terminal):
  Promise<void> {
  terminal.write(":");
  await terminal.waitForText(/Command|:/, { timeout: 2000 });
}
```

#### 6. Update package.json Scripts
**File**: `package.json`

**Changes**: Add test scripts for specific test files

```json
{
  "scripts": {
    "test": "tui-test",
    "test:debug": "tui-test --debug",
    "test:update-snapshots": "tui-test --update-snapshots",
    "test:navigation": "tui-test e2e/tests/navigation.test.ts",
    "test:filtering": "tui-test e2e/tests/filtering.test.ts",
    "test:palette": "tui-test e2e/tests/command-palette.test.ts",
    "test:context": "tui-test e2e/tests/context-switching.test.ts",
    "typecheck": "tsc --noEmit"
  }
}
```

### Success Criteria

#### Automated Verification:
- [ ] `npm run test:navigation` passes all navigation tests
- [ ] `npm run test:filtering` passes all filtering tests
- [ ] `npm run test:palette` passes all command palette tests
- [ ] `npm run test:context` passes all context switching tests
- [ ] `npm test` runs all 15+ tests and reports pass
- [ ] `npx tsc --noEmit` type-checks with no errors
- [ ] Snapshots generated for all 11 resource screens
- [ ] No flaky tests (retry count < 2)

#### Manual Verification:
- [ ] Navigation test correctly visits all 11 resource screens
- [ ] Each screen shows expected resources from test fixtures
- [ ] Filtering correctly narrows down results to matching items
- [ ] Filter can be cleared with Esc and shows all results again
- [ ] Command palette opens and shows available commands
- [ ] Describe command execution shows resource details
- [ ] Context switching updates the UI to show new context
- [ ] All snapshots in `e2e/__snapshots__/` are legible and show correct
      screens
- [ ] Test execution time is reasonable (< 2 minutes total)
- [ ] Test output is clear and identifies failures precisely

**Implementation Note**: After completing this phase and all automated
verification passes, pause here for manual confirmation that all E2E
tests pass reliably before proceeding to Phase 3.

---

## Phase 3: Cleanup, Documentation, and Polish

### Overview
Remove dummy mode references, update documentation, add README for E2E
tests, and ensure developer experience is smooth.

### Changes Required

#### 1. Remove Dummy Mode References
**Files**: `Makefile`, `CLAUDE.md`, `internal/components/commandbar/commandbar.go`

**Changes**: Delete all references to broken `-dummy` flag

**`Makefile`** (lines 35-37):
```makefile
# DELETE THESE LINES:
# Run the application with dummy data
run-dummy:
	@go run cmd/k1/main.go -dummy
```

**`CLAUDE.md`** (lines 44, 48, 94):
```markdown
# FIND AND REMOVE references to -dummy flag:
# OLD: "go run cmd/k1/main.go -dummy  # UI dev without cluster"
# OLD: "make run-dummy         # Run with mock data"
# NEW: Use kind cluster for testing instead

# UPDATE Quick Commands section:
make run               # Run with live cluster
make run-test-cluster  # Run against kind test cluster

# ADD New section:
### E2E Testing
make setup-test-cluster  # Create kind cluster for E2E tests
make test-e2e           # Run E2E tests (assumes cluster exists)
make test-e2e-full      # Full test cycle (setup + test + teardown)
make teardown-test-cluster  # Delete test cluster
```

**`internal/components/commandbar/commandbar.go:34`**:
```go
// FIND AND REMOVE or UPDATE hint text:
// OLD: "use -dummy for offline testing"
// NEW: (remove entirely or update to mention kind cluster)
```

#### 2. Add E2E Testing Documentation
**File**: `CLAUDE.md`

**Changes**: Add comprehensive E2E testing section

Insert after existing "Testing Strategy" section (~line 117):

```markdown
## E2E Testing with tui-test

k1 uses Microsoft's tui-test framework for end-to-end testing of user
workflows.

### Quick Start

```bash
# One-time setup
npm install              # Install tui-test
make setup-test-cluster  # Create kind cluster + fixtures

# Run tests
make test-e2e            # Fast (assumes cluster exists)

# Cleanup
make teardown-test-cluster  # When done
```

### Test Organization

**Test Files** (`e2e/tests/`):
- `navigation.test.ts` - Screen switching, visual regression
- `filtering.test.ts` - Filter activation, query, clear
- `command-palette.test.ts` - Command execution
- `context-switching.test.ts` - Multi-context management

**Fixtures** (`e2e/fixtures/test-resources.yaml`):
- Predictable K8s resources in `test-app` namespace
- Covers 9 of 11 resource types (Pods, Deployments, Services, etc.)

**Configuration**:
- `tui-test.config.ts` - Test runner config (retries, timeouts)
- `e2e/kind-cluster.yaml` - ctlptl cluster definition

### Running Tests

**Local Development** (persistent cluster):
```bash
make setup-test-cluster  # Once
make test-e2e           # Many times (fast: ~30s)
```

**Full Test Cycle** (ephemeral):
```bash
make test-e2e-full  # Setup + test + teardown (~90-120s)
```

**Specific Test Files**:
```bash
npm run test:navigation  # Just navigation tests
npm run test:filtering   # Just filtering tests
npm run test:palette     # Just command palette tests
npm run test:context     # Just context tests
```

**Update Snapshots** (after UI changes):
```bash
npm run test:update-snapshots
```

### Test Strategy

**What E2E Tests Cover**:
- ✅ User workflows (navigation, filtering, commands)
- ✅ Keyboard interactions
- ✅ Visual regression (snapshots)
- ✅ Multi-context behavior

**What Unit Tests Cover**:
- ✅ Code logic (76.7% coverage)
- ✅ Repository operations (envtest)
- ✅ Screen configuration
- ✅ Command validation

**Complementary Approaches**:
- **Unit tests**: Fast feedback (5-10s), TDD, code coverage
- **E2E tests**: User workflows (30s), integration, visual regression

### Troubleshooting

**Tests fail with "cluster not found"**:
```bash
make setup-test-cluster  # Create cluster first
kubectl config use-context kind-k1-test  # Switch to test cluster
```

**Snapshots differ**:
```bash
npm run test:update-snapshots  # Regenerate after intentional UI changes
```

**Test timeouts**:
- Check if kind cluster is healthy: `kubectl get nodes`
- Increase timeout in `tui-test.config.ts` if needed

**Flaky tests**:
- tui-test has built-in retries (configured to 2)
- If a test flakes often, increase wait timeouts in test code
```

#### 3. Create E2E README
**File**: `e2e/README.md` (new)

**Changes**: Detailed guide for E2E testing

```markdown
# k1 E2E Testing

End-to-end tests for k1 Kubernetes TUI using Microsoft's tui-test
framework.

## Architecture

**Framework**: [@microsoft/tui-test](https://github.com/microsoft/tui-test)
- Launches k1 binary in isolated terminal
- Simulates keyboard input
- Captures and asserts on terminal output
- Takes snapshots for visual regression

**Test Cluster**: kind + ctlptl
- Declarative cluster configuration
- Predictable test fixtures
- Fast reuse (~200ms) or ephemeral setup (~30s)

## Directory Structure

```
e2e/
├── tests/              # Test files
│   ├── navigation.test.ts
│   ├── filtering.test.ts
│   ├── command-palette.test.ts
│   └── context-switching.test.ts
├── helpers/            # Shared utilities
│   └── test-utils.ts
├── fixtures/           # K8s test resources
│   └── test-resources.yaml
├── kind-cluster.yaml   # Cluster configuration
└── __snapshots__/      # Visual regression snapshots (gitignored)
```

## Prerequisites

**System Requirements**:
- Node.js 18+
- Docker Desktop (for kind)
- kubectl
- Go 1.24+ (for building k1)

**One-Time Setup**:
```bash
# Install Node.js dependencies
npm install

# Install ctlptl (cluster management)
brew install tilt-dev/tap/ctlptl

# Create test cluster
make setup-test-cluster
```

## Running Tests

**Fast Mode** (assumes cluster exists):
```bash
make test-e2e
# or
npm test
```

**Full Cycle** (ephemeral):
```bash
make test-e2e-full
```

**Specific Tests**:
```bash
npm run test:navigation  # Navigation tests only
npm run test:filtering   # Filtering tests only
npm run test:palette     # Command palette only
npm run test:context     # Context switching only
```

**Debug Mode**:
```bash
npm run test:debug
```

## Writing Tests

**Test Structure**:
```typescript
import { test, expect } from "@microsoft/tui-test";

test.use({
  program: {
    file: "./k1",  // Path to k1 binary
  },
});

test("my test", async ({ terminal }) => {
  // Wait for k1 to load
  await expect(terminal.getByText("Pods")).toBeVisible();

  // Simulate keyboard input
  terminal.write("d");  // Press 'd' key

  // Assert on output
  await expect(terminal.getByText("Deployments")).toBeVisible();

  // Visual regression
  await expect(terminal).toMatchSnapshot("deployments-screen.png");
});
```

**Best Practices**:
- Use `await expect(...).toBeVisible()` for async assertions
- Set appropriate timeouts for screen transitions (2-3s)
- Use `.toMatchSnapshot()` for visual regression
- Extract common patterns to `e2e/helpers/test-utils.ts`

## Test Fixtures

**Resources in test-app namespace**:
- nginx-deployment (3 replicas)
- nginx-service (ClusterIP)
- standalone-pod (busybox)
- test-config (ConfigMap)
- test-secret (Secret)
- web (StatefulSet, 2 replicas)
- fluentd (DaemonSet)
- pi-job (Job)
- hello-cron (CronJob)

**Modify fixtures**:
```bash
# Edit fixtures
vim e2e/fixtures/test-resources.yaml

# Apply changes
kubectl apply -f e2e/fixtures/test-resources.yaml
```

## Cluster Management

**Check cluster status**:
```bash
kubectl config current-context  # Should show "kind-k1-test"
kubectl get pods -n test-app    # Should show test resources
```

**Recreate cluster**:
```bash
make teardown-test-cluster
make setup-test-cluster
```

**Cluster configuration**:
- File: `e2e/kind-cluster.yaml`
- Cluster name: `k1-test` (kubeconfig context: `kind-k1-test`)
- Kubernetes version: v1.31.0
- Registry: `k1-registry` (for future use)

## Snapshots

**Location**: `e2e/__snapshots__/`

**Update after UI changes**:
```bash
npm run test:update-snapshots
```

**Review changes**:
```bash
git diff e2e/__snapshots__/
```

**Note**: Snapshots are gitignored. Commit manually if needed for
baseline.

## Troubleshooting

**Problem**: Tests fail with "command not found"
**Solution**: Run `make build` first to compile k1 binary

**Problem**: Tests timeout waiting for screen
**Solution**: Check if cluster is healthy (`kubectl get nodes`), rebuild
k1 binary

**Problem**: Tests flake occasionally
**Solution**: Increase timeouts in test code or `tui-test.config.ts`
(retries: 2 by default)

**Problem**: Snapshots differ on CI vs local
**Solution**: Pin terminal dimensions in tests, use consistent theme

## CI Integration (Future)

**Not yet implemented** - when ready:
1. Add GitHub Actions workflow
2. Use ephemeral clusters (`make test-e2e-full`)
3. Upload snapshots as artifacts
4. Run on PR or nightly

## References

- tui-test docs: https://github.com/microsoft/tui-test
- ctlptl docs: https://github.com/tilt-dev/ctlptl
- kind docs: https://kind.sigs.k8s.io/
- Research: `thoughts/shared/research/2025-11-05-tui-test-integration.md`
```

#### 4. Update Main README
**File**: `README.md`

**Changes**: Add E2E testing section

Insert before "Development" section:

```markdown
## Testing

k1 uses a comprehensive testing strategy:

**Unit/Integration Tests** (76.7% coverage):
```bash
make test              # Run all Go tests
make test-coverage     # View coverage report
```

**E2E Tests** (user workflows):
```bash
npm install                 # One-time: Install tui-test
make setup-test-cluster     # One-time: Create kind cluster
make test-e2e              # Run E2E tests
```

See [e2e/README.md](e2e/README.md) for detailed E2E testing guide.
```

#### 5. Add .gitignore for E2E Artifacts
**File**: `.gitignore`

**Changes**: Ensure E2E artifacts are properly ignored

```gitignore
# E2E Test Artifacts
e2e/__snapshots__/*.png
e2e/test-results/
e2e/playwright-report/
e2e/*.log
```

### Success Criteria

#### Automated Verification:
- [ ] `make build` succeeds
- [ ] `grep -r "\-dummy" Makefile` returns no results
- [ ] `grep -r "use -dummy" CLAUDE.md` returns no results
- [ ] `grep -r "dummy.*offline" internal/` returns no results (or
      updated hint)
- [ ] `make test-e2e` runs and all tests pass
- [ ] README.md includes E2E testing section
- [ ] e2e/README.md exists and is complete

#### Manual Verification:
- [ ] CLAUDE.md has comprehensive E2E testing section (not just a few
      lines)
- [ ] E2E README is clear and complete with all commands, troubleshooting
- [ ] Main README mentions E2E tests with link to e2e/README.md
- [ ] No references to dummy mode remain in any documentation
- [ ] All Makefile targets work as documented (setup, test, teardown)
- [ ] New developer can follow e2e/README.md and run tests successfully
- [ ] Documentation explains the difference between unit tests (76.7%
      coverage) and E2E tests (workflows)

**Implementation Note**: After completing this phase, the E2E testing
infrastructure is complete and ready for daily use.

---

## Testing Strategy

### Unit Tests (Existing, Keep As-Is)
**Approach**: envtest + DummyRepository
**Coverage**: 76.7% (k8s), 71.0% (screens)
**Speed**: Fast (~5-10 seconds)
**Use Cases**:
- TDD workflow
- Code coverage
- Repository operations
- Screen configuration
- Command validation

**Testing Pattern**:
```bash
make test              # Run all unit tests
make test-coverage     # Check coverage
```

### E2E Tests (New, This Plan)
**Approach**: tui-test + kind cluster
**Coverage**: 15-20 critical workflows
**Speed**: Moderate (~30 seconds with persistent cluster)
**Use Cases**:
- User workflow validation
- Keyboard interaction testing
- Visual regression (snapshots)
- Integration testing

**Testing Pattern**:
```bash
make setup-test-cluster  # Once
make test-e2e           # Many times
```

### Test Cluster Strategy

**Persistent Mode** (local development):
```bash
make setup-test-cluster  # Once
make test-e2e           # Fast: ~30s per run
```

**Ephemeral Mode** (CI-ready):
```bash
make test-e2e-full  # Setup + test + teardown: ~90-120s
```

**Benefits of Hybrid Approach**:
- Fast iteration locally (persistent cluster)
- Clean CI runs (ephemeral cluster)
- Easy to switch between modes

## Performance Considerations

**Unit Tests**: ~5-10 seconds (unchanged)
**E2E Tests**: ~30 seconds (persistent cluster) or ~90-120 seconds
(ephemeral)

**Optimization Strategy**:
- Run unit tests frequently (TDD)
- Run E2E tests before commits
- Use parallel test execution (tui-test workers: 4)
- Snapshot updates only when UI changes

**Expected Developer Workflow**:
1. TDD: `make test` (5-10s feedback loop)
2. Feature complete: `make test-e2e` (30s validation)
3. Before PR: `make test-e2e-full` (90-120s clean run)

## Migration Notes

**No Migration Needed**:
- Existing unit tests unchanged
- DummyRepository stays for unit tests
- Go test infrastructure unchanged

**Additive Changes**:
- Node.js/TypeScript added to project
- E2E tests supplement (not replace) unit tests
- New Makefile targets added

**Developer Impact**:
- Must run `npm install` once
- Must create kind cluster once (`make setup-test-cluster`)
- E2E tests optional for TDD, required before merging

## References

- Original research:
  `thoughts/shared/research/2025-11-05-tui-test-integration.md`
- tui-test documentation: https://github.com/microsoft/tui-test
- ctlptl documentation: https://github.com/tilt-dev/ctlptl
- kind documentation: https://kind.sigs.k8s.io/
- Current test coverage: `make test-coverage` (76.7% k8s, 71.0% screens)
