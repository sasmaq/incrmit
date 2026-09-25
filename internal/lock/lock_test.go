package lock

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sasmaq/incrmit/internal/config"
	"github.com/sasmaq/incrmit/internal/testutil"
)

// acquire takes the lock in dir and fails the test if it cannot.
func acquire(t *testing.T, dir string) *Lock {
	t.Helper()
	l, err := Acquire(dir)
	if err != nil {
		t.Fatalf("Acquire(%q) = %v, want the lock", dir, err)
	}
	if l.Degraded() {
		t.Fatalf("Acquire(%q) degraded: %v", dir, l.Reason())
	}
	return l
}

// A second acquisition of a held lock is contention, and only contention: the
// caller has to be able to tell it apart from a filesystem that cannot lock.
func TestAcquireContends(t *testing.T) {
	dir := t.TempDir()
	first := acquire(t, dir)

	second, err := Acquire(dir)
	if !errors.Is(err, ErrContended) {
		t.Fatalf("second Acquire = (%v, %v), want ErrContended", second, err)
	}
	if second != nil {
		t.Errorf("second Acquire returned a lock alongside ErrContended: %+v", second)
	}

	if err := first.Release(); err != nil {
		t.Fatalf("Release: %v", err)
	}
	third := acquire(t, dir)
	_ = third.Release()
}

// The lock is scoped to one directory, so separate projects never wait on each
// other.
func TestAcquireIsPerDirectory(t *testing.T) {
	root := t.TempDir()
	a, b := filepath.Join(root, "a"), filepath.Join(root, "b")
	for _, d := range []string{a, b} {
		if err := os.Mkdir(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	la := acquire(t, a)
	defer func() { _ = la.Release() }()
	lb := acquire(t, b)
	defer func() { _ = lb.Release() }()
}

// Releasing twice (and releasing a nil lock) must be safe: callers defer the
// release on every return path.
func TestReleaseIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	l := acquire(t, dir)
	if err := l.Release(); err != nil {
		t.Fatalf("first Release: %v", err)
	}
	if err := l.Release(); err != nil {
		t.Errorf("second Release: %v", err)
	}
	var nilLock *Lock
	if err := nilLock.Release(); err != nil {
		t.Errorf("Release on a nil lock: %v", err)
	}
	if nilLock.Degraded() {
		t.Error("a nil lock reports itself degraded")
	}
}

// The lock file is left behind on purpose — unlinking it would let the next run
// lock a fresh file at the same name while this one still held the old inode —
// and it explains itself to whoever finds it.
func TestLockFileIsExplainedAndKept(t *testing.T) {
	dir := t.TempDir()
	l := acquire(t, dir)
	path := filepath.Join(dir, config.LockFileName)
	if l.Path() != path {
		t.Errorf("Path() = %q, want %q", l.Path(), path)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading the lock file: %v", err)
	}
	if !strings.Contains(string(body), "safe to delete") {
		t.Errorf("lock file = %q, want a note explaining what it is", body)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("lock file missing while held: %v", err)
	}
	if err := l.Release(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("lock file removed on release: %v", err)
	}
}

// The note must never read as a bump target, or incrmit's own lock file could
// be discovered and rewritten.
func TestNoteHasNoVersionToken(t *testing.T) {
	for _, bad := range []string{".0", "0.", "1.2.3"} {
		if strings.Contains(note, bad) {
			t.Errorf("note contains %q, which risks being matched as a version: %q", bad, note)
		}
	}
}

// AcquireWait gives up at its timeout rather than waiting forever.
func TestAcquireWaitTimesOut(t *testing.T) {
	dir := t.TempDir()
	held := acquire(t, dir)
	defer func() { _ = held.Release() }()

	start := time.Now()
	if _, err := AcquireWait(dir, 50*time.Millisecond); !errors.Is(err, ErrContended) {
		t.Fatalf("AcquireWait = %v, want ErrContended", err)
	}
	if elapsed := time.Since(start); elapsed < 50*time.Millisecond {
		t.Errorf("AcquireWait returned after %v, before its 50ms timeout", elapsed)
	}
}

// AcquireWait takes the lock as soon as the holder lets go.
func TestAcquireWaitSucceedsAfterRelease(t *testing.T) {
	dir := t.TempDir()
	held := acquire(t, dir)

	done := make(chan struct{})
	go func() {
		defer close(done)
		time.Sleep(30 * time.Millisecond)
		_ = held.Release()
	}()

	l, err := AcquireWait(dir, 10*time.Second)
	if err != nil {
		t.Fatalf("AcquireWait = %v, want the lock once it was released", err)
	}
	<-done
	_ = l.Release()
}

// When the lock cannot be taken for a reason that is not contention, Acquire
// reports no error: it hands back a degraded lock so the caller can warn and
// carry on rather than refuse to run. A directory sitting where the lock file
// belongs makes the open fail the way an unsupported filesystem makes the lock
// call fail.
func TestAcquireDegradesWhenUnavailable(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, config.LockFileName), 0o755); err != nil {
		t.Fatal(err)
	}

	l, err := Acquire(dir)
	if err != nil {
		t.Fatalf("Acquire = %v, want a degraded lock rather than an error", err)
	}
	if !l.Degraded() {
		t.Fatal("Acquire did not report the lock as degraded")
	}
	if l.Reason() == nil {
		t.Error("a degraded lock has no reason to report")
	}
	if err := l.Release(); err != nil {
		t.Errorf("Release on a degraded lock: %v", err)
	}
	// A degraded lock never contends, so a second run also proceeds.
	second, err := Acquire(dir)
	if err != nil || !second.Degraded() {
		t.Errorf("second Acquire = (%+v, %v), want another degraded lock", second, err)
	}
}

// assertNotRegular checks that l is degraded because the lock path holds the
// kind of file typ describes.
func assertNotRegular(t *testing.T, l *Lock, typ fs.FileMode) {
	t.Helper()
	if !l.Degraded() {
		_ = l.Release()
		t.Fatal("Acquire took the lock through something that is not a regular file")
	}
	var nr *NotRegularError
	if !errors.As(l.Reason(), &nr) {
		t.Fatalf("Reason() = %v, want a *NotRegularError", l.Reason())
	}
	if nr.Mode.Type()&typ == 0 {
		t.Errorf("NotRegularError.Mode = %v, want type %v", nr.Mode, typ)
	}
	if !strings.Contains(l.Reason().Error(), l.Path()) {
		t.Errorf("Reason() = %q, want it to name %q", l.Reason(), l.Path())
	}
}

// A lock path committed as a link to a file elsewhere must not be followed: the
// open refuses the link, the lock degrades, and the file it names keeps its
// bytes. Before Milestone 32 it was truncated and given the note.
func TestAcquireDoesNotFollowSymlink(t *testing.T) {
	outside := filepath.Join(t.TempDir(), "victim.txt")
	if err := os.WriteFile(outside, []byte("precious data\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	link := filepath.Join(dir, config.LockFileName)
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlinks unsupported: %v", err)
	}

	l, err := Acquire(dir)
	if err != nil {
		t.Fatalf("Acquire = %v, want a degraded lock rather than an error", err)
	}
	assertNotRegular(t, l, fs.ModeSymlink)
	if !strings.Contains(l.Reason().Error(), "symbolic link") {
		t.Errorf("Reason() = %q, want it to say the path is a symbolic link", l.Reason())
	}
	if got, _ := os.ReadFile(outside); string(got) != "precious data\n" {
		t.Errorf("link target = %q, want it untouched", got)
	}
	if info, err := os.Lstat(link); err != nil || info.Mode()&fs.ModeSymlink == 0 {
		t.Errorf("the link was not left as it was (lstat = %v, %v)", info, err)
	}
}

// O_CREATE on its own follows a dangling link and creates the file it names.
func TestAcquireDoesNotCreateThroughDanglingSymlink(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "created-by-incrmit")
	dir := t.TempDir()
	if err := os.Symlink(missing, filepath.Join(dir, config.LockFileName)); err != nil {
		t.Skipf("symlinks unsupported: %v", err)
	}

	l, err := Acquire(dir)
	if err != nil {
		t.Fatalf("Acquire = %v, want a degraded lock", err)
	}
	assertNotRegular(t, l, fs.ModeSymlink)
	if _, err := os.Lstat(missing); !os.IsNotExist(err) {
		t.Errorf("Acquire created the file a dangling link names (lstat err = %v)", err)
	}
}

// A FIFO opens without complaint, so what was opened is checked through the
// descriptor. The open must not wait for a writer either.
func TestAcquireRejectsFIFO(t *testing.T) {
	dir := t.TempDir()
	if err := testutil.Mkfifo(filepath.Join(dir, config.LockFileName)); err != nil {
		t.Skipf("FIFOs unsupported: %v", err)
	}

	done := make(chan *Lock, 1)
	go func() {
		l, _ := Acquire(dir)
		done <- l
	}()
	select {
	case l := <-done:
		assertNotRegular(t, l, fs.ModeNamedPipe)
	case <-time.After(10 * time.Second):
		t.Fatal("Acquire blocked opening a FIFO at the lock path")
	}
}

// A regular file already at the lock path is a usable lock file, but its
// contents are someone else's: the lock is taken and the note is not written.
func TestAcquireKeepsExistingContents(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, config.LockFileName)
	const text = "not incrmit's\n"
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}

	l := acquire(t, dir)
	if got, _ := os.ReadFile(path); string(got) != text {
		t.Errorf("lock file = %q while held, want %q left alone", got, text)
	}
	if err := l.Release(); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(path); string(got) != text {
		t.Errorf("lock file = %q after release, want %q left alone", got, text)
	}
}

// An empty lock file — one a run created and then lost the race to lock, or
// one left by a run that died before writing — gets the note on the next run.
func TestAcquireWritesNoteIntoEmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, config.LockFileName)
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	l := acquire(t, dir)
	defer func() { _ = l.Release() }()
	if got, _ := os.ReadFile(path); string(got) != note {
		t.Errorf("lock file = %q, want the note", got)
	}
}

// An empty lock file with a second name shares its contents with that name, so
// writing the note would change a file known by another; it is locked but left
// empty.
func TestAcquireDoesNotWriteNoteThroughHardLink(t *testing.T) {
	other := filepath.Join(t.TempDir(), "empty-elsewhere")
	if err := os.WriteFile(other, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.Link(other, filepath.Join(dir, config.LockFileName)); err != nil {
		t.Skipf("hard links unsupported here: %v", err)
	}

	l := acquire(t, dir)
	defer func() { _ = l.Release() }()
	if got, _ := os.ReadFile(other); len(got) != 0 {
		t.Errorf("the other name now holds %q, want it left empty", got)
	}
}

// The warning a user sees says what sits at the lock path, so each kind of file
// is named rather than lumped together as "not a regular file".
func TestNotRegularErrorNamesTheKind(t *testing.T) {
	for _, tt := range []struct {
		mode fs.FileMode
		want string
	}{
		{fs.ModeSymlink, "is a symbolic link, not a regular file"},
		{fs.ModeDir, "is a directory, not a regular file"},
		{fs.ModeNamedPipe, "is a named pipe, not a regular file"},
		{fs.ModeSocket, "is a socket, not a regular file"},
		{fs.ModeDevice | fs.ModeCharDevice, "is a device, not a regular file"},
		{fs.ModeIrregular, "is a special file, not a regular file"},
	} {
		if got := (&NotRegularError{Mode: tt.mode}).Error(); got != tt.want {
			t.Errorf("NotRegularError{%v}.Error() = %q, want %q", tt.mode, got, tt.want)
		}
	}
}
