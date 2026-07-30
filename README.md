# Incrmit

A small command-line tool written in Go that parses a file, finds a version
value inside it, and increments it (increment + commit).

## Version: 0.1.13

## Overview

`incrmit` reads a TOML configuration file that lists one or more target
files (for example a `VERSION` file, a manifest, or any text/config file
containing a semantic version), locates the version value in each, and
bumps it by one. The updated version is written back to every file so
it can be used in release scripts, CI pipelines, or local development
workflows.

## Features

- Discovers files containing version strings and generates a config automatically.
- Reads file locations from a TOML config file (no need to pass paths manually).
- Updates multiple files in a single run, keeping their versions in sync.
- Parses each file and locates a semantic version (`MAJOR.MINOR.PATCH`).
- Increments the major, minor, or patch component.
- Writes the updated version back to the source files in place (atomic,
  only the version token changes).
- Reverts the most recent bump with `incrmit undo`, restoring the previous
  version in every file (and in `incrmit.toml`).
- Ships as a single self-contained binary with predictable exit codes
  for scripting and CI.

## Installation

### Download from GitHub Releases

Pre-built binaries, Linux packages, and macOS installer packages are published
on [GitHub Releases](https://github.com/sasmaq/incrmit/releases). Open the latest
`vX.Y.Z` release and download the asset that matches your platform, or fetch one
directly (replace `X.Y.Z` with the release version):

| Platform | Asset |
| -------- | ----- |
| Linux amd64 | `incrmit-X.Y.Z-linux-amd64.tar.gz` (`.deb` too) |
| Linux arm64 | `incrmit-X.Y.Z-linux-arm64.tar.gz` (`.deb` too) |
| macOS amd64 | `incrmit-X.Y.Z-darwin-amd64.tar.gz` (`.pkg` too) |
| macOS arm64 | `incrmit-X.Y.Z-darwin-arm64.tar.gz` (`.pkg` too) |
| Windows amd64 | `incrmit-X.Y.Z-windows-amd64.zip` |
| Fedora / RHEL x86_64 | `incrmit-X.Y.Z-1.x86_64.rpm` |
| Fedora / RHEL aarch64 | `incrmit-X.Y.Z-1.aarch64.rpm` |

Each release includes `checksums.txt` with SHA-256 hashes of every artifact.
After downloading an asset, verify its integrity by fetching `checksums.txt`
from the same release and comparing hashes (replace `X.Y.Z` with the release
version):

```bash
VERSION=0.1.13
curl -fsSL -O "https://github.com/sasmaq/incrmit/releases/download/v${VERSION}/checksums.txt"

# Linux: verify only the assets you downloaded (ignores missing entries)
sha256sum --ignore-missing -c checksums.txt

# macOS: verify a single asset against its recorded hash
shasum -a 256 -c checksums.txt --ignore-missing
```

A successful check prints `OK` next to each verified file. `checksums.txt`
holds one `<sha256>␣␣<filename>` line per artifact, so you can also compare a
single hash by hand:

```bash
# Recompute the hash and eyeball it against the matching line in checksums.txt
shasum -a 256 "incrmit-${VERSION}-darwin-arm64.pkg"   # sha256sum on Linux
grep "incrmit-${VERSION}-darwin-arm64.pkg" checksums.txt
```

**Tarball or zip** — extract the binary and place it on your `PATH`:

```bash
VERSION=0.1.13
curl -fsSL -O "https://github.com/sasmaq/incrmit/releases/download/v${VERSION}/incrmit-${VERSION}-linux-amd64.tar.gz"
tar xzf "incrmit-${VERSION}-linux-amd64.tar.gz"
sudo install -m 0755 incrmit /usr/local/bin/
```

**Debian or Ubuntu** — download the `.deb` from the release page, then install:

```bash
VERSION=0.1.13
curl -fsSL -O "https://github.com/sasmaq/incrmit/releases/download/v${VERSION}/incrmit_${VERSION}-1_amd64.deb"
sudo dpkg -i "incrmit_${VERSION}-1_amd64.deb"   # use _arm64.deb on arm64
man incrmit
```

**Fedora, RHEL, or other RPM-based systems** — download the `.rpm` from the
release page, then install:

```bash
VERSION=0.1.13
curl -fsSL -O "https://github.com/sasmaq/incrmit/releases/download/v${VERSION}/incrmit-${VERSION}-1.x86_64.rpm"
sudo dnf install "./incrmit-${VERSION}-1.x86_64.rpm"   # use .aarch64.rpm on arm64
man incrmit
```

**macOS** — download the `.pkg` from the release page, then install it (it
places `incrmit` in `/usr/local/bin` and the man page in
`/usr/local/share/man/man1`):

```bash
VERSION=0.1.13
curl -fsSL -O "https://github.com/sasmaq/incrmit/releases/download/v${VERSION}/incrmit-${VERSION}-darwin-arm64.pkg"
# use -darwin-amd64.pkg on Intel Macs
sudo installer -pkg "incrmit-${VERSION}-darwin-arm64.pkg" -target /
incrmit version
man incrmit
```

The `.pkg` is unsigned, so the first install may require approving it under
**System Settings → Privacy & Security**. To uninstall, remove the two files
and forget the receipt:

```bash
sudo rm -f /usr/local/bin/incrmit /usr/local/share/man/man1/incrmit.1
sudo pkgutil --forget com.github.sasmaq.incrmit
```

To build `.deb`, `.rpm`, or `.pkg` packages locally instead of downloading them,
see [doc/DEVELOPMENT.md](doc/DEVELOPMENT.md) (`make deb` / `make rpm` require
[nFPM](https://nfpm.goreleaser.com/); `make pkg` runs on macOS).

### Install with Go

Requires Go 1.26 or later:

```bash
go install github.com/sasmaq/incrmit@v0.1.13
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

Each `[[files]]` entry describes one file to update. Every listed file is
parsed and bumped together so their versions stay in sync.

An entry may also record a `version`, which pins the exact value to bump
(useful for files that contain several version-like strings). When `version`
is omitted, `incrmit` scans the file for the first `MAJOR.MINOR.PATCH` token.
After a successful bump, `incrmit` rewrites `incrmit.toml` so each entry's
`version` reflects the new value, keeping the config in step with the files
it manages.

That rewrite regenerates the file from its parsed contents in a fixed layout, so
your `[[files]]` entries and `ignore` list are preserved but hand-written
comments and custom formatting are not. Keep notes you want to survive a bump
outside `incrmit.toml`.

A `--dry-run` previews the change and writes nothing — neither the targets
nor the config.

### Ignoring folders and files

An optional top-level `ignore` list tells `discover` which folders and files to
skip, in addition to the built-in noise directories (`.git`, `node_modules`,
`vendor`, and build outputs like `dist`, `build`, `target`). It is written near
the top of the config, before the `[[files]]` entries:

```toml
# incrmit.toml

ignore = [
  "testdata/",   # a directory (trailing slash): prune the whole subtree
  "node_modules", # a bare name: match that folder/file at any depth
  "*.lock",      # a glob: match matching files anywhere in the tree
  "docs/**",     # a path with **: prune docs and everything under it
]

[[files]]
  path = "VERSION"
  version = "0.1.0"
```

Matching rules (patterns are compared against the path **relative to the scan
root**, always using forward slashes, and matching is **case-sensitive**):

- A **trailing slash** (`testdata/`) marks a pattern as directory-only: it
  prunes a matching directory (and its subtree) but never matches a file of the
  same name.
- A pattern with **no slash** (`node_modules`, `*.lock`) matches the base name
  of any file or directory at **any depth**. Globs use
  [`path.Match`](https://pkg.go.dev/path#Match) syntax (`*`, `?`, `[…]`).
- A pattern **containing a slash** (`docs/**`, `a/b/*.txt`) is matched against
  the whole relative path, segment by segment. Each segment is a `path.Match`
  glob, and `**` matches zero or more path segments — so `docs/**` prunes the
  `docs` directory and everything inside it.

The `ignore` list is preserved when `discover` regenerates the config and when a
bump rewrites it, so hand-authored entries are never dropped. `discover` reads
the list from the config already present at its `--output` path. Whenever
`incrmit` writes a config that has no `ignore` list yet — either from `discover`
or when a bump rewrites the file — it includes a short description of the option
along with a **commented-out example**, so the feature is easy to find and
enable: just uncomment the line and edit the patterns.

## Discovery

Rather than writing the config by hand, run the `discover` command to scan
the project for files that contain a semantic version string and generate an
`incrmit.toml` for you.

Discovery walks the directory tree and inspects the contents of every text
file, recording every `MAJOR.MINOR.PATCH` token it finds in each. It is
not limited to specific file names or types — any file (`VERSION`,
`package.json`, `pyproject.toml`, `Cargo.toml`, source files, plain text,
etc.) is matched the same way.

A file may contain a version in more than one place. Discovery captures every
occurrence and records **one `[[files]]` entry per distinct version**: if all
the occurrences agree, that is a single entry (and a bump rewrites every copy);
if a file holds several *different* versions, it gets one entry per version, all
sharing the same `path`. Each such version is bumped independently, and all of
its occurrences in the file are updated in a single pass.

A version token may carry an optional leading `v` or `V` (for example
`v1.2.3` or `V1.2.3`, as commonly used by tags and `VERSION` files). The
prefix is preserved everywhere: it is written to `incrmit.toml` as part of
the recorded `version`, and an in-place bump keeps it (so `v1.2.3` bumps to
`v1.2.4`, while a bare `1.2.3` stays bare). A `v`/`V` is only treated as a
prefix when it stands at a word boundary, so embedded look-alikes such as
`rev1.2.3` or `dev1.2.3` are not detected as versions.

A version has exactly three components, so IPv4 addresses are never mistaken
for versions: a four-octet token such as `192.168.1.1`, `10.0.0.255`, or
`127.0.0.1` is skipped entirely (and no three-component slice like
`168.1.1` is pulled out of it), even when every octet is a version-like
integer. A real version on the same line as an address (`server 10.0.0.1
running 2.3.4`) is still detected.

Binary files and common noise directories (`.git`, `node_modules`, `vendor`,
build outputs) are skipped, as is the config file itself (`incrmit.toml` and
the `--output` path), so it is never listed as a target. Any folders and files
matched by the config's [`ignore` list](#ignoring-folders-and-files) are skipped
too.

```bash
incrmit discover
```

This writes an `incrmit.toml` listing each discovered file along with the
version value(s) that were found. A file with two differing versions appears
once per distinct version:

```toml
# incrmit.toml (generated by `incrmit discover`)

[[files]]
  path = "VERSION"
  version = "1.2.3"

[[files]]
  path = "package.json"
  version = "1.2.3"

[[files]]
  path = "notes.md"
  version = "1.2.3"

[[files]]
  path = "notes.md"
  version = "2.0.0"
```

`--dry-run` prints each occurrence it finds, with its line number and the text
of the line for context, rather than writing the config:

```text
Discovered 1 file(s) under . (no config written):
  notes.md:
    L1: release 1.2.3
    L5: legacy 2.0.0
```

When the config has an `ignore` list, `--dry-run` notes the applied rules on a
`(ignoring: …)` line and, like a normal run, never lists any skipped path as a
finding.

### Discovery flags

| Flag | Short | Description | Default |
| ---- | ----- | ----------- | ------- |
| `--path` | `-P` | Root directory to scan | `.` |
| `--output` | `-o` | Path to write the generated config file | `incrmit.toml` |
| `--dry-run` | `-d` | Print discovered files without writing config | `false` |

```bash
# Scan a specific directory and preview the results
incrmit discover --path ./src --dry-run

# Write the config to a custom location
incrmit discover --output release/incrmit.toml
```

## Undo

Made a bump by mistake? `incrmit undo` reverts the most recent bump, restoring
the previous version in every file it changed — and the versions recorded in
`incrmit.toml` — in one step:

```bash
incrmit
# 1.2.3 -> 1.2.4
incrmit undo
# 1.2.4 -> 1.2.3
```

To make undo possible, each successful bump records a small entry (the files it
touched and their `old`/`new` versions, plus a timestamp) in a state file named
`.incrmit.state.toml`, kept next to `incrmit.toml`. This file is **local working
state** — it is not meant to be committed, so add it to your `.gitignore`:

```gitignore
.incrmit.state.toml
```

A few details worth knowing:

- **Repeated undos walk back through history.** Each `undo` reverts one bump and
  removes its entry, so undoing twice reverts the two most recent bumps. The
  journal retains the last several bumps (older ones are dropped).
- **Your edits are never clobbered.** If a file was changed after the bump so it
  no longer contains the version the bump wrote, `undo` refuses that revert and
  writes nothing, exiting with an error instead.
- **Nothing to undo is not an error.** With no recorded history, `undo` prints a
  friendly message and exits `0`.
- **`--dry-run` previews the revert** (`new -> old`) without writing anything.
- **`--file` bumps are not recorded.** A single-file bump (`--file`) has no
  config to anchor the state file to, so it does not create undo history; `undo`
  is a config-driven operation.

### Undo flags

| Flag | Short | Description | Default |
| ---- | ----- | ----------- | ------- |
| `--config` | `-c` | Path to the TOML config file (used to locate the state file) | `incrmit.toml` |
| `--dry-run` | `-d` | Preview the revert without writing | `false` |

## Version

Print the version of the `incrmit` tool itself (not a target file's version)
with the `version` subcommand or the `--version` / `-version` / `-v` flag:

```bash
incrmit version
incrmit --version
incrmit -v
# incrmit 0.1.13
```

The version is baked into the binary and can be overridden at build time
(for example to stamp a git tag).

## Usage

```bash
incrmit [flags]
```

### Flags

| Flag | Short | Description | Default |
| ---- | ----- | ----------- | ------- |
| `--config` | `-c` | Path to the TOML config file | `incrmit.toml` |
| `--file` | `-f` | Bump the version in one file (skips config) | *none* |
| `--major` | `-M` | Bump the major version (resets minor and patch) | `false` |
| `--minor` | `-m` | Bump the minor version (resets patch) | `false` |
| `--patch` | `-p` | Bump the patch version | `true` |
| `--dry-run` | `-d` | Print the new version without writing files | `false` |

When no `--major`, `--minor`, or `--patch` flag is given, a patch bump is
applied. If more than one component flag is supplied, the highest wins (major
> minor > patch).

Using `--file` bypasses the config entirely: only that file is updated and
`incrmit.toml` is not rewritten.

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

Run `incrmit help` for a top-level overview that lists every command **and
its flags** (the bump and discover flags are shown inline so you can see them
without drilling into each command), or `incrmit help <command>` for details on
a specific one. Each command also responds to `-h` / `--help` with the same
text:

```bash
incrmit help              # overview of all commands
incrmit help discover     # help for the discover command
incrmit help undo         # help for the undo command
incrmit help version      # help for the version command
incrmit help bump         # the default bump command's flags
```

The overview opens with an ASCII banner:

```text
 _                           _ _
(_)_ __   ___ _ __ _ __ ___ (_) |_
| | '_ \ / __| '__| '_ ` _ \| | __|
| | | | | (__| |  | | | | | | | |_
|_|_| |_|\___|_|  |_| |_| |_|_|\__|

incrmit — increment semantic versions across one or more files

usage:
  incrmit [flags]            bump the version in the configured files (default)
  incrmit discover [flags]   scan the tree for version-bearing files and write a config
  incrmit undo [flags]       revert the most recent bump
  incrmit version            print the incrmit tool version
  incrmit help [command]     show this overview, or help for a specific command
...
```

The banner is plain ASCII, fits in 80 columns, and is always shown — including
when the output is piped — so the overview is byte-for-byte reproducible. Pass
`--no-banner` to leave it out:

```bash
incrmit help --no-banner
incrmit -h --no-banner
```

The banner appears on the overview only; per-command help such as
`incrmit help discover` never carries it.

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

`incrmit` returns predictable exit codes so it can be used reliably in
scripts and CI pipelines:

| Code | Meaning                                                        |
| ---- | -------------------------------------------------------------- |
| 0    | Success.                                                       |
| 1    | Generic runtime error (e.g. missing config, filesystem error). |
| 2    | Invalid arguments or flags.                                    |
| 3    | No version found, or an ambiguous/unparseable version.         |

## Further reading

- [CHANGELOG.md](CHANGELOG.md) — release history.
- [doc/DEVELOPMENT.md](doc/DEVELOPMENT.md) — architecture, design, and release checklist.
