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
