package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sasmaq/incrmit/internal/config"
	"github.com/sasmaq/incrmit/internal/testutil"
)

// runMain invokes Main with the given args inside dir (as the working
// directory) and returns the exit code plus captured stdout/stderr.
func runMain(t *testing.T, dir string, args ...string) (int, string, string) {
	t.Helper()
	if dir != "" {
		cwd, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Chdir(dir); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chdir(cwd) })
	}
	var stdout, stderr bytes.Buffer
	code := Main(args, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

// project writes a config plus targets (name -> contents) into a temp dir and
// returns the dir.
func project(t *testing.T, configBody string, targets map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range targets {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if configBody != "" {
		if err := os.WriteFile(filepath.Join(dir, "incrmit.toml"), []byte(configBody), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestBumpDefaultPatch(t *testing.T) {
	dir := project(t, "[[files]]\npath = \"VERSION\"\n", map[string]string{"VERSION": "1.2.3\n"})

	code, stdout, stderr := runMain(t, dir)
	if code != ExitOK {
		t.Fatalf("exit = %d, stderr = %q", code, stderr)
	}
	if got, err := os.ReadFile(filepath.Join(dir, "VERSION")); err != nil || string(got) != "1.2.4\n" {
		t.Errorf("VERSION = %q (err %v), want \"1.2.4\\n\"", got, err)
	}
	if !strings.Contains(stdout, "patch bump") || !strings.Contains(stdout, "1.2.3 -> 1.2.4") {
		t.Errorf("stdout missing summary: %q", stdout)
	}
}

// A "v"-prefixed token bumps to a "v"-prefixed token, and the config self-sync
// records the prefixed version so repeated runs stay consistent.
func TestBumpPreservesVPrefix(t *testing.T) {
	body := "[[files]]\npath = \"VERSION\"\nversion = \"v1.2.3\"\n"
	dir := project(t, body, map[string]string{"VERSION": "v1.2.3\n"})

	code, stdout, stderr := runMain(t, dir, "--minor")
	if code != ExitOK {
		t.Fatalf("exit = %d, stderr = %q", code, stderr)
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "VERSION")); string(got) != "v1.3.0\n" {
		t.Errorf("VERSION = %q, want \"v1.3.0\\n\"", got)
	}
	if !strings.Contains(stdout, "v1.2.3 -> v1.3.0") {
		t.Errorf("stdout missing prefixed summary: %q", stdout)
	}
	cfg, err := config.Load(filepath.Join(dir, "incrmit.toml"))
	if err != nil {
		t.Fatalf("loading config: %v", err)
	}
	if cfg.Files[0].Version != "v1.3.0" {
		t.Errorf("config version = %q, want v1.3.0", cfg.Files[0].Version)
	}
}

// A single file listed under two config entries (one per distinct version it
// contains) has both versions bumped in one run, and the self-maintained config
// keeps both entries updated to their new values.
func TestBumpFileWithMultipleVersions(t *testing.T) {
	body := "[[files]]\npath = \"notes.md\"\nversion = \"1.2.3\"\n" +
		"[[files]]\npath = \"notes.md\"\nversion = \"2.0.0\"\n"
	dir := project(t, body, map[string]string{
		"notes.md": "app 1.2.3\nlib 2.0.0\napp 1.2.3 again\n",
	})

	code, stdout, stderr := runMain(t, dir)
	if code != ExitOK {
		t.Fatalf("exit = %d, stderr = %q", code, stderr)
	}
	got, _ := os.ReadFile(filepath.Join(dir, "notes.md"))
	if want := "app 1.2.4\nlib 2.0.1\napp 1.2.4 again\n"; string(got) != want {
		t.Errorf("notes.md = %q, want %q", got, want)
	}
	if !strings.Contains(stdout, "1.2.3 -> 1.2.4") || !strings.Contains(stdout, "2.0.0 -> 2.0.1") {
		t.Errorf("stdout missing a per-version summary: %q", stdout)
	}
	if !strings.Contains(stdout, "to 1 file(s)") {
		t.Errorf("stdout should count the file once: %q", stdout)
	}

	cfg, err := config.Load(filepath.Join(dir, "incrmit.toml"))
	if err != nil {
		t.Fatalf("loading config: %v", err)
	}
	if len(cfg.Files) != 2 {
		t.Fatalf("config has %d entries, want 2: %+v", len(cfg.Files), cfg.Files)
	}
	wantVers := map[string]bool{"1.2.4": false, "2.0.1": false}
	for _, f := range cfg.Files {
		if f.Path != "notes.md" {
			t.Errorf("entry path = %q, want notes.md", f.Path)
		}
		if _, ok := wantVers[f.Version]; !ok {
			t.Errorf("unexpected entry version %q", f.Version)
			continue
		}
		wantVers[f.Version] = true
	}
	for v, seen := range wantVers {
		if !seen {
			t.Errorf("config missing self-bumped version %q", v)
		}
	}
}

// discover then bump end to end: a file with two differing versions is
// discovered into two entries, and a subsequent bump updates both occurrences.
func TestDiscoverThenBumpMultipleVersions(t *testing.T) {
	dir := project(t, "", map[string]string{
		"notes.md": "release 1.2.3\nlegacy 2.0.0\n",
	})

	if code, _, stderr := runMain(t, dir, "discover"); code != ExitOK {
		t.Fatalf("discover exit = %d, stderr = %q", code, stderr)
	}
	cfg, err := config.Load(filepath.Join(dir, "incrmit.toml"))
	if err != nil {
		t.Fatalf("loading config: %v", err)
	}
	if len(cfg.Files) != 2 {
		t.Fatalf("discovered config has %d entries, want 2: %+v", len(cfg.Files), cfg.Files)
	}

	if code, _, stderr := runMain(t, dir, "--minor"); code != ExitOK {
		t.Fatalf("bump exit = %d, stderr = %q", code, stderr)
	}
	got, _ := os.ReadFile(filepath.Join(dir, "notes.md"))
	if want := "release 1.3.0\nlegacy 2.1.0\n"; string(got) != want {
		t.Errorf("notes.md = %q, want %q", got, want)
	}
}

// In --file mode (no config) a bare "v" prefix in the file is detected and
// preserved through the bump.
func TestBumpFileModePreservesVPrefix(t *testing.T) {
	dir := project(t, "", map[string]string{"VERSION": "current V0.9.0\n"})
	target := filepath.Join(dir, "VERSION")

	code, stdout, stderr := runMain(t, "", "--file", target, "--major")
	if code != ExitOK {
		t.Fatalf("exit = %d, stderr = %q", code, stderr)
	}
	if got, _ := os.ReadFile(target); string(got) != "current V1.0.0\n" {
		t.Errorf("VERSION = %q, want \"current V1.0.0\\n\"", got)
	}
	if !strings.Contains(stdout, "V0.9.0 -> V1.0.0") {
		t.Errorf("stdout = %q", stdout)
	}
}

// discover --dry-run surfaces the "v" prefix in its findings, and the generated
// config carries it so a subsequent bump preserves it end to end.
func TestDiscoverThenBumpVPrefix(t *testing.T) {
	dir := project(t, "", map[string]string{"VERSION": "v2.3.4\n"})

	code, stdout, stderr := runMain(t, dir, "discover", "--dry-run")
	if code != ExitOK {
		t.Fatalf("discover dry-run exit = %d, stderr = %q", code, stderr)
	}
	if !strings.Contains(stdout, "v2.3.4") {
		t.Errorf("dry-run stdout = %q, want prefixed finding", stdout)
	}

	if code, _, stderr := runMain(t, dir, "discover"); code != ExitOK {
		t.Fatalf("discover exit = %d, stderr = %q", code, stderr)
	}
	cfg, err := config.Load(filepath.Join(dir, "incrmit.toml"))
	if err != nil {
		t.Fatalf("loading config: %v", err)
	}
	if cfg.Files[0].Version != "v2.3.4" {
		t.Errorf("generated config version = %q, want v2.3.4", cfg.Files[0].Version)
	}

	if code, _, stderr := runMain(t, dir); code != ExitOK {
		t.Fatalf("bump exit = %d, stderr = %q", code, stderr)
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "VERSION")); string(got) != "v2.3.5\n" {
		t.Errorf("VERSION = %q, want \"v2.3.5\\n\"", got)
	}
}

func TestBumpComponentResolution(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"patch-default", []string{}, "1.2.4\n"},
		{"explicit-patch", []string{"--patch"}, "1.2.4\n"},
		{"minor", []string{"--minor"}, "1.3.0\n"},
		{"major", []string{"--major"}, "2.0.0\n"},
		{"major-wins-over-minor", []string{"--minor", "--major"}, "2.0.0\n"},
		{"minor-wins-over-patch", []string{"--patch", "--minor"}, "1.3.0\n"},
		{"short-major", []string{"-M"}, "2.0.0\n"},
		{"short-minor", []string{"-m"}, "1.3.0\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := project(t, "[[files]]\npath = \"VERSION\"\n", map[string]string{"VERSION": "1.2.3\n"})
			code, _, stderr := runMain(t, dir, tt.args...)
			if code != ExitOK {
				t.Fatalf("exit = %d, stderr = %q", code, stderr)
			}
			got, _ := os.ReadFile(filepath.Join(dir, "VERSION"))
			if string(got) != tt.want {
				t.Errorf("VERSION = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBumpMultipleFiles(t *testing.T) {
	body := "[[files]]\npath = \"VERSION\"\n[[files]]\npath = \"sub/pkg.json\"\n"
	dir := project(t, body, map[string]string{
		"VERSION":      "1.2.3\n",
		"sub/pkg.json": "{\n  \"version\": \"1.2.3\"\n}\n",
	})

	code, stdout, stderr := runMain(t, dir, "--minor")
	if code != ExitOK {
		t.Fatalf("exit = %d, stderr = %q", code, stderr)
	}
	v, _ := os.ReadFile(filepath.Join(dir, "VERSION"))
	p, _ := os.ReadFile(filepath.Join(dir, "sub/pkg.json"))
	if string(v) != "1.3.0\n" {
		t.Errorf("VERSION = %q", v)
	}
	if !strings.Contains(string(p), "\"version\": \"1.3.0\"") {
		t.Errorf("pkg.json = %q", p)
	}
	if !strings.Contains(stdout, "2 file(s)") {
		t.Errorf("stdout = %q", stdout)
	}
}

func TestBumpSingleFileFlag(t *testing.T) {
	dir := project(t, "", map[string]string{"CHANGES": "current: 0.9.0\n"})
	target := filepath.Join(dir, "CHANGES")

	code, stdout, stderr := runMain(t, "", "--file", target, "--major")
	if code != ExitOK {
		t.Fatalf("exit = %d, stderr = %q", code, stderr)
	}
	got, _ := os.ReadFile(target)
	if string(got) != "current: 1.0.0\n" {
		t.Errorf("CHANGES = %q, want \"current: 1.0.0\\n\"", got)
	}
	if !strings.Contains(stdout, "0.9.0 -> 1.0.0") {
		t.Errorf("stdout = %q", stdout)
	}
}

func TestBumpDryRun(t *testing.T) {
	dir := project(t, "[[files]]\npath = \"VERSION\"\n", map[string]string{"VERSION": "1.2.3\n"})

	code, stdout, stderr := runMain(t, dir, "--minor", "--dry-run")
	if code != ExitOK {
		t.Fatalf("exit = %d, stderr = %q", code, stderr)
	}
	got, _ := os.ReadFile(filepath.Join(dir, "VERSION"))
	if string(got) != "1.2.3\n" {
		t.Errorf("dry-run modified file: %q", got)
	}
	if !strings.Contains(stdout, "Dry run") || !strings.Contains(stdout, "1.2.3 -> 1.3.0") {
		t.Errorf("stdout = %q", stdout)
	}
}

func TestBumpSyncsConfigVersions(t *testing.T) {
	body := "[[files]]\npath = \"VERSION\"\nversion = \"1.2.3\"\n[[files]]\npath = \"pkg.json\"\nversion = \"1.2.3\"\n"
	dir := project(t, body, map[string]string{
		"VERSION":  "1.2.3\n",
		"pkg.json": "{\n  \"version\": \"1.2.3\"\n}\n",
	})

	code, _, stderr := runMain(t, dir, "--minor")
	if code != ExitOK {
		t.Fatalf("exit = %d, stderr = %q", code, stderr)
	}

	cfg, err := config.Load(filepath.Join(dir, "incrmit.toml"))
	if err != nil {
		t.Fatalf("loading config: %v", err)
	}
	for _, f := range cfg.Files {
		if f.Version != "1.3.0" {
			t.Errorf("config %s version = %q, want 1.3.0", f.Path, f.Version)
		}
	}
}

// A repeated bump only works when the config's recorded version is updated to
// match the file after each run; otherwise the second run cannot find the old
// version in the changed file.
func TestBumpRepeatedRunsStayInSync(t *testing.T) {
	body := "[[files]]\npath = \"VERSION\"\nversion = \"1.2.3\"\n"
	dir := project(t, body, map[string]string{"VERSION": "1.2.3\n"})

	for run := 1; run <= 3; run++ {
		code, _, stderr := runMain(t, dir)
		if code != ExitOK {
			t.Fatalf("run %d: exit = %d, stderr = %q", run, code, stderr)
		}
	}
	got, _ := os.ReadFile(filepath.Join(dir, "VERSION"))
	if string(got) != "1.2.6\n" {
		t.Errorf("VERSION = %q, want \"1.2.6\\n\"", got)
	}
	cfg, _ := config.Load(filepath.Join(dir, "incrmit.toml"))
	if cfg.Files[0].Version != "1.2.6" {
		t.Errorf("config version = %q, want 1.2.6", cfg.Files[0].Version)
	}
}

func TestBumpDryRunLeavesConfigUntouched(t *testing.T) {
	body := "[[files]]\npath = \"VERSION\"\nversion = \"1.2.3\"\n"
	dir := project(t, body, map[string]string{"VERSION": "1.2.3\n"})
	before, _ := os.ReadFile(filepath.Join(dir, "incrmit.toml"))

	code, _, stderr := runMain(t, dir, "--minor", "--dry-run")
	if code != ExitOK {
		t.Fatalf("exit = %d, stderr = %q", code, stderr)
	}
	after, _ := os.ReadFile(filepath.Join(dir, "incrmit.toml"))
	if string(before) != string(after) {
		t.Errorf("dry-run rewrote config:\nbefore %q\nafter  %q", before, after)
	}
}

// --file mode has no config to maintain; the bump must not create one.
func TestBumpFileModeDoesNotWriteConfig(t *testing.T) {
	dir := project(t, "", map[string]string{"VERSION": "1.2.3\n"})
	code, _, stderr := runMain(t, "", "--file", filepath.Join(dir, "VERSION"))
	if code != ExitOK {
		t.Fatalf("exit = %d, stderr = %q", code, stderr)
	}
	if _, err := os.Stat(filepath.Join(dir, "incrmit.toml")); !os.IsNotExist(err) {
		t.Errorf("--file mode created a config (err = %v)", err)
	}
}

func TestBumpDryRunShort(t *testing.T) {
	dir := project(t, "[[files]]\npath = \"VERSION\"\n", map[string]string{"VERSION": "1.2.3\n"})
	code, stdout, _ := runMain(t, dir, "-d")
	if code != ExitOK {
		t.Fatalf("exit = %d", code)
	}
	if !strings.Contains(stdout, "Dry run") {
		t.Errorf("stdout = %q", stdout)
	}
	got, _ := os.ReadFile(filepath.Join(dir, "VERSION"))
	if string(got) != "1.2.3\n" {
		t.Errorf("file changed: %q", got)
	}
}

func TestBumpCustomConfigPath(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "VERSION"), []byte("1.2.3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := filepath.Join(dir, "custom.toml")
	if err := os.WriteFile(cfg, []byte("[[files]]\npath = \"VERSION\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	code, _, stderr := runMain(t, "", "--config", cfg)
	if code != ExitOK {
		t.Fatalf("exit = %d, stderr = %q", code, stderr)
	}
	got, _ := os.ReadFile(filepath.Join(dir, "VERSION"))
	if string(got) != "1.2.4\n" {
		t.Errorf("VERSION = %q", got)
	}
}

func TestBumpMissingConfig(t *testing.T) {
	dir := t.TempDir()
	code, _, stderr := runMain(t, dir)
	if code != ExitError {
		t.Errorf("exit = %d, want %d", code, ExitError)
	}
	if !strings.Contains(stderr, "discover") {
		t.Errorf("stderr should suggest discover: %q", stderr)
	}
}

func TestBumpNoVersionInFile(t *testing.T) {
	dir := project(t, "", map[string]string{"F": "no version here\n"})
	code, _, stderr := runMain(t, "", "--file", filepath.Join(dir, "F"))
	if code != ExitNoVersion {
		t.Errorf("exit = %d, want %d (stderr %q)", code, ExitNoVersion, stderr)
	}
}

func TestBumpAmbiguousFile(t *testing.T) {
	dir := project(t, "", map[string]string{"F": "a=1.2.3 b=4.5.6\n"})
	code, _, stderr := runMain(t, "", "--file", filepath.Join(dir, "F"))
	if code != ExitNoVersion {
		t.Errorf("exit = %d, want %d", code, ExitNoVersion)
	}
	if !strings.Contains(stderr, "ambiguous") {
		t.Errorf("stderr = %q", stderr)
	}
}

func TestBumpFailFastDoesNotPartiallyWrite(t *testing.T) {
	body := "[[files]]\npath = \"good\"\n[[files]]\npath = \"bad\"\n"
	dir := project(t, body, map[string]string{
		"good": "1.2.3\n",
		"bad":  "no version\n",
	})
	code, _, _ := runMain(t, dir)
	if code != ExitNoVersion {
		t.Fatalf("exit = %d, want %d", code, ExitNoVersion)
	}
	// The good file must be untouched because planning failed before writing.
	got, _ := os.ReadFile(filepath.Join(dir, "good"))
	if string(got) != "1.2.3\n" {
		t.Errorf("good file was modified despite fail-fast: %q", got)
	}
}

// A bump reads its targets whatever their size unless --max-file-size asks for
// a cap; a target over the cap is reported (naming the flag) and, because the
// check happens while planning, no other file is written.
func TestBumpMaxFileSize(t *testing.T) {
	body := "[[files]]\npath = \"VERSION\"\n[[files]]\npath = \"notes.txt\"\nversion = \"2.0.0\"\n"
	targets := map[string]string{
		"VERSION":   "1.2.3\n",
		"notes.txt": strings.Repeat("x", 4096) + "\n2.0.0\n",
	}

	t.Run("no cap by default", func(t *testing.T) {
		dir := project(t, body, targets)
		code, stdout, stderr := runMain(t, dir)
		if code != ExitOK {
			t.Fatalf("exit = %d, stderr = %q", code, stderr)
		}
		if !strings.Contains(stdout, "2.0.0 -> 2.0.1") {
			t.Errorf("stdout = %q, want the large file bumped", stdout)
		}
	})

	t.Run("cap above the file", func(t *testing.T) {
		dir := project(t, body, targets)
		code, stdout, stderr := runMain(t, dir, "--max-file-size", "1MiB")
		if code != ExitOK {
			t.Fatalf("exit = %d, stderr = %q", code, stderr)
		}
		if !strings.Contains(stdout, "2.0.0 -> 2.0.1") {
			t.Errorf("stdout = %q, want the large file bumped", stdout)
		}
	})

	for _, flag := range []string{"--max-file-size", "-s"} {
		t.Run("cap below the file "+flag, func(t *testing.T) {
			dir := project(t, body, targets)
			code, _, stderr := runMain(t, dir, flag, "1KiB")
			if code != ExitError {
				t.Fatalf("exit = %d, want %d (stderr %q)", code, ExitError, stderr)
			}
			if !strings.Contains(stderr, "notes.txt") || !strings.Contains(stderr, "--max-file-size limit of 1KiB") {
				t.Errorf("stderr = %q, want it to name the file and the limit", stderr)
			}
			// Planning fails before any write, so the other target keeps its
			// version and the oversized file is left alone.
			if got, _ := os.ReadFile(filepath.Join(dir, "VERSION")); string(got) != "1.2.3\n" {
				t.Errorf("VERSION = %q, want it untouched", got)
			}
			if got, _ := os.ReadFile(filepath.Join(dir, "notes.txt")); !strings.Contains(string(got), "2.0.0") {
				t.Errorf("notes.txt was modified despite being over the cap")
			}
		})
	}
}

// A bad --max-file-size is a usage error, reported before any file is read.
func TestBumpBadMaxFileSize(t *testing.T) {
	dir := project(t, "[[files]]\npath = \"VERSION\"\n", map[string]string{"VERSION": "1.2.3\n"})
	code, _, stderr := runMain(t, dir, "--max-file-size", "-4")
	if code != ExitUsage {
		t.Fatalf("exit = %d, want %d", code, ExitUsage)
	}
	if !strings.Contains(stderr, "0 for no limit") {
		t.Errorf("stderr = %q, want the hint that 0 disables the limit", stderr)
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "VERSION")); string(got) != "1.2.3\n" {
		t.Errorf("VERSION = %q, want it untouched", got)
	}
}

func TestBadFlag(t *testing.T) {
	code, _, _ := runMain(t, "", "--nope")
	if code != ExitUsage {
		t.Errorf("exit = %d, want %d", code, ExitUsage)
	}
}

// A stray positional argument in bump context (after a flag, so it is not
// treated as a subcommand) is reported as an unexpected argument.
func TestUnexpectedArg(t *testing.T) {
	code, _, stderr := runMain(t, "", "--minor", "surprise")
	if code != ExitUsage {
		t.Errorf("exit = %d, want %d", code, ExitUsage)
	}
	if !strings.Contains(stderr, "unexpected argument") {
		t.Errorf("stderr = %q", stderr)
	}
}

func TestDiscoverWritesConfig(t *testing.T) {
	dir := project(t, "", map[string]string{
		"VERSION":      "1.2.3\n",
		"sub/pkg.json": "ignored: not a package.json name\n",
	})

	code, stdout, stderr := runMain(t, dir, "discover")
	if code != ExitOK {
		t.Fatalf("exit = %d, stderr = %q", code, stderr)
	}
	if !strings.Contains(stdout, "Wrote incrmit.toml") {
		t.Errorf("stdout = %q", stdout)
	}

	cfg, err := config.Load(filepath.Join(dir, "incrmit.toml"))
	if err != nil {
		t.Fatalf("loading generated config: %v", err)
	}
	if len(cfg.Files) != 1 || cfg.Files[0].Path != "VERSION" || cfg.Files[0].Version != "1.2.3" {
		t.Errorf("config files = %+v", cfg.Files)
	}
}

// A pre-existing incrmit.toml must never list itself as a target.
func TestDiscoverExcludesConfigFile(t *testing.T) {
	dir := project(t, "# stale\n[[files]]\npath = \"x\"\nversion = \"0.0.1\"\n", map[string]string{
		"VERSION": "1.2.3\n",
	})

	code, _, stderr := runMain(t, dir, "discover")
	if code != ExitOK {
		t.Fatalf("exit = %d, stderr = %q", code, stderr)
	}
	cfg, err := config.Load(filepath.Join(dir, "incrmit.toml"))
	if err != nil {
		t.Fatalf("loading config: %v", err)
	}
	for _, f := range cfg.Files {
		if f.Path == "incrmit.toml" {
			t.Fatalf("config listed itself as a target: %+v", cfg.Files)
		}
	}
}

// A custom --output path that does not end in incrmit.toml is still excluded.
func TestDiscoverExcludesCustomOutput(t *testing.T) {
	dir := project(t, "", map[string]string{
		"VERSION":  "1.2.3\n",
		"conf.cfg": "1.0.0\n",
	})
	out := filepath.Join(dir, "conf.cfg")

	code, _, stderr := runMain(t, "", "discover", "--path", dir, "--output", out)
	if code != ExitOK {
		t.Fatalf("exit = %d, stderr = %q", code, stderr)
	}
	cfg, err := config.Load(out)
	if err != nil {
		t.Fatalf("loading config: %v", err)
	}
	for _, f := range cfg.Files {
		if filepath.Base(f.Path) == "conf.cfg" {
			t.Fatalf("config listed the output file as a target: %+v", cfg.Files)
		}
	}
}

func TestDiscoverDryRun(t *testing.T) {
	dir := project(t, "", map[string]string{"VERSION": "1.2.3\n"})

	code, stdout, stderr := runMain(t, dir, "discover", "--dry-run")
	if code != ExitOK {
		t.Fatalf("exit = %d, stderr = %q", code, stderr)
	}
	if !strings.Contains(stdout, "VERSION:") || !strings.Contains(stdout, "1.2.3") || !strings.Contains(stdout, "no config written") {
		t.Errorf("stdout = %q", stdout)
	}
	if !strings.Contains(stdout, "L1:") {
		t.Errorf("stdout = %q, want a line-numbered occurrence", stdout)
	}
	if _, err := os.Stat(filepath.Join(dir, "incrmit.toml")); !os.IsNotExist(err) {
		t.Errorf("dry-run wrote a config file (err = %v)", err)
	}
}

// discover reads an existing config's ignore list, skips matching paths, and
// writes the ignore list back so it survives regeneration.
func TestDiscoverHonorsAndPreservesIgnore(t *testing.T) {
	dir := project(t, "ignore = [\"docs/**\", \"*.lock\"]\n[[files]]\npath = \"VERSION\"\nversion = \"0.0.1\"\n", map[string]string{
		"VERSION":       "1.2.3\n",
		"deps.lock":     "2.0.0\n",
		"docs/guide.md": "3.4.5\n",
	})

	code, _, stderr := runMain(t, dir, "discover")
	if code != ExitOK {
		t.Fatalf("exit = %d, stderr = %q", code, stderr)
	}
	cfg, err := config.Load(filepath.Join(dir, "incrmit.toml"))
	if err != nil {
		t.Fatalf("loading config: %v", err)
	}
	for _, f := range cfg.Files {
		if f.Path == "deps.lock" || strings.HasPrefix(f.Path, "docs/") {
			t.Errorf("ignored path was discovered: %+v", cfg.Files)
		}
	}
	if len(cfg.Ignore) != 2 || cfg.Ignore[0] != "docs/**" || cfg.Ignore[1] != "*.lock" {
		t.Errorf("ignore list not preserved: %v", cfg.Ignore)
	}
}

// discover --dry-run notes the applied ignore rules and omits skipped paths.
func TestDiscoverDryRunShowsIgnore(t *testing.T) {
	dir := project(t, "ignore = [\"*.lock\"]\n[[files]]\npath = \"VERSION\"\nversion = \"0.0.1\"\n", map[string]string{
		"VERSION":   "1.2.3\n",
		"deps.lock": "2.0.0\n",
	})

	code, stdout, stderr := runMain(t, dir, "discover", "--dry-run")
	if code != ExitOK {
		t.Fatalf("exit = %d, stderr = %q", code, stderr)
	}
	if !strings.Contains(stdout, "ignoring: *.lock") {
		t.Errorf("dry-run did not note the ignore rules: %q", stdout)
	}
	if strings.Contains(stdout, "deps.lock") {
		t.Errorf("dry-run listed an ignored file: %q", stdout)
	}
}

// A bump must not drop the user-authored ignore list when it rewrites the config.
func TestBumpPreservesIgnore(t *testing.T) {
	dir := project(t, "ignore = [\"docs/**\"]\n[[files]]\npath = \"VERSION\"\nversion = \"1.2.3\"\n", map[string]string{
		"VERSION": "1.2.3\n",
	})

	code, _, stderr := runMain(t, dir)
	if code != ExitOK {
		t.Fatalf("exit = %d, stderr = %q", code, stderr)
	}
	cfg, err := config.Load(filepath.Join(dir, "incrmit.toml"))
	if err != nil {
		t.Fatalf("loading config: %v", err)
	}
	if len(cfg.Ignore) != 1 || cfg.Ignore[0] != "docs/**" {
		t.Errorf("bump dropped the ignore list: %v", cfg.Ignore)
	}
	if len(cfg.Files) != 1 || cfg.Files[0].Version != "1.2.4" {
		t.Errorf("bump did not update the version: %+v", cfg.Files)
	}
}

// discover --dry-run must not report IPv4 addresses: a file holding only an IP
// yields no findings, while a real version on the same line is shown alone.
func TestDiscoverDryRunExcludesIPv4(t *testing.T) {
	dir := project(t, "", map[string]string{
		"hosts.cfg": "gateway 192.168.1.1\nmask 255.255.255.255\n",
		"app.cfg":   "listen 10.0.0.1 version 2.3.4\n",
	})

	code, stdout, stderr := runMain(t, dir, "discover", "--dry-run")
	if code != ExitOK {
		t.Fatalf("exit = %d, stderr = %q", code, stderr)
	}
	// A file whose only version-like tokens are IPv4 addresses yields no
	// findings, so it must not be listed at all.
	if strings.Contains(stdout, "hosts.cfg") {
		t.Errorf("dry-run reported an IPv4-only file: %q", stdout)
	}
	if strings.Contains(stdout, "192.168") || strings.Contains(stdout, "255.255") {
		t.Errorf("dry-run leaked an IPv4-only address: %q", stdout)
	}
	// The real version is reported (its line context may legitimately include
	// the neighbouring IP, which is not itself treated as a version).
	if !strings.Contains(stdout, "app.cfg:") || !strings.Contains(stdout, "2.3.4") {
		t.Errorf("dry-run missing the real version: %q", stdout)
	}
}

func TestDiscoverCustomOutputAndPath(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "src")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "VERSION"), []byte("1.2.3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "release", "incrmit.toml")
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		t.Fatal(err)
	}

	code, _, stderr := runMain(t, "", "discover", "--path", srcDir, "--output", out)
	if code != ExitOK {
		t.Fatalf("exit = %d, stderr = %q", code, stderr)
	}
	if _, err := os.Stat(out); err != nil {
		t.Errorf("expected config at %s: %v", out, err)
	}
}

func TestDiscoverNoFindings(t *testing.T) {
	dir := project(t, "", map[string]string{"README.md": "nothing here\n"})
	code, _, stderr := runMain(t, dir, "discover")
	if code != ExitNoVersion {
		t.Errorf("exit = %d, want %d", code, ExitNoVersion)
	}
	if !strings.Contains(stderr, "no version-bearing files") {
		t.Errorf("stderr = %q", stderr)
	}
}

func TestDiscoverBadFlag(t *testing.T) {
	code, _, _ := runMain(t, "", "discover", "--nope")
	if code != ExitUsage {
		t.Errorf("exit = %d, want %d", code, ExitUsage)
	}
}

// --max-file-size tightens (or lifts) discovery's read cap: a file over the
// given size is not scanned, and the same tree scanned with the cap disabled
// finds it again.
func TestDiscoverMaxFileSize(t *testing.T) {
	targets := map[string]string{
		"VERSION":   "1.2.3\n",
		"notes.txt": strings.Repeat("x", 4096) + "\n2.0.0\n",
	}

	for _, tt := range []struct {
		name    string
		args    []string
		wantBig bool
	}{
		{"cap below the file", []string{"discover", "--max-file-size", "1KiB", "--dry-run"}, false},
		{"shorthand cap", []string{"discover", "-s", "1K", "--dry-run"}, false},
		{"cap above the file", []string{"discover", "-s", "1MiB", "--dry-run"}, true},
		{"cap disabled", []string{"discover", "-s", "0", "--dry-run"}, true},
		{"default cap", []string{"discover", "--dry-run"}, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dir := project(t, "", targets)
			code, stdout, stderr := runMain(t, dir, tt.args...)
			if code != ExitOK {
				t.Fatalf("exit = %d, stderr = %q", code, stderr)
			}
			if got := strings.Contains(stdout, "notes.txt"); got != tt.wantBig {
				t.Errorf("notes.txt discovered = %v, want %v:\n%s", got, tt.wantBig, stdout)
			}
			if !strings.Contains(stdout, "VERSION") {
				t.Errorf("stdout missing VERSION:\n%s", stdout)
			}
		})
	}
}

// A bad --max-file-size is a usage error, reported before anything is scanned.
func TestDiscoverBadMaxFileSize(t *testing.T) {
	dir := project(t, "", map[string]string{"VERSION": "1.2.3\n"})
	code, stdout, stderr := runMain(t, dir, "discover", "--max-file-size", "lots")
	if code != ExitUsage {
		t.Fatalf("exit = %d, want %d", code, ExitUsage)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty on a usage error", stdout)
	}
	if !strings.Contains(stderr, `"lots"`) {
		t.Errorf("stderr = %q, want it to name the bad value", stderr)
	}
	if _, err := os.Stat(filepath.Join(dir, "incrmit.toml")); !os.IsNotExist(err) {
		t.Errorf("a config was written despite the usage error (err %v)", err)
	}
}

func TestBumpUsesConfigVersionForAmbiguousFile(t *testing.T) {
	// README-like file with several version-like strings; the config pins the
	// one to bump, so it must not fail as ambiguous.
	body := "[[files]]\npath = \"README.md\"\nversion = \"0.1.0\"\n"
	dir := project(t, body, map[string]string{
		"README.md": "title 0.1.0\nexample 1.2.3 -> 1.2.4\nrelease 2.0.0\n",
	})

	code, stdout, stderr := runMain(t, dir, "--minor")
	if code != ExitOK {
		t.Fatalf("exit = %d, stderr = %q", code, stderr)
	}
	if !strings.Contains(stdout, "0.1.0 -> 0.2.0") {
		t.Errorf("stdout = %q", stdout)
	}
	got, _ := os.ReadFile(filepath.Join(dir, "README.md"))
	want := "title 0.2.0\nexample 1.2.3 -> 1.2.4\nrelease 2.0.0\n"
	if string(got) != want {
		t.Errorf("README.md = %q, want %q (only the config version should change)", got, want)
	}
}

func TestBumpConfigVersionNotInFile(t *testing.T) {
	// The config records a version that is not present in the file.
	body := "[[files]]\npath = \"VERSION\"\nversion = \"9.9.9\"\n"
	dir := project(t, body, map[string]string{"VERSION": "1.2.3\n"})

	code, _, stderr := runMain(t, dir)
	if code != ExitNoVersion {
		t.Errorf("exit = %d, want %d", code, ExitNoVersion)
	}
	if !strings.Contains(stderr, "not found") {
		t.Errorf("stderr = %q", stderr)
	}
	// File must be untouched.
	if got, _ := os.ReadFile(filepath.Join(dir, "VERSION")); string(got) != "1.2.3\n" {
		t.Errorf("VERSION changed: %q", got)
	}
}

func TestBumpInvalidConfigVersion(t *testing.T) {
	body := "[[files]]\npath = \"VERSION\"\nversion = \"not-a-version\"\n"
	dir := project(t, body, map[string]string{"VERSION": "1.2.3\n"})

	code, _, stderr := runMain(t, dir)
	if code != ExitNoVersion {
		t.Errorf("exit = %d, want %d", code, ExitNoVersion)
	}
	if !strings.Contains(stderr, "invalid version") {
		t.Errorf("stderr = %q", stderr)
	}
}

func TestBumpFileDoesNotExist(t *testing.T) {
	dir := t.TempDir()
	code, _, stderr := runMain(t, "", "--file", filepath.Join(dir, "missing"))
	if code != ExitError {
		t.Errorf("exit = %d, want %d", code, ExitError)
	}
	if !strings.Contains(stderr, "file does not exist") {
		t.Errorf("stderr = %q, want a clear missing-file message", stderr)
	}
}

func TestBumpUnreadableFile(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root: file permissions are not enforced")
	}
	dir := project(t, "", map[string]string{"VERSION": "1.2.3\n"})
	target := filepath.Join(dir, "VERSION")
	if err := os.Chmod(target, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(target, 0o644) })

	code, _, stderr := runMain(t, "", "--file", target)
	if code != ExitError {
		t.Errorf("exit = %d, want %d (stderr %q)", code, ExitError, stderr)
	}
	if !strings.Contains(stderr, "permission denied") {
		t.Errorf("stderr = %q, want a clear permission message", stderr)
	}
}

// A target that is not an ordinary file must be reported, not opened: opening a
// named pipe waits for a writer, which used to hang incrmit with no output. The
// test's own deadline catches a regression.
func TestBumpNonRegularFileTarget(t *testing.T) {
	dir := t.TempDir()
	fifo := filepath.Join(dir, "pipe")
	if err := testutil.Mkfifo(fifo); err != nil {
		t.Skipf("FIFOs unsupported: %v", err)
	}

	type outcome struct {
		code   int
		stderr string
	}
	done := make(chan outcome, 1)
	go func() {
		code, _, stderr := runMain(t, "", "--file", fifo)
		done <- outcome{code, stderr}
	}()

	select {
	case got := <-done:
		if got.code != ExitError {
			t.Errorf("exit = %d, want %d (stderr %q)", got.code, ExitError, got.stderr)
		}
		if !strings.Contains(got.stderr, "not a regular file") {
			t.Errorf("stderr = %q, want it to say the target is not a regular file", got.stderr)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("bump blocked on the FIFO instead of reporting it")
	}
}

func TestBumpWriteToReadOnlyDir(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root: directory permissions are not enforced")
	}
	dir := t.TempDir()
	sub := filepath.Join(dir, "ro")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(sub, "VERSION")
	if err := os.WriteFile(target, []byte("1.2.3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Make the directory read-only so the atomic temp-file create fails.
	if err := os.Chmod(sub, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(sub, 0o755) })

	code, _, stderr := runMain(t, "", "--file", target)
	if code != ExitError {
		t.Errorf("exit = %d, want %d", code, ExitError)
	}
	if !strings.Contains(stderr, "writing") {
		t.Errorf("stderr = %q, want a clear write-error message", stderr)
	}
	// The original file must be untouched.
	got, _ := os.ReadFile(target)
	if string(got) != "1.2.3\n" {
		t.Errorf("file changed despite write failure: %q", got)
	}
}

// Explicit help requests exit 0 and print usage text to stdout (not stderr).
func TestHelpExitsZero(t *testing.T) {
	for _, args := range [][]string{
		{"-h"}, {"--help"}, {"-help"},
		{"discover", "-h"}, {"discover", "--help"},
		{"undo", "-h"}, {"undo", "--help"},
		{"version", "-h"},
		{"--minor", "-h"}, // help requested in bump context
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			code, stdout, stderr := runMain(t, "", args...)
			if code != ExitOK {
				t.Errorf("exit = %d, want %d (stderr %q)", code, ExitOK, stderr)
			}
			if !strings.Contains(stdout, "usage:") {
				t.Errorf("stdout = %q, want usage text", stdout)
			}
		})
	}
}

// `incrmit help` prints the banner and the top-level overview listing every
// command.
func TestHelpOverview(t *testing.T) {
	for _, args := range [][]string{{"help"}, {"-h"}, {"--help"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			code, stdout, stderr := runMain(t, "", args...)
			if code != ExitOK {
				t.Fatalf("exit = %d, stderr = %q", code, stderr)
			}
			if !strings.HasPrefix(stdout, banner) {
				t.Errorf("overview does not start with the banner: %q", stdout)
			}
			for _, want := range []string{"incrmit", "discover", "undo", "version", "help"} {
				if !strings.Contains(stdout, want) {
					t.Errorf("overview missing %q: %q", want, stdout)
				}
			}
			for _, want := range []string{"-c, --config", "-M, --major", "-d, --dry-run", "-P, --path", "-o, --output", "--no-banner"} {
				if !strings.Contains(stdout, want) {
					t.Errorf("overview missing flag %q: %q", want, stdout)
				}
			}
		})
	}
}

// --no-banner suppresses the banner on every path that prints the overview,
// without changing the overview text or the exit code.
func TestHelpOverviewNoBanner(t *testing.T) {
	for _, args := range [][]string{
		{"help", "--no-banner"},
		{"-h", "--no-banner"},
		{"--help", "--no-banner"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			code, stdout, stderr := runMain(t, "", args...)
			if code != ExitOK {
				t.Fatalf("exit = %d, want %d (stderr %q)", code, ExitOK, stderr)
			}
			if strings.Contains(stdout, banner) {
				t.Errorf("stdout still contains the banner: %q", stdout)
			}
			if stdout != overviewHelp {
				t.Errorf("stdout = %q, want overviewHelp", stdout)
			}
		})
	}
}

// `incrmit help <command>` reuses the same text as the command's own -h/--help.
func TestHelpForCommand(t *testing.T) {
	tests := []struct {
		command string
		want    string
	}{
		{"bump", "Bump the semantic version"},
		{"discover", "Scan a directory tree"},
		{"undo", "Revert the most recent bump"},
		{"version", "Print the incrmit tool version"},
		{"help", "Show the incrmit overview"},
	}
	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			code, viaHelp, stderr := runMain(t, "", "help", tt.command)
			if code != ExitOK {
				t.Fatalf("exit = %d, stderr = %q", code, stderr)
			}
			if !strings.Contains(viaHelp, tt.want) {
				t.Errorf("help %s = %q, want to contain %q", tt.command, viaHelp, tt.want)
			}
		})
	}
}

// `incrmit help <command>` and `incrmit <command> -h` produce identical text,
// confirming the centralized help is reused rather than duplicated. (For bump,
// the flag form is `--minor -h` because a bare top-level -h shows the overview.)
func TestHelpMatchesFlagHelp(t *testing.T) {
	tests := []struct {
		command  string
		flagArgs []string
	}{
		{"bump", []string{"--minor", "-h"}},
		{"discover", []string{"discover", "-h"}},
		{"undo", []string{"undo", "-h"}},
		{"version", []string{"version", "-h"}},
	}
	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			_, viaHelp, _ := runMain(t, "", "help", tt.command)
			_, viaFlag, _ := runMain(t, "", tt.flagArgs...)
			if viaHelp != viaFlag {
				t.Errorf("help %s = %q, flag help = %q (should match)", tt.command, viaHelp, viaFlag)
			}
		})
	}
}

func TestHelpUnknownCommand(t *testing.T) {
	code, stdout, stderr := runMain(t, "", "help", "bogus")
	if code != ExitUsage {
		t.Errorf("exit = %d, want %d", code, ExitUsage)
	}
	if !strings.Contains(stderr, "unknown command") {
		t.Errorf("stderr = %q, want unknown-command message", stderr)
	}
	if !strings.Contains(stderr, "incrmit help") {
		t.Errorf("stderr = %q, want hint to run 'incrmit help'", stderr)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty on error", stdout)
	}
}

func TestUnknownSubcommand(t *testing.T) {
	code, _, stderr := runMain(t, "", "frobnicate")
	if code != ExitUsage {
		t.Errorf("exit = %d, want %d", code, ExitUsage)
	}
	if !strings.Contains(stderr, "unknown command") {
		t.Errorf("stderr = %q, want unknown-command message", stderr)
	}
	if !strings.Contains(stderr, "incrmit help") {
		t.Errorf("stderr = %q, want hint to run 'incrmit help'", stderr)
	}
}

func TestExitCodeMatrix(t *testing.T) {
	noVerDir := project(t, "", map[string]string{"F": "no version\n"})
	ambDir := project(t, "", map[string]string{"F": "a=1.2.3 b=4.5.6\n"})
	okDir := project(t, "[[files]]\npath = \"VERSION\"\n", map[string]string{"VERSION": "1.2.3\n"})
	missingCfgDir := t.TempDir()

	tests := []struct {
		name string
		dir  string
		args []string
		want int
	}{
		{"success", okDir, nil, ExitOK},
		{"usage-bad-flag", "", []string{"--nope"}, ExitUsage},
		{"usage-stray-arg", "", []string{"surprise"}, ExitUsage},
		{"generic-missing-config", missingCfgDir, nil, ExitError},
		{"noversion", "", []string{"--file", filepath.Join(noVerDir, "F")}, ExitNoVersion},
		{"ambiguous", "", []string{"--file", filepath.Join(ambDir, "F")}, ExitNoVersion},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, _, _ := runMain(t, tt.dir, tt.args...)
			if code != tt.want {
				t.Errorf("exit = %d, want %d", code, tt.want)
			}
		})
	}
}

// A bump records a history entry; a following undo restores the file and the
// config to their pre-bump values and leaves an empty journal.
func TestUndoRevertsSingleFile(t *testing.T) {
	body := "[[files]]\npath = \"VERSION\"\nversion = \"1.2.3\"\n"
	dir := project(t, body, map[string]string{"VERSION": "1.2.3\n"})

	if code, _, stderr := runMain(t, dir, "--minor"); code != ExitOK {
		t.Fatalf("bump exit = %d, stderr = %q", code, stderr)
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "VERSION")); string(got) != "1.3.0\n" {
		t.Fatalf("VERSION after bump = %q, want 1.3.0", got)
	}

	code, stdout, stderr := runMain(t, dir, "undo")
	if code != ExitOK {
		t.Fatalf("undo exit = %d, stderr = %q", code, stderr)
	}
	if !strings.Contains(stdout, "1.3.0 -> 1.2.3") {
		t.Errorf("undo stdout missing revert summary: %q", stdout)
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "VERSION")); string(got) != "1.2.3\n" {
		t.Errorf("VERSION after undo = %q, want 1.2.3", got)
	}
	cfg, err := config.Load(filepath.Join(dir, "incrmit.toml"))
	if err != nil {
		t.Fatalf("loading config: %v", err)
	}
	if cfg.Files[0].Version != "1.2.3" {
		t.Errorf("config version after undo = %q, want 1.2.3", cfg.Files[0].Version)
	}
}

// Undo restores every file (and both entries of a multi-version file) written
// by the most recent bump.
func TestUndoRevertsMultipleFiles(t *testing.T) {
	body := "[[files]]\npath = \"VERSION\"\nversion = \"1.2.3\"\n" +
		"[[files]]\npath = \"notes.md\"\nversion = \"1.2.3\"\n" +
		"[[files]]\npath = \"notes.md\"\nversion = \"2.0.0\"\n"
	dir := project(t, body, map[string]string{
		"VERSION":  "1.2.3\n",
		"notes.md": "app 1.2.3\nlib 2.0.0\n",
	})

	if code, _, stderr := runMain(t, dir, "--minor"); code != ExitOK {
		t.Fatalf("bump exit = %d, stderr = %q", code, stderr)
	}
	if code, _, stderr := runMain(t, dir, "undo"); code != ExitOK {
		t.Fatalf("undo exit = %d, stderr = %q", code, stderr)
	}

	if got, _ := os.ReadFile(filepath.Join(dir, "VERSION")); string(got) != "1.2.3\n" {
		t.Errorf("VERSION after undo = %q, want 1.2.3", got)
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "notes.md")); string(got) != "app 1.2.3\nlib 2.0.0\n" {
		t.Errorf("notes.md after undo = %q", got)
	}
	cfg, _ := config.Load(filepath.Join(dir, "incrmit.toml"))
	for _, f := range cfg.Files {
		if f.Version != "1.2.3" && f.Version != "2.0.0" {
			t.Errorf("config version after undo = %q, want 1.2.3 or 2.0.0", f.Version)
		}
	}
}

// Repeated undos walk back through successive bumps, one bump per undo.
func TestUndoWalksBackThroughBumps(t *testing.T) {
	body := "[[files]]\npath = \"VERSION\"\nversion = \"1.2.3\"\n"
	dir := project(t, body, map[string]string{"VERSION": "1.2.3\n"})

	for i := 0; i < 3; i++ {
		if code, _, stderr := runMain(t, dir); code != ExitOK {
			t.Fatalf("bump %d exit = %d, stderr = %q", i, code, stderr)
		}
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "VERSION")); string(got) != "1.2.6\n" {
		t.Fatalf("VERSION after 3 bumps = %q, want 1.2.6", got)
	}

	wants := []string{"1.2.5\n", "1.2.4\n", "1.2.3\n"}
	for i, want := range wants {
		if code, _, stderr := runMain(t, dir, "undo"); code != ExitOK {
			t.Fatalf("undo %d exit = %d, stderr = %q", i, code, stderr)
		}
		if got, _ := os.ReadFile(filepath.Join(dir, "VERSION")); string(got) != want {
			t.Errorf("VERSION after undo %d = %q, want %q", i, got, want)
		}
	}
}

// undo --dry-run previews the revert (new -> old) without touching the file,
// the config, or the journal.
func TestUndoDryRun(t *testing.T) {
	body := "[[files]]\npath = \"VERSION\"\nversion = \"1.2.3\"\n"
	dir := project(t, body, map[string]string{"VERSION": "1.2.3\n"})
	if code, _, stderr := runMain(t, dir, "--minor"); code != ExitOK {
		t.Fatalf("bump exit = %d, stderr = %q", code, stderr)
	}

	code, stdout, stderr := runMain(t, dir, "undo", "--dry-run")
	if code != ExitOK {
		t.Fatalf("undo dry-run exit = %d, stderr = %q", code, stderr)
	}
	if !strings.Contains(stdout, "Dry run") || !strings.Contains(stdout, "1.3.0 -> 1.2.3") {
		t.Errorf("dry-run stdout = %q", stdout)
	}
	// The file must remain bumped: a dry-run writes nothing.
	if got, _ := os.ReadFile(filepath.Join(dir, "VERSION")); string(got) != "1.3.0\n" {
		t.Errorf("dry-run modified file: %q", got)
	}
	// A real undo must still be possible, proving the journal was untouched.
	if code, _, stderr := runMain(t, dir, "undo"); code != ExitOK {
		t.Fatalf("undo after dry-run exit = %d, stderr = %q", code, stderr)
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "VERSION")); string(got) != "1.2.3\n" {
		t.Errorf("VERSION after real undo = %q, want 1.2.3", got)
	}
}

// A --dry-run bump records nothing, so a following undo has nothing to revert.
func TestUndoNothingAfterDryRunBump(t *testing.T) {
	body := "[[files]]\npath = \"VERSION\"\nversion = \"1.2.3\"\n"
	dir := project(t, body, map[string]string{"VERSION": "1.2.3\n"})
	if code, _, stderr := runMain(t, dir, "--minor", "--dry-run"); code != ExitOK {
		t.Fatalf("bump dry-run exit = %d, stderr = %q", code, stderr)
	}
	if _, err := os.Stat(filepath.Join(dir, config.StateFileName)); !os.IsNotExist(err) {
		t.Errorf("dry-run bump wrote a state file (err = %v)", err)
	}
	code, stdout, stderr := runMain(t, dir, "undo")
	if code != ExitOK {
		t.Fatalf("undo exit = %d, stderr = %q", code, stderr)
	}
	if !strings.Contains(stdout, "Nothing to undo") {
		t.Errorf("stdout = %q, want a friendly nothing-to-undo message", stdout)
	}
}

// With no history at all, undo prints a friendly message and exits 0.
func TestUndoEmptyHistory(t *testing.T) {
	dir := project(t, "[[files]]\npath = \"VERSION\"\n", map[string]string{"VERSION": "1.2.3\n"})
	code, stdout, stderr := runMain(t, dir, "undo")
	if code != ExitOK {
		t.Fatalf("exit = %d, want %d (stderr %q)", code, ExitOK, stderr)
	}
	if !strings.Contains(stdout, "Nothing to undo") {
		t.Errorf("stdout = %q, want nothing-to-undo message", stdout)
	}
}

// If a file was edited after the bump so it no longer holds the recorded "new"
// token, undo refuses and writes nothing (the user's edit is not clobbered).
func TestUndoConflictRefuses(t *testing.T) {
	body := "[[files]]\npath = \"VERSION\"\nversion = \"1.2.3\"\n"
	dir := project(t, body, map[string]string{"VERSION": "1.2.3\n"})
	if code, _, stderr := runMain(t, dir, "--minor"); code != ExitOK {
		t.Fatalf("bump exit = %d, stderr = %q", code, stderr)
	}
	// Edit the file so its current token no longer matches the recorded new.
	if err := os.WriteFile(filepath.Join(dir, "VERSION"), []byte("9.9.9\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	code, _, stderr := runMain(t, dir, "undo")
	if code != ExitError {
		t.Errorf("exit = %d, want %d", code, ExitError)
	}
	if !strings.Contains(stderr, "changed since") {
		t.Errorf("stderr = %q, want a conflict message", stderr)
	}
	// The edited file must be left exactly as the user left it.
	if got, _ := os.ReadFile(filepath.Join(dir, "VERSION")); string(got) != "9.9.9\n" {
		t.Errorf("undo clobbered an edited file: %q", got)
	}
}

// --file bumps have no config-anchored state, so no history is recorded and a
// following config-mode undo has nothing to revert.
func TestUndoNotRecordedForFileMode(t *testing.T) {
	dir := project(t, "[[files]]\npath = \"VERSION\"\n", map[string]string{"VERSION": "1.2.3\n"})
	if code, _, stderr := runMain(t, dir, "--file", filepath.Join(dir, "VERSION")); code != ExitOK {
		t.Fatalf("file bump exit = %d, stderr = %q", code, stderr)
	}
	if _, err := os.Stat(filepath.Join(dir, config.StateFileName)); !os.IsNotExist(err) {
		t.Errorf("--file bump wrote a state file (err = %v)", err)
	}
}

func TestUndoBadFlag(t *testing.T) {
	code, _, _ := runMain(t, "", "undo", "--nope")
	if code != ExitUsage {
		t.Errorf("exit = %d, want %d", code, ExitUsage)
	}
}

func TestUndoUnexpectedArg(t *testing.T) {
	code, _, stderr := runMain(t, "", "undo", "surprise")
	if code != ExitUsage {
		t.Errorf("exit = %d, want %d", code, ExitUsage)
	}
	if !strings.Contains(stderr, "unexpected argument") {
		t.Errorf("stderr = %q", stderr)
	}
}

func TestVersionCommand(t *testing.T) {
	for _, arg := range []string{"version", "--version", "-version", "-v"} {
		t.Run(arg, func(t *testing.T) {
			code, stdout, stderr := runMain(t, "", arg)
			if code != ExitOK {
				t.Fatalf("exit = %d, stderr = %q", code, stderr)
			}
			if !strings.HasPrefix(stdout, "incrmit ") {
				t.Errorf("stdout = %q, want prefix \"incrmit \"", stdout)
			}
		})
	}
}
