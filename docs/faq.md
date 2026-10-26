# Questions about the evidence

Saidran is a free, open-source CLI for developers who use coding agents like
Claude Code or Codex. It checks what the agent claimed about tests, builds,
linters and type checks against the session log and your Git changes. These
are the questions that come up when people read their first reports.

## Does Verified mean the bug is fixed?

No. Verified means a matching check passed with enough coverage, and no
relevant file changed after it. The test can still miss the bug or assert the
wrong behavior. Saidran does not judge test quality, and it does not confirm
that a bug is fixed. The [verdict guide](verdicts.md) explains coverage with
examples.

## Why are there no claims?

Saidran found no sentence about checks that it recognizes in the selected
final response. A common reason is that the agent sent a newer progress message
that replaced an earlier claim, while the recorded commands stayed the same.
Read the command results and the later edits before you decide what to rerun.
An empty claim list is not a clean result. A
[diff-only CI report](github-action.md#run-the-command-locally) has no session
and no claims because it was made without a transcript.

## Why is a result unknown or missing?

An incomplete log can lose a command's result. Saidran may not recognize the
test runner, or it may not know which files the run covered. Edits during a
check also prevent Verified, and so does a pipe that hides the test summary.
"Can't tell" means the evidence is there but unreliable. "No matching run"
means no supporting run was found in that log, which does not prove there was
none. Check that you selected the right session with the
[quickstart](quickstart.md#check-your-repository), then read the
[verdicts](verdicts.md) and the [limits of each log format](agents.md).

## Can a report certify the logs?

No. The logs are local files, and anyone can edit them. Saidran compares the
commands they record with the claims and your Git changes. It cannot tell
whether someone changed the log. Findings describe patterns in the diff, and a
pattern can have a good reason. See [findings](findings.md) and
[privacy](privacy.md) before you share evidence.

## Does Saidran send my code or session to a service?

No. Making a report uses no network and no model. A report contains the
recorded commands with the end of each command's output, plus shortened
prompts. It never contains the full transcript. Installing downloads the
release files. [Posting to a pull request](pull-requests.md) goes through
GitHub CLI and asks you to confirm after a preview, and the
[GitHub Action](github-action.md) can post a report comment. Redaction can miss
unusual credentials and sensitive project text, so read a saved report and the
PR preview before you share them. The [privacy guide](privacy.md) lists what
reports contain and where they are saved.
