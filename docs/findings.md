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

The report shows the path, line and evidence. The baseline determines which
diff is inspected. These are pattern checks for supported syntax, not a full
language analysis or a judgment about test quality. See the fixtures for
examples such as `claude-skip`. Skip findings include focus markers like
`test.only` and `describe.only`. Deleting a test function from a retained file
is not detected; `test_deleted` covers deleted test files and excludes renames.

Record an intentional reason with `saidran dismiss <id> --reason "..."`.
The reason is stored under the worktree's Git directory and appears in later
reports while the same finding context matches. List reasons with
`saidran dismissals`, and remove one with `--remove <id>`. Unknown or
ambiguous IDs fail. Claim verdicts cannot be dismissed.

Use `--fail-on test_skipped` to request exit 10 for an active skip finding.
Dismissed findings stay visible but do not trigger that exit policy.
