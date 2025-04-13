# Project Configuration
APP_NAME := fss
VERSION := 1.0.0
MAIN_PACKAGE := ./cmd/fss

# Platform-specific Configuration
ifeq ($(OS),Windows_NT)
    BINARY := $(APP_NAME).exe
    BINARY_WINDOWS := $(APP_NAME).exe
else
    BINARY := $(APP_NAME)
    BINARY_WINDOWS := $(APP_NAME).exe
endif

# Build Directories
BUILD_DIR := ./bin
COVERAGE_DIR := ./coverage

# Go Tools Configuration
GO := go
GO_BUILD := $(GO) build
GO_TEST := $(GO) test
GO_MOD := $(GO) mod
GO_LDFLAGS := -ldflags "-X main.Version=$(VERSION) -w -s"  # -w -s reduces binary size

# Default target (first target is the default)
.PHONY: all
all: build

# Build for current platform (auto-detects OS)
.PHONY: build
build:
	@mkdir -p $(BUILD_DIR)
	$(GO_BUILD) $(GO_LDFLAGS) -o $(BUILD_DIR)/$(BINARY) $(MAIN_PACKAGE)

# Cross-compilation targets
.PHONY: build-all
build-all: build-linux build-windows build-darwin

build-linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO_BUILD) $(GO_LDFLAGS) -o $(BUILD_DIR)/$(APP_NAME)-linux $(MAIN_PACKAGE)

build-windows:
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 $(GO_BUILD) $(GO_LDFLAGS) -o $(BUILD_DIR)/$(BINARY_WINDOWS) $(MAIN_PACKAGE)

build-darwin:
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 $(GO_BUILD) $(GO_LDFLAGS) -o $(BUILD_DIR)/$(APP_NAME)-darwin $(MAIN_PACKAGE)

# Run the application
.PHONY: run
run:
	$(GO) run $(MAIN_PACKAGE)

# Testing targets
.PHONY: test
test:
	$(GO_TEST) -v ./...

.PHONY: test-cover
test-cover:
	@mkdir -p $(COVERAGE_DIR)
	$(GO_TEST) -coverprofile=$(COVERAGE_DIR)/coverage.out ./...
	$(GO) tool cover -html=$(COVERAGE_DIR)/coverage.out -o $(COVERAGE_DIR)/coverage.html

# Dependency management
.PHONY: mod-tidy
mod-tidy:
	$(GO_MOD) tidy

# Code quality targets
.PHONY: fmt
fmt:
	$(GO) fmt ./...

.PHONY: vet
vet:
	$(GO) vet ./...

.PHONY: lint
lint:
	golangci-lint run ./...

# Cleanup
.PHONY: clean
clean:
	rm -rf $(BUILD_DIR) $(COVERAGE_DIR)
	$(GO) clean

# Help (self-documenting makefile)
.PHONY: help
help:
	@echo "Available targets:"
	@echo "  build         - Build for current platform (auto-detects OS)"
	@echo "  build-all     - Build for Linux, Windows and Darwin"
	@echo "  run           - Run the application"
	@echo "  test          - Run tests"
	@echo "  test-cover    - Run tests with coverage report"
	@echo "  mod-tidy      - Tidy Go modules"
	@echo "  fmt           - Format code"
	@echo "  vet           - Run go vet"
	@echo "  lint          - Run golangci-lint (requires installation)"
	@echo "  clean         - Clean build artifacts"
	@echo "  help          - Show this help message"
	