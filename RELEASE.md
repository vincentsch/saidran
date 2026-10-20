# Releasing Gaugestone

Release binaries include the version and commit from the source being built.
Report generation runs offline. Installation and GitHub publication use the network.

## Before tagging

1. Set the install step's `GAUGESTONE_VERSION` in action.yml to the release tag.
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
5. Require native `gaugestone demo` runs on macOS and Windows before tagging.
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

Run the README's agent prompt from the public source with each documented agent.
Confirm its install and first report work without private files. Public install
and public agent-prompt checks happen after publication; record those results then.
