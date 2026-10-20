# gaugestone install

Install a Claude hook or print Codex notify configuration

## Usage

```
gaugestone install <claude|codex>
```

## Examples

```
gaugestone install claude --dry-run
gaugestone install claude --user
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
