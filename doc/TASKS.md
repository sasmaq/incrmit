# Incrmit — Development Tasks

A checklist of work to implement `incrmit` as described in `README.md` and
`DEVELOPMENT.md`. Tasks are grouped into milestones; check items off as they are
completed.

## Milestone 1 — Project Setup

- [x] Initialize the Go module (`go mod init github.com/sasmaq/incrmit`).
- [x] Create the project layout (`main.go`, `internal/`, `doc/`).
- [x] Add `.gitignore` for build artifacts and editor files.
- [x] Set up `go vet`, `gofmt`, and `golangci-lint`.
- [x] Add a basic CI workflow (build, test, lint).

## Milestone 2 — Version Core

- [x] Define the `Version` type (`Major`, `Minor`, `Patch`).
- [x] Implement parsing of `MAJOR.MINOR.PATCH` strings.
- [x] Implement major bump (reset minor and patch to `0`).
- [x] Implement minor bump (reset patch to `0`).
- [x] Implement patch bump.
- [x] Implement `String()` formatting back to `MAJOR.MINOR.PATCH`.
- [x] Unit tests covering parsing, each bump, and edge cases.

## Milestone 3 — Config

- [x] Define `Config` and `FileEntry` structs with TOML tags.
- [x] Load and parse `incrmit.toml`.
- [x] Validate entries (non-empty paths, existing files).
- [x] Resolve the default config path (`incrmit.toml`).
- [x] Unit tests for valid and invalid config files.

## Milestone 4 — File I/O

- [x] Read a target file and locate its version token.
- [x] Replace only the version token, preserving surrounding formatting.
- [x] Write changes back in place safely (atomic write).
- [x] Golden-file tests confirming only the version changes.

## Milestone 5 — Bump Command

- [x] Parse flags: `--config`/`-c`, `--file`/`-f`, `--major`/`-M`,
      `--minor`/`-m`, `--patch`/`-p`, `--dry-run`/`-d`.
- [x] Resolve the bump component (highest of major/minor/patch wins).
- [x] Resolve targets from `--file` or the config.
- [x] Apply the bump to each target.
- [x] Implement `--dry-run` preview (`old -> new`).
- [x] Print a clear summary of updated files.
- [x] Integration tests for default, `--file`, and `--dry-run` flows.

## Milestone 6 — Discovery

- [x] Implement the `discover` subcommand and its flags
      (`--path`/`-P`, `--output`/`-o`, `--dry-run`/`-d`).
- [x] Walk the directory tree, skipping ignored dirs
      (`.git`, `node_modules`, `vendor`, build outputs).
- [x] Detect versions in `VERSION`, `package.json`, `pyproject.toml`,
      `Cargo.toml`, and Go source files.
- [x] Generate `incrmit.toml` with discovered paths and versions.
- [x] Implement `--dry-run` to print findings without writing.
- [x] Tests over a fixture tree covering each supported file type.

## Milestone 7 — Version Command

- [x] Embed the tool version (build-time `-ldflags` var with a sensible default).
- [x] Implement the `version` subcommand to print the tool version.
- [x] Support a `--version`/`-v` flag as an alias for the subcommand.
- [x] Include build metadata when available (commit, build date) via `runtime/debug`.
- [x] Tests asserting the version command output and exit code `0`.

## Milestone 8 — Error Handling and UX

- [x] Friendly message when the config is missing (suggest `discover`).
- [x] Handle "no version found" and ambiguous matches.
- [x] Surface filesystem and permission errors clearly.
- [x] Implement exit codes (`0`, `1`, `2`, `3`) per the design doc.

## Milestone 9 — Testing

- [x] Set up a shared `testdata/` layout for fixtures and golden files.
- [x] Add table-driven test helpers and shared assertion utilities.
- [x] Run the suite with the race detector (`go test -race ./...`).
- [x] Measure coverage (`go test -cover ./...`) and set a target threshold.
- [x] Add end-to-end CLI tests that build the binary and assert exit codes.
- [x] Add a `-update` golden-file flag to regenerate expected outputs.
- [x] Wire `go test ./...` into CI as a required gate.

## Milestone 10 — Config Self-Maintenance

- [x] After a successful bump, update the `version` field of each entry in
      `incrmit.toml` to the new version (keep the config in sync).
- [x] Skip the config update on `--dry-run` (preview only, write nothing).
- [x] Exclude the config file from discovery (`incrmit.toml` and the discover
      `--output` path) so it is never added as a target.
- [x] Tests for config self-bump and discovery exclusion.

## Milestone 11 — Release

- [x] Cross-compile binaries (Linux, macOS, Windows) via `make dist`.
- [x] Verify `go install` works (local `go install .` reports the right
      version; tagged `@vX.Y.Z` install verified after the tag is pushed).
- [x] Write release notes (`CHANGELOG.md`) for the first version `0.1.3`.
- [x] Tag the first version: `git tag v0.1.3 && git push origin v0.1.3`
      (left to the maintainer).
- [x] Confirm `README.md` examples match actual behavior.

## Milestone 12 — Help Command

- [x] Add a `help` subcommand that prints a top-level overview (tool name,
      one-line description, and a list of commands: default bump, `discover`,
      `version`, and `help`).
- [x] Support command-specific help: `incrmit help discover`, `incrmit help
      version`, and `incrmit help` (or `incrmit help bump`) for the default
      bump flags — reusing the same text as each command's `-h` / `--help`
      output.
- [x] Route `-h` / `--help` at the top level (with no subcommand) to the same
      overview as `incrmit help` (exit code `0`).
- [x] Handle unknown subcommands with a clear error and a hint to run
      `incrmit help` (exit code `2`).
- [x] Centralize usage/help text in one place so bump, discover, version, and
      `help` stay in sync (avoid duplicated `fs.Usage` strings).
- [x] Tests for `help`, `help <command>`, top-level `-h` / `--help`, and
      unknown-command messaging (assert output content and exit code `0` or
      `2` as appropriate).
- [x] Document the `help` subcommand and top-level `-h` / `--help` in
      `README.md` and `doc/DEVELOPMENT.md`.

## Milestone 13 — Automated Release (CI)

- [x] Add a `release` GitHub Actions workflow triggered on tag pushes matching
      `v*` (`on: push: tags: ['v*']`).
- [x] Derive the version from the tag (`${GITHUB_REF_NAME}`) and pass it to
      `make dist VERSION=…` so binaries are stamped with the released version.
- [x] Cross-compile the release matrix (Linux, macOS, Windows; amd64 + arm64)
      and produce per-platform archives plus a `checksums.txt` (SHA-256).
- [x] Create the GitHub Release for the tag and upload the built artifacts
      (e.g. `softprops/action-gh-release` or `gh release create`), using the
      matching `CHANGELOG.md` section as the release notes.
- [x] Grant the workflow `contents: write` permission and use the built-in
      `GITHUB_TOKEN` (no extra secrets required).
- [x] Guard the release job so it only runs on tags (not branch pushes) and,
      optionally, depends on the existing build/test/lint CI passing.
- [x] Document the tag-to-release flow in `README.md` / `doc/DEVELOPMENT.md`
      (push a `vX.Y.Z` tag → CI publishes the release).
- [x] Verify end-to-end on a test tag (e.g. `v0.0.0-test`) and confirm
      `go install github.com/sasmaq/incrmit@vX.Y.Z` resolves the release.

## Milestone 14 — Debian Package (.deb)

- [x] Choose a packaging approach (e.g. `nfpm`, a `debian/` tree with
      `debhelper`, or a `dpkg-deb`-based Makefile recipe) and document the
      rationale in `doc/DEVELOPMENT.md`.
- [x] Add packaging metadata: package name (`incrmit`), version (from `VERSION`),
      architecture (`amd64`, `arm64`), maintainer, short and long description,
      homepage, and license.
- [x] Install the binary to `/usr/bin/incrmit` with mode `0755`; no bundled
      runtime dependencies beyond what a static Go binary needs.
- [x] Build `.deb` artifacts for Linux `amd64` and `arm64`, reusing the same
      `-ldflags` version stamping as `make build` / `make dist`.
- [x] Add a `make deb` (or `make package-deb`) target that writes packages under
      `dist/` alongside the existing release archives.
- [x] Include a man page (`incrmit(1)`) in the package and install it under
      `/usr/share/man/man1/` (source can live in `doc/man/incrmit.1`).
- [x] Verify locally: `sudo dpkg -i dist/incrmit_*.deb`, then `incrmit version`
      and a smoke bump with `--dry-run`; confirm `dpkg -r incrmit` removes the
      binary cleanly.
- [x] Attach the `.deb` files to GitHub Releases (extend the Milestone 13
      release workflow or document a manual upload step until CI is wired).
- [x] Document Debian install and build instructions in `README.md` (e.g.
      `sudo dpkg -i incrmit_<version>_amd64.deb` and `make deb`).

## Milestone 15 — RPM Package (.rpm)

- [x] Choose a packaging approach (e.g. `nfpm`, an `incrmit.spec` for
      `rpmbuild`, or `fpm`) and document the rationale in `doc/DEVELOPMENT.md`
      (reuse the same tool as `.deb` when practical).
- [x] Add RPM metadata: package name (`incrmit`), version (from `VERSION`),
      release suffix (e.g. `1`), target architectures (`x86_64`, `aarch64`),
      summary, description, license, URL, and packager/maintainer fields.
- [x] Install the binary to `/usr/bin/incrmit` with mode `0755`; no bundled
      runtime dependencies beyond what a static Go binary needs.
- [x] Build `.rpm` artifacts for Linux `x86_64` and `aarch64`, reusing the same
      `-ldflags` version stamping as `make build` / `make dist`.
- [x] Add a `make rpm` (or `make package-rpm`) target that writes packages under
      `dist/` alongside the existing release archives.
- [x] Include the shared man page (`incrmit(1)`) under `/usr/share/man/man1/`.
- [x] Verify locally: `sudo rpm -i dist/incrmit-*.rpm` (or `sudo dnf install
      ./dist/incrmit-*.rpm`), then `incrmit version` and a smoke bump with
      `--dry-run`; confirm `sudo rpm -e incrmit` removes the binary cleanly.
- [x] Attach the `.rpm` files to GitHub Releases (extend the Milestone 13
      release workflow or document a manual upload step until CI is wired).
- [x] Document RPM install and build instructions in `README.md` (e.g.
      `sudo rpm -i incrmit-<version>-1.x86_64.rpm` and `make rpm`).

## Milestone 16 — macOS Package (.pkg)

- [x] Choose a packaging approach (e.g. `pkgbuild` / `productbuild`, or a helper
      such as `nfpm` or `fpm`) and document the rationale in
      `doc/DEVELOPMENT.md`.
- [x] Add package metadata: identifier (e.g. `com.github.sasmaq.incrmit`),
      version (from `VERSION`), title, description, and install location
      (`/usr/local/bin/incrmit`).
- [x] Build `.pkg` artifacts for macOS `amd64` and `arm64` (or a single
      universal binary via `lipo`), reusing the same `-ldflags` version stamping
      as `make build` / `make dist`.
- [x] Add a `make pkg` (or `make package-pkg`) target that writes packages under
      `dist/` alongside the existing release archives.
- [x] Include the shared man page (`incrmit(1)`) under
      `/usr/local/share/man/man1/`.
- [x] Verify locally: `sudo installer -pkg dist/incrmit-*.pkg -target /`, then
      `incrmit version` and a smoke bump with `--dry-run`; confirm uninstall
      removes the binary (document the removal steps if no uninstaller is
      shipped).
- [x] Attach the `.pkg` files to GitHub Releases (extend the Milestone 13
      release workflow or document a manual upload step until CI is wired).
- [x] Document macOS install and build instructions in `README.md` (e.g.
      `sudo installer -pkg incrmit-<version>-darwin-arm64.pkg -target /` and
      `make pkg`).
- [ ] Optional: codesign and notarize the `.pkg` (and binary) with Apple
      Developer ID credentials to reduce Gatekeeper warnings on distribution.

## Milestone 17 — Discover `v`-prefixed Versions

- [x] Recognize an optional leading `v` (and `V`) before `MAJOR.MINOR.PATCH`
      during discovery (e.g. `v1.2.3`), so tags and `VERSION`-style files
      using a `v` prefix are detected.
- [x] Update the version token detection/regex in discovery to match `vX.Y.Z`
      without matching unrelated tokens (e.g. avoid `rev1.2.3` or `dev1.2.3`).
- [x] Preserve the original `v` prefix when writing the discovered version to
      `incrmit.toml` and when bumping in place (a `v1.2.3` token bumps to
      `v1.2.4`, a bare `1.2.3` stays bare).
- [x] Decide and document how the prefix is represented in config/state (e.g.
      store the prefix per entry or infer it from the existing token on bump).
- [x] Extend `--dry-run` discovery output to show the `v`-prefixed findings.
- [x] Add fixtures and tests covering `vX.Y.Z` and `VX.Y.Z` detection, prefix
      preservation on bump, and rejection of near-miss tokens (`rev`, `dev`).
- [x] Document `v`-prefix support in `README.md` and `doc/DEVELOPMENT.md`.

## Milestone 18 — Ignore IPv4 Addresses

- [x] Detect and skip IPv4 addresses (e.g. `192.168.1.1`, `10.0.0.255`) during
      discovery so they are not mistaken for `MAJOR.MINOR.PATCH` versions.
- [x] Treat a four-octet `A.B.C.D` token as an IPv4 address, not a version,
      even when each octet is a valid integer (versions have exactly three
      components).
- [x] Avoid matching version-like substrings inside a larger IPv4 address
      (e.g. don't pull `168.1.1` out of `192.168.1.1`).
- [x] Add fixtures and tests covering common IPv4 forms (loopback, private
      ranges, broadcast) and confirm they produce no discovered version.
- [x] Ensure `--dry-run` discovery output excludes IPv4 matches.
- [x] Document the IPv4-skipping behavior in `README.md` and
      `doc/DEVELOPMENT.md`.

## Milestone 19 — Show Flags in the Main Help Command

- [x] Extend the top-level overview (`overviewHelp`) so `incrmit help` and
      top-level `-h` / `--help` list the available flags, not just the
      subcommands.
- [x] Include the default bump flags in the overview (`-c`/`--config`,
      `-f`/`--file`, `-M`/`--major`, `-m`/`--minor`, `-p`/`--patch`,
      `-d`/`--dry-run`) with their short descriptions and defaults.
- [x] Reference each command's flags from the overview (or point to
      `incrmit help <command>`) so discover/version flags remain discoverable.
- [x] Keep the flag text centralized in `internal/cli/help.go` so the overview,
      per-command help, and `-h` / `--help` output stay in sync (no duplicated
      flag strings).
- [x] Ensure the expanded overview still exits with code `0` and renders for
      both `incrmit help` and top-level `-h` / `--help`.
- [x] Update tests to assert the overview output now contains the flag lines
      (and still passes for `help` and top-level `-h` / `--help`).
- [x] Document the richer `incrmit help` overview in `README.md` and
      `doc/DEVELOPMENT.md`.

## Milestone 20 — Discover Multiple Occurrences in a File

- [x] Detect every version occurrence within a single file during discovery
      rather than stopping at the first match.
- [x] Decide how multiple matches map to config entries (e.g. one entry per
      occurrence, line/column or match index to disambiguate, or a count) and
      document the chosen model.
- [x] Handle consistent vs. conflicting versions in the same file (all matches
      agree → single version; differing versions → surface clearly).
- [x] Ensure in-place bumping updates all targeted occurrences in the file, not
      just the first one.
- [x] Extend `--dry-run` discovery output to list each occurrence (with its
      location/context) instead of a single per-file result.
- [x] Add fixtures and tests for files with several identical and several
      differing version tokens, asserting all are found and bumped correctly.
- [x] Document multi-occurrence discovery behavior in `README.md` and
      `doc/DEVELOPMENT.md`.

## Milestone 21 — Ignore Folders and Files in Discovery

- [x] Add an `ignore` field to the TOML config (e.g. a top-level `ignore = [...]`
      list of folder/file path patterns) and model it on `config.Config` with a
      TOML tag.
- [x] Parse and validate `ignore` entries when loading the config (non-empty
      patterns; trim and normalize separators to slashes).
- [x] Make `discover` read the config's `ignore` list (when a config exists at
      the `--output` path) and skip matching folders and files during the walk,
      in addition to the built-in ignored dirs (`.git`, `node_modules`,
      `vendor`, build outputs).
- [x] Match ignore patterns against paths relative to the scan root, supporting
      both directory names (prune the subtree) and file globs
      (e.g. `*.lock`, `docs/**`, `testdata/`).
- [x] Decide and document the matching semantics (glob via `path.Match` vs.
      prefix/exact match, case sensitivity, trailing-slash = directory) and how
      patterns combine with the built-in ignore list.
- [x] Preserve the `ignore` list when `discover` regenerates `incrmit.toml`
      (don't drop user-authored ignore entries on rewrite).
- [x] Extend `--dry-run` discovery output to reflect the applied ignore rules
      (skipped paths are not listed as findings).
- [x] Add fixtures and tests covering ignored directories, file globs, nested
      patterns, and confirmation that non-matching files are still discovered.
- [x] Document the `ignore` config option and its matching rules in `README.md`
      and `doc/DEVELOPMENT.md`.

## Milestone 22 — Undo Command

- [ ] Design the undo model: an `undo` subcommand reverts the most recent bump,
      restoring the previous version token in every file that was written (and
      the `incrmit.toml` self-update), and document the chosen approach in
      `doc/DEVELOPMENT.md`.
- [ ] Persist bump history so undo has something to revert to: after a
      successful (non-`--dry-run`) bump, record a journal entry capturing each
      affected file's path, the old and new version tokens, and a timestamp
      (e.g. a state file such as `.incrmit-history` or `.incrmit.state.toml`).
- [ ] Decide and document the state file's location, format, and lifecycle
      (where it lives, whether it is committed or git-ignored, and how many
      entries are retained — at minimum the last bump).
- [ ] Implement the `undo` subcommand: read the latest journal entry and rewrite
      each recorded file's current token back to its previous value using the
      same atomic, in-place write path as bump (only the version token changes).
- [ ] Restore `incrmit.toml` entries to their pre-bump versions as part of undo
      so the config stays in sync with the reverted files.
- [ ] Detect and handle conflicts safely: if a file's current token no longer
      matches the recorded "new" value (edited since the bump), surface a clear
      error and skip or abort rather than clobbering user changes.
- [ ] Add flags: `--dry-run`/`-d` to preview the revert (`new -> old`) without
      writing, and consider `--config`/`-c` to locate the config/state.
- [ ] Pop or mark the journal entry as undone after a successful revert so
      repeated `undo` does not re-apply the same revert (define behavior when
      there is nothing left to undo).
- [ ] Handle the empty-history case with a friendly message and a sensible exit
      code (no journal / nothing to undo).
- [ ] Wire `undo` into the help system: add it to the top-level overview and
      `incrmit help undo`, reusing the centralized help text in
      `internal/cli/help.go`.
- [ ] Add unit and integration tests: history is written on bump (and not on
      `--dry-run`), a single-file and multi-file bump reverts cleanly, `--dry-run`
      undo writes nothing, conflict detection triggers correctly, and the
      empty-history path returns the expected message and exit code.
- [ ] Document the `undo` command (with examples and the state-file behavior) in
      `README.md`, the help text, and `incrmit(1)` man page.

## Milestone 23 — ASCII Art in the Help Command

- [ ] Design an `incrmit` ASCII-art banner (the tool name/logo) and add it as a
      centralized constant in `internal/cli/help.go` alongside the existing help
      text (keep it in one place so all help paths stay in sync).
- [ ] Render the banner at the top of the top-level overview (`incrmit help` and
      top-level `-h` / `--help`), above the existing description, command list,
      and flag lines.
- [ ] Keep the banner to the overview only (don't repeat it in per-command help
      like `incrmit help discover`) unless a consistent placement is decided and
      documented.
- [ ] Ensure the banner width is terminal-friendly (fits within ~80 columns) and
      uses plain ASCII so it renders correctly on Linux, macOS, and Windows
      terminals without relying on Unicode or color.
- [ ] Confirm the banner does not affect exit codes: `incrmit help` and top-level
      `-h` / `--help` still exit `0`, and error/usage paths are unchanged.
- [ ] Consider suppressing the banner when output is not a TTY (piped/redirected)
      or behind a `--no-banner` / `NO_COLOR`-style opt-out; decide and document
      the behavior (default on vs. TTY-only).
- [ ] Update tests to assert the overview contains the banner (and still contains
      the command and flag lines), and that per-command help is unchanged; update
      any golden files accordingly.
- [ ] Document the banner in `README.md` (e.g. a sample of the `incrmit help`
      output) and note any opt-out flag in the help text and `incrmit(1)` man page.

## Milestone 24 — v1.0.0 Release Readiness: Checks

- [ ] Run `gofmt -l .` and confirm it reports no files (matches the CI
      formatting gate).
- [ ] Run `go vet ./...` and resolve every reported issue.
- [ ] Run `golangci-lint run ./...` locally with the same version CI uses and
      clear all findings (or justify each in-code with a documented `//nolint`).
- [ ] Run `go build ./...` and `make build` and confirm the version is stamped
      correctly (`incrmit version` shows the intended `1.0.0`).
- [ ] Run `go mod tidy` and verify `go.mod`/`go.sum` are unchanged (no stray or
      missing dependencies); confirm the Go version pin is intentional.
- [ ] Audit the public/CLI surface for v1.0.0 stability: confirm flags,
      subcommands, exit codes, config schema, and `incrmit.toml` self-write
      format are final (breaking changes belong before 1.0.0, not after).
- [ ] Review all `internal/` packages (`version`, `config`, `files`,
      `discovery`, `cli`, `buildinfo`) for `TODO`/`FIXME`/`XXX` markers and
      resolve or ticket each.
- [ ] Confirm `README.md`, `doc/DEVELOPMENT.md`, and `incrmit(1)` man page match
      the actual behavior of the shipped binary (flags, examples, exit codes).

## Milestone 25 — v1.0.0 Release Readiness: Functional & Bug Testing

- [ ] Run the full suite with the race detector and coverage
      (`go test -race -cover ./...`) and confirm it passes and meets the
      `make cover` threshold.
- [ ] Regenerate golden files (`go test ./... -update`) and confirm the diff is
      empty (no drift between expected and actual output).
- [ ] Exercise every command end-to-end against a real temp project: default
      patch bump, `--major`/`--minor`/`--patch`, `--file`, `--dry-run`,
      `discover` (with `--path`/`--output`/`--dry-run`), `undo`, `version`, and
      `help`.
- [ ] Verify the bump→undo round trip: after a bump, `undo` restores every
      target file and `incrmit.toml` to their exact pre-bump versions, and a
      conflicting/edited file is refused rather than clobbered.
- [ ] Verify each documented exit code is actually returned: `0` success,
      `1` runtime/missing-config/filesystem error, `2` bad flags/unknown
      command, `3` no/ambiguous/unparseable version.
- [ ] Test edge-case version tokens: `v`/`V` prefix preservation, IPv4 tokens
      skipped, multiple identical vs. differing versions in one file, and
      near-miss tokens (`rev1.2.3`, `dev1.2.3`) rejected.
- [ ] Test file-handling edge cases: missing target file, empty file, file with
      no version, read-only/permission-denied file, very large file, files with
      CRLF vs. LF line endings, and files without a trailing newline.
- [ ] Confirm atomic in-place writes preserve file mode and surrounding content,
      and that a failed/interrupted write never corrupts or truncates the target.
- [ ] Verify config self-maintenance: after a bump the `incrmit.toml` entries are
      updated to the new version; `--dry-run` writes nothing; the config file is
      excluded from discovery.
- [ ] Test config errors: missing config (suggests `discover`), malformed TOML,
      empty/duplicate/ambiguous `[[files]]` entries, and nonexistent paths.
- [ ] Cross-platform smoke test the release binaries (Linux, macOS, Windows;
      amd64 + arm64) — at minimum `version` and a `--dry-run` bump on each.
- [ ] Install-path smoke tests: `go install`, `.deb` (`dpkg -i`/`-r`),
      `.rpm` (`rpm -i`/`-e`), `.pkg` (`installer`), and a tarball/zip extract —
      confirm `incrmit version`, a `--dry-run` bump, and `man incrmit` work.

## Milestone 26 — v1.0.0 Release Readiness: Security Testing

- [ ] Run `govulncheck ./...` and address any reported vulnerabilities in the
      code or dependencies; wire it into CI as a gate.
- [ ] Run `go list -m all` and review every dependency for maintenance status,
      known CVEs, and license compatibility; pin/upgrade as needed.
- [ ] Audit path handling in `discovery` and `files` for path traversal and
      symlink escape (e.g. a config `path` or discovered file pointing outside
      the intended tree, or a symlink to a sensitive file).
- [ ] Confirm atomic writes create temp files securely (restrictive permissions,
      same directory, no predictable/guessable names, no world-writable temp).
- [ ] Verify the tool never follows or writes through symlinks unexpectedly and
      preserves (does not widen) the original file mode on write.
- [ ] Review resource-exhaustion vectors: deep/large directory trees and huge or
      pathological files during discovery (bounded memory, no unbounded reads);
      ensure binary/non-text files are reliably skipped.
- [ ] Confirm no sensitive data (file contents, paths, environment) is leaked to
      logs, error messages, or the rewritten config beyond what is intended.
- [ ] Review the release pipeline supply chain: pinned GitHub Actions, minimal
      `GITHUB_TOKEN`/workflow permissions, no secret leakage in logs, and
      reproducible `-ldflags` version stamping.
- [ ] Verify published `checksums.txt` (SHA-256) covers every artifact and the
      documented verification steps in `README.md` actually succeed against the
      released assets.
- [ ] Run a Bugbot and/or security review pass over the final diff for v1.0.0 and
      triage every finding before tagging.

## Milestone 27 — v1.0.0 Release: Publish

- [ ] Bump the tool version to `1.0.0` across all tracked files (run `incrmit`
      on its own `incrmit.toml`) and confirm `README.md` "Version" and
      `go install …@v1.0.0` references are updated.
- [ ] Add a `[1.0.0]` section to `CHANGELOG.md` summarizing the stable release
      and add the matching release-tag link at the bottom.
- [ ] Verify CI is green on `main` (build, vet, fmt, test with race + coverage,
      lint, and `govulncheck`) before tagging.
- [ ] Tag and push the release: `git tag v1.0.0 && git push origin v1.0.0`, then
      confirm the release workflow builds all archives/packages and publishes the
      GitHub Release with the `1.0.0` changelog notes.
- [ ] Post-release verification: `go install github.com/sasmaq/incrmit@v1.0.0`
      resolves, and each published artifact installs and reports `1.0.0`.
