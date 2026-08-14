package files

import (
	"errors"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sasmaq/incrmit/internal/testutil"
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

// An IPv4 address is a four-component dotted number, not a version, so a file
// containing only IPv4 addresses has no version.
func TestFindVersionIgnoresIPv4(t *testing.T) {
	for _, in := range []string{
		"bind 127.0.0.1\n",
		"gateway = 192.168.1.1",
		"mask 255.255.255.255 broadcast 10.0.0.255",
		"net 1.2.3.4",
	} {
		if _, err := FindVersion([]byte(in)); !errors.Is(err, ErrNoVersion) {
			t.Errorf("FindVersion(%q) error = %v, want ErrNoVersion", in, err)
		}
	}
}

// A real version alongside an IPv4 address is found unambiguously: the IP is not
// a competing version, and no three-component slice is pulled out of it.
func TestFindVersionWithIPv4Present(t *testing.T) {
	got, err := FindVersion([]byte("server 192.168.1.1 version 1.2.3\n"))
	if err != nil {
		t.Fatalf("FindVersion: %v", err)
	}
	if want := (version.Version{Major: 1, Minor: 2, Patch: 3}); got != want {
		t.Errorf("FindVersion = %v, want %v", got, want)
	}
}

// Bumping a file that also contains an IPv4 address rewrites only the version.
func TestSetKnownVersionLeavesIPv4Untouched(t *testing.T) {
	in := []byte("endpoint 10.0.0.255\nversion 1.2.3\n")
	out, err := SetKnownVersion(in,
		version.Version{Major: 1, Minor: 2, Patch: 3},
		version.Version{Major: 1, Minor: 2, Patch: 4})
	if err != nil {
		t.Fatalf("SetKnownVersion: %v", err)
	}
	if want := "endpoint 10.0.0.255\nversion 1.2.4\n"; string(out) != want {
		t.Errorf("SetKnownVersion = %q, want %q", out, want)
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

// repls builds a Replacement list from alternating old/new token strings, so a
// test can name the tokens it means and let version.Parse split them.
func repls(t *testing.T, pairs ...string) []Replacement {
	t.Helper()
	out := make([]Replacement, 0, len(pairs)/2)
	for i := 0; i+1 < len(pairs); i += 2 {
		oldVer, err := version.Parse(pairs[i])
		if err != nil {
			t.Fatalf("Parse(%q): %v", pairs[i], err)
		}
		newVer, err := version.Parse(pairs[i+1])
		if err != nil {
			t.Fatalf("Parse(%q): %v", pairs[i+1], err)
		}
		out = append(out, Replacement{Old: oldVer, New: newVer})
	}
	return out
}

// SetKnownVersions rewrites several distinct versions in one file, replacing
// every occurrence of each and leaving unrelated version-like tokens alone.
func TestSetKnownVersions(t *testing.T) {
	in := []byte("app 1.2.3\nlib 2.0.0\napp 1.2.3 again\nkeep 4.5.6\n")
	out, counts := SetKnownVersions(in, repls(t, "1.2.3", "1.2.4", "2.0.0", "2.0.1"))
	want := "app 1.2.4\nlib 2.0.1\napp 1.2.4 again\nkeep 4.5.6\n"
	if string(out) != want {
		t.Errorf("SetKnownVersions = %q, want %q", out, want)
	}
	if counts["1.2.3"] != 2 {
		t.Errorf("counts[1.2.3] = %d, want 2", counts["1.2.3"])
	}
	if counts["2.0.0"] != 1 {
		t.Errorf("counts[2.0.0] = %d, want 1", counts["2.0.0"])
	}
}

// A single pass over the original data means overlapping bumps do not cascade:
// replacing 1.2.3 -> 1.2.4 alongside 1.2.4 -> 1.2.5 rewrites each original token
// exactly once rather than turning the first token into the second's input.
func TestSetKnownVersionsNoCascade(t *testing.T) {
	in := []byte("first 1.2.3 then 1.2.4\n")
	out, counts := SetKnownVersions(in, repls(t, "1.2.3", "1.2.4", "1.2.4", "1.2.5"))
	want := "first 1.2.4 then 1.2.5\n"
	if string(out) != want {
		t.Errorf("SetKnownVersions = %q, want %q", out, want)
	}
	if counts["1.2.3"] != 1 || counts["1.2.4"] != 1 {
		t.Errorf("counts = %v, want each old token replaced once", counts)
	}
}

// A token that never appears reports a zero count so callers can detect an
// out-of-sync config.
func TestSetKnownVersionsMissingToken(t *testing.T) {
	in := []byte("only 1.2.3 here\n")
	out, counts := SetKnownVersions(in, repls(t, "1.2.3", "1.2.4", "9.9.9", "9.9.10"))
	if want := "only 1.2.4 here\n"; string(out) != want {
		t.Errorf("SetKnownVersions = %q, want %q", out, want)
	}
	if counts["9.9.9"] != 0 {
		t.Errorf("counts[9.9.9] = %d, want 0 (absent token)", counts["9.9.9"])
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

// ReadTarget has no size cap of its own: a file listed in the config is read
// whatever its size unless the caller asks for a limit.
func TestReadTargetHasNoSizeCap(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "big.txt")
	body := strings.Repeat("x", 4<<20) + "\n1.2.3\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := ReadTarget(path)
	if err != nil {
		t.Fatalf("ReadTarget: %v", err)
	}
	if len(got) != len(body) {
		t.Errorf("read %d bytes, want the whole %d byte file", len(got), len(body))
	}
}

// Under an explicit cap, a file up to the limit is read whole and a larger one
// is refused with a *TooLargeError reporting both sizes. It is never returned
// truncated: the caller writes the result back over the file.
func TestReadTargetWithLimit(t *testing.T) {
	dir := t.TempDir()
	body := "version 1.2.3\n"
	path := filepath.Join(dir, "VERSION")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	size := int64(len(body))

	tests := []struct {
		name     string
		limit    int64
		wantRead bool
	}{
		{"under the cap", size + 10, true},
		{"exactly at the cap", size, true},
		{"over the cap", size - 1, false},
		{"cap disabled", 0, true},
		{"negative cap disabled", -1, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ReadTargetWithLimit(path, tt.limit)
			if tt.wantRead {
				if err != nil {
					t.Fatalf("ReadTargetWithLimit: %v", err)
				}
				if string(got) != body {
					t.Errorf("content = %q, want %q", got, body)
				}
				return
			}
			var tooLarge *TooLargeError
			if !errors.As(err, &tooLarge) {
				t.Fatalf("err = %v, want a *TooLargeError", err)
			}
			if got != nil {
				t.Errorf("content = %q, want nil so a truncated file is never written back", got)
			}
			if tooLarge.Size != size || tooLarge.Limit != tt.limit {
				t.Errorf("error = %+v, want size %d and limit %d", tooLarge, size, tt.limit)
			}
		})
	}
}

// The size check never turns a non-regular target into a size complaint: the
// kind of file is still what gets reported.
func TestReadTargetWithLimitRejectsNonRegularFile(t *testing.T) {
	dir := t.TempDir()
	fifo := filepath.Join(dir, "pipe")
	if err := testutil.Mkfifo(fifo); err != nil {
		t.Skipf("FIFOs unsupported: %v", err)
	}
	if _, err := ReadTargetWithLimit(fifo, 1<<20); !errors.Is(err, ErrNotRegular) {
		t.Errorf("err = %v, want ErrNotRegular", err)
	}
}

// A non-regular target must be reported rather than opened: opening a named pipe
// blocks until a writer appears, which would hang incrmit with no output. The
// test's own deadline catches a regression.
func TestReadTargetRejectsNonRegularFile(t *testing.T) {
	dir := t.TempDir()
	fifo := filepath.Join(dir, "pipe")
	if err := testutil.Mkfifo(fifo); err != nil {
		t.Skipf("FIFOs unsupported: %v", err)
	}

	type result struct {
		err error
	}
	done := make(chan result, 1)
	go func() {
		_, err := ReadTarget(fifo)
		done <- result{err}
	}()

	select {
	case got := <-done:
		if !errors.Is(got.err, ErrNotRegular) {
			t.Errorf("err = %v, want ErrNotRegular", got.err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("ReadTarget blocked on the FIFO instead of rejecting it")
	}
}

// A symlink to a real file is still a usable target: only the type check follows
// the link, and a later write replaces the link rather than passing through it.
func TestReadTargetFollowsSymlinkToRegularFile(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "VERSION")
	if err := os.WriteFile(target, []byte("1.2.3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks unsupported: %v", err)
	}

	got, err := ReadTarget(link)
	if err != nil {
		t.Fatalf("ReadTarget: %v", err)
	}
	if string(got) != "1.2.3\n" {
		t.Errorf("content = %q, want %q", got, "1.2.3\n")
	}
}

// A write must never widen a file's permissions, whatever mode it starts with.
func TestWriteAtomicNeverWidensMode(t *testing.T) {
	for _, mode := range []os.FileMode{0o400, 0o600, 0o640, 0o644, 0o700, 0o755} {
		dir := t.TempDir()
		path := filepath.Join(dir, "VERSION")
		if err := os.WriteFile(path, []byte("1.2.3\n"), mode); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(path, mode); err != nil {
			t.Fatal(err)
		}

		if err := WriteAtomic(path, []byte("1.2.4\n")); err != nil {
			t.Fatalf("WriteAtomic with mode %v: %v", mode, err)
		}

		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if got := info.Mode().Perm(); got != mode {
			t.Errorf("mode %v became %v, want it preserved", mode, got)
		}
	}
}

// A target that is a symlink must be replaced by a regular file rather than
// written through, so a link in the tree cannot redirect a write to whatever it
// points at.
func TestWriteAtomicDoesNotWriteThroughSymlink(t *testing.T) {
	outside := t.TempDir()
	target := filepath.Join(outside, "outside.txt")
	if err := os.WriteFile(target, []byte("untouched\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	link := filepath.Join(dir, "VERSION")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks unsupported: %v", err)
	}

	if err := WriteAtomic(link, []byte("1.2.4\n")); err != nil {
		t.Fatalf("WriteAtomic: %v", err)
	}

	outsideData, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(outsideData) != "untouched\n" {
		t.Errorf("the link target was written through: content = %q", outsideData)
	}

	info, err := os.Lstat(link)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Error("path is still a symlink, so the write went through the link")
	}
	linkData, err := os.ReadFile(link)
	if err != nil {
		t.Fatal(err)
	}
	if string(linkData) != "1.2.4\n" {
		t.Errorf("content = %q, want the new version written in place of the link", linkData)
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

// The token matcher spans the whole semver token, so a prerelease or build
// section is found as part of the version rather than left dangling beside it.
func TestFindVersionWholeToken(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"prerelease", "version = \"1.2.3-rc.1\"\n", "1.2.3-rc.1"},
		{"build", "ver=1.2.3+build.7\n", "1.2.3+build.7"},
		{"both", "v = v2.0.0-beta.1+exp.sha.5114f85\n", "v2.0.0-beta.1+exp.sha.5114f85"},
		{"hyphenated identifier", "1.2.3-rc-1\n", "1.2.3-rc-1"},
		// A trailing dot ends a sentence, not an identifier.
		{"end of sentence", "Ships as 1.2.3-rc.1.\n", "1.2.3-rc.1"},
		// "_" is a word character, so no token boundary falls before it and the
		// match stops at the numeric core rather than swallowing a malformed
		// suffix.
		{"underscore suffix", "pkg-1.2.3-rc_1\n", "1.2.3"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := FindVersion([]byte(tt.in))
			if err != nil {
				t.Fatalf("FindVersion(%q): %v", tt.in, err)
			}
			if got.String() != tt.want {
				t.Errorf("FindVersion(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// A prerelease and the release it names are distinct tokens: neither is
// rewritten when the other one is the known version.
func TestSetKnownVersionDistinguishesPrerelease(t *testing.T) {
	in := []byte("current: 1.2.3-rc.1\nreleased: 1.2.3\n")

	pre := version.Version{Major: 1, Minor: 2, Patch: 3, Prerelease: "rc.1"}
	bare := version.Version{Major: 1, Minor: 2, Patch: 3}
	next := version.Version{Major: 1, Minor: 2, Patch: 4}

	got, err := SetKnownVersion(in, pre, next)
	if err != nil {
		t.Fatalf("SetKnownVersion(prerelease): %v", err)
	}
	if want := "current: 1.2.4\nreleased: 1.2.3\n"; string(got) != want {
		t.Errorf("got %q, want %q", got, want)
	}

	got, err = SetKnownVersion(in, bare, next)
	if err != nil {
		t.Fatalf("SetKnownVersion(release): %v", err)
	}
	if want := "current: 1.2.3-rc.1\nreleased: 1.2.4\n"; string(got) != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// SetKnownVersions keys on the full token too, so a prerelease and its release
// can be rewritten in the same pass without either matching the other.
func TestSetKnownVersionsDistinguishesPrerelease(t *testing.T) {
	in := []byte("preview: 1.2.4-rc.1\nstable: 1.2.3\n")
	got, counts := SetKnownVersions(in, repls(t, "1.2.4-rc.1", "1.2.4-rc.2", "1.2.3", "1.2.4"))
	if want := "preview: 1.2.4-rc.2\nstable: 1.2.4\n"; string(got) != want {
		t.Errorf("got %q, want %q", got, want)
	}
	for tok, n := range counts {
		if n != 1 {
			t.Errorf("token %q replaced %d times, want 1", tok, n)
		}
	}
}

// A file holding a version and its prerelease holds two distinct versions, so
// a generic (unpinned) replacement is ambiguous rather than silently guessing.
func TestFindVersionPrereleaseAndBareAreAmbiguous(t *testing.T) {
	var ambiguous *AmbiguousError
	if _, err := FindVersion([]byte("1.2.3-rc.1 and 1.2.3\n")); !errors.As(err, &ambiguous) {
		t.Fatalf("err = %v, want *AmbiguousError", err)
	}
}

// A version welded into a release filename keeps only its numeric core, so a
// bump rewrites the numbers and leaves the rest of the line intact. Without the
// guard in version.FindTokens, "1.2.3-linux-amd64.tar.gz" parses as a perfectly
// legal prerelease and rewriting that token eats the filename.
func TestSetVersionPreservesReleaseFilenames(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			"download url",
			"curl -O https://x/download/incrmit-1.2.3-linux-amd64.tar.gz\n",
			"curl -O https://x/download/incrmit-1.2.4-linux-amd64.tar.gz\n",
		},
		{
			"zip",
			"asset: incrmit-1.2.3-windows-amd64.zip\n",
			"asset: incrmit-1.2.4-windows-amd64.zip\n",
		},
		{
			"v-prefixed",
			"asset: incrmit-v1.2.3-darwin-arm64.tar.gz\n",
			"asset: incrmit-v1.2.4-darwin-arm64.tar.gz\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cur, err := FindVersion([]byte(tt.in))
			if err != nil {
				t.Fatalf("FindVersion: %v", err)
			}
			got, err := SetVersion([]byte(tt.in), cur.BumpPatch())
			if err != nil {
				t.Fatalf("SetVersion: %v", err)
			}
			if string(got) != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// The guard is about where the version sits, not about the file: a standalone
// prerelease on one line and a release filename on the next are handled
// differently in the same pass.
func TestSetKnownVersionsMixedFilenameAndPrerelease(t *testing.T) {
	in := []byte("version = \"1.2.3-rc.1\"\nasset: incrmit-1.2.3-linux-amd64.tar.gz\n")
	got, counts := SetKnownVersions(in, repls(t, "1.2.3-rc.1", "1.2.3-rc.2", "1.2.3", "1.2.4"))
	want := "version = \"1.2.3-rc.2\"\nasset: incrmit-1.2.4-linux-amd64.tar.gz\n"
	if string(got) != want {
		t.Errorf("got %q, want %q", got, want)
	}
	for tok, n := range counts {
		if n != 1 {
			t.Errorf("token %q replaced %d times, want 1", tok, n)
		}
	}
}

// A pinned prerelease says which suffix is real, so it is consumed even inside a
// release filename that the token matcher cut back to its numeric core. This is
// what makes the bump semver-correct there: "app-1.2.3-rc.1.zip" becomes
// "app-1.2.4.zip", not "app-1.2.4-rc.1.zip".
func TestSetKnownVersionsConsumesPinnedSuffixInFilenames(t *testing.T) {
	tests := []struct {
		name  string
		in    string
		pairs []string
		want  string
	}{
		{
			"promote inside a filename",
			"asset: app-1.2.3-rc.1.zip\n",
			[]string{"1.2.3-rc.1", "1.2.4"},
			"asset: app-1.2.4.zip\n",
		},
		{
			"advance inside a filename",
			"asset: app-1.2.3-rc.1.zip\n",
			[]string{"1.2.3-rc.1", "1.2.3-rc.2"},
			"asset: app-1.2.3-rc.2.zip\n",
		},
		{
			"suffix followed by more hyphenated parts",
			"url: incrmit-1.2.4-rc.1-linux-amd64.tar.gz\n",
			[]string{"1.2.4-rc.1", "1.2.4"},
			"url: incrmit-1.2.4-linux-amd64.tar.gz\n",
		},
		{
			"build metadata too",
			"asset: app-1.2.3-rc.1+build.7.zip\n",
			[]string{"1.2.3-rc.1+build.7", "1.2.4"},
			"asset: app-1.2.4.zip\n",
		},
		{
			"standalone and filename in one pass",
			"version = \"1.2.3-rc.1\"\nasset: app-1.2.3-rc.1.zip\n",
			[]string{"1.2.3-rc.1", "1.2.4"},
			"version = \"1.2.4\"\nasset: app-1.2.4.zip\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := SetKnownVersions([]byte(tt.in), repls(t, tt.pairs...))
			if string(got) != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// Consuming a pinned suffix must not run into a longer one: "-rc.1" is not the
// start of "-rc.10", and a pin whose suffix is simply absent leaves the
// occurrence to be matched on its numeric core alone.
func TestSetKnownVersionsPinnedSuffixBoundaries(t *testing.T) {
	tests := []struct {
		name  string
		in    string
		pairs []string
		want  string
	}{
		{
			"rc.1 does not match rc.10",
			"asset: app-1.2.3-rc.10.zip\n",
			[]string{"1.2.3-rc.1", "1.2.4"},
			"asset: app-1.2.3-rc.10.zip\n",
		},
		{
			"different suffix left alone",
			"asset: app-1.2.3-linux-amd64.tar.gz\n",
			[]string{"1.2.3-rc.1", "1.2.4"},
			"asset: app-1.2.3-linux-amd64.tar.gz\n",
		},
		{
			"bare pin still matches the core in a filename",
			"asset: app-1.2.3-linux-amd64.tar.gz\n",
			[]string{"1.2.3", "1.2.4"},
			"asset: app-1.2.4-linux-amd64.tar.gz\n",
		},
		{
			"bare pin does not reach into a standalone prerelease",
			"version = \"1.2.3-rc.1\"\n",
			[]string{"1.2.3", "1.2.4"},
			"version = \"1.2.3-rc.1\"\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := SetKnownVersions([]byte(tt.in), repls(t, tt.pairs...))
			if string(got) != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// When a bare pin and a suffixed pin could both match an occurrence, the
// suffixed one wins: it is the pin that says the suffix belongs to the version.
func TestSetKnownVersionsPrefersTheSuffixedPin(t *testing.T) {
	in := []byte("asset: app-1.2.3-rc.1.zip\nplain: 1.2.3\n")
	got, counts := SetKnownVersions(in, repls(t,
		"1.2.3", "1.2.4",
		"1.2.3-rc.1", "1.2.4",
	))
	want := "asset: app-1.2.4.zip\nplain: 1.2.4\n"
	if string(got) != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if counts["1.2.3-rc.1"] != 1 || counts["1.2.3"] != 1 {
		t.Errorf("counts = %v, want one match each", counts)
	}
}
