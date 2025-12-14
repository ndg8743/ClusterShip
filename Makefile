# ClusterShip Makefile
# Cross-platform build and test automation

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOVET=$(GOCMD) vet
GOFMT=gofmt

# Binary name
BINARY_NAME=clustership
BINARY_UNIX=$(BINARY_NAME)_unix
BINARY_LINUX=$(BINARY_NAME)_linux
BINARY_DARWIN=$(BINARY_NAME)_darwin
BINARY_WINDOWS=$(BINARY_NAME).exe

# Directories
CMD_DIR=./cmd/clustership
BUILD_DIR=./bin
COVERAGE_DIR=./coverage

# Versioning
VERSION?=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT?=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME?=$(shell date -u '+%Y-%m-%d_%H:%M:%S')

# Linker flags for versioning
LDFLAGS=-ldflags "-X main.Version=$(VERSION) -X main.Commit=$(COMMIT) -X main.BuildTime=$(BUILD_TIME)"

# Docker parameters
DOCKER_IMAGE?=clustership
DOCKER_TAG?=latest
DOCKER_GPU_TAG?=$(DOCKER_TAG)-gpu

# Kubernetes parameters
K8S_NAMESPACE?=clustership

# golangci-lint version
GOLANGCI_LINT_VERSION=v1.61.0

.PHONY: all build build-all test clean run lint fmt vet tidy help
.PHONY: coverage coverage-html integration bench docker docker-gpu
.PHONY: install-tools check-tools k8s-setup k8s-clean
.DEFAULT_GOAL := help

##@ General

help: ## Display this help message
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

all: clean lint test build ## Clean, lint, test, and build

##@ Development

fmt: ## Format Go code
	@echo "Formatting Go code..."
	@$(GOFMT) -s -w .
	@echo "Code formatted successfully"

vet: ## Run go vet
	@echo "Running go vet..."
	@$(GOVET) ./...
	@echo "go vet passed"

tidy: ## Tidy Go modules
	@echo "Tidying Go modules..."
	@$(GOMOD) tidy
	@$(GOMOD) verify
	@echo "Go modules tidied"

lint: check-tools ## Run golangci-lint
	@echo "Running golangci-lint..."
	@golangci-lint run --timeout=5m
	@echo "Linting passed"

lint-fix: check-tools ## Run golangci-lint with auto-fix
	@echo "Running golangci-lint with fixes..."
	@golangci-lint run --fix --timeout=5m
	@echo "Linting completed with fixes"

##@ Build

build: ## Build the binary for current platform
	@echo "Building $(BINARY_NAME) for current platform..."
	@mkdir -p $(BUILD_DIR)
	@$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) $(CMD_DIR)
	@echo "Build complete: $(BUILD_DIR)/$(BINARY_NAME)"

build-linux-amd64: ## Build for Linux AMD64
	@echo "Building for Linux AMD64..."
	@mkdir -p $(BUILD_DIR)
	@GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)_linux_amd64 $(CMD_DIR)
	@echo "Build complete: $(BUILD_DIR)/$(BINARY_NAME)_linux_amd64"

build-linux-arm64: ## Build for Linux ARM64
	@echo "Building for Linux ARM64..."
	@mkdir -p $(BUILD_DIR)
	@GOOS=linux GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)_linux_arm64 $(CMD_DIR)
	@echo "Build complete: $(BUILD_DIR)/$(BINARY_NAME)_linux_arm64"

build-darwin-amd64: ## Build for macOS AMD64
	@echo "Building for macOS AMD64..."
	@mkdir -p $(BUILD_DIR)
	@GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)_darwin_amd64 $(CMD_DIR)
	@echo "Build complete: $(BUILD_DIR)/$(BINARY_NAME)_darwin_amd64"

build-darwin-arm64: ## Build for macOS ARM64 (Apple Silicon)
	@echo "Building for macOS ARM64..."
	@mkdir -p $(BUILD_DIR)
	@GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)_darwin_arm64 $(CMD_DIR)
	@echo "Build complete: $(BUILD_DIR)/$(BINARY_NAME)_darwin_arm64"

build-windows-amd64: ## Build for Windows AMD64
	@echo "Building for Windows AMD64..."
	@mkdir -p $(BUILD_DIR)
	@GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)_windows_amd64.exe $(CMD_DIR)
	@echo "Build complete: $(BUILD_DIR)/$(BINARY_NAME)_windows_amd64.exe"

build-all: build-linux-amd64 build-linux-arm64 build-darwin-amd64 build-darwin-arm64 build-windows-amd64 ## Build for all platforms
	@echo "All builds complete"
	@ls -lh $(BUILD_DIR)

##@ Testing

test: ## Run unit tests with race detector
	@echo "Running tests with race detector..."
	@$(GOTEST) -race -short -v ./...
	@echo "Tests passed"

test-verbose: ## Run tests with verbose output
	@echo "Running tests with verbose output..."
	@$(GOTEST) -race -v ./...
	@echo "Tests passed"

test-coverage: ## Run tests with coverage
	@echo "Running tests with coverage..."
	@mkdir -p $(COVERAGE_DIR)
	@$(GOTEST) -race -coverprofile=$(COVERAGE_DIR)/coverage.out -covermode=atomic ./...
	@$(GOCMD) tool cover -func=$(COVERAGE_DIR)/coverage.out
	@echo "Coverage report saved to $(COVERAGE_DIR)/coverage.out"

coverage: test-coverage ## Alias for test-coverage

coverage-html: test-coverage ## Generate HTML coverage report
	@echo "Generating HTML coverage report..."
	@$(GOCMD) tool cover -html=$(COVERAGE_DIR)/coverage.out -o $(COVERAGE_DIR)/coverage.html
	@echo "HTML coverage report saved to $(COVERAGE_DIR)/coverage.html"
	@echo "Open in browser: file://$(shell pwd)/$(COVERAGE_DIR)/coverage.html"

bench: ## Run benchmarks
	@echo "Running benchmarks..."
	@$(GOTEST) -bench=. -benchmem -run=^$$ ./...
	@echo "Benchmarks complete"

bench-verbose: ## Run benchmarks with verbose output
	@echo "Running benchmarks with verbose output..."
	@$(GOTEST) -bench=. -benchmem -benchtime=10s -run=^$$ -v ./...
	@echo "Benchmarks complete"

##@ Integration

integration: ## Run all integration tests (K8s + game flow)
	@echo "Running integration tests..."
	@$(GOTEST) -v -tags=integration ./tests/integration/... -timeout=10m
	@echo "Integration tests passed"

integration-k8s: ## Run K8s integration tests only (requires kind cluster)
	@echo "Running K8s integration tests..."
	@echo "Checking for kind cluster..."
	@kind get clusters | grep -q clustership-test || $(MAKE) k8s-setup
	@$(GOTEST) -v -tags=integration ./tests/integration/ -run TestK8s -timeout=10m
	@echo "K8s integration tests passed"

integration-game: ## Run game flow integration tests only
	@echo "Running game flow integration tests..."
	@$(GOTEST) -v -tags=integration ./tests/integration/ -run TestGameFlow -timeout=5m
	@echo "Game flow integration tests passed"

integration-short: ## Run integration tests in short mode (skip long tests)
	@echo "Running integration tests (short mode)..."
	@$(GOTEST) -v -tags=integration -short ./tests/integration/...
	@echo "Integration tests passed"

integration-bench: ## Run integration benchmarks
	@echo "Running integration benchmarks..."
	@$(GOTEST) -tags=integration -bench=. -benchmem -run=^$$ ./tests/integration/...
	@echo "Benchmarks complete"

k8s-setup: ## Create kind cluster for testing
	@echo "Creating kind cluster for ClusterShip..."
	@kind create cluster --name clustership-test --wait 60s || true
	@kubectl cluster-info --context kind-clustership-test
	@echo "Kind cluster ready"

k8s-clean: ## Delete kind cluster
	@echo "Deleting kind cluster..."
	@kind delete cluster --name clustership-test || true
	@echo "Kind cluster deleted"

##@ Docker

docker: ## Build Docker image
	@echo "Building Docker image $(DOCKER_IMAGE):$(DOCKER_TAG)..."
	@docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) .
	@echo "Docker image built: $(DOCKER_IMAGE):$(DOCKER_TAG)"

docker-gpu: ## Build Docker image with GPU support
	@echo "Building Docker image with GPU support $(DOCKER_IMAGE):$(DOCKER_GPU_TAG)..."
	@docker build --build-arg GPU_SUPPORT=true -t $(DOCKER_IMAGE):$(DOCKER_GPU_TAG) .
	@echo "Docker image built: $(DOCKER_IMAGE):$(DOCKER_GPU_TAG)"

docker-run: ## Run Docker container
	@echo "Running Docker container..."
	@docker run -it --rm $(DOCKER_IMAGE):$(DOCKER_TAG)

docker-run-gpu: ## Run Docker container with GPU
	@echo "Running Docker container with GPU..."
	@docker run -it --rm --gpus all $(DOCKER_IMAGE):$(DOCKER_GPU_TAG)

docker-push: docker ## Build and push Docker image
	@echo "Pushing Docker image $(DOCKER_IMAGE):$(DOCKER_TAG)..."
	@docker push $(DOCKER_IMAGE):$(DOCKER_TAG)
	@echo "Docker image pushed"

##@ Run

run: build ## Build and run the application
	@echo "Running $(BINARY_NAME)..."
	@$(BUILD_DIR)/$(BINARY_NAME)

run-dev: ## Run from source without building
	@echo "Running from source..."
	@$(GOCMD) run $(CMD_DIR)

##@ Cleanup

clean: ## Remove build artifacts and coverage reports
	@echo "Cleaning build artifacts..."
	@$(GOCLEAN)
	@rm -rf $(BUILD_DIR)
	@rm -rf $(COVERAGE_DIR)
	@rm -f *.out
	@rm -f *.test
	@echo "Clean complete"

clean-all: clean k8s-clean ## Clean everything including kind cluster
	@echo "Deep clean complete"

##@ Tools

install-tools: ## Install development tools
	@echo "Installing golangci-lint $(GOLANGCI_LINT_VERSION)..."
	@curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(shell go env GOPATH)/bin $(GOLANGCI_LINT_VERSION)
	@echo "Tools installed"

check-tools: ## Check if required tools are installed
	@echo "Checking for required tools..."
	@which golangci-lint > /dev/null || (echo "golangci-lint not found. Run 'make install-tools'" && exit 1)
	@echo "All tools are installed"

##@ CI/CD

ci: lint test build ## Run CI pipeline locally (lint, test, build)
	@echo "CI pipeline complete"

ci-full: lint test-coverage build-all docker ## Run full CI pipeline with coverage and multi-platform builds
	@echo "Full CI pipeline complete"

release: clean lint test build-all ## Prepare release (clean, lint, test, build all platforms)
	@echo "Release artifacts ready in $(BUILD_DIR)"
	@ls -lh $(BUILD_DIR)

##@ Documentation

deps: ## Show dependency tree
	@echo "Dependency tree:"
	@$(GOCMD) mod graph

deps-update: ## Update all dependencies
	@echo "Updating dependencies..."
	@$(GOGET) -u ./...
	@$(GOMOD) tidy
	@echo "Dependencies updated"

deps-verify: ## Verify dependencies
	@echo "Verifying dependencies..."
	@$(GOMOD) verify
	@echo "Dependencies verified"
