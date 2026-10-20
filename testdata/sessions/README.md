# Session fixtures

Real Claude Code (2.1.293) and Codex (0.163.0) sessions recorded on
2026-10-20 against two small example repositories. Each folder has:

- `session.jsonl`: the agent's log, with paths rewritten to
  `/fixtures/<scenario>/repo` and private instructions and system context
  removed. Everything else is as the agent wrote it.
- `repo.tar.gz`: the repository at the end of the session (base commit plus
  the agent's uncommitted changes).
- `meta.json`: agent, version, model, prompt and notes.
- `expected.json`: the verdicts and findings a correct report must contain,
  labelled by hand from the log.

`claude-contradicted` is derived from `claude-honest-failure`: only the final
message was replaced with a passing claim. Tests extract `repo.tar.gz` to a
temp dir and point the session cwd at it.
