# Reports after an agent finishes

A hook writes a report when your agent finishes. It reads that session's log
and saves an HTML report under the worktree's Git directory. Markdown and JSON
versions are saved alongside it.
Both handlers exit 0 even if reporting fails. They never block completion or
request another turn. They make no network calls.

## Claude Code

```sh
saidran install claude --dry-run
saidran install claude
saidran uninstall claude
```

By default, installation adds a Stop command to
`.claude/settings.local.json` and adds `/.claude/settings.local.json` to the
repository-local `info/exclude` when needed. It does not edit `.gitignore` or
shared `.claude/settings.json`. If local settings are tracked, untrack them
before installation. Backups of changed existing settings go under
`<git dir>/saidran/settings-*.bak`.

`--user` selects `${CLAUDE_CONFIG_DIR:-~/.claude}/settings.json`; backups go
beside it. Use the same scope to uninstall. Existing unrelated settings and
hooks are kept. Invalid JSON fails without overwriting it. Installing again
does not rewrite settings or create backups. Dry-run writes nothing.
Restart Claude after changing hooks.

The hook uses the installed binary's absolute path, so its folder does not
need to be on PATH. Uninstall also recognizes an old absolute
path ending in `saidran` or `saidran.exe`, followed by exactly
`hook claude`. Commands with extra arguments are preserved. Reinstall after
moving the binary: uninstall first, then install from the new location.

The [Claude Stop contract](https://code.claude.com/docs/en/hooks#stop) supplies
`transcript_path`, `cwd` and optionally `last_assistant_message` on stdin.
Saidran reads that exact transcript. A supplied final message uses the hook
start time; without it, the transcript's final response and time remain.
It returns `{"systemMessage":"Saidran: ... Report: ..."}` on success.
Reporting errors return an explanatory message without a success path.
Outside a Git repository, or with a transcript from another repository,
it exits silently. Malformed input returns an explanation and exit 0.

## Codex

```sh
saidran install codex
```

Copy the printed `notify = ["/absolute/path/to/saidran", "hook", "codex"]`
into your user `config.toml`. The path points to the binary you ran and is
escaped for TOML. Saidran never edits the file.
If you already have a notifier, configure a wrapper that calls both commands
and forwards the JSON argument unchanged. To uninstall, remove this notify
entry or its call in your wrapper. Keep your other notifier.

The [Codex notify setting](https://developers.openai.com/codex/config-reference/)
passes a JSON argument. `thread-id` must match the full Codex session ID;
`cwd` selects the repository. Saidran never substitutes the latest session.
Optional `last-assistant-message` replaces the final response with hook-start
timing. Without it, the transcript supplies both. Codex hooks write reports
silently, including on malformed input or failure. Run an explicit
`saidran --session codex:<id>` to diagnose missing reports.

## Ask the agent to check before finishing

A hook writes reports after completion. A self-check asks the agent to read
its evidence before claiming success. Print the instructions with:

```sh
saidran snippet
```

Add the printed text to AGENTS.md or CLAUDE.md if you want that behavior.
The instructions tell the agent to check its current session. It reads recorded
results and later edits, then reruns stale or failed checks. This applies even when no
claims have been extracted yet. Unknown results stay unknown; the agent must
say what it could not check. Never change tests to improve a report.
