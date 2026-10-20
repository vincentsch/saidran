# gaugestone uninstall

Remove the Gaugestone Claude hook

## Usage

```
gaugestone uninstall <claude|codex>
```

## Examples

```
gaugestone uninstall claude --dry-run
gaugestone uninstall claude --user
```

## Flags

- `--project` Use repository-local Claude settings (default)
- `--user` Use user Claude settings

## Output modes

human, json

## Changes state

This command changes state and honors `--dry-run`.

## Related commands

- gaugestone hook
- gaugestone snippet
