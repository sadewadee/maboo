.PHONY: build test clean run fmt lint docker docker-push docker-run release

BINARY=maboo
VERSION?=$(shell git describe --tags --always --dirty 2>/dev/null || echo "0.1.0-dev")
GOFLAGS=-ldflags "-s -w -X main.version=$(VERSION)"
IMAGE?=ghcr.io/sadewadee/maboo

# Build binary
build:
	go build -trimpath $(GOFLAGS) -o $(BINARY) ./cmd/maboo

# Build for multiple platforms
build-all:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath $(GOFLAGS) -o dist/$(BINARY)-linux-amd64 ./cmd/maboo
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath $(GOFLAGS) -o dist/$(BINARY)-linux-arm64 ./cmd/maboo
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -trimpath $(GOFLAGS) -o dist/$(BINARY)-darwin-amd64 ./cmd/maboo
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -trimpath $(GOFLAGS) -o dist/$(BINARY)-darwin-arm64 ./cmd/maboo

# Run tests
test:
	go test -v -race -coverprofile=coverage.out ./...

# Clean build artifacts
clean:
	rm -f $(BINARY)
	rm -rf dist/
	go clean

# Build and run locally
run: build
	./$(BINARY) serve

# Format code
fmt:
	go fmt ./...

# Run linter
lint:
	golangci-lint run ./...

# Build Docker image
docker:
	docker build -t $(IMAGE):$(VERSION) -t $(IMAGE):latest .

# Build multi-platform Docker image
docker-multi:
	docker buildx build --platform linux/amd64,linux/arm64 -t $(IMAGE):$(VERSION) -t $(IMAGE):latest .

# Push Docker image
docker-push:
	docker push $(IMAGE):$(VERSION)
	docker push $(IMAGE):latest

# Run Docker container
docker-run:
	docker run --rm -p 8080:8080 -p 8443:8443 $(IMAGE):latest

# Run with docker-compose
compose-up:
	docker compose up -d

# Stop docker-compose
compose-down:
	docker compose down

# Create release tag
release:
	@if [ -z "$(v)" ]; then echo "Usage: make release v=1.0.0"; exit 1; fi
	git tag -a v$(v) -m "Release v$(v)"
	git push origin v$(v)

.DEFAULT_GOAL := build
