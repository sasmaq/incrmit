package files

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/sasmaq/incrmit/internal/testutil"
	"github.com/sasmaq/incrmit/internal/version"
)

func TestLocateYAML(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    []located
		skipped []string // "key=value"
	}{
		{"plain, single-quoted, and double-quoted", "a: 1.2.3\nb: '1.2.4'\nc: \"1.2.5\"\n",
			[]located{{"a", "1.2.3"}, {"b", "1.2.4"}, {"c", "1.2.5"}}, nil},
		{"v prefix, prerelease, and build", "a: v1.2.3\nb: 1.2.3-rc.1+build.5\nc: 2.3.1+45\n",
			[]located{{"a", "v1.2.3"}, {"b", "1.2.3-rc.1+build.5"}, {"c", "2.3.1+45"}}, nil},
		{"nested mappings and sequences", "a:\n  b:\n    - 1.0.0\n    - c: 2.0.0\n",
			[]located{{"a.b[0]", "1.0.0"}, {"a.b[1].c", "2.0.0"}}, nil},
		{"flow mapping and sequence", "a: {v: 1.0.0, w: [2.0.0, {x: '3.0.0'}]}\n",
			[]located{{"a.v", "1.0.0"}, {"a.w[0]", "2.0.0"}, {"a.w[1].x", "3.0.0"}}, nil},
		{"only whole values", "image: ghcr.io/me/app:1.4.2\nrange: ~17.11.3\nk8s: \">=1.22.0-0\"\nd: version 1.2.3\nt: ' 1.2.3'\n", nil, nil},
		{"a comment after a value", "version: 1.2.3 # was 1.2.2\n# 1.2.1\n", []located{{"version", "1.2.3"}}, nil},
		{"block scalars hold text", "notes: |\n  version: 1.2.3\nmore: >\n  version: 1.2.4\nkept: |-\n  1.2.5\nfolded: >-\n  1.2.6\n", nil, nil},
		{"a plain scalar folded over lines", "v: spread over\n  1.2.3\nw:\n  1.2.4\n", []located{{"w", "1.2.4"}}, nil},
		{"explicit !!str tag", "a: !!str 1.2.3\nb: !!str '1.2.4'\nc: !<tag:yaml.org,2002:str> 1.2.5\n",
			[]located{{"a", "1.2.3"}, {"b", "1.2.4"}, {"c", "1.2.5"}}, nil},
		{"a tag of its own", "a: !semver 1.2.3\nb: !!binary 1.2.4\n", nil, nil},
		{"a key is never a candidate", "1.2.3: x\n\"2.0.0\": {3.0.0: 4.0.0}\n",
			[]located{{`"2.0.0"."3.0.0"`, "4.0.0"}}, nil},
		{"a value under a key that is not a scalar", "? [1.2.3, x]\n: 2.0.0\n? {a: 3.0.0}\n: [4.0.0]\n", nil, nil},
		{"root sequence", "- 1.0.0\n- version: 2.0.0\n", []located{{"[0]", "1.0.0"}, {"[1].version", "2.0.0"}}, nil},
		{"root scalar", "1.2.3\n", []located{{"", "1.2.3"}}, nil},
		{"duplicate keys are both candidates", "version: 1.2.3\nversion: 1.2.4\n",
			[]located{{"version", "1.2.3"}, {"version", "1.2.4"}}, nil},
		{"escaped value is skipped", "version: \"1\\x2e2.3\"\nplain: 1.2.3\nu: \"\\u0031.2.3\"\n",
			[]located{{"plain", "1.2.3"}}, []string{"version=1.2.3", "u=1.2.3"}},
		{"only an escaped version", "v: \"\\x31.2.3\"\n", nil, []string{"v=1.2.3"}},
		{"a line continuation in double quotes", "v: \"\\\n  1.2.3\"\n", []located{{"v", "1.2.3"}}, nil},
		{"keys holding dots and odd characters", "a.b:\n  \"@types/node\": 1.2.3\n\"\": 4.5.6\nk8s.io/version: 7.8.9\n",
			[]located{{`"a.b"."@types/node"`, "1.2.3"}, {`""`, "4.5.6"}, {`"k8s.io/version"`, "7.8.9"}}, nil},
		{"keys that are not strings", "1: 1.0.0\ntrue: 2.0.0\n~: 3.0.0\n", []located{{"1", "1.0.0"}, {"true", "2.0.0"}, {`"~"`, "3.0.0"}}, nil},
		{"numbers, dates, and two components", "a: 1.5\nb: 2024-05-01\nc: 3.9\nd: 192.168.1.1\ne: 1_000\n", nil, nil},
		{"a tab after a key", "version:\t1.2.3\n", []located{{"version", "1.2.3"}}, nil},
		{"CRLF", "version: 1.2.3\r\nx:\r\n  - 2.0.0 # 9.9.9\r\n", []located{{"version", "1.2.3"}, {"x[0]", "2.0.0"}}, nil},
		{"lone CR", "version: 1.2.3\rx: 2.0.0\r", []located{{"version", "1.2.3"}, {"x", "2.0.0"}}, nil},
		{"BOM", bomUTF8 + "version: 1.2.3\n", []located{{"version", "1.2.3"}}, nil},
		{"document markers", "%YAML 1.1\n---\nversion: 1.2.3\n...\n", []located{{"version", "1.2.3"}}, nil},
		{"filename value", "asset: app-1.2.3-rc.1.zip\n", nil, nil},
		{"empty", "", nil, nil},
		{"only comments", "# version: 1.2.3\n", nil, nil},
		{"an empty document", "---\n# 1.2.3\n", nil, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := []byte(tt.in)
			loc, err := ParserYAML.Locate(data)
			if err != nil {
				t.Fatalf("Locate: %v", err)
			}
			if got := locatedOf(t, data, loc.Candidates); !slices.Equal(got, tt.want) {
				t.Errorf("candidates = %v, want %v", got, tt.want)
			}
			var skipped []string
			for _, s := range loc.Skipped {
				skipped = append(skipped, s.Key.String()+"="+s.Value)
			}
			if !slices.Equal(skipped, tt.skipped) {
				t.Errorf("skipped = %v, want %v", skipped, tt.skipped)
			}
			assertRewritesCleanly(t, ParserYAML, data)
		})
	}
}

// The decoder accepts a byte-order mark, and the probe moves no byte outside
// a token, so unlike JSON nothing is offset: every range indexes the file as
// it is, BOM included, and a skipped value's line is the decoder's.
func TestLocateYAMLOffsetsAndLines(t *testing.T) {
	data := []byte(bomUTF8 + "a: x\r\nesc: \"1\\x2e2.3\"\rversion: 1.2.3\n")
	loc, err := ParserYAML.Locate(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(loc.Candidates) != 1 {
		t.Fatalf("candidates = %+v, want one", loc.Candidates)
	}
	c := loc.Candidates[0]
	if want := strings.Index(string(data), "1.2.3"); c.Start != want || c.End != want+5 {
		t.Errorf("candidate at [%d,%d), want [%d,%d)", c.Start, c.End, want, want+5)
	}
	if len(loc.Skipped) != 1 || loc.Skipped[0].Line != 2 {
		t.Errorf("skipped = %+v, want one on line 2", loc.Skipped)
	}
}

// The fixtures the YAML parser refuses: a file that is not YAML, a stream of
// two documents, and YAML whose probe copy does not decode because the
// placeholders push a key past the decoder's 1,024-character limit.
func TestLocateYAMLRefusesFixtures(t *testing.T) {
	tests := []struct {
		input string
		want  string
		probe bool
		docs  int
	}{
		{"tabs.yaml.input", "not YAML (line 4: found character that cannot start any token)", false, 0},
		{"meta.yaml.input", "not YAML (found character that cannot start any token)", false, 0},
		{"repro/deploy.yaml", "2 YAML documents", false, 2},
		{"implicit-key.yaml.input", "YAML whose versions incrmit cannot place (line 2: could not find expected ':')", true, 0},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("testdata", tt.input))
			if err != nil {
				t.Fatal(err)
			}
			_, err = ParserYAML.Locate(data)
			var pe *ParseError
			if !errors.As(err, &pe) {
				t.Fatalf("Locate error = %v, want a *ParseError", err)
			}
			if pe.Error() != tt.want || pe.Probe != tt.probe || pe.Docs != tt.docs || pe.Parser != ParserYAML {
				t.Errorf("ParseError = %+v (%q), want %q", pe, pe.Error(), tt.want)
			}
		})
	}

	// The key that overflows in the probe copy is well within the limit as
	// written, so the file itself is YAML.
	data, err := os.ReadFile(filepath.Join("testdata", "implicit-key.yaml.input"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := decodeYAML(data, false); err != nil {
		t.Fatalf("the implicit-key fixture does not decode as written: %v", err)
	}
	pr := newProbe(data, nil, TextCandidates(data))
	if key := strings.SplitN(string(pr.data), "\n", 3)[1]; len(key) <= 1024 {
		t.Fatalf("the probe copy's key is %d bytes, not past the decoder's limit", len(key))
	}
}

func TestLocateYAMLRejects(t *testing.T) {
	tests := []struct {
		name, in string
		want     string
	}{
		{"JSON with a trailing comma", "{\"version\": \"1.2.3\",,}\n", "not YAML ("},
		{"unknown anchor", "a: 1\nb: *nope\n", "not YAML (unknown anchor 'nope' referenced)"},
		{"unterminated quote", "a: 1\nb: \"x\n", "not YAML (line 2: found unexpected end of stream)"},
		{"three documents", "a: 1\n---\nb: 2\n---\nc: 3\n", "3 YAML documents"},
		{"an empty second document", "a: 1.2.3\n---\n", "2 YAML documents"},
		{"a bad second document", "a: 1\n---\nb: [\n", "not YAML (line 3: did not find expected node content)"},
		{"tab indentation", "a:\n\tb: 1.2.3\n", "not YAML (line 2: found character that cannot start any token)"},
		// The decoder reads YAML 1.1 and refuses a document that says it is
		// 1.2, although it reads the same document without the directive.
		{"a YAML 1.2 directive", "%YAML 1.2\n---\nversion: 1.2.3\n", "not YAML (found incompatible YAML document)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParserYAML.Locate([]byte(tt.in))
			var pe *ParseError
			if !errors.As(err, &pe) || pe.Probe {
				t.Fatalf("Locate error = %v, want a *ParseError", err)
			}
			if !strings.HasPrefix(pe.Error(), tt.want) {
				t.Errorf("Error() = %q, want it to start %q", pe.Error(), tt.want)
			}
		})
	}
}

func TestYAMLParseErrorMessage(t *testing.T) {
	tests := []struct {
		err  *ParseError
		want string
	}{
		{&ParseError{Parser: ParserYAML, Line: 3, Msg: "did not find expected key"}, "not YAML (line 3: did not find expected key)"},
		{&ParseError{Parser: ParserYAML, Docs: 2}, "2 YAML documents"},
		{&ParseError{Parser: ParserYAML, Probe: true, Msg: "could not find expected ':'"}, "YAML whose versions incrmit cannot place (could not find expected ':')"},
		{yamlParseError(errors.New("yaml: line 12: mapping values are not allowed in this context"), false), "not YAML (line 12: mapping values are not allowed in this context)"},
		{yamlParseError(errors.New("yaml: unknown anchor 'x' referenced"), false), "not YAML (unknown anchor 'x' referenced)"},
		{yamlParseError(errors.New("yaml: line x: odd"), false), "not YAML (line x: odd)"},
	}
	for _, tt := range tests {
		if got := tt.err.Error(); got != tt.want {
			t.Errorf("Error() = %q, want %q", got, tt.want)
		}
	}
}

// An alias is never a candidate: the version counts once, at its anchor, and
// Aliases lists every alias that reads it, directly or through an anchored
// node holding it, in document order. A merge key is an alias too.
func TestLocateYAMLAliases(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want map[string][]string // candidate key -> its aliases
	}{
		{"an anchored scalar", "base: &v 1.2.3\npinned: *v\nagain: [*v]\n",
			map[string][]string{"base": {"pinned", "again[0]"}}},
		{"a merge key", "defaults: &d\n  version: 1.2.3\nprod:\n  <<: *d\n  name: x\nstage:\n  <<: [*d]\n",
			map[string][]string{"defaults.version": {`prod."<<"`, `stage."<<"[0]`}}},
		{"an alias of what holds an alias", "a: &a 1.2.3\nb: &b {v: *a}\nc: *b\nd: [*b]\n",
			map[string][]string{"a": {"b.v", "c", "d[0]"}}},
		{"nested anchors", "outer: &o\n  inner: &i\n    version: 1.2.3\n  other: 2.0.0\nx: *i\ny: *o\n",
			map[string][]string{"outer.inner.version": {"x", "y"}, "outer.other": {"y"}}},
		{"an alias inside its own anchor", "a: &a [*a, 1.2.3]\n", map[string][]string{"a[1]": {"a[0]"}}},
		{"an alias as a key", "a: &k {v: 1.2.3}\nm:\n  *k : x\n", map[string][]string{"a.v": {"m"}}},
		{"two versions under one anchor", "d: &d {a: 1.2.3, b: 2.0.0}\nx: *d\n",
			map[string][]string{"d.a": {"x"}, "d.b": {"x"}}},
		{"no alias", "a: &a 1.2.3\nb: 2.0.0\n", map[string][]string{"a": nil, "b": nil}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := []byte(tt.in)
			loc, err := ParserYAML.Locate(data)
			if err != nil {
				t.Fatalf("Locate: %v", err)
			}
			got := map[string][]string{}
			for _, c := range loc.Candidates {
				var names []string
				for _, a := range loc.Aliases(c) {
					names = append(names, a.String())
				}
				got[c.Key.String()] = names
			}
			if len(got) != len(tt.want) {
				t.Errorf("candidates = %v, want %v", got, tt.want)
			}
			for k, want := range tt.want {
				if g, ok := got[k]; !ok || !slices.Equal(g, want) {
					t.Errorf("aliases of %s = %v, want %v", k, g, want)
				}
			}
			assertRewritesCleanly(t, ParserYAML, data)
		})
	}

	// Aliases is nil for every other format.
	loc, err := ParserJSON.Locate([]byte(`{"v":"1.2.3"}`))
	if err != nil || loc.Aliases(loc.Candidates[0]) != nil {
		t.Errorf("JSON Aliases = %v, %v", loc.Aliases(loc.Candidates[0]), err)
	}
}

// A key names where a value is written. One that names an alias, or a path
// the data only has through an alias or a merge key, names a value with no
// bytes of its own there, so it is an error saying where the value is
// written; so is every value a key can name that is not a candidate.
func TestClaimYAML(t *testing.T) {
	data := []byte(`version: 1.2.3
base: &v 1.2.3
pinned: *v
defaults: &d
  version: 2.0.0
prod:
  <<: *d
  name: x
inline:
  <<: {version: 3.0.0}
copy: *d
dup: 1.2.3
dup: 1.2.3
block: |-
  1.2.3
custom: !semver 1.2.3
range: ^1.2.3
num: 1.5
yes: true
none: null
when: 2024-05-01
map: {a: b}
seq: [a]
esc: "1\x2e2.3"
keyed:
  ? [a, b]
  : 1.2.3
list: &l [1.2.3]
copylist: *l
stage:
  <<: [*d]
nested:
  <<: {<<: *d}
`)
	loc, err := ParserYAML.Locate(data)
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		key  string
		pin  version.Version
		want string // the error, or "" for a claim
	}{
		{"version", v123, ""},
		{"base", v123, ""},
		{"defaults.version", version.Version{Major: 2}, ""},
		{"version", v124, "key version holds 1.2.3, not 1.2.4"},
		{"pinned", v123, "key pinned reads base through an alias, and incrmit rewrites a value where it is written; name base instead"},
		{"prod.version", version.Version{Major: 2}, "key prod.version reads defaults.version through an alias, and incrmit rewrites a value where it is written; name defaults.version instead"},
		{`prod."<<"`, version.Version{Major: 2}, `key prod."<<" reads defaults through an alias`},
		{`prod."<<".version`, version.Version{Major: 2}, `key prod."<<".version reads defaults.version through an alias`},
		{"copy.version", version.Version{Major: 2}, "key copy.version reads defaults.version through an alias"},
		{"inline.version", version.Version{Major: 3}, `key inline.version reads inline."<<".version through a merge key, and incrmit rewrites a value where it is written; name inline."<<".version instead`},
		{`inline."<<".version`, version.Version{Major: 3}, ""},
		{"prod.missing", v123, "key prod.missing is not in the file"},
		{"pinned.deeper", v123, "key pinned.deeper is not in the file"},
		{"dup", v123, "key dup appears twice in the file, so it names no one value"},
		{"block", v123, "key block holds a block scalar; incrmit rewrites a version written plain or in quotes"},
		{"custom", v123, `key custom holds "1.2.3" tagged !semver; a version is a string, untagged or tagged !!str`},
		{"range", v123, `key range holds "^1.2.3", which is not a version`},
		{"num", v123, "key num holds a number; a version is a string"},
		{"yes", v123, "key yes holds a boolean, not a version"},
		{"none", v123, "key none holds null, not a version"},
		{"when", v123, "key when holds a date or time, not a version"},
		{"map", v123, "key map holds a mapping, not a version"},
		{"seq", v123, "key seq holds a sequence, not a version"},
		{"esc", v123, `key esc holds "1.2.3" only once its escapes are decoded`},
		{"keyed", v123, "key keyed holds a mapping, not a version"},
		{"copylist[0]", v123, "key copylist[0] reads list[0] through an alias"},
		{"copylist[1]", v123, "key copylist[1] is not in the file"},
		{"stage.version", version.Version{Major: 2}, "key stage.version reads defaults.version through an alias"},
		{"nested.version", version.Version{Major: 2}, "key nested.version reads defaults.version through an alias"},
		{"prod.missing.deeper", v123, "key prod.missing.deeper is not in the file"},
	} {
		t.Run(tt.key, func(t *testing.T) {
			k, err := ParseKey(ParserYAML, tt.key)
			if err != nil {
				t.Fatal(err)
			}
			at, err := loc.Claim(k, tt.pin)
			if tt.want == "" {
				if err != nil || len(at) != 1 {
					t.Errorf("Claim = %v, %v; want one occurrence", at, err)
				}
				return
			}
			var keyErr *KeyError
			if !errors.As(err, &keyErr) || !strings.HasPrefix(err.Error(), tt.want) {
				t.Errorf("Claim error = %v, want it to start %q", err, tt.want)
			}
		})
	}
}

// A path through an alias to a node no key path names (an anchor in a key)
// still says it goes through an alias, and a merge that leads back to its
// own mapping ends rather than loops.
func TestClaimYAMLThroughOddAliases(t *testing.T) {
	data := []byte("m:\n  ? &k {v: 1.2.3}\n  : x\nn: *k\nloop: &l\n  <<: *l\n  a: 1.0.0\ns: &s x\nbad:\n  <<: *s\n")
	loc, err := ParserYAML.Locate(data)
	if err != nil {
		t.Fatal(err)
	}
	for key, want := range map[string]string{
		"n":         "key n reads its value through an alias, from a node no key can name",
		"n.v":       "key n.v reads its value through an alias, from a node no key can name",
		"loop.b":    "key loop.b is not in the file",
		"loop.a.b":  "key loop.a.b is not in the file",
		"loop[0]":   "key loop[0] is not in the file",
		"m.v":       "key m.v is not in the file",
		"[0]":       "key [0] is not in the file",
		"loop.a[0]": "key loop.a[0] is not in the file",
		"bad.v":     "key bad.v is not in the file",
	} {
		k, err := ParseKey(ParserYAML, key)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := loc.Named(k); err == nil || !strings.HasPrefix(err.Error(), want) {
			t.Errorf("Named(%s) = %v, want %q", key, err, want)
		}
	}
}

// Rewrite's check compares an alias as the anchor it names, so bumping an
// anchored version, which changes what every alias reads, is the plan; a
// candidate placed wrongly is refused as for TOML.
func TestRewriteYAML(t *testing.T) {
	data := []byte("base: &v 1.2.3\npinned: *v\nlist: [*v]\nother: 1.2.3 # 1.2.3\n")
	loc, err := ParserYAML.Locate(data)
	if err != nil || len(loc.Candidates) != 2 {
		t.Fatalf("Locate = %+v, %v", loc, err)
	}
	bump := []Replacement{{Old: v123, New: v124}}
	out, _, err := Rewrite(ParserYAML, data, loc.Candidates, bump)
	if err != nil {
		t.Fatal(err)
	}
	if want := "base: &v 1.2.4\npinned: *v\nlist: [*v]\nother: 1.2.4 # 1.2.3\n"; string(out) != want {
		t.Errorf("Rewrite = %q, want %q", out, want)
	}

	comment := strings.LastIndex(string(data), "1.2.3")
	// A version holding a document marker cannot come from a bump; it stands
	// for any rewrite that splits the file.
	split := []Replacement{{Old: v123, New: version.Version{Major: 1, Minor: 2, Patch: 3, Build: "x\n---\nb"}}}
	for _, tt := range []struct {
		name  string
		data  []byte
		cands []Candidate
		repls []Replacement
		msg   string
	}{
		{"comment token reported as a value", data,
			[]Candidate{{Start: comment, End: comment + 5, Version: v123, Key: KeyPath{keyStep("other")}}},
			bump, "did not change other"},
		{"value reported at the wrong key", data,
			[]Candidate{{Start: loc.Candidates[0].Start, End: loc.Candidates[0].End, Version: v123, Key: KeyPath{keyStep("pinned")}}},
			bump, "would change base"},
		{"key reported as a value", []byte("1.2.3: x\n"),
			[]Candidate{{Start: 0, End: 5, Version: v123, Key: KeyPath{keyStep("1.2.3")}}},
			bump, "changed the document's keys"},
		{"a rewrite that is no longer one document", []byte("a: 1.2.3\n"),
			[]Candidate{{Start: 3, End: 8, Version: v123, Key: KeyPath{keyStep("a")}}},
			split, "no longer YAML"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			out, _, err := Rewrite(ParserYAML, tt.data, tt.cands, tt.repls)
			if err == nil || !strings.Contains(err.Error(), tt.msg) {
				t.Fatalf("Rewrite = %q, %v; want an error containing %q", out, err, tt.msg)
			}
		})
	}
	if _, _, err := Rewrite(ParserYAML, []byte("a: [\n1.2.3"), []Candidate{{Start: 5, End: 10, Version: v123}}, bump); err == nil {
		t.Error("Rewrite of input that does not parse succeeded")
	}
}

// The probe's placeholders use digits the file does not hold, in its text or
// in anything it decodes to, so a file already holding them is placed
// exactly too.
func TestLocateYAMLPlaceholderCollisions(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []located
	}{
		{"placeholder text in a comment", "# 987654321.987654321.000000001\nversion: 1.2.3\n",
			[]located{{"version", "1.2.3"}}},
		{"placeholder text in a value", "id: \"987654321987654321000000001\"\nversion: 1.2.3\n",
			[]located{{"version", "1.2.3"}}},
		{"placeholder digits as a key", "987654321: {987654321: 1.2.3}\nversion: 4.5.6\n",
			[]located{{"987654321.987654321", "1.2.3"}, {"version", "4.5.6"}}},
		{"placeholder digits only once decoded", "k: \"9\\x3876543210\"\nversion: 1.2.3\n",
			[]located{{"version", "1.2.3"}}},
		{"placeholder digits in a tag", "k: !<tag:x,9%3876543210> y\nversion: 1.2.3\n",
			[]located{{"version", "1.2.3"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := []byte(tt.in)
			loc, err := ParserYAML.Locate(data)
			if err != nil {
				t.Fatalf("Locate: %v", err)
			}
			if got := locatedOf(t, data, loc.Candidates); !slices.Equal(got, tt.want) {
				t.Errorf("candidates = %v, want %v", got, tt.want)
			}
			assertRewritesCleanly(t, ParserYAML, data)
		})
	}
}

// With more than ten tokens an unpadded placeholder would be a prefix of
// another; every candidate here must sit at its own key, and no token in a
// comment may be one.
func TestLocateYAMLManyTokens(t *testing.T) {
	var b strings.Builder
	var want []located
	b.WriteString("bin:\n")
	for i := 1; i <= 25; i++ {
		fmt.Fprintf(&b, "  # release 1.%d.0 notes\n  - version: 1.%d.1 # was 1.%d.0\n", i, i, i)
		want = append(want, located{fmt.Sprintf("bin[%d].version", i-1), fmt.Sprintf("1.%d.1", i)})
	}
	data := []byte(b.String())
	loc, err := ParserYAML.Locate(data)
	if err != nil {
		t.Fatal(err)
	}
	if got := locatedOf(t, data, loc.Candidates); !slices.Equal(got, want) {
		t.Errorf("candidates = %v\nwant %v", got, want)
	}
	assertRewritesCleanly(t, ParserYAML, data)
}

// All tokens share one probe decode, as in TOML.
func TestLocateYAMLIsLinearInTokens(t *testing.T) {
	n := 5000
	if testing.Short() {
		n = 1000
	}
	var b strings.Builder
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, "- name: p%d # 1.%d.0\n  version: 2.%d.0\n", i, i, i)
	}
	data := []byte(b.String())
	testutil.Within(t, "locating versions in a YAML file of many tokens", func() {
		loc, err := ParserYAML.Locate(data)
		if err != nil {
			t.Error(err)
			return
		}
		if len(loc.Candidates) != n {
			t.Errorf("found %d candidates, want %d", len(loc.Candidates), n)
		}
	})
}
