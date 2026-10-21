# Reports in GitHub Actions

The Saidran composite Action runs on Linux and macOS hosted runners. It installs
a checksum-verified release, writes HTML, Markdown and JSON locally, appends the
Markdown to the job summary, uploads the HTML artifact, then creates or updates one
`github-actions[bot]` PR comment marked `<!-- saidran -->`. The Action never
invokes an agent or model. Direct Windows binary use remains supported; the Unix
installer and this Action do not support Windows runners.

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

Choose a published release, or pin the Saidran reference to its reviewed commit
SHA. The binary version is embedded in that checkout's `action.yml`: this version
installs **v0.1.0** from `download/v0.1.0/`, including its checksums. A SHA pin or
local `uses: ./` uses that same embedded version; neither selects the latest
release or builds the checkout's source. An unavailable pinned release fails
installation without falling back. Development checkouts therefore need the
embedded release to be available, or a matching `SAIDRAN_DOWNLOAD_BASE` mirror.
The Action's upload dependency is pinned by SHA.
No agent or model credentials are needed for this example.

The default `base` is the event's PR base SHA. Keep checkout's default merge-ref
checkout and `fetch-depth: 0`: `--base` compares directly against that commit,
without inferring a merge base. If you check out the head branch instead, choose
the intended comparison commit explicitly. Saidran never fetches missing bases.
For non-PR events supply an explicit locally available `base`; the Action skips
comment calls and still saves the summary and artifact.

Inputs are `transcript` (optional CLI log), `base` (optional on PRs) and `fail-on`
(optional comma-separated policy). A policy match returns exit 10 only after
reports are written. The Action applies that status after summary, artifact and
comment handling. A report error stops publication and cannot reuse old reports:
every run gets a fresh output directory. Unexpected comment errors fail the Action
after summary and artifact preservation. A forbidden comment write (HTTP 403)
produces a warning; it does not erase a report or override a policy failure.

The token needs `pull-requests: write` to publish the comment. Standard
`pull_request` fork runs normally have read-only tokens; the report remains in the
summary and artifact when GitHub forbids the comment. Do not use
`pull_request_target` to execute fork code with a write token. Organization policy
can further restrict permissions. Hosted Unix runners provide Bash, Python 3,
Git, curl and GitHub CLI, which the Action uses.

## Include a log from the same checkout

A transcript must come from a supported agent CLI run earlier in the same job,
inside this checkout. This optional example replaces the quickstart's steps.
The caller supplies Claude CLI installation, its authentication and any permission
setup needed for its task. `ANTHROPIC_API_KEY` below is a caller-managed repository
secret; it is not needed by Saidran. Pick the model and effort explicitly.

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

Saidran accepts Claude Code, Codex, OpenCode and Pi CLI recordings, not arbitrary
agent output. The Claude Code GitHub Action's `execution_file` is a JSON array of
SDK messages and is unsupported in this release; it is not the CLI JSONL log.
Do not upload or pass a developer-machine transcript: recorded working directories
must belong to this checkout, with existing symlinks resolved. A foreign transcript
exits 5; broken or unsupported recordings fail instead of becoming diff-only reports.
OpenCode takes `storage/session/<project>/<id>.json` and reads its sibling
`storage/message/<session>/*.json` and `storage/part/<message>/*.json` trees.
Preserve that complete layout when passing the session entry point.

## Run the command locally

```sh
saidran ci
saidran ci --base HEAD --out reports --fail-on test_skipped
saidran ci --transcript /path/to/checkout-local-session.jsonl --json
```

Without `--out`, both modes replace `report.html`, `report.md` and `report.json`
under this worktree's absolute Git directory at `saidran/ci/`. Explicit `--out`
writes directly into that directory. `--dry-run` creates no output paths. Shared
`--json` and `--format text|json|markdown|html` conventions apply.
Without a transcript no session discovery happens: JSON has `session: null` and
empty claims. Human views say “Diff-only report” and explain the missing log.
Changed files, findings, dismissals and evidence limits remain visible. No checks
are verified by a diff-only report. Reports summarize editable local evidence;
they do not certify correctness. Existing `--fail-on` values and dismissal rules
apply. Usage errors exit 1, runtime failures 2, missing repository or foreign
session 5, policy matches 10, and successful reports without matches 0.
