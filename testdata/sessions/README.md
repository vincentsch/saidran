# Session fixtures

Real Claude Code (2.1.293) and Codex (0.163.0) sessions recorded on
2026-10-20 against two small example repositories. Each folder has:

- `session.jsonl`: the agent's log, with paths rewritten to
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
