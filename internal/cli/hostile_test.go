package cli

import (
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/sasmaq/incrmit/internal/config"
	"github.com/sasmaq/incrmit/internal/history"
)

// hostileNames are file names built to attack the terminal that prints them:
// clear the screen, retitle the window, plant a hyperlink, move the cursor back
// over text already printed, and reorder a line with a bidi override.
var hostileNames = []string{
	"esc\x1b[2J.txt",
	"title\x1b]0;pwned\x07.txt",
	"link\x1b]8;;evil\x1b\\click\x1b]8;;\x1b\\.txt",
	"cr\roverwrite.txt",
	"bs\b\b.txt",
	"bel\a.txt",
	"c1\u009b31m.txt",
	"invoice\u202etxt.exe",
	"tab\tname.txt",
	"nl\nname.txt",
}

// hostileBody holds a version on a line that is itself hostile, so the dry
// run's context carries escapes too, along with a raw CSI byte and Latin-1.
const hostileBody = "# \x1b[2J\x1b]0;pwned\x07 \u202e \x9b31m caf\xe9\n" +
	"version = 1.2.3 \x1b[31mred\x1b[0m\b\n"

// assertTerminalSafe fails when out holds anything a terminal would act on:
// every character must be printable, a newline, or a tab.
func assertTerminalSafe(t *testing.T, what, out string) {
	t.Helper()
	if !utf8.ValidString(out) {
		t.Errorf("%s: output is not valid UTF-8: %q", what, out)
		return
	}
	for _, r := range out {
		if r != '\n' && r != '\t' && !strconv.IsPrint(r) {
			t.Errorf("%s: output holds %U: %q", what, r, out)
			return
		}
	}
}

// assertNamesQuoted fails when a hostile name appears in out other than in its
// quoted form. The terminal writer would have escaped a name printed without
// displayName, so the output would still be safe; but the name would be
// ambiguous, and this is how a print site that forgot displayName is found.
// Names in want must appear at least once.
func assertNamesQuoted(t *testing.T, what, out string, want []string) {
	t.Helper()
	for _, name := range hostileNames {
		rest := strings.ReplaceAll(out, displayName(name), "")
		if bare := terminalText(name); strings.Contains(rest, bare) {
			t.Errorf("%s: %q is printed without displayName: %q", what, name, out)
		}
	}
	for _, name := range want {
		if !strings.Contains(out, displayName(name)) {
			t.Errorf("%s: output does not name %s: %q", what, displayName(name), out)
		}
	}
}

// hostileTree writes every hostile name, and on a system that allows it one
// whose name is not UTF-8, into a temp dir holding hostileBody. It returns the
// dir and the names that are expected to reach the config.
func hostileTree(t *testing.T) (string, []string, string) {
	t.Helper()
	dir := t.TempDir()
	for _, name := range hostileNames {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(hostileBody), 0o644); err != nil {
			t.Fatalf("creating %q: %v", name, err)
		}
	}
	// Linux takes any bytes as a name; macOS refuses what is not UTF-8.
	raw := "raw\x9b31m.txt"
	if err := os.WriteFile(filepath.Join(dir, raw), []byte(hostileBody), 0o644); err != nil {
		raw = ""
	}
	return dir, hostileNames, raw
}

// Every command, run over a tree of hostile names and contents, prints nothing
// a terminal would act on, and names every file in its quoted form. The names
// stay raw where the file system and the TOML files need them.
func TestHostileTreeNeverReachesTheTerminal(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows file names cannot hold control characters")
	}
	dir, names, raw := hostileTree(t)

	run := func(what string, want []string, args ...string) (int, string) {
		t.Helper()
		code, stdout, stderr := runMain(t, dir, args...)
		out := stdout + stderr
		assertTerminalSafe(t, what, out)
		assertNamesQuoted(t, what, out, want)
		return code, out
	}
	// Some text is not incrmit's to format: the flag package's complaint about
	// an unknown flag, and an OS error naming a path it could not walk. The
	// writer still escapes it, which is all that is asserted.
	runUnformatted := func(what string, args ...string) {
		t.Helper()
		_, stdout, stderr := runMain(t, dir, args...)
		assertTerminalSafe(t, what, stdout+stderr)
	}

	if code, out := run("discover --dry-run", names, "discover", "--dry-run"); code != ExitOK {
		t.Fatalf("discover --dry-run exit = %d: %q", code, out)
	} else if !strings.Contains(out, `version = 1.2.3 \x1b[31mred\x1b[0m\b`) {
		t.Errorf("dry-run context is not shown escaped: %q", out)
	}

	code, out := run("discover", names, "discover")
	if code != ExitOK {
		t.Fatalf("discover exit = %d: %q", code, out)
	}
	if raw != "" && !strings.Contains(out, "warning: skipping "+displayName(raw)+": its name is not valid UTF-8") {
		t.Errorf("discover did not warn about the name that is not UTF-8: %q", out)
	}

	// The config holds the raw names: display is escaped, storage is not.
	cfg, err := config.Load(filepath.Join(dir, "incrmit.toml"))
	if err != nil {
		t.Fatalf("the generated config does not load: %v", err)
	}
	stored := map[string]bool{}
	for _, f := range cfg.Files {
		stored[f.Path] = true
	}
	for _, name := range names {
		if !stored[name] {
			t.Errorf("config does not record %q under its real name", name)
		}
	}
	if len(cfg.Files) != len(names) {
		t.Errorf("config lists %d files, want %d", len(cfg.Files), len(names))
	}

	run("preview", names, "preview")
	run("bump --dry-run", names, "--dry-run")
	if code, out := run("bump", names, "--minor"); code != ExitOK {
		t.Fatalf("bump exit = %d: %q", code, out)
	}
	for _, name := range names {
		got, _ := os.ReadFile(filepath.Join(dir, name))
		if want := strings.Replace(hostileBody, "1.2.3", "1.3.0", 1); string(got) != want {
			t.Errorf("%q after bump = %q, want %q", name, got, want)
		}
	}

	// The journal records the raw names too, so undo finds the files again.
	h, err := history.Load(history.ResolvePath(filepath.Join(dir, "incrmit.toml")))
	if err != nil {
		t.Fatal(err)
	}
	entry, ok := h.Latest()
	if !ok {
		t.Fatal("the bump recorded no journal entry")
	}
	for _, c := range entry.Changes {
		if !stored[c.Path] || filepath.Base(c.FS) != c.Path {
			t.Errorf("journal records %q (%q), not a real name", c.Path, c.FS)
		}
	}

	run("undo --dry-run", names, "undo", "--dry-run")
	if code, out := run("undo", names, "undo"); code != ExitOK {
		t.Fatalf("undo exit = %d: %q", code, out)
	}
	for _, name := range names {
		if got, _ := os.ReadFile(filepath.Join(dir, name)); string(got) != hostileBody {
			t.Errorf("%q after undo = %q, want the original", name, got)
		}
	}

	target := names[0]
	run("--file", []string{target}, "--file", target)
	run("--file --dry-run", []string{target}, "--file", target, "--dry-run")
	if code, _ := run("--file on a missing hostile name", nil, "--file", "gone\x1b[2J"); code == ExitOK {
		t.Error("bumping a missing file succeeded")
	}
	runUnformatted("unknown flag", "--"+names[1])
	runUnformatted("discover --path missing", "discover", "--dry-run", "--path", "gone\x1b]0;x\x07")
}

// The failure paths that name a file print it quoted as well.
func TestHostileNamesInFailureMessages(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows file names cannot hold control characters")
	}
	name := hostileNames[0]
	newProject := func(t *testing.T) string {
		t.Helper()
		body := "[[files]]\npath = " + strconv.Quote(name) + "\nversion = \"1.2.3\"\n"
		return project(t, body, map[string]string{name: "version 1.2.3\n"})
	}
	check := func(t *testing.T, dir string, wantCode int, wantText string, args ...string) {
		t.Helper()
		code, stdout, stderr := runMain(t, dir, args...)
		out := stdout + stderr
		assertTerminalSafe(t, strings.Join(args, " "), out)
		assertNamesQuoted(t, strings.Join(args, " "), out, []string{name})
		if code != wantCode || !strings.Contains(out, wantText) {
			t.Errorf("exit = %d, output %q; want %d and %q", code, out, wantCode, wantText)
		}
	}

	t.Run("version not found", func(t *testing.T) {
		dir := newProject(t)
		if err := os.WriteFile(filepath.Join(dir, name), []byte("no version\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		check(t, dir, ExitNoVersion, "expected version not found")
	})
	t.Run("unreadable target", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("running as root: file permissions are not enforced")
		}
		dir := newProject(t)
		path := filepath.Join(dir, name)
		if err := os.Chmod(path, 0o000); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(path, 0o644) })
		check(t, dir, ExitError, "permission denied")
	})
	t.Run("conflicted undo", func(t *testing.T) {
		dir := newProject(t)
		if code, _, stderr := runMain(t, dir); code != ExitOK {
			t.Fatalf("bump exit = %d: %q", code, stderr)
		}
		if err := os.WriteFile(filepath.Join(dir, name), []byte("edited since\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		check(t, dir, ExitError, "refusing to undo", "undo")
	})
}
