# Reports after an agent finishes

Saidran is a free, open-source CLI for developers who use coding agents like
Claude Code or Codex. It checks what the agent claimed about tests, builds,
linters and type checks against the session log and your Git changes. This
page shows how to get a report automatically after each session, and how to
make the agent check itself before it answers.

A hook writes a report when the agent finishes. It reads the log of that
session and saves an HTML report under the Git directory of the worktree, with
Markdown and JSON versions next to it. Both hook handlers exit 0 even when the
report fails. They never block the agent, never ask for another turn and make
no network calls.

## Claude Code

```sh
saidran install claude --dry-run
saidran install claude
saidran uninstall claude
```

By default the install adds a Stop command to `.claude/settings.local.json`
and, if needed, adds `/.claude/settings.local.json` to the repository's
`info/exclude`. It does not touch `.gitignore` or the shared
`.claude/settings.json`. If the local settings file is tracked in Git, untrack
it before you install. When an existing settings file changes, a backup goes to
`<git dir>/saidran/settings-*.bak`.

`--user` installs into `${CLAUDE_CONFIG_DIR:-~/.claude}/settings.json`
instead, with the backup next to it. Uninstall with the same scope. Other
settings and hooks in the file stay as they are. Invalid JSON is an error, and
the file is not overwritten. Installing a second time changes nothing and
makes no backup. `--dry-run` writes nothing. Restart Claude Code after you
change hooks.

The hook calls the binary by its absolute path, so its folder does not need to
be on PATH. Uninstall also recognizes an older absolute path that ends in
`saidran` or `saidran.exe`, followed by `hook claude` and nothing else. A
command with extra arguments is left alone. If you move the binary, uninstall first and
then install again from the new location.

The [Claude Stop hook](https://code.claude.com/docs/en/hooks#stop) sends
`transcript_path`, `cwd` and, optionally, `last_assistant_message` on stdin.
Saidran reads that transcript and no other. If the payload has a final message,
Saidran uses it together with the start time of the hook. Otherwise it takes
the final response and its time from the transcript. On success the hook
returns `{"systemMessage":"Saidran: ... Report: ..."}`. If the report fails,
the message explains why and has no report path. Outside a Git repository, or
with a transcript that belongs to another repository, the hook exits silently.
On malformed input it returns an explanation and exits 0.

## Codex

```sh
saidran install codex
```

Copy the printed `notify = ["/absolute/path/to/saidran", "hook", "codex"]`
into your user `config.toml`. The path is the binary you ran, escaped for
TOML. Saidran never edits that file. If you already have a notifier, write a
wrapper that calls both and passes the JSON argument through unchanged. To
uninstall, remove the notify entry or the call in your wrapper, and keep your
other notifier.

Codex's [notify setting](https://developers.openai.com/codex/config-reference/)
passes one JSON argument. Its `thread-id` has to be the full Codex session ID,
and `cwd` selects the repository. Saidran never falls back to the latest
session. An optional `last-assistant-message` replaces the final response and
uses the start time of the hook; without it, both come from the transcript. The
Codex hook writes its reports silently, also when the input is malformed or the
report fails. If a report is missing, run `saidran --session codex:<id>` to see
the error.

## Ask the agent to check before finishing

A hook writes the report after the agent is done. A self-check makes the agent
read its evidence before it claims success. Print the instructions with:

```sh
saidran snippet
```

Add the printed text to AGENTS.md or CLAUDE.md. It tells the agent to run
Saidran on its current session, read the recorded results and the edits after
them, and rerun stale or failed checks. That applies even when the report
lists no claims yet. Unknown results stay unknown, and the agent has to say
what it could not check. It must never change tests to make the report look
better.
