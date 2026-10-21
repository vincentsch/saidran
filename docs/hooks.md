# Reports after an agent finishes

Hooks read completion payloads and the existing log readers supply execution
facts. They write HTML, Markdown and JSON reports under the worktree's Git
directory. Both handlers exit 0, never block completion and never request
another turn. They make no network calls.

## Claude Code

```sh
gaugestone install claude --dry-run
gaugestone install claude
gaugestone uninstall claude
```

Project scope is the default. It merges a Stop command into
`.claude/settings.local.json` and adds `/.claude/settings.local.json` to the
repository-local `info/exclude` when needed. It does not edit `.gitignore` or
shared `.claude/settings.json`. If local settings are tracked, untrack them
before installation. Backups of changed existing settings go under
`<git dir>/gaugestone/settings-*.bak`.

`--user` selects `${CLAUDE_CONFIG_DIR:-~/.claude}/settings.json`; backups go
beside it. Use the same scope to uninstall. Existing unrelated settings and
hooks are kept. Invalid JSON fails without overwriting it. Repeat installation
does not rewrite settings or create backups. Dry-run writes nothing.
Restart Claude after changing hooks.

The command uses `os.Executable`, quoted as an absolute path, so the binary's
folder does not need to be on PATH. Uninstall also recognizes an old absolute
path ending in `gaugestone` or `gaugestone.exe`, followed by exactly
`hook claude`. Commands with extra arguments are preserved. Reinstall after
moving the binary: uninstall first, then install from the new location.

The [Claude Stop contract](https://code.claude.com/docs/en/hooks#stop) supplies
`transcript_path`, `cwd` and optionally `last_assistant_message` on stdin.
Gaugestone reads that exact transcript. A supplied final message uses the hook
start time; without it, the transcript's final response and time remain.
It returns `{"systemMessage":"Gaugestone: ... Report: ..."}` on success.
Reporting errors return an explanatory message without a success path.
Outside a Git repository, or with a transcript from another repository,
it exits silently. Malformed input returns an explanation and exit 0.

## Codex

```sh
gaugestone install codex
```

Copy the printed `notify = ["/absolute/path/to/gaugestone", "hook", "codex"]`
into your user `config.toml`. The printed executable comes from
`os.Executable` and is escaped for TOML. Gaugestone never edits the file.
If you already have a notifier, configure a wrapper that calls both commands
and forwards the JSON argument unchanged. To uninstall, remove this notify
entry or its call in your wrapper. Keep your other notifier.

The [Codex notify setting](https://developers.openai.com/codex/config-reference/)
passes a JSON argument. `thread-id` must match the full Codex session ID;
`cwd` selects the repository. Gaugestone never substitutes the latest session.
Optional `last-assistant-message` replaces the final response with hook-start
timing. Without it, the transcript supplies both. Codex hooks write reports
silently, including on malformed input or failure. Run an explicit
`gaugestone --session codex:<id>` to diagnose missing reports.

## Ask the agent to check before finishing

A hook writes reports after completion. A self-check asks the agent to read
its evidence before claiming success. Print the instructions with:

```sh
gaugestone snippet
```

Add the printed text to AGENTS.md or CLAUDE.md if you want that behavior.
It tells the agent to select its current session, inspect recorded results
and later edits, and rerun stale or failed checks. This applies even when no
claims have been extracted yet. Unknown results stay unknown; the agent must
say what it could not check. Never change tests to improve a report.
