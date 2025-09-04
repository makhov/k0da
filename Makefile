SHELL := /bin/bash

BINARY := k0da
MODULE := github.com/makhov/k0da

TAG := $(shell git describe --tags --abbrev=0 2>/dev/null || echo dev)
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
DATE := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS := -s -w -X $(MODULE)/cmd.Version=$(TAG) -X $(MODULE)/cmd.Commit=$(COMMIT) -X $(MODULE)/cmd.BuildDate=$(DATE)

.PHONY: build test clean build-all deps test-coverage fmt lint run help

build:
	go build -trimpath -ldflags "$(LDFLAGS)" -o $(BINARY) .

test:
	go test ./internal/...

test-e2e:
	$(MAKE) build
	K0DA_BIN=$(PWD)/$(BINARY) go test ./e2e -v

clean:
	rm -f $(BINARY)
	rm -rf dist

# Convenience target to build release-like binaries for multiple OS/ARCH
build-all:
	mkdir -p dist
	GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-linux-amd64 .
	GOOS=linux GOARCH=arm64 go build -trimpath -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-linux-arm64 .
	GOOS=linux GOARCH=arm   go build -trimpath -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-linux-arm .
	GOOS=darwin GOARCH=amd64 go build -trimpath -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-darwin-amd64 .
	GOOS=darwin GOARCH=arm64 go build -trimpath -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-darwin-arm64 .
	GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-windows-amd64.exe .
	GOOS=windows GOARCH=arm64 go build -trimpath -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-windows-arm64.exe .

# Tooling
deps:
	go mod download
	go mod tidy

test-coverage:
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

fmt:
	go fmt ./...

lint:
	golangci-lint run

run: build
	./$(BINARY)

help:
	@echo "Available targets:"
	@echo "  build          - Build the binary"
	@echo "  build-all      - Build for multiple platforms"
	@echo "  deps           - Install dependencies"
	@echo "  test           - Run tests"
	@echo "  test-coverage  - Run tests with coverage"
	@echo "  lint           - Run linter"
	@echo "  fmt            - Format code"
	@echo "  clean          - Clean dist and binary"
	@echo "  run            - Build and run the binary"
