# What a verdict means

Saidran is a free, open-source CLI for developers who use coding agents like
Claude Code or Codex. It reads the agent's session log and your Git changes
and gives each claim about tests, builds, linters and type checks a verdict.
This page explains the six verdicts and how Saidran matches a claim to a run.

A verdict says whether a recorded check backs up a claim in the agent's final
response. Saidran also looks at the edits the agent made before that response.
It cannot say whether a bug is fixed, and it cannot tell whether someone edited
the session log.

| Verdict | What happened | What to do |
| --- | --- | --- |
| Verified | A matching check covered the whole claim and passed, and no relevant file changed after it. | Nothing for this claim, as far as the recorded session goes. |
| Stale | `go test ./...` passed, then `calc/stats.go` changed. When both happen in the same minute, the explanation shows seconds. | Run the check again. |
| Contradicted | The agent says the tests pass, but a matching test run failed and no later passing run covers that failure. | Fix the failure and run the check again. |
| Partial | The agent says all tests pass after running only `go test ./calc`. For a claim about all tests, a run that only lists tests, finds none, or leaves tests out with `-short` or `-skip` is also Partial. | Run the whole suite. |
| No matching run | The agent says the tests pass, but the log only shows `git status --short`. | Find or record a run that supports the claim. An incomplete log may have lost one. |
| Can't tell | `go test ./...` was piped through `tail`, and the output has no readable test summary. The exit code belongs to the pipe, not to the tests. | Run the check again so the result gets recorded. |

The Verified, Stale and Contradicted rows come from the `codex-verified`,
`claude-stale` and `claude-contradicted` [recorded sessions](../testdata/sessions/README.md).
The other rows are [synthetic examples](../testdata/synthetic/README.md).
`saidran demo` shows the Stale example without any agent history on your
machine.

A passing run in `calc` does not clear a failure in `other`. Edits made while a
check was running prevent Verified. Edits that only touch documentation do not
make a check stale.

If Saidran does not recognize the test runner, or cannot tell what a command
changed, it leaves the result unknown. A missing result also stays unknown.
The JSON report keeps full timestamps and commit IDs.

## When no claims appear

No claims means Saidran found no sentence about checks it recognizes in the
selected final response. Read the recorded commands and the later edits anyway.
The agent may have sent a newer progress message that replaced an earlier
claim while the recorded commands stayed the same. Files that changed after the
session ended are listed separately and are outside the checks.

## How commands and directories match

Each quoted command gets its own claim. In "`go test ./other` passed;
`go test ./calc` passed", and in "`go test ./other` and `go test ./calc`
passed", a run in `calc` cannot verify the tests in `other`. "All tests pass,
but I did not run lint" keeps the test claim and records the lint gap the agent
admitted.

A general claim like "all tests pass" is judged from the directory the session
started in. If the session started at the repository root and the agent ran
`cd calc && go test ./...`, the claim is Partial. If the session started in
`calc`, the same run can verify the claim for `calc`. A quoted `go test ./...`
describes that command in the directory it was recorded in. A bare `go build`
or `go vet` covers the current package only, so it gives Partial when the claim
says all packages were checked.

A quoted TypeScript command has to match the same project configuration.
Relative `-p` and `--project` paths are resolved against the directory of each
run, and against the session directory for the claim. Without `-p` or
`--project`, a claim only matches a run from the same directory that also has
neither flag. A claim that does not quote a command follows the usual coverage
rules. Saidran does not follow configuration inheritance.

## Go results and package coverage

Go's text and JSON output tell Saidran how many packages ran and which had no
tests. A run where every package has no tests is Partial. Go JSON that is
unreadable or incomplete, with no sign that the run finished, is Can't tell
even when the exit code is 0. A later run of one package can clear a failure
Saidran saw in that package earlier, and a claim about the whole suite still
gets Partial.

## Edits made by shell commands

Saidran recognizes writes made by `cp`, `eslint --fix`, `ruff check --fix` and
`git apply`. A known write to a relevant file after a passing check makes the
check Stale. If Saidran cannot tell which files a patch or a fix changed, it
gives Can't tell. Read-only lint and `git apply --check` produce no edit. An
explicit `--apply` turns patch writing back on, even next to `--check` or
`--stat`. A fresh check after the write gives current evidence. Saidran never
runs recorded commands, and it does not read the current contents of a patch
to guess what an earlier run changed.
