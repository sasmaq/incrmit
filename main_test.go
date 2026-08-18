package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// binPath is the compiled incrmit binary used by the end-to-end tests. It is
// built once in TestMain so the tests exercise the real program (flag parsing,
// dispatch, and process exit codes) rather than calling into the cli package
// in-process.
var binPath string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "incrmit-e2e")
	if err != nil {
		fmt.Fprintln(os.Stderr, "e2e: temp dir:", err)
		os.Exit(1)
	}
	binPath = filepath.Join(dir, "incrmit")
	if out, err := exec.Command("go", "build", "-o", binPath, ".").CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "e2e: build failed: %v\n%s", err, out)
		_ = os.RemoveAll(dir)
		os.Exit(1)
	}

	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

// runBin runs the compiled binary in workDir and returns its exit code plus
// captured stdout/stderr.
func runBin(t *testing.T, workDir string, args ...string) (int, string, string) {
	t.Helper()
	cmd := exec.Command(binPath, args...)
	cmd.Dir = workDir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	code := 0
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			code = ee.ExitCode()
		} else {
			t.Fatalf("running binary: %v", err)
		}
	}
	return code, stdout.String(), stderr.String()
}

func writeFile(t *testing.T, dir, name, body string) {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestE2EDefaultBump(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "incrmit.toml", "[[files]]\npath = \"VERSION\"\n")
	writeFile(t, dir, "VERSION", "1.2.3\n")

	code, stdout, stderr := runBin(t, dir)
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, stderr)
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "VERSION")); string(got) != "1.2.4\n" {
		t.Errorf("VERSION = %q, want \"1.2.4\\n\"", got)
	}
	if !strings.Contains(stdout, "1.2.3 -> 1.2.4") {
		t.Errorf("stdout = %q", stdout)
	}
}

func TestE2EMinorAndDryRun(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "incrmit.toml", "[[files]]\npath = \"VERSION\"\n")
	writeFile(t, dir, "VERSION", "1.2.3\n")

	code, stdout, _ := runBin(t, dir, "--minor", "--dry-run")
	if code != 0 {
		t.Fatalf("exit = %d", code)
	}
	if !strings.Contains(stdout, "Dry run") || !strings.Contains(stdout, "1.2.3 -> 1.3.0") {
		t.Errorf("stdout = %q", stdout)
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "VERSION")); string(got) != "1.2.3\n" {
		t.Errorf("dry-run modified file: %q", got)
	}
}

func TestE2ESingleFileMajor(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "V", "0.9.0\n")
	code, stdout, stderr := runBin(t, dir, "--file", "V", "--major")
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, stderr)
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "V")); string(got) != "1.0.0\n" {
		t.Errorf("V = %q", got)
	}
	if !strings.Contains(stdout, "0.9.0 -> 1.0.0") {
		t.Errorf("stdout = %q", stdout)
	}
}

func TestE2EDiscover(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "VERSION", "1.2.3\n")
	writeFile(t, dir, "package.json", `{"version":"2.0.1"}`)

	code, stdout, stderr := runBin(t, dir, "discover")
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, stderr)
	}
	if !strings.Contains(stdout, "Wrote incrmit.toml") {
		t.Errorf("stdout = %q", stdout)
	}
	if _, err := os.Stat(filepath.Join(dir, "incrmit.toml")); err != nil {
		t.Errorf("config not written: %v", err)
	}
}

func TestE2EPreview(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "incrmit.toml", "[[files]]\npath = \"VERSION\"\nversion = \"1.2.3\"\n")
	writeFile(t, dir, "VERSION", "1.2.3\n")

	code, stdout, stderr := runBin(t, dir, "preview")
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, stderr)
	}
	for _, want := range []string{"CURRENT", "1.2.3", "1.2.4", "1.3.0", "2.0.0"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout missing %q: %q", want, stdout)
		}
	}
	// Preview is read-only: the target keeps its version and no bump history
	// (which only a real bump writes) appears next to the config.
	if got, _ := os.ReadFile(filepath.Join(dir, "VERSION")); string(got) != "1.2.3\n" {
		t.Errorf("preview modified the file: %q", got)
	}
	if _, err := os.Stat(filepath.Join(dir, ".incrmit.state.toml")); err == nil {
		t.Errorf("preview wrote a state file")
	}
}

func TestE2EUndo(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "incrmit.toml", "[[files]]\npath = \"VERSION\"\nversion = \"1.2.3\"\n")
	writeFile(t, dir, "VERSION", "1.2.3\n")

	if code, _, stderr := runBin(t, dir, "--minor"); code != 0 {
		t.Fatalf("bump exit = %d, stderr = %q", code, stderr)
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "VERSION")); string(got) != "1.3.0\n" {
		t.Fatalf("VERSION after bump = %q, want 1.3.0", got)
	}

	code, stdout, stderr := runBin(t, dir, "undo")
	if code != 0 {
		t.Fatalf("undo exit = %d, stderr = %q", code, stderr)
	}
	if !strings.Contains(stdout, "1.3.0 -> 1.2.3") {
		t.Errorf("undo stdout = %q", stdout)
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "VERSION")); string(got) != "1.2.3\n" {
		t.Errorf("VERSION after undo = %q, want 1.2.3", got)
	}

	// A second undo has nothing left and exits 0 with a friendly message.
	code, stdout, stderr = runBin(t, dir, "undo")
	if code != 0 {
		t.Fatalf("second undo exit = %d, stderr = %q", code, stderr)
	}
	if !strings.Contains(stdout, "Nothing to undo") {
		t.Errorf("second undo stdout = %q, want nothing-to-undo message", stdout)
	}
}

func TestE2EVersion(t *testing.T) {
	code, stdout, stderr := runBin(t, t.TempDir(), "version")
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, stderr)
	}
	if !strings.HasPrefix(stdout, "incrmit ") {
		t.Errorf("stdout = %q, want prefix \"incrmit \"", stdout)
	}
}

func TestE2EHelp(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"help", []string{"help"}, "incrmit"},
		{"top-level-h", []string{"-h"}, "usage:"},
		{"top-level-help", []string{"--help"}, "usage:"},
		{"help-discover", []string{"help", "discover"}, "Scan a directory tree"},
		{"help-preview", []string{"help", "preview"}, "usage: incrmit preview"},
		{"help-undo", []string{"help", "undo"}, "Revert the most recent bump"},
		{"help-version", []string{"help", "version"}, "Print the incrmit tool version"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, stdout, stderr := runBin(t, t.TempDir(), tt.args...)
			if code != 0 {
				t.Fatalf("exit = %d, stderr = %q", code, stderr)
			}
			if !strings.Contains(stdout, tt.want) {
				t.Errorf("stdout = %q, want to contain %q", stdout, tt.want)
			}
		})
	}
}

func TestE2EUnknownCommand(t *testing.T) {
	code, _, stderr := runBin(t, t.TempDir(), "bogus")
	if code != 2 {
		t.Errorf("exit = %d, want 2", code)
	}
	if !strings.Contains(stderr, "unknown command") {
		t.Errorf("stderr = %q, want unknown-command message", stderr)
	}
}

func TestE2EExitCodes(t *testing.T) {
	noVer := t.TempDir()
	writeFile(t, noVer, "F", "no version here\n")

	tests := []struct {
		name    string
		workDir string
		args    []string
		want    int
	}{
		{"missing-config", t.TempDir(), nil, 1},
		{"bad-flag", t.TempDir(), []string{"--nope"}, 2},
		{"stray-arg", t.TempDir(), []string{"surprise"}, 2},
		{"no-version", noVer, []string{"--file", "F"}, 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, _, _ := runBin(t, tt.workDir, tt.args...)
			if code != tt.want {
				t.Errorf("exit = %d, want %d", code, tt.want)
			}
		})
	}
}
