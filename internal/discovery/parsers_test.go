package discovery

import (
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"

	"github.com/sasmaq/incrmit/internal/config"
	"github.com/sasmaq/incrmit/internal/files"
)

// writeTree writes name -> body under a new temporary directory and returns it.
func writeTree(t *testing.T, tree map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for name, body := range tree {
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

// keyed renders a result's occurrences as "Lline key=version", the key empty
// for text.
func keyed(r Result) []string {
	var out []string
	for _, o := range r.Occurrences {
		out = append(out, "L"+strconv.Itoa(o.Line)+" "+o.Key.String()+"="+o.Version.String())
	}
	return out
}

func resultFor(t *testing.T, results []Result, path string) Result {
	t.Helper()
	i := slices.IndexFunc(results, func(r Result) bool { return r.Path == path })
	if i < 0 {
		t.Fatalf("no result for %s in %+v", path, results)
	}
	return results[i]
}

// A .json or .toml file is read by its format, so a version inside a URL, a
// range, or a comment is not a candidate there, and every candidate carries
// its key. Every other file is scanned as text, as before.
func TestDiscoverReadsJSONAndTOMLByFormat(t *testing.T) {
	root := writeTree(t, map[string]string{
		"package.json": "{\n  \"$schema\": \"https://x.test/2.0.0/s.json\",\n  \"version\": \"1.2.3\",\n" +
			"  \"engines\": { \"node\": \">=20.0.0\" }\n}\n",
		"Cargo.toml":        "[package]\nversion = \"0.4.0\" # was 0.3.9\n[dependencies]\nserde = { version = \"1.0.200\" }\n",
		"sub/SETTINGS.JSON": "{\"v\": [\"4.5.6\", \"x 4.5.7\"]}",
		"notes.md":          "See https://x.test/2.0.0/ and version = \"1.2.3\" # 0.3.9\n",
		"data.json5":        "{ version: '7.7.7' }",
	})
	results, err := Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		path   string
		parser files.Parser
		want   []string
	}{
		{"Cargo.toml", files.ParserTOML, []string{"L2 package.version=0.4.0", "L4 dependencies.serde.version=1.0.200"}},
		{"data.json5", files.ParserText, []string{"L1 =7.7.7"}},
		{"notes.md", files.ParserText, []string{"L1 =2.0.0", "L1 =1.2.3", "L1 =0.3.9"}},
		{"package.json", files.ParserJSON, []string{"L3 version=1.2.3"}},
		{"sub/SETTINGS.JSON", files.ParserJSON, []string{"L1 v[0]=4.5.6"}},
	}
	if len(results) != len(tests) {
		t.Fatalf("got %d results, want %d: %+v", len(results), len(tests), results)
	}
	for _, tt := range tests {
		r := resultFor(t, results, tt.path)
		if r.Parser != tt.parser || r.Fallback != nil {
			t.Errorf("%s: parser %q, fallback %v; want %q and none", tt.path, r.Parser, r.Fallback, tt.parser)
		}
		if got := keyed(r); !slices.Equal(got, tt.want) {
			t.Errorf("%s: occurrences %v, want %v", tt.path, got, tt.want)
		}
	}
}

// A .json or .toml file its parser cannot read is scanned as text, and the
// result says why, so discover can record parser = "text" and print the
// reason.
func TestDiscoverFallsBackToText(t *testing.T) {
	root := writeTree(t, map[string]string{
		"tsconfig.json":   "{\n  // a comment\n  \"version\": \"1.2.3\"\n}\n",
		"broken.toml":     "[package\nversion = \"2.0.0\"\n",
		"no-version.json": "{\n  // a comment\n}\n",
	})
	results, err := Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("got %+v, want tsconfig.json and broken.toml (a file with no version is no result)", results)
	}
	for path, want := range map[string]struct {
		parser files.Parser
		err    string
		occ    []string
	}{
		"tsconfig.json": {files.ParserJSON, "not JSON (line 2: invalid character '/'", []string{"L3 =1.2.3"}},
		"broken.toml":   {files.ParserTOML, "not TOML (line 2: ", []string{"L2 =2.0.0"}},
	} {
		r := resultFor(t, results, path)
		if r.Parser != files.ParserText || r.Fallback == nil || r.Fallback.Parser != want.parser {
			t.Fatalf("%s: parser %q, fallback %v; want text, falling back from %s", path, r.Parser, r.Fallback, want.parser)
		}
		if !strings.HasPrefix(r.Fallback.Error(), want.err) {
			t.Errorf("%s: fallback reason %q, want it to start %q", path, r.Fallback, want.err)
		}
		if got := keyed(r); !slices.Equal(got, want.occ) {
			t.Errorf("%s: occurrences %v, want %v", path, got, want.occ)
		}
	}
}

// A value that is a version only once its escapes are decoded is no
// candidate, but the result keeps it so the dry run can say why it was left
// out, even when the file has no candidate at all.
func TestDiscoverKeepsSkippedValues(t *testing.T) {
	root := writeTree(t, map[string]string{
		"package.json": "{\n  \"version\": \"1\\u002e2.3\"\n}\n",
		"other.json":   "{\"a\": \"1.0.0\", \"b\": \"\\u0032.0.0\"}",
	})
	results, err := Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	pkg := resultFor(t, results, "package.json")
	if len(pkg.Occurrences) != 0 || len(pkg.Skipped) != 1 || pkg.Skipped[0].Line != 2 || pkg.Skipped[0].Key.String() != "version" {
		t.Errorf("package.json = %+v, want one skipped value at line 2, key version", pkg)
	}
	other := resultFor(t, results, "other.json")
	if got := keyed(other); !slices.Equal(got, []string{"L1 a=1.0.0"}) || len(other.Skipped) != 1 {
		t.Errorf("other.json = %+v", other)
	}

	// The generated config lists only the candidates.
	data, err := Generate(results)
	if err != nil {
		t.Fatal(err)
	}
	var cfg config.Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		t.Fatal(err)
	}
	if len(cfg.Files) != 1 || cfg.Files[0] != (config.FileEntry{Path: "other.json", Version: "1.0.0"}) {
		t.Errorf("generated entries %+v, want only other.json at 1.0.0", cfg.Files)
	}
}

// Generate writes `parser` only where it differs from what the extension
// selects, which is a .json or .toml file scanned as text: a bump then reads
// the file the way discover did.
func TestGenerateWritesParserOnlyWhenItDiffers(t *testing.T) {
	root := writeTree(t, map[string]string{
		"tsconfig.json": "{\n  // c\n  \"version\": \"1.2.3\"\n}\n",
		"package.json":  "{\"version\": \"1.2.3\"}",
		"Cargo.toml":    "version = \"1.2.3\"\n",
		"VERSION":       "1.2.3\n",
	})
	results, err := Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	data, err := Generate(results)
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(string(data), "parser"); n != 1 {
		t.Errorf("generated config names a parser %d times, want once:\n%s", n, data)
	}
	var cfg config.Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		t.Fatal(err)
	}
	for _, f := range cfg.Files {
		want := ""
		if f.Path == "tsconfig.json" {
			want = "text"
		}
		if f.Parser != want {
			t.Errorf("%s: parser %q, want %q", f.Path, f.Parser, want)
		}
	}
}
