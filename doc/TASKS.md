# Incrmit — Development Tasks

A checklist of work to implement `incrmit` as described in `README.md` and
`DEVELOPMENT.md`. Tasks are grouped into milestones; check items off as they are
completed.

## Milestone 1 — Project Setup

- [x] Initialize the Go module (`go mod init github.com/sasmaq/incrmit`).
- [x] Create the project layout (`main.go`, `internal/`, `doc/`).
- [ ] Add `.gitignore` for build artifacts and editor files.
- [ ] Set up `go vet`, `gofmt`, and `golangci-lint`.
- [ ] Add a basic CI workflow (build, test, lint).

## Milestone 2 — Version Core

- [ ] Define the `Version` type (`Major`, `Minor`, `Patch`).
- [ ] Implement parsing of `MAJOR.MINOR.PATCH` strings.
- [ ] Implement major bump (reset minor and patch to `0`).
- [ ] Implement minor bump (reset patch to `0`).
- [ ] Implement patch bump.
- [ ] Implement `String()` formatting back to `MAJOR.MINOR.PATCH`.
- [ ] Unit tests covering parsing, each bump, and edge cases.

## Milestone 3 — Config

- [ ] Define `Config` and `FileEntry` structs with TOML tags.
- [ ] Load and parse `incrmit.toml`.
- [ ] Validate entries (non-empty paths, existing files).
- [ ] Resolve the default config path (`incrmit.toml`).
- [ ] Unit tests for valid and invalid config files.

## Milestone 4 — File I/O

- [ ] Read a target file and locate its version token.
- [ ] Replace only the version token, preserving surrounding formatting.
- [ ] Write changes back in place safely (atomic write).
- [ ] Golden-file tests confirming only the version changes.

## Milestone 5 — Bump Command

- [ ] Parse flags: `--config`/`-c`, `--file`/`-f`, `--major`/`-M`,
      `--minor`/`-m`, `--patch`/`-p`, `--dry-run`/`-d`.
- [ ] Resolve the bump component (highest of major/minor/patch wins).
- [ ] Resolve targets from `--file` or the config.
- [ ] Apply the bump to each target.
- [ ] Implement `--dry-run` preview (`old -> new`).
- [ ] Print a clear summary of updated files.
- [ ] Integration tests for default, `--file`, and `--dry-run` flows.

## Milestone 6 — Discovery

- [ ] Implement the `discover` subcommand and its flags
      (`--path`/`-P`, `--output`/`-o`, `--dry-run`/`-d`).
- [ ] Walk the directory tree, skipping ignored dirs
      (`.git`, `node_modules`, `vendor`, build outputs).
- [ ] Detect versions in `VERSION`, `package.json`, `pyproject.toml`,
      `Cargo.toml`, and Go source files.
- [ ] Generate `incrmit.toml` with discovered paths and versions.
- [ ] Implement `--dry-run` to print findings without writing.
- [ ] Tests over a fixture tree covering each supported file type.

## Milestone 7 — Error Handling and UX

- [ ] Friendly message when the config is missing (suggest `discover`).
- [ ] Handle "no version found" and ambiguous matches.
- [ ] Surface filesystem and permission errors clearly.
- [ ] Implement exit codes (`0`, `1`, `2`, `3`) per the design doc.

## Milestone 8 — Testing

- [ ] Set up a shared `testdata/` layout for fixtures and golden files.
- [ ] Add table-driven test helpers and shared assertion utilities.
- [ ] Run the suite with the race detector (`go test -race ./...`).
- [ ] Measure coverage (`go test -cover ./...`) and set a target threshold.
- [ ] Add end-to-end CLI tests that build the binary and assert exit codes.
- [ ] Add a `-update` golden-file flag to regenerate expected outputs.
- [ ] Wire `go test ./...` into CI as a required gate.

## Milestone 9 — Release

- [ ] Cross-compile binaries (Linux, macOS, Windows).
- [ ] Verify `go install` works on a tagged version.
- [ ] Write release notes and tag the first version.
- [ ] Confirm `README.md` examples match actual behavior.

## Backlog / Future Work

- [ ] Pre-release and build metadata (`-rc.1`, `+build.5`).
- [ ] Optional git integration (tag and commit after a bump).
- [ ] Per-file custom match patterns in the config.
- [ ] Set an explicit version instead of incrementing.
