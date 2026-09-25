package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
)

// The tests in this file drive the pathological file shapes of
// internal/files/shapes_test.go through the commands a user runs, where the
// shape has to survive reading, planning, writing, the config rewrite, and undo.

// awkwardTargets are file names a user might pass to --file. Each is taken
// literally from the command line to the rename that replaces the file.
func awkwardTargets() map[string]string {
	names := map[string]string{
		"spaces":       "my version file.txt",
		"non-ASCII":    "versión-版本.txt",
		"leading dash": "-VERSION",
		"flag-shaped":  "--major",
		"quotes":       `it's "the" version`,
		"length limit": strings.Repeat("v", 255),
	}
	if runtime.GOOS != "windows" {
		names["newline"] = "VER\nSION"
		names["star"] = "v*.txt"
		names["class"] = "[ab].txt"
		names["question mark"] = "?.txt"
		names["backslash"] = `back\slash`
	}
	return names
}

// decoys are files that a glob expansion of one of the awkward names would
// reach. None of them may be touched by a bump of the name itself.
var decoys = []string{"vX.txt", "a.txt", "b.txt", "x.txt", "VERSION"}

// --file takes its argument literally, end to end: a name with spaces, a
// newline, a leading dash, or glob metacharacters is the one file bumped, and a
// file the name would match as a pattern is left alone. The name is given
// relative to the working directory, so nothing but the flag parser stands
// between a leading dash and being read as an option.
func TestBumpFileFlagTakesAwkwardNamesLiterally(t *testing.T) {
	for label, name := range awkwardTargets() {
		for _, form := range []string{"--file NAME", "--file=NAME", "-f NAME"} {
			t.Run(label+"/"+form, func(t *testing.T) {
				dir := t.TempDir()
				if err := os.WriteFile(filepath.Join(dir, name), []byte("1.2.3\n"), 0o644); err != nil {
					t.Skipf("cannot create %q here: %v", name, err)
				}
				for _, d := range decoys {
					if err := os.WriteFile(filepath.Join(dir, d), []byte("1.2.3\n"), 0o644); err != nil {
						t.Fatal(err)
					}
				}

				args := []string{strings.Replace(form, "NAME", name, 1)}
				if flagName, _, separate := strings.Cut(form, " "); separate {
					args = []string{flagName, name}
				}
				code, stdout, stderr := runMain(t, dir, args...)
				if code != ExitOK {
					t.Fatalf("exit = %d, stderr = %q", code, stderr)
				}
				if !strings.Contains(stdout, "1.2.3 -> 1.2.4") {
					t.Errorf("stdout = %q, want the bump reported", stdout)
				}
				if got, _ := os.ReadFile(filepath.Join(dir, name)); string(got) != "1.2.4\n" {
					t.Errorf("%q = %q, want it bumped", name, got)
				}
				for _, d := range decoys {
					if d == name {
						continue
					}
					if got, _ := os.ReadFile(filepath.Join(dir, d)); string(got) != "1.2.3\n" {
						t.Errorf("decoy %q = %q, want it untouched", d, got)
					}
				}
			})
		}
	}
}

// A config path is as literal as --file: "[ab].txt" names that file, not a.txt.
// Discovery writes such a name into the config and a bump reads it back, so the
// round trip through TOML must keep every byte of it — the newline and the
// backslash included, which TOML stores escaped.
func TestDiscoverThenBumpAwkwardNames(t *testing.T) {
	names := awkwardTargets()
	dir := t.TempDir()
	for _, name := range names {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("version 1.2.3\n"), 0o644); err != nil {
			t.Skipf("cannot create %q here: %v", name, err)
		}
	}

	if code, _, stderr := runMain(t, dir, "discover"); code != ExitOK {
		t.Fatalf("discover exit = %d, stderr = %q", code, stderr)
	}
	var cfg struct {
		Files []struct{ Path string }
	}
	if _, err := toml.DecodeFile(filepath.Join(dir, "incrmit.toml"), &cfg); err != nil {
		t.Fatalf("reading the generated config: %v", err)
	}
	recorded := map[string]bool{}
	for _, f := range cfg.Files {
		recorded[f.Path] = true
	}
	for label, name := range names {
		if !recorded[name] {
			t.Errorf("%s: config does not record %q exactly; it has %v", label, name, recorded)
		}
	}

	// Plant the decoys only now, so discovery did not list them.
	for _, d := range decoys {
		if err := os.WriteFile(filepath.Join(dir, d), []byte("version 1.2.3\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if code, _, stderr := runMain(t, dir, "--minor"); code != ExitOK {
		t.Fatalf("bump exit = %d, stderr = %q", code, stderr)
	}
	for label, name := range names {
		if got, _ := os.ReadFile(filepath.Join(dir, name)); string(got) != "version 1.3.0\n" {
			t.Errorf("%s: %q = %q, want it bumped", label, name, got)
		}
	}
	for _, d := range decoys {
		if got, _ := os.ReadFile(filepath.Join(dir, d)); string(got) != "version 1.2.3\n" {
			t.Errorf("decoy %q = %q, want it untouched", d, got)
		}
	}
}

// A metacharacter in the config's ignore list is a pattern, and wrapping it in
// brackets is how a pattern names that character literally.
func TestDiscoverIgnoreBracketsAMetacharacter(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows file names cannot hold *")
	}
	dir := project(t, "ignore = [\"v[*].txt\"]\n[[files]]\npath = \"VERSION\"\n", map[string]string{
		"v*.txt":  "1.2.3\n",
		"vX.txt":  "1.2.3\n",
		"VERSION": "1.2.3\n",
	})

	code, stdout, stderr := runMain(t, dir, "discover", "--dry-run")
	if code != ExitOK {
		t.Fatalf("exit = %d, stderr = %q", code, stderr)
	}
	if strings.Contains(stdout, "v*.txt:") {
		t.Errorf("v*.txt was discovered despite ignore = [\"v[*].txt\"]:\n%s", stdout)
	}
	if !strings.Contains(stdout, "vX.txt:") {
		t.Errorf("vX.txt was ignored, but v[*].txt names only the literal v*.txt:\n%s", stdout)
	}
}

// A bump and its undo leave a CRLF file with a BOM byte-for-byte as it was, with
// only the token different in between: neither the rewrite nor its reversal
// normalizes line endings or drops the mark.
func TestBumpAndUndoKeepFileShape(t *testing.T) {
	tmpl := "\xEF\xBB\xBF<Project>\r\n  <Version>@</Version>\n  <!-- @ -->\r\n</Project>"
	before := strings.ReplaceAll(tmpl, "@", "1.2.3")
	after := strings.ReplaceAll(tmpl, "@", "1.3.0")
	dir := project(t, "[[files]]\npath = \"app.props\"\nversion = \"1.2.3\"\n", map[string]string{"app.props": before})
	path := filepath.Join(dir, "app.props")

	if code, _, stderr := runMain(t, dir, "--minor"); code != ExitOK {
		t.Fatalf("bump exit = %d, stderr = %q", code, stderr)
	}
	if got, _ := os.ReadFile(path); string(got) != after {
		t.Errorf("after bump:\n got %q\nwant %q", got, after)
	}
	if code, _, stderr := runMain(t, dir, "undo"); code != ExitOK {
		t.Fatalf("undo exit = %d, stderr = %q", code, stderr)
	}
	if got, _ := os.ReadFile(path); string(got) != before {
		t.Errorf("after undo:\n got %q\nwant %q", got, before)
	}
}

// A UTF-16 file holds no version incrmit can see, and says so with exit 3
// rather than succeeding or rewriting it: scanned, it reports "no semantic
// version found"; pinned in the config, "expected version not found".
func TestBumpUTF16FileReportsNoVersion(t *testing.T) {
	utf16 := []byte{0xFF, 0xFE} // little-endian, as Windows writes it
	for _, c := range []byte("<Version>1.2.3</Version>\r\n") {
		utf16 = append(utf16, c, 0)
	}

	t.Run("scanned", func(t *testing.T) {
		dir := project(t, "", map[string]string{"app.props": string(utf16)})
		code, _, stderr := runMain(t, dir, "--file", "app.props")
		if code != ExitNoVersion || !strings.Contains(stderr, "no semantic version found") {
			t.Errorf("exit = %d, stderr = %q; want %d and no semantic version found", code, stderr, ExitNoVersion)
		}
	})
	t.Run("pinned", func(t *testing.T) {
		dir := project(t, "[[files]]\npath = \"app.props\"\nversion = \"1.2.3\"\n", map[string]string{"app.props": string(utf16)})
		code, _, stderr := runMain(t, dir)
		if code != ExitNoVersion || !strings.Contains(stderr, "expected version not found") {
			t.Errorf("exit = %d, stderr = %q; want %d and expected version not found", code, stderr, ExitNoVersion)
		}
		if got, _ := os.ReadFile(filepath.Join(dir, "app.props")); !bytes.Equal(got, utf16) {
			t.Error("the UTF-16 file was rewritten")
		}
	})
}

// An empty or blank target is exit 3, never a crash or a write.
func TestBumpEmptyFileReportsNoVersion(t *testing.T) {
	for _, body := range []string{"", "\r\n \t\n"} {
		code, _, stderr, got := bumpOneFile(t, body)
		if code != ExitNoVersion || !strings.Contains(stderr, "no semantic version found") {
			t.Errorf("body %q: exit = %d, stderr = %q; want %d", body, code, stderr, ExitNoVersion)
		}
		if got != body {
			t.Errorf("body %q was rewritten to %q", body, got)
		}
	}
}

// A read-only target in config mode bumps and stays read-only: the write is a
// rename in the directory, and the mode is carried across.
func TestBumpReadOnlyTarget(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX file modes")
	}
	dir := project(t, "[[files]]\npath = \"VERSION\"\n", map[string]string{"VERSION": "1.2.3\n"})
	path := filepath.Join(dir, "VERSION")
	if err := os.Chmod(path, 0o444); err != nil {
		t.Fatal(err)
	}

	if code, _, stderr := runMain(t, dir); code != ExitOK {
		t.Fatalf("exit = %d, stderr = %q", code, stderr)
	}
	if got, _ := os.ReadFile(path); string(got) != "1.2.4\n" {
		t.Errorf("VERSION = %q, want 1.2.4", got)
	}
	if info, err := os.Stat(path); err != nil || info.Mode().Perm() != 0o444 {
		t.Errorf("mode = %v (err %v), want 0444 kept", info.Mode().Perm(), err)
	}
}

// The discover dry run prints each occurrence's line; for a file with classic
// Mac line endings that line must not carry a carriage return, which a terminal
// would obey by returning to column 0 and overprinting what came before.
func TestDiscoverDryRunLoneCRFile(t *testing.T) {
	dir := project(t, "", map[string]string{"setup.cfg": "[metadata]\rname = app\rversion = 1.2.3\r"})

	code, stdout, stderr := runMain(t, dir, "discover", "--dry-run")
	if code != ExitOK {
		t.Fatalf("exit = %d, stderr = %q", code, stderr)
	}
	if !strings.Contains(stdout, "L3: version = 1.2.3\n") {
		t.Errorf("stdout = %q, want the version reported on line 3", stdout)
	}
	if strings.Contains(stdout, "\r") {
		t.Errorf("stdout carries a carriage return: %q", stdout)
	}
}

// Two hard-linked names listed in the config are both bumped, even though the
// first write breaks the link: planning reads every target before anything is
// written, so the second name is rewritten from the contents it had, not from a
// file the first rename already replaced.
func TestBumpHardLinkedNamesInConfig(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("hard-link semantics are verified on Unix only")
	}
	dir := project(t, "[[files]]\npath = \"VERSION\"\n[[files]]\npath = \"VERSION.link\"\n", map[string]string{"VERSION": "1.2.3\n"})
	if err := os.Link(filepath.Join(dir, "VERSION"), filepath.Join(dir, "VERSION.link")); err != nil {
		t.Skipf("hard links unsupported: %v", err)
	}

	if code, _, stderr := runMain(t, dir); code != ExitOK {
		t.Fatalf("exit = %d, stderr = %q", code, stderr)
	}
	for _, name := range []string{"VERSION", "VERSION.link"} {
		if got, _ := os.ReadFile(filepath.Join(dir, name)); string(got) != "1.2.4\n" {
			t.Errorf("%s = %q, want 1.2.4", name, got)
		}
	}
}
