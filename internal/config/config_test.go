package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sasmaq/incrmit/internal/testutil"
	"github.com/sasmaq/incrmit/internal/version"
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

// A path may be listed more than once as long as each entry pins a distinct,
// non-empty version (a file that contains several differing versions).
func TestValidateAllowsDuplicatePathDistinctVersions(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "notes.md"), []byte("1.2.3 and 2.0.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := &Config{Files: []FileEntry{
		{Path: "notes.md", Version: "1.2.3"},
		{Path: "notes.md", Version: "2.0.0"},
	}}
	if err := cfg.Validate(dir); err != nil {
		t.Errorf("Validate() = %v, want nil for distinct versions on one path", err)
	}
}

// An exact (path, version) duplicate is still rejected: it adds nothing and
// would bump the same token twice.
func TestValidateRejectsExactDuplicate(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "notes.md"), []byte("1.2.3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := &Config{Files: []FileEntry{
		{Path: "notes.md", Version: "1.2.3"},
		{Path: "notes.md", Version: "1.2.3"},
	}}
	err := cfg.Validate(dir)
	if err == nil {
		t.Fatal("Validate() = nil, want error for exact (path, version) duplicate")
	}
	if !strings.Contains(err.Error(), "duplicate path") {
		t.Errorf("Validate() error = %q, want it to mention a duplicate", err)
	}
}

// A repeated path where one entry omits the version is ambiguous and rejected,
// even if another entry for the same path pins a version.
func TestValidateRejectsDuplicatePathWithBareEntry(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "notes.md"), []byte("1.2.3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := &Config{Files: []FileEntry{
		{Path: "notes.md", Version: "1.2.3"},
		{Path: "notes.md"},
	}}
	if err := cfg.Validate(dir); err == nil {
		t.Error("Validate() = nil, want error for a bare duplicate path")
	}
}

// The same clash in the other order: a versioned entry after a bare one for the
// same path. Order must not decide whether an ambiguous config is accepted.
func TestValidateRejectsVersionedEntryAfterBare(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "notes.md"), []byte("1.2.3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := &Config{Files: []FileEntry{
		{Path: "notes.md"},
		{Path: "notes.md", Version: "1.2.3"},
	}}
	err := cfg.Validate(dir)
	if err == nil {
		t.Fatal("Validate() = nil, want error for a bare entry followed by a versioned one")
	}
	if !strings.Contains(err.Error(), "notes.md") {
		t.Errorf("err = %v, want it to name the path", err)
	}
}

// A target whose path runs through a regular file cannot be stat-ed at all.
// That is neither "missing" nor "a directory", so it takes the general branch
// and must still report the path rather than a bare OS error.
func TestValidateStatErrorOtherThanMissing(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "notafolder"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := &Config{Files: []FileEntry{{Path: "notafolder/VERSION", Version: "1.2.3"}}}

	err := cfg.Validate(dir)
	if err == nil {
		t.Fatal("Validate() = nil, want an error for a path under a regular file")
	}
	if !strings.Contains(err.Error(), "notafolder/VERSION") {
		t.Errorf("err = %v, want it to name the target", err)
	}
}

// Validation errors should name the specific problem so users can fix the
// config without guessing.
func TestValidateErrorMessages(t *testing.T) {
	dir := t.TempDir()
	// The duplicate check only fires once the first entry passes the existence
	// check, so the target must exist on disk.
	if err := os.WriteFile(filepath.Join(dir, "VERSION"), []byte("1.0.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		cfg  *Config
		want string
	}{
		{"no files", &Config{}, "no files listed"},
		{"empty path", &Config{Files: []FileEntry{{Path: ""}}}, "empty path"},
		{
			"duplicate",
			&Config{Files: []FileEntry{{Path: "VERSION"}, {Path: "VERSION"}}},
			"duplicate path",
		},
		{"missing", &Config{Files: []FileEntry{{Path: "nope"}}}, "does not exist"},
		{"directory", &Config{Files: []FileEntry{{Path: "."}}}, "is a directory"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate(dir)
			if err == nil {
				t.Fatalf("Validate() = nil, want error containing %q", tt.want)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("Validate() error = %q, want substring %q", err, tt.want)
			}
		})
	}
}

// A target that exists but is not an ordinary file must be rejected during
// validation, naming the config entry at fault. Validate only stats the path, so
// it never blocks on the pipe; the test's own deadline catches a regression that
// opened it instead.
func TestValidateRejectsNonRegularTarget(t *testing.T) {
	dir := t.TempDir()
	if err := testutil.Mkfifo(filepath.Join(dir, "pipe")); err != nil {
		t.Skipf("FIFOs unsupported: %v", err)
	}

	done := make(chan error, 1)
	go func() {
		cfg := &Config{Files: []FileEntry{{Path: "pipe"}}}
		done <- cfg.Validate(dir)
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("Validate() = nil, want error for a named pipe target")
		}
		if !strings.Contains(err.Error(), "is not a regular file") {
			t.Errorf("Validate() error = %q, want substring %q", err, "is not a regular file")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Validate blocked on a named pipe instead of rejecting it")
	}
}

// Load must distinguish a missing file (NotExistError) from other read errors,
// such as the path resolving to a directory.
func TestLoadReadErrorNotMissing(t *testing.T) {
	dir := t.TempDir()
	_, err := Load(dir) // a directory, not a file
	if err == nil {
		t.Fatal("Load(dir) = nil error, want error")
	}
	if IsNotExist(err) {
		t.Errorf("IsNotExist(%v) = true, want false (not a missing-file error)", err)
	}
}

func TestNotExistErrorMessage(t *testing.T) {
	err := &NotExistError{Path: "some/incrmit.toml"}
	msg := err.Error()
	if !strings.Contains(msg, "some/incrmit.toml") || !strings.Contains(msg, "incrmit discover") {
		t.Errorf("Error() = %q, want it to mention the path and `incrmit discover`", msg)
	}
}

func TestMarshalRoundTrip(t *testing.T) {
	cfg := &Config{Files: []FileEntry{
		{Path: "VERSION", Version: "1.2.3"},
		{Path: "sub/pkg.json", Version: "1.2.3"},
	}}

	data, err := Marshal(cfg)
	if err != nil {
		t.Fatalf("Marshal() error: %v", err)
	}
	if !strings.HasPrefix(string(data), "# incrmit.toml (maintained by incrmit)") {
		t.Errorf("Marshal() output missing header:\n%s", data)
	}

	// Write it out and load it back to confirm it is valid, parseable TOML.
	dir := t.TempDir()
	for _, f := range cfg.Files {
		p := filepath.Join(dir, f.Path)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("1.2.3\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cfgPath := filepath.Join(dir, "incrmit.toml")
	if err := os.WriteFile(cfgPath, data, 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() of marshaled config: %v", err)
	}
	if len(got.Files) != 2 || got.Files[0] != cfg.Files[0] || got.Files[1] != cfg.Files[1] {
		t.Errorf("round-trip mismatch: got %+v, want %+v", got.Files, cfg.Files)
	}
}

// The top-level ignore list is loaded, and separators are normalized to
// forward slashes so patterns compare consistently across platforms.
func TestLoadIgnore(t *testing.T) {
	body := `
ignore = ["docs\\generated", "  *.lock  ", "testdata/"]

[[files]]
path = "VERSION"
`
	cfgPath := writeConfig(t, body, "VERSION")
	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}
	want := []string{"docs/generated", "*.lock", "testdata/"}
	if len(cfg.Ignore) != len(want) {
		t.Fatalf("Ignore = %v, want %v", cfg.Ignore, want)
	}
	for i := range want {
		if cfg.Ignore[i] != want[i] {
			t.Errorf("Ignore[%d] = %q, want %q", i, cfg.Ignore[i], want[i])
		}
	}
}

// An empty (or whitespace-only) ignore pattern is rejected with a clear error.
func TestLoadIgnoreRejectsEmptyPattern(t *testing.T) {
	body := "ignore = [\"\", \"VERSION\"]\n[[files]]\npath = \"VERSION\"\n"
	cfgPath := writeConfig(t, body, "VERSION")
	_, err := Load(cfgPath)
	if err == nil {
		t.Fatal("Load() = nil error, want error for empty ignore pattern")
	}
	if !strings.Contains(err.Error(), "ignore") {
		t.Errorf("error = %q, want it to mention ignore", err)
	}
}

// LoadIgnore returns just the ignore list without requiring the listed targets
// to exist, so discover can read patterns from a stale config.
func TestLoadIgnoreStandalone(t *testing.T) {
	body := "ignore = [\"docs/**\"]\n[[files]]\npath = \"does-not-exist\"\n"
	cfgPath := writeConfig(t, body) // no targets created
	got, err := LoadIgnore(cfgPath)
	if err != nil {
		t.Fatalf("LoadIgnore() error: %v", err)
	}
	if len(got) != 1 || got[0] != "docs/**" {
		t.Errorf("LoadIgnore() = %v, want [docs/**]", got)
	}
}

// A missing or unparseable file yields no patterns and no error, since discover
// overwrites its --output and may point it at a non-config file.
func TestLoadIgnoreLenient(t *testing.T) {
	if got, err := LoadIgnore(filepath.Join(t.TempDir(), "nope.toml")); err != nil || got != nil {
		t.Errorf("LoadIgnore(missing) = (%v, %v), want (nil, nil)", got, err)
	}

	bad := filepath.Join(t.TempDir(), "conf.cfg")
	if err := os.WriteFile(bad, []byte("1.0.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, err := LoadIgnore(bad); err != nil || got != nil {
		t.Errorf("LoadIgnore(non-config) = (%v, %v), want (nil, nil)", got, err)
	}
}

// Being lenient about a missing or non-config file must not extend to a config
// that exists and cannot be read: silently ignoring it would drop the user's
// ignore list and scan the directories they asked to skip.
func TestLoadIgnoreReportsReadErrors(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root: file permissions are not enforced")
	}
	path := filepath.Join(t.TempDir(), "incrmit.toml")
	if err := os.WriteFile(path, []byte("ignore = [\"docs/\"]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o644) })

	got, err := LoadIgnore(path)
	if err == nil {
		t.Fatal("LoadIgnore() = nil error for an unreadable config")
	}
	if got != nil {
		t.Errorf("patterns = %v, want nil alongside the error", got)
	}
	if !strings.Contains(err.Error(), "incrmit.toml") {
		t.Errorf("err = %v, want it to name the config", err)
	}
}

// The ignore list round-trips through Marshal, and must be encoded before the
// [[files]] table to stay valid TOML.
func TestMarshalIgnoreRoundTrip(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "VERSION"), []byte("1.2.3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := &Config{
		Ignore: []string{"docs/**", "*.lock"},
		Files:  []FileEntry{{Path: "VERSION", Version: "1.2.3"}},
	}
	data, err := Marshal(cfg)
	if err != nil {
		t.Fatalf("Marshal() error: %v", err)
	}
	out := string(data)
	if strings.Index(out, "ignore =") > strings.Index(out, "[[files]]") {
		t.Fatalf("ignore must be encoded before [[files]]:\n%s", out)
	}

	cfgPath := filepath.Join(dir, "incrmit.toml")
	if err := os.WriteFile(cfgPath, data, 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() of marshaled config: %v", err)
	}
	if len(got.Ignore) != 2 || got.Ignore[0] != "docs/**" || got.Ignore[1] != "*.lock" {
		t.Errorf("round-trip ignore = %v, want [docs/** *.lock]", got.Ignore)
	}
}

// When a config has no ignore patterns, Marshal (used by bump's rewrite) still
// documents the option with a commented-out example, and writes no active key.
func TestMarshalWritesCommentedIgnoreExample(t *testing.T) {
	data, err := Marshal(&Config{Files: []FileEntry{{Path: "VERSION"}}})
	if err != nil {
		t.Fatalf("Marshal() error: %v", err)
	}
	out := string(data)
	if !strings.Contains(out, "# ignore:") {
		t.Errorf("Marshal() missing the ignore description comment:\n%s", out)
	}
	if !strings.Contains(out, `# ignore = ["testdata/", "*.lock", "docs/**"]`) {
		t.Errorf("Marshal() missing the commented ignore example:\n%s", out)
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "ignore =") {
			t.Errorf("Marshal() emitted an active ignore key for an empty list: %q", line)
		}
	}
}

// An empty Version must be omitted from the encoded output (omitempty), not
// written as version = "".
func TestMarshalOmitsEmptyVersion(t *testing.T) {
	data, err := Marshal(&Config{Files: []FileEntry{{Path: "VERSION"}}})
	if err != nil {
		t.Fatalf("Marshal() error: %v", err)
	}
	if strings.Contains(string(data), "version") {
		t.Errorf("Marshal() included an empty version field:\n%s", data)
	}
}

// A version's prerelease and build sections live in their own keys, and Token
// reassembles the whole thing.
func TestFileEntryToken(t *testing.T) {
	tests := []struct {
		entry FileEntry
		want  string
	}{
		{FileEntry{Path: "p"}, ""},
		{FileEntry{Path: "p", Version: "1.2.3"}, "1.2.3"},
		{FileEntry{Path: "p", Version: "v1.2.3"}, "v1.2.3"},
		{FileEntry{Path: "p", Version: "1.2.3", Prerelease: "rc.1"}, "1.2.3-rc.1"},
		{FileEntry{Path: "p", Version: "1.2.3", Build: "build.7"}, "1.2.3+build.7"},
		{FileEntry{Path: "p", Version: "1.2.3", Prerelease: "rc.1", Build: "b.7"}, "1.2.3-rc.1+b.7"},
		// A prerelease with no version pins nothing at all.
		{FileEntry{Path: "p", Prerelease: "rc.1"}, ""},
	}
	for _, tt := range tests {
		if got := tt.entry.Token(); got != tt.want {
			t.Errorf("%+v.Token() = %q, want %q", tt.entry, got, tt.want)
		}
	}
}

// SetToken is Token's inverse: whatever Token assembles has to split back into
// the same three keys, so a config round-trips through either direction.
func TestFileEntrySetToken(t *testing.T) {
	tests := []struct {
		name  string
		token string
		want  FileEntry
	}{
		{"bare", "1.2.3", FileEntry{Path: "p", Version: "1.2.3"}},
		{"prefixed", "v1.2.3", FileEntry{Path: "p", Version: "v1.2.3"}},
		{"prerelease", "1.2.3-rc.1", FileEntry{Path: "p", Version: "1.2.3", Prerelease: "rc.1"}},
		{"build", "1.2.3+build.7", FileEntry{Path: "p", Version: "1.2.3", Build: "build.7"}},
		{
			"both sections",
			"v1.2.3-rc.1+build.7",
			FileEntry{Path: "p", Version: "v1.2.3", Prerelease: "rc.1", Build: "build.7"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := FileEntry{Path: "p"}
			entry.SetToken(tt.token)
			if entry != tt.want {
				t.Errorf("SetToken(%q) = %+v, want %+v", tt.token, entry, tt.want)
			}
			if got := entry.Token(); got != tt.token {
				t.Errorf("Token() = %q, want the token it was set from (%q)", got, tt.token)
			}
		})
	}
}

// A token that does not parse is kept verbatim rather than dropped, so the
// command that needs it reports the bad value instead of an empty one. Any
// sections left from a previous value are cleared, so no half-updated entry
// survives.
func TestFileEntrySetTokenKeepsUnparseableValue(t *testing.T) {
	entry := FileEntry{Path: "p", Version: "1.2.3", Prerelease: "rc.1", Build: "build.7"}
	entry.SetToken("not-a-version")

	want := FileEntry{Path: "p", Version: "not-a-version"}
	if entry != want {
		t.Errorf("SetToken(bad) = %+v, want %+v", entry, want)
	}
}

// EntryFor splits a version across the keys, and a version with no prerelease
// leaves the key out entirely rather than writing an empty one.
func TestEntryForSplitsVersion(t *testing.T) {
	v, err := version.Parse("v1.2.3-rc.1+build.7")
	if err != nil {
		t.Fatal(err)
	}
	got := EntryFor("VERSION", v)
	want := FileEntry{Path: "VERSION", Version: "v1.2.3", Prerelease: "rc.1", Build: "build.7"}
	if got != want {
		t.Errorf("EntryFor = %+v, want %+v", got, want)
	}

	// Bumping past a prerelease clears both keys.
	got.SetVersion(v.BumpPatch())
	want = FileEntry{Path: "VERSION", Version: "v1.2.4"}
	if got != want {
		t.Errorf("after a patch bump = %+v, want %+v", got, want)
	}
}

// Configs written before the keys existed spelled the whole token in `version`.
// Load splits them, so an old config behaves exactly like a new one.
func TestLoadSplitsInlineToken(t *testing.T) {
	path := writeConfig(t, `
[[files]]
  path = "VERSION"
  version = "v1.2.3-rc.1+build.7"
`, "VERSION")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	got := cfg.Files[0]
	want := FileEntry{Path: "VERSION", Version: "v1.2.3", Prerelease: "rc.1", Build: "build.7"}
	if got != want {
		t.Errorf("entry = %+v, want %+v", got, want)
	}
	if tok := got.Token(); tok != "v1.2.3-rc.1+build.7" {
		t.Errorf("Token() = %q, want the original token back", tok)
	}
}

// Spelling a section both ways is ambiguous, so it is rejected rather than
// guessed at.
func TestLoadRejectsDuplicateSuffixSpelling(t *testing.T) {
	path := writeConfig(t, `
[[files]]
  path = "VERSION"
  version = "1.2.3-rc.1"
  prerelease = "rc.2"
`, "VERSION")

	_, err := Load(path)
	if err == nil {
		t.Fatal("Load succeeded, want an error")
	}
	if !strings.Contains(err.Error(), "keep one") {
		t.Errorf("error = %v, want it to explain the conflict", err)
	}
}

// A prerelease key with no version pins nothing; say so rather than silently
// ignoring it.
func TestLoadRejectsPrereleaseWithoutVersion(t *testing.T) {
	path := writeConfig(t, `
[[files]]
  path = "VERSION"
  prerelease = "rc.1"
`, "VERSION")

	_, err := Load(path)
	if err == nil {
		t.Fatal("Load succeeded, want an error")
	}
	if !strings.Contains(err.Error(), "without a version") {
		t.Errorf("error = %v, want it to name the missing version", err)
	}
}

// A version that does not parse is left exactly as written, so the command that
// needs it reports the problem (exit 3) instead of Load failing as a config
// error (exit 1).
func TestLoadKeepsUnparseableVersion(t *testing.T) {
	path := writeConfig(t, `
[[files]]
  path = "VERSION"
  version = "not-a-version"
`, "VERSION")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := cfg.Files[0].Version; got != "not-a-version" {
		t.Errorf("Version = %q, want it left as written", got)
	}
}

// A path may repeat once per distinct version, and the prerelease is part of
// what makes two entries distinct.
func TestValidateDistinguishesEntriesByPrerelease(t *testing.T) {
	path := writeConfig(t, `
[[files]]
  path = "VERSION"
  version = "1.2.3"

[[files]]
  path = "VERSION"
  version = "1.2.3"
  prerelease = "rc.1"
`, "VERSION")

	if _, err := Load(path); err != nil {
		t.Fatalf("Load: %v", err)
	}
}

// Two entries that pin the very same token are still duplicates.
func TestValidateRejectsDuplicateSplitEntries(t *testing.T) {
	path := writeConfig(t, `
[[files]]
  path = "VERSION"
  version = "1.2.3"
  prerelease = "rc.1"

[[files]]
  path = "VERSION"
  version = "1.2.3"
  prerelease = "rc.1"
`, "VERSION")

	_, err := Load(path)
	if err == nil {
		t.Fatal("Load succeeded, want a duplicate error")
	}
	if !strings.Contains(err.Error(), "1.2.3-rc.1") {
		t.Errorf("error = %v, want the full token named", err)
	}
}
