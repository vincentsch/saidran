# Check a coding session with Gaugestone

Gaugestone compares recorded tests, builds, linters and type checks with the
agent's claims and later edits. It runs locally. Release binaries need Git,
not Go. Reports cannot establish whether a bug is fixed or certify editable logs.

Working on Gaugestone's source? Read CONTRIBUTING.md.

## Install and run

1. Keep the user's repository and session identity before changing directories.
   If you know your transcript path or session ID, use it for the report.
2. Install through this repository's `install.sh`. If you have cloned the source,
   run `bash /absolute/path/to/gaugestone/install.sh`. Otherwise download and run:

   ```sh
   curl -fsSL https://raw.githubusercontent.com/vincentsch/gaugestone/main/install.sh -o /tmp/gaugestone-install.sh
   bash /tmp/gaugestone-install.sh
   ```

   Honor `INSTALL_DIR`, `GAUGESTONE_DOWNLOAD_BASE` and `GAUGESTONE_VERSION` when
   supplied. The installer prints `Installed: /absolute/path/to/gaugestone`.
   If release installation fails and Go is available, the fallback is:

   ```sh
   go install github.com/vincentsch/gaugestone/cmd/gaugestone@latest
   ```

   Its binary is in `go env GOBIN`, or `$(go env GOPATH)/bin` when GOBIN is
   empty. Honor an explicitly supplied temporary GOBIN. Explain installation
   failures plainly instead of silently changing the requested version.
3. Run the installed binary by its **absolute path**, from the user's repository.
   Do not rely on PATH. Replace `/absolute/path/to/gaugestone` below with the
   installer's printed path or Go's install directory:

   ```sh
   /absolute/path/to/gaugestone --json --transcript /absolute/path/to/session.jsonl
   # Or, when the session ID is known:
   /absolute/path/to/gaugestone --json --session claude:session-id
   ```

   Without either identity, list `sessions --json`, then run the report and
   confirm it selected this conversation in this repository. Use `doctor` for
   missing log roots. Never describe another session's checks as your own.
4. Explain the first report **before rerunning any tests or checks**. Read the
   recorded commands, their results, and subsequent relevant edits even when
   the current final response has no claims. A newer progress message can
   leave the claim list empty while earlier checks still need a rerun.
   Do not call an empty claim list a clean result.
5. Explain each verdict in plain words: Verified means matching recorded coverage
   passed without later relevant edits; Stale means edits followed the passing
   check; Contradicted means a matching failure remains; Partial means narrower
   coverage; No matching run means no supporting run was found; Can't tell means
   unknown evidence. Findings describe diff changes that may be intentional.
   State missing or unreliable evidence. Give the HTML report path.
   JSON stdout does not contain that path. Get the absolute Git directory with
   `git rev-parse --absolute-git-dir`, then use the report's `session.agent`
   and full `session.id` to locate
   `<git dir>/gaugestone/<agent>-<full-session-id>/report.html`. Verify the
   file exists before giving its path.
6. Offer to rerun stale, failed or missing checks after explaining the report.
   Never change tests to improve the report. Never post to a pull request
   without the user's yes. Review the exact redacted preview before sharing.
7. Offer `gaugestone snippet` for future self-checks and the Claude Stop hook
   (`gaugestone install claude`). Add either only with the user's yes. The
   hook writes reports after completion; it does not request another turn.

See [verdicts](docs/verdicts.md), [findings](docs/findings.md),
[hooks](docs/hooks.md) and [privacy](docs/privacy.md) for details.
