# What a verdict means

Verdicts compare a claim in the final response with recorded execution and
edits before that response. They cover tests, builds, linters and type checks.
They cannot say whether a bug was fixed. Session logs can be edited.

| Verdict | Example and next step |
| --- | --- |
| Verified | The `codex-verified` recording has passing checks after its last relevant edit. No rerun is needed for that claim within the recorded session. |
| Stale | In `claude-stale`, `go test ./...` passed, then `calc/stats.go` changed in the same minute. The explanation shows seconds. Run the check again. |
| Contradicted | `claude-contradicted` claims passing tests despite a matching failure. Fix the failure and rerun. |
| Partial | The synthetic narrow-coverage example claims all tests after `go test ./calc`. Run the wider suite. Listing tests or finding none also gives Partial. |
| No matching run | The synthetic missing-run example claims passing tests after only `git status --short`. Find or record a supporting run. An incomplete log may omit one. |
| Can't tell | The synthetic unreliable-result example pipes `go test ./...` through `tail` without a readable summary. The pipe's exit does not prove the test result. Rerun with reliable evidence. |

These examples live in [recorded fixtures](../testdata/sessions/README.md) and
[synthetic examples](../testdata/synthetic/README.md). `saidran demo` shows
the recorded Stale example without needing agent history.

A targeted passing rerun cannot erase an unresolved failure outside its coverage.
Unknown runners, uncertain edits and missing results stay unknown. Edits during
a check prevent Verified. Documentation-only edits do not make checks stale.
The report preserves complete timestamps and commit identities in JSON.

An empty claim list means the selected final response contained no recognized
claims. Inspect the command results and later edits anyway. New progress text
can replace an earlier claim without changing the execution history. Changes
after the recorded session end are marked separately and are not covered.
