# Changelog

All notable changes to `incrmit` are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.4] - 2026-06-18

### Added

- `help` subcommand (`incrmit help [command]`) with per-command help for `bump`,
  `discover`, `version`, and `help`.
- Centralized help text so `-h` / `--help` and `incrmit help` stay in sync across
  all commands.
- GitHub Actions release workflow triggered by pushing a `v*` tag: runs CI
  gates, builds cross-compiled archives, and publishes a GitHub Release with
  notes extracted from this changelog.
- `make dist-archives` and `make release` targets that package per-platform
  `.tar.gz` / `.zip` archives and a `checksums.txt` with SHA-256 hashes.
- `scripts/changelog-notes.sh` to extract the matching section from
  `CHANGELOG.md` for release notes.

### Changed

- Explicit `-h` / `--help` on `bump` and `discover` now prints help to stdout;
  usage errors still print help to stderr.
- Unknown subcommands are rejected with a clear error instead of being treated as
  bump flags.
- README and development docs updated for the help commands and tag-to-release
  flow.

## [0.1.3] - 2026-06-17

First public release.

### Added

- `bump` (default command) that locates a `MAJOR.MINOR.PATCH` version in each
  configured file and increments the major, minor, or patch component
  (`--major`/`-M`, `--minor`/`-m`, `--patch`/`-p`; patch is the default).
- TOML configuration (`incrmit.toml`) listing the target files to keep in sync,
  with an optional per-entry `version` that pins the exact token to bump.
- `--file`/`-f` to bump a single file without a config.
- `--dry-run`/`-d` to preview `old -> new` changes without writing anything.
- `discover` command that scans a directory tree and generates `incrmit.toml`
  from the first version token found in each text file. Binary files, common
  noise directories (`.git`, `node_modules`, `vendor`, build outputs), and the
  config file itself are skipped.
- `version` subcommand and `--version`/`-v` flag that print the tool's own
  version.
- Config self-maintenance: after a successful bump, each entry's `version` in
  `incrmit.toml` is updated to the new value so repeated runs stay in sync.
- Atomic, in-place writes that change only the version token and preserve the
  surrounding file contents and mode.
- Predictable exit codes for scripting and CI (success, error, usage, and a
  dedicated code for "no version found").
- Cross-compiled release binaries for Linux, macOS, and Windows
  (`make dist`), and version stamping via `-ldflags` (`make build`).

[0.1.4]: https://github.com/sasmaq/incrmit/releases/tag/v0.1.4
[0.1.3]: https://github.com/sasmaq/incrmit/releases/tag/v0.1.3
