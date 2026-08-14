package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// These tests cover semver prerelease and build metadata end to end (Milestone
// 28). They started life pinning the old, incorrect handling: version.Parse
// understood only a bare MAJOR.MINOR.PATCH while the token matcher stopped at
// the numeric core, so a bump rewrote the "1.2.3" inside "1.2.3-rc.1" and left
// the "-rc.1" dangling on a version it no longer belonged to. The expectations
// below are the corrected ones — the whole token is matched, and a component
// bump drops both suffixes.

func bumpOneFile(t *testing.T, body string, args ...string) (int, string, string, string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "VERSION")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr := runMain(t, "", append([]string{"--file", path}, args...)...)
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return code, stdout, stderr, string(got)
}

// A patch bump off a prerelease is the next patch release: the prerelease and
// the build metadata are both dropped, and the rest of the line is untouched.
func TestBumpDropsPrereleaseAndBuild(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"prerelease", "version = \"1.2.3-rc.1\"\n", "version = \"1.2.4\"\n"},
		{"build metadata", "ver=1.2.3+build.7\n", "ver=1.2.4\n"},
		{"both", "v = v2.0.0-beta.1+exp.sha.5114f85\n", "v = v2.0.1\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, _, stderr, got := bumpOneFile(t, tc.in)
			if code != ExitOK {
				t.Fatalf("exit = %d, want %d (stderr %q)", code, ExitOK, stderr)
			}
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// A minor or major bump drops the suffixes the same way.
func TestBumpDropsPrereleaseAcrossComponents(t *testing.T) {
	cases := []struct {
		flag string
		want string
	}{
		{"--minor", "1.3.0\n"},
		{"--major", "2.0.0\n"},
	}
	for _, tc := range cases {
		t.Run(tc.flag, func(t *testing.T) {
			code, _, stderr, got := bumpOneFile(t, "1.2.3-rc.1\n", tc.flag)
			if code != ExitOK {
				t.Fatalf("exit = %d (stderr %q)", code, stderr)
			}
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// The dry-run summary shows the whole token on both sides, so what the preview
// promises is what a real bump writes.
func TestDryRunShowsWholePrereleaseToken(t *testing.T) {
	code, stdout, stderr, got := bumpOneFile(t, "1.2.3-rc.1\n", "--dry-run")
	if code != ExitOK {
		t.Fatalf("exit = %d (stderr %q)", code, stderr)
	}
	if !strings.Contains(stdout, "1.2.3-rc.1 -> 1.2.4") {
		t.Errorf("stdout = %q, want the full token in the preview", stdout)
	}
	if got != "1.2.3-rc.1\n" {
		t.Errorf("dry run modified the file: %q", got)
	}
}

// --release promotes a prerelease to the release it names, without touching the
// numbers. Build metadata goes with it.
func TestReleaseFlagPromotesPrerelease(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"prerelease", "1.2.3-rc.1\n", "1.2.3\n"},
		{"with build", "v1.2.3-rc.1+build.7\n", "v1.2.3\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, stdout, stderr, got := bumpOneFile(t, tc.in, "--release")
			if code != ExitOK {
				t.Fatalf("exit = %d (stderr %q)", code, stderr)
			}
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
			if !strings.Contains(stdout, "release promotion") {
				t.Errorf("stdout = %q, want the summary to name the promotion", stdout)
			}
		})
	}
}

// --release on a version that has no prerelease is a usage error: there is
// nothing to promote, and silently doing nothing would look like a success.
func TestReleaseFlagRejectsPlainVersion(t *testing.T) {
	code, _, stderr, got := bumpOneFile(t, "1.2.3\n", "--release")
	if code != ExitUsage {
		t.Fatalf("exit = %d, want %d (stderr %q)", code, ExitUsage, stderr)
	}
	if !strings.Contains(stderr, "1.2.3") || !strings.Contains(stderr, "--release") {
		t.Errorf("stderr = %q, want the offending version and the flag named", stderr)
	}
	if got != "1.2.3\n" {
		t.Errorf("file was modified: %q", got)
	}
}

// --pre starts a prerelease off a release (bumping the patch first, since the
// release it previews cannot be the one already published) and advances one
// that is already running.
func TestPreFlagStartsAndAdvances(t *testing.T) {
	cases := []struct {
		name string
		in   string
		args []string
		want string
	}{
		{"start from release", "1.2.3\n", []string{"--pre", "rc"}, "1.2.4-rc.1\n"},
		{"advance same series", "1.2.4-rc.1\n", []string{"--pre", "rc"}, "1.2.4-rc.2\n"},
		{"advance past nine", "1.2.4-rc.9\n", []string{"--pre", "rc"}, "1.2.4-rc.10\n"},
		{"switch series", "1.2.4-beta.2\n", []string{"--pre", "rc"}, "1.2.4-rc.1\n"},
		{"with minor", "1.2.3\n", []string{"--minor", "--pre", "rc"}, "1.3.0-rc.1\n"},
		{"with major", "1.2.3\n", []string{"--major", "--pre", "rc"}, "2.0.0-rc.1\n"},
		{"minor off a prerelease", "1.2.4-rc.1\n", []string{"--minor", "--pre", "rc"}, "1.3.0-rc.1\n"},
		{"prefix preserved", "v1.2.3\n", []string{"--pre", "beta"}, "v1.2.4-beta.1\n"},
		{"build dropped", "1.2.4-rc.1+build.7\n", []string{"--pre", "rc"}, "1.2.4-rc.2\n"},
		{"shorthand", "1.2.3\n", []string{"-e", "rc"}, "1.2.4-rc.1\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, _, stderr, got := bumpOneFile(t, tc.in, tc.args...)
			if code != ExitOK {
				t.Fatalf("exit = %d (stderr %q)", code, stderr)
			}
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// Meaningless or malformed flag combinations exit 2 and write nothing.
func TestPrereleaseFlagUsageErrors(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string // substring expected on stderr
	}{
		{"release and pre", []string{"--release", "--pre", "rc"}, "mutually exclusive"},
		{"release with major", []string{"--release", "--major"}, "changes no numbers"},
		{"empty pre id", []string{"--pre", ""}, "invalid --pre"},
		{"bad pre id", []string{"--pre", "rc_1"}, "invalid --pre"},
		{"leading-zero pre id", []string{"--pre", "rc.01"}, "invalid --pre"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, stdout, stderr, got := bumpOneFile(t, "1.2.3-rc.1\n", tc.args...)
			if code != ExitUsage {
				t.Fatalf("exit = %d, want %d (stderr %q)", code, ExitUsage, stderr)
			}
			if !strings.Contains(stderr, tc.want) {
				t.Errorf("stderr = %q, want it to contain %q", stderr, tc.want)
			}
			if stdout != "" {
				t.Errorf("stdout = %q, want empty on a usage error", stdout)
			}
			if got != "1.2.3-rc.1\n" {
				t.Errorf("file was modified: %q", got)
			}
		})
	}
}

// discover records the whole token, so a prerelease target survives a
// regeneration and the config says exactly what is written in the file.
func TestDiscoverRecordsWholePrereleaseToken(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "VERSION"), []byte("1.2.3-rc.1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	code, _, stderr := runMain(t, dir, "discover")
	if code != ExitOK {
		t.Fatalf("exit = %d (stderr %q)", code, stderr)
	}
	cfg, err := os.ReadFile(filepath.Join(dir, "incrmit.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(cfg), "version = \"1.2.3\"\n  prerelease = \"rc.1\"") {
		t.Errorf("config = %q, want the version and its prerelease recorded", cfg)
	}
}

// A config pinning a prerelease must not match the bare release elsewhere in
// the same file: the recorded token is matched whole, so only the entry's own
// occurrence is rewritten.
func TestConfiguredPrereleaseDoesNotMatchBareVersion(t *testing.T) {
	dir := project(t, `
[[files]]
  path = "notes.md"
  version = "1.2.3-rc.1"
`, map[string]string{
		"notes.md": "current: 1.2.3-rc.1\nreleased: 1.2.3\n",
	})

	code, _, stderr := runMain(t, dir, "--pre", "rc")
	if code != ExitOK {
		t.Fatalf("exit = %d (stderr %q)", code, stderr)
	}
	got, err := os.ReadFile(filepath.Join(dir, "notes.md"))
	if err != nil {
		t.Fatal(err)
	}
	if want := "current: 1.2.3-rc.2\nreleased: 1.2.3\n"; string(got) != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// The mirror of the case above: an entry pinning the bare release leaves a
// prerelease of the same numbers alone.
func TestConfiguredBareVersionDoesNotMatchPrerelease(t *testing.T) {
	dir := project(t, `
[[files]]
  path = "notes.md"
  version = "1.2.3"
`, map[string]string{
		"notes.md": "released: 1.2.3\npreview: 1.2.3-rc.1\n",
	})

	code, _, stderr := runMain(t, dir)
	if code != ExitOK {
		t.Fatalf("exit = %d (stderr %q)", code, stderr)
	}
	got, err := os.ReadFile(filepath.Join(dir, "notes.md"))
	if err != nil {
		t.Fatal(err)
	}
	if want := "released: 1.2.4\npreview: 1.2.3-rc.1\n"; string(got) != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// A full round trip: start a prerelease, iterate on it, then promote it, with
// the config and the file staying in step throughout.
func TestPrereleaseLifecycleThroughConfig(t *testing.T) {
	dir := project(t, `
[[files]]
  path = "VERSION"
  version = "1.2.3"
`, map[string]string{"VERSION": "1.2.3\n"})

	// wantCfg is the entry the config must hold after each step: starting or
	// advancing a prerelease records it in its own key, and promoting or bumping
	// takes that key back out.
	steps := []struct {
		args    []string
		want    string
		wantCfg string
	}{
		{[]string{"--pre", "rc"}, "1.2.4-rc.1\n", "version = \"1.2.4\"\n  prerelease = \"rc.1\"\n"},
		{[]string{"--pre", "rc"}, "1.2.4-rc.2\n", "version = \"1.2.4\"\n  prerelease = \"rc.2\"\n"},
		{[]string{"--release"}, "1.2.4\n", "version = \"1.2.4\"\n"},
		{nil, "1.2.5\n", "version = \"1.2.5\"\n"},
	}
	for _, step := range steps {
		code, _, stderr := runMain(t, dir, step.args...)
		if code != ExitOK {
			t.Fatalf("%v: exit = %d (stderr %q)", step.args, code, stderr)
		}
		got, err := os.ReadFile(filepath.Join(dir, "VERSION"))
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != step.want {
			t.Fatalf("%v: file = %q, want %q", step.args, got, step.want)
		}
		cfg, err := os.ReadFile(filepath.Join(dir, "incrmit.toml"))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasSuffix(string(cfg), step.wantCfg) {
			t.Fatalf("%v: config out of step with the file: got %q, want it to end with %q", step.args, cfg, step.wantCfg)
		}
	}
}

// undo reverts a prerelease step like any other bump, restoring the exact token.
func TestUndoRestoresPrereleaseToken(t *testing.T) {
	dir := project(t, `
[[files]]
  path = "VERSION"
  version = "1.2.3-rc.1"
`, map[string]string{"VERSION": "1.2.3-rc.1\n"})

	if code, _, stderr := runMain(t, dir, "--release"); code != ExitOK {
		t.Fatalf("bump: exit = %d (stderr %q)", code, stderr)
	}
	if code, _, stderr := runMain(t, dir, "undo"); code != ExitOK {
		t.Fatalf("undo: exit = %d (stderr %q)", code, stderr)
	}
	got, err := os.ReadFile(filepath.Join(dir, "VERSION"))
	if err != nil {
		t.Fatal(err)
	}
	if want := "1.2.3-rc.1\n"; string(got) != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// A release filename or download URL is not a prerelease. A hyphen is a legal
// prerelease character, so "incrmit-1.2.3-linux-amd64.tar.gz" would otherwise be
// read as 1.2.3 with the prerelease "linux-amd64.tar.gz" — and bumping it would
// rewrite the whole token, silently reducing the line to "incrmit-1.2.4".
func TestBumpPreservesReleaseFilenames(t *testing.T) {
	body := "" +
		"curl -O https://x/releases/download/v1.2.3/incrmit-1.2.3-linux-amd64.tar.gz\n" +
		"tar xzf incrmit-1.2.3-linux-amd64.tar.gz\n"
	want := "" +
		"curl -O https://x/releases/download/v1.2.4/incrmit-1.2.4-linux-amd64.tar.gz\n" +
		"tar xzf incrmit-1.2.4-linux-amd64.tar.gz\n"

	dir := project(t, `
[[files]]
  path = "install.sh"
  version = "1.2.3"

[[files]]
  path = "install.sh"
  version = "v1.2.3"
`, map[string]string{"install.sh": body})

	code, _, stderr := runMain(t, dir)
	if code != ExitOK {
		t.Fatalf("exit = %d (stderr %q)", code, stderr)
	}
	got, err := os.ReadFile(filepath.Join(dir, "install.sh"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// The full prerelease cycle against a file where the version also lives inside a
// download URL and a release filename. Because the config records the prerelease
// in its own key, every occurrence stays in step: the filename carries the
// prerelease while one is running and loses it on promotion, rather than being
// left behind at -rc.1.
func TestPrereleaseLifecycleInsideFilenames(t *testing.T) {
	body := "" +
		"VERSION=1.2.3\n" +
		"curl -O https://x/releases/download/v1.2.3/incrmit-1.2.3-linux-amd64.tar.gz\n"

	dir := project(t, `
[[files]]
  path = "install.sh"
  version = "1.2.3"

[[files]]
  path = "install.sh"
  version = "v1.2.3"
`, map[string]string{"install.sh": body})

	steps := []struct {
		args []string
		want string
	}{
		{
			[]string{"--pre", "rc"},
			"VERSION=1.2.4-rc.1\n" +
				"curl -O https://x/releases/download/v1.2.4-rc.1/incrmit-1.2.4-rc.1-linux-amd64.tar.gz\n",
		},
		{
			[]string{"--pre", "rc"},
			"VERSION=1.2.4-rc.2\n" +
				"curl -O https://x/releases/download/v1.2.4-rc.2/incrmit-1.2.4-rc.2-linux-amd64.tar.gz\n",
		},
		{
			[]string{"--release"},
			"VERSION=1.2.4\n" +
				"curl -O https://x/releases/download/v1.2.4/incrmit-1.2.4-linux-amd64.tar.gz\n",
		},
		{
			nil,
			"VERSION=1.2.5\n" +
				"curl -O https://x/releases/download/v1.2.5/incrmit-1.2.5-linux-amd64.tar.gz\n",
		},
	}
	for _, step := range steps {
		code, _, stderr := runMain(t, dir, step.args...)
		if code != ExitOK {
			t.Fatalf("%v: exit = %d (stderr %q)", step.args, code, stderr)
		}
		got, err := os.ReadFile(filepath.Join(dir, "install.sh"))
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != step.want {
			t.Fatalf("%v: got %q, want %q", step.args, got, step.want)
		}
	}
}

// A prerelease recorded in the config makes the bump semver-correct even where
// the version sits inside a filename: promoting drops the suffix there too,
// which the token matcher alone cannot do (it cannot tell "-rc.1" from
// "-linux-amd64").
func TestReleasePromotesInsideFilename(t *testing.T) {
	dir := project(t, `
[[files]]
  path = "assets.txt"
  version = "1.2.3"
  prerelease = "rc.1"
`, map[string]string{"assets.txt": "app-1.2.3-rc.1.zip\n"})

	code, _, stderr := runMain(t, dir, "--release")
	if code != ExitOK {
		t.Fatalf("exit = %d (stderr %q)", code, stderr)
	}
	got, err := os.ReadFile(filepath.Join(dir, "assets.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if want := "app-1.2.3.zip\n"; string(got) != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// An older config that spells the whole token in `version` keeps working, and
// the rewrite after the bump moves it to the split form.
func TestInlineTokenConfigIsMigratedOnBump(t *testing.T) {
	dir := project(t, `
[[files]]
  path = "VERSION"
  version = "1.2.3-rc.1"
`, map[string]string{"VERSION": "1.2.3-rc.1\n"})

	code, _, stderr := runMain(t, dir, "--pre", "rc")
	if code != ExitOK {
		t.Fatalf("exit = %d (stderr %q)", code, stderr)
	}
	got, err := os.ReadFile(filepath.Join(dir, "VERSION"))
	if err != nil {
		t.Fatal(err)
	}
	if want := "1.2.3-rc.2\n"; string(got) != want {
		t.Errorf("file = %q, want %q", got, want)
	}
	cfg, err := os.ReadFile(filepath.Join(dir, "incrmit.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(cfg), "version = \"1.2.3\"\n  prerelease = \"rc.2\"") {
		t.Errorf("config = %q, want the split form after the rewrite", cfg)
	}
}
