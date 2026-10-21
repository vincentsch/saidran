# saidran command reference

See what your coding agent checked

## Commands

- [saidran check](saidran_check.md) - Show session evidence and repository changes
- [saidran ci](saidran_ci.md) - Write CI reports from a diff and optional checkout-local log
- [saidran demo](saidran_demo.md) - Try a recorded session without agent history
- [saidran dismiss](saidran_dismiss.md) - Mark a finding as intentional
- [saidran dismissals](saidran_dismissals.md) - List or remove intentional findings
- [saidran doctor](saidran_doctor.md) - Check Git and agent log availability
- [saidran hook](saidran_hook.md) - Write a report for an agent completion event
- [saidran install](saidran_install.md) - Install a Claude hook or print Codex notify configuration
- [saidran pr](saidran_pr.md) - Preview and update a report section in this branch's pull request
- [saidran sessions](saidran_sessions.md) - List agent sessions for this repository
- [saidran snippet](saidran_snippet.md) - Print agent self-check instructions
- [saidran uninstall](saidran_uninstall.md) - Remove the Saidran Claude hook

## Global flags

- `--config` Path to the config file
- `--dry-run` Preview changes without performing them
- `--json` Output stable JSON instead of the human view
- `--no-prompt` Never block on an interactive prompt
- `--quiet` Suppress non-essential output
