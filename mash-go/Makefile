.PHONY: build test test-unit test-integration generate mocks install-mockery lint clean help

# Build all packages
build:
	go build ./...

# Run all tests (unit + integration)
test: test-unit test-integration

# Run unit tests only (default, fast)
test-unit:
	go test ./...

# Run integration tests (requires network)
test-integration:
	go test -tags=integration ./...

# Run tests with verbose output
test-verbose:
	go test -v ./...

# Run integration tests with verbose output
test-integration-verbose:
	go test -v -tags=integration ./...

# Generate all code (mocks, etc.)
generate: mocks

# Generate mocks using mockery
mocks:
	go generate ./...

# Install mockery v2
install-mockery:
	go install github.com/vektra/mockery/v2@latest

# Run linter (if golangci-lint installed)
lint:
	golangci-lint run ./...

# Clean generated files
clean:
	rm -rf pkg/transport/mocks
	rm -rf pkg/discovery/mocks

# Show help
help:
	@echo "Available targets:"
	@echo "  build               - Build all packages"
	@echo "  test                - Run all tests"
	@echo "  test-unit           - Run unit tests only (fast)"
	@echo "  test-integration    - Run integration tests"
	@echo "  generate/mocks      - Generate mock files"
	@echo "  install-mockery     - Install mockery v2"
	@echo "  lint                - Run golangci-lint"
	@echo "  clean               - Remove generated files"
