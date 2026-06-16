# Incrmit — Software Development Document

This document describes the design, architecture, and implementation plan for
`incrmit`, a Go command-line tool that parses files, discovers semantic version
strings, and increments them.

## 1. Purpose and Scope

`incrmit` automates version bumping across one or more project files. It is
intended for use in release scripts, CI/CD pipelines, and local development
where several files (a `VERSION` file, a `package.json`, a Go source file, etc.)
must share the same version number and be incremented together.

In scope:

- Parsing semantic versions (`MAJOR.MINOR.PATCH`) from text files.
- Incrementing the major, minor, or patch component.
- Reading target files from a TOML configuration file.
- Discovering version-bearing files and generating a config automatically.
- Bumping a single file directly, bypassing the config.
- A dry-run mode that previews changes without writing.

Out of scope (for the initial version):

- Pre-release and build metadata segments (e.g. `1.2.3-rc.1+build.5`).
- Git tagging, committing, or changelog generation.
- Non-semantic version schemes (date-based, monotonic integers, etc.).

## 2. Goals and Non-Goals

### Goals

- Keep versions across multiple files in sync with a single command.
- Be dependency-light and produce a single static binary.
- Provide safe defaults (patch bump, dry-run preview) and clear output.
- Be scriptable and CI-friendly with predictable exit codes.

### Non-Goals

- Acting as a full release manager.
- Supporting arbitrary, user-defined versioning schemes in v1.

## 3. Terminology

| Term      | Meaning                                                    |
| --------- | ---------------------------------------------------------- |
| Version   | A semantic version of the form `MAJOR.MINOR.PATCH`.        |
| Bump      | Incrementing one version component by one.                 |
| Target    | A file whose version value `incrmit` reads and updates.    |
| Config    | The TOML file listing targets (`incrmit.toml` by default). |
| Discovery | Scanning the tree to find targets and generate a config.   |

## 4. Requirements

### 4.1 Functional Requirements

- FR-1: Parse a semantic version from a target file.
- FR-2: Increment the major, minor, or patch component.
  - Bumping major resets minor and patch to `0`.
  - Bumping minor resets patch to `0`.
- FR-3: Write the updated version back to each target in place.
- FR-4: Read target file locations from a TOML config file.
- FR-5: Support `--file` to bump a single file, bypassing the config.
- FR-6: Support `discover` to scan the tree and generate a config.
- FR-7: Support `--dry-run` to preview without writing.
- FR-8: Default to a patch bump using `incrmit.toml` in the current directory.

### 4.2 Non-Functional Requirements

- NFR-1: Single self-contained binary, no runtime dependencies.
- NFR-2: Deterministic, idempotent writes (only the version changes).
- NFR-3: Clear error messages and non-zero exit codes on failure.
- NFR-4: Cross-platform (Linux, macOS, Windows).

## 5. Architecture Overview

`incrmit` is structured as a thin CLI layer over a small set of focused
packages:

```text
+-------------------+
|       main        |  flag parsing, command dispatch
+---------+---------+
          |
   +------v------+   +-------------+   +-------------+
   |   config    |   |   version   |   |  discovery  |
   | (TOML load) |   | (parse/bump)|   |  (scan/gen) |
   +------+------+   +------+------+   +------+------+
          |                 |                 |
          +--------+--------+--------+--------+
                   |                 |
              +----v----+       +----v----+
              |  files  |       |  output |
              | (io rw) |       | (stdout)|
              +---------+       +---------+
```

### Components

- `main` — parses flags, selects the command (default bump or `discover`), and
  wires components together.
- `config` — loads and validates the TOML config into an in-memory model.
- `version` — parses semantic versions and applies major/minor/patch bumps.
- `discovery` — walks the filesystem, detects version strings, and emits config.
- `files` — reads and writes target files, replacing only the version token.
- `output` — formats human-readable results and dry-run previews.

## 6. Data Model

### 6.1 Config Schema (TOML)

```toml
# incrmit.toml
[[files]]
path = "VERSION"
version = "1.2.3"   # optional; populated by `discover`
```

In code:

```go
type Config struct {
    Files []FileEntry `toml:"files"`
}

type FileEntry struct {
    Path    string `toml:"path"`
    Version string `toml:"version,omitempty"`
}
```

### 6.2 Version

```go
type Version struct {
    Major int
    Minor int
    Patch int
}
```

## 7. Command-Line Interface

### Default command (bump)

```text
incrmit [flags]
```

| Flag        | Short | Description                                        | Default        |
| ----------- | ----- | -------------------------------------------------- | -------------- |
| `--config`  | `-c`  | Path to the TOML config file                       | `incrmit.toml` |
| `--file`    | `-f`  | Bump the version in one file (skips config)        | _none_         |
| `--major`   | `-M`  | Bump the major version (resets minor and patch)    | `false`        |
| `--minor`   | `-m`  | Bump the minor version (resets patch)              | `false`        |
| `--patch`   | `-p`  | Bump the patch version                             | `true`         |
| `--dry-run` | `-d`  | Print the new version without writing to the files | `false`        |

### Discover command

```text
incrmit discover [flags]
```

| Flag        | Short | Description                                       | Default        |
| ----------- | ----- | ------------------------------------------------- | -------------- |
| `--path`    | `-P`  | Root directory to scan                            | `.`            |
| `--output`  | `-o`  | Path to write the generated config file           | `incrmit.toml` |
| `--dry-run` | `-d`  | Print discovered files without writing the config | `false`        |

If more than one of `--major`, `--minor`, `--patch` is supplied, the highest
component wins (major > minor > patch). When none is supplied, patch is used.

## 8. Processing Flow

### 8.1 Bump

1. Parse flags and resolve the bump component.
2. Resolve targets:
   - If `--file` is set, use that single file.
   - Otherwise load the config from `--config`.
3. For each target:
   - Read the file and locate the version string.
   - Parse it into a `Version`.
   - Apply the bump.
4. If `--dry-run`, print `old -> new` for each target and exit.
5. Otherwise write each updated file in place and report results.

### 8.2 Discover

1. Walk the tree from `--path`, skipping ignored directories
   (e.g. `.git`, `node_modules`, `vendor`, build outputs).
2. For each candidate file, attempt to extract a semantic version.
3. Collect matches as `FileEntry` records (path + detected version).
4. If `--dry-run`, print the findings and exit.
5. Otherwise write the generated config to `--output`.

## 9. Version Detection Strategy

- Match semantic versions with a regular expression: `\b\d+\.\d+\.\d+\b`.
- Discovery is content-based and format-agnostic: it scans the bytes of every
  text file, regardless of name or extension, and records the first
  `MAJOR.MINOR.PATCH` token found in each.
- Binary files (those containing a NUL byte) are skipped so version-like byte
  sequences in compiled artifacts are not matched.
- Two-component numbers (e.g. `3.9`) and other non-`X.Y.Z` strings do not match.
- On write, replace only the matched token to preserve surrounding formatting.

## 10. Error Handling

- Missing config file: clear message; suggest running `incrmit discover`.
- No version found in a target: report the file and continue or fail fast
  (configurable; fail fast by default for bump).
- Multiple versions found in a single file: report ambiguity and skip unless a
  format-specific rule resolves it.
- Filesystem/permission errors: surface the underlying error and exit non-zero.

### Exit Codes

| Code | Meaning                           |
| ---- | --------------------------------- |
| `0`  | Success.                          |
| `1`  | Generic runtime error.            |
| `2`  | Invalid arguments or flags.       |
| `3`  | No version found / parse failure. |

## 11. Project Layout

```text
incrmit/
├── main.go                 # entry point, flag parsing, dispatch
├── internal/
│   ├── config/             # TOML load and validation
│   ├── version/            # semantic version parse and bump
│   ├── discovery/          # filesystem scan and config generation
│   └── files/              # read/write helpers
├── doc/
│   └── DEVELOPMENT.md      # this document
├── go.mod
└── README.md
```

## 12. Testing Strategy

- Unit tests for `version` (parsing, each bump kind, reset rules, edge cases).
- Unit tests for `config` load/validation against valid and invalid TOML.
- Golden-file tests for `files` write behavior (only the version changes).
- Discovery tests over a fixture tree covering each supported file type.
- CLI integration tests covering default bump, `--file`, `discover`, and
  `--dry-run`, asserting output and exit codes.

## 13. Build and Release

- Build: `go build -o incrmit .`
- Test: `go test ./...`
- Lint: `go vet ./...` and `gofmt`/`golangci-lint`.
- Release: cross-compile per platform; publish binaries and `go install`
  support via tagged versions.

## 14. Future Work

- Pre-release and build metadata support (`-rc.1`, `+build.5`).
- Optional git integration (tag and commit after a successful bump).
- Per-file custom match patterns in the config.
- Setting an explicit version instead of incrementing.
