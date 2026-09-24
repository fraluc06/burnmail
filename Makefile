.PHONY: build install clean run test bench deps lint

# Binary name
BINARY_NAME=burnmail
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS=-s -w -X burnmail/internal/config.Version=$(VERSION)

# Build the project (optimized)
build:
	@echo "Building $(BINARY_NAME) $(VERSION)..."
	go build -o $(BINARY_NAME) -ldflags="$(LDFLAGS)" -trimpath .
	@echo "Binary size: $$(du -h $(BINARY_NAME) | cut -f1)"

# Build for development (with debug symbols)
build-dev:
	go build -o $(BINARY_NAME) .

# Install to /usr/local/bin
install: build
	sudo mv $(BINARY_NAME) /usr/local/bin/

# Build for multiple platforms
build-all:
	@echo "Building for multiple platforms..."
	GOOS=linux GOARCH=amd64 go build -o $(BINARY_NAME)-linux-amd64 -ldflags="$(LDFLAGS)" -trimpath .
	GOOS=linux GOARCH=arm64 go build -o $(BINARY_NAME)-linux-arm64 -ldflags="$(LDFLAGS)" -trimpath .
	GOOS=darwin GOARCH=amd64 go build -o $(BINARY_NAME)-darwin-amd64 -ldflags="$(LDFLAGS)" -trimpath .
	GOOS=darwin GOARCH=arm64 go build -o $(BINARY_NAME)-darwin-arm64 -ldflags="$(LDFLAGS)" -trimpath .
	GOOS=windows GOARCH=amd64 go build -o $(BINARY_NAME)-windows-amd64.exe -ldflags="$(LDFLAGS)" -trimpath .
	@echo "Build complete. Binary sizes:"
	@du -h $(BINARY_NAME)-*

# Clean build artifacts
clean:
	go clean
	rm -f $(BINARY_NAME)
	rm -f $(BINARY_NAME)-*
	rm -f ~/.burnmail-cache.json

# Run the application
run:
	go run main.go

# Run tests
test:
	go test -v -race -cover ./...

# Run benchmarks
bench:
	go test -bench=. -benchmem ./...

# Run linter
lint:
	go vet ./...
	gofmt -s -w .

# Download dependencies
deps:
	go mod download
	go mod tidy
	go mod verify

# Show binary info
info:
	@echo "Version: $(VERSION)"
	@echo "Build Time: $(BUILD_TIME)"
	@if [ -f $(BINARY_NAME) ]; then \
		echo "Binary: $(BINARY_NAME)"; \
		ls -lh $(BINARY_NAME); \
		file $(BINARY_NAME); \
	fi
