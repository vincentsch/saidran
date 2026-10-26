# Reports in GitHub Actions

Saidran is a free, open-source CLI for developers who use coding agents like
Claude Code or Codex. It checks what the agent claimed about tests, builds,
linters and type checks against the session log and the Git changes. The
Saidran Action runs it in a GitHub Actions job: it writes the report, puts the
Markdown into the job summary, uploads the HTML as an artifact and posts or
updates one pull request comment. This page shows both ways to use it, from
the diff alone and with an agent log.

The Action runs on the hosted Linux and macOS runners. It verifies the
checksum of the release binary and saves the HTML, Markdown and JSON reports in
the job. The comment comes from `github-actions[bot]` and is marked
`<!-- saidran -->`, so a later run updates it instead of adding another one.
The Action never starts an agent and never calls a model. Windows runners are
not supported by the Action or the Unix installer; on Windows you can run the
binary directly.

The Action and a local `saidran ci` both need at least one commit. On a branch
without commits they exit 2 with a hint and write no report.

## Diff-only quickstart

Save this as `.github/workflows/saidran.yml`:

```yaml
name: Saidran report
on:
  pull_request:
permissions:
  contents: read
  pull-requests: write
jobs:
  report:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7
        with:
          fetch-depth: 0
      - uses: vincentsch/saidran@v0.1.0
        with:
          fail-on: test_skipped,check_disabled,ci_ignores_failure
```

Use a published release, or pin the Saidran reference to a commit SHA you have
reviewed. The version of the binary is embedded in the `action.yml` of that
checkout: this version installs **v0.1.0** from `download/v0.1.0/`, together
with its checksums. A SHA pin or a local `uses: ./` installs that same embedded
version. Neither picks the latest release, and neither builds the source of the
checkout. If the pinned release is not available, the install fails and there
is no fallback. A development checkout therefore needs that release to exist,
or a mirror in `SAIDRAN_DOWNLOAD_BASE`. The upload action that the Action uses
is pinned by SHA. This example needs no agent or model credentials.

The default `base` is the base SHA of the pull request event. Keep checkout's
default merge ref and `fetch-depth: 0`. `--base` compares directly against
that commit and does not look for a merge base. If you check out the head
branch instead, set the comparison commit yourself. Saidran never fetches a
missing base. For events other than a pull request, pass a `base` that exists
locally. The Action then skips the comment and still saves the summary and the
artifact.

The inputs are `transcript` (a CLI log, optional), `base` (optional on pull
requests) and `fail-on` (an optional comma-separated policy). A policy match
exits 10 only after the reports are written, and the Action applies that
status after the summary, the artifact and the comment. If the report itself
fails, nothing is published, and old reports are never reused because each run
gets a fresh output folder. An unexpected error while commenting fails the
Action after the summary and the artifact are saved. A forbidden comment
(HTTP 403) only produces a warning. It does not remove the report and does not
override a policy failure.

The token needs `pull-requests: write` for the comment. A normal
`pull_request` run from a fork has a read-only token, and the report then stays
in the summary and the artifact. Do not use `pull_request_target` to run fork
code with a write token. Your organization's policy can restrict the
permissions further. The Action uses Bash, Python 3, Git, curl and GitHub CLI,
which the hosted Unix runners provide.

## Include a log from the same checkout

The transcript has to come from a supported agent CLI that ran earlier in the
same job, inside this checkout. The example below replaces the steps of the
quickstart. Install the Claude CLI and set up its authentication before this
step, and give it the permissions the task needs. `ANTHROPIC_API_KEY` is a
repository secret you manage; Saidran does not need it. Pick the model and the
effort yourself.

```yaml
name: Saidran with a CLI log
on:
  workflow_dispatch:
    inputs:
      base:
        description: Available comparison commit
        required: true
permissions:
  contents: read
jobs:
  report:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7
        with:
          fetch-depth: 0
      # The caller installs and authenticates Claude CLI before this step.
      - name: Run Claude CLI
        id: agent
        shell: bash
        env:
          ANTHROPIC_API_KEY: ${{ secrets.ANTHROPIC_API_KEY }}
          CLAUDE_CONFIG_DIR: ${{ runner.temp }}/claude-config
        run: |
          set -euo pipefail
          mkdir -p "$CLAUDE_CONFIG_DIR"
          claude -p --model claude-sonnet-5-5 --effort medium --output-format json \
            'Review the checkout and run its relevant checks. Do not publish anything.' \
            > "$RUNNER_TEMP/claude-result.json"
          python3 - <<'PY'
          import json, os, pathlib
          result = json.loads(pathlib.Path(os.environ['RUNNER_TEMP'], 'claude-result.json').read_text())
          session_id = result['session_id']
          root = pathlib.Path(os.environ['CLAUDE_CONFIG_DIR'], 'projects')
          matches = list(root.glob('*/' + session_id + '.jsonl'))
          if len(matches) != 1:
              raise SystemExit('Expected exactly one matching CLI JSONL recording')
          with open(os.environ['GITHUB_OUTPUT'], 'a') as output:
              output.write('transcript=' + str(matches[0]) + '\n')
          PY
      - uses: vincentsch/saidran@v0.1.0
        with:
          transcript: ${{ steps.agent.outputs.transcript }}
          base: ${{ inputs.base }}
          fail-on: contradicted,stale
```

Saidran reads Claude Code, Codex, OpenCode and Pi CLI recordings, and nothing
else. The `execution_file` of the Claude Code GitHub Action is a JSON array of
SDK messages, not the CLI's JSONL log, and this release does not support it.
Do not upload or pass a transcript from a developer machine: the recorded
working directories have to belong to this checkout, with symlinks resolved. A
transcript from another checkout exits 5. A broken or unsupported recording
fails the run; it does not fall back to a diff-only report. For OpenCode, pass
`storage/session/<project>/<id>.json`. Saidran reads the matching
`storage/message/<session>/*.json` and `storage/part/<message>/*.json` next to
it, so keep that layout intact.

## Run the command locally

```sh
saidran ci
saidran ci --base HEAD --out reports --fail-on test_skipped
saidran ci --transcript /path/to/checkout-local-session.jsonl --json
```

Without `--out`, both modes replace `report.html`, `report.md` and
`report.json` under `saidran/ci/` in the absolute Git directory of this
worktree. With `--out`, the files go straight into that folder. `--dry-run`
writes nothing. `--json` or `--format text|json|markdown|html` picks what goes
to stdout. Without a transcript, Saidran does not look for sessions: the JSON
has `session: null` and no claims, and the text, Markdown and HTML say
"Diff-only report" and explain that no log was given. Changed files, findings,
dismissals and the evidence limits are still there. A diff-only report
verifies no checks. A report summarizes local evidence that can be edited, and
it does not certify that the code is correct. `--fail-on` values and
dismissals work as usual. The exit codes match the
[command guide](commands.md#choose-an-exit-policy): 0 without a policy match,
and 10 after the reports are saved when a policy matched.
