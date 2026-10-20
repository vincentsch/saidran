#!/usr/bin/env bash
set -euo pipefail
# Authentication, update and destructive operations are absent.
project_root="$(cd "$(dirname "$0")/.." && pwd)"
task_temp="$(mktemp -d)"
trap 'rm -rf -- "$task_temp"' EXIT
cd "$project_root"
CGO_ENABLED=0 go build -o "$task_temp/gaugestone" ./cmd/gaugestone
go build -o "$task_temp/rungrad" github.com/vincentsch/rungrad/cmd/rungrad
export HOME="$task_temp/home" CLAUDE_CONFIG_DIR="$task_temp/claude" CODEX_HOME="$task_temp/codex"
mkdir -p "$HOME"
tar -xzf testdata/sessions/claude-verified/repo.tar.gz -C "$task_temp"
python3 - "$project_root" "$task_temp" <<'PY'
import pathlib, re, sys
project, root = map(pathlib.Path, sys.argv[1:])
repo = root / 'repo'
encoded = re.sub(r'[^a-zA-Z0-9]', '-', str(repo))
logs = root / 'claude' / 'projects' / encoded
logs.mkdir(parents=True)
transcript = (project / 'testdata/sessions/claude-verified/session.jsonl').read_text()
remapped = transcript.replace('/fixtures/claude-verified/repo', str(repo))
(logs / 'session.jsonl').write_text(remapped)
(root / 'session.jsonl').write_text(remapped)
PY
cd "$task_temp/repo"
"$task_temp/rungrad" score "$task_temp/gaugestone" --manifest auto --strict --read sessions \
  --mutate "check --transcript $task_temp/session.jsonl" \
  --not-found "check --session absent" \
  --api-error "check --transcript $task_temp/session.jsonl --base missing-revision"
