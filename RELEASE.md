# Releasing Saidran

Release binaries include the version and commit from the source being built.
Report generation runs offline. Installation and GitHub publication use the network.

## Before tagging

1. Set the install step's `SAIDRAN_VERSION` in action.yml to the release tag.
   Update the simulation's expected release version and the Action guide's version
   examples together. The Action installs that embedded version even from a SHA
   pin or local checkout; never replace it with an implicit latest download.
   Update CHANGELOG.md with implemented behavior. Change the version heading from
   unreleased to its release date. The release job extracts that section as its notes.
2. Check README.md, docs, SECURITY.md and CONTRIBUTING.md against the current CLI.
   Confirm `check`, `ci`, `pr`, hooks and their shared output/exit conventions.
   Run `make docs` after changing commands. Keep handwritten docs outside docs/reference.
3. Run the local checks with the toolchain from go.mod:

   ```bash
   make lint build test race docs-check conformance
   scripts/check-fixtures.sh
   git diff --check
   make build-all VERSION=v0.1.0
   python3 scripts/release-notes.py v0.1.0
   ```

4. Run the Linux asset's `--version` and `demo` commands. Inspect all six assets and
   verify checksums.txt. Check a local fake release with `scripts/fake-release.sh`
   before testing the installer. Keep temporary release trees outside tracked source.
5. Require native `saidran demo` runs on macOS and Windows before tagging.
   Confirm the report files exist and the example shows Stale. Cross-builds alone
   do not establish native success. Windows CI runs build and vet because the current
   tests assume Unix paths; macOS CI runs the Go tests.
6. Inspect tracked public files for credentials, home paths and private operator
   files. Confirm `.vroni` is absent from `git ls-files`.
7. Require a green first hosted CI run, including all platform jobs, before tagging.
   Local syntax checks do not establish hosted success. Validate the composite
   Action on Linux and macOS with both diff-only and same-checkout CLI evidence,
   merge-ref checkout and an available PR base SHA. Verify summary/artifact
   preservation for policy failures and forbidden fork comment writes. Require
   public latest/pinned installation after publication; local fake releases do
   not establish public availability.

## Publish a version

After the checks above, create an annotated `vX.Y.Z` tag and push it through the
normal review process. The release job repeats CI, builds six binaries, writes
checksums and attests the binaries' build provenance. It publishes the matching
CHANGELOG.md section with the assets. A missing or unreleased section fails the job.

Never move a public tag. Publish a new patch version if a release needs a fix.

## After publication

Check the public release assets, checksums and provenance. Test both latest and
pinned installation in a clean environment without repository access. Run the
installed binary's `--version` and `demo` commands. On Windows, download the matching
`.exe`, verify its checksum and run `demo` natively.

Check `go install github.com/vincentsch/saidran/cmd/saidran@latest`
without private repository access, then run the installed binary's `--version`
and `demo` commands.

Run the README's agent prompt from the public source with each documented agent.
Confirm its install and first report work without private files. Public install
and public agent-prompt checks happen after publication; record those results then.

## Public v0.1.0 verification, 2026-10-23

Published [saidran v0.1.0](https://github.com/vincentsch/saidran/releases/tag/v0.1.0)
from `248f823ec97f1912a60f74febc3532d12f2e6c56`. The [release workflow](https://github.com/vincentsch/saidran/actions/runs/39947454548)
passed Linux, macOS and Windows CI before the publishing job started. The release
contains six platform executables and `checksums.txt`. All six downloaded SHA-256
hashes matched. All six GitHub build attestations verified with `gh attestation verify`, the release workflow identity, tag ref and source digest pinned, and
`--deny-self-hosted-runners`.

On Linux amd64, latest and pinned (`v0.1.0`) public installers succeeded in fresh
temporary folders. The installed CLI's version and bundled demo were checked.
`go install github.com/vincentsch/saidran/cmd/saidran@v0.1.0` succeeded with an
initially empty module cache, the public Go proxy and checksum database. A separate
empty-cache `go install` with `@latest` also resolved v0.1.0 successfully. The
README agent prompt and `AGENTS.md` were fetched anonymously from GitHub and
matched the published source. Fetching the prompt does not establish a trial
with every supported coding agent.

The [native public-release smoke run](https://github.com/vincentsch/saidran/actions/runs/39948040059)
passed on Linux amd64, macOS arm64 and Windows amd64: public native downloads,
checksum checks, latest and pinned installation where supported, version and demo
checks. Windows checked the `.exe` hash with PowerShell `Get-FileHash`. This is
native execution coverage for those three hosts; the other three assets were
cross-built and their checksums and provenance verified.
