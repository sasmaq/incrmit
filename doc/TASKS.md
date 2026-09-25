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
      version is bumped to `1.0.0` in Milestone 42.
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

- [ ] Reproduce the damage before fixing it, as Milestone 29 did, so the fix
      has something to prove: plant `.incrmit.lock` as a symlink to a file
      outside the project, run `discover`, a bump, and `undo` through
      `cli.Main`, and assert that file's bytes and mode are unchanged. Add a
      dangling link whose target must still not exist afterward.
- [ ] Open the lock file without following a symlink, so the check and the
      open are one step rather than an `Lstat` followed by a racing `Open`:
      `O_NOFOLLOW` in `lock_unix.go` and `FILE_FLAG_OPEN_REPARSE_POINT` in
      `lock_windows.go`, behind a build-tagged `openLockFile` that mirrors
      `tryLock`. Then `Stat` the descriptor and accept only a regular file.
- [ ] Decide what a lock path that is not a regular file does: a symlink, a
      directory, a FIFO. Recommend a degraded lock, keeping Milestone 29's
      rule that only contention refuses: warn naming the path and why ("is a
      symbolic link"), continue unlocked, and sweep nothing. Nothing is opened
      through the link either way, and the warning is what points the user at
      a planted file. Record the decision and the reason.
- [ ] Stop truncating. Write the note only into a file that was empty when the
      lock was taken, which in practice means one this run just created, so
      even a regular file that happens to sit at that name is never rewritten.
      The note is a courtesy and must never be the reason a file loses data.
- [ ] Confirm the lock is the only place incrmit writes through an existing
      path: `WriteAtomic` renames over a link rather than writing through it,
      and `SweepTemps` removes a link named like a temp file rather than its
      target. Add a test for the sweep case, a `.incrmit-x.tmp` symlink to a
      file outside the tree, which must be removed with its target untouched.
- [ ] Cover the other shapes on Unix, skipping where the system will not
      create them: a symlink to a directory, a FIFO at the lock path, and a
      regular file holding unrelated text, which must come out unchanged. In
      every case assert the command behaves as decided above and that nothing
      outside the project changed.
- [ ] Document the behavior in `README.md` ("Concurrent runs"),
      `doc/DEVELOPMENT.md` §6.4, and the man page's CONCURRENT RUNS section,
      and add a Security entry to `CHANGELOG.md`.

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

- [ ] Write the failing tests first, each under a deadline so a regression
      fails rather than hanging the suite (the pattern
      `TestBumpNonRegularFileTarget` already uses): a FIFO at the config for a
      bump, `--dry-run`, `preview`, and `undo`; at the `--output` path for
      `discover` with and without `--dry-run`; and at the state file for a
      bump and `undo`. Create them with `testutil.Mkfifo`.
- [ ] Route all three reads through one checked helper, `files.ReadTarget` or
      a sibling for tool-maintained files, so one function decides that a
      file is safe to open. A symlink to a regular file is still followed, as
      it is for targets, since a symlinked config is a legitimate setup; a
      link to a device or a pipe fails the same type check.
- [ ] Cap the size of these reads. The config and the journal are small, and
      the journal holds at most `history.MaxEntries` entries, so a fixed cap
      in a named constant is enough; `--max-file-size` is about targets and
      stays that way. Report an oversized file as an error that names it.
- [ ] Decide what `LoadIgnore` does with a `--output` that is not a regular
      file. It is lenient today (a missing or unparseable file yields no
      patterns), but a FIFO or a device is not a stale config, and `discover`
      would go on to replace it. Recommend an error with exit `1`, decided
      together with Milestone 36's rule for what `--output` may overwrite.
- [ ] Read the journal before phase 2 of a bump instead of after the writes,
      so a state file that cannot be read fails the bump with nothing
      written. Milestone 40 moves the journal *write* ahead of phase 2 too;
      this item is only the read.
- [ ] Add a symlink to `/dev/zero` as the config and as the state file (Unix
      only), and assert each command fails promptly with a "not a regular
      file" message and exit `1`.
- [ ] Document the checks next to the other read boundaries in `README.md`,
      `doc/DEVELOPMENT.md`, and the man page's CAVEATS, and add a Security
      entry to `CHANGELOG.md`.

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

- [ ] Reproduce first: the copied project (assert `orig` is untouched), the
      moved project (today `undo` fails with "reading VERSION: file does not
      exist" beside a `VERSION` that exists), and a hand-written state file
      whose `fs` names a file outside the project.
- [ ] Resolve every journal path against the directory of the config `undo`
      was given (`-c`, or `incrmit.toml` by default), the same way
      `resolveTargets` resolves config entries for a bump, and write the
      reverted config back to that path rather than to the recorded `config`.
      `undo` still works from any working directory, because it finds the
      state file through the config.
- [ ] Stop writing `fs` and `config` into new entries. `path` is already the
      config-relative path, so no new field is needed. Old state files keep
      loading: both keys are ignored on read, so an entry written by an
      earlier version is undone against the current project rather than its
      old location.
- [ ] Only undo what the current config names. Before planning, check that
      every change's `path` matches a `[[files]]` entry in the config being
      undone whose version is the change's `new`, and refuse otherwise,
      naming the path, with nothing written. The config is the trusted input,
      so this limits what a journal can reach to what the config could
      already bump. An absolute `path`, or one with `../`, stays allowed
      exactly when the config lists it.
- [ ] Confirm the lock, the state file, and every write now belong to one
      project. The lock is already taken beside the config `undo` was given;
      with the recorded `config` gone, `undo` can no longer rewrite a config
      in a directory it did not lock.
- [ ] Update the tests that assert absolute journal paths, and add undo from
      a subdirectory with `-c ../incrmit.toml`, after the project was moved,
      after it was copied, with a config that lists a `../` path, and from an
      old state file that still carries `fs` and `config`.
- [ ] Update the `internal/history` package doc, `doc/DEVELOPMENT.md` (the
      state file format, which says paths are stored absolute), `README.md`,
      and the man page, and add a `Changed` entry to `CHANGELOG.md`.

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

- [ ] Write table tests at the boundary against the intended behavior, so
      they fail today and pass after the fix: each of major, minor, and patch
      at `math.MaxInt64`, and a prerelease counter at `math.MaxInt64`, through
      a bump, `--dry-run`, `preview`, and `--pre`.
- [ ] Decide what an unbumpable version does. Recommend refusing: the command
      exits `3` naming the file and the component, and writes nothing, which
      is how incrmit treats every other version it cannot handle. Clamping
      and wrapping both write a version that is not greater than the one it
      replaced. The `Bump*` methods return no error today; either give them
      one or add checked variants for the CLI, and surface the error through
      `bumpFunc`, whose error path in `planGroups` already fails before any
      write.
- [ ] Make `preview` show a projection that would overflow as unavailable
      rather than as a wrapped number, with the table still aligned.
- [ ] Settle the prerelease counter's other edge separately: a numeric
      identifier too large for an `int` has `.1` appended today
      (`rc.99999999999999999999` -> `rc.99999999999999999999.1`) rather than
      being counted. Keep that (it does sort higher) or refuse it like the
      numeric core, and test whichever is chosen.
- [ ] Add a fuzz target for the property all of this breaks: for any version
      `Parse` accepts, every component bump and every same-series prerelease
      advance that succeeds yields a token that parses back to itself through
      `String()` and that `version.Compare` ranks above the input. Leave out
      switching series, since `rc` to `beta` may rank lower by design. Seed
      it with the `MaxInt64` boundaries.
- [ ] Document the limit in `README.md` and in `doc/DEVELOPMENT.md` §12.1 next
      to the other tokens incrmit refuses, and add a `Fixed` entry to
      `CHANGELOG.md`.

## Milestone 36 — Discover Overwriting an Unrelated File

`discover -o PATH` replaces whatever is at `PATH`. Regenerating an existing
`incrmit.toml` is the point, but nothing checks that the file being replaced
*is* one: `incrmit discover -o NOTES.md` turned a Markdown file into a
generated config and exited `0`, and a slip such as `-o package.json` would do
the same to a manifest. The read that comes first does not catch it either:
`config.LoadIgnore` is deliberately lenient, so a file that does not parse
yields no ignore patterns rather than an error, and discover carries on as if
it had found a config with nothing to keep.

- [ ] Reproduce first: `-o` naming an existing Markdown file, an existing
      target such as `VERSION` (which `excludeOutput` drops from the results
      just before overwriting it), an empty file, and an existing
      `incrmit.toml`, which must still be regenerated.
- [ ] Define what `--output` may replace: a path with no file yet, an empty
      file, or a file that parses as TOML and holds only keys incrmit knows
      (`ignore` and `files`). That accepts every config incrmit has written
      and every hand-written one, and rejects a Markdown file, a JSON
      manifest, or a `pyproject.toml`. Keep the list of known keys next to
      `config.Config` so Milestone 38's `[git]` table extends it in one place.
- [ ] Refuse anything else with exit `1` before the scan starts, so a mistake
      costs nothing, and say why and what to do: "NOTES.md exists and is not
      an incrmit config; choose another --output or remove the file". No
      `--force` is needed: removing the file is the explicit way to ask.
- [ ] Apply the same check in `--dry-run`, so the dry run predicts the refusal
      rather than printing a plan the real run will not carry out.
- [ ] Retire `LoadIgnore`'s leniency along with it: a file that fails the
      check never reaches it, so an unparseable `--output` becomes an error
      rather than "no patterns". Share the check with Milestone 33's type
      check, so a FIFO or a device at `--output` is refused by the same code.
- [ ] Assert in every refusal test that the file's bytes are unchanged.
      Document the rule in `README.md`, `incrmit help discover`, and the man
      page, and add a `Fixed` entry to `CHANGELOG.md`.

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

- [ ] Reproduce first, with a decoy holding the same version at the location
      the bad path resolves to, since that is the silent case: `--path sub`
      with the default output, `-o sub/incrmit.toml` with the default path,
      and the two flags naming sibling directories. After each, assert that a
      bump changes the scanned file and nothing else.
- [ ] Write every path relative to the output config's directory: make the
      root and the output absolute, join each result to the root, and take
      `filepath.Rel` from the config's directory, in slash form. A root
      outside the config's directory gives `../` paths, which config loading
      already accepts. Rework `excludeOutput`, which joins paths to the root
      today.
- [ ] Decide what `ignore` patterns are relative to. They are matched against
      paths relative to the scan root, but they live in the config, so one
      pattern means something different under a different `--path`.
      Recommend the config's directory, so a config means the same thing
      wherever discover is run from; either way, say so in the comment
      `IgnoreComment` writes into every config.
- [ ] Print the config-relative paths in the `--dry-run` listing and the
      "Wrote ..." summary too, so the paths a user reviews are the ones a bump
      will resolve.
- [ ] A config written by the old behavior cannot be detected in general,
      since its paths may exist and hold the pinned version. Add a `Fixed`
      entry to `CHANGELOG.md` telling anyone who ran `discover` with `--path`
      or `-o` pointing elsewhere to run it again.
- [ ] Test the cases above plus the default run (paths unchanged), an absolute
      `--path`, and a `--path` written with `./`. Update `README.md`
      (Discovery) and the man page to say paths are written relative to the
      config.

## Milestone 38 — Release Tagging Helper (the `tag` command)

The name reads as "increment + commit", but there is no git integration at all:
no tag, no push. The gap is closed without incrmit ever running git: `os/exec`
is banned in this repo (a `depguard` rule in `.golangci.yml`), so `incrmit tag`
does the part only incrmit knows — which version to release — and hands the git
part back as commands to run. It lists the versions recorded in `incrmit.toml`,
the user picks one with the arrow keys and Enter (or quits without touching
anything), and it prints the `git tag` and `git push` commands for that
version. incrmit never reads or writes the repository and never touches the
network. Because the user's own `git` runs the commands, their remotes,
credential helpers, SSH config, `insteadOf` rewrites, and signing setup all
apply unchanged, and incrmit itself gains no runtime dependency on git. Bump
stays git-free exactly as it is today.

- [ ] Record the decision in `doc/DEVELOPMENT.md`: incrmit never starts a
      subprocess, and `depguard` denies the `os/exec` import. A Go git library
      such as `go-git` was rejected too: v5.19.2 adds 20 modules (19 new) to a
      project whose only dependencies are `BurntSushi/toml` and
      `golang.org/x/sys` (a minimal program using it builds to 9.1 MB against
      incrmit's 4.0 MB), and its push ignores credential helpers, reads only
      `Hostname` and `Port` from
      `~/.ssh/config` (no `IdentityFile`, no `ProxyJump`), and signs only with
      an OpenPGP key loaded in-process (no gpg-agent, no SSH signing). Printing
      the commands keeps all of those working. Replace the "Optional git
      integration" bullet under Future Work with a pointer to this design.
- [ ] Add the `tag` subcommand: dispatch it in `cli.Main` alongside `discover`,
      `preview`, and `undo`, add it to the top-level overview and to
      `incrmit help tag` via the centralized text in `internal/cli/help.go`,
      and reject unknown arguments with exit `2` the way the other commands do.
      The command is read-only, so it takes no `--dry-run`: the printed
      commands are the preview. Thread stdin through `cli.Main` as an
      `io.Reader`, with a way to ask whether it is a terminal, so the picker
      (and Milestone 39's `--auto`) can be tested in-process.
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
      prompt never leaves the shell without echo. Draw the picker on stderr,
      never stdout, so stdout carries only the final commands.
- [ ] Define the key map and the redraw, and keep both testable: arrows arrive
      as `ESC [ A` / `ESC [ B`, so decode key events from a byte stream in a
      pure function that tests feed fixed sequences; accept `k`/`j` as aliases,
      wrap at the ends of the list, confirm on Enter, and cancel on `q`, Esc, or
      Ctrl-C — a cancel exits `0` having printed no commands and says so on
      stderr. Redraw by moving up the N rows just written and clearing each line
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
- [ ] Build the commands: an annotated tag named with a configurable prefix
      (default `v`, so `v1.2.4`) settable as `--prefix` or `[git] tag_prefix`
      in `incrmit.toml`, with a `--tag-message` whose default template is
      `Release {{.Version}}`, pushed to `--remote` (default `origin`). The
      default output is one line:
      `git tag -a v1.2.4 -m 'Release 1.2.4' && git push origin refs/tags/v1.2.4`.
      Push exactly the one tag refspec — never a branch, never a bulk `--tags`.
      `--no-push` prints only the `git tag` command. `--sign` prints `git tag
      -s`, so git applies `user.signingkey` and `gpg.format` (OpenPGP or SSH)
      as configured, and `tag.gpgSign = true` works with no flag at all. No
      `--force` is offered: git refuses an existing local tag and the remote
      rejects an existing remote one.
- [ ] Make the output safe to paste or pipe: stdout is exactly the command
      line, joined with `&&` so a failed `git tag` never reaches the push, and
      everything else (the picker, notes, reminders) goes to stderr, so
      `incrmit tag --version 1.2.4 | sh` runs exactly what was shown. Validate
      the prefix against git's ref-name rules (no leading `-`, whitespace,
      `..`, `~`, `^`, `:`, `?`, `*`, `[`, `\`, or trailing `.lock`) and exit
      `2` naming the rule, so the tag name never needs quoting and can never be
      read as a flag. Quote the message with POSIX single quotes (`'` becomes
      `'\''`) and document that the output targets POSIX shells.
- [ ] Say what incrmit cannot check: with no repository access it cannot
      confirm the worktree is clean or that `HEAD` holds the selected version,
      and the printed `git tag` tags `HEAD`. Print a one-line reminder on
      stderr ("this tags HEAD — commit the bump first") and document the
      release order (bump, commit, `incrmit tag`) so a version that was bumped
      but never committed is not tagged. The drift marking above still catches
      a half-finished bump in the working tree.
- [ ] Add tests that run in-process through `cli.Main` with no git and no
      repository: golden output for the printed commands (defaults, `--prefix`,
      `[git] tag_prefix`, `--remote`, `--no-push`, `--sign`, and a message
      template containing `'`); the stdout/stderr split, with stdout exactly
      the command line; prefix validation, one case per rejected form. Drive
      the selector through the decoder rather than a real terminal — fixed byte
      sequences for down-down-Enter, wrap-around at both ends, `q`, Esc, and
      Ctrl-C — with golden frames for the rendered list. Cover the
      single-candidate and `--version` paths, an unknown token, drift marking,
      a missing config, and a non-TTY stdin refusing to prompt.
- [ ] Document the command in `README.md`, the `incrmit(1)` man page, and
      `doc/DEVELOPMENT.md` — including a release recipe (bump, commit by hand,
      `incrmit tag`, then run the printed line or pipe it to `sh`), the
      selector's keys, the `--version` flag CI needs because the prompt
      requires a TTY, the stdout/stderr contract, and the fact that
      authentication and signing are whatever the user's `git` does because
      incrmit never runs it — and add a `CHANGELOG.md` entry under `Added`.
      Confirm the `govulncheck` gate from Milestone 26 still passes with the
      `x/term` tree in `go.sum`.

## Milestone 39 — Conventional-Commit Bump Inference (`--auto`)

Reading the commits since the last tag and inferring the bump component turns
`discover` + language-agnostic + single-binary from a narrow story into a real
one: no other tool does automatic inference *and* arbitrary-file rewriting
without a per-ecosystem plugin. As in Milestone 38, incrmit does not run git:
`--auto` reads commit messages from stdin, and the user's own `git log`
supplies them.

- [ ] Add `--auto` to the bump command: read commit messages from stdin,
      separated by NUL bytes, as produced by
      `git log --format=%B%x00 "$(git describe --tags --abbrev=0)..HEAD"`,
      and infer the component. When stdin is a terminal, refuse with exit `2`
      and a usage hint showing that pipeline instead of waiting for input.
- [ ] Implement the inference rules and document them: a `feat:` commit implies
      minor, a `fix:`/`perf:` commit implies patch, and `BREAKING CHANGE:` in a
      trailer or a `!` before the colon implies major. The highest match wins.
      Non-conforming commits are ignored, not errors.
- [ ] Decide and document what happens when nothing is inferable (empty input,
      or no conforming commits): recommend exiting `0` with "no
      version-relevant commits; nothing to bump" and writing nothing, with
      `--auto --fallback patch` available for CI that wants a bump regardless.
      Before the first tag, `git describe` fails and the pipeline above feeds
      no commits, so document the first-release form that logs from the root
      (`git log --format=%B%x00 | incrmit --auto`).
- [ ] Reject `--auto` combined with an explicit `--major`/`--minor`/`--patch`
      with exit code `2` rather than silently letting one win.
- [ ] Make `--auto --dry-run` explain the decision: print the inferred component
      and the subject line of each commit that drove it, so the inference is
      auditable.
- [ ] Add tests that feed fixed stdin byte streams through `cli.Main` — no git
      and no scripted repository — covering each rule, the highest-wins
      precedence, the `!` and trailer forms of a breaking change, CRLF line
      endings, empty input, no conforming commits, the `--fallback` path, and a
      terminal stdin refusing to wait.
- [ ] Document the rules and a full CI recipe in `README.md`, the man page, and
      `doc/DEVELOPMENT.md` — the `git log | incrmit --auto` pipeline, matching
      the Milestone 38 tag prefix with `git describe --match 'v*'`, and
      `fetch-depth: 0` on `actions/checkout` so the tags and history are there
      to read; add a `CHANGELOG.md` entry under `Added`.

## Milestone 40 — Crash-Safe Multi-File Writes

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

## Milestone 41 — Code and Repository Hygiene

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

## Milestone 42 — v1.0.0 Release: Publish

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

## Milestone 43 — apt / dnf Repo via GitHub Pages

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
