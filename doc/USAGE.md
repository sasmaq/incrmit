# Incrmit — User Guide

The full reference for `incrmit`: the config file, every command and flag, and
the edge cases behind them. For installation and a quick start see the
[README](../README.md).

- [Configuration](#configuration)
- [Bumping](#bumping)
- [Discovery](#discovery)
- [Preview](#preview)
- [Undo](#undo)
- [Version](#version)
- [Help](#help)
- [Concurrent runs](#concurrent-runs)
- [Exit codes](#exit-codes)

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
  key = "version"

[[files]]
  path = "internal/buildinfo/buildinfo.go"
  version = "0.1.0"
```

Each `[[files]]` entry describes one file to update. Every listed file is
parsed and bumped together so their versions stay in sync.

An entry may also record a `version`, which pins the exact value to bump
(useful for files that contain several version-like strings). When `version`
is omitted, `incrmit` scans the file for the first version token. A pinned
version is matched whole, prerelease and build sections included, so an entry
holding `1.2.3-rc.1` never matches a bare `1.2.3` elsewhere in the same file.
After a successful bump, `incrmit` rewrites `incrmit.toml` so each entry
reflects the new value, keeping the config in step with the files it manages.

A prerelease or build section is recorded in its own key rather than inside
`version` (see [Prereleases and build metadata](#prereleases-and-build-metadata)):

```toml
[[files]]
  path = "VERSION"
  version = "1.2.3"
  prerelease = "rc.1"   # the file holds 1.2.3-rc.1
```

An entry may also set `parser` to choose how its file is read: `"json"`,
`"toml"`, or `"text"`. Omitted, the file's extension decides (see
[How a file is read](#how-a-file-is-read)). And it may set `key` to say which
occurrence is the project's version, so a bump leaves a dependency pinned at
the same version alone (see [Which occurrence: `key`](#which-occurrence-key)).

That rewrite regenerates the file from its parsed contents in a fixed layout, so
your `[[files]]` entries (with their `parser` and `key`) and `ignore` list are
preserved but hand-written comments and custom formatting are not. Keep notes
you want to survive a bump outside `incrmit.toml`.

A `--dry-run` previews the change and writes nothing — neither the targets
nor the config.

A `path` is resolved relative to the directory holding the config, and may also
be absolute or reach outside that directory with `../`. `incrmit.toml` is
therefore treated as trusted input, on a par with a `Makefile`: whatever it lists
is what gets rewritten. Review the config before running `incrmit` in a checkout
you do not control, and prefer paths inside the project. The
[state file](#undo) is not trusted the same way: `undo` reverts only files the
config lists.

If a listed `path` happens to be a symlink, `incrmit` writes the new version *in
place of the link* rather than through it: the link becomes an ordinary file and
whatever it pointed at is left untouched. Point entries at real files to avoid
the surprise.

A target must be an ordinary file. A named pipe, device, or socket is reported as
`reading <path>: not a regular file` (exit `1`) rather than opened, since reading
one could block forever.

The same goes for the files `incrmit` reads for itself: the config (including
the [`--output`](#discovery-flags) that `discover` reads an `ignore` list from)
and the [state file](#undo). One that is a named pipe or a device, or a link to
one such as `incrmit.toml -> /dev/zero`, is reported as
`config: reading "incrmit.toml": not a regular file` (or
`history: reading ".incrmit.state.toml": ...`) with exit `1`, before anything is
written. A link to an ordinary file is still followed, so a symlinked config
works. Each of these files is also capped at 16 MiB, whatever `--max-file-size`
says; that flag is about targets.

A `path`, like a `--file` argument, names one file literally: spaces, a leading
dash, non-ASCII characters, and glob characters such as `*`, `?`, and `[` are
just part of the name, so `path = "[ab].txt"` bumps the file called `[ab].txt`
and never `a.txt`. Only the [`ignore` list](#ignoring-folders-and-files) treats
those characters as patterns.

### What a bump keeps

A bump rewrites the version token and nothing else. Every other byte of the file
comes back exactly as it was: CRLF, LF, mixed, or classic-Mac (`\r`) line
endings, a UTF-8 byte-order mark, a missing final newline, and any non-ASCII
text around the version. Nothing is normalized.

`incrmit` reads a file as bytes and looks for an ASCII version token in them,
which works in UTF-8 and in any other encoding that writes digits and dots as
ASCII (Latin-1, Windows-1252, ...). **UTF-16 is not supported**: it stores every
character in two bytes, one of them `NUL`, so there is no `1.2.3` in the file
for `incrmit` to see. A UTF-16 target reports `no semantic version found` (or
`expected version not found` when the config pins a version) with exit `3` and
is left untouched, and `discover` skips UTF-16 files as binary. Convert such a
file to UTF-8 to manage it with `incrmit`.

The new contents are written to a temporary file beside the target and renamed
over it, so a crash can never leave a half-written file. The rename puts a *new*
file under the target's name, and what the old file carried beyond its contents
follows from that:

- **Permission bits are kept**, and a **read-only file is still bumped** and
  stays read-only: nothing opens it for writing. To protect a file from
  `incrmit`, make its directory unwritable.
- **The setuid, setgid, and sticky bits are dropped.** A bumped file is a new
  file written by whoever ran `incrmit`, and it is not given privileges it was
  never written with.
- **Hard links are broken.** The name you bumped gets the new version; any other
  name for the same file keeps the old contents. List every name in the config
  if each should be bumped. (A symlink is no workaround: it is replaced by a
  regular file, as described above.)
- **Ownership, extended attributes, and ACLs** are those of a new file created
  by the user running `incrmit`, not the original's.

### How a file is read

Which occurrences of a version count depends on the file's format, which
`incrmit` takes from its extension, ignoring case:

| File                        | Read as | A version counts where      |
| --------------------------- | ------- | --------------------------- |
| `*.json`                    | JSON    | it is a whole string value  |
| `*.toml`                    | TOML    | it is a whole string value  |
| `*.yaml`, `*.yml`, `*.cff`  | YAML    | it is a whole string value  |
| anything else               | text    | any version token appears   |

A `.cff` file is a `CITATION.cff`, which the Citation File Format defines as
YAML.

In a JSON, TOML, or YAML file, `"version": "1.2.3"`, `version = "v1.2.3"`, and
`version: 1.2.3` count, and so does a version in an array (`["1.2.3"]`). A
version inside a longer string does not (a `$schema` URL, a range such as
`"^18.3.1"` or `">=20.0.0"`, a description), and neither does one in a comment,
in a key, or in a multi-line string holding other text. Each occurrence that
counts knows the key it sits under, written the way TOML writes a dotted key
(`package.version`, `dependencies.serde.version`, `bin[0].version`, and `"a.b"`
for one key holding a dot), and `discover --dry-run` prints it next to the line
number.

A dependency pinned at the project's version is a whole string value as well
(`"left-pad": "1.2.3"`, `serde = "1.0.200"`), so it still counts. An entry's
[`key`](#which-occurrence-key) is what tells the two apart.

**YAML.** A version counts written plain (`version: 1.2.3`), single-quoted, or
double-quoted, and with the `!!str` tag (`!!str 1.2.3`). It does not count
inside an image reference (`image: ghcr.io/me/app:1.4.2`), a range
(`version: "~17.11.3"`, `kubeVersion: ">=1.22.0-0"`), or a description; in a
`|` or `>` block, even one holding nothing else; with a tag of its own
(`!semver 1.2.3`); or in a comment or a key. An anchored version counts once,
where it is written:

```yaml
base: &v 1.2.3    # the version, key base
pinned: *v        # an alias: no bytes of its own, so it changes with base
```

`discover --dry-run` lists the aliases a bump changes with it
(`L1 base: base: &v 1.2.3 (aliased at pinned)`), including merge keys
(`<<: *defaults`), and a [`key`](#which-occurrence-key) that names an alias
fails, naming where the value is written. A key written twice gives two
values at one key, as in JSON.

A YAML file holds one document. One with several, such as a Kubernetes
manifest with a `---` between a Deployment and a Service, is read as text,
where every version token counts, image tags included. To move one image tag
and nothing else, track it as text with the text in front of it as its key:

```toml
[[files]]
  path = "deploy.yaml"     # two documents, so read as text
  version = "1.4.2"
  parser = "text"
  key = "ghcr.io/me/app:"  # only the image tag, not the version labels
```

A value that is a version only once its escapes are decoded, such as
`"1\u002e2.3"`, does not count: `incrmit` rewrites a version's bytes in place
and never re-encodes a string. `discover --dry-run` lists it as skipped.

Set `parser` on an entry to choose for yourself:

```toml
[[files]]
  path = "tsconfig.json"   # JSON with comments, which is not JSON
  version = "5.4.2"
  parser = "text"

[[files]]
  path = "Pipfile"         # TOML under another name
  version = "1.2.3"
  parser = "toml"
```

The values are `"json"`, `"toml"`, `"yaml"`, and `"text"`, spelled exactly;
anything else is a config error naming the entry (exit `1`). Omitting the key
means "by extension", so every config written before the key existed still
loads. Entries listing the same file must agree on its parser, since the file is
read once. A bump keeps each entry's `parser` when it rewrites the config.

**When a file does not parse.** `discover` scans a `.json`, `.toml`, `.yaml`,
`.yml`, or `.cff` file its parser cannot read as text, and so a YAML file of
more than one document, says why in the dry run, and writes `parser = "text"`
into its entries, so a bump reads the file the way `discover` did:

```text
  tsconfig.json: not JSON (line 2: invalid character '/' looking for beginning of object key string), scanned as text
    L3: "version": "5.4.2"
  deploy.yaml: 2 YAML documents, scanned as text
    L7: app.kubernetes.io/version: "1.4.2"
```

A conda `meta.yaml` with Jinja templates (`{% set version = "1.2.3" %}`) is
not YAML, and is read as text the same way. So is a file that declares
`%YAML 1.2`, which the YAML reader refuses although it reads the same document
without that line.

A bump never falls back. An entry whose parser cannot read its file fails while
planning, with exit `3` and nothing written, naming the file and the line:

```text
incrmit: package.json: not JSON (line 4: invalid character '}' looking for beginning of object key string); to read it as text, set parser = "text" on its [[files]] entry
```

A file that parsed when `discover` ran and no longer does has changed shape, and
guessing is how a bump goes wrong silently. A `--file` target has no entry
recording how it was read, so it is read the way `discover` would read it for
the first time: by its extension, falling back to text with a note on stderr.

A TOML file can also be refused because `incrmit` cannot be sure where its
versions are. It is read with the same library as the config, which accepts a
few things the TOML specification forbids, chiefly a key defined twice
(`a.b = "x"` followed by `a = "y"`), of which it silently keeps the first. In
such a file, and in one that names an array of tables after a version more
than once (`[[1.2.3]]`), the message reads `TOML whose versions incrmit cannot
place`. Fix the file, or set `parser = "text"`. A YAML file is refused the same
way, as `YAML whose versions incrmit cannot place`, when a key holds a few
dozen versions: the YAML reader limits a key to 1,024 characters, and the copy
`incrmit` reads to place versions widens each one.

**Configs written by an earlier version.** An entry for a `.json` or `.toml`
file now matches only whole string values, so an entry an older `discover`
wrote for a version that appeared only in a URL, a range, or a comment finds
nothing. The bump fails with `expected version not found` (exit `3`) before it
writes anything. None of those versions was the project's: run
`incrmit discover` again to regenerate the config, or set `parser = "text"` on
an entry to keep the old behavior for that file. `undo` reads each file with the
parser the bump used, which the state file records, so a bump made by an earlier
version is undone as text, the way it was made.

The same holds for a `.yaml`, `.yml`, or `.cff` file since YAML has been read
by its format: an entry for a version found only in a comment, a range, or a
longer string fails while planning, and so does an entry for a manifest of
several documents, saying to set `parser = "text"`. The likeliest surprise is
an image tag in a Kubernetes manifest; the recipe above keeps tracking it.

### Which occurrence: `key`

An entry names a path and a version, and without more a bump rewrites every
occurrence of that version in the file. In a manifest that is often more than
the project's version: a `package.json` at `1.2.3` that pins
`"left-pad": "1.2.3"` would bump the dependency too. `key` says which
occurrence is the project's, and an entry with one rewrites exactly the
occurrences it names and nothing else in the file:

```toml
[[files]]
  path = "package.json"
  version = "1.2.3"
  key = "version"            # not dependencies.left-pad

[[files]]
  path = "Cargo.toml"
  version = "0.4.0"
  key = "package.version"    # not anyhow = { version = "0.4.0" }

[[files]]
  path = "Chart.yaml"
  version = "1.4.2"
  key = "appVersion"         # not dependencies[0].version

[[files]]
  path = "Makefile"
  version = "1.2.3"
  key = "VERSION ?= "        # not .../tool-1.2.3.tar.gz
```

A key is read the way its file is (see [How a file is read](#how-a-file-is-read)):

- **In a JSON, TOML, or YAML file** it is a path from the top of the document,
  written the way TOML writes a dotted key: bare parts made of letters,
  digits, `_`, and `-`; any other part in double quotes, with TOML's escapes;
  and an array index as `[N]` after the part it indexes. So
  `key = "bin[0].version"` names the first `[[bin]]` table's `version`,
  `key = 'dependencies."@types/node"'` a key holding a slash, and
  `key = '"."'` the `.` key of a `.release-please-manifest.json`. The path is
  matched whole, never as a suffix: `version` is the top-level key alone, never
  `dependencies.version`. The value there must be a string holding the entry's
  version. In YAML it must be where the version is written: not an alias of
  it, nor a path the data has only through an alias or a merge key.
- **In any other file** it is the text right before the version on its line,
  with no pattern language: `key = "VERSION ?= "` names the version in
  `VERSION ?= 1.2.3` and not the one in `.../tool-1.2.3.tar.gz`. Matching is
  byte for byte, so case and whitespace count, and it names every occurrence
  that text comes before. A `v` prefix is part of the version, so the key for
  `TAG=v1.2.3` is `"TAG="`.

An entry with a key and no `version` starts from the version its key names.

A key that does not name a version the bump can rewrite fails while planning,
with exit `3` and nothing written, naming the file and the key:

```text
incrmit: package.json: key dependencies.version is not in the file
incrmit: package.json: key engines holds an object, not a version
incrmit: package.json: key version holds 1.2.3, not 1.2.4
incrmit: package.json: key name appears twice in the file, so it names no one value
incrmit: values.yaml: key pinned reads base through an alias, and incrmit rewrites a value where it is written; name base instead
incrmit: Makefile: key "VERSION = " is not followed by a version anywhere in the file
```

A key its parser cannot read is a config error naming the entry (exit `1`): an
empty key, a text key holding a line break, and anything that is not a path in a
JSON, TOML, or YAML file, such as `key = "VERSION ?= "` on a `package.json`. So
is a pair of entries that would both rewrite one occurrence, such as a keyed
entry and one without a key for the same version, or two keys naming the same
value; that one is found while planning, since it takes the file to tell:

```text
incrmit: config: files[0] (key version) and files[1] (no key) both claim the 1.2.3 on line 2 of package.json; remove one, or give each a key that names its own occurrence
```

Since a key is read by the entry's parser, it changes meaning when the parser
does, through `parser` or `discover`'s fallback to text. That fails rather than
rewrite another occurrence: a path read as text names nothing, since a version
never comes right after a word, and text read as a path is either not a path or
names a key the file does not have.

With `--file`, give the key as `--key` (see [Bumping](#bumping)). A bump keeps
each entry's `key` when it rewrites the config, and records it in the state
file, so `undo` reverts the same occurrences.

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

Matching rules (patterns are compared against the path **relative to the
config's folder** — the same path the config lists a file under — always using
forward slashes, and matching is **case-sensitive**):

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
- To match `*`, `?`, or `[` **literally**, wrap it in brackets: `v[*].txt`
  ignores a file named `v*.txt` but not `vX.txt`, and `[[]ab].txt` ignores
  `[ab].txt`. A backslash cannot escape a metacharacter, because backslashes in
  `ignore` are read as path separators (so a config written on Windows works
  everywhere).

Because patterns are relative to the config rather than to the folder being
scanned, a config means the same thing whatever [`--path`](#discovery-flags)
`discover` is run with: in a root `incrmit.toml`, `src/gen/` names `src/gen`
whether `discover` scans the whole project or only `src`, and `gen/**` never
names it. A scan outside the config's folder is matched with a leading `../`,
just as it is listed: a config in `release/` that scans `src` skips `src/gen`
with `../src/gen/`. A bare name such as `*.lock` matches at any depth either
way. The folder `--path` names is always scanned, even when a pattern (or a
built-in ignore) names it; patterns apply to what is inside it.

The `ignore` list is preserved when `discover` regenerates the config and when a
bump rewrites it, so hand-authored entries are never dropped. `discover` reads
the list from the config already present at its `--output` path; a file there
that is not a config is [refused](#discovery-flags), not read as an empty list.
Whenever `incrmit` writes a config that has no `ignore` list yet — either from
`discover` or when a bump rewrites the file — it includes a short description of
the option along with a **commented-out example**, so the feature is easy to
find and enable: just uncomment the line and edit the patterns.

## Bumping

Running `incrmit` with no command bumps the version in every configured file:

```bash
incrmit [flags]
```

### Flags

| Flag | Short | Description | Default |
| ---- | ----- | ----------- | ------- |
| `--config` | `-c` | Path to the TOML config file | `incrmit.toml` |
| `--file` | `-f` | Bump the version in one file (skips config) | *none* |
| `--key` | `-k` | With `--file`, the [key](#which-occurrence-key) that names the version | *none* |
| `--major` | `-M` | Bump the major version (resets minor and patch) | `false` |
| `--minor` | `-m` | Bump the minor version (resets patch) | `false` |
| `--patch` | `-p` | Bump the patch version | `true` |
| `--release` | `-r` | [Promote a prerelease](#prereleases-and-build-metadata) (`1.2.3-rc.1` -> `1.2.3`) | `false` |
| `--pre` | `-e` | [Start or advance a prerelease](#prereleases-and-build-metadata) (`1.2.3` -> `1.2.4-rc.1`) | *none* |
| `--max-file-size` | `-s` | [Refuse to read a target larger than this](#limiting-how-much-is-read) | *no limit* |
| `--wait` | `-w` | [Wait for another `incrmit` run](#concurrent-runs) instead of failing | `false` |
| `--dry-run` | `-d` | Print the new version without writing files | `false` |

When no `--major`, `--minor`, or `--patch` flag is given, a patch bump is
applied. If more than one component flag is supplied, the highest wins (major
> minor > patch).

Using `--file` bypasses the config entirely: only that file is updated and
`incrmit.toml` is not rewritten. A file holding several versions is ambiguous
on its own; `--key` names the one to bump, read as an entry's
[`key`](#which-occurrence-key) is, so
`incrmit --file package.json --key version` and
`incrmit --file Makefile --key 'VERSION ?= '` work without a config. `--key`
without `--file`, an empty one, and one that is not a path for a JSON, TOML, or
YAML file are usage errors (exit `2`).

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

Dry-run the change without writing it (see [Preview](#preview) to see all three
component bumps at once):

```bash
incrmit --dry-run
incrmit -d
```

### Prereleases and build metadata

A version token is `[v]MAJOR.MINOR.PATCH[-PRERELEASE][+BUILD]`, following
[semver 2.0.0](https://semver.org): `1.2.3`, `v1.2.3`, `1.2.3-rc.1`,
`1.2.3+build.7`, `v2.0.0-beta.1+exp.sha.5114f85`. The prerelease and build
sections are dot-separated identifiers of ASCII letters, digits, and hyphens; a
numeric prerelease identifier may not carry a leading zero (`1.2.3-rc.01` is
not a version).

Two spellings are refused that a reading of semver alone might allow, because
`incrmit` has to find the token again in the file after writing it. A numeric
component may not carry a leading zero (`1.02.3`), which would be written back
as `1.2.3` and no longer match the text on disk, and a token may not end in a
hyphen (`1.2.3+0-`), which the scanner reads back as `1.2.3+0`. Either one is
reported as "no semantic version found" rather than bumped to something the
file does not say.

Every number `incrmit` counts stops at 9223372036854775807, the largest signed
64-bit integer: each of the three components, and the counter at the end of a
prerelease. A bump that would go past it exits `3`, naming the file and the
component, and writes nothing. Adding one would wrap to a negative number,
which is not a version, so the file would be left holding a token no later
command could find. Only the number being incremented matters:
`1.2.9223372036854775807` has no next patch, but `--minor` still takes it to
`1.3.0`. `--pre rc` refuses a counter already past the limit
(`1.2.3-rc.99999999999999999999`) the same way, and `--pre beta` still starts a
new series there.

The whole token is matched and rewritten as a unit, so `1.2.3-rc.1` is never
confused with the `1.2.3` inside it — a config entry pinning one does not match
the other, even in the same file.

A `--major`, `--minor`, or `--patch` bump drops both sections, which is how
every other bump tool behaves. Build metadata describes one specific build and
is never carried forward:

```bash
incrmit --file VERSION
# 1.2.3-rc.1 -> 1.2.4
# 1.2.3+build.7 -> 1.2.4
```

Use `--release` to promote a prerelease to the release it names, leaving the
numbers alone:

```bash
incrmit --release
incrmit -r
# 1.2.3-rc.1 -> 1.2.3
```

Use `--pre <id>` to start or advance one. Off a release it bumps the patch
first (the preview cannot claim the version already published); within the same
series it advances the counter; in a different series it restarts on the same
numbers:

```bash
incrmit --pre rc
# 1.2.3 -> 1.2.4-rc.1
incrmit --pre rc
# 1.2.4-rc.1 -> 1.2.4-rc.2
incrmit --pre beta
# 1.2.4-rc.2 -> 1.2.4-beta.1
```

Naming a component alongside `--pre` opens a new release line instead:

```bash
incrmit --minor --pre rc
# 1.2.3 -> 1.3.0-rc.1
```

Combinations that cannot mean anything exit with code `2` and write nothing:
`--release` together with `--pre`, `--release` with a component flag,
`--release` on a version that has no prerelease, and a `--pre` value that is not
a valid identifier list.

#### The prerelease in `incrmit.toml`

A running prerelease is recorded in its own key, next to the version it
previews:

```toml
[[files]]
path = "install.sh"
version = "1.2.4"
prerelease = "rc.1"
```

`--pre` writes that key, `--release` and any component bump remove it again, so
the config always reads as the project's actual state. (Build metadata gets a
`build` key the same way.) Configs that spell the whole token in `version`
(`version = "1.2.4-rc.1"`) still work and are moved to this form on the next
write.

Keeping the prerelease separate is not cosmetic — it is what lets `incrmit`
find a prerelease inside a filename. A hyphen is a legal prerelease character,
so `incrmit-1.2.3-linux-amd64.tar.gz` is, to semver, the version `1.2.3` with
the prerelease `linux-amd64.tar.gz`, and rewriting that as one token would
reduce the whole line to `incrmit-1.2.4`. The config says which suffix is real,
so exactly that much is rewritten:

```bash
# with prerelease = "rc.1" recorded:
incrmit --release
# app-1.2.3-rc.1.zip -> app-1.2.3.zip

# with no prerelease recorded:
incrmit
# incrmit-1.2.3-linux-amd64.tar.gz -> incrmit-1.2.4-linux-amd64.tar.gz
```

Without a config to consult — `--file` mode, or an unpinned entry — `incrmit`
falls back to context: a version welded into a longer hyphen-joined word keeps
only its numbers, while a version that stands on its own (after a quote, an `=`,
whitespace, or at the start of a line) keeps its full token. That is always
safe, but it cannot promote a prerelease inside a filename, so
`app-1.2.3-rc.1.zip` bumps to `app-1.2.4-rc.1.zip` there. Run `incrmit
discover` to pin the versions, and use `--dry-run` when in doubt.

## Discovery

Rather than writing the config by hand, run the `discover` command to scan
the project for files that contain a semantic version string and generate an
`incrmit.toml` for you.

Discovery walks the directory tree and inspects the contents of every text file,
recording every `[v]MAJOR.MINOR.PATCH[-PRERELEASE][+BUILD]` token it finds in
each — the whole token, so a prerelease target survives a regeneration of the
config exactly as written in the file. It is not limited to specific file names:
`VERSION`, source files, plain text, and anything else is scanned. A file's name
decides only how it is read: a `.json`, `.toml`, `.yaml`, `.yml`, or `.cff` file
by its format, where only a whole string value counts, and anything else as text
(see [How a file is read](#how-a-file-is-read)).

A file may contain a version in more than one place. In a JSON, TOML, or YAML
file, discovery records **one `[[files]]` entry per occurrence, with its
[`key`](#which-occurrence-key)**, so each entry rewrites exactly one value, can
be deleted on its own, and says in the config what it is. In a manifest it
knows, it records the project's own version alone:

| Manifest | The project's version is at |
| -------- | --------------------------- |
| `package.json`, `composer.json`, `deno.json`, `jsr.json` | `version` |
| `Cargo.toml` | `package.version` or `workspace.package.version` |
| `pyproject.toml` | `project.version` or `tool.poetry.version` |
| `pubspec.yaml`, `snapcraft.yaml`, `galaxy.yml`, `CITATION.cff` | `version` |
| `Chart.yaml` | `version` and `appVersion` |

Every other version in such a manifest is a dependency's, and `--dry-run` lists
it as skipped with the key that would track it. A known manifest with none of
those keys holding a version, such as a private monorepo root, gets no entry,
and `--dry-run` says so rather than writing an entry per dependency.

**`Chart.yaml`.** A Helm chart has two versions of its own: `version` is the
chart's and `appVersion` the application's. Both belong to the project and
many charts keep them equal, so discovery writes an entry for each; if you
version them separately, delete the entry for the one a bump should leave
alone.

**`pubspec.yaml`.** Flutter reads the part after `+` in `version: 2.3.1+45` as
the build number. A bump drops build metadata (see
[Prereleases and build metadata](#prereleases-and-build-metadata)), so a patch
bump writes `version: 2.3.2`, without a build number, as `incrmit` always has.
Add the build number yourself if your release needs one.

In any other file, discovery records **one `[[files]]` entry per distinct
version**, with no key: if all the occurrences agree, that is a single entry
(and a bump rewrites every copy); if a file holds several *different* versions,
it gets one entry per version, all sharing the same `path`. Each such version is
bumped independently, and all of its occurrences in the file are updated in a
single pass. The text in front of a version is often prose, and rewording the
sentence would break a key, so add one by hand where you want it.

Running `discover` again over a config written before keys existed writes them
in, keeping its `ignore` list.

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

Binary files (any file holding a `NUL` byte, which includes UTF-16 text) and
common noise directories (`.git`, `node_modules`, `vendor`, build outputs) are
skipped, as is the config file itself (`incrmit.toml` and
the `--output` path), so it is never listed as a target. Any folders and files
matched by the config's [`ignore` list](#ignoring-folders-and-files) are skipped
too.

Discovery also stays strictly inside the directory it was pointed at, and only
reads ordinary files:

- **Symlinks are never followed.** A link to a file or folder outside the tree is
  not scanned and not listed, so scanning an unfamiliar checkout cannot read (or
  later copy in) a file the scan was never aimed at. A file inside the tree is
  still found under its real path.
- **Only regular files are read.** Named pipes, devices, and sockets are skipped
  rather than opened, since reading one could block indefinitely or stream
  without end.
- **Files larger than 32 MiB are skipped.** Version strings live in small text
  files, so this bounds how much a scan reads no matter what the tree contains.
  Use [`--max-file-size`](#limiting-how-much-is-read) to raise, lower, or remove
  the cap. Files listed in the config by hand are not subject to it.
- **A name that is not valid UTF-8 is skipped, with a warning.** Linux allows
  any bytes in a file name, but a TOML config can only hold UTF-8, so listing
  such a file would produce a config that nothing could load.

```bash
incrmit discover
```

This writes an `incrmit.toml` listing each discovered file along with the
version value(s) that were found. A text file with two differing versions
appears once per distinct version:

```toml
# incrmit.toml (generated by `incrmit discover`)

[[files]]
  path = "VERSION"
  version = "1.2.3"

[[files]]
  path = "package.json"
  version = "1.2.3"
  key = "version"

[[files]]
  path = "notes.md"
  version = "1.2.3"

[[files]]
  path = "notes.md"
  version = "2.0.0"
```

Every `path` is written relative to the folder the config goes in, which is
where every command resolves it, so [`--path` and `--output`](#discovery-flags)
may name different folders. Scanning `src` for a config in the project root
lists `src/VERSION`, and a config written to `release/` lists the same file as
`../src/VERSION`:

```bash
incrmit discover --path src --output release/incrmit.toml
```

```text
Wrote release/incrmit.toml with 1 file(s):
  ../src/VERSION: 1.2.3
```

`--dry-run` prints each occurrence it finds, with its line number and the text
of the line for context, rather than writing the config. In a JSON, TOML, or
YAML file, each occurrence's key comes after its line number, and a manifest's
dependencies are listed as skipped:

```text
Discovered 3 file(s) under . (no config written):
  Cargo.toml:
    L3 package.version: version = "0.4.0" # was 0.3.9
    L8 dependencies.anyhow.version: skipped, not the project's own version; to track it, add an entry with key = "dependencies.anyhow.version"
  notes.md:
    L1: release 1.2.3
    L5: legacy 2.0.0
  packages/package.json: no entry, since key version holds no version
    L4 devDependencies.typescript: skipped, not the project's own version; to track it, add an entry with key = "devDependencies.typescript"
```

The summary of a real run names each entry's key after its path
(`Cargo.toml package.version: 0.4.0`). A YAML occurrence that aliases read
lists them at the end of its line (`L1 base: base: &v 1.2.3 (aliased at
pinned)`), since a bump of it changes them too.

A file read as text when its name says JSON, TOML, or YAML is headed by the
reason (`tsconfig.json: not JSON (line 2: ...), scanned as text`,
`deploy.yaml: 2 YAML documents, scanned as text`), and the summary of a real
run marks its entries `(not JSON, scanned as text)` or `(2 YAML documents,
scanned as text)`.

Line numbers count `\n`, `\r\n`, and a lone `\r` as line breaks, so they match
what an editor shows whatever the file's line endings. A long line — a minified
bundle is one line from start to finish — is clipped to 80 bytes on each side of
the version, with `...` marking each cut.

Whatever `incrmit` prints from the files it reads — names and context lines, in
every command, on stdout and stderr — is escaped before it reaches the
terminal, so a file in the tree cannot use escape sequences to clear the
screen, retitle the window, plant a link, or write to the clipboard. A name
holding a control character, a bidirectional override, or a byte that is not
UTF-8 is shown Go-quoted (`"invoice\u202efdp.exe"`); an ordinary name, spaces
and non-ASCII included, is shown exactly as it is. In a context line such
characters appear as escapes in place (`\x1b[2J`), and tabs are kept. Only the
display changes: the file is read, written, and recorded in `incrmit.toml`
under its real name.

When the config has an `ignore` list, `--dry-run` notes the applied rules on a
`(ignoring: …)` line and, like a normal run, never lists any skipped path as a
finding. When the config goes somewhere other than the folder being scanned,
the paths it lists are not relative to that folder, so a line above says what
they are relative to:

```text
Discovered 1 file(s) under src (no config written):
  (paths relative to release/incrmit.toml)
  ../src/VERSION:
    L1: 1.2.3
```

### Discovery flags

| Flag | Short | Description | Default |
| ---- | ----- | ----------- | ------- |
| `--path` | `-P` | Root directory to scan | `.` |
| `--output` | `-o` | Path to write the generated config file | `incrmit.toml` |
| `--max-file-size` | `-s` | Skip files larger than this size | `32MiB` |
| `--wait` | `-w` | [Wait for another `incrmit` run](#concurrent-runs) instead of failing | `false` |
| `--dry-run` | `-d` | Print discovered files without writing config | `false` |

```bash
# Scan a specific directory and preview the results (listed as src/...)
incrmit discover --path ./src --dry-run

# Write the config to a custom location (paths listed from release/, as ../...)
incrmit discover --output release/incrmit.toml
```

`--output` may name a file that does not exist yet, an empty file, or an
`incrmit` config, which is regenerated with its `ignore` list kept. A config
here means TOML that sets only the keys `incrmit` uses (`ignore`, and `files`
with its `path`, `version`, `prerelease`, `build`, and `parser`), so every config
`incrmit` writes qualifies, and so does a hand-written one. Anything else is
refused with exit `1` before the scan, and left exactly as it was:

```text
incrmit: NOTES.md exists and is not an incrmit config (line 3 is not TOML); choose another --output or remove the file
```

That covers a README or other prose, a target such as `VERSION`, a manifest
such as `package.json`, another tool's TOML such as `pyproject.toml` (`it sets
"project", which incrmit does not use`), a file of nothing but comments, and a
named pipe or a device. `--dry-run` refuses the same way, so it never shows a
plan the real run will not carry out. There is no `--force`: if the file really
should be replaced, remove it and run `discover` again.

### Limiting how much is read

`--max-file-size` (`-s`) sets the largest file `incrmit` will read. A value is
either a plain byte count or a number with a unit suffix — `512KB`, `32MiB`,
`2G` — and `0` means no limit. The bare `K`, `M`, and `G` suffixes are binary
(1024-based), while `KB`, `MB`, and `GB` are decimal (1000-based).

The two commands treat the cap differently, because they face different input:

- **`discover` skips** a file over the cap and carries on, since it is scanning
  a whole tree and a huge file is simply not a plausible version target. The
  default cap is `32MiB`.
- **A bump fails** on a target over the cap, naming the file and the limit, and
  writes nothing at all (the size is checked while planning, before any file is
  touched). Targets are listed by hand, so a bump has **no cap by default** —
  pass `--max-file-size` to opt in.

```bash
# Scan a tree with big generated files, keeping the scan light
incrmit discover --max-file-size 256KB

# Scan everything, however large
incrmit discover -s 0

# Refuse to bump a target that has grown unexpectedly large
incrmit --max-file-size 1MiB
```

## Preview

`incrmit preview` answers "what would the next version be?" for every file at
once. It prints each configured file's current version next to what a
`--patch`, `--minor`, and `--major` bump would write, so all three outcomes are
visible without running a `--dry-run` per component:

```bash
incrmit preview
```

```text
PATH                             CURRENT  PATCH    MINOR   MAJOR
Makefile                         0.1.15   0.1.16   0.2.0   1.0.0
README.md                        0.1.15   0.1.16   0.2.0   1.0.0
README.md                        v0.1.15  v0.1.16  v0.2.0  v1.0.0
internal/buildinfo/buildinfo.go  0.1.15   0.1.16   0.2.0   1.0.0
```

`preview` is **read-only**: it writes no target file, no config, and no bump
history. A few details:

- **The `v` prefix is preserved** in every projection, so a `v1.2.3` entry
  previews as `v1.2.4` / `v1.3.0` / `v2.0.0` — exactly what a bump would write.
- **A prerelease or build section is dropped** by all three component bumps, so
  `1.2.3-rc.1` previews as `1.2.4` / `1.3.0` / `2.0.0`. Use `--release` and
  `--pre` on a real bump to work within a prerelease.
- **A file listed once per version it contains** (see
  [Discovery](#discovery)) gets one row per version; an identical `path` and
  `version` is never printed twice.
- **An entry's [`key`](#which-occurrence-key) gets a `KEY` column**, so you see
  which value a bump will move. A text key is quoted, since its spaces count. A
  config without keys has no such column.

  ```text
  PATH          KEY              CURRENT  PATCH  MINOR  MAJOR
  package.json  version          1.2.3    1.2.4  1.3.0  2.0.0
  Cargo.toml    package.version  1.2.3    1.2.4  1.3.0  2.0.0
  Makefile      "VERSION ?= "    1.2.3    1.2.4  1.3.0  2.0.0
  ```

- **A bump past the limit shows `n/a`**: a component already at
  9223372036854775807 cannot be bumped (see
  [Prereleases and build metadata](#prereleases-and-build-metadata)), so its
  column reads `n/a`, with a note under the table. The other columns still show
  what a bump would write, and `preview` still exits `0`.
- **`--file` previews a single target** without a config, with the same
  semantics as a `--file` bump, `--key` included.
- **`--max-file-size` applies**, exactly as it does to a bump: no cap by
  default, and an oversized target is reported rather than read.

### Out-of-sync entries

When the config holds versions that disagree, the rows that differ from the
version most entries hold are marked `*` and explained under the table, so a
file left behind by a partial bump is visible at a glance:

```text
PATH       CURRENT  PATCH   MINOR   MAJOR
VERSION    1.2.3    1.2.4   1.3.0   2.0.0
README.md  v1.2.3   v1.2.4  v1.3.0  v2.0.0
stale.txt  1.0.0    1.0.1   1.1.0   2.0.0   *

* differs from 1.2.3, the version most entries hold
```

Only semver precedence counts as a difference: `v1.2.3`, `1.2.3`, and
`1.2.3+build.7` name the same release and are never marked. A file that holds a
deliberately different version — a vendored dependency, say — is marked too,
since the tool cannot tell it apart from drift.

### Preview flags

| Flag | Short | Description | Default |
| ---- | ----- | ----------- | ------- |
| `--config` | `-c` | Path to the TOML config file | `incrmit.toml` |
| `--file` | `-f` | Preview one file (skips config) | *none* |
| `--key` | `-k` | With `--file`, the [key](#which-occurrence-key) that names the version | *none* |
| `--max-file-size` | `-s` | [Refuse to read a target larger than this](#limiting-how-much-is-read) | *no limit* |

Exit codes follow the rest of the tool: a missing config exits `1` with a hint
to run `incrmit discover`, bad flags exit `2`, and a version that cannot be
parsed (or a `--file` target with no version in it) exits `3` naming the file.

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

To make undo possible, each bump records a small entry (the files it touches
and their `old`/`new` versions, plus a timestamp) in a state file named
`.incrmit.state.toml`, kept next to `incrmit.toml`. The entry is written before
any file is changed, so even a bump that stops partway can be undone. This file
is **local working state** — it is not meant to be committed, so add it to your
`.gitignore`:

```gitignore
.incrmit.state.toml
```

A few details worth knowing:

- **Repeated undos walk back through history.** Each `undo` reverts one bump and
  removes its entry, so undoing twice reverts the two most recent bumps. The
  journal retains the last 20 bumps (older ones are dropped), or fewer in a
  project with thousands of files, so the state file stays under its 16 MiB
  cap.
- **A state file that cannot be read stops a bump before it writes.** The bump
  fails with nothing changed rather than leave a bump `undo` cannot see, and
  `--dry-run` reports the same failure. The file is safe to delete.
- **Your edits are never clobbered.** If a file was changed after the bump so it
  no longer contains the version the bump wrote, `undo` refuses that revert and
  writes nothing, exiting with an error instead — naming the file that diverged.
  This covers a second `incrmit` that bumped past the recorded version as well
  as an edit of your own, so an older version is never written back over newer
  work.
- **Undo acts on the project it is given.** The state file records each path
  as `incrmit.toml` lists it, and `undo` resolves it against the config it is
  pointed at (`-c`, or `incrmit.toml` in the current directory), so it works
  from a subdirectory with `-c ../incrmit.toml`, and a project that was copied
  or moved undoes its own files. A state file written by an earlier version
  also records absolute paths (`fs` and `config`); those are ignored, so an old
  entry is undone in the project where it now sits.
- **Undo reverts only what the config lists.** Every file in the entry must
  still be listed in `incrmit.toml` at the version the bump wrote. If one is
  not, because the config was edited since the bump or the state file names a
  file of its own, `undo` refuses, naming it
  (`VERSION: not listed in incrmit.toml at 1.2.4, ...`), writes nothing, and
  exits `1`. Unlike the config, a state file is not trusted input: a repository
  can commit one, and this check is what keeps it from reaching any file the
  config could not already bump.
- **A bump is all or nothing.** If a file cannot be written (a read-only
  directory, a full disk), the bump puts back the files it had already
  written and exits `1` having changed nothing:

  ```text
  incrmit: writing c/VERSION: permission denied
  incrmit: rolled back a/VERSION, b/VERSION; no file was changed
  ```

  If it cannot put one of them back either, it names the files left changed
  and tells you to run `incrmit undo` once the problem is fixed. That undo,
  like one after a bump that was killed partway, reverts the files the bump
  reached and reports the rest as `already at` their old version. Until then a
  new bump refuses to run, because the config no longer matches the files, and
  points at `undo`. `undo` is protected the same way: one that fails puts the
  files back, and one that stops partway can simply be run again.
- **Undo reads each file as the bump did.** The state file records the parser
  the bump read each file with, and `undo` uses it rather than the config's
  `parser`, so the same occurrences are put back. An entry written before the
  parser was recorded is read as text, which is how that bump was made. A file
  that parser cannot read any more is refused (exit `3`), with nothing written.
  It records each entry's [`key`](#which-occurrence-key) the same way, and
  `undo` reverts exactly the occurrences that key names, even if the config's
  key has been edited or removed since; a keyed value changed since the bump
  counts as the file being changed.
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
| `--wait` | `-w` | [Wait for another `incrmit` run](#concurrent-runs) instead of failing | `false` |
| `--dry-run` | `-d` | Preview the revert without writing | `false` |

## Version

Print the version of the `incrmit` tool itself (not a target file's version)
with the `version` subcommand or the `--version` / `-version` / `-v` flag:

```bash
incrmit version
incrmit --version
incrmit -v
# incrmit X.Y.Z
```

The version is baked into the binary and can be overridden at build time
(for example to stamp a git tag).

## Help

Run `incrmit help` for a top-level overview that lists every command **and
its flags** (the bump and discover flags are shown inline so you can see them
without drilling into each command), or `incrmit help <command>` for details on
a specific one. Each command also responds to `-h` / `--help` with the same
text:

```bash
incrmit help              # overview of all commands
incrmit help discover     # help for the discover command
incrmit help preview      # help for the preview command
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
  incrmit preview [flags]    show each file's version and its next patch/minor/major
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

## Concurrent runs

**One `incrmit` writes to a project at a time.** A bump is a read-modify-write
over shared state — it reads `incrmit.toml` and every target, then writes all of
them back along with the undo journal — so two runs at once (a `make -j` rule, a
CI matrix, a file watcher, two terminals) would otherwise interleave and erase
each other: the tree gets bumped twice while the config records one version and
the journal one entry, and the erased bump can never be undone.

To prevent that, every writing command (`bump`, `discover`, `undo`) takes an
exclusive lock on the project before it reads anything, and holds it until it is
done. The lock is a file named `.incrmit.lock` next to `incrmit.toml` (with
`--file`, next to the file being bumped), so separate projects never wait on
each other. It exists only while a run is writing: the run removes it when it
finishes. You will only find one left by a run that crashed, which the next run
removes, or on Windows, where it is kept between runs. Add it to your
`.gitignore` alongside the state file for those cases:

```gitignore
.incrmit.lock
```

The file is safe to delete. There is no such thing as a stale lock to clear by
hand: the lock is held by the operating system, which releases it when the
process exits for **any** reason — including a crash or a `kill -9`.

**A second run fails fast.** A bump takes milliseconds, so another one arriving
mid-run is usually a mistake rather than a queue, and a tool that blocked
silently would turn a CI misconfiguration into a hung job instead of a failed
one:

```console
$ incrmit
incrmit: another incrmit run is already writing in .
Wait for it to finish and run again, or pass --wait to queue behind it.
$ echo $?
1
```

A run refused this way writes nothing at all — no file, no config, no journal
entry. If you really are serializing work, `--wait` (`-w`) queues behind the
first run instead of failing.

**Read-only commands take no lock.** `preview`, and any command with
`--dry-run`, neither block nor are blocked, so inspecting a project is always
possible. The trade-off is that they may observe a bump in progress and show a
partly updated tree.

**Where locking is unavailable** — some NFS mounts, a few CI overlay
filesystems — `incrmit` prints a warning and carries on unlocked rather than
refusing to run, since a tool that cannot bump at all is worse than one that
cannot detect a second run.

**The lock file is never opened through a symbolic link.** A repository can
commit `.incrmit.lock` as a link to a file somewhere else, and following it
would write the lock into that file. If the path holds a link, a directory, a
named pipe, or anything else that is not a regular file, `incrmit` leaves it as
it is, warns, and carries on unlocked:

```console
$ incrmit
incrmit: warning: cannot lock .incrmit.lock: is a symbolic link, not a regular file
incrmit: warning: continuing unlocked; a concurrent incrmit run could erase this one's work.
```

Remove it to lock again. A regular file already at that name is used as the
lock but never rewritten: the explanatory note goes only into a lock file that
is empty. Nor is it removed: a run removes only a lock file `incrmit` made, one
holding its note or nothing, with no other name.

## Exit codes

`incrmit` returns predictable exit codes so it can be used reliably in
scripts and CI pipelines:

| Code | Meaning                                                                                                                                           |
| ---- | ------------------------------------------------------------------------------------------------------------------------------------------------- |
| 0    | Success.                                                                                                                                          |
| 1    | Generic runtime error (e.g. missing config, filesystem error).                                                                                    |
| 2    | Invalid arguments or flags.                                                                                                                       |
| 3    | No version found, an ambiguous/unparseable version, a key that names no version to bump, a file its parser cannot read, or one too large to bump. |

A run that finds the project already held by another `incrmit` also exits `1`,
having written nothing — see [Concurrent runs](#concurrent-runs).
