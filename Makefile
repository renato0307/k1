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
	@go build -o timoneiro cmd/timoneiro/main.go

# Clean build artifacts
clean:
	@rm -f timoneiro coverage.out

# Run the application with dummy data
run-dummy:
	@go run cmd/timoneiro/main.go -dummy

# Run the application with live cluster
run:
	@go run cmd/timoneiro/main.go
