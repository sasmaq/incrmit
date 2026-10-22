package files

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/sasmaq/incrmit/internal/version"
)

// A path is written the way KeyPath.String writes one, so every path a parser
// reports reads back to itself, and a part that need not be quoted may be.
func TestParsePath(t *testing.T) {
	tests := []struct {
		in   string
		want KeyPath
	}{
		{"version", KeyPath{keyStep("version")}},
		{"package.version", KeyPath{keyStep("package"), keyStep("version")}},
		{"bin[0].version", KeyPath{keyStep("bin"), indexStep(0), keyStep("version")}},
		{"[12][0]", KeyPath{indexStep(12), indexStep(0)}},
		{`"."`, KeyPath{keyStep(".")}},
		{`dependencies."@types/node"`, KeyPath{keyStep("dependencies"), keyStep("@types/node")}},
		{`"version"`, KeyPath{keyStep("version")}},
		{`""`, KeyPath{keyStep("")}},
		{`"a\"b\\c\tdé\U0001F600"`, KeyPath{keyStep("a\"b\\c\tdé\U0001F600")}},
		{"x-1_2.3", KeyPath{keyStep("x-1_2"), keyStep("3")}},
		{`"\b\f\n\r"`, KeyPath{keyStep("\b\f\n\r")}},
	}
	for _, tt := range tests {
		got, err := ParsePath(tt.in)
		if err != nil || got.String() != tt.want.String() || len(got) != len(tt.want) {
			t.Errorf("ParsePath(%q) = %v, %v, want %v", tt.in, got, err, tt.want)
			continue
		}
		back, err := ParsePath(got.String())
		if err != nil || back.String() != got.String() {
			t.Errorf("%q renders as %q, which reads back as %v, %v", tt.in, got.String(), back, err)
		}
	}

	refused := []struct{ in, want string }{
		{"", "it is empty"},
		{"VERSION ?= ", `' ' cannot follow "VERSION"`},
		{"a b", `' ' cannot follow "a"`},
		{".a", `'.' cannot start a path`},
		{"a.", "it ends in a dot"},
		{"a..b", `'.' cannot follow "a."`},
		{"a.[0]", `'[' cannot follow "a."`},
		{"a[0]b", `'b' cannot follow "a[0]"`},
		{`"a"b`, `'b' cannot follow "\"a\""`},
		{"a[0", "the [ at byte 2 is not closed"},
		{"a[]", "[] is not an array index"},
		{"a[01]", "[01] is not an array index"},
		{"a[-1]", "[-1] is not an array index"},
		{"a[+1]", "[+1] is not an array index"},
		{"a[99999999999999999999]", "is not an array index"},
		{`"a`, "the quote at byte 1 is not closed"},
		{`"a\q"`, `"\\q" is not an escape a path may use`},
		{`"a\u12"`, `\u12" is not a Unicode character`},
		{`"\uD800"`, `\uD800 is not a Unicode character`},
		{`"a\`, "it ends in a backslash"},
		{"\"a\x01\"", "control character"},
		{"é", "'é' cannot start a path"},
		{"\xff", `"\xff" cannot start a path`},
	}
	for _, tt := range refused {
		if _, err := ParsePath(tt.in); err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("ParsePath(%q) = %v, want an error holding %q", tt.in, err, tt.want)
		}
	}
}

func TestParseKey(t *testing.T) {
	for _, tt := range []struct {
		p    Parser
		in   string
		want string // the key as messages show it, or the error
	}{
		{ParserText, "VERSION ?= ", `"VERSION ?= "`},
		{ParserText, "version", `"version"`},
		{ParserText, "\t=", `"\t="`},
		{ParserJSON, `"version"`, "version"},
		{ParserTOML, `"package".version`, "package.version"},
		{ParserText, "", `key is empty; for a text file, a key is the text right before the version on its line, such as "VERSION ?= "`},
		{ParserJSON, "", "key is empty; for a JSON file, a key is a path such as package.version"},
		{ParserText, "a\nb", `key "a\nb" holds a line break`},
		{ParserText, "a\rb", `key "a\rb" holds a line break`},
		{ParserTOML, "VERSION ?= ", `key "VERSION ?= " is not a key path (' ' cannot follow "VERSION"); for a TOML file, a key is a path such as package.version`},
		{Parser("ini"), "version", `unknown parser "ini"`},
	} {
		k, err := ParseKey(tt.p, tt.in)
		got := k.String()
		if err != nil {
			got = err.Error()
		} else if k.Raw() != tt.in {
			t.Errorf("ParseKey(%s, %q).Raw() = %q", tt.p, tt.in, k.Raw())
		}
		if !strings.Contains(got, tt.want) {
			t.Errorf("ParseKey(%s, %q) = %q, want %q", tt.p, tt.in, got, tt.want)
		}
	}
}

// claimAll locates data with p and claims key at pin, failing the test on a
// parse error. It returns the claimed tokens' lines and the error.
func claimAll(t *testing.T, p Parser, data, key, pin string) ([]int, error) {
	t.Helper()
	loc, err := p.Locate([]byte(data))
	if err != nil {
		t.Fatalf("Locate: %v", err)
	}
	k, err := ParseKey(p, key)
	if err != nil {
		t.Fatalf("ParseKey(%q): %v", key, err)
	}
	at, err := loc.Claim(k, mustVersion(pin))
	var lines []int
	for _, a := range at {
		lines = append(lines, lineAt([]byte(data), a))
	}
	return lines, err
}

// A JSON or TOML key names the whole path from the root, never a suffix, and
// the value there must be a candidate holding the pinned version. Anything
// else is a *KeyError saying what the path holds.
func TestClaimPath(t *testing.T) {
	pkg := "{\n" +
		"  \"version\": \"1.2.3\",\n" + // 2
		"  \"dependencies\": { \"version\": \"1.2.3\", \"@types/node\": \"20.1.0\" },\n" + // 3
		"  \"bin\": [ { \"version\": \"0.1.0\" }, { \"version\": \"0.2.0\" } ],\n" + // 4
		"  \".\": \"4.5.6\",\n" + // 5
		"  \"a.b\": \"7.0.0\",\n" + // 6
		"  \"a\": { \"b\": \"7.0.1\" },\n" + // 7
		"  \"n\": 1.2, \"t\": true, \"z\": null, \"o\": {}, \"r\": \"^1.2.3\",\n" + // 8
		"  \"esc\": \"1\\u002e2.3\",\n" + // 9
		"  \"twice\": \"1.0.0\", \"twice\": \"1.0.0\",\n" + // 10
		"  \"long\": \"" + strings.Repeat("x", 50) + "\"\n" + // 11
		"}\n"
	cargo := "[package]\n" + // 1
		"version = \"0.4.0\"\n" + // 2
		"[dependencies]\n" + // 3
		"anyhow = { version = \"0.4.0\" }\n" + // 4
		"[[bin]]\n" + // 5
		"version = \"1.0.0\"\n" + // 6
		"[[bin]]\n" + // 7
		"version = \"2.0.0\"\n" + // 8
		"\"a.b\" = \"3.0.0\"\n" + // 9
		"when = 2024-01-02\n" + // 10
		"list = [\"1.0.0\", 2]\n" // 11
	tests := []struct {
		p                   Parser
		data, key, pin, err string
		lines               []int
	}{
		{ParserJSON, pkg, "version", "1.2.3", "", []int{2}},
		{ParserJSON, pkg, "dependencies.version", "1.2.3", "", []int{3}},
		{ParserJSON, pkg, `dependencies."@types/node"`, "20.1.0", "", []int{3}},
		{ParserJSON, pkg, "bin[1].version", "0.2.0", "", []int{4}},
		{ParserJSON, pkg, `"."`, "4.5.6", "", []int{5}},
		{ParserJSON, pkg, `"a.b"`, "7.0.0", "", []int{6}},
		{ParserJSON, pkg, "a.b", "7.0.1", "", []int{7}},
		{ParserJSON, pkg, "version", "1.2.4", "key version holds 1.2.3, not 1.2.4", nil},
		{ParserJSON, pkg, "dependencies.version.x", "1.2.3", "key dependencies.version.x is not in the file", nil},
		{ParserJSON, pkg, "bin[2].version", "0.2.0", "key bin[2].version is not in the file", nil},
		{ParserJSON, pkg, "bin.version", "0.2.0", "key bin.version is not in the file", nil},
		{ParserJSON, pkg, "Version", "1.2.3", "key Version is not in the file", nil},
		{ParserJSON, pkg, "n", "1.2.3", "key n holds a number; a version is a string", nil},
		{ParserJSON, pkg, "t", "1.2.3", "key t holds a boolean, not a version", nil},
		{ParserJSON, pkg, "z", "1.2.3", "key z holds null, not a version", nil},
		{ParserJSON, pkg, "o", "1.2.3", "key o holds an object, not a version", nil},
		{ParserJSON, pkg, "bin", "1.2.3", "key bin holds an array, not a version", nil},
		{ParserJSON, pkg, "r", "1.2.3", `key r holds "^1.2.3", which is not a version`, nil},
		{ParserJSON, pkg, "long", "1.2.3", `key long holds "` + strings.Repeat("x", 40) + `"..., which is not a version`, nil},
		// A clip never splits a character: é is two bytes, and the fortieth
		// byte here is the second of one.
		{ParserJSON, `{"s": "x` + strings.Repeat("é", 30) + `"}`, "s", "1.2.3", `key s holds "x` + strings.Repeat("é", 19) + `"..., which is not a version`, nil},
		{ParserJSON, pkg, "esc", "1.2.3", `key esc holds "1.2.3" only once its escapes are decoded; incrmit rewrites a version written out plainly`, nil},
		{ParserJSON, pkg, "twice", "1.0.0", "key twice appears twice in the file, so it names no one value", nil},
		{ParserJSON, `[{"v":"1.0.0"},"2.0.0"]`, "[1]", "2.0.0", "", []int{1}},
		{ParserJSON, `{"a":1,"a":2,"a":3}`, "a", "1.0.0", "key a appears 3 times in the file, so it names no one value", nil},

		{ParserTOML, cargo, "package.version", "0.4.0", "", []int{2}},
		{ParserTOML, cargo, "dependencies.anyhow.version", "0.4.0", "", []int{4}},
		{ParserTOML, cargo, "bin[0].version", "1.0.0", "", []int{6}},
		{ParserTOML, cargo, "bin[1].version", "2.0.0", "", []int{8}},
		{ParserTOML, cargo, `bin[1]."a.b"`, "3.0.0", "", []int{9}},
		{ParserTOML, cargo, "bin[1].list[0]", "1.0.0", "", []int{11}},
		{ParserTOML, cargo, "version", "0.4.0", "key version is not in the file", nil},
		{ParserTOML, cargo, "bin[1].a.b", "3.0.0", "key bin[1].a.b is not in the file", nil},
		{ParserTOML, cargo, "package", "0.4.0", "key package holds a table, not a version", nil},
		{ParserTOML, cargo, "bin", "0.4.0", "key bin holds an array, not a version", nil},
		{ParserTOML, cargo, "bin[1].when", "0.4.0", "key bin[1].when holds a date or time, not a version", nil},
		{ParserTOML, cargo, "bin[1].list[1]", "0.4.0", "key bin[1].list[1] holds a number; a version is a string", nil},
	}
	for _, tt := range tests {
		lines, err := claimAll(t, tt.p, tt.data, tt.key, tt.pin)
		var keyErr *KeyError
		switch {
		case tt.err == "" && err != nil:
			t.Errorf("%s %s: %v", tt.p, tt.key, err)
		case tt.err != "" && (err == nil || !errors.As(err, &keyErr) || err.Error() != tt.err):
			t.Errorf("%s %s: error %v, want the *KeyError %q", tt.p, tt.key, err, tt.err)
		case !slices.Equal(lines, tt.lines):
			t.Errorf("%s %s: claims lines %v, want %v", tt.p, tt.key, lines, tt.lines)
		}
	}
}

// A text key is the literal bytes right before a version: byte for byte,
// case and whitespace included, and on the version's own line, since a key
// holds no line break. It names every occurrence it comes before.
func TestClaimText(t *testing.T) {
	mk := "VERSION ?= 1.2.3\n" + // 1
		"VERSION  ?= 1.2.3\n" + // 2
		"version ?= 1.2.3\n" + // 3
		"VERSION ?=\n1.2.3\n" + // 4, 5
		"url = https://x.test/tool-1.2.3.tar.gz\n" + // 6
		"again: VERSION ?= 1.2.3\n" + // 7
		"VERSION ?= 2.0.0\n" + // 8
		"ARCHIVE = app-1.2.3-rc.1.zip\n" // 9
	tests := []struct {
		key, pin, err string
		lines         []int
	}{
		{"VERSION ?= ", "1.2.3", "", []int{1, 7}},
		{"VERSION  ?= ", "1.2.3", "", []int{2}},
		{"version ?= ", "1.2.3", "", []int{3}},
		{"VERSION ?= ", "2.0.0", "", []int{8}},
		{"tool-", "1.2.3", "", []int{6}},
		{"VERSION ?=", "1.2.3", `key "VERSION ?=" is not followed by a version anywhere in the file`, nil},
		{"Version ?= ", "1.2.3", `key "Version ?= " is not followed by a version anywhere in the file`, nil},
		{"VERSION ?= ", "3.0.0", `key "VERSION ?= " is followed by 1.2.3 and 2.0.0, not 3.0.0`, nil},
		// A pinned prerelease extends over the suffix the scanner cut off.
		{"app-", "1.2.3-rc.1", "", []int{9}},
		{"app-", "1.2.3", "", []int{9}},
	}
	for _, tt := range tests {
		lines, err := claimAll(t, ParserText, mk, tt.key, tt.pin)
		switch {
		case tt.err == "" && err != nil:
			t.Errorf("%q: %v", tt.key, err)
		case tt.err != "" && (err == nil || err.Error() != tt.err):
			t.Errorf("%q: error %v, want %q", tt.key, err, tt.err)
		case !slices.Equal(lines, tt.lines):
			t.Errorf("%q at %s: claims lines %v, want %v", tt.key, tt.pin, lines, tt.lines)
		}
	}
}

// A key is read for one parser and looked up in a file read by that parser.
func TestNamedWantsTheKeysParser(t *testing.T) {
	loc, err := ParserJSON.Locate([]byte(`{"v": "1.2.3"}`))
	if err != nil {
		t.Fatal(err)
	}
	k, err := ParseKey(ParserText, "v")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := loc.Named(k); err == nil || !strings.Contains(err.Error(), "a key for a TEXT file cannot name a value in a JSON file") {
		t.Errorf("err = %v", err)
	}
}

// A keyed replacement rewrites the candidates it claims and no others, in the
// same single pass as the rest, so overlapping bumps still do not cascade.
func TestSetKnownVersionsKeyed(t *testing.T) {
	data := []byte(`{"version": "1.2.3", "dep": "1.2.3", "next": "1.2.4"}`)
	loc, err := ParserJSON.Locate(data)
	if err != nil {
		t.Fatal(err)
	}
	claim := func(key, pin string) []int {
		at, err := loc.Claim(PathKey(ParserJSON, mustPath(t, key)), mustVersion(pin))
		if err != nil {
			t.Fatal(err)
		}
		return at
	}
	repls := []Replacement{
		{Old: mustVersion("1.2.3"), New: mustVersion("1.2.4"), Keyed: true, At: claim("version", "1.2.3")},
		{Old: mustVersion("1.2.4"), New: mustVersion("1.2.5"), Keyed: true, At: claim("next", "1.2.4")},
	}
	out, counts, err := Rewrite(ParserJSON, data, loc.Candidates, repls)
	if err != nil {
		t.Fatal(err)
	}
	if want := `{"version": "1.2.4", "dep": "1.2.3", "next": "1.2.5"}`; string(out) != want {
		t.Errorf("got %s, want %s", out, want)
	}
	if !slices.Equal(counts, []int{1, 1}) {
		t.Errorf("counts = %v", counts)
	}

	// Two keyed replacements of one version are counted apart by Matches,
	// and together by SetKnownVersions.
	repls = []Replacement{
		{Old: mustVersion("1.2.3"), New: mustVersion("1.2.4"), Keyed: true, At: claim("version", "1.2.3")},
		{Old: mustVersion("1.2.3"), New: mustVersion("1.2.4"), Keyed: true},
	}
	if got := Matches(data, loc.Candidates, repls); !slices.Equal(got, []int{1, 0}) {
		t.Errorf("Matches = %v, want [1 0]", got)
	}
	if _, m := SetKnownVersions(data, loc.Candidates, repls); m["1.2.3"] != 1 {
		t.Errorf("SetKnownVersions counts %v", m)
	}

	// A keyed replacement must reach every candidate it claims; a claim the
	// pass cannot rewrite is refused rather than dropped.
	bad := []Replacement{{Old: mustVersion("1.2.3"), New: mustVersion("1.2.4"), Keyed: true, At: []int{0}}}
	if _, _, err := Rewrite(ParserJSON, data, loc.Candidates, bad); err == nil || !strings.Contains(err.Error(), "reached 0 of the 1 occurrences of 1.2.3 its key names") {
		t.Errorf("err = %v", err)
	}
}

func mustPath(t *testing.T, s string) KeyPath {
	t.Helper()
	p, err := ParsePath(s)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// Overlap finds a candidate two replacements would both rewrite where one is
// keyed, and ignores two without keys, which the most specific pin settles.
func TestOverlap(t *testing.T) {
	// The scanner cuts app-1.2.3-rc.1.zip back to 1.2.3, which a bare 1.2.3
	// holds and a pinned 1.2.3-rc.1 extends over.
	data := []byte("V=1.2.3 W=1.2.3 X=app-1.2.3-rc.1.zip Y=1.2.3-rc.1\n")
	cands := TextCandidates(data)
	v123, rc := mustVersion("1.2.3"), mustVersion("1.2.3-rc.1")
	at := func(key string, pin version.Version) Replacement {
		loc, err := ParserText.Locate(data)
		if err != nil {
			t.Fatal(err)
		}
		k, _ := ParseKey(ParserText, key)
		claims, err := loc.Claim(k, pin)
		if err != nil {
			t.Fatal(err)
		}
		return Replacement{Old: pin, Keyed: true, At: claims}
	}
	tests := []struct {
		name  string
		repls []Replacement
		i, j  int
		start int // -1: no overlap
	}{
		{"two keys, two occurrences", []Replacement{at("V=", v123), at("W=", v123)}, 0, 0, -1},
		{"two keys, one occurrence", []Replacement{at("V=", v123), at("=", v123)}, 0, 1, 2},
		{"a key and no key", []Replacement{{Old: v123}, at("W=", v123)}, 0, 1, 10},
		{"a key and another version", []Replacement{{Old: mustVersion("2.0.0")}, at("W=", v123)}, 0, 0, -1},
		{"no key and a prerelease's key", []Replacement{{Old: v123}, at("app-", rc)}, 0, 1, 22},
		{"no key and a whole prerelease", []Replacement{{Old: v123}, at("Y=", rc)}, 0, 0, -1},
		{"no keys", []Replacement{{Old: v123}, {Old: rc}}, 0, 0, -1},
	}
	for _, tt := range tests {
		i, j, c, ok := Overlap(data, cands, tt.repls)
		if tt.start < 0 {
			if ok {
				t.Errorf("%s: overlap %d and %d at %d", tt.name, i, j, c.Start)
			}
			continue
		}
		if !ok || i != tt.i || j != tt.j || c.Start != tt.start {
			t.Errorf("%s: Overlap = %d, %d, %d, %v; want %d, %d, %d", tt.name, i, j, c.Start, ok, tt.i, tt.j, tt.start)
		}
	}
	// The caller names the occurrence by its line, counted as discovery
	// counts lines.
	if got := LineOf([]byte("a\r\nb\rc 1.2.3"), 7); got != 3 {
		t.Errorf("LineOf = %d, want 3", got)
	}
}

func mustVersion(s string) version.Version {
	v, err := version.Parse(s)
	if err != nil {
		panic(err)
	}
	return v
}
