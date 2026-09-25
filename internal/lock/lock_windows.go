//go:build windows

package lock

import (
	"errors"
	"os"

	"golang.org/x/sys/windows"
)

// tryLock takes an exclusive lock on the first byte of f without blocking.
//
// Windows has no flock; LockFileEx locks a byte range, and locking a single
// byte of an otherwise unused file is the standard stand-in. The lock is held
// by the file handle, so — as on Unix — two handles opened separately contend
// even inside one process, and closing the handle releases it.
func tryLock(f *os.File) error {
	var ol windows.Overlapped
	err := windows.LockFileEx(
		windows.Handle(f.Fd()),
		windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY,
		0,
		1, // one byte, low word
		0, // one byte, high word
		&ol,
	)
	switch {
	case err == nil:
		return nil
	case errors.Is(err, windows.ERROR_LOCK_VIOLATION), errors.Is(err, windows.ERROR_IO_PENDING):
		// ERROR_LOCK_VIOLATION is what LockFileEx reports when the range is
		// already held and it was told not to wait.
		return ErrContended
	default:
		return err
	}
}

// unlock releases the byte range locked by tryLock.
func unlock(f *os.File) error {
	var ol windows.Overlapped
	return windows.UnlockFileEx(windows.Handle(f.Fd()), 0, 1, 0, &ol)
}
