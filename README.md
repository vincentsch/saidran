<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="docs/media/wordmark-dark.svg">
    <img src="docs/media/wordmark-light.svg" alt="Saidran" width="238">
  </picture>
</p>

**See what your coding agent checked, and what changed afterwards.**

Saidran reads your coding agent's local session log and Git changes. It
compares claims about tests and builds with recorded runs. It also checks
claims about linters and type checks. If code changed after a passing test,
the report shows it. It flags skipped tests and disabled checks in the diff.

[Website](https://www.saidran.com) · [Quickstart](docs/quickstart.md) · [FAQ](docs/faq.md) ·
[Commands](docs/commands.md) · [AGENTS.md](AGENTS.md) ·
[Releases](https://github.com/vincentsch/saidran/releases)

## Ask your agent to check its work

Copy this into the agent that worked on your repository:

```text
Check what you just did with Saidran.
Use https://github.com/vincentsch/saidran: read its AGENTS.md and
follow the steps, then explain the report to me.
```

The agent installs Saidran, checks its session and explains the evidence.
It shows why checks need a rerun before offering to run them again.

## Install and try the demo

Linux and macOS, on amd64 or arm64:

```sh
curl -fsSL https://raw.githubusercontent.com/vincentsch/saidran/main/install.sh -o /tmp/saidran-install.sh
bash /tmp/saidran-install.sh
```

The installer verifies SHA-256 checksums and prints the absolute binary path.
Release binaries need Git. If the install folder is not on PATH, use the installer's printed absolute path for the commands below.
See the [quickstart](docs/quickstart.md#install) for Windows, Go installation
and installer overrides.

```sh
saidran demo
```

The bundled Claude Code recording has a passing test followed by an edit.
It produces a Stale verdict and leaves an example repository in a temporary
folder. You can remove that folder afterwards.

[![The demo's Stale claim, with the passing test and later edit](docs/media/demo-claim.png)](docs/media/demo-html.png)

[Full HTML report](docs/media/demo-html.png) · [Terminal output](docs/media/demo-terminal.png)

## Check your repository

Run inside the repository your agent changed. The repository needs at least one
commit; in a new project, review and stage your files and make the first commit
before checking changes:

```sh
saidran
```

Confirm the report names the session you intended to check. Saidran shows the
recorded checks and later changes. It saves HTML, Markdown and JSON reports.
The terminal prints the HTML path. See the [first-check instructions](docs/quickstart.md#check-your-repository)
for session selection, missing logs and opening the saved report.

## Read the verdicts

| Verdict | What the evidence says |
| --- | --- |
| Verified | A matching check covered the claim and passed. No relevant files changed afterwards. |
| Stale | A matching check passed, then relevant files changed. Run it again. |
| Contradicted | A matching check failed, and no later passing run covers that failure. |
| Partial | The run succeeded but covered less than the claim. It may have only listed tests or found none. |
| No matching run | No supporting run was found in the selected log. |
| Can't tell | There isn't enough reliable evidence to judge the check. |

A report can't confirm that a bug is fixed or authenticate local logs.
An empty claim list still has commands and edits
worth reading. Files changed after the session ended are marked separately.
See [verdict examples](docs/verdicts.md) and the [FAQ](docs/faq.md).

## Supported agents

Saidran reads Claude Code, Codex, OpenCode and Pi recordings. The
[version table and log guide](docs/agents.md) list tested versions and limits.
It never runs the agents or executes commands from their logs.

## Use it with your agent or in CI

The [self-check snippet and hooks](docs/hooks.md) let an agent inspect evidence
before finishing or write a report afterwards. Hooks never request another turn.

The [pull request guide](docs/pull-requests.md) explains previews and publication
through GitHub CLI. Review the exact redacted preview before posting.
The [GitHub Action](docs/github-action.md) saves a summary and HTML artifact;
its diff-only mode needs no agent credentials. Local `saidran ci` also requires
a repository with a first commit.

[Findings](docs/findings.md) explains skip markers, deleted test files and disabled
checks. [Commands](docs/commands.md) covers dismissals, baselines and exit policies.

## Privacy and contributing

Local reports make no network or model calls and contain no full transcripts.
Common tokens, secret assignments and home paths are redacted. Redaction is not
a secret scanner; review reports before sharing. Installation downloads release
files; PR publication uses GitHub. Read the [privacy guide](docs/privacy.md).

The [command reference](docs/reference/index.md) lists all flags and examples.
See [CONTRIBUTING.md](CONTRIBUTING.md) for builds and checks.
Saidran is available under the [MIT license](LICENSE).
