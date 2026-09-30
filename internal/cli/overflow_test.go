package cli

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// These tests pin Milestone 35. Parse accepts any numeric component
// strconv.Atoi can read, up to math.MaxInt, and the bump methods used to add
// one without checking: 1.2.9223372036854775807 bumped to
// 1.2.-9223372036854775808, which was written into the file and the config and
// which the next command could not find at all. A bump that would carry a
// component, or a prerelease counter, past math.MaxInt is now refused with exit
// 3, naming the file and the component, before anything is written; preview
// shows that projection as unavailable.

var (
	maxN     = strconv.Itoa(math.MaxInt)
	belowMax = strconv.Itoa(math.MaxInt - 1)
)

// overflowProject writes a one-file project whose VERSION holds token and whose
// config pins it, and returns its directory.
func overflowProject(t *testing.T, token string) string {
	t.Helper()
	return project(t, fmt.Sprintf("[[files]]\npath = \"VERSION\"\nversion = %q\n", token),
		map[string]string{"VERSION": token + "\n"})
}

// Each numeric component at the ceiling is refused by the bump that increments
// it, by that bump's --dry-run, and by --pre when it opens a new line through
// that component; a prerelease counter at the ceiling, or past it, is refused
// by --pre. Nothing is written: not the file, not the config, and no journal
// is started.
func TestBumpRefusesToOverflow(t *testing.T) {
	cases := []struct {
		name  string
		token string
		args  []string
		part  string // what the refusal must name
	}{
		{"patch", "1.2." + maxN, nil, "patch component"},
		{"patch flag", "1.2." + maxN, []string{"--patch"}, "patch component"},
		{"minor", "1." + maxN + ".3", []string{"--minor"}, "minor component"},
		{"major", maxN + ".2.3", []string{"--major"}, "major component"},
		{"prefix and build", "v1.2." + maxN + "+build.7", nil, "patch component"},
		{"prerelease dropped", "1.2." + maxN + "-rc.1", nil, "patch component"},
		{"pre from a release", "1.2." + maxN, []string{"--pre", "rc"}, "patch component"},
		{"pre with minor", "1." + maxN + ".3-rc.1", []string{"--minor", "--pre", "rc"}, "minor component"},
		{"pre with major", maxN + ".2.3", []string{"--major", "--pre", "rc"}, "major component"},
		{"pre counter", "1.2.3-rc." + maxN, []string{"--pre", "rc"}, "prerelease counter"},
		{"pre counter with build", "v1.2.3-rc." + maxN + "+build.7", []string{"--pre", "rc"}, "prerelease counter"},
		// Too large for an int at all. It used to gain a ".1" instead of
		// counting, which ranks higher but leaves a series the next --pre rc
		// no longer recognizes, so it restarted at rc.1, below both.
		{"pre counter past an int", "1.2.3-rc.99999999999999999999", []string{"--pre", "rc"}, "prerelease counter"},
	}
	for _, tc := range cases {
		for _, dryRun := range []bool{false, true} {
			name := tc.name
			args := tc.args
			if dryRun {
				name += "/dry run"
				args = append([]string{"--dry-run"}, args...)
			}
			t.Run(name, func(t *testing.T) {
				dir := overflowProject(t, tc.token)
				before := snapshotProject(t, dir)

				code, stdout, stderr := runMain(t, dir, args...)
				if code != ExitNoVersion {
					t.Fatalf("exit = %d, want %d (stdout %q, stderr %q)", code, ExitNoVersion, stdout, stderr)
				}
				for _, want := range []string{"VERSION", tc.part, tc.token} {
					if !strings.Contains(stderr, want) {
						t.Errorf("stderr = %q, want it to name %q", stderr, want)
					}
				}
				if stdout != "" {
					t.Errorf("stdout = %q, want nothing printed for a refused bump", stdout)
				}
				assertSnapshotEqual(t, before, snapshotProject(t, dir))
			})
		}
	}
}

// --file mode plans through the same path, so it refuses the same way.
func TestFileBumpRefusesToOverflow(t *testing.T) {
	body := "1.2." + maxN + "\n"
	code, stdout, stderr, got := bumpOneFile(t, body)
	if code != ExitNoVersion {
		t.Fatalf("exit = %d, want %d (stdout %q, stderr %q)", code, ExitNoVersion, stdout, stderr)
	}
	if !strings.Contains(stderr, "patch component") {
		t.Errorf("stderr = %q, want the patch component named", stderr)
	}
	if got != body {
		t.Errorf("file = %q, want it unchanged", got)
	}
}

// One file at the ceiling stops the whole bump, including the files planned
// before it that could have been bumped.
func TestBumpOverflowInOneFileWritesNothing(t *testing.T) {
	full := "1.2." + maxN
	dir := project(t, fmt.Sprintf(
		"[[files]]\npath = \"VERSION\"\nversion = \"1.2.3\"\n[[files]]\npath = \"full.txt\"\nversion = %q\n", full),
		map[string]string{"VERSION": "1.2.3\n", "full.txt": full + "\n"})
	before := snapshotProject(t, dir)

	code, _, stderr := runMain(t, dir)
	if code != ExitNoVersion {
		t.Fatalf("exit = %d, want %d (stderr %q)", code, ExitNoVersion, stderr)
	}
	if !strings.Contains(stderr, "full.txt") {
		t.Errorf("stderr = %q, want the file at the ceiling named", stderr)
	}
	assertSnapshotEqual(t, before, snapshotProject(t, dir))
}

// Up to the ceiling itself everything still counts. The last value an int
// holds is written and recorded, and undo finds it in the file again and
// restores the original. A component at the ceiling does not stop a bump of a
// higher one, which resets it, nor --release or a switch of prerelease series,
// which do not count.
func TestBumpUpToTheCeiling(t *testing.T) {
	cases := []struct {
		name  string
		token string
		args  []string
		want  string
	}{
		{"patch", "1.2." + belowMax, nil, "1.2." + maxN},
		{"minor", "1." + belowMax + ".3", []string{"--minor"}, "1." + maxN + ".0"},
		{"major", belowMax + ".2.3", []string{"--major"}, maxN + ".0.0"},
		{"pre counter", "1.2.3-rc." + belowMax, []string{"--pre", "rc"}, "1.2.3-rc." + maxN},
		{"minor over a full patch", "1.2." + maxN, []string{"--minor"}, "1.3.0"},
		{"major over full minor and patch", "1." + maxN + "." + maxN, []string{"--major"}, "2.0.0"},
		{"release", maxN + "." + maxN + "." + maxN + "-rc.1", []string{"--release"}, maxN + "." + maxN + "." + maxN},
		{"switch series", "1.2.3-beta." + maxN, []string{"--pre", "rc"}, "1.2.3-rc.1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := overflowProject(t, tc.token)
			versionFile := filepath.Join(dir, "VERSION")

			code, stdout, stderr := runMain(t, dir, tc.args...)
			if code != ExitOK {
				t.Fatalf("exit = %d (stderr %q)", code, stderr)
			}
			if want := tc.token + " -> " + tc.want; !strings.Contains(stdout, want) {
				t.Errorf("stdout = %q, want %q", stdout, want)
			}
			if got, err := os.ReadFile(versionFile); err != nil {
				t.Fatal(err)
			} else if string(got) != tc.want+"\n" {
				t.Fatalf("file = %q, want %q", got, tc.want+"\n")
			}

			if code, _, stderr := runMain(t, dir, "undo"); code != ExitOK {
				t.Fatalf("undo exit = %d (stderr %q), want the written token found again", code, stderr)
			}
			if got, err := os.ReadFile(versionFile); err != nil {
				t.Fatal(err)
			} else if string(got) != tc.token+"\n" {
				t.Errorf("file after undo = %q, want %q", got, tc.token+"\n")
			}
		})
	}
}

// preview shows a projection that would overflow as n/a, in whichever column
// it falls, and still exits 0: the other projections are real, and preview
// reports rather than refuses.
func TestPreviewShowsOverflowAsUnavailable(t *testing.T) {
	cases := []struct {
		token               string
		patch, minor, major string
	}{
		{"1.2." + maxN, "n/a", "1.3.0", "2.0.0"},
		{"1." + maxN + ".3", "1." + maxN + ".4", "n/a", "2.0.0"},
		{maxN + ".2.3", maxN + ".2.4", maxN + ".3.0", "n/a"},
		{"v" + maxN + "." + maxN + "." + maxN + "-rc.1", "n/a", "n/a", "n/a"},
	}
	for _, tc := range cases {
		t.Run(tc.token, func(t *testing.T) {
			dir := overflowProject(t, tc.token)
			code, stdout, stderr := runMain(t, dir, "preview")
			if code != ExitOK {
				t.Fatalf("exit = %d (stderr %q)", code, stderr)
			}
			if stderr != "" {
				t.Errorf("stderr = %q, want empty", stderr)
			}
			lines := strings.Split(stdout, "\n")
			if len(lines) < 2 {
				t.Fatalf("stdout = %q, want a header and a row", stdout)
			}
			want := []string{"VERSION", tc.token, tc.patch, tc.minor, tc.major}
			if got := strings.Fields(lines[1]); !slices.Equal(got, want) {
				t.Errorf("row = %q, want %q", got, want)
			}
			if !strings.Contains(stdout, "\nn/a: ") {
				t.Errorf("stdout = %q, want a footnote explaining n/a", stdout)
			}
		})
	}
}

// With every column holding an n/a somewhere, and a drifting row beside them,
// the table still lines up and both footnotes are printed.
func TestPreviewOverflowGolden(t *testing.T) {
	golden := goldenPath(t, "preview_overflow.golden")
	full := maxN + "." + maxN + "." + maxN
	dir := project(t, fmt.Sprintf(
		"[[files]]\npath = \"VERSION\"\nversion = \"1.2.%[1]s\"\n"+
			"[[files]]\npath = \"README.md\"\nversion = \"v1.2.%[1]s\"\n"+
			"[[files]]\npath = \"full.txt\"\nversion = %[2]q\n", maxN, full),
		map[string]string{
			"VERSION":   "1.2." + maxN + "\n",
			"README.md": "release v1.2." + maxN + "\n",
			"full.txt":  full + "\n",
		})

	code, stdout, stderr := runMain(t, dir, "preview")
	if code != ExitOK {
		t.Fatalf("exit = %d, stderr = %q", code, stderr)
	}
	checkGolden(t, golden, stdout)
}
