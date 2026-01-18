.PHONY: all build test test-race vet lint coverage clean

# Default target
all: lint vet test build

# Build all binaries
build:
	go build ./cmd/...

# Run tests
test:
	go test ./...

# Run tests with race detector
test-race:
	go test -race ./...

# Run go vet
vet:
	go vet ./...

# Run linter (requires golangci-lint)
lint:
	golangci-lint run

# Generate coverage report
coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

# Clean build artifacts
clean:
	rm -f cmd/vision3/vision3
	rm -f cmd/vision3-config/vision3-config
	rm -f cmd/vision3-bbsconfig/vision3-bbsconfig
	rm -f cmd/install/install
	rm -f coverage.out coverage.html
	rm -rf dist/

# Version info (override with: make release VERSION=1.0.0)
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)
DIST_NAME = vision3-$(VERSION)-$(GOOS)-$(GOARCH)
DIST_DIR = dist/$(DIST_NAME)

# Build release binary with version info
build-release:
	CGO_ENABLED=0 go build -ldflags="-s -w" -o $(DIST_DIR)/vision3 ./cmd/vision3

# Create release bundle
release: clean build-release
	@echo "Creating release bundle: $(DIST_NAME)"
	@mkdir -p $(DIST_DIR)/configs
	@mkdir -p $(DIST_DIR)/data/users
	@mkdir -p $(DIST_DIR)/data/files/general
	@mkdir -p $(DIST_DIR)/data/logs
	@# Copy configs (excluding SSH keys)
	@cp configs/config.json $(DIST_DIR)/configs/
	@cp configs/doors.json $(DIST_DIR)/configs/
	@cp configs/file_areas.json $(DIST_DIR)/configs/
	@cp configs/menu_renderer.json $(DIST_DIR)/configs/
	@cp configs/strings.json $(DIST_DIR)/configs/
	@cp configs/ssh_host_keys.example $(DIST_DIR)/configs/
	@# Copy menus
	@cp -r menus $(DIST_DIR)/
	@# Copy default data files
	@echo "[]" > $(DIST_DIR)/data/oneliners.json
	@cp data/message_areas.json $(DIST_DIR)/data/ 2>/dev/null || echo '[]' > $(DIST_DIR)/data/message_areas.json
	@# Copy setup script (release version)
	@cp scripts/setup-release.sh $(DIST_DIR)/setup.sh
	@chmod +x $(DIST_DIR)/setup.sh
	@# Create archive
	@cd dist && tar -czf $(DIST_NAME).tar.gz $(DIST_NAME)
	@echo "Release bundle created: dist/$(DIST_NAME).tar.gz"

# Cross-compile releases for common platforms
release-all: clean
	GOOS=linux GOARCH=amd64 $(MAKE) release
	GOOS=linux GOARCH=arm64 $(MAKE) release
	GOOS=darwin GOARCH=amd64 $(MAKE) release
	GOOS=darwin GOARCH=arm64 $(MAKE) release
	@echo "All release bundles created in dist/"
