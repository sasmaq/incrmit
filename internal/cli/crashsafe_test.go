package cli

import (
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/sasmaq/incrmit/internal/config"
	"github.com/sasmaq/incrmit/internal/history"
)

// These tests stop a bump or an undo partway through, by failing one of its
// writes or by "crashing" at one, and check that the project ends up as it
// started: rolled back by the run itself when it can, finished by a following
// `undo` when it cannot. Before Milestone 38 a bump wrote its targets one at a
// time and its journal entry last, so a failure on the third of five targets
// left two bumped with no entry for undo to find.

// crashTargets are the files each project here bumps, in the order a run
// writes them. The config is written after them.
var crashTargets = []string{"a/VERSION", "b/VERSION", "c/VERSION"}

// crashProject writes a project listing crashTargets at 1.0.0 and bumps it
// once, to 1.0.1, so its journal already holds an entry when the run under
// test adds one: a rollback must put that entry back as it was, and an undo
// that finishes an interrupted run must stop at it.
func crashProject(t *testing.T) string {
	t.Helper()
	var body strings.Builder
	targets := map[string]string{}
	for _, p := range crashTargets {
		fmt.Fprintf(&body, "[[files]]\npath = %q\nversion = \"1.0.0\"\n", p)
		targets[p] = "version: 1.0.0\n"
	}
	dir := project(t, body.String(), targets)
	if code, _, stderr := runMain(t, dir); code != ExitOK {
		t.Fatalf("setup bump exit = %d, stderr = %q", code, stderr)
	}
	return dir
}

// targetVersions returns the version each of crashTargets holds in dir.
func targetVersions(t *testing.T, dir string) []string {
	t.Helper()
	var out []string
	for _, p := range crashTargets {
		data, err := os.ReadFile(filepath.Join(dir, p))
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, strings.TrimSuffix(strings.TrimPrefix(string(data), "version: "), "\n"))
	}
	return out
}

// assertTargets checks the version each of crashTargets holds in dir.
func assertTargets(t *testing.T, dir string, want ...string) {
	t.Helper()
	if got := targetVersions(t, dir); strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("targets hold %v, want %v", got, want)
	}
}

// assertSnapshot checks that dir is exactly as want recorded it, the journal
// and the config included, byte for byte.
func assertSnapshot(t *testing.T, dir string, want map[string]string, when string) {
	t.Helper()
	got := snapshotProject(t, dir)
	if maps.Equal(got, want) {
		return
	}
	for p, w := range want {
		if g, ok := got[p]; !ok {
			t.Errorf("%s: %s is gone", when, p)
		} else if g != w {
			t.Errorf("%s: %s is\n%s\nwant\n%s", when, p, g, w)
		}
	}
	for p := range got {
		if _, ok := want[p]; !ok {
			t.Errorf("%s: %s was created", when, p)
		}
	}
}

// latestEntry returns the newest entry of dir's journal.
func latestEntry(t *testing.T, dir string) history.Entry {
	t.Helper()
	h, err := history.Load(stateFile(dir))
	if err != nil {
		t.Fatal(err)
	}
	e, ok := h.Latest()
	if !ok {
		t.Fatal("the journal is empty")
	}
	return e
}

// assertPendingBump checks that dir's newest journal entry is the bump from
// 1.0.1 to 1.0.2 of every target, still marked pending.
func assertPendingBump(t *testing.T, dir string) {
	t.Helper()
	e := latestEntry(t, dir)
	if !e.Pending() {
		t.Errorf("newest entry status = %q, want %q", e.Status, history.StatusPending)
	}
	var got []string
	for _, c := range e.Changes {
		got = append(got, fmt.Sprintf("%s %s->%s", c.Path, c.Old, c.New))
	}
	want := []string{"a/VERSION 1.0.1->1.0.2", "b/VERSION 1.0.1->1.0.2", "c/VERSION 1.0.1->1.0.2"}
	if strings.Join(got, ", ") != strings.Join(want, ", ") {
		t.Errorf("newest entry records %v, want %v", got, want)
	}
}

// faultWrites routes writeFile through fault until the test ends or the
// returned function is called. fault sees each write's sequence number in the
// run (from 1) and its path, and returns the error to fail it with, or nil to
// let it through.
func faultWrites(t *testing.T, fault func(seq int, path string) error) (heal func()) {
	t.Helper()
	orig, seq := writeFile, 0
	writeFile = func(path string, data []byte) error {
		seq++
		if err := fault(seq, path); err != nil {
			return err
		}
		return orig(path, data)
	}
	heal = func() { writeFile = orig }
	t.Cleanup(heal)
	return heal
}

// faultJournal does for saveJournal what faultWrites does for writeFile.
func faultJournal(t *testing.T, fault func(seq int) error) (heal func()) {
	t.Helper()
	orig, seq := saveJournal, 0
	saveJournal = func(path string, h *history.History) error {
		seq++
		if err := fault(seq); err != nil {
			return err
		}
		return orig(path, h)
	}
	heal = func() { saveJournal = orig }
	t.Cleanup(heal)
	return heal
}

// denied is the error an injected write fails with, the one a read-only
// directory gives.
func denied(path string) error {
	return &fs.PathError{Op: "open", Path: path, Err: fs.ErrPermission}
}

// isTarget reports whether path, as a run passes it to writeFile, names the
// target p.
func isTarget(path, p string) bool {
	return strings.HasSuffix(filepath.ToSlash(path), p)
}

// crash is what an injected crash panics with. runCrashing recovers exactly
// this value: the panic unwinds the run without its rollback or anything after
// the point it stopped at, as a kill -9 or a power cut would.
type crash struct{}

// runCrashing runs Main like runMain and expects it to crash partway through.
// Deferred calls still run as the panic unwinds, which releases the project
// lock, as the kernel does when a process dies.
func runCrashing(t *testing.T, dir string, args ...string) {
	t.Helper()
	crashed := func() (crashed bool) {
		defer func() {
			if r := recover(); r != nil {
				if _, ok := r.(crash); !ok {
					panic(r)
				}
				crashed = true
			}
		}()
		runMain(t, dir, args...)
		return false
	}()
	if !crashed {
		t.Fatal("the run finished instead of crashing")
	}
}

// A write that fails, as the Nth of the run's writes, puts back every file the
// run already wrote and the journal as it was. The config is the last write,
// so failing it rolls back all three targets. Afterwards the project is
// byte-identical to how it started, and a bump without the fault goes ahead.
func TestBumpWriteFailureRollsBack(t *testing.T) {
	writes := append(append([]string{}, crashTargets...), config.DefaultPath)
	for n, failing := range writes {
		t.Run(failing, func(t *testing.T) {
			dir := crashProject(t)
			start := snapshotProject(t, dir)
			heal := faultWrites(t, func(seq int, path string) error {
				if seq == n+1 {
					return denied(path)
				}
				return nil
			})

			code, stdout, stderr := runMain(t, dir)
			if code != ExitError {
				t.Errorf("exit = %d, want %d (stderr %q)", code, ExitError, stderr)
			}
			if !strings.Contains(stderr, "writing "+failing+": permission denied") {
				t.Errorf("stderr = %q, want the write error for %s", stderr, failing)
			}
			if n > 0 {
				want := "rolled back " + strings.Join(crashTargets[:n], ", ") + "; no file was changed"
				if !strings.Contains(stderr, want) {
					t.Errorf("stderr = %q, want %q", stderr, want)
				}
			}
			if stdout != "" {
				t.Errorf("stdout = %q, want nothing for a failed bump", stdout)
			}
			assertSnapshot(t, dir, start, "after the failed bump")

			heal()
			if code, _, stderr := runMain(t, dir); code != ExitOK {
				t.Fatalf("retried bump exit = %d, stderr = %q", code, stderr)
			}
			assertTargets(t, dir, "1.0.2", "1.0.2", "1.0.2")
			if code, _, stderr := runMain(t, dir, "undo"); code != ExitOK {
				t.Fatalf("undo exit = %d, stderr = %q", code, stderr)
			}
			assertSnapshot(t, dir, start, "after the retried bump and its undo")
		})
	}
}

// A project's first bump creates the state file for its pending entry, so
// rolling that bump back removes the file again rather than leaving an empty
// journal behind.
func TestFirstBumpRollbackLeavesNoJournal(t *testing.T) {
	dir := project(t, "[[files]]\npath = \"a\"\n[[files]]\npath = \"b\"\n", map[string]string{"a": "1.0.0\n", "b": "1.0.0\n"})
	start := snapshotProject(t, dir)
	faultWrites(t, func(_ int, path string) error {
		if isTarget(path, "b") {
			return denied(path)
		}
		return nil
	})

	code, _, stderr := runMain(t, dir)
	if code != ExitError {
		t.Errorf("exit = %d, want %d (stderr %q)", code, ExitError, stderr)
	}
	if !strings.Contains(stderr, "rolled back a; no file was changed") {
		t.Errorf("stderr = %q, want the rollback reported", stderr)
	}
	assertSnapshot(t, dir, start, "after the failed bump")
}

// When the files are all put back but the journal cannot be, the run says so:
// its entry stays pending, and the undo it names clears it without changing a
// file, since every file is at the version the bump started from.
func TestBumpJournalRestoreFailureIsReported(t *testing.T) {
	dir := crashProject(t)
	start := snapshotProject(t, dir)
	healWrites := faultWrites(t, func(_ int, path string) error {
		if isTarget(path, "c/VERSION") {
			return denied(path)
		}
		return nil
	})
	healJournal := faultJournal(t, func(seq int) error {
		if seq == 2 {
			return denied(stateFile(dir))
		}
		return nil
	})

	code, _, stderr := runMain(t, dir)
	if code != ExitError {
		t.Errorf("exit = %d, want %d (stderr %q)", code, ExitError, stderr)
	}
	want := "warning: restoring " + config.StateFileName + ": permission denied; its newest entry stays marked unfinished, which `incrmit undo` resolves"
	if !strings.Contains(stderr, want) {
		t.Errorf("stderr = %q, want %q", stderr, want)
	}
	assertTargets(t, dir, "1.0.1", "1.0.1", "1.0.1")
	assertPendingBump(t, dir)

	healWrites()
	healJournal()
	code, stdout, stderr := runMain(t, dir, "undo")
	if code != ExitOK {
		t.Fatalf("undo exit = %d, stderr = %q", code, stderr)
	}
	if !strings.Contains(stdout, "Undid the unfinished bump in 0 file(s):") {
		t.Errorf("undo stdout = %q, want no file reverted", stdout)
	}
	assertSnapshot(t, dir, start, "after the undo")
}

// The same rollback through a real failure: the third target's directory is
// read-only, so its temp file cannot be created.
func TestBumpReadOnlyTargetDirRollsBack(t *testing.T) {
	if os.Geteuid() == 0 || runtime.GOOS == "windows" {
		t.Skip("directory permissions are not enforced here")
	}
	dir := crashProject(t)
	sub := filepath.Join(dir, "c")
	if err := os.Chmod(sub, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(sub, 0o755) })
	start := snapshotProject(t, dir)

	code, _, stderr := runMain(t, dir)
	if code != ExitError {
		t.Errorf("exit = %d, want %d (stderr %q)", code, ExitError, stderr)
	}
	for _, want := range []string{"writing c/VERSION: permission denied", "rolled back a/VERSION, b/VERSION; no file was changed"} {
		if !strings.Contains(stderr, want) {
			t.Errorf("stderr = %q, want %q", stderr, want)
		}
	}
	assertSnapshot(t, dir, start, "after the failed bump")
}

// A project whose config directory cannot be written fails at the journal,
// which is now written before any target: the targets, in directories of their
// own that can be written, are left alone. The config used to be the first
// write to fail, after every target had been bumped.
func TestBumpUnwritableJournalWritesNothing(t *testing.T) {
	if os.Geteuid() == 0 || runtime.GOOS == "windows" {
		t.Skip("directory permissions are not enforced here")
	}
	dir := crashProject(t)
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
	start := snapshotProject(t, dir)

	code, _, stderr := runMain(t, dir)
	if code != ExitError {
		t.Errorf("exit = %d, want %d (stderr %q)", code, ExitError, stderr)
	}
	if !strings.Contains(stderr, "writing "+config.StateFileName+": permission denied") {
		t.Errorf("stderr = %q, want the journal's write error", stderr)
	}
	assertSnapshot(t, dir, start, "after the failed bump")
}

// When a rollback write fails too, the run cannot put the project back itself.
// It names the file it could not restore and the undo command that will, and
// leaves the pending entry for that undo to find. The undo then returns the
// project to exactly how it started.
func TestBumpRollbackFailureLeavesPendingEntry(t *testing.T) {
	for _, viaFlag := range []bool{false, true} {
		t.Run(fmt.Sprintf("config flag %v", viaFlag), func(t *testing.T) {
			dir := crashProject(t)
			start := snapshotProject(t, dir)
			aWrites := 0
			heal := faultWrites(t, func(_ int, path string) error {
				if isTarget(path, "a/VERSION") {
					aWrites++
				}
				if isTarget(path, "c/VERSION") || (isTarget(path, "a/VERSION") && aWrites == 2) {
					return denied(path)
				}
				return nil
			})

			cwd, args, undo := dir, []string{}, "incrmit undo"
			if viaFlag {
				cfg := filepath.Join(dir, config.DefaultPath)
				cwd, args, undo = "", []string{"-c", cfg}, "incrmit undo -c "+shellWord(cfg)
			}
			code, _, stderr := runMain(t, cwd, args...)
			if code != ExitError {
				t.Errorf("exit = %d, want %d (stderr %q)", code, ExitError, stderr)
			}
			for _, want := range []string{
				"writing c/VERSION: permission denied",
				"rolling back a/VERSION: permission denied",
				"the bump stopped partway, and these files could not be put back:\n  a/VERSION\n",
				"run `" + undo + "` to revert what it wrote",
			} {
				if !strings.Contains(stderr, want) {
					t.Errorf("stderr = %q, want %q", stderr, want)
				}
			}
			if strings.Contains(stderr, "no file was changed") {
				t.Errorf("stderr = %q claims nothing changed", stderr)
			}
			assertTargets(t, dir, "1.0.2", "1.0.1", "1.0.1")
			assertPendingBump(t, dir)

			heal()
			code, stdout, stderr := runMain(t, cwd, append([]string{"undo"}, args...)...)
			if code != ExitOK {
				t.Fatalf("undo exit = %d, stderr = %q", code, stderr)
			}
			want := "Undid the unfinished bump in 1 file(s):\n" +
				"  a/VERSION: 1.0.2 -> 1.0.1\n  b/VERSION: already at 1.0.1\n  c/VERSION: already at 1.0.1\n"
			if stdout != want {
				t.Errorf("undo stdout = %q, want %q", stdout, want)
			}
			assertSnapshot(t, dir, start, "after the undo")
		})
	}
}

// A bump can stop at any of its writes without running another line: a kill
// -9, a power cut. Whatever point it stops at, either nothing has changed or
// the journal holds a pending entry, and `undo` returns the project to exactly
// how it started.
func TestBumpCrashIsUndoable(t *testing.T) {
	type point struct {
		name    string
		write   int // the writeFile call to crash at, or 0
		journal int // the saveJournal call to crash at, or 0
	}
	points := []point{{name: "journal pending", journal: 1}}
	for i, p := range crashTargets {
		points = append(points, point{name: p, write: i + 1})
	}
	points = append(points,
		point{name: config.DefaultPath, write: len(crashTargets) + 1},
		point{name: "journal complete", journal: 2})

	for _, pt := range points {
		t.Run(pt.name, func(t *testing.T) {
			dir := crashProject(t)
			start := snapshotProject(t, dir)
			faultWrites(t, func(seq int, _ string) error {
				if seq == pt.write {
					panic(crash{})
				}
				return nil
			})
			faultJournal(t, func(seq int) error {
				if seq == pt.journal {
					panic(crash{})
				}
				return nil
			})

			runCrashing(t, dir)
			if pt.journal == 1 {
				assertSnapshot(t, dir, start, "after a crash before the journal was written")
				return
			}
			assertPendingBump(t, dir)
			if code, _, stderr := runMain(t, dir, "undo"); code != ExitOK {
				t.Fatalf("undo exit = %d, stderr = %q", code, stderr)
			}
			assertSnapshot(t, dir, start, "after the undo")
		})
	}
}

// A bump that cannot mark its entry complete has still written every file,
// and a pending entry undoes exactly as a complete one does, so it succeeds
// with a warning rather than reporting a bump that happened as a failure.
func TestBumpUnmarkedEntryStillApplies(t *testing.T) {
	dir := crashProject(t)
	start := snapshotProject(t, dir)
	heal := faultJournal(t, func(seq int) error {
		if seq == 2 {
			return denied(stateFile(dir))
		}
		return nil
	})

	code, stdout, stderr := runMain(t, dir)
	if code != ExitOK {
		t.Errorf("exit = %d, want %d (stderr %q)", code, ExitOK, stderr)
	}
	if !strings.Contains(stdout, "Applied a patch bump to 3 file(s)") {
		t.Errorf("stdout = %q, want the bump's summary", stdout)
	}
	if !strings.Contains(stderr, "warning: writing "+config.StateFileName+": permission denied; the bump is applied") {
		t.Errorf("stderr = %q, want a warning that the entry is unmarked", stderr)
	}
	assertTargets(t, dir, "1.0.2", "1.0.2", "1.0.2")
	assertPendingBump(t, dir)

	heal()
	if code, _, stderr := runMain(t, dir, "undo"); code != ExitOK {
		t.Fatalf("undo exit = %d, stderr = %q", code, stderr)
	}
	assertSnapshot(t, dir, start, "after the undo")
}

// A config out of sync with a later target is found while planning. The check
// used to run as each file was written, so the first target was bumped before
// the second was found not to hold its pinned version, and --dry-run, which
// stopped before it, predicted success.
func TestBumpOutOfSyncPinWritesNothing(t *testing.T) {
	body := "[[files]]\npath = \"A\"\nversion = \"1.2.3\"\n[[files]]\npath = \"B\"\nversion = \"9.9.9\"\n"
	dir := project(t, body, map[string]string{"A": "1.2.3\n", "B": "1.2.3\n"})
	start := snapshotProject(t, dir)

	for _, args := range [][]string{nil, {"--dry-run"}} {
		code, _, stderr := runMain(t, dir, args...)
		if code != ExitNoVersion {
			t.Errorf("%v: exit = %d, want %d (stderr %q)", args, code, ExitNoVersion, stderr)
		}
		if !strings.Contains(stderr, "B: files: expected version not found: 9.9.9") {
			t.Errorf("%v: stderr = %q, want it to name B and its pinned version", args, stderr)
		}
	}
	assertSnapshot(t, dir, start, "after the refused bumps")
}

// A bump left half-applied makes the next bump fail, because the config no
// longer matches the files. That failure points at undo when the journal says
// why.
func TestBumpAfterCrashPointsAtUndo(t *testing.T) {
	dir := crashProject(t)
	heal := faultWrites(t, func(seq int, _ string) error {
		if seq == 2 {
			panic(crash{})
		}
		return nil
	})
	runCrashing(t, dir)

	heal()
	code, _, stderr := runMain(t, dir)
	if code != ExitNoVersion {
		t.Errorf("exit = %d, want %d (stderr %q)", code, ExitNoVersion, stderr)
	}
	if !strings.Contains(stderr, "stopped partway; run `incrmit undo` to finish putting it back") {
		t.Errorf("stderr = %q, want it to point at undo", stderr)
	}
}

// bumpedTwice is a crashProject bumped a second time, to 1.0.2, and the
// snapshot taken between the two bumps: what an undo of the second returns
// the project to.
func bumpedTwice(t *testing.T) (string, map[string]string) {
	t.Helper()
	dir := crashProject(t)
	once := snapshotProject(t, dir)
	if code, _, stderr := runMain(t, dir); code != ExitOK {
		t.Fatalf("second bump exit = %d, stderr = %q", code, stderr)
	}
	return dir, once
}

// An undo writes several files too, and gets the same treatment: a write that
// fails puts back what the undo already reverted and marks the entry complete
// again, so the project is as the bump left it and the undo can be retried.
func TestUndoWriteFailureRollsBack(t *testing.T) {
	writes := append(append([]string{}, crashTargets...), config.DefaultPath)
	for n, failing := range writes {
		t.Run(failing, func(t *testing.T) {
			dir, once := bumpedTwice(t)
			twice := snapshotProject(t, dir)
			heal := faultWrites(t, func(seq int, path string) error {
				if seq == n+1 {
					return denied(path)
				}
				return nil
			})

			code, _, stderr := runMain(t, dir, "undo")
			if code != ExitError {
				t.Errorf("exit = %d, want %d (stderr %q)", code, ExitError, stderr)
			}
			if !strings.Contains(stderr, "writing "+failing+": permission denied") {
				t.Errorf("stderr = %q, want the write error for %s", stderr, failing)
			}
			assertSnapshot(t, dir, twice, "after the failed undo")

			heal()
			if code, _, stderr := runMain(t, dir, "undo"); code != ExitOK {
				t.Fatalf("retried undo exit = %d, stderr = %q", code, stderr)
			}
			assertSnapshot(t, dir, once, "after the retried undo")
		})
	}
}

// An undo that cannot mark its entry pending stops before writing anything.
func TestUndoUnmarkableEntryWritesNothing(t *testing.T) {
	dir, _ := bumpedTwice(t)
	twice := snapshotProject(t, dir)
	faultJournal(t, func(int) error { return denied(stateFile(dir)) })

	code, _, stderr := runMain(t, dir, "undo")
	if code != ExitError {
		t.Errorf("exit = %d, want %d (stderr %q)", code, ExitError, stderr)
	}
	if !strings.Contains(stderr, "writing "+config.StateFileName+": permission denied") {
		t.Errorf("stderr = %q, want the journal's write error", stderr)
	}
	assertSnapshot(t, dir, twice, "after the failed undo")
}

// An undo that cannot pop its entry has still reverted every file and the
// config, so it succeeds with a warning. The entry it leaves is pending, and
// the next undo clears it without writing a file.
func TestUndoUnpoppedEntryStillApplies(t *testing.T) {
	dir, once := bumpedTwice(t)
	heal := faultJournal(t, func(seq int) error {
		if seq == 2 {
			return denied(stateFile(dir))
		}
		return nil
	})

	code, stdout, stderr := runMain(t, dir, "undo")
	if code != ExitOK {
		t.Errorf("exit = %d, want %d (stderr %q)", code, ExitOK, stderr)
	}
	if !strings.Contains(stdout, "Undid the most recent bump in 3 file(s):") {
		t.Errorf("stdout = %q, want the undo's summary", stdout)
	}
	if !strings.Contains(stderr, "the undo is applied, but its journal entry stays, marked unfinished, until `incrmit undo` clears it") {
		t.Errorf("stderr = %q, want a warning that the entry stays", stderr)
	}
	assertTargets(t, dir, "1.0.1", "1.0.1", "1.0.1")
	assertPendingBump(t, dir)

	heal()
	before := snapshotProject(t, dir)
	code, stdout, stderr = runMain(t, dir, "undo")
	if code != ExitOK {
		t.Fatalf("second undo exit = %d, stderr = %q", code, stderr)
	}
	if !strings.Contains(stdout, "Undid the unfinished bump in 0 file(s):") {
		t.Errorf("second undo stdout = %q, want no file reverted", stdout)
	}
	assertSnapshot(t, dir, once, "after the second undo")
	for path, entry := range before {
		if filepath.Base(path) != config.StateFileName && once[path] != entry {
			t.Errorf("the second undo changed %s", path)
		}
	}
}

// An undo whose rollback fails leaves the entry pending and says to run undo
// again, which finishes the job from the files it finds.
func TestUndoRollbackFailureCanBeFinished(t *testing.T) {
	dir, once := bumpedTwice(t)
	aWrites := 0
	heal := faultWrites(t, func(_ int, path string) error {
		if isTarget(path, "a/VERSION") {
			aWrites++
		}
		if isTarget(path, "c/VERSION") || (isTarget(path, "a/VERSION") && aWrites == 2) {
			return denied(path)
		}
		return nil
	})

	code, _, stderr := runMain(t, dir, "undo")
	if code != ExitError {
		t.Errorf("exit = %d, want %d (stderr %q)", code, ExitError, stderr)
	}
	for _, want := range []string{
		"the undo stopped partway, and these files could not be put back:\n  a/VERSION\n",
		"run `incrmit undo` to finish the undo",
	} {
		if !strings.Contains(stderr, want) {
			t.Errorf("stderr = %q, want %q", stderr, want)
		}
	}
	assertTargets(t, dir, "1.0.1", "1.0.2", "1.0.2")
	assertPendingBump(t, dir)

	heal()
	code, stdout, stderr := runMain(t, dir, "undo")
	if code != ExitOK {
		t.Fatalf("undo exit = %d, stderr = %q", code, stderr)
	}
	want := "Undid the unfinished bump in 2 file(s):\n" +
		"  a/VERSION: already at 1.0.1\n  b/VERSION: 1.0.2 -> 1.0.1\n  c/VERSION: 1.0.2 -> 1.0.1\n"
	if stdout != want {
		t.Errorf("undo stdout = %q, want %q", stdout, want)
	}
	assertSnapshot(t, dir, once, "after the finished undo")
}

// An undo can stop at any of its writes as well. Run again, it finishes, from
// whichever files the first one reached.
func TestUndoCrashCanBeRunAgain(t *testing.T) {
	type point struct {
		name    string
		write   int
		journal int
	}
	points := []point{{name: "journal pending", journal: 1}}
	for i, p := range crashTargets {
		points = append(points, point{name: p, write: i + 1})
	}
	points = append(points,
		point{name: config.DefaultPath, write: len(crashTargets) + 1},
		point{name: "journal pop", journal: 2})

	for _, pt := range points {
		t.Run(pt.name, func(t *testing.T) {
			dir, once := bumpedTwice(t)
			healWrites := faultWrites(t, func(seq int, _ string) error {
				if seq == pt.write {
					panic(crash{})
				}
				return nil
			})
			healJournal := faultJournal(t, func(seq int) error {
				if seq == pt.journal {
					panic(crash{})
				}
				return nil
			})
			runCrashing(t, dir, "undo")

			healWrites()
			healJournal()
			if code, _, stderr := runMain(t, dir, "undo"); code != ExitOK {
				t.Fatalf("second undo exit = %d, stderr = %q", code, stderr)
			}
			assertSnapshot(t, dir, once, "after the second undo")
		})
	}
}

// pendingProject writes a project whose bump from 1.0.0 to 1.0.1 stopped
// partway: the config is still at 1.0.0, a hand-written one with a comment the
// bump's rewrite would drop, the targets hold what the caller gives, and the
// journal records the bump as pending.
func pendingProject(t *testing.T, a, b string) string {
	t.Helper()
	cfg := "# hand-written\n[[files]]\npath = \"a\"\nversion = \"1.0.0\"\n[[files]]\npath = \"b\"\nversion = \"1.0.0\"\n"
	dir := project(t, cfg, map[string]string{"a": a, "b": b})
	writeJournal(t, dir, "[[entries]]\ntimestamp = 2026-10-17T00:00:00Z\nstatus = \"pending\"\n\n"+
		"[[entries.changes]]\npath = \"a\"\nold = \"1.0.0\"\nnew = \"1.0.1\"\n\n"+
		"[[entries.changes]]\npath = \"b\"\nold = \"1.0.0\"\nnew = \"1.0.1\"\n")
	return dir
}

// Undo of a pending entry reverts the files the bump wrote and leaves the ones
// it never reached, instead of refusing them as edited, and leaves a config
// the bump never rewrote exactly as it is.
func TestUndoPendingEntryToleratesUnwrittenFiles(t *testing.T) {
	dir := pendingProject(t, "1.0.1\n", "1.0.0\n")
	cfgBefore, err := os.ReadFile(filepath.Join(dir, config.DefaultPath))
	if err != nil {
		t.Fatal(err)
	}

	code, stdout, stderr := runMain(t, dir, "undo", "--dry-run")
	if code != ExitOK {
		t.Fatalf("dry run exit = %d, stderr = %q", code, stderr)
	}
	want := "Dry run: would undo the unfinished bump (no files changed)\n  a: 1.0.1 -> 1.0.0\n  b: already at 1.0.0\n"
	if stdout != want {
		t.Errorf("dry run stdout = %q, want %q", stdout, want)
	}

	code, stdout, stderr = runMain(t, dir, "undo")
	if code != ExitOK {
		t.Fatalf("undo exit = %d, stderr = %q", code, stderr)
	}
	want = "Undid the unfinished bump in 1 file(s):\n  a: 1.0.1 -> 1.0.0\n  b: already at 1.0.0\n"
	if stdout != want {
		t.Errorf("undo stdout = %q, want %q", stdout, want)
	}
	for name, body := range map[string]string{"a": "1.0.0\n", "b": "1.0.0\n", config.DefaultPath: string(cfgBefore)} {
		if got, _ := os.ReadFile(filepath.Join(dir, name)); string(got) != body {
			t.Errorf("%s = %q, want %q", name, got, body)
		}
	}
	h, err := history.Load(stateFile(dir))
	if err != nil {
		t.Fatal(err)
	}
	if len(h.Entries) != 0 {
		t.Errorf("journal has %d entries, want the pending one popped", len(h.Entries))
	}
}

// Tolerating unwritten files does not mean tolerating edited ones. A file
// that holds neither side of its change was changed since, and one that holds
// both cannot be told apart from a file that already held the new version, so
// either refuses the undo with nothing written.
func TestUndoPendingEntryRefusesEditedFiles(t *testing.T) {
	tests := []struct {
		name, b, want string
	}{
		{"neither", "2.0.0\n", "b: holds neither 1.0.1, which the unfinished bump wrote, nor 1.0.0, which it started from"},
		{"both", "1.0.0 then 1.0.1\n", "b: holds both 1.0.1, which the unfinished bump wrote, and 1.0.0, which it started from"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := pendingProject(t, "1.0.1\n", tt.b)
			start := snapshotProject(t, dir)
			code, _, stderr := runMain(t, dir, "undo")
			if code != ExitError {
				t.Errorf("exit = %d, want %d (stderr %q)", code, ExitError, stderr)
			}
			if !strings.Contains(stderr, tt.want) {
				t.Errorf("stderr = %q, want %q", stderr, tt.want)
			}
			assertSnapshot(t, dir, start, "after the refused undo")
		})
	}
}

// The config check widens for a pending entry only as far as the bump could
// have left it: every change listed at its new version or every one at its
// old. A config at neither, or split between the two, has been edited.
func TestUndoPendingEntryChecksTheConfig(t *testing.T) {
	tests := []struct {
		name, cfg, want string
	}{
		{"neither", "[[files]]\npath = \"a\"\nversion = \"1.0.0\"\n[[files]]\npath = \"b\"\nversion = \"7.0.0\"\n",
			"b: not listed in incrmit.toml at 1.0.1 or 1.0.0, the versions the unfinished bump recorded"},
		{"split", "[[files]]\npath = \"a\"\nversion = \"1.0.1\"\n[[files]]\npath = \"b\"\nversion = \"1.0.0\"\n",
			"incrmit.toml lists some of the unfinished bump's files at the versions it wrote and others at the ones it started from"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := pendingProject(t, "1.0.1\n", "1.0.0\n")
			if err := os.WriteFile(filepath.Join(dir, config.DefaultPath), []byte(tt.cfg), 0o644); err != nil {
				t.Fatal(err)
			}
			start := snapshotProject(t, dir)
			code, _, stderr := runMain(t, dir, "undo")
			if code != ExitError {
				t.Errorf("exit = %d, want %d (stderr %q)", code, ExitError, stderr)
			}
			if !strings.Contains(stderr, tt.want) {
				t.Errorf("stderr = %q, want %q", stderr, tt.want)
			}
			assertSnapshot(t, dir, start, "after the refused undo")
		})
	}
}

// The undo command in the recovery advice is pasted into a shell, so a config
// path with a space or a quote in it must survive that.
func TestShellWord(t *testing.T) {
	tests := []struct{ in, want string }{
		{"incrmit.toml", "incrmit.toml"},
		{"release/incrmit.toml", "release/incrmit.toml"},
		{"/tmp/x-1.2/a_b+c@d:e,f=g%h", "/tmp/x-1.2/a_b+c@d:e,f=g%h"},
		{"My Project/incrmit.toml", "'My Project/incrmit.toml'"},
		{"it's/incrmit.toml", `'it'\''s/incrmit.toml'`},
		{"$HOME/incrmit.toml", "'$HOME/incrmit.toml'"},
		{"", "''"},
		{"tab\there", `"tab\there"`},
	}
	for _, tt := range tests {
		if got := shellWord(tt.in); got != tt.want {
			t.Errorf("shellWord(%q) = %s, want %s", tt.in, got, tt.want)
		}
	}
}
