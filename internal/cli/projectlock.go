package cli

import (
	"errors"
	"io"
	"path/filepath"

	"github.com/sasmaq/incrmit/internal/config"
	"github.com/sasmaq/incrmit/internal/files"
	"github.com/sasmaq/incrmit/internal/lock"
)

// This file holds the CLI half of the one-writer-per-project rule (see package
// lock for the mechanism). Every command that writes takes the lock before it
// reads anything it will later write back, and holds it until it returns; the
// read-only commands (preview, every --dry-run) never take it, so inspecting a
// project can neither block nor be blocked.

// projectDir returns the directory a command's lock belongs in. In config mode
// that is the directory holding the config, so two runs over one project
// contend while runs over separate projects never do. In --file mode there is
// no config to anchor to, so the lock goes beside the file being bumped, which
// is what two concurrent --file runs on the same target have in common.
func projectDir(configPath, file string) string {
	if file != "" {
		return filepath.Dir(file)
	}
	return filepath.Dir(config.ResolvePath(configPath))
}

// acquireProject takes the project lock in dir and reports an exit code:
// ExitOK when the caller may proceed (holding the lock, or — with a warning —
// without it), ExitError when another run holds the project.
//
// Failing fast is the default because a bump takes milliseconds: a second one
// arriving mid-run is far more often a mistake (a `make -j` rule that bumps
// twice, two CI steps racing) than a queue, and a tool that blocks silently
// turns that misconfiguration into a hung job instead of a failed one. Callers
// that really are serializing work opt into waiting with --wait.
func acquireProject(dir string, wait bool, stderr io.Writer) (*lock.Lock, int) {
	var (
		lk  *lock.Lock
		err error
	)
	if wait {
		lk, err = lock.AcquireWait(dir, 0)
	} else {
		lk, err = lock.Acquire(dir)
	}

	if errors.Is(err, lock.ErrContended) {
		fprintf(stderr, "incrmit: another incrmit run is already writing in %s\n", dir)
		fprintln(stderr, "Wait for it to finish and run again, or pass --wait to queue behind it.")
		return nil, ExitError
	}
	if err != nil {
		// Unreachable today: contention is the only error lock reports.
		fprintln(stderr, "incrmit:", err)
		return nil, ExitError
	}

	// Locking was unavailable rather than contended — an NFS mount or a CI
	// overlay filesystem that does not implement it, a directory that cannot be
	// written. Warn and carry on: a tool that cannot bump at all is worse than
	// one that cannot detect a second run.
	if lk.Degraded() {
		fprintf(stderr, "incrmit: warning: cannot lock %s: %v\n", lk.Path(), lk.Reason())
		fprintln(stderr, "incrmit: warning: continuing unlocked; a concurrent incrmit run could erase this one's work.")
	}
	return lk, ExitOK
}

// sweepTemps removes leftover WriteAtomic temp files from dirs, ignoring
// duplicates and failures. It is called only while lk is genuinely held, which
// is the one moment a temp file is provably stale: no other run can have a
// write in flight, so anything matching the pattern is the residue of a run
// that died before its rename. A degraded (or absent) lock buys no such
// guarantee, so nothing is swept.
func sweepTemps(lk *lock.Lock, dirs ...string) {
	if lk == nil || lk.Degraded() {
		return
	}
	seen := make(map[string]struct{}, len(dirs))
	for _, dir := range dirs {
		if _, dup := seen[dir]; dup {
			continue
		}
		seen[dir] = struct{}{}
		_, _ = files.SweepTemps(dir)
	}
}

// dirsOf returns the directories holding paths, in first-seen order, for
// sweepTemps: a temp file only ever appears beside the file being written.
func dirsOf(paths ...string) []string {
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		if p == "" {
			continue
		}
		out = append(out, filepath.Dir(p))
	}
	return out
}
