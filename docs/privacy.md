# What stays local

Report generation reads local logs and Git state. It makes no network calls,
uses no models and needs no account. Installation downloads release assets;
`gaugestone pr` uses GitHub CLI to read or update a pull request.

Reports contain command evidence and output tails, changed paths, claims and
shortened prompts. They never include full transcripts. Output tails keep the
last 40 lines per command. Common token shapes, secret assignments and home
directory paths pass through redaction in every format. JSON retains complete
timestamps and commit identities.

Redaction is not a secret scanner. Unusual credentials and sensitive project
text can remain. Review the exact preview before posting it to GitHub and
review saved reports before sharing. PR publication asks for confirmation;
`--confirm` authorizes publication without that question.

Reports and dismissal reasons live under `<git dir>/gaugestone/`, separately
for each worktree. Claude project hooks use ignored local settings; their
backups stay under the Git directory. User-scope settings backups stay beside
the user settings file. Delete report folders and obsolete backups locally
when you no longer need them.

Local logs can be edited. A report summarizes recorded tests, builds, linters
and type checks; it cannot certify logs or establish whether a bug is fixed.
Unknown or missing evidence stays unknown.
