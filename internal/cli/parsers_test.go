package cli

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/sasmaq/incrmit/internal/config"
	"github.com/sasmaq/incrmit/internal/history"
)

// reproFiles are the files Milestone 39 was reproduced with, kept in
// internal/files/testdata/repro so the parser tests there and the tests here
// (and Milestone 40's) share them: a package.json with a $schema URL, two
// ranges, and a dependency pinned at the project's version; a Cargo.toml whose
// version line ends in a comment holding the previous version; and a
// pyproject.toml with the version in its description, a URL, a comment, and
// dependency specifiers.
var reproFiles = []string{"package.json", "Cargo.toml", "pyproject.toml"}

// reproDir holds the reproduction files, resolved before any test changes
// directory (runMain does).
var reproDir, _ = filepath.Abs(filepath.Join("..", "files", "testdata", "repro"))

// reproProject copies the reproduction files into a new project directory,
// and any of extra, also from internal/files/testdata/repro.
func reproProject(t *testing.T, extra ...string) string {
	t.Helper()
	dir := t.TempDir()
	for _, name := range append(slices.Clone(reproFiles), extra...) {
		b, err := os.ReadFile(filepath.Join(reproDir, name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// The reproduction, end to end. Read as text, discover listed eight entries
// for package.json and Cargo.toml and a minor bump moved all of them: the
// schema URL, both ranges, and the TOML comment included, with exit 0. Read by
// format, those are no longer candidates. The dependency pins still are,
// since each is a whole string value, but these are manifests discover knows
// (Milestone 40): it writes an entry for each one's own version, keyed to
// where it is, and lists the pins as skipped, so the bump moves the project's
// version alone. TestReproductionPinsMoveWithoutAKey bumps the config the
// discover of Milestone 39 wrote instead.
func TestReproductionReadsManifestsByFormat(t *testing.T) {
	golden := func(name string) string { return goldenPath(t, name) } // before runMain changes directory
	dryRunGolden, configGolden, bumpGolden := golden("repro_discover_dry_run.golden"), golden("repro_incrmit.toml.golden"), golden("repro_bump_minor.golden")
	fileGolden := map[string]string{}
	for _, name := range reproFiles {
		fileGolden[name] = golden("repro_minor_" + name + ".golden")
	}
	dir := reproProject(t)
	before := map[string]string{}
	for _, name := range reproFiles {
		before[name] = readFile(t, filepath.Join(dir, name))
	}

	code, stdout, stderr := runMain(t, dir, "discover", "--dry-run")
	if code != ExitOK || stderr != "" {
		t.Fatalf("discover --dry-run: exit %d, stderr %q", code, stderr)
	}
	// Seventeen candidates as text; eight, each with its key, read by format.
	checkGolden(t, dryRunGolden, stdout)

	if code, _, stderr = runMain(t, dir, "discover"); code != ExitOK {
		t.Fatalf("discover: exit %d, stderr %q", code, stderr)
	}
	checkGolden(t, configGolden, readFile(t, filepath.Join(dir, config.DefaultPath)))

	code, stdout, stderr = runMain(t, dir, "--minor")
	if code != ExitOK {
		t.Fatalf("bump: exit %d, stderr %q", code, stderr)
	}
	checkGolden(t, bumpGolden, stdout)
	for _, name := range reproFiles {
		checkGolden(t, fileGolden[name], readFile(t, filepath.Join(dir, name)))
	}
	pkg := readFile(t, filepath.Join(dir, "package.json"))
	for _, kept := range []string{"/schemas/2.0.0/", `"^18.3.1"`, `">=20.0.0"`} {
		if !strings.Contains(pkg, kept) {
			t.Errorf("package.json lost %s:\n%s", kept, pkg)
		}
	}
	if cargo := readFile(t, filepath.Join(dir, "Cargo.toml")); !strings.Contains(cargo, `version = "0.5.0" # was 0.3.9`) {
		t.Errorf("Cargo.toml's comment moved:\n%s", cargo)
	}
	assertPinsKept(t, dir)

	// The journal records the parser each file was read with, and undo puts
	// back exactly the bytes the bump started from.
	for _, c := range latestEntry(t, dir).Changes {
		if want := map[string]string{"package.json": "json", "Cargo.toml": "toml", "pyproject.toml": "toml"}[c.Path]; c.Parser != want {
			t.Errorf("journal records %s read as %q, want %q", c.Path, c.Parser, want)
		}
	}
	if code, _, stderr = runMain(t, dir, "undo"); code != ExitOK {
		t.Fatalf("undo: exit %d, stderr %q", code, stderr)
	}
	for _, name := range reproFiles {
		if got := readFile(t, filepath.Join(dir, name)); got != before[name] {
			t.Errorf("undo left %s as\n%s\nwant\n%s", name, got, before[name])
		}
	}
}

// discover scans a .json file that is not JSON as text, says why in the dry
// run, and records parser = "text" so a bump reads the file the way discover
// did. The bump keeps the key when it rewrites the config.
func TestDiscoverFallsBackAndRecordsTextParser(t *testing.T) {
	tsconfig := "{\n  // compiler options\n  \"version\": \"1.2.3\"\n}\n"
	dir := project(t, "", map[string]string{"tsconfig.json": tsconfig, "VERSION": "1.2.3\n"})

	code, stdout, stderr := runMain(t, dir, "discover", "--dry-run")
	if code != ExitOK {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
	if want := "  tsconfig.json: not JSON (line 2: invalid character '/' looking for beginning of object key string), scanned as text\n    L3: \"version\": \"1.2.3\"\n"; !strings.Contains(stdout, want) {
		t.Errorf("dry run does not say why tsconfig.json was scanned as text:\n%s", stdout)
	}

	code, stdout, stderr = runMain(t, dir, "discover")
	if code != ExitOK {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
	if !strings.Contains(stdout, "  tsconfig.json: 1.2.3 (not JSON, scanned as text)\n") || !strings.Contains(stdout, "  VERSION: 1.2.3\n") {
		t.Errorf("summary = %q", stdout)
	}
	cfg := readFile(t, filepath.Join(dir, config.DefaultPath))
	if strings.Count(cfg, "parser") != 1 || !strings.Contains(cfg, "path = \"tsconfig.json\"\n  version = \"1.2.3\"\n  parser = \"text\"\n") {
		t.Errorf("config does not record parser = \"text\" for tsconfig.json alone:\n%s", cfg)
	}

	if code, _, stderr = runMain(t, dir); code != ExitOK {
		t.Fatalf("bump: exit %d, stderr %q", code, stderr)
	}
	if got := readFile(t, filepath.Join(dir, "tsconfig.json")); got != strings.Replace(tsconfig, "1.2.3", "1.2.4", 1) {
		t.Errorf("tsconfig.json = %q", got)
	}
	cfg = readFile(t, filepath.Join(dir, config.DefaultPath))
	if !strings.Contains(cfg, "path = \"tsconfig.json\"\n  version = \"1.2.4\"\n  parser = \"text\"\n") {
		t.Errorf("the bump dropped tsconfig.json's parser:\n%s", cfg)
	}
}

// TOML whose versions the probe cannot place is TOML all the same, so the
// summary does not call it "not TOML"; the dry run gives the reason.
func TestDiscoverFallsBackFromTheProbe(t *testing.T) {
	dir := project(t, "", map[string]string{"Cargo.toml": "1.0.0 = \"x\"\n\"1\" = \"y\"\nversion = \"1.2.3\"\n"})
	code, stdout, stderr := runMain(t, dir, "discover", "--dry-run")
	if code != ExitOK || !strings.Contains(stdout, "  Cargo.toml: TOML whose versions incrmit cannot place (masking them changes what the file holds, which a key defined twice can cause), scanned as text\n") {
		t.Errorf("dry run: exit %d, stdout %q, stderr %q", code, stdout, stderr)
	}
	code, stdout, stderr = runMain(t, dir, "discover")
	if code != ExitOK || !strings.Contains(stdout, "  Cargo.toml: 1.2.3 (scanned as text)\n") {
		t.Errorf("discover: exit %d, stdout %q, stderr %q", code, stdout, stderr)
	}
}

// A bump never falls back: discover recorded how it read the file, so an entry
// whose parser cannot read its file fails in planning, naming the file and
// the line and saying how to read it as text, with nothing written.
func TestBumpRefusesFileItsParserCannotRead(t *testing.T) {
	body := "[[files]]\npath = \"VERSION\"\nversion = \"1.2.3\"\n" +
		"[[files]]\npath = \"package.json\"\nversion = \"1.2.3\"\n" +
		"[[files]]\npath = \"Cargo.toml\"\nversion = \"1.2.3\"\n"
	tests := map[string]struct {
		pkg, cargo, want string
	}{
		"JSON with a comment": {"{\n  \"version\": \"1.2.3\" // now\n}\n", "version = \"1.2.3\"\n",
			"incrmit: package.json: not JSON (line 2: invalid character '/' after object key:value pair); to read it as text, set parser = \"text\" on its [[files]] entry\n"},
		"TOML that is not TOML": {"{\"version\": \"1.2.3\"}", "version = 1.2.3\n",
			"incrmit: Cargo.toml: not TOML (line 1: "},
		// The decoder keeps the first of the two definitions of "1" and
		// drops the second, which the specification forbids; masked, the two
		// no longer collide, so the probe cannot be trusted.
		"TOML the probe refuses": {"{\"version\": \"1.2.3\"}", "1.0.0 = \"x\"\n\"1\" = \"y\"\nversion = \"1.2.3\"\n",
			"incrmit: Cargo.toml: TOML whose versions incrmit cannot place ("},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			dir := project(t, body, map[string]string{"VERSION": "1.2.3\n", "package.json": tt.pkg, "Cargo.toml": tt.cargo})
			snap := snapshotProject(t, dir)
			for _, args := range [][]string{{"--dry-run"}, nil} {
				code, _, stderr := runMain(t, dir, args...)
				if code != ExitNoVersion {
					t.Errorf("%v: exit %d, want %d; stderr %q", args, code, ExitNoVersion, stderr)
				}
				if !strings.HasPrefix(stderr, tt.want) || !strings.Contains(stderr, "set parser = \"text\"") {
					t.Errorf("%v: stderr = %q, want it to start %q", args, stderr, tt.want)
				}
			}
			assertSnapshot(t, dir, snap, "after the refused bump")
		})
	}
}

// The parser key overrides the extension in both directions: text for a
// .json file restores the scanner's behavior, and toml reads TOML under a
// name with no extension.
func TestParserKeyOverridesExtension(t *testing.T) {
	pkg := "{\n  \"$schema\": \"https://x.test/1.2.3/schema.json\",\n  \"version\": \"1.2.3\"\n}\n"
	pipfile := "[tool]\nversion = \"1.2.3\"\nhomepage = \"https://x.test/1.2.3/\"\n# 1.2.3\n"
	tests := []struct {
		name, file, body, parser, want string
	}{
		{"json by extension", "package.json", pkg, "",
			"{\n  \"$schema\": \"https://x.test/1.2.3/schema.json\",\n  \"version\": \"1.2.4\"\n}\n"},
		{"json read as text", "package.json", pkg, "text",
			"{\n  \"$schema\": \"https://x.test/1.2.4/schema.json\",\n  \"version\": \"1.2.4\"\n}\n"},
		{"no extension is text", "Pipfile", pipfile, "",
			"[tool]\nversion = \"1.2.4\"\nhomepage = \"https://x.test/1.2.4/\"\n# 1.2.4\n"},
		{"no extension read as toml", "Pipfile", pipfile, "toml",
			"[tool]\nversion = \"1.2.4\"\nhomepage = \"https://x.test/1.2.3/\"\n# 1.2.3\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := "[[files]]\npath = \"" + tt.file + "\"\nversion = \"1.2.3\"\n"
			if tt.parser != "" {
				body += "parser = \"" + tt.parser + "\"\n"
			}
			dir := project(t, body, map[string]string{tt.file: tt.body})
			if code, _, stderr := runMain(t, dir); code != ExitOK {
				t.Fatalf("exit %d, stderr %q", code, stderr)
			}
			if got := readFile(t, filepath.Join(dir, tt.file)); got != tt.want {
				t.Errorf("%s =\n%s\nwant\n%s", tt.file, got, tt.want)
			}
			cfg, err := config.Load(filepath.Join(dir, config.DefaultPath))
			if err != nil {
				t.Fatal(err)
			}
			if cfg.Files[0].Parser != tt.parser || cfg.Files[0].Version != "1.2.4" {
				t.Errorf("rewritten entry = %+v, want parser %q kept at 1.2.4", cfg.Files[0], tt.parser)
			}
			if c := latestEntry(t, dir).Changes[0]; c.Parser != map[string]string{"": map[string]string{"package.json": "json", "Pipfile": "text"}[tt.file], "text": "text", "toml": "toml"}[tt.parser] {
				t.Errorf("journal records parser %q", c.Parser)
			}
		})
	}
}

func TestUnknownParserIsAConfigError(t *testing.T) {
	body := "[[files]]\npath = \"VERSION\"\n[[files]]\npath = \"tsconfig.json\"\nversion = \"1.2.3\"\nparser = \"jsonc\"\n"
	dir := project(t, body, map[string]string{"VERSION": "1.2.3\n", "tsconfig.json": "{\"version\": \"1.2.3\"}"})
	snap := snapshotProject(t, dir)
	for _, args := range [][]string{nil, {"--dry-run"}, {"preview"}} {
		code, _, stderr := runMain(t, dir, args...)
		if code != ExitError || !strings.Contains(stderr, `files[1] (tsconfig.json): unknown parser "jsonc"; use "json", "toml", "yaml", or "text"`) {
			t.Errorf("%v: exit %d, stderr %q", args, code, stderr)
		}
	}
	assertSnapshot(t, dir, snap, "after the refused runs")
}

// A config written before the parsers changes meaning: an entry for a .json or
// .toml file now matches only whole string values, so an entry pinning a
// version that appeared only in a URL, a range, or a comment finds nothing.
// That fails in planning, before the first write, so the files listed ahead
// of it are not bumped either.
func TestOldConfigFailsBeforeAnythingIsWritten(t *testing.T) {
	dir := reproProject(t)
	if err := os.WriteFile(filepath.Join(dir, "VERSION"), []byte("1.2.3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// What discover wrote for these files before Milestone 39, VERSION first.
	old := "[[files]]\npath = \"VERSION\"\nversion = \"1.2.3\"\n"
	for _, e := range []struct{ path, version string }{
		{"Cargo.toml", "0.4.0"}, {"Cargo.toml", "0.3.9"}, {"Cargo.toml", "1.0.200"},
		{"package.json", "2.0.0"}, {"package.json", "1.2.3"}, {"package.json", "4.17.21"},
		{"package.json", "18.3.1"}, {"package.json", "20.0.0"},
	} {
		old += "[[files]]\npath = \"" + e.path + "\"\nversion = \"" + e.version + "\"\n"
	}
	if err := os.WriteFile(filepath.Join(dir, config.DefaultPath), []byte(old), 0o644); err != nil {
		t.Fatal(err)
	}
	snap := snapshotProject(t, dir)
	for _, args := range [][]string{{"--minor", "--dry-run"}, {"--minor"}} {
		code, stdout, stderr := runMain(t, dir, args...)
		if code != ExitNoVersion || stderr != "incrmit: Cargo.toml: files: expected version not found: 0.3.9\n" || stdout != "" {
			t.Errorf("%v: exit %d, stdout %q, stderr %q", args, code, stdout, stderr)
		}
	}
	assertSnapshot(t, dir, snap, "after the old config was refused")
	if _, err := os.Stat(stateFile(dir)); !os.IsNotExist(err) {
		t.Errorf("a journal was written: %v", err)
	}
}

// undo reads each file with the parser the journal records, whatever the
// config says now, and an entry from before the parser was recorded is read
// as text, which is what wrote it. Here the schema URL holds the version the
// bump wrote, so a revert read as text would move it and one read as JSON
// does not.
func TestUndoUsesTheJournalsParser(t *testing.T) {
	bumped := "{\n  \"$schema\": \"https://x.test/1.2.4/schema.json\",\n  \"version\": \"1.2.4\"\n}\n"
	asJSON := strings.Replace(bumped, "\"version\": \"1.2.4\"", "\"version\": \"1.2.3\"", 1)
	asText := strings.ReplaceAll(bumped, "1.2.4", "1.2.3")
	tests := []struct {
		name, cfgParser, journalParser, want string
	}{
		{"recorded json", "", "json", asJSON},
		{"recorded json, config now text", "text", "json", asJSON},
		{"recorded text", "", "text", asText},
		{"recorded before parsers existed", "", "", asText},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := "[[files]]\npath = \"package.json\"\nversion = \"1.2.4\"\n"
			if tt.cfgParser != "" {
				body += "parser = \"" + tt.cfgParser + "\"\n"
			}
			dir := project(t, body, map[string]string{"package.json": bumped})
			saveEntry(t, dir, history.Change{Path: "package.json", Old: "1.2.3", New: "1.2.4", Parser: tt.journalParser})
			if code, _, stderr := runMain(t, dir, "undo"); code != ExitOK {
				t.Fatalf("undo: exit %d, stderr %q", code, stderr)
			}
			if got := readFile(t, filepath.Join(dir, "package.json")); got != tt.want {
				t.Errorf("package.json =\n%s\nwant\n%s", got, tt.want)
			}
		})
	}

	t.Run("unknown parser", func(t *testing.T) {
		dir := project(t, "[[files]]\npath = \"package.json\"\nversion = \"1.2.4\"\n", map[string]string{"package.json": bumped})
		saveEntry(t, dir, history.Change{Path: "package.json", Old: "1.2.3", New: "1.2.4", Parser: "ini"})
		snap := snapshotProject(t, dir)
		code, _, stderr := runMain(t, dir, "undo")
		if code != ExitError || !strings.Contains(stderr, `package.json: the journal records parser "ini", which this version of incrmit does not know (refusing to undo)`) {
			t.Errorf("exit %d, stderr %q", code, stderr)
		}
		assertSnapshot(t, dir, snap, "after the refused undo")
	})

	t.Run("two parsers for one file", func(t *testing.T) {
		body := "[[files]]\npath = \"package.json\"\nversion = \"1.2.4\"\n[[files]]\npath = \"package.json\"\nversion = \"2.0.1\"\n"
		dir := project(t, body, map[string]string{"package.json": bumped + "\n"})
		saveEntry(t, dir,
			history.Change{Path: "package.json", Old: "1.2.3", New: "1.2.4", Parser: "json"},
			history.Change{Path: "package.json", Old: "2.0.0", New: "2.0.1", Parser: "text"})
		snap := snapshotProject(t, dir)
		code, _, stderr := runMain(t, dir, "undo")
		if code != ExitError || !strings.Contains(stderr, "package.json: the journal records it read as both json and text (refusing to undo)") {
			t.Errorf("exit %d, stderr %q", code, stderr)
		}
		assertSnapshot(t, dir, snap, "after the refused undo")
	})

	t.Run("file no longer parses", func(t *testing.T) {
		dir := project(t, "[[files]]\npath = \"package.json\"\nversion = \"1.2.4\"\n", map[string]string{"package.json": bumped + "}"})
		saveEntry(t, dir, history.Change{Path: "package.json", Old: "1.2.3", New: "1.2.4", Parser: "json"})
		snap := snapshotProject(t, dir)
		code, _, stderr := runMain(t, dir, "undo")
		if code != ExitNoVersion || !strings.Contains(stderr, "package.json: not JSON (line 5: invalid character '}' after top-level value); the last bump read it as json (refusing to undo)") {
			t.Errorf("exit %d, stderr %q", code, stderr)
		}
		assertSnapshot(t, dir, snap, "after the refused undo")
	})
}

// saveEntry saves a journal of one complete entry holding changes.
func saveEntry(t *testing.T, dir string, changes ...history.Change) {
	t.Helper()
	h := &history.History{}
	h.Push(history.Entry{Status: history.StatusComplete, Changes: changes})
	if err := history.Save(stateFile(dir), h); err != nil {
		t.Fatal(err)
	}
}

// A --file target has no recorded parser, so it is read as discover would
// read it the first time: by its extension, falling back to text with a note
// when it is not that format. Read as JSON, a manifest whose only other
// versions sit in ranges is no longer ambiguous.
func TestFileModeReadsByFormat(t *testing.T) {
	pkg := "{\n  \"version\": \"1.2.3\",\n  \"engines\": { \"node\": \">=20.0.0\" }\n}\n"
	dir := project(t, "", map[string]string{"package.json": pkg, "tsconfig.json": "{\n  // c\n  \"version\": \"1.2.3\"\n}\n"})

	code, stdout, stderr := runMain(t, dir, "preview", "--file", "package.json")
	if code != ExitOK || !strings.Contains(stdout, "package.json  1.2.3") || stderr != "" {
		t.Errorf("preview: exit %d, stdout %q, stderr %q", code, stdout, stderr)
	}
	if code, _, stderr := runMain(t, dir, "--file", "package.json"); code != ExitOK {
		t.Fatalf("bump: exit %d, stderr %q", code, stderr)
	}
	if got := readFile(t, filepath.Join(dir, "package.json")); got != strings.Replace(pkg, "1.2.3", "1.2.4", 1) {
		t.Errorf("package.json = %q", got)
	}

	code, stdout, stderr = runMain(t, dir, "--file", "tsconfig.json")
	if code != ExitOK || !strings.Contains(stdout, "tsconfig.json: 1.2.3 -> 1.2.4") {
		t.Fatalf("bump: exit %d, stdout %q, stderr %q", code, stdout, stderr)
	}
	if want := "incrmit: note: tsconfig.json: not JSON (line 2: invalid character '/' looking for beginning of object key string), so it is read as text\n"; stderr != want {
		t.Errorf("stderr = %q, want %q", stderr, want)
	}
}

// A value that is a version only once decoded is not a candidate, and the dry
// run says why rather than leaving the file out silently. A tree whose only
// versions are such values has nothing to write. package.json is a manifest
// discover knows, so the dry run also says it gets no entry.
func TestDiscoverReportsEscapedVersions(t *testing.T) {
	dir := project(t, "", map[string]string{"package.json": "{\n  \"version\": \"1\\u002e2.3\"\n}\n"})
	code, stdout, stderr := runMain(t, dir, "discover", "--dry-run")
	if code != ExitOK || stderr != "" {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
	if want := "  package.json: no entry, since key version holds no version\n    L2 version: skipped, \"1.2.3\" is a version only once its escapes are decoded; incrmit rewrites a version written out plainly\n"; !strings.Contains(stdout, want) {
		t.Errorf("dry run =\n%s\nwant it to hold\n%s", stdout, want)
	}
	code, _, stderr = runMain(t, dir, "discover")
	if code != ExitNoVersion || !strings.Contains(stderr, "no version-bearing files found") {
		t.Errorf("discover: exit %d, stderr %q", code, stderr)
	}
	if _, err := os.Stat(filepath.Join(dir, config.DefaultPath)); !os.IsNotExist(err) {
		t.Errorf("discover wrote a config: %v", err)
	}
}
