#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
python3 - <<'PY'
import json, pathlib, re, tarfile

prohibited = re.compile(rb'vincent|thinkpad|schmalbach|creator_user_id|creator_account_id|rate_limits', re.I)
secrets = re.compile(rb'(?:sk-or-v1-|sk-proj-|ghp_)[a-zA-Z0-9_-]{20,}')
failures = []
def check(label, data):
    if prohibited.search(data) or secrets.search(data):
        failures.append(label + ': prohibited identifier, account field or token')

def archive(path):
    with tarfile.open(path, 'r:gz') as tar:
        for h in tar:
            check(str(path) + ': member header', (h.name + h.uname + h.gname + h.linkname + json.dumps(h.pax_headers)).encode())
            name = pathlib.PurePosixPath(h.name)
            if name.is_absolute() or '..' in name.parts:
                failures.append(str(path) + ': unsafe member name')
            if h.uid != 0 or h.gid != 0 or h.uname or h.gname:
                failures.append(str(path) + ': nonnumeric or nonzero ownership')
            if h.isfile():
                data = tar.extractfile(h).read()
                try:
                    data.decode('utf-8')
                except UnicodeDecodeError:
                    continue
                check(str(path) + ': member content ' + h.name, data)

for root in [pathlib.Path('testdata/sessions'), pathlib.Path('internal/demo/data')]:
    for path in sorted(root.rglob('*')):
        if not path.is_file():
            continue
        if path.name.endswith('.tar.gz'):
            archive(path)
        elif path.suffix in {".json", ".jsonl"} or root.name == "data":
            check(str(path), path.read_bytes())
if failures:
    raise SystemExit('\n'.join(failures))
print('Fixture headers, decompressed text, session storage and embedded demo are sanitized.')
PY
