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

- [x] Design the undo model: an `undo` subcommand reverts the most recent bump,
      restoring the previous version token in every file that was written (and
      the `incrmit.toml` self-update), and document the chosen approach in
      `doc/DEVELOPMENT.md`.
- [x] Persist bump history so undo has something to revert to: after a
      successful (non-`--dry-run`) bump, record a journal entry capturing each
      affected file's path, the old and new version tokens, and a timestamp
      (e.g. a state file such as `.incrmit-history` or `.incrmit.state.toml`).
- [x] Decide and document the state file's location, format, and lifecycle
      (where it lives, whether it is committed or git-ignored, and how many
      entries are retained — at minimum the last bump).
- [x] Implement the `undo` subcommand: read the latest journal entry and rewrite
      each recorded file's current token back to its previous value using the
      same atomic, in-place write path as bump (only the version token changes).
- [x] Restore `incrmit.toml` entries to their pre-bump versions as part of undo
      so the config stays in sync with the reverted files.
- [x] Detect and handle conflicts safely: if a file's current token no longer
      matches the recorded "new" value (edited since the bump), surface a clear
      error and skip or abort rather than clobbering user changes.
- [x] Add flags: `--dry-run`/`-d` to preview the revert (`new -> old`) without
      writing, and consider `--config`/`-c` to locate the config/state.
- [x] Pop or mark the journal entry as undone after a successful revert so
      repeated `undo` does not re-apply the same revert (define behavior when
      there is nothing left to undo).
- [x] Handle the empty-history case with a friendly message and a sensible exit
      code (no journal / nothing to undo).
- [x] Wire `undo` into the help system: add it to the top-level overview and
      `incrmit help undo`, reusing the centralized help text in
      `internal/cli/help.go`.
- [x] Add unit and integration tests: history is written on bump (and not on
      `--dry-run`), a single-file and multi-file bump reverts cleanly, `--dry-run`
      undo writes nothing, conflict detection triggers correctly, and the
      empty-history path returns the expected message and exit code.
- [x] Document the `undo` command (with examples and the state-file behavior) in
      `README.md`, the help text, and `incrmit(1)` man page.

## Milestone 23 — ASCII Art in the Help Command

- [x] Design an `incrmit` ASCII-art banner (the tool name/logo) and add it as a
      centralized constant in `internal/cli/help.go` alongside the existing help
      text (keep it in one place so all help paths stay in sync).
- [x] Render the banner at the top of the top-level overview (`incrmit help` and
      top-level `-h` / `--help`), above the existing description, command list,
      and flag lines.
- [x] Keep the banner to the overview only (don't repeat it in per-command help
      like `incrmit help discover`) unless a consistent placement is decided and
      documented.
- [x] Ensure the banner width is terminal-friendly (fits within ~80 columns) and
      uses plain ASCII so it renders correctly on Linux, macOS, and Windows
      terminals without relying on Unicode or color.
- [x] Confirm the banner does not affect exit codes: `incrmit help` and top-level
      `-h` / `--help` still exit `0`, and error/usage paths are unchanged.
- [x] Consider suppressing the banner when output is not a TTY (piped/redirected)
      or behind a `--no-banner` / `NO_COLOR`-style opt-out; decide and document
      the behavior (default on vs. TTY-only). Decision: default on regardless of
      TTY (keeps the overview reproducible and testable), with `--no-banner` as
      the opt-out on `incrmit help` and top-level `-h` / `--help`.
- [x] Update tests to assert the overview contains the banner (and still contains
      the command and flag lines), and that per-command help is unchanged; update
      any golden files accordingly.
- [x] Document the banner in `README.md` (e.g. a sample of the `incrmit help`
      output) and note any opt-out flag in the help text and `incrmit(1)` man page.

## Milestone 24 — v1.0.0 Release Readiness: Checks

- [x] Run `gofmt -l .` and confirm it reports no files (matches the CI
      formatting gate).
- [x] Run `go vet ./...` and resolve every reported issue.
- [x] Run `golangci-lint run ./...` locally with the same version CI uses and
      clear all findings (or justify each in-code with a documented `//nolint`).
      Reports `0 issues`. CI's `golangci-lint-action` is now pinned to `v2.12.2`
      so the local and CI linter versions match.
- [x] Run `go build ./...` and `make build` and confirm the version is stamped
      correctly (`incrmit version` shows the intended `1.0.0`). Stamping is
      verified working (`-ldflags` override honored, and an unstamped `go build`
      falls back to the same source default); the value is `0.1.13` until the
      version is bumped to `1.0.0` in Milestone 48.
- [x] Run `go mod tidy` and verify `go.mod`/`go.sum` are unchanged (no stray or
      missing dependencies); confirm the Go version pin is intentional. Tidy is
      a no-op and there is one dependency (`github.com/BurntSushi/toml v1.6.0`).
      The pin was `go 1.26.4`, which contradicted README's "Go 1.26 or later";
      loosened to `go 1.26`.
- [x] Audit the public/CLI surface for v1.0.0 stability: confirm flags,
      subcommands, exit codes, config schema, and `incrmit.toml` self-write
      format are final (breaking changes belong before 1.0.0, not after). All
      four exit codes verified end-to-end against the built binary; the config
      self-write is deterministic (byte-identical across repeat bumps) but drops
      user comments, which is now documented.
- [x] Review all `internal/` packages (`version`, `config`, `files`,
      `discovery`, `cli`, `buildinfo`) for `TODO`/`FIXME`/`XXX` markers and
      resolve or ticket each. No markers found anywhere in the Go sources.
- [x] Confirm `README.md`, `doc/DEVELOPMENT.md`, and `incrmit(1)` man page match
      the actual behavior of the shipped binary (flags, examples, exit codes).
      Fixed: stale project layout in `DEVELOPMENT.md` (missing `internal/cli`
      and `internal/buildinfo`), out-of-order sections 6.2/6.3, the undocumented
      config-comment loss, and the missing bump-flag precedence note in the man
      page.

## Milestone 25 — v1.0.0 Release Readiness: Functional & Bug Testing

- [x] Run the full suite with the race detector and coverage
      (`go test -race -cover ./...`) and confirm it passes and meets the
      `make cover` threshold. Passes; total coverage 89.1% (threshold 80%).
- [x] Regenerate golden files (`go test ./... -update`) and confirm the diff is
      empty (no drift between expected and actual output). No drift. Note that
      only `internal/files` defines `-update`, so the working command is
      `go test ./internal/files/ -update`; `go test ./... -update` fails on the
      packages that do not define the flag.
- [x] Exercise every command end-to-end against a real temp project: default
      patch bump, `--major`/`--minor`/`--patch`, `--file`, `--dry-run`,
      `discover` (with `--path`/`--output`/`--dry-run`), `undo`, `version`, and
      `help`.
- [x] Verify the bump→undo round trip: after a bump, `undo` restores every
      target file and `incrmit.toml` to their exact pre-bump versions, and a
      conflicting/edited file is refused rather than clobbered. Restores are
      byte-for-byte; repeated undos walk back through successive bumps.
- [x] Verify each documented exit code is actually returned: `0` success,
      `1` runtime/missing-config/filesystem error, `2` bad flags/unknown
      command, `3` no/ambiguous/unparseable version.
- [x] Test edge-case version tokens: `v`/`V` prefix preservation, IPv4 tokens
      skipped, multiple identical vs. differing versions in one file, and
      near-miss tokens (`rev1.2.3`, `dev1.2.3`) rejected. Also confirmed
      overlapping bumps in one file do not cascade.
- [x] Test file-handling edge cases: missing target file, empty file, file with
      no version, read-only/permission-denied file, very large file, files with
      CRLF vs. LF line endings, and files without a trailing newline. A
      read-only file is still rewritten when its directory is writable (a
      consequence of the rename-based write, now documented); an unwritable
      directory fails without touching the target.
- [x] Confirm atomic in-place writes preserve file mode and surrounding content,
      and that a failed/interrupted write never corrupts or truncates the target.
      Modes 600/640/664/755 all survive; no temp files are ever left behind.
- [x] Verify config self-maintenance: after a bump the `incrmit.toml` entries are
      updated to the new version; `--dry-run` writes nothing; the config file is
      excluded from discovery. `--dry-run` also creates no state file and leaves
      an existing one untouched.
- [x] Test config errors: missing config (suggests `discover`), malformed TOML,
      empty/duplicate/ambiguous `[[files]]` entries, and nonexistent paths.
- [x] Cross-platform smoke test the release binaries (Linux, macOS, Windows;
      amd64 + arm64) — at minimum `version` and a `--dry-run` bump on each.
      `windows/arm64` was added to `PLATFORMS`, so the matrix is now six
      artifacts, all of the correct format (`Mach-O x86_64`/`arm64`,
      `ELF x86-64`/`aarch64`, `PE32+ x86-64`/`Aarch64`). Both darwin binaries
      and both linux binaries (in containers) complete a full
      `version` → `--dry-run` → bump → `undo` cycle. Windows binaries can only
      be format-checked here; executing them needs a Windows host or CI runner.
- [x] Install-path smoke tests: `go install`, `.deb` (`dpkg -i`/`-r`),
      `.rpm` (`rpm -i`/`-e`), `.pkg` (`installer`), and a tarball/zip extract —
      confirm `incrmit version`, a `--dry-run` bump, and `man incrmit` work.
      `go install` works from the local module and from
      `github.com/sasmaq/incrmit@v0.1.13` via the proxy. Tarball and zip extract
      with the executable bit intact. `checksums.txt` covers all ten artifacts
      and verifies with `shasum -a 256 -c`. `dpkg -i`/`-r` (Debian, amd64 and
      arm64) and `rpm -i`/`-e` (Fedora, x86_64 and aarch64) install
      `/usr/bin/incrmit` plus the man page, run a bump/undo cycle, and remove
      both files cleanly; `man incrmit` renders from the installed package.
      The `.pkg` was verified by payload inspection
      (`pkgutil --expand-full` shows a clean tree staging
      `/usr/local/bin/incrmit` and the man page, with no literal `._*` files)
      rather than by running `installer`, which needs root and would modify the
      host's `/usr/local`.

## Milestone 26 — v1.0.0 Release Readiness: Security Testing

- [x] Run `govulncheck ./...` and address any reported vulnerabilities in the
      code or dependencies; wire it into CI as a gate.
      Clean: "No vulnerabilities found" with `govulncheck@v1.6.0` against Go
      1.26.5 and DB `vuln.go.dev`. Added a `vulncheck` job to `ci.yml` and a
      `govulncheck` step to the release workflow's validation job, both pinned to
      `@v1.6.0` so a scanner upgrade is a deliberate change.
- [x] Run `go list -m all` and review every dependency for maintenance status,
      known CVEs, and license compatibility; pin/upgrade as needed.
      One direct dependency, `github.com/BurntSushi/toml v1.6.0`, and zero
      transitive ones. v1.6.0 is the latest release; the module is actively
      maintained and MIT licensed (permissive, compatible). Both the module and
      its `go.mod` hashes are recorded in `go.sum`, so builds are verified against
      the Go checksum database.
- [x] Audit path handling in `discovery` and `files` for path traversal and
      symlink escape (e.g. a config `path` or discovered file pointing outside
      the intended tree, or a symlink to a sensitive file).
      Found and fixed a real symlink escape in discovery: `filepath.WalkDir`
      reports a symlinked *file* as an ordinary entry, and `os.ReadFile` then
      followed it. A link to a 0600 file outside the tree was scanned, its matched
      line printed in `--dry-run` output, and after `discover` recorded it the next
      bump copied the outside file's contents into the tree (replacing the link).
      `Discover` now skips any entry whose type includes `fs.ModeSymlink`.
      Symlinked *directories* were already safe (WalkDir does not descend into
      them), as was a symlinked scan root (it yields no results either way, both
      before and after the change).
      Config `path` traversal is intentional and now documented: a relative path
      resolves against the config's directory, and an absolute path or one with
      `../` is honoured, so `incrmit.toml` is trusted input on a par with a
      `Makefile`. Verified both cases write where the config points.
- [x] Confirm atomic writes create temp files securely (restrictive permissions,
      same directory, no predictable/guessable names, no world-writable temp).
      All good. `os.CreateTemp(dir, ".incrmit-*.tmp")` creates the temp file
      in the *target's own* directory (never a shared temp dir, so the rename
      is always same-filesystem and atomic) with `O_EXCL` and a random suffix
      — observed `.incrmit-1623281899.tmp` at mode 0600 mid-write. Nothing is
      left behind on success or failure. Hardened one narrow race: the mode is
      now set through the open descriptor (`(*os.File).Chmod`) instead of by
      name, so a name swapped between close and chmod cannot redirect the mode
      change.
- [x] Verify the tool never follows or writes through symlinks unexpectedly and
      preserves (does not widen) the original file mode on write.
      Confirmed both. A symlinked target is *replaced* by a regular file rather
      than written through — the link's target is left untouched, so a link cannot
      redirect a write elsewhere. Mode is preserved exactly across 400, 600, 640,
      660, 664, 700, 755, and 777 (0644 only for a newly created file); a write
      never widens permissions. Locked in by
      `TestWriteAtomicDoesNotWriteThroughSymlink` and
      `TestWriteAtomicNeverWidensMode`.
- [x] Review resource-exhaustion vectors: deep/large directory trees and huge or
      pathological files during discovery (bounded memory, no unbounded reads);
      ensure binary/non-text files are reliably skipped.
      Found and fixed two denial-of-service vectors. A FIFO anywhere in the tree
      hung `discover` forever, because `os.ReadFile` on a named pipe blocks until
      a writer appears — and the block is in `open`, so the type has to be
      checked *before* opening. A symlink to `/dev/zero` read without end.
      `detect` now `os.Lstat`s first and reads only regular files, re-checking
      on the open descriptor so a swapped path is still rejected. Reads are also
      capped at `maxScanBytes` (32 MiB), enforced by the size check and again by
      an `io.LimitReader` in case the file grows in between; a 250 MB file
      previously went entirely into memory. Deep trees are fine (300 levels,
      exit 0), and binary files are still reliably skipped via the NUL-byte
      check. The same hang existed on the bump side, where a target is named
      explicitly rather than discovered: a FIFO in `incrmit.toml` or passed to
      `--file` blocked forever. Both read sites now go through
      `files.ReadTarget`, which checks the type before opening and reports
      `reading <path>: not a regular file` (exit 1); a symlink to a real file
      is still accepted.
- [x] Confirm no sensitive data (file contents, paths, environment) is leaked to
      logs, error messages, or the rewritten config beyond what is intended.
      Clean. The code never touches the environment (no `os.Getenv`, `os.Environ`,
      or `os.LookupEnv` anywhere). Error messages name paths and positions but
      never file contents — a malformed config reports
      `toml: line 3: expected …` without echoing the line, and an unreadable
      target, a version-less target, and an undo conflict all report only the
      path and versions. The state file holds absolute paths of the config and
      targets (needed so `undo` works from any directory), is documented as
      local working state, and is gitignored here; it carries nothing secret.
      With symlink following removed, dry-run line context can now only come
      from files inside the tree that was scanned on purpose.
- [x] Review the release pipeline supply chain: pinned GitHub Actions, minimal
      `GITHUB_TOKEN`/workflow permissions, no secret leakage in logs, and
      reproducible `-ldflags` version stamping.
      Fixed three weaknesses. (1) All four actions were pinned to movable major
      tags; they are now pinned to commit SHAs with the release in a trailing
      comment (`actions/checkout` v4.4.0, `actions/setup-go` v5.6.0,
      `golangci/golangci-lint-action` v8.0.0, `softprops/action-gh-release`
      v2.6.2) — the last of these runs in a job that can write to the repository.
      (2) `release.yml` granted `contents: write` at the workflow level, so the
      validation job held a write-capable token; the default is now `contents:
      read` with `write` raised only on the two publishing jobs. (3) The release
      job's `golangci-lint` was unpinned while CI pinned v2.12.2, so the release
      gate could differ from CI; both now pin v2.12.2.
      Version stamping: builds were deterministic but not path-independent, and
      embedded 10 absolute source paths from the build machine. All Makefile builds
      now pass `-trimpath`; a released linux/amd64 binary contains zero builder
      paths and rebuilds byte-identically. The only credential is the built-in
      `GITHUB_TOKEN`; no workflow step echoes a secret.
- [x] Verify published `checksums.txt` (SHA-256) covers every artifact and the
      documented verification steps in `README.md` actually succeed against the
      released assets.
      Found a real gap: of 12 publishable artifacts, `checksums.txt` covered
      10 — both macOS `.pkg` installers had no published hash, even though
      `README.md` claimed hashes "of every artifact" and showed an example
      verifying a `.pkg` against `checksums.txt`, a step that could not succeed.
      The `.pkg` files are built on a separate macOS runner, so two machines
      cannot append to one file; added a `pkg-checksums` target writing
      `dist/checksums-macos.txt`, a `release-macos` target that runs both, and
      made the macOS job upload it. Re-verified with a full local build: all 12
      artifacts now hash-covered and `shasum -a 256 -c` passes `OK` against both
      files.
- [x] Run a Bugbot and/or security review pass over the final diff for v1.0.0 and
      triage every finding before tagging.
      Ran a security review pass over the change set. It found no high or critical
      issues and confirmed the discovery boundaries, atomic write, and workflow
      permission model. Triage of everything it raised:
      - *Medium — `govulncheck` installed from a "movable" tag while Actions are
        SHA-pinned.* Partly accepted. The stated attack path (someone moves the
        `v1.6.0` tag and CI runs their code) does not apply to Go modules: the proxy
        serves a version's content immutably and the go command verifies it against
        the checksum database, so a moved tag fails the build. Confirmed the pinned
        version has a published hash in the transparency log
        (`golang.org/x/vuln v1.6.0 h1:FeMO9Rm/…`, commit `19b0bb6a`) and that
        `GOSUMDB=sum.golang.org` is the default. The real residual is that the
        guarantee depends on the runner's environment, so `GOPROXY` and `GOSUMDB`
        are now set explicitly on the `govulncheck` steps and on the `nfpm` install
        (which runs in a job that can write to the repository). Recording the tool
        hashes in this repo's `go.sum` via a Go `tool` directive was rejected: it
        would pull the tools' dependency trees into `go.mod`, and one dependency
        with no transitive ones is worth more than re-pinning what the checksum
        database already pins.
      - *Hard link inside the tree pointing at a file elsewhere is still read.*
        Accepted as-is, and the reviewer agreed it is not medium+. A hard link cannot
        be delivered by a git clone and needs local write access on the same
        filesystem, so it is not reachable through the "scan an unfamiliar checkout"
        path that motivated the symlink fix.
      - *Local TOCTOU: `Open` without `O_NOFOLLOW` after the `Lstat`.*
        Accepted. It needs a colluding writer racing inside the directory being
        scanned, i.e. same host and same user; the descriptor re-check already
        rejects the result.
      - *A `.incrmit-*.tmp` file is briefly listable at the target's final mode
        before the rename.* Accepted. It holds the same bytes the target is
        about to hold, at the same mode, in the same directory.
      - *No cap on file count or total walk time; config/bump reads are uncapped.*
        Accepted and already documented. Discovery reads sequentially so peak
        memory stays bounded by the per-file cap, and config targets are trusted
        input.
      - *Pre-existing and out of scope for this diff:* `go-version: stable` and
        the unsigned macOS `.pkg`.

## Milestone 27 — Bump Preview Command

A read-only command that visualizes, for every entry in `incrmit.toml`, the
current version alongside what a `--patch`, `--minor`, and `--major` bump would
produce — so the user can see all three outcomes at once without running a
`--dry-run` per component.

- [x] Decide the command name and document it (`preview` is the working name;
      alternatives considered: `show`, `plan`, `list`). Pick one, and keep it a
      read-only command that never writes files, config, or state.
      Kept `preview`: it names what the output is (a projection) without
      implying a staged change the way `plan` does, and `show`/`list` read as
      "print the current state", which is only one of the four columns.
- [x] Implement the subcommand in `internal/cli`: load the config (same
      resolution as bump, honoring `--config`/`-c`), parse each entry's version,
      and compute the patch, minor, and major results for it.
      `internal/cli/preview.go`. Rather than reimplement the load-and-parse
      pass, `planGroups` was split in two: `readGroups` does the part that does
      not depend on which bump was asked for (read each distinct file once,
      resolve the config-pinned token or scan for one), and `planGroups` applies
      the bump on top. Preview calls `readGroups`, so the "current" column is by
      construction the version a bump would start from.
- [x] Render an aligned table with one row per config entry and the columns
      `path`, `current`, `patch`, `minor`, `major` (e.g.
      `README.md  0.1.15  0.1.16  0.2.0  1.0.0`), padding columns to the widest
      value so the output stays readable in a plain terminal.
      Uppercase headers, a two-space gutter, and every column padded to its
      widest cell. Lines are right-trimmed so no row carries trailing
      whitespace (the last column, and an empty drift marker, would otherwise
      leave some).
- [x] Preserve the `v`/`V` prefix in every projected version so a `v1.2.3` entry
      previews as `v1.2.4` / `v1.3.0` / `v2.0.0`, matching what a real bump
      would write.
      Free, because the projections come from `version.BumpPatch/Minor/Major`
      themselves rather than from reformatted numbers — which also means a
      prerelease or build section is dropped in the preview exactly as a real
      component bump drops it.
- [x] Group or de-duplicate repeated paths sensibly: a file with several
      occurrences (Milestone 20) appears once per distinct version token, and
      identical `path` + `version` rows are not printed twice.
      `previewRows` flattens the file groups into one row per
      `(path, version)`, keeping config order.
- [x] Add `--file`/`-f` to preview a single target (bypassing the config, same
      semantics as bump) and reuse the shared target-resolution code rather than
      duplicating it.
      `resolveTargets` now takes the config path and file directly instead of a
      `bumpOptions`, so bump and preview share it unchanged.
- [x] Highlight entries that are out of sync — when the config holds versions
      that differ from each other, mark the rows (or print a short note) so a
      drifting file is visible in the preview.
      Both: the drifting rows get a `*` and a footnote names the version most
      entries hold. Drift is judged by semver precedence, so entries differing
      only in the `v` prefix or in build metadata are not marked — they name the
      same release, and marking them would bury the rows that really are behind
      (this repo's own config, which lists `README.md` at both `0.2.0` and
      `v0.2.0`, would otherwise be permanently flagged). Ties go to the higher
      version, which is deterministic and reads the stragglers as stale. A file
      that holds a deliberately different version, such as a vendored
      dependency, is marked too; the tool cannot tell it from drift, and the
      README says so.
- [x] Handle the error paths with the documented exit codes: missing config
      suggests `discover` (exit `1`), bad flags exit `2`, and an unparseable or
      missing version token exits `3` with the offending path named.
      All four reuse the existing `classify` mapping and `config.NotExistError`,
      so preview cannot drift from the codes the other commands return.
- [x] Wire the command into the help system: add it to `overviewHelp`, add a
      `previewHelp` block with its flags, and support `incrmit help preview`,
      keeping all text centralized in `internal/cli/help.go`.
      Done, with `previewFlags` factored out and composed into both the overview
      and `previewHelp` the way the other commands' blocks are.
- [x] Add tests: table rendering against a golden file (multi-entry config,
      mixed `v`-prefixed and bare versions, differing column widths), the
      `--file` path, the machine-readable output, and each error/exit-code case;
      assert the command writes nothing to disk.
      `internal/cli/preview_test.go` with two goldens under
      `internal/cli/testdata` (in-sync table, drift table), regenerated
      with `go test ./internal/cli/ -update`. No machine-readable output is
      covered because none ships. The read-only claim is tested by
      snapshotting every file in the tree before and after and comparing both
      the contents and the file count, so a stray write or a new state file
      fails. `internal/cli` coverage is 94.9%.
- [x] Document the command in `README.md` (with sample output), the
      `incrmit(1)` man page, and `doc/DEVELOPMENT.md`, and add a `CHANGELOG.md`
      entry under `Added`.
      README gained a `Preview` section (table, drift, flags), the man page a
      `preview` command entry plus an example, and `DEVELOPMENT.md` sections 7
      and 8.5 covering the design decisions.
      The CHANGELOG entry is under a new `[Unreleased]` heading.

## Milestone 28 — Prerelease and Build Metadata

`version.Parse` accepts only a bare `MAJOR.MINOR.PATCH`, but `versionRe`
(`\b[vV]?\d+(?:\.\d+)+\b`) matches the numeric core inside a larger token, so a
prerelease is silently mangled rather than rejected: `1.2.3-rc.1` bumps to
`1.2.4-rc.1` and `1.2.3+build.7` to `1.2.4+build.7`. Both are wrong under
semver — a patch bump off `1.2.3-rc.1` is `1.2.3` (promote) or `1.2.3-rc.2`
(iterate), never `1.2.4-rc.1`. This is a correctness bug and should be closed
before v1.0.0 freezes the behavior.

- [x] Add a regression test that pins the current wrong behavior first, so the
      fix is demonstrably a change: `1.2.3-rc.1`, `1.2.3+build.7`, and
      `v2.0.0-beta.1+exp.sha.5114f85` through a real `--file` bump.
      Pinned in `internal/cli/prerelease_test.go` (bump, per-component bump,
      dry-run summary, and the config `discover` writes), then replaced in place
      with the corrected expectations once the fix landed.
- [x] Extend `versionRe` (or add a wider "candidate token" pattern) so the
      matcher sees the *whole* semver token including any `-prerelease` and
      `+build` suffix, rather than stopping at the numeric core. Keep the IPv4
      and two-component rejections from Milestone 18 working.
      The pattern gained
      `(?:-[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?`,
      and both packages now call one `version.FindTokens` instead of keeping
      identical copies of the regex. The greedy numeric core still swallows an
      IPv4 address whole, and the trailing `\b` keeps a suffix from running into
      an adjacent word — RE2 has no look-ahead, so that boundary is what makes
      `1.2.3-rc_1` match only `1.2.3`.
      `FindTokens` also carries a filename guard, added after the first cut of
      this milestone destroyed data: a hyphen is a legal prerelease character, so
      `incrmit-1.2.3-linux-amd64.tar.gz` parsed as `1.2.3` with the prerelease
      `linux-amd64.tar.gz`, and a bump rewrote the line to `incrmit-1.2.4` and
      exited `0`. A token preceded by `-` that is itself preceded by a word
      character now keeps only its numeric core, so filenames and download URLs
      bump the way they did before this milestone.
- [x] Extend `version.Version` with `Prerelease` and `Build` fields and teach
      `Parse` the semver 2.0.0 grammar for both (dot-separated alphanumeric
      identifiers; numeric prerelease identifiers must not have leading zeros).
      Keep `Prefix` behavior from Milestone 17 intact.
      Build metadata is split off first, since `+` cannot appear anywhere else
      while `-` may appear inside a build identifier (`1.2.3+exp-1`). Both
      characters are consumed by the splits, so the old explicit sign check is
      gone: `+1.2.3` and `1.-2.3` now fail as an empty or short numeric core.
- [x] Update `String()` to round-trip the full token
      (`v1.2.3-rc.1+build.7` in, same out), and confirm the golden-file tests in
      `internal/files/testdata` still show only the version token changing.
      The four golden files pass unchanged.
- [x] Decide and document the bump semantics for a prerelease input, then
      implement them: `BumpMajor`/`BumpMinor`/`BumpPatch` on `1.2.3-rc.1` should
      drop the prerelease and build metadata (`1.2.4`), matching how every other
      bump tool behaves. Build metadata is never carried forward.
- [x] Add explicit promote/iterate flags rather than overloading the existing
      ones: `--release` (drop the prerelease: `1.2.3-rc.1` -> `1.2.3`) and
      `--pre <id>` (start or advance a prerelease: `1.2.3` -> `1.2.4-rc.1`,
      `1.2.4-rc.1` -> `1.2.4-rc.2`). Reject combinations that are meaningless
      (e.g. `--release` on a version with no prerelease) with exit code `2`.
      Shipped as `-r, --release` and `-e, --pre <id>`. `--pre` combines with a
      component flag by rule: naming one explicitly opens a new release line
      (`--minor --pre rc` -> `1.3.0-rc.1`), while with none the current version
      decides (a release starts the next patch's prerelease, a prerelease
      iterates in place). Since `--patch` defaults to `true`, `FlagSet.Visit`
      records which flags the user actually named. Rejected with exit `2`:
      `--release` with `--pre`, `--release` with a component flag, an invalid
      `--pre` identifier, and `--release` on a version with no prerelease — the
      last is per-target, so the transform returns an error that `classify` maps
      to `ExitUsage`, and the plan phase aborts before any file is written.
- [x] Make `discover` record the full token in `incrmit.toml` so a prerelease
      target survives a regeneration, and confirm `SetKnownVersions` matches on
      the full token (a config holding `1.2.3-rc.1` must not match a bare
      `1.2.3` elsewhere in the same file).
      Both directions are covered by tests: a pinned prerelease leaves the bare
      release alone, and a pinned release leaves the prerelease alone.
      The token is recorded in three keys rather than one string —
      `version = "1.2.3"` plus `prerelease = "rc.1"` (and `build`) — which is
      what lets the rewriter tell a real prerelease from a hyphenated filename
      part instead of guessing: with `rc.1` pinned, `--release` promotes
      `app-1.2.3-rc.1.zip` to `app-1.2.3.zip`, and a prerelease written into a
      download URL is still found on the following step. `SetKnownVersions`
      therefore takes parsed `files.Replacement` values rather than a text map,
      matching an exact token or a guard-trimmed core whose following bytes
      continue with exactly the pinned suffix. Old configs holding an inline
      token are migrated by `config.Load`; a bump that drops the prerelease
      drops the key too.
- [x] Verify precedence ordering is not needed anywhere, or implement
      `Compare` if the preview/out-of-sync check in Milestone 27 relies on it.
      Nothing in bump, discover, or undo compares versions — they match tokens —
      so nothing needed it. `Compare` was implemented anyway (full semver
      precedence, ignoring prefix and build) because Milestone 27's out-of-sync
      check is the one place that will, and prereleases are exactly what makes
      that ordering non-obvious.
- [x] Update `README.md`, the `incrmit(1)` man page, and `doc/DEVELOPMENT.md`
      with the supported grammar and the promote/iterate flags; add a
      `CHANGELOG.md` entry under `Fixed` (mangling) and `Added` (flags).
      README gained a "Prereleases and build metadata" section, the man page a
      VERSION GRAMMAR section plus the two options, and DEVELOPMENT.md section
      9.2. All three record the filename guard and its one real cost: a genuine
      prerelease inside a filename (`app-1.2.3-rc.1.zip`) is matched as its core
      alone, so it bumps to `app-1.2.4-rc.1.zip` rather than `app-1.2.4.zip`.

## Milestone 29 — Concurrent Runs

`incrmit` starts no goroutines, so this is not about internal races — it is
about two processes. Every mutating command is a read-modify-write over shared
on-disk state and nothing coordinated them: `bump` loads the journal, pushes an
entry, and saves it, and rewrites `incrmit.toml` in place; `undo` loads, pops,
and saves. Run two bumps at once — `make -j`, a CI matrix, a file watcher, two
terminals — and the second save silently erases the first: the tree gets bumped
twice while the config records one version and the journal one entry, so the
erased bump can never be undone. `WriteAtomic` is why this is easy to miss.
Every individual write is all-or-nothing, so nothing is ever *corrupt*; it is
only lost.

- [x] Reproduce the loss before fixing it, so the fix has something to prove.
      Drive two runs against one project concurrently (goroutines calling the
      CLI entry point is fair here, since all the contended state is on disk)
      and assert the specific damage: a journal holding one entry after two
      bumps, and a config whose recorded version disagrees with the files.
      `internal/cli/concurrent_test.go` starts eight `cli.Main` calls behind one
      barrier. Before the fix all eight exited `0` while the tree advanced a
      single patch (1.0.0 -> 1.0.1), the config recorded that one version, and
      the journal held **three** entries — seven bumps reported as applied,
      one applied, three recorded. The assertion is the invariant rather than
      the damage (tree, config, and journal all agree on the number of applied
      bumps), so the same test proves the fix instead of being replaced by it.
      Goroutines are fair for a second reason that only became clear while
      implementing: flock is held by the open file description, not the process,
      so two runs inside one test binary contend exactly as two shells do.
- [x] Add an `internal/lock` package taking one exclusive advisory file lock per
      project, next to the config. Use `syscall.Flock` on Unix and
      `LockFileEx` on Windows in build-tagged files, mirroring the existing
      `internal/testutil/fifo_unix.go` / `fifo_windows.go` split, so this costs
      no new module. Prefer an OS advisory lock over an `O_EXCL` PID file
      specifically because of stale locks: the kernel releases a flock when the
      process exits for any reason, `SIGKILL` and panics included, so there is
      never a leftover lock to clear by hand — a PID file outlives the crash and
      forces the tool to guess whether the owner is still alive.
      Shipped as `lock.Acquire` / `lock.AcquireWait` / `Lock.Release` over
      `.incrmit.lock` (`config.LockFileName`, kept beside `StateFileName` so
      every tool-maintained file name lives in one place). The file is never
      unlinked on release: removing it would let a second run create and lock a
      fresh file at the same name while the first still held a lock on the old
      inode, which is the race the lock exists to prevent. It is left holding a
      two-line note explaining what it is, written only once the lock is held,
      and carrying no version-like token so the scan can never match it.
      All six release platforms (linux, darwin, windows × amd64, arm64) build.
- [x] Hold the lock across the whole read-modify-write of every mutating
      command (`bump`, `undo`, and `init`/`discover` when it writes the config),
      releasing it on every return path. Keep read-only commands (`status`,
      `preview`, any `--dry-run`) lock-free so inspecting a project can neither
      block nor be blocked; note in their docs that they may therefore observe a
      bump in progress. Scope the lock to the config's directory so separate
      projects never contend.
      The lock is taken *before* the config is read, not just before the write:
      the config names the targets and the targets give the starting versions, so
      a run that read between our read and our write would compute from the same
      version and erase us anyway. Released with `defer` so every error path
      gives it back, which `TestLockReleasedAfterFailedBump` checks by taking the
      lock directly after a failed bump rather than inferring it from a second
      run. `--file` mode has no config to anchor to, so the lock goes beside the
      file being bumped — what two concurrent `--file` runs on one target have in
      common.
- [x] Decide, and state, what a contended lock does. Fail fast is the better
      default: a bump takes milliseconds, so a second one arriving mid-run is
      usually a mistake rather than a queue, and a tool that blocks silently
      turns a CI misconfiguration into a hung job instead of a failed one. Exit
      non-zero with a message that says another `incrmit` holds the project and
      what to do about it, and offer opt-in waiting (`--wait`) for the callers
      who really are serializing work.
      Exit `1` (the existing generic-error code) with
      `another incrmit run is already writing in <dir>` plus a line naming
      `--wait`; a new exit code was considered and rejected as public contract
      the milestone did not ask for.
      `-w, --wait` is on all three writing commands and waits indefinitely by
      poll (`lock.AcquireWait` takes a timeout, which the CLI passes as 0 and the
      tests use to assert it gives up when asked).
- [x] Degrade rather than refuse where locking is unavailable. On filesystems
      that do not implement it (some NFS mounts, a few CI overlay filesystems),
      an error that is not contention must warn and continue, because a tool
      that cannot bump at all is worse than one that cannot detect a second run.
      `Acquire` therefore returns `ErrContended` or nothing at all: every other
      failure comes back as a *degraded* `*Lock` that reports why, so "someone
      else holds it" stays distinguishable from "locking is not available" at
      the type level rather than by inspecting errno at the call site. The
      degrade path is covered by putting a directory where the lock file belongs,
      which fails the open the way an unsupported filesystem fails the flock.
- [x] Skip in-flight temp files while scanning. `WriteAtomic` writes
      `.incrmit-*.tmp` beside its target, and the walk has no dotfile rule and
      no such entry in `ignoredDirs`, so a concurrent — or previously
      crashed — run's temp file can be scanned and written into the generated
      config as a real target. Exclude the pattern in `discovery`, and sweep
      stale ones the next time the project is locked, which is the one moment it
      is provably safe to delete them.
      The pattern now has one definition (`files.IsTempName`, next to the
      `CreateTemp` call that produces it) which both the walk and the sweep use,
      with a test asserting the two cannot drift. `files.SweepTemps` is
      non-recursive and the CLI calls it with exactly the directories the command
      is about to write in; it is skipped entirely when the lock is degraded,
      since without a real lock a matching file may belong to a run still writing.
      The walk also skips `.incrmit.lock` by name.
- [x] Make `undo` verify before it reverts, since a lock only serializes runs
      that use it. Refuse when a file no longer holds the version the journal
      recorded — another run may have bumped past it — and say which file
      diverged instead of writing an older version back over newer work.
      The check already existed from Milestone 22 (`planReverts` fails before
      phase 2 when the recorded `new` token is gone) and already named the file;
      what was missing was a test for the concurrent case rather than the edited-
      by-hand one, so `TestUndoRefusesAfterAnotherRunMovedPast` drives a `--file`
      bump — the kind that keeps no journal — past a recorded bump and asserts
      the refusal leaves the newer version in place.
- [x] Cover the new behavior with tests that a serialized implementation cannot
      pass by accident: the reproduction cases above must now come out
      consistent, a contended second run must fail cleanly without having
      written anything, and the lock must be released after an error partway
      through a bump. Document the model in `README.md` and
      `doc/DEVELOPMENT.md`: one writer per project at a time, readers
      unsynchronized.
      The case a serialized implementation cannot fake is the `--wait` one:
      eight queued bumps must land eight patch increments *and* eight journal
      entries, not one of each. Alongside it: contention writes nothing (file,
      config, and journal all checked), `undo` and `discover` contend the same
      way, the read-only commands run while the lock is held, four `--file` runs
      serialize, and an unavailable lock warns and proceeds. `internal/lock` has
      its own unit tests (contention, per-directory scoping, idempotent release,
      wait timeout and wake-up, degraded acquisition). Documented in README
      ("Concurrent runs"), `doc/DEVELOPMENT.md` §6.4 and §8.6, and the man page's
      CONCURRENT RUNS section. Suite passes under `-race`; coverage 95.8% ->
      95.9%.

## Milestone 30 — Fuzz Testing

The suite covers 95.8% of statements and still has a blind spot: every test
feeds input someone thought to write down. `incrmit` rewrites other people's
files in place, so the failure that matters most is a scanner or rewriter bug
that eats bytes around the version token — and the "only the token changed"
promise is currently checked against four handcrafted fixtures. Go's built-in
fuzzing exercises the same invariants against input nobody imagined.

- [x] Add `FuzzSetKnownVersions` in `internal/files` asserting the invariant the
      whole tool rests on: for arbitrary input bytes and a set of replacements,
      every byte outside the replaced ranges is identical to the input. Compare
      ranges rather than reusing `assertOnlyVersionChanged`, whose
      `strings.ReplaceAll` round-trip can mask an error when the same token
      appears more than once. Assert the returned counts match the replacements
      actually made, and that a replacement never produces an output containing
      a token the input did not have.
      The byte check searches for an alignment of input and output that uses
      only two moves — copy one identical byte, or consume a pin's old token
      against its new one — over a memoized (i, j) state space. The search is
      exhaustive, so a failure means some byte outside a token really changed
      rather than that the check failed to guess the decomposition; inputs large
      enough to make the search expensive fall back to the cheaper properties.
      The counts are tied to the output two ways that do not depend on where any
      replacement landed: the output's length must equal the input's plus the
      per-token deltas, and no token may be replaced more times than its text
      occurs in the input. `assertNoInventedTokens` allows a new version's
      numeric core as well as its full token, since a pin whose suffix was
      consumed inside a filename leaves the core behind.
      The "never produces a token the input did not have" half turned out to be
      stated more strongly than the rewriter can promise, and the fuzzer said so:
      a new token can weld to bytes the scanner left behind when a token is
      followed by something its grammar cannot absorb but a shorter token can
      (`0.0.0+H+0` is the version `0.0.0+H` with `+0` left over, so a pin
      rewriting it to `0.0.0` yields `0.0.0+0`). Neither string is a valid
      version, and the file ends up reporting one the config does not pin, which
      the next command refuses rather than acts on — so the case is documented
      in `doc/DEVELOPMENT.md` §12.1 and kept in the regression corpus, and the
      assertion allows exactly that weld: a token beginning where a new token
      was written and running past its end. It still catches a version conjured
      anywhere else, and a token written but scanned back short — which is the
      shape the trailing-hyphen defect took.
- [x] Add `FuzzFindTokens` in `internal/version` checking that the returned
      ranges are in bounds, strictly ordered, non-overlapping, and that the
      bytes each range spans parse with `version.Parse` — the property the
      rewriter assumes when it walks the ranges in one pass.
      The last of those is not the property the package actually has, and
      asserting it would have been asserting a bug: `FindTokens` deliberately
      reports candidates for `Parse` to reject, which is how an IPv4 address is
      rejected whole instead of having a three-component slice pulled out of it.
      What the rewriter assumes is the weaker statement, and it is what the
      target checks: a range `Parse` *accepts* spans exactly the bytes
      `String()` produces, because that equality is how `matchAt` decides an
      occurrence is the pinned version.
- [x] Add `FuzzParse` in `internal/version`: `Parse` never panics on arbitrary
      input, and anything it accepts round-trips through `String()` back to the
      same token (prefix, prerelease, and build sections included). Feed the
      corpus the near-miss forms already in the table tests (`rev1.2.3`,
      IPv4 addresses, leading zeros, empty identifiers) so the fuzzer starts
      from the known boundaries rather than rediscovering them.
      The near-miss seeds earned their place immediately: the leading-zero forms
      failed the round trip on the first run, before any fuzzing engine was
      involved. The target also asserts the two halves of the package agree on
      where a token ends — `FindTokens` must locate an accepted token whole —
      which is the property that caught the trailing-hyphen case. Both are the
      same statement in the end: a version the tool accepts must be one it can
      find again in a file.
- [x] Add fuzz targets for the two remaining parsers of untrusted text:
      `cli.parseSize` (arbitrary strings must return an error, never panic or
      overflow) and config loading (arbitrary bytes must be reported as a config
      error, never panic — the config is trusted input, but a corrupt file is
      not the same as a hostile one).
      `FuzzParseSize` checks the accepted values as well as the rejected ones:
      a size must be non-negative and must survive `formatSize` and back, which
      is what the flag prints as its default and what error messages quote.
      `FuzzFormatSize` drives the same round trip from the number. `FuzzLoad`
      writes the bytes to a real config in a temp directory next to a real
      target, so validation can succeed and the fuzzer reaches the code past it;
      every error must carry the `config:` prefix the CLI keys its exit codes
      off, and a config that loads must marshal and reload identically, because
      a bump rewrites the config it just read.
- [x] Seed each target with a corpus under `testdata/fuzz/` covering the shapes
      the table tests already know matter, and commit any input the fuzzer finds
      as a regression case so a fixed crash stays fixed.
      Split by purpose rather than put both in one place. The table-derived
      shapes are `f.Add` calls sitting next to the invariant they exercise,
      where a reviewer reads them with the property instead of as a directory of
      one-line files. `testdata/fuzz/<Target>/` holds the regression corpus, one
      file per input a run has found, named for what it proves
      (`leading_zero_in_core`, `trailing_hyphen_in_build`,
      `printed_byte_count`). The go command treats both as seed corpora, so
      `go test ./...` replays all of them as ordinary subtests.
- [x] Wire fuzzing into the workflow in two places: `go test ./...` already runs
      every seed corpus entry as a unit test, so make sure the seeds alone catch
      the known cases, and add a `make fuzz` target that runs each target for a
      bounded `-fuzztime` (e.g. 30s) for local use. Decide and document whether
      CI runs a short fuzz pass on every push or a longer one on a schedule —
      a fixed `-fuzztime` in the existing test job is simplest, but note that
      fuzzing is non-deterministic, so it belongs in its own job rather than
      gating the build/test job on a random failure.
      The seeds do catch the known cases: two of the three defects below were
      reported by `go test` alone. `make fuzz` enumerates every target with
      `go test -list '^Fuzz'` and runs each for `FUZZTIME` (30s by default), so
      a new target is picked up without editing the Makefile; it is not part of
      `make check`. CI runs the same bounded pass on every push in a job of its
      own — short and frequent, because the value is catching a broken invariant
      while the change that broke it is still in front of the author, and
      separate, because a non-deterministic failure must never decide whether
      Build & Test is green. The job prints any input it found before exiting,
      since `testdata/fuzz/` in a runner's workspace is discarded with it.
- [x] Fix whatever the fuzzers find before moving on, and record in
      `doc/DEVELOPMENT.md` what each target proves — the invariants above are
      the actual specification of the rewriter, and they are worth stating in
      prose next to the code they constrain.
      Three defects, all the same shape: a token the tool could write or accept
      but never find again. `Parse` accepted leading zeros in the numeric core,
      so `1.02.3` read as `1.2.3`, the rewriter searched for the text `1.2.3`,
      found nothing, and returned the file unchanged with no error — the bump
      printed `1.2.3 -> 1.2.4` over a file that still said `1.02.3`. `Parse`
      also accepted a token ending in `-` (`1.2.3+0-`), which semver permits but
      the scanner's trailing `\b` cuts short, so a config could pin a token
      `FindTokens` can never locate. Both are now rejected, which turns a silent
      no-op into an exit-3 "no semantic version found" naming the file. Third,
      `formatSize` printed `1234 bytes` for a size with no whole unit and
      `parseSize` refused that spelling, so the default the flag showed could
      not be pasted back; `parseSize` accepts it now. Written up in
      `doc/DEVELOPMENT.md` §12.1, with the user-visible half in `README.md` and
      `CHANGELOG.md`.

## Milestone 31 — Pathological File Shapes

Every fixture in the suite is a file a person would sit down and type: a handful
of lines, LF endings, a trailing newline, ASCII. Real trees hold files that are
none of those, and `incrmit` rewrites them in place. The shapes most likely to
break the "only the version token changed" promise are the ones no test names —
CRLF endings, a leading BOM, no trailing newline, a token at byte 0 or flush
against EOF, one enormous minified line. Fuzzing (Milestone 30) generates
*bytes*, but not file shapes: it never creates a hard link, a setuid bit, or a
read-only file. Those need fixtures.

- [x] Cover line endings and encodings, asserting the file keeps its shape
      rather than being normalized: CRLF throughout, mixed CRLF and LF, a lone
      CR, and a UTF-8 BOM before the first key (the BOM must survive and must
      not shift the token ranges). Add the two encodings that are not text as
      far as the scanner is concerned — UTF-16, whose NUL bytes split every
      token, and Latin-1 bytes with no NUL, which is not caught by `isBinary` —
      and pin what each does today so a "no version found" on a UTF-16 file is a
      documented answer rather than a surprise.
      The rewriter already kept every shape: it only interprets the bytes of a
      token and the word boundaries around it, so `\r`, the BOM, and Latin-1
      bytes are all just non-word bytes. Each expected output is built from the
      same template as its input and compared byte for byte, rather than by a
      `strings.ReplaceAll` revert. UTF-16 is pinned in both halves: a scan
      reports `ErrNoVersion`, a pinned version `ErrVersionNotFound` (both exit
      3, and the file is not written), and discovery skips it as binary.
      Latin-1 is scanned as text and passes through untouched. Discovery was
      where the lone CR went wrong (below).
- [x] Cover the boundary positions the code special-cases: a file that is
      exactly `1.2.3` with no trailing newline, which puts the token at byte 0
      and flush against EOF and so takes the `start < 2` branch in
      `suffixBelongs` and the `after < len(data)` check in `matchAt`; a token as
      the final byte of a longer file; and an empty or whitespace-only file,
      which must report `ErrNoVersion` rather than panic.
      The `after < len(data)` check is only reached when a pinned suffix is
      consumed past a guard-cut core, so the table adds `app-1.2.3-rc.1` flush
      against EOF (and `-rc.10` there, which must not match), plus
      `-1.2.3-rc.1`, whose token starts at byte 1 — the case where the
      `start < 2` guard is what keeps `data[start-2]` in bounds. Blank files
      include a lone BOM and bare `\r`; every entry point, `ApplyBump`
      included, reports no version and writes nothing.
- [x] Cover files with no line structure at all: minified JSON on one very long
      line, and a file holding thousands of occurrences of the same version.
      Assert both the replacement counts and byte preservation. Keep the largest
      case behind `testing.Short()` if it measurably slows `go test ./...`.
      The rewriter is linear and passed; the cost is `FindTokens` at about
      390 ns a token. 500,000 occurrences took 12 s under `-race`, which is
      what CI runs, so the sizes are kept to what still catches a quadratic
      regression: a ~625 KB one-line manifest and 100,000 packed tokens
      normally, and 2,000 dependencies and 5,000 tokens under `-short`.
      Discovery did not pass — see the last item.
- [x] Cover the metadata shapes, which is where the atomic write's design shows
      through. A read-only `0444` file bumps successfully, because the write is
      a rename in the parent directory rather than a write through the file, and
      the mode survives. A setuid, setgid, or sticky file loses those bits,
      because `WriteAtomic` copies `Perm()` only. A hard-linked file has its
      link broken by the rename, so the other name keeps the old contents. Each
      is a deliberate consequence, not a bug — decide, test, and state them in
      `WriteAtomic`'s doc comment the way the symlink behavior already is.
      All three kept as they are. Dropping setuid/setgid is the safe answer:
      re-applying it would mint a setuid file owned by whoever ran incrmit,
      which is why the kernel clears those bits on an unprivileged write
      anyway. Keeping a hard link would mean rewriting the shared file in
      place, the partial write the rename exists to prevent; a config listing
      every name still bumps them all, since planning reads every target first
      (tested). Ownership, xattrs, and ACLs belong to the same list and are
      stated but not tested, since that needs root. The special-bit test uses
      `os.ModeSetuid` and friends, not `0o4755` (`os.Chmod` ignores bits above
      `Perm()` in a plain octal), and skips a bit the system will not set.
- [x] Cover awkward paths, since the target is whatever the user names: spaces,
      a newline, non-ASCII characters, a leading dash, glob metacharacters
      (`*`, `[`, `?`), and a name near the OS length limit. The metacharacter
      case matters twice — a `--file` argument must be taken literally end to
      end, while the same characters from `ignore` in the config are patterns.
      Every name works through `--file NAME`, `--file=NAME`, and `-f NAME`
      given relative to the working directory, including one called `--major`,
      with decoys (`vX.txt` for `v*.txt`, `a.txt` for `[ab].txt`) left
      untouched. A `discover` → bump round trip keeps each name exactly through
      TOML's escaping. A 255-byte name works because the temp file's name does
      not derive from the target's. One limit surfaced on the `ignore` side: a
      backslash cannot escape a metacharacter, since config loading turns every
      backslash into a slash, so brackets (`v[*].txt`) are the only way to
      match one literally. Documented and tested rather than changed; the
      normalization is what makes a Windows-authored config portable.
- [x] Add golden fixtures for the readable shapes (CRLF, BOM, no trailing
      newline, minified single line) in `internal/files/testdata`, alongside the
      four format fixtures already there. A golden diff is the clearest
      statement the rewriter can make about leaving everything else alone.
      `Directory.Build.props` (CRLF), `appsettings.json` (BOM),
      `setup.cfg` (version flush against EOF), and `package.min.json`. The
      repository's `* text=auto` would have stored the CRLF fixture as LF,
      leaving a golden test that passed while testing nothing, so
      `.gitattributes` marks the fixtures `-text`, and
      `TestShapeFixturesKeepTheirShape` fails if a fixture ever loses its
      shape anyway.
- [x] Cover the discovery side of the same shapes: a zero-length file, a deeply
      nested tree, and a file whose only version sits past a NUL byte, so it is
      skipped as binary. Confirm a file whose size cap falls mid-token is
      refused whole rather than scanned truncated.
      The size check refused it, but the second guard did not: a file that grew
      between the stat and the read went through `io.LimitReader(f, max)` and
      was scanned truncated, so `1.2.34` could be recorded as `1.2.3`.
      `readAtMost` now reads one byte past the cap and refuses the file if that
      byte arrives. The tree test is 128 levels deep, with a directory-only
      ignore pattern pruning halfway down.
- [x] Fix what turns out to be wrong and document what turns out to be merely
      lossy. Where a shape cannot round-trip (a dropped setuid bit, a broken
      hard link, a UTF-16 file that reads as versionless), say so in `README.md`
      and `doc/DEVELOPMENT.md` — a documented limit is a feature, an undocumented
      one is a bug report waiting to be filed.
      Three defects, all in discovery; the rewriter needed nothing. The scan was
      quadratic on a long line: each occurrence recounted lines from the start
      of the file and copied its whole line as context, so 20,000 occurrences
      in a 620 KB minified file took 3.8 s and allocated about 12 GB (and the
      dry run would have printed the file 20,000 times). A forward-only
      `lineCursor` makes it one pass, and context is clipped to 80 bytes each
      side of the token on a UTF-8 boundary. Lines ended only at `\n`, so a
      lone-CR file was one line whose carriage returns reached the terminal
      and overprinted the dry run; `\r\n` and a lone `\r` now end a line too,
      and a leading BOM is left out of the context. The third is the size-cap
      race above. `discovery.FuzzScan` checks the cursor against a reference
      line count. The lossy shapes are in `README.md` ("What a bump keeps"),
      `doc/DEVELOPMENT.md` (§9.1, the atomic-write consequences, and a new §9.4
      File shapes), the man page's CAVEATS, and `CHANGELOG.md`.

The same work showed that file shapes reach the terminal as well as the disk.
The lone-CR fix stopped one control character from reaching the dry run, but
any other still gets through: `incrmit` prints file names and file contents it
did not write, and `discover --dry-run` is meant for exactly the trees a user
does not own. An `ESC` sequence in a scanned file can clear the screen and hide
output, retitle the window, plant a fake hyperlink, or — on terminals that
honor OSC 52 — write to the clipboard. The same bytes also reach CI logs, which
render ANSI.

- [x] Add one function in `internal/cli` that renders untrusted text for the
      terminal, so that every character printed is either printable or a
      visible escape. Escape the C0 controls (`ESC`, `CR`, `BS`, `BEL`, ...),
      `DEL`, the C1 controls (U+0080–U+009F, and the raw byte `0x9B`, which some
      terminals read as a one-byte CSI), invalid UTF-8, and the bidirectional
      overrides and isolates (U+202A–U+202E, U+2066–U+2069) that make a line
      display in a different order than its bytes. Printable non-ASCII (`é`,
      `版本`) passes through unchanged. Use Go-style escapes (`\x1b`, `\u202e`)
      so the output matches what the `%q` sites already print. Two decisions
      to make and record: whether a backslash is escaped (unambiguous output
      wants `\\`, but Windows paths would print doubled; escaping it only in a
      string that needed another escape is one answer), and whether a tab in
      context text is shown as a space rather than as `\t`.
      It became two layers in `internal/cli/display.go` rather than one
      function, because names and text want different things. `displayName`
      renders a name unchanged when every character is printable
      (`strconv.IsPrint`) and Go-quoted otherwise — the same predicate and
      escapes `%q` already uses, so bidi and zero-width characters are covered
      with no hand-kept list. `terminalWriter` wraps stdout and stderr in
      `Main` and escapes in place whatever still arrives, and it is the actual
      guarantee: it also catches the `flag` package, which echoes an unknown
      flag raw, and any print site written later. The backslash question was
      settled by the leading quote: a quoted rendering always starts with `"`,
      an unquoted one never does (a printable name beginning with `"` is
      quoted too), so Windows paths print as typed and the rendering is still
      injective. Tabs are kept rather than turned into spaces: a tab only moves
      the cursor forward, and it is how Makefile context is indented. A tab in
      a name is still quoted. Escaping is always on, TTY or not, because CI
      logs are not a terminal and render ANSI all the same.
- [x] Apply it at every site that prints bytes `incrmit` did not produce: the
      `discover --dry-run` context, every path (discovered, listed in the
      config, recorded in the journal for `undo`, and the `--path`, `--output`,
      and `--file` values), the `(ignoring: …)` echo, the `preview` table, and
      error messages. That includes OS errors: an `*fs.PathError` embeds the raw
      path, and `fsErrorMessage` prints its default branch with `%v`. The `%q`
      sites escape through `strconv` already and need no change. In `preview`,
      measure column widths on the escaped text so the table still lines up.
      Every name argument in `cli.go`, `preview.go`, and `projectlock.go` goes
      through `displayName`, and `fsErrorMessage` renders the name itself, so
      its callers pass the raw path. It also drops an `*fs.PathError`'s own
      path, wrapped or not, which repeated the name raw ahead of the reason
      (`reading X: stat X: file name too long`). Context lines and wrapped
      error text are left to the writer, which escapes in place without
      quoting.
- [x] Escape for display only. The name used to open, lock, and rename a file,
      and the name written to the config and the journal, must stay the raw
      bytes. TOML already escapes control characters in a string, so a name
      holding `ESC` round-trips exactly. Test that such a file is bumped, is
      shown escaped, and is still recorded under its real name.
      True for control characters, tabs, newlines, and bidi characters, and
      checked in the config and the journal. It is false for a name that is not
      UTF-8, which Linux allows: TOML strings must be UTF-8 with no escape for
      a raw byte, and the encoder wrote the byte as-is, so `discover` produced
      a config every later command refused to load. `discover` now skips such
      a file with a warning (`excludeUnlistable`), and `discovery.Generate`
      refuses one as a backstop. The end-to-end case runs only where the file
      system accepts such a name, so on Linux (CI) but not macOS; unit tests
      cover both functions everywhere.
- [x] Add an end-to-end regression test over a hostile tree: file names and
      contents holding `\x1b[2J`, `\x1b]0;title\x07`, an OSC 8 link, `CR`,
      `BS`, `BEL`, `0x9B`, UTF-8 C1 controls, bidi overrides, and invalid
      UTF-8. Run every command against it — `discover` with and without
      `--dry-run`, a bump and its dry run, `preview`, `undo` — plus the failure
      paths that print a path (unreadable target, version not found, conflicted
      undo). Assert that nothing on stdout or stderr holds a control character
      other than `\n`, or a bidi control. This test, rather than a review, is
      what catches a print site added later without the escaping.
      `internal/cli/hostile_test.go`, with tab allowed as well as `\n` per the
      decision above. With the writer in place a forgotten site is still safe,
      so the test also asserts that every hostile name appears only in its
      quoted form, which is what actually finds the site: dropping
      `displayName` from undo's summary fails it, and so does removing the
      writer. Text that `incrmit` does not format (the `flag` package's
      message, an OS error for a `--path` it cannot walk) is checked for safety
      only.
- [x] Add a fuzz target for the escaping function: for arbitrary bytes, the
      output is valid UTF-8 with no control or bidi character, printable input
      comes back unchanged, and distinct inputs never render the same (so the
      escaping cannot make two different names look alike).
      Two targets, one per layer. `FuzzDisplayName` proves injectivity by
      giving it a left inverse: an unquoted rendering is the name itself, and
      a quoted one reads back with `strconv.Unquote` to exactly the name.
      `FuzzTerminalText` checks that the output is valid UTF-8 holding only
      printable characters, newlines, and tabs, and that it is unchanged
      exactly when the input was already safe. About 6M inputs between them,
      nothing found.
- [x] Document the behavior in `README.md` (discovery and the dry run: names
      and context are shown with control characters escaped), in
      `doc/DEVELOPMENT.md` next to the scan boundaries, in the man page, and in
      `CHANGELOG.md`.
      `README.md` (Discovery, plus the UTF-8 skip among the scan boundaries),
      `doc/DEVELOPMENT.md` §9.5 Terminal output with the decisions above, the
      man page's CAVEATS, and a Security entry in `CHANGELOG.md`.

## Milestone 32 — Symlinked Lock File

`lock.Acquire` opens `.incrmit.lock` with `O_CREATE|O_RDWR`, which follows a
symlink, and `writeNote` then truncates whatever it opened and writes the lock
note into it. A repository can commit `.incrmit.lock` as a symlink, so cloning
one and running `incrmit discover` (which needs no config at all) or a plain
bump replaces the contents of any file the user can write with two lines of
lock text. Reproduced: `.incrmit.lock -> ../victim.txt` left `victim.txt`
holding the note after a `discover`, and again after a bump. A dangling link
does damage of its own: `O_CREATE` follows it and creates the file it names.
Milestone 26 closed the same hole on the read side of discovery; the lock is
the one place incrmit opens an existing path for writing instead of replacing
it by rename.

- [x] Reproduce the damage before fixing it, as Milestone 29 did, so the fix
      has something to prove: plant `.incrmit.lock` as a symlink to a file
      outside the project, run `discover`, a bump, and `undo` through
      `cli.Main`, and assert that file's bytes and mode are unchanged. Add a
      dangling link whose target must still not exist afterward.
      `internal/cli/lockfile_test.go` builds the project one level down in a
      sandbox and snapshots everything outside it (type, mode, and contents or
      link target) before and after each command. Before the fix all three
      commands left `victim.txt` holding the note at its original `0640`, the
      dangling link created `created-by-incrmit` at `0600`, and a regular file
      of unrelated text at the lock path came out as the note. As in
      Milestone 29 the assertion is the invariant (nothing outside the project
      changed), so the same tests prove the fix.
- [x] Open the lock file without following a symlink, so the check and the
      open are one step rather than an `Lstat` followed by a racing `Open`:
      `O_NOFOLLOW` in `lock_unix.go` and `FILE_FLAG_OPEN_REPARSE_POINT` in
      `lock_windows.go`, behind a build-tagged `openLockFile` that mirrors
      `tryLock`. Then `Stat` the descriptor and accept only a regular file.
      The Unix open also passes `O_NONBLOCK`, so a FIFO or a device cannot
      block it. The Windows side calls `windows.CreateFile` with `OPEN_ALWAYS`
      and the sharing mode `os.OpenFile` uses, since `os.OpenFile` cannot pass
      the flag, and wraps the handle with `os.NewFile`. When the open fails, an
      `Lstat` only words the warning: nothing is opened after it, so it has
      nothing to race. All six release platforms build and vet; the Windows
      path is compiled but not run here.
- [x] Decide what a lock path that is not a regular file does: a symlink, a
      directory, a FIFO. Recommend a degraded lock, keeping Milestone 29's
      rule that only contention refuses: warn naming the path and why ("is a
      symbolic link"), continue unlocked, and sweep nothing. Nothing is opened
      through the link either way, and the warning is what points the user at
      a planted file. Record the decision and the reason.
      Degraded, as recommended. The reason is a `lock.NotRegularError` inside
      an `*fs.PathError`, and the CLI now builds the warning with
      `fsErrorMessage`, so it reads
      `cannot lock .incrmit.lock: is a symbolic link, not a regular file` and
      other lock failures name the file once too. The planted file is left
      exactly as it was. Refusing was rejected because it protects nothing
      more (nothing is opened through the path either way) and would make a
      stray file a reason no bump can run. Recorded in `doc/DEVELOPMENT.md`
      §6.4.
- [x] Stop truncating. Write the note only into a file that was empty when the
      lock was taken, which in practice means one this run just created, so
      even a regular file that happens to sit at that name is never rewritten.
      The note is a courtesy and must never be the reason a file loses data.
      `writeNote` checks the held descriptor. It also requires the file to have
      exactly one name (`Nlink` on Unix, `NumberOfLinks` from
      `GetFileInformationByHandle` on Windows): an empty file hard-linked from
      elsewhere is otherwise "empty", and writing the note into it changes the
      other name. This matters for `--file` in a shared directory, where the
      lock sits beside the target. Removing either check fails a unit test.
- [x] Confirm the lock is the only place incrmit writes through an existing
      path: `WriteAtomic` renames over a link rather than writing through it,
      and `SweepTemps` removes a link named like a temp file rather than its
      target. Add a test for the sweep case, a `.incrmit-x.tmp` symlink to a
      file outside the tree, which must be removed with its target untouched.
      Confirmed by audit. Outside the lock, the only filesystem writes are
      `CreateTemp` (a new, unpredictable name with `O_EXCL`), `Chmod` through
      the temp file's descriptor, `Rename`, and `Remove`. `WriteAtomic`'s
      `Stat` of the target follows a link but only reads the mode.
      `TestWriteAtomicDoesNotWriteThroughSymlink` already covered the rename.
      `TestSweepTempsRemovesLinksNotTargets` covers the sweep with a link to a
      file and a link to a directory: both links go, and both targets keep
      their contents and mode.
- [x] Cover the other shapes on Unix, skipping where the system will not
      create them: a symlink to a directory, a FIFO at the lock path, and a
      regular file holding unrelated text, which must come out unchanged. In
      every case assert the command behaves as decided above and that nothing
      outside the project changed.
      Each shape runs `discover`, a bump, and `undo` under a 10-second
      deadline, plus a directory at the lock path, which now gets a warning
      that says it is a directory instead of the raw `EISDIR`. On macOS a FIFO
      opened `O_RDWR` without blocking and the old code failed at `flock`
      with "operation not supported". It is now rejected by the descriptor
      check before any lock is tried. `internal/lock` has matching unit tests
      (link, dangling link, FIFO, existing text, empty file, hard link, and
      the wording for each file kind). Suite passes under `-race` and
      `make check`; coverage 96.5% -> 96.5%.
- [x] Document the behavior in `README.md` ("Concurrent runs"),
      `doc/DEVELOPMENT.md` §6.4, and the man page's CONCURRENT RUNS section,
      and add a Security entry to `CHANGELOG.md`.
      §8.6's "degrades, it does not refuse" bullet now lists a non-regular lock
      path too. The CHANGELOG entry opens a new `[Unreleased]` section, with a
      Fixed line for the lock warning naming the file once.

## Milestone 33 — File-Type Checks for the Config, Ignore List, and Journal

Milestone 26 routed every *target* read through `files.ReadTarget`, which
checks the file type before opening, because opening a FIFO blocks until a
writer appears and `/dev/zero` never ends. The files incrmit reads for itself
were left out: `config.Load`, `config.LoadIgnore`, and `history.Load` all call
`os.ReadFile` directly. Reproduced with a FIFO: `preview` and
`discover --dry-run` hang on one at `incrmit.toml`, and those are the two
commands meant to be safe on an unfamiliar tree; `undo` hangs on one at
`.incrmit.state.toml`. A bump hangs there too, and worse, it hangs *after*
rewriting the targets and the config, because the journal is read last, so
killing it leaves a bump that `undo` has no record of. A repository cannot
commit a FIFO, but it can commit `incrmit.toml -> /dev/zero`, which reads
without end.

- [x] Write the failing tests first, each under a deadline so a regression
      fails rather than hanging the suite (the pattern
      `TestBumpNonRegularFileTarget` already uses): a FIFO at the config for a
      bump, `--dry-run`, `preview`, and `undo`; at the `--output` path for
      `discover` with and without `--dry-run`; and at the state file for a
      bump and `undo`. Create them with `testutil.Mkfifo`.
      `TestOwnFilesNotRegular` in `internal/cli/ownfiles_test.go` runs eleven
      commands under `runMainWithin`: those listed, plus `undo --dry-run` at
      the config and `--dry-run` and `undo --dry-run` at the state file. Each
      asserts exit `1`, a message naming the file, and an unchanged snapshot
      of the project. Before the fix ten hung for the full 10 seconds, and
      `--dry-run` over a FIFO journal exited `0` because it never read it.
      `TestBumpReadsJournalBeforeWriting` needs no FIFO: over a corrupt
      journal the old bump left `VERSION` at `1.0.1`, the config rewritten,
      and no entry to undo.
- [x] Route all three reads through one checked helper, `files.ReadTarget` or
      a sibling for tool-maintained files, so one function decides that a
      file is safe to open. A symlink to a regular file is still followed, as
      it is for targets, since a symlinked config is a legitimate setup; a
      link to a device or a pipe fails the same type check.
      `files.ReadOwnFile` is `ReadTargetWithLimit` at a fixed cap, so
      `ReadTargetWithLimit` stays the one function that decides.
      `config.Load`, `config.LoadIgnore`, and `history.Load` call it. Links
      to a regular file are followed (tests in `files`, `config`, and `cli`
      through `preview`, `--dry-run`, and `discover --dry-run`). The unit
      tests share a new `testutil.Within` deadline. Separately, a bump still
      replaces a symlinked config with a regular file when it rewrites it,
      as `WriteAtomic` does for every path; this item is only the read.
- [x] Cap the size of these reads. The config and the journal are small, and
      the journal holds at most `history.MaxEntries` entries, so a fixed cap
      in a named constant is enough; `--max-file-size` is about targets and
      stays that way. Report an oversized file as an error that names it.
      `files.MaxOwnFileBytes` is 16 MiB, reported as
      `config: reading "incrmit.toml": files: file is 16777217 bytes, over the
      16777216 byte limit` (`history: ...` for the state file), exit `1`.
      Sized by measurement: decoding TOML allocates about 20 times the file,
      so the cap bounds a planted file at about 330 MiB. But a journal change
      costs about 196 bytes, so twenty full entries reach 16 MiB at about
      4,300 targets, and a fixed read cap alone would have failed such a
      project's seventeenth bump. `history.Save` therefore drops the oldest
      entries until the encoding fits (an entry over the cap on its own is an
      error), so incrmit never writes a journal it would refuse to read.
      Tested through `saveWithin` with a small cap.
- [x] Decide what `LoadIgnore` does with a `--output` that is not a regular
      file. It is lenient today (a missing or unparseable file yields no
      patterns), but a FIFO or a device is not a stale config, and `discover`
      would go on to replace it. Recommend an error with exit `1`, decided
      together with Milestone 36's rule for what `--output` may overwrite.
      An error with exit `1`, as recommended, before the scan and with nothing
      written; `--dry-run` refuses the same way. The leniency now covers only
      a missing or unparseable file (Milestone 36 retires the second). This
      fits Milestone 36's rule: `--output` may replace a missing file, an
      empty file, or an incrmit config, and a FIFO or a device is none of
      those. Its check can sit behind the same `ReadOwnFile` read.
- [x] Read the journal before phase 2 of a bump instead of after the writes,
      so a state file that cannot be read fails the bump with nothing
      written. Milestone 38 moves the journal *write* ahead of phase 2 too;
      this item is only the read.
      `runBump` loads it right after planning, and `recordHistory` pushes
      onto that copy instead of reading the file again. `--dry-run` reads it
      too, so the dry run predicts the failure the real run would hit.
      `TestBumpUnreadableStateFileFailsAfterWriting`, which pinned the old
      order and said to update it when the read moved, is now
      `TestBumpUnreadableStateFileWritesNothing`.
- [x] Add a symlink to `/dev/zero` as the config and as the state file (Unix
      only), and assert each command fails promptly with a "not a regular
      file" message and exit `1`.
      `TestOwnFilesNotRegular` runs the same eleven commands with the link in
      place of the FIFO, skipped on Windows. This half was not run against the
      old code, which reads `/dev/zero` until memory runs out rather than
      until the deadline. `TestOwnFilesOverSizeCap` runs the table again over
      a sparse file one byte over the cap; `snapshotOutside` now records a
      file over 64 KiB by size and SHA-256 so the snapshot holds no copy of
      it. Suite passes under `-race` and `make check`; coverage 96.5% ->
      96.4%, the gap being `history.encode`'s unreachable encoder error.
- [x] Document the checks next to the other read boundaries in `README.md`,
      `doc/DEVELOPMENT.md`, and the man page's CAVEATS, and add a Security
      entry to `CHANGELOG.md`.
      The README's reference material moved to `doc/USAGE.md` in 0.3.4, so
      the checks are there, beside "A target must be an ordinary file", and
      in its Undo section. `doc/DEVELOPMENT.md` covers them in §6.2 (journal
      size), §8.1 and §8.2 (the flows), §9.1, and §10; the man page in CAVEATS
      and FILES. `CHANGELOG.md` opens `[Unreleased]` with two Security
      entries and a Fixed one for the journal read.

## Milestone 34 — Undo from Config-Relative Paths

Each journal change records the path as the config lists it (`path`) and the
absolute path it resolved to (`fs`), and each entry records the config's
absolute path (`config`). `undo` acts on the absolute ones, which ties the
journal to the directory the bump ran in rather than to the project. After
`cp -R orig copy`, running `undo` in `copy` reverted `orig/VERSION` and
`orig/incrmit.toml`, left `copy` at the bumped version, printed
`VERSION: 1.0.1 -> 1.0.0` as though the change were local, and popped the
entry from `copy`'s journal, while `orig`'s journal still records a bump that
has now been undone. It is also the one way incrmit writes to a path no trusted
input named: `incrmit.toml` is documented as trusted, like a Makefile, but a
committed `.incrmit.state.toml` is not, and its `fs` values can point `undo`
at any file the user can write.

- [x] Reproduce first: the copied project (assert `orig` is untouched), the
      moved project (today `undo` fails with "reading VERSION: file does not
      exist" beside a `VERSION` that exists), and a hand-written state file
      whose `fs` names a file outside the project.
      Reproduced with a built binary and then in
      `internal/cli/undopaths_test.go`, which snapshots everything outside the
      project with `snapshotOutside` from the lock tests. Copied: `orig`'s
      `VERSION` and config went to `1.0.0` and the copy stayed at `1.0.1`, as
      reported. Moved: the failure came earlier than the item says, since
      undo loads the recorded config first, and read
      `config: ".../before/incrmit.toml" not found; run incrmit discover`
      beside a config that exists. Planted: `fs` naming `../victim.txt`
      reverted that file, and a planted `config` naming another project's
      config rewrote it. `TestUndoConfigListsParentPath` passed before the
      fix and stays as a guard.
- [x] Resolve every journal path against the directory of the config `undo`
      was given (`-c`, or `incrmit.toml` by default), the same way
      `resolveTargets` resolves config entries for a bump, and write the
      reverted config back to that path rather than to the recorded `config`.
      `undo` still works from any working directory, because it finds the
      state file through the config.
      "The same way" is one function: `config.TargetPath(baseDir, path)`,
      now called by `Validate`, `resolveTargets`, and `planReverts`, which
      also groups by the resolved path instead of `fs`. `runUndo` always
      loads and rewrites the config at `cfgPath`; the `entry.Config != ""`
      branch is gone.
- [x] Stop writing `fs` and `config` into new entries. `path` is already the
      config-relative path, so no new field is needed. Old state files keep
      loading: both keys are ignored on read, so an entry written by an
      earlier version is undone against the current project rather than its
      old location.
      `history.Change.FS` and `history.Entry.Config` are removed, and
      `recordHistory` no longer takes the config path. The TOML decoder
      ignores unknown keys, so no migration code is needed, and the next
      `Save` drops them (`TestLoadIgnoresRetiredKeys`).
- [x] Only undo what the current config names. Before planning, check that
      every change's `path` matches a `[[files]]` entry in the config being
      undone whose version is the change's `new`, and refuse otherwise,
      naming the path, with nothing written. The config is the trusted input,
      so this limits what a journal can reach to what the config could
      already bump. An absolute `path`, or one with `../`, stays allowed
      exactly when the config lists it.
      Refused with exit `1` and
      `VERSION: not listed in incrmit.toml at 1.0.1, the version the last bump
      recorded (refusing to undo)`, `--dry-run` included, before any target
      is read. Paths match as written, the same rule the config rewrite
      uses, so `./VERSION` does not match `VERSION`; a bump writes the
      config and the journal from the same value, so only a hand edit can
      split them. Both lookups key on a `listing` struct rather than the old
      `path + "\x00" + version` string. This also changes one legitimate
      case: a config edited since the bump used to get its files reverted
      and its edited entries left alone, and is now refused.
- [x] Confirm the lock, the state file, and every write now belong to one
      project. The lock is already taken beside the config `undo` was given;
      with the recorded `config` gone, `undo` can no longer rewrite a config
      in a directory it did not lock.
      Confirmed by audit: the lock (`acquireProject(filepath.Dir(cfgPath))`),
      the journal (`history.ResolvePath(cfgPath)`), the config write, and
      every target (`config.TargetPath(filepath.Dir(cfgPath), c.Path)` for a
      listed `c.Path`) derive from `cfgPath`, and nothing is read from the
      journal as a location. A target the config lists outside its directory
      is still written unlocked, exactly as a bump writes it.
      `TestUndoPlantedJournal` covers a planted `config` naming another
      project's config, which is left untouched.
- [x] Update the tests that assert absolute journal paths, and add undo from
      a subdirectory with `-c ../incrmit.toml`, after the project was moved,
      after it was copied, with a config that lists a `../` path, and from an
      old state file that still carries `fs` and `config`.
      `hostile_test.go` checked `filepath.Base(c.FS)`; the `history` tests
      used `Entry.Config` as a label and now label by the change's path. The
      new tests are in `undopaths_test.go`: the subdirectory test also bumps
      from there and asserts the new entry has no `fs` or `config`; the
      `../` test undoes from the parent directory with `-c project/...`; the
      old-state-file test holds two entries naming a directory that is gone
      and undoes both. `TestUndoRefusesWhatTheConfigDoesNotList` covers an
      edited version, a removed entry, and a respelled path. Suite passes
      under `-race` and `make check`, and all six release platforms vet;
      coverage 96.4% -> 96.5%.
- [x] Update the `internal/history` package doc, `doc/DEVELOPMENT.md` (the
      state file format, which says paths are stored absolute), `README.md`,
      and the man page, and add a `Changed` entry to `CHANGELOG.md`.
      `doc/DEVELOPMENT.md` §6.2 (schema, the retired keys, and a "not trusted
      like the config" bullet), §8.1 step 9, §8.4 (new check and resolve
      steps), §9.1, §10, and §12. As in Milestone 33, the README's reference
      material lives in `doc/USAGE.md`, so the change is there (Undo, and the
      trusted-input paragraph); the README says nothing about the journal's
      paths. The man page covers it under `undo` and FILES, and `undo -h`
      gets one sentence. `CHANGELOG.md` has two Changed entries and, beyond
      what the item asks, a Security entry for the planted state file, as
      Milestones 32 and 33 filed theirs.

## Milestone 35 — Integer Overflow on Bump

`Parse` accepts any numeric component `strconv.Atoi` can read, up to
`math.MaxInt64`, and the bump methods add one without checking.
`1.2.9223372036854775807` bumps to `1.2.-9223372036854775808`: the bump
reports success, writes that into the file, and records it in the config, and
the next command finds no version at all and exits `3`. Major and minor wrap
the same way, and so does a numeric prerelease counter in `AdvancePrerelease`,
where the result, `rc.-9223372036854775808`, is legal semver but no longer a
counter: the next `--pre rc` appends `.1` to it instead of counting. `preview`
prints the wrapped values too. It is the defect Milestone 30 fixed three
times, incrmit writing a token it cannot find again, reached through
arithmetic instead of parsing.

- [x] Write table tests at the boundary against the intended behavior, so
      they fail today and pass after the fix: each of major, minor, and patch
      at `math.MaxInt64`, and a prerelease counter at `math.MaxInt64`, through
      a bump, `--dry-run`, `preview`, and `--pre`.
      `internal/cli/overflow_test.go`. `TestBumpRefusesToOverflow` runs twelve
      cases as a bump and as `--dry-run`: each component, `--pre` opening a
      line through each component, and a counter at the ceiling and past it.
      Each asserts exit `3`, the file, component, and token named on stderr,
      nothing on stdout, and the project's snapshot unchanged. With it are a
      `--file` case, a two-file config where only the second overflows, and
      `preview` per component plus a golden. All of them failed before the fix,
      exiting `0` with the wrapped values. `TestBumpUpToTheCeiling` passed
      before and after: one below the ceiling bumps to it and `undo` finds it,
      and a full patch still bumps its minor, so the fix cannot refuse too
      much. The version package has `TestBumpOverflow` and `TestBumpUpToMaxInt`.
      Tests spell the boundary `math.MaxInt`, which is `math.MaxInt64` on all
      six release platforms.
- [x] Decide what an unbumpable version does. Recommend refusing: the command
      exits `3` naming the file and the component, and writes nothing, which
      is how incrmit treats every other version it cannot handle. Clamping
      and wrapping both write a version that is not greater than the one it
      replaced. The `Bump*` methods return no error today; either give them
      one or add checked variants for the CLI, and surface the error through
      `bumpFunc`, whose error path in `planGroups` already fails before any
      write.
      Refused. `BumpMajor`, `BumpMinor`, `BumpPatch`, `AdvancePrerelease`, and
      `BumpPrerelease` now return `(Version, error)`; giving the methods the
      error, rather than adding checked twins, leaves no wrapping version for
      a caller to pick up, and `files.ApplyBump` takes the new signature too.
      The error is `*version.OverflowError{Version, Part}`, which `classify`
      maps to `ExitNoVersion`:
      `incrmit: VERSION: version: cannot bump the patch component of
      1.2.9223372036854775807: incrmit counts only up to 9223372036854775807`.
      Only the incremented number is checked. `componentBump` now returns a
      `bumpFunc`, and `prereleaseBump` picks the component to open a line with
      and then starts the series, so both `--pre` paths share one error
      check. The lock is still taken for a real run, but no file, config, or
      journal is written.
- [x] Make `preview` show a projection that would overflow as unavailable
      rather than as a wrapped number, with the table still aligned.
      The cell reads `n/a`, and a footnote follows the drift one:
      `n/a: that component is already 9223372036854775807, as high as incrmit
      counts`. `previewRow`'s projections are `*version.Version`, nil when the
      bump fails (`projectBump`); the widths are measured after rendering, as
      for the path, so the column still lines up. `preview` still exits `0`,
      since the other columns are real. `testdata/preview_overflow.golden` has
      an `n/a` in every column beside a drifting row.
- [x] Settle the prerelease counter's other edge separately: a numeric
      identifier too large for an `int` has `.1` appended today
      (`rc.99999999999999999999` -> `rc.99999999999999999999.1`) rather than
      being counted. Keep that (it does sort higher) or refuse it like the
      numeric core, and test whichever is chosen.
      Refused, with the same `OverflowError` as a counter at `math.MaxInt`.
      Keeping it only put the break off by one step: `PrereleaseID` of
      `rc.99999999999999999999.1` is `rc.99999999999999999999`, so the next
      `--pre rc` saw a different series and restarted at `rc.1`, below both.
      Refusing also gives one rule for every counter rather than a cliff
      between `rc.9223372036854775807` and `rc.9223372036854775808`. `Parse`
      still accepts the token and `Compare` still orders it; only advancing it
      is refused, and `--pre beta` still starts a new series there. Covered by
      "counter past an int" in `TestBumpOverflow` and "pre counter past an int"
      in the CLI table.
- [x] Add a fuzz target for the property all of this breaks: for any version
      `Parse` accepts, every component bump and every same-series prerelease
      advance that succeeds yields a token that parses back to itself through
      `String()` and that `version.Compare` ranks above the input. Leave out
      switching series, since `rc` to `beta` may rank lower by design. Seed
      it with the `MaxInt64` boundaries.
      `version.FuzzBump`, which also asserts the converse: a bump refuses only
      with an `OverflowError`, and only when its number is at or past the
      ceiling. The test decides that by comparing digit strings, not by
      calling `Atoi` as the implementation does. The seeds are `seedTokens`
      plus ten boundary tokens: each component at `math.MaxInt`, all three one
      below it, a full core with a prerelease, and counters below, at, just
      past, and far past the ceiling. With the check disabled, the seeds fail
      on the first three boundaries; with it, a 60s run found nothing in 26
      million executions.
- [x] Document the limit in `README.md` and in `doc/DEVELOPMENT.md` §12.1 next
      to the other tokens incrmit refuses, and add a `Fixed` entry to
      `CHANGELOG.md`.
      As in Milestones 33 and 34, the README's reference material lives in
      `doc/USAGE.md`, so the limit is there, in a paragraph after the two
      refused spellings, with a `preview` bullet and exit `3` in the table. The
      README does not describe the grammar at all. The man page has the same
      paragraph under VERSION GRAMMAR, plus the `preview` entry and EXIT
      STATUS. `doc/DEVELOPMENT.md` has it in §12.1 after the three defects
      the first fuzzing pass found, and in §7 (preview), §8.1 step 4, §8.5,
      §9.2, §10 and its exit-code table, and §12. `CHANGELOG.md` has two
      `Fixed` entries under `[Unreleased]`, one for the wrap and one for the
      oversized counter. Suite passes under `-race` and `make check`, all six
      release platforms vet, and coverage is 96.5% -> 96.6%.

## Milestone 36 — Discover Overwriting an Unrelated File

`discover -o PATH` replaces whatever is at `PATH`. Regenerating an existing
`incrmit.toml` is the point, but nothing checks that the file being replaced
*is* one: `incrmit discover -o NOTES.md` turned a Markdown file into a
generated config and exited `0`, and a slip such as `-o package.json` would do
the same to a manifest. The read that comes first does not catch it either:
`config.LoadIgnore` is deliberately lenient, so a file that does not parse
yields no ignore patterns rather than an error, and discover carries on as if
it had found a config with nothing to keep.

- [x] Reproduce first: `-o` naming an existing Markdown file, an existing
      target such as `VERSION` (which `excludeOutput` drops from the results
      just before overwriting it), an empty file, and an existing
      `incrmit.toml`, which must still be regenerated.
      `internal/cli/discoveroutput_test.go`. `TestDiscoverRefusesUnrelatedOutput`
      runs six files as a real run and as `--dry-run`: `NOTES.md`, a Markdown
      file of headings alone, `VERSION`, `package.json`, `pyproject.toml`, and
      a config with an extra key in an entry. The tree always holds other
      versions, so a refusal is never just "no version found". All twelve
      exited `0` before the fix, and the real runs left each file holding a
      generated config. `TestDiscoverReplacesConfigOutput` passed before and
      after: no file, an empty file, a blank one, a config discover wrote, a
      hand-written one, an `ignore` list alone, a stale config, and both
      prerelease spellings are regenerated, with the `ignore` list kept.
- [x] Define what `--output` may replace: a path with no file yet, an empty
      file, or a file that parses as TOML and holds only keys incrmit knows
      (`ignore` and `files`). That accepts every config incrmit has written
      and every hand-written one, and rejects a Markdown file, a JSON
      manifest, or a `pyproject.toml`. Keep the list of known keys next to
      `config.Config` so Milestone 43's `[git]` table extends it in one place.
      Empty means nothing but whitespace. Known keys are checked with the TOML
      decoder's `MetaData.Undecoded`, so the fields of `Config` and `FileEntry`
      are the list, at every depth: `files.owner` is refused as well as
      `project`. A new table is declared on `Config` and nothing else changes;
      the comment on `Config` says so. No key has ever been retired from the
      config (git history has only `path`, `version`, `prerelease`, `build`,
      `ignore`, `files`), so no older config is refused. A value of the wrong
      type (`files = 3`, `[files]` as one table) is refused too. One addition
      to the rule: a file that is not empty but sets no key at all (comments
      alone) is refused. It parses as TOML, but so does a Markdown file of
      `#` headings, which is the motivating case, and incrmit never writes
      such a file.
- [x] Refuse anything else with exit `1` before the scan starts, so a mistake
      costs nothing, and say why and what to do: "NOTES.md exists and is not
      an incrmit config; choose another --output or remove the file". No
      `--force` is needed: removing the file is the explicit way to ask.
      `config.LoadIgnore` returns a `*config.NotConfigError{Path, Reason, Err}`
      and `runDiscover` prints `incrmit: NOTES.md exists and is not an incrmit
      config (line 3 is not TOML); choose another --output or remove the
      file`. The other reasons are `it sets "project", which incrmit does not
      use`, `it uses incrmit's keys with values of the wrong type`, `it sets
      nothing, but is not empty`, `it is not a regular file`, and the size
      over the cap. A key from the file is Go-quoted, so one that decodes to an
      escape sequence cannot reach the terminal raw (a case in
      `TestHostileNamesInFailureMessages`). `TestDiscoverRefusesOutputBeforeScanning`
      gives a `--path` that does not exist and gets the refusal, not the scan's
      error. A real run still takes the lock first, so `.incrmit.lock` may be
      created, as it is for every writing command; nothing else is.
- [x] Apply the same check in `--dry-run`, so the dry run predicts the refusal
      rather than printing a plan the real run will not carry out.
      `--dry-run` already called `LoadIgnore`, so it takes the same path;
      every refusal case runs both ways.
- [x] Retire `LoadIgnore`'s leniency along with it: a file that fails the
      check never reaches it, so an unparseable `--output` becomes an error
      rather than "no patterns". Share the check with Milestone 33's type
      check, so a FIFO or a device at `--output` is refused by the same code.
      The check is `LoadIgnore` itself, after the `files.ReadOwnFile` read, so
      there is one read and one function deciding. A FIFO, a device, or a file
      over the cap is now a `NotConfigError` wrapping `files.ErrNotRegular` or
      the `*files.TooLargeError`, with the same message and advice as any other
      refusal (`TestDiscoverRefusesFIFOOutput`); `TestOwnFilesNotRegular` and
      `TestOwnFilesOverSizeCap` pass unchanged. A file that cannot be read is
      still a read error, since it may be a config. Three tests pinned the old
      leniency and were rewritten: `TestLoadIgnoreLenient` is now
      `TestLoadIgnoreMissingOrEmpty`, `TestLoadIgnoreRefusesNonConfig` (each
      reason), and `TestLoadIgnoreAcceptsConfigs`;
      `TestDiscoverExcludesCustomOutput` used a `conf.cfg` holding `1.0.0`,
      which is now refused, so it holds a config that pins a version;
      and `FuzzLoad`, which required garbage to come back as no patterns, now
      requires `LoadIgnore` to fail only with a `NotConfigError` and to accept
      every config `Marshal` writes with the same `ignore` list. A 60s run
      found nothing in 24,000 executions.
- [x] Assert in every refusal test that the file's bytes are unchanged.
      Document the rule in `README.md`, `incrmit help discover`, and the man
      page, and add a `Fixed` entry to `CHANGELOG.md`.
      Each refusal test compares the file's bytes and the project's snapshot.
      As in Milestones 33 to 35, the reference material is in `doc/USAGE.md`:
      a paragraph after the Discovery flags examples, with the message and the
      cases, and a pointer from Ignoring folders and files. The README has one
      sentence under Configuration. `incrmit help discover` has a new
      `outputNote` (`TestDiscoverHelpDocumentsOutputRule`); the man page has
      the rule under `-o, --output`. `doc/DEVELOPMENT.md` has it in §6.1, §7,
      §8.2 step 2, §10, §12, and §12.1. `CHANGELOG.md` opens `[Unreleased]`
      with a `Fixed` entry. Suite passes under `-race` and `make check`, all six
      release platforms vet, and coverage is 96.6% -> 96.7%.

## Milestone 37 — Discover Paths Relative to the Config

`discover` records each file's path relative to the scan root (`--path`), but
every command resolves a config path relative to the directory holding the
config (`--output`). The two agree only when the config is written into the
directory that was scanned, which the defaults do. Run from the root,
`incrmit discover --path sub` writes `path = "VERSION"` into `./incrmit.toml`
for `sub/VERSION`, and the next bump resolves that to `./VERSION`. If no such
file exists, the config fails to load. If one exists and holds the same
version, the bump rewrites the wrong file and reports success. Reproduced
with `1.0.0` in both: the bump moved the root `VERSION` to `1.0.1` and left
`sub/VERSION` at `1.0.0`. `-o sub/incrmit.toml` run from the root breaks the
same way in the other direction, and `README.md` shows `--path ./src` as an
example.

- [x] Reproduce first, with a decoy holding the same version at the location
      the bad path resolves to, since that is the silent case: `--path sub`
      with the default output, `-o sub/incrmit.toml` with the default path,
      and the two flags naming sibling directories. After each, assert that a
      bump changes the scanned file and nothing else.
      `internal/cli/discoverpaths_test.go`. `TestDiscoverPathsRelativeToConfig`
      runs `discover` (as `--dry-run`, then for real) and then a bump of the
      config it wrote, and compares every file in the tree. The scanned files
      must move to `1.0.1` and the rest stay at `1.0.0`. Of its twelve
      original cases, nine failed before the fix. Seven were silent, each
      bumping its decoy and leaving the scanned file at `1.0.0`: `--path sub`,
      `--path ./sub`, an absolute `--path`, absolute `--path` and `-o`,
      `-o build/incrmit.toml` (the walk skips `build/`, so its `VERSION` is a
      decoy nothing lists), siblings (`--path src -o release/incrmit.toml`),
      and nested siblings. `-o sub/incrmit.toml` and `--path ..` run from
      `sub` failed loudly instead, because `sub` is scanned too and its
      `sub/VERSION` resolved to `sub/sub/VERSION`, so the config did not load.
      The three cases with the config in the scanned directory passed before
      and after.
- [x] Write every path relative to the output config's directory: make the
      root and the output absolute, join each result to the root, and take
      `filepath.Rel` from the config's directory, in slash form. A root
      outside the config's directory gives `../` paths, which config loading
      already accepts. Rework `excludeOutput`, which joins paths to the root
      today.
      `discovery.DiscoverWithLimit` takes the config's directory as a new
      `configDir` argument, and `Discover` passes the root for both. One
      change from the plan: making the two directories absolute is not
      enough. On macOS `t.TempDir()` is under `/var`, but `os.Getwd` after a
      chdir reports `/private/var`, so an absolute `--path` gave
      `../../../../../../../../var/folders/.../sub/VERSION`. That path
      resolves to the right file, but it is absolute in all but name. A shell
      whose `$PWD` is `/tmp/x` with an argument spelled `/private/tmp/x` hits
      the same thing. `resolveDir` therefore also resolves symbolic links,
      in the longest prefix that exists, since a `--dry-run` may name an
      `--output` directory that is not there yet. The walk still uses
      `--path` as given, so its errors name the path as the user wrote it. On
      Windows, a root on another volume than the config has no relative form,
      so the absolute path is written. `excludeOutput(results, output)` now
      drops the result whose path `config.TargetPath` resolves to `--output`,
      the way a bump resolves it. `TestDiscoverExcludesOutputBelowRoot` covers
      `-o sub/conf.cfg` and `-o ./sub/../sub/conf.cfg`. A side effect:
      `--path FILE` recorded `path = "."`, a config that never loaded, and now
      records the file's path.
- [x] Decide what `ignore` patterns are relative to. They are matched against
      paths relative to the scan root, but they live in the config, so one
      pattern means something different under a different `--path`.
      Recommend the config's directory, so a config means the same thing
      wherever discover is run from; either way, say so in the comment
      `IgnoreComment` writes into every config.
      The config's directory: patterns are matched against the same paths the
      config lists, `../` included (`../src/gen/` from a config in
      `release/`). A bare pattern matches a base name at any depth, so only a
      pattern with a slash changes meaning, and only under a `--path` other
      than the config's directory. The scan root itself is still never
      pruned, by a pattern or a built-in name, as before; patterns apply to
      what is inside it. `IgnoreComment` now says "Like each file's path, a
      pattern is relative to the folder this file is in."
      (`TestIgnoreCommentSaysRelativeToConfig`). This repo's `incrmit.toml`
      picked up the new comment when the version bump rewrote it.
- [x] Print the config-relative paths in the `--dry-run` listing and the
      "Wrote ..." summary too, so the paths a user reviews are the ones a bump
      will resolve.
      Both print `Result.Path`, which is now config-relative. A dry run whose
      config is not in the scanned directory (`discovery.SameDir`) adds a
      `(paths relative to release/incrmit.toml)` line under its heading.
      Without it, `../src/VERSION` listed under "Discovered 1 file(s) under
      src" reads like a mistake. The default output is unchanged
      (`TestDiscoverDryRunNotesConfigDir`, plus a presence check in every
      table case).
- [x] A config written by the old behavior cannot be detected in general,
      since its paths may exist and hold the pinned version. Add a `Fixed`
      entry to `CHANGELOG.md` telling anyone who ran `discover` with `--path`
      or `-o` pointing elsewhere to run it again.
      `[0.3.9]` has three `Fixed` entries: the paths, with the advice to rerun
      in bold; the change in meaning for slashed `ignore` patterns under a
      different `--path`, with the rewrite (`sub/gen/**`); and `--path FILE`.
- [x] Test the cases above plus the default run (paths unchanged), an absolute
      `--path`, and a `--path` written with `./`. Update `README.md`
      (Discovery) and the man page to say paths are written relative to the
      config.
      The table also has `--path ./`, `--path` naming a file, and two cases
      run from a subdirectory. `internal/discovery/configdir_test.go` covers
      the shapes at the unit level, `ignore` patterns relative to the config
      (`gen/**` under `--path sub` no longer prunes `sub/gen`), the root never
      being pruned, and a directory reached through a symbolic link or not
      created yet. `TestDiscoverIgnoreRelativeToConfig` checks the patterns
      through `cli.Main`. The `--path ./src` example is in `doc/USAGE.md`, not
      the README; it was correct only because the config stays in `.`. The
      README's Configuration section now says paths and patterns are written
      relative to the config, with the sibling example. `doc/USAGE.md` covers
      it in Discovery, the dry-run note, and the matching rules; the man page
      under `discover`, `-P`, and `-o`. `incrmit help discover` has a new
      `pathsNote` (`TestDiscoverHelpDocumentsPaths`), and `doc/DEVELOPMENT.md`
      has it in §6.1, §8.2 steps 3, 6, and 7, §8.3, and §12. The suite
      passes under `-race` and `make check`, and all six release platforms
      vet. Coverage went from 96.7% to 96.5%: the new lines not covered are
      the error branches for `os.Getwd` failing, a `filepath.Rel` that cannot
      fail inside the walk, and the Windows cross-volume fallback.

## Milestone 38 — Crash-Safe Multi-File Writes

Planning is already fail-fast (Milestone 22's phase 1/2 split), but phase 2 is
not: `runBump` writes files one at a time, so a failure on file 3 of 5 leaves
1–2 bumped and 3–5 untouched. Worse, `recordHistory` runs *after* every write
succeeds, so exactly the case where `undo` is needed is the case where no
journal entry exists. Close this before v1.0.0.

- [x] Write the journal entry *before* phase 2 rather than after, marked
      `pending`, and flip it to `complete` once every write lands. Bump
      `history` file format handling so an older state file still loads.
      Reproduced first with three targets and `c/` read-only: `a/VERSION` and
      `b/VERSION` were bumped, the run exited `1`, and `undo` answered
      "Nothing to undo". `history.Entry` has a `status`, `pending` or
      `complete`. `startEntry` pushes the entry `pending` and saves it before
      the first target, and the bump marks it `complete` after the config. An
      entry with no `status` loads as `complete`, since only finished bumps
      were recorded before, and an unknown status fails the load
      (`TestLoadEntryWithoutStatusIsComplete`,
      `TestLoadRejectsUnknownStatus`). One addition to the plan: phase 2 was
      not only writes. The pinned-version check ran there, file by file, so
      `version = "9.9.9"` on the second of two files left the first bumped
      and exited `3`, while `--dry-run` reported success. That check moved
      into `planGroups`, which now computes every target's new bytes, and the
      config is rendered before the first write too. Phase 2 only writes
      (`TestBumpOutOfSyncPinWritesNothing`).
- [x] On a phase-2 write failure, roll back the files already written in this
      run (the pre-bump bytes are still in memory from planning) before
      returning, and report both the original failure and whether the rollback
      itself succeeded.
      `commitWrites` in the new `internal/cli/commit.go` does it for the bump
      and for undo. It writes in order, and on a failure puts back the files
      already written, newest first, attempting every one even after one
      fails. If all of them are put back, it restores the journal: the
      entries from before the run, or no state file at all if the run created
      it, so a first bump that fails leaves no empty journal behind. The
      report is the write error, then `rolled back a/VERSION, b/VERSION; no
      file was changed`. A journal that cannot be restored is a warning,
      since its pending entry is still one `undo` can clear.
- [x] When rollback is impossible or partially fails, leave the `pending` entry
      in place and print an explicit recovery instruction naming `incrmit undo`
      and the affected files, rather than exiting with a bare error.
      Each failed rollback is reported (`rolling back a/VERSION: permission
      denied`), then the files left changed, one per line, and a line saying to
      run `incrmit undo` once the problem is fixed. A
      run given `-c` names `incrmit undo -c <config>`, with the path quoted for
      a POSIX shell when it needs it (`shellWord`), so the line can be pasted.
      A crash leaves the same entry with no message, so a bump that fails in
      planning while the newest entry is `pending` adds a line pointing at
      `undo` (`TestBumpAfterCrashPointsAtUndo`).
- [x] Teach `undo` to recognize a `pending` entry and treat it as the thing to
      revert, tolerating files that were never written (the recorded `new` token
      is absent because the write never happened, which is not the "file was
      edited since" case that currently aborts the whole undo).
      For a `pending` entry, a file holding every `new` token and not every
      `old` one is reverted, and one holding every `old` token and not every
      `new` one is left alone and listed as `already at 1.0.1`. A file holding
      both, or neither, is refused. A bump only raises precedence, so a file
      it wrote cannot still hold its smallest `old` version, and a file it
      never reached holds its largest `new` version only as text the bump does
      not own. The config is judged the same way, as a whole: at the new
      versions it is reverted, at the old ones it is left untouched, comments
      included, and split between them it is refused. The trust check widens
      only that far. One addition to the plan: undo is a multi-file write too,
      and a failed one used to leave the reverted files at the old version,
      after which every undo refused them as edited. Undo now marks the entry
      `pending` before its first write, rolls back through `commitWrites`
      (marking the entry `complete` again), and can simply be run again after
      a crash. The output says `Undid the unfinished bump` for a pending
      entry.
- [x] Order the config rewrite and the target writes so a crash between them is
      recoverable in one direction only, and document which one is written first
      and why in `doc/DEVELOPMENT.md`.
      Targets first, config last: the config is the commit record, so a config
      at the new versions means every target was written. A run that stops
      between them leaves the targets new and the config old. A rerun of the
      bump refuses that state (exit `3`, the pinned versions are gone from
      the bumped files, and it now points at `undo`), and `undo` reverts it,
      so the only way out is back. It is also what lets undo decide from the
      config alone whether to rewrite it. DEVELOPMENT.md §8.7 explains why
      the config does not go first.
- [x] Add tests that inject a write failure mid-run (e.g. a read-only directory
      or an unwritable target as the Nth of M files) and assert: files are
      restored, the journal reflects the interrupted run, and a following `undo`
      leaves the tree exactly as it started.
      `internal/cli/crashsafe_test.go`. A bump's and an undo's writes go
      through two package variables, `writeFile` and `saveJournal`, which a
      test replaces to fail the Nth write, fail a rollback too, or crash:
      panic, so the run unwinds without its rollback or anything after it, as
      a `kill -9` would. The project is three targets in their own
      directories with an earlier bump in the journal. Bump and undo are each
      failed at every target and the config, and crashed there and at both
      journal writes. Every case must end byte-identical to the start
      (`snapshotProject`), straight away for a clean rollback, or after one
      `undo` for a failed rollback or a crash, which must leave a `pending`
      entry naming the bump. A read-only target directory and an unwritable
      config directory cover the real failures. The second stopped the old
      build at the config write, after every target was bumped. Hand-written
      `pending` journals cover unwritten files, files holding both sides or
      neither, and configs at neither version or split. Six mutations
      (dropping the rollback, the journal restore, the pending check, the
      both-check, the pending mark, or the config check) each fail at least
      one test.
- [x] Document the crash-safety guarantee — and its limits — in
      `doc/DEVELOPMENT.md`; add a `CHANGELOG.md` entry under `Fixed`.
      DEVELOPMENT.md has a new §8.7 (guarantee, write order, rollback, undo of
      a pending entry, the last journal write failing, and limits). It is
      referenced from §6.2 (the `status` field and how older and newer
      versions read it), §8.1 and §8.4 (steps renumbered), §10, and §12. The
      limits: readers can see a partly bumped tree; process death is covered
      but power loss is not, since no directory is synced after a rename; a
      rollback, like any write, leaves a replaced symlink a regular file; it
      restores the bytes the run read, so an editor's change in that window
      is lost; and undo judges files by tokens, not checksums. `Unreleased`
      has four `Fixed` entries: the rollback, the out-of-sync pin, undo, and
      the unwritable config folder. `doc/USAGE.md` (Undo), the man page
      (`undo`, FILES, CAVEATS), and `incrmit help undo` describe the new
      behavior. The suite passes under `-race` and `make check`, all six
      release platforms vet, and coverage went from 96.9% to 97.7%. The new
      lines not covered are the two `config.Marshal` error branches, which
      cannot fail on a config `Load` accepted.

## Milestone 39 — JSON and TOML Parsers

Every file goes through the same text scanner, `version.FindTokens`, which
knows nothing about the format it reads. In a JSON or TOML manifest every
version-shaped run is a candidate: one inside a URL, a dependency range, or a
comment counts the same as the project's own `version` key. Reproduced with a
`package.json` holding `"version": "1.2.3"`, a `$schema` URL with `2.0.0` in
its path, three dependencies (`1.2.3`, `4.17.21`, `^18.3.1`), and
`"engines": { "node": ">=20.0.0" }`, next to a `Cargo.toml` whose `version`
line ends in `# was 0.3.9` and which has two dependencies. `discover` wrote
eight entries for the two files and `--minor` moved all eight: the schema URL
became `.../2.1.0/schema.json`, `^18.3.1` became `^18.4.0`, `>=20.0.0` became
`>=20.1.0`, and the comment became `# was 0.4.0`, all with exit `0`.

Give `.json` and `.toml` files a parser that reads the format, so a candidate
is a string value that is exactly a version, and each candidate knows the key
it sits under. Every other file keeps the text scanner, and so does a `.json`
or `.toml` file these parsers cannot read. This alone does not stop the
dependency pins from moving, since `"left-pad": "1.2.3"` and
`serde = "1.0.200"` are whole string values too; Milestone 40 selects the
project's version by the key this milestone records.

- [x] Reproduce first, with the two files above plus a `pyproject.toml`:
      assert what `discover --dry-run` lists, what `discover` writes, and what
      a bump changes, byte for byte. The schema URL, both ranges, and the TOML
      comment must stop being candidates; the dependency pins are still
      candidates, which is Milestone 40's case. Keep the fixtures for
      Milestone 40's tests.
      `internal/cli/parsers_test.go` (`TestReproductionReadsManifestsByFormat`).
      The fixtures are `internal/files/testdata/repro/{package.json,
      Cargo.toml,pyproject.toml}`, under their real names since the extension
      picks the parser; `.gitignore` ignores `*.toml`, so it now un-ignores
      that directory's. The pyproject adds the version in its description, a
      URL, a comment, `requests>=2.31.0`, and `click==8.1.7`. Goldens hold the
      dry run, the written config, the `--minor` summary, and each bumped file
      (`internal/cli/testdata/repro_*.golden`); `undo` then restores every
      byte. As text the three files hold seventeen candidates; read by format,
      eight, each with its key. The schema URL, both ranges, the comment, the
      description, the URL, and the specifiers are gone; `left-pad`, `lodash`,
      `serde`, `anyhow`'s `version`, and pyproject's `tool.example.schema-version`
      remain for Milestone 40.
- [x] Put the parser behind one seam, so §9's rule still holds: what discovery
      records is exactly what a bump rewrites. A parser takes a file's bytes
      and returns each candidate as a byte range, its parsed version, and its
      key path (none from the text scanner). `discovery.scan`,
      `files.SetKnownVersions`, `files.FindVersion`, `preview`, and `undo`
      take their ranges from it instead of calling `version.FindTokens`
      themselves. The text parser is `FindTokens` plus `version.Parse`,
      unchanged, so every file that is not `.json` or `.toml` behaves exactly
      as before. Fix the comment at the top of `internal/files/files.go`: it
      says structured detection lives in `discovery`, but nothing there reads
      a format today.
      `internal/files/parser.go`: `files.Parser` (`text`, `json`, `toml`) and
      `Parser.Locate`, which returns a `Located` of `Candidate`s
      (`Start`, `End`, `Version`, `Key files.KeyPath`) in byte order plus the
      `Skipped` values. `FindVersion`, `SetVersion`, `SetKnownVersion`, and
      `SetKnownVersions` take the candidates instead of scanning; `ReadVersion`
      and `ApplyBump` locate with `ParserFor(path)`. `discovery.scan`,
      `readGroups`/`planGroups` (shared by `preview`), and `planReverts` take
      their ranges from `Locate`, and nothing outside `files` calls `FindTokens`
      to find a version in a file. The text parser is `TextCandidates`:
      `FindTokens` plus `version.Parse`, so existing tests pass it explicitly
      and are otherwise unchanged. The comment at the top of `files.go` now
      says where candidates come from.
- [x] Choose the parser by extension, ignoring case: `.json` gets the JSON
      parser, `.toml` the TOML parser, and everything else text. Add an
      optional `parser` key to `[[files]]` (`"json"`, `"toml"`, or `"text"`)
      that overrides it, for a `.json` file with comments (`tsconfig.json`,
      VS Code's `settings.json`) or TOML under another name (`Pipfile`).
      Omitted means "by extension", so every existing config still loads.
      Declaring the field on `FileEntry` is all Milestone 36's known-key check
      needs, and an unknown value is a config error naming the entry. The
      bump's config rewrite rebuilds each entry with
      `config.EntryFor(path, version)`, which would drop the key, so carry it
      through and test that a bump keeps it.
      `files.ParserFor` and `FileEntry.FileParser`. `Validate` refuses an
      unknown value (`config: files[1] (tsconfig.json): unknown parser
      "jsonc"; use "json", "toml", or "text"`, exit `1`) and entries for one
      path that read it two ways, since the file is read once; an omitted key
      and the extension's own parser count as the same choice. `bumpedConfig`
      sets `Parser` on each `EntryFor` result, and the CLI tests check a bump
      keeps it for `text` and `toml` overrides. `LoadIgnore` accepts the key
      with no change beyond the field.
- [x] JSON: use the standard library's `encoding/json` `Decoder`, walking
      `Token()` with a stack of object keys and array indexes, and take each
      string value's byte range from `InputOffset()`. That needs no new
      dependency; `encoding/json/jsontext` reports offsets directly, but it is
      still behind `GOEXPERIMENT=jsonv2` in Go 1.27. A candidate is a string
      value, never an object key, whose whole content is one token that
      `version.Parse` accepts: `"1.2.3"` and `"v1.2.3"` are candidates, and
      `"^18.3.1"`, `">=20.0.0"`, and a URL holding `2.0.0` are not. The
      decoder rejects a byte-order mark, which §9.4 says incrmit keeps, so
      skip a UTF-8 BOM before decoding and offset every range past it. A value
      whose raw bytes differ from its decoded text (an escape such as
      `\u002e`) is not a candidate, and the dry run says so, rather than
      incrmit re-encoding the string. `encoding/json` accepts duplicate keys,
      so both values are candidates under the same key path.
      `internal/files/json.go`. `json.Valid` decides whether the file is JSON
      at all, because the `Decoder` reads `{} {}` as a stream; a
      `SyntaxError`'s offset gives the line. The value's bytes start at the
      first `"` after the previous `InputOffset`, since separators and
      whitespace come first. Numbers stay `json.Number` (`1e999` is valid JSON
      but no `float64`). A candidate must also be one of the text parser's
      tokens with the same range. An escaped value that decodes to exactly a
      version (`version.Parse` alone trims whitespace) is `Skipped`, and the
      dry run prints `L2 version: skipped, "1.2.3" is a version only once its
      escapes are decoded; ...`.
- [x] TOML: `BurntSushi/toml`, which already reads the config, is the only
      TOML reader. It decides whether the file is TOML and what each key
      holds, but v1.6.0 keeps key positions in an unexported field, so it
      cannot say where a value's bytes are. Find them by probing rather than
      lexing: replace every token `version.FindTokens` finds (and
      `version.Parse` accepts) with its own placeholder, decode the probe copy
      once, and walk it through tables, inline tables, arrays, and arrays of
      tables (`bin[0]`). A token whose placeholder is now a key's whole value
      is a candidate at that key. One whose placeholder is part of a longer
      string (a URL, a description, a multi-line string) is not, and one whose
      placeholder is in no value was in a comment or a key. Token boundaries
      stay `FindTokens`'s, so §9's single definition of a token holds in TOML
      files too.
      `internal/files/toml.go` (`locateTOML`). One change to the plan: a
      placeholder has one digit run per dot-separated segment of its token,
      not one fixed `A.B.N` shape, so a token used as a bare dotted key
      (`[versions.1.2.3]`, `x-1.2.3`) splits into as many keys as before and
      the key path maps back segment by segment (`probe.unmap`). The walk
      covers tables, inline tables, `[]any` arrays, and `[]map[string]any`
      arrays of tables. A string that holds a placeholder's prefix but is not
      a placeholder is a longer string; one that holds none but decodes to a
      version is an escape, `Skipped` as in JSON.
- [x] Make the probe exact and cheap. Placeholders are digits and dots, so
      each is valid wherever its token was, have a fixed width
      (`987654321.987654321.000000001`), and must not already occur in the
      file. In a prototype, unpadded placeholders collided (`...1` is a
      prefix of `...10`), and a comment token was reported as part of
      `bin[0].version`. A probe copy that does not decode when the original
      did is refused like a file that does not parse. All tokens share one
      probe decode: the prototype classified 2,000 tokens in under 0.1 s,
      while one decode per token took 2.8 s at 1,000 and grows quadratically.
      After planning a bump, decode the rewritten bytes in phase 1 and check
      that exactly the planned keys changed, to the planned values.
      Each run is a prefix plus a counter zero-padded to nine digits (wider
      past a billion segments). The prefix is `987654321` unless the file, or
      any key or string it decodes to, holds it; then `absentDigits` picks one
      a digit at a time by least frequency, nine passes however the file is
      planted. One more check than planned: the probe's answer is compared
      with the file's own decode (`probe.agrees`), scalars at the same keys,
      because the first fuzzing run found `BurntSushi/toml` v1.6.0 accepting a
      key defined twice (`1.0.0 = "x"` then `"1" = "y"`, the second dropped),
      which masking hid from the copy. A copy that disagrees is refused like
      one that does not decode, and so is the one valid shape it splits, a
      version naming an array of tables declared twice. `files.Rewrite` does
      the planned phase-1 check for both formats (`checkRewrite`), and
      `TestRewriteRefusesUnplannedChanges` feeds it a comment token reported
      as `bin[0].version`. `TestLocateTOMLIsLinearInTokens` locates 5,000
      tokens within the 10 s test deadline.
- [x] Record the TOML decision in `doc/DEVELOPMENT.md`. A hand-written locator
      that lexes TOML to find byte ranges was the first plan, and
      `pelletier/go-toml/v2` exposes ranges through its `unstable` package.
      Both were rejected: each is a second TOML reader that could disagree with
      the one the config is read with, and the second is also a new dependency
      with an API marked as subject to change. The probe leaves every question
      of TOML syntax to `BurntSushi/toml`.
      §9.6, "The TOML decision", beside why the probe is checked against the
      file and what that check refuses.
- [x] Decide what happens when a file does not parse. Recommend: `discover`
      scans a `.json` or `.toml` file that fails to parse as text, writes
      `parser = "text"` into its entries so a bump uses what discover used, and
      says why in the dry run (`tsconfig.json: not JSON (line 3: invalid
      character '/'), scanned as text`). A bump never falls back: an entry
      whose parser cannot read its file fails with exit `3`, naming the file
      and line and suggesting `parser = "text"`, in phase 1, so nothing is
      written. A file that parsed when discover ran and no longer does has
      changed shape, and guessing is how a bump goes wrong silently.
      As recommended. `discovery.Result` gains `Parser` and `Fallback` (the
      `*files.ParseError`), `Generate` writes `parser = "text"` for a fallback,
      the dry run heads the file with `tsconfig.json: not JSON (line 2: invalid
      character '/' looking for beginning of object key string), scanned as
      text`, and the summary marks its entries `(not JSON, scanned as text)`.
      A bump's entry fails in `readGroups` with exit `3`: `package.json: not
      JSON (line 2: ...); to read it as text, set parser = "text" on its
      [[files]] entry`. One case the item did not cover: a `--file` target has
      no entry, so it is read as discover would read it, falling back with
      `incrmit: note: ...` on stderr, which is also what `--file` did before.
      `classify` maps a `*files.ParseError` to exit `3`.
- [x] Show the key in discover's output: `Occurrence` gains the key path, and
      the dry run prints it next to the line number. `Generate` still writes
      one entry per distinct version with no key, so the generated config for
      a JSON or TOML file changes only by losing the entries that are no
      longer candidates. Write `parser` only when it differs from the
      extension's default.
      `Occurrence.Key`; the dry run prints `L3 package.version: version =
      "0.4.0" # was 0.3.9`, and text files print `L3: ...` as before. A file
      whose only versions were skipped is listed by the dry run but gives no
      entry, and a tree of only such files is still `no version-bearing files
      found`.
- [x] Existing configs change meaning: an entry for a `.json` or `.toml` file
      now matches only whole string values, so an entry pinning a version that
      appeared only in a comment, a URL, or a range (four of the eight above)
      finds nothing, and the bump exits `3` with the existing "not found"
      error. That is right, since none of them was the project's version, but
      the check runs in phase 2 today, after earlier files have been written,
      and this change makes it far more likely to fire. Move it into phase 1,
      which already holds every file's bytes, so an old config fails with
      nothing written. Add a `Changed` entry to `CHANGELOG.md` telling users
      to run `incrmit discover` again, or to set `parser = "text"` on an entry
      to keep the old behavior for that file. `undo` uses the parser the bump
      used: record it in the journal entry, and read an older entry that has
      none as text, which is what wrote it.
      The pinned-version check has run in phase 1 (`planGroups`) since
      Milestone 38, so nothing needed moving. `TestOldConfigFailsBeforeAnythingIsWritten`
      runs the eight entries the old discover wrote, behind a `VERSION` entry,
      and asserts exit `3`, `Cargo.toml: files: expected version not found:
      0.3.9`, the snapshot unchanged, and no journal; `--dry-run` fails the
      same way. `history.Change` gains `parser`, written by the bump, and
      `planReverts` reads each file with it, refusing an unknown parser or two
      for one file. `CHANGELOG.md` has the `Changed` entry, plus `Added` (the
      key) and `Fixed` (the reproduction).
- [x] Test each parser with golden fixtures in `internal/files/testdata`: the
      existing `package.json`, `package.min.json`, `appsettings.json`, and
      `pyproject.toml`, plus a `Cargo.toml` and the reproduction files. Cover
      a BOM, CRLF, minified and tab-indented JSON, nested arrays, duplicate
      keys, and an escaped value. In TOML, cover dotted keys, a quoted key
      holding a dot (`"a.b" = "1.2.3"` is one key, not two), a quoted key that
      is a version, inline tables, a sub-table (`[dependencies.tokio]`),
      arrays of tables, an array of version strings, a URL, a multi-line
      string holding `version = "1.2.3"` as text, and a comment holding a
      version. Cover placeholder collisions too: more than ten tokens, and a
      file that already holds the placeholder text. Test the fallback, the
      `parser` override in both directions, a bump keeping `parser`, an
      unknown `parser` value, and an old config failing in phase 1 with every
      file unchanged.
      `internal/files/parser_test.go`, with candidate goldens in
      `testdata/candidates/` for the four existing fixtures, a new
      `Cargo.toml.input` (`[dependencies.tokio]`, two `[[bin]]`s, inline
      tables, a URL, comments), and the reproduction files. JSON and TOML
      table tests cover every shape listed, and more (a root array and string,
      a token welded into a key, a line-ending backslash, dates and floats).
      Collisions: twenty-five `[[bin]]`s with comment tokens between them, the
      placeholder in a comment, a value, a key, and an escape, and two
      thousand planted prefixes. The fallback, the override both ways, a bump
      keeping `parser`, an unknown value, and the old config are in
      `internal/cli/parsers_test.go`, with `internal/discovery/parsers_test.go`
      and `internal/config/parser_test.go` at the unit level.
- [x] Fuzz the JSON locator and the TOML probe (§12.1, Milestone 30): for any
      input the decoder accepts, every candidate's bytes decode to the value
      the decoder reports at its key path, and rewriting one candidate with a
      version of a different length, then decoding again, changes that key
      and no other. Neither may panic on input the decoder rejects. Seed the
      corpora with the fixtures.
      `files.FuzzLocateJSON` and `files.FuzzLocateTOML`
      (`parser_fuzz_test.go`), seeded with the golden fixtures. The reference
      reads the document independently of the parser's walk, and TOML the
      decoder accepts may be refused only with the probe's own error. The TOML
      target failed within seconds on the decoder leniency above; the input
      is kept as `testdata/fuzz/FuzzLocateTOML/key_defined_twice_hidden_by_masking`.
      After the fix, 120 s ran 17 million TOML inputs and 60 s 4.5 million JSON
      inputs with nothing found. `config.FuzzLoad` gained `parser` seeds and
      checks every loaded entry's parser is known.
- [x] Document the parsers in `README.md`, `doc/USAGE.md`,
      `incrmit help discover`, and the man page: which file gets which parser,
      what counts as a candidate, the `parser` key, and the fallback. In
      `doc/DEVELOPMENT.md`, rewrite the §9 bullet that calls discovery
      "content-based and format-agnostic", and cover the parser seam in §5,
      the `parser` key in §6.1, and the fuzz targets in §12.1.
      README (Features and Configuration), USAGE ("How a file is read" under
      Configuration, Discovery, `--output`, Undo, exit codes), `incrmit help
      discover` (`parsersNote`), and the man page (a PARSERS section, plus
      DESCRIPTION, `discover`, `--output`, FILES, and EXIT STATUS).
      DEVELOPMENT.md: §1, §5, §6.1, §6.2 (the journal's `parser`), §8.1, §8.2,
      §8.4, the §9 bullets, a new §9.6 for the design, §10, §11, §12, and
      §12.1 (the targets and what the first run found). `make check` passes,
      and coverage is 97.6% (97.7% before); the lines not covered are
      branches for input already validated, such as a probe copy that fails
      to decode.

## Milestone 40 — Version Key per Parser

After Milestone 39 a JSON or TOML candidate must be a whole string value, but a
dependency pin is a whole string value too. A config entry still names only a
path and a version, and a bump rewrites every candidate in that file holding
the version. In Milestone 39's reproduction, the `package.json` entry for
`1.2.3` rewrites both `version` and `dependencies.left-pad`, the `Cargo.toml`
entry for `0.4.0` rewrites `package.version` and `anyhow`'s inline
`{ version = "0.4.0" }`, and the entries discover writes for `4.17.21` and
`1.0.200` bump two dependencies outright. A Node project gets a version bump
for a package it does not own, and nothing says so.

Let an entry say which occurrence is the project's version, with a `key` that
the entry's parser interprets. For JSON and TOML the key is a path (`version`,
`package.version`, `project.version`). Text has no structure, so for a text
file the key is the literal text in front of the version (in a Makefile,
`VERSION ?=` and the space after it). An entry with a key rewrites exactly the
occurrences it names and nothing else in the file.

- [x] Reproduce first, on Milestone 39's fixtures: assert that a bump moves
      the dependency pins listed above. Add a text case, a `Makefile` holding
      `VERSION ?= 1.2.3` and a line that downloads a tool at `1.2.3`.
      `TestReproductionPinsMoveWithoutAKey` (`internal/cli/keys_test.go`) bumps
      the config Milestone 39's discover wrote, kept as
      `repro_unkeyed_incrmit.toml.golden`, with `--minor`: `left-pad` goes to
      `1.3.0`, `anyhow` to `0.5.0`, `lodash` to `4.18.0`, and `serde` to
      `1.1.0`, all with exit `0`. The goldens are Milestone 39's, renamed
      `repro_unkeyed_*`, since they record exactly that bump; omitting `key`
      keeps it. The `Makefile` is `internal/files/testdata/repro/Makefile`, and
      without a key its bump moves the tool's URL directory and archive name.
- [x] Add one optional `key` field to `[[files]]`, read according to the
      entry's parser, so there is one thing to learn whatever the file. The
      cost is that a key changes meaning when its entry's parser does (a
      `parser` override, or Milestone 39's fallback to text), so make sure
      that case fails loudly instead of matching something else. A path read
      as text matches nothing: most paths end in a letter or digit, and a
      token needs a word boundary in front of it. A text key read as a path
      is not valid path syntax (the Makefile key holds a space and a `?`), or
      names a key the file does not have. Either way it is an error, never a
      rewrite of the wrong occurrence; test both directions. Omitting `key`
      keeps today's behavior: every candidate holding the pinned version.
      `FileEntry.Key` is a `*string`, so `key = ""` is refused rather than
      read as no key; `FileEntry.FileKey` reads it with the entry's parser
      (`files.ParseKey`, `internal/files/key.go`). `Validate` refuses a key the
      parser cannot read (exit `1`), and uniqueness is now by
      `(path, version, key)`, so one version may be listed once per key.
      `TestKeyWhoseParserChangedFails` runs both directions: `version` on a
      `package.json` with `parser = "text"` is `key "version" is not followed
      by a version anywhere in the file` (exit `3`), and `v=` on a JSON file,
      or `"VERSION ?= "` with `parser = "toml"`, is not a key path (exit `1`);
      a text key that happens to be a path names a key the file lacks.
- [x] Define `key` for JSON and TOML: a dotted path from the document root,
      written the way TOML writes a dotted key, with bare segments,
      double-quoted segments for keys that hold a dot or a space
      (`key = '"."'` for the `.` key in `.release-please-manifest.json`), and
      `[N]` for an array index (`bin[0].version`, for a TOML `[[bin]]` or a
      JSON array). It matches the whole path, never a suffix: `version` in
      `package.json` is the top-level key alone, never `dependencies.version`.
      The value there must be a string candidate holding the pinned version.
      A key that is absent, holds anything else, or appears twice (a duplicate
      JSON key) is an error that names the file and the key, with exit `3`,
      raised in phase 1 so nothing is written.
      `files.ParsePath` reads what `KeyPath.String` writes: bare parts, double
      quoted parts with TOML's basic-string escapes, and `[N]` without leading
      zeros, with no whitespace. `Located.Named` looks the path up in an index
      of every value of the document, built on first use (one more decode, paid
      only by a file with a keyed entry), and `Located.Claim` keeps what holds
      the pinned version. Each failure is a `*files.KeyError` saying what the
      path holds: `key dependencies.version is not in the file`, `holds an
      object, not a version`, `holds a number; a version is a string`,
      `holds "^1.2.3", which is not a version`, `holds 1.2.3, not 1.2.4`,
      `holds "1.2.3" only once its escapes are decoded`, `appears twice in the
      file, so it names no one value`. `classify` maps it to exit `3`.
- [x] Define `key` for text: literal bytes that must come right before the
      token on the same line, with no pattern language. `key = "VERSION ?= "`
      matches `VERSION ?= 1.2.3` and not `.../tool-1.2.3.tar.gz`. Matching is
      byte-exact and case-sensitive, and whitespace counts. An empty key, or
      one holding a line break, is a config error. A text key that matches no
      occurrence of the pinned version is the same phase-1 error as a missing
      JSON or TOML key. A regular expression would cover more (the "Per-file
      custom match patterns" bullet under Future Work in `doc/DEVELOPMENT.md`),
      but a literal covers the common cases without a pattern language in the
      config; replace that bullet with a pointer here and leave patterns for
      later.
      `bytes.HasSuffix` on the bytes before each candidate; a key names every
      occurrence it precedes. A key that precedes no candidate is `key
      "VERSION = " is not followed by a version anywhere in the file`, and one
      that precedes only other versions `is followed by 2.0.0, not 1.2.3`.
      A key is the bytes right before the version wherever they start, so
      `"VERSION := "` also ends `"TOOL_VERSION := "`; the test keeps that
      case. The §14 bullet now points at §9.3.
- [x] Make `discover` write `key` where it can. Every JSON and TOML candidate
      now has a key path, so write one entry per candidate with its `key`:
      each entry then rewrites exactly one value, can be deleted on its own,
      and shows in the config what it is. For manifests whose project version
      is known, write only that entry and list the other candidates in the dry
      run as skipped, with the `key` to add to track one. Recommend a short
      table: `package.json`, `composer.json`, `deno.json`, and `jsr.json` use
      `version`; `Cargo.toml` uses `package.version` or
      `workspace.package.version`; `pyproject.toml` uses `project.version` or
      `tool.poetry.version`. A known manifest without its project key (a
      private monorepo root) gets no entry, and the dry run says so, rather
      than an entry per dependency. Do not infer a `key` for text files: the
      text in front of a version is often prose, and rewording the sentence
      would break the entry. A user adds one by hand.
      As recommended, in `discovery.projectKeys`, by exact file name.
      `Result.Entries` gives a file's entries (shared by `Generate` and the
      summary), and `Occurrence.Skip` marks what gets none: `NotProject` for a
      known manifest's other candidates, and `RepeatedKey` for a candidate
      whose key names more than one value (a JSON key written twice), which a
      key would fail on. A manifest with both of its project keys (Poetry 2's
      `project.version` and `tool.poetry.version`) gets both. A document that
      is one string has no key path and gets an entry without one. The dry run
      prints `L6 dependencies.left-pad: skipped, not the project's own version;
      to track it, add an entry with key = "dependencies.left-pad"`, a key
      holding quotes as a literal string (`'dependencies."@types/node"'`), and
      heads a manifest with no project version `package.json: no entry, since
      key version holds no version`. Summary lines name the key after the path
      (`Cargo.toml package.version: 0.4.0`).
- [x] Plan keyed entries by occurrence: a `files.Replacement` carries the
      ranges its entry claims, so `SetKnownVersions` rewrites exactly those,
      still in one pass so overlapping bumps do not cascade. Two entries that
      claim the same occurrence (an entry without a key and a keyed one for
      the same version, or two entries with the same key) are a config error
      naming both, raised in phase 1. Carry `key` through the config rewrite
      after a bump (the same `config.EntryFor` problem as Milestone 39's
      `parser`) and through the journal, so `undo` reverts the same
      occurrences even after the config has been edited.
      `Replacement` gains `Keyed` and `At` (the claimed candidates' start
      offsets); `setKnownVersions` counts per replacement, so two keyed entries
      for one version are told apart, and `Rewrite` returns those counts and
      refuses a keyed replacement that did not reach every claim.
      `SetKnownVersions` keeps its per-version map, and `Matches` gives the
      per-replacement counts `undo`'s pending check needs. `readGroups` claims
      each entry's occurrences and runs `files.Overlap`, which `preview`
      shares: `config: files[0] (key version) and files[1] (no key) both claim
      the 1.2.3 on line 2 of package.json; remove one, or give each a key that
      names its own occurrence`, exit `1`. Two entries without keys are left
      to the most specific pin, as before. `bumpedConfig` writes the key as
      written, `history.Change` gains `key`, and `planReverts` reverts what the
      journal's key claims, counting a key that claims nothing as the version
      missing. One case the item did not name: undo's config rewrite matches an
      entry to its change by key first, since a patch bump can take
      `1.2.3` and `1.2.3-rc.1` under two keys both to `1.2.4`.
- [x] Extend `--file` mode and `preview`. Add `--key` to the bump, so
      `incrmit --file package.json --key version` and
      `incrmit --file Makefile --key 'VERSION ?= '` work without a config,
      where today both files are ambiguous. A `--key` that is not a valid path
      for a JSON or TOML file, or an empty one, is exit `2`. `preview` shows
      each entry's key, so the user sees which value will move.
      `--key`/`-k` on the bump and on `preview` (whose `--file` was ambiguous
      the same way). It is read once the file's parser is known, so a
      `tsconfig.json` that falls back to text takes a text key, and a path on
      it names nothing (exit `3`). `--key` without `--file` is exit `2` too.
      `preview` adds a `KEY` column after `PATH` when any entry has a key, a
      text key Go-quoted since its spaces count.
- [x] Test each fixture bumped with and without `key`, asserting that no
      dependency pin moves once the project's key is given. Cover path
      matching (top level only, quoted segments, `[N]` indexes, a JSON key
      containing a dot) and text matching (whitespace counts, case counts,
      same line only). Cover each error (an absent key, a wrong value, a
      duplicate key, an invalid path, an empty or multi-line text key, two
      entries claiming one occurrence, a key whose parser changed), each
      raised with every file unchanged; discover's output for each known
      manifest, including one without its project key; a bump and an `undo`
      keeping `key`; and the new `--file` flag. Extend Milestone 39's fuzz
      targets so every key path a parser reports resolves back to the same
      range.
      `internal/cli/keys_test.go` (`TestKeyStopsPinsMoving` over the
      reproduction files and the `Makefile`, byte for byte, then `undo`; each
      error with a snapshot; `discover` over every known manifest; `--file`;
      `preview`), `internal/files/key_test.go`, `internal/discovery/keys_test.go`,
      and `internal/config/key_test.go`. Milestone 39's
      `TestReproductionReadsManifestsByFormat` now checks the keyed config and
      that no pin moves. `FuzzLocateJSON` and `FuzzLocateTOML` read every
      candidate's key path back with `ParseKey` and require `Claim` to give
      exactly its range, or, where the reference holds the path twice, to
      refuse it; 45 s of each found nothing (5.2 million JSON and 6.9 million
      TOML inputs). `FuzzLoad` gained key seeds and checks a loaded key reads.
- [x] Document `key` in `README.md`, `doc/USAGE.md`, `incrmit help`,
      `incrmit help discover`, and the man page, with an example per parser:
      `package.json` (the motivating case), `Cargo.toml`, and a `Makefile`.
      In `doc/DEVELOPMENT.md`, cover it in §6.1, §8.1, and §8.2, and extend
      §9.3: the config now says where the version is, not only what it is.
      Add an `Added` entry to `CHANGELOG.md` saying that running
      `incrmit discover` again writes the keys into an existing config.
      README (Features, Configuration, the flag table), USAGE (a "Which
      occurrence: `key`" section, Configuration, Bumping, Discovery, Preview,
      Undo, exit codes), `incrmit help` (the overview, `keyNote` in the bump
      help, the `--key` flag lines), `incrmit help discover` (the manifest
      table in `parsersNote`), `incrmit help preview`, and the man page (a KEYS
      section, plus SYNOPSIS, `--key`, `discover`, `preview`, FILES, EXIT
      STATUS, and EXAMPLES). DEVELOPMENT.md: §1, §5, §6.1, §6.2, §8.1, §8.2,
      §8.4, §8.5, §9.3 (the design), §9.6, §10, §11, §12, §12.1, and §14.
      `CHANGELOG.md` has `Added`, `Changed` (summary lines name the key), and
      `Fixed` under Unreleased. `make check` passes, and coverage is 97.4%
      (97.6% before).

## Milestone 41 — YAML Parser

Milestone 39 reads `.json` and `.toml` files by their format and leaves YAML
to the text scanner, yet YAML holds the project version in Helm charts
(`Chart.yaml`), Dart and Flutter packages (`pubspec.yaml`), snaps
(`snapcraft.yaml`), Ansible collections (`galaxy.yml`), and `CITATION.cff`,
and those files have the problem Milestone 39 describes. A prototype ran
Milestone 39's TOML probe through a YAML decoder over a `Chart.yaml` holding
`version: 0.7.3`, `appVersion: "1.4.2"`, a `kubeVersion` range, two
dependencies (`12.5.6` and `"~17.11.3"`), the version in its description, and
an image reference in an annotation. The text scanner makes all seven tokens
candidates; read as YAML, three are (`version`, `appVersion`, and
`dependencies[0].version`). A `pubspec.yaml` went from six candidates to
three, and this repository's `ci.yml` and `release.yml` from eighteen and
twelve to one each. The rest sat in comments (`# v7.0.1`) or inside
`go install …@v1.6.0` commands, which is why this repository's config ignores
`.github/`.

The cost is a dependency. The standard library has no YAML reader, §2 asks for
few dependencies, and §13 counts two, with none of their own.
`go.yaml.in/yaml/v3` is the YAML organization's continuation of
`gopkg.in/yaml.v3`, whose repository is archived, and it requires no other
module, so incrmit would have three dependencies and still no transitive ones.
In a minimal program it added about 0.9 MB to the binary (2.4 MB to 3.3 MB).
Without it, Milestone 40's text `key` (`key = "appVersion: "`) still picks out
the version in a YAML file, but discover keeps offering every range, comment,
and dependency pin. The items below assume the dependency.

- [x] Reproduce first, once Milestones 39 and 40 have landed, since this
      milestone builds on 39's parser seam and probe and on 40's `key`. Use
      the `Chart.yaml` and `pubspec.yaml` above and a two-document Kubernetes
      manifest; assert what `discover --dry-run` lists, what `discover`
      writes, and what a bump changes, byte for byte. Keep the fixtures for
      the tests below.
      `internal/files/testdata/repro/{Chart.yaml,pubspec.yaml,deploy.yaml}`,
      beside Milestone 39's. Run through the 0.3.12 binary first: seven
      candidates in the chart and six in the pubspec, eleven entries, and
      `--minor` moved all of them with exit `0`, `kubeVersion: ">=1.22.0-0"`
      becoming `">=1.23.0"`. `TestYAMLReproduction`
      (`internal/cli/yaml_test.go`) asserts the dry run, the config, the bump
      summary, and each bumped file against `yaml_repro_*.golden`, then `undo`
      back to every original byte: three candidates each, entries for the
      chart's `version` and `appVersion` and the pubspec's `version`, the
      dependency pins listed as skipped, and `deploy.yaml: 2 YAML documents,
      scanned as text`. `TestYAMLOldConfigFailsBeforeAnythingIsWritten` runs
      the config the 0.3.12 `discover` wrote; with `parser = "text"` on its
      entries it moves every token, and its `yaml_repro_text_minor_*.golden`
      files are byte for byte what 0.3.12 wrote. This repository's `ci.yml`
      and `release.yml` go from eighteen and twelve candidates to one each.
- [x] Record the dependency in `doc/DEVELOPMENT.md`: update §2 and the
      sentence in §13 that counts two dependencies, and list what was
      rejected. `go.yaml.in/yaml/v4` is still a release candidate
      (`v4.0.0-rc.6`); revisit it at its first stable release.
      `gopkg.in/yaml.v3` is archived. `github.com/goccy/go-yaml` reports
      token positions, which the probe does not need. A hand-written reader is
      out of proportion to the job, since YAML's grammar is far larger than
      JSON's or TOML's. Measure the binary-size change on incrmit itself, not
      only on the minimal program above, and record it.
      `go.yaml.in/yaml/v3` v3.0.5, which requires no module. §2 names the
      three dependencies, §13 counts three, and §9.7 ("The YAML decision")
      lists what was rejected, with v4 under §14. On incrmit itself, built
      with the release flags, the binary grows 0.31 to 0.38 MB, about 6 to 7
      percent: 5.17 to 5.49 MB on `darwin/arm64`, 5.41 to 5.79 MB on
      `linux/amd64`, 5.95 to 6.34 MB on `windows/amd64`. That is well under the
      minimal program's 0.9 MB, since incrmit already links most of what the
      decoder uses.
- [x] Choose the parser as Milestone 39 does, by extension and ignoring case:
      `.yaml` and `.yml` get the YAML parser, and so does `.cff`, since the
      Citation File Format is YAML by definition. Add `"yaml"` to the values
      the `parser` key accepts. Milestone 39's fallback carries over
      unchanged: `discover` scans a YAML file that does not parse as text and
      writes `parser = "text"`, and a bump never falls back.
      `files.ParserYAML`, `ParserFor`, and `ParseParser`, whose error now
      lists `"json", "toml", "yaml", or "text"`. The tests that used `yaml` as
      the example of an unknown parser use `ini`. Nothing else in the seam
      needed changing: `discovery.detect`, `readGroups`, and `planReverts`
      already took any `*files.ParseError` as the fallback or the exit-`3`
      refusal.
- [x] Reuse Milestone 39's TOML probe with the same placeholders, decoded with
      `go.yaml.in/yaml/v3` into a `yaml.Node` tree, never into `any`. A
      candidate is a scalar that is a value, not a mapping key, whose decoded
      text is exactly one placeholder and whose tag is `!!str`, implicit or
      written (`!!str 1.2.3` counts, `!semver 1.2.3` does not). Plain,
      single-quoted, and double-quoted scalars all qualify. A placeholder
      inside a longer scalar (an image reference such as
      `ghcr.io/me/app:1.4.2`, a range such as `~17.11.3`, a `|` or `>` block
      scalar) is not a candidate, and neither is one in a comment or a key. A
      placeholder resolves to `!!str` just as a version does, so the probe
      never changes a value's type. After planning a bump, decode the
      rewritten bytes in phase 1 and check that exactly the planned keys
      changed, as Milestone 39 does for TOML.
      `internal/files/yaml.go` (`locateYAML`). `newProbe` and `absentDigits`
      now take the decoded strings rather than a TOML document, so both
      formats share them; YAML gives every scalar and tag (`yamlTexts`). A
      `|` or `>` block scalar is never a candidate, even `|-` holding exactly
      a version, since the item names plain and quoted scalars as the ones
      that qualify. A value under a key that is not a scalar (`? [a, b]`) is
      not one either, since no key path can name it. As for TOML, the probe
      copy is compared with the file before it is trusted (`agreesYAML`, over
      `flattenYAMLTree`); no input is known to make them disagree. `Rewrite`
      checks YAML with `flattenYAML`.
- [x] Handle anchors and aliases. A `yaml.Node` keeps an alias as an alias
      node, so an anchored version (`base: &v 1.2.3`) is one candidate at the
      anchor's key, and an alias of it (`pinned: *v`) is not a candidate of
      its own: it has no bytes to rewrite. Rewriting the anchored value
      changes every alias, so the dry run lists the aliasing keys beside the
      candidate, the phase-1 check counts them as planned, and a Milestone 40
      `key` that names an alias is an error naming the anchor's key. A merge
      key (`<<: *defaults`) is an alias too and follows the same rule.
      `Located.Aliases` lists them, transitively (an alias of an anchored
      mapping holding the candidate, or of one holding such an alias), at the
      alias's own path (`pinned`, `prod."<<"`), with a cost that is the size
      of the answer however they nest or cycle; `Occurrence.Aliases` carries
      them and the dry run prints `L1 base: base: &v 1.2.3 (aliased at
      pinned)`. The check counts them as planned by comparing an alias as the
      anchor it names. A key naming an alias, or a path the data has only
      through one or through a merge key, is `key pinned reads base through an
      alias, and incrmit rewrites a value where it is written; name base
      instead` (`resolveYAML` follows the path as a reader of the data would).
- [x] Read one document per file. A YAML stream can hold several, and
      Milestone 40's key path has no way to name one. Recommend: the YAML
      parser accepts exactly one document (a leading `---` or a trailing `...`
      is fine), and a file with more takes Milestone 39's fallback, with the
      reason in the dry run (`deploy.yaml: 2 YAML documents, scanned as
      text`). Multi-document files are mostly Kubernetes manifests, whose
      versions sit inside image references that the YAML parser would not
      offer anyway, so text with a text `key` is what they need. A document
      selector in the key syntax can come later if someone asks for one.
      As recommended. `ParseError.Docs` holds the count and renders as
      `2 YAML documents`; the discover summary marks the entries
      `(2 YAML documents, scanned as text)`, and a bump of an entry without
      `parser = "text"` fails with exit `3`. A file with no document (empty,
      or only comments) is accepted and holds nothing, rather than taking the
      fallback, which would offer its commented-out versions. The selector is
      under §14.
- [x] Treat duplicate keys as Milestone 39 treats them in JSON. YAML forbids
      them, but decoding into a `yaml.Node` accepts them, so both values are
      candidates under the same key path, and Milestone 40's error for a key
      that appears twice applies.
      Nothing YAML-specific was needed: the key index and `markSkips` count
      values by path.
- [x] Cover hostile input. The prototype confirmed that the decoder refuses
      nesting past 10,000 levels with an error rather than exhausting the
      stack (100,000 nested `[` in 13 ms, 20,000 nested block mappings in
      0.3 s), and that decoding into a `yaml.Node` expands no aliases, so a
      billion-laughs document decodes at once (decoded into `any`, it is
      refused as excessive aliasing). Add both to Milestone 31's
      pathological-shapes tests. The decoder also limits an implicit key to
      1,024 characters, and a placeholder is wider than most tokens, so a key
      holding several tokens can pass the limit in the probe copy only.
      Milestone 39 already refuses a probe copy that does not decode; add a
      fixture for this case.
      `TestYAMLNestingIsRefused` and `TestYAMLAliasBombs`
      (`internal/files/shapes_test.go`), each within `testutil.Within`: the
      decoder's depth error for 100,000 nested flow sequences and mappings and
      20,000 block sequences on one line (block mappings that deep are a
      50 MB file, past discover's cap, and took 8.5 s under `-race`), a
      version 9,990 levels deep located in milliseconds, and billion laughs
      placed, listed with its 72 aliases, and rewritten, plus a 20,000-link
      alias chain. The near-limit case found the probe's check rendering
      every node's whole path, 1.8 s at that depth; it now compares each
      node's last step (`yamlShapes`), 17 ms.
      `testdata/implicit-key.yaml.input` is a 199-byte key of thirty versions
      that decodes as written and is refused as `YAML whose versions incrmit
      cannot place (line 2: could not find expected ':')`.
- [x] Add the YAML manifests to Milestone 40's table of known project keys:
      `pubspec.yaml`, `snapcraft.yaml`, `galaxy.yml`, and `CITATION.cff` use
      `version`. For `Chart.yaml`, write entries for both `version` (the
      chart's) and `appVersion` (the application's): both belong to the
      project, many charts keep them equal, and the user deletes the one they
      version separately. Dependencies are listed as skipped, as for the other
      manifests.
      `discovery.projectKeys`, with `Chart.yaml`'s two keys explained there.
      `TestDiscoverKnownYAMLManifests` covers each, a library chart with no
      `appVersion`, and a pubspec with no version of its own.
- [x] Note pubspec's build number. Flutter reads the part after `+` in
      `version: 2.3.1+45` as the build number, and §9.2 drops build metadata
      on a bump, so the bump writes `2.3.2`. The text scanner does the same
      today; document it under `pubspec.yaml` in `doc/USAGE.md` and leave
      §9.2 as it is.
      Under Discovery in `doc/USAGE.md`, and in the man page's KEYS.
      `TestPubspecBuildNumberIsDropped` patches `2.3.1+45` to `2.3.2` with
      `--file --key version`.
- [x] Existing configs change meaning, as in Milestone 39: an entry for a
      `.yaml`, `.yml`, or `.cff` file that pinned a version found only in a
      comment, a range, or a longer string now finds nothing and fails in
      phase 1. The case most likely to surprise is an image tag in a
      Kubernetes manifest, fixed with `parser = "text"` and a text `key`
      (`key = "ghcr.io/me/app:"`). Say so in the `CHANGELOG.md` `Changed`
      entry, and have `undo` take the parser from the journal, as in
      Milestone 39.
      The journal already recorded each change's parser, so `undo` needed
      nothing new; `TestUndoUsesTheJournalsYAMLParser` reverts a value whose
      comment holds the bumped version, with the config since changed to
      `parser = "text"`. An entry for a manifest of several documents fails
      too, as one the YAML parser cannot read. `CHANGELOG.md` has the
      `Changed` entry under Unreleased.
- [x] Test with golden fixtures in `internal/files/testdata`: the reproduction
      files; a BOM (the decoder accepts one, and the probe moves no byte
      outside a token, so unlike JSON nothing needs offsetting; assert it);
      CRLF; flow mappings and sequences; all three quoting styles; a comment
      after a value; `|` and `>` block scalars holding `version: 1.2.3` as
      text; anchors, aliases, and a merge key; a duplicate key; an explicit
      `!!str` tag and a custom one; a key that is a version; a two-document
      file; tab indentation; a Jinja-templated conda `meta.yaml`, which is not
      YAML and takes the fallback; and the implicit-key overflow. Test the
      `parser` override in both directions and an old config failing in
      phase 1 with every file unchanged.
      Candidate goldens for the reproduction files, `shapes.yaml.input` (every
      shape that parses, in one document), and `bom-crlf.yaml.input`, each
      also rewritten through `Rewrite`; the refused fixtures (`tabs`, `meta`,
      `implicit-key`, and the two-document `deploy.yaml`) are asserted with
      their messages. Table tests in `internal/files/yaml_test.go` cover each
      shape again with the key it is placed at, every `*KeyError` YAML adds,
      placeholder collisions, and 5,000 tokens within the deadline. The
      override, the old config, the image-tag recipe, and the alias errors are
      in `internal/cli/yaml_test.go`, and discovery's side in
      `internal/discovery/yaml_test.go`. The decoder refuses a `%YAML 1.2`
      directive (it reads 1.1), which the rejects table pins.
- [x] Fuzz the YAML probe with Milestone 39's properties (§12.1, Milestone 30),
      seeding the corpus with the fixtures.
      `files.FuzzLocateYAML`, against a reference that walks the decoder's
      tree on its own. A stream of several documents must be refused with
      its count, and one document the decoder accepts may be refused only by
      the probe. 90 s and then 5 minutes ran 12.3 million inputs and found
      nothing.
- [x] Document YAML wherever Milestone 39 documents its parsers (`README.md`,
      `doc/USAGE.md`, `incrmit help discover`, the man page, and
      `doc/DEVELOPMENT.md` §5, §6.1, §9, and §12.1), including the image-tag
      recipe above, and add an `Added` entry to `CHANGELOG.md`.
      README (Features, Configuration), USAGE ("How a file is read" with a
      YAML section and the recipe, keys, `--key`, Discovery with the manifest
      table, `Chart.yaml`, and `pubspec.yaml`, the dry run), `incrmit help`
      (the overview, `keyNote`), `incrmit help discover` (`parsersNote`), and
      the man page (DESCRIPTION, PARSERS with the recipe, KEYS, `discover`,
      `--key`). DEVELOPMENT.md: §1, §2, §5, §6.1, §6.2, §8.1, §8.2, §9, §9.3,
      §9.6, a new §9.7 for the design, §10, §11, §12, §12.1, §13, and §14.
      `CHANGELOG.md` has `Added`, `Changed`, and `Fixed` under Unreleased.
      `make check` passes, `go test -race ./...` too, and coverage is 97.5%
      (97.4% before); the one YAML branch not covered is the probe disagreeing
      with the file, which no input is known to cause.

## Milestone 42 — Remove the Lock File Between Runs

Every project incrmit has written in keeps a `.incrmit.lock` afterward, and
every user has to add it to `.gitignore`. The file carries no state. It is left
behind because Milestone 29 found that unlinking it on release reopens the race
the lock closes: a run that opened the file just before the holder unlinked it
goes on to lock the now nameless file, a run arriving after creates a fresh
file at the name and locks that, and both proceed. `AcquireWait` reopens the
file on every poll, so the window is the moment between any run's open and its
`flock`, whether it waits or not. The standard fix is to check after locking,
with two rules. Only the holder removes the name, and it does so *before* it
unlocks. A run that gets a lock trusts it only if the name still leads to the
file it locked, and otherwise drops it and starts over. The holder is then the
only one who can remove the name, so once a run's check passes, every newcomer
opens the same file and contends. A crash still skips the release and can
leave the file behind, but the next clean run removes it.

- [ ] Reproduce the race before fixing it, as Milestones 29 and 32 did, so the
      fix has something to prove. Add test seams in `lock.Acquire` after the
      open and after the lock (unexported function variables the tests swap),
      make `Release` unlink the file with no other change, and drive the
      interleaving in `internal/lock`: A holds the lock, B opens the file, the
      seam releases A, B locks the orphaned file, and C creates and locks a
      new one. Run it with C arriving both before and after B takes its lock.
      Goroutines are fair here, since a flock belongs to the open file
      description (Milestone 29). Assert the invariant rather than the damage
      (at most one run holds the project), so the same test proves the fix.
- [ ] Check after locking. Once `tryLock` succeeds, `Stat` the descriptor and
      `Lstat` the path, and keep the lock only when `os.SameFile` says they are
      the same file. A missing name or a different file means a previous
      holder removed the name after this run opened it: unlock, close, and
      open the name again. Use `Lstat`, not `Stat`, so a link swapped in at the
      name never matches and the reopen refuses it as Milestone 32 does. The
      check must come after the lock, not before: until this run holds it, the
      holder can still remove the name between the check and the lock.
- [ ] Decide how many times `Acquire` starts over. Each mismatch means another
      run finished in between, so the loop ends on its own in practice, but a
      name that keeps changing would spin. Recommend a small cap, past which
      `Acquire` reports `ErrContended`: a name that keeps moving means other
      runs keep getting there first, and contention stays the only error
      `Acquire` reports (Milestone 29). `AcquireWait` already retries
      `Acquire`, so a waiting run keeps queueing. Record the decision and the
      reason.
- [ ] Remove the name in `Release` while the lock is still held, then unlock
      and close. Remove it only when the name still leads to the held file
      (`os.SameFile` again): if someone deleted or replaced it by hand, the
      file there now may be another run's lock. Removing after the unlock
      instead reopens the race one step later (a run locks the file between
      the unlock and the removal, passes its check, then loses the name to a
      newcomer), so add a seam between the two steps and a test that fails
      when they are swapped.
- [ ] Delete only a file incrmit made. Milestone 32 decided that a regular file
      already at the lock path is used as the lock but never rewritten, and
      deleting it is a bigger change than rewriting it. Remove the file only
      when it has one name and holds exactly the note or nothing, which is the
      test `writeNote` already applies plus a check of the contents. A file
      holding anything else, or hard-linked from elsewhere, is unlocked and
      left as it was. A file left by a crashed run holds the note, so the next
      clean run removes it.
- [ ] Decide what Windows does. `openLockFile` opens without
      `FILE_SHARE_DELETE`, so the holder cannot delete the file while it, or a
      polling run, has it open. Adding the flag is not enough on its own: with
      classic delete semantics the name stays, "delete pending", until the last
      handle closes, and opening it meanwhile fails with `ERROR_ACCESS_DENIED`.
      `Acquire` would report that as a degraded lock, so a queued run would go
      on unlocked. POSIX-semantics deletion
      (`FILE_DISPOSITION_FLAG_POSIX_SEMANTICS` through
      `SetFileInformationByHandle`, NTFS on Windows 10 1607 and later) removes
      the name at once and is the alternative to evaluate. Recommend keeping
      the file on Windows for this milestone, behind a build-tagged
      `removeLockFile` that does nothing there, while the check after locking
      runs on every platform so the code path is shared. A leftover file costs
      less than an unlocked run. Record the decision and the reason.
- [ ] Confirm the rest of the lock's users still hold with the file now
      transient. The walk keeps skipping `.incrmit.lock` by name, since a run
      in progress or a crashed one still leaves it in the tree. `SweepTemps`
      still runs only under a held lock, a degraded lock has nothing to
      remove, and `preview` and every `--dry-run` still never create the file.
- [ ] Cover the change with tests: the reproduction above now ends with one
      holder in both orders; `Release` removes a file incrmit created and
      leaves one holding other text or a second name; a file left by a crash
      (locked, then closed without `Release`) is reused and then removed; and
      the cap reports `ErrContended`. Through `cli.Main`, `bump`, `undo`, and
      `discover` leave no `.incrmit.lock`, and Milestone 29's eight queued
      `--wait` bumps still land eight increments and eight journal entries,
      then leave no file. Build and vet all six release platforms, run the
      suite under `-race` and `make check`, and record coverage before and
      after.
- [ ] Document the change in `README.md` (the `.gitignore` advice now covers a
      file left only by a crashed run, or by any run on Windows),
      `doc/DEVELOPMENT.md` §6.4 and §8.6, the man page's FILES and CONCURRENT
      RUNS sections, the `internal/lock` package comment, `Release`'s comment,
      and the note itself. Add a `CHANGELOG.md` entry under `Changed`.

## Milestone 43 — Release Tagging Helper (the `tag` command)

The name reads as "increment + commit", but there is no git integration at all:
no tag, no push. `incrmit tag` closes the gap the way `npm version` does, by
running the user's own `git` binary. It lists the versions recorded in
`incrmit.toml`, the user picks one with the arrow keys and Enter (or quits
without touching anything), and incrmit proves that the commit at `HEAD` holds
that version, creates an annotated tag for that commit, and pushes exactly that
tag. Because the real `git` does the work, the user's remotes, credential
helpers, SSH config, `insteadOf` rewrites, signing setup, and hooks all apply
unchanged, and incrmit gains no Go dependency on git. Starting a subprocess is
new to incrmit, so it happens in one package under fixed rules: an argument
list and never a shell, no value git could read as an option, a repository
chosen by the directory and never by inherited variables, and no prompt that
can hang a CI run. Bump stays git-free exactly as it is today.

- [ ] Record the decision in `doc/DEVELOPMENT.md`: incrmit runs git, and only
      git, from a new `internal/git` package, the one package allowed to import
      `os/exec`. Narrow the `depguard` rule in `.golangci.yml` to exempt
      `internal/git` and its subpackages with a `!` pattern under `files`, and
      reword its `desc`, so every other package and every test still calls
      `cli.Main` in-process. `depguard` sees only imports, so add a
      `forbidigo` rule, with the same exemption, for the other ways to start a
      process: `os.StartProcess`, `syscall.Exec`, `syscall.ForkExec`, and
      `golang.org/x/sys/unix.Exec`. A Go git library such as `go-git` was
      rejected: v5.19.2 adds 20 modules (19 new) to a project whose only
      dependencies are `BurntSushi/toml` and `golang.org/x/sys` (a minimal
      program using it builds to 9.1 MB against incrmit's 4.0 MB), and its push
      ignores credential helpers, reads only `Hostname` and `Port` from
      `~/.ssh/config` (no `IdentityFile`, no `ProxyJump`), and signs only with
      an OpenPGP key loaded in-process (no gpg-agent, no SSH signing). Printing
      the git commands for the user to run was rejected too: it kept all of
      that working, but left every check below to the user. Narrow the "Git
      tagging, committing, or changelog generation" exclusion in §1 to
      committing and changelog generation, and replace the "Optional git
      integration" bullet under Future Work with a pointer to this design.
- [ ] Write `internal/git` to the rules `npm version` follows, and state them in
      the package comment. Build each command as an argument list with
      `exec.Command`, never through a shell, so a message holding `'`, `;`, or
      `$(...)` reaches git as text. Resolve `git` once with `exec.LookPath` and
      run that absolute path. Since Go 1.19 a `git` found through a relative
      `PATH` entry such as `.` or `bin` comes back with `exec.ErrDot`, but
      `LookPath` returns the planted path alongside that error, so treat
      `ErrDot` as "git not found" and never run the path. On Windows, refuse a
      `git` that resolves to anything but an `.exe`: a `git.cmd` or `git.bat`
      shim runs under `cmd.exe`, which splits the command line by its own
      rules, so a message holding `&` or `"` could run a command (the
      "BatBadBut" class of bugs). Set `cmd.Dir` on every call, and remove the
      variables `git rev-parse --local-env-vars` prints (`GIT_DIR`,
      `GIT_WORK_TREE`, `GIT_INDEX_FILE`, `GIT_CONFIG_PARAMETERS`, and the rest;
      ask the installed git, so the list matches its version), so the
      directory alone picks the repository: git exports `GIT_INDEX_FILE` to
      its hooks, and `GIT_DIR` when it was started with `--git-dir`, so a run
      started from a hook would otherwise check another index or tag in
      another repository. Pass everything else through untouched (`HOME`,
      `SSH_AUTH_SOCK`, and `GPG_TTY` are what make the user's setup work),
      adding only `GIT_TERMINAL_PROMPT=0` when stdin is not a terminal, so a
      missing credential fails the push instead of waiting for an answer.
      Never hand git incrmit's stdin: git, ssh, and pinentry open the terminal
      themselves to prompt.
- [ ] Never kill git, and keep its output behind incrmit's terminal guarantee.
      `exec.CommandContext` kills on cancel with `SIGKILL`, which git cannot
      catch, so a `git tag` stopped mid-write can leave
      `refs/tags/<name>.lock` behind and fail every later run until someone
      deletes it by hand: run git without a context, let a Ctrl-C reach it
      through the terminal's process group, and wait for it to exit. Start
      every command with `--no-pager`, and every read-only one with
      `--no-optional-locks`, so a check never takes `index.lock` from a git the
      user is running at the same time. Read only machine-readable output
      (porcelain formats with `-z`, `rev-parse`, `for-each-ref`, exit status)
      and never git's messages, which change with the locale. Pipe git's own
      stdout and stderr through the `terminalWriter` (§9.5) instead of handing
      it the file descriptors: a remote's `remote:` lines reach the terminal
      through git and can carry escape sequences. A missing git and a failed
      command both exit `1`, after git's own stderr.
- [ ] Keep every value from being read as an option. The tag name is the
      validated prefix followed by a version token, so it never starts with
      `-`; a remote name that starts with `-` is refused; and both still go
      after `--end-of-options`, git's marker that what follows are arguments.
      Name refs in full (`refs/tags/v1.2.4`, `HEAD`) and commits by hash,
      which no option starts with, and pass file paths after `--` with
      `--literal-pathspecs`, so a name holding `*` or starting with `:` is a
      file and not a pattern. `--end-of-options` arrived in git 2.24 (2019):
      document that as the minimum. An older git rejects the marker with its
      own unknown-option error, so no version probe is needed.
- [ ] Add the `tag` subcommand: dispatch it in `cli.Main` alongside `discover`,
      `preview`, and `undo`, add it to the top-level overview and to
      `incrmit help tag` via the centralized text in `internal/cli/help.go`,
      and reject unknown arguments with exit `2` the way the other commands do.
      `--dry-run` runs every read-only check below and prints the commands
      instead of running them. Thread stdin through `cli.Main` as an
      `io.Reader`, with a way to ask whether it is a terminal, so the picker can
      be tested in-process.
- [ ] Build the candidate list from `incrmit.toml`: collect the versions the
      `[[files]]` entries pin (`FileEntry.Token()`, so a prerelease is offered
      as `1.2.4-rc.1`, not `1.2.4`), group them the way `preview` judges drift
      (`precedenceKey`), order the groups by `version.Compare` with the newest
      first, and show each one with the files that hold it. Entries that
      differ only in the `v` prefix or in build metadata name one release, and
      this repository's own config pins both `0.3.9` and `v0.3.9` in
      `README.md`: listed apart, they would be two candidates, so `--yes`
      could never pick, and one of them would tag `vv0.3.9`. Name the tag from
      the version without its source `v` and without build metadata, so the
      prefix is the only `v`. When entries disagree, mark the minority rows the
      way `preview` marks drift so a half-finished bump is visible before
      anything is tagged. A missing or empty config reuses the existing "run
      discover" error path and its exit code.
- [ ] Implement the selection as an arrow-key list, not a typed answer: the
      candidates are drawn with one row highlighted, the user moves the
      highlight with the up and down arrows and confirms with Enter, and nothing
      is typed or echoed. This needs the terminal in raw mode — use
      `golang.org/x/term` (`IsTerminal`, `MakeRaw`, `Restore`), which is small
      and stdlib-adjacent; a full TUI framework would dwarf the tool and
      hand-rolled termios syscalls would mean per-platform code. Restore the
      terminal on every exit path, including a `SIGINT` handler, so a cancelled
      prompt never leaves the shell without echo, and before any git command
      starts, so a passphrase prompt from git, ssh, or gpg echoes normally.
      Draw the picker on stderr, never stdout. Its cursor and erase sequences
      cannot pass through the `terminalWriter`, which would escape them, so the
      picker writes around it and the guarantee does not cover what it draws:
      render every file name through `displayName`, since the names come from
      the project.
- [ ] Define the key map and the redraw, and keep both testable: arrows arrive
      as `ESC [ A` / `ESC [ B`, so decode key events from a byte stream in a
      pure function that tests feed fixed sequences; accept `k`/`j` as aliases,
      wrap at the ends of the list, confirm on Enter, and cancel on `q`, Esc, or
      Ctrl-C — a cancel exits `1` having tagged and pushed nothing, and says so
      on stderr, so `incrmit tag && …` stops there. Redraw by moving up the N
      rows just written and clearing each line (`ESC [ A`, `ESC [ 2 K`) rather
      than clearing the screen, so scrollback survives, and render frames to an
      `io.Writer` so they can be golden-tested the way `preview` output is in
      `internal/cli/testdata`.
- [ ] Make the command usable from CI: `--version <token>` selects a candidate
      without prompting (unknown token → exit `2`, listing what is available),
      and `--yes`/`-y` accepts the sole candidate when there is exactly one.
      When stdin is not a terminal — a pipe, a CI runner, `< /dev/null` — the
      arrow-key prompt cannot run at all, so never attempt raw mode there: fail
      with exit `2` and the usage hint naming `--version` instead of blocking on
      a prompt that no one can answer.
- [ ] Check the repository before the picker opens, so nothing is chosen only to
      fail. Find the work tree with `git rev-parse --show-toplevel` from the
      config's directory (a config in a subdirectory works), resolve `HEAD` to
      a commit hash once and use that hash from then on, and, unless
      `--no-push`, require the remote to be one that `git remote` lists, so a
      typo or a URL fails before a tag exists rather than after. Pushing a tag
      publishes the commit it names, with all its history, whether or not any
      branch holding it was pushed, and the release workflow builds whatever
      the tag names; so, unless `--no-push`, also refuse a commit that no ref
      under `refs/remotes/<remote>/` contains
      (`git for-each-ref --contains=<hash> refs/remotes/<remote>/`), with the
      hint to push the branch first. Then refuse when a file any `[[files]]`
      entry pins has uncommitted changes:
      `git status --porcelain=v1 -z --untracked-files=all --ignored=matching`
      on those paths must print nothing. That check is the early, clear error
      ("commit the bump first"), not the guarantee: `git status` reports a
      file marked `assume-unchanged` or `skip-worktree` as clean whatever it
      holds, and a tracked symlink as clean whatever its target holds (all
      three tried); the check at the commit below is what proves the version.
      Unlike `npm version`, other changes in the work tree are not checked,
      since they are not in the tag either, and neither is `incrmit.toml`,
      which is often git-ignored (this repository ignores `*.toml`). Each
      refusal exits `1` naming the files and the fix.
- [ ] Prove the version at the commit, after the selection and immediately
      before tagging. For each pinned file, read its mode with
      `git ls-tree -z <hash> -- ./<path>` and its contents with
      `git cat-file blob <hash>:./<path>`, both run from the config's
      directory, so no path arithmetic against the top level is needed
      (macOS's `/var` → `/private/var` link would break it). Refuse a symlink
      (mode `120000`), a submodule (`160000`), or a path missing at the
      commit; check the size first with `git cat-file -s` against the limit
      `--max-file-size` sets, and offer that flag on `tag` too; then parse the
      blob with the parser a bump uses. Tag only when every file holds the
      selected version, and tag the hash, not `HEAD`, so a commit or checkout
      made while the picker was open cannot change what is tagged.
- [ ] Build the commands: an annotated tag named with a configurable prefix
      (default `v`, so `v1.2.4`) settable as `--prefix` or `[git] tag_prefix`
      in `incrmit.toml`, with a `--tag-message` whose default template is
      `Release {{.Version}}`, pushed to `--remote` (default `origin`):
      `git tag -a -m 'Release 1.2.4' --end-of-options v1.2.4 <hash>`, then
      `git push --no-follow-tags --recurse-submodules=no --end-of-options
      origin refs/tags/v1.2.4:refs/tags/v1.2.4`. Push exactly the one tag
      refspec — never a branch, never a bulk `--tags` or `--follow-tags` — and
      switch off what the user's config can add to it: with
      `push.followTags = true`, a push of one tag also sends every annotated
      local tag on the commit's history that the remote lacks (tried), and
      `push.recurseSubmodules` can push submodule commits. `--no-push` runs
      only `git tag`. `--sign` runs `git tag -s`, so git applies
      `user.signingkey` and `gpg.format` (OpenPGP or SSH) as configured, and
      `tag.gpgSign = true` works with no flag at all. No `--force` is offered:
      an existing tag elsewhere is refused below, and the remote rejects an
      existing remote one.
- [ ] Validate before anything runs. Check the prefix against git's ref-name
      rules (no leading `-`, whitespace, `..`, `~`, `^`, `:`, `?`, `*`, `[`,
      `\`, or trailing `.lock`) and exit `2` naming the rule, then confirm the
      full name with `git check-ref-format refs/tags/<name>`, so git's own rules
      have the last word. Refuse a remote that starts with `-`, an empty
      message, and a message holding a NUL byte, which no argument list can
      carry, with exit `2` too.
- [ ] Run the commands, echoing each to stderr first. Before tagging, look up
      `refs/tags/<name>` with `git for-each-ref` (`%(objecttype)`,
      `%(*objectname)`, `%(contents:signature)`). If it is an annotated tag of
      the selected commit, and a signed one when `--sign` is given, skip
      `git tag` and push it as it is, saying so on stderr, so running the
      command again after a failed push finishes the job. Refuse, with exit
      `1` naming what was found, a lightweight tag, an unsigned one under
      `--sign`, or a tag of any other commit. When the push fails, keep the
      local tag, say that it exists and that the same command retries the
      push, and exit `1`.
- [ ] Keep the output scriptable: on success stdout is exactly the tag name
      (`v1.2.4`), and everything else (the picker, the echoed commands, notes,
      and git's own stdout and stderr) goes to stderr. `--dry-run` prints the
      commands on stdout instead, quoted for a POSIX shell (`'` becomes
      `'\''`), so they can still be run by hand; they name the commit by hash,
      so running them later still tags the commit that was checked. Document
      that the quoting targets POSIX shells.
- [ ] Say what incrmit trusts, as `npm version` does: the first `git` on
      `PATH`, and the repository. A repository's config and hooks can run
      commands (`core.fsmonitor` on `git status`, a `pre-push` hook on the
      push), so running `incrmit tag` in a repository trusts it exactly as
      running `git status` there does. Leave git's `safe.directory` check in
      force: never pass `-c safe.directory=…`, or any other `-c` override, so
      git still refuses a repository another user owns.
- [ ] Add an `internal/git/gittest` package, inside the `depguard` exemption,
      that creates a repository in `t.TempDir()` with a bare one beside it as
      `origin` (a local path, so no network), and isolate it from the
      developer's repository as well as from their configuration. The
      pre-commit hook runs `make check` inside `git commit`, where git exports
      `GIT_INDEX_FILE` pointing at the index being committed, and a fixture's
      `git add` under it writes into that commit (tried: the commit then fails
      on the fixture's object). So clear the variables
      `git rev-parse --local-env-vars` prints, set `GIT_CONFIG_GLOBAL`,
      `GIT_CONFIG_NOSYSTEM`, a temporary `HOME`, `GIT_CEILING_DIRECTORIES` at
      the temporary root, and `GIT_ALLOW_PROTOCOL=file`, so no test can reach
      a network remote even by mistake, and assert that
      `git rev-parse --show-toplevel` is inside `t.TempDir()` before any
      command that writes. Fail rather than skip when git is missing, so CI
      cannot pass by skipping. Unit-test the argv and environment each command
      is built with, without running git, and the Windows `.exe` rule as a
      pure function of the resolved path, so it is tested on every platform.
- [ ] Cover the command through `cli.Main` against `gittest` repositories: the
      defaults (an annotated tag with the message, present in `origin`),
      `--prefix`, `[git] tag_prefix`, `--remote` to a second bare repository,
      `--no-push`, `--sign` through `--dry-run`, a message holding `'`, `;`,
      `$(...)`, and a leading `-` arriving in the tag unchanged, and a `v` and
      a bare entry of one version making one candidate tagged `v0.3.9`. Cover
      the stdout/stderr split (stdout exactly the tag name, or exactly the
      quoted commands under `--dry-run`) and each refusal: no repository, no
      commit, a commit not on the remote, a pinned file modified, staged,
      untracked, and ignored (one case each) while an unrelated change is
      allowed, a pinned file marked `assume-unchanged` or `skip-worktree` or
      tracked as a symlink (each refused by the check at the commit), an
      unknown remote, a tag at another commit, a lightweight tag, and one
      prefix per rejected form. `push.followTags = true` in the repository's
      config still pushes only the one tag. A tag already at the commit is
      pushed without being recreated, and a failed push (an `origin` path that
      no longer exists) keeps the local tag, which a second run pushes once
      `origin` is back. `GIT_DIR` and `GIT_INDEX_FILE` set to another
      repository change nothing. Refuse a missing git (an empty `PATH`) and a
      fake `git` reached through `PATH=.` (`exec.ErrDot`). Drive the selector
      through the decoder rather than a real terminal — fixed byte sequences
      for down-down-Enter, wrap-around at both ends, `q`, Esc, and Ctrl-C (each
      cancel exiting `1`) — with golden frames for the rendered list, and a
      file name holding an escape sequence drawn quoted. Cover the
      single-candidate and `--version` paths, an unknown token, drift marking,
      a missing config, and a non-TTY stdin refusing to prompt. Build and vet
      all six release platforms.
- [ ] Document the command in `README.md`, the `incrmit(1)` man page, and
      `doc/DEVELOPMENT.md` — including a release recipe (bump, commit and push
      the branch, `incrmit tag`), the selector's keys, the `--version` flag CI
      needs because the prompt requires a TTY, the checks made before tagging
      (the commit on the remote, the version proven at the commit), the
      stdout/stderr contract, `--dry-run` for running the commands by hand, the
      trust model, the git variables incrmit clears, and the git 2.24 minimum,
      and the fact that authentication and signing are whatever the user's
      `git` does — and add a `CHANGELOG.md` entry under `Added`. Switch step 3
      of the release checklist in §13 to `incrmit tag`. Confirm the
      `govulncheck` gate from Milestone 26 still passes with the `x/term` tree
      in `go.sum`.

## Milestone 44 — Conventional-Commit Bump Inference (`--auto`)

Reading the commits since the last tag and inferring the bump component turns
`discover` + language-agnostic + single-binary from a narrow story into a real
one: no other tool does automatic inference *and* arbitrary-file rewriting
without a per-ecosystem plugin. `--auto` reads the history through Milestone
41's `internal/git`, under the same rules, and runs only read-only commands: it
finds the last release tag and reads the commit messages since it. A bump still
never writes to the repository.

- [ ] Add `--auto` to the bump command: find the work tree from the project
      directory (`projectDir`, so `--file` works too) as `incrmit tag` does,
      read the messages since the last release tag with
      `git log --format=%B%x00 refs/tags/v1.2.3..HEAD --`, split them on the
      NUL bytes as they stream in rather than reading the whole log into
      memory, and infer the component. The range is built from full ref names,
      so it can never be read as an option. Outside a repository, exit `1`
      saying that `--auto` needs the history and `--major`, `--minor`, and
      `--patch` do not.
- [ ] Make sure the history is the project's. The work tree is found by walking
      up from the project directory, so a project that is not a repository but
      sits inside one (a home directory kept in git, say) would be inferred
      from that repository's commits. Before reading the log, require every
      target to exist at `HEAD` (`git cat-file -e HEAD:./<path>` from the
      config's directory), and exit `1` naming the first one that does not.
- [ ] Decide how to find the last release tag. Recommend
      `git for-each-ref --merged=HEAD --format=%(refname) refs/tags/v*` (with
      the config's `[git] tag_prefix` from Milestone 43, default `v`), keeping
      the names whose remainder after the prefix parses as a version and taking
      the highest by `version.Compare`. Prefer it over
      `git describe --tags --abbrev=0`, which reports a repository with no tag
      only as an error message that would have to be parsed and changes with
      the locale; `for-each-ref` prints nothing instead. The prefix passed
      Milestone 43's ref-name rules, which refuse `*`, `?`, and `[`, so it goes
      into the pattern as it is. Decide too whether a prerelease tag counts as
      the last release: recommend not, so the commits since the last stable
      release decide the component. Record the decisions and the reasons.
- [ ] Refuse a shallow clone. When `git rev-parse --is-shallow-repository`
      prints `true`, exit `1` with a hint naming `fetch-depth: 0`: a truncated
      history can hide the last tag or the commits since it, and the bump
      inferred from it is then too small, with no error at all.
- [ ] Implement the inference rules and document them: a `feat:` commit implies
      minor, a `fix:`/`perf:` commit implies patch, and `BREAKING CHANGE:` in a
      trailer or a `!` before the colon implies major. The highest match wins.
      Non-conforming commits are ignored, not errors.
- [ ] Decide and document what happens when nothing is inferable (no commits
      since the tag, or no conforming commits): recommend exiting `0` with "no
      version-relevant commits; nothing to bump" and writing nothing, with
      `--auto --fallback patch` available for CI that wants a bump regardless.
      With no release tag yet, read the whole history of `HEAD`
      (`git log --format=%B%x00 HEAD --`), so the first release needs no
      special form. The trailing `--` matters here: without it, a file named
      `HEAD` in the work tree makes the argument ambiguous and git refuses
      (tried).
- [ ] Reject `--auto` combined with an explicit `--major`/`--minor`/`--patch`
      with exit code `2` rather than silently letting one win.
- [ ] Make `--auto --dry-run` explain the decision: print the tag it measured
      from (or that there is none and the whole history was read), the
      inferred component, and the subject line of each commit that drove it,
      so the inference is auditable. The subjects are whatever contributors
      wrote, so they go through the `terminalWriter` like every other line.
- [ ] Test the parsing apart from the history. Feed fixed byte streams to the
      parser: each rule, the highest-wins precedence, the `!` and trailer forms
      of a breaking change, CRLF line endings, empty input, and records split
      on NUL with the newline git adds between them. Through `cli.Main`, use
      Milestone 43's `gittest` repositories: no tag (the whole history read),
      the highest of several version tags, a tag on a branch not merged into
      `HEAD` ignored, a tag with the prefix but no version ignored, a
      prerelease tag per the decision above, no conforming commits, the
      `--fallback` path, `--file`, a file named `HEAD` in the work tree, a
      project inside a repository that does not track it refused, a shallow
      clone refused (`git clone --depth 1` from a `file://` URL, since a plain
      local path ignores `--depth`), a directory outside any repository, and a
      commit subject holding an escape sequence printed escaped.
- [ ] Document the rules and a full CI recipe in `README.md`, the man page, and
      `doc/DEVELOPMENT.md` — `incrmit --auto`, then a commit and a push of the
      branch, then `incrmit tag --yes`; how the last release tag is found, with
      the same `[git] tag_prefix` as `incrmit tag`; and `fetch-depth: 0` on
      `actions/checkout`, without which the run is refused as a shallow clone.
      Say that a tag pushed with the workflow's default `GITHUB_TOKEN` does not
      start other workflows, so the tag-triggered release workflow
      (Milestone 13) never runs from it: release in the same job, or push with
      a deploy key limited to the repository, rather than a personal access
      token that reaches every repository its owner can. Add a `CHANGELOG.md`
      entry under `Added`.

## Milestone 45 — Code and Repository Hygiene

Small cleanups worth doing before v1.0.0 freezes the surface.

- [ ] Fix the decorative `--patch`/`-p` flag: it defaults to `true`, so
      `resolveBump` cannot distinguish "explicitly requested" from "not given" —
      hence the `_ = patch` discard in `internal/cli/cli.go`. Default it to
      `false` and let the `default:` branch handle the none-given case, then
      drop the discard. Confirm `--patch`, no flag, and `--patch=false` all
      still behave as documented.
- [ ] Remove the dead exported surface in `internal/files`: `ApplyBump`,
      `ReadVersion`, `SetVersion`, and `SetKnownVersion` are unreferenced
      outside tests, and the package is `internal/` so nothing external can ever
      call them. Delete them with their tests (`SetVersion` is only reachable
      through `ApplyBump`), or document why one is deliberately kept.
- [ ] Narrow the blanket `*.toml` entry in `.gitignore`. It forced this repo's
      own `incrmit.toml` to be force-added, and anyone who copies the pattern
      will silently fail to commit their config for a TOML-configured tool.
      Ignore only what actually needs ignoring (e.g. `.incrmit.state.toml`) and
      confirm `git check-ignore -v incrmit.toml` reports nothing afterward.
- [ ] Remove the stray `incrmit copy.toml` from the working tree, and confirm
      the untracked build artifacts sitting in the repo root (the `incrmit`
      binary, `coverage.out`, `.DS_Store`) are all covered by `.gitignore`.
- [ ] Add a `make tidy` or equivalent check — or a CI step — that fails when a
      build artifact or stray file appears at the repo root, so the tree stays
      clean without relying on remembering.

## Milestone 46 — Shell Completion (bash, zsh, fish)

incrmit has a handful of commands and a fixed set of flags, so tab completion
is one small hand-written script per shell rather than one generated by a CLI
framework. Cobra, the usual source of generated completion, would add
`spf13/pflag` and `mousetrap` to a module that keeps its dependencies few on
purpose (§2, §13). pflag would also break the single-dash long forms: the help
documents `-version`, `-help`, and `-no-banner`, and the flag package accepts
one dash for every flag, but pflag reads a single dash as a run of shorthands,
so `-config x.toml` becomes `-c onfig` followed by a stray argument. The
scripts are embedded in the binary, printed by `incrmit completion`, and
installed by every package.

- [ ] Record the decision in `doc/DEVELOPMENT.md` §7: hand-written scripts over
      a framework, for the reasons above, and static ones. A script never runs
      `incrmit` or reads the config to find what to offer, so pressing Tab in a
      tree someone else wrote runs no program and reads no file in it.
- [ ] Write the scripts in a new `internal/completion` package that embeds
      them with `//go:embed` (an embed pattern cannot reach outside its
      package's directory): `incrmit.bash`, `_incrmit` for zsh, and
      `incrmit.fish`. In the first position each offers the commands
      (`discover`, `preview`, `undo`, `version`, `help`, `completion`), the
      bump flags, since bump is the default command, and `--version`/`-v`.
      After a command it offers that command's flags, long and short, with
      `-h`/`--help`; after `help`, a command name, `bump`, or `--no-banner`;
      after `completion`, `bash`, `zsh`, or `fish`. For flag values: a file
      after `--config`, `--file`, and `--output`, preferring `*.toml` for
      `--config`; a directory after `--path`; `alpha`, `beta`, and `rc` after
      `--pre`, as suggestions only, since any identifier is accepted; nothing
      after `--max-file-size`. Add `tag` and `--auto` when Milestones 43 and 44
      land; the drift test below fails until every script has them. Target
      bash 4.2 or later, which current bash-completion releases require; the
      3.2 that macOS ships as `/bin/bash` is out of scope.
- [ ] Leave the rules for combining flags to the binary: offer `--release`
      after `--pre` and let incrmit refuse it with exit `2`. Copied into three
      scripts, those rules would each have to track `resolveBump`.
- [ ] Add the `completion` command: `incrmit completion bash` (or `zsh`, or
      `fish`) prints that script to stdout and exits `0`; a missing or unknown
      shell, or an extra argument, exits `2` with the command's help on
      stderr, as the other commands do. Dispatch it in `cli.Main`, and add it
      to the overview and to `incrmit help completion` through the
      centralized text in `internal/cli/help.go`. The scripts hold only
      printable ASCII, newlines, and tabs, which `terminalWriter` passes
      unchanged; a test compares each shell's output with its embedded file
      byte for byte, which catches a script edited to hold a character the
      writer would escape.
- [ ] Keep the scripts in step with the flags. Add a test that reads the
      commands from the overview (or from Milestone 47's command table, if it
      has landed) and each command's flags from its flag block in
      `internal/cli/help.go` (`bumpFlags`, `discoverFlags`, and the rest), and
      fails when a script leaves one out or offers a `--flag` the help does not
      list. The help text is already the single source of truth, so a flag
      added there later fails the build until it completes too.
- [ ] Test the scripts in their own shells. The `no-os-exec` rule in
      `.golangci.yml` keeps `os/exec` out of the Go tests, so add
      `scripts/test-completion.sh` and a `make completion-test` target, and
      run it in CI on `ubuntu-latest` with `zsh` and `fish` installed. Locally
      it skips a shell that is not installed and says so; in CI it requires
      all three. Check each script's syntax (`bash -n`, `zsh -n`,
      `fish --no-execute`). For bash, source the script, set `COMP_WORDS` and
      `COMP_CWORD`, call the completion function, and compare `COMPREPLY`; for
      fish, compare the output of `complete --do-complete 'incrmit dis'`. For
      zsh, load the function under `compinit` with an empty `ZDOTDIR` and
      confirm it is registered for `incrmit`; driving zsh's completion from a
      script needs `zpty`, which is more machinery than the check needs.
      Cover the first position, a command's flags, `help <command>`,
      `completion <shell>`, both kinds of value (`--config` offering a `.toml`
      file, `--path` offering only directories), and a flag after a value
      (`incrmit --file x --<Tab>`).
- [ ] Install the scripts with the packages. In `packaging/nfpm.yaml`, put
      bash's at `/usr/share/bash-completion/completions/incrmit` and fish's at
      `/usr/share/fish/vendor_completions.d/incrmit.fish`. zsh's goes where
      each distribution's zsh looks, and the two differ: Debian's reads
      `/usr/share/zsh/vendor-completions` and Fedora's
      `/usr/share/zsh/site-functions`, so give the entry a `packager:` per
      format and confirm both in a container with `zsh -c 'print -l $fpath'`.
      The macOS package (`scripts/build-pkg.sh`) installs under
      `/usr/local/share` (`bash-completion/completions`, `zsh/site-functions`,
      `fish/vendor_completions.d`); confirm each shell finds its script there,
      including Homebrew's shells on Apple Silicon, whose prefix is
      `/opt/homebrew`. Add a `completions/` directory to the `.tar.gz`
      archives. Every script is mode `0644`.
- [ ] Document completion in `README.md`, `doc/USAGE.md`, and the man page
      (`completion` under COMMANDS, and a SHELL COMPLETION section). After a
      package install there is nothing to do but open a new shell (bash needs
      the `bash-completion` package). Otherwise: for bash,
      `source <(incrmit completion bash)` in `~/.bashrc`; for zsh,
      `incrmit completion zsh > ~/.zfunc/_incrmit` with `~/.zfunc` added to
      `fpath` before `compinit` runs; for fish,
      `incrmit completion fish > ~/.config/fish/completions/incrmit.fish`.
      Add a `CHANGELOG.md` entry under `Added`.

## Milestone 47 — Suggest the Nearest Command

A mistyped command (`incrmit discvoer`, `incrmit prevew`) exits `2` with
`unknown command` and a pointer to `incrmit help`, which leaves the user to
find the typo. Naming the nearest command saves the round trip. With fewer
than ten commands the check is a plain edit distance to each name: a few dozen
lines and no dependency. The suggestion is only printed. incrmit never runs
the command it guessed, as git's `help.autoCorrect` can, because a guess can
land on a command that writes: `incrmit und` run as `undo` would revert a bump
nobody asked to revert.

- [ ] Keep the command names in one place. `cli.Main` and `runHelp` each
      switch over them today, and the suggester needs the same list, as does
      Milestone 46's drift test. Recommend one table of commands in overview
      order (name, run function, help text) that `cli.Main`, `runHelp`, and
      the suggester all read, so a command cannot be added to one and missed
      by another. `bump` is in the table for `incrmit help bump` but has no
      name `cli.Main` dispatches (see below).
- [ ] Add `suggest(name string, candidates []string) []string` in a new
      `internal/cli/suggest.go`. It returns the candidates that start with
      `name`, when `name` is at least two runes long (`di`, `ver`, which are
      too far from `discover` and `version` by distance alone); failing that,
      the candidates at the smallest optimal-string-alignment distance from
      `name`, if that distance is at most `2`. Either way they come back in
      table order. Use optimal string alignment rather than plain Levenshtein
      because two swapped neighbors are the commonest typo, and they should
      cost one edit, not two. Compare case-folded, so `Discover` suggests
      `discover`, and count runes, not bytes, so a non-ASCII typo is one edit.
      Return early when `name` is more than two runes longer than every
      candidate: none can then be within distance `2` or start with it, so a
      megabyte-long argument costs nothing.
- [ ] Recognize a bump flag typed without its dashes. `incrmit minor` and
      `incrmit major` are easy mistakes, since the flags read as words, and no
      command is close to them. Before the distance check, when `name` is
      exactly one of the bump command's long flags (taken from the flag set,
      not a second list), suggest it with its dashes, as `--minor`. This runs
      first so `pre` suggests `--pre`, not `preview` by prefix.
- [ ] Handle `incrmit bump`. `bump` is a topic of `incrmit help bump` but not a
      command `cli.Main` dispatches, so `incrmit bump --minor` is an unknown
      command today even though the help names it. Decide between dispatching
      it as an alias of the default command and keeping it unknown with a hint
      of its own, saying that bump is the default command, run as
      `incrmit --minor`. Recommend the hint: an alias is a second spelling to
      document and keep forever, and the hint teaches the one that exists.
      Record the decision in `doc/DEVELOPMENT.md` §7.
- [ ] Print the suggestion between the two lines printed today, in the same
      words from both places that report an unknown command (`cli.Main` and
      `runHelp`):

      ```text
      incrmit: unknown command "discvoer"
      Did you mean "discover"?
      Run 'incrmit help' to see available commands.
      ```

      With several matches, list them one per line, indented, under
      `The most similar commands are:`, as git does. With none, print the two
      lines of today. The exit code stays `2` and stdout stays empty. `runHelp`
      offers its own topics, which include `bump`, and skips the flag rule,
      since a flag is not a help topic. The name the user typed is already
      quoted with `%q` and goes through `terminalWriter`; the suggestions are
      incrmit's own names.
- [ ] Test `suggest` with a table: one substitution, insertion, deletion, and
      transposition each; mixed case; a two-rune prefix and a one-rune one; a
      tie; a name too far from everything; the empty string; a name three
      runes longer than any command; non-ASCII and invalid UTF-8. Add a
      `FuzzSuggest` target beside the existing ones that checks it never
      panics and that every name it returns is a candidate meeting the rules
      above. Through `cli.Main`, cover a typo of each command, a typo after
      `help`, `incrmit minor`, `incrmit pre`, `incrmit bump --minor`, and a
      typo holding an escape sequence, printed escaped: each exits `2` with
      nothing on stdout. Confirm the existing unknown-command tests in
      `internal/cli/help_test.go`, `internal/cli/cli_test.go`, and
      `main_test.go` pass unchanged.
- [ ] Document the suggestion in `doc/USAGE.md` and under EXIT STATUS in the
      man page, where exit `2` is described; record the matching rules and the
      decision never to run a guess in `doc/DEVELOPMENT.md` §7; and add a
      `CHANGELOG.md` entry under `Added`.

## Milestone 48 — v1.0.0 Release: Publish

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

## Milestone 49 — apt / dnf Repo via GitHub Pages

Host signed apt and dnf repositories on GitHub Pages so users can
`apt install incrmit` / `dnf install incrmit` after adding the repo once.
Reuses the existing nFPM `.deb` / `.rpm` artifacts from the release workflow;
does not replace per-release download instructions.

- [ ] Choose tooling for repo metadata and document the rationale in
      `doc/DEVELOPMENT.md`: apt (`reprepro`, `aptly`, or `dpkg-scanpackages` +
      `apt-ftparchive`) and dnf (`createrepo_c`). Prefer tools that run cleanly
      on `ubuntu-latest` in Actions.
- [ ] Decide the public Pages URL and on-disk layout (e.g.
      `https://sasmaq.github.io/incrmit/deb/` with `pool/` + `dists/`, and
      `…/rpm/$basearch/` with `repodata/`). Enable GitHub Pages for the chosen
      source (branch such as `gh-pages`, or a `docs/` / Actions upload).
- [ ] Create a long-lived GPG key for signing apt `Release` / `InRelease` (and
      preferably RPM packages / `repomd.xml`). Store the private key and
      passphrase as GitHub Actions secrets; publish the public key at a stable
      Pages URL (e.g. `…/incrmit.gpg` and/or `…/RPM-GPG-KEY`).
- [ ] Add a release-workflow job (or extend the existing publish job) that,
      after `.deb` / `.rpm` are built:
      1. Checks out or downloads the current Pages site content.
      2. Imports the new packages into the apt and yum trees.
      3. Regenerates metadata (`Packages` / `Release` / `InRelease`,
         `repodata/`).
      4. Signs with the GPG secret.
      5. Publishes the updated tree to Pages.
- [ ] Keep older package versions in the repo (or document a retention policy)
      so `apt` / `dnf` upgrades remain deterministic across releases.
- [ ] Add a small install helper or copy-paste snippets: Debian/Ubuntu
      `sources.list.d` entry with `signed-by=` pointing at the published keyring,
      and a Fedora/RHEL `.repo` file with `baseurl`, `gpgcheck=1`, and `gpgkey=`.
- [ ] Document end-user install in `README.md` (add repo → `apt update && apt
      install incrmit` / `dnf install incrmit`) and the maintainer flow in
      `doc/DEVELOPMENT.md` (secrets, Pages branch, how a tag updates the repo).
- [ ] Confirm Pages size/bandwidth stays reasonable as versions accumulate;
      prune or archive old packages if the tree grows too large.
