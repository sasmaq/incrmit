//go:build !windows

package lock

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// Only EWOULDBLOCK means another run holds the lock. Any other flock failure is
// returned as itself so Acquire degrades instead of refusing to run; a closed
// descriptor (EBADF) stands in for the EOPNOTSUPP or ENOLCK a filesystem
// without flock would give.
func TestTryLockReportsNonContentionErrors(t *testing.T) {
	f, err := os.Create(filepath.Join(t.TempDir(), "lock"))
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	err = tryLock(f)
	if errors.Is(err, ErrContended) {
		t.Fatalf("tryLock = %v, want the flock error rather than contention", err)
	}
	if !errors.Is(err, syscall.EBADF) {
		t.Errorf("tryLock = %v, want EBADF", err)
	}
}

// A check after locking that cannot read the name at all, rather than finding
// it gone or changed, cannot tell whether the lock counts. That is not
// contention, so the lock degrades with the reason instead of starting over.
// Here the project directory is replaced by a file between the lock and the
// check, so the Lstat fails with ENOTDIR.
func TestCheckAfterLockDegradesOnLstatError(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "project")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	seam(t, &afterLock, func() {
		if err := os.Rename(dir, dir+".moved"); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(dir, nil, 0o600); err != nil {
			t.Fatal(err)
		}
	})

	l, err := Acquire(dir)
	if err != nil {
		t.Fatalf("Acquire = %v, want a degraded lock", err)
	}
	if !l.Degraded() {
		_ = l.Release()
		t.Fatal("Acquire kept a lock it could not check")
	}
	if !errors.Is(l.Reason(), syscall.ENOTDIR) {
		t.Errorf("Reason() = %v, want ENOTDIR", l.Reason())
	}
}
