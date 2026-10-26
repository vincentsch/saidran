# Add a report to a pull request

Saidran is a free, open-source CLI for developers who use coding agents like
Claude Code or Codex. It checks what the agent claimed about tests, builds,
linters and type checks against the session log and your Git changes.
`saidran pr` puts that report into the description of the pull request for
your current branch, so a reviewer sees it first.

Install [GitHub CLI](https://cli.github.com/) and run `gh auth login` first.
The command reads your local session log and uses `gh` to read and edit the
pull request. It never runs recorded commands, and it writes no report files
on your machine.

## Preview and publish

See the section without touching the pull request:

```sh
saidran pr --dry-run
```

`saidran pr` without `--dry-run` shows the final redacted section and asks
whether to publish it. The preview already includes any shortening needed to
fit the description. `--confirm` publishes without the question, and you still
see the preview. `--dry-run` always wins, also together with `--confirm`.
`--no-prompt` refuses to publish unless you also pass `--confirm`.

For scripts, `--json` returns one document with `section`, `url` and `status`.
The section string includes its markers. The status is `previewed` for a dry
run and `updated` after a successful edit. JSON mode asks no questions and
needs `--confirm` to publish. A failed command returns the usual JSON error on
stderr and no document on stdout.

```sh
saidran pr --dry-run --json
saidran pr --confirm --json
```

A report summarizes a local log and your Git changes. Saidran cannot tell
whether someone edited the log. Read the preview before you share it.
Redaction masks common tokens and home paths, but it is not a secret scanner.
For the full HTML and JSON evidence, within the privacy limits, run
`saidran check` on your machine.

## Choose the session and comparison

The session is picked the same way as for `saidran check`. `--session <id>`
picks a discovered session, and `--transcript <file>` an explicit recording.
You cannot use both. All four supported agents go through the same report
model and the same Markdown renderer.

The default comparison is the merge base of HEAD and the target commit of the
pull request, which `gh pr view` reports as `baseRefOid`. That works for a
non-default target branch and for a fork whose `origin/main` differs from the
target repository. Saidran never fetches on its own. If that commit is missing
on your machine, fetch the base branch of the target repository and run the
command again.

`--base <revision>` compares directly against that revision, like
`saidran check --base`.

## Keep the rest of the description

The report sits between these two comments:

```html
<!-- saidran:start -->
<!-- saidran:end -->
```

If the description has no markers, Saidran appends the section after two
newlines. On later runs it replaces the text between one ordered pair of
markers. Everything outside stays byte for byte as it was, including
whitespace and line endings. Markers that are malformed, duplicated, reversed
or unbalanced stop the command before it edits anything. To repair a
description, remove both markers or leave one ordered pair. Saidran writes the
whole body to a temporary file, passes it to `gh pr edit --body-file` and
deletes the file afterwards.

## Description size and errors

GitHub caps the body of a pull request at 65,536 UTF-8 bytes. Your own text
and the markers count, with the newlines between them. Saidran uses at most
60,000 bytes of what is left for the report. If that is not enough, it drops
the command details first, then shortens long fields, then summarizes lists
that overflow. It keeps the verdict totals, the completeness counts, the note
about local logs, the footer and the notices about what was left out. A
shortened report is still valid Markdown and valid UTF-8.

A description of 64,000 bytes can still get a report if the shortest summary
fits in the remaining space. If even that does not fit, Saidran exits 2
without editing the pull request. Shorten the description and try again.

Missing or unauthenticated `gh` exits 3 with a hint. No current pull request
exits 5. Usage and confirmation errors exit 1, and other runtime failures
exit 2.

## GitHub Action

The [GitHub Action](github-action.md) saves the summary and the HTML artifact
and then creates or updates its own marked bot comment. It works from the diff
alone, or with a CLI transcript recorded in the same checkout. The Action
compares directly against the base SHA of the pull request event and expects
checkout's default merge ref with `fetch-depth: 0`. That differs from the
merge base that `saidran pr` infers.

```yaml
- uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7
  with:
    fetch-depth: 0
- uses: vincentsch/saidran@v0.1.0
  with:
    fail-on: contradicted,stale,test_skipped
```

The Action installs the binary version embedded in it (v0.1.0 here), also with
a SHA pin or a local checkout. It never picks the latest release on its own.

Use `pull_request` with `contents: read` and `pull-requests: write`. A run
from a fork may not be able to comment, and the summary and the artifact are
still there. The [guide](github-action.md) has a complete workflow with
permissions and the rules for transcripts.
