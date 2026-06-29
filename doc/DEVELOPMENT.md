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

### Version command

```text
incrmit version
```

Prints the tool version. The `--version`, `-version`, and `-v` flags are
aliases for the subcommand.

### Help command

```text
incrmit help [command]
```

| Invocation                | Output                                              |
| ------------------------- | --------------------------------------------------- |
| `incrmit help`            | Top-level overview listing every command.           |
| `incrmit help bump`       | The default bump command's flags.                   |
| `incrmit help discover`   | The discover command's flags.                       |
| `incrmit help version`    | The version command's help.                         |
| `incrmit help help`       | The help command's own usage.                       |
| `incrmit -h` / `--help`   | Same overview as `incrmit help` (no subcommand).    |
| `incrmit <cmd> -h`        | The same help text as `incrmit help <cmd>`.         |

Help requested explicitly is written to stdout and exits `0`. An unknown
command prints an error and a hint to run `incrmit help` to stderr and exits
`2` (invalid arguments).

All usage and help text lives in one place (`internal/cli/help.go`) so the
`-h` / `--help` output and the `help` command stay in sync; the commands'
`flag.FlagSet` usage handlers and the `help` dispatch both reference those
shared strings rather than duplicating them.

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
4. If `--dry-run`, print `old -> new` for each target and exit (no writes).
5. Otherwise write each updated file in place.
6. In config mode (not `--file`), rewrite `incrmit.toml` so each entry's
   `version` records the new value, keeping the config in sync for the next run.
7. Report results.

### 8.2 Discover

1. Walk the tree from `--path`, skipping ignored directories
   (e.g. `.git`, `node_modules`, `vendor`, build outputs) and the config file
   (`incrmit.toml` by name, plus the resolved `--output` path).
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

- Build: `make build` (stamps the version via `-ldflags`) or `go build -o incrmit .`.
- Test: `go test ./...` (or `make check` for fmt/vet/lint/coverage).
- Lint: `go vet ./...` and `gofmt`/`golangci-lint`.
- Cross-compile: `make dist VERSION=X.Y.Z` builds static binaries for
  `linux/amd64`, `linux/arm64`, `darwin/amd64`, `darwin/arm64`, and
  `windows/amd64` into `dist/`, named `incrmit-<version>-<os>-<arch>`.
- Release packages: `make release VERSION=X.Y.Z` runs `dist`, then creates
  per-platform archives (`.tar.gz` on Unix, `.zip` on Windows), Linux `.deb`
  and `.rpm` packages, and `dist/checksums.txt` (SHA-256 of each artifact).
- Debian packages: `make deb VERSION=X.Y.Z` builds `incrmit_<version>-1_<arch>.deb`
  files under `dist/` using [nFPM](https://nfpm.goreleaser.com/) and
  `packaging/nfpm.yaml`. Requires `nfpm` on `PATH` (install with
  `go install github.com/goreleaser/nfpm/v2/cmd/nfpm@v2.43.0`). The package
  installs `/usr/bin/incrmit` and `/usr/share/man/man1/incrmit.1`.
- RPM packages: `make rpm VERSION=X.Y.Z` builds `incrmit-<version>-1.<arch>.rpm`
  files (`x86_64`, `aarch64`) under `dist/` using the same nFPM config.
- macOS packages: `make pkg VERSION=X.Y.Z` builds
  `incrmit-<version>-darwin-<arch>.pkg` installers (`amd64`, `arm64`) under
  `dist/` with `pkgbuild` (see `scripts/build-pkg.sh`). Requires the macOS
  toolchain, so this target must run on macOS. The package installs
  `/usr/local/bin/incrmit` and `/usr/local/share/man/man1/incrmit.1`.

### Debian packaging (`.deb`)

**Approach:** [nFPM](https://nfpm.goreleaser.com/) with `packaging/nfpm.yaml`.

**Rationale:** A full `debian/` tree with `debhelper` is aimed at distro
maintainers and adds Ruby/shell packaging overhead for a single static binary.
A raw `dpkg-deb` Makefile recipe is lightweight but duplicates metadata and
file-layout logic across formats. nFPM is a zero-dependency Go tool that reads
one YAML file, cross-builds `.deb` and `.rpm` from any host, and matches how
many Go projects ship Linux packages.

**Layout:** The `.deb` installs the version-stamped Linux binary to
`/usr/bin/incrmit` (mode `0755`) and the man page from `doc/man/incrmit.1` to
`/usr/share/man/man1/incrmit.1`. No runtime dependencies beyond the static Go
binary.

**Local verification:**

```bash
make deb VERSION=X.Y.Z
sudo dpkg -i dist/incrmit_X.Y.Z-1_amd64.deb   # or _arm64.deb on arm64
incrmit version
incrmit --dry-run
sudo dpkg -r incrmit
```

### RPM packaging (`.rpm`)

**Approach:** Same [nFPM](https://nfpm.goreleaser.com/) config as `.deb`
(`packaging/nfpm.yaml`), with top-level `release: "1"` and RPM architecture
names (`x86_64`, `aarch64`) mapped from the Go Linux builds (`amd64`, `arm64`).

**Rationale:** An `incrmit.spec` plus `rpmbuild` is the traditional Fedora/RHEL
path but adds spec-file and build-root ceremony for a single binary. Reusing nFPM
keeps metadata, file layout, and the man page in one place alongside `.deb`.

**Layout:** Same as `.deb` — `/usr/bin/incrmit` and
`/usr/share/man/man1/incrmit.1`. Package files are named
`incrmit-<version>-1.x86_64.rpm` and `incrmit-<version>-1.aarch64.rpm`.

**Local verification:**

```bash
make rpm VERSION=X.Y.Z
sudo rpm -i dist/incrmit-X.Y.Z-1.x86_64.rpm   # or .aarch64.rpm on arm64
# or: sudo dnf install ./dist/incrmit-X.Y.Z-1.x86_64.rpm
incrmit version
incrmit --dry-run
sudo rpm -e incrmit
```

### macOS packaging (`.pkg`)

**Approach:** `pkgbuild` driven by `scripts/build-pkg.sh`, invoked per
architecture from `make pkg`.

**Rationale:** nFPM (used for `.deb`/`.rpm`) does not emit macOS `.pkg`
installers, and `.pkg` files require Apple's packaging tools (`pkgbuild`,
which exist only on macOS). `pkgbuild` ships with the Xcode command line tools
and builds an installable component package directly from a staged file tree —
no extra dependencies. `productbuild` (a distribution wrapper around the
component package) is optional and unnecessary for a single CLI binary.

**Layout:** Each `.pkg` installs the version-stamped macOS binary to
`/usr/local/bin/incrmit` (mode `0755`) and the man page from `doc/man/incrmit.1`
to `/usr/local/share/man/man1/incrmit.1`, with install location `/` and bundle
identifier `com.github.sasmaq.incrmit`. Package files are named
`incrmit-<version>-darwin-amd64.pkg` and `incrmit-<version>-darwin-arm64.pkg`.
Per-architecture packages match the existing archive naming; a single universal
binary (`lipo -create`) is an alternative if a single asset is preferred later.

**Local verification:**

```bash
make pkg VERSION=X.Y.Z
sudo installer -pkg dist/incrmit-X.Y.Z-darwin-arm64.pkg -target /   # or -amd64 on Intel
incrmit version
incrmit --dry-run
```

`pkgbuild` does not generate an uninstaller. Because the package only adds two
files, remove them manually to uninstall, then forget the receipt:

```bash
sudo rm -f /usr/local/bin/incrmit /usr/local/share/man/man1/incrmit.1
sudo pkgutil --forget com.github.sasmaq.incrmit
```

> macOS marks downloaded binaries and packages with a Gatekeeper quarantine.
> The `.pkg` is currently unsigned, so distribution-quality builds should
> codesign and notarize it with an Apple Developer ID (out of scope for the
> default `make pkg`).

### Automated release (CI)

The `.github/workflows/release.yml` workflow runs when a tag matching `v*` is
pushed (not on branch pushes). It:

1. Runs the same validation gates as CI (fmt, vet, test, coverage, lint).
2. Derives the version from `${GITHUB_REF_NAME}` (strips the `v` prefix) and
   passes it to `make release VERSION=…`.
3. Extracts the matching `CHANGELOG.md` section with `scripts/changelog-notes.sh`.
4. Creates a GitHub Release via `softprops/action-gh-release`, uploading the
   archives, `.deb` and `.rpm` packages, and `checksums.txt`. The workflow uses
   the built-in `GITHUB_TOKEN` with `contents: write` (no extra secrets).

A separate `release-macos` job runs on a `macos-latest` runner, builds the
`.pkg` installers with `make pkg`, and uploads them to the same release (the
macOS toolchain needed by `pkgbuild` is unavailable on the Linux runner).

Release checklist:

1. Reconcile the version everywhere (`internal/buildinfo/buildinfo.go`,
   `README.md`, `Makefile` fallback, `incrmit.toml`). These files are kept in
   sync by `incrmit` itself, so a `make build && ./incrmit --<component>` bump
   updates them together.
2. Update `CHANGELOG.md` with a `## [X.Y.Z]` section (the release workflow
   fails if this section is missing).
3. Tag and push: `git tag vX.Y.Z && git push origin vX.Y.Z`.
4. Wait for the release workflow to finish; it publishes the GitHub Release
   automatically.
5. Verify install: `go install github.com/sasmaq/incrmit@vX.Y.Z` then
   `incrmit version`.

To smoke-test the pipeline before a real release, push a disposable tag such as
`v0.0.0-test` (delete the tag and GitHub Release afterward). Confirm the
workflow uploads the expected archives and that `go install` resolves the tag.

## 14. Future Work

- Pre-release and build metadata support (`-rc.1`, `+build.5`).
- Optional git integration (tag and commit after a successful bump).
- Per-file custom match patterns in the config.
- Setting an explicit version instead of incrementing.
