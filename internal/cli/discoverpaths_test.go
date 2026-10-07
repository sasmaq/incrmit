package cli

import (
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/sasmaq/incrmit/internal/config"
)

// These tests run discover with --path or --output pointing somewhere other
// than one directory, then bump the config it wrote. Before Milestone 37
// discover recorded each path relative to the scan root, while every command
// resolves a config path relative to the config's directory, so the two agreed
// only when the config was written into the directory that was scanned. Most
// trees hold a decoy with the same version where the root-relative path
// resolves: that is the silent case, where the bump rewrote the decoy, left the
// scanned file alone, and reported success. Paths are now written relative to
// the config's directory.

// discoverPathCase is a discover run from cwd (relative to the project root)
// with args, which writes the config at cfg (relative to the project root).
// paths are the `path` values that config must list, in order, and scanned
// are the files (relative to the project root) a bump of it must change; every
// other file must be left alone. inRoot says the config is written in the
// scanned directory, where the dry run's paths need no note.
type discoverPathCase struct {
	name    string
	tree    []string
	cwd     string
	args    func(root string) []string
	cfg     string
	paths   []string
	scanned []string
	inRoot  bool
}

// flags returns args unchanged, for a case whose flags do not need the
// project's absolute path.
func flags(args ...string) func(string) []string {
	return func(string) []string { return args }
}

func discoverPathCases() []discoverPathCase {
	return []discoverPathCase{
		{
			name: "default",
			tree: []string{"VERSION", "sub/VERSION"},
			args: flags(), cfg: "incrmit.toml",
			paths:   []string{"VERSION", "sub/VERSION"},
			scanned: []string{"VERSION", "sub/VERSION"},
			inRoot:  true,
		},
		{
			// The reported case: "VERSION" resolved to the root's decoy.
			name: "--path sub",
			tree: []string{"VERSION", "sub/VERSION"},
			args: flags("--path", "sub"), cfg: "incrmit.toml",
			paths:   []string{"sub/VERSION"},
			scanned: []string{"sub/VERSION"},
		},
		{
			name: "--path ./sub",
			tree: []string{"VERSION", "sub/VERSION"},
			args: flags("--path", "./sub"), cfg: "incrmit.toml",
			paths:   []string{"sub/VERSION"},
			scanned: []string{"sub/VERSION"},
		},
		{
			name: "--path ./",
			tree: []string{"VERSION", "sub/VERSION"},
			args: flags("--path", "./"), cfg: "incrmit.toml",
			paths:   []string{"VERSION", "sub/VERSION"},
			scanned: []string{"VERSION", "sub/VERSION"},
			inRoot:  true,
		},
		{
			// A file root used to be recorded as ".", a directory.
			name: "--path naming a file",
			tree: []string{"VERSION", "sub/VERSION"},
			args: flags("--path", "sub/VERSION"), cfg: "incrmit.toml",
			paths:   []string{"sub/VERSION"},
			scanned: []string{"sub/VERSION"},
		},
		{
			name: "absolute --path",
			tree: []string{"VERSION", "sub/VERSION"},
			args: func(root string) []string {
				return []string{"--path", filepath.Join(root, "sub")}
			},
			cfg:     "incrmit.toml",
			paths:   []string{"sub/VERSION"},
			scanned: []string{"sub/VERSION"},
		},
		{
			name: "absolute --path and --output",
			tree: []string{"VERSION", "sub/VERSION", "release/VERSION"},
			args: func(root string) []string {
				return []string{"--path", filepath.Join(root, "sub"), "-o", filepath.Join(root, "release", "incrmit.toml")}
			},
			cfg:     "release/incrmit.toml",
			paths:   []string{"../sub/VERSION"},
			scanned: []string{"sub/VERSION"},
		},
		{
			// The walk skips build/, so its VERSION is never listed, and the
			// root's "VERSION" resolved to it from build/incrmit.toml.
			name: "-o build/incrmit.toml",
			tree: []string{"VERSION", "build/VERSION"},
			args: flags("-o", "build/incrmit.toml"), cfg: "build/incrmit.toml",
			paths:   []string{"../VERSION"},
			scanned: []string{"VERSION"},
		},
		{
			// sub is scanned too, so its "sub/VERSION" resolved to
			// sub/sub/VERSION and the config failed to load.
			name: "-o sub/incrmit.toml",
			tree: []string{"VERSION", "sub/VERSION"},
			args: flags("-o", "sub/incrmit.toml"), cfg: "sub/incrmit.toml",
			paths:   []string{"../VERSION", "VERSION"},
			scanned: []string{"VERSION", "sub/VERSION"},
		},
		{
			// The README's two examples together: the scanned src/VERSION
			// was recorded as "VERSION", which resolved to release/VERSION.
			name: "siblings",
			tree: []string{"src/VERSION", "release/VERSION"},
			args: flags("--path", "src", "-o", "release/incrmit.toml"), cfg: "release/incrmit.toml",
			paths:   []string{"../src/VERSION"},
			scanned: []string{"src/VERSION"},
		},
		{
			name: "siblings nested",
			tree: []string{"VERSION", "src/app/VERSION", "src/VERSION", "ops/release/VERSION", "ops/release/app/VERSION"},
			args: flags("--path", "src", "-o", "ops/release/incrmit.toml"), cfg: "ops/release/incrmit.toml",
			paths:   []string{"../../src/VERSION", "../../src/app/VERSION"},
			scanned: []string{"src/VERSION", "src/app/VERSION"},
		},
		{
			// Run from below the project, scanning its parent, with the
			// config written where the command runs.
			name: "--path .. from sub",
			tree: []string{"VERSION", "sub/VERSION"},
			cwd:  "sub", args: flags("--path", ".."), cfg: "sub/incrmit.toml",
			paths:   []string{"../VERSION", "VERSION"},
			scanned: []string{"VERSION", "sub/VERSION"},
		},
		{
			// Run from below the project, with both flags naming its root.
			name: "from sub, both flags at the root",
			tree: []string{"VERSION", "sub/VERSION"},
			cwd:  "sub", args: flags("--path", "..", "-o", "../incrmit.toml"), cfg: "incrmit.toml",
			paths:   []string{"VERSION", "sub/VERSION"},
			scanned: []string{"VERSION", "sub/VERSION"},
			inRoot:  true,
		},
	}
}

// pathLine matches a `path = "..."` line of a generated config.
var pathLine = regexp.MustCompile(`(?m)^\s*path = "(.*)"$`)

// treeFiles returns the contents of every file under root except incrmit's
// own, keyed by its slash-form path from root.
func treeFiles(t *testing.T, root string) map[string]string {
	t.Helper()
	got := map[string]string{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		switch d.Name() {
		case config.DefaultPath, config.StateFileName, config.LockFileName:
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, p)
		got[filepath.ToSlash(rel)] = string(b)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return got
}

// A config discover writes lists each file relative to the config's
// directory, so a bump of it changes exactly the files the scan covered: not
// a decoy at the scan-root-relative path, and not nothing because the config
// fails to load. The dry run and the summary print those same paths.
func TestDiscoverPathsRelativeToConfig(t *testing.T) {
	for _, tc := range discoverPathCases() {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			for _, f := range tc.tree {
				p := filepath.Join(root, filepath.FromSlash(f))
				if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(p, []byte("1.0.0\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			cwd := filepath.Join(root, filepath.FromSlash(tc.cwd))
			cfgPath := filepath.Join(root, filepath.FromSlash(tc.cfg))
			args := append([]string{"discover"}, tc.args(root)...)

			code, stdout, stderr := runMain(t, cwd, append(args, "--dry-run")...)
			if code != ExitOK {
				t.Fatalf("dry run exit = %d, stderr = %q", code, stderr)
			}
			for _, p := range tc.paths {
				if !strings.Contains(stdout, "\n  "+p+":\n") {
					t.Errorf("dry run = %q, want it to list %s", stdout, p)
				}
			}
			if noted := strings.Contains(stdout, "\n  (paths relative to "); noted == tc.inRoot {
				t.Errorf("dry run = %q, want a note on what the paths are relative to: %v", stdout, !tc.inRoot)
			}

			code, stdout, stderr = runMain(t, cwd, args...)
			if code != ExitOK {
				t.Fatalf("discover exit = %d, stderr = %q", code, stderr)
			}
			for _, p := range tc.paths {
				if !strings.Contains(stdout, "\n  "+p+": 1.0.0\n") {
					t.Errorf("summary = %q, want it to list %s", stdout, p)
				}
			}
			body, err := os.ReadFile(cfgPath)
			if err != nil {
				t.Fatalf("reading the config discover wrote: %v", err)
			}
			var paths []string
			for _, m := range pathLine.FindAllStringSubmatch(string(body), -1) {
				paths = append(paths, m[1])
			}
			if !slices.Equal(paths, tc.paths) {
				t.Errorf("config paths = %q, want %q\n%s", paths, tc.paths, body)
			}

			before := treeFiles(t, root)
			if code, _, stderr := runMain(t, root, "-c", cfgPath); code != ExitOK {
				t.Fatalf("bump exit = %d, stderr = %q\nconfig:\n%s", code, stderr, body)
			}
			after := treeFiles(t, root)
			for _, f := range slices.Sorted(maps.Keys(before)) {
				want := before[f]
				if slices.Contains(tc.scanned, f) {
					want = "1.0.1\n"
				}
				if after[f] != want {
					t.Errorf("after the bump %s = %q, want %q", f, after[f], want)
				}
			}
		})
	}
}

// A dry run whose paths are not relative to the scanned directory says what
// they are relative to, ahead of the ignore rules it read from that config.
func TestDiscoverDryRunNotesConfigDir(t *testing.T) {
	dir := project(t, "", map[string]string{
		"src/VERSION": "1.0.0\n",
		"src/a.lock":  "2.0.0\n",
		"release/incrmit.toml": "ignore = [\"*.lock\"]\n\n" +
			"[[files]]\n  path = \"../src/VERSION\"\n  version = \"1.0.0\"\n",
	})

	code, stdout, stderr := runMain(t, dir, "discover", "--path", "src", "-o", "release/incrmit.toml", "--dry-run")
	if code != ExitOK {
		t.Fatalf("exit = %d, stderr = %q", code, stderr)
	}
	want := "Discovered 1 file(s) under src (no config written):\n" +
		"  (paths relative to release/incrmit.toml)\n" +
		"  (ignoring: *.lock)\n" +
		"  ../src/VERSION:\n" +
		"    L1: 1.0.0\n"
	if stdout != want {
		t.Errorf("stdout =\n%s\nwant\n%s", stdout, want)
	}
}

// The ignore list discover reads from --output is matched against the paths
// that config lists, so a pattern means the same thing whichever --path is
// scanned: sub/gen/ names sub/gen from a config in the project root.
func TestDiscoverIgnoreRelativeToConfig(t *testing.T) {
	for _, tc := range []struct {
		pattern string
		want    []string
	}{
		{"sub/gen/", []string{"sub/VERSION"}},
		// Relative to the scan root, as it used to be read, this was sub/gen.
		{"gen/**", []string{"sub/VERSION", "sub/gen/VERSION"}},
	} {
		t.Run(tc.pattern, func(t *testing.T) {
			dir := project(t, "ignore = [\""+tc.pattern+"\"]\n[[files]]\npath = \"VERSION\"\nversion = \"1.0.0\"\n", map[string]string{
				"VERSION":         "1.0.0\n",
				"sub/VERSION":     "1.0.0\n",
				"sub/gen/VERSION": "1.0.0\n",
			})

			code, _, stderr := runMain(t, dir, "discover", "--path", "sub")
			if code != ExitOK {
				t.Fatalf("exit = %d, stderr = %q", code, stderr)
			}
			cfg, err := config.Load(filepath.Join(dir, config.DefaultPath))
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, f := range cfg.Files {
				got = append(got, f.Path)
			}
			if !slices.Equal(got, tc.want) {
				t.Errorf("paths = %q, want %q", got, tc.want)
			}
			if !slices.Equal(cfg.Ignore, []string{tc.pattern}) {
				t.Errorf("ignore = %q, want it kept", cfg.Ignore)
			}
		})
	}
}

// A custom --output inside the scanned tree is found by the walk under its
// config-relative name, and still never listed as a target of itself.
func TestDiscoverExcludesOutputBelowRoot(t *testing.T) {
	for _, output := range []string{"sub/conf.cfg", "./sub/../sub/conf.cfg"} {
		t.Run(output, func(t *testing.T) {
			dir := project(t, "", map[string]string{
				"VERSION":      "1.0.0\n",
				"sub/conf.cfg": "[[files]]\npath = \"../VERSION\"\nversion = \"1.0.0\"\n",
			})

			code, _, stderr := runMain(t, dir, "discover", "-o", output)
			if code != ExitOK {
				t.Fatalf("exit = %d, stderr = %q", code, stderr)
			}
			cfg, err := config.Load(filepath.Join(dir, "sub", "conf.cfg"))
			if err != nil {
				t.Fatal(err)
			}
			if len(cfg.Files) != 1 || cfg.Files[0].Path != "../VERSION" {
				t.Errorf("config files = %+v, want only ../VERSION", cfg.Files)
			}
		})
	}
}
