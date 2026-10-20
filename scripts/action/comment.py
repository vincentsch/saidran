"""Publish only the completed report, using structured gh API payloads."""
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile


def api(*args):
    return subprocess.run(["gh", "api", *args], capture_output=True, text=True)


def require(result):
    if result.returncode:
        # Do not print API responses that might echo untrusted report content.
        sys.exit("GitHub comment lookup failed; gh api returned an error.")
    return json.loads(result.stdout)


def main():
    event = json.loads(Path(os.environ["GITHUB_EVENT_PATH"]).read_text())
    if "pull_request" not in event:
        return
    repo = os.environ["GITHUB_REPOSITORY"]
    number = event["pull_request"]["number"]
    pages = require(api(f"repos/{repo}/issues/{number}/comments", "--paginate", "--slurp"))
    marker = "<!-- gaugestone -->"
    existing = next((comment for page in pages for comment in page
                     if comment.get("user", {}).get("login") == "github-actions[bot]"
                     and comment.get("user", {}).get("type") == "Bot"
                     and comment.get("body", "").startswith(marker + "\n")), None)
    body = marker + "\n" + Path(os.environ["REPORT_DIR"], "report.md").read_text()
    endpoint = (f"repos/{repo}/issues/comments/{existing['id']}" if existing
                else f"repos/{repo}/issues/{number}/comments")
    # JSON files keep report content out of shell source and command arguments.
    with tempfile.TemporaryDirectory(prefix="gaugestone-comment-") as temp:
        payload = Path(temp, "body.json")
        payload.write_text(json.dumps({"body": body}))
        result = api(endpoint, "--method", "PATCH" if existing else "POST", "--input", str(payload))
    if result.returncode:
        if "HTTP 403" in result.stderr:
            print("::warning::GitHub denied the PR comment write (403). The job summary and HTML artifact remain available.")
            return
        sys.exit("GitHub comment publication failed; the summary and HTML artifact remain available.")


if __name__ == "__main__":
    main()
