# ClusterShip Makefile

BINARY=clustership
BUILD_DIR=./bin
COVERAGE_DIR=./coverage
CMD_DIR=./cmd/clustership
VERSION?=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS=-ldflags "-X main.Version=$(VERSION)"
DOCKER_IMAGE?=clustership
K8S_NAMESPACE?=clustership

.PHONY: all build test lint clean run help
.DEFAULT_GOAL := help

help:
	@grep -E '^[a-zA-Z_-]+:.*?##' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?##"}; {printf "  %-15s %s\n", $$1, $$2}'

all: lint test build ## Lint, test, and build

build: ## Build binary
	@mkdir -p $(BUILD_DIR)
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY) $(CMD_DIR)

build-all: ## Build all platforms
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY)_linux_amd64 $(CMD_DIR)
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY)_linux_arm64 $(CMD_DIR)
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY)_darwin_amd64 $(CMD_DIR)
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY)_darwin_arm64 $(CMD_DIR)
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY)_windows_amd64.exe $(CMD_DIR)

test: ## Run tests
	go test -race -short ./...

test-coverage: ## Run tests with coverage
	@mkdir -p $(COVERAGE_DIR)
	go test -race -coverprofile=$(COVERAGE_DIR)/coverage.out -covermode=atomic ./...
	go tool cover -html=$(COVERAGE_DIR)/coverage.out -o $(COVERAGE_DIR)/coverage.html

bench: ## Run benchmarks
	go test -bench=. -benchmem -run=^$$ ./...

lint: ## Run linter
	golangci-lint run --timeout=5m

fmt: ## Format code
	gofmt -s -w .

vet: ## Run go vet
	go vet ./...

tidy: ## Tidy modules
	go mod tidy && go mod verify

integration: ## Run integration tests
	go test -v -tags=integration ./tests/integration/... -timeout=10m

k8s-setup: ## Create kind cluster
	kind create cluster --name clustership-test --wait 60s || true

k8s-clean: ## Delete kind cluster
	kind delete cluster --name clustership-test || true

docker: ## Build Docker image
	docker build -t $(DOCKER_IMAGE):latest .

run: build ## Build and run
	$(BUILD_DIR)/$(BINARY)

clean: ## Clean build artifacts
	go clean
	rm -rf $(BUILD_DIR) $(COVERAGE_DIR)

ci: lint test build ## CI pipeline
