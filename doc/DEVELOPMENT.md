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
- Undoing the most recent bump, restoring the previous version in every file it
  wrote (and in `incrmit.toml`), using a locally recorded bump history.

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
ignore = ["testdata/", "*.lock", "docs/**"]   # optional; folders/files discover skips

[[files]]
path = "VERSION"
version = "1.2.3"   # optional; populated by `discover`
```

A `path` may appear in more than one `[[files]]` entry when a file contains
several *differing* versions, as long as each entry pins a distinct, non-empty
`version` (one entry per distinct version). Validation rejects exact
`(path, version)` duplicates and any repeated path where an entry omits the
version (which would be ambiguous). Identical repeats of the same version within
a file collapse to a single entry.

The optional top-level `ignore` list holds folder/file patterns that `discover`
skips (see [section 8.3](#83-ignore-matching)). Empty patterns are rejected;
each pattern is trimmed and its separators normalized to `/` on load.

In code (`Ignore` is declared before `Files` so it encodes as a top-level array
ahead of the `[[files]]` array-of-tables — a bare key emitted after a table
would otherwise be parsed as belonging to that table):

```go
type Config struct {
    Ignore []string    `toml:"ignore,omitempty"`
    Files  []FileEntry `toml:"files"`
}

type FileEntry struct {
    Path    string `toml:"path"`
    Version string `toml:"version,omitempty"`
}
```

`config.LoadIgnore` reads only the `ignore` list, without validating the listed
targets, so `discover` can honor a stale config's patterns. It is deliberately
lenient: a missing or unparseable `--output` file yields no patterns and no
error, since `discover` overwrites that path and it may not currently be a valid
config.

### 6.2 Bump history / state file (`internal/history`)

After a successful, non-`--dry-run` bump in config mode, `incrmit` appends an
entry to a **state file** so the bump can be reverted by `incrmit undo`.

- **Location:** the file is named `.incrmit.state.toml` (`config.StateFileName`)
  and lives in the **same directory as the config** (`incrmit.toml`), so the two
  are always found together. `history.ResolvePath(configPath)` derives it from
  the resolved config path; `undo` uses the same rule (default or `--config`).
- **Lifecycle / retention:** it is a **stack** of entries, oldest first. Each
  bump pushes one entry; each `undo` pops the most recent. Only the last
  `history.MaxEntries` (20) bumps are retained so the file cannot grow without
  bound — at minimum the last bump is always undoable, and successive undos walk
  back through history.
- **Committed vs. ignored:** it is **local working state**, not meant to be
  committed. `undo` restores files in the working copy, so the journal belongs
  beside them; the file header recommends adding it to `.gitignore`. (In this
  repo `*.toml` is already git-ignored, which covers it.) Discovery never treats
  it (or `incrmit.toml`) as a target.
- **`--file` mode records nothing:** a single-file bump has no config-anchored
  location to keep or later find the state file, so no history is written and
  `undo` is a config-mode operation only.

The schema (TOML), encoded with the same `github.com/BurntSushi/toml` library:

```go
type Change struct {
    Path string `toml:"path"` // display path, as listed in the config
    FS   string `toml:"fs"`   // resolved (absolute) filesystem path
    Old  string `toml:"old"`  // version token before the bump
    New  string `toml:"new"`  // version token after the bump
}

type Entry struct {
    Timestamp time.Time `toml:"timestamp"`
    Config    string    `toml:"config,omitempty"` // resolved config path
    Changes   []Change  `toml:"changes"`
}

type History struct {
    Entries []Entry `toml:"entries"`
}
```

Paths are stored **resolved (absolute)** so `undo` can locate the files and the
config regardless of the working directory it is later run from. The file is
written atomically via `files.WriteAtomic`, the same path used for target files.

### 6.3 Version

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

### Undo command

```text
incrmit undo [flags]
```

| Flag        | Short | Description                                    | Default        |
| ----------- | ----- | ---------------------------------------------- | -------------- |
| `--config`  | `-c`  | Path to the TOML config file (locates state)   | `incrmit.toml` |
| `--dry-run` | `-d`  | Preview the revert (`new -> old`) without writing | `false`     |

Reverts the most recent recorded bump: it restores the previous version token
in every file that bump wrote and rewrites the config's recorded versions to
match, then pops the journal entry so a repeated `undo` walks further back. If a
file was edited since the bump (its current token no longer matches the recorded
`new` value), `undo` refuses and writes nothing, so user edits are never
clobbered. With no history to revert it prints a friendly message and exits `0`.

### Version command

```text
incrmit version
```

Prints the tool version. The `--version`, `-version`, and `-v` flags are
aliases for the subcommand.

### Help command

```text
incrmit help [command] [--no-banner]
```

| Invocation                | Output                                              |
| ------------------------- | --------------------------------------------------- |
| `incrmit help`            | Overview listing every command and its flags.       |
| `incrmit help bump`       | The default bump command's flags.                   |
| `incrmit help discover`   | The discover command's flags.                       |
| `incrmit help undo`       | The undo command's flags.                           |
| `incrmit help version`    | The version command's help.                         |
| `incrmit help help`       | The help command's own usage.                       |
| `incrmit -h` / `--help`   | Same overview as `incrmit help` (no subcommand).    |
| `incrmit <cmd> -h`        | The same help text as `incrmit help <cmd>`.         |

Help requested explicitly is written to stdout and exits `0`. An unknown
command prints an error and a hint to run `incrmit help` to stderr and exits
`2` (invalid arguments).

The overview is prefixed with an ASCII banner (the `banner` constant in
`internal/cli/help.go`). It is plain 7-bit ASCII, 35 columns wide, and carries
no color, so it renders the same on every platform. The banner is shown
unconditionally rather than only on a TTY, which keeps the overview
byte-for-byte reproducible and testable; `--no-banner` (accepted by
`incrmit help` and by top-level `-h` / `--help`) is the opt-out. It is scoped to
the overview, so per-command help stays terse.

All usage and help text lives in one place (`internal/cli/help.go`) so the
`-h` / `--help` output and the `help` command stay in sync; the commands'
`flag.FlagSet` usage handlers and the `help` dispatch both reference those
shared strings rather than duplicating them. Each command's flag block is
factored into a shared constant (`bumpFlags`, `discoverFlags`) that is composed
into both the per-command help and the top-level overview, so the overview can
list every flag without duplicating the flag text.

## 8. Processing Flow

### 8.1 Bump

1. Parse flags and resolve the bump component.
2. Resolve targets:
   - If `--file` is set, use that single file.
   - Otherwise load the config from `--config`.
3. Group config entries by file (a file listed once per distinct version is a
   single group) and read each file once. For every entry, determine the old
   version (from the config `version`, or by scanning the file when none is
   recorded) and apply the bump to get the new version.
4. If `--dry-run`, print `old -> new` for each entry and exit (no writes).
5. Otherwise rewrite each file once, replacing all of its known version tokens
   in a single pass (`files.SetKnownVersions`). A single pass over the original
   bytes keeps overlapping bumps from cascading (e.g. `1.2.3 -> 1.2.4` alongside
   `1.2.4 -> 1.2.5`) and avoids one entry's write clobbering another's when two
   versions live in the same file.
6. In config mode (not `--file`), rewrite `incrmit.toml` so each entry's
   `version` records the new value (one entry per distinct version per file),
   keeping the config in sync for the next run. The file is regenerated through
   `config.Marshal`, so the `[[files]]` entries and `ignore` list survive but
   user-authored comments and formatting do not. The output is deterministic:
   the same config bumped twice produces byte-identical layout.
7. In config mode, append a history entry (each file's path, resolved path, and
   `old`/`new` tokens, plus a timestamp and the config path) to the state file
   beside the config so the bump can be undone (see
   [section 6.2](#62-bump-history--state-file-internalhistory)).
8. Report results (files bumped, and each `old -> new`).

### 8.2 Discover

1. Read the `ignore` list from any config already at `--output`
   (`config.LoadIgnore`); an absent/unparseable file just yields no patterns.
2. Walk the tree from `--path`, skipping the built-in ignored directories
   (e.g. `.git`, `node_modules`, `vendor`, build outputs), the config file
   (`incrmit.toml` by name, plus the resolved `--output` path), and anything
   matched by the `ignore` patterns (see [section 8.3](#83-ignore-matching)).
3. For each candidate file, extract *every* semantic version occurrence,
   recording each one's line number and the trimmed text of its line
   (`discovery.Occurrence`). A file with at least one occurrence becomes a
   `Result`.
4. Turn results into config entries: one `[[files]]` entry per distinct version
   in each file (first-seen order), so identical repeats collapse and differing
   versions each get an entry sharing the path. The `ignore` list is written
   back verbatim so regeneration never drops it (a bump preserves it the same
   way when it rewrites the config). A description of the `ignore` option is
   always written above it; when there are no patterns yet, a commented-out
   example (`# ignore = [...]`) is emitted in their place so the feature is
   discoverable from a freshly written config. The comment block
   (`config.IgnoreComment`) is shared by both the discover generation and the
   bump-time rewrite (`config.Marshal`) so both files carry identical guidance.
5. If `--dry-run`, note the applied ignore rules and print each occurrence with
   its line number and context, then exit.
6. Otherwise write the generated config to `--output`.

### 8.3 Ignore matching

The `ignore` patterns are compiled into an `ignoreMatcher` (`discovery/ignore.go`)
and applied during the walk against paths **relative to the scan root**, always
in slash form, **case-sensitively**:

- A **trailing slash** marks a pattern as directory-only (`testdata/` prunes a
  directory but never matches a file).
- A pattern with **no slash** matches the base name of any file or directory at
  **any depth** via `path.Match` (`node_modules`, `*.lock`).
- A pattern **with a slash** is matched against the whole relative path,
  segment by segment; each segment is a `path.Match` glob and `**` matches zero
  or more segments (`docs/**` prunes `docs` and its subtree).

A matching directory returns `filepath.SkipDir` to prune its subtree; a matching
file is simply not recorded. These rules are applied *in addition to* the
built-in `ignoredDirs`: a path is skipped if either the built-in set or any
configured pattern matches.

### 8.4 Undo

1. Resolve the config path (default or `--config`) and derive the state file
   path beside it (`history.ResolvePath`). Load the journal; a missing file is
   an empty history.
2. Take the most recent entry. If there is none, print a friendly
   "nothing to undo" message and exit `0`.
3. In config mode load the config up front so a config problem aborts before any
   write (fail-fast, mirroring bump).
4. Read every recorded file once, group its changes, and build the reverse
   (`new -> old`) replacement in a single pass (`files.SetKnownVersions`), so
   overlapping reverts do not cascade. If a file no longer contains the recorded
   `new` token, it was edited since the bump: report the conflict and abort
   without writing anything.
5. If `--dry-run`, print `new -> old` for each change and exit (no writes).
6. Otherwise rewrite each reverted file once, then restore the config by setting
   each `(path, new)` entry back to its `old` version and rewriting
   `incrmit.toml` (the bump's self-update in reverse).
7. Pop the entry and save the journal so a repeated `undo` does not re-apply the
   same revert (and instead reverts the previous bump, if any).
8. Report the reverted files (each `new -> old`).

## 9. Version Detection Strategy

- Match candidate tokens with a regular expression: `\b[vV]?\d+(?:\.\d+)+\b`
  (a run of two or more dot-separated integer groups). The same pattern is used
  by both `discovery` (scanning) and `files` (rewriting) so the two never
  disagree about what counts as a token. The pattern intentionally matches more
  than `MAJOR.MINOR.PATCH`; `version.Parse` is the authority on validity and
  accepts only exactly three components.
- IPv4 addresses are not versions. Because the pattern matches the whole dotted
  run greedily, an address such as `192.168.1.1` is captured as a single
  four-component token and rejected by `Parse` — rather than having `192.168.1`
  (or `168.1.1`) sliced out of it. The same rule rejects two-component numbers
  (`3.9`) and any other `A.B.C.D...` token whose component count is not three,
  even when every component is a valid integer. Callers iterate matches and keep
  every token that `Parse` accepts (`detect`, which records all occurrences) or
  collect the distinct valid tokens (`FindVersion`).
- An optional single leading `v` or `V` is recognized (e.g. `v1.2.3`). Because
  the leading `\b` sits before the optional `[vV]`, the prefix is only consumed
  at a word boundary: look-alikes where the letter is part of a longer word
  (`rev1.2.3`, `dev1.2.3`) match neither the prefix nor the trailing digits, so
  they are rejected entirely. RE2 (Go's `regexp`) has no look-behind, so this
  boundary placement is what distinguishes a real prefix from an embedded one.
- The prefix is carried on the `version.Version` value (a `Prefix` field, the
  last struct field so existing keyed/zero literals are unaffected). `Parse`
  records it, `String` re-emits it, and the bump methods preserve it. This makes
  the whole pipeline prefix-aware for free: the prefix round-trips through the
  config `version` string and is re-detected from the file on each bump, so no
  separate config field is needed. A `v`-prefixed token and its bare form are
  therefore distinct tokens (a file containing both `v1.2.3` and `1.2.3` is
  ambiguous), and a bump rewrites only the exact written token.
- Discovery is content-based and format-agnostic: it scans the bytes of every
  text file, regardless of name or extension, and records every
  `[v]MAJOR.MINOR.PATCH` token found in each (with its line number and context).
  Distinct versions in one file become one config entry each; identical repeats
  collapse to a single entry and are all rewritten together on bump.
- Binary files (those containing a NUL byte) are skipped so version-like byte
  sequences in compiled artifacts are not matched.
- Two-component numbers (e.g. `3.9`) and other non-`X.Y.Z` strings do not match.
- On write, replace only the matched token to preserve surrounding formatting.

### Consequences of the atomic write

`files.WriteAtomic` writes a temp file in the target's directory and renames it
over the target, carrying the original file mode across. Two behaviors follow
from that and are intentional:

- A **read-only target is still rewritten** when its directory is writable,
  because the rename never opens the target for writing. Write protection comes
  from the containing directory's permissions, not the file's mode.
- An **unwritable directory fails before any change**: the temp file cannot be
  created, so the target keeps its original contents and no partial write or
  stray temp file is left behind.

## 10. Error Handling

- Missing config file: clear message; suggest running `incrmit discover`.
- No version found in a target: report the file and continue or fail fast
  (configurable; fail fast by default for bump).
- Multiple versions found in a single file: report ambiguity and skip unless a
  format-specific rule resolves it.
- Filesystem/permission errors: surface the underlying error and exit non-zero.
- Undo with nothing to revert (no journal or an emptied one): print a friendly
  message and exit `0` — it is not an error.
- Undo conflict (a file edited since the bump no longer holds the recorded `new`
  token): report the conflict, write nothing, and exit `1` (generic error).

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
├── main.go                 # entry point; delegates to internal/cli
├── internal/
│   ├── cli/                # flag parsing, dispatch, help text, exit codes
│   ├── config/             # TOML load and validation
│   ├── version/            # semantic version parse and bump
│   ├── discovery/          # filesystem scan and config generation
│   ├── history/            # bump journal (state file) for undo
│   ├── files/              # read/write helpers
│   └── buildinfo/          # tool version (stamped via -ldflags)
├── doc/
│   ├── DEVELOPMENT.md      # this document
│   ├── tasks.md            # milestone checklist
│   └── man/incrmit.1       # man page
├── packaging/nfpm.yaml     # .deb / .rpm package definition
├── scripts/                # release helpers (.pkg build, changelog notes)
├── incrmit.toml            # incrmit's own config (it bumps itself)
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
  `linux/amd64`, `linux/arm64`, `darwin/amd64`, `darwin/arm64`,
  `windows/amd64`, and `windows/arm64` into `dist/`, named
  `incrmit-<version>-<os>-<arch>`.
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
