# Commands

Gaugestone checks recorded agent sessions against commands and repository changes.
The [command reference](reference/index.md) lists the flags and examples generated
from the current CLI.

| Command | What it does |
| --- | --- |
| `gaugestone check` | Prints a terminal summary and saves HTML, Markdown and JSON reports. This is the default command. |
| `gaugestone ci` | Writes CI reports from the diff and an optional checkout-local CLI transcript. See [GitHub Action](github-action.md). |
| `gaugestone pr` | Previews or updates a report section in the current pull request. See [pull requests](pull-requests.md). |
| `gaugestone install claude` / `uninstall claude` | Add or remove the local Claude Stop hook; `--user` selects user settings. |
| `gaugestone install codex` | Print an absolute-path notify entry for manual configuration. |
| `gaugestone hook claude` / `hook codex <json>` | Handle completion payloads without blocking. See [hooks](hooks.md). |
| `gaugestone snippet` | Print agent self-check instructions. |
| `gaugestone sessions` | Lists sessions for the current repository. |
| `gaugestone demo` | Checks an embedded example without needing your own agent logs. |
| `gaugestone doctor` | Checks git, session folders and supported log formats. |
| `gaugestone dismiss` | Records a reason for an intentional finding. |
| `gaugestone dismissals` | Lists or removes recorded dismissals. |

## Choose the comparison

For `check` and local `ci`, the default baseline is the merge base of HEAD
with `origin/HEAD`, then `origin/main`, then `origin/master`, in that order.
Without those refs, Gaugestone falls back to HEAD for uncommitted changes only.
The report states the baseline and why it was chosen. `--base <revision>`
compares directly against the supplied revision.

The [PR command](pull-requests.md#choose-the-session-and-comparison) instead
uses the merge base with the PR target commit. The [GitHub Action](github-action.md)
supplies the PR base SHA for a direct comparison with checkout's merge ref.

## Choose the output

| Command | Output flags and behavior |
| --- | --- |
| `check` | `--format text\|json\|markdown\|html` chooses stdout; `--json` aliases `--format json`. `--out <directory>` selects the saved-report directory. `--no-write` prints only; `--open` opens saved HTML. `--dry-run` previews writes without creating report paths. |
| `ci` | Supports the same `--format`, `--json`, `--out` and `--dry-run` flags. It has neither `--no-write` nor `--open`. Its default output directory is `<git dir>/gaugestone/ci/`. |
| `pr` | `--dry-run` previews without editing. `--json` returns the section, URL and status. It saves no local reports and has no `--out`, `--no-write` or `--open`. See [PR output and confirmation](pull-requests.md#preview-and-publish). |

Without `--out`, `check` saves all three report files under
`<git dir>/gaugestone/<agent>-<full-session-id>/`. `--no-write` and `--dry-run`
create no local report files. JSON stdout does not include the HTML path.
See the [quickstart](quickstart.md#open-the-report) for locating it.
Other commands' supported output flags appear in the [command reference](reference/index.md).

## Choose an exit policy

Reports normally exit 0 even when evidence needs attention. For `check` and
`ci`, `--fail-on` takes comma-separated verdicts or active finding codes:

```sh
gaugestone --fail-on stale,contradicted,test_skipped
```

A match requests exit 10 after reporting. Dismissed findings stay visible but
do not trigger the policy. See [findings](findings.md) for local dismissals.

| Exit | Meaning |
| --- | --- |
| 0 | Success without a policy match. Hook handlers always exit 0, including on errors. |
| 1 | Usage or confirmation error. |
| 2 | Runtime failure. |
| 3 | GitHub CLI is missing or unauthenticated. |
| 5 | No repository, session or current PR; also a foreign session rejected by `ci`. |
| 10 | A requested `--fail-on` policy matched. |

Start with the [quickstart](quickstart.md), read the [verdict guide](verdicts.md),
or use the [exit-code table](#choose-an-exit-policy) when calling from a script.
