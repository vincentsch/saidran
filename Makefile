.PHONY: build test conformance
build:
	CGO_ENABLED=0 go build -trimpath -o bin/gaugestone ./cmd/gaugestone

test:
	go test ./...
	go test -race ./...
	go vet ./...

conformance:
	scripts/conformance.sh
