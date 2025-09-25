# Makefile for cs

# Variables
BINARY_NAME=cs
MODULE_NAME=cs
BUILD_DIR=.
INSTALL_DIR=/usr/local/bin
CONFIG_DIR=$(HOME)/.config/cs

# Go build flags
GOOS=$(shell go env GOOS)
GOARCH=$(shell go env GOARCH)
LDFLAGS=-w -s
BUILD_FLAGS=-trimpath -ldflags="$(LDFLAGS)"

# Default target
.PHONY: all
all: build

# Build the binary
.PHONY: build
build:
	@echo "Building $(BINARY_NAME)..."
	go build $(BUILD_FLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) .
	@echo "Build complete: $(BUILD_DIR)/$(BINARY_NAME)"

# Install the binary and set up configuration (idempotent)
.PHONY: install
install: build
	@echo "Installing $(BINARY_NAME) to $(INSTALL_DIR)..."
	sudo install -m 755 $(BUILD_DIR)/$(BINARY_NAME) $(INSTALL_DIR)/$(BINARY_NAME)
	@echo "Installed $(BINARY_NAME) to $(INSTALL_DIR)/$(BINARY_NAME)"
	@echo ""
	@echo "Setting up configuration directory..."
	@mkdir -p $(CONFIG_DIR)/snippets
	@if [ ! -f "$(CONFIG_DIR)/config.yaml" ]; then \
		echo "  Creating config.yaml from default template..."; \
		go run . --generate-config > $(CONFIG_DIR)/config.yaml; \
		echo "Configuration created at $(CONFIG_DIR)/config.yaml"; \
	else \
		echo "Configuration already exists at $(CONFIG_DIR)/config.yaml - skipping"; \
	fi
	@echo ""
	@echo "Installation complete! Reference examples in ./snippets/ directory"

# Uninstall the binary
.PHONY: uninstall
uninstall:
	@echo "Removing $(BINARY_NAME) from $(INSTALL_DIR)..."
	sudo rm -f $(INSTALL_DIR)/$(BINARY_NAME)
	@echo "Uninstalled $(BINARY_NAME)"

# Remove configuration (use with caution)
.PHONY: clean-config
clean-config:
	@echo "Removing configuration from $(CONFIG_DIR)..."
	rm -rf $(CONFIG_DIR)
	@echo "Configuration removed"

# Clean build artifacts
.PHONY: clean
clean:
	@echo "Cleaning build artifacts..."
	rm -f $(BUILD_DIR)/$(BINARY_NAME)
	go clean
	@echo "Clean complete"

# Run tests
.PHONY: test
test:
	@echo "Running tests..."
	go test -v ./...

# Run linter
.PHONY: lint
lint:
	@echo "Running golangci-lint..."
	golangci-lint run

# Format code
.PHONY: fmt
fmt:
	@echo "Formatting code..."
	go fmt ./...

# Tidy dependencies
.PHONY: tidy
tidy:
	@echo "Tidying dependencies..."
	go mod tidy

# Build for multiple platforms
.PHONY: build-all
build-all:
	@echo "Building for multiple platforms..."
	GOOS=linux GOARCH=amd64 go build $(BUILD_FLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 .
	GOOS=linux GOARCH=arm64 go build $(BUILD_FLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 .
	GOOS=darwin GOARCH=amd64 go build $(BUILD_FLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 .
	GOOS=darwin GOARCH=arm64 go build $(BUILD_FLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 .
	GOOS=windows GOARCH=amd64 go build $(BUILD_FLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe .
	@echo "Multi-platform build complete"

# Development target - build and run with sample config
.PHONY: dev
dev: build
	@echo "Running $(BINARY_NAME) with local config..."
	./$(BINARY_NAME) --config ./cs.yaml

# Show help
.PHONY: help
help:
	@echo "Available targets:"
	@echo "  build        - Build the binary"
	@echo "  install      - Build and install binary + setup config (idempotent)"
	@echo "  uninstall    - Remove binary from $(INSTALL_DIR)"
	@echo "  clean-config - Remove configuration directory (use with caution)"
	@echo "  clean        - Remove build artifacts"
	@echo "  test         - Run tests"
	@echo "  lint         - Run linter (requires golangci-lint)"
	@echo "  fmt          - Format code"
	@echo "  tidy         - Tidy dependencies"
	@echo "  build-all    - Build for multiple platforms"
	@echo "  dev          - Build and run with local config"
	@echo "  help         - Show this help message"

# Check if binary exists and show version/info
.PHONY: info
info:
	@echo "Project: $(MODULE_NAME)"
	@echo "Binary: $(BINARY_NAME)"
	@echo "GOOS: $(GOOS)"
	@echo "GOARCH: $(GOARCH)"
	@if [ -f "$(BUILD_DIR)/$(BINARY_NAME)" ]; then \
		echo "Built binary: $(BUILD_DIR)/$(BINARY_NAME)"; \
		ls -la $(BUILD_DIR)/$(BINARY_NAME); \
	else \
		echo "Binary not built yet. Run 'make build' first."; \
	fi
	@if command -v $(BINARY_NAME) >/dev/null 2>&1; then \
		echo "Installed version: $$(command -v $(BINARY_NAME))"; \
	else \
		echo "Not installed in PATH"; \
	fi
