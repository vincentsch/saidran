# saidran pr

Preview and update a report section in this branch's pull request

## Usage

```
saidran pr
```

## Examples

```
saidran pr --dry-run
saidran pr --confirm
saidran pr --transcript session.jsonl --dry-run --json
```

## Flags

- `--base` Compare directly against this Git revision instead of the PR merge base
- `--confirm` Publish the preview without asking
- `--session` Select an ID or claude:ID, codex:ID, opencode:ID, pi:ID
- `--transcript` Read a JSONL recording or OpenCode storage/session/<project>/<id>.json

## Output

Shows text you can read in your terminal.

Use --json to get JSON for scripts and other programs.

## Preview changes

This command makes changes. Use `--dry-run` to see what would change without changing anything.

## Related commands

- saidran check
- saidran sessions
