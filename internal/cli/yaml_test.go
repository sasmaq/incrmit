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

// yamlReproFiles are the files Milestone 41 was reproduced with, kept beside
// Milestone 39's in internal/files/testdata/repro: a Helm Chart.yaml with
// the version in its description, a kubeVersion range, a dependency pinned
// and one given as a range, and an image reference in an annotation; a
// pubspec.yaml with a build number, an SDK range, and dependencies; and a
// Kubernetes manifest of two documents.
var yamlReproFiles = []string{"Chart.yaml", "pubspec.yaml", "deploy.yaml"}

// yamlReproProject copies the YAML reproduction files into a new project
// directory.
func yamlReproProject(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, name := range yamlReproFiles {
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

// What the YAML reproduction files hold that is not the project's version:
// read as text, a minor bump moved every one of them, kubeVersion's range
// losing its -0 on the way, with exit 0.
var yamlReproKept = map[string][]string{
	"Chart.yaml": {"webapp 1.4.2 on Kubernetes", `kubeVersion: ">=1.22.0-0"`, "version: 12.5.6", `version: "~17.11.3"`,
		"image: ghcr.io/example/webapp:1.4.2"},
	"pubspec.yaml": {`sdk: ">=3.4.0 <4.0.0"`, "http: 1.2.1", "provider: ^6.1.2", "flutter_lints: 4.0.0"},
}

// The reproduction, end to end. Read as text, the seven tokens in Chart.yaml
// and the six in pubspec.yaml were all candidates; read as YAML, three of
// each are, each with its key, and these are manifests discover knows, so it
// writes an entry for the chart's version and appVersion and for the pubspec's
// version, and lists the dependency pins as skipped. The manifest of two
// documents is scanned as text, saying why. A minor bump moves the project's
// versions alone, dropping the pubspec's build number, and undo puts back
// every byte.
func TestYAMLReproduction(t *testing.T) {
	golden := func(name string) string { return goldenPath(t, name) } // before runMain changes directory
	dryRunGolden, configGolden, bumpGolden := golden("yaml_repro_discover_dry_run.golden"), golden("yaml_repro_incrmit.toml.golden"), golden("yaml_repro_bump_minor.golden")
	fileGolden := map[string]string{}
	for _, name := range yamlReproFiles {
		fileGolden[name] = golden("yaml_repro_minor_" + name + ".golden")
	}
	dir := yamlReproProject(t)
	before := map[string]string{}
	for _, name := range yamlReproFiles {
		before[name] = readFile(t, filepath.Join(dir, name))
	}

	code, stdout, stderr := runMain(t, dir, "discover", "--dry-run")
	if code != ExitOK || stderr != "" {
		t.Fatalf("discover --dry-run: exit %d, stderr %q", code, stderr)
	}
	checkGolden(t, dryRunGolden, stdout)

	code, stdout, stderr = runMain(t, dir, "discover")
	if code != ExitOK {
		t.Fatalf("discover: exit %d, stderr %q", code, stderr)
	}
	if !strings.Contains(stdout, "  deploy.yaml: 1.4.2 (2 YAML documents, scanned as text)\n") {
		t.Errorf("discover summary does not say why deploy.yaml was scanned as text:\n%s", stdout)
	}
	checkGolden(t, configGolden, readFile(t, filepath.Join(dir, config.DefaultPath)))

	code, stdout, stderr = runMain(t, dir, "--minor")
	if code != ExitOK {
		t.Fatalf("bump: exit %d, stderr %q", code, stderr)
	}
	checkGolden(t, bumpGolden, stdout)
	for _, name := range yamlReproFiles {
		checkGolden(t, fileGolden[name], readFile(t, filepath.Join(dir, name)))
	}
	for name, kept := range yamlReproKept {
		got := readFile(t, filepath.Join(dir, name))
		for _, k := range kept {
			if !strings.Contains(got, k) {
				t.Errorf("%s lost %s:\n%s", name, k, got)
			}
		}
	}

	for _, c := range latestEntry(t, dir).Changes {
		if want := map[string]string{"Chart.yaml": "yaml", "pubspec.yaml": "yaml", "deploy.yaml": "text"}[c.Path]; c.Parser != want {
			t.Errorf("journal records %s read as %q, want %q", c.Path, c.Parser, want)
		}
	}
	if code, _, stderr = runMain(t, dir, "undo"); code != ExitOK {
		t.Fatalf("undo: exit %d, stderr %q", code, stderr)
	}
	for _, name := range yamlReproFiles {
		if got := readFile(t, filepath.Join(dir, name)); got != before[name] {
			t.Errorf("undo left %s as\n%s\nwant\n%s", name, got, before[name])
		}
	}
}

// yamlOldConfig is the config discover wrote for the reproduction files before
// Milestone 41, when every token in them was a candidate, with parser added to
// each entry when parser is not empty, and without the entries for the files
// in drop.
func yamlOldConfig(parser string, drop ...string) string {
	var b strings.Builder
	for _, e := range []struct{ path, version, extra string }{
		{"Chart.yaml", "1.4.2", ""}, {"Chart.yaml", "0.7.3", ""}, {"Chart.yaml", "1.22.0", "prerelease = \"0\"\n"},
		{"Chart.yaml", "12.5.6", ""}, {"Chart.yaml", "17.11.3", ""}, {"deploy.yaml", "1.4.2", ""},
		{"pubspec.yaml", "2.3.1", "build = \"45\"\n"}, {"pubspec.yaml", "3.4.0", ""}, {"pubspec.yaml", "4.0.0", ""},
		{"pubspec.yaml", "1.2.1", ""}, {"pubspec.yaml", "6.1.2", ""},
	} {
		if slices.Contains(drop, e.path) {
			continue
		}
		b.WriteString("[[files]]\npath = \"" + e.path + "\"\nversion = \"" + e.version + "\"\n" + e.extra)
		if parser != "" {
			b.WriteString("parser = \"" + parser + "\"\n")
		}
	}
	return b.String()
}

// A config written before the YAML parser changes meaning: a manifest of two
// documents is no longer read at all, and an entry for a YAML file now
// matches only whole string values, so an entry pinning a version found only
// in a range, a longer string, or a comment finds nothing. Each fails in
// planning, before the first write. Setting parser = "text" on the entries
// restores the old bump exactly, which is what it did: every token moved.
func TestYAMLOldConfigFailsBeforeAnythingIsWritten(t *testing.T) {
	golden := func(name string) string { return goldenPath(t, name) }
	textGolden := map[string]string{}
	for _, name := range yamlReproFiles {
		textGolden[name] = golden("yaml_repro_text_minor_" + name + ".golden")
	}
	dir := yamlReproProject(t)
	for _, tt := range []struct{ config, stderr string }{
		{yamlOldConfig(""), "incrmit: deploy.yaml: 2 YAML documents; to read it as text, set parser = \"text\" on its [[files]] entry\n"},
		{yamlOldConfig("", "deploy.yaml"), "incrmit: Chart.yaml: files: expected version not found: 1.22.0-0\n"},
	} {
		if err := os.WriteFile(filepath.Join(dir, config.DefaultPath), []byte(tt.config), 0o644); err != nil {
			t.Fatal(err)
		}
		snap := snapshotProject(t, dir)
		for _, args := range [][]string{{"--minor", "--dry-run"}, {"--minor"}} {
			code, stdout, stderr := runMain(t, dir, args...)
			if code != ExitNoVersion || stderr != tt.stderr || stdout != "" {
				t.Errorf("%v: exit %d, stdout %q, stderr %q, want %q", args, code, stdout, stderr, tt.stderr)
			}
		}
		assertSnapshot(t, dir, snap, "after the old config was refused")
	}
	if _, err := os.Stat(stateFile(dir)); !os.IsNotExist(err) {
		t.Errorf("a journal was written: %v", err)
	}

	if err := os.WriteFile(filepath.Join(dir, config.DefaultPath), []byte(yamlOldConfig("text")), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, _, stderr := runMain(t, dir, "--minor"); code != ExitOK {
		t.Fatalf("bump with parser = \"text\": exit %d, stderr %q", code, stderr)
	}
	for _, name := range yamlReproFiles {
		checkGolden(t, textGolden[name], readFile(t, filepath.Join(dir, name)))
	}
	if chart := readFile(t, filepath.Join(dir, "Chart.yaml")); !strings.Contains(chart, `kubeVersion: ">=1.23.0"`) {
		t.Errorf("read as text, the kubeVersion range should move as it did before:\n%s", chart)
	}
}

// A YAML file of more than one document is no YAML the parser reads, and a
// bump never falls back: an entry for one fails in planning, saying how to read
// it as text, with nothing written. A --file target has no entry, so it is
// read as discover would read it, falling back with a note.
func TestYAMLBumpRefusesSeveralDocuments(t *testing.T) {
	manifest := "image: ghcr.io/me/app:1.2.3\n---\nversion: \"1.2.3\"\n"
	dir := project(t, "[[files]]\npath = \"VERSION\"\nversion = \"1.2.3\"\n[[files]]\npath = \"deploy.yaml\"\nversion = \"1.2.3\"\n",
		map[string]string{"VERSION": "1.2.3\n", "deploy.yaml": manifest})
	snap := snapshotProject(t, dir)
	for _, args := range [][]string{{"--dry-run"}, nil, {"preview"}} {
		code, _, stderr := runMain(t, dir, args...)
		want := "incrmit: deploy.yaml: 2 YAML documents; to read it as text, set parser = \"text\" on its [[files]] entry\n"
		if code != ExitNoVersion || stderr != want {
			t.Errorf("%v: exit %d, stderr %q, want %q", args, code, stderr, want)
		}
	}
	assertSnapshot(t, dir, snap, "after the refused bump")

	code, _, stderr := runMain(t, dir, "--file", "deploy.yaml")
	if code != ExitOK || stderr != "incrmit: note: deploy.yaml: 2 YAML documents, so it is read as text\n" {
		t.Errorf("--file: exit %d, stderr %q", code, stderr)
	}
	if got := readFile(t, filepath.Join(dir, "deploy.yaml")); got != strings.ReplaceAll(manifest, "1.2.3", "1.2.4") {
		t.Errorf("deploy.yaml = %q", got)
	}
}

// The image-tag recipe: a manifest of two documents is read as text, where
// every token counts, so to move the image tag alone its entry takes the text
// before the tag as its key.
func TestYAMLImageTagRecipe(t *testing.T) {
	manifest := readFile(t, filepath.Join(reproDir, "deploy.yaml"))
	body := "[[files]]\npath = \"deploy.yaml\"\nversion = \"1.4.2\"\nparser = \"text\"\nkey = \"ghcr.io/example/webapp:\"\n"
	dir := project(t, body, map[string]string{"deploy.yaml": manifest})
	code, stdout, stderr := runMain(t, dir, "--minor")
	if code != ExitOK {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
	if want := "  deploy.yaml \"ghcr.io/example/webapp:\": 1.4.2 -> 1.5.0\n"; !strings.Contains(stdout, want) {
		t.Errorf("summary = %q, want it to hold %q", stdout, want)
	}
	want := strings.Replace(manifest, "webapp:1.4.2", "webapp:1.5.0", 1)
	if got := readFile(t, filepath.Join(dir, "deploy.yaml")); got != want || strings.Count(got, `version: "1.4.2"`) != 2 {
		t.Errorf("deploy.yaml =\n%s\nwant only the image tag moved", got)
	}
}

// The parser key overrides the extension in both directions: text for a
// .yaml file restores the scanner's behavior, and yaml reads YAML under a
// name that is not .yaml, .yml, or .cff.
func TestYAMLParserKeyOverridesExtension(t *testing.T) {
	values := "# chart 1.2.3\nimage:\n  tag: 1.2.3\n  ref: ghcr.io/me/app:1.2.3\n"
	tests := []struct {
		name, file, parser, want, journal string
	}{
		{"yaml by extension", "values.yaml", "", "# chart 1.2.3\nimage:\n  tag: 1.2.4\n  ref: ghcr.io/me/app:1.2.3\n", "yaml"},
		{"yaml read as text", "values.yaml", "text", strings.ReplaceAll(values, "1.2.3", "1.2.4"), "text"},
		{"no extension is text", "values", "", strings.ReplaceAll(values, "1.2.3", "1.2.4"), "text"},
		{"no extension read as yaml", "values", "yaml", "# chart 1.2.3\nimage:\n  tag: 1.2.4\n  ref: ghcr.io/me/app:1.2.3\n", "yaml"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := "[[files]]\npath = \"" + tt.file + "\"\nversion = \"1.2.3\"\n"
			if tt.parser != "" {
				body += "parser = \"" + tt.parser + "\"\n"
			}
			dir := project(t, body, map[string]string{tt.file: values})
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
			if c := latestEntry(t, dir).Changes[0]; c.Parser != tt.journal {
				t.Errorf("journal records parser %q, want %q", c.Parser, tt.journal)
			}
		})
	}
}

// An anchored version is one candidate: the dry run lists the aliases a bump
// changes with it, the bump rewrites the anchor's bytes alone, and a key that
// names an alias, or a path the data has only through one, fails with exit 3
// naming the anchored value, with nothing written.
func TestYAMLAliases(t *testing.T) {
	values := "base: &v 1.2.3\npinned: *v\ndefaults: &d\n  tag: 1.2.3\nprod:\n  <<: *d\n"

	dir := project(t, "", map[string]string{"values.yaml": values})
	code, stdout, stderr := runMain(t, dir, "discover", "--dry-run")
	want := "  values.yaml:\n    L1 base: base: &v 1.2.3 (aliased at pinned)\n    L4 defaults.tag: tag: 1.2.3 (aliased at prod.\"<<\")\n"
	if code != ExitOK || !strings.Contains(stdout, want) {
		t.Errorf("dry run: exit %d, stdout %q, stderr %q; want it to hold %q", code, stdout, stderr, want)
	}

	for key, msg := range map[string]string{
		"pinned":   "incrmit: values.yaml: key pinned reads base through an alias, and incrmit rewrites a value where it is written; name base instead\n",
		"prod.tag": "incrmit: values.yaml: key prod.tag reads defaults.tag through an alias, and incrmit rewrites a value where it is written; name defaults.tag instead\n",
	} {
		t.Run(key, func(t *testing.T) {
			dir := project(t, "[[files]]\npath = \"values.yaml\"\nversion = \"1.2.3\"\nkey = \""+key+"\"\n", map[string]string{"values.yaml": values})
			snap := snapshotProject(t, dir)
			for _, args := range [][]string{{"--dry-run"}, nil} {
				if code, _, stderr := runMain(t, dir, args...); code != ExitNoVersion || stderr != msg {
					t.Errorf("%v: exit %d, stderr %q, want %q", args, code, stderr, msg)
				}
			}
			assertSnapshot(t, dir, snap, "after the refused bump")
		})
	}

	dir = project(t, "[[files]]\npath = \"values.yaml\"\nversion = \"1.2.3\"\nkey = \"base\"\n", map[string]string{"values.yaml": values})
	if code, stdout, stderr := runMain(t, dir); code != ExitOK || stdout != "Applied a patch bump to 1 file(s):\n  values.yaml base: 1.2.3 -> 1.2.4\n" {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, stdout, stderr)
	}
	if got, want := readFile(t, filepath.Join(dir, "values.yaml")), strings.Replace(values, "1.2.3", "1.2.4", 1); got != want {
		t.Errorf("values.yaml =\n%s\nwant\n%s", got, want)
	}
}

// Flutter reads the part of a pubspec version after "+" as the build number,
// and a bump drops build metadata (doc/DEVELOPMENT.md §9.2), as the text
// scanner always did: a patch bump of 2.3.1+45 writes 2.3.2.
func TestPubspecBuildNumberIsDropped(t *testing.T) {
	dir := project(t, "", map[string]string{"pubspec.yaml": "name: app\nversion: 2.3.1+45\ndependencies:\n  http: 2.3.1\n"})
	code, stdout, stderr := runMain(t, dir, "--file", "pubspec.yaml", "--key", "version")
	if code != ExitOK || !strings.Contains(stdout, "pubspec.yaml version: 2.3.1+45 -> 2.3.2") {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, stdout, stderr)
	}
	if got := readFile(t, filepath.Join(dir, "pubspec.yaml")); got != "name: app\nversion: 2.3.2\ndependencies:\n  http: 2.3.1\n" {
		t.Errorf("pubspec.yaml = %q", got)
	}
}

// undo reads a YAML file with the parser the journal records, whatever the
// config says now. Here a comment holds the version the bump wrote, so a
// revert read as text would move it, and one read as YAML does not.
func TestUndoUsesTheJournalsYAMLParser(t *testing.T) {
	bumped := "version: 1.2.4 # next is 1.2.4\n"
	for name, tt := range map[string]struct{ cfgParser, journalParser, want string }{
		"recorded yaml":                   {"", "yaml", "version: 1.2.3 # next is 1.2.4\n"},
		"recorded yaml, config now text":  {"text", "yaml", "version: 1.2.3 # next is 1.2.4\n"},
		"recorded before the YAML parser": {"", "", "version: 1.2.3 # next is 1.2.3\n"},
	} {
		t.Run(name, func(t *testing.T) {
			body := "[[files]]\npath = \"app.yaml\"\nversion = \"1.2.4\"\n"
			if tt.cfgParser != "" {
				body += "parser = \"" + tt.cfgParser + "\"\n"
			}
			dir := project(t, body, map[string]string{"app.yaml": bumped})
			saveEntry(t, dir, history.Change{Path: "app.yaml", Old: "1.2.3", New: "1.2.4", Parser: tt.journalParser})
			if code, _, stderr := runMain(t, dir, "undo"); code != ExitOK {
				t.Fatalf("undo: exit %d, stderr %q", code, stderr)
			}
			if got := readFile(t, filepath.Join(dir, "app.yaml")); got != tt.want {
				t.Errorf("app.yaml = %q, want %q", got, tt.want)
			}
		})
	}
}
