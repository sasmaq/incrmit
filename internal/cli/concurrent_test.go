package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/sasmaq/incrmit/internal/config"
	"github.com/sasmaq/incrmit/internal/history"
	"github.com/sasmaq/incrmit/internal/lock"
)

// These tests drive two or more runs against one project at the same time.
// Goroutines calling Main stand in for separate processes, which is fair here:
// all the contended state is on disk, and an flock (like a Windows byte-range
// lock) is held by the open file description rather than by the process, so two
// runs inside one test binary contend exactly as two shells would. None of them
// chdir, so the project is reached through an absolute --config path.

// runConcurrent starts n copies of Main(args...) at once and returns their exit
// codes and their stderr output, indexed alike.
func runConcurrent(t *testing.T, n int, args ...string) ([]int, []string) {
	t.Helper()
	codes := make([]int, n)
	errs := make([]string, n)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			var stdout, stderr strings.Builder
			codes[i] = Main(args, &stdout, &stderr)
			errs[i] = stderr.String()
		}()
	}
	close(start)
	wg.Wait()
	return codes, errs
}

// assertConsistent checks the invariant every concurrent case must hold: the
// tree, the config, and the journal all agree on how many bumps happened. Before
// the project lock existed, eight simultaneous runs all reported success while
// the tree advanced one patch and the journal kept three entries — the lost
// bumps could never be undone, because nothing recorded them.
func assertConsistent(t *testing.T, dir string, base string, applied int) {
	t.Helper()
	cfgPath := filepath.Join(dir, "incrmit.toml")
	want := fmt.Sprintf("%s.%d", base, applied)

	got, err := os.ReadFile(filepath.Join(dir, "VERSION"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(got)) != want {
		t.Errorf("VERSION = %q after %d applied bump(s), want %q", got, applied, want)
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("loading config: %v", err)
	}
	if len(cfg.Files) != 1 || cfg.Files[0].Token() != want {
		t.Errorf("config records %+v, want the version the file holds (%s)", cfg.Files, want)
	}

	h, err := history.Load(history.ResolvePath(cfgPath))
	if err != nil {
		t.Fatalf("loading history: %v", err)
	}
	if len(h.Entries) != applied {
		t.Errorf("journal has %d entr(ies) after %d applied bump(s); an unrecorded bump can never be undone",
			len(h.Entries), applied)
	}
}

// concurrentProject builds a one-file project at 1.0.0 and returns its
// directory and the absolute path of its config.
func concurrentProject(t *testing.T) (string, string) {
	t.Helper()
	dir := project(t, "[[files]]\npath = \"VERSION\"\nversion = \"1.0.0\"\n",
		map[string]string{"VERSION": "1.0.0\n"})
	return dir, filepath.Join(dir, "incrmit.toml")
}

// Eight bumps at once: one takes the project, the rest fail fast. Whatever the
// split, the tree, the config, and the journal must agree afterwards.
func TestConcurrentBumpsKeepStateConsistent(t *testing.T) {
	dir, cfgPath := concurrentProject(t)

	const n = 8
	codes, errs := runConcurrent(t, n, "-c", cfgPath)
	applied, refused := 0, 0
	for i, c := range codes {
		switch c {
		case ExitOK:
			applied++
		case ExitError:
			refused++
			if !strings.Contains(errs[i], "already writing") {
				t.Errorf("run %d failed without the contention message: %q", i, errs[i])
			}
		default:
			t.Errorf("run %d exited %d, want %d or %d", i, c, ExitOK, ExitError)
		}
	}
	if applied != 1 {
		t.Errorf("%d of %d runs applied a bump, want exactly 1 (the rest should fail fast)", applied, n)
	}
	if applied+refused != n {
		t.Errorf("%d applied + %d refused != %d runs", applied, refused, n)
	}
	assertConsistent(t, dir, "1.0", applied)
}

// The same eight runs with --wait queue up instead of failing, so every one of
// them applies. This is the case a serialized implementation cannot pass by
// accident: eight bumps must land eight patch increments and eight journal
// entries, not one.
func TestConcurrentBumpsWithWaitAllApply(t *testing.T) {
	dir, cfgPath := concurrentProject(t)

	const n = 8
	codes, errs := runConcurrent(t, n, "-c", cfgPath, "--wait")
	for i, c := range codes {
		if c != ExitOK {
			t.Errorf("waiting run %d exited %d, stderr = %q", i, c, errs[i])
		}
	}
	assertConsistent(t, dir, "1.0", n)
}

// A contended run must write nothing at all: not the target, not the config,
// and not the journal.
func TestContendedBumpWritesNothing(t *testing.T) {
	dir, cfgPath := concurrentProject(t)
	held, err := lock.Acquire(dir)
	if err != nil {
		t.Fatalf("taking the lock the test holds: %v", err)
	}
	t.Cleanup(func() { _ = held.Release() })

	code, stdout, stderr := runMain(t, "", "-c", cfgPath)
	if code != ExitError {
		t.Fatalf("exit = %d, want %d (stdout %q, stderr %q)", code, ExitError, stdout, stderr)
	}
	if !strings.Contains(stderr, "already writing") || !strings.Contains(stderr, "--wait") {
		t.Errorf("stderr = %q, want a contention message naming --wait", stderr)
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "VERSION")); strings.TrimSpace(string(got)) != "1.0.0" {
		t.Errorf("VERSION = %q, want it untouched at 1.0.0", got)
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Files[0].Token() != "1.0.0" {
		t.Errorf("config records %q, want it untouched at 1.0.0", cfg.Files[0].Token())
	}
	if _, err := os.Stat(history.ResolvePath(cfgPath)); !os.IsNotExist(err) {
		t.Errorf("a refused bump wrote a journal (stat err = %v)", err)
	}
}

// undo and discover take the same lock as bump, so neither can interleave with
// a run already writing in the project.
func TestContendedUndoAndDiscoverFail(t *testing.T) {
	dir, cfgPath := concurrentProject(t)
	// A bump first, so undo has something to revert and cannot exit 0 for want
	// of history.
	if code, _, stderr := runMain(t, "", "-c", cfgPath); code != ExitOK {
		t.Fatalf("setup bump exit = %d, stderr = %q", code, stderr)
	}

	held, err := lock.Acquire(dir)
	if err != nil {
		t.Fatalf("taking the lock the test holds: %v", err)
	}
	t.Cleanup(func() { _ = held.Release() })

	for _, tt := range []struct {
		name string
		args []string
	}{
		{"undo", []string{"undo", "-c", cfgPath}},
		{"discover", []string{"discover", "-P", dir, "-o", cfgPath}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			code, _, stderr := runMain(t, "", tt.args...)
			if code != ExitError {
				t.Errorf("exit = %d, want %d", code, ExitError)
			}
			if !strings.Contains(stderr, "already writing") {
				t.Errorf("stderr = %q, want a contention message", stderr)
			}
			if got, _ := os.ReadFile(filepath.Join(dir, "VERSION")); strings.TrimSpace(string(got)) != "1.0.1" {
				t.Errorf("VERSION = %q, want the setup bump left in place", got)
			}
		})
	}
}

// The read-only commands are deliberately lock-free: inspecting a project can
// neither block nor be blocked, even while a bump holds it.
func TestReadOnlyCommandsIgnoreTheLock(t *testing.T) {
	dir, cfgPath := concurrentProject(t)
	held, err := lock.Acquire(dir)
	if err != nil {
		t.Fatalf("taking the lock the test holds: %v", err)
	}
	t.Cleanup(func() { _ = held.Release() })

	for _, tt := range []struct {
		name string
		args []string
		want string
	}{
		{"bump dry-run", []string{"-c", cfgPath, "--dry-run"}, "1.0.0 -> 1.0.1"},
		{"preview", []string{"preview", "-c", cfgPath}, "1.0.0"},
		{"discover dry-run", []string{"discover", "-P", dir, "-d"}, "VERSION"},
		{"undo dry-run", []string{"undo", "-c", cfgPath, "-d"}, "Nothing to undo"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			code, stdout, stderr := runMain(t, "", tt.args...)
			if code != ExitOK {
				t.Fatalf("exit = %d, stderr = %q", code, stderr)
			}
			if !strings.Contains(stdout, tt.want) {
				t.Errorf("stdout = %q, want it to contain %q", stdout, tt.want)
			}
		})
	}
}

// A bump that fails after taking the lock must still give it back, or the
// project would stay locked until the shell exited.
func TestLockReleasedAfterFailedBump(t *testing.T) {
	// The second target does not exist, so planning fails with the lock held.
	dir := project(t, "[[files]]\npath = \"VERSION\"\n[[files]]\npath = \"MISSING\"\n",
		map[string]string{"VERSION": "1.0.0\n"})
	cfgPath := filepath.Join(dir, "incrmit.toml")

	code, _, stderr := runMain(t, "", "-c", cfgPath)
	if code != ExitError {
		t.Fatalf("exit = %d, want %d (stderr %q)", code, ExitError, stderr)
	}
	if !strings.Contains(stderr, "does not exist") {
		t.Fatalf("stderr = %q, want the missing-file error", stderr)
	}

	// The lock must be free: take it directly rather than inferring from a
	// second run, so the assertion is about the lock and nothing else.
	held, err := lock.Acquire(dir)
	if err != nil {
		t.Fatalf("lock still held after a failed bump: %v", err)
	}
	_ = held.Release()

	// And a following run must get as far as the same error, not a contention
	// one.
	if _, _, stderr := runMain(t, "", "-c", cfgPath); strings.Contains(stderr, "already writing") {
		t.Errorf("second run reported contention, so the first never released: %q", stderr)
	}
}

// Where locking is unavailable the run must warn and carry on: a tool that
// cannot bump at all is worse than one that cannot detect a second run. A
// directory sitting where the lock file goes makes the open fail the way an
// unsupported filesystem makes the flock fail.
func TestBumpDegradesWhenLockUnavailable(t *testing.T) {
	dir, cfgPath := concurrentProject(t)
	if err := os.Mkdir(filepath.Join(dir, config.LockFileName), 0o755); err != nil {
		t.Fatal(err)
	}

	code, stdout, stderr := runMain(t, "", "-c", cfgPath)
	if code != ExitOK {
		t.Fatalf("exit = %d, want the bump to proceed unlocked (stderr %q)", code, stderr)
	}
	if !strings.Contains(stderr, "warning") || !strings.Contains(stderr, "continuing unlocked") {
		t.Errorf("stderr = %q, want a warning that the run is unlocked", stderr)
	}
	if !strings.Contains(stdout, "1.0.0 -> 1.0.1") {
		t.Errorf("stdout = %q, want the bump to have been applied anyway", stdout)
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "VERSION")); strings.TrimSpace(string(got)) != "1.0.1" {
		t.Errorf("VERSION = %q, want 1.0.1", got)
	}
}

// Two --file bumps of one file contend too: the lock goes beside the file when
// there is no config to anchor it to.
func TestConcurrentFileModeBumps(t *testing.T) {
	dir := project(t, "", map[string]string{"VERSION": "1.0.0\n"})
	target := filepath.Join(dir, "VERSION")

	codes, _ := runConcurrent(t, 4, "--file", target, "--wait")
	for i, c := range codes {
		if c != ExitOK {
			t.Errorf("run %d exited %d, want %d", i, c, ExitOK)
		}
	}
	if got, _ := os.ReadFile(target); strings.TrimSpace(string(got)) != "1.0.4" {
		t.Errorf("VERSION = %q after 4 serialized --file bumps, want 1.0.4", got)
	}
}

// A run that dies between writing its temp file and renaming it leaves the temp
// file behind. The next run to hold the project lock is the one moment those are
// provably stale — nothing else can have a write in flight — so that is when
// they are cleared.
func TestBumpSweepsStaleTempFiles(t *testing.T) {
	dir, cfgPath := concurrentProject(t)
	sub := filepath.Join(dir, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "OTHER"), []byte("1.0.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfgPath, []byte("[[files]]\npath = \"VERSION\"\n[[files]]\npath = \"sub/OTHER\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	stale := []string{
		filepath.Join(dir, ".incrmit-111.tmp"), // beside the config and a target
		filepath.Join(sub, ".incrmit-222.tmp"), // beside a target in a subdirectory
	}
	untouched := filepath.Join(dir, "incrmit-333.tmp") // not ours
	for _, p := range append(append([]string{}, stale...), untouched) {
		if err := os.WriteFile(p, []byte("half-written\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if code, _, stderr := runMain(t, "", "-c", cfgPath); code != ExitOK {
		t.Fatalf("exit = %d, stderr = %q", code, stderr)
	}
	for _, p := range stale {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("%s survived the bump (stat err = %v)", p, err)
		}
	}
	if _, err := os.Stat(untouched); err != nil {
		t.Errorf("the bump removed a file that is not one of ours: %v", err)
	}
}

// A lock only serializes the runs that take it: a --file bump, an editor, or an
// older incrmit can still move a file on. undo therefore verifies before it
// reverts, and names the file that diverged rather than writing an older
// version back over newer work.
func TestUndoRefusesAfterAnotherRunMovedPast(t *testing.T) {
	dir, cfgPath := concurrentProject(t)
	if code, _, stderr := runMain(t, "", "-c", cfgPath); code != ExitOK {
		t.Fatalf("first bump exit = %d, stderr = %q", code, stderr)
	}
	// A second bump, of the kind that keeps no journal, moves the file past the
	// version the first bump recorded.
	target := filepath.Join(dir, "VERSION")
	if code, _, stderr := runMain(t, "", "--file", target); code != ExitOK {
		t.Fatalf("second bump exit = %d, stderr = %q", code, stderr)
	}

	code, _, stderr := runMain(t, "", "undo", "-c", cfgPath)
	if code != ExitError {
		t.Errorf("exit = %d, want %d", code, ExitError)
	}
	if !strings.Contains(stderr, "VERSION") || !strings.Contains(stderr, "no longer present") {
		t.Errorf("stderr = %q, want it to name the diverged file", stderr)
	}
	if got, _ := os.ReadFile(target); strings.TrimSpace(string(got)) != "1.0.2" {
		t.Errorf("VERSION = %q, want the newer work left alone at 1.0.2", got)
	}
}
