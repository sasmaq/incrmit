package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/sasmaq/incrmit/internal/config"
	"github.com/sasmaq/incrmit/internal/files"
	"github.com/sasmaq/incrmit/internal/testutil"
)

// These tests put something other than a regular file where incrmit reads a
// file of its own: the config, the --output that discover reads its ignore list
// from, and the bump journal. Before Milestone 33 all three were read with
// os.ReadFile, which opens whatever is there. Opening a named pipe blocks until
// a writer appears, so preview and discover --dry-run hung on one at the config,
// and a bump hung on one at the state file after it had already rewritten the
// targets and the config. A link to /dev/zero read without end. Every command
// runs under runMainWithin's deadline, so a regression fails instead of hanging
// the suite.

const ownFilesConfig = "[[files]]\npath = \"VERSION\"\nversion = \"1.0.0\"\n"

// ownFileCommand is a command run over a project with something planted at
// one of incrmit's own files. at is the file replaced, and bumped says whether
// the project is bumped once first, so an undo has an entry to revert.
type ownFileCommand struct {
	name   string
	at     string
	bumped bool
	args   func(dir, cfgPath string) []string
}

func ownFileCommands() []ownFileCommand {
	bump := func(extra ...string) func(_, cfgPath string) []string {
		return func(_, cfgPath string) []string { return append([]string{"-c", cfgPath}, extra...) }
	}
	undo := func(extra ...string) func(_, cfgPath string) []string {
		return func(_, cfgPath string) []string { return append([]string{"undo", "-c", cfgPath}, extra...) }
	}
	discover := func(extra ...string) func(dir, cfgPath string) []string {
		return func(dir, cfgPath string) []string {
			return append([]string{"discover", "-P", dir, "-o", cfgPath}, extra...)
		}
	}
	return []ownFileCommand{
		{name: "config/bump", at: config.DefaultPath, args: bump()},
		{name: "config/bump --dry-run", at: config.DefaultPath, args: bump("--dry-run")},
		{name: "config/preview", at: config.DefaultPath, args: func(_, cfgPath string) []string {
			return []string{"preview", "-c", cfgPath}
		}},
		{name: "config/undo", at: config.DefaultPath, bumped: true, args: undo()},
		{name: "config/undo --dry-run", at: config.DefaultPath, bumped: true, args: undo("--dry-run")},
		{name: "output/discover", at: config.DefaultPath, args: discover()},
		{name: "output/discover --dry-run", at: config.DefaultPath, args: discover("--dry-run")},
		{name: "state/bump", at: config.StateFileName, args: bump()},
		{name: "state/bump --dry-run", at: config.StateFileName, args: bump("--dry-run")},
		{name: "state/undo", at: config.StateFileName, bumped: true, args: undo()},
		{name: "state/undo --dry-run", at: config.StateFileName, bumped: true, args: undo("--dry-run")},
	}
}

// snapshotProject records every file in the project the way snapshotOutside
// does, except the lock file, which a writing command creates before it reads
// anything else.
func snapshotProject(t *testing.T, dir string) map[string]string {
	t.Helper()
	snap := snapshotOutside(t, dir, "")
	delete(snap, filepath.Join(dir, config.LockFileName))
	return snap
}

// ownFilePlant replaces path with something that is not a regular file.
type ownFilePlant struct {
	name  string
	plant func(t *testing.T, path string)
}

func ownFilePlants() []ownFilePlant {
	return []ownFilePlant{
		{name: "fifo", plant: func(t *testing.T, path string) {
			t.Helper()
			if err := testutil.Mkfifo(path); err != nil {
				t.Skipf("FIFOs unsupported: %v", err)
			}
		}},
		// A repository cannot commit a FIFO, but it can commit this link.
		{name: "link to /dev/zero", plant: func(t *testing.T, path string) {
			t.Helper()
			if runtime.GOOS == "windows" {
				t.Skip("no /dev/zero on Windows")
			}
			if _, err := os.Stat("/dev/zero"); err != nil {
				t.Skipf("no /dev/zero here: %v", err)
			}
			if err := os.Symlink("/dev/zero", path); err != nil {
				t.Skipf("cannot create a symbolic link here: %v", err)
			}
		}},
	}
}

// Every command that reads the config, the --output, or the journal refuses
// one that is not a regular file promptly, with exit 1, a message naming the
// file, and nothing written.
func TestOwnFilesNotRegular(t *testing.T) {
	for _, p := range ownFilePlants() {
		t.Run(p.name, func(t *testing.T) {
			for _, c := range ownFileCommands() {
				t.Run(c.name, func(t *testing.T) {
					dir := project(t, ownFilesConfig, map[string]string{"VERSION": "1.0.0\n"})
					cfgPath := filepath.Join(dir, config.DefaultPath)
					if c.bumped {
						if code, _, stderr := runMain(t, "", "-c", cfgPath); code != ExitOK {
							t.Fatalf("setup bump exit = %d, stderr = %q", code, stderr)
						}
					}
					at := filepath.Join(dir, c.at)
					if err := os.Remove(at); err != nil && !os.IsNotExist(err) {
						t.Fatal(err)
					}
					p.plant(t, at)
					before := snapshotProject(t, dir)

					code, _, stderr := runMainWithin(t, c.args(dir, cfgPath)...)
					if code != ExitError {
						t.Errorf("exit = %d, want %d (stderr %q)", code, ExitError, stderr)
					}
					for _, want := range []string{c.at, "not a regular file"} {
						if !strings.Contains(stderr, want) {
							t.Errorf("stderr = %q, want it to contain %q", stderr, want)
						}
					}
					assertSnapshotEqual(t, before, snapshotProject(t, dir))
				})
			}
		})
	}
}

// A journal that cannot be read fails a bump before anything is written. It
// was read last, after the targets and the config already held the new
// version, so the bump went through with no record for undo to find.
func TestBumpReadsJournalBeforeWriting(t *testing.T) {
	for _, args := range [][]string{nil, {"--dry-run"}} {
		t.Run("args="+strings.Join(args, " "), func(t *testing.T) {
			dir := project(t, ownFilesConfig, map[string]string{
				"VERSION":            "1.0.0\n",
				config.StateFileName: "entries = [ this is not toml\n",
			})
			cfgPath := filepath.Join(dir, config.DefaultPath)
			before := snapshotProject(t, dir)

			code, stdout, stderr := runMainWithin(t, append([]string{"-c", cfgPath}, args...)...)
			if code != ExitError {
				t.Errorf("exit = %d, want %d (stdout %q, stderr %q)", code, ExitError, stdout, stderr)
			}
			if !strings.Contains(stderr, config.StateFileName) {
				t.Errorf("stderr = %q, want it to name the state file", stderr)
			}
			assertSnapshotEqual(t, before, snapshotProject(t, dir))
		})
	}
}

// A config, --output, or journal over files.MaxOwnFileBytes is refused with a
// message naming it, before anything is written. The planted file is sparse,
// so the test costs no disk.
func TestOwnFilesOverSizeCap(t *testing.T) {
	for _, c := range ownFileCommands() {
		t.Run(c.name, func(t *testing.T) {
			dir := project(t, ownFilesConfig, map[string]string{"VERSION": "1.0.0\n"})
			cfgPath := filepath.Join(dir, config.DefaultPath)
			if c.bumped {
				if code, _, stderr := runMain(t, "", "-c", cfgPath); code != ExitOK {
					t.Fatalf("setup bump exit = %d, stderr = %q", code, stderr)
				}
			}
			at := filepath.Join(dir, c.at)
			if err := os.WriteFile(at, nil, 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.Truncate(at, files.MaxOwnFileBytes+1); err != nil {
				t.Fatal(err)
			}
			before := snapshotProject(t, dir)

			code, _, stderr := runMainWithin(t, c.args(dir, cfgPath)...)
			if code != ExitError {
				t.Errorf("exit = %d, want %d (stderr %q)", code, ExitError, stderr)
			}
			for _, want := range []string{c.at, fmt.Sprint(files.MaxOwnFileBytes), "limit"} {
				if !strings.Contains(stderr, want) {
					t.Errorf("stderr = %q, want it to contain %q", stderr, want)
				}
			}
			assertSnapshotEqual(t, before, snapshotProject(t, dir))
		})
	}
}

// A symlinked config is a legitimate setup, so every command that only reads
// it still reads through the link.
func TestSymlinkedConfigIsRead(t *testing.T) {
	dir := project(t, "", map[string]string{
		"VERSION":     "1.0.0\n",
		"shared.toml": "ignore = [\"docs/\"]\n\n" + ownFilesConfig,
	})
	cfgPath := filepath.Join(dir, config.DefaultPath)
	if err := os.Symlink("shared.toml", cfgPath); err != nil {
		t.Skipf("cannot create a symbolic link here: %v", err)
	}

	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"-c", cfgPath, "--dry-run"}, "VERSION: 1.0.0 -> 1.0.1"},
		{[]string{"preview", "-c", cfgPath}, "1.0.1"},
		{[]string{"discover", "-P", dir, "-o", cfgPath, "--dry-run"}, "(ignoring: docs/)"},
	} {
		code, stdout, stderr := runMainWithin(t, tc.args...)
		if code != ExitOK {
			t.Errorf("incrmit %s: exit = %d, stderr = %q", strings.Join(tc.args, " "), code, stderr)
			continue
		}
		if !strings.Contains(stdout, tc.want) {
			t.Errorf("incrmit %s: stdout = %q, want it to contain %q", strings.Join(tc.args, " "), stdout, tc.want)
		}
	}
}
