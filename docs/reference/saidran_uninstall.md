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

## Output

Prints text output by default.

Use `--json` for JSON output.

## Preview changes

Use `--dry-run` to preview changes without applying them.

## Related commands

- saidran hook
- saidran snippet
