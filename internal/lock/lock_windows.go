//go:build windows

package lock

import (
	"errors"
	"io/fs"
	"os"

	"golang.org/x/sys/windows"
)

// openLockFile opens the lock file at path for reading and writing, creating it
// when nothing is there, without following a symbolic link or junction:
// FILE_FLAG_OPEN_REPARSE_POINT opens the link itself rather than what it names,
// and Acquire then rejects it as not a regular file. os.OpenFile has no way to
// pass that flag, so this calls CreateFile directly, with the sharing mode
// os.OpenFile uses. OPEN_ALWAYS is O_CREATE without O_TRUNC: the lock is not
// held yet and the contents are not ours to discard.
func openLockFile(path string) (*os.File, error) {
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: path, Err: err}
	}
	h, err := windows.CreateFile(name,
		windows.GENERIC_READ|windows.GENERIC_WRITE,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE,
		nil,
		windows.OPEN_ALWAYS,
		windows.FILE_ATTRIBUTE_NORMAL|windows.FILE_FLAG_OPEN_REPARSE_POINT,
		0,
	)
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: path, Err: err}
	}
	return os.NewFile(uintptr(h), path), nil
}

// soleName reports whether the open file f has exactly one name, so writing
// into it cannot change a file known by another. os.FileInfo carries no link
// count on Windows, so this asks the handle.
func soleName(f *os.File, _ fs.FileInfo) bool {
	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(windows.Handle(f.Fd()), &info); err != nil {
		return false
	}
	return info.NumberOfLinks == 1
}

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

// removeLockFile does nothing on Windows: the lock file is kept between runs.
// openLockFile shares the file without FILE_SHARE_DELETE, so it cannot be
// removed while the holder, or a run polling for the lock, has it open. Adding
// the flag is not enough on its own. With classic delete semantics the name
// stays, "delete pending", until the last handle closes, and an open in the
// meantime fails with ERROR_ACCESS_DENIED, which Acquire reports as a degraded
// lock, so a run queued behind this one would go on unlocked.
// POSIX-semantics deletion (FILE_DISPOSITION_FLAG_POSIX_SEMANTICS through
// SetFileInformationByHandle) removes the name at once, but only on NTFS from
// Windows 10 1607, and only when every open handle shares delete, which no
// earlier incrmit's does. Elsewhere it falls back to classic semantics. A
// leftover file costs less than an unlocked run. The check after locking
// still runs here, so every platform shares that code path.
func removeLockFile(string) error {
	return nil
}
