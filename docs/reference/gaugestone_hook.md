# gaugestone hook

Write a report for an agent completion event

## Usage

```
gaugestone hook <claude|codex> [json]
```

## Examples

```
gaugestone hook claude
gaugestone hook codex '{"thread-id":"session-id","cwd":"/repo"}'
```

## Changes state

This command changes state and honors `--dry-run`.

## Related commands

- gaugestone install
