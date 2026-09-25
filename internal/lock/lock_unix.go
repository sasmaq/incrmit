//go:build !windows

package lock

import (
	"errors"
	"io/fs"
	"os"
	"syscall"
)

// openLockFile opens the lock file at path for reading and writing, creating it
// when nothing is there, without following a symbolic link: O_NOFOLLOW makes
// the open fail on a link instead of opening what it names, so the check and
// the open are one step with no window for the link to be swapped in between.
// O_CREATE without O_TRUNC, because the lock is not held yet and the contents
// are not ours to discard. O_NONBLOCK keeps the open itself from waiting on a
// FIFO or a device; it has no effect on a regular file, the only kind Acquire
// goes on to use.
func openLockFile(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_CREATE|os.O_RDWR|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0o600)
}

// soleName reports whether the open file f, described by info, has exactly one
// name, so writing into it cannot change a file known by another.
func soleName(_ *os.File, info fs.FileInfo) bool {
	st, ok := info.Sys().(*syscall.Stat_t)
	return ok && st.Nlink == 1
}

// tryLock takes an exclusive advisory lock on f without blocking.
//
// flock locks are held by the open file description rather than by the
// process, so two descriptors opened separately contend even inside one
// process. That is what lets the concurrency tests drive real contention
// without spawning children, and it is also why Release closes the file: the
// lock lives with the descriptor.
func tryLock(f *os.File) error {
	err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	switch {
	case err == nil:
		return nil
	case errors.Is(err, syscall.EWOULDBLOCK):
		// EWOULDBLOCK (== EAGAIN) is the one answer that means "someone else
		// holds it". Everything else — EOPNOTSUPP or ENOLCK from a filesystem
		// that does not implement flock, say — is reported as itself so the
		// caller can degrade rather than refuse to run.
		return ErrContended
	default:
		return err
	}
}

// unlock drops the lock held on f. Closing the descriptor would release it
// anyway; doing it explicitly keeps the release visible at the point it
// happens and surfaces any error.
func unlock(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
}
