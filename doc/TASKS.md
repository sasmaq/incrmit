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
      version is bumped to `1.0.0` in Milestone 33.
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

## Milestone 29 — Git Integration (the `push` command)

The name reads as "increment + commit", but there is no git integration at all:
no tag, no push. The gap is closed by one interactive command rather than a set
of bump flags: `incrmit push` lists the versions recorded in `incrmit.toml`,
the user picks one with the arrow keys and Enter (or quits without touching
anything), and the tool tags that version and pushes the tag. Bump itself stays
git-free — no `--commit`, no implicit tagging — so the default bump keeps
working in a non-git directory exactly as it does today, and committing the
bump is left to the user (or to a later milestone). Git access goes
through `go-git` as a library, not through the `git` binary, so the tool keeps
working where `git` is absent and the behavior is the same on every platform.

- [ ] Add an `internal/git` package built on `github.com/go-git/go-git/v5`
      (v5 is the current stable line; v6 is still alpha). This is the first
      dependency the project has taken since `BurntSushi/toml` and by far the
      largest — record the decision and its cost in `doc/DEVELOPMENT.md`
      alongside the smaller `x/term` one the selector needs, and keep every
      go-git import inside this one package so the rest of the tree stays
      unaware of it. Expose a narrow surface (open repo, HEAD hash, tree status,
      list tags, create annotated tag, push a ref) behind an interface the CLI
      tests can fake, and return typed errors for "not a repository", "tag
      already exists", "dirty tree", "no such remote", and "authentication
      failed".
- [ ] Add the `push` subcommand: dispatch it in `cli.Main` alongside `discover`,
      `preview`, and `undo`, add it to the top-level overview and to
      `incrmit help push` via the centralized text in `internal/cli/help.go`,
      and reject unknown arguments with exit `2` the way the other commands do.
- [ ] Build the candidate list from `incrmit.toml`: collect the distinct tokens
      the `[[files]]` entries pin (`FileEntry.Token()`, so a prerelease is
      offered as `1.2.4-rc.1`, not `1.2.4`), order them by `version.Compare`
      with the newest first, and show each one with the files that hold it.
      When entries disagree, mark the minority rows the way `preview` marks
      drift so a half-finished bump is visible before anything is tagged. A
      missing or empty config reuses the existing "run discover" error path and
      its exit code.
- [ ] Implement the selection as an arrow-key list, not a typed answer: the
      candidates are drawn with one row highlighted, the user moves the
      highlight with the up and down arrows and confirms with Enter, and nothing
      is typed or echoed. This needs the terminal in raw mode — use
      `golang.org/x/term` (`IsTerminal`, `MakeRaw`, `Restore`), which is small
      and stdlib-adjacent; a full TUI framework would dwarf the tool and
      hand-rolled termios syscalls would mean per-platform code. Restore the
      terminal on every exit path, including a `SIGINT` handler, so a cancelled
      prompt never leaves the shell without echo.
- [ ] Define the key map and the redraw, and keep both testable: arrows arrive
      as `ESC [ A` / `ESC [ B`, so decode key events from a byte stream in a
      pure function that tests feed fixed sequences; accept `k`/`j` as aliases,
      wrap at the ends of the list, confirm on Enter, and cancel on `q`, Esc, or
      Ctrl-C — a cancel exits `0` having written nothing and says so. Redraw by
      moving up the N rows just written and clearing each line
      (`ESC [ A`, `ESC [ 2 K`) rather than clearing the screen, so scrollback
      survives, and render frames to an `io.Writer` so they can be golden-tested
      the way `preview` output is in `internal/cli/testdata`.
- [ ] Make the command usable from CI: `--version <token>` selects a candidate
      without prompting (unknown token → exit `2`, listing what is available),
      and `--yes`/`-y` accepts the sole candidate when there is exactly one.
      When stdin is not a terminal — a pipe, a CI runner, `< /dev/null` — the
      arrow-key prompt cannot run at all, so never attempt raw mode there: fail
      with exit `2` and the usage hint naming `--version` instead of blocking on
      a prompt that no one can answer.
- [ ] Create the tag: an annotated tag on the current `HEAD`, named with a
      configurable prefix (default `v`, so `v1.2.4`) settable as `--prefix` or
      `[git] tag_prefix` in `incrmit.toml`, with a `--tag-message` whose default
      template is `Release {{.Version}}`. Refuse to overwrite an existing local
      or remote tag — exit `1` naming the tag; no `--force` is offered.
- [ ] Guard against tagging something that does not exist: refuse when the
      worktree is dirty (unless `--allow-dirty`), and refuse when the files at
      `HEAD` do not actually hold the selected token, so a version that was
      bumped but never committed cannot be tagged. Both messages name the
      offending files; document the check and its escape hatch.
- [ ] Push the tag: `--remote` (default `origin`), pushing exactly the one tag
      refspec — never a branch, never a bulk `--tags`. Report the remote and ref
      that were pushed, and map a rejected push (remote tag exists, no such
      remote) onto the typed errors above rather than surfacing go-git's raw
      text. `--no-push` creates the tag locally and stops.
- [ ] Resolve credentials explicitly, since go-git does not read git's
      credential helper: SSH agent then `~/.ssh/id_*` for `git@`/`ssh://`
      remotes, and a token from `GIT_TOKEN`/`GITHUB_TOKEN` (or `--token-env`)
      for HTTPS remotes. Never prompt for a passphrase or password; on failure
      say which mechanism was tried and what would fix it. Document that a
      passphrase-protected key must be loaded into the agent.
- [ ] Decide and document tag signing: `--sign` needs an OpenPGP entity because
      go-git takes a key rather than delegating to `gpg`, so either load the key
      named by `user.signingkey` and sign the tag object, or leave the flag out
      of the first cut and say so explicitly in the docs. Whichever is chosen, a
      signing failure aborts instead of falling back to an unsigned tag.
- [ ] Support `--dry-run`/`-d`: print the selected version, the tag name and
      message, the target commit (short SHA and subject), the remote, and the
      refspec that would be pushed — touching neither the repository nor the
      network.
- [ ] Add hermetic tests: build a repository with go-git in `t.TempDir()` (fixed
      author/committer identity, no signing), commit fixture files and an
      `incrmit.toml`, and init a bare repository on disk as `origin` so a real
      push is exercised over a `file://` remote with no network. Drive the
      selector through the decoder rather than a real terminal — fixed byte
      sequences for down-down-Enter, wrap-around at both ends, `q`, Esc, and
      Ctrl-C — with golden frames for the rendered list. Cover the
      single-candidate and `--version` paths, an unknown token, tag collision,
      dirty tree, a version not present at `HEAD`, not-a-repository, a missing
      remote, a non-TTY stdin refusing to prompt, and `--dry-run` writing
      nothing.
- [ ] Document the command in `README.md`, the `incrmit(1)` man page, and
      `doc/DEVELOPMENT.md` — including a release recipe (bump, commit by hand,
      `incrmit push`), the selector's keys, the credential rules, the
      `--version` flag CI needs because the prompt requires a TTY, and a note
      that nothing is ever pushed without running `push` — and add a
      `CHANGELOG.md` entry under `Added`. Confirm the `govulncheck` gate from
      Milestone 26 still passes with the go-git and `x/term` trees in `go.sum`.

## Milestone 30 — Conventional-Commit Bump Inference (`--auto`)

Depends on Milestone 29. Reading the commits since the last tag and inferring
the bump component turns `discover` + language-agnostic + single-binary from a
narrow story into a real one: no other tool does automatic inference *and*
arbitrary-file rewriting without a per-ecosystem plugin.

- [ ] Add `--auto` to the bump command: resolve the most recent tag reachable
      from `HEAD` (respecting the Milestone 29 tag prefix), read the commit
      subjects and bodies since it, and infer the component.
- [ ] Implement the inference rules and document them: a `feat:` commit implies
      minor, a `fix:`/`perf:` commit implies patch, and `BREAKING CHANGE:` in a
      trailer or a `!` before the colon implies major. The highest match wins.
      Non-conforming commits are ignored, not errors.
- [ ] Decide and document what happens when nothing is inferable (no tags yet,
      or no conforming commits since the last tag): recommend exiting `0` with
      "no version-relevant commits; nothing to bump" and writing nothing, with
      `--auto --fallback patch` available for CI that wants a bump regardless.
- [ ] Reject `--auto` combined with an explicit `--major`/`--minor`/`--patch`
      with exit code `2` rather than silently letting one win.
- [ ] Make `--auto --dry-run` explain the decision: print the inferred component
      and the specific commits that drove it, so the inference is auditable.
- [ ] Add tests over a scripted temporary repo covering each rule, the highest-
      wins precedence, the `!` and trailer forms of a breaking change, no-tags,
      no-conforming-commits, and the `--fallback` path.
- [ ] Document the rules and a full CI recipe in `README.md`, the man page, and
      `doc/DEVELOPMENT.md`; add a `CHANGELOG.md` entry under `Added`.

## Milestone 31 — Crash-Safe Multi-File Writes

Planning is already fail-fast (Milestone 22's phase 1/2 split), but phase 2 is
not: `runBump` writes files one at a time, so a failure on file 3 of 5 leaves
1–2 bumped and 3–5 untouched. Worse, `recordHistory` runs *after* every write
succeeds, so exactly the case where `undo` is needed is the case where no
journal entry exists. Close this before v1.0.0.

- [ ] Write the journal entry *before* phase 2 rather than after, marked
      `pending`, and flip it to `complete` once every write lands. Bump
      `history` file format handling so an older state file still loads.
- [ ] On a phase-2 write failure, roll back the files already written in this
      run (the pre-bump bytes are still in memory from planning) before
      returning, and report both the original failure and whether the rollback
      itself succeeded.
- [ ] When rollback is impossible or partially fails, leave the `pending` entry
      in place and print an explicit recovery instruction naming `incrmit undo`
      and the affected files, rather than exiting with a bare error.
- [ ] Teach `undo` to recognize a `pending` entry and treat it as the thing to
      revert, tolerating files that were never written (the recorded `new` token
      is absent because the write never happened, which is not the "file was
      edited since" case that currently aborts the whole undo).
- [ ] Order the config rewrite and the target writes so a crash between them is
      recoverable in one direction only, and document which one is written first
      and why in `doc/DEVELOPMENT.md`.
- [ ] Add tests that inject a write failure mid-run (e.g. a read-only directory
      or an unwritable target as the Nth of M files) and assert: files are
      restored, the journal reflects the interrupted run, and a following `undo`
      leaves the tree exactly as it started.
- [ ] Document the crash-safety guarantee — and its limits — in
      `doc/DEVELOPMENT.md`; add a `CHANGELOG.md` entry under `Fixed`.

## Milestone 32 — Code and Repository Hygiene

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

## Milestone 33 — v1.0.0 Release: Publish

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

## Milestone 34 — apt / dnf Repo via GitHub Pages

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
