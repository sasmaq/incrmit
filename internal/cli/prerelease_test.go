package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// These tests pin the CURRENT, INCORRECT handling of semver prerelease and
// build metadata, as the first task of Milestone 28 asks. They are not a
// statement of desired behavior.
//
// version.Parse only understands a bare MAJOR.MINOR.PATCH, but the token
// matcher in internal/files (`\b[vV]?\d+(?:\.\d+)+\b`) matches the numeric core
// inside a larger token. The suffix is therefore left dangling on a bumped
// version instead of the input being rejected or the prerelease being handled:
// a patch bump off 1.2.3-rc.1 is 1.2.3 (promote) or 1.2.3-rc.2 (iterate) under
// semver 2.0.0, never 1.2.4-rc.1.
//
// When Milestone 28 lands, these expectations must be REPLACED with the correct
// ones. Do not delete them to make a change pass.

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

func TestBumpCurrentlyMishandlesPrerelease(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string // current (wrong) output; see the file comment
	}{
		{"prerelease", "version = \"1.2.3-rc.1\"\n", "version = \"1.2.4-rc.1\"\n"},
		{"build metadata", "ver=1.2.3+build.7\n", "ver=1.2.4+build.7\n"},
		{"both", "v = v2.0.0-beta.1+exp.sha.5114f85\n", "v = v2.0.1-beta.1+exp.sha.5114f85\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, _, stderr, got := bumpOneFile(t, tc.in)
			if code != ExitOK {
				t.Fatalf("exit = %d, want %d (stderr %q)", code, ExitOK, stderr)
			}
			if got != tc.want {
				t.Errorf("got %q, want %q (current behavior; Milestone 28 changes this)", got, tc.want)
			}
		})
	}
}

// A minor or major bump loses the same way: the suffix rides along instead of
// being dropped.
func TestBumpCurrentlyKeepsPrereleaseAcrossComponents(t *testing.T) {
	cases := []struct {
		flag string
		want string
	}{
		{"--minor", "1.3.0-rc.1\n"},
		{"--major", "2.0.0-rc.1\n"},
	}
	for _, tc := range cases {
		t.Run(tc.flag, func(t *testing.T) {
			code, _, stderr, got := bumpOneFile(t, "1.2.3-rc.1\n", tc.flag)
			if code != ExitOK {
				t.Fatalf("exit = %d (stderr %q)", code, stderr)
			}
			if got != tc.want {
				t.Errorf("got %q, want %q (current behavior; Milestone 28 changes this)", got, tc.want)
			}
		})
	}
}

// The dry-run summary reports only the numeric core, so the preview does not
// reveal that the suffix will be left behind. Worth pinning: it is the output a
// user checks before committing to a bump.
func TestDryRunCurrentlyHidesPrereleaseSuffix(t *testing.T) {
	code, stdout, stderr, got := bumpOneFile(t, "1.2.3-rc.1\n", "--dry-run")
	if code != ExitOK {
		t.Fatalf("exit = %d (stderr %q)", code, stderr)
	}
	if !strings.Contains(stdout, "1.2.3 -> 1.2.4") {
		t.Errorf("stdout = %q, want the bare-core preview the tool prints today", stdout)
	}
	if got != "1.2.3-rc.1\n" {
		t.Errorf("dry run modified the file: %q", got)
	}
}

// A file whose only version carries a prerelease suffix is discovered as its
// bare core, so the generated config records a version that is not the token in
// the file. Pinning this makes the config-side half of Milestone 28 visible.
func TestDiscoverCurrentlyRecordsBareCoreOfPrerelease(t *testing.T) {
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
	if !strings.Contains(string(cfg), "version = \"1.2.3\"") {
		t.Errorf("config = %q, want the bare core it records today", cfg)
	}
	if strings.Contains(string(cfg), "rc.1") {
		t.Errorf("config unexpectedly kept the prerelease; Milestone 28 may already be done: %q", cfg)
	}
}
