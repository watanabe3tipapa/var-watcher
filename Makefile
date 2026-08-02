GO ?= go
BIN := varwatch
LDFLAGS := -ldflags="-s -w"

.PHONY: dev build test vet fmt frontend astro demo gen

dev:
	$(GO) run ./cmd/varwatch --tui

build:
	$(GO) build $(LDFLAGS) -o $(BIN) ./cmd/varwatch

test:
	$(GO) test ./...

vet:
	$(GO) vet ./...

fmt:
	gofmt -w .

frontend:
	cd frontend && npm ci && npm run build
	rm -rf internal/web/dist && cp -R frontend/dist internal/web/dist

astro:
	cd astro && npm ci && npm run build

demo:
	cd demo && npm ci && npm run build
