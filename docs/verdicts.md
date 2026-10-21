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

Independent assertions receive separate claims and quotes. For example,
“`go test ./other` passed; `go test ./calc` passed” does not let the calc run
verify the other package. “All tests pass, but I did not run lint” keeps the
test assertion and a separate admitted lint gap.

Generic suite claims use the session's starting directory. Running
`cd calc && go test ./...` in a root-started session gives Partial for “All
tests pass”; a session starting in calc can verify that claim within calc.
A quoted `go test ./...` assertion describes that command at its recorded
working directory. Bare `go build` and `go vet` cover the current package,
so they give Partial for explicit recursive assertions.

Quoted TypeScript commands must match the same project configuration. Relative
`-p` and `--project` paths are resolved against each run's working directory
and the session directory for claims. Omitting the selector matches only
another omitted selector in the same directory. Generic prose keeps its usual
coverage rules; Saidran does not discover configuration inheritance.

Go text and JSON output retain package counts and no-tests results. A run
where all packages have no tests gives Partial. Unreadable or incomplete Go
JSON without completion evidence gives Can't tell even with a recorded exit 0.
A later passing package rerun can resolve an identified earlier package failure,
while still giving Partial for a broader suite assertion.

Recognized shell writes include `cp` destinations, `eslint --fix`,
`ruff check --fix` and `git apply`. Known relevant writes after a passing
check make it Stale; unresolved patch or fix destinations give Can't tell.
Read-only lint and `git apply --check` create no edit evidence. Explicit
`--apply` re-enables patch writing even alongside `--check` or `--stat`. A fresh check
after the write can provide current evidence. Recorded commands are never run,
and current patch contents are never used to infer historical targets.
