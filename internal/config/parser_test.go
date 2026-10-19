package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sasmaq/incrmit/internal/files"
)

// The parser key overrides how an entry's file is read, and omitting it keeps
// the extension's choice, which is what every config written before the key
// existed means.
func TestLoadParserKey(t *testing.T) {
	body := `
[[files]]
path = "package.json"

[[files]]
path = "tsconfig.json"
parser = "text"

[[files]]
path = "Pipfile"
parser = "toml"

[[files]]
path = "Cargo.toml"
parser = "toml"

[[files]]
path = "VERSION"
`
	cfgPath := writeConfig(t, body, "package.json", "tsconfig.json", "Pipfile", "Cargo.toml", "VERSION")
	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	want := map[string]files.Parser{
		"package.json":  files.ParserJSON,
		"tsconfig.json": files.ParserText,
		"Pipfile":       files.ParserTOML,
		"Cargo.toml":    files.ParserTOML,
		"VERSION":       files.ParserText,
	}
	for _, f := range cfg.Files {
		p, err := f.FileParser()
		if err != nil || p != want[f.Path] {
			t.Errorf("%s: FileParser() = %q, %v; want %q", f.Path, p, err, want[f.Path])
		}
	}
}

func TestLoadRejectsBadParser(t *testing.T) {
	tests := []struct {
		name, body string
		targets    []string
		want       []string
	}{
		{"unknown value", "[[files]]\npath = \"VERSION\"\n[[files]]\npath = \"tsconfig.json\"\nparser = \"jsonc\"\n",
			[]string{"VERSION", "tsconfig.json"}, []string{"files[1] (tsconfig.json)", `"jsonc"`, `"json"`, `"toml"`, `"text"`}},
		{"case matters", "[[files]]\npath = \"package.json\"\nparser = \"JSON\"\n",
			[]string{"package.json"}, []string{"files[0] (package.json)", `"JSON"`}},
		{"one path read two ways", "[[files]]\npath = \"package.json\"\nversion = \"1.2.3\"\n" +
			"[[files]]\npath = \"package.json\"\nversion = \"2.0.0\"\nparser = \"text\"\n",
			[]string{"package.json"}, []string{"files[1] (package.json)", "as text", "as json"}},
		{"not a string", "[[files]]\npath = \"VERSION\"\nparser = 1\n", []string{"VERSION"}, []string{"config: parsing"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfgPath := writeConfig(t, tt.body, tt.targets...)
			_, err := Load(cfgPath)
			if err == nil {
				t.Fatal("Load accepted the config")
			}
			for _, w := range tt.want {
				if !strings.Contains(err.Error(), w) {
					t.Errorf("error %q does not contain %q", err, w)
				}
			}
		})
	}
}

// Entries for one file agree when their parsers do, however each is spelled:
// an omitted key and the extension's own parser are the same choice.
func TestLoadAllowsOneParserSpelledTwoWays(t *testing.T) {
	body := "[[files]]\npath = \"package.json\"\nversion = \"1.2.3\"\n" +
		"[[files]]\npath = \"package.json\"\nversion = \"2.0.0\"\nparser = \"json\"\n"
	if _, err := Load(writeConfig(t, body, "package.json")); err != nil {
		t.Fatalf("Load: %v", err)
	}
}

// parser is one of the keys incrmit knows, so discover still regenerates a
// config that sets it, and a bump's rewrite keeps it (it is empty, and
// omitted, for an entry that never set it).
func TestParserKeyRoundTrips(t *testing.T) {
	cfg := &Config{Files: []FileEntry{
		{Path: "tsconfig.json", Version: "1.2.3", Parser: "text"},
		{Path: "package.json", Version: "1.2.3"},
	}}
	data, err := Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(string(data), "parser"); n != 1 {
		t.Errorf("marshaled config names a parser %d times, want once:\n%s", n, data)
	}
	path := filepath.Join(t.TempDir(), DefaultPath)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadIgnore(path); err != nil {
		t.Errorf("LoadIgnore refuses a config with a parser key: %v", err)
	}
	for _, name := range []string{"tsconfig.json", "package.json"} {
		if err := os.WriteFile(filepath.Join(filepath.Dir(path), name), []byte("1.2.3\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	again, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if again.Files[0].Parser != "text" || again.Files[1].Parser != "" {
		t.Errorf("reloaded entries %+v", again.Files)
	}
}
