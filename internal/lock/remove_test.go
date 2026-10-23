package lock

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/sasmaq/incrmit/internal/config"
)

// These tests drive the race that kept the lock file on disk between runs until
// Milestone 42. Removing the name on release lets a run that opened the file
// just before the removal go on to lock the nameless file, while a run arriving
// after creates a fresh file at the name and locks that, and both proceed. The
// seams in Acquire and Release stop a run between its steps, so a test can let
// other runs in at each point. A lock belongs to the open file description, so
// runs inside one goroutine contend exactly as separate processes would. The
// assertion is the invariant rather than the damage (at most one run holds the
// project), so the tests that reproduced the race are the ones that prove the
// fix.

// seam points *hook at f for the rest of the test. f runs only the first time
// the hook is called: a run started from inside f passes the same seam, and
// must go straight through it.
func seam(t *testing.T, hook *func(), f func()) {
	t.Helper()
	saved := *hook
	t.Cleanup(func() { *hook = saved })
	fired := false
	*hook = func() {
		if !fired {
			fired = true
			f()
		}
	}
}

// holding returns the locks among ls that hold the project: taken, not
// degraded, and not yet released.
func holding(ls ...*Lock) []*Lock {
	var held []*Lock
	for _, l := range ls {
		if l != nil && !l.Degraded() && l.f != nil {
			held = append(held, l)
		}
	}
	return held
}

// assertOneHolder checks the invariant: exactly one of ls holds the project,
// and a newcomer contends with it. Two would mean the race got through, and
// none that a run lost the project to nobody.
func assertOneHolder(t *testing.T, dir string, ls ...*Lock) {
	t.Helper()
	if held := holding(ls...); len(held) != 1 {
		t.Fatalf("%d runs hold the project, want exactly 1", len(held))
	}
	if l, err := Acquire(dir); !errors.Is(err, ErrContended) {
		_ = l.Release()
		t.Errorf("a newcomer got (%+v, %v) while a run held the project, want ErrContended", l, err)
	}
}

// acquireOrContend runs Acquire in dir for a run the test names, and fails the
// test on anything but the lock or contention.
func acquireOrContend(t *testing.T, run, dir string) *Lock {
	t.Helper()
	l, err := Acquire(dir)
	if err != nil && !errors.Is(err, ErrContended) {
		t.Errorf("%s: Acquire = %v, want the lock or ErrContended", run, err)
	}
	if l.Degraded() {
		t.Errorf("%s: Acquire degraded: %v", run, l.Reason())
	}
	t.Cleanup(func() { _ = l.Release() })
	return l
}

// assertRemoved checks that a released lock file is gone, or, on Windows, that
// it was kept (see removeLockFile).
func assertRemoved(t *testing.T, path string) {
	t.Helper()
	_, err := os.Lstat(path)
	switch {
	case runtime.GOOS == "windows" && err != nil:
		t.Errorf("lock file gone on Windows, where Release keeps it: %v", err)
	case runtime.GOOS != "windows" && !errors.Is(err, fs.ErrNotExist):
		t.Errorf("lock file still there after release (lstat err = %v)", err)
	}
}

// skipOnWindows skips a test that removes or renames a held lock file, which
// Windows does not allow while the file is open.
func skipOnWindows(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("a held lock file cannot be removed or renamed on Windows")
	}
}

// A holds the project, and B has opened the lock file when A releases. B goes
// on to lock what it opened, and C arrives at the name A released, either
// before B takes its lock or after. One of B and C holds the project, and the
// other is told it is contended.
func TestReleaseRaceLeavesOneHolder(t *testing.T) {
	for _, tc := range []struct {
		name       string
		afterBLock bool // C arrives once B holds its lock, not before
	}{
		{"C before B locks", false},
		{"C after B locks", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			a := acquire(t, dir)

			var c *Lock
			arriveC := func() { c = acquireOrContend(t, "C", dir) }
			seam(t, &afterOpen, func() {
				if err := a.Release(); err != nil {
					t.Errorf("releasing A: %v", err)
				}
				if !tc.afterBLock {
					arriveC()
				}
			})
			if tc.afterBLock {
				seam(t, &afterLock, arriveC)
			}
			b := acquireOrContend(t, "B", dir)

			assertOneHolder(t, dir, b, c)
		})
	}
}

// Release removes the name before it unlocks. In the other order a run could
// lock the file in between and pass its check, since the name still leads
// there, and then lose the name to a newcomer that creates and locks a fresh
// file. B arrives in that gap and C after it; swapping the two steps in Release
// leaves both holding the project.
func TestReleaseRemovesBeforeUnlocking(t *testing.T) {
	dir := t.TempDir()
	a := acquire(t, dir)

	var b *Lock
	seam(t, &afterRemove, func() { b = acquireOrContend(t, "B", dir) })
	if err := a.Release(); err != nil {
		t.Fatalf("releasing A: %v", err)
	}
	c := acquireOrContend(t, "C", dir)

	assertOneHolder(t, dir, b, c)
}

// Release removes a lock file incrmit made, whoever made it: this run, a run
// that crashed before writing the note, or an incrmit from before the file was
// removed between runs, whose note was different.
func TestReleaseRemovesTheFileIncrmitMade(t *testing.T) {
	for _, tc := range []struct {
		name  string
		plant bool // a file holding body is already at the lock path
		body  string
	}{
		{"created by this run", false, ""},
		{"empty", true, ""},
		{"holding the note", true, note},
		{"holding the earlier note", true, notes[1]},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, config.LockFileName)
			if tc.plant {
				if err := os.WriteFile(path, []byte(tc.body), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			l := acquire(t, dir)
			if err := l.Release(); err != nil {
				t.Fatal(err)
			}
			assertRemoved(t, path)
		})
	}
}

// A lock file holding anything but a note, or known by a second name, is not
// incrmit's to remove: Release unlocks it and leaves it exactly as it was.
func TestReleaseKeepsAFileIncrmitDidNotMake(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
		link bool // the lock path is a second name for a file elsewhere
	}{
		{"other text", "not incrmit's\n", false},
		{"the note and more", note + "and a line someone added\n", false},
		{"the start of the note", note[:20], false},
		{"empty with a second name", "", true},
		{"the note with a second name", note, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, config.LockFileName)
			src := path
			if tc.link {
				src = filepath.Join(t.TempDir(), "elsewhere")
			}
			if err := os.WriteFile(src, []byte(tc.body), 0o600); err != nil {
				t.Fatal(err)
			}
			if tc.link {
				if err := os.Link(src, path); err != nil {
					t.Skipf("hard links unsupported here: %v", err)
				}
			}

			l := acquire(t, dir)
			if err := l.Release(); err != nil {
				t.Fatal(err)
			}
			got, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("Release removed a file incrmit did not make: %v", err)
			}
			if string(got) != tc.body {
				t.Errorf("lock file = %q after release, want %q left alone", got, tc.body)
			}
		})
	}
}

// A run that dies holding the lock skips Release: the kernel drops the lock,
// and the file stays with the note in it. The next run takes the same file and
// removes it when it is done.
func TestFileLeftByCrashIsReusedAndRemoved(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, config.LockFileName)
	crashed := acquire(t, dir)
	_ = crashed.f.Close() // what the kernel does for a process that dies
	crashed.f = nil
	left, err := os.Lstat(path)
	if err != nil {
		t.Fatalf("no lock file left by the crashed run: %v", err)
	}

	l := acquire(t, dir)
	info, err := l.f.Stat()
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(left, info) {
		t.Error("the next run locked a new file instead of the one the crash left")
	}
	if err := l.Release(); err != nil {
		t.Fatal(err)
	}
	assertRemoved(t, path)
}

// A holder whose lock file was deleted or moved aside by hand finds another
// run's lock file at the name when it releases, and leaves it: that run still
// holds the project through it. Moved aside, A's file still has one name and
// holds the note, so only the check that the name leads to it keeps Release
// from removing B's.
func TestReleaseLeavesAnotherRunsFile(t *testing.T) {
	skipOnWindows(t)
	for _, tc := range []struct {
		name   string
		remove func(path string) error
	}{
		{"deleted", os.Remove},
		{"moved aside", func(path string) error { return os.Rename(path, path+".aside") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, config.LockFileName)
			a := acquire(t, dir)
			if err := tc.remove(path); err != nil {
				t.Fatal(err)
			}
			b := acquire(t, dir)
			t.Cleanup(func() { _ = b.Release() })

			if err := a.Release(); err != nil {
				t.Fatal(err)
			}
			assertOneHolder(t, dir, a, b)
			if _, err := os.Lstat(path); err != nil {
				t.Errorf("releasing A removed B's lock file: %v", err)
			}
		})
	}
}

// A name that moves every time this run takes its lock would spin forever. Past
// maxOpens, Acquire reports what it is, contention, having locked nothing.
func TestAcquireCapsItsOpens(t *testing.T) {
	skipOnWindows(t)
	dir := t.TempDir()
	path := filepath.Join(dir, config.LockFileName)
	saved := afterLock
	t.Cleanup(func() { afterLock = saved })
	locks := 0
	afterLock = func() {
		locks++
		if err := os.Remove(path); err != nil {
			t.Errorf("removing the name: %v", err)
		}
	}

	if l, err := Acquire(dir); !errors.Is(err, ErrContended) {
		_ = l.Release()
		t.Fatalf("Acquire = (%+v, %v) with a name that keeps moving, want ErrContended", l, err)
	}
	if locks != maxOpens {
		t.Errorf("Acquire locked %d times, want %d", locks, maxOpens)
	}
	afterLock = saved
	l := acquire(t, dir)
	_ = l.Release()
}

// The check after locking uses Lstat. A link swapped in at the name, here one to
// the very file this run locked under another name, does not match, and the
// open that follows refuses it as any link is refused: the lock degrades and
// nothing is written through the link.
func TestCheckAfterLockRejectsALinkSwappedIn(t *testing.T) {
	skipOnWindows(t)
	dir := t.TempDir()
	path := filepath.Join(dir, config.LockFileName)
	moved := filepath.Join(dir, "moved")
	if err := os.WriteFile(path, []byte("not incrmit's\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	seam(t, &afterLock, func() {
		if err := os.Rename(path, moved); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink("moved", path); err != nil {
			t.Skipf("symlinks unsupported: %v", err)
		}
	})

	l, err := Acquire(dir)
	if err != nil {
		t.Fatalf("Acquire = %v, want a degraded lock", err)
	}
	assertNotRegular(t, l, fs.ModeSymlink)
	if got, _ := os.ReadFile(moved); string(got) != "not incrmit's\n" {
		t.Errorf("the linked file = %q, want it untouched", got)
	}
}
