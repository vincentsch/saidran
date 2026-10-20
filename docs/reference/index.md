# gaugestone command reference

See what your coding agent checked

## Commands

- [gaugestone check](gaugestone_check.md) - Show session evidence and repository changes
- [gaugestone ci](gaugestone_ci.md) - Write CI reports from a diff and optional checkout-local log
- [gaugestone demo](gaugestone_demo.md) - Try a recorded session without agent history
- [gaugestone dismiss](gaugestone_dismiss.md) - Mark a finding as intentional
- [gaugestone dismissals](gaugestone_dismissals.md) - List or remove intentional findings
- [gaugestone doctor](gaugestone_doctor.md) - Check Git and agent log availability
- [gaugestone hook](gaugestone_hook.md) - Write a report for an agent completion event
- [gaugestone install](gaugestone_install.md) - Install a Claude hook or print Codex notify configuration
- [gaugestone pr](gaugestone_pr.md) - Preview and update a report section in this branch's pull request
- [gaugestone sessions](gaugestone_sessions.md) - List agent sessions for this repository
- [gaugestone snippet](gaugestone_snippet.md) - Print agent self-check instructions
- [gaugestone uninstall](gaugestone_uninstall.md) - Remove the Gaugestone Claude hook

## Global flags

- `--config` Path to the config file
- `--dry-run` Preview changes without performing them
- `--json` Output stable JSON instead of the human view
- `--no-prompt` Never block on an interactive prompt
- `--quiet` Suppress non-essential output
