package cli

import (
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/sasmaq/incrmit/internal/config"
	"github.com/sasmaq/incrmit/internal/testutil"
)

// These tests plant something other than incrmit's own file at .incrmit.lock
// and run every writing command over it. A repository can commit the lock file
// as a symbolic link, and before Milestone 32 the lock followed one: it opened
// the path with O_CREATE|O_RDWR and truncated whatever it got to write its
// note, so `.incrmit.lock -> ../victim.txt` left victim.txt holding two lines
// of lock text after a discover or a bump, and a dangling link created the file
// it named. The assertions are about everything outside the project, so the
// tests that reproduced the damage are the ones that prove the fix.

// lockSandbox builds a one-file project at 1.0.0 one level down inside a
// sandbox directory, so a planted link has somewhere outside the project to
// point. The sandbox holds a file and a directory for links to name.
func lockSandbox(t *testing.T) (root, dir, cfgPath string) {
	t.Helper()
	root = t.TempDir()
	dir = filepath.Join(root, "project")
	for _, d := range []string{dir, filepath.Join(root, "outside-dir")} {
		if err := os.Mkdir(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for name, body := range map[string]string{
		filepath.Join(root, "victim.txt"):          "precious data\n",
		filepath.Join(root, "outside-dir", "kept"): "also precious\n",
		filepath.Join(dir, "VERSION"):              "1.0.0\n",
		filepath.Join(dir, "incrmit.toml"):         "[[files]]\npath = \"VERSION\"\nversion = \"1.0.0\"\n",
	} {
		if err := os.WriteFile(name, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// A mode the lock would not produce, so a rewrite that preserved the bytes
	// but not the mode still shows.
	if err := os.Chmod(filepath.Join(root, "victim.txt"), 0o640); err != nil {
		t.Fatal(err)
	}
	return root, dir, filepath.Join(dir, "incrmit.toml")
}

// snapshotOutside records every path under root that is not inside dir: its
// type and permission bits, and its contents or, for a link, its target. Two
// snapshots that compare equal mean nothing outside the project was created,
// removed, or changed.
func snapshotOutside(t *testing.T, root, dir string) map[string]string {
	t.Helper()
	snap := map[string]string{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == dir {
			return filepath.SkipDir
		}
		info, err := d.Info() // an lstat: WalkDir does not follow links
		if err != nil {
			return err
		}
		var body string
		switch {
		case info.Mode()&fs.ModeSymlink != 0:
			body, err = os.Readlink(path)
		case info.Mode().IsRegular():
			var b []byte
			b, err = os.ReadFile(path)
			body = string(b)
		}
		if err != nil {
			return err
		}
		snap[path] = fmt.Sprintf("%v %q", info.Mode(), body)
		return nil
	})
	if err != nil {
		t.Fatalf("snapshotting %s: %v", root, err)
	}
	return snap
}

// assertSnapshotEqual reports every difference between two snapshots.
func assertSnapshotEqual(t *testing.T, before, after map[string]string) {
	t.Helper()
	for _, p := range slices.Sorted(maps.Keys(before)) {
		switch a, ok := after[p]; {
		case !ok:
			t.Errorf("%s was removed (it was %s)", p, before[p])
		case a != before[p]:
			t.Errorf("%s changed outside the project:\n  before %s\n  after  %s", p, before[p], a)
		}
	}
	for _, p := range slices.Sorted(maps.Keys(after)) {
		if _, ok := before[p]; !ok {
			t.Errorf("%s was created outside the project: %s", p, after[p])
		}
	}
}

// runMainWithin is runMain under a deadline, so a lock path that blocks the open
// (a FIFO, say) fails the test instead of hanging the suite.
func runMainWithin(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	type outcome struct {
		code           int
		stdout, stderr string
	}
	done := make(chan outcome, 1)
	go func() {
		code, stdout, stderr := runMain(t, "", args...)
		done <- outcome{code, stdout, stderr}
	}()
	select {
	case got := <-done:
		return got.code, got.stdout, got.stderr
	case <-time.After(10 * time.Second):
		t.Fatalf("incrmit %s blocked on the lock file instead of finishing", strings.Join(args, " "))
		return 0, "", ""
	}
}

// lockCommand is a writing command run over a lockSandbox project. setup, when
// set, runs before anything is planted at the lock path, so an undo has a bump
// to revert.
type lockCommand struct {
	name  string
	args  func(dir, cfgPath string) []string
	setup bool
}

func lockCommands() []lockCommand {
	return []lockCommand{
		{name: "discover", args: func(dir, cfgPath string) []string {
			return []string{"discover", "-P", dir, "-o", cfgPath}
		}},
		{name: "bump", args: func(_, cfgPath string) []string {
			return []string{"-c", cfgPath}
		}},
		{name: "undo", setup: true, args: func(_, cfgPath string) []string {
			return []string{"undo", "-c", cfgPath}
		}},
	}
}

// lockPlant is something put at .incrmit.lock before a command runs. degraded
// says whether the command must run unlocked, and why names the reason the
// warning has to give. check, when set, runs last with the lock path.
type lockPlant struct {
	plant    func(t *testing.T, root, lockPath string)
	degraded bool
	why      string
	check    func(t *testing.T, lockPath string)
}

// symlinkTo plants lockPath as a relative link to name in the sandbox root, the
// way a repository would commit one. It skips where the system will not create
// links (Windows without the privilege).
func symlinkTo(name string) func(t *testing.T, root, lockPath string) {
	return func(t *testing.T, _, lockPath string) {
		t.Helper()
		if err := os.Symlink(filepath.Join("..", name), lockPath); err != nil {
			t.Skipf("cannot create a symbolic link here: %v", err)
		}
	}
}

// runLockPlant runs every writing command over a fresh project with p planted
// at its lock path, and checks that the command behaved as decided (unlocked
// with a warning for anything that is not a regular file) and that nothing
// outside the project changed.
func runLockPlant(t *testing.T, p lockPlant) {
	t.Helper()
	for _, c := range lockCommands() {
		t.Run(c.name, func(t *testing.T) {
			root, dir, cfgPath := lockSandbox(t)
			lockPath := filepath.Join(dir, config.LockFileName)
			if c.setup {
				if code, _, stderr := runMain(t, "", "-c", cfgPath); code != ExitOK {
					t.Fatalf("setup bump exit = %d, stderr = %q", code, stderr)
				}
				if err := os.Remove(lockPath); err != nil {
					t.Fatal(err)
				}
			}
			p.plant(t, root, lockPath)
			before := snapshotOutside(t, root, dir)
			planted, err := os.Lstat(lockPath)
			if err != nil {
				t.Fatal(err)
			}

			code, _, stderr := runMainWithin(t, c.args(dir, cfgPath)...)
			if code != ExitOK {
				t.Fatalf("exit = %d, want the command to run (stderr %q)", code, stderr)
			}
			assertSnapshotEqual(t, before, snapshotOutside(t, root, dir))

			if p.check != nil {
				defer p.check(t, lockPath)
			}
			if !p.degraded {
				if stderr != "" {
					t.Errorf("stderr = %q, want a clean run over a regular lock file", stderr)
				}
				return
			}
			for _, want := range []string{"warning", config.LockFileName, p.why, "continuing unlocked"} {
				if !strings.Contains(stderr, want) {
					t.Errorf("stderr = %q, want it to contain %q", stderr, want)
				}
			}
			// The planted file is reported, not repaired: it stays exactly as it
			// was so the user can see what was put there.
			after, err := os.Lstat(lockPath)
			if err != nil {
				t.Fatalf("the planted lock path is gone: %v", err)
			}
			if after.Mode() != planted.Mode() {
				t.Errorf("lock path mode = %v, want it left as planted (%v)", after.Mode(), planted.Mode())
			}
		})
	}
}

// The case the milestone was opened for: a committed link to a file outside the
// project. The file must keep its bytes and its mode through every command.
func TestLockSymlinkToFileIsNotWrittenThrough(t *testing.T) {
	runLockPlant(t, lockPlant{
		plant:    symlinkTo("victim.txt"),
		degraded: true,
		why:      "symbolic link",
	})
}

// O_CREATE follows a dangling link and creates the file it names, so the file
// a dangling link points at must still not exist afterward.
func TestLockDanglingSymlinkCreatesNothing(t *testing.T) {
	runLockPlant(t, lockPlant{
		plant:    symlinkTo("created-by-incrmit"),
		degraded: true,
		why:      "symbolic link",
	})
}

// A link to a directory outside the project: nothing may be created inside it.
func TestLockSymlinkToDirectory(t *testing.T) {
	runLockPlant(t, lockPlant{
		plant:    symlinkTo("outside-dir"),
		degraded: true,
		why:      "symbolic link",
	})
}

// A FIFO at the lock path is not a lock file either. Opening one can block
// until a writer arrives, which runMainWithin turns into a failure.
func TestLockFIFO(t *testing.T) {
	runLockPlant(t, lockPlant{
		plant: func(t *testing.T, _, lockPath string) {
			t.Helper()
			if err := testutil.Mkfifo(lockPath); err != nil {
				t.Skipf("FIFOs unsupported: %v", err)
			}
		},
		degraded: true,
		why:      "named pipe",
	})
}

// A directory at the lock path cannot be opened as a file; the warning says
// what it is rather than passing on the open error.
func TestLockDirectory(t *testing.T) {
	runLockPlant(t, lockPlant{
		plant: func(t *testing.T, _, lockPath string) {
			t.Helper()
			if err := os.Mkdir(lockPath, 0o755); err != nil {
				t.Fatal(err)
			}
		},
		degraded: true,
		why:      "directory",
	})
}

// A regular file that happens to sit at the lock path is a usable lock file,
// so the command runs locked and quietly — but it holds someone's text, and the
// note is never written over it.
func TestLockRegularFileKeepsItsContents(t *testing.T) {
	const text = "not incrmit's: notes someone keeps here\n"
	runLockPlant(t, lockPlant{
		plant: func(t *testing.T, _, lockPath string) {
			t.Helper()
			if err := os.WriteFile(lockPath, []byte(text), 0o640); err != nil {
				t.Fatal(err)
			}
		},
		check: func(t *testing.T, lockPath string) {
			t.Helper()
			got, err := os.ReadFile(lockPath)
			if err != nil {
				t.Fatalf("reading the lock file back: %v", err)
			}
			if string(got) != text {
				t.Errorf("lock file = %q, want its text left alone (%q)", got, text)
			}
			if info, err := os.Stat(lockPath); err == nil && runtime.GOOS != "windows" && info.Mode().Perm() != 0o640 {
				t.Errorf("lock file mode = %v, want 0640 left alone", info.Mode().Perm())
			}
		},
	})
}
