.PHONY: build test conformance
build:
	CGO_ENABLED=0 go build -trimpath -o bin/assaylight ./cmd/assaylight

test:
	go test ./...
	go test -race ./...
	go vet ./...

conformance:
	scripts/conformance.sh
