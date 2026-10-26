# Install Saidran and check a session

Saidran is a free, open-source CLI for developers using coding agents.
It checks test claims against session logs and later code changes.

Release binaries require Git, but no Go or Node runtime. Reading existing logs
does not require the agent to be installed. Reports use no network or model calls.
Installation downloads release files.

## Install

On Linux or macOS, with amd64 or arm64:

```sh
curl -fsSL https://raw.githubusercontent.com/vincentsch/saidran/main/install.sh -o /tmp/saidran-install.sh
bash /tmp/saidran-install.sh
```

The installer verifies SHA-256 checksums and defaults to `~/.local/bin`.
It prints `Installed: /absolute/path/to/saidran`. Use that absolute path if
the install directory is not on PATH.

Installer environment variables:

| Variable | What it changes |
| --- | --- |
| `INSTALL_DIR` | Folder for the binary |
| `SAIDRAN_VERSION` | Release to install |
| `SAIDRAN_DOWNLOAD_BASE` | Release mirror to download from |

For example:

```sh
INSTALL_DIR="$HOME/.local/bin" SAIDRAN_VERSION=v0.1.0 bash /tmp/saidran-install.sh
```

### Windows

Download the matching `saidran-windows-amd64.exe` or
`saidran-windows-arm64.exe` and `checksums.txt` from the same
[release](https://github.com/vincentsch/saidran/releases).
Before running the binary, use PowerShell to compute its checksum:

```powershell
Get-FileHash .\saidran-windows-amd64.exe -Algorithm SHA256
Select-String -Path .\checksums.txt -Pattern 'saidran-windows-amd64.exe'
```

Use the arm64 filename in both commands on arm64. Compare the full hash with
the matching entry, ignoring letter case. Stop if they differ. Rename the
verified binary to `saidran.exe` and put it in a folder on PATH, or invoke
its absolute path. Git must also be available on PATH.

### Install with Go

Install from source with Go:

```sh
go install github.com/vincentsch/saidran/cmd/saidran@latest
```

The binary goes into `go env GOBIN`, or `$(go env GOPATH)/bin` when GOBIN is
empty. Use its absolute path if that directory is not on PATH.

## Try the bundled demo

```sh
saidran demo
```

The demo needs no agent history. It extracts a sanitised Claude Code recording
and repository into a temporary folder. Its test passed before a later edit,
which gives the claim a Stale verdict. The terminal prints the HTML report path
and the temporary folder, which remains until you delete it.
The [README screenshots](../README.md#install-and-try-the-demo) show this example.

## Check your repository

Run in the Git repository your agent changed. Saidran compares against Git
history, so the repository needs at least one commit:

```sh
saidran
```

By default, Saidran picks the latest active session from the last seven days.
Check its session ID and repository. The agent name and times should also match
the conversation you intended. Read its command results and later edits even if no claims appear.
When different agents finished less than ten minutes apart, interactive use
asks which one to read. Scripts pick the newest and explain the choice.

List recordings or select one explicitly when the default is not yours:

```sh
saidran sessions
saidran --session claude:session-id
saidran --transcript /absolute/path/to/session.jsonl
```

Session references accept full IDs or unique prefixes. Explicit transcripts
bypass the seven-day limit.
Do not combine `--session` and `--transcript`. See [supported log locations](agents.md)
and the [selection flags](reference/saidran_check.md).
If logs are missing or unsupported, run:

```sh
saidran doctor
```

The [doctor reference](reference/saidran_doctor.md) explains its diagnostics.
Saidran never runs the agent or executes commands from its log.

## Open the report

A successful check saves reports under
`<git dir>/saidran/<agent>-<full-session-id>/`. The HTML file is `report.html`;
Markdown and JSON versions are saved as `report.md` and `report.json`. Get the absolute Git directory
with `git rev-parse --absolute-git-dir`; linked worktrees have separate reports.
The terminal prints the saved HTML path. Open that file in your browser, or run:

```sh
saidran --open
```

Use the same explicit session flag again if you selected one earlier.
JSON stdout does not include the HTML path. Use the report's `session.agent`
and full `session.id` with the directory pattern above, and verify the file
exists. [Output flags](commands.md#choose-the-output) let you choose another format or
folder. They also support previews without saving files.

[Verdicts](verdicts.md) explain coverage and later edits. The [FAQ](faq.md)
explains missing claims and unknown results. Reports summarize editable logs;
they cannot establish whether a bug is fixed.
