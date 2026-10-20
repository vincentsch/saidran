# Assaylight

See what your coding agent checked, and what changed afterwards.

Assaylight reads Claude Code and Codex recordings for a Git repository.
It reports recorded commands and edits, grouped repository changes, and review
findings. Claim checks and HTML/Markdown reports come later.

## Build and run

You need Go 1.27.2 to build and Git to inspect repositories.
Go downloads the matching toolchain when automatic toolchain selection is enabled.

```sh
make build
bin/assaylight
bin/assaylight check --no-write --json
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
results appear in the check report.

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

## Repository evidence reports

`assaylight` and `assaylight check` select a session and compare the current
repository with a baseline. Use `--session <ID>` (or `claude:<ID>` /
`codex:<ID>`) to select a recording, or `--transcript <file>` to read one
explicitly, regardless of age. The recording must belong to this repository.

```sh
assaylight check --base origin/main
assaylight check --transcript /path/to/session.jsonl --format json --no-write
assaylight check --session codex:session-id --out ./evidence
```

An explicit `--base` wins. Otherwise the baseline is the merge base of HEAD
with `origin/HEAD`, `origin/main`, or `origin/master`, in that order. Without
one of those refs, HEAD is used and the report says “uncommitted changes only”.
The branch's own upstream never replaces this baseline. Changes include
committed work since the baseline, staged and unstaged work, and non-ignored
untracked files. Git's rename detection preserves renamed paths, including
moves that have not been staged. Inspection uses a disposable index copy
for untracked paths; it leaves your index and working files unchanged. Git
may create its standard empty-file object for intent-to-add entries.
Changed submodules appear in the other group without inspecting their content.

Files are grouped as tests, CI, lockfiles, config, docs, source, or other.
Current modification times later than the recording's end are labeled
“changed after the session ended”; this does not identify who changed a file.
Review findings describe added test skip/focus markers, deleted test files,
check suppressions, CI commands that allow failure, and changes to the `test`
script in package.json/composer.json or the literal Makefile `test` target.
Findings have stable IDs tied to the exact diff context. Removed test functions
inside surviving files are outside the current detection scope.

The JSON model has type `assaylight.report/v1`. It includes the baseline and
reason, session summary, statement ledger, commands with output tails, edits,
file groups, findings, and completeness notes. Claim and dismissal lists are
currently empty. Reports summarize editable local logs and current Git state;
they do not certify correctness or reconstruct a historical checkout.

By default, `report.json` is saved in the worktree's Git directory under
`assaylight/<agent>-<full-session-id>/`. `--out <dir>` writes directly into that
directory. `--no-write` creates no report files or directories. `--dry-run`
previews the report write. `--json`, `--format json`, and `--format=json` print
one redacted JSON document, with no progress or report-path lines; errors use
one JSON document on stderr. `--format text` selects the text summary.
Conflicting formats exit 1. No matching session or no repository exits 5;
runtime failures exit 2. Findings do not change the exit code.

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
