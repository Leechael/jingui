VERSION   ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT    ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
LDFLAGS   := -X github.com/aspect-build/jingui/internal/version.Version=$(VERSION) \
             -X github.com/aspect-build/jingui/internal/version.GitCommit=$(COMMIT)

.PHONY: build build-client build-server test clean lint bdd ci \
	build-client-linux-amd64 build-client-linux-arm64 build-client-darwin-amd64 build-client-darwin-arm64 \
	build-server-linux-amd64 build-server-linux-arm64 build-server-darwin-amd64 build-server-darwin-arm64 \
	build-all

# Default: build for current platform
build: build-client build-server

build-client:
	go build -ldflags '$(LDFLAGS)' -o bin/jingui ./cmd/jingui

build-server:
	go build -ldflags '$(LDFLAGS)' -o bin/jingui-server ./cmd/jingui-server

# ── Cross-compilation (client) ──────────────────────────────────────
build-client-linux-amd64:
	GOOS=linux GOARCH=amd64 go build -ldflags '$(LDFLAGS)' -o bin/linux-amd64/jingui ./cmd/jingui

build-client-linux-arm64:
	GOOS=linux GOARCH=arm64 go build -ldflags '$(LDFLAGS)' -o bin/linux-arm64/jingui ./cmd/jingui

build-client-darwin-amd64:
	GOOS=darwin GOARCH=amd64 go build -ldflags '$(LDFLAGS)' -o bin/darwin-amd64/jingui ./cmd/jingui

build-client-darwin-arm64:
	GOOS=darwin GOARCH=arm64 go build -ldflags '$(LDFLAGS)' -o bin/darwin-arm64/jingui ./cmd/jingui

# ── Cross-compilation (server) ──────────────────────────────────────
build-server-linux-amd64:
	GOOS=linux GOARCH=amd64 go build -ldflags '$(LDFLAGS)' -o bin/linux-amd64/jingui-server ./cmd/jingui-server

build-server-linux-arm64:
	GOOS=linux GOARCH=arm64 go build -ldflags '$(LDFLAGS)' -o bin/linux-arm64/jingui-server ./cmd/jingui-server

build-server-darwin-amd64:
	GOOS=darwin GOARCH=amd64 go build -ldflags '$(LDFLAGS)' -o bin/darwin-amd64/jingui-server ./cmd/jingui-server

build-server-darwin-arm64:
	GOOS=darwin GOARCH=arm64 go build -ldflags '$(LDFLAGS)' -o bin/darwin-arm64/jingui-server ./cmd/jingui-server

# ── Build all platforms ─────────────────────────────────────────────
build-all: \
	build-client-linux-amd64 build-client-linux-arm64 \
	build-client-darwin-amd64 build-client-darwin-arm64 \
	build-server-linux-amd64 build-server-linux-arm64 \
	build-server-darwin-amd64 build-server-darwin-arm64

test:
	go test ./... -v -race

lint:
	go vet ./...
	@test -z "$$(gofmt -l .)" || { echo "gofmt: unformatted files:"; gofmt -l .; exit 1; }

bdd:
	go test -tags bdd ./internal -v -run TestBDD

ci: lint test bdd

clean:
	rm -rf bin/
