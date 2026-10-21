# saidran hook

Write a report for an agent completion event

## Usage

```
saidran hook <claude|codex> [json]
```

## Examples

```
saidran hook claude
saidran hook codex '{"thread-id":"session-id","cwd":"/repo"}'
```

## Changes state

This command changes state and honors `--dry-run`.

## Related commands

- saidran install
