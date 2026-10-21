# Questions about the evidence

## Does Verified mean the bug is fixed?

Verified means a matching recorded check passed with sufficient coverage and
no later relevant edits. A test may miss the bug or assert the wrong behavior.
Gaugestone does not inspect test quality or establish bug correctness.
See the [verdict guide](verdicts.md) for coverage rules and examples.

## Why are there no claims?

The selected final response may contain no recognized claim about tests,
builds, linters or type checks. New progress text can replace an earlier claim
without changing the recorded commands. Read command results and later edits
before deciding what needs a rerun. An empty claim list is not a clean result.
A [diff-only CI report](github-action.md#run-the-command-locally) has no session
or claims because no transcript was supplied.

## Why is a result unknown or missing?

An incomplete log may omit an exit result. An unknown runner or scope, a pipe
with no reliable test summary, or edits during a check can prevent a verdict
of Verified. “Can't tell” means the evidence is uncertain. “No matching run”
means no supporting run was found in that log; it does not prove none happened.
Check the selected session using the [quickstart](quickstart.md#check-your-repository),
then read the [verdicts](verdicts.md) and [agent format limits](agents.md).

## Can a report certify the logs?

Local logs can be edited. Gaugestone compares their recorded execution with
claims and Git changes, but cannot authenticate that history. Findings describe
patterns in the diff and may have intentional reasons. See [findings](findings.md)
and [privacy](privacy.md) before sharing evidence.

## Does Gaugestone send my code or session to a service?

Local report generation makes no network or model calls. Reports contain
command evidence, output tails and shortened prompts, never full transcripts.
Installation downloads release files. [PR publication](pull-requests.md) uses
GitHub CLI and requires confirmation after a preview; the [GitHub Action](github-action.md)
can publish a report comment. Redaction can miss sensitive text and unusual
credentials. Review saved reports and the exact PR preview before sharing.
The [privacy guide](privacy.md) describes what reports contain and where they live.
