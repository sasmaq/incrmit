package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sasmaq/incrmit/internal/config"
	"github.com/sasmaq/incrmit/internal/history"
)

// stateFile returns the path of the bump journal inside dir.
func stateFile(dir string) string {
	return filepath.Join(dir, config.StateFileName)
}

func TestVersionUnexpectedArg(t *testing.T) {
	code, _, stderr := runMain(t, "", "version", "extra")
	if code != ExitUsage {
		t.Errorf("exit = %d, want %d", code, ExitUsage)
	}
	if !strings.Contains(stderr, "unexpected argument") {
		t.Errorf("stderr = %q, want it to name the unexpected argument", stderr)
	}
}

func TestDiscoverUnexpectedArg(t *testing.T) {
	code, _, stderr := runMain(t, t.TempDir(), "discover", "extra")
	if code != ExitUsage {
		t.Errorf("exit = %d, want %d", code, ExitUsage)
	}
	if !strings.Contains(stderr, "unexpected argument") {
		t.Errorf("stderr = %q, want it to name the unexpected argument", stderr)
	}
}

// discover reads any existing config at --output to pick up its ignore list. A
// config that cannot be read is a real problem (unlike an unparseable one,
// which is deliberately tolerated), so it must stop rather than silently
// regenerate without the user's ignore patterns.
func TestDiscoverUnreadableExistingConfig(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root: file permissions are not enforced")
	}
	dir := project(t, "ignore = [\"docs/**\"]\n[[files]]\npath = \"VERSION\"\n", map[string]string{
		"VERSION": "1.2.3\n",
	})
	cfg := filepath.Join(dir, "incrmit.toml")
	if err := os.Chmod(cfg, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(cfg, 0o644) })

	code, _, stderr := runMain(t, dir, "discover")
	if code != ExitError {
		t.Errorf("exit = %d, want %d (stderr %q)", code, ExitError, stderr)
	}
	if !strings.Contains(stderr, "incrmit.toml") {
		t.Errorf("stderr = %q, want it to name the unreadable config", stderr)
	}
}

func TestDiscoverWriteToReadOnlyDir(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root: directory permissions are not enforced")
	}
	dir := t.TempDir()
	sub := filepath.Join(dir, "ro")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "VERSION"), []byte("1.2.3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// 0500 still allows the walk to read the tree, but not to create the
	// config's temp file.
	if err := os.Chmod(sub, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(sub, 0o755) })

	code, _, stderr := runMain(t, "", "discover", "--path", sub, "--output", filepath.Join(sub, "incrmit.toml"))
	if code != ExitError {
		t.Errorf("exit = %d, want %d (stderr %q)", code, ExitError, stderr)
	}
	if !strings.Contains(stderr, "writing") {
		t.Errorf("stderr = %q, want a clear write-error message", stderr)
	}
}

// A corrupt journal must be reported rather than treated as an empty history,
// which would silently answer "nothing to undo" and lose the recorded bump.
func TestUndoCorruptStateFile(t *testing.T) {
	dir := project(t, "[[files]]\npath = \"VERSION\"\n", map[string]string{"VERSION": "1.2.3\n"})
	if err := os.WriteFile(stateFile(dir), []byte("entries = [ this is not toml\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	code, stdout, stderr := runMain(t, dir, "undo")
	if code != ExitError {
		t.Errorf("exit = %d, want %d", code, ExitError)
	}
	if strings.Contains(stdout, "Nothing to undo") {
		t.Error("a corrupt journal was reported as an empty one")
	}
	if !strings.Contains(stderr, "parsing") {
		t.Errorf("stderr = %q, want it to name the parse failure", stderr)
	}
}

// undo loads the config before writing anything, so a config that has gone
// missing since the bump aborts the revert with every file left alone.
func TestUndoMissingConfigLeavesFilesUntouched(t *testing.T) {
	dir := project(t, "[[files]]\npath = \"VERSION\"\n", map[string]string{"VERSION": "1.2.3\n"})
	if code, _, stderr := runMain(t, dir); code != ExitOK {
		t.Fatalf("bump exit = %d (stderr %q)", code, stderr)
	}
	if err := os.Remove(filepath.Join(dir, "incrmit.toml")); err != nil {
		t.Fatal(err)
	}

	code, _, stderr := runMain(t, dir, "undo")
	if code != ExitError {
		t.Errorf("exit = %d, want %d (stderr %q)", code, ExitError, stderr)
	}
	got, err := os.ReadFile(filepath.Join(dir, "VERSION"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "1.2.4\n" {
		t.Errorf("VERSION = %q, want the bumped value left in place by the aborted undo", got)
	}
	// The journal entry must survive so the undo can be retried once the
	// config is restored.
	h, err := history.Load(stateFile(dir))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := h.Latest(); !ok {
		t.Error("journal entry was popped despite the undo failing")
	}
}

// A journal that cannot be read must stop the bump rather than let it report
// success with no way to undo it.
//
// This test also pins the gap tracked by Milestone 31: the failure happens
// after phase 2, so the target file is already rewritten when the command
// exits non-zero. Update the assertion below when the journal moves ahead of
// the writes.
func TestBumpUnreadableStateFileFailsAfterWriting(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root: file permissions are not enforced")
	}
	dir := project(t, "[[files]]\npath = \"VERSION\"\n", map[string]string{"VERSION": "1.2.3\n"})
	state := stateFile(dir)
	if err := os.WriteFile(state, []byte("entries = []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(state, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(state, 0o644) })

	code, _, stderr := runMain(t, dir)
	if code != ExitError {
		t.Errorf("exit = %d, want %d (stderr %q)", code, ExitError, stderr)
	}
	if !strings.Contains(stderr, config.StateFileName) {
		t.Errorf("stderr = %q, want it to name the state file", stderr)
	}

	got, err := os.ReadFile(filepath.Join(dir, "VERSION"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "1.2.4\n" {
		t.Errorf("VERSION = %q, want 1.2.4 (see Milestone 31: the write lands before the journal)", got)
	}
}

// The same failure through a corrupt (rather than unreadable) journal, which
// needs no permission trickery and so runs on every platform.
func TestBumpCorruptStateFileReportsParseError(t *testing.T) {
	dir := project(t, "[[files]]\npath = \"VERSION\"\n", map[string]string{"VERSION": "1.2.3\n"})
	if err := os.WriteFile(stateFile(dir), []byte("entries = [ not toml\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	code, _, stderr := runMain(t, dir)
	if code != ExitError {
		t.Errorf("exit = %d, want %d", code, ExitError)
	}
	if !strings.Contains(stderr, "parsing") {
		t.Errorf("stderr = %q, want it to name the parse failure", stderr)
	}
}

// distinctVersions backs the discover summary: a file holding the same version
// several times is listed once, while genuinely different versions all appear.
func TestDiscoverSummaryListsEachVersionOnce(t *testing.T) {
	dir := t.TempDir()
	body := "a = \"1.2.3\"\nb = \"1.2.3\"\nc = \"4.5.6\"\n"
	if err := os.WriteFile(filepath.Join(dir, "conf.txt"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	code, stdout, stderr := runMain(t, dir, "discover")
	if code != ExitOK {
		t.Fatalf("exit = %d (stderr %q)", code, stderr)
	}
	if got := strings.Count(stdout, "conf.txt: 1.2.3"); got != 1 {
		t.Errorf("repeated version listed %d times, want 1\n%s", got, stdout)
	}
	if !strings.Contains(stdout, "conf.txt: 4.5.6") {
		t.Errorf("second distinct version missing from the summary\n%s", stdout)
	}
}

// fsErrorMessage has a catch-all branch for errors that are neither permission,
// missing-file, non-regular, nor over the size cap. A directory named as a
// --file target reaches it.
func TestBumpDirectoryTargetReportsPlainError(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "adirectory")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}

	code, _, stderr := runMain(t, "", "--file", sub)
	if code != ExitError {
		t.Errorf("exit = %d, want %d", code, ExitError)
	}
	if !strings.Contains(stderr, "not a regular file") {
		t.Errorf("stderr = %q, want it to explain the target is not a regular file", stderr)
	}
}

func TestSizeValueString(t *testing.T) {
	var v *sizeValue
	if got := v.String(); got != "" {
		t.Errorf("nil sizeValue String() = %q, want empty", got)
	}
	set := sizeValue(2048)
	if got := set.String(); got != "2KiB" {
		t.Errorf("String() = %q, want %q", got, "2KiB")
	}
}
