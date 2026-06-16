# Changelog

All notable changes to `incrmit` are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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

[0.1.3]: https://github.com/sasmaq/incrmit/releases/tag/v0.1.3
