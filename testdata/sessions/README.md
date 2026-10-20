# Session fixtures

Twelve real Claude Code (2.1.293), Codex (0.163.0), OpenCode (1.1.4) and
Pi (0.85.1) fixtures recorded on 2026-10-20 against small example repositories.
Each folder has:

- `session.jsonl` (or OpenCode's `storage/` tree): the agent's log, with paths rewritten to
  `/fixtures/<scenario>/repo`, and private instructions, system context and
  Codex account, plan and rate-limit fields (`creator_user_id`,
  `creator_account_id`, `rate_limits`) removed. Everything else is as the
  agent wrote it.
- `repo.tar.gz`: the repository at the end of the session (base commit plus
  the agent's uncommitted changes).
- `meta.json`: agent, version, model, prompt and notes.
- `expected.json`: the verdicts and findings a correct report must contain,
  labelled by hand from the log.

`claude-contradicted` is derived from `claude-honest-failure`: only the final
message was replaced with a passing claim.

Tests extract `repo.tar.gz` into a temp dir (keeping file times) and replace
the `/fixtures/<scenario>/repo` prefix in every path-bearing field with that
dir: session and command cwd, edit tool paths, Codex FileChange keys and
absolute paths inside command text. Claims come from the final assistant
response of the session; earlier progress messages (like "Tests pass. Now
the rename." in `claude-stale`) are context, not claims.


OpenCode's `meta.json.transcript` names the relative
`storage/session/<project>/<session>.json` entry point. Its `message/` and
`part/` sibling directories are required. Fixture helpers read this field and
default to `session.jsonl` for older recordings. Remap paths in every JSON file,
including nested output and diff keys, when copying the storage tree.

The OpenCode and Pi verified recordings start at the archived Claude verified
repository's `Base commit`; stale recordings start at the corresponding stale
base, using the same prompts. Their model is OpenRouter
`deepseek/deepseek-v4-flash-0731`, low effort, with environment credentials,
no fallback routing and no training. They ran sequentially, once per scenario.
Pi disabled both recovery and provider retries. OpenCode 1.1.4 has an unbounded
native transient retry loop with no supported disable switch; these two
supervised recordings explicitly accept that exception, with no launcher reruns.
OpenCode stores the command-line prompt as quoted text with a trailing newline;
the reader preserves that recorded text. No child sessions occurred. Configuration, observed retries and per-call reported
USD costs are summarized in the repository's operational QA records. OpenCode
reports zero local cost for this custom model; that does not confirm zero billing.

All archives use uid/gid 0 and empty owner/group names. The eight original
archives were repacked with ownership metadata changed only; member names,
contents, modes and modification times were checked unchanged. The embedded demo
archive is copied from the sanitized stale recording. Run
`scripts/check-fixtures.sh` to check archive headers, decompressed text, JSON logs
and the embedded demo for prohibited identifiers, account fields and token shapes.
