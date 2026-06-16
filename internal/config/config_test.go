package config

import (
	"os"
	"path/filepath"
	"testing"
)

// writeConfig writes a config file plus any named target files into a fresh
// temp dir and returns the config path.
func writeConfig(t *testing.T, configBody string, targets ...string) string {
	t.Helper()
	dir := t.TempDir()
	for _, name := range targets {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatalf("mkdir for target %q: %v", name, err)
		}
		if err := os.WriteFile(p, []byte("1.0.0\n"), 0o644); err != nil {
			t.Fatalf("write target %q: %v", name, err)
		}
	}
	cfgPath := filepath.Join(dir, "incrmit.toml")
	if err := os.WriteFile(cfgPath, []byte(configBody), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return cfgPath
}

func TestResolvePath(t *testing.T) {
	if got := ResolvePath(""); got != DefaultPath {
		t.Errorf("ResolvePath(\"\") = %q, want %q", got, DefaultPath)
	}
	if got := ResolvePath("custom.toml"); got != "custom.toml" {
		t.Errorf("ResolvePath(\"custom.toml\") = %q, want %q", got, "custom.toml")
	}
}

func TestLoadValid(t *testing.T) {
	body := `
[[files]]
path = "VERSION"
version = "1.2.3"

[[files]]
path = "sub/pkg.json"
`
	cfgPath := writeConfig(t, body, "VERSION", "sub/pkg.json")

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}
	if len(cfg.Files) != 2 {
		t.Fatalf("len(Files) = %d, want 2", len(cfg.Files))
	}
	if cfg.Files[0].Path != "VERSION" || cfg.Files[0].Version != "1.2.3" {
		t.Errorf("Files[0] = %+v, want {VERSION 1.2.3}", cfg.Files[0])
	}
	if cfg.Files[1].Path != "sub/pkg.json" || cfg.Files[1].Version != "" {
		t.Errorf("Files[1] = %+v, want {sub/pkg.json <empty>}", cfg.Files[1])
	}
}

func TestLoadMissingFile(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "nope.toml"))
	if err == nil {
		t.Fatal("Load() of missing file = nil error, want error")
	}
	if !IsNotExist(err) {
		t.Errorf("IsNotExist(%v) = false, want true", err)
	}
}

func TestLoadInvalidTOML(t *testing.T) {
	cfgPath := writeConfig(t, "this is = = not toml")
	if _, err := Load(cfgPath); err == nil {
		t.Fatal("Load() of malformed TOML = nil error, want error")
	}
}

func TestLoadValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		targets []string
	}{
		{
			name: "no files",
			body: "# empty config\n",
		},
		{
			name: "empty path",
			body: "[[files]]\npath = \"\"\n",
		},
		{
			name:    "missing target",
			body:    "[[files]]\npath = \"VERSION\"\n",
			targets: nil, // VERSION not created
		},
		{
			name:    "duplicate path",
			body:    "[[files]]\npath = \"VERSION\"\n[[files]]\npath = \"VERSION\"\n",
			targets: []string{"VERSION"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfgPath := writeConfig(t, tt.body, tt.targets...)
			if _, err := Load(cfgPath); err == nil {
				t.Errorf("Load() = nil error, want validation error")
			}
		})
	}
}

func TestValidateRejectsDirectoryTarget(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "adir"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := &Config{Files: []FileEntry{{Path: "adir"}}}
	if err := cfg.Validate(dir); err == nil {
		t.Error("Validate() with directory target = nil error, want error")
	}
}

func TestValidateAbsolutePath(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "VERSION")
	if err := os.WriteFile(target, []byte("1.0.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := &Config{Files: []FileEntry{{Path: target}}}
	if err := cfg.Validate("/some/other/base"); err != nil {
		t.Errorf("Validate() with absolute path = %v, want nil", err)
	}
}
