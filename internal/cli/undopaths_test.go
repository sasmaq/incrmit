package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/sasmaq/incrmit/internal/config"
	"github.com/sasmaq/incrmit/internal/history"
)

// These tests run undo somewhere other than where the bump ran. Before
// Milestone 34 each journal entry recorded the config's absolute path and each
// change the absolute path of its file, and undo acted on those, which tied
// the journal to the directory the bump ran in rather than to the project.
// Undo in a copy reverted the original and popped the copy's entry; undo in a
// moved project looked for the config where it used to be; and a hand-written
// state file could point undo at any file the user can write. Journal paths
// are now the config's own `path` values, resolved against the config undo was
// given, and undo reverts only what that config lists.

// newProject writes a one-file project into root/name, with VERSION and the
// config both at ver, and returns its directory.
func newProject(t *testing.T, root, name, ver string) string {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for file, body := range map[string]string{
		"VERSION":          ver + "\n",
		config.DefaultPath: fmt.Sprintf("[[files]]\npath = \"VERSION\"\nversion = %q\n", ver),
	} {
		if err := os.WriteFile(filepath.Join(dir, file), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// bumpedProject is a newProject at 1.0.0 bumped once, to 1.0.1. The bump is
// run with -c rather than from inside the project, so no test is left with
// its working directory in a project it goes on to move.
func bumpedProject(t *testing.T, root, name string) string {
	t.Helper()
	dir := newProject(t, root, name, "1.0.0")
	if code, _, stderr := runMain(t, "", "-c", filepath.Join(dir, config.DefaultPath)); code != ExitOK {
		t.Fatalf("bump in %s exit = %d, stderr = %q", name, code, stderr)
	}
	return dir
}

// assertProjectAt checks that a newProject's VERSION and config both hold ver
// and that its journal has entries entries left.
func assertProjectAt(t *testing.T, dir, ver string, entries int) {
	t.Helper()
	if got, _ := os.ReadFile(filepath.Join(dir, "VERSION")); string(got) != ver+"\n" {
		t.Errorf("%s/VERSION = %q, want %s", filepath.Base(dir), got, ver)
	}
	cfg, err := config.Load(filepath.Join(dir, config.DefaultPath))
	if err != nil {
		t.Fatalf("loading %s's config: %v", filepath.Base(dir), err)
	}
	if got := cfg.Files[0].Token(); got != ver {
		t.Errorf("%s's config records %s, want %s", filepath.Base(dir), got, ver)
	}
	h, err := history.Load(stateFile(dir))
	if err != nil {
		t.Fatal(err)
	}
	if len(h.Entries) != entries {
		t.Errorf("%s's journal has %d entries, want %d", filepath.Base(dir), len(h.Entries), entries)
	}
}

// writeJournal writes body as dir's state file.
func writeJournal(t *testing.T, dir, body string) {
	t.Helper()
	if err := os.WriteFile(stateFile(dir), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// oldEntry renders a journal entry in the form versions before Milestone 34
// wrote: the config's absolute path on the entry and each file's absolute path
// in `fs`. Literal strings keep Windows paths free of escapes.
func oldEntry(cfg, path, fs, from, to string) string {
	return fmt.Sprintf("[[entries]]\ntimestamp = 2026-01-01T00:00:00Z\nconfig = '%s'\n\n"+
		"[[entries.changes]]\npath = '%s'\nfs = '%s'\nold = %q\nnew = %q\n\n", cfg, path, fs, from, to)
}

// Undo in a copy of a project reverts the copy and leaves the original alone.
// It used to revert the original, print the change as though it were local,
// and pop the copy's entry, while the original's journal went on recording a
// bump that had been undone.
func TestUndoInCopiedProject(t *testing.T) {
	root := t.TempDir()
	orig := bumpedProject(t, root, "orig")
	copied := filepath.Join(root, "copy")
	if err := os.CopyFS(copied, os.DirFS(orig)); err != nil {
		t.Fatal(err)
	}
	before := snapshotOutside(t, root, copied)

	code, stdout, stderr := runMain(t, copied, "undo")
	if code != ExitOK {
		t.Fatalf("undo exit = %d, stderr = %q", code, stderr)
	}
	if !strings.Contains(stdout, "VERSION: 1.0.1 -> 1.0.0") {
		t.Errorf("stdout = %q, want the revert summary", stdout)
	}
	assertSnapshotEqual(t, before, snapshotOutside(t, root, copied))
	assertProjectAt(t, copied, "1.0.0", 0)

	// The original still has its own bump to undo.
	if code, _, stderr := runMain(t, orig, "undo"); code != ExitOK {
		t.Fatalf("undo in the original exit = %d, stderr = %q", code, stderr)
	}
	assertProjectAt(t, orig, "1.0.0", 0)
}

// Undo in a project that was moved after the bump reverts it where it now is.
// It used to fail looking for the config at the old location, beside a config
// that exists.
func TestUndoInMovedProject(t *testing.T) {
	root := t.TempDir()
	moved := filepath.Join(root, "after")
	if err := os.Rename(bumpedProject(t, root, "before"), moved); err != nil {
		t.Fatal(err)
	}

	if code, _, stderr := runMain(t, moved, "undo"); code != ExitOK {
		t.Fatalf("undo exit = %d, stderr = %q", code, stderr)
	}
	assertProjectAt(t, moved, "1.0.0", 0)
}

// A bump and an undo run from a subdirectory with -c ../incrmit.toml record
// and resolve paths against the config, not the working directory, and a new
// entry carries neither of the absolute paths older versions stored.
func TestUndoFromSubdirectory(t *testing.T) {
	dir := newProject(t, t.TempDir(), "project", "1.0.0")
	sub := filepath.Join(dir, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := filepath.Join("..", config.DefaultPath)

	if code, _, stderr := runMain(t, sub, "-c", cfg); code != ExitOK {
		t.Fatalf("bump exit = %d, stderr = %q", code, stderr)
	}
	h, err := history.Load(stateFile(dir))
	if err != nil {
		t.Fatal(err)
	}
	e, ok := h.Latest()
	if !ok || len(e.Changes) != 1 || e.Changes[0].Path != "VERSION" {
		t.Fatalf("journal = %+v, want one change recorded as the config lists it", h.Entries)
	}
	data, err := os.ReadFile(stateFile(dir))
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"fs", "config"} {
		if regexp.MustCompile(`(?m)^\s*` + key + ` = `).Match(data) {
			t.Errorf("the new entry still records %q:\n%s", key, data)
		}
	}

	if code, _, stderr := runMain(t, sub, "undo", "-c", cfg); code != ExitOK {
		t.Fatalf("undo exit = %d, stderr = %q", code, stderr)
	}
	assertProjectAt(t, dir, "1.0.0", 0)
}

// A config that lists a file outside its own directory (../shared/VERSION)
// is undone through that path, resolved against the config even when undo runs
// from somewhere else.
func TestUndoConfigListsParentPath(t *testing.T) {
	root := t.TempDir()
	shared := filepath.Join(root, "shared", "VERSION")
	dir := filepath.Join(root, "project")
	for _, d := range []string{filepath.Dir(shared), dir} {
		if err := os.Mkdir(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(shared, []byte("1.0.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(dir, config.DefaultPath)
	if err := os.WriteFile(cfgPath, []byte("[[files]]\npath = \"../shared/VERSION\"\nversion = \"1.0.0\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if code, _, stderr := runMain(t, dir); code != ExitOK {
		t.Fatalf("bump exit = %d, stderr = %q", code, stderr)
	}
	if got, _ := os.ReadFile(shared); string(got) != "1.0.1\n" {
		t.Fatalf("shared/VERSION after bump = %q, want 1.0.1", got)
	}

	code, stdout, stderr := runMain(t, root, "undo", "-c", filepath.Join("project", config.DefaultPath))
	if code != ExitOK {
		t.Fatalf("undo exit = %d, stderr = %q", code, stderr)
	}
	if !strings.Contains(stdout, "../shared/VERSION: 1.0.1 -> 1.0.0") {
		t.Errorf("stdout = %q, want the path as the config lists it", stdout)
	}
	if got, _ := os.ReadFile(shared); string(got) != "1.0.0\n" {
		t.Errorf("shared/VERSION after undo = %q, want 1.0.0", got)
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.Files[0].Token(); got != "1.0.0" {
		t.Errorf("config records %s after undo, want 1.0.0", got)
	}
}

// A journal written by an earlier version still loads, and its entries are
// undone against the current project: the recorded `fs` and `config`, which
// here name a directory that no longer exists, are ignored. Saving the journal
// after the pop drops both keys from the entries that remain.
func TestUndoReadsOldStateFile(t *testing.T) {
	root := t.TempDir()
	dir := newProject(t, root, "project", "1.0.1")
	gone := filepath.Join(root, "gone")
	writeJournal(t, dir,
		oldEntry(filepath.Join(gone, config.DefaultPath), "VERSION", filepath.Join(gone, "VERSION"), "0.9.0", "1.0.0")+
			oldEntry(filepath.Join(gone, config.DefaultPath), "VERSION", filepath.Join(gone, "VERSION"), "1.0.0", "1.0.1"))

	if code, _, stderr := runMain(t, dir, "undo"); code != ExitOK {
		t.Fatalf("undo exit = %d, stderr = %q", code, stderr)
	}
	assertProjectAt(t, dir, "1.0.0", 1)
	data, err := os.ReadFile(stateFile(dir))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), gone) {
		t.Errorf("the rewritten journal still records the old location:\n%s", data)
	}

	if code, _, stderr := runMain(t, dir, "undo"); code != ExitOK {
		t.Fatalf("second undo exit = %d, stderr = %q", code, stderr)
	}
	assertProjectAt(t, dir, "0.9.0", 0)
}

// A state file is not trusted input the way the config is: a repository can
// commit one. Whatever it records, undo writes only to files the config lists
// and to the config it was given. A recorded `fs` or `config` elsewhere is
// ignored, and a change to a path the config does not list is refused before
// anything is written.
func TestUndoPlantedJournal(t *testing.T) {
	tests := []struct {
		name string
		// journal renders the state file from the sandbox paths.
		journal func(root, victim, otherCfg string) string
		refused bool
	}{
		{
			name: "fs names a file outside the project",
			journal: func(_, victim, _ string) string {
				return oldEntry(config.DefaultPath, "VERSION", victim, "1.0.0", "1.0.1")
			},
		},
		{
			name: "config names another project's config",
			journal: func(_, _, otherCfg string) string {
				return oldEntry(otherCfg, "VERSION", "VERSION", "1.0.0", "1.0.1")
			},
		},
		{
			name: "path climbs out of the project",
			journal: func(_, victim, _ string) string {
				return oldEntry(config.DefaultPath, "../victim.txt", victim, "1.0.0", "1.0.1")
			},
			refused: true,
		},
		{
			name: "path is absolute",
			journal: func(_, victim, _ string) string {
				return oldEntry(config.DefaultPath, victim, victim, "1.0.0", "1.0.1")
			},
			refused: true,
		},
	}
	for _, tt := range tests {
		for _, args := range [][]string{{"undo", "--dry-run"}, {"undo"}} {
			t.Run(tt.name+"/"+strings.Join(args, " "), func(t *testing.T) {
				root := t.TempDir()
				dir := newProject(t, root, "project", "1.0.1")
				other := newProject(t, root, "other", "1.0.1")
				victim := filepath.Join(root, "victim.txt")
				if err := os.WriteFile(victim, []byte("app 1.0.1\n"), 0o644); err != nil {
					t.Fatal(err)
				}
				writeJournal(t, dir, tt.journal(root, victim, filepath.Join(other, config.DefaultPath)))
				before, beforeProject := snapshotOutside(t, root, dir), snapshotProject(t, dir)

				code, _, stderr := runMain(t, dir, args...)
				assertSnapshotEqual(t, before, snapshotOutside(t, root, dir))
				if !tt.refused {
					if code != ExitOK {
						t.Fatalf("exit = %d, stderr = %q", code, stderr)
					}
					if len(args) == 1 {
						assertProjectAt(t, dir, "1.0.0", 0)
					}
					return
				}
				if code != ExitError {
					t.Errorf("exit = %d, want %d (stderr %q)", code, ExitError, stderr)
				}
				if !strings.Contains(stderr, "victim.txt") || !strings.Contains(stderr, "not listed") {
					t.Errorf("stderr = %q, want it to name the path the config does not list", stderr)
				}
				assertSnapshotEqual(t, beforeProject, snapshotProject(t, dir))
			})
		}
	}
}

// A config edited since the bump, so that it no longer lists a recorded file
// at the version the bump wrote, stops the undo before anything is written,
// and the entry stays for a retry once the config is put back.
func TestUndoRefusesWhatTheConfigDoesNotList(t *testing.T) {
	tests := []struct {
		name   string
		config string
	}{
		{"version edited", "[[files]]\npath = \"VERSION\"\nversion = \"1.0.5\"\n" +
			"[[files]]\npath = \"notes.md\"\nversion = \"2.0.1\"\n"},
		{"entry removed", "[[files]]\npath = \"notes.md\"\nversion = \"2.0.1\"\n"},
		{"path respelled", "[[files]]\npath = \"./VERSION\"\nversion = \"1.0.1\"\n" +
			"[[files]]\npath = \"notes.md\"\nversion = \"2.0.1\"\n"},
	}
	for _, tt := range tests {
		for _, args := range [][]string{{"undo", "--dry-run"}, {"undo"}} {
			t.Run(tt.name+"/"+strings.Join(args, " "), func(t *testing.T) {
				dir := project(t, "[[files]]\npath = \"VERSION\"\nversion = \"1.0.0\"\n"+
					"[[files]]\npath = \"notes.md\"\nversion = \"2.0.0\"\n",
					map[string]string{"VERSION": "1.0.0\n", "notes.md": "lib 2.0.0\n"})
				if code, _, stderr := runMain(t, dir); code != ExitOK {
					t.Fatalf("bump exit = %d, stderr = %q", code, stderr)
				}
				if err := os.WriteFile(filepath.Join(dir, config.DefaultPath), []byte(tt.config), 0o644); err != nil {
					t.Fatal(err)
				}
				before := snapshotProject(t, dir)

				code, _, stderr := runMain(t, dir, args...)
				if code != ExitError {
					t.Errorf("exit = %d, want %d (stderr %q)", code, ExitError, stderr)
				}
				if !strings.Contains(stderr, "VERSION: not listed in incrmit.toml at 1.0.1") {
					t.Errorf("stderr = %q, want it to name the unlisted change", stderr)
				}
				assertSnapshotEqual(t, before, snapshotProject(t, dir))
			})
		}
	}
}
