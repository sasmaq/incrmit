package discovery

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sasmaq/incrmit/internal/config"
	"github.com/sasmaq/incrmit/internal/version"
)

// fixtureTree writes a tree exercising every supported file type plus files and
// directories that must be ignored, and returns the root.
func fixtureTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"VERSION":              "1.2.3\n",
		"package.json":         `{"name":"x","version":"2.0.1","dependencies":{"left-pad":"1.0.0"}}`,
		"pyproject.toml":       "[project]\nname = \"x\"\nversion = \"3.4.5\"\n",
		"sub/Cargo.toml":       "[package]\nname = \"x\"\nversion = \"0.1.9\"\n",
		"sub/build_info.go":    "package build\n\nconst Version = \"7.8.9\"\n",
		"README.md":            "no version token of interest here\n",
		"empty/VERSION":        "not a version\n",
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
	return root
}

func TestDiscover(t *testing.T) {
	root := fixtureTree(t)
	got, err := Discover(root)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}

	want := []Result{
		{Path: "VERSION", Version: version.Version{Major: 1, Minor: 2, Patch: 3}},
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

func TestDiscoverPoetryFallback(t *testing.T) {
	root := t.TempDir()
	body := "[tool.poetry]\nname = \"x\"\nversion = \"4.5.6\"\n"
	if err := os.WriteFile(filepath.Join(root, "pyproject.toml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Version != (version.Version{Major: 4, Minor: 5, Patch: 6}) {
		t.Errorf("got %+v, want one 4.5.6 result", got)
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

func TestDiscoverMalformedFilesSkipped(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, root, "package.json", "{ this is not json")
	mustWrite(t, root, "Cargo.toml", "this is = = not toml")
	mustWrite(t, root, "VERSION", "definitely not a version")
	got, err := Discover(root)
	if err != nil {
		t.Fatalf("Discover should not fail on malformed files: %v", err)
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
