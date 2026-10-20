# Add a report to a pull request

`gaugestone pr` adds a report section to the current branch's pull request.
Install [GitHub CLI](https://cli.github.com/) and run `gh auth login` first.
The command reads local session evidence, then uses `gh` to read or update the PR.
It never executes recorded commands or writes local report files or directories.

## Preview and publish

Preview the section without changing the PR:

```sh
gaugestone pr --dry-run
```

Run `gaugestone pr` to see the final redacted section and answer the confirmation
question. The preview includes any shortening needed to fit the description.
Use `--confirm` to publish without the question; the preview still appears.
`--dry-run` always takes precedence, including with `--confirm`.
`--no-prompt` refuses publication unless you also give `--confirm`.

For scripts, `--json` returns one document with `section`, `url` and `status`.
The section string includes its markers. Status is `previewed` for dry-run and
`updated` after a successful edit. JSON mode suppresses questions and requires
`--confirm` for publication. Failed commands return the usual JSON error on
stderr without a success document on stdout.

```sh
gaugestone pr --dry-run --json
gaugestone pr --confirm --json
```

Reports summarize local logs and Git changes. Logs can be edited, and a report
is not a certificate. Check the preview before sharing it. Redaction masks common
tokens and home paths, but it is not a secret scanner. Run `gaugestone check`
locally to obtain full HTML/JSON evidence within privacy limits.

## Choose the session and comparison

Automatic session selection works like `gaugestone check`. Use `--session <id>`
for a discovered session or `--transcript <file>` for an explicit recording.
These two flags cannot be combined. All four supported agents use the same
report model and Markdown renderer.

The default comparison uses the merge base of HEAD and the PR's target commit,
reported by `gh pr view` as `baseRefOid`. This works with non-default branches
and forks whose `origin/main` differs from the target repository. Gaugestone
never fetches implicitly. If that commit is missing locally, fetch the PR target
repository's base branch and try again.

`--base <revision>` overrides that default and compares directly against the
given revision, just as `gaugestone check --base` does.

## Keep the rest of the description

The report is wrapped in these comments:

```html
<!-- gaugestone:start -->
<!-- gaugestone:end -->
```

When neither marker exists, Gaugestone appends the section with two separating
newlines. One ordered pair is replaced on later runs. Text outside the marked
section stays byte for byte unchanged, including whitespace and line endings.
Malformed, duplicate, reversed or unbalanced markers stop the command before
editing. Remove both markers or keep one ordered pair to repair the description.
The complete body is passed to `gh pr edit --body-file` through a temporary file,
which is removed afterwards.

## Description size and errors

The complete PR body is capped at 65,536 UTF-8 bytes. The renderer gets the space
left after preserved text, markers and separators, up to 60,000 bytes. It removes
command details first, shortens long fields next, then summarizes overflowing
lists. It keeps verdict totals, completeness counts, the local-log disclaimer,
footer and omission notices. Escaping and shortening preserve balanced markup
and UTF-8 text.

A 64,000-byte description can fit if the shortened summary fits in the remaining
space. If even the shortest summary cannot fit, shorten the existing description
and try again. Gaugestone exits 2 without editing the PR.

Missing or unauthenticated `gh` exits 3 with a fix. No current PR exits 5.
Usage and confirmation errors exit 1; other runtime failures exit 2.
