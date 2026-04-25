.PHONY: build test clean install lint ci

VERSION ?= dev
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
LDFLAGS := -ldflags="-s -w -X 'github.com/luuuc/council/internal/cmd.version=$(VERSION)' -X 'github.com/luuuc/council/internal/cmd.commit=$(COMMIT)'"

build:
	go build $(LDFLAGS) -o bin/council ./cmd/council

test:
	go test -v ./...

clean:
	rm -rf bin/

install: build
	mkdir -p $(HOME)/.local/bin
	cp bin/council $(HOME)/.local/bin/council

lint:
	@command -v golangci-lint >/dev/null 2>&1 || \
		(echo "Installing golangci-lint..." && go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest)
	@PATH="$$PATH:$$(go env GOPATH)/bin" golangci-lint run

# Run all CI checks locally before pushing
ci: build test lint
	@echo "All CI checks passed!"

# Development
run:
	go run ./cmd/council $(ARGS)

# Cross-compilation
build-all:
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o bin/council-darwin-amd64 ./cmd/council
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o bin/council-darwin-arm64 ./cmd/council
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o bin/council-linux-amd64 ./cmd/council
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o bin/council-linux-arm64 ./cmd/council
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o bin/council-windows-amd64.exe ./cmd/council
