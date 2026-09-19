// Package lock serializes incrmit's mutating commands across processes.
//
// incrmit starts no goroutines, so nothing here guards against an internal
// race: it guards against a second incrmit. Every mutating command is a
// read-modify-write over shared on-disk state — `bump` reads the config and the
// targets, then rewrites both plus the journal; `undo` reads the journal and
// rewrites it — and two of them running at once would interleave those steps
// and silently erase one another's work. files.WriteAtomic makes each
// individual write all-or-nothing, which means the loss is never visible as a
// corrupt file: it is only ever a missing bump.
//
// The lock is one exclusive advisory lock (flock on Unix, LockFileEx on
// Windows) on a file kept next to the config, so it is scoped to one project
// and separate projects never contend. An OS advisory lock is used rather than
// an O_EXCL PID file specifically because of stale locks: the kernel drops the
// lock when the holder exits for any reason, SIGKILL and panics included, so
// there is never a leftover lock to clear by hand.
//
// Acquisition fails only on contention. Any other failure — a filesystem with
// no lock support, a directory that cannot be written — degrades to running
// unlocked (see Lock.Degraded), because a tool that cannot bump at all is worse
// than one that cannot detect a second run.
package lock

import (
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/sasmaq/incrmit/internal/config"
)

// ErrContended reports that another incrmit run holds the project lock. It is
// the only error Acquire returns.
var ErrContended = errors.New("lock: the project is locked by another incrmit run")

// note is written into the lock file once the lock is held, so someone who
// finds the file in their tree can tell what it is. It deliberately contains no
// version-like token: a stray lock file must never read as a bump target.
const note = "incrmit project lock (maintained by incrmit; safe to delete).\n" +
	"Held only while a bump, undo, or discover is writing in this directory.\n"

// pollInterval is how often AcquireWait retries a contended lock. A bump takes
// milliseconds, so a short interval keeps queued runs responsive without
// spinning.
const pollInterval = 20 * time.Millisecond

// Lock is a held project lock, or — when Degraded reports true — a record that
// the project is running unlocked because the lock could not be taken for a
// reason that was not contention. Release is safe to call on either, and on a
// nil *Lock, so callers can defer it unconditionally.
type Lock struct {
	path   string
	f      *os.File
	reason error // non-nil when the lock could not be taken; see Degraded
}

// Path returns the lock file's path, whether or not the lock was taken.
func (l *Lock) Path() string {
	if l == nil {
		return ""
	}
	return l.path
}

// Degraded reports whether the caller is proceeding without the lock. Reason
// says why.
func (l *Lock) Degraded() bool { return l != nil && l.reason != nil }

// Reason returns why the lock could not be taken, or nil when it was.
func (l *Lock) Reason() error {
	if l == nil {
		return nil
	}
	return l.reason
}

// Release drops the lock. The lock file itself is left behind: unlinking it
// would let a second run create and lock a fresh file at the same name while
// this one still holds a lock on the old inode, which is exactly the race the
// lock exists to prevent. The file is empty apart from a note and costs
// nothing to keep.
func (l *Lock) Release() error {
	if l == nil || l.f == nil {
		return nil
	}
	f := l.f
	l.f = nil
	err := unlock(f)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	return err
}

// Acquire takes the project lock for the directory dir without waiting. It
// returns ErrContended when another run holds it, and otherwise always returns
// a usable *Lock — one that reports Degraded when locking was unavailable.
func Acquire(dir string) (*Lock, error) {
	path := filepath.Join(dir, config.LockFileName)

	// O_CREATE without O_TRUNC: the file's contents are not state, and
	// truncating before the lock is held would write into a file another run
	// is using.
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return &Lock{path: path, reason: err}, nil
	}

	switch err := tryLock(f); {
	case err == nil:
		writeNote(f)
		return &Lock{path: path, f: f}, nil
	case errors.Is(err, ErrContended):
		_ = f.Close()
		return nil, ErrContended
	default:
		_ = f.Close()
		return &Lock{path: path, reason: err}, nil
	}
}

// AcquireWait is Acquire, but waits for a contended lock instead of failing.
// A timeout of zero or less waits indefinitely.
func AcquireWait(dir string, timeout time.Duration) (*Lock, error) {
	deadline := time.Now().Add(timeout)
	for {
		l, err := Acquire(dir)
		if !errors.Is(err, ErrContended) {
			return l, err
		}
		if timeout > 0 && !time.Now().Before(deadline) {
			return nil, ErrContended
		}
		time.Sleep(pollInterval)
	}
}

// writeNote stamps the explanatory note into the held lock file. It is
// best-effort: the note is a courtesy to whoever finds the file, and failing to
// write it is no reason to refuse a bump.
func writeNote(f *os.File) {
	if err := f.Truncate(0); err != nil {
		return
	}
	_, _ = f.WriteAt([]byte(note), 0)
}
