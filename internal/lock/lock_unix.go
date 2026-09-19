//go:build !windows

package lock

import (
	"errors"
	"os"
	"syscall"
)

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
