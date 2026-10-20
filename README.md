# Assaylight

See what your coding agent checked, and what changed afterwards.

Assaylight reads Claude Code and Codex recordings for a Git repository.
This early version lists sessions. Claim checks and reports come later.

## Build and run

You need Go 1.27.2 to build and Git to list sessions.
Go downloads the matching toolchain when automatic toolchain selection is enabled.

```sh
make build
bin/assaylight sessions
bin/assaylight sessions --json
bin/assaylight --help
bin/assaylight --version
```

Run the command inside the repository you want to inspect. The list shows
agent, session ID, start, end, duration, first prompt, commands, and confirmed
edits. An asterisk marks the automatic pick. Prompts show at most 60 characters.
Failed or pending edit calls do not count as confirmed edits. The session list
counts edits recorded by the agent tools. Shell edits and individual check
results are analyzed internally for the reports that follow.

Assaylight searches `${CLAUDE_CONFIG_DIR:-~/.claude}/projects/` and
`${CODEX_HOME:-~/.codex}/sessions/`. Sessions started in repository subfolders
belong to the same repository. Automatic selection uses sessions from the last
seven days with at least one command or confirmed edit. When two agents finish
less than ten minutes apart, interactive selection offers both; other callers
use the newest. Listing shows that choice without asking a question.

Claude Code tool calls are paired with their results. Codex recordings need
completed-item events, as produced by Codex 0.163. Older formats need an update
to Codex. Unsupported entries and incomplete recordings produce completeness
notes. Text output groups notes by kind; `--json` includes individual details.
Tokens, secret assignments, and home paths are redacted in output.

`--json` prints one JSON document. An empty list exits 0. A folder outside a
Git repository exits 5. Usage errors exit 1; runtime errors exit 2. Session
listing reads logs and Git metadata without running recorded commands or
writing reports.

## Development checks

```sh
make test
make conformance
gofmt -l .
```

The tests extract all eight recordings into temporary repositories and isolate
agent log roots. Conformance checks use an isolated fixture repository.

## License

MIT
