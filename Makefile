.PHONY: build test clean run fmt lint

BINARY=maboo
VERSION?=0.1.0-dev
GOFLAGS=-ldflags "-X main.version=$(VERSION)"

build:
	go build $(GOFLAGS) -o $(BINARY) ./cmd/maboo

test:
	go test -v -race ./...

clean:
	rm -f $(BINARY)
	go clean

run: build
	./$(BINARY) serve

fmt:
	go fmt ./...
	goimports -w .

lint:
	golangci-lint run ./...

.DEFAULT_GOAL := build
