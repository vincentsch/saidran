# What stays local

Saidran is a free, open-source CLI for developers who use coding agents like
Claude Code or Codex. It reads the agent's session log and your Git state on
your machine and checks what the agent claimed about tests, builds, linters
and type checks. This page says what a report contains, where it is saved and
what leaves your machine.

You need no account. Making a report uses no network and no model. Installing
downloads the release files, and `saidran pr` uses GitHub CLI to read and
update a pull request.

A report shows the recorded checks and the changed paths. It contains the
agent's claims and shortened prompts. Of each command's output it keeps the
last 40 lines. Findings come with short excerpts from the diff, and Saidran
redacts an excerpt before it shortens it. A report never contains the full
transcript or a complete source file. In every format, Saidran redacts common
token shapes, secret assignments and home directory paths. The JSON keeps full
timestamps and commit IDs.

Redaction is not a secret scanner. An unusual credential or sensitive project
text can get through. Read the preview before you post to GitHub, and read a
saved report before you share it. `saidran pr` asks for confirmation before it
publishes; `--confirm` skips that question.

Reports and dismissed findings are saved under `<git dir>/saidran/`, and each
worktree has its own folder. The Claude project hook lives in a local settings
file that Git ignores, with its backups under the Git directory. The backup of
the user settings sits next to the user settings file. Delete report folders
and old backups when you no longer need them.

The logs are local files, and anyone can edit them. A report summarizes what
they record. It cannot tell whether someone changed a log, and it cannot
confirm that a bug is fixed. Unknown or missing evidence stays unknown.
