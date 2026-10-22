# Incrmit

A small command-line tool written in Go that finds the semantic version in one
or more files and increments it, keeping them all in sync (increment + commit).

## Version: 0.3.12

## Features

- Discovers files containing version strings and generates a config for you.
- Reads `.json` and `.toml` files by their format, so a version inside a URL,
  a range such as `^18.3.1`, or a comment is never mistaken for the project's.
- Bumps the project's own version and not the dependencies pinned beside it:
  each config entry can say where the version is (`key = "version"`).
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

GitHub records a SHA-256 digest for every release asset and shows it next to
the download on the release page. To verify a download, recompute its hash and
compare it with the one listed there:

```bash
VERSION=0.3.12
shasum -a 256 "incrmit-${VERSION}-linux-amd64.tar.gz"   # sha256sum on Linux
```

**Tarball or zip** — extract the binary and place it on your `PATH`:

```bash
VERSION=0.3.12
curl -fsSL -O "https://github.com/sasmaq/incrmit/releases/download/v${VERSION}/incrmit-${VERSION}-linux-amd64.tar.gz"
tar xzf "incrmit-${VERSION}-linux-amd64.tar.gz"
sudo install -m 0755 incrmit /usr/local/bin/
```

#### Debian or Ubuntu

Download the `.deb` from the release page, then install:

```bash
VERSION=0.3.12
curl -fsSL -O "https://github.com/sasmaq/incrmit/releases/download/v${VERSION}/incrmit_${VERSION}-1_amd64.deb"
sudo dpkg -i "incrmit_${VERSION}-1_amd64.deb"   # use _arm64.deb on arm64
incrmit -v
```

#### Fedora, RHEL, or other RPM-based systems

Download the `.rpm` from the
release page, then install:

```bash
VERSION=0.3.12
curl -fsSL -O "https://github.com/sasmaq/incrmit/releases/download/v${VERSION}/incrmit-${VERSION}-1.x86_64.rpm"
sudo dnf install "./incrmit-${VERSION}-1.x86_64.rpm"   # use .aarch64.rpm on arm64
incrmit -v
```

#### MacOS

Download the `.pkg` from the release page, then install it (it
places `incrmit` in `/usr/local/bin` and the man page in
`/usr/local/share/man/man1`):

```bash
VERSION=0.3.12
curl -fsSL -O "https://github.com/sasmaq/incrmit/releases/download/v${VERSION}/incrmit-${VERSION}-darwin-arm64.pkg"
# use -darwin-amd64.pkg on Intel Macs
sudo installer -pkg "incrmit-${VERSION}-darwin-arm64.pkg" -target /
incrmit -v
```

The `.pkg` is unsigned, so the first install may require approving it under
**System Settings → Privacy & Security**. To uninstall, remove the two files
and forget the receipt:

```bash
sudo rm -f /usr/local/bin/incrmit /usr/local/share/man/man1/incrmit.1
sudo pkgutil --forget com.github.sasmaq.incrmit
```

### Install with Go

Requires Go 1.27 or later:

```bash
go install github.com/sasmaq/incrmit@v0.3.12
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

To build `.deb`, `.rpm`, or `.pkg` packages locally instead of downloading them,
see [doc/DEVELOPMENT.md](doc/DEVELOPMENT.md) (`make deb` / `make rpm` require
[nFPM](https://nfpm.goreleaser.com/); `make pkg` runs on macOS).

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
  key = "version"

[[files]]
  path = "Cargo.toml"
  version = "1.2.3"
  key = "package.version"

[[files]]
  path = "Makefile"
  version = "1.2.3"
  key = "VERSION ?= "
```

Paths are relative to the config file. `version` pins the exact value to bump,
which matters when a file holds several version-like strings. After each bump
`incrmit` rewrites the config with the new versions.

`key` says which occurrence is the project's version, so a bump leaves the
rest of the file alone: in the `package.json` above, `"left-pad": "1.2.3"` keeps
its version when `"version": "1.2.3"` moves. For a `.json` or `.toml` file the
key is a path from the top of the document, written like a TOML dotted key
(`package.version`, `bin[0].version`, `'dependencies."@types/node"'`). For any
other file it is the text right before the version on its line, matched byte
for byte: `"VERSION ?= "` names `VERSION ?= 1.2.3` and not a download URL
ending in `tool-1.2.3.tar.gz`. An entry without a key bumps every occurrence of
its version. A key that names nothing holding the entry's version fails the
bump before anything is written.

A `.json` or `.toml` file is read by its format: a version counts only where it
is a whole string value, such as `"version": "1.2.3"`, so a `$schema` URL, a
dependency range, or a TOML comment holding a version is left alone. Every
other file is scanned as text, where any version token counts. An entry's
optional `parser` key (`"json"`, `"toml"`, or `"text"`) chooses for that file,
for a `tsconfig.json` with comments or TOML under another name such as
`Pipfile`. `discover` scans a `.json` or `.toml` file that does not parse as
text and writes `parser = "text"` into its entries; a bump never guesses, and
an entry whose parser cannot read its file fails with nothing written.

`discover` writes a key for every version in a `.json` or `.toml` file. In
`package.json`, `composer.json`, `deno.json`, `jsr.json`, `Cargo.toml`, and
`pyproject.toml` it writes only the project's own version and lists each
dependency in `--dry-run` as skipped, with the key that would track it; such a
manifest with no version of its own, like a private monorepo root, gets no
entry. It writes no key for a text file, so add one by hand where a file holds
the version more than once. Running `discover` again adds the keys to a config
written before they existed. An optional `ignore` list
tells `discover` which folders and files to skip; its patterns are relative to
the config file too. `discover` writes every path relative to the config it
writes, so `incrmit discover --path src -o release/incrmit.toml` lists
`../src/VERSION`. Running `discover` again regenerates the config and keeps the
`ignore` list, but it never replaces a file that is not a config:
`incrmit discover -o README.md` exits `1` and leaves the README alone.

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
| `--key` | `-k` | With `--file`, the key that names the version | *none* |
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
incrmit --file package.json --key version   # its version, not its dependencies
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
