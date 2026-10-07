package discovery

import (
	"os"
	"path/filepath"
	"testing"
)

// These tests give DiscoverWithLimit a config directory other than the scan
// root. Before Milestone 37 every path was relative to the root, and every
// command resolves a config's paths against the config's directory, so a
// config written anywhere but the scanned directory named the wrong files.
// Paths, and the paths ignore patterns are matched against, are now relative
// to the config's directory.

// configDirTree writes VERSION, sub/VERSION, sub/gen/VERSION, src/VERSION, and
// src/gen/VERSION under a new temp directory and returns it, with release/ as
// an empty directory for a config beside src.
func configDirTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, f := range []string{"VERSION", "sub/VERSION", "sub/gen/VERSION", "src/VERSION", "src/gen/VERSION"} {
		mustWriteNested(t, root, f, "1.2.3\n")
	}
	if err := os.Mkdir(filepath.Join(root, "release"), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestDiscoverPathsRelativeToConfigDir(t *testing.T) {
	for _, tc := range []struct {
		name, root, configDir string
		want                  []string
	}{
		{"config in the root", "sub", "sub", []string{"VERSION", "gen/VERSION"}},
		{"root below the config", "sub", ".", []string{"sub/VERSION", "sub/gen/VERSION"}},
		{"root two below the config", "sub/gen", ".", []string{"sub/gen/VERSION"}},
		{"config below the root", "sub", "sub/gen", []string{"../VERSION", "VERSION"}},
		{"siblings", "src", "release", []string{"../src/VERSION", "../src/gen/VERSION"}},
		{"config outside a nested root", "src/gen", "release", []string{"../src/gen/VERSION"}},
		// WalkDir visits a file root alone; it used to be recorded as ".".
		{"root is a file", "sub/VERSION", ".", []string{"sub/VERSION"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tree := configDirTree(t)
			got, err := DiscoverWithLimit(filepath.Join(tree, tc.root), filepath.Join(tree, tc.configDir), DefaultMaxScanBytes)
			if err != nil {
				t.Fatal(err)
			}
			if !equalStrings(paths(got), tc.want) {
				t.Errorf("paths = %q, want %q", paths(got), tc.want)
			}
		})
	}
}

// Ignore patterns live in the config, so they are matched against the paths
// the config lists: relative to its directory, whichever root is scanned. A
// bare pattern names a base name at any depth and is unaffected.
func TestDiscoverIgnoreRelativeToConfigDir(t *testing.T) {
	for _, tc := range []struct {
		name, root, configDir, pattern string
		want                           []string
	}{
		{"path from the config", "sub", ".", "sub/gen/", []string{"sub/VERSION"}},
		// Matched against the root, as it used to be, this pruned sub/gen.
		{"path from the root names nothing", "sub", ".", "gen/**", []string{"sub/VERSION", "sub/gen/VERSION"}},
		{"double star from the config", "sub", ".", "sub/**", nil},
		{"leading double star", "sub", ".", "**/gen/", []string{"sub/VERSION"}},
		{"bare name", "sub", ".", "gen", []string{"sub/VERSION"}},
		{"up and over", "src", "release", "../src/gen/", []string{"../src/VERSION"}},
		{"up and over, double star", "src", "release", "../src/gen/**", []string{"../src/VERSION"}},
		{"sibling pattern names nothing", "src", "release", "src/gen/", []string{"../src/VERSION", "../src/gen/VERSION"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tree := configDirTree(t)
			got, err := DiscoverWithLimit(filepath.Join(tree, tc.root), filepath.Join(tree, tc.configDir), DefaultMaxScanBytes, tc.pattern)
			if err != nil {
				t.Fatal(err)
			}
			if !equalStrings(paths(got), tc.want) {
				t.Errorf("paths = %q, want %q", paths(got), tc.want)
			}
		})
	}
}

// The directory being scanned is walked even when a pattern or a built-in
// name matches it: the patterns apply to what is inside it.
func TestDiscoverRootNeverPruned(t *testing.T) {
	tree := configDirTree(t)
	mustWriteNested(t, tree, "build/VERSION", "1.2.3\n")

	got, err := DiscoverWithLimit(filepath.Join(tree, "sub"), tree, DefaultMaxScanBytes, "sub/")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"sub/VERSION", "sub/gen/VERSION"}; !equalStrings(paths(got), want) {
		t.Errorf("pattern naming the root: paths = %q, want %q", paths(got), want)
	}

	got, err = DiscoverWithLimit(filepath.Join(tree, "build"), tree, DefaultMaxScanBytes)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"build/VERSION"}; !equalStrings(paths(got), want) {
		t.Errorf("built-in name for the root: paths = %q, want %q", paths(got), want)
	}
}

// Two spellings of one directory, one through a symbolic link, still give a
// plain relative path rather than one that climbs to / and back down. On macOS
// /tmp is a link to /private/tmp, and os.Getwd may report either, so a
// relative --output and an absolute --path can disagree this way.
func TestDiscoverConfigDirThroughLink(t *testing.T) {
	tree := configDirTree(t)
	alias := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(tree, alias); err != nil {
		t.Skipf("symlinks unsupported: %v", err)
	}

	for _, tc := range []struct {
		name, root, configDir string
		want                  []string
	}{
		{"root through the link", filepath.Join(alias, "sub"), tree, []string{"sub/VERSION", "sub/gen/VERSION"}},
		{"config through the link", filepath.Join(tree, "sub"), alias, []string{"sub/VERSION", "sub/gen/VERSION"}},
		{"siblings through the link", filepath.Join(alias, "src"), filepath.Join(tree, "release"), []string{"../src/VERSION", "../src/gen/VERSION"}},
		// A --dry-run may name an --output directory that does not exist yet:
		// the part that exists is resolved, and the rest kept.
		{"config not created yet", filepath.Join(tree, "src"), filepath.Join(alias, "new", "dir"), []string{"../../src/VERSION", "../../src/gen/VERSION"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := DiscoverWithLimit(tc.root, tc.configDir, DefaultMaxScanBytes)
			if err != nil {
				t.Fatal(err)
			}
			if !equalStrings(paths(got), tc.want) {
				t.Errorf("paths = %q, want %q", paths(got), tc.want)
			}
		})
	}

	for _, tc := range []struct {
		a, b string
		want bool
	}{
		{tree, alias, true},
		{filepath.Join(tree, "sub"), filepath.Join(alias, "sub", "gen", ".."), true},
		{filepath.Join(tree, "new"), filepath.Join(alias, "new"), true},
		{tree, filepath.Join(alias, "sub"), false},
	} {
		if got := SameDir(tc.a, tc.b); got != tc.want {
			t.Errorf("SameDir(%s, %s) = %v, want %v", tc.a, tc.b, got, tc.want)
		}
	}
}
