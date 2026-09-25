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
// no lock support, a directory that cannot be written, something other than a
// regular file at the lock path — degrades to running unlocked (see
// Lock.Degraded), because a tool that cannot bump at all is worse than one that
// cannot detect a second run.
//
// The lock file is the one path incrmit opens for writing instead of replacing
// by rename, so it is opened without following a symbolic link and used only
// when it is a regular file. A repository can commit .incrmit.lock as a link,
// and following one would write the lock note into whatever file it names.
package lock

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/sasmaq/incrmit/internal/config"
)

// ErrContended reports that another incrmit run holds the project lock. It is
// the only error Acquire returns.
var ErrContended = errors.New("lock: the project is locked by another incrmit run")

// NotRegularError is the reason a lock is degraded when something other than a
// regular file sits at the lock path: a symbolic link, a directory, a named
// pipe. Acquire opens nothing through such a path and leaves it as it is, so
// the warning built from this error is what points the user at it. It arrives
// wrapped in an *fs.PathError naming the path.
type NotRegularError struct {
	Mode fs.FileMode // the type bits of what sits at the path
}

func (e *NotRegularError) Error() string {
	return "is " + describeType(e.Mode) + ", not a regular file"
}

// describeType names the kind of file a mode's type bits describe.
func describeType(m fs.FileMode) string {
	switch {
	case m&fs.ModeSymlink != 0:
		return "a symbolic link"
	case m.IsDir():
		return "a directory"
	case m&fs.ModeNamedPipe != 0:
		return "a named pipe"
	case m&fs.ModeSocket != 0:
		return "a socket"
	case m&fs.ModeDevice != 0:
		return "a device"
	default:
		return "a special file"
	}
}

// notRegular is the degraded reason for the lock path holding mode's kind of
// file.
func notRegular(path string, mode fs.FileMode) error {
	return &fs.PathError{Op: "lock", Path: path, Err: &NotRegularError{Mode: mode.Type()}}
}

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

	f, err := openLockFile(path)
	if err != nil {
		// The open refuses a link rather than following it, and a directory
		// cannot be opened for writing; say which it was. This Lstat only words
		// the warning — nothing is opened after it, so it has nothing to race.
		if info, lerr := os.Lstat(path); lerr == nil && !info.Mode().IsRegular() {
			err = notRegular(path, info.Mode())
		}
		return &Lock{path: path, reason: err}, nil
	}
	// Check what was actually opened, through the descriptor: a FIFO or a
	// device opens without complaint, and is no more a lock file than a link.
	info, err := f.Stat()
	if err == nil && !info.Mode().IsRegular() {
		err = notRegular(path, info.Mode())
	}
	if err != nil {
		_ = f.Close()
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

// writeNote stamps the explanatory note into the held lock file, but only into
// one that is empty and has no other name — in practice, one this run or an
// earlier one just created. A regular file that happens to sit at the lock path
// holds whatever someone put there, and a hard link shares its contents with
// another name, so neither is ever written: the note is a courtesy and must
// never be the reason a file loses data. It is best-effort for the same reason,
// and failing to write it is no reason to refuse a bump.
func writeNote(f *os.File) {
	info, err := f.Stat()
	if err != nil || info.Size() != 0 || !soleName(f, info) {
		return
	}
	_, _ = f.WriteAt([]byte(note), 0)
}
