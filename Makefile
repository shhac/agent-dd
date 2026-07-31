BINARY := agent-dd
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)

build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/agent-dd

# -race is load-bearing: mockdd's per-server store is reached concurrently
# (httptest serves each request on its own goroutine), and a lock-discipline
# regression there would pass every assertion while silently corrupting the
# fixture every other package's tests are driven from.
test:
	go test ./... -count=1 -race

test-short:
	go test ./... -count=1 -short

lint:
	golangci-lint run ./...

# Scoped to tracked files: this repo keeps a module cache under .cache/, which
# the go tool skips (dot-directory) but gofmt and goimports walk into, so a bare
# `-w .` rewrites vendored dependencies and makes `gofmt -l .` report noise.
fmt:
	gofmt -w $$(git ls-files '*.go')
	@command -v goimports >/dev/null && goimports -w $$(git ls-files '*.go') || echo "goimports not installed (optional; install: go install golang.org/x/tools/cmd/goimports@latest)"

clean:
	rm -f $(BINARY)
	rm -f release/agent-dd-*

dev:
	go run ./cmd/agent-dd $(ARGS)

vet:
	go vet ./...

mock:
	go run ./cmd/mockdd

mock-dev:
	DD_API_URL=http://localhost:8321/api DD_API_KEY=mock DD_APP_KEY=mock go run ./cmd/agent-dd $(ARGS)

.PHONY: build test test-short lint fmt clean dev vet mock mock-dev
