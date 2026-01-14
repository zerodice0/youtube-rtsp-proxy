# youtube-rtsp-proxy Makefile

BINARY_NAME=youtube-rtsp-proxy
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS=-ldflags "-X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME)"

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod

# Directories
BIN_DIR=bin
CMD_DIR=cmd/youtube-rtsp-proxy

.PHONY: all build build-linux build-darwin clean test deps install uninstall lint help

# Default target
all: deps build

# Build for current platform
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BIN_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(BIN_DIR)/$(BINARY_NAME) ./$(CMD_DIR)
	@echo "Built: $(BIN_DIR)/$(BINARY_NAME)"

# Build for Linux (amd64 and arm64)
build-linux:
	@echo "Building for Linux..."
	@mkdir -p $(BIN_DIR)
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BIN_DIR)/$(BINARY_NAME)-linux-amd64 ./$(CMD_DIR)
	GOOS=linux GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BIN_DIR)/$(BINARY_NAME)-linux-arm64 ./$(CMD_DIR)
	@echo "Built: $(BIN_DIR)/$(BINARY_NAME)-linux-amd64"
	@echo "Built: $(BIN_DIR)/$(BINARY_NAME)-linux-arm64"

# Build for macOS (amd64 and arm64)
build-darwin:
	@echo "Building for macOS..."
	@mkdir -p $(BIN_DIR)
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BIN_DIR)/$(BINARY_NAME)-darwin-amd64 ./$(CMD_DIR)
	GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BIN_DIR)/$(BINARY_NAME)-darwin-arm64 ./$(CMD_DIR)
	@echo "Built: $(BIN_DIR)/$(BINARY_NAME)-darwin-amd64"
	@echo "Built: $(BIN_DIR)/$(BINARY_NAME)-darwin-arm64"

# Build for all platforms
build-all: build-linux build-darwin

# Clean build artifacts
clean:
	@echo "Cleaning..."
	rm -rf $(BIN_DIR)
	@echo "Cleaned."

# Run tests
test:
	@echo "Running tests..."
	$(GOTEST) -v ./...

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	$(GOTEST) -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	$(GOMOD) download
	$(GOMOD) tidy
	@echo "Dependencies updated."

# Install to system
install: build
	@echo "Installing $(BINARY_NAME)..."
	sudo cp $(BIN_DIR)/$(BINARY_NAME) /usr/local/bin/
	sudo chmod +x /usr/local/bin/$(BINARY_NAME)
	@echo "Creating directories..."
	sudo mkdir -p /etc/youtube-rtsp-proxy
	@if [ ! -f /etc/youtube-rtsp-proxy/config.yaml ]; then \
		sudo cp configs/config.example.yaml /etc/youtube-rtsp-proxy/config.yaml; \
		echo "Config file created: /etc/youtube-rtsp-proxy/config.yaml"; \
	fi
	@echo "Installation complete!"
	@echo "Usage: $(BINARY_NAME) --help"

# Uninstall from system
uninstall:
	@echo "Uninstalling $(BINARY_NAME)..."
	sudo rm -f /usr/local/bin/$(BINARY_NAME)
	@echo "Note: Config directory /etc/youtube-rtsp-proxy was not removed."
	@echo "Remove manually if needed: sudo rm -rf /etc/youtube-rtsp-proxy"
	@echo "Uninstallation complete."

# Run linter
lint:
	@echo "Running linter..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not found. Install with:"; \
		echo "  go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
	fi

# Development run
run: build
	./$(BIN_DIR)/$(BINARY_NAME) --help

# Show help
help:
	@echo "youtube-rtsp-proxy Makefile"
	@echo ""
	@echo "Usage:"
	@echo "  make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  all           Build the binary (default)"
	@echo "  build         Build for current platform"
	@echo "  build-linux   Build for Linux (amd64, arm64)"
	@echo "  build-darwin  Build for macOS (amd64, arm64)"
	@echo "  build-all     Build for all platforms"
	@echo "  clean         Remove build artifacts"
	@echo "  test          Run tests"
	@echo "  test-coverage Run tests with coverage report"
	@echo "  deps          Download and tidy dependencies"
	@echo "  install       Install to /usr/local/bin"
	@echo "  uninstall     Remove from /usr/local/bin"
	@echo "  lint          Run golangci-lint"
	@echo "  run           Build and show help"
	@echo "  help          Show this help"
