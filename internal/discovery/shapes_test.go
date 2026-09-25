package discovery

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"testing/iotest"
	"unicode/utf8"

	"github.com/sasmaq/incrmit/internal/version"
)

// The tests in this file are the discovery half of the pathological file shapes
// covered in internal/files/shapes_test.go: what a scan reports for a file whose
// line endings, encoding, size, or position in the tree is unusual.

// discoverOne scans a tree holding just name with body and returns what
// discovery reported for it, or nil when the file was skipped.
func discoverOne(t *testing.T, name, body string) *Result {
	t.Helper()
	root := t.TempDir()
	mustWrite(t, root, name, body)
	results, err := Discover(root)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(results) == 0 {
		return nil
	}
	return &results[0]
}

// Every line-ending convention an editor recognizes starts a new line: "\n",
// "\r\n", and a lone "\r". The reported text is the line without its
// terminator, so no carriage return reaches the dry run's output, where a
// terminal would act on it and overwrite the line it was printing.
func TestDiscoverLineEndings(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		wantLine int
	}{
		{"LF", "a = 1\nb = 2\nversion = 1.2.3\nc = 3\n", 3},
		{"CRLF", "a = 1\r\nb = 2\r\nversion = 1.2.3\r\nc = 3\r\n", 3},
		{"mixed CRLF and LF", "a = 1\r\nb = 2\nversion = 1.2.3\r\nc = 3\n", 3},
		{"lone CR", "a = 1\rb = 2\rversion = 1.2.3\rc = 3\r", 3},
		{"CR before CRLF", "a = 1\r\r\nversion = 1.2.3\r\n", 3},
		{"blank CRLF lines", "\r\n\r\n\r\nversion = 1.2.3", 4},
		{"no trailing newline", "a = 1\nversion = 1.2.3", 2},
		{"BOM", bomUTF8 + "version = 1.2.3\n", 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := discoverOne(t, "Cargo.toml", tt.body)
			if r == nil || len(r.Occurrences) != 1 {
				t.Fatalf("Discover reported %+v, want one occurrence", r)
			}
			o := r.Occurrences[0]
			if o.Line != tt.wantLine {
				t.Errorf("line = %d, want %d", o.Line, tt.wantLine)
			}
			if o.Text != "version = 1.2.3" {
				t.Errorf("text = %q, want %q", o.Text, "version = 1.2.3")
			}
		})
	}
}

// Several tokens across lines with different endings are each placed on the
// right line, which is what the forward-only line cursor has to get right when
// it steps over one terminator at a time.
func TestDiscoverNumbersEveryOccurrence(t *testing.T) {
	body := "1.2.3\r\n\r\nx 1.2.3\ry\r1.2.3 1.2.3\n\n\r\nlast 1.2.3"
	r := discoverOne(t, "notes.txt", body)
	if r == nil {
		t.Fatal("Discover skipped the file")
	}
	var got []int
	for _, o := range r.Occurrences {
		got = append(got, o.Line)
	}
	if want := []int{1, 3, 5, 5, 8}; !slices.Equal(got, want) {
		t.Errorf("lines = %v, want %v", got, want)
	}
}

const bomUTF8 = "\xEF\xBB\xBF"

// utf16LE encodes ASCII s as little-endian UTF-16 with a byte-order mark.
func utf16LE(s string) string {
	var b strings.Builder
	b.WriteString("\xFF\xFE")
	for i := 0; i < len(s); i++ {
		b.WriteByte(s[i])
		b.WriteByte(0)
	}
	return b.String()
}

// Encodings: UTF-16 is skipped as binary, because every other byte of it is NUL,
// so discovery never offers a UTF-16 file as a target it could not bump. Latin-1
// holds no NUL and is scanned as text, as is UTF-8 with or without a BOM.
func TestDiscoverEncodings(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, root, "utf16.props", utf16LE("<Version>1.2.3</Version>\r\n"))
	mustWrite(t, root, "latin1.txt", "# caf\xE9 \xA9\nversion = 1.2.3\n")
	mustWrite(t, root, "bom.json", bomUTF8+"{\"version\": \"1.2.3\"}\n")

	results, err := Discover(root)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if got, want := paths(results), []string{"bom.json", "latin1.txt"}; !slices.Equal(got, want) {
		t.Fatalf("paths = %v, want %v", got, want)
	}
	for _, r := range results {
		if v := firstVersion(r); v.String() != "1.2.3" {
			t.Errorf("%s: version = %v, want 1.2.3", r.Path, v)
		}
	}
	if text := results[0].Occurrences[0].Text; text != `{"version": "1.2.3"}` {
		t.Errorf("bom.json text = %q, want the BOM left out", text)
	}
}

// A zero-length file is skipped without error, like any file with no version.
func TestDiscoverZeroLengthFile(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, root, "empty", "")
	mustWrite(t, root, "blank", " \r\n\t\n")
	mustWrite(t, root, "VERSION", "1.2.3")

	results, err := Discover(root)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if got := paths(results); !slices.Equal(got, []string{"VERSION"}) {
		t.Errorf("paths = %v, want only VERSION", got)
	}
}

// A NUL anywhere marks a file as binary, even when the version sits in a
// perfectly readable stretch after it: the check is on the whole file, not on
// the bytes before the first token.
func TestDiscoverSkipsVersionPastNUL(t *testing.T) {
	for name, body := range map[string]string{
		"NUL first":         "\x00\nversion = 1.2.3\n",
		"NUL before":        "header\x00trailer\nversion = 1.2.3\n",
		"NUL after":         "version = 1.2.3\n\x00",
		"NUL against token": "1.2.3\x00",
	} {
		t.Run(name, func(t *testing.T) {
			if r := discoverOne(t, "blob", body); r != nil {
				t.Errorf("Discover reported %+v for a file holding a NUL", r)
			}
		})
	}
}

// A tree far deeper than any real project is walked to the bottom: the version
// at the deepest level is found under its full relative path, and an ignore
// pattern prunes a directory however deep it sits.
func TestDiscoverDeeplyNestedTree(t *testing.T) {
	const depth = 128
	root := t.TempDir()
	segs := make([]string, depth)
	for i := range segs {
		segs[i] = "d"
	}
	deep := filepath.Join(append([]string{root}, segs...)...)
	pruned := filepath.Join(append(append([]string{root}, segs[:depth/2]...), "testdata")...)
	for _, dir := range []string{deep, pruned} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Skipf("cannot build a %d-level tree here: %v", depth, err)
		}
	}
	mustWrite(t, deep, "VERSION", "1.2.3\n")
	mustWrite(t, pruned, "VERSION", "9.9.9\n")

	results, err := Discover(root, "testdata/")
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	want := strings.Repeat("d/", depth) + "VERSION"
	if got := paths(results); !slices.Equal(got, []string{want}) {
		t.Errorf("paths = %v, want only the %d-level VERSION", got, depth)
	}
}

// A file larger than the cap is refused whole, even when the cap falls inside a
// token. Scanning the first maxBytes would read "1.2.34" as "1.2.3" and record a
// version the file does not hold.
func TestDiscoverRefusesFileWhoseCapFallsMidToken(t *testing.T) {
	root := t.TempDir()
	body := "version = 1.2.34\n"
	mustWrite(t, root, "VERSION", body)
	mustWrite(t, root, "small", "1.0.0\n")

	cut := int64(strings.Index(body, "1.2.34") + len("1.2.3"))
	results, err := DiscoverWithLimit(root, cut)
	if err != nil {
		t.Fatalf("DiscoverWithLimit: %v", err)
	}
	if got := paths(results); !slices.Equal(got, []string{"small"}) {
		t.Errorf("paths = %v, want only small: VERSION is over the cap", got)
	}
}

// readAtMost is the guard for a file that grows between detect's size check and
// its read: the extra bytes must make it refuse the file rather than hand back
// the first maxBytes of it.
func TestReadAtMost(t *testing.T) {
	const body = "version = 1.2.34\n"
	size := int64(len(body))
	tests := []struct {
		name   string
		limit  int64
		wantOK bool
	}{
		{"under the cap", size + 1, true},
		{"exactly at the cap", size, true},
		{"one byte over", size - 1, false},
		{"cap falls mid-token", size - 2, false},
		{"no cap", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, ok := readAtMost(strings.NewReader(body), tt.limit)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if ok && string(data) != body {
				t.Errorf("data = %q, want the whole body", data)
			}
			if !ok && data != nil {
				t.Errorf("data = %q, want nil so a truncated file is never scanned", data)
			}
		})
	}

	failing := io.MultiReader(strings.NewReader("1.2.3"), iotest.ErrReader(io.ErrUnexpectedEOF))
	if _, ok := readAtMost(failing, 0); ok {
		t.Error("a read error was reported as a successful read")
	}
}

// A minified file is one enormous line. Every occurrence on it is reported on
// line 1, and each carries only the stretch of the line around its own token:
// copying the whole line into every occurrence made memory grow with
// occurrences times line length.
func TestDiscoverMinifiedLine(t *testing.T) {
	const n = 20000
	body := "[" + strings.Repeat(`{"name":"x","version":"1.2.3"},`, n) + "{}]"
	r := discoverOne(t, "bundle.min.json", body)
	if r == nil || len(r.Occurrences) != n {
		t.Fatalf("Discover reported %d occurrences, want %d", len(r.Occurrences), n)
	}
	limit := 2*maxContext + len("1.2.3") + 2*len("...")
	for i, o := range r.Occurrences {
		if o.Line != 1 {
			t.Fatalf("occurrence %d is on line %d, want 1", i, o.Line)
		}
		if len(o.Text) > limit || !strings.Contains(o.Text, `"version":"1.2.3"`) {
			t.Fatalf("occurrence %d text is %d bytes (%q), want at most %d around the token", i, len(o.Text), o.Text, limit)
		}
	}
	if first := r.Occurrences[0].Text; !strings.HasPrefix(first, `[{"name"`) || !strings.HasSuffix(first, "...") {
		t.Errorf("first text = %q, want the start of the line kept and the rest cut", first)
	}
	if last := r.Occurrences[n-1].Text; !strings.HasPrefix(last, "...") || !strings.HasSuffix(last, "{}]") {
		t.Errorf("last text = %q, want the end of the line kept and the rest cut", last)
	}
}

func TestContextText(t *testing.T) {
	long := strings.Repeat("a", 200)
	tests := []struct {
		name string
		line string // "@" marks the token 1.2.3
		want string
	}{
		{"short line whole and trimmed", "  version = @  ", "version = 1.2.3"},
		{"long line cut on both sides", long + " @ " + long,
			"..." + strings.Repeat("a", maxContext-1) + " 1.2.3 " + strings.Repeat("a", maxContext-1) + "..."},
		{"cut only after", "v=@ " + long, "v=1.2.3 " + strings.Repeat("a", maxContext-1) + "..."},
		{"cut only before", long + " @", "..." + strings.Repeat("a", maxContext-1) + " 1.2.3"},
		{"exactly maxContext each side", strings.Repeat("b", maxContext) + "@" + strings.Repeat("c", maxContext),
			strings.Repeat("b", maxContext) + "1.2.3" + strings.Repeat("c", maxContext)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := []byte(strings.ReplaceAll(tt.line, "@", "1.2.3"))
			tok := bytes.Index(data, []byte("1.2.3"))
			if got := contextText(data, 0, len(data), tok, tok+5); got != tt.want {
				t.Errorf("contextText =\n %q\nwant\n %q", got, tt.want)
			}
		})
	}
}

// A cut through multi-byte UTF-8 moves to the next character boundary rather
// than splitting a sequence, wherever the cut falls; bytes that are not UTF-8 at
// all still end the search.
func TestContextTextNeverSplitsUTF8(t *testing.T) {
	for pad := 0; pad < 4; pad++ {
		side := strings.Repeat("x", pad) + strings.Repeat("版", maxContext)
		data := []byte(side + " 1.2.3 " + side)
		tok := bytes.Index(data, []byte("1.2.3"))
		got := contextText(data, 0, len(data), tok, tok+5)
		if !utf8.ValidString(got) {
			t.Errorf("pad %d: contextText split a character: %q", pad, got)
		}
		if !strings.Contains(got, " 1.2.3 ") {
			t.Errorf("pad %d: contextText = %q, lost the token", pad, got)
		}
	}

	latin1 := []byte(strings.Repeat("\xA9", 200) + "1.2.3" + strings.Repeat("\xA9", 200))
	got := contextText(latin1, 0, len(latin1), 200, 205)
	if !strings.Contains(got, "1.2.3") || len(got) > 2*maxContext+2*utf8.UTFMax+5+6 {
		t.Errorf("Latin-1 context = %q (%d bytes), want the token and a bounded window", got, len(got))
	}
}

// Glob metacharacters in a file name are only characters: discovery reports the
// name as it is. In the config's ignore list the same characters are patterns,
// and a pattern matches a metacharacter literally only when it is wrapped in
// brackets — a backslash cannot escape it, because config loading reads every
// backslash as a path separator.
func TestIgnoreMetacharactersArePatterns(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows file names cannot hold * or ?")
	}
	names := []string{"v*.txt", "vX.txt", "[ab].txt", "a.txt", "?.txt", "x.txt"}
	root := t.TempDir()
	for _, name := range names {
		mustWrite(t, root, name, "1.2.3\n")
	}

	tests := []struct {
		pattern string
		ignored []string
	}{
		{"v*.txt", []string{"v*.txt", "vX.txt"}},
		{"v[*].txt", []string{"v*.txt"}},
		{"[ab].txt", []string{"a.txt"}},
		{"[[]ab].txt", []string{"[ab].txt"}},
		{"?.txt", []string{"?.txt", "a.txt", "x.txt"}},
		{"[?].txt", []string{"?.txt"}},
	}
	for _, tt := range tests {
		t.Run(tt.pattern, func(t *testing.T) {
			results, err := Discover(root, tt.pattern)
			if err != nil {
				t.Fatalf("Discover: %v", err)
			}
			var want []string
			for _, name := range names {
				if !slices.Contains(tt.ignored, name) {
					want = append(want, name)
				}
			}
			slices.Sort(want)
			if got := paths(results); !slices.Equal(got, want) {
				t.Errorf("ignore %q found %v, want %v", tt.pattern, got, want)
			}
		})
	}
}

// Awkward names are recorded exactly, so the config discovery writes names the
// file the user has, and the version found in each is the one it holds.
func TestDiscoverAwkwardNames(t *testing.T) {
	names := []string{"my version file.txt", "versión-版本.txt", "-VERSION", `it's "quoted"`, strings.Repeat("v", 255)}
	if runtime.GOOS != "windows" {
		names = append(names, "VER\nSION", "v*.txt", "[ab].txt", `back\slash`)
	}
	root := t.TempDir()
	for _, name := range names {
		mustWrite(t, root, name, "1.2.3\n")
	}
	results, err := Discover(root)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	slices.Sort(names)
	if got := paths(results); !slices.Equal(got, names) {
		t.Errorf("paths = %q, want %q", got, names)
	}
	for _, r := range results {
		if firstVersion(r) != (version.Version{Major: 1, Minor: 2, Patch: 3}) {
			t.Errorf("%q: version = %v, want 1.2.3", r.Path, firstVersion(r))
		}
	}
}
