# Findings in the diff

Saidran is a free, open-source CLI for developers who use coding agents like
Claude Code or Codex. Besides checking the agent's claims against its session
log, it reads the diff of your repository for changes to the tests and checks
themselves. Those are the findings. A skip can have a good reason, so Saidran
shows the change and leaves the judgement to you.

| Code | What changed |
| --- | --- |
| `test_skipped` | A skip or focus marker was added to a test. |
| `test_deleted` | A test file was deleted. |
| `test_command_changed` | The test command of a package changed. The report shows the old and the new command. |
| `check_disabled` | A lint or type-check suppression was added. |
| `ci_ignores_failure` | A CI setting or command now ignores failures. |

Each finding shows the path, the line and a short excerpt. A Go skip, for
example, shows `t.Skipf("later %s", reason)`. Saidran redacts the whole line
first and then cuts the excerpt to at most 240 characters, with an ellipsis
where text was left out. A changed test command shows the old and the new
command. A deleted file has an explanation but no excerpt. Full source files
never go into a report. Read the excerpts before you share a report, because
the [redaction limits](privacy.md) also apply to them.

The baseline decides which diff Saidran reads. It looks for known patterns in
the syntax it supports. It does not parse the whole language, and it does not
judge test quality. The `claude-skip` fixture shows an example. Skip findings
include focus markers like `test.only` and `describe.only`, and Go's `t.Skip`,
`t.Skipf` and `t.SkipNow`. Changed test commands are found in `package.json`,
`composer.json` and `Makefile`, at the root and in subfolders, and the report
shows the full relative path. Only the test entries are compared; other scripts
are ignored. Deleting a test function from a file that still exists is not
detected. `test_deleted` covers deleted test files, and a renamed file does
not count.

If a finding is intentional, record why with
`saidran dismiss <id> --reason "..."`. The reason is stored under the Git
directory of the worktree and shows up in later reports as long as the same
finding is in the diff. `saidran dismissals` lists the reasons, and
`--remove <id>` deletes one. An unknown or ambiguous ID is an error. Only
findings can be dismissed, not verdicts.

`--fail-on test_skipped` makes Saidran exit 10 when an active skip finding is
present. A dismissed finding stays visible but does not trigger that exit.
