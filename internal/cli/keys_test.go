package cli

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/sasmaq/incrmit/internal/config"
	"github.com/sasmaq/incrmit/internal/files"
	"github.com/sasmaq/incrmit/internal/history"
)

// mustKey reads key as the parser file's name selects reads it.
func mustKey(t *testing.T, file, key string) *files.Key {
	t.Helper()
	k, err := files.ParseKey(files.ParserFor(file), key)
	if err != nil {
		t.Fatal(err)
	}
	return &k
}

// The reproduction files' pins, as the files hold them: Milestone 40's
// reproduction moves every one of them, and a config with keys moves none.
var reproPins = map[string][]string{
	"package.json":   {`"left-pad": "1.2.3"`, `"lodash": "4.17.21"`},
	"Cargo.toml":     {`serde = "1.0.200"`, `anyhow = { version = "0.4.0" }`},
	"pyproject.toml": {`schema-version = "2.1.0"`},
}

// assertPinsKept checks that no dependency pin in the reproduction files
// under dir has moved.
func assertPinsKept(t *testing.T, dir string) {
	t.Helper()
	for name, pins := range reproPins {
		got := readFile(t, filepath.Join(dir, name))
		for _, pin := range pins {
			if !strings.Contains(got, pin) {
				t.Errorf("%s no longer holds %s:\n%s", name, pin, got)
			}
		}
	}
}

// Milestone 40's reproduction. The config Milestone 39's discover wrote for the
// reproduction files names a path and a version and nothing more, so a bump
// rewrites every candidate holding the version: package.json's left-pad moves
// with the project's 1.2.3, anyhow's inline { version = "0.4.0" } with
// Cargo.toml's, and the entries for 4.17.21 and 1.0.200 bump two dependencies
// outright, all with exit 0. In a Makefile, a tool download moves with the
// project's VERSION. The goldens are that bump's output and files; omitting
// `key` keeps this behavior.
func TestReproductionPinsMoveWithoutAKey(t *testing.T) {
	golden := func(name string) string { return goldenPath(t, name) } // before runMain changes directory
	cfgGolden, bumpGolden := golden("repro_unkeyed_incrmit.toml.golden"), golden("repro_unkeyed_bump_minor.golden")
	fileGolden := map[string]string{}
	for _, name := range reproFiles {
		fileGolden[name] = golden("repro_unkeyed_minor_" + name + ".golden")
	}
	dir := reproProject(t)
	old, err := os.ReadFile(cfgGolden)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, config.DefaultPath), old, 0o644); err != nil {
		t.Fatal(err)
	}

	code, stdout, stderr := runMain(t, dir, "--minor")
	if code != ExitOK {
		t.Fatalf("bump: exit %d, stderr %q", code, stderr)
	}
	checkGolden(t, bumpGolden, stdout)
	for _, name := range reproFiles {
		checkGolden(t, fileGolden[name], readFile(t, filepath.Join(dir, name)))
	}
	for name, moved := range map[string][]string{
		"package.json": {`"left-pad": "1.3.0"`, `"lodash": "4.18.0"`},
		"Cargo.toml":   {`serde = "1.1.0"`, `anyhow = { version = "0.5.0" }`},
	} {
		got := readFile(t, filepath.Join(dir, name))
		for _, pin := range moved {
			if !strings.Contains(got, pin) {
				t.Errorf("%s: the reproduction no longer moves the pin to %s:\n%s", name, pin, got)
			}
		}
	}

	t.Run("Makefile", func(t *testing.T) {
		dir := reproProject(t, "Makefile")
		body := "[[files]]\npath = \"Makefile\"\nversion = \"1.2.3\"\n"
		if err := os.WriteFile(filepath.Join(dir, config.DefaultPath), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		before := readFile(t, filepath.Join(dir, "Makefile"))
		if code, _, stderr := runMain(t, dir); code != ExitOK {
			t.Fatalf("bump: exit %d, stderr %q", code, stderr)
		}
		if got := readFile(t, filepath.Join(dir, "Makefile")); got != strings.ReplaceAll(before, "1.2.3", "1.2.4") {
			t.Errorf("Makefile =\n%s\nwant every 1.2.3 moved, the tool's download included", got)
		}
	})
}

// fixtureBumps are the reproduction files, each with the key that names its
// project's version, the version, and the line holding it before and after a
// patch bump. Everything else holding the version is a pin.
var fixtureBumps = []struct {
	file, key, version, line, bumped string
}{
	{"package.json", "version", "1.2.3", `"version": "1.2.3"`, `"version": "1.2.4"`},
	{"Cargo.toml", "package.version", "0.4.0", `version = "0.4.0" # was 0.3.9`, `version = "0.4.1" # was 0.3.9`},
	{"pyproject.toml", "project.version", "2.1.0", "version = \"2.1.0\"\ndescription", "version = \"2.1.1\"\ndescription"},
	{"Makefile", "VERSION ?= ", "1.2.3", "VERSION ?= 1.2.3", "VERSION ?= 1.2.4"},
}

// Each fixture bumped with and without its key. Without, every candidate
// holding the version moves, the pins included; with, the one occurrence the
// key names moves and the rest of the file is byte for byte as it was. The
// journal records the key, and undo puts back exactly what the bump wrote.
func TestKeyStopsPinsMoving(t *testing.T) {
	for _, fx := range fixtureBumps {
		t.Run(fx.file, func(t *testing.T) {
			src := readFile(t, filepath.Join(reproDir, fx.file))
			keyed := strings.Replace(src, fx.line, fx.bumped, 1)
			if keyed == src {
				t.Fatalf("%s does not hold %q", fx.file, fx.line)
			}

			for _, tt := range []struct {
				name, key, want string
			}{
				{"without a key", "", ""},
				{"with its key", fx.key, keyed},
			} {
				t.Run(tt.name, func(t *testing.T) {
					body := "[[files]]\npath = \"" + fx.file + "\"\nversion = \"" + fx.version + "\"\n"
					if tt.key != "" {
						body += "key = " + tomlString(tt.key) + "\n"
					}
					dir := project(t, body, map[string]string{fx.file: src})
					code, stdout, stderr := runMain(t, dir)
					if code != ExitOK {
						t.Fatalf("exit %d, stderr %q", code, stderr)
					}
					got := readFile(t, filepath.Join(dir, fx.file))
					if tt.key == "" {
						if strings.Count(got, fx.version) >= strings.Count(src, fx.version)-1 {
							t.Errorf("without a key only the project's version moved:\n%s", got)
						}
						return
					}
					if got != tt.want {
						t.Errorf("%s =\n%s\nwant\n%s", fx.file, got, tt.want)
					}
					if want := "  " + fx.file + keyLabel(mustKey(t, fx.file, fx.key)) + ": " + fx.version + " -> "; !strings.Contains(stdout, want) {
						t.Errorf("summary %q does not name the key (%q)", stdout, want)
					}
					cfg, err := config.Load(filepath.Join(dir, config.DefaultPath))
					if err != nil {
						t.Fatal(err)
					}
					if k := cfg.Files[0].Key; k == nil || *k != fx.key {
						t.Errorf("the bump's config rewrite dropped the key: %+v", cfg.Files[0])
					}
					if c := latestEntry(t, dir).Changes[0]; c.Key != fx.key {
						t.Errorf("the journal records key %q, want %q", c.Key, fx.key)
					}
					if code, _, stderr := runMain(t, dir, "undo"); code != ExitOK {
						t.Fatalf("undo: exit %d, stderr %q", code, stderr)
					}
					if got := readFile(t, filepath.Join(dir, fx.file)); got != src {
						t.Errorf("undo left %s as\n%s", fx.file, got)
					}
					cfg, err = config.Load(filepath.Join(dir, config.DefaultPath))
					if err != nil {
						t.Fatal(err)
					}
					if f := cfg.Files[0]; f.Version != fx.version || f.Key == nil || *f.Key != fx.key {
						t.Errorf("undo's config rewrite = %+v, want %s at key %q", f, fx.version, fx.key)
					}
				})
			}
		})
	}
}

// A key that names no candidate holding the entry's version is an error that
// names the file and the key, with exit 3, raised while planning, so neither
// that file nor the one listed ahead of it is written; --dry-run and preview
// fail the same way.
func TestKeyErrorsWriteNothing(t *testing.T) {
	pkg := "{\n  \"version\": \"1.2.3\",\n  \"count\": 3,\n  \"range\": \"^1.2.3\",\n" +
		"  \"esc\": \"1\\u002e2.3\",\n  \"dup\": \"1.2.3\",\n  \"dup\": \"1.2.3\",\n" +
		"  \"deps\": { \"version\": \"1.2.3\" }\n}\n"
	makefile := "VERSION ?= 1.2.3\nversion ?= 1.2.3\nTOOL_VERSION := 2.0.0\n"
	tests := []struct {
		name, file, version, key, want string
	}{
		{"absent", "package.json", "1.2.3", "dependencies.version", "package.json: key dependencies.version is not in the file"},
		{"top level only", "package.json", "1.2.3", "deps.version.x", "package.json: key deps.version.x is not in the file"},
		{"a number", "package.json", "1.2.3", "count", "package.json: key count holds a number; a version is a string"},
		{"an object", "package.json", "1.2.3", "deps", "package.json: key deps holds an object, not a version"},
		{"a range", "package.json", "1.2.3", "range", `package.json: key range holds "^1.2.3", which is not a version`},
		{"another version", "package.json", "1.2.4", "version", "package.json: key version holds 1.2.3, not 1.2.4"},
		{"an escape", "package.json", "1.2.3", "esc", `package.json: key esc holds "1.2.3" only once its escapes are decoded`},
		{"a duplicate key", "package.json", "1.2.3", "dup", "package.json: key dup appears twice in the file, so it names no one value"},
		{"text no version follows", "Makefile", "1.2.3", "VERSION = ", `Makefile: key "VERSION = " is not followed by a version anywhere in the file`},
		// The key is the bytes right before the version, wherever they
		// start: "VERSION := " ends TOOL_VERSION's line too.
		{"text a longer name", "Makefile", "1.2.3", "VERSION := ", `Makefile: key "VERSION := " is followed by 2.0.0, not 1.2.3`},
		{"text another version", "Makefile", "1.2.3", "TOOL_VERSION := ", `Makefile: key "TOOL_VERSION := " is followed by 2.0.0, not 1.2.3`},
		{"text case counts", "Makefile", "1.2.3", "Version ?= ", `Makefile: key "Version ?= " is not followed by a version anywhere in the file`},
		{"text whitespace counts", "Makefile", "1.2.3", "VERSION ?=", `Makefile: key "VERSION ?=" is not followed by a version anywhere in the file`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := "[[files]]\npath = \"VERSION\"\nversion = \"1.2.3\"\n" +
				"[[files]]\npath = \"" + tt.file + "\"\nversion = \"" + tt.version + "\"\nkey = " + tomlString(tt.key) + "\n"
			dir := project(t, body, map[string]string{"VERSION": "1.2.3\n", "package.json": pkg, "Makefile": makefile})
			snap := snapshotProject(t, dir)
			for _, args := range [][]string{nil, {"--dry-run"}, {"preview"}} {
				code, stdout, stderr := runMain(t, dir, args...)
				if code != ExitNoVersion || stderr != "incrmit: "+tt.want+"\n" && !strings.HasPrefix(stderr, "incrmit: "+tt.want) {
					t.Errorf("%v: exit %d, stderr %q, want exit %d and %q", args, code, stderr, ExitNoVersion, tt.want)
				}
				if stdout != "" {
					t.Errorf("%v: stdout %q", args, stdout)
				}
			}
			assertSnapshot(t, dir, snap, "after the refused bump")
			if _, err := os.Stat(stateFile(dir)); !os.IsNotExist(err) {
				t.Errorf("a journal was written: %v", err)
			}
		})
	}
}

// A key that cannot be read by its entry's parser is a config error (exit 1),
// raised as the config loads: an empty key, a text key holding a line break,
// and anything a JSON or TOML file's key is not a path for.
func TestInvalidKeyIsAConfigError(t *testing.T) {
	tests := []struct {
		name, file, parser, key, want string
	}{
		{"empty", "Makefile", "", `""`, `files[1] (Makefile): key is empty; for a text file, a key is the text right before the version on its line, such as "VERSION ?= "`},
		{"empty path", "package.json", "", `""`, "files[1] (package.json): key is empty; for a JSON file, a key is a path such as package.version"},
		{"a line break", "Makefile", "", `"VERSION\n?= "`, `files[1] (Makefile): key "VERSION\n?= " holds a line break`},
		{"not a path", "package.json", "", `"version "`, `files[1] (package.json): key "version " is not a key path (' ' cannot follow "version"); for a JSON file, a key is a path such as package.version`},
		{"an unclosed quote", "Cargo.toml", "", `'package."version'`, `files[1] (Cargo.toml): key "package.\"version" is not a key path (the quote at byte 9 is not closed)`},
		{"a bad index", "Cargo.toml", "", `"bin[01].version"`, `files[1] (Cargo.toml): key "bin[01].version" is not a key path ([01] is not an array index, which is a number such as [0])`},
		// A text key read as a path, through a parser override.
		{"a text key read as TOML", "Makefile", "toml", `"VERSION ?= "`, `files[1] (Makefile): key "VERSION ?= " is not a key path (' ' cannot follow "VERSION"); for a TOML file`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := "[[files]]\npath = \"VERSION\"\n[[files]]\npath = \"" + tt.file + "\"\nversion = \"1.2.3\"\nkey = " + tt.key + "\n"
			if tt.parser != "" {
				body += "parser = \"" + tt.parser + "\"\n"
			}
			dir := project(t, body, map[string]string{"VERSION": "1.2.3\n", tt.file: "VERSION ?= 1.2.3\n"})
			saveEntry(t, dir, history.Change{Path: "VERSION", Old: "1.2.2", New: "1.2.3"})
			snap := snapshotProject(t, dir)
			for _, args := range [][]string{nil, {"--dry-run"}, {"preview"}, {"undo"}} {
				code, _, stderr := runMain(t, dir, args...)
				if code != ExitError || !strings.Contains(stderr, "incrmit: config: "+tt.want) {
					t.Errorf("%v: exit %d, stderr %q, want %q", args, code, stderr, tt.want)
				}
			}
			assertSnapshot(t, dir, snap, "after the refused runs")
		})
	}
}

// A key changes meaning when its entry's parser does, and both directions fail
// rather than rewrite another occurrence. A path read as text names nothing:
// the bytes before a version are never "version", since the scanner wants a
// word boundary there. A text key read as a path is not a path (above), or,
// when it happens to be one, names a key the file does not have.
func TestKeyWhoseParserChangedFails(t *testing.T) {
	pkg := "{\n  \"version\": \"1.2.3\",\n  \"dependencies\": { \"left-pad\": \"1.2.3\" }\n}\n"
	tests := []struct {
		name, file, body, parser, key, want string
		code                                int
	}{
		{"a path read as text", "package.json", pkg, "text", "version", `package.json: key "version" is not followed by a version anywhere in the file`, ExitNoVersion},
		{"a dotted path read as text", "package.json", pkg, "text", "dependencies.left-pad", `package.json: key "dependencies.left-pad" is not followed by a version anywhere in the file`, ExitNoVersion},
		{"a text key that is a path, read as TOML", "settings", "VERSION=1.2.3\n", "toml", "VERSION", "settings: not TOML (line 1: ", ExitNoVersion},
		{"a text key that is a path, read as JSON", "settings.json", "{\"v\": \"1.2.3\"}", "", "v=", `config: files[0] (settings.json): key "v=" is not a key path`, ExitError},
		{"a text key naming no key", "Cargo.toml", "[package]\nversion = \"1.2.3\"\n", "", "version", "Cargo.toml: key version is not in the file", ExitNoVersion},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := "[[files]]\npath = \"" + tt.file + "\"\nversion = \"1.2.3\"\nkey = " + tomlString(tt.key) + "\n"
			if tt.parser != "" {
				body += "parser = \"" + tt.parser + "\"\n"
			}
			dir := project(t, body, map[string]string{tt.file: tt.body})
			snap := snapshotProject(t, dir)
			code, _, stderr := runMain(t, dir)
			if code != tt.code || !strings.Contains(stderr, tt.want) {
				t.Errorf("exit %d, stderr %q, want exit %d and %q", code, stderr, tt.code, tt.want)
			}
			assertSnapshot(t, dir, snap, "after the refused bump")
		})
	}
}

// Two entries that would rewrite one occurrence are a config error naming
// both, raised while planning: an entry without a key and a keyed one for the
// same version, or two keys that name the same value however each is
// written. Entries whose keys name different occurrences of one version are
// fine, which is what discover writes for a file it does not know.
func TestEntriesClaimingOneOccurrence(t *testing.T) {
	pkg := "{\n  \"version\": \"1.2.3\",\n  \"dependencies\": { \"left-pad\": \"1.2.3\" }\n}\n"
	entry := func(version, key string) string {
		e := "[[files]]\npath = \"package.json\"\nversion = \"" + version + "\"\n"
		if key != "" {
			e += "key = " + tomlString(key) + "\n"
		}
		return e
	}
	tests := []struct {
		name, body, want string
	}{
		{"a key and no key", entry("1.2.3", "version") + entry("1.2.3", ""),
			"incrmit: config: files[0] (key version) and files[1] (no key) both claim the 1.2.3 on line 2 of package.json; remove one, or give each a key that names its own occurrence\n"},
		{"one key written two ways", entry("1.2.3", "version") + entry("1.2.3", `"version"`),
			"incrmit: config: files[0] (key version) and files[1] (key version) both claim the 1.2.3 on line 2 of package.json; "},
		{"no key, then a key", entry("1.2.3", "") + entry("1.2.3", "dependencies.left-pad"),
			"incrmit: config: files[0] (no key) and files[1] (key dependencies.left-pad) both claim the 1.2.3 on line 3 of package.json; "},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := project(t, tt.body, map[string]string{"package.json": pkg})
			snap := snapshotProject(t, dir)
			for _, args := range [][]string{nil, {"--dry-run"}, {"preview"}} {
				code, _, stderr := runMain(t, dir, args...)
				if code != ExitError || !strings.HasPrefix(stderr, tt.want) {
					t.Errorf("%v: exit %d, stderr %q, want it to start %q", args, code, stderr, tt.want)
				}
			}
			assertSnapshot(t, dir, snap, "after the refused bump")
		})
	}

	t.Run("one key written twice is a duplicate", func(t *testing.T) {
		dir := project(t, entry("1.2.3", "version")+entry("1.2.3", "version"), map[string]string{"package.json": pkg})
		code, _, stderr := runMain(t, dir)
		if code != ExitError || !strings.Contains(stderr, `config: duplicate path "package.json" with version "1.2.3" and key "version"`) {
			t.Errorf("exit %d, stderr %q", code, stderr)
		}
	})

	t.Run("two keys, two occurrences", func(t *testing.T) {
		dir := project(t, entry("1.2.3", "version")+entry("1.2.3", "dependencies.left-pad"), map[string]string{"package.json": pkg})
		code, stdout, stderr := runMain(t, dir)
		if code != ExitOK {
			t.Fatalf("exit %d, stderr %q", code, stderr)
		}
		if got := readFile(t, filepath.Join(dir, "package.json")); got != strings.ReplaceAll(pkg, "1.2.3", "1.2.4") {
			t.Errorf("package.json = %q", got)
		}
		if want := "  package.json version: 1.2.3 -> 1.2.4\n  package.json dependencies.left-pad: 1.2.3 -> 1.2.4\n"; !strings.HasSuffix(stdout, want) {
			t.Errorf("summary = %q, want it to end %q", stdout, want)
		}
	})
}

// An entry with a key and no version starts from the version the key names,
// as one without either starts from the file's one version.
func TestKeyWithoutAVersion(t *testing.T) {
	pkg := "{\n  \"version\": \"1.2.3\",\n  \"dependencies\": { \"lodash\": \"4.17.21\" }\n}\n"
	dir := project(t, "[[files]]\npath = \"package.json\"\nkey = \"version\"\n", map[string]string{"package.json": pkg})
	code, stdout, stderr := runMain(t, dir, "--minor")
	if code != ExitOK || !strings.Contains(stdout, "package.json version: 1.2.3 -> 1.3.0") {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, stdout, stderr)
	}
	if got := readFile(t, filepath.Join(dir, "package.json")); got != strings.Replace(pkg, "1.2.3", "1.3.0", 1) {
		t.Errorf("package.json = %q", got)
	}

	// A text key that names two versions is as ambiguous as a file holding
	// two.
	dir = project(t, "[[files]]\npath = \"Makefile\"\nkey = \"V=\"\n", map[string]string{"Makefile": "V=1.2.3\nV=2.0.0\n"})
	code, _, stderr = runMain(t, dir)
	if code != ExitNoVersion || !strings.Contains(stderr, "Makefile: files: ambiguous version: found multiple distinct versions [1.2.3 2.0.0]") {
		t.Errorf("exit %d, stderr %q", code, stderr)
	}
}

// A text key matches the bytes right before a version, whatever line it is
// on, and only on that line: a key cannot hold a line break, so the bytes it
// names end where the version starts. Every occurrence it names moves.
func TestTextKeyMatching(t *testing.T) {
	body := "VERSION ?= 1.2.3\n# VERSION ?=\n1.2.3\nIMAGE := app:1.2.3\nalso VERSION ?= 1.2.3 here\n"
	dir := project(t, "[[files]]\npath = \"Makefile\"\nversion = \"1.2.3\"\nkey = \"VERSION ?= \"\n", map[string]string{"Makefile": body})
	if code, _, stderr := runMain(t, dir); code != ExitOK {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
	want := "VERSION ?= 1.2.4\n# VERSION ?=\n1.2.3\nIMAGE := app:1.2.3\nalso VERSION ?= 1.2.4 here\n"
	if got := readFile(t, filepath.Join(dir, "Makefile")); got != want {
		t.Errorf("Makefile =\n%s\nwant\n%s", got, want)
	}
}

// undo reverts the occurrences the journal's key names, whatever the config
// says by then. Here the config lost its key after the bump, and left-pad was
// moved to the version the bump wrote: read without the key, the revert would
// move it back too.
func TestUndoUsesTheJournalsKey(t *testing.T) {
	bumped := "{\n  \"version\": \"1.2.4\",\n  \"dependencies\": { \"left-pad\": \"1.2.4\" }\n}\n"
	dir := project(t, "[[files]]\npath = \"package.json\"\nversion = \"1.2.4\"\n", map[string]string{"package.json": bumped})
	saveEntry(t, dir, history.Change{Path: "package.json", Old: "1.2.3", New: "1.2.4", Parser: "json", Key: "version"})
	code, stdout, stderr := runMain(t, dir, "undo")
	if code != ExitOK || !strings.Contains(stdout, "  package.json version: 1.2.4 -> 1.2.3\n") {
		t.Fatalf("undo: exit %d, stdout %q, stderr %q", code, stdout, stderr)
	}
	if got := readFile(t, filepath.Join(dir, "package.json")); got != strings.Replace(bumped, "1.2.4", "1.2.3", 1) {
		t.Errorf("package.json = %q", got)
	}

	t.Run("the keyed value changed since", func(t *testing.T) {
		dir := project(t, "[[files]]\npath = \"package.json\"\nversion = \"1.2.4\"\nkey = \"version\"\n",
			map[string]string{"package.json": strings.Replace(bumped, "\"version\": \"1.2.4\"", "\"version\": \"1.2.5\"", 1)})
		saveEntry(t, dir, history.Change{Path: "package.json", Old: "1.2.3", New: "1.2.4", Parser: "json", Key: "version"})
		snap := snapshotProject(t, dir)
		code, _, stderr := runMain(t, dir, "undo")
		if code != ExitError || !strings.Contains(stderr, "package.json: expected version 1.2.4 from the last bump is no longer present; the file was changed since (refusing to undo)") {
			t.Errorf("exit %d, stderr %q", code, stderr)
		}
		assertSnapshot(t, dir, snap, "after the refused undo")
	})

	t.Run("a key the journal's parser cannot read", func(t *testing.T) {
		dir := project(t, "[[files]]\npath = \"package.json\"\nversion = \"1.2.4\"\n", map[string]string{"package.json": bumped})
		saveEntry(t, dir, history.Change{Path: "package.json", Old: "1.2.3", New: "1.2.4", Parser: "json", Key: "VERSION ?= "})
		snap := snapshotProject(t, dir)
		code, _, stderr := runMain(t, dir, "undo")
		if code != ExitError || !strings.Contains(stderr, `package.json: the journal records a key it cannot use: key "VERSION ?= " is not a key path`) {
			t.Errorf("exit %d, stderr %q", code, stderr)
		}
		assertSnapshot(t, dir, snap, "after the refused undo")
	})

	// Two keyed entries a patch bump took to one version from two: undo puts
	// each back where it started, matching the config's entries by key.
	t.Run("two keys bumped to one version", func(t *testing.T) {
		pkg := "{\n  \"version\": \"1.2.3\",\n  \"next\": \"1.2.3-rc.1\"\n}\n"
		body := "[[files]]\npath = \"package.json\"\nversion = \"1.2.3\"\nkey = \"version\"\n" +
			"[[files]]\npath = \"package.json\"\nversion = \"1.2.3\"\nprerelease = \"rc.1\"\nkey = \"next\"\n"
		dir := project(t, body, map[string]string{"package.json": pkg})
		if code, _, stderr := runMain(t, dir); code != ExitOK {
			t.Fatalf("bump: exit %d, stderr %q", code, stderr)
		}
		if got := readFile(t, filepath.Join(dir, "package.json")); got != "{\n  \"version\": \"1.2.4\",\n  \"next\": \"1.2.4\"\n}\n" {
			t.Errorf("package.json = %q", got)
		}
		if code, _, stderr := runMain(t, dir, "undo"); code != ExitOK {
			t.Fatalf("undo: exit %d, stderr %q", code, stderr)
		}
		if got := readFile(t, filepath.Join(dir, "package.json")); got != pkg {
			t.Errorf("undo left package.json as %q", got)
		}
		cfg, err := config.Load(filepath.Join(dir, config.DefaultPath))
		if err != nil {
			t.Fatal(err)
		}
		if got := []string{cfg.Files[0].Token(), cfg.Files[0].KeyText(), cfg.Files[1].Token(), cfg.Files[1].KeyText()}; !slices.Equal(got, []string{"1.2.3", "version", "1.2.3-rc.1", "next"}) {
			t.Errorf("undo's config holds %v", got)
		}
	})
}

// --file takes --key, so a file that is ambiguous on its own (a manifest
// holding dependencies, a Makefile downloading a tool) bumps without a config.
// The key is read as the file is: by its extension, or as text when the file
// falls back to text.
func TestFileModeKey(t *testing.T) {
	pkg := "{\n  \"version\": \"1.2.3\",\n  \"dependencies\": { \"lodash\": \"4.17.21\", \"left-pad\": \"1.2.3\" }\n}\n"
	makefile := "VERSION ?= 1.2.3\n\ttool-1.2.3.tar.gz\n"
	tsconfig := "{\n  // c\n  \"version\": \"1.2.3\", \"x\": \"4.5.6\"\n}\n"
	dir := project(t, "", map[string]string{"package.json": pkg, "Makefile": makefile, "tsconfig.json": tsconfig})

	code, _, stderr := runMain(t, dir, "--file", "package.json")
	if code != ExitNoVersion || !strings.Contains(stderr, "ambiguous version") {
		t.Errorf("without --key: exit %d, stderr %q", code, stderr)
	}
	code, stdout, stderr := runMain(t, dir, "preview", "--file", "package.json", "--key", "version")
	if code != ExitOK || stdout != "PATH          KEY      CURRENT  PATCH  MINOR  MAJOR\npackage.json  version  1.2.3    1.2.4  1.3.0  2.0.0\n" {
		t.Errorf("preview: exit %d, stdout %q, stderr %q", code, stdout, stderr)
	}
	for _, tt := range []struct {
		args       []string
		file, want string
	}{
		{[]string{"--file", "package.json", "--key", "version"}, "package.json", strings.Replace(pkg, "1.2.3", "1.2.4", 1)},
		{[]string{"-f", "Makefile", "-k", "VERSION ?= "}, "Makefile", strings.Replace(makefile, "1.2.3", "1.2.4", 1)},
		{[]string{"--file", "tsconfig.json", "--key", `"version": "`}, "tsconfig.json", strings.Replace(tsconfig, "1.2.3", "1.2.4", 1)},
	} {
		code, stdout, stderr := runMain(t, dir, tt.args...)
		if code != ExitOK {
			t.Errorf("%v: exit %d, stderr %q", tt.args, code, stderr)
			continue
		}
		if got := readFile(t, filepath.Join(dir, tt.file)); got != tt.want {
			t.Errorf("%v: %s = %q, want %q", tt.args, tt.file, got, tt.want)
		}
		if !strings.Contains(stdout, tt.file+" ") {
			t.Errorf("%v: summary %q does not name the key", tt.args, stdout)
		}
	}

	snap := snapshotProject(t, dir)
	for _, tt := range []struct {
		args []string
		code int
		want string
	}{
		{[]string{"--key", "version"}, ExitUsage, "incrmit: --key needs --file; in config mode, set key on the [[files]] entry\n"},
		{[]string{"preview", "--key", "version"}, ExitUsage, "incrmit: --key needs --file"},
		{[]string{"--file", "package.json", "--key", ""}, ExitUsage, "incrmit: --key is empty; give the key that names the version in package.json\n"},
		{[]string{"--file", "package.json", "--key", "VERSION ?= "}, ExitUsage, `incrmit: --key: key "VERSION ?= " is not a key path (' ' cannot follow "VERSION"); for a JSON file, a key is a path such as package.version` + "\n"},
		{[]string{"--file", "Makefile", "--key", "A\nB"}, ExitUsage, `incrmit: --key: key "A\nB" holds a line break`},
		{[]string{"--file", "package.json", "--key", "dependencies.react"}, ExitNoVersion, "incrmit: package.json: key dependencies.react is not in the file\n"},
		// tsconfig.json falls back to text, so the path is read as text and
		// names nothing.
		{[]string{"--file", "tsconfig.json", "--key", "version"}, ExitNoVersion, `incrmit: tsconfig.json: key "version" is not followed by a version anywhere in the file`},
	} {
		code, _, stderr := runMain(t, dir, tt.args...)
		if code != tt.code || !strings.Contains(stderr, tt.want) {
			t.Errorf("%v: exit %d, stderr %q, want exit %d and %q", tt.args, code, stderr, tt.code, tt.want)
		}
	}
	assertSnapshot(t, dir, snap, "after the refused runs")
}

// preview shows each entry's key in a column of its own, so the value a bump
// will move is plain; a text key is quoted, since its spaces count. A config
// without keys gets no such column.
func TestPreviewShowsKeys(t *testing.T) {
	dir := reproProject(t, "Makefile")
	body := "[[files]]\npath = \"package.json\"\nversion = \"1.2.3\"\nkey = \"version\"\n" +
		"[[files]]\npath = \"package.json\"\nversion = \"1.2.3\"\nkey = '\"dependencies\".\"left-pad\"'\n" +
		"[[files]]\npath = \"Makefile\"\nversion = \"1.2.3\"\nkey = \"VERSION ?= \"\n" +
		"[[files]]\npath = \"Cargo.toml\"\nversion = \"0.4.0\"\n"
	if err := os.WriteFile(filepath.Join(dir, config.DefaultPath), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr := runMain(t, dir, "preview")
	if code != ExitOK {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
	want := "PATH          KEY                    CURRENT  PATCH  MINOR  MAJOR  \n" +
		"package.json  version                1.2.3    1.2.4  1.3.0  2.0.0\n" +
		"package.json  dependencies.left-pad  1.2.3    1.2.4  1.3.0  2.0.0\n" +
		"Makefile      \"VERSION ?= \"          1.2.3    1.2.4  1.3.0  2.0.0\n" +
		"Cargo.toml                           0.4.0    0.4.1  0.5.0  1.0.0  *\n\n" +
		"* differs from 1.2.3, the version most entries hold\n"
	if stdout != strings.Replace(want, "MAJOR  \n", "MAJOR\n", 1) {
		t.Errorf("preview =\n%s\nwant\n%s", stdout, want)
	}
}

// manifestTree holds one of each manifest discover knows, a private monorepo
// root with no version of its own, and a JSON file it does not know.
var manifestTree = map[string]string{
	"package.json":       "{\n  \"private\": true,\n  \"devDependencies\": { \"typescript\": \"5.4.5\" }\n}\n",
	"web/package.json":   "{\n  \"version\": \"1.2.3\",\n  \"dependencies\": { \"@types/node\": \"20.1.0\" }\n}\n",
	"php/composer.json":  "{\n  \"version\": \"1.2.3\",\n  \"require\": { \"php\": \"8.3.0\" }\n}\n",
	"deno/deno.json":     "{\n  \"version\": \"1.2.3\"\n}\n",
	"deno/jsr.json":      "{\n  \"version\": \"1.2.3\",\n  \"exports\": \"./mod.ts\"\n}\n",
	"Cargo.toml":         "[workspace.package]\nversion = \"1.2.3\"\n\n[workspace.dependencies]\nserde = \"1.0.200\"\n",
	"crate/Cargo.toml":   "[package]\nversion.workspace = true\n\n[dependencies]\nanyhow = \"1.0.86\"\n",
	"py/pyproject.toml":  "[tool.poetry]\nversion = \"1.2.3\"\n\n[tool.poetry.dependencies]\nclick = \"8.1.7\"\n",
	"appsettings.json":   "{\n  \"App\": { \"Version\": \"1.2.3\" },\n  \"Sdk\": \"8.0.100\"\n}\n",
	"release/VERSION.md": "Release 1.2.3\n",
}

// discover writes one entry per known manifest, keyed to the project's
// version, and lists each dependency as skipped with the key that would track
// it; a known manifest with no version of its own gets no entry, and the dry
// run says so. A file it does not know gets an entry per candidate.
func TestDiscoverKnownManifestsOutput(t *testing.T) {
	dir := project(t, "", manifestTree)
	code, stdout, stderr := runMain(t, dir, "discover", "--dry-run")
	if code != ExitOK || stderr != "" {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
	want := `Discovered 10 file(s) under . (no config written):
  Cargo.toml:
    L2 workspace.package.version: version = "1.2.3"
    L5 workspace.dependencies.serde: skipped, not the project's own version; to track it, add an entry with key = "workspace.dependencies.serde"
  appsettings.json:
    L2 App.Version: "App": { "Version": "1.2.3" },
    L3 Sdk: "Sdk": "8.0.100"
  crate/Cargo.toml: no entry, since neither key package.version nor workspace.package.version holds a version
    L5 dependencies.anyhow: skipped, not the project's own version; to track it, add an entry with key = "dependencies.anyhow"
  deno/deno.json:
    L2 version: "version": "1.2.3"
  deno/jsr.json:
    L2 version: "version": "1.2.3",
  package.json: no entry, since key version holds no version
    L3 devDependencies.typescript: skipped, not the project's own version; to track it, add an entry with key = "devDependencies.typescript"
  php/composer.json:
    L2 version: "version": "1.2.3",
    L3 require.php: skipped, not the project's own version; to track it, add an entry with key = "require.php"
  py/pyproject.toml:
    L2 tool.poetry.version: version = "1.2.3"
    L5 tool.poetry.dependencies.click: skipped, not the project's own version; to track it, add an entry with key = "tool.poetry.dependencies.click"
  release/VERSION.md:
    L1: Release 1.2.3
  web/package.json:
    L2 version: "version": "1.2.3",
    L3 dependencies."@types/node": skipped, not the project's own version; to track it, add an entry with key = 'dependencies."@types/node"'
`
	if stdout != want {
		t.Errorf("dry run =\n%s\nwant\n%s", stdout, want)
	}

	code, stdout, stderr = runMain(t, dir, "discover")
	if code != ExitOK || stderr != "" {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
	want = `Wrote incrmit.toml with 8 file(s):
  Cargo.toml workspace.package.version: 1.2.3
  appsettings.json App.Version: 1.2.3
  appsettings.json Sdk: 8.0.100
  deno/deno.json version: 1.2.3
  deno/jsr.json version: 1.2.3
  php/composer.json version: 1.2.3
  py/pyproject.toml tool.poetry.version: 1.2.3
  release/VERSION.md: 1.2.3
  web/package.json version: 1.2.3
`
	if stdout != want {
		t.Errorf("summary =\n%s\nwant\n%s", stdout, want)
	}

	// The written config bumps every project's version and no dependency.
	before := map[string]string{}
	for name := range manifestTree {
		before[name] = readFile(t, filepath.Join(dir, name))
	}
	if code, _, stderr := runMain(t, dir, "--minor"); code != ExitOK {
		t.Fatalf("bump: exit %d, stderr %q", code, stderr)
	}
	for name, body := range before {
		want := body
		if name != "package.json" && name != "crate/Cargo.toml" {
			want = strings.Replace(body, "1.2.3", "1.3.0", 1)
		}
		if name == "appsettings.json" {
			want = strings.Replace(want, "8.0.100", "8.1.0", 1)
		}
		if got := readFile(t, filepath.Join(dir, name)); got != want {
			t.Errorf("%s =\n%s\nwant\n%s", name, got, want)
		}
	}
}

// Running discover again over a config an earlier version wrote replaces its
// entries with keyed ones, keeping its ignore list, so an existing project
// picks up the keys by regenerating.
func TestDiscoverAgainWritesKeys(t *testing.T) {
	dir := reproProject(t)
	old := readFile(t, goldenPath(t, "repro_unkeyed_incrmit.toml.golden"))
	old = strings.Replace(old, "# ignore = [\"testdata/\", \"*.lock\", \"docs/**\"]\n", "ignore = [\"docs/\"]\n", 1)
	if err := os.WriteFile(filepath.Join(dir, config.DefaultPath), []byte(old), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, _, stderr := runMain(t, dir, "discover"); code != ExitOK {
		t.Fatalf("discover: exit %d, stderr %q", code, stderr)
	}
	cfg, err := config.Load(filepath.Join(dir, config.DefaultPath))
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, f := range cfg.Files {
		got = append(got, f.Path+" "+f.KeyText()+"="+f.Token())
	}
	if want := []string{"Cargo.toml package.version=0.4.0", "package.json version=1.2.3", "pyproject.toml project.version=2.1.0"}; !slices.Equal(got, want) || !slices.Equal(cfg.Ignore, []string{"docs/"}) {
		t.Errorf("regenerated config holds %v, ignore %v; want %v, ignore [docs/]", got, cfg.Ignore, want)
	}
}
