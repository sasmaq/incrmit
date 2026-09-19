//go:build windows

package lock

import (
	"os"
	"syscall"
	"unsafe"
)

var (
	modkernel32      = syscall.NewLazyDLL("kernel32.dll")
	procLockFileEx   = modkernel32.NewProc("LockFileEx")
	procUnlockFileEx = modkernel32.NewProc("UnlockFileEx")
)

const (
	lockfileFailImmediately = 0x00000001
	lockfileExclusiveLock   = 0x00000002
	// ERROR_LOCK_VIOLATION is what LockFileEx reports when the range is
	// already held and it was told not to wait.
	errorLockViolation = syscall.Errno(33)
)

// tryLock takes an exclusive lock on the first byte of f without blocking.
//
// Windows has no flock; LockFileEx locks a byte range, and locking a single
// byte of an otherwise unused file is the standard stand-in. The lock is held
// by the file handle, so — as on Unix — two handles opened separately contend
// even inside one process, and closing the handle releases it.
func tryLock(f *os.File) error {
	var ol syscall.Overlapped
	r1, _, err := procLockFileEx.Call(
		f.Fd(),
		uintptr(lockfileExclusiveLock|lockfileFailImmediately),
		0,
		1, // one byte, low word
		0, // one byte, high word
		uintptr(unsafe.Pointer(&ol)),
	)
	if r1 != 0 {
		return nil
	}
	if err == errorLockViolation || err == syscall.ERROR_IO_PENDING {
		return ErrContended
	}
	return err
}

// unlock releases the byte range locked by tryLock.
func unlock(f *os.File) error {
	var ol syscall.Overlapped
	r1, _, err := procUnlockFileEx.Call(f.Fd(), 0, 1, 0, uintptr(unsafe.Pointer(&ol)))
	if r1 != 0 {
		return nil
	}
	return err
}
