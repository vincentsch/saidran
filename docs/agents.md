# Supported coding agents

Gaugestone reads local recordings from four coding agents. It never runs the
agents or sends their transcripts over the network.

| Agent | Recorded version | Default log root | Override | Session reference |
| --- | --- | --- | --- | --- |
| Claude Code | 2.1.293 | `~/.claude/projects` | `CLAUDE_CONFIG_DIR` (parent of `projects`) | `claude:<id>` |
| Codex | 0.163.0 | `~/.codex/sessions` | `CODEX_HOME` (parent of `sessions`) | `codex:<id>` |
| OpenCode | 1.1.4 | `~/.local/share/opencode/storage` | `XDG_DATA_HOME` (parent of `opencode`) | `opencode:<id>` |
| Pi | 0.85.1 (journal version 3) | `~/.pi/agent/sessions` | `PI_CODING_AGENT_DIR` (parent of `sessions`) | `pi:<id>` |

Run `gaugestone sessions` inside a Git repository to see its recordings, and
`gaugestone doctor` to see all four roots, counts and format diagnostics. A
recording belongs to the repository when its recorded working directory is the
repository root or a directory inside it. Symlinked log roots work too.

```sh
gaugestone sessions
gaugestone --session opencode:ses_example
gaugestone --session pi:01a15b72
```

Full IDs and unique ID prefixes work. Agent prefixes disambiguate IDs shared by
different agents; `claude-code:<id>` also works. Automatic selection uses the
most recent session with commands or edits that ended within seven days. When
two different agents finish less than ten minutes apart, interactive use asks
which to select; noninteractive use picks the newest and explains the choice.
Explicit transcripts bypass the seven-day limit.

For OpenCode, keep the complete storage layout:

```text
storage/
  session/<project-id>/<session-id>.json
  message/<session-id>/*.json
  part/<message-id>/*.json
```

Pass the session JSON, rather than a message, part or exported stream:

```sh
gaugestone --transcript /path/to/storage/session/project/session.json
gaugestone --transcript /path/to/pi-session.jsonl
```

Gaugestone detects the agent from the content. OpenCode's session file supplies
the working directory and version; tool parts supply command times, output and
`state.metadata.exit`. Pending tools and results without an exit remain unknown.
OpenCode child sessions are not merged.

Pi calls and results are paired by tool-call ID. A completed built-in bash
result with `isError: false` supplies exit 0, even without `details`. On an
error, only an appended `Command exited with code N` trailer supplies a nonzero
exit. Successful output containing that text is ordinary output. Aborted,
timed-out and missing results remain unknown. Pi's `details` describes output
truncation, not exit status. Its journal records the format version, rather than
the Pi CLI version, so doctor reports the CLI version as not recorded.

Older Codex logs without completed-item events and unsupported OpenCode or Pi
formats produce an error, with instructions to record a supported session. They
cannot supply Verified evidence. Unknown entries and incomplete recordings are
reported through completeness notes. All reports apply the same redaction and
verdict rules regardless of agent.

Claude Stop and Codex notify integrations select the payload's exact transcript
or full thread ID. See [hooks](hooks.md) for installation and final-message
overrides. OpenCode and Pi use manual reports or the self-check snippet.
