# Security

Please report vulnerabilities privately through GitHub private vulnerability
reporting when enabled, or email `vschmalbach@vschmalbach.com`. Wait for a fix
before opening a public issue with details.

Saidran reads local agent transcripts and git state. Report generation runs
offline and makes no model calls. Reports can contain prompts, command output
and source paths. Redaction reduces accidental exposure but cannot identify all
sensitive text. Read reports before sharing them. Do not include credentials or
unredacted private transcripts in bug reports or fixtures.

The installer downloads release assets and verifies SHA-256 checksums. GitHub
release publication uses the network. Saidran has no self-update command.

Saidran is pre-1.0. Security fixes may ship in patch releases.
