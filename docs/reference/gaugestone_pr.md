# gaugestone pr

Preview and update a report section in this branch's pull request

## Usage

```
gaugestone pr
```

## Examples

```
gaugestone pr --dry-run
gaugestone pr --confirm
gaugestone pr --transcript session.jsonl --dry-run --json
```

## Flags

- `--base` Compare directly against this Git revision instead of the PR merge base
- `--confirm` Publish the preview without asking
- `--session` Select an ID or claude:ID, codex:ID, opencode:ID, pi:ID
- `--transcript` Read a JSONL recording or OpenCode storage/session/<project>/<id>.json

## Output modes

human, json

## Changes state

This command changes state and honors `--dry-run`.

## Related commands

- gaugestone check
- gaugestone sessions
