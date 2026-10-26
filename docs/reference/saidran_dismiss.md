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

Prints text output by default.

Use `--json` for JSON output.

## Preview changes

Use `--dry-run` to preview changes without applying them.

## Related commands

- saidran dismissals
- saidran check
