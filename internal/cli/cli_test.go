package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sasmaq/incrmit/internal/config"
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

func TestBadFlag(t *testing.T) {
	code, _, _ := runMain(t, "", "--nope")
	if code != ExitUsage {
		t.Errorf("exit = %d, want %d", code, ExitUsage)
	}
}

func TestUnexpectedArg(t *testing.T) {
	code, _, stderr := runMain(t, "", "surprise")
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

func TestDiscoverDryRun(t *testing.T) {
	dir := project(t, "", map[string]string{"VERSION": "1.2.3\n"})

	code, stdout, stderr := runMain(t, dir, "discover", "--dry-run")
	if code != ExitOK {
		t.Fatalf("exit = %d, stderr = %q", code, stderr)
	}
	if !strings.Contains(stdout, "VERSION: 1.2.3") || !strings.Contains(stdout, "no config written") {
		t.Errorf("stdout = %q", stdout)
	}
	if _, err := os.Stat(filepath.Join(dir, "incrmit.toml")); !os.IsNotExist(err) {
		t.Errorf("dry-run wrote a config file (err = %v)", err)
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

func TestHelpExitsZero(t *testing.T) {
	for _, args := range [][]string{{"-h"}, {"--help"}, {"discover", "-h"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			code, _, stderr := runMain(t, "", args...)
			if code != ExitOK {
				t.Errorf("exit = %d, want %d", code, ExitOK)
			}
			if !strings.Contains(stderr, "usage:") {
				t.Errorf("stderr = %q, want usage text", stderr)
			}
		})
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
