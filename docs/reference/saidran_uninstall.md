# saidran uninstall

Remove the Saidran Claude hook

## Usage

```
saidran uninstall <claude|codex>
```

## Examples

```
saidran uninstall claude --dry-run
saidran uninstall claude --user
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
