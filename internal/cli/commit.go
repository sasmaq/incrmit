package cli

import (
	"io"
	"slices"
	"strings"

	"github.com/sasmaq/incrmit/internal/files"
	"github.com/sasmaq/incrmit/internal/history"
)

// A bump or an undo rewrites several files. Each write is atomic on its own
// (files.WriteAtomic), but no filesystem offers a transaction over several
// renames, so the set is made safe another way. Every byte a run writes is
// computed before its first write, and the bytes each file held before are
// kept. The journal marks the run's entry pending before anything else is
// touched. A write that fails puts back the files already written. A run that
// cannot put one back, or that dies outright, leaves the pending entry, and
// `undo` can finish it whatever mix of written and unwritten files it finds.
// See doc/DEVELOPMENT.md section 8.7.

// writeFile and saveJournal are the writes a bump or an undo makes. They are
// variables so a test can fail or interrupt one partway through a run
// (crashsafe_test.go). Nothing else assigns them.
var (
	writeFile   = files.WriteAtomic
	saveJournal = history.Save
)

// fileWrite is one file a run rewrites: the bytes it held when the run read
// it, which a rollback puts back, and the bytes the run writes.
type fileWrite struct {
	display string // path as the user/config sees it
	fsPath  string // resolved filesystem path
	before  []byte
	after   []byte
}

// writeRun describes the run commitWrites is carrying out, for the advice it
// prints when it cannot put everything back.
type writeRun struct {
	what    string // "bump" or "undo"
	journal string // the state file restore writes
	undo    string // the undo command for this project, ready to paste
	then    string // what running that command does from here
}

// commitWrites writes ws in order and returns ExitOK once every write has
// landed.
//
// The first write that fails stops the run. The files already written get
// their before bytes back, newest first, and every rollback is attempted even
// after one fails, so as little as possible is left changed. When all of them
// are put back, restore (nil when there is nothing to restore) puts the journal
// back too, and the run has changed nothing. When one cannot be put back, the
// pending entry stays, and the advice names each file still changed and the
// undo command that finishes the job.
//
// A run lists its config last, so the config never needs rolling back: either
// its write failed, which changed nothing, or every write has landed.
func commitWrites(ws []fileWrite, restore func() error, run writeRun, stderr io.Writer) int {
	n := 0
	var err error
	for ; n < len(ws); n++ {
		if err = writeFile(ws[n].fsPath, ws[n].after); err != nil {
			break
		}
	}
	if err == nil {
		return ExitOK
	}
	fprintf(stderr, "incrmit: %s\n", fsErrorMessage("writing", ws[n].display, err))

	var stuck, back []string
	for i := n - 1; i >= 0; i-- {
		w := ws[i]
		if rerr := writeFile(w.fsPath, w.before); rerr != nil {
			fprintf(stderr, "incrmit: %s\n", fsErrorMessage("rolling back", w.display, rerr))
			stuck = append(stuck, w.display)
			continue
		}
		back = append(back, w.display)
	}
	slices.Reverse(stuck)
	slices.Reverse(back)

	if len(stuck) > 0 {
		fprintf(stderr, "incrmit: the %s stopped partway, and these files could not be put back:\n", run.what)
		for _, s := range stuck {
			fprintf(stderr, "  %s\n", displayName(s))
		}
		fprintf(stderr, "The journal records the %s as unfinished. Once the problem above is fixed, run `%s` to %s.\n", run.what, run.undo, run.then)
		return classify(err)
	}

	if len(back) > 0 {
		fprintf(stderr, "incrmit: rolled back %s; no file was changed\n", strings.Join(displayNames(back), ", "))
	}
	if restore != nil {
		if rerr := restore(); rerr != nil {
			fprintf(stderr, "incrmit: warning: %s; its newest entry stays marked unfinished, which `%s` resolves\n",
				fsErrorMessage("restoring", run.journal, rerr), run.undo)
		}
	}
	return classify(err)
}

// undoCommand is the undo command for the config a run was given, ready to
// paste into a shell in the same directory: plain `incrmit undo` for the
// default config, with -c otherwise.
func undoCommand(configPath string) string {
	if configPath == "" {
		return "incrmit undo"
	}
	return "incrmit undo -c " + shellWord(configPath)
}

// shellWord renders s as one word of a POSIX shell command line: unchanged
// when it holds nothing a shell treats specially, single-quoted when it is
// printable, and as displayName renders it otherwise, which a terminal shows
// safely even though a shell would not read it back.
func shellWord(s string) string {
	switch {
	case s != "" && !strings.ContainsFunc(s, func(r rune) bool { return !shellPlain(r) }):
		return s
	case isPrintable(s):
		return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
	default:
		return displayName(s)
	}
}

// shellPlain reports whether r stands for itself in a POSIX shell word.
func shellPlain(r rune) bool {
	return r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune("_-./,:@%+=", r)
}
