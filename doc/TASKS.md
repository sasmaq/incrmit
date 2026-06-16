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

- [ ] After a successful bump, update the `version` field of each entry in
      `incrmit.toml` to the new version (keep the config in sync).
- [ ] Skip the config update on `--dry-run` (preview only, write nothing).
- [ ] Exclude the config file from discovery (`incrmit.toml` and the discover
      `--output` path) so it is never added as a target.
- [ ] Tests for config self-bump and discovery exclusion.

## Milestone 11 — Release

- [ ] Cross-compile binaries (Linux, macOS, Windows).
- [ ] Verify `go install` works on a tagged version.
- [ ] Write release notes and tag the first version.
- [ ] Confirm `README.md` examples match actual behavior.

## Backlog / Future Work

- [ ] Pre-release and build metadata (`-rc.1`, `+build.5`).
- [ ] Optional git integration (tag and commit after a bump).
- [ ] Per-file custom match patterns in the config.
- [ ] Set an explicit version instead of incrementing.
