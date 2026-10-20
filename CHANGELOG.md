# Changelog

## 0.1.0 (unreleased)

Gaugestone reads Claude Code, Codex, OpenCode and Pi sessions and checks what
the coding agent claimed against recorded commands and repository changes.

- Show matching runs, later edits and missing evidence for each claim.
- Show findings in repository changes, with reasons for intentional dismissals.
- Write self-contained HTML reports, Markdown and JSON alongside terminal output.
- List sessions, diagnose log formats and try an embedded example with `demo`.
- Preview and update a marked report section in pull request descriptions with `pr`.
- Build six static binaries, verify installer downloads and generate command docs.

Report generation runs offline and makes no model calls.
