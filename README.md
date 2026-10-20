# Gaugestone

See what your coding agent checked, and what changed afterwards.

Gaugestone reads Claude Code, Codex, OpenCode and Pi recordings for a Git repository.
It checks final-response claims about tests, builds, linters, and type checks
against recorded runs. Reports also show edits, grouped repository changes,
and review findings in text, HTML, Markdown and JSON reports.

## Install

On Linux and macOS, download and run the installer:

```sh
curl -fsSL https://raw.githubusercontent.com/vincentsch/gaugestone/main/install.sh -o install.sh
bash install.sh
```

The installer verifies SHA-256 and installs to `~/.local/bin`. Set `INSTALL_DIR`
to choose another folder, or `GAUGESTONE_VERSION=v0.1.0` to pin a published version.
It prints the installed path and a PATH setup line when needed. Release downloads
become available after the first public release.

On Windows, download the [amd64 executable](https://github.com/vincentsch/gaugestone/releases/latest/download/gaugestone-windows-amd64.exe)
or [arm64 executable](https://github.com/vincentsch/gaugestone/releases/latest/download/gaugestone-windows-arm64.exe).
Verify its SHA-256 against the release’s `checksums.txt`, rename it to
`gaugestone.exe` and add its folder to PATH.

Git is required. Report generation runs offline and makes no model calls.
Installation and GitHub publication use the network. See the
[command reference](docs/reference/index.md), [command guide](docs/commands.md)
and [contributor checks](CONTRIBUTING.md).

## Build and run

You need Go 1.27.2 to build and Git to inspect repositories.
Go downloads the matching toolchain when automatic toolchain selection is enabled.

```sh
make build
bin/gaugestone
bin/gaugestone check --no-write --json
bin/gaugestone sessions
bin/gaugestone sessions --json
bin/gaugestone --help
bin/gaugestone --version
```

Run the command inside the repository you want to inspect. The list shows
agent, session ID, start, end, duration, first prompt, commands, and confirmed
edits. An asterisk marks the automatic pick. Prompts show at most 60 characters.
Failed or pending edit calls do not count as confirmed edits. The session list
counts edits recorded by the agent tools. Shell edits and individual check
results appear in the check report.

Gaugestone searches `${CLAUDE_CONFIG_DIR:-~/.claude}/projects/` and
`${CODEX_HOME:-~/.codex}/sessions/`. Sessions started in repository subfolders
belong to the same repository. Automatic selection uses sessions from the last
seven days with at least one command or confirmed edit. When two agents finish
less than ten minutes apart, interactive selection offers both; other callers
use the newest. Listing shows that choice without asking a question.

Claude Code, OpenCode and Pi tool calls are paired with their results.
OpenCode 1.1.4 uses its session JSON and sibling message/part storage; Pi 0.85.1
uses version 3 JSONL journals. See [supported agents](docs/agents.md) for log roots,
environment overrides and session-selection examples. Codex recordings need
completed-item events, as produced by Codex 0.163. Older formats need an update
to Codex. Unsupported entries and incomplete recordings produce completeness
notes. Text output groups notes by kind; `--json` includes individual details.
Tokens, secret assignments, and home paths are redacted in output.

`--json` prints one JSON document. An empty list exits 0. A folder outside a
Git repository exits 5. Usage errors exit 1; runtime errors exit 2. Session
listing reads logs and Git metadata without running recorded commands or
writing reports.

## Repository evidence reports

`gaugestone` and `gaugestone check` select a session and compare the current
repository with a baseline. Use `--session <ID>` (or `claude:<ID>` /
`codex:<ID>`) to select a recording, or `--transcript <file>` to read one
explicitly, regardless of age. The recording must belong to this repository.

```sh
gaugestone check --base origin/main
gaugestone check --transcript /path/to/session.jsonl --format json --no-write
gaugestone check --session codex:session-id --out ./evidence
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

The JSON model has type `gaugestone.report/v1`. It includes the baseline and
reason, session summary, statement ledger, commands with output tails, edits,
file groups, findings, completeness notes, claims, and admitted check gaps.
Claims include the exact quote, verdict, deciding runs, relevant edits,
explanation, and next step. Dismissals include reasons and the matching finding IDs.
Reports summarize editable local logs and current Git state;
they do not certify correctness or reconstruct a historical checkout.

Final-response claims are matched in English and German. Earlier progress
messages do not create claims. Negated, conditional, and future assertions
are excluded. Passing lists such as “Tests, vet and build pass” produce one
claim per supported check type. An admission that checks could not be run appears as a gap,
without a verdict. Markdown emphasis and commands keep their original quotes.

The terminal summary shows claims first, with these verdicts:

- **Verified**: a matching run passed with enough coverage and no later edits.
- **Contradicted**: an unresolved failure exists within the claimed scope.
- **Can't tell**: the result, test scope, or edit evidence is uncertain.
- **Stale**: relevant files changed after a passing run.
- **Partial**: a successful run covered less than claimed, only listed tests,
  or found no tests.
- **No matching run**: no matching evidence was found in the session log.

A later failing package run contradicts an earlier passing whole-suite run.
A passing rerun must cover that failed target to resolve it. A filtered run
can verify the exact named test, but gives Partial for a broader package claim.
Scope comes from the recorded runner, directory, paths, and filters. Make
targets are command names; passing build, lint and typecheck targets do not
imply narrower file coverage. Non-Go tools treat `.` as their directory tree.
Gaugestone
does not analyze dependencies or execute commands to discover coverage.
Docs-only edits and edits outside the repository do not make claims stale.
Unknown file writes and edits during a run prevent Verified.

Command counts use original command lines. A failed summary can establish a
failure even when the shell line exits successfully. Only unresolved check
results count as unreliable; exit-attribution notes remain visible when a
summary establishes the outcome. Findings show their full stable ID.
Header and evidence times use your local time zone. The header uses a session
ID prefix long enough to distinguish the discovered recordings.
When no claims are found, the summary says so and keeps the other evidence.
Verdicts check recorded evidence, not whether a bug is fixed.

By default, `report.html`, `report.md` and `report.json` are saved in the
worktree's Git directory under
`gaugestone/<agent>-<full-session-id>/`. `--out <dir>` writes directly into that
directory. `--no-write` creates no report files or directories. `--dry-run`
previews the report write. `--json`, `--format json`, and `--format=json` print
one redacted JSON document, with no progress or report-path lines; errors use
one JSON document on stderr. `--format text` selects the text summary.

Use `--format html` or `--format markdown` to print only that document.
`--open` opens the HTML report after all three files were saved. Suppressed
writes never open a browser. The text summary prints the HTML path only after
a successful write. Reports work from disk and make no external requests.

The HTML report has light and dark themes, command filters, redacted output
tails, and finding IDs with a button to copy a dismissal command. Print views
expand the evidence. Markdown includes check statements and failed command
lines, up to 30, even when the same line also formats files. It stays below
60,000 characters by shortening command details first, then long values and
lists. Omitted items have counts. HTML and JSON keep the complete evidence
within the existing privacy limits.

Both views include admitted gaps, completeness notes, and files changed after
the session ended. Local session logs can be edited. A report is not a
certificate.

Conflicting formats exit 1. No matching session or no repository exits 5;
runtime failures exit 2. Findings do not change the exit code unless `--fail-on` is given.

## Try the example or check your setup

`gaugestone demo` needs no agent history and works outside a repository. It
extracts a recorded Claude Code session on a small Go project, shows a Stale
claim, and writes HTML, Markdown and JSON reports. The transcript stays beside
the extracted repository. The printed report path is inside a temporary
`gaugestone-demo-*` directory; delete that directory when you are done.
`demo --dry-run` previews extraction without creating files.

`gaugestone doctor` checks Git, the current repository and configured agent
log roots. It shows session counts for this repository, the newest observed
agent format versions, unsupported recordings and steps to fix problems.
`--json` provides the same metadata without private session messages.
Outside a Git repository it exits 5 with a hint; missing Git exits 2 with
an installation command. Set `CLAUDE_CONFIG_DIR` or `CODEX_HOME` to use other
log roots.

## Mark intentional findings

```sh
gaugestone dismiss F-0123456789ab --reason "Intentional platform skip"
gaugestone dismiss F-012345 --reason "Intentional platform skip" --dry-run
gaugestone dismissals --json
gaugestone dismissals --remove F-012345
```

Use the full finding ID or a unique prefix with `F-` and at least six hex
characters. Dismiss resolves against the current diff, using check's baseline
rules and an optional `--base`. It needs no session logs or saved report.
Only findings can be dismissed. An unknown ID exits 5; an ambiguous or invalid
prefix exits 1. Neither writes anything.

Reasons are redacted before storage. Dismissals are saved atomically in this
worktree's Git directory under `gaugestone/dismissals.json`. Each entry keeps
the time, code, path and exact redacted diff context. Changed context gives a
new ID and requires another dismissal. Reports retain matching findings and
reasons: collapsed in HTML and Markdown, explicit in text and JSON.
`--dry-run` previews dismissals and removals without changing files.

## Choose an exit policy

```sh
gaugestone check --fail-on contradicted,stale,test_skipped
gaugestone check --fail-on "stale, test_skipped" --no-write --json
```

`--fail-on` accepts comma-separated `verified`, `stale`, `contradicted`,
`partial`, `no_matching_run`, `cant_tell`, `test_skipped`, `test_deleted`,
`check_disabled`, `ci_ignores_failure`, and `test_command_changed`. Whitespace
around values is ignored. Unknown or empty values exit 1.

Matching claim verdicts or active findings give exit 10 after successful
report generation. Dismissed findings do not count. Normal stdout stays
intact, including one complete JSON report with either JSON alias. One stderr
line names the matches. Report-write failures exit 2 instead; ordinary JSON
errors still use empty stdout and a JSON error on stderr. `--no-write`
evaluates the report without saving files. Without `--fail-on`, findings
and verdicts leave exit 0.

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
