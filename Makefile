.PHONY: test test-coverage setup-envtest build clean

# Setup envtest binaries
setup-envtest:
	@echo "Installing setup-envtest..."
	@go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest
	@echo "Downloading envtest binaries..."
	@setup-envtest use

# Run tests with envtest
test:
	@export KUBEBUILDER_ASSETS=$$(setup-envtest use -p path) && \
	go test -v ./... -timeout 60s

# Run tests with coverage
test-coverage:
	@export KUBEBUILDER_ASSETS=$$(setup-envtest use -p path) && \
	go test -v ./... -coverprofile=coverage.out -timeout 60s && \
	go tool cover -func=coverage.out

# View coverage in browser
test-coverage-html:
	@export KUBEBUILDER_ASSETS=$$(setup-envtest use -p path) && \
	go test -v ./... -coverprofile=coverage.out -timeout 60s && \
	go tool cover -html=coverage.out

# Build the application
build:
	@go build -o k1 cmd/k1/main.go

# Clean build artifacts
clean:
	@rm -f k1 coverage.out

# Run the application with live cluster
run:
	@go run cmd/k1/main.go

# E2E Test Cluster Management
.PHONY: setup-test-cluster teardown-test-cluster test-e2e test-e2e-with-cluster install-ctlptl test-all

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

# Run E2E tests (assumes cluster exists and context is set to kind-k1-test)
test-e2e:
	@echo "Running E2E tests..."
	@go test -v -tags=e2e ./internal/... -timeout=2m
	@echo "E2E tests complete!"

# Run E2E tests with cluster setup
test-e2e-with-cluster: setup-test-cluster test-e2e

# Run all tests (unit + E2E)
test-all: test test-e2e
