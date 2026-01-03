# Modrinth Mod Updater Makefile

# Variables
BINARY_NAME = modrinth-mod-updater
VERSION = $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS = -ldflags "-X main.version=$(VERSION)"

# Default target
.PHONY: all
all: build

# Build the binary with optimized settings
.PHONY: build
build:
	@echo "Building $(BINARY_NAME)..."
	go build -v -x -p $$(nproc) $(LDFLAGS) -o $(BINARY_NAME) .
	@echo "Build complete: $(BINARY_NAME)"

# Quick build
.PHONY: build-quick
build-quick:
	@echo "Building $(BINARY_NAME) (quick)..."
	go build $(LDFLAGS) -o $(BINARY_NAME) .
	@echo "Build complete: $(BINARY_NAME)"

# Install to system
.PHONY: install
install: build
	@echo "Installing $(BINARY_NAME) to /usr/local/bin..."
	sudo install $(BINARY_NAME) /usr/local/bin/
	@echo "Installation complete"

# Clean
.PHONY: clean
clean:
	@echo "Cleaning up..."
	rm -f $(BINARY_NAME)
	rm -rf dist/
	@echo "Clean complete"

# Lint
.PHONY: lint
lint:
	@echo "Running linters..."
	PATH=$(PATH):$(HOME)/go/bin golangci-lint run ./...
	@echo "Linting complete"

# Test
.PHONY: test
test:
	@echo "Running tests..."
	go test -v ./...
	@echo "Tests complete"

# Test Coverage
.PHONY: test-coverage
test-coverage:
	@echo "Running tests with coverage..."
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Help
.PHONY: help
help:
	@echo "Available targets:"
	@echo "  build          - Build with verbose output and parallel compilation"
	@echo "  build-quick    - Quick build without verbose output"
	@echo "  install        - Build and install to /usr/local/bin"
	@echo "  lint           - Run linters (golangci-lint)"
	@echo "  test           - Run tests"
	@echo "  test-coverage  - Run tests and generate coverage report"
	@echo "  clean          - Remove build artifacts"
	@echo "  help           - Show this help message"
