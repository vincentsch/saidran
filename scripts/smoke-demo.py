#!/usr/bin/env python3
"""Check the native executable and bundled demo on the current host."""
import os
from pathlib import Path
import subprocess
import tempfile

work = Path(tempfile.mkdtemp(prefix="saidran-native-smoke-"))
binary = work / "saidran.exe"
subprocess.run(["go", "build", "-o", str(binary), "./cmd/saidran"], check=True)
subprocess.run([str(binary), "--version"], check=True)
env = dict(os.environ, TMPDIR=str(work), TMP=str(work), TEMP=str(work))
result = subprocess.run([str(binary), "demo"], env=env, text=True, capture_output=True)
print(result.stdout)
print(result.stderr)
assert result.returncode == 0, result.returncode
assert "Stale" in result.stdout, "Demo must explain the later edit"
reports = list(work.glob("saidran-demo-*/repo/.git/saidran/*/report.json"))
assert len(reports) == 1, reports
for name in ["report.json", "report.html", "report.md"]:
    assert (reports[0].parent / name).stat().st_size > 0, name
print("Native demo and all three report files verified")
