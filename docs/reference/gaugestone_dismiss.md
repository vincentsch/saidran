# gaugestone dismiss

Mark a finding as intentional

## Usage

```
gaugestone dismiss <finding-id>
```

## Examples

```
gaugestone dismiss F-0123456789ab --reason "Intentional skip"
gaugestone dismiss F-012345 --reason test --dry-run
```

## Flags

- `--base` Compare against this Git revision
- `--reason` Why this finding is intentional (required)

## Output modes

human, json

## Changes state

This command changes state and honors `--dry-run`.

## Related commands

- gaugestone dismissals
- gaugestone check
