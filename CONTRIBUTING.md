# Contributing

Saidran is pre-1.0. Command names and JSON formats may change before 1.0.
Use the Go toolchain specified in go.mod and install git before running tests.

## Check a change

Run these checks before sending a change:

```bash
make lint build test race docs-check conformance
scripts/check-fixtures.sh
git diff --check
```

`make build-all` builds static binaries for Linux, macOS and Windows on amd64
and arm64. `make clean` removes bin/ and dist/. Windows CI builds and vets the
code; tests currently need Unix paths. macOS CI runs `go test ./...`.

## Change commands and docs together

Use `make docs` to regenerate docs/reference/ through rungrad's docsgen API.
Keep explanations in docs/commands.md or other handwritten pages outside that
folder. `make docs-check` detects missing, stale and orphaned generated files.

Prefer tests of observable behavior, including verdicts, reports and exit codes.
Use sanitised fixtures and local fakes. Recorded commands must never execute
while Saidran evaluates a session. Report generation must stay offline and
must not call a model.

Keep public text plain and specific. Do not commit credentials, private session
logs, machine-specific paths, build output or local operator files. Read
RELEASE.md before preparing a release.
