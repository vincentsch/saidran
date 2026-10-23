# Findings in the diff

Findings describe changes that deserve a look. A skip can be intentional;
Saidran does not decide whether the change is justified.

| Code | What changed |
| --- | --- |
| `test_skipped` | A recognized skip or focus marker was added. |
| `test_deleted` | A test file was deleted. |
| `test_command_changed` | The package test command changed; old and new values are shown. |
| `check_disabled` | A recognized lint or type-check suppression was added. |
| `ci_ignores_failure` | A recognized CI setting or command began ignoring failures. |

Each report shows a short excerpt beside the finding's path and line. For example, a Go skip finding shows `t.Skipf("later %s", reason)`.
The complete triggering line is redacted before selecting an excerpt of up to
240 characters. An ellipsis marks omitted text. Test-command changes show the
old and new commands; deleted files keep their explanation without an excerpt.
Full source files stay outside reports. Review excerpts before sharing; the
[redaction limits](privacy.md) still apply.

The baseline determines which diff Saidran reads. It looks for known patterns
in supported syntax. It does not parse the whole language or judge test quality.
The `claude-skip` fixture shows an example. Skip findings include focus markers like
`test.only` and `describe.only`, and Go’s `t.Skip`, `t.Skipf` and `t.SkipNow`.
Test-command changes are detected in `package.json`, `composer.json` and
`Makefile` at the root or in subdirectories, with the full relative path shown in the report.
Only their supported test entries are compared; unrelated scripts are ignored.
Deleting a test function from a retained file is not detected; `test_deleted`
covers deleted test files and excludes renames.

If a finding is intentional, record why with `saidran dismiss <id> --reason "..."`.
The reason is stored under the worktree's Git directory and appears in later
reports while the same finding context matches. List reasons with
`saidran dismissals`, and remove one with `--remove <id>`. Unknown or
ambiguous IDs fail. Claim verdicts cannot be dismissed.

Use `--fail-on test_skipped` to request exit 10 for an active skip finding.
Dismissed findings stay visible but do not trigger that exit policy.
