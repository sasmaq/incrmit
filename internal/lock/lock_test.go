package lock

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sasmaq/incrmit/internal/config"
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
