package cli

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sasmaq/incrmit/internal/version"
)

// updatePreview regenerates the preview .golden files from current behaviour:
//
//	go test ./internal/cli/ -update
var updatePreview = flag.Bool("update", false, "regenerate golden files")

// checkGolden compares got against testdata/<name>, rewriting the file instead
// when -update is given. The golden path is resolved to an absolute path by the
// caller, because runMain leaves the working directory inside a temp project.
func checkGolden(t *testing.T, goldenPath, got string) {
	t.Helper()
	if *updatePreview {
		if err := os.WriteFile(goldenPath, []byte(got), 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
	}
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden (run with -update to create it): %v", err)
	}
	if got != string(want) {
		t.Errorf("golden mismatch for %s\n--- got ---\n%s\n--- want ---\n%s", filepath.Base(goldenPath), got, want)
	}
}

// goldenPath returns the absolute path of a testdata golden file, resolved
// before any chdir so it stays valid from inside a temp project directory.
func goldenPath(t *testing.T, name string) string {
	t.Helper()
	p, err := filepath.Abs(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// syncedProject is a config whose entries all agree on 1.2.3, mixing a bare and
// a "v"-prefixed token and paths of very different lengths so the golden table
// exercises column padding.
func syncedProject(t *testing.T) string {
	t.Helper()
	body := "[[files]]\npath = \"VERSION\"\nversion = \"1.2.3\"\n" +
		"[[files]]\npath = \"docs/deeply/nested/notes.md\"\nversion = \"v1.2.3\"\n" +
		"[[files]]\npath = \"app.py\"\nversion = \"1.2.3\"\n"
	return project(t, body, map[string]string{
		"VERSION":                     "1.2.3\n",
		"docs/deeply/nested/notes.md": "release v1.2.3\n",
		"app.py":                      "__version__ = \"1.2.3\"\n",
	})
}

// The table lines up every column and keeps the "v" prefix in the projected
// versions.
func TestPreviewTableGolden(t *testing.T) {
	golden := goldenPath(t, "preview_table.golden")
	dir := syncedProject(t)

	code, stdout, stderr := runMain(t, dir, "preview")
	if code != ExitOK {
		t.Fatalf("exit = %d, stderr = %q", code, stderr)
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want empty", stderr)
	}
	checkGolden(t, golden, stdout)
}

// A file listed once per version it contains gets one row per version, each
// projected independently.
func TestPreviewFileWithMultipleVersions(t *testing.T) {
	body := "[[files]]\npath = \"notes.md\"\nversion = \"1.2.3\"\n" +
		"[[files]]\npath = \"notes.md\"\nversion = \"10.20.30\"\n"
	dir := project(t, body, map[string]string{"notes.md": "app 1.2.3, vendored lib 10.20.30\n"})

	code, stdout, stderr := runMain(t, dir, "preview")
	if code != ExitOK {
		t.Fatalf("exit = %d, stderr = %q", code, stderr)
	}
	rows := strings.Split(strings.TrimSpace(stdout), "\n")
	if got := strings.Fields(rows[1]); len(got) < 5 || got[1] != "1.2.3" || got[4] != "2.0.0" {
		t.Errorf("first row = %v, want notes.md at 1.2.3", got)
	}
	if got := strings.Fields(rows[2]); len(got) < 5 || got[1] != "10.20.30" || got[4] != "11.0.0" {
		t.Errorf("second row = %v, want notes.md at 10.20.30", got)
	}
}

// A config whose entries disagree marks the drifting rows and explains the
// marker under the table; the majority version is the one named.
func TestPreviewDriftGolden(t *testing.T) {
	golden := goldenPath(t, "preview_drift.golden")
	body := "[[files]]\npath = \"VERSION\"\nversion = \"1.2.3\"\n" +
		"[[files]]\npath = \"README.md\"\nversion = \"v1.2.3\"\n" +
		"[[files]]\npath = \"stale.txt\"\nversion = \"1.0.0\"\n" +
		"[[files]]\npath = \"ahead.txt\"\nversion = \"2.0.0\"\n"
	dir := project(t, body, map[string]string{
		"VERSION":   "1.2.3\n",
		"README.md": "v1.2.3\n",
		"stale.txt": "1.0.0\n",
		"ahead.txt": "2.0.0\n",
	})

	code, stdout, stderr := runMain(t, dir, "preview")
	if code != ExitOK {
		t.Fatalf("exit = %d, stderr = %q", code, stderr)
	}
	checkGolden(t, golden, stdout)

	if !strings.Contains(stdout, "* differs from 1.2.3") {
		t.Errorf("stdout missing the drift note naming the common version:\n%s", stdout)
	}
	for _, line := range strings.Split(strings.TrimSpace(stdout), "\n") {
		marked := strings.HasSuffix(line, " *")
		wantMarked := strings.HasPrefix(line, "stale.txt") || strings.HasPrefix(line, "ahead.txt")
		if marked != wantMarked {
			t.Errorf("line %q marked = %v, want %v", line, marked, wantMarked)
		}
	}
}

// A "v" prefix and build metadata do not affect semver precedence, so entries
// that differ only that way are in sync: no marker, no note, no marker column.
func TestPreviewDriftIgnoresPrefixAndBuild(t *testing.T) {
	body := "[[files]]\npath = \"VERSION\"\nversion = \"1.2.3\"\n" +
		"[[files]]\npath = \"README.md\"\nversion = \"v1.2.3\"\n" +
		"[[files]]\npath = \"build.txt\"\nversion = \"1.2.3\"\nbuild = \"exp.7\"\n"
	dir := project(t, body, map[string]string{
		"VERSION":   "1.2.3\n",
		"README.md": "v1.2.3\n",
		"build.txt": "1.2.3+exp.7\n",
	})

	code, stdout, stderr := runMain(t, dir, "preview")
	if code != ExitOK {
		t.Fatalf("exit = %d, stderr = %q", code, stderr)
	}
	if strings.Contains(stdout, driftMarker) {
		t.Errorf("stdout marks a row that is only prefix/build-different:\n%s", stdout)
	}
	if strings.Contains(stdout, "differs from") {
		t.Errorf("stdout carries a drift note with nothing out of sync:\n%s", stdout)
	}
}

// --file previews a single target without a config, finding the version by
// scanning the file, and projects it with the "v" prefix intact. The -f
// shorthand is the same flag.
func TestPreviewFile(t *testing.T) {
	dir := project(t, "", map[string]string{"VERSION": "v9.8.7\n"})

	for _, flag := range []string{"--file", "-f"} {
		t.Run(flag, func(t *testing.T) {
			code, stdout, stderr := runMain(t, dir, "preview", flag, "VERSION")
			if code != ExitOK {
				t.Fatalf("exit = %d, stderr = %q", code, stderr)
			}
			rows := strings.Split(strings.TrimSpace(stdout), "\n")
			if len(rows) != 2 {
				t.Fatalf("got %d lines, want a header and one row:\n%s", len(rows), stdout)
			}
			got := strings.Fields(rows[1])
			want := []string{"VERSION", "v9.8.7", "v9.8.8", "v9.9.0", "v10.0.0"}
			if len(got) != len(want) {
				t.Fatalf("row = %v, want %v", got, want)
			}
			for i := range want {
				if got[i] != want[i] {
					t.Errorf("column %d = %q, want %q", i, got[i], want[i])
				}
			}
		})
	}
}

// A component bump drops any prerelease and build section, and the preview must
// project exactly what such a bump would write.
func TestPreviewDropsPrereleaseAndBuild(t *testing.T) {
	dir := project(t, "", map[string]string{"VERSION": "1.2.3-rc.1+exp.5\n"})

	code, stdout, stderr := runMain(t, dir, "preview", "--file", "VERSION")
	if code != ExitOK {
		t.Fatalf("exit = %d, stderr = %q", code, stderr)
	}
	fields := strings.Fields(strings.Split(strings.TrimSpace(stdout), "\n")[1])
	want := []string{"VERSION", "1.2.3-rc.1+exp.5", "1.2.4", "1.3.0", "2.0.0"}
	if len(fields) != len(want) {
		t.Fatalf("row = %v, want %v", fields, want)
	}
	for i := range want {
		if fields[i] != want[i] {
			t.Errorf("column %d = %q, want %q", i, fields[i], want[i])
		}
	}
}

// preview is read-only: no target, config, or state file may change, and no new
// file (such as the bump journal) may appear.
func TestPreviewWritesNothing(t *testing.T) {
	dir := syncedProject(t)
	before := snapshotTree(t, dir)

	code, _, stderr := runMain(t, dir, "preview")
	if code != ExitOK {
		t.Fatalf("exit = %d, stderr = %q", code, stderr)
	}

	after := snapshotTree(t, dir)
	if len(after) != len(before) {
		t.Fatalf("tree has %d files after preview, want %d (before: %v, after: %v)", len(after), len(before), before, after)
	}
	for path, content := range before {
		if after[path] != content {
			t.Errorf("%s changed: %q -> %q", path, content, after[path])
		}
	}
}

// snapshotTree returns every regular file under dir keyed by its relative path,
// with its contents, so a test can assert nothing was written.
func snapshotTree(t *testing.T, dir string) map[string]string {
	t.Helper()
	out := make(map[string]string)
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		out[rel] = string(data)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// A missing config is exit 1 with the message that suggests `incrmit discover`.
func TestPreviewMissingConfig(t *testing.T) {
	dir := t.TempDir()

	code, stdout, stderr := runMain(t, dir, "preview")
	if code != ExitError {
		t.Fatalf("exit = %d, want %d (stderr %q)", code, ExitError, stderr)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty on error", stdout)
	}
	if !strings.Contains(stderr, "incrmit discover") {
		t.Errorf("stderr = %q, want a hint to run `incrmit discover`", stderr)
	}
}

// Bad flags and stray arguments are usage errors (exit 2) that print the
// preview help to stderr.
func TestPreviewUsageErrors(t *testing.T) {
	dir := syncedProject(t)
	for _, args := range [][]string{
		{"preview", "--bogus"},
		{"preview", "stray"},
		{"preview", "--max-file-size", "-1"},
	} {
		t.Run(strings.Join(args[1:], " "), func(t *testing.T) {
			code, stdout, stderr := runMain(t, dir, args...)
			if code != ExitUsage {
				t.Fatalf("exit = %d, want %d (stderr %q)", code, ExitUsage, stderr)
			}
			if stdout != "" {
				t.Errorf("stdout = %q, want empty on a usage error", stdout)
			}
			if !strings.Contains(stderr, "usage: incrmit preview") {
				t.Errorf("stderr = %q, want the preview help", stderr)
			}
		})
	}
}

// A version token that cannot be parsed, or a target with no version at all, is
// exit 3 with the offending path named.
func TestPreviewNoVersionExitCodes(t *testing.T) {
	t.Run("unparseable config version", func(t *testing.T) {
		body := "[[files]]\npath = \"VERSION\"\nversion = \"1.2\"\n"
		dir := project(t, body, map[string]string{"VERSION": "1.2\n"})

		code, stdout, stderr := runMain(t, dir, "preview")
		if code != ExitNoVersion {
			t.Fatalf("exit = %d, want %d (stderr %q)", code, ExitNoVersion, stderr)
		}
		if stdout != "" {
			t.Errorf("stdout = %q, want empty on error", stdout)
		}
		if !strings.Contains(stderr, "VERSION") {
			t.Errorf("stderr = %q, want the offending path named", stderr)
		}
	})

	t.Run("no version in file", func(t *testing.T) {
		dir := project(t, "", map[string]string{"NOTES.md": "nothing to see here\n"})

		code, _, stderr := runMain(t, dir, "preview", "--file", "NOTES.md")
		if code != ExitNoVersion {
			t.Fatalf("exit = %d, want %d (stderr %q)", code, ExitNoVersion, stderr)
		}
		if !strings.Contains(stderr, "NOTES.md") {
			t.Errorf("stderr = %q, want the offending path named", stderr)
		}
	})
}

// --max-file-size applies to preview's reads too, so an oversized target is
// reported rather than read.
func TestPreviewMaxFileSize(t *testing.T) {
	dir := project(t, "", map[string]string{"VERSION": "1.2.3\n"})

	code, _, stderr := runMain(t, dir, "preview", "--file", "VERSION", "--max-file-size", "2")
	if code != ExitError {
		t.Fatalf("exit = %d, want %d (stderr %q)", code, ExitError, stderr)
	}
	if !strings.Contains(stderr, "--max-file-size") {
		t.Errorf("stderr = %q, want the size limit explained", stderr)
	}
}

// An identical (path, version) pair is printed once, however many times it
// reaches the renderer.
func TestPreviewRowsDeduplicates(t *testing.T) {
	v123 := version.Version{Major: 1, Minor: 2, Patch: 3}
	v200 := version.Version{Major: 2}
	groups := []fileGroup{
		{display: "VERSION", entries: []entryPlan{{oldVer: v123}, {oldVer: v200}, {oldVer: v123}}},
		{display: "other.md", entries: []entryPlan{{oldVer: v123}}},
	}

	rows := previewRows(groups)
	if len(rows) != 3 {
		t.Fatalf("got %d rows, want 3: %+v", len(rows), rows)
	}
	want := []struct{ path, current string }{
		{"VERSION", "1.2.3"},
		{"VERSION", "2.0.0"},
		{"other.md", "1.2.3"},
	}
	for i, w := range want {
		if rows[i].path != w.path || rows[i].current.String() != w.current {
			t.Errorf("row %d = (%s, %s), want (%s, %s)", i, rows[i].path, rows[i].current, w.path, w.current)
		}
	}
}

// A single row can never be out of sync with itself, and a tie between two
// versions is resolved in favour of the higher one.
func TestMarkDrift(t *testing.T) {
	v100 := version.Version{Major: 1}
	v200 := version.Version{Major: 2}

	rows := []previewRow{{current: v100, inSync: true}}
	if _, drifted := markDrift(rows); drifted {
		t.Errorf("a lone row was reported as drifting")
	}

	rows = []previewRow{{current: v100, inSync: true}, {current: v200, inSync: true}}
	common, drifted := markDrift(rows)
	if !drifted {
		t.Fatalf("two differing rows were not reported as drifting")
	}
	if common.String() != "2.0.0" {
		t.Errorf("common = %s, want 2.0.0 (ties go to the higher version)", common)
	}
	if rows[0].inSync || !rows[1].inSync {
		t.Errorf("marked rows = %v/%v, want the 1.0.0 row marked", rows[0].inSync, rows[1].inSync)
	}
}

// The common version is the one most rows hold, not the one the first row
// happens to hold, so a single stale entry listed first does not drag the
// whole table into being marked.
func TestMarkDriftFollowsTheMajority(t *testing.T) {
	stale := version.Version{Major: 1}
	current := version.Version{Major: 2}

	rows := []previewRow{
		{current: stale, inSync: true},
		{current: current, inSync: true},
		{current: current, inSync: true},
	}
	common, drifted := markDrift(rows)
	if !drifted {
		t.Fatal("differing rows were not reported as drifting")
	}
	if common.String() != "2.0.0" {
		t.Errorf("common = %s, want 2.0.0 (held by two of three rows)", common)
	}
	if rows[0].inSync {
		t.Error("the lone 1.0.0 row was not marked")
	}
	if !rows[1].inSync || !rows[2].inSync {
		t.Error("a majority row was marked as drifting")
	}
}

// A majority that is *lower* than the outlier still wins: the count decides,
// and precedence only breaks a tie. Otherwise one file bumped ahead of the rest
// would make every other file look stale.
func TestMarkDriftMajorityBeatsHigherVersion(t *testing.T) {
	common, drifted := markDrift([]previewRow{
		{current: version.Version{Major: 1}},
		{current: version.Version{Major: 1}},
		{current: version.Version{Major: 9}},
	})
	if !drifted {
		t.Fatal("differing rows were not reported as drifting")
	}
	if common.String() != "1.0.0" {
		t.Errorf("common = %s, want 1.0.0 (held by two of three rows)", common)
	}
}

// preview is reachable from the help system: the overview lists it, and both
// `incrmit help preview` and `incrmit preview -h` print its centralized help.
func TestPreviewHelp(t *testing.T) {
	if !strings.Contains(overviewHelp, "incrmit preview") {
		t.Errorf("overviewHelp does not list the preview command:\n%s", overviewHelp)
	}
	if !strings.Contains(overviewHelp, previewFlags) {
		t.Errorf("overviewHelp does not embed previewFlags verbatim:\n%s", overviewHelp)
	}
	if !strings.Contains(previewHelp, previewFlags) {
		t.Errorf("previewHelp does not embed previewFlags verbatim:\n%s", previewHelp)
	}
	for _, f := range []string{"--config", "--file", "--max-file-size"} {
		if !strings.Contains(previewHelp, f) {
			t.Errorf("previewHelp missing flag %q:\n%s", f, previewHelp)
		}
	}

	code, stdout, stderr := runHelpCmd("preview")
	if code != ExitOK || stdout != previewHelp || stderr != "" {
		t.Errorf("help preview = (%d, %q, %q), want (0, previewHelp, \"\")", code, stdout, stderr)
	}

	for _, flag := range []string{"-h", "--help"} {
		code, stdout, stderr := runMain(t, "", "preview", flag)
		if code != ExitOK || stdout != previewHelp || stderr != "" {
			t.Errorf("preview %s = (%d, %q, %q), want (0, previewHelp, \"\")", flag, code, stdout, stderr)
		}
	}
}
