# What stays local

Saidran reads local logs and Git state to make a report. You do not need an
account. Making a report uses no network or model calls. Installation downloads release assets;
`saidran pr` uses GitHub CLI to read or update a pull request.

Reports show recorded checks and changed paths. They include the agent's
claims and shortened prompts. Command output keeps the last 40 lines per command.
Findings include short excerpts from the diff. Saidran redacts the excerpts before shortening them. Reports never include full transcripts or complete source files.
Saidran redacts common token shapes and secret assignments in every format.
It also redacts home directory paths. JSON retains complete
timestamps and commit identities.

Redaction is not a secret scanner. Unusual credentials and sensitive project
text can remain. Review the exact preview before posting it to GitHub and
review saved reports before sharing. PR publication asks for confirmation;
`--confirm` authorizes publication without that question.

Reports and reasons you record for intentional findings are saved under
`<git dir>/saidran/`. Each worktree has its own folder. Claude project hooks use ignored local settings; their
backups stay under the Git directory. User-scope settings backups stay beside
the user settings file. Delete report folders and obsolete backups locally
when you no longer need them.

Local logs can be edited. A report summarizes recorded checks.
It cannot tell whether someone changed a log or confirm that a bug is fixed.
Unknown or missing evidence stays unknown.
