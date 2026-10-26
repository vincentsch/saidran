# Install Saidran and check a session

Saidran is a free, open-source CLI for developers who use coding agents like
Claude Code or Codex. It reads the agent's session log and your Git changes
and shows whether a recorded test, build, lint or type-check run backs up what
the agent claimed. This page takes you from the install to your first report.

The release binaries need Git. They need no Go or Node, and the agent itself
does not have to be installed, because Saidran only reads the logs it left
behind. Making a report uses no network and no model. Installing downloads the
release files.

## Install

On Linux or macOS, amd64 or arm64:

```sh
curl -fsSL https://raw.githubusercontent.com/vincentsch/saidran/main/install.sh -o /tmp/saidran-install.sh
bash /tmp/saidran-install.sh
```

The installer verifies the SHA-256 checksum and puts the binary in
`~/.local/bin`. It prints `Installed: /absolute/path/to/saidran`. If that
folder is not on your PATH, run Saidran by the printed path.

These environment variables change what the installer does:

| Variable | What it changes |
| --- | --- |
| `INSTALL_DIR` | The folder the binary goes into |
| `SAIDRAN_VERSION` | The release to install |
| `SAIDRAN_DOWNLOAD_BASE` | The mirror to download from |

For example:

```sh
INSTALL_DIR="$HOME/.local/bin" SAIDRAN_VERSION=v0.1.0 bash /tmp/saidran-install.sh
```

### Windows

Download `saidran-windows-amd64.exe` or `saidran-windows-arm64.exe` and
`checksums.txt` from the [releases page](https://github.com/vincentsch/saidran/releases).
Before you run the file, check its hash in PowerShell:

```powershell
Get-FileHash .\saidran-windows-amd64.exe -Algorithm SHA256
Select-String -Path .\checksums.txt -Pattern 'saidran-windows-amd64.exe'
```

On arm64, use the arm64 file name in both commands. The two hashes must match,
letter case aside. If they differ, do not run the file. Then rename the file
to `saidran.exe` and put it in a folder on your PATH, or run it by its absolute
path. Git also has to be on PATH.

### Install with Go

```sh
go install github.com/vincentsch/saidran/cmd/saidran@latest
```

The binary goes into `go env GOBIN`, or `$(go env GOPATH)/bin` when GOBIN is
empty. If that folder is not on your PATH, run Saidran by its absolute path.

## Try the bundled demo

```sh
saidran demo
```

The demo needs no agent history. It unpacks a sanitized Claude Code recording
and its small Go repository into a temporary folder and checks them. In that
session the test passed, then the agent edited the code, so the claim gets the
Stale verdict. The terminal prints the path of the HTML report and the path of
the temporary folder. The folder stays until you delete it. The
[README](../README.md#install-and-try-the-demo) has screenshots of this report.

## Check your repository

Run Saidran in the Git repository your agent worked on. It compares your
changes with the Git history, so the repository needs at least one commit:

```sh
saidran
```

Saidran picks the newest session for this repository that ran commands or made
edits and ended in the last seven days. Check the session ID, the agent name
and the times in the summary against the conversation you meant. Read the
recorded commands and the edits after them even when the report lists no
claims. When two different agents finished less than ten minutes apart, Saidran
asks you which one to read. In a script it takes the newest one and says so.

If the automatic pick is not the session you want, list the sessions or name
one:

```sh
saidran sessions
saidran --session claude:session-id
saidran --transcript /absolute/path/to/session.jsonl
```

A session reference is the full ID or a unique prefix of it. An explicit
transcript is not limited to the last seven days. `--session` and
`--transcript` cannot be combined. The [agent guide](agents.md) lists where
each agent keeps its logs, and the [check reference](reference/saidran_check.md)
lists the selection flags.

If Saidran finds no logs, or does not support their format, run:

```sh
saidran doctor
```

The [doctor reference](reference/saidran_doctor.md) explains what it reports.
Saidran never runs the agent and never executes commands from a log.

## Open the report

Each check saves `report.html`, `report.md` and `report.json` under
`<git dir>/saidran/<agent>-<full-session-id>/`. The Git directory is what
`git rev-parse --absolute-git-dir` prints, and a linked worktree has its own.
The terminal prints the path of the HTML file. Open it in your browser, or run:

```sh
saidran --open
```

If you selected the session with a flag, pass the same flag again. JSON output
on stdout does not include the HTML path. Take `session.agent` and the full
`session.id` from the report, build the path with the pattern above and check
that the file exists. The [output flags](commands.md#choose-the-output) choose
another format or folder, or print a report without saving it.

The [verdict guide](verdicts.md) explains coverage and later edits. The
[FAQ](faq.md) explains missing claims and unknown results. A report summarizes
a log that anyone could have edited, and it cannot tell you that a bug is
fixed.
