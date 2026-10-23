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
// The lock file exists only while a run holds it. Removing a lock file's name
// is a race of its own: a run that opened the file just before the name went
// would lock the nameless file, a run arriving after would create and lock a
// fresh one, and both would proceed. Two rules close it. Only the holder
// removes the name, and it does so before it unlocks. A run that takes a lock
// keeps it only if the name still leads to the file it locked, and otherwise
// drops it and starts over. The holder is then the only one who can remove the
// name, so once a run's check passes, every newcomer opens the same file and
// contends. A crash skips the release and leaves the file, which the next run
// to hold the lock removes. On Windows the file is kept (see removeLockFile).
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
	"slices"
	"time"

	"github.com/sasmaq/incrmit/internal/config"
)

// ErrContended reports that another incrmit run holds the project lock. It is
// the only error Acquire returns.
var ErrContended = errors.New("lock: the project is locked by another incrmit run")

// errMoved reports that a lock was taken on a file the lock path no longer led
// to: a holder removed the name after this run opened the file. Acquire starts
// over rather than return it.
var errMoved = errors.New("lock: the lock file was removed after it was opened")

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
	"Held while a bump, undo, or discover is writing in this directory. The run\n" +
	"removes it when it finishes, except on Windows, and the next run removes\n" +
	"one left by a run that crashed.\n"

// notes are what a lock file incrmit made can hold besides nothing: the note,
// and the one written before the file was removed between runs, so the file an
// earlier incrmit left in every project is removed like one left by a crash.
var notes = []string{
	note,
	"incrmit project lock (maintained by incrmit; safe to delete).\n" +
		"Held only while a bump, undo, or discover is writing in this directory.\n",
}

// pollInterval is how often AcquireWait retries a contended lock. A bump takes
// milliseconds, so a short interval keeps queued runs responsive without
// spinning.
const pollInterval = 20 * time.Millisecond

// maxOpens caps how many times one Acquire opens the lock path. Each open after
// the first follows a lock taken on a file the name no longer led to, which
// means another run took the project and released it in the moment between
// this run's open and its lock. Twice in a row is already rare, but a name that
// kept moving would otherwise spin. Past the cap, other runs keep getting there
// first, which is contention, so Acquire reports ErrContended (still the only
// error it reports) and AcquireWait polls again.
const maxOpens = 4

// Test seams: Acquire calls afterOpen once the lock file is open and afterLock
// once the lock on it is taken, before it checks the name; Release calls
// afterRemove between removing the name and unlocking. The tests swap them to
// stop a run between those steps and let another one in.
var (
	afterOpen   = func() {}
	afterLock   = func() {}
	afterRemove = func() {}
)

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

// Release removes the lock file, then drops the lock. The name goes while the
// lock is still held, which is what keeps the removal from reopening the race
// it would otherwise cause: removed after the unlock, a run could lock the file
// in between, pass its check because the name still led there, and then lose
// the name to a newcomer that created and locked a fresh file, leaving two
// runs holding the project. The file is removed only when the name still leads
// to it and incrmit made it (see madeByIncrmit); anything else is unlocked and
// left as it was. Removal is best-effort, since a file left behind costs only
// its name, so only the unlock and the close are reported.
func (l *Lock) Release() error {
	if l == nil || l.f == nil {
		return nil
	}
	f := l.f
	l.f = nil
	if info, err := f.Stat(); err == nil && madeByIncrmit(f, info) {
		if same, err := named(l.path, info); err == nil && same {
			_ = removeLockFile(l.path)
		}
	}
	afterRemove()
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
	for range maxOpens {
		l, err := acquireOnce(path)
		if !errors.Is(err, errMoved) {
			return l, err
		}
	}
	return nil, ErrContended
}

// acquireOnce is one attempt of Acquire: it opens the lock path, locks what it
// opened, and keeps the lock only if the name still leads there. It returns
// errMoved when it did not, for Acquire to open the path again.
func acquireOnce(path string) (*Lock, error) {
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
	afterOpen()
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
	case errors.Is(err, ErrContended):
		_ = f.Close()
		return nil, ErrContended
	default:
		_ = f.Close()
		return &Lock{path: path, reason: err}, nil
	}
	afterLock()

	// The lock counts only if the name still leads to the file it is on. A
	// missing name or a different file means a holder removed the name after
	// this run opened it, so the lock is on a file no newcomer will ever open.
	// The check has to come after the lock: until this run holds it, the holder
	// can still remove the name between the check and the lock. Once it passes,
	// only this run can remove the name.
	same, err := named(path, info)
	if err != nil || !same {
		_ = unlock(f)
		_ = f.Close()
		if err != nil {
			return &Lock{path: path, reason: err}, nil
		}
		return nil, errMoved
	}
	writeNote(f)
	return &Lock{path: path, f: f}, nil
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

// named reports whether path still leads to the file info describes, which was
// opened through it. A missing name is an answer rather than an error. It uses
// Lstat, so a link swapped in at the name never matches, even one to this very
// file, and the next open refuses it.
func named(path string, info fs.FileInfo) (bool, error) {
	cur, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return os.SameFile(info, cur), nil
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

// madeByIncrmit reports whether the held lock file f, described by info, is one
// incrmit made, and so one Release may remove: it has one name and holds a note
// or nothing. That is writeNote's test plus a check of the contents, and it
// keeps the file Milestone 32 decided to use but never rewrite: a regular file
// that happened to sit at the lock path holds someone's text, and a hard link
// is known by another name. A file left by a crashed run holds the note, so
// the next run removes it.
func madeByIncrmit(f *os.File, info fs.FileInfo) bool {
	// Every note is a few hundred bytes, so a larger file is none of them and is
	// never read.
	if !soleName(f, info) || info.Size() > 1<<10 {
		return false
	}
	buf := make([]byte, info.Size())
	n, _ := f.ReadAt(buf, 0)
	return n == len(buf) && (n == 0 || slices.Contains(notes, string(buf)))
}
