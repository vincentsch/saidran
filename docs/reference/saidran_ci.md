# saidran ci

Write CI reports from a diff and optional checkout-local log

## Usage

```
saidran ci
```

## Examples

```
saidran ci
saidran ci --base origin/main --fail-on test_skipped
saidran ci --transcript session.jsonl --out reports --json
```

## Flags

- `--base` Compare against this Git revision
- `--fail-on` Exit 10 after reporting matching verdicts or active finding codes (comma-separated)
- `--format` Output format: text, html, markdown or json
- `--out` Write report.html, report.md and report.json into this directory
- `--transcript` Read a JSONL recording or OpenCode storage/session/<project>/<id>.json

## Output modes

human, json

## Changes state

This command changes state and honors `--dry-run`.

## Related commands

- saidran check
- saidran pr
