# saidran

See what your coding agent checked

## Usage

```
saidran
```

## Flags

- `--base` Compare against this Git revision
- `--fail-on` Exit 10 after reporting matching verdicts or active finding codes (comma-separated)
- `--format` Output format: text, html, markdown or json
- `--no-write` Print evidence without writing a report
- `--open` Open the written HTML report in your browser
- `--out` Write report.html, report.md and report.json into this directory
- `--session` Select an ID or claude:ID, codex:ID, opencode:ID, pi:ID
- `--transcript` Read a JSONL recording or OpenCode storage/session/<project>/<id>.json

## Output modes

human, json

## Changes state

This command changes state and honors `--dry-run`.
