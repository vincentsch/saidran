# Commands

Gaugestone checks recorded agent sessions against commands and repository changes.
The [command reference](reference/index.md) lists the flags and examples generated
from the current CLI.

| Command | What it does |
| --- | --- |
| `gaugestone check` | Checks a session and writes text, HTML, Markdown and JSON reports. This is the default command. |
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

See the [README](../README.md) for session selection, verdicts and exit codes.
