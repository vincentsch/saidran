VERSION ?= $(shell git describe --tags --always 2>/dev/null || echo dev)
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo dev)
LDFLAGS = -s -w -X github.com/vincentsch/gaugestone/internal/version.Version=$(VERSION) -X github.com/vincentsch/gaugestone/internal/version.Commit=$(COMMIT)
PLATFORMS = linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64

.PHONY: build build-all test race lint conformance docs docs-check clean
build:
	CGO_ENABLED=0 go build -trimpath -ldflags '$(LDFLAGS)' -o bin/gaugestone ./cmd/gaugestone

build-all:
	@mkdir -p dist
	@rm -f dist/gaugestone-* dist/checksums.txt
	@set -e; for platform in $(PLATFORMS); do \
		os=$${platform%/*}; arch=$${platform#*/}; \
		output=dist/gaugestone-$$os-$$arch; \
		if [ "$$os" = windows ]; then output=$$output.exe; fi; \
		echo "Building $$output"; \
		CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch go build -trimpath -ldflags '$(LDFLAGS)' -o "$$output" ./cmd/gaugestone; \
	done
	@cd dist && if command -v sha256sum >/dev/null 2>&1; then sha256sum gaugestone-* > checksums.txt; else shasum -a 256 gaugestone-* > checksums.txt; fi

test:
	go test ./...
race:
	go test -race ./...
lint:
	@test -z "$$(gofmt -l .)" || { gofmt -l .; exit 1; }
	go vet ./...
conformance:
	scripts/conformance.sh
docs:
	go run ./cmd/generate-command-docs
docs-check:
	go run ./cmd/generate-command-docs --check
clean:
	rm -rf bin dist
