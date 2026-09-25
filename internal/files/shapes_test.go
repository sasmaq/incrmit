package files

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/sasmaq/incrmit/internal/version"
)

// The tests in this file cover file shapes nobody types into a fixture: other
// line endings, a byte-order mark, encodings that are not UTF-8, tokens at the
// very edges of the data, files with no line structure at all, and the metadata
// an in-place rewrite either keeps or deliberately gives up.
//
// Each rewrite is checked against an expected output built independently from
// the same template as the input, rather than by reverting the output with
// strings.ReplaceAll: the comparison is byte-exact, so a CR dropped, a BOM
// stripped, or a newline added anywhere in the file fails it.

var (
	v123 = version.Version{Major: 1, Minor: 2, Patch: 3}
	v124 = version.Version{Major: 1, Minor: 2, Patch: 4}
)

// shaped expands a template in which "@" stands for the version token.
func shaped(tmpl, tok string) []byte {
	return []byte(strings.ReplaceAll(tmpl, "@", tok))
}

// bomUTF8 is the UTF-8 encoding of U+FEFF, which Windows editors and PowerShell
// write at the start of a file.
const bomUTF8 = "\xEF\xBB\xBF"

// A rewrite keeps the file's line endings and encoding exactly as they were: it
// never normalizes CRLF to LF, strips a BOM, or re-encodes bytes that are not
// UTF-8. Only the version token changes.
func TestSetVersionKeepsFileShape(t *testing.T) {
	tests := []struct {
		name string
		tmpl string
	}{
		{"CRLF throughout", "[package]\r\nname = \"app\"\r\nversion = \"@\"\r\n"},
		{"mixed CRLF and LF", "[package]\r\nname = \"app\"\nversion = \"@\"\r\nedition = \"2021\"\n"},
		{"lone CR", "[package]\rname = \"app\"\rversion = \"@\"\r"},
		{"CR right against the token", "version=@\r\r\n\r"},
		{"UTF-8 BOM before the first key", bomUTF8 + "{\n  \"version\": \"@\"\n}\n"},
		{"UTF-8 BOM right against the token", bomUTF8 + "@\n"},
		// Latin-1 bytes are not valid UTF-8, but hold no NUL either, so neither
		// the scanner nor discovery's binary check treats them specially: the
		// file is text as far as incrmit is concerned, and its bytes pass
		// through untouched. \xE9 is "é", \xA9 "©" (a UTF-8 continuation byte),
		// and \xFF is never valid UTF-8 at all.
		{"Latin-1 bytes", "# caf\xE9 \xA9 2024\nversion = @\n# fin\xFF\n"},
		{"Latin-1 byte right against the token", "\xE9@\xE9"},
		{"non-ASCII UTF-8 around the token", "# 版本 → @ ✓\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := shaped(tt.tmpl, "1.2.3")
			want := shaped(tt.tmpl, "1.2.4")

			if got, err := FindVersion(in); err != nil || got != v123 {
				t.Fatalf("FindVersion = %v, %v; want %v", got, err, v123)
			}
			got, err := SetVersion(in, v124)
			if err != nil {
				t.Fatalf("SetVersion: %v", err)
			}
			if !bytes.Equal(got, want) {
				t.Errorf("SetVersion changed more than the token\n got: %q\nwant: %q", got, want)
			}
		})
	}
}

// A BOM is three bytes the scanner sees like any other non-word bytes: it does
// not hide a token that follows it directly, and every token range sits exactly
// three bytes further on than in the same file without the BOM.
func TestBOMDoesNotShiftTokenRanges(t *testing.T) {
	body := []byte("1.2.3\n{\"version\": \"1.2.3\", \"api\": \"v2.0.0\"}\n")
	plain := version.FindTokens(body)
	withBOM := version.FindTokens(append([]byte(bomUTF8), body...))

	if len(plain) != 3 || len(withBOM) != len(plain) {
		t.Fatalf("FindTokens found %v without the BOM and %v with it, want three each", plain, withBOM)
	}
	for i := range plain {
		if withBOM[i][0] != plain[i][0]+len(bomUTF8) || withBOM[i][1] != plain[i][1]+len(bomUTF8) {
			t.Errorf("token %d is at %v with the BOM and %v without, want an offset of exactly %d",
				i, withBOM[i], plain[i], len(bomUTF8))
		}
	}
}

// utf16 encodes s (ASCII only) as UTF-16 with a byte-order mark, in the given
// byte order, which is how Windows tools write a "Unicode" text file.
func utf16(s string, bigEndian bool) []byte {
	out := []byte{0xFF, 0xFE}
	if bigEndian {
		out = []byte{0xFE, 0xFF}
	}
	for i := 0; i < len(s); i++ {
		if bigEndian {
			out = append(out, 0, s[i])
		} else {
			out = append(out, s[i], 0)
		}
	}
	return out
}

// A UTF-16 file has no version as far as incrmit is concerned: every character
// is two bytes, one of them NUL, and a NUL between each digit and dot splits
// every token apart. This pins the documented answer, so that "no semantic
// version found" on a UTF-16 file is the expected result rather than a surprise
// — and so that a pinned version is reported missing instead of the file being
// rewritten with a UTF-8 token spliced into it.
func TestUTF16HasNoVersion(t *testing.T) {
	for _, tt := range []struct {
		name      string
		bigEndian bool
	}{
		{"little-endian", false},
		{"big-endian", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			in := utf16("<Version>1.2.3</Version>\r\n", tt.bigEndian)

			if _, err := FindVersion(in); !errors.Is(err, ErrNoVersion) {
				t.Errorf("FindVersion err = %v, want ErrNoVersion", err)
			}
			if _, err := SetKnownVersion(in, v123, v124); !errors.Is(err, ErrVersionNotFound) {
				t.Errorf("SetKnownVersion err = %v, want ErrVersionNotFound", err)
			}
			out, counts := SetKnownVersions(in, []Replacement{{Old: v123, New: v124}})
			if counts["1.2.3"] != 0 || !bytes.Equal(out, in) {
				t.Errorf("SetKnownVersions rewrote a UTF-16 file: counts %v\n in: %q\nout: %q", counts, in, out)
			}
		})
	}
}

// The scanner special-cases the edges of the data: a token at byte 0 has no
// bytes before it for the filename guard to inspect (the `start < 2` branch of
// suffixBelongs), and a token flush against EOF has no byte after it for
// matchAt's boundary check (`after < len(data)`). Each edge must still be found
// whole and rewritten without reading out of bounds.
func TestTokensAtTheEdgesOfTheFile(t *testing.T) {
	tests := []struct {
		name     string
		in       string
		old, new string
		want     string
	}{
		{"exactly the version, no newline", "1.2.3", "1.2.3", "1.2.4", "1.2.4"},
		{"exactly a prefixed version", "v1.2.3", "v1.2.3", "v1.2.4", "v1.2.4"},
		{"exactly a prerelease", "1.2.3-rc.1", "1.2.3-rc.1", "1.2.3-rc.2", "1.2.3-rc.2"},
		// Start 1, so the guard would read data[-1] without its bounds check;
		// with nothing but a hyphen before it the suffix belongs to the token.
		{"one hyphen before a prerelease", "-1.2.3-rc.1", "1.2.3-rc.1", "1.2.3-rc.2", "-1.2.3-rc.2"},
		{"token as the final byte", "version = 1.2.3", "1.2.3", "1.2.4", "version = 1.2.4"},
		{"token as the first byte", "1.2.3 is the version\n", "1.2.3", "1.2.4", "1.2.4 is the version\n"},
		// The filename guard cuts the token to its core and the pin extends it
		// over "-rc.1", which ends exactly at EOF.
		{"pinned suffix flush against EOF", "app-1.2.3-rc.1", "1.2.3-rc.1", "1.2.3-rc.2", "app-1.2.3-rc.2"},
		{"pinned suffix one byte before EOF", "app-1.2.3-rc.1\n", "1.2.3-rc.1", "1.2.3-rc.2", "app-1.2.3-rc.2\n"},
		// At EOF the pin must still not claim the start of a longer suffix.
		{"longer suffix flush against EOF", "app-1.2.3-rc.10", "1.2.3-rc.1", "1.2.3-rc.2", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldVer, err := version.Parse(tt.old)
			if err != nil {
				t.Fatal(err)
			}
			newVer, err := version.Parse(tt.new)
			if err != nil {
				t.Fatal(err)
			}
			got, err := SetKnownVersion([]byte(tt.in), oldVer, newVer)
			if tt.want == "" {
				if !errors.Is(err, ErrVersionNotFound) {
					t.Errorf("SetKnownVersion(%q) = %q, %v; want ErrVersionNotFound", tt.in, got, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("SetKnownVersion(%q): %v", tt.in, err)
			}
			if string(got) != tt.want {
				t.Errorf("SetKnownVersion(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// A file with nothing in it, or nothing but whitespace, has no version. It must
// be reported as such by every entry point, never panic, and never be written.
func TestEmptyAndBlankFilesHaveNoVersion(t *testing.T) {
	for _, body := range []string{"", " ", "\n", "\r\n", "\r", " \t\r\n\n\t", bomUTF8, bomUTF8 + "\r\n"} {
		t.Run(strconv.Quote(body), func(t *testing.T) {
			if _, err := FindVersion([]byte(body)); !errors.Is(err, ErrNoVersion) {
				t.Errorf("FindVersion err = %v, want ErrNoVersion", err)
			}
			if _, err := SetVersion([]byte(body), v124); !errors.Is(err, ErrNoVersion) {
				t.Errorf("SetVersion err = %v, want ErrNoVersion", err)
			}
			if _, err := SetKnownVersion([]byte(body), v123, v124); !errors.Is(err, ErrVersionNotFound) {
				t.Errorf("SetKnownVersion err = %v, want ErrVersionNotFound", err)
			}

			path := filepath.Join(t.TempDir(), "VERSION")
			if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
			if _, _, err := ApplyBump(path, version.Version.BumpPatch, false); !errors.Is(err, ErrNoVersion) {
				t.Errorf("ApplyBump err = %v, want ErrNoVersion", err)
			}
			if got, _ := os.ReadFile(path); string(got) != body {
				t.Errorf("ApplyBump wrote %q over a file that had no version", got)
			}
		})
	}
}

// minifiedManifest returns a package manifest the way a bundler emits it: one
// line, no trailing newline, with deps dependencies whose versions all differ
// from the package's own "@" token. It comes out at about 25 bytes per
// dependency.
func minifiedManifest(deps int) string {
	var b strings.Builder
	b.WriteString(`{"name":"app","version":"@","dependencies":{`)
	for i := 0; i < deps; i++ {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(`"dep-`)
		b.WriteString(strconv.Itoa(i))
		b.WriteString(`":"^4.`)
		b.WriteString(strconv.Itoa(i % 97))
		b.WriteString(`.`)
		b.WriteString(strconv.Itoa(i))
		b.WriteByte('"')
	}
	b.WriteString(`},"homepage":"https://example.com/app"}`)
	return b.String()
}

// A minified file is one enormous line: the rewriter has no line structure to
// lean on, so the pinned token must be found among tens of thousands of other
// versions and replaced exactly once, with every other byte left alone.
func TestSetKnownVersionsMinifiedSingleLine(t *testing.T) {
	deps := 2000 // ~45 KB on one line
	if !testing.Short() {
		deps = 25000 // ~625 KB
	}
	tmpl := minifiedManifest(deps)
	in, want := shaped(tmpl, "1.2.3"), shaped(tmpl, "1.2.4")

	got, counts := SetKnownVersions(in, []Replacement{{Old: v123, New: v124}})
	if counts["1.2.3"] != 1 {
		t.Errorf("replaced 1.2.3 %d times, want once", counts["1.2.3"])
	}
	if !bytes.Equal(got, want) {
		t.Errorf("rewriting a %d byte minified line changed more than the token", len(in))
	}
}

// A file holding the same version thousands of times — a generated lockfile, a
// changelog of one release — has every occurrence rewritten, counted, and
// nothing else touched. The separators vary so the tokens are packed against
// each kind of neighbor the scanner accepts.
//
// The sizes are kept modest on purpose. Both passes are linear, so a larger file
// adds only time; what the larger case buys is that a quadratic regression shows
// up as a test running for minutes rather than one passing unnoticed. -short
// drops to the smaller case; the larger one costs a couple of seconds under
// -race.
func TestSetKnownVersionsThousandsOfOccurrences(t *testing.T) {
	n := 5000
	if !testing.Short() {
		n = 100000 // ~600 KB, as dense as tokens can be packed
	}
	seps := []string{",", " ", "\n", "\r\n", "\"", "=", "\t"}
	var tmpl strings.Builder
	for i := 0; i < n; i++ {
		tmpl.WriteString("@")
		tmpl.WriteString(seps[i%len(seps)])
	}
	in, want := shaped(tmpl.String(), "1.2.3"), shaped(tmpl.String(), "10.0.0")

	got, counts := SetKnownVersions(in, []Replacement{{Old: v123, New: version.Version{Major: 10}}})
	if counts["1.2.3"] != n {
		t.Errorf("replaced 1.2.3 %d times, want %d", counts["1.2.3"], n)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("rewriting %d occurrences changed more than the tokens", n)
	}

	// FindVersion reads the same file as one version, not n of them.
	if v, err := FindVersion(in); err != nil || v != v123 {
		t.Errorf("FindVersion = %v, %v; want %v", v, err, v123)
	}
}

// skipOnWindows skips a test of POSIX file metadata: mode bits, and what a
// rename does to a read-only or hard-linked target. Windows has no mode bits
// beyond read-only, and these behaviors are only verified on Unix.
func skipOnWindows(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("POSIX file metadata semantics, verified on Unix only")
	}
}

// A read-only file still bumps, and stays read-only: the write is a rename in
// the parent directory, never an open of the target for writing, so the
// target's own mode does not stand in the way. Write protection comes from the
// directory.
func TestApplyBumpReadOnlyFile(t *testing.T) {
	skipOnWindows(t)
	path := filepath.Join(t.TempDir(), "VERSION")
	if err := os.WriteFile(path, []byte("1.2.3\n"), 0o444); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o444); err != nil {
		t.Fatal(err)
	}

	if _, _, err := ApplyBump(path, version.Version.BumpPatch, false); err != nil {
		t.Fatalf("ApplyBump on a 0444 file: %v", err)
	}
	if got, _ := os.ReadFile(path); string(got) != "1.2.4\n" {
		t.Errorf("content = %q, want %q", got, "1.2.4\n")
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o444 {
		t.Errorf("mode = %v, want 0444 kept", info.Mode().Perm())
	}
}

// The setuid, setgid, and sticky bits do not survive a write: WriteAtomic
// carries only the permission bits over to the file it renames into place. That
// is deliberate. The file after a bump is a new file written by whoever ran
// incrmit, and re-applying setuid to it would mint a setuid file owned by that
// user — which is exactly why the kernel clears those bits when an unprivileged
// process writes to such a file in place.
func TestWriteAtomicDropsSpecialModeBits(t *testing.T) {
	skipOnWindows(t)
	for _, bit := range []os.FileMode{os.ModeSetuid, os.ModeSetgid, os.ModeSticky} {
		t.Run(bit.String(), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "VERSION")
			if err := os.WriteFile(path, []byte("1.2.3\n"), 0o755); err != nil {
				t.Fatal(err)
			}
			// Not every system lets an unprivileged user set every bit: BSD
			// refuses the sticky bit on a regular file, and setgid needs
			// membership of the file's group. Skip what cannot be set up.
			if err := os.Chmod(path, 0o755|bit); err != nil {
				t.Skipf("cannot set %v: %v", bit, err)
			}
			if info, err := os.Stat(path); err != nil || info.Mode()&bit == 0 {
				t.Skipf("the system did not keep %v on the file", bit)
			}

			if err := WriteAtomic(path, []byte("1.2.4\n")); err != nil {
				t.Fatalf("WriteAtomic: %v", err)
			}
			info, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			if special := info.Mode() & (os.ModeSetuid | os.ModeSetgid | os.ModeSticky); special != 0 {
				t.Errorf("mode = %v, want %v dropped", info.Mode(), special)
			}
			if info.Mode().Perm() != 0o755 {
				t.Errorf("permission bits = %v, want 0755 kept", info.Mode().Perm())
			}
		})
	}
}

// A hard link is broken by a write: the rename puts a new file under the target's
// name, so the other name keeps the old inode and the old contents. That is the
// price of atomicity — keeping the link would mean truncating and rewriting the
// shared inode in place, which is exactly the partial write the rename exists to
// rule out.
func TestWriteAtomicBreaksHardLinks(t *testing.T) {
	skipOnWindows(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "VERSION")
	other := filepath.Join(dir, "VERSION.link")
	if err := os.WriteFile(path, []byte("1.2.3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(path, other); err != nil {
		t.Skipf("hard links unsupported: %v", err)
	}

	if err := WriteAtomic(path, []byte("1.2.4\n")); err != nil {
		t.Fatalf("WriteAtomic: %v", err)
	}
	if got, _ := os.ReadFile(path); string(got) != "1.2.4\n" {
		t.Errorf("target = %q, want the new contents", got)
	}
	if got, _ := os.ReadFile(other); string(got) != "1.2.3\n" {
		t.Errorf("other link = %q, want the old contents kept", got)
	}
	a, errA := os.Stat(path)
	b, errB := os.Stat(other)
	if errA != nil || errB != nil {
		t.Fatal(errA, errB)
	}
	if os.SameFile(a, b) {
		t.Error("the two names still share a file after the write")
	}
}

// awkwardNames are file names a user might hand incrmit: nothing in the read or
// write path may interpret any part of one. The glob metacharacters matter
// because a target path is taken literally, while the same characters in the
// config's ignore list are patterns.
func awkwardNames(t *testing.T) map[string]string {
	t.Helper()
	names := map[string]string{
		"spaces":         "my version file.txt",
		"non-ASCII":      "versión-版本.txt",
		"leading dash":   "-VERSION",
		"double dash":    "--file",
		"quotes":         `it's "the" version`,
		"length limit":   strings.Repeat("v", 255), // NAME_MAX on Linux and macOS
		"only a dot run": "...",
	}
	if runtime.GOOS != "windows" {
		names["newline"] = "VER\nSION"
		names["star"] = "v*.txt"
		names["class"] = "[ab].txt"
		names["question mark"] = "?.txt"
		names["backslash"] = `back\slash.txt`
		names["carriage return"] = "VER\rSION"
	}
	return names
}

// Every awkward name reads and writes like any other, and the temp file used for
// the atomic write never depends on the target's name — which is what lets a
// name already at the length limit be written at all.
func TestApplyBumpAwkwardNames(t *testing.T) {
	for label, name := range awkwardNames(t) {
		t.Run(label, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, name)
			if err := os.WriteFile(path, []byte("1.2.3\n"), 0o644); err != nil {
				t.Skipf("cannot create %q here: %v", name, err)
			}
			if _, _, err := ApplyBump(path, version.Version.BumpPatch, false); err != nil {
				t.Fatalf("ApplyBump(%q): %v", name, err)
			}
			if got, _ := os.ReadFile(path); string(got) != "1.2.4\n" {
				t.Errorf("content = %q, want %q", got, "1.2.4\n")
			}
			entries, err := os.ReadDir(dir)
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != 1 || entries[0].Name() != name {
				t.Errorf("directory holds %v, want only %q", entries, name)
			}
		})
	}
}
