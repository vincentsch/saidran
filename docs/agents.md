# Supported coding agents

Saidran is a free, open-source CLI for developers who use coding agents. It
reads the session logs of Claude Code, Codex, OpenCode and Pi and checks what
the agent claimed about tests, builds, linters and type checks against the
recorded runs and your Git changes. This page lists the tested versions, where
each agent keeps its logs and what each log format can and cannot tell
Saidran. Saidran never runs the agents and never sends their transcripts over
the network.

| Agent | Recorded version | Default log root | Override | Session reference |
| --- | --- | --- | --- | --- |
| Claude Code | 2.1.293 | `~/.claude/projects` | `CLAUDE_CONFIG_DIR` (parent of `projects`) | `claude:<id>` |
| Codex | 0.163.0 | `~/.codex/sessions` | `CODEX_HOME` (parent of `sessions`) | `codex:<id>` |
| OpenCode | 1.1.4 | `~/.local/share/opencode/storage` | `XDG_DATA_HOME` (parent of `opencode`) | `opencode:<id>` |
| Pi | 0.85.1 (journal version 3) | `~/.pi/agent/sessions` | `PI_CODING_AGENT_DIR` (parent of `sessions`) | `pi:<id>` |

Run `saidran sessions` inside a Git repository to list its recordings.
`saidran doctor` shows all four log roots with their session counts and format
diagnostics. A recording belongs to a repository when its recorded working
directory is the repository root or a folder inside it. A log root behind a
symlink also works.

```sh
saidran sessions
saidran --session opencode:ses_example
saidran --session pi:01a15b72
```

Use the full ID or a unique prefix of it. Add the agent name when two agents
share an ID; `claude-code:<id>` also works. The automatic pick is the newest
session with commands or edits that ended in the last seven days. When two
different agents finished less than ten minutes apart, Saidran asks you which
one to read. In a script it picks the newest one and says so. An explicit
transcript is not limited to the last seven days.

For OpenCode, keep the whole storage layout together:

```text
storage/
  session/<project-id>/<session-id>.json
  message/<session-id>/*.json
  part/<message-id>/*.json
```

Pass the session JSON. A message file, a part file or an exported stream does
not work:

```sh
saidran --transcript /path/to/storage/session/project/session.json
saidran --transcript /path/to/pi-session.jsonl
```

Saidran detects the agent from the content of the file. OpenCode's session
file gives the working directory and the version. Its tool parts hold the
command times and output, with the exit code in `state.metadata.exit`. A tool
that is still pending, or a result without an exit code, stays unknown. OpenCode
child sessions are not merged.

For Pi, Saidran pairs each tool call with its result by the tool-call ID. A
completed built-in bash result with `isError: false` counts as exit 0, even
without `details`. On an error, only the appended `Command exited with code N`
trailer gives Saidran a nonzero exit; the same text inside successful output is
ordinary output. Aborted, timed-out and missing results stay unknown. Pi's
`details` field describes output truncation, not the exit status. Pi's journal
records the journal format version and not the CLI version, so doctor reports
the CLI version as not recorded.

Older Codex logs without completed-item events, and OpenCode or Pi formats
Saidran does not support, produce an error that tells you how to record a
supported session. Such logs cannot give Verified evidence. Entries Saidran
does not understand, and recordings that end mid-line, show up in the
completeness notes. Redaction and the verdict rules are the same for all four
agents.

The Claude Stop hook and the Codex notify hook read the exact transcript path
or the full thread ID from the hook payload. See [hooks](hooks.md) for
installing them and for the final-message override. For OpenCode and Pi, run a
report by hand or use the self-check snippet.
