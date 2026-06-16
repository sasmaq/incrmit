package discovery

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sasmaq/incrmit/internal/config"
	"github.com/sasmaq/incrmit/internal/version"
)

// fixtureTree writes a tree exercising content-based detection across arbitrary
// file names and types, plus files and directories that must be skipped, and
// returns the root.
func fixtureTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"VERSION":              "1.2.3\n",
		"package.json":         `{"name":"x","version":"2.0.1","dependencies":{"left-pad":"1.0.0"}}`,
		"pyproject.toml":       "[project]\nname = \"x\"\nversion = \"3.4.5\"\n",
		"sub/Cargo.toml":       "[package]\nname = \"x\"\nversion = \"0.1.9\"\n",
		"sub/build_info.go":    "package build\n\nconst Version = \"7.8.9\"\n",
		"notes.txt":            "Released version 5.6.7 yesterday.\n",
		"app.config":           "endpoint=https://x\nversion=10.20.30\n",
		"Dockerfile":           "FROM alpine:3.19\nLABEL version=\"8.0.1\"\n",
		"README.md":            "no version token of interest here\n",
		"empty/marker":         "not a version\n",
		".git/VERSION":         "9.9.9\n",
		"node_modules/VERSION": "9.9.9\n",
		"vendor/pkg/VERSION":   "9.9.9\n",
		"dist/VERSION":         "9.9.9\n",
	}
	for name, body := range files {
		p := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// A binary file containing a version-like byte sequence must be ignored.
	if err := os.WriteFile(filepath.Join(root, "logo.png"), []byte("\x89PNG\x00\x00 4.5.6"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestDiscover(t *testing.T) {
	root := fixtureTree(t)
	got, err := Discover(root)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}

	// Detection is content-based and works for any file name. Within a file the
	// first MAJOR.MINOR.PATCH token wins (e.g. package.json -> 2.0.1, not the
	// dependency's 1.0.0; Dockerfile -> 8.0.1, since "3.19" is not a full
	// version). Results are sorted by path.
	want := []Result{
		{Path: "Dockerfile", Version: version.Version{Major: 8, Minor: 0, Patch: 1}},
		{Path: "VERSION", Version: version.Version{Major: 1, Minor: 2, Patch: 3}},
		{Path: "app.config", Version: version.Version{Major: 10, Minor: 20, Patch: 30}},
		{Path: "notes.txt", Version: version.Version{Major: 5, Minor: 6, Patch: 7}},
		{Path: "package.json", Version: version.Version{Major: 2, Minor: 0, Patch: 1}},
		{Path: "pyproject.toml", Version: version.Version{Major: 3, Minor: 4, Patch: 5}},
		{Path: "sub/Cargo.toml", Version: version.Version{Major: 0, Minor: 1, Patch: 9}},
		{Path: "sub/build_info.go", Version: version.Version{Major: 7, Minor: 8, Patch: 9}},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d results, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("result[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}

// The config file itself must never be reported as a target, even though it is
// a text file containing version tokens.
func TestDiscoverSkipsConfigFile(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "VERSION"), []byte("1.2.3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, config.DefaultPath), []byte("[[files]]\npath = \"VERSION\"\nversion = \"1.2.3\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range got {
		if r.Path == config.DefaultPath {
			t.Fatalf("Discover returned the config file: %+v", got)
		}
	}
	if len(got) != 1 || got[0].Path != "VERSION" {
		t.Errorf("got %+v, want only VERSION", got)
	}
}

func TestDiscoverSkipsIgnoredDirs(t *testing.T) {
	root := fixtureTree(t)
	got, err := Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range got {
		for _, ignored := range []string{".git", "node_modules", "vendor", "dist"} {
			if strings.HasPrefix(r.Path, ignored+"/") {
				t.Errorf("result %q is inside ignored dir %q", r.Path, ignored)
			}
		}
		if r.Version == (version.Version{Major: 9, Minor: 9, Patch: 9}) {
			t.Errorf("picked up a 9.9.9 version from an ignored dir: %+v", r)
		}
	}
}

func TestDiscoverArbitraryFileName(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, root, "anything.weirdext", "the build is 4.5.6 now\n")
	got, err := Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Path != "anything.weirdext" ||
		got[0].Version != (version.Version{Major: 4, Minor: 5, Patch: 6}) {
		t.Errorf("got %+v, want one 4.5.6 result for anything.weirdext", got)
	}
}

func TestDiscoverFirstMatchWins(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, root, "f", "current 1.2.3 and later 9.9.9\n")
	got, err := Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Version != (version.Version{Major: 1, Minor: 2, Patch: 3}) {
		t.Errorf("got %+v, want first match 1.2.3", got)
	}
}

func TestDiscoverIgnoresTwoComponentNumbers(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, root, "f", "python 3.9 then 1.2.3\n")
	got, err := Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Version != (version.Version{Major: 1, Minor: 2, Patch: 3}) {
		t.Errorf("got %+v, want 1.2.3 (3.9 is not a full version)", got)
	}
}

func TestDiscoverSkipsBinaryFiles(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "bin"), []byte("\x00\x01\x02 1.2.3"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("got %+v, want 0 results (binary file should be skipped)", got)
	}
}

func TestDiscoverEmptyTree(t *testing.T) {
	got, err := Discover(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("got %d results, want 0", len(got))
	}
}

func TestDiscoverFilesWithoutVersionSkipped(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, root, "a.txt", "no version here\n")
	mustWrite(t, root, "b.json", "{ totally unrelated }")
	mustWrite(t, root, "c", "year 2024, build 42\n")
	got, err := Discover(root)
	if err != nil {
		t.Fatalf("Discover should not fail: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("got %+v, want 0 results", got)
	}
}

func TestGenerateRoundTrips(t *testing.T) {
	root := fixtureTree(t)
	results, err := Discover(root)
	if err != nil {
		t.Fatal(err)
	}

	data, err := Generate(results)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	// Write the generated config into the scanned tree and load it back; since
	// every discovered path exists, validation must pass.
	cfgPath := filepath.Join(root, "incrmit.toml")
	if err := os.WriteFile(cfgPath, data, 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("loading generated config: %v", err)
	}
	if len(cfg.Files) != len(results) {
		t.Fatalf("config has %d files, want %d", len(cfg.Files), len(results))
	}
	for i, r := range results {
		if cfg.Files[i].Path != r.Path || cfg.Files[i].Version != r.Version.String() {
			t.Errorf("entry %d = %+v, want path %q version %q", i, cfg.Files[i], r.Path, r.Version)
		}
	}
}

func mustWrite(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
