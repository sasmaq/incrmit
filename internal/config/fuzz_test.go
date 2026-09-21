package config

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// fuzzDir returns a directory holding a real target file for a config to point
// at, so validation can succeed and the fuzzer reaches the code past it rather
// than stopping at "target does not exist" on every input.
func fuzzDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "app.txt"), []byte("1.2.3\n"), 0o644); err != nil {
		t.Fatalf("writing target: %v", err)
	}
	return dir
}

// FuzzLoad feeds arbitrary bytes to the config loader. The config is trusted
// input — the user (or `incrmit discover`) writes it — but a truncated,
// half-merged, or hand-mangled file is not the same thing as a hostile one, and
// the difference a user should see is an error naming the problem, never a
// panic or a config that loaded into a shape no command expects.
//
// Every failure path here is required to be a "config: ..." error, because that
// prefix is what the CLI's exit-2 usage handling keys the message off; an error
// escaping from the TOML decoder or the filesystem in some other wording would
// reach the user unexplained.
func FuzzLoad(f *testing.F) {
	f.Add([]byte("[[files]]\n  path = \"app.txt\"\n  version = \"1.2.3\"\n"))
	f.Add([]byte("ignore = [\"vendor\"]\n\n[[files]]\n  path = \"app.txt\"\n"))
	f.Add([]byte("[[files]]\n  path = \"app.txt\"\n  version = \"1.2.3-rc.1\"\n"))
	f.Add([]byte("[[files]]\n  path = \"app.txt\"\n  version = \"1.2.3\"\n  prerelease = \"rc.1\"\n"))
	f.Add([]byte("[[files]]\n  path = \"app.txt\"\n  version = \"1.2.3+build.7\"\n"))
	f.Add([]byte("[[files]]\n  path = \"app.txt\"\n  version = \"not-a-version\"\n"))
	f.Add([]byte("[[files]]\n  path = \"missing.txt\"\n"))
	f.Add([]byte("[[files]]\n  path = \"\"\n"))
	f.Add([]byte("files = 3\n"))
	f.Add([]byte("ignore = [\"\"]\n[[files]]\n  path = \"app.txt\"\n"))
	f.Add([]byte(""))
	f.Add([]byte("\x00\xff\xfe"))

	f.Fuzz(func(t *testing.T, data []byte) {
		// Work here is linear in the number of [[files]] entries and each one
		// costs a stat, so a megabyte of repeated entries is a single execution
		// that runs for seconds. Left unbounded it starves the run: the engine
		// spends a 30-second budget on a handful of enormous inputs instead of
		// the small structural ones where a parsing bug actually lives.
		const maxInput = 16 << 10
		if len(data) > maxInput {
			return
		}

		dir := fuzzDir(t)
		path := filepath.Join(dir, DefaultPath)
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatalf("writing config: %v", err)
		}

		// LoadIgnore is deliberately lenient — discover calls it on a file it is
		// about to overwrite — so garbage must come back as an empty list, not
		// an error.
		if _, err := LoadIgnore(path); err != nil {
			t.Fatalf("LoadIgnore(%q) on %q: %v", path, data, err)
		}

		cfg, err := Load(path)
		if err != nil {
			if !strings.HasPrefix(err.Error(), "config: ") {
				t.Errorf("Load error %q does not name the config; input %q", err, data)
			}
			if cfg != nil {
				t.Errorf("Load returned both a config and an error for %q", data)
			}
			return
		}

		// A config that loaded is one every command will now act on, so the
		// guarantees Validate makes must actually hold on the returned value.
		if len(cfg.Files) == 0 {
			t.Fatalf("Load accepted a config with no files: %q", data)
		}
		for i, fe := range cfg.Files {
			if fe.Path == "" {
				t.Errorf("Load accepted files[%d] with an empty path: %q", i, data)
			}
			if fe.Version == "" && (fe.Prerelease != "" || fe.Build != "") {
				t.Errorf("Load accepted files[%d] pinning a prerelease/build with no version: %q", i, data)
			}
			// normalizeVersions splits an inline suffix out of `version`, so no
			// loaded entry may still carry one there while the keys exist.
			if strings.ContainsAny(fe.Version, "+") && fe.Build != "" {
				t.Errorf("Load accepted files[%d] with build metadata in both places: %q", i, data)
			}
		}

		// What loads must also marshal, and marshal back into itself: the bump
		// commands rewrite the config they just read, and a shape that survives
		// loading but not writing would lose the user's file.
		out, err := Marshal(cfg)
		if err != nil {
			t.Fatalf("Marshal of a config that loaded from %q failed: %v", data, err)
		}
		rewritten := filepath.Join(dir, "rewritten.toml")
		if err := os.WriteFile(rewritten, out, 0o644); err != nil {
			t.Fatalf("writing marshaled config: %v", err)
		}
		again, err := Load(rewritten)
		if err != nil {
			t.Fatalf("config marshaled from %q does not load again: %v\n--- marshaled ---\n%s", data, err, out)
		}

		// The targets must come back exactly: they are what every command acts
		// on, and a bump that rewrote the config must not have moved, dropped,
		// or re-spelled one.
		if !reflect.DeepEqual(cfg.Files, again.Files) {
			t.Errorf("round trip changed the targets\n--- input ---\n%q\n--- loaded ---\n%#v\n--- reloaded ---\n%#v", data, cfg.Files, again.Files)
		}

		// The whole file is compared as the bytes it marshals to rather than as
		// a struct, because the two are not the same question. `ignore = []`
		// loads as an empty slice and marshals back out as nothing at all
		// (the key is omitempty), which reloads as nil: a different Go value
		// for the same config. What has to hold is that rewriting the file a
		// second time changes nothing, so a bump is not followed by a silent
		// reformat of the user's config.
		outAgain, err := Marshal(again)
		if err != nil {
			t.Fatalf("Marshal of the reloaded config from %q failed: %v", data, err)
		}
		if !bytes.Equal(out, outAgain) {
			t.Errorf("marshaling is not stable across a reload\n--- input ---\n%q\n--- first ---\n%s\n--- second ---\n%s", data, out, outAgain)
		}
	})
}
