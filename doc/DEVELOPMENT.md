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
  text files, and reading JSON, TOML, and YAML files by their format so that
  only a whole string value counts as a version there.
- Incrementing the major, minor, or patch component.
- Promoting a prerelease to its release (`--release`) and starting or advancing
  one (`--pre`).
- Reading target files from a TOML configuration file, where an entry can say
  which occurrence in its file is the project's version (`key`), so a
  dependency pinned at the same version is left alone.
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
- Be dependency-light and produce a single static binary. The module requires
  three others, `github.com/BurntSushi/toml` (the config, the journal, and
  TOML targets), `golang.org/x/sys` (the Windows lock, §8.6), and
  `go.yaml.in/yaml/v3` (YAML targets, §9.7), and none of them requires
  anything, so there are no transitive dependencies. A fourth needs a format or
  platform the standard library cannot serve and a record of what was weighed,
  as §9.6 and §9.7 keep for the two decoders.
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
- `files` — reads and writes target files, replacing only the version token,
  and locates the versions in a file with a parser chosen for its format (text,
  JSON, TOML, or YAML). The parser is the seam every command takes its ranges
  from (see [section 9.6](#96-parsers-reading-json-and-toml-by-format) and
  [section 9.7](#97-parsers-reading-yaml-by-format)), and an
  entry's key is looked up in what it found (`Located.Claim`, see
  [section 9.3](#93-what-the-config-knows-that-the-grammar-cannot)).
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
parser = "text"       # optional; "json", "toml", "yaml", or "text" (default: by extension)
key = "VERSION ?= "   # optional; which occurrence is the project's version
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
several *differing* versions, or one version in several places, as long as each
entry pins a distinct, non-empty version or names a distinct `key`.
Distinctness is judged on the whole token, so `1.2.3` and `1.2.3` +
`prerelease = "rc.1"` are two entries, not a duplicate. Validation rejects
exact `(path, token, key)` duplicates and any repeated path where an entry has
neither a version nor a key (which would be ambiguous). Two entries whose keys
name one occurrence, or a keyed entry and one without a key for the same
version, take the file to detect, so the bump refuses them in phase 1 (§8.1).
In a text file, identical repeats of the same version collapse to a single
entry.

A `path` is relative to the directory holding the config
(`config.TargetPath`), and so is every path `discover` writes: since Milestone
37 it records each file relative to `--output`'s directory rather than to
`--path`, so the two flags may name different directories (see
[section 8.2](#82-discover)).

The optional top-level `ignore` list holds folder/file patterns that `discover`
skips (see [section 8.3](#83-ignore-matching)), relative to the config's
directory like the paths. Empty patterns are rejected; each pattern is trimmed
and its separators normalized to `/` on load.

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
    Build      string  `toml:"build,omitempty"`
    Parser     string  `toml:"parser,omitempty"`
    Key        *string `toml:"key,omitempty"`
}
```

`parser` (Milestone 39) chooses how the entry's file is read
(`FileEntry.FileParser`): `"json"`, `"toml"`, `"yaml"` (Milestone 41), or
`"text"`, spelled exactly. Omitted, the extension decides (`files.ParserFor`),
which is what every config written before the key existed means, so they all
still load. It is there for a `.json` file with comments (`tsconfig.json`, VS
Code's `settings.json`) to be read as text, for TOML or YAML under another name
(`Pipfile`) to be read by its format, and for a Kubernetes manifest's image tag
to be tracked as text (§9.7).
An unknown value is a config error naming the entry, and entries that list one
path must agree on its parser, however each spells it, since the file is read
once and rewritten in one pass. `discover` writes the key only when it differs
from the extension's choice, which is a `.json`, `.toml`, `.yaml`, `.yml`, or
`.cff` file it scanned as text, and a bump's config rewrite carries each
entry's value through
(`config.EntryFor` builds an entry without it, so `bumpedConfig` sets it).
Declaring the field is all `LoadIgnore`'s known-key check needed.

`key` (Milestone 40) says which occurrence in the file is the project's
version, read by the entry's parser (`FileEntry.FileKey`, `files.ParseKey`):
for JSON, TOML, and YAML a path from the document root, written the way
`files.KeyPath.String` writes one (`version`, `package.version`,
`bin[0].version`, `'"."'`); for text the literal bytes right before the
version (`"VERSION ?= "`). An entry with a key rewrites exactly the occurrences
it names; one without rewrites every candidate holding its version, which is
what every config written before the key means. It is a `*string`, so
`key = ""` is told apart from no key and refused, since it names nothing.
`Validate` refuses a key the entry's parser cannot read with exit `1`: an empty
one, a text key holding a line break, and one that is not a path for JSON,
TOML, or YAML. An entry with a key and no `version` starts from the version
the key names. `discover` writes the key for JSON, TOML, and YAML (§8.2), the
bump's config
rewrite carries it through as written (`bumpedConfig`, the same
`config.EntryFor` problem as `parser`), and `undo`'s rewrite edits the entries
in place, so it keeps them. See [section 9.3](#93-what-the-config-knows-that-the-grammar-cannot)
for the design.

`config.LoadIgnore` reads only the `ignore` list, without validating the listed
targets, so `discover` can honor a stale config's patterns. It is also where
`discover` decides whether it may replace its `--output` at all. It may when
nothing is there or the file is empty (only whitespace), which yield no
patterns, and when the file is an incrmit config: TOML that sets at least one
key and no key the `Config` and `FileEntry` structs do not declare, checked with
the decoder's `MetaData.Undecoded`. The struct fields are the list of known
keys, so a new table (Milestone 43's `[git]`) extends it by being declared.
Anything else is a `*config.NotConfigError` whose `Reason` says why (`line 3 is
not TOML`, `it sets "project", which incrmit does not use`, `it sets nothing,
but is not empty`, `it is not a regular file`), and `discover` refuses. Before
Milestone 36 it was lenient instead, reading any file that did not parse as "no
patterns", so `discover -o NOTES.md` replaced the Markdown file and exited `0`.

### 6.2 Bump history / state file (`internal/history`)

Every non-`--dry-run` bump in config mode pushes an entry onto a **state file**
so the bump can be reverted by `incrmit undo`. The entry is written before any
other file, marked `pending`, and marked `complete` once every write has
landed (see [section 8.7](#87-crash-safety-all-or-nothing-across-files)).

- **Location:** the file is named `.incrmit.state.toml` (`config.StateFileName`)
  and lives in the **same directory as the config** (`incrmit.toml`), so the two
  are always found together. `history.ResolvePath(configPath)` derives it from
  the resolved config path; `undo` uses the same rule (default or `--config`).
- **Lifecycle / retention:** it is a **stack** of entries, oldest first. Each
  bump pushes one entry; each `undo` pops the most recent. Only the last
  `history.MaxEntries` (20) bumps are retained so the file cannot grow without
  bound — at minimum the last bump is always undoable, and successive undos walk
  back through history.
- **Size:** the journal is read with a 16 MiB cap (`files.MaxOwnFileBytes`, see
  [section 9.1](#91-scan-boundaries)), and `history.Save` keeps it under the same
  cap so incrmit never writes a journal it would refuse to read back. A change
  costs about 200 bytes, so twenty full entries reach the cap at around four
  thousand targets; past that, `Save` drops the oldest entries until the
  encoding fits, and the project keeps fewer than twenty bumps instead of
  failing its next one. A newest entry over the cap on its own is an error.
- **Committed vs. ignored:** it is **local working state**, not meant to be
  committed. `undo` restores files in the working copy, so the journal belongs
  beside them; the file header recommends adding it to `.gitignore`. (In this
  repo `*.toml` is already git-ignored, which covers it.) Discovery never treats
  it (or `incrmit.toml`) as a target.
- **Not trusted like the config:** a repository can still commit one, so
  `undo` treats it as a list of changes to check, not as locations to write.
  It reverts an entry only when every change's `path` is listed in the config
  at the change's `new` version (see
  [section 8.4](#84-undo)), which limits the journal to what a bump of that
  config could already reach.
- **`--file` mode records nothing:** a single-file bump has no config-anchored
  location to keep or later find the state file, so no history is written and
  `undo` is a config-mode operation only.
- **Parser:** each change records the parser the bump read its file with
  (Milestone 39), and `undo` reads the file with that parser rather than the
  one the config names now, so it reverts the occurrences the bump wrote. An
  entry written before the field existed was written by the text scanner, so a
  missing `parser` is read as text. A parser this version does not know, or
  two for one file, refuses the undo instead of guessing; a bump only pushes
  onto the journal, so it does not check.
- **Key:** each change records its entry's `key` as written (Milestone 40), or
  nothing for an entry without one; an empty key is never valid, so the two do
  not collide. `undo` reverts exactly the occurrences that key names, read with
  the recorded parser, even when the config's key has been edited since. A key
  the recorded parser cannot read refuses the undo. A journal is not trusted
  input, but a key only narrows what a change reverts, among files and
  versions the config already lists.
- **Status:** `pending` marks an entry whose changes a run was writing when it
  stopped (or is writing now), so each file may hold either side of its
  change; `complete` marks a bump that wrote them all. An undo marks the entry
  it is reverting `pending` before its own first write. Journals written
  before Milestone 38 have no `status`, and they recorded only bumps that had
  finished, so `Load` reads a missing status as `complete` and the next save
  writes it out. A status this version does not know fails the load instead of
  being read as either one. An older `incrmit` ignores the key and reads a
  `pending` entry as `complete`, which fails safe: its undo requires every file
  to hold the version the bump wrote and the config to list it, so it refuses
  a half-applied entry rather than writing over a file the bump never reached.

The schema (TOML), encoded with the same `github.com/BurntSushi/toml` library:

```go
type Change struct {
    Path string `toml:"path"` // the path exactly as the config lists it
    Old  string `toml:"old"`  // version token before the bump
    New  string `toml:"new"`  // version token after the bump
    Parser string `toml:"parser,omitempty"` // "json", "toml", "yaml", or "text"; "" is text
    Key  string `toml:"key,omitempty"`  // the entry's key as written; "" for none
}

type Entry struct {
    Timestamp time.Time `toml:"timestamp"`
    Status    string    `toml:"status,omitempty"` // "pending" or "complete"
    Changes   []Change  `toml:"changes"`
}

type History struct {
    Entries []Entry `toml:"entries"`
}
```

Paths are stored **as the config lists them**, never resolved. `undo` resolves
each against the directory of the config it was given (`-c`, or `incrmit.toml`),
through the same `config.TargetPath` a bump uses, and rewrites that config. The
journal therefore belongs to the project rather than to the directory the bump
ran in: a copied or moved project undoes its own files, and `undo` still works
from any working directory because it finds the state file through the config.
Before Milestone 34 each entry also stored the config's absolute path
(`config`) and each change the file's (`fs`), and `undo` acted on those, so undo
in a copy reverted the original. Both keys are ignored on read, and the next
save drops them. The file is written atomically via `files.WriteAtomic`, the
same path used for target files.

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
| `--output`        | `-o`  | Path to write the generated config file (§8.2)    | `incrmit.toml` |
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
A projection whose bump fails with `version.OverflowError` is printed as `n/a`,
with a footnote saying why, rather than as the number it would wrap to; the
other columns are still real, so the preview still exits `0`.

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
   - If `--file` is set, use that single file, with `--key` as its key.
     `--key` without `--file`, or empty, is a usage error (exit `2`), and so is
     one the file's parser cannot read; it is read once the parser is known,
     so a `.json` file that falls back to text reads it as text.
   - Otherwise load the config from `--config`.
4. In config mode, load the journal beside the config (`history.Load`), so a
   state file that cannot be read fails the bump before anything is written. It
   used to be read last, after the targets and the config already held the new
   version, which left a bump with no entry for `undo` to find. A `--dry-run`
   reads it too, so it predicts that failure.
5. Group config entries by file (a file listed once per distinct version is a
   single group) and read each file once, refusing one larger than
   `--max-file-size` when a cap is set (see
   [section 9.1](#91-scan-boundaries)), and locate its candidates with the
   entry's parser (see
   [section 9.6](#96-parsers-reading-json-and-toml-by-format)). An entry whose
   parser cannot read its file fails here with exit `3`, never falling back to
   text; a `--file` target, which has no entry, falls back with a note. For
   every entry, determine the old version (from the config `version`, the one
   version its key names, or the one version among the candidates when it
   records neither). An entry with a key claims the occurrences it names that
   hold that version (`files.Located.Claim`); a key that is absent, names a
   path the file has twice, holds anything but a candidate, holds another
   version, or (for text) comes before no candidate fails here with exit `3`,
   naming the file and the key (`*files.KeyError`). Then two entries that would
   rewrite one occurrence, where at least one has a key (`files.Overlap`), fail
   with exit `1` as a config error naming both
   (`config: files[0] (key version) and files[1] (no key) both claim the 1.2.3
   on line 2 of package.json; ...`). With claims settled, apply the bump to
   each entry to get the new version. A bump that fails for
   one entry (`--release` on a version with no prerelease, or a number already
   at `math.MaxInt`; see [section 9.2](#92-prerelease-and-build-metadata))
   fails the whole command here, before anything is written. If the newest
   journal entry is `pending`, a failure here also says to run `incrmit undo`:
   a run that stopped partway is the likeliest reason the config and the files
   disagree.
6. Compute the bytes each file will hold, replacing every candidate holding one
   of its pinned versions, and for an entry with a key only the candidates it
   claimed (a `files.Replacement` carries them, `Keyed` and `At`), in a single
   pass (`files.Rewrite`, which applies `files.SetKnownVersions` and, for JSON,
   TOML, and YAML, decodes the result to check that exactly the planned values
   changed, to the planned versions). A single pass over the original bytes
   keeps overlapping bumps from cascading (e.g. `1.2.3 -> 1.2.4`
   alongside `1.2.4 -> 1.2.5`) and avoids one entry's write clobbering
   another's when two versions live in the same file. A pinned version the
   file no longer holds fails the bump here, `--dry-run` included. This check
   used to run as each file was written, so a config out of sync with its
   third file left the first two bumped. In config mode, render the config too:
   each entry's `version` set to the new value (one entry per entry read,
   each keeping its `parser` and `key`), keeping the config in sync for the
   next run. It is regenerated
   through `config.Marshal`, so the `[[files]]` entries and `ignore` list
   survive but user-authored comments and formatting do not. The output is
   deterministic: the same config bumped twice produces byte-identical layout.
7. If `--dry-run`, print `old -> new` for each entry, after its path and key
   (`package.json version: 1.2.3 -> 1.2.4`), and exit (no writes).
8. In config mode, push a history entry (each file's path as the config lists
   it, its `old`/`new` tokens, its parser, and its key, plus a timestamp)
   marked `pending` onto the journal read in step 4, and save it beside the
   config (see [section 6.2](#62-bump-history--state-file-internalhistory)). If
   that save fails, nothing has been written.
9. Write each target, then the config (`commitWrites`). If a write fails, put
   back the files already written and the journal, and report both the failure
   and the rollback (see
   [section 8.7](#87-crash-safety-all-or-nothing-across-files)).
10. Mark the entry `complete` and save the journal. If that save fails, every
    file already holds the new version and a pending entry undoes like a
    complete one, so the bump stands: warn and carry on.
11. Report results (files bumped, and each `old -> new`).

### 8.2 Discover

1. Unless `--dry-run`, take the project lock for the directory holding
   `--output` and hold it until the command returns
   (see [section 8.6](#86-concurrency-one-writer-per-project)). `discover`
   rewrites the config it reads its `ignore` list from, so that read and the
   write are one read-modify-write like a bump's.
2. Read the `ignore` list from any config already at `--output`
   (`config.LoadIgnore`); an absent or empty file yields no patterns. Anything
   at `--output` that is not an incrmit config (see
   [section 6.1](#61-config-schema-toml)) is refused here with exit `1`, before
   the scan and with nothing written: `<output> exists and is not an incrmit
   config (<reason>); choose another --output or remove the file`. That covers
   a file that is not TOML, TOML with a key incrmit does not use, a file of
   comments alone, and, through the same `files.ReadOwnFile` check `Load`
   makes, a named pipe, a device, or a file over 16 MiB. `--dry-run` takes the
   same path, so it predicts the refusal. There is no `--force`; removing the
   file is the explicit way to ask for it to be replaced.
3. Walk the tree from `--path`, skipping the built-in ignored directories
   (e.g. `.git`, `node_modules`, `vendor`, build outputs), incrmit's own files
   (`incrmit.toml` by name plus the resolved `--output` path, the state file,
   the lock file, and any `.incrmit-*.tmp` write in flight), symlinks of any
   kind, and anything matched by the `ignore` patterns (see
   [section 8.3](#83-ignore-matching)). Each file is recorded relative to the
   directory holding `--output`, which is where every command will resolve it
   (`discovery.DiscoverWithLimit` takes that directory as `configDir`):
   scanning `sub` for `./incrmit.toml` records `sub/VERSION`, and scanning
   `src` for `release/incrmit.toml` records `../src/VERSION`. Both directories
   are made absolute with symbolic links resolved (`resolveDir`) before
   `filepath.Rel` relates them, so two spellings of one place — `/tmp` and
   `/private/tmp` on macOS, where `os.Getwd` may report either — do not
   produce a path that climbs to `/` and back down. Only the longest existing
   prefix is resolved, since a `--dry-run` may name an `--output` directory
   that does not exist yet. On Windows, a file on another volume than the
   config has no relative form and is recorded by its absolute path. The walk
   itself keeps `--path` as given, so its errors name the path the way the
   user wrote it. Before Milestone 37 every path was relative to `--path`,
   which agreed with the config's directory only when the two were the same:
   `discover --path sub` wrote `path = "VERSION"` for `sub/VERSION`, and a
   bump rewrote `./VERSION` instead if it existed.
   `excludeOutput` then drops the result that is `--output` itself, which is
   the one whose path `config.TargetPath` resolves to `--output`, exactly as a
   bump would.
4. For each candidate file, extract *every* semantic version occurrence its
   parser finds (the one its name selects, see
   [section 9.6](#96-parsers-reading-json-and-toml-by-format)), recording each
   one's line number, the trimmed text of its line, and its key path
   (`discovery.Occurrence`), and for YAML the aliases that read it
   (`Occurrence.Aliases`). A `.json`, `.toml`, `.yaml`, `.yml`, or `.cff` file
   its parser cannot read, or a YAML file of more than one document, is
   scanned as text, and the `Result` keeps the parser's error as `Fallback`;
   values skipped because they hold a version only once decoded are kept as
   `Skipped`, for the dry run. A file with at least one
   occurrence becomes a `Result`. Only regular files no larger than the scan
   cap (`--max-file-size`, default `discovery.DefaultMaxScanBytes` = 32 MiB)
   are read; see [section 9.1](#91-scan-boundaries).
5. Turn results into config entries (`discovery.Result.Entries`). A file read
   as text gets one `[[files]]` entry per distinct version (first-seen order),
   so identical repeats collapse and differing versions each get an entry
   sharing the path, with no key: the text in front of a version is often
   prose, and rewording the sentence would break the entry. A JSON, TOML, or
   YAML file gets one entry per candidate, each with its key (Milestone 40), so
   each
   rewrites exactly one value, can be deleted on its own, and says in the
   config what it is. Two kinds of candidate get none, marked on the
   `Occurrence` as `Skip`. In a manifest discover knows (`projectKeys`, by
   exact file name), every candidate at none of its project keys is
   `NotProject`: `package.json`, `composer.json`, `deno.json`, and `jsr.json`
   hold the project's version at `version`, `Cargo.toml` at `package.version`
   or `workspace.package.version`, `pyproject.toml` at `project.version` or
   `tool.poetry.version`, `pubspec.yaml`, `snapcraft.yaml`, `galaxy.yml`, and
   `CITATION.cff` at `version`, and a Helm `Chart.yaml` at both `version` (the
   chart's) and `appVersion` (the application's), so a dependency pin gets no
   entry. A chart gets an entry for each, since both belong to the project and
   many charts keep them equal; a user who versions them separately deletes
   one. A known manifest with no version at those keys, such as a private
   monorepo root, gets no entry at all rather than one per dependency. And
   anywhere, a candidate whose key names more than one value (a JSON or YAML
   key written twice) is `RepeatedKey`, since a key naming it would fail the
   bump. An entry records
   `parser` only when the file was read with a parser other than its
   extension's, which is a fallback to text, so a bump reads the file the way
   discover did. The `ignore` list is written back verbatim so regeneration
   never drops it (a bump preserves it the same way when it rewrites the
   config). A description of the `ignore` option is always written above it;
   when there are no patterns yet, a commented-out example (`# ignore = [...]`)
   is emitted in their place so the feature is discoverable from a freshly
   written config. The comment block (`config.IgnoreComment`) is shared by
   both the discover generation and the bump-time rewrite (`config.Marshal`)
   so both files carry identical guidance.
6. If `--dry-run`, print each occurrence with its line number, its key (for
   JSON, TOML, and YAML, `L3 package.version: ...`), and context, then exit. A
   YAML occurrence that aliases read ends with them
   (`L1 base: base: &v 1.2.3 (aliased at pinned)`), since a bump changes them
   too. A file that fell back is headed by the reason (`tsconfig.json: not
   JSON (line 2: ...), scanned as text`, `deploy.yaml: 2 YAML documents,
   scanned as text`), and a skipped value gets a line saying why: an
   escape, a dependency in a known manifest (`skipped, not the project's own
   version; to track it, add an entry with key = "dependencies.left-pad"`), or
   a key written twice. A known manifest that gets no entry is headed by why
   (`package.json: no entry, since key version holds no version`). A file
   whose only versions were skipped is listed by the dry run but gives the
   config no entry. When `--path` and `--output`'s directory differ
   (`discovery.SameDir`), a `(paths relative to <output>)` line comes first,
   since the paths listed are then not relative to the directory in the
   heading; the applied ignore rules follow on an `(ignoring: …)` line.
7. Otherwise write the generated config to `--output`, and print the same
   config-relative paths in the summary, one line per entry with its key
   (`Cargo.toml package.version: 0.4.0`). Run over a config written before
   keys, this is how an existing project gets them.

### 8.3 Ignore matching

The `ignore` patterns are compiled into an `ignoreMatcher` (`discovery/ignore.go`)
and applied during the walk against paths **relative to the config's
directory** — the same paths the config lists, so a root outside that
directory gives paths that start with `../` — always in slash form,
**case-sensitively**:

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
configured pattern matches. The scan root itself is never pruned, by either:
`--path build` scans `build`, and the patterns apply to what is inside it.

Patterns are relative to the config, not to `--path`, because they live in the
config: matched against the scan root, as they were before Milestone 37, one
pattern named a different directory under each `--path` (`gen/**` was `gen`
for a scan of `.` and `sub/gen` for a scan of `sub`). Matched against the
config's paths, a config means the same thing wherever `discover` runs from,
and a pattern can be checked by reading the `path` values beside it.
`config.IgnoreComment`, written into every config, says so. A bare pattern
matches a base name at any depth, so it is unaffected; only a pattern with a
slash in it changes meaning, and only for a `--path` other than the config's
directory.

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
3. Load that config up front so a config problem aborts before any write
   (fail-fast, mirroring bump). It is the config undo was given, not a location
   the journal records, so the lock, the journal, the config, and every target
   all derive from one path.
4. Check that every change in the entry names a `[[files]]` entry of that config
   by its `path` as written, at the change's `new` version. Otherwise report the
   first change that does not and abort without writing anything, `--dry-run`
   included. The config is trusted input and the journal is not, so this keeps
   what a journal can reach to what the config could already bump: an absolute
   `path`, or one with `../`, is allowed exactly when the config lists it. It
   also catches a config edited since the bump, which is refused rather than
   partly undone. A `pending` entry may instead find every change listed at
   its `old` version, because the config is the bump's last write; a config
   at neither, or split between the two, is refused.
5. Resolve each recorded `path` against the config's directory
   (`config.TargetPath`, as `resolveTargets` does for a bump). Read every
   recorded file once, with the parser the journal records for it (text for an
   entry written before Milestone 39), group its changes, and build the reverse
   (`new -> old`) replacement in a single pass (`files.Rewrite`), so
   overlapping reverts do not cascade. A change with a key reverts only the
   occurrences that key, as the journal records it, claims at `new`; a key
   that claims none counts as `new` missing. If a file no longer contains the recorded
   `new` token, it was edited since the bump — or another run bumped past it,
   since a lock only serializes the runs that take it: report which file
   diverged and abort without writing anything, rather than putting an older
   version back over newer work. For a `pending` entry, a file that holds every
   `old` token and not every `new` one was never written (or was put back), so
   it is left alone; one that holds all of both, or all of neither, is refused
   (see [section 8.7](#87-crash-safety-all-or-nothing-across-files)).
6. Render the config with each `(path, new)` entry set back to its `old`
   version (the bump's self-update in reverse), unless a `pending` entry found
   it at the `old` versions already, in which case it is not rewritten. An
   entry is matched to its change by key too where one matches, since two keyed
   entries may reach one version from two (a patch bump takes `1.2.3` and
   `1.2.3-rc.1` both to `1.2.4`), and by path and version alone otherwise.
7. If `--dry-run`, print `new -> old` for each change, or `already at <old>`
   for a file left alone, and exit (no writes).
8. Mark the entry `pending` and save the journal, unless it already is.
9. Write each reverted file, then the config at the path undo was given
   (`commitWrites`). If a write fails, put back what this undo already wrote,
   mark the entry `complete` again, and report it, exactly as a bump does.
10. Pop the entry and save the journal so a repeated `undo` does not re-apply the
    same revert (and instead reverts the previous bump, if any). If that save
    fails, warn and carry on: every file is back at its old version, so the next
    undo finds the pending entry with nothing left to write and pops it.
11. Report the reverted files (each `new -> old`, or `already at <old>`).

### 8.5 Preview

1. Parse the preview flags. Resolve the targets exactly as a bump does
   (`resolveTargets`): the single `--file` target, or every entry in the config.
2. Read each distinct target once and determine the version it holds
   (`readGroups`) — the config-pinned token when there is one, otherwise the
   version the key names or the version found by scanning. Keys are claimed
   and checked here, so a key a bump would refuse fails the preview too. This
   is steps 2–3 of the bump flow, shared rather than reimplemented, so a
   preview cannot disagree with the bump it previews.
3. Project each version through all three component bumps and flatten the groups
   into rows, dropping an exact `(path, key, version)` repeat (`previewRows`).
   When any row has a key, a `KEY` column follows `PATH`, a text key quoted
   since its spaces count. A bump
   that would overflow projects to nothing, rendered `n/a` in step 5.
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
  cannot detect a second run. Contention is therefore the only error `Acquire`
  reports, which is what keeps "someone else holds it" distinguishable from
  "locking is not available".

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

### 8.7 Crash safety: all or nothing across files

Each write `incrmit` makes is atomic on its own (`files.WriteAtomic`), but a
bump writes several files, and no filesystem offers a transaction over several
renames. Before Milestone 38 a bump planned every file before writing any, then
wrote them one at a time and recorded its journal entry last. A failure on the
third of five targets left two bumped, and because the entry came after every
write, the run that most needed `undo` was the one it could not see: `undo`
answered "Nothing to undo", and the next bump failed because the config's
pinned versions were no longer in the bumped files. The pinned-version check
also ran during the writes, so a config out of sync with a later file did the
same.

**The guarantee.** A bump or an undo that exits `0` has written every file it
names. One that does not has done one of three things:

- put back every file it wrote, and the journal, and changed nothing. It
  reports the failure and `rolled back <files>; no file was changed`;
- failed to put some file back. It names each such file and the `incrmit undo`
  command that finishes the job (with `-c` when the run was given one,
  shell-quoted), and leaves its journal entry `pending` for that undo to find;
- died outright (a `kill -9`, a panic, a power cut; but see the limits below),
  leaving the `pending` entry with the same effect.

`incrmit undo` returns any of these to exactly the state before the run, and
the tests compare the whole project byte for byte to check that
(`internal/cli/crashsafe_test.go`).

**How.** Everything is computed before the first write: each target's new
bytes, with the pinned-version check (`planGroups`), and the config's. Writing
then does nothing that can fail for a reason of incrmit's own, only for one
outside it, such as permissions, a full disk, or a directory that went away.
The writes go in this order:

1. The journal entry, marked `pending`. If this write fails, nothing has been
   written.
2. Each target, in config order.
3. The config.
4. The entry, marked `complete`.

**Targets first, config last.** The config records which version each file
holds, so it goes last, as the commit record: a config at the new versions
means every target holds them. A run that stops between the two leaves the
targets new and the config old, and that state can only be recovered
backward. Running the bump again cannot carry it forward, since planning
finds the config's pinned versions missing from the files already bumped and
exits `3`, pointing at `undo` because the newest entry is `pending`; `undo`
then finishes it. A rerun that succeeded would bump the written files twice
and the rest once, so refusing it is the point. Writing the config first would
also refuse a rerun, but the config would then claim versions no file holds,
and a config at the new versions would no longer show that the targets had
been written. With the config last, its state tells `undo` whether to rewrite
it, and because it is one atomic write it is never partly reverted.

**Rollback.** When a write fails, the files this run already wrote get their
bytes from planning back, newest first, each through `files.WriteAtomic`. Every
one is attempted even after one fails, so as little as possible is left
changed. If all of them are put back, the journal goes back to the entries it
held before the run, or the state file is removed if this run created it. If
one is not, the `pending` entry stays and the run prints the advice above. The
config is never rolled back: it is the last write, so either it is the write
that failed, which changed nothing, or there was no failure.

**Undo of a `pending` entry.** Each file is checked against both sides of its
changes:

- every `new` token present, and not every `old` one: the run wrote it, so
  it is reverted as for a finished bump;
- every `old` token present, and not every `new` one: the run never wrote it,
  or put it back, so it is left alone and reported `already at <old>`;
- both, or neither: refused, with nothing written. Neither means the file was
  edited since. Both cannot come from the bump itself. Every bump raises a
  version's precedence, so a file's smallest `old` version is no change's
  `new` version, and its largest `new` version is no change's `old` version.
  The run's write replaced every `old` token, so a written file cannot still
  hold that smallest `old` version. A file the run never reached cannot hold
  that largest `new` version unless it already held it as text of its own,
  and reverting that would rewrite text the bump never owned.

The config is checked the same way, as a whole: every change listed at its
`new` version means the run wrote the config, so it is reverted; every change
at its `old` version means the run never reached it, and it is left exactly as
it is, comments and all; a mixture is refused. Trust is widened only that far:
a `pending` entry may name a file the config lists at the entry's `old`
version.

**Undo is made safe the same way.** It marks the entry `pending` before its
first write, writes the files and then the config, and pops the entry last. A
write that fails rolls back what the undo had written and marks the entry
`complete` again. An undo that stops partway leaves files on either side of the
entry, and running `undo` again finishes from there.

**The last journal write can fail too.** Marking a bump's entry `complete`,
or popping an undone entry, can fail after every file is already where the run
meant it to be. The run then exits `0` with a warning rather than reporting
work that happened as a failure, which would invite a script to retry it. The
entry stays `pending`, which is still correct. For a bump, it undoes exactly as
a complete entry would. For an undo, every file is at its `old` version, so
the next `undo` writes nothing and only pops the entry.

**Limits.**

- **Not atomic for readers.** Between the first write and the last, `preview`,
  a dry run, or a build can see a partly bumped tree. The lock only serializes
  writers ([section 8.6](#86-concurrency-one-writer-per-project)).
- **Process death, not power loss.** `files.WriteAtomic` syncs each temp file
  before renaming it, but no directory is synced after a rename. The operating
  system may therefore persist two renames out of order across a power cut,
  so the journal's rename might not survive while a target's does. A crash,
  `kill -9`, or failed write is covered; losing power mid-run is not
  guaranteed to be.
- **A rollback is a write like any other.** It puts back contents and
  permission bits under a new file, as the bump did, so a symbolic link the
  bump replaced stays a regular file and a hard link stays broken (see
  [Consequences of the atomic write](#consequences-of-the-atomic-write)).
- **A rollback restores the bytes the run read.** Something other than
  `incrmit` that edits an already-written target in the milliseconds before its
  rollback loses that edit. The lock keeps other `incrmit` runs out, but not
  editors.
- **Undo judges a file by its version tokens, not a checksum.** If a target is
  edited after a crash so that it holds exactly the `old` versions again, undo
  treats it as never written, which is the result undo would have produced
  anyway.
- **`--file` mode** writes a single file, which `files.WriteAtomic` already
  makes all or nothing, and keeps no journal.

## 9. Version Detection Strategy

- `version.FindTokens` is the single definition of where a version begins and
  ends. Both `discovery` (scanning) and `files` (rewriting) take their tokens
  from it, through the parsers of
  [section 9.6](#96-parsers-reading-json-and-toml-by-format) and
  [section 9.7](#97-parsers-reading-yaml-by-format), so the two cannot
  disagree about what counts as a token — they used to hold identical copies
  of the pattern, which is exactly the kind of duplication that drifts.
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
- **incrmit's own files get the same type check, and a fixed cap.** The config
  (`config.Load`, and `config.LoadIgnore` for discover's `--output`) and the
  journal (`history.Load`) are read through `files.ReadOwnFile`, which is
  `ReadTargetWithLimit` at `files.MaxOwnFileBytes` (16 MiB), so one function
  decides that any file is safe to open. They were read with `os.ReadFile`
  before Milestone 33: a named pipe at the config hung `preview` and
  `discover --dry-run`, the two commands meant for an unfamiliar tree, and one
  at the state file hung a bump after it had rewritten the targets. A
  repository cannot commit a FIFO, but it can commit `incrmit.toml ->
  /dev/zero`, which read without end. A link to a regular file is still
  followed, since a symlinked config is a legitimate setup. The cap is fixed
  rather than `--max-file-size`, which is about targets, and is sized for the
  parse: decoding TOML allocates about twenty times the file's size, so 16 MiB
  holds a planted file to a few hundred MiB while leaving room for a config of
  some two hundred thousand entries. The journal is kept under it on write
  (see [section 6.2](#62-bump-history--state-file-internalhistory)).
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
config's directory. The journal is not trusted input, and `undo` reverts only
the paths the config lists (see [section 8.4](#84-undo)).

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
- Every method that adds one returns `(Version, error)` and refuses with a
  `*version.OverflowError` (naming the part and the version) when the number it
  increments is already `math.MaxInt`, the largest component `Parse` reads.
  Wrapping wrote `1.2.9223372036854775807` forward as
  `1.2.-9223372036854775808`, which is not a version, so the next command found
  nothing in the file (Milestone 35). Clamping would write a version no greater
  than the one it replaced. Only the incremented number is checked, so a full
  patch still bumps its minor. `AdvancePrerelease` also refuses a counter too
  large for an int at all: it used to append `.1`, which ranks higher, but
  `PrereleaseID` then read the old counter as part of the series name, so the
  next `--pre rc` restarted at `rc.1`, below both. The methods return an error
  rather than having checked twins beside them so that no caller, `ApplyBump`
  included, can reach the wrapping arithmetic. `classify` maps the error to exit
  `3`, as for every other version incrmit cannot handle.

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

**Where the version is: `key`.** Since Milestone 40 the config can also say
*where* a file's version is, not only what it is. After Milestone 39 a JSON or
TOML candidate had to be a whole string value, but a dependency pin is one too,
and an entry naming only a path and a version rewrote every candidate holding
that version. On the reproduction files (`internal/files/testdata/repro`), the
`package.json` entry for `1.2.3` rewrote both `version` and
`dependencies.left-pad`, the `Cargo.toml` entry for `0.4.0` rewrote
`package.version` and `anyhow`'s inline `{ version = "0.4.0" }`, and the
entries `discover` wrote for `4.17.21` and `1.0.200` bumped two dependencies
outright, all with exit `0`. In a `Makefile` holding `VERSION ?= 1.2.3`, the
tool download at `1.2.3` moved with it. `TestReproductionPinsMoveWithoutAKey`
keeps that bump, which is still what an entry without a key does.

An entry's optional `key` says which occurrence is the project's, and an entry
with one rewrites exactly the occurrences it names. It is one field, read by
the entry's parser (`files.Key`, `files.ParseKey`), so there is one thing to
learn whatever the file:

- **JSON, TOML, and YAML: a path.** `files.ParsePath` reads what
  `KeyPath.String` writes: bare parts (ASCII letters, digits, `_`, `-`),
  double-quoted parts with TOML's basic-string escapes, and `[N]` for an array
  index, with no whitespace, so a path has one spelling apart from quoting a
  part that need not be. Every path a parser reports reads back to itself
  (§12.1). It matches the whole path, never a suffix, so `version` is the
  top-level key alone. `Located.Named` looks the path up among every value of
  the document (an index built on first use, one more decode, paid only by a
  file with a keyed entry): absent, held twice (a duplicate JSON or YAML key;
  TOML has none), or holding anything but a candidate (a number, an object, a
  range, an escaped version) is a `*files.KeyError` that says what the path
  holds. In YAML, so is a path naming an alias, or reaching a value only through
  one or through a merge key, which says where the value is written (§9.7).
- **Text: the bytes in front.** Literal bytes that must come right before the
  token, compared with `bytes.HasSuffix`, byte for byte, case and whitespace
  included. A key cannot hold a line break, so it is on the token's line. It
  names every candidate it precedes, and one that precedes none is the same
  `*files.KeyError` as a missing path. A regular expression would cover more
  (the "Per-file custom match patterns" bullet that §14 used to carry), but a
  literal covers the common cases without a pattern language in the config.

`Located.Claim` then keeps the named candidates that hold the entry's version
(by `matchAt`, so a pinned prerelease extends over a suffix the scanner cut, as
for an entry without a key); none is a `*files.KeyError` too (`holds 1.2.3, not
1.2.4`). Every such error names the file and the key, exits `3`, and is raised
in phase 1, so nothing is written.

**One field, so its meaning follows the parser.** A key changes meaning when
its entry's parser does, through a `parser` override or `discover`'s fallback
to text. Neither direction can name a different occurrence than the one meant.
A path read as text names nothing: a path ends in a letter, a digit, a quote,
or a bracket, and `version.FindTokens` wants a word boundary before a version,
so the bytes before a token never end in `version`. A text key read as a path
is not a path (`"VERSION ?= "` holds a space and a `?`), which `Validate`
refuses with exit `1`, or names a key the file does not have, which the bump
refuses with exit `3`. `TestKeyWhoseParserChangedFails` runs both directions.

**Planning by occurrence.** A `files.Replacement` for an entry with a key is
`Keyed` and carries the start offsets of the candidates it claimed (`At`), and
`SetKnownVersions` rewrites only those for it, still in one pass over the
original bytes so overlapping bumps do not cascade. `Rewrite` checks that a
keyed replacement reached every candidate it claimed. Its counts are now per
replacement, in order, since two keyed entries for one version must be counted
apart; `SetKnownVersions` still sums them per version for its callers. Before
rewriting, `files.Overlap` refuses two entries that would rewrite one
occurrence where at least one has a key: an entry without a key and a keyed
one for the same version, or two keys naming one value (however each is
written). One of them is wrong, and which cannot be told, so it is a config
error naming both, raised in phase 1. Two entries without keys are left to the
most specific pin, as before.

**Discover writes the keys it can be sure of.** Every JSON, TOML, and YAML
candidate has a key path, so `discover` writes one entry per candidate with its
key (§8.2), and for the manifests whose project version is known only that
entry. It infers none for text: the bytes in front of a version are as often
prose (`Released version 1.2.3`) as an assignment, and an entry that broke when
someone reworded a sentence would be worse than one that rewrites every copy.
Running `discover` again over an older config writes the keys in.

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
- Discovery is content-based: it scans every text file, whatever its name, and
  records every candidate the file's parser finds in it (with its line number,
  context, and key). The name decides only the parser. A `.json`, `.toml`,
  `.yaml`, `.yml`, or `.cff` file is read by its format, so there only a whole
  string value is a candidate, and every other file is scanned as text, where
  every `[v]MAJOR.MINOR.PATCH` token is one (see
  [section 9.6](#96-parsers-reading-json-and-toml-by-format) and
  [section 9.7](#97-parsers-reading-yaml-by-format)). Before Milestone 39 it
  read every file as text, and a version in a manifest's URL, range, or
  comment counted the same as its `version` key. In a text file, distinct
  versions become one config entry each; identical repeats collapse to a
  single entry and are all rewritten together on bump. In a JSON, TOML, or
  YAML file, each candidate becomes an entry with its key, and a known
  manifest's dependencies none (see §8.2 and the key design above).
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

### 9.6 Parsers: reading JSON and TOML by format

Until Milestone 39 every file went through the text scanner, which knows
nothing about the format it reads, so in a JSON or TOML manifest every
version-shaped run was a candidate: one inside a URL, a dependency range, or a
comment counted the same as the project's own `version` key. With a
`package.json` holding `"version": "1.2.3"`, a `$schema` URL with `2.0.0` in its
path, three dependencies (`1.2.3`, `4.17.21`, `^18.3.1`), and
`"engines": { "node": ">=20.0.0" }`, next to a `Cargo.toml` whose `version`
line ends in `# was 0.3.9`, `discover` wrote eight entries, and `--minor`
moved all eight, the schema URL and the comment included, with exit `0`. Those
files, and a `pyproject.toml` with the version in its description, a URL, a
comment, and dependency specifiers, are kept in `internal/files/testdata/repro`.

**The seam.** `files.Parser` (`text`, `json`, `toml`, and since Milestone 41
`yaml`, §9.7) is the one place a file's bytes become candidates. `Parser.Locate`
returns each candidate as a byte range, its parsed version, and its key path
(`files.KeyPath`, nil from the text parser), along with the values it skipped.
`discovery.scan`, `files.FindVersion`, `files.SetKnownVersions`, the bump's
planning (`readGroups` and `planGroups`, which `preview` shares), and `undo`'s
`planReverts` all take their ranges from it, and nothing outside `files` calls
`version.FindTokens` to find a version in a file. So the rule at the top of §9
still holds: what discovery records is exactly what a bump rewrites, and every
command reads a file the same way. The text parser is `FindTokens` plus
`version.Parse`, unchanged, so every file no format parser reads behaves exactly
as before. The JSON, TOML, and YAML parsers return a subset of the text parser's
candidates with the same boundaries, so §9's single definition of a token holds
in those files too; the golden test checks it on every fixture. Before this
milestone the comment at the top of `files.go` said structured detection lived
in `discovery`, but nothing there read a format.

**Choosing a parser.** By extension, ignoring case (`files.ParserFor`): `.json`
gets the JSON parser, `.toml` the TOML parser, `.yaml`, `.yml`, and `.cff` the
YAML parser (§9.7), and everything else text. An entry's `parser` key
overrides it (§6.1).

**What counts.** A candidate is a string value, never an object key, whose
whole content is one token `version.Parse` accepts: `"1.2.3"` and `"v1.2.3"`
are candidates, and `"^18.3.1"`, `">=20.0.0"`, and a URL holding `2.0.0` are
not. Each knows its key path, rendered the way TOML writes a dotted key:
`package.version`, `bin[0].version`, `dependencies."@types/node"`, and `"a.b"`
for one key holding a dot. A dependency pin (`"left-pad": "1.2.3"`,
`serde = "1.0.200"`) is a whole string value too, so it is still a candidate;
an entry's `key` (Milestone 40, §9.3) selects the project's version by the key
path recorded here.

**JSON.** The standard library's `encoding/json` `Decoder`, walked with
`Token()` keeping a stack of object keys and array indexes (`walkJSON`). A
string value's bytes run from the first `"` at or after the `InputOffset`
before the token (the separators and whitespace ahead of a value come first)
to the `InputOffset` after it. That needs no dependency:
`encoding/json/jsontext` reports offsets directly, but it is still behind
`GOEXPERIMENT=jsonv2` in Go 1.27. `json.Valid` decides whether a file is JSON,
since the `Decoder` alone would read a second top-level value as a stream
rather than refuse the file. The decoder rejects a UTF-8 byte-order mark,
which §9.4 says incrmit keeps, so the parser decodes the bytes after it and
offsets every range by three. A value whose raw bytes differ from its decoded
text (an escape such as `\u002e`) is not a candidate, because rewriting it
would mean re-encoding the string; one that decodes to exactly a version is
reported as `Skipped`, and the dry run says so. `encoding/json` accepts
duplicate keys, so both values are candidates under the same key path.

**TOML: probing rather than lexing.** `BurntSushi/toml`, which already reads
the config, is the only TOML reader. It decides whether a file is TOML and what
each key holds, but v1.6.0 keeps key positions in an unexported field, so it
cannot say where a value's bytes are. `locateTOML` finds them by probing:
replace every token the text parser finds with a placeholder of its own,
decode the copy once, and walk it (`walkTOML`) through tables, inline tables,
arrays, and arrays of tables (`bin[0]`). A token whose placeholder is now a
value's whole text is a candidate at that value's key. One whose placeholder
is part of a longer string (a URL, a description, a multi-line string) is not,
and one whose placeholder is in no value was in a comment or a key.

A placeholder is built so the probe is exact:

- It is digits and dots, so it is valid wherever its token was: in any string,
  in a comment, and in a bare key, whose characters include digits.
- Each dot-separated segment of the token becomes one run of digits, so a token
  used as a bare dotted key (`[versions.1.2.3]`) splits into exactly as many
  keys as it did, and a key path maps back to the file's keys segment by
  segment (`probe.unmap`), even where a token is welded into a longer bare key
  (`x-1.2.3`).
- Every run is one prefix plus a zero-padded counter of fixed width. In a
  prototype, unpadded placeholders collided (`...1` is a prefix of `...10`),
  and a comment's token was reported as part of `bin[0].version`.
- The prefix occurs nowhere in the file, nor in any key or string the file
  decodes to, since an escape can decode to digits the bytes do not hold. It is
  `987654321` when that is free, as it nearly always is. Otherwise
  `absentDigits` builds one a digit at a time, each time taking the digit that
  fewest runs of digits in the text continue the prefix with: the count falls
  at least tenfold per digit, so nine digits suffice for any file with fewer
  than a billion digits, and a file planted with likely prefixes costs nine
  passes rather than one per candidate.

All tokens share one probe decode. The prototype classified 2,000 tokens in
under 0.1 s, while one decode per token took 2.8 s at 1,000 and grows
quadratically; `TestLocateTOMLIsLinearInTokens` locates 5,000.

**The probe is checked before it is trusted.** With every placeholder put back,
in keys and in strings, the decoded copy must hold exactly the file's scalar
values, each at the same key, none missing and none extra (`probe.agrees`).
The first fuzzing run showed why within seconds: `BurntSushi/toml` v1.6.0
accepts some TOML the specification forbids. `a.b = "x"` followed by `a = "y"`
decodes without an error, keeping the first definition and silently dropping
the second. Mask a token in the first key (`1.0.0 = "x"`, then `"1" = "0.0.0"`)
and the two no longer collide, so the copy held a value the file's own decode
does not, and the probe reported a candidate whose key held a table. A copy
that disagrees is refused the way a copy that does not decode is, as TOML
whose versions incrmit cannot place (exit `3` for a bump, a fallback to text
for `discover`). It also refuses one valid shape: a version naming an array of
tables declared more than once (`[[1.2.3]]` twice), because each declaration
gets its own placeholder and the array splits in two, renumbering its
elements. Tables themselves are not compared, only scalars, since a table
named by a token splits harmlessly (`[1.2.3]` and `[1.2.3.x]` become two tables
in the copy, and every value still maps back to its key).

**Rewrites are checked too.** After planning, `files.Rewrite` decodes the
rewritten bytes in phase 1 and compares them with the original
(`checkRewrite`): every value must be as it was except the planned ones, which
must now hold the planned versions. JSON gets the same check, although its
offsets come from the decoder itself; it costs two decodes of a small file. A
rewrite that fails it is refused before anything is written.

**The TOML decision.** A hand-written locator that lexes TOML to find byte
ranges was the first plan, and `pelletier/go-toml/v2` exposes ranges through
its `unstable` package. Both were rejected: each is a second TOML reader that
could disagree with the one the config is read with, and the second is also a
new dependency with an API marked as subject to change. The probe leaves every
question of TOML syntax to `BurntSushi/toml`, and the agreement check is what
keeps that reader's leniencies from becoming a wrong answer.

**When a file does not parse.** `discover` scans a `.json` or `.toml` file its
parser cannot read as text (`Result.Fallback` holds the `*files.ParseError`),
writes `parser = "text"` into its entries so a bump uses what discover used,
and says why in the dry run (`tsconfig.json: not JSON (line 2: invalid
character '/' ...), scanned as text`). A bump never falls back: an entry whose
parser cannot read its file fails with exit `3` in phase 1, naming the file and
the line and suggesting `parser = "text"`, so nothing is written. A file that
parsed when discover ran and no longer does has changed shape, and guessing is
how a bump goes wrong silently. A `--file` target (for the bump and for
`preview`) has no entry recording how it was read, which is the position
discover is in, so it is read by its extension and falls back to text with a
note on stderr; that is also what `--file` did before the parsers.

**Existing configs change meaning.** An entry for a `.json` or `.toml` file now
matches only whole string values, so an entry an older `discover` wrote for a
version found only in a comment, a URL, or a range (four of the reproduction's
eight) finds nothing, and the bump exits `3` with the existing "expected
version not found" error. That check has run in phase 1 since Milestone 38
(§8.7), so an old config fails with nothing written. None of those versions
was the project's. `undo` uses the parser the bump used, recorded in the
journal (§6.2), and reads an older entry that has none as text, which is what
wrote it.

**Limits.**

- JSON with comments (`tsconfig.json`, VS Code's `settings.json`) and JSON5 are
  not JSON: such a file is scanned as text, by discover's fallback or by
  `parser = "text"`.
- A value written with escapes is never rewritten, even when it decodes to the
  pinned version.
- A TOML file the reader accepts but the specification forbids (a key defined
  twice) can be refused rather than placed, and so can a version naming an
  array of tables declared twice.
- A dependency pinned at the project's version still moves with it under an
  entry without a key. `discover` writes keys since Milestone 40 (§9.3).

### 9.7 Parsers: reading YAML by format

Milestone 39 read `.json` and `.toml` files by their format and left YAML to
the text scanner, yet YAML holds the project version in Helm charts
(`Chart.yaml`), Dart and Flutter packages (`pubspec.yaml`), snaps
(`snapcraft.yaml`), Ansible collections (`galaxy.yml`), and `CITATION.cff`,
and those files had §9.6's problem. The reproduction is a `Chart.yaml` holding
`version: 0.7.3`, `appVersion: "1.4.2"`, a `kubeVersion` range, two
dependencies (`12.5.6` and `"~17.11.3"`), the version in its description, and
an image reference in a block-scalar annotation; a `pubspec.yaml` with
`version: 2.3.1+45`, an SDK range, and three dependencies; and a Kubernetes
manifest of two documents (`internal/files/testdata/repro`). As text,
`discover` wrote eleven entries for the first two, and `--minor` moved every
token with exit `0`: the description, the image tag, both dependencies, and the
range, which came out as `>=1.23.0`, its `-0` dropped. Read as YAML, the chart
holds three candidates of seven and the pubspec three of six, and since both
are manifests discover knows (§8.2), one entry each for the project's version,
two for the chart. This repository's own `ci.yml` and `release.yml` go from
eighteen and twelve candidates to one each; the rest sat in comments
(`# v7.0.1`) or inside `go install ...@v1.6.0` commands, which is why its
config ignores `.github/`.

**The YAML decision.** The standard library has no YAML reader, so this is a
dependency, the module's third (§2, §13). `go.yaml.in/yaml/v3` (v3.0.5) is the
YAML organization's continuation of `gopkg.in/yaml.v3`, and it requires no
other module, so there are still no transitive dependencies. Rejected:
`gopkg.in/yaml.v3`, whose repository is archived; `go.yaml.in/yaml/v4`, still
a release candidate (`v4.0.0-rc.6`), to revisit at its first stable release;
`github.com/goccy/go-yaml`, whose selling point is token positions, which the
probe does not need, and which would be a second, larger dependency for it; and
a hand-written reader, out of proportion to the job, since YAML's grammar is
far larger than JSON's or TOML's. In a minimal program the decoder added about
0.9 MB (2.4 MB to 3.3 MB); on incrmit itself, which already links most of what
the decoder uses, it adds 0.31 to 0.38 MB, about 6 to 7 percent, measured with
the release build's flags: 5.17 to 5.49 MB on `darwin/arm64`, 5.41 to
5.79 MB on `linux/amd64`, and 5.95 to 6.34 MB on `windows/amd64`. Without it, a
Milestone 40 text `key` (`key = "appVersion: "`) still picks out the version in
a YAML file, but `discover` would go on offering every range, comment, and
dependency pin.

**Choosing it.** By extension, ignoring case: `.yaml` and `.yml`, and `.cff`,
since the Citation File Format is YAML by definition. `parser = "yaml"` reads
YAML under any other name, and §9.6's fallback carries over unchanged:
`discover` scans a YAML file that does not parse as text and writes
`parser = "text"`, and a bump never falls back.

**What counts.** `locateYAML` reuses §9.6's TOML probe, with the same
placeholders, `absentDigits` given every scalar and tag the document decodes
to, and decodes with `go.yaml.in/yaml/v3` into a `yaml.Node` tree, never into
Go values. A candidate is a scalar that is a value, not a mapping key, written
plain, single-quoted, or double-quoted, whose tag is `!!str`, implicit or
written (`!!str 1.2.3` counts, `!semver 1.2.3` does not), and whose decoded
text is exactly one placeholder. A placeholder inside a longer scalar (an
image reference such as `ghcr.io/me/app:1.4.2`, a range such as `~17.11.3`),
in a `|` or `>` block scalar however little it holds, in a comment, or in a key
is not a candidate, and neither is a value under a key that is not a scalar
(`? [a, b]`), which no key path can name. A scalar holding a placeholder has at
least two dots, which no YAML number or date has, so a placeholder resolves to
`!!str` just as the version does, and the probe never changes a value's type.
Key paths are §9.6's, built from each key's decoded text (`k8s.io/version` is
`"k8s.io/version"`, a merge key `"<<"`). A double-quoted value that decodes to a
version only through an escape is `Skipped`, as in JSON. Lines come from the
decoder's nodes, and the decoder accepts a byte-order mark; since the probe
moves no byte outside a token, no range needs offsetting, unlike JSON.

**The probe is checked.** As for TOML, the copy must decode, and with every
placeholder put back, in keys, scalars, and tags, it must hold exactly the
file's nodes, in order and at the same paths (`probe.agreesYAML`, comparing
`flattenYAMLTree`s). Neither failure is known to happen except one: the decoder
limits an implicit key to 1,024 characters, and a placeholder (about 56
characters for a three-part token) is wider than most tokens, so a key holding
a few dozen versions passes the limit in the copy alone. That copy does not
decode, and the file is refused as YAML whose versions incrmit cannot place
(`testdata/implicit-key.yaml.input`). After planning a bump, `files.Rewrite`
decodes the rewritten bytes and checks that exactly the planned keys changed,
as for TOML.

**Anchors and aliases.** A `yaml.Node` keeps an alias as an alias node rather
than a copy, so an anchored version (`base: &v 1.2.3`) is one candidate at the
anchor's key, and an alias of it (`pinned: *v`) is not a candidate of its own:
it has no bytes to rewrite. Rewriting the anchored value changes every alias,
so:

- the dry run lists the aliasing keys beside the candidate
  (`L1 base: base: &v 1.2.3 (aliased at pinned)`). `Located.Aliases` gives
  them: an alias of the candidate's own anchor, of an anchored mapping or
  sequence holding it, or of an anchored node holding such an alias, and so on,
  in document order. A merge key (`<<: *defaults`) is an alias too and is
  listed at its own path (`prod."<<"`). The search climbs from the candidate's
  innermost anchor outward, and from each alias found climbs again, stopping at
  the first anchor an earlier climb passed, so each anchor is climbed past once
  and each alias found once: the cost is the size of the answer, however the
  aliases nest or cycle (an alias inside its own anchor, `a: &a [*a]`, decodes).
- the phase-1 check counts them as planned: `flattenYAMLTree` records an alias
  as the leaf of the node it names, not as that node's value, so an alias is
  unchanged by a rewrite of its anchor while what it reads changes with it.
- a Milestone 40 `key` that names an alias is an error naming the anchor's
  key (`key pinned reads base through an alias, and incrmit rewrites a value
  where it is written; name base instead`), and so is a path the data has only
  through an alias or a merge key (`prod.version` for a merged
  `defaults.version`), which `resolveYAML` follows the way a reader of the
  data would, a mapping's own keys before merged ones.

**One document per file.** A YAML stream can hold several documents, and a key
path has no way to name one, so the parser accepts exactly one (a leading
`---` or a trailing `...` is fine); a file with none, empty or only comments,
holds nothing. A file with more is a `*files.ParseError` with `Docs` set,
which takes §9.6's fallback with the reason in the dry run
(`deploy.yaml: 2 YAML documents, scanned as text`) and fails a bump of an entry
without `parser = "text"` with exit `3`. Multi-document files are mostly
Kubernetes manifests, whose versions sit inside image references the YAML
parser would not offer anyway, so text with a text `key` is what they need:
`parser = "text"` and `key = "ghcr.io/me/app:"` move the image tag alone. A
document selector in the key syntax can come later if someone asks for one.

**Duplicate keys** are treated as in JSON. YAML forbids them, but decoding into
a `yaml.Node` accepts them, so both values are candidates under the same key
path, and Milestone 40's error for a key that appears twice applies.

**Hostile input.** The decoder refuses nesting past 10,000 levels with an error
rather than exhausting the stack (100,000 nested `[` in milliseconds, 20,000
nested block mappings in about 0.3 s), which bounds every recursive walk of the
tree. Block mappings nested that deep need a line of indentation per level,
50 MB of it by the limit, past `discover`'s 32 MiB cap, so the tests use flow
sequences, flow mappings, and block sequences opened on one line. Within the
limit, the probe's check compares each node's depth and last path step
(`yamlShapes`): in the walk's order those fix every node's path, and rendering
the whole path per node, as the first version did, made a 50 KB file nested
9,990 deep take 1.8 s to locate rather than 17 ms. Decoding into a
`yaml.Node` expands no aliases, so a billion-laughs document decodes at once
(decoded into Go values, the decoder refuses it as excessive aliasing), and
neither the walks, the checks, nor `Aliases` expand one.
`TestYAMLNestingIsRefused` and `TestYAMLAliasBombs` pin all of it, the latter
with a 20,000-link alias chain too.

**pubspec's build number.** Flutter reads the part after `+` in
`version: 2.3.1+45` as the build number, and §9.2 drops build metadata on a
bump, so a patch bump writes `2.3.2`. The text scanner did the same; §9.2 is
unchanged, and `doc/USAGE.md` says so under `pubspec.yaml`.

**Existing configs change meaning**, as in §9.6: an entry for a `.yaml`,
`.yml`, or `.cff` file that pinned a version found only in a comment, a range,
or a longer string now finds nothing and fails in phase 1, and an entry for a
manifest of two documents fails as one the YAML parser cannot read. The case
most likely to surprise is an image tag in a Kubernetes manifest, fixed with
`parser = "text"` and a text `key`. `undo` takes the parser from the journal.

**Limits.**

- A block scalar is never a candidate, even `|-` holding exactly a version,
  and neither is a version with a tag of its own.
- The decoder reads YAML 1.1 and refuses a `%YAML 1.2` directive, although it
  reads the same document without one; such a file is scanned as text.
- The decoder's error lines are where the construct around the problem began,
  at times the line before the problem itself.
- A key holding a few dozen versions is refused by the probe (above).
- Values under a key that is not a scalar cannot be named, so they are not
  candidates.
- `Rewrite`'s check renders every node's whole path, as it does for JSON, so
  bumping a file nested thousands of levels deep costs the square of its
  depth: about 1.5 s for 9,990 levels. It runs only for a file a config
  lists; `discover` never pays it.

## 10. Error Handling

- Missing config file: clear message; suggest running `incrmit discover`.
- No version found in a target: report the file and continue or fail fast
  (configurable; fail fast by default for bump).
- Multiple versions found in a single file: report ambiguity and skip unless a
  format-specific rule resolves it.
- A bump that would carry a number past `math.MaxInt` (a component, or the
  prerelease counter `--pre` advances): report
  `<path>: version: cannot bump the patch component of <version>: incrmit
  counts only up to 9223372036854775807`, write nothing, and exit `3`.
  `--dry-run` fails the same way; `preview` shows the projection as `n/a`.
- Filesystem/permission errors: surface the underlying error and exit non-zero.
- A config entry whose parser cannot read its file (see
  [section 9.6](#96-parsers-reading-json-and-toml-by-format) and
  [section 9.7](#97-parsers-reading-yaml-by-format)): report
  `<path>: not JSON (line <n>: <reason>); to read it as text, set parser =
  "text" on its [[files]] entry` (or `not TOML`, `not YAML`, `TOML whose
  versions incrmit cannot place` or `YAML whose versions incrmit cannot place`
  when the probe is refused, or `<n> YAML documents`) and exit `3` in
  planning, before anything is written, `--dry-run` and `preview` included. A
  `--file` target is read as text instead, after a note on stderr
  (`incrmit: note: <path>: not JSON (...), so it is read as text`).
- A `parser` value incrmit does not know, or two parsers for one path: report
  `config: files[<i>] (<path>): unknown parser "<value>"; use "json", "toml",
  "yaml", or "text"` (or that the entries read the file two ways) and exit `1`.
- A key its entry's parser cannot read (see
  [section 9.3](#93-what-the-config-knows-that-the-grammar-cannot)): report
  `config: files[<i>] (<path>): key "<key>" is not a key path (<why>); ...`
  (or that it is empty, or holds a line break) and exit `1` as the config
  loads. The same `--key` is a usage error, exit `2`.
- A key that names no candidate holding the entry's version: report
  `<path>: key <key> is not in the file` (or `holds an object, not a version`,
  `holds 1.2.3, not 1.2.4`, `appears twice in the file, so it names no one
  value`, `is not followed by a version anywhere in the file`, or for YAML
  `reads base through an alias, and incrmit rewrites a value where it is
  written; name base instead`) and exit `3` in planning, `--dry-run` and
  `preview` included, with nothing written.
- Two entries that would rewrite one occurrence, one of them keyed: report
  `config: files[<i>] (key <key>) and files[<j>] (no key) both claim the
  <version> on line <n> of <path>; remove one, or give each a key that names
  its own occurrence` and exit `1` in planning, with nothing written.
- A rewrite that the phase-1 decode does not confirm (a JSON, TOML, or YAML
  value changed that was not planned, or a planned one that did not change):
  report `<path>: files: the rewrite would change <key>, which was not planned`
  and exit `1` (`3` when the result no longer parses), with nothing written.
  This would be a parser bug; the check is there so it cannot reach a file.
- Undo of a change whose journal `parser` this version does not know, that
  records two parsers for one file, or whose journal `key` that parser cannot
  read: refuse and exit `1`. A file the recorded
  parser cannot read any more: report it, as for a bump, and exit `3`. Nothing
  is written either way.
- A target that is not an ordinary file (a named pipe, device, or socket, whether
  named directly or reached through a symlink): report
  `reading <path>: not a regular file` and exit `1`. `files.ReadTarget` checks the
  type before opening, because opening a pipe blocks until a writer appears and
  would otherwise leave incrmit hanging with no output at all.
- A config or state file that is not an ordinary file (the same kinds,
  directly or through a symlink): report
  `config: reading <path>: not a regular file` (`history: reading ...` for the
  state file) and exit `1`, before anything is written. One over 16 MiB is
  reported the same way with its size and the limit.
- A `discover --output` that exists and is not an incrmit config (not TOML, a
  key incrmit does not use, comments alone, not an ordinary file, or over
  16 MiB): report `<output> exists and is not an incrmit config (<reason>);
  choose another --output or remove the file` and exit `1` before the scan,
  `--dry-run` included. A missing or empty file, or a config, is replaced.
- Undo with nothing to revert (no journal or an emptied one): print a friendly
  message and exit `0` — it is not an error.
- Undo conflict (a file edited since the bump — or bumped past by another run —
  no longer holds the recorded `new` token): report which file diverged, write
  nothing, and exit `1` (generic error).
- Undo of a change the config does not list (the config was edited since the
  bump, or the state file names a path of its own): report
  `<path>: not listed in <config> at <new>, the version the last bump recorded
  (refusing to undo)`, write nothing, and exit `1`. For a `pending` entry the
  message names both versions (`at <new> or <old>, the versions the unfinished
  bump recorded`), and a config listing some changes at one and some at the
  other is refused the same way.
- Undo of a `pending` entry where a file holds every version the run wrote and
  every one it started from, or neither: report
  `<path>: holds both ...` or `<path>: holds neither ...`, write nothing, and
  exit `1`.
- A write that fails partway through a bump or an undo: report it, put back
  the files the run already wrote and the journal, report
  `rolled back <files>; no file was changed`, and exit `1`. A rollback that
  fails as well is reported per file, followed by the files that could not be
  put back and the `incrmit undo` command that finishes the job; the journal
  keeps the entry `pending`, and the exit is `1`. See
  [section 8.7](#87-crash-safety-all-or-nothing-across-files).
- A bump that fails in planning while the newest journal entry is `pending`:
  report the failure, then say the last bump or undo stopped partway and name
  the undo command.
- Marking a bump's entry `complete`, or popping an undone entry, fails after
  every file was written: warn on stderr and exit `0`, since the work is done
  and the `pending` entry left behind still undoes correctly.
- A state file with an entry whose `status` is neither `pending` nor
  `complete`: report `history: parsing <path>: entry <n> has unknown status
  "<status>"` and exit `1` before anything is written.
- A project already held by another `incrmit` run: report it, name `--wait`,
  write nothing, and exit `1` (generic error). See
  [section 8.6](#86-concurrency-one-writer-per-project) for why this fails fast
  rather than blocking.
- Locking unavailable rather than contended (a filesystem without lock support,
  an unwritable directory): warn on stderr and carry on unlocked. This is not an
  error and does not change the exit code.

### Exit Codes

| Code | Meaning                                                        |
| ---- | -------------------------------------------------------------- |
| `0`  | Success.                                                       |
| `1`  | Generic runtime error.                                         |
| `2`  | Invalid arguments or flags.                                    |
| `3`  | No version found / parse failure / a number too large to bump. |

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
│   ├── files/              # parsers (text, JSON, TOML, YAML), keys, read/write helpers
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
- Undo away from where the bump ran (`internal/cli/undopaths_test.go`): in a
  copied project, a moved one, a subdirectory with `-c ../incrmit.toml`, over a
  config that lists a `../` path, and from a state file an earlier version
  wrote. A hand-written journal whose `fs` or `config` names somewhere else, or
  whose `path` the config does not list, must leave everything outside the
  project unchanged, compared by snapshot as in the lock tests.
- Bumps at the integer ceiling (`internal/cli/overflow_test.go`): each
  component, and a prerelease counter, at `math.MaxInt`, through a bump,
  `--dry-run`, `--pre`, and `preview`. A bump must exit `3` naming the file and
  the component, with the project's snapshot unchanged; `preview` must show
  `n/a` in that column, aligned (`testdata/preview_overflow.golden`). One below
  the ceiling must bump to it, and `undo` must find the token it wrote.
- What `discover --output` may replace (`internal/cli/discoveroutput_test.go`):
  a Markdown file, one of headings alone, a target, `package.json`,
  `pyproject.toml`, and a config with an extra key are each refused, by a real
  run and by `--dry-run`, with exit `1`, the advice on stderr, and the file's
  bytes and the project's snapshot unchanged; a missing, empty, or blank file
  and each shape of config are regenerated. `config.TestLoadIgnoreRefusesNonConfig`
  covers each reason at the unit level.
- Paths relative to the config (`internal/cli/discoverpaths_test.go`):
  `discover` with `--path` and `-o` naming the same directory, a directory
  below it, one above it, and a sibling, spelled relative, with `./`, from a
  subdirectory, and absolute, plus a `--path` that names a file. Each tree
  holds a decoy with the same version where a scan-root-relative path would
  resolve, and the test bumps the config `discover` wrote and asserts that
  exactly the scanned files changed. The dry run and the summary must print
  the paths the config holds. `internal/discovery/configdir_test.go` covers
  the same shapes at the unit level, `ignore` patterns under a `--path` other
  than the config's directory, and a directory reached through a symbolic
  link or not created yet.
- Writes that fail or stop partway (`internal/cli/crashsafe_test.go`). The
  writes go through two package variables, `writeFile` and `saveJournal`, which
  a test replaces to fail the Nth write of a run, fail a rollback write too, or
  "crash" there: panic, so the run unwinds without its rollback or anything
  after that point, as a `kill -9` would. A bump over three targets in their own
  directories, with an earlier bump already in the journal, is failed and
  crashed at each target, the config, and both journal writes, and so is an
  undo of it. After each, the project must be byte-identical to how it started,
  either straight away (a clean rollback) or after one `incrmit undo` (a failed
  rollback or a crash, which must leave a `pending` entry naming the bump).
  Real permission failures cover the same ground without the variables: a
  read-only target directory, and a config directory where the journal cannot
  be written, which must now stop the bump before any target changes. Undo of
  hand-written `pending` entries covers files never written, edited files that
  hold both sides or neither, and configs at neither version or split.
- The parsers (`internal/files/parser_test.go`, and `yaml_test.go` for YAML).
  Golden fixtures in `internal/files/testdata`: the existing `package.json`,
  `package.min.json`, `appsettings.json` (a BOM), and `pyproject.toml`, a
  `Cargo.toml`, the reproduction files in `testdata/repro`, and for YAML
  `shapes.yaml.input` (every shape below in one document) and
  `bom-crlf.yaml.input`. Each golden
  (`testdata/candidates/*.golden`, regenerated with `go test ./internal/files/
  -update`) lists every token the text parser finds, with the key the file's
  parser places it at or `(not a candidate)`, and for YAML the aliases that read
  it, so it shows what reading the format excludes; the test also checks that
  each candidate is one of the text
  parser's, and rewrites every candidate through `files.Rewrite` and compares
  the bytes outside them. Table tests cover, for JSON, a BOM, CRLF, minified and
  tab-indented files, nested arrays, a root array and a root string, duplicate
  keys, escaped values, keys holding dots, and rejected input with its line;
  for TOML, dotted keys, a quoted key holding a dot, a quoted key and a bare key
  that are versions, a token welded into a key, inline tables, a sub-table
  (`[dependencies.tokio]`), arrays of tables, an array of version strings, a
  URL, multi-line strings holding `version = "1.2.3"` as text, comments, and
  escapes. Placeholder collisions get their own: more than ten tokens with
  comment tokens between them, a file already holding the placeholder text in
  a comment, a value, a key, or an escape, and a file planted with two thousand
  likely prefixes. Further tests cover the probe's agreement check, `Rewrite`
  refusing an unplanned change, and a TOML file of 5,000 tokens located within
  a deadline. For YAML (Milestone 41): a BOM needing no offset, CRLF and a lone
  CR, flow mappings and sequences, all three quoting styles, a comment after a
  value, `|` and `>` block scalars holding `version: 1.2.3` as text, anchors,
  aliases, and merge keys with the aliases each candidate lists, a duplicate
  key, an explicit `!!str` tag and a custom one, a key that is a version, a key
  that is not a scalar, escapes, and placeholder collisions; and the files it
  refuses: a two-document manifest, tab indentation, a Jinja-templated conda
  `meta.yaml`, and the implicit-key overflow (`*.yaml.input` fixtures). Every
  `*files.KeyError` YAML adds, aliases and merges included, is in
  `TestClaimYAML`, and the hostile shapes (nesting past the decoder's limit,
  billion laughs, an alias chain) are in `shapes_test.go` with Milestone 31's.
- The reproduction and the parser's effect on each command
  (`internal/cli/parsers_test.go`): `discover --dry-run`, the config `discover`
  writes, and a `--minor` bump compared byte for byte against goldens
  (`internal/cli/testdata/repro_*.golden`), then an `undo` back to the original
  bytes. Also discover's fallback writing `parser = "text"` and a bump keeping
  it, a bump refusing a file its parser cannot read with every file unchanged,
  the `parser` override in both directions, an unknown `parser` value, a config
  written before the parsers failing in phase 1 with every file unchanged, undo
  reading each file with the journal's parser (and an older entry's as text),
  `--file` and `preview --file` reading by format, and escaped values in the dry
  run. YAML's (`internal/cli/yaml_test.go`) reproduce Milestone 41 the same way
  (`yaml_repro_*.golden`), and run the config the earlier `discover` wrote: it
  fails in phase 1 with every file unchanged, and with `parser = "text"` on its
  entries moves every token exactly as the previous release did
  (`yaml_repro_text_minor_*.golden`, checked against that release's output).
  They also cover a two-document file refused by a bump and read as text by
  `--file`, the image-tag recipe, the `parser` override in both directions, a
  key naming an alias or reaching through one, the dry run's alias list, a
  pubspec build number dropped by a patch bump, and undo reading a YAML file
  with the journal's parser.
- Keys (Milestone 40). `internal/files/key_test.go` covers path syntax both
  ways (every rendered path reads back, and the malformed ones each fail with
  their reason), key parsing per parser, path matching (top level only, quoted
  segments, `[N]` indexes into arrays and arrays of tables, a key holding a
  dot, `"."`), text matching (whitespace, case, the same line only, every
  occurrence it precedes, a prerelease suffix the scanner cut), every
  `*KeyError`, keyed replacements in one pass, and `Overlap`.
  `internal/cli/keys_test.go` reproduces the pins moving without a key
  (`repro_unkeyed_*.golden`, the Milestone 39 config and bump), bumps each
  reproduction file and a `Makefile` with and without its key byte for byte,
  and covers each error with every file unchanged (an absent key, each kind of
  wrong value, a duplicate key, an invalid path, an empty or multi-line text
  key, two entries claiming one occurrence, a key whose parser changed in both
  directions), `discover`'s output for each known manifest and one without its
  project key, a bump and an `undo` keeping `key`, `undo` using the journal's
  key after the config changed, `--file --key`, and `preview`'s `KEY` column.
  `internal/discovery/keys_test.go` covers the manifest table and one entry per
  candidate, and `internal/config/key_test.go` validation.
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
- `version.FuzzBump` — for any version `Parse` accepts, every component bump and
  every same-series prerelease advance either refuses with a
  `*version.OverflowError`, exactly when the number it increments is at or past
  `math.MaxInt`, or yields a token that round-trips through `String()` and that
  `Compare` ranks above the input. Switching series is left out, since `rc` to
  `beta` may rank lower by design. The test decides "at or past" by comparing
  digit strings rather than calling `Atoi`, so it checks the implementation
  against a second reading of the rule, not a copy of it.
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
- `files.FuzzLocateJSON` / `files.FuzzLocateTOML` / `files.FuzzLocateYAML` — for
  any input the decoder accepts, every candidate's bytes are exactly the string
  the decoder reports at the candidate's key path, and rewriting one candidate
  with a version of a different length, then decoding again, changes that key
  and no other. The reference reads the document independently of the parser's
  own walk: a recursive `Decoder` read that keeps duplicate keys for JSON, and a
  map lookup in a plain `toml.Decode` for TOML. Input the decoder rejects must
  be refused, never panicked on, and TOML the decoder accepts may be refused
  only with the probe's own error. Since Milestone 40, every candidate's key
  path, rendered and read back with `files.ParseKey`, must claim exactly that
  candidate's range, unless the reference holds more than one value at the path
  (a JSON key written twice), where `Claim` must refuse it; that is the promise
  `discover` relies on when it writes a key. A 45-second run of each found
  nothing (5.2 million JSON and 6.9 million TOML inputs). The YAML target's
  reference walks the decoder's `yaml.Node` tree on its own (an alias as the
  anchor it names, a node under a key that is not a scalar out of any key's
  reach) and requires a candidate to be a plain or quoted `!!str` there. A
  stream of more than one document must be refused with its count, and one
  document the decoder accepts may be refused only with the probe's own error.
  Milestone 41 ran it for 90 s and then 5 minutes, 12.3 million inputs in all,
  and found nothing. The corpora are seeded with the golden fixtures.
- `cli.FuzzDisplayName` / `cli.FuzzTerminalText` — a rendered name is
  printable and reads back, with `strconv.Unquote` when quoted, to the one name
  it came from, which is what makes the rendering injective; and escaped text
  is valid UTF-8 holding only printable characters, newlines, and tabs, with
  text that was already safe passed through unchanged.
- `config.FuzzLoad` — the config is trusted input, but a truncated or
  hand-mangled file is not a hostile one: arbitrary bytes must produce a
  `config: ...` error, never a panic, and never a config that loaded into a
  shape the commands do not expect. What loads must also marshal and reload
  identically, because a bump rewrites the config it just read. `LoadIgnore`
  may fail only with a `*config.NotConfigError`, and must accept every config
  `Marshal` writes with the same `ignore` list, or `discover` would refuse to
  regenerate a config incrmit wrote. A loaded entry's `key` must be one its
  parser reads.

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

Milestone 35 found a fourth defect of the same kind, reached through arithmetic
rather than parsing. `Parse` reads a component up to `math.MaxInt`, and the bump
methods added one without checking, so `1.2.9223372036854775807` bumped to
`1.2.-9223372036854775808`. That was written to the file and the config, and
then no command could find it. It is the limit that sits beside the two
spellings `Parse` refuses: every number incrmit counts stops at
9223372036854775807, and a bump past it is refused (§9.2). `FuzzBump`, seeded
with each component and a prerelease counter at the boundary, now guards it.

Milestone 39's first run of `FuzzLocateTOML` found, within seconds, that the
TOML reader is more lenient than the specification. `#00`, `1.0.0= "00000"`,
`"1"="0.0.0"` decodes without an error, the second definition of `1` silently
dropped, and with the first key's token masked the probe no longer saw the
collision, so it reported a candidate at a key that held a table. The probe now
checks its answer against the file's own decode and refuses the file when they
differ (§9.6). The input is kept as
`internal/files/testdata/fuzz/FuzzLocateTOML/key_defined_twice_hidden_by_masking`.

## 13. Build and Release

- Build: `make build` (stamps the version via `-ldflags`) or
  `go build -o incrmit .`. All Makefile builds pass `-trimpath`, so a binary
  carries no absolute path from the machine that produced it and the same source
  yields the same bytes anywhere.
- Test: `go test ./...` (or `make check` for fmt/vet/lint/coverage).
- Lint: `go vet ./...` and `gofmt`/`golangci-lint`.
- Pre-commit hook: `make hooks` (once per clone) points git at `.githooks/`,
  whose `pre-commit` runs `make check` before every commit. It checks the
  staged content, copied out of the index, not the working tree, so a commit is
  judged on exactly what it contains. It needs `golangci-lint` on `PATH`, at
  the version pinned in `.github/workflows/ci.yml`. `git commit --no-verify`
  skips it.
- Cross-compile: `make dist VERSION=X.Y.Z` builds static binaries for
  `linux/amd64`, `linux/arm64`, `darwin/amd64`, `darwin/arm64`,
  `windows/amd64`, and `windows/arm64` into `dist/`, named
  `incrmit-<version>-<os>-<arch>`.
- Release packages: `make release VERSION=X.Y.Z` runs `dist`, then creates
  per-platform archives (`.tar.gz` on Unix, `.zip` on Windows) and Linux `.deb`
  and `.rpm` packages. No checksum file is published: GitHub records a SHA-256
  digest for every release asset itself.
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
- macOS release: `make release-macos VERSION=X.Y.Z` runs `pkg`, the target the
  macOS job in the release workflow builds with.

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
   archives and the `.deb` and `.rpm` packages.

A separate `release-macos` job runs on a `macos-latest` runner, builds the
`.pkg` installers with `make release-macos`, and uploads them to the same
release (the macOS toolchain needed by `pkgbuild` is unavailable on the Linux
runner).

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
  trees into `go.mod` as requirements, and keeping the module at three
  dependencies (`BurntSushi/toml`, `golang.org/x/sys`, and, since Milestone
  41, `go.yaml.in/yaml/v3`, §9.7) with no transitive ones is worth more than
  re-pinning something the checksum database already pins.

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
- Per-file custom match patterns in the config. An entry's `key` (Milestone
  40, §9.3) covers the common cases with a literal: a path for JSON, TOML, and
  YAML, the text in front of the version for anything else. A pattern language
  is left for when a literal is not enough.
- A document selector in a YAML key, so a manifest of several documents can be
  read by its format rather than as text (§9.7), if someone asks for one.
- `go.yaml.in/yaml/v4` once it has a stable release (§9.7).
- Setting an explicit version instead of incrementing.
