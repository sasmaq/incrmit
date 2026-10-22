package config

import (
	"strings"
	"testing"
)

// A key is read by its entry's parser, so the same text is a fine key for one
// file and a config error for another. Several entries may list one path when
// each pins its own version or names its own key; which occurrences two keys
// name can only be told with the file in hand, so Load leaves that to the
// bump.
func TestLoadKey(t *testing.T) {
	body := `
[[files]]
path = "package.json"
version = "1.2.3"
key = "version"

[[files]]
path = "package.json"
version = "1.2.3"
key = 'dependencies."left-pad"'

[[files]]
path = "Makefile"
key = "VERSION ?= "

[[files]]
path = "Makefile"
key = "TOOL := "

[[files]]
path = "tsconfig.json"
parser = "text"
key = '"version": "'

[[files]]
path = "VERSION"
`
	cfgPath := writeConfig(t, body, "package.json", "Makefile", "tsconfig.json", "VERSION")
	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	want := []string{"version", "dependencies.left-pad", `"VERSION ?= "`, `"TOOL := "`, `"\"version\": \""`, ""}
	for i, f := range cfg.Files {
		k, err := f.FileKey()
		got := ""
		if k != nil {
			got = k.String()
			if k.Raw() != f.KeyText() {
				t.Errorf("files[%d]: Raw() = %q, written %q", i, k.Raw(), f.KeyText())
			}
		}
		if err != nil || got != want[i] {
			t.Errorf("files[%d]: FileKey() = %q, %v; want %q", i, got, err, want[i])
		}
	}

	out, err := Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "  path = \"Makefile\"\n  key = \"VERSION ?= \"\n") ||
		!strings.Contains(string(out), "  key = \"dependencies.\\\"left-pad\\\"\"\n") {
		t.Errorf("Marshal did not keep the keys:\n%s", out)
	}
}

func TestLoadRejectsBadKey(t *testing.T) {
	tests := []struct {
		name, body, want string
	}{
		{"empty", "[[files]]\npath = \"VERSION\"\nkey = \"\"\n",
			`config: files[0] (VERSION): key is empty; for a text file, a key is the text right before the version on its line, such as "VERSION ?= "`},
		{"a line break", "[[files]]\npath = \"VERSION\"\nkey = \"v=\\r\"\n",
			`config: files[0] (VERSION): key "v=\r" holds a line break`},
		{"not a path", "[[files]]\npath = \"package.json\"\nkey = \"version=\"\n",
			`config: files[0] (package.json): key "version=" is not a key path ('=' cannot follow "version"); for a JSON file, a key is a path such as package.version`},
		{"not a path for its parser", "[[files]]\npath = \"VERSION\"\nparser = \"toml\"\nkey = \"V = \"\n",
			`config: files[0] (VERSION): key "V = " is not a key path (' ' cannot follow "V"); for a TOML file`},
		{"a key with an unknown parser", "[[files]]\npath = \"VERSION\"\nparser = \"ini\"\nkey = \"v\"\n",
			`config: files[0] (VERSION): unknown parser "ini"`},
		{"one key twice", "[[files]]\npath = \"VERSION\"\nversion = \"1.2.3\"\nkey = \"v=\"\n[[files]]\npath = \"VERSION\"\nversion = \"1.2.3\"\nkey = \"v=\"\n",
			`config: duplicate path "VERSION" with version "1.2.3" and key "v="`},
		{"a bare entry beside a keyed one", "[[files]]\npath = \"VERSION\"\nkey = \"v=\"\n[[files]]\npath = \"VERSION\"\n",
			`config: duplicate path "VERSION"`},
		{"a keyed entry after a bare one", "[[files]]\npath = \"VERSION\"\n[[files]]\npath = \"VERSION\"\nkey = \"v=\"\n",
			`config: duplicate path "VERSION"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Load(writeConfig(t, tt.body, "VERSION", "package.json"))
			if err == nil || !strings.HasPrefix(err.Error(), tt.want) {
				t.Errorf("Load error = %v, want %q", err, tt.want)
			}
		})
	}
}

// An entry without a key has no FileKey, and LoadIgnore accepts the key as one
// incrmit knows, so discover may regenerate a config that has keys.
func TestKeyIsAKnownKey(t *testing.T) {
	if k, err := (FileEntry{Path: "VERSION"}).FileKey(); k != nil || err != nil {
		t.Errorf("FileKey() = %v, %v for an entry without a key", k, err)
	}
	cfgPath := writeConfig(t, "ignore = [\"x/\"]\n[[files]]\npath = \"package.json\"\nkey = \"version\"\n", "package.json")
	ignore, err := LoadIgnore(cfgPath)
	if err != nil || len(ignore) != 1 {
		t.Errorf("LoadIgnore = %v, %v", ignore, err)
	}
}
