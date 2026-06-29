package files

import (
	"errors"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sasmaq/incrmit/internal/version"
)

// update regenerates the .golden files from current behaviour:
//
//	go test ./internal/files/ -update
var update = flag.Bool("update", false, "regenerate golden files")

func TestSetVersionGolden(t *testing.T) {
	// Each case bumps 1.2.3 -> 2.0.0 (major) so the change is easy to eyeball
	// across formats while still exercising a multi-component rewrite.
	newVer := version.Version{Major: 2, Minor: 0, Patch: 0}
	cases := []string{
		"VERSION",
		"package.json",
		"pyproject.toml",
		"version.go",
	}
	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			inPath := filepath.Join("testdata", name+".input")
			goldenPath := filepath.Join("testdata", name+".golden")

			in, err := os.ReadFile(inPath)
			if err != nil {
				t.Fatalf("read input: %v", err)
			}

			got, err := SetVersion(in, newVer)
			if err != nil {
				t.Fatalf("SetVersion: %v", err)
			}

			if *update {
				if err := os.WriteFile(goldenPath, got, 0o644); err != nil {
					t.Fatalf("write golden: %v", err)
				}
			}

			want, err := os.ReadFile(goldenPath)
			if err != nil {
				t.Fatalf("read golden (run with -update to create it): %v", err)
			}
			if string(got) != string(want) {
				t.Errorf("golden mismatch for %s\n--- got ---\n%s\n--- want ---\n%s", name, got, want)
			}

			// Only the version token may differ between input and output.
			assertOnlyVersionChanged(t, in, got, "1.2.3", newVer.String())
		})
	}
}

// assertOnlyVersionChanged verifies that replacing newToken back with oldToken
// in the output reproduces the input exactly, proving nothing else changed.
func assertOnlyVersionChanged(t *testing.T, in, out []byte, oldToken, newToken string) {
	t.Helper()
	reverted := strings.ReplaceAll(string(out), newToken, oldToken)
	if reverted != string(in) {
		t.Errorf("more than the version changed\n--- input ---\n%s\n--- reverted output ---\n%s", in, reverted)
	}
}

func TestFindVersion(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want version.Version
	}{
		{"plain", "1.2.3", version.Version{Major: 1, Minor: 2, Patch: 3}},
		{"with-newline", "1.2.3\n", version.Version{Major: 1, Minor: 2, Patch: 3}},
		{"quoted", `version = "4.5.6"`, version.Version{Major: 4, Minor: 5, Patch: 6}},
		{"repeated-same", "1.2.3 ... 1.2.3", version.Version{Major: 1, Minor: 2, Patch: 3}},
		{"surrounded-text", "Release 10.0.42 is out", version.Version{Major: 10, Minor: 0, Patch: 42}},
		{"v-prefix", "tag v1.2.3 here", version.Version{Major: 1, Minor: 2, Patch: 3, Prefix: "v"}},
		{"V-prefix", "V2.0.1", version.Version{Major: 2, Minor: 0, Patch: 1, Prefix: "V"}},
		{"repeated-same-v", "v1.2.3 ... v1.2.3", version.Version{Major: 1, Minor: 2, Patch: 3, Prefix: "v"}},
		{"rev-near-miss-ignored", "rev1.2.3 and 4.5.6", version.Version{Major: 4, Minor: 5, Patch: 6}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := FindVersion([]byte(tt.in))
			if err != nil {
				t.Fatalf("FindVersion(%q) error: %v", tt.in, err)
			}
			if got != tt.want {
				t.Errorf("FindVersion(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestFindVersionNoVersion(t *testing.T) {
	for _, in := range []string{"", "no version here", "1.2", "year 2024", "v1"} {
		if _, err := FindVersion([]byte(in)); !errors.Is(err, ErrNoVersion) {
			t.Errorf("FindVersion(%q) error = %v, want ErrNoVersion", in, err)
		}
	}
}

func TestFindVersionAmbiguous(t *testing.T) {
	in := []byte(`"version": "1.2.3", "dep": "4.5.6"`)
	_, err := FindVersion(in)
	var ae *AmbiguousError
	if !errors.As(err, &ae) {
		t.Fatalf("FindVersion error = %v, want *AmbiguousError", err)
	}
	if len(ae.Versions) != 2 {
		t.Errorf("AmbiguousError.Versions = %v, want 2 entries", ae.Versions)
	}
}

func TestSetVersionRepeatedToken(t *testing.T) {
	in := []byte("badge: 1.2.3\nheader: 1.2.3\n")
	out, err := SetVersion(in, version.Version{Major: 1, Minor: 2, Patch: 4})
	if err != nil {
		t.Fatalf("SetVersion: %v", err)
	}
	want := "badge: 1.2.4\nheader: 1.2.4\n"
	if string(out) != want {
		t.Errorf("SetVersion = %q, want %q", out, want)
	}
}

func TestSetKnownVersion(t *testing.T) {
	// A file with several distinct versions; only the known one is rewritten.
	in := []byte("title 0.1.0\nexample 1.2.3 -> 1.2.4\n")
	out, err := SetKnownVersion(in,
		version.Version{Major: 0, Minor: 1, Patch: 0},
		version.Version{Major: 0, Minor: 2, Patch: 0})
	if err != nil {
		t.Fatalf("SetKnownVersion: %v", err)
	}
	want := "title 0.2.0\nexample 1.2.3 -> 1.2.4\n"
	if string(out) != want {
		t.Errorf("SetKnownVersion = %q, want %q", out, want)
	}
}

func TestSetKnownVersionAllOccurrences(t *testing.T) {
	in := []byte("a 1.2.3 b 1.2.3 c 4.5.6\n")
	out, err := SetKnownVersion(in,
		version.Version{Major: 1, Minor: 2, Patch: 3},
		version.Version{Major: 1, Minor: 2, Patch: 4})
	if err != nil {
		t.Fatalf("SetKnownVersion: %v", err)
	}
	if want := "a 1.2.4 b 1.2.4 c 4.5.6\n"; string(out) != want {
		t.Errorf("SetKnownVersion = %q, want %q", out, want)
	}
}

// A "v"-prefixed token and its bare form are distinct version tokens, so a file
// containing both is ambiguous rather than a repeated single version.
func TestFindVersionPrefixedAndBareAreAmbiguous(t *testing.T) {
	in := []byte("docs say v1.2.3 but file has 1.2.3\n")
	_, err := FindVersion(in)
	var ae *AmbiguousError
	if !errors.As(err, &ae) {
		t.Fatalf("FindVersion error = %v, want *AmbiguousError", err)
	}
}

// SetVersion preserves the original "v" prefix when rewriting the token.
func TestSetVersionPreservesPrefix(t *testing.T) {
	in := []byte("release = \"v1.2.3\"\n")
	cur, err := FindVersion(in)
	if err != nil {
		t.Fatalf("FindVersion: %v", err)
	}
	out, err := SetVersion(in, cur.BumpMinor())
	if err != nil {
		t.Fatalf("SetVersion: %v", err)
	}
	if want := "release = \"v1.3.0\"\n"; string(out) != want {
		t.Errorf("SetVersion = %q, want %q", out, want)
	}
}

// SetKnownVersion only rewrites the exact written token: a bare known version is
// not matched against a "v"-prefixed occurrence and vice versa.
func TestSetKnownVersionDistinguishesPrefix(t *testing.T) {
	in := []byte("bare 1.2.3 and tag v1.2.3\n")
	out, err := SetKnownVersion(in,
		version.Version{Major: 1, Minor: 2, Patch: 3, Prefix: "v"},
		version.Version{Major: 1, Minor: 2, Patch: 4, Prefix: "v"})
	if err != nil {
		t.Fatalf("SetKnownVersion: %v", err)
	}
	if want := "bare 1.2.3 and tag v1.2.4\n"; string(out) != want {
		t.Errorf("SetKnownVersion = %q, want %q", out, want)
	}
}

func TestSetKnownVersionNotFound(t *testing.T) {
	in := []byte("only 1.2.3 here\n")
	_, err := SetKnownVersion(in,
		version.Version{Major: 9, Minor: 9, Patch: 9},
		version.Version{Major: 9, Minor: 9, Patch: 10})
	if !errors.Is(err, ErrVersionNotFound) {
		t.Errorf("err = %v, want ErrVersionNotFound", err)
	}
}

func TestWriteAtomicPreservesMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "VERSION")
	if err := os.WriteFile(path, []byte("1.2.3\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := WriteAtomic(path, []byte("1.2.4\n")); err != nil {
		t.Fatalf("WriteAtomic: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "1.2.4\n" {
		t.Errorf("content = %q, want %q", got, "1.2.4\n")
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("mode = %v, want 0600", info.Mode().Perm())
	}

	// No leftover temp files in the directory.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Errorf("directory has %d entries, want 1 (leftover temp file?)", len(entries))
	}
}

func TestApplyBump(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "VERSION")
	if err := os.WriteFile(path, []byte("1.2.3\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	old, neu, err := ApplyBump(path, version.Version.BumpMinor, false)
	if err != nil {
		t.Fatalf("ApplyBump: %v", err)
	}
	if old != (version.Version{Major: 1, Minor: 2, Patch: 3}) {
		t.Errorf("old = %v", old)
	}
	if neu != (version.Version{Major: 1, Minor: 3, Patch: 0}) {
		t.Errorf("new = %v", neu)
	}
	got, _ := os.ReadFile(path)
	if string(got) != "1.3.0\n" {
		t.Errorf("file content = %q, want %q", got, "1.3.0\n")
	}
}

func TestApplyBumpDryRun(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "VERSION")
	if err := os.WriteFile(path, []byte("1.2.3\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	old, neu, err := ApplyBump(path, version.Version.BumpPatch, true)
	if err != nil {
		t.Fatalf("ApplyBump dry-run: %v", err)
	}
	if old != (version.Version{Major: 1, Minor: 2, Patch: 3}) || neu != (version.Version{Major: 1, Minor: 2, Patch: 4}) {
		t.Errorf("old=%v new=%v", old, neu)
	}
	got, _ := os.ReadFile(path)
	if string(got) != "1.2.3\n" {
		t.Errorf("dry-run modified file: %q", got)
	}
}
