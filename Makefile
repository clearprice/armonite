# Armonite Distributed Load Testing Framework
# Makefile for building coordinator, agents, and React UI

# Variables
BINARY_NAME=armonite
GO_FILES=$(shell find . -name "*.go" -not -path "./ui-react/*")
UI_SRC_DIR=ui-react
UI_BUILD_DIR=ui-build
VERSION?=1.0.0
LDFLAGS=-ldflags "-X main.version=$(VERSION)"

# Default target
.PHONY: all
all: clean build

# Help target
.PHONY: help
help:
	@echo "Armonite Build System"
	@echo "===================="
	@echo ""
	@echo "Available targets:"
	@echo "  build         - Build the Armonite binary with embedded UI"
	@echo "  build-ui      - Build the React UI only"
	@echo "  build-all     - Alias for build (UI now embedded automatically)"
	@echo "  clean         - Clean build artifacts"
	@echo "  clean-ui      - Clean UI build artifacts"
	@echo "  dev-ui        - Start UI development server"
	@echo "  test          - Run Go tests"
	@echo "  fmt           - Format Go code"
	@echo "  lint          - Lint Go code (requires golangci-lint)"
	@echo "  run-coord     - Run coordinator with UI"
	@echo "  run-agent     - Run agent (requires MASTER_HOST)"
	@echo "  run-agent-dev - Run agent with development mode limits"
	@echo "  install-deps  - Install UI dependencies"
	@echo "  docker-build  - Build Docker image"
	@echo "  release       - Build release binaries for multiple platforms"
	@echo "  help          - Show this help message"

# Go binary build (now depends on UI being built first)
.PHONY: build
build: build-ui
	@echo "Building Armonite binary with embedded UI..."
	go build $(LDFLAGS) -o $(BINARY_NAME) .
	@echo "✅ Binary built with embedded UI: $(BINARY_NAME)"

# React UI build
.PHONY: build-ui
build-ui: install-deps
	@echo "Building React UI..."
	cd $(UI_SRC_DIR) && npm run build
	@echo "✅ UI built in $(UI_BUILD_DIR)/"

# Build both UI and binary (now build automatically includes UI)
.PHONY: build-all
build-all: build

# Install UI dependencies
.PHONY: install-deps
install-deps:
	@echo "Installing UI dependencies..."
	cd $(UI_SRC_DIR) && npm install
	@echo "✅ Dependencies installed"

# Development UI server
.PHONY: dev-ui
dev-ui: install-deps
	@echo "Starting UI development server..."
	cd $(UI_SRC_DIR) && npm run dev

# Clean build artifacts
.PHONY: clean
clean: clean-ui
	@echo "Cleaning Go build artifacts..."
	rm -f $(BINARY_NAME)
	rm -rf dist/
	@echo "✅ Cleaned build artifacts"

# Clean UI build artifacts
.PHONY: clean-ui
clean-ui:
	@echo "Cleaning UI build artifacts..."
	rm -rf $(UI_BUILD_DIR)
	rm -rf $(UI_SRC_DIR)/node_modules
	@echo "✅ Cleaned UI artifacts"

# Run tests
.PHONY: test
test:
	@echo "Running Go tests..."
	go test -v ./...

# Format Go code
.PHONY: fmt
fmt:
	@echo "Formatting Go code..."
	go fmt ./...
	@echo "✅ Code formatted"

# Lint Go code
.PHONY: lint
lint:
	@echo "Linting Go code..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "⚠️  golangci-lint not found. Install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
		go vet ./...; \
	fi

# Run coordinator with UI
.PHONY: run-coord
run-coord: build
	@echo "Starting Armonite coordinator with embedded UI..."
	./$(BINARY_NAME) coordinator --ui --http-port 8050

# Run coordinator without UI
.PHONY: run-coord-api
run-coord-api: build
	@echo "Starting Armonite coordinator (API only)..."
	./$(BINARY_NAME) coordinator --http-port 8050

# Run agent (requires MASTER_HOST environment variable)
.PHONY: run-agent
run-agent: build
	@if [ -z "$(MASTER_HOST)" ]; then \
		echo "❌ MASTER_HOST environment variable required"; \
		echo "Usage: make run-agent MASTER_HOST=localhost"; \
		exit 1; \
	fi
	@echo "Starting Armonite agent connecting to $(MASTER_HOST)..."
	./$(BINARY_NAME) agent --master-host $(MASTER_HOST) --master-port 4222 --concurrency 100 --region local

# Run agent in development mode with resource limits
.PHONY: run-agent-dev
run-agent-dev: build
	@if [ -z "$(MASTER_HOST)" ]; then \
		echo "❌ MASTER_HOST environment variable required"; \
		echo "Usage: make run-agent-dev MASTER_HOST=localhost"; \
		exit 1; \
	fi
	@echo "Starting Armonite agent in development mode connecting to $(MASTER_HOST)..."
	./$(BINARY_NAME) agent --master-host $(MASTER_HOST) --master-port 4222 --dev --region local

# Docker build
.PHONY: docker-build
docker-build: build
	@echo "Building Docker image with embedded UI..."
	docker build -t armonite:$(VERSION) -t armonite:latest .
	@echo "✅ Docker image built: armonite:$(VERSION)"

# Release builds for multiple platforms
.PHONY: release
release: clean build-ui
	@echo "Building release binaries with embedded UI..."
	mkdir -p dist
	
	# Linux AMD64
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY_NAME)-linux-amd64 .
	
	# Linux ARM64
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o dist/$(BINARY_NAME)-linux-arm64 .
	
	# macOS AMD64
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY_NAME)-darwin-amd64 .
	
	# macOS ARM64 (Apple Silicon)
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o dist/$(BINARY_NAME)-darwin-arm64 .
	
	# Windows AMD64
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY_NAME)-windows-amd64.exe .
	
	@echo "✅ Release binaries built with embedded UI in dist/"
	@ls -la dist/

# Development workflow
.PHONY: dev
dev: clean install-deps
	@echo "Starting development environment..."
	@echo "This will start the coordinator and UI dev server in parallel"
	@echo "Press Ctrl+C to stop both"
	make run-coord-api & \
	sleep 3 && \
	make dev-ui

# Quick test of the build
.PHONY: smoke-test
smoke-test: build
	@echo "Running smoke test..."
	./$(BINARY_NAME) --version || ./$(BINARY_NAME) --help
	@echo "✅ Smoke test passed"

# Check dependencies
.PHONY: check-deps
check-deps:
	@echo "Checking dependencies..."
	@echo "Go version:"
	@go version
	@echo ""
	@echo "Node.js version:"
	@node --version 2>/dev/null || echo "❌ Node.js not found"
	@echo ""
	@echo "npm version:"
	@npm --version 2>/dev/null || echo "❌ npm not found"
	@echo ""
	@echo "Docker version:"
	@docker --version 2>/dev/null || echo "⚠️  Docker not found (optional)"

# Watch and rebuild on changes (requires entr or similar)
.PHONY: watch
watch:
	@if command -v entr >/dev/null 2>&1; then \
		echo "Watching for changes..."; \
		find . -name "*.go" | entr -r make build; \
	else \
		echo "❌ entr not found. Install with: brew install entr (macOS) or apt install entr (Ubuntu)"; \
	fi

# Generate version info
.PHONY: version
version:
	@echo "Armonite version: $(VERSION)"
	@echo "Git commit: $$(git rev-parse --short HEAD 2>/dev/null || echo 'unknown')"
	@echo "Build date: $$(date -u +%Y-%m-%dT%H:%M:%SZ)"

# Setup development environment
.PHONY: setup
setup: check-deps install-deps
	@echo "Setting up development environment..."
	@if ! command -v golangci-lint >/dev/null 2>&1; then \
		echo "Installing golangci-lint..."; \
		go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest; \
	fi
	@echo "✅ Development environment ready"
	@echo ""
	@echo "Next steps:"
	@echo "  make build-all    # Build everything"
	@echo "  make run-coord    # Start coordinator with UI"
	@echo "  make dev          # Start development environment"