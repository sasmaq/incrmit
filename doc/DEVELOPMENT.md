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

- Parsing semantic versions (`[v]MAJOR.MINOR.PATCH[-PRERELEASE][+BUILD]`) from
  text files.
- Incrementing the major, minor, or patch component.
- Promoting a prerelease to its release (`--release`) and starting or advancing
  one (`--pre`).
- Reading target files from a TOML configuration file.
- Discovering version-bearing files and generating a config automatically.
- Bumping a single file directly, bypassing the config.
- A dry-run mode that previews changes without writing.
- Undoing the most recent bump, restoring the previous version in every file it
  wrote (and in `incrmit.toml`), using a locally recorded bump history.

Out of scope (for the initial version):

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

| Term      | Meaning                                                            |
| --------- | ------------------------------------------------------------------ |
| Version   | A semver 2.0.0 token: `[v]MAJOR.MINOR.PATCH[-PRERELEASE][+BUILD]`. |
| Bump      | Incrementing one version component by one.                         |
| Promote   | Dropping a prerelease to reach its release.                        |
| Target    | A file whose version value `incrmit` reads and updates.            |
| Config    | The TOML file listing targets (`incrmit.toml` by default).         |
| Discovery | Scanning the tree to find targets and generate a config.           |

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
- `version` — parses semantic versions and applies major/minor/patch bumps,
  prerelease promotion, and prerelease iteration.
- `discovery` — walks the filesystem, detects version strings, and emits config.
- `files` — reads and writes target files, replacing only the version token.
- `lock` — takes one exclusive advisory lock per project so only one `incrmit`
  writes to it at a time (see [section 8.6](#86-concurrency-one-writer-per-project)).
- `output` — formats human-readable results and dry-run previews.

## 6. Data Model

### 6.1 Config Schema (TOML)

```toml
# incrmit.toml
ignore = ["testdata/", "*.lock", "docs/**"]   # optional; folders/files discover skips

[[files]]
path = "VERSION"
version = "1.2.3"     # optional; populated by `discover`
prerelease = "rc.1"   # optional; the section after "-" in the file's token
build = "exp.sha.5"   # optional; the section after "+"
```

A semver token is stored in three keys rather than one string: `version` holds
the numeric core (with any `v` prefix) and `prerelease`/`build` hold the sections
that follow, without their punctuation. `FileEntry.Token()` reassembles them.
See [section 9.2](#92-prerelease-and-build-metadata) for why the split matters —
in short, it is what tells the rewriter which hyphenated suffix is a prerelease
and which is part of a filename. A bump that drops the prerelease drops the key
with it, so the config reads as the project's actual state.

`Load` migrates a token written inline (`version = "1.2.3-rc.1"`, the shape
written before these keys existed) into the split form, and rejects an entry
that spells a section both ways. A `version` that does not parse is left
verbatim, so an unparseable version stays an exit-3 failure from the command
that needs it rather than becoming an exit-1 config-loading error.

A `path` may appear in more than one `[[files]]` entry when a file contains
several *differing* versions, as long as each entry pins a distinct, non-empty
version (one entry per distinct version). Distinctness is judged on the whole
token, so `1.2.3` and `1.2.3` + `prerelease = "rc.1"` are two entries, not a
duplicate. Validation rejects exact `(path, token)` duplicates and any repeated
path where an entry omits the version (which would be ambiguous). Identical
repeats of the same version within a file collapse to a single entry.

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
    Path       string `toml:"path"`
    Version    string `toml:"version,omitempty"`
    Prerelease string `toml:"prerelease,omitempty"`
    Build      string `toml:"build,omitempty"`
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

### 6.4 Project lock file (`internal/lock`)

`.incrmit.lock` (`config.LockFileName`) sits next to the config and carries no
state: what matters is the exclusive advisory lock held on it while a command
writes (see [section 8.6](#86-concurrency-one-writer-per-project)). Its contents
are a two-line note explaining what the file is, written once the lock is held,
so someone who finds it in a tree is not left guessing. The note deliberately
contains no version-like token, and the walk skips the file by name, so incrmit
can never discover its own lock as a target.

Like the state file it is local working state and belongs in `.gitignore`. It is
never unlinked on release: removing it would let a second run create and lock a
fresh file at the same name while the first still held a lock on the old inode,
which is the exact race the lock exists to prevent.

**Only a regular file, never through a link.** The lock file is the one path
incrmit opens for writing instead of replacing by rename: `files.WriteAtomic`
renames over a link rather than writing through it, and `files.SweepTemps`
unlinks a link named like a temp file rather than its target. Before
Milestone 32 the lock opened with `O_CREATE|O_RDWR`, which follows a symlink,
and truncated whatever it got to write the note. A repository can commit
`.incrmit.lock` as a link, so running `discover` in a fresh clone (it needs no
config) or a bump replaced any file the user could write with the note, and a
dangling link created the file it named. `openLockFile` now opens with
`O_NOFOLLOW` on Unix and `FILE_FLAG_OPEN_REPARSE_POINT` on Windows, so refusing
the link and opening the file are one step, with no window between an `Lstat`
and an `Open` for a link to be swapped in. `O_NONBLOCK` keeps a FIFO or a device
from blocking the open. The descriptor is then `Stat`ed, and only a regular file
is locked.

Anything else at the path — a link, a directory, a FIFO — **degrades** the lock,
keeping section 8.6's rule that only contention refuses. The reason is a
`lock.NotRegularError` naming the kind of file; the command warns with the path
and that kind ("is a symbolic link, not a regular file"), continues unlocked,
and sweeps no temp files. Refusing was considered and rejected: nothing is
opened through the path either way, so refusing protects nothing more, and it
would make a stray file at that name a reason no bump can run. The warning is
what points the user at a planted file, and the file is left exactly as it was
so they can see what was put there.

The note is written only into a lock file that is empty and has one name, which
in practice means one a run just created. A regular file that happens to sit at
the lock path is locked but never rewritten, whether it holds someone's text or
is an empty file hard-linked from elsewhere: the note is a courtesy and must
never be the reason a file loses data.

## 7. Command-Line Interface

### Default command (bump)

```text
incrmit [flags]
```

| Flag              | Short | Description                                        | Default        |
| ----------------- | ----- | -------------------------------------------------- | -------------- |
| `--config`        | `-c`  | Path to the TOML config file                       | `incrmit.toml` |
| `--file`          | `-f`  | Bump the version in one file (skips config)        | *none*         |
| `--major`         | `-M`  | Bump the major version (resets minor and patch)    | `false`        |
| `--minor`         | `-m`  | Bump the minor version (resets patch)              | `false`        |
| `--patch`         | `-p`  | Bump the patch version                             | `true`         |
| `--release`       | `-r`  | Promote a prerelease (`1.2.3-rc.1` -> `1.2.3`)     | `false`        |
| `--pre`           | `-e`  | Start or advance a prerelease with this identifier | *none*         |
| `--max-file-size` | `-s`  | Refuse to read a target larger than this size      | `0` (no limit) |
| `--dry-run`       | `-d`  | Print the new version without writing to the files | `false`        |

### Discover command

```text
incrmit discover [flags]
```

| Flag              | Short | Description                                       | Default        |
| ----------------- | ----- | ------------------------------------------------- | -------------- |
| `--path`          | `-P`  | Root directory to scan                            | `.`            |
| `--output`        | `-o`  | Path to write the generated config file           | `incrmit.toml` |
| `--max-file-size` | `-s`  | Skip files larger than this size                  | `32MiB`        |
| `--dry-run`       | `-d`  | Print discovered files without writing the config | `false`        |

If more than one of `--major`, `--minor`, `--patch` is supplied, the highest
component wins (major > minor > patch). When none is supplied, patch is used.

`--release` and `--pre` select the prerelease transforms instead of a plain
component bump; see [section 9.2](#92-prerelease-and-build-metadata) for their
semantics and the combinations that exit `2`.

A `--max-file-size` value is parsed by `cli.parseSize`: a plain byte count
(`1048576`) or a value with a unit suffix (`512KB`, `32MiB`, `2G`; bare `K`/`M`/
`G` are binary, `KB`/`MB`/`GB` decimal), case-insensitive. A size of `0` means
no limit; a negative or unparseable value is a usage error (exit `2`).

### Preview command

```text
incrmit preview [flags]
```

| Flag              | Short | Description                                   | Default        |
| ----------------- | ----- | --------------------------------------------- | -------------- |
| `--config`        | `-c`  | Path to the TOML config file                  | `incrmit.toml` |
| `--file`          | `-f`  | Preview one file (skips config)               | *none*         |
| `--max-file-size` | `-s`  | Refuse to read a target larger than this size | `0` (no limit) |

A read-only view of what the three component bumps would produce for every
configured file, so all of them are visible at once without a `--dry-run` per
component. It writes nothing: no target, no config, no history. The name
`preview` was chosen over `show`, `plan`, and `list` because it says what the
output is (a projection, not the current state) without implying a staged
change the way `plan` does.

Target resolution and the read/parse pass are shared with the bump command
(`resolveTargets` and `readGroups`), so the current version a preview reports is
by construction the one a bump would start from — including the config-pinned
token for a file that holds several versions.

Output is an aligned table with one row per `(path, version)`; columns are
padded to their widest value, and no line carries trailing whitespace. The
projected versions come from `version.BumpPatch/BumpMinor/BumpMajor`, so the
`v` prefix is carried through and a prerelease or build section is dropped
exactly as a real bump would. An exact `(path, version)` repeat is printed once.

Rows whose version differs from the one most entries hold are marked `*`, with a
footnote naming that version. Drift is judged by semver precedence
(`precedenceKey`), so entries differing only in the `v` prefix or in build
metadata are *not* marked — they name the same release, and marking them would
bury the rows that really are behind. The most common version wins; ties go to
the higher one, which is deterministic and reads the stragglers as stale.

There is deliberately no machine-readable mode: the table is for people, and a
`--json` (or tab-separated `--plain`) output would be a second, separately
stable contract for data the config already holds. Scripts that need the next
version should read `incrmit.toml` or run a `--dry-run` bump.

Error paths use the shared exit codes: a missing config is exit `1` with the
`incrmit discover` hint from `config.NotExistError`, bad flags exit `2`, and an
unparseable or absent version token exits `3` naming the file.

### Undo command

```text
incrmit undo [flags]
```

| Flag        | Short | Description                                       | Default        |
| ----------- | ----- | ------------------------------------------------- | -------------- |
| `--config`  | `-c`  | Path to the TOML config file (locates state)      | `incrmit.toml` |
| `--dry-run` | `-d`  | Preview the revert (`new -> old`) without writing | `false`        |

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
| `incrmit help preview`    | The preview command's flags.                        |
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
factored into a shared constant (`bumpFlags`, `discoverFlags`, `previewFlags`,
`undoFlags`, `helpFlags`) that is composed
into both the per-command help and the top-level overview, so the overview can
list every flag without duplicating the flag text.

## 8. Processing Flow

### 8.1 Bump

1. Parse flags and resolve the bump component.
2. Unless `--dry-run`, take the project lock and hold it until the command
   returns (see [section 8.6](#86-concurrency-one-writer-per-project)). It is
   taken *before* the config is read, because everything from that read to the
   last write is one read-modify-write. Once the targets are known, clear any
   temp file a crashed run left in a directory this bump writes to.
3. Resolve targets:
   - If `--file` is set, use that single file.
   - Otherwise load the config from `--config`.
4. Group config entries by file (a file listed once per distinct version is a
   single group) and read each file once, refusing one larger than
   `--max-file-size` when a cap is set (see
   [section 9.1](#91-scan-boundaries)). For every entry, determine the old
   version (from the config `version`, or by scanning the file when none is
   recorded) and apply the bump to get the new version.
5. If `--dry-run`, print `old -> new` for each entry and exit (no writes).
6. Otherwise rewrite each file once, replacing all of its known version tokens
   in a single pass (`files.SetKnownVersions`). A single pass over the original
   bytes keeps overlapping bumps from cascading (e.g. `1.2.3 -> 1.2.4` alongside
   `1.2.4 -> 1.2.5`) and avoids one entry's write clobbering another's when two
   versions live in the same file.
7. In config mode (not `--file`), rewrite `incrmit.toml` so each entry's
   `version` records the new value (one entry per distinct version per file),
   keeping the config in sync for the next run. The file is regenerated through
   `config.Marshal`, so the `[[files]]` entries and `ignore` list survive but
   user-authored comments and formatting do not. The output is deterministic:
   the same config bumped twice produces byte-identical layout.
8. In config mode, append a history entry (each file's path, resolved path, and
   `old`/`new` tokens, plus a timestamp and the config path) to the state file
   beside the config so the bump can be undone (see
   [section 6.2](#62-bump-history--state-file-internalhistory)).
9. Report results (files bumped, and each `old -> new`).

### 8.2 Discover

1. Unless `--dry-run`, take the project lock for the directory holding
   `--output` and hold it until the command returns
   (see [section 8.6](#86-concurrency-one-writer-per-project)). `discover`
   rewrites the config it reads its `ignore` list from, so that read and the
   write are one read-modify-write like a bump's.
2. Read the `ignore` list from any config already at `--output`
   (`config.LoadIgnore`); an absent/unparseable file just yields no patterns.
3. Walk the tree from `--path`, skipping the built-in ignored directories
   (e.g. `.git`, `node_modules`, `vendor`, build outputs), incrmit's own files
   (`incrmit.toml` by name plus the resolved `--output` path, the state file,
   the lock file, and any `.incrmit-*.tmp` write in flight), symlinks of any
   kind, and anything matched by the `ignore` patterns (see
   [section 8.3](#83-ignore-matching)).
4. For each candidate file, extract *every* semantic version occurrence,
   recording each one's line number and the trimmed text of its line
   (`discovery.Occurrence`). A file with at least one occurrence becomes a
   `Result`. Only regular files no larger than the scan cap
   (`--max-file-size`, default `discovery.DefaultMaxScanBytes` = 32 MiB) are
   read; see [section 9.1](#91-scan-boundaries).
5. Turn results into config entries: one `[[files]]` entry per distinct version
   in each file (first-seen order), so identical repeats collapse and differing
   versions each get an entry sharing the path. The `ignore` list is written
   back verbatim so regeneration never drops it (a bump preserves it the same
   way when it rewrites the config). A description of the `ignore` option is
   always written above it; when there are no patterns yet, a commented-out
   example (`# ignore = [...]`) is emitted in their place so the feature is
   discoverable from a freshly written config. The comment block
   (`config.IgnoreComment`) is shared by both the discover generation and the
   bump-time rewrite (`config.Marshal`) so both files carry identical guidance.
6. If `--dry-run`, note the applied ignore rules and print each occurrence with
   its line number and context, then exit.
7. Otherwise write the generated config to `--output`.

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

1. Resolve the config path (default or `--config`). Unless `--dry-run`, take the
   project lock for its directory and hold it until the command returns, before
   the journal is read (see
   [section 8.6](#86-concurrency-one-writer-per-project)); an undo is the same
   read-modify-write as a bump, in reverse, so the two can never interleave.
   Derive the state file path beside the config (`history.ResolvePath`) and load
   the journal; a missing file is an empty history.
2. Take the most recent entry. If there is none, print a friendly
   "nothing to undo" message and exit `0`.
3. In config mode load the config up front so a config problem aborts before any
   write (fail-fast, mirroring bump).
4. Read every recorded file once, group its changes, and build the reverse
   (`new -> old`) replacement in a single pass (`files.SetKnownVersions`), so
   overlapping reverts do not cascade. If a file no longer contains the recorded
   `new` token, it was edited since the bump — or another run bumped past it,
   since a lock only serializes the runs that take it: report which file
   diverged and abort without writing anything, rather than putting an older
   version back over newer work.
5. If `--dry-run`, print `new -> old` for each change and exit (no writes).
6. Otherwise rewrite each reverted file once, then restore the config by setting
   each `(path, new)` entry back to its `old` version and rewriting
   `incrmit.toml` (the bump's self-update in reverse).
7. Pop the entry and save the journal so a repeated `undo` does not re-apply the
   same revert (and instead reverts the previous bump, if any).
8. Report the reverted files (each `new -> old`).

### 8.5 Preview

1. Parse the preview flags. Resolve the targets exactly as a bump does
   (`resolveTargets`): the single `--file` target, or every entry in the config.
2. Read each distinct target once and determine the version it holds
   (`readGroups`) — the config-pinned token when there is one, otherwise the
   version found by scanning. This is steps 2–3 of the bump flow, shared rather
   than reimplemented, so a preview cannot disagree with the bump it previews.
3. Project each version through all three component bumps and flatten the groups
   into rows, dropping an exact `(path, version)` repeat (`previewRows`).
4. Compare the rows by semver precedence and mark those that differ from the
   most common version (`markDrift`).
5. Render the aligned table, plus the drift footnote when anything is marked.
   Nothing is written to disk on any path.

### 8.6 Concurrency: one writer per project

`incrmit` starts no goroutines, so this is not about internal races — it is
about two processes. Every mutating command is a read-modify-write over shared
on-disk state, and before Milestone 29 nothing coordinated them: run two bumps
at once and the second save silently erased the first, leaving the tree bumped
twice while the config recorded one version and the journal one entry. The
erased bump could never be undone, because nothing recorded it.
`files.WriteAtomic` is what made this easy to miss: every individual write is
all-or-nothing, so nothing is ever *corrupt*; it is only lost.

The rule is now **one writer per project at a time, readers unsynchronized**:

- **What takes the lock.** `bump`, `undo`, and `discover` when it writes the
  config. Each takes it *before* its first read and holds it until it returns,
  on every path, so the whole read-modify-write is covered.
- **What does not.** `preview` and every `--dry-run`. Inspecting a project can
  neither block nor be blocked; the price is that a reader may observe a bump in
  progress and see a partly updated tree.
- **Scope.** The directory holding the config, so separate projects never
  contend. In `--file` mode there is no config to anchor to, so the lock goes
  beside the file being bumped — which is what two concurrent `--file` runs on
  the same target have in common.
- **Mechanism.** One exclusive advisory lock (`syscall.Flock` on Unix,
  `LockFileEx` on Windows, in build-tagged files mirroring the
  `internal/testutil/fifo_unix.go` / `fifo_windows.go` split). The standard
  library's `syscall` package has no `LockFileEx`, so the Windows side calls
  the typed wrapper in `golang.org/x/sys/windows` rather than loading
  `kernel32.dll` by hand, which would need `unsafe` to pass the `OVERLAPPED`
  pointer. `x/sys` is maintained by the Go team and has no dependencies of its
  own. An OS advisory lock is used rather than an `O_EXCL` PID file
  specifically because of stale locks: the kernel releases the lock when the
  process exits for any reason, `SIGKILL` and panics included, so there is never
  a leftover lock to clear by hand. A PID file outlives the crash and forces the
  tool to guess whether the owner is still alive.
- **Contention fails fast**, exiting `1` with a message naming the project and
  `--wait`. A bump takes milliseconds, so a second one arriving mid-run is
  usually a mistake rather than a queue, and a tool that blocks silently turns a
  CI misconfiguration into a hung job instead of a failed one. A refused run has
  written nothing. `--wait` (`-w`) opts into queueing for callers who really are
  serializing work.
- **Unavailable locking degrades, it does not refuse.** On a filesystem that
  does not implement locking (some NFS mounts, a few CI overlay filesystems), in
  a directory that cannot be written, or when the lock path holds something
  other than a regular file (a symbolic link, say; see
  [section 6.4](#64-project-lock-file-internallock)), `lock.Acquire` returns a
  *degraded* lock rather than an error: the command warns and continues
  unlocked, because a tool that cannot bump at all is worse than one that
  cannot detect a second run. Contention is therefore the only error `Acquire` reports, which is what
  keeps "someone else holds it" distinguishable from "locking is not available".

Because flock (like a Windows byte-range lock) is held by the open file
description rather than by the process, two runs inside one test binary contend
exactly as two shells would. That is what lets the concurrency tests drive real
contention with goroutines calling `cli.Main` instead of spawning children.

**In-flight temp files.** `files.WriteAtomic` writes `.incrmit-*.tmp` beside its
target, and such a file is a copy of a target with a new version already in it.
The discovery walk skips the pattern (`files.IsTempName`), so a scan that catches
a concurrent run mid-write — or finds the residue of a crashed one — never
records one as a real target. A run that holds the lock also sweeps them
(`files.SweepTemps`) from the directories it is about to write in, which is the
one moment they are provably safe to delete: no other run can have a write in
flight. A degraded lock buys no such guarantee, so nothing is swept then.

## 9. Version Detection Strategy

- `version.FindTokens` is the single definition of where a version begins and
  ends. Both `discovery` (scanning) and `files` (rewriting) call it, so the two
  cannot disagree about what counts as a token — they used to hold identical
  copies of the pattern, which is exactly the kind of duplication that drifts.
  It matches candidates with:

  ```text
  \b[vV]?\d+(?:\.\d+)+(?:-[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?\b
  ```

  a run of two or more dot-separated integer groups, plus the optional
  `-prerelease` and `+build` sections. The pattern intentionally matches more
  than a valid version; `version.Parse` is the authority on validity and accepts
  only exactly three numeric components.
- The suffixes are part of the token, not trailing text beside it. Stopping at
  the numeric core would rewrite the `1.2.3` inside `1.2.3-rc.1` and leave the
  `-rc.1` dangling on a version it no longer names — the bug Milestone 28 fixed.
  Because RE2 has no look-ahead, the trailing `\b` is what stops a suffix from
  running into an adjacent word: `1.2.3-rc_1` matches only `1.2.3`, since `_` is
  a word character and no boundary falls before it, and a token at the end of a
  sentence (`... 1.2.3-rc.1.`) keeps its identifiers but not the full stop.
- **The filename guard.** A hyphen is a legal prerelease identifier character,
  so semver alone cannot tell `1.2.3-rc.1` from the version inside
  `incrmit-1.2.3-linux-amd64.tar.gz` — the latter parses perfectly well as
  `1.2.3` with the prerelease `linux-amd64.tar.gz` (the dotted-identifier run
  swallows `.tar` and `.gz` as further identifiers). Taking that as the version
  makes a bump rewrite the whole token, silently reducing the line to
  `incrmit-1.2.4`. That is data loss on exit `0`, and it hits precisely the
  install instructions and release scripts a version-bumping tool is aimed at.
- The distinguishing signal is not the suffix but what *precedes* the version: a
  real prerelease token stands on its own (after a quote, an `=`, whitespace, or
  the start of a line), while the filename case has the version welded into a
  longer hyphen-joined word. So `FindTokens` disowns the suffix — keeping only
  the numeric core, exactly what the matcher found before prerelease support
  existed — when the token is preceded by `-` and that hyphen is itself preceded
  by a word character. RE2 has no look-behind, so this is a check on the
  surrounding bytes after matching rather than part of the pattern.
- The guard alone cannot promote a prerelease that lives inside a filename: it
  cuts `app-1.2.3-rc.1.zip` back to `1.2.3`, so the bump writes
  `app-1.2.4-rc.1.zip` rather than the semver-correct `app-1.2.4.zip`. The
  config closes that gap — see below.

### 9.1 Scan boundaries

Discovery may be pointed at a tree the user did not author, so `Discover` reads
only what it was aimed at and only what it can read in bounded time and memory:

- **Symlinks are skipped** (`d.Type()&fs.ModeSymlink`), both to files and to
  directories. Go's `filepath.WalkDir` already declines to descend into a
  symlinked directory, but it reports symlinked *files* as ordinary entries, and
  reading one follows the link. Following a link would let an entry outside the
  tree be read, printed as dry-run context, and — once written to the config —
  copied into the tree by the next bump.
- **Only regular files are read.** `detect` stats the path *before* opening it,
  because opening a FIFO blocks until a writer appears (which would hang the
  whole scan), and a character device such as `/dev/zero` streams without end.
  The check is repeated on the open descriptor so a path swapped in between is
  still rejected.
- **Reads are capped at `DefaultMaxScanBytes` (32 MiB)**, enforced by the size
  check and again on the read (`readAtMost`) in case the file grows in between.
  A file that grew past the cap is refused whole rather than scanned up to it:
  a cut falling mid-token would read `1.2.34` as `1.2.3` and record a version
  the file does not hold. Version tokens live in small text files, so the cap
  costs nothing in practice while keeping peak memory bounded regardless of what
  the tree contains.
  `DiscoverWithLimit` takes the cap as a parameter (`Discover` supplies the
  default), which is what `discover --max-file-size` sets; a cap of `0` disables
  it and scans every regular file.
- **Bump targets have no cap by default.** Files listed in the config are chosen
  by the user, so `files.ReadTarget` reads them whole. Passing
  `--max-file-size` to a bump switches it to `files.ReadTargetWithLimit`, which
  refuses an oversized target with a `*files.TooLargeError` during planning, so
  nothing is written. Unlike the scan, a capped read never returns truncated
  data: the bumped bytes are written back over the file, so a file that grew
  past the cap mid-read is rejected rather than shortened.
- **A scan is linear in the file, however many tokens it holds.** Each
  occurrence records its line number and the text around it, and both come from
  a cursor (`lineCursor`) that only moves forward as the tokens are visited in
  order. Recomputing them from the start of the file for every token, as the
  scan once did, was quadratic: a minified file is one line, and twenty
  thousand occurrences on one 620 KB line took seconds and allocated about 12
  GB, because every occurrence also held its own copy of the whole line. The
  context kept per occurrence is now capped at `maxContext` (80) bytes on each
  side of the token, cut on a UTF-8 boundary and marked with `...`.
- **A line ends at `\n`, `\r\n`, or a lone `\r`**, the conventions an editor
  recognizes, so line numbers match what the user sees. Counting only `\n`
  reported a classic-Mac file as one line and printed its carriage returns into
  the dry run, where the terminal obeyed them and overprinted the output.

Config target paths are a separate matter: they are trusted input (see
[section 6.1](#61-config-schema-toml)) and may be absolute or reach outside the
config's directory.

### Consequences of the atomic write

`files.WriteAtomic` writes a temp file in the target's directory and renames it
over the target, carrying the original permission bits across. The file at the
path afterwards is therefore a new file, not the old one rewritten, and several
behaviors follow from that. All are intentional, and each is pinned by a test
(`internal/files/shapes_test.go`, `files_test.go`):

- A **read-only target is still rewritten** when its directory is writable, and
  stays read-only, because the rename never opens the target for writing. Write
  protection comes from the containing directory's permissions, not the file's
  mode.
- An **unwritable directory fails before any change**: the temp file cannot be
  created, so the target keeps its original contents and no partial write or
  stray temp file is left behind.
- A **symlinked target is replaced, not written through**: the rename swaps the
  name itself, so the link becomes a regular file and whatever it pointed at is
  untouched. A link inside the tree therefore cannot redirect a write elsewhere.
- The **setuid, setgid, and sticky bits are dropped**, since only `Perm()` is
  carried across. Re-applying setuid would mint a setuid file owned by whoever
  ran `incrmit` — the same reason the kernel clears those bits when an
  unprivileged process writes such a file in place.
- A **hard link is broken**: the other names keep the old file and the old
  contents. Keeping the link would mean truncating and rewriting the shared file
  in place, which is exactly the partial write the rename exists to rule out. A
  config that lists every name still bumps them all, because planning reads
  every target before the first write.
- **Ownership, extended attributes, and ACLs** are those of a new file created
  by the running user. This one is not tested, since changing a file's owner
  needs privileges the test suite does not have.

The temp file is created with `os.CreateTemp` in the target's own directory —
never a shared temp directory — which gives it an unpredictable name, `O_EXCL`
creation, and mode `0600` for the duration of the write. Its mode is set through
the open descriptor (`(*os.File).Chmod`) rather than by name, so the mode lands
on the file just written even if the name is replaced first. The final mode is
exactly the target's previous mode, or `0644` for a newly created file: a write
never widens permissions.

### 9.2 Prerelease and build metadata

`version.Version` carries `Prerelease` and `Build` alongside `Prefix`, each
holding its section without the leading `-`/`+`. `Parse` enforces the semver
2.0.0 grammar for both (dot-separated identifiers of ASCII letters, digits, and
hyphens; a numeric prerelease identifier may not carry a leading zero, though
build identifiers may), and `String` re-emits them, so any accepted token
round-trips unchanged. Build metadata is split off first: `+` cannot appear
anywhere else, while `-` may appear inside a build identifier (`1.2.3+exp-1`).
Because both characters are consumed by those splits, a signed component such as
`+1.2.3` or `1.-2.3` now fails as an empty or short numeric core rather than
through a dedicated sign check.

Bump semantics:

- `BumpMajor` / `BumpMinor` / `BumpPatch` **drop both sections**: `1.2.3-rc.1`
  patches to `1.2.4`. Carrying a prerelease forward would claim the new version
  is still a preview of a release it no longer names, and build metadata
  describes one build, so it is never inherited. This matches `bump2version`,
  `npm version`, and `cargo-release`.
- `Release` promotes in place (`1.2.3-rc.1` -> `1.2.3`), changing no numbers.
- `StartPrerelease(id)` sets `-id.1`; `AdvancePrerelease` increments the trailing
  numeric identifier (`rc.1` -> `rc.2`, `rc` -> `rc.1`); `BumpPrerelease(id)`
  advances when the series matches and starts it otherwise. All three drop build
  metadata.

The CLI exposes these as `--release`/`-r` and `--pre`/`-e <id>` rather than
overloading the component flags. `--pre` combines with them by rule: naming a
component explicitly always opens a new release line
(`--minor --pre rc` -> `1.3.0-rc.1`), while with no component flag the current
version decides — a release starts the next patch's prerelease
(`1.2.3` -> `1.2.4-rc.1`) and a version already in a prerelease iterates on the
spot (`1.2.4-rc.1` -> `1.2.4-rc.2`) rather than skipping a patch per preview.
`--patch` defaults to `true` and `--pre` to `""`, so the values alone cannot
distinguish an explicit choice from a default; `flag.FlagSet.Visit` records
which flags the user actually named.

Meaningless combinations exit `2` (`ExitUsage`): `--release` with `--pre`,
`--release` with a component flag, an invalid `--pre` identifier, and `--release`
on a version that has no prerelease. The last one can only be detected once a
target's version is known, so the bump transform returns an error rather than a
bare `Version`; `cli.usageError` marks it and `classify` maps it to `ExitUsage`.

Because a bump transform can now fail per target, the plan phase (`planGroups`)
reports the error and aborts before anything is written — an out-of-sync config
where only some entries carry a prerelease fails fast rather than half-applying.

`version.Compare` implements semver precedence (numeric components, then the
prerelease rules: a prerelease precedes its release, numeric identifiers compare
numerically and rank below alphanumeric ones, a shorter run of otherwise-equal
identifiers ranks lower). Neither `Prefix` nor `Build` affects precedence.
Nothing in the bump, discover, or undo paths needs ordering — they match tokens,
never compare them — so `Compare` exists for the out-of-sync/preview reporting
of Milestone 27 and for any future check that has to say which of two versions
is newer.

### 9.3 What the config knows that the grammar cannot

The guard is a heuristic about *where* a version sits, and a heuristic is all
that is available when scanning free text. A config entry is not a heuristic: it
states outright that this file's version is `1.2.3` with the prerelease `rc.1`.

That is why the prerelease and build sections are separate keys
([section 6.1](#61-config-schema-toml)) rather than one token string.
`files.SetKnownVersions` takes `[]files.Replacement` (parsed versions, not text)
and matches in two ways:

1. an exact token match, as before; or
2. a token the guard cut back to its core, where the bytes that follow continue
   with *exactly* the pinned suffix — in which case the match extends over it.

So a pin of `1.2.3-rc.1` rewrites the whole `1.2.3-rc.1` inside
`app-1.2.3-rc.1.zip` (promoting it to `app-1.2.3.zip`), while a pin of plain
`1.2.3` rewrites only the numbers inside
`incrmit-1.2.3-linux-amd64.tar.gz`. The byte after a consumed suffix must not be
alphanumeric, which stops a pinned `-rc.1` from matching the front of `-rc.10`;
when several pins could match one occurrence, the one with the longest suffix
wins, since it is the pin that claims the suffix.

This also keeps a prerelease cycle in step across a whole file: `--pre` writes
`1.2.4-rc.1` into a download URL, and `--release` finds it there again and
promotes it, instead of leaving the filename stranded at `-rc.1`.

Two consequences worth knowing:

- `--file` mode has no config, so it keeps the guard's behavior:
  `app-1.2.3-rc.1.zip` bumps to `app-1.2.4-rc.1.zip`. Pinning versions with
  `incrmit discover` is what buys the semver-correct result.
- A file holding the same version both standalone with a prerelease
  (`1.2.3-rc.1`) and inside a filename (`app-1.2.3-rc.1.zip`) presents two
  distinct tokens to an unpinned scan, so `--file` reports it as ambiguous
  rather than guessing. In config mode a single pinned entry covers both.
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

### 9.4 File shapes

The scanner and the rewriter work on bytes, not text, and the only bytes they
interpret are the ASCII digits, dots, letters, and hyphens of a token and the
word boundaries around it. Everything else passes through untouched, which is
what makes the shapes below round-trip:

- **Line endings are never normalized.** CRLF, LF, mixed, and lone-CR files keep
  every terminator; `\r` is a non-word byte like any other, so a token right
  against one is found whole.
- **A UTF-8 byte-order mark survives** and does not hide a token that follows
  it directly: its three bytes are non-word bytes, so every token range sits
  exactly three bytes later than in the same file without the mark. Discovery
  leaves the mark out of the first line's dry-run context.
- **A missing trailing newline stays missing**, and a token at byte 0 or flush
  against EOF is found whole: the filename guard's `start < 2` branch and
  `matchAt`'s `after < len(data)` check are the two places that read past a
  token, and both are bounds-checked.
- **ASCII-compatible encodings work.** Latin-1 and its relatives hold no `NUL`,
  so discovery treats them as text, and a non-UTF-8 byte next to a token is a
  word boundary like any other non-ASCII byte.
- **UTF-16 does not.** Every character is two bytes, one of them `NUL`, so the
  digits of a version are never adjacent and there is no token to find: a bump
  reports `no semantic version found` (or `expected version not found` for a
  pinned version) with exit 3 and writes nothing, and discovery skips the file
  as binary. Supporting it would mean decoding and re-encoding the file, which
  is exactly the normalization this design avoids; a documented refusal is the
  better failure than a rewrite that changes the encoding.
- **An empty or whitespace-only file has no version**, reported as such by every
  entry point.

Names are taken as literally as bytes: a `--file` argument or a config `path` is
a file name, never a pattern, and nothing in the read, lock, or write path
interprets a leading dash, a newline, or a glob metacharacter in one. The temp
file's name does not derive from the target's, so a name already at the
255-byte limit is written like any other. The config's `ignore` list is the one
place those characters are patterns; bracketing a metacharacter (`[*]`) is the
way to match it literally, since backslashes there are normalized to slashes
and cannot escape anything.

### 9.5 Terminal output

`incrmit` prints bytes it did not write: file names from the tree it scanned,
lines of those files as dry-run context, paths recorded in the config and the
journal, and OS errors that embed them. `discover` is meant for trees the user
does not own, so any of these can carry an escape sequence that clears the
screen, retitles the window, plants an OSC 8 hyperlink, or — on terminals that
honor OSC 52 — writes to the clipboard, and CI logs render ANSI as well.
`internal/cli/display.go` keeps all of it off the terminal in two layers:

- **`terminalWriter` is the guarantee.** `cli.Main` wraps stdout and stderr in
  it, so every byte printed passes through `terminalText`, which replaces each
  character that is not printable (`strconv.IsPrint`), a newline, or a tab with
  its Go escape: `\x1b`, `\u202e`, and `\xff` for a byte that is not UTF-8. That
  covers the C0 and C1 controls, `DEL`, the raw one-byte CSI `0x9B`, and every
  format character, the bidirectional overrides and isolates among them. It
  also covers output `incrmit` does not format itself, such as the `flag`
  package echoing an unknown flag, and any print site written later.
- **`displayName` makes a name readable under it.** Paths, flag values, and
  ignore patterns are rendered unchanged when every character is printable,
  and as a Go-quoted string otherwise, which is also how the messages that
  format a path with `%q` already show one. The rendering is injective: a
  quoted rendering always starts with `"`, an unquoted one never does (a
  printable name that begins with `"` is quoted too), and a quoted one reads
  back with `strconv.Unquote`, so no hostile file can be named to look like
  another one. The writer alone would be safe but ambiguous, since a literal
  `\x1b` in a name reads the same as an escape character.

Decisions recorded with it:

- **Tabs pass through the writer** but are quoted in a name. A tab only moves
  the cursor forward, so it cannot overprint or hide anything, and it is how
  Makefiles and Go files indent — exactly the context lines it appears in. In a
  name it would make two names look alike.
- **A backslash is escaped only inside a quoted name.** Windows paths print as
  typed, and injectivity comes from the leading quote rather than from
  escaping every backslash.
- **Escaping is always on**, not only when stdout is a terminal: CI logs are
  not a terminal and render escape sequences anyway, and output that does not
  depend on where it goes is easier to test and to diff.
- **Only the display is escaped.** The name used to open, lock, and rename a
  file, and the one written to the config and the journal, stay the raw bytes;
  TOML escapes control characters in a string itself, so such a name
  round-trips exactly.
- **`fsErrorMessage` drops an `*fs.PathError`'s own path**, wrapped or not,
  which repeats the name raw ahead of the reason, and keeps only the reason
  after the name it has already rendered.
- **A discovered name that is not UTF-8 is skipped with a warning**
  (`excludeUnlistable`). Linux allows any bytes in a name, but a TOML string
  must be UTF-8 and has no escape for a raw byte, so the encoder wrote such a
  name as-is and produced a config every later command refused to load.
  `discovery.Generate` refuses one as well, as a backstop.

`internal/cli/hostile_test.go` runs every command over a tree of hostile names
and contents, the failure paths that name a file included, and asserts that
nothing printed holds a character other than a printable one, a newline, or a
tab. It also asserts that every hostile name appears only in its quoted form,
which is how a print site that forgot `displayName` is found: the writer would
have kept the output safe, so only that check notices the ambiguity.

## 10. Error Handling

- Missing config file: clear message; suggest running `incrmit discover`.
- No version found in a target: report the file and continue or fail fast
  (configurable; fail fast by default for bump).
- Multiple versions found in a single file: report ambiguity and skip unless a
  format-specific rule resolves it.
- Filesystem/permission errors: surface the underlying error and exit non-zero.
- A target that is not an ordinary file (a named pipe, device, or socket, whether
  named directly or reached through a symlink): report
  `reading <path>: not a regular file` and exit `1`. `files.ReadTarget` checks the
  type before opening, because opening a pipe blocks until a writer appears and
  would otherwise leave incrmit hanging with no output at all.
- Undo with nothing to revert (no journal or an emptied one): print a friendly
  message and exit `0` — it is not an error.
- Undo conflict (a file edited since the bump — or bumped past by another run —
  no longer holds the recorded `new` token): report which file diverged, write
  nothing, and exit `1` (generic error).
- A project already held by another `incrmit` run: report it, name `--wait`,
  write nothing, and exit `1` (generic error). See
  [section 8.6](#86-concurrency-one-writer-per-project) for why this fails fast
  rather than blocking.
- Locking unavailable rather than contended (a filesystem without lock support,
  an unwritable directory): warn on stderr and carry on unlocked. This is not an
  error and does not change the exit code.

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
│   ├── lock/               # per-project advisory lock (one writer at a time)
│   ├── files/              # read/write helpers
│   ├── buildinfo/          # tool version (stamped via -ldflags)
│   └── testutil/           # helpers shared by tests in several packages
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
- Concurrency tests that start several `cli.Main` calls against one project at
  once (`internal/cli/concurrent_test.go`), asserting the tree, the config, and
  the journal agree afterwards. The `--wait` case is the one a serialized
  implementation cannot pass by accident: eight queued bumps must land eight
  patch increments and eight journal entries. Alongside them: a contended run
  writes nothing, a failed bump still releases the lock, the read-only commands
  run while it is held, and an unavailable lock degrades to a warning.
- Golden-file tests for the `preview` table, in sync and drifting
  (`internal/cli/testdata/preview_*.golden`, regenerated with
  `go test ./internal/cli/ -update`), alongside a test that asserts a preview
  leaves every file in the tree byte-identical.
- Pathological file shapes (`shapes_test.go` in `files`, `discovery`, and
  `cli`): line endings, a BOM, UTF-16 and Latin-1, tokens at the edges of the
  data, empty files, minified single-line files, thousands of occurrences, file
  metadata (read-only, setuid, hard links), and awkward names. Expected output
  is built from the same template as the input and compared byte for byte. The
  largest cases shrink under `go test -short`.
- Golden fixtures for the readable shapes — CRLF (`Directory.Build.props`), a
  BOM (`appsettings.json`), no trailing newline (`setup.cfg`), and a minified
  line (`package.min.json`) — sit beside the format fixtures in
  `internal/files/testdata`. `.gitattributes` marks the fixtures `-text`,
  because the repository's `* text=auto` would otherwise store the CRLF fixture
  as LF, and `TestShapeFixturesKeepTheirShape` fails if a fixture loses the
  shape it is there to test, rather than letting its golden test pass while
  proving nothing.
- Terminal output (`internal/cli/hostile_test.go`, `display_test.go`): every
  command over a tree of hostile names and contents prints nothing a terminal
  would act on, and names each file only in its quoted form (§9.5).
- Fuzz targets over the parsers, the rewriter, discovery's scan, and the
  terminal escaping (§12.1).

### 12.1 Fuzzing

Every test above feeds input someone thought to write down. `incrmit` rewrites
other people's files in place, so the failure that matters most is a scanner or
rewriter bug that eats bytes around the version token — and a handful of
fixtures cannot cover the shapes a real tree holds. The fuzz targets state the
invariants as properties and let the engine look for input that breaks them.

What each target proves:

- `version.FuzzParse` — `Parse` never panics, and anything it accepts
  round-trips through `String()` back to the identical token. The round trip is
  not cosmetic: `files.matchAt` locates an occurrence by comparing the bytes in
  the file against `pin.String()`, so a token `Parse` accepts but `String()`
  re-spells differently can never be matched, and the bump reports success
  having written nothing. It also asserts that `FindTokens` finds any accepted
  token *whole*, which is what keeps the config's reading of a version and the
  file's reading of it the same string.
- `version.FuzzFindTokens` — the returned ranges are in bounds, strictly
  ordered, and non-overlapping, which is what lets the rewriter walk them in one
  pass writing `data[prev:start]` without panicking or emitting a byte twice.
  Not every range parses, by design: `FindTokens` reports candidates (an IPv4
  address, a two-component number) for `Parse` to reject. The property is that a
  range `Parse` *accepts* spans exactly the bytes `String()` produces.
- `files.FuzzSetKnownVersions` — the promise the whole tool rests on: for
  arbitrary bytes and a set of pins, every byte outside the replaced ranges is
  the input's, in the input's order. The check searches for an alignment of
  input and output using only two moves (copy an identical byte, or consume a
  pin's old token against its new one); the search is exhaustive, so a failure
  means some byte outside a token really did change. It deliberately does not
  reuse `assertOnlyVersionChanged`, whose `strings.ReplaceAll` round trip
  repairs a stray extra replacement as readily as the intended one when the same
  token appears twice. Alongside it: the counts match the replacements actually
  made (tied to the output through its length, and bounded by the occurrences of
  each token in the input), and a rewrite never leaves behind a version the file
  did not have and the pins did not ask for — with one weld allowed, because the
  rewriter cannot prevent it (below).
- `cli.FuzzParseSize` / `cli.FuzzFormatSize` — `--max-file-size` comes off the
  command line, so anything a shell can pass must come back as an error rather
  than a panic or a silently wrong limit; and every size the tool *prints*
  parses back to the same number, in both directions.
- `discovery.FuzzScan` — every occurrence's line number matches a reference
  count of the line breaks before it (`\n`, `\r\n`, and a lone `\r` once each),
  and its context holds the token, no line terminator, and no more than the
  context window. The scan finds lines with a forward-only cursor to stay linear
  on minified files, and a cursor has state for `\r\n` to be counted twice in.
- `cli.FuzzDisplayName` / `cli.FuzzTerminalText` — a rendered name is
  printable and reads back, with `strconv.Unquote` when quoted, to the one name
  it came from, which is what makes the rendering injective; and escaped text
  is valid UTF-8 holding only printable characters, newlines, and tabs, with
  text that was already safe passed through unchanged.
- `config.FuzzLoad` — the config is trusted input, but a truncated or
  hand-mangled file is not a hostile one: arbitrary bytes must produce a
  `config: ...` error, never a panic, and never a config that loaded into a
  shape the commands do not expect. What loads must also marshal and reload
  identically, because a bump rewrites the config it just read.

Seeds live in `f.Add` calls next to the invariant they exercise, where a
reviewer reads them with the property rather than as an opaque file. The
`testdata/fuzz/<Target>/` directories hold the regression corpus: every input a
fuzzing run has found, kept so a fixed crash stays fixed. Both are seed corpora
to the go command, so `go test ./...` replays all of them as ordinary subtests —
the seeds alone catch the known cases, with no fuzzing engine involved.

`make fuzz` runs every target for `FUZZTIME` (30s each by default) and is
deliberately not part of `make check`: fuzzing is non-deterministic, so a green
run proves only that nothing was found in the time given. CI runs the same
bounded pass on every push, in a job of its own, so a random failure never
decides whether Build & Test is green. An input that fails is written to the
package's `testdata/fuzz/<Target>/`; commit it under a name that says what it
proves, and it becomes a regression case from then on.

One limitation is documented rather than fixed. A new token can run on into the
bytes that followed the token it replaced, when the scanner left bytes behind
that its grammar could not absorb but a shorter token can: `0.0.0+H+0` is read
as the version `0.0.0+H` with `+0` left over, so a pin rewriting it to `0.0.0`
produces `0.0.0+0`. Neither string is a valid version to begin with — semver
allows one build section, not two — and the result is a file reporting a version
the config does not pin, which the next command refuses with "expected version
not found" rather than acting on. Refusing the rewrite instead would trade a
bumped-but-odd file for an unbumped one and an error, on input nobody writes.
`FuzzSetKnownVersions` therefore allows a token that begins exactly where a new
token was written and runs past its end, and nothing else; the regression corpus
keeps the case under
`internal/files/testdata/fuzz/FuzzSetKnownVersions/weld_onto_second_build_section`.

The first pass found three real defects, all of the same shape — a token the
tool could write or accept but never find again:

- `Parse` accepted leading zeros in the numeric core, reading `1.02.3` as
  `1.2.3`. `SetVersion` then looked for the literal text `1.2.3`, found nothing,
  and returned the file unchanged with no error: the bump printed
  `1.2.3 -> 1.2.4` and wrote nothing. Leading zeros are now rejected, as semver
  requires, so such a file reports "no semantic version found" instead.
- `Parse` accepted a token ending in `-` (`1.2.3+0-`), which semver permits but
  the scanner's trailing `\b` cuts short, so `FindTokens` reads it back out of a
  file as `1.2.3+0`. A token must now end with a letter or digit.
- `formatSize` printed a size with no whole unit as `1234 bytes`, which
  `parseSize` refused — so the limit the flag showed as its default could not be
  pasted back. `parseSize` now accepts the spelling `formatSize` prints.

## 13. Build and Release

- Build: `make build` (stamps the version via `-ldflags`) or
  `go build -o incrmit .`. All Makefile builds pass `-trimpath`, so a binary
  carries no absolute path from the machine that produced it and the same source
  yields the same bytes anywhere.
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
- macOS release: `make release-macos VERSION=X.Y.Z` runs `pkg` and then
  `pkg-checksums`, writing `dist/checksums-macos.txt`. The `.pkg` installers need
  their own checksum file because they are built on a macOS runner while
  `checksums.txt` is produced on the Linux runner, and two machines cannot append
  to one file. Together the two files cover every published artifact.

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

1. Runs the same validation gates as CI (fmt, vet, test, coverage, lint,
   `govulncheck`).
2. Derives the version from `${GITHUB_REF_NAME}` (strips the `v` prefix) and
   passes it to `make release VERSION=…`.
3. Extracts the matching `CHANGELOG.md` section with `scripts/changelog-notes.sh`.
4. Creates a GitHub Release via `softprops/action-gh-release`, uploading the
   archives, `.deb` and `.rpm` packages, and `checksums.txt`.

A separate `release-macos` job runs on a `macos-latest` runner, builds the
`.pkg` installers and their checksum file with `make release-macos`, and uploads
both to the same release (the macOS toolchain needed by `pkgbuild` is unavailable
on the Linux runner).

Supply-chain measures in both workflows:

- **Actions are pinned to a commit SHA**, with the corresponding release in a
  trailing comment. A tag can be moved to point at different code, so pinning is
  what makes a run reproducible and keeps a hijacked release out of a job that
  publishes artifacts. Bump the SHA and the comment together.
- **Permissions default to `contents: read`** at the workflow level, and only the
  two publishing jobs raise themselves to `contents: write`. The validation job
  never holds a token that can write to the repository.
- The built-in `GITHUB_TOKEN` is the only credential; there are no extra secrets,
  and nothing in the workflows echoes one.
- **`govulncheck` gates both workflows** (its own job in CI, a step in the release
  validation), so a known vulnerability reachable from this module's code fails
  the build rather than shipping.
- **Go tools installed in CI** (`govulncheck`, `nfpm`) are pinned to a module
  version, which is a stronger guarantee than the SHA pinning above rather than
  a weaker one: the module proxy serves a given version's content immutably
  and the go command verifies it against the checksum database, so a moved
  upstream tag fails the build instead of quietly running new code. A GitHub
  Action tag has no such protection, which is why those need explicit SHAs.
  Because that guarantee rests on `GOPROXY` and `GOSUMDB`, both are set
  explicitly on the install steps rather than inherited from the runner's
  defaults.

  Recording those tool hashes in this repo's own `go.sum` (via a Go `tool`
  directive) was considered and rejected: it would pull the tools' dependency
  trees into `go.mod` as requirements, and keeping the module at two
  dependencies (`BurntSushi/toml` and `golang.org/x/sys`) with no transitive
  ones is worth more than re-pinning something the checksum database already
  pins.

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
