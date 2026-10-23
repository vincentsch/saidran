# Questions about the evidence

## Does Verified mean the bug is fixed?

Verified means a matching recorded check passed with sufficient coverage and
no later relevant edits. A test may miss the bug or assert the wrong behavior.
Saidran does not judge test quality or confirm that a bug is fixed.
See the [verdict guide](verdicts.md) for coverage rules and examples.

## Why are there no claims?

The selected final response may contain no recognized claim about checks.
A newer progress message can replace an earlier claim
while the recorded commands stay the same. Read command results and later edits
before deciding what needs a rerun. An empty claim list is not a clean result.
A [diff-only CI report](github-action.md#run-the-command-locally) has no session
or claims because no transcript was supplied.

## Why is a result unknown or missing?

An incomplete log may omit a command's result. Saidran may not recognize the
test runner or know which files it covered. Edits during a check can also
prevent Verified, as can a pipe with no reliable test summary.
“Can't tell” means the evidence is uncertain. “No matching run”
means no supporting run was found in that log; it does not prove none happened.
Check the selected session using the [quickstart](quickstart.md#check-your-repository),
then read the [verdicts](verdicts.md) and [agent format limits](agents.md).

## Can a report certify the logs?

Local logs can be edited. Saidran compares the commands they record with
claims and Git changes. It cannot tell whether someone changed the log. Findings describe
patterns in the diff and may have intentional reasons. See [findings](findings.md)
and [privacy](privacy.md) before sharing evidence.

## Does Saidran send my code or session to a service?

Local report generation makes no network or model calls. Reports contain
recorded commands and their results. They keep the end of each command's output
and shortened prompts. They never contain full transcripts.
Installation downloads release files. [PR publication](pull-requests.md) uses
GitHub CLI and requires confirmation after a preview; the [GitHub Action](github-action.md)
can publish a report comment. Redaction can miss sensitive text and unusual
credentials. Review saved reports and the exact PR preview before sharing.
The [privacy guide](privacy.md) describes what reports contain and where they live.
