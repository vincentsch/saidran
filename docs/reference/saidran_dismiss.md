# saidran dismiss

Mark a finding as intentional

## Usage

```
saidran dismiss <finding-id>
```

## Examples

```
saidran dismiss F-0123456789ab --reason "Intentional skip"
saidran dismiss F-012345 --reason test --dry-run
```

## Flags

- `--base` Compare against this Git revision
- `--reason` Why this finding is intentional (required)

## Output

Shows text you can read in your terminal.

Use --json to get JSON for scripts and other programs.

## Preview changes

This command makes changes. Use `--dry-run` to see what would change without changing anything.

## Related commands

- saidran dismissals
- saidran check
