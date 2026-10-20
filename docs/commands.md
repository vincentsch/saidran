# Commands

Gaugestone checks recorded agent sessions against commands and repository changes.
The [command reference](reference/index.md) lists the flags and examples generated
from the current CLI.

| Command | What it does |
| --- | --- |
| `gaugestone check` | Checks a session and writes text, HTML, Markdown and JSON reports. This is the default command. |
| `gaugestone sessions` | Lists sessions for the current repository. |
| `gaugestone demo` | Checks an embedded example without needing your own agent logs. |
| `gaugestone doctor` | Checks git, session folders and supported log formats. |
| `gaugestone dismiss` | Records a reason for an intentional finding. |
| `gaugestone dismissals` | Lists or removes recorded dismissals. |

See the [README](../README.md) for session selection, verdicts and exit codes.
