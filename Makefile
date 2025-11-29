.PHONY: build run test clean fmt vet install help

# Variable definitions
BINARY_NAME=vsftp-exporter
GO=go
GOFLAGS=-v
VERSION?=1.0.0
BUILD_TIME=$(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS=-ldflags "-X main.appVersion=$(VERSION) -X main.buildTime=$(BUILD_TIME)"

# Default target
all: fmt vet build

# Build binary file
build:
	@echo "Building $(BINARY_NAME)..."
	$(GO) build $(GOFLAGS) $(LDFLAGS) -o $(BINARY_NAME) vsftp-exporter.go
	@echo "Build complete: $(BINARY_NAME)"

# Run program
run: build
	@echo "Starting $(BINARY_NAME)..."
	./$(BINARY_NAME) -config=./config.json

# Run tests
test:
	@echo "Running tests..."
	$(GO) test -v -race -coverprofile=coverage.txt -covermode=atomic ./...
	@echo "Tests complete"

# View test coverage
coverage: test
	@echo "Generating coverage report..."
	$(GO) tool cover -html=coverage.txt -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Format code
fmt:
	@echo "Formatting code..."
	$(GO) fmt ./...

# Code check
vet:
	@echo "Running code checks..."
	$(GO) vet ./...

# Tidy dependencies
tidy:
	@echo "Tidying dependencies..."
	$(GO) mod tidy

# Install to system
install: build
	@echo "Installing $(BINARY_NAME) to /usr/local/bin/..."
	sudo cp $(BINARY_NAME) /usr/local/bin/
	@echo "Installation complete"

# Clean build files
clean:
	@echo "Cleaning build files..."
	rm -f $(BINARY_NAME)
	rm -f coverage.txt coverage.html
	@echo "Cleanup complete"

# Cross-compilation
build-linux:
	@echo "Building Linux version..."
	GOOS=linux GOARCH=amd64 $(GO) build $(LDFLAGS) -o $(BINARY_NAME)-linux-amd64 vsftp-exporter.go

build-windows:
	@echo "Building Windows version..."
	GOOS=windows GOARCH=amd64 $(GO) build $(LDFLAGS) -o $(BINARY_NAME)-windows-amd64.exe vsftp-exporter.go

build-darwin:
	@echo "Building macOS version..."
	GOOS=darwin GOARCH=amd64 $(GO) build $(LDFLAGS) -o $(BINARY_NAME)-darwin-amd64 vsftp-exporter.go

build-all: build-linux build-windows build-darwin
	@echo "All platform builds complete"

# Help information
help:
	@echo "Available make targets:"
	@echo "  make build        - Build binary file"
	@echo "  make run          - Build and run program"
	@echo "  make test         - Run tests"
	@echo "  make coverage     - Generate test coverage report"
	@echo "  make fmt          - Format code"
	@echo "  make vet          - Run code checks"
	@echo "  make tidy         - Tidy dependencies"
	@echo "  make install      - Install to system"
	@echo "  make clean        - Clean build files"
	@echo "  make build-all    - Cross-compile for all platforms"
	@echo "  make help         - Show this help information"
