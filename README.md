# Incrmit

A small command-line tool written in Go that parses a file, finds a version value inside it, and increments it (increment + commit).

## Version: 0.1.5

## Overview

`incrmit` reads a TOML configuration file that lists one or more target files (for example a `VERSION` file, a manifest, or any text/config file containing a semantic version), locates the version value in each, and bumps it by one. The updated version is written back to every file so it can be used in release scripts, CI pipelines, or local development workflows.

## Features

- Discovers files containing version strings and generates a config automatically.
- Reads file locations from a TOML config file (no need to pass paths manually).
- Updates multiple files in a single run, keeping their versions in sync.
- Parses each file and locates a semantic version (`MAJOR.MINOR.PATCH`).
- Increments the major, minor, or patch component.
- Writes the updated version back to the source files in place (atomic, only the version token changes).
- Ships as a single self-contained binary with predictable exit codes for scripting and CI.

## Installation

### Download from GitHub Releases

Pre-built binaries and Linux packages are published on
[GitHub Releases](https://github.com/sasmaq/incrmit/releases). Open the latest
`vX.Y.Z` release and download the asset that matches your platform, or fetch one
directly (replace `X.Y.Z` with the release version):

| Platform | Asset |
| -------- | ----- |
| Linux amd64 | `incrmit-X.Y.Z-linux-amd64.tar.gz` or `incrmit_X.Y.Z-1_amd64.deb` |
| Linux arm64 | `incrmit-X.Y.Z-linux-arm64.tar.gz` or `incrmit_X.Y.Z-1_arm64.deb` |
| macOS amd64 | `incrmit-X.Y.Z-darwin-amd64.tar.gz` |
| macOS arm64 | `incrmit-X.Y.Z-darwin-arm64.tar.gz` |
| Windows amd64 | `incrmit-X.Y.Z-windows-amd64.zip` |
| Fedora / RHEL x86_64 | `incrmit-X.Y.Z-1.x86_64.rpm` |
| Fedora / RHEL aarch64 | `incrmit-X.Y.Z-1.aarch64.rpm` |

Each release includes `checksums.txt` with SHA-256 hashes of every artifact.

**Tarball or zip** — extract the binary and place it on your `PATH`:

```bash
VERSION=0.1.5
curl -fsSL -O "https://github.com/sasmaq/incrmit/releases/download/v${VERSION}/incrmit-${VERSION}-linux-amd64.tar.gz"
tar xzf "incrmit-${VERSION}-linux-amd64.tar.gz"
sudo install -m 0755 incrmit /usr/local/bin/
```

**Debian or Ubuntu** — download the `.deb` from the release page, then install:

```bash
VERSION=0.1.5
curl -fsSL -O "https://github.com/sasmaq/incrmit/releases/download/v${VERSION}/incrmit_${VERSION}-1_amd64.deb"
sudo dpkg -i "incrmit_${VERSION}-1_amd64.deb"   # use _arm64.deb on arm64
man incrmit
```

**Fedora, RHEL, or other RPM-based systems** — download the `.rpm` from the
release page, then install:

```bash
VERSION=0.1.5
curl -fsSL -O "https://github.com/sasmaq/incrmit/releases/download/v${VERSION}/incrmit-${VERSION}-1.x86_64.rpm"
sudo rpm -i "incrmit-${VERSION}-1.x86_64.rpm"   # use .aarch64.rpm on arm64
# or: sudo dnf install "./incrmit-${VERSION}-1.x86_64.rpm"
man incrmit
```

To build `.deb` or `.rpm` packages locally instead of downloading them, see
[doc/DEVELOPMENT.md](doc/DEVELOPMENT.md) (`make deb` / `make rpm`; requires
[nFPM](https://nfpm.goreleaser.com/)).

### Install with Go

Requires Go 1.26 or later:

```bash
go install github.com/sasmaq/incrmit@v0.1.5
```

### Build from source

```bash
git clone https://github.com/sasmaq/incrmit.git
cd incrmit
go build -o incrmit .
```

Or use the `Makefile`, which stamps the binary with the version via `-ldflags`
(recommended over a plain `go build` when producing release binaries):

```bash
make build
```

## Configuration

`incrmit` reads its target file locations from a TOML config file. By default it
looks for `incrmit.toml` in the current directory; use `--config` / `-c` to point
to a different path.

```toml
# incrmit.toml

# Files whose version value should be incremented.
[[files]]
  path = "VERSION"
  version = "0.1.0"

[[files]]
  path = "package.json"
  version = "0.1.0"

[[files]]
  path = "internal/buildinfo/buildinfo.go"
  version = "0.1.0"
```

Each `[[files]]` entry describes one file to update. Every listed file is parsed and bumped together so their versions stay in sync.

An entry may also record a `version`, which pins the exact value to bump (useful for files that contain several version-like strings). When `version` is omitted, `incrmit` scans the file for the first `MAJOR.MINOR.PATCH` token. After a successful bump, `incrmit` rewrites `incrmit.toml` so each entry's `version` reflects the new value, keeping the config in step with the files it manages.

A `--dry-run` previews the change and writes nothing — neither the targets nor the config.

## Discovery

Rather than writing the config by hand, run the `discover` command to scan the project for files that contain a semantic version string and generate an `incrmit.toml` for you.

Discovery walks the directory tree and inspects the contents of every text file, recording the first `MAJOR.MINOR.PATCH` token it finds in each. It is not limited to specific file names or types — any file (`VERSION`, `package.json`, `pyproject.toml`, `Cargo.toml`, source files, plain text, etc.) is matched the same way.

Binary files and common noise directories (`.git`, `node_modules`, `vendor`, build outputs) are skipped, as is the config file itself (`incrmit.toml` and the `--output` path), so it is never listed as a target.

```bash
incrmit discover
```

This writes an `incrmit.toml` listing each discovered file along with the
version value that was found:

```toml
# incrmit.toml (generated by `incrmit discover`)

[[files]]
  path = "VERSION"
  version = "1.2.3"

[[files]]
  path = "package.json"
  version = "1.2.3"
```

### Discovery flags

| Flag        | Short | Description                                       | Default        |
| ----------- | ----- | ------------------------------------------------- | -------------- |
| `--path`    | `-P`  | Root directory to scan                            | `.`            |
| `--output`  | `-o`  | Path to write the generated config file           | `incrmit.toml` |
| `--dry-run` | `-d`  | Print discovered files without writing the config | `false`        |

```bash
# Scan a specific directory and preview the results
incrmit discover --path ./src --dry-run

# Write the config to a custom location
incrmit discover --output release/incrmit.toml
```

## Version

Print the version of the `incrmit` tool itself (not a target file's version)
with the `version` subcommand or the `--version` / `-version` / `-v` flag:

```bash
incrmit version
incrmit --version
incrmit -v
# incrmit 0.1.5
```

The version is baked into the binary and can be overridden at build time (for example to stamp a git tag).

## Usage

```bash
incrmit [flags]
```

### Flags

| Flag        | Short | Description                                        | Default        |
| ----------- | ----- | -------------------------------------------------- | -------------- |
| `--config`  | `-c`  | Path to the TOML config file                       | `incrmit.toml` |
| `--file`    | `-f`  | Bump the version in one file (skips config)        | _none_         |
| `--major`   | `-M`  | Bump the major version (resets minor and patch)    | `false`        |
| `--minor`   | `-m`  | Bump the minor version (resets patch)              | `false`        |
| `--patch`   | `-p`  | Bump the patch version                             | `true`         |
| `--dry-run` | `-d`  | Print the new version without writing to the files | `false`        |

When no `--major`, `--minor`, or `--patch` flag is given, a patch bump is applied. If more than one component flag is supplied, the highest wins (major > minor > patch).

Using `--file` bypasses the config entirely: only that file is updated and `incrmit.toml` is not rewritten.

### Examples

Increment the patch version in all configured files (default):

```bash
incrmit
# 1.2.3 -> 1.2.4
```

Use a custom config file:

```bash
incrmit --config release/incrmit.toml
incrmit -c release/incrmit.toml
```

Increment the version in a single specific file (bypassing the config):

```bash
incrmit --file VERSION
incrmit -f VERSION
# 1.2.3 -> 1.2.4
```

The `--file` flag can be combined with the version-part flags:

```bash
incrmit --file package.json --minor
incrmit -f package.json -m
# 1.2.3 -> 1.3.0
```

Increment the minor version:

```bash
incrmit --minor
incrmit -m
# 1.2.3 -> 1.3.0
```

Increment the major version:

```bash
incrmit --major
incrmit -M
# 1.2.3 -> 2.0.0
```

Preview the change without writing it:

```bash
incrmit --dry-run
incrmit -d
```

## Help

Run `incrmit help` for a top-level overview that lists every command, or
`incrmit help <command>` for details on a specific one. Each command also
responds to `-h` / `--help` with the same text:

```bash
incrmit help              # overview of all commands
incrmit help discover     # help for the discover command
incrmit help version      # help for the version command
incrmit help bump         # the default bump command's flags
```

Top-level `-h` / `--help` (with no subcommand) prints the same overview as
`incrmit help`, while passing `-h` / `--help` to a command prints that command's
help:

```bash
incrmit -h                # same as `incrmit help`
incrmit discover --help   # same as `incrmit help discover`
```

All of the above exit with code `0`. An unknown command (for example
`incrmit frobnicate`) prints an error with a hint to run `incrmit help` and
exits with code `2`.

## Exit codes

`incrmit` returns predictable exit codes so it can be used reliably in scripts and CI pipelines:

| Code | Meaning                                                        |
| ---- | -------------------------------------------------------------- |
| 0    | Success.                                                       |
| 1    | Generic runtime error (e.g. missing config, filesystem error). |
| 2    | Invalid arguments or flags.                                    |
| 3    | No version found, or an ambiguous/unparseable version.         |

## Further reading

- [CHANGELOG.md](CHANGELOG.md) — release history.
- [doc/DEVELOPMENT.md](doc/DEVELOPMENT.md) — architecture, design, and release checklist.
