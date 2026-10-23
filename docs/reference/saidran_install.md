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

## Output

Shows text you can read in your terminal.

Use --json to get JSON for scripts and other programs.

## Preview changes

This command makes changes. Use `--dry-run` to see what would change without changing anything.

## Related commands

- saidran hook
- saidran snippet
