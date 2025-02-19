# Project configuration
BINARY_NAME := ratelimiter
PROJECT_NAME := rate-limiter

# Protobuf configuration
PROTO_DIR := proto
PROTO_FILES := $(PROTO_DIR)/rate_limiter.proto

# Docker configuration
DOCKER_DEV_IMAGE := $(PROJECT_NAME)-tools

.PHONY: all build clean test integration-test proto fmt lint up down help

# Default target
all: proto fmt lint test build

# Build the binary
build: proto fmt
	go build -o $(BINARY_NAME) ./cmd/ratelimiter

# Build development tools image
dev-tools:
	docker build --target tools -t $(DOCKER_DEV_IMAGE) .

# Generate protobuf code
proto: dev-tools
	docker run --rm -v $(PWD):/app -w /app $(DOCKER_DEV_IMAGE) \
		protoc --proto_path=$(PROTO_DIR) \
		--go_out=$(PROTO_DIR) --go-grpc_out=$(PROTO_DIR) \
		--go_opt=paths=source_relative --go-grpc_opt=paths=source_relative \
		$(PROTO_FILES)

# Format code
fmt:
	go fmt ./...

# Lint code (excluding generated files)
lint: proto
	go vet $(shell go list ./... | grep -v "$(PROTO_DIR)")
	go run honnef.co/go/tools/cmd/staticcheck@latest -checks=all,-SA1019 $(shell go list ./... | grep -v "$(PROTO_DIR)")

# Run unit tests
test:
	go test -v ./internal/... -cover

# Run integration tests and keep services running
integration-test-keep: up
	go test -v ./tests/... -tags=integration -count=1

# Run integration tests
integration-test: up
	go test -v ./tests/... -tags=integration -count=1
	$(MAKE) down

# Start the application and dependencies
up:
	docker build -t $(PROJECT_NAME) .
	docker-compose up --build -d

# Stop the application and dependencies
down:
	docker-compose down

# Clean up generated files and build artifacts
clean:
	rm -f $(BINARY_NAME)
	rm -f $(PROTO_DIR)/*.pb.go
	docker-compose down -v
	docker rmi $(PROJECT_NAME) $(DOCKER_DEV_IMAGE) || true

# Show help
help:
	@printf "%-25s %s\n" "Available targets:"
	@printf "%-25s %s\n" "  all" "Run all checks and build (default)"
	@printf "%-25s %s\n" "  build" "Build the binary"
	@printf "%-25s %s\n" "  clean" "Clean up generated files and containers"
	@printf "%-25s %s\n" "  dev-tools" "Build development tools Docker image"
	@printf "%-25s %s\n" "  fmt" "Format code"
	@printf "%-25s %s\n" "  lint" "Run linters"
	@printf "%-25s %s\n" "  proto" "Generate protobuf code"
	@printf "%-25s %s\n" "  test" "Run unit tests"
	@printf "%-25s %s\n" "  integration-test" "Run integration tests and stop services after completion"
	@printf "%-25s %s\n" "  integration-test-keep" "Run integration tests and keep services running"
	@printf "%-25s %s\n" "  up" "Start application and dependencies"
	@printf "%-25s %s\n" "  down" "Stop application and dependencies"
