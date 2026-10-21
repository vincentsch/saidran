#!/usr/bin/env bash
set -euo pipefail
base=$INPUT_BASE
if [ -z "$base" ]; then
    base=$(python3 -c 'import json,os; print(json.load(open(os.environ["GITHUB_EVENT_PATH"])).get("pull_request", {}).get("base", {}).get("sha", ""))')
fi
if [ -z "$base" ]; then
    echo 'Error: non-PR runs require an explicit available base input.' >&2
    exit 1
fi
report_dir=$(mktemp -d "$RUNNER_TEMP/saidran-reports.XXXXXX")
args=(ci --base "$base" --out "$report_dir")
if [ -n "$INPUT_TRANSCRIPT" ]; then args+=(--transcript "$INPUT_TRANSCRIPT"); fi
if [ -n "$INPUT_FAIL_ON" ]; then args+=(--fail-on "$INPUT_FAIL_ON"); fi
status=0
saidran "${args[@]}" || status=$?
printf 'status=%s\nreport_dir=%s\n' "$status" "$report_dir" >> "$GITHUB_OUTPUT"
# Preserve report errors here; policy results are applied after publication.
if [ "$status" != 0 ] && [ "$status" != 10 ]; then exit "$status"; fi
for document in report.html report.md report.json; do test -f "$report_dir/$document"; done
