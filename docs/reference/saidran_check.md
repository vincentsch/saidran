# saidran check

Show session evidence and repository changes

## Usage

```
saidran check
```

## Examples

```
saidran check
saidran check --transcript session.jsonl --json
saidran check --base origin/main --no-write
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

## Output

Shows text you can read in your terminal.

Use --json to get JSON for scripts and other programs.

## Preview changes

This command makes changes. Use `--dry-run` to see what would change without changing anything.

## Related commands

- saidran sessions
