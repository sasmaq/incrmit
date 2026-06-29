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

- [ ] Recognize an optional leading `v` (and `V`) before `MAJOR.MINOR.PATCH`
      during discovery (e.g. `v1.2.3`), so tags and `VERSION`-style files using a
      `v` prefix are detected.
- [ ] Update the version token detection/regex in discovery to match `vX.Y.Z`
      without matching unrelated tokens (e.g. avoid `rev1.2.3` or `dev1.2.3`).
- [ ] Preserve the original `v` prefix when writing the discovered version to
      `incrmit.toml` and when bumping in place (a `v1.2.3` token bumps to
      `v1.2.4`, a bare `1.2.3` stays bare).
- [ ] Decide and document how the prefix is represented in config/state (e.g.
      store the prefix per entry or infer it from the existing token on bump).
- [ ] Extend `--dry-run` discovery output to show the `v`-prefixed findings.
- [ ] Add fixtures and tests covering `vX.Y.Z` and `VX.Y.Z` detection, prefix
      preservation on bump, and rejection of near-miss tokens (`rev`, `dev`).
- [ ] Document `v`-prefix support in `README.md` and `doc/DEVELOPMENT.md`.

## Milestone 18 — Ignore IPv4 Addresses

- [ ] Detect and skip IPv4 addresses (e.g. `192.168.1.1`, `10.0.0.255`) during
      discovery so they are not mistaken for `MAJOR.MINOR.PATCH` versions.
- [ ] Treat a four-octet `A.B.C.D` token as an IPv4 address, not a version,
      even when each octet is a valid integer (versions have exactly three
      components).
- [ ] Avoid matching version-like substrings inside a larger IPv4 address
      (e.g. don't pull `168.1.1` out of `192.168.1.1`).
- [ ] Add fixtures and tests covering common IPv4 forms (loopback, private
      ranges, broadcast) and confirm they produce no discovered version.
- [ ] Ensure `--dry-run` discovery output excludes IPv4 matches.
- [ ] Document the IPv4-skipping behavior in `README.md` and
      `doc/DEVELOPMENT.md`.

## Milestone 19 — Discover Multiple Occurrences in a File

- [ ] Detect every version occurrence within a single file during discovery
      rather than stopping at the first match.
- [ ] Decide how multiple matches map to config entries (e.g. one entry per
      occurrence, line/column or match index to disambiguate, or a count) and
      document the chosen model.
- [ ] Handle consistent vs. conflicting versions in the same file (all matches
      agree → single version; differing versions → surface clearly).
- [ ] Ensure in-place bumping updates all targeted occurrences in the file, not
      just the first one.
- [ ] Extend `--dry-run` discovery output to list each occurrence (with its
      location/context) instead of a single per-file result.
- [ ] Add fixtures and tests for files with several identical and several
      differing version tokens, asserting all are found and bumped correctly.
- [ ] Document multi-occurrence discovery behavior in `README.md` and
      `doc/DEVELOPMENT.md`.
