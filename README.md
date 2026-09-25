# Incrmit

A small command-line tool written in Go that finds the semantic version in one
or more files and increments it, keeping them all in sync (increment + commit).

## Version: 0.3.4

## Features

- Discovers files containing version strings and generates a config for you.
- Bumps the major, minor, or patch component in every configured file at once.
- Handles prereleases: start or advance one with `--pre rc`, promote it with
  `--release`.
- Rewrites only the version token, atomically; every other byte of the file is
  kept as it was.
- Shows each file's next patch, minor, and major version with `incrmit preview`,
  and reverts the last bump with `incrmit undo`.
- Ships as a single binary with predictable exit codes for scripts and CI.

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
| Windows arm64 | `incrmit-X.Y.Z-windows-arm64.zip` |
| Fedora / RHEL x86_64 | `incrmit-X.Y.Z-1.x86_64.rpm` |
| Fedora / RHEL aarch64 | `incrmit-X.Y.Z-1.aarch64.rpm` |

Every artifact has a published SHA-256 hash, split across two files because the
macOS installers are built on a separate machine from everything else:

- `checksums.txt` — the tarballs, zips, `.deb`, and `.rpm` packages.
- `checksums-macos.txt` — the macOS `.pkg` installers.

After downloading an asset, verify its integrity by fetching the matching
checksum file from the same release and comparing hashes (replace `X.Y.Z` with
the release version):

```bash
VERSION=0.3.4
curl -fsSL -O "https://github.com/sasmaq/incrmit/releases/download/v${VERSION}/checksums.txt"

# Linux: verify only the assets you downloaded (ignores missing entries)
sha256sum --ignore-missing -c checksums.txt

# macOS: verify a single asset against its recorded hash
shasum -a 256 -c checksums.txt --ignore-missing
```

A successful check prints `OK` next to each verified file. Each file holds one
`<sha256>␣␣<filename>` line per artifact, so you can also compare a single hash
by hand — here for a `.pkg`, whose hashes live in `checksums-macos.txt`:

```bash
curl -fsSL -O "https://github.com/sasmaq/incrmit/releases/download/v${VERSION}/checksums-macos.txt"

# Recompute the hash and eyeball it against the matching line
shasum -a 256 "incrmit-${VERSION}-darwin-arm64.pkg"   # sha256sum on Linux
grep "incrmit-${VERSION}-darwin-arm64.pkg" checksums-macos.txt
```

**Tarball or zip** — extract the binary and place it on your `PATH`:

```bash
VERSION=0.3.4
curl -fsSL -O "https://github.com/sasmaq/incrmit/releases/download/v${VERSION}/incrmit-${VERSION}-linux-amd64.tar.gz"
tar xzf "incrmit-${VERSION}-linux-amd64.tar.gz"
sudo install -m 0755 incrmit /usr/local/bin/
```

**Debian or Ubuntu** — download the `.deb` from the release page, then install:

```bash
VERSION=0.3.4
curl -fsSL -O "https://github.com/sasmaq/incrmit/releases/download/v${VERSION}/incrmit_${VERSION}-1_amd64.deb"
sudo dpkg -i "incrmit_${VERSION}-1_amd64.deb"   # use _arm64.deb on arm64
man incrmit
```

**Fedora, RHEL, or other RPM-based systems** — download the `.rpm` from the
release page, then install:

```bash
VERSION=0.3.4
curl -fsSL -O "https://github.com/sasmaq/incrmit/releases/download/v${VERSION}/incrmit-${VERSION}-1.x86_64.rpm"
sudo dnf install "./incrmit-${VERSION}-1.x86_64.rpm"   # use .aarch64.rpm on arm64
man incrmit
```

**macOS** — download the `.pkg` from the release page, then install it (it
places `incrmit` in `/usr/local/bin` and the man page in
`/usr/local/share/man/man1`):

```bash
VERSION=0.3.4
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

Requires Go 1.27 or later:

```bash
go install github.com/sasmaq/incrmit@v0.3.4
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

## Quick start

```bash
incrmit discover   # scan the project and write incrmit.toml
incrmit preview    # show each file's current and next versions
incrmit            # bump the patch version everywhere: 1.2.3 -> 1.2.4
incrmit undo       # revert the most recent bump
```

`incrmit` keeps two local files next to `incrmit.toml` (the undo history and a
lock against concurrent runs). Add them to your `.gitignore`:

```gitignore
.incrmit.state.toml
.incrmit.lock
```

## Configuration

`incrmit` reads the files to bump from `incrmit.toml` in the current directory
(use `--config` / `-c` for another path). `incrmit discover` writes it for you,
or you can write it by hand:

```toml
# incrmit.toml

[[files]]
  path = "VERSION"
  version = "1.2.3"

[[files]]
  path = "package.json"
  version = "1.2.3"
```

Paths are relative to the config file. `version` pins the exact value to bump,
which matters when a file holds several version-like strings. After each bump
`incrmit` rewrites the config with the new versions. An optional `ignore` list
tells `discover` which folders and files to skip.

## Usage

```bash
incrmit [command] [flags]
```

| Command | Description |
| ------- | ----------- |
| *(none)* | Bump the version in the configured files |
| `discover` | Scan the tree for version-bearing files and write a config |
| `preview` | Show each file's version and its next patch/minor/major |
| `undo` | Revert the most recent bump |
| `version` | Print the `incrmit` tool version |
| `help [command]` | Show the overview, or help for one command |

Flags for the default bump command:

| Flag | Short | Description | Default |
| ---- | ----- | ----------- | ------- |
| `--config` | `-c` | Path to the TOML config file | `incrmit.toml` |
| `--file` | `-f` | Bump the version in one file (skips config) | *none* |
| `--major` | `-M` | Bump the major version (resets minor and patch) | `false` |
| `--minor` | `-m` | Bump the minor version (resets patch) | `false` |
| `--patch` | `-p` | Bump the patch version | `true` |
| `--release` | `-r` | Promote a prerelease (`1.2.3-rc.1` -> `1.2.3`) | `false` |
| `--pre` | `-e` | Start or advance a prerelease (`1.2.3` -> `1.2.4-rc.1`) | *none* |
| `--max-file-size` | `-s` | Refuse to read a target larger than this | *no limit* |
| `--wait` | `-w` | Wait for another `incrmit` run instead of failing | `false` |
| `--dry-run` | `-d` | Print the new version without writing files | `false` |

```bash
incrmit --minor            # 1.2.3 -> 1.3.0
incrmit --major            # 1.2.3 -> 2.0.0
incrmit --file VERSION     # bump one file without a config
incrmit --pre rc           # 1.2.3 -> 1.2.4-rc.1
incrmit --release          # 1.2.4-rc.1 -> 1.2.4
incrmit --dry-run          # show the change without writing it
```

Run `incrmit help <command>` for each command's flags.

## Documentation

- [doc/USAGE.md](doc/USAGE.md) — full guide: config options, every command and
  flag, prereleases, discovery rules, undo, concurrent runs, and
  [exit codes](doc/USAGE.md#exit-codes).
- [doc/man/incrmit.1](doc/man/incrmit.1) — the man page (`man incrmit`).
- [doc/DEVELOPMENT.md](doc/DEVELOPMENT.md) — architecture, design, and release
  checklist.
- [CHANGELOG.md](CHANGELOG.md) — release history.
