# saidran install

Install a Claude hook or print Codex notify configuration

## Usage

```
saidran install <claude|codex>
```

## Examples

```
saidran install claude --dry-run
saidran install claude --user
```

## Flags

- `--project` Use repository-local Claude settings (default)
- `--user` Use user Claude settings

## Output modes

human, json

## Changes state

This command changes state and honors `--dry-run`.

## Related commands

- saidran hook
- saidran snippet
