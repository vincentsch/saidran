# Commands

Saidran is a free, open-source CLI for developers who use coding agents like
Claude Code or Codex. It checks what the agent claimed about tests, builds,
linters and type checks against the session log and your Git changes. This
page is the overview of its commands and of the shared flags for the baseline,
the output and the exit code. The [command reference](reference/index.md) is
generated from the CLI and lists every flag with examples.

| Command | What it does |
| --- | --- |
| `saidran check` | Prints the summary to the terminal and saves an HTML report with Markdown and JSON versions. This is the default command. |
| `saidran ci` | Writes CI reports from the diff, and from a CLI transcript recorded in the same checkout if you pass one. See [GitHub Action](github-action.md). |
| `saidran pr` | Previews or updates the report section in the pull request of the current branch. See [pull requests](pull-requests.md). |
| `saidran install claude` / `uninstall claude` | Adds or removes the local Claude Stop hook. `--user` uses the user settings instead. |
| `saidran install codex` | Prints a notify entry with the absolute path for you to put in the Codex config. |
| `saidran hook claude` / `hook codex <json>` | Handles the completion payload of a hook without blocking the agent. See [hooks](hooks.md). |
| `saidran snippet` | Prints the self-check instructions for the agent. |
| `saidran sessions` | Lists the sessions for the current repository. |
| `saidran demo` | Checks a bundled example session, so you need no agent logs of your own. |
| `saidran doctor` | Checks Git, the log folders and the log formats. |
| `saidran dismiss` | Records why a finding is intentional. |
| `saidran dismissals` | Lists or removes the recorded reasons. |

## Choose the comparison

For `check` and a local `ci`, the baseline is the merge base of HEAD with
`origin/HEAD`, or else `origin/main`, or else `origin/master`. Without any of
those, Saidran compares against HEAD, which covers uncommitted changes only.
The report names the baseline and says why it was chosen. `--base <revision>`
compares directly against that revision.

[`saidran pr`](pull-requests.md#choose-the-session-and-comparison) uses the
merge base with the target commit of the pull request instead. The
[GitHub Action](github-action.md) passes the base SHA of the pull request,
which is compared directly with checkout's merge ref.

## Choose the output

| Command | Output flags |
| --- | --- |
| `check` | `--format text\|json\|markdown\|html` picks what goes to stdout; `--json` is the same as `--format json`. `--out <directory>` picks the folder for the saved reports. `--no-write` prints without saving, `--open` opens the saved HTML, and `--dry-run` shows what would be written without creating the report folder. |
| `ci` | Has the same `--format`, `--json`, `--out` and `--dry-run` flags, but no `--no-write` and no `--open`. Its default output folder is `<git dir>/saidran/ci/`. |
| `pr` | `--dry-run` previews without editing the pull request. `--json` returns the section, the URL and the status. It saves no local reports and has no `--out`, `--no-write` or `--open`. See [preview and publish](pull-requests.md#preview-and-publish). |

Without `--out`, `check` saves all three files under
`<git dir>/saidran/<agent>-<full-session-id>/`. With `--no-write` or
`--dry-run` it creates no files. The JSON on stdout does not contain the HTML
path; the [quickstart](quickstart.md#open-the-report) shows how to find it.
The output flags of the other commands are in the
[command reference](reference/index.md).

## Choose an exit policy

A report normally exits 0 even when the evidence needs attention. For `check`
and `ci`, `--fail-on` takes a comma-separated list of verdicts and finding
codes:

```sh
saidran --fail-on stale,contradicted,test_skipped
```

When one of them is present, Saidran writes the report and then exits 10. A
dismissed finding stays visible in the report but does not trigger the policy.
[Findings](findings.md) explains dismissals.

| Exit | Meaning |
| --- | --- |
| 0 | Success without a policy match. Hook handlers always exit 0, also on errors. |
| 1 | Usage or confirmation error. |
| 2 | Runtime failure. |
| 3 | GitHub CLI is missing or not logged in. |
| 5 | No repository, no session or no current pull request. `ci` also exits 5 for a transcript from another checkout. |
| 10 | A `--fail-on` policy matched. |

The [quickstart](quickstart.md) walks through a first check. The
[verdict guide](verdicts.md) shows how claims and checks are matched.
