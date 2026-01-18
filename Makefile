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
