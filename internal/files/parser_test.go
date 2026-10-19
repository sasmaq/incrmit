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

func TestParserFor(t *testing.T) {
	tests := map[string]Parser{
		"package.json":          ParserJSON,
		"sub/dir/settings.json": ParserJSON,
		"PACKAGE.JSON":          ParserJSON,
		"tsconfig.Json":         ParserJSON,
		"Cargo.toml":            ParserTOML,
		"pyproject.TOML":        ParserTOML,
		"Pipfile":               ParserText,
		"VERSION":               ParserText,
		"package.json5":         ParserText,
		"notes.json.bak":        ParserText,
		"config.tom":            ParserText,
		".json":                 ParserJSON,
	}
	for path, want := range tests {
		if got := ParserFor(path); got != want {
			t.Errorf("ParserFor(%q) = %q, want %q", path, got, want)
		}
	}
}

func TestParseParser(t *testing.T) {
	for _, name := range []string{"json", "toml", "text"} {
		if p, err := ParseParser(name); err != nil || string(p) != name {
			t.Errorf("ParseParser(%q) = %q, %v", name, p, err)
		}
	}
	for _, name := range []string{"", "JSON", "yaml", "jsonc", " json"} {
		_, err := ParseParser(name)
		if err == nil {
			t.Errorf("ParseParser(%q) accepted it", name)
			continue
		}
		if !strings.Contains(err.Error(), fmt.Sprintf("%q", name)) || !strings.Contains(err.Error(), `"text"`) {
			t.Errorf("ParseParser(%q) error %q should name the value and the choices", name, err)
		}
	}
}

func TestKeyPathString(t *testing.T) {
	tests := []struct {
		path KeyPath
		want string
	}{
		{nil, ""},
		{KeyPath{keyStep("version")}, "version"},
		{KeyPath{keyStep("package"), keyStep("version")}, "package.version"},
		{KeyPath{keyStep("bin"), indexStep(0), keyStep("version")}, "bin[0].version"},
		{KeyPath{keyStep("a.b")}, `"a.b"`},
		{KeyPath{keyStep("dependencies"), keyStep("@types/node")}, `dependencies."@types/node"`},
		{KeyPath{keyStep("$schema")}, `"$schema"`},
		{KeyPath{keyStep("")}, `""`},
		{KeyPath{keyStep("say \"hi\"\\")}, `"say \"hi\"\\"`},
		{KeyPath{keyStep("tab\there\x1b")}, `"tab\there\u001B"`},
		{KeyPath{keyStep("\b\n\f\r\x7f")}, `"\b\n\f\r\u007F"`},
		{KeyPath{keyStep("x"), indexStep(1), indexStep(2)}, "x[1][2]"},
		{KeyPath{indexStep(0), keyStep("version")}, "[0].version"},
		{KeyPath{keyStep("left-pad"), keyStep("under_score"), keyStep("1")}, "left-pad.under_score.1"},
		{KeyPath{keyStep("café")}, `"café"`},
	}
	for _, tt := range tests {
		if got := tt.path.String(); got != tt.want {
			t.Errorf("%#v.String() = %s, want %s", tt.path, got, tt.want)
		}
	}
}

// parserFixtures are the golden fixtures each parser is tested against: the
// manifests the format fixtures already hold, a Cargo.toml, and the files
// Milestone 39 was reproduced with (testdata/repro, kept as a project tree so
// the cli tests can copy it). Each golden lists every token the text parser
// finds, with the key the file's parser places it at or a note that it is not
// a candidate there, so the golden shows what reading the format excludes.
var parserFixtures = []struct{ golden, input string }{
	{"package.json", "package.json.input"},
	{"package.min.json", "package.min.json.input"},
	{"appsettings.json", "appsettings.json.input"},
	{"pyproject.toml", "pyproject.toml.input"},
	{"Cargo.toml", "Cargo.toml.input"},
	{"repro-package.json", "repro/package.json"},
	{"repro-Cargo.toml", "repro/Cargo.toml"},
	{"repro-pyproject.toml", "repro/pyproject.toml"},
}

func TestLocateGolden(t *testing.T) {
	for _, fx := range parserFixtures {
		t.Run(fx.golden, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("testdata", fx.input))
			if err != nil {
				t.Fatal(err)
			}
			p := ParserFor(strings.TrimSuffix(fx.input, ".input"))
			got := describeCandidates(t, p, data)

			goldenPath := filepath.Join("testdata", "candidates", fx.golden+".golden")
			if *update {
				if err := os.WriteFile(goldenPath, []byte(got), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			want, err := os.ReadFile(goldenPath)
			if err != nil {
				t.Fatalf("read golden (run with -update to create it): %v", err)
			}
			if got != string(want) {
				t.Errorf("candidates differ from %s\n--- got ---\n%s--- want ---\n%s", goldenPath, got, want)
			}
			assertRewritesCleanly(t, p, data)
		})
	}
}

// describeCandidates renders what p finds in data for a golden file. It also
// checks the invariant every parser keeps: each of its candidates is one of
// the text parser's, with the same range and version.
func describeCandidates(t *testing.T, p Parser, data []byte) string {
	t.Helper()
	loc, err := p.Locate(data)
	if err != nil {
		t.Fatalf("%s.Locate: %v", p, err)
	}
	text := TextCandidates(data)
	byStart := make(map[int]Candidate, len(loc.Candidates))
	for _, c := range loc.Candidates {
		i := slices.IndexFunc(text, func(tc Candidate) bool { return tc.Start == c.Start })
		if i < 0 || text[i].End != c.End || text[i].Version != c.Version {
			t.Errorf("%s candidate %+v is not a text candidate", p, c)
		}
		byStart[c.Start] = c
	}
	var b strings.Builder
	fmt.Fprintf(&b, "parser: %s\n", p)
	for _, c := range text {
		where := "(not a candidate)"
		if pc, ok := byStart[c.Start]; ok {
			where = pc.Key.String()
			if p == ParserText {
				where = "(text)"
			}
		}
		fmt.Fprintf(&b, "L%d\t%s\t%s\n", lineAt(data, c.Start), c.Version, where)
	}
	for _, s := range loc.Skipped {
		fmt.Fprintf(&b, "skipped\t%s\t%q\n", s.Key, s.Value)
	}
	return b.String()
}

// assertRewritesCleanly rewrites every candidate in data to a version of a
// different length through Rewrite, whose own check decodes the result, and
// asserts that the bytes outside the candidates are untouched.
func assertRewritesCleanly(t *testing.T, p Parser, data []byte) {
	t.Helper()
	loc, err := p.Locate(data)
	if err != nil {
		t.Fatal(err)
	}
	var repls []Replacement
	seen := map[string]bool{}
	for _, c := range loc.Candidates {
		if !seen[c.Version.String()] {
			seen[c.Version.String()] = true
			next := c.Version
			next.Major += 1000
			repls = append(repls, Replacement{Old: c.Version, New: next})
		}
	}
	out, counts, err := Rewrite(p, data, loc.Candidates, repls)
	if err != nil {
		t.Fatalf("Rewrite: %v", err)
	}
	total := 0
	for _, n := range counts {
		total += n
	}
	if total != len(loc.Candidates) {
		t.Errorf("Rewrite replaced %d candidates, want all %d", total, len(loc.Candidates))
	}
	// Rebuild the expected output from the input, a range at a time.
	var want []byte
	prev := 0
	for _, c := range loc.Candidates {
		want = append(want, data[prev:c.Start]...)
		next := c.Version
		next.Major += 1000
		want = append(want, next.String()...)
		prev = c.End
	}
	want = append(want, data[prev:]...)
	if string(out) != string(want) {
		t.Errorf("Rewrite changed bytes outside the candidates\n--- got ---\n%s\n--- want ---\n%s", out, want)
	}
}

// located is a candidate as the shape tests state it: its key and its token.
type located struct{ key, token string }

func locatedOf(t *testing.T, data []byte, cands []Candidate) []located {
	t.Helper()
	out := make([]located, 0, len(cands))
	for _, c := range cands {
		tok := string(data[c.Start:c.End])
		if tok != c.Version.String() {
			t.Errorf("candidate bytes %q are not its version %s", tok, c.Version)
		}
		out = append(out, located{c.Key.String(), tok})
	}
	return out
}

func TestLocateJSON(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    []located
		skipped []string // "key=value"
	}{
		{"top-level version", `{"name":"x","version":"1.2.3"}`, []located{{"version", "1.2.3"}}, nil},
		{"v prefix and prerelease", `{"a":"v1.2.3","b":"1.2.3-rc.1+build.5"}`,
			[]located{{"a", "v1.2.3"}, {"b", "1.2.3-rc.1+build.5"}}, nil},
		{"only whole values", `{"$schema":"https://x.test/2.0.0/s.json","r":"^18.3.1","e":">=20.0.0","d":"version 1.2.3","t":" 1.2.3"}`, nil, nil},
		{"two components and IPv4", `{"py":"3.9","ip":"192.168.1.1"}`, nil, nil},
		{"a number is not a string", `{"v":1.2,"w":[1.5e999,-0.0]}`, nil, nil},
		{"an object key is never a candidate", `{"1.2.3":"x","2.0.0":{"3.0.0":"4.0.0"}}`, []located{{`"2.0.0"."3.0.0"`, "4.0.0"}}, nil},
		{"nested arrays", `{"a":[["1.0.0",["2.0.0"]],{"b":["3.0.0"]}]}`,
			[]located{{"a[0][0]", "1.0.0"}, {"a[0][1][0]", "2.0.0"}, {"a[1].b[0]", "3.0.0"}}, nil},
		{"root array", `["1.0.0",{"version":"2.0.0"}]`, []located{{"[0]", "1.0.0"}, {"[1].version", "2.0.0"}}, nil},
		{"root string", `"1.2.3"`, []located{{"", "1.2.3"}}, nil},
		{"duplicate keys are both candidates", `{"version":"1.2.3","version":"1.2.4"}`,
			[]located{{"version", "1.2.3"}, {"version", "1.2.4"}}, nil},
		{"escaped value is skipped", "{\"version\":\"1\\u002e2.3\",\"plain\":\"1.2.3\"}",
			[]located{{"plain", "1.2.3"}}, []string{"version=1.2.3"}},
		{"escaped digit", "{\"version\":\"\\u0031.2.3\"}", nil, []string{"version=1.2.3"}},
		{"escape elsewhere in the value", "{\"version\":\"1.2.3\\n\",\"w\":\" 1.2.3\"}", nil, nil},
		{"keys holding dots and odd characters", `{"a.b":{"@types/node":"1.2.3"},"":"4.5.6"}`,
			[]located{{`"a.b"."@types/node"`, "1.2.3"}, {`""`, "4.5.6"}}, nil},
		{"minified", `{"name":"app","version":"1.2.3","config":{"releaseTag":"1.2.3"},"files":["dist"]}`,
			[]located{{"version", "1.2.3"}, {"config.releaseTag", "1.2.3"}}, nil},
		{"tab-indented", "{\n\t\"version\":\t\"1.2.3\",\n\t\"deps\": {\n\t\t\"x\" :\t\"2.0.0\"\n\t}\n}\n",
			[]located{{"version", "1.2.3"}, {"deps.x", "2.0.0"}}, nil},
		{"CRLF", "{\r\n  \"version\": \"1.2.3\",\r\n  \"x\": [\r\n    \"2.0.0\"\r\n  ]\r\n}\r\n",
			[]located{{"version", "1.2.3"}, {"x[0]", "2.0.0"}}, nil},
		{"BOM", bomUTF8 + `{"version":"1.2.3"}`, []located{{"version", "1.2.3"}}, nil},
		{"BOM and whitespace", bomUTF8 + "\n  [ \"1.2.3\" ]\n", []located{{"[0]", "1.2.3"}}, nil},
		{"separators before a value", "{\"a\" :\n\n \"1.2.3\" , \"b\":[ \"2.0.0\" ,\"3.0.0\"]}",
			[]located{{"a", "1.2.3"}, {"b[0]", "2.0.0"}, {"b[1]", "3.0.0"}}, nil},
		{"empty containers", `{"a":{},"b":[],"c":"1.2.3"}`, []located{{"c", "1.2.3"}}, nil},
		{"other values beside a candidate", `{"n":1.5e999,"b":true,"z":null,"v":"1.2.3"}`, []located{{"v", "1.2.3"}}, nil},
		{"filename value", `{"asset":"app-1.2.3-rc.1.zip"}`, nil, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := []byte(tt.in)
			loc, err := ParserJSON.Locate(data)
			if err != nil {
				t.Fatalf("Locate: %v", err)
			}
			if got := locatedOf(t, data, loc.Candidates); !slices.Equal(got, tt.want) {
				t.Errorf("candidates = %v, want %v", got, tt.want)
			}
			for _, c := range loc.Candidates {
				if data[c.Start-1] != '"' || data[c.End] != '"' {
					t.Errorf("candidate %q at [%d,%d) is not a whole string value", data[c.Start:c.End], c.Start, c.End)
				}
			}
			var skipped []string
			for _, s := range loc.Skipped {
				skipped = append(skipped, s.Key.String()+"="+s.Value)
			}
			if !slices.Equal(skipped, tt.skipped) {
				t.Errorf("skipped = %v, want %v", skipped, tt.skipped)
			}
			assertRewritesCleanly(t, ParserJSON, data)
		})
	}
}

// A BOM is kept and every range sits past it, and a skipped value's line is
// counted the way discovery counts lines.
func TestLocateJSONOffsetsAndLines(t *testing.T) {
	data := []byte(bomUTF8 + "{\r\n\"a\": \"x\",\r\"esc\": \"1\\u002e2.3\",\n\"version\": \"1.2.3\"}")
	loc, err := ParserJSON.Locate(data)
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
	if len(loc.Skipped) != 1 || loc.Skipped[0].Line != 3 {
		t.Errorf("skipped = %+v, want one on line 3", loc.Skipped)
	}
}

func TestLocateJSONRejects(t *testing.T) {
	tests := []struct {
		name, in string
		line     int
		msg      string
	}{
		{"line comment", "{\n  \"a\": 1,\n  // note\n  \"version\": \"1.2.3\"\n}\n", 3, "invalid character '/'"},
		{"trailing comma", "{\n\"version\": \"1.2.3\",\n}", 3, "invalid character '}'"},
		{"two values", "{}\n{}\n", 2, "after top-level value"},
		{"empty", "", 1, "unexpected end of JSON input"},
		{"blank", "\n\n  \n", 3, "unexpected end of JSON input"},
		{"truncated", "{\"version\": \"1.2.3\"", 1, "unexpected end of JSON input"},
		{"BOM twice", bomUTF8 + bomUTF8 + "{}", 1, "invalid character"},
		{"UTF-16", "\xFF\xFE{\x00}\x00", 1, "invalid character"},
		{"bare version", "1.2.3\n", 1, "invalid character '.'"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParserJSON.Locate([]byte(tt.in))
			var pe *ParseError
			if !errors.As(err, &pe) {
				t.Fatalf("Locate error = %v, want a *ParseError", err)
			}
			if pe.Line != tt.line || !strings.Contains(pe.Msg, tt.msg) || pe.Parser != ParserJSON {
				t.Errorf("ParseError = %+v, want line %d and %q", pe, tt.line, tt.msg)
			}
			if !strings.HasPrefix(pe.Error(), fmt.Sprintf("not JSON (line %d: ", tt.line)) {
				t.Errorf("Error() = %q", pe.Error())
			}
		})
	}
}

func TestLocateTOML(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    []located
		skipped []string
	}{
		{"table key", "[package]\nversion = \"1.2.3\"\n", []located{{"package.version", "1.2.3"}}, nil},
		{"dotted key", "package.version = \"1.2.3\"\ntool . x . y = '2.0.0'\n",
			[]located{{"package.version", "1.2.3"}, {"tool.x.y", "2.0.0"}}, nil},
		{"quoted key holding a dot is one key", "\"a.b\" = \"1.2.3\"\n[c]\n\"d.e\".f = \"2.0.0\"\n",
			[]located{{`"a.b"`, "1.2.3"}, {`c."d.e".f`, "2.0.0"}}, nil},
		{"quoted key that is a version", "[releases]\n\"1.2.3\" = \"2024-01-01\"\n\"2.0.0\" = \"2.0.0\"\n",
			[]located{{`releases."2.0.0"`, "2.0.0"}}, nil},
		{"bare key that is a version", "[releases]\n1.2.3 = \"4.5.6\"\nv7.8.9 = { latest = \"7.8.9\" }\n",
			[]located{{"releases.1.2.3", "4.5.6"}, {"releases.v7.8.9.latest", "7.8.9"}}, nil},
		{"token welded into a bare key", "[x-1.2.3]\nversion = \"1.2.3\"\n[y.x-1.2.3-rc.1.z]\nv = \"2.0.0\"\n",
			[]located{{"x-1.2.3.version", "1.2.3"}, {"y.x-1.2.3-rc.1.z.v", "2.0.0"}}, nil},
		{"inline table", "[dependencies]\nserde = { version = \"1.0.200\", features = [\"derive\"] }\nanyhow = {version=\"1.0.86\"}\n",
			[]located{{"dependencies.serde.version", "1.0.200"}, {"dependencies.anyhow.version", "1.0.86"}}, nil},
		{"sub-table", "[dependencies.tokio]\nversion = \"1.37.0\"\nfeatures = [\"full\"]\n",
			[]located{{"dependencies.tokio.version", "1.37.0"}}, nil},
		{"arrays of tables", "[[bin]]\nname = \"a\"\nversion = \"1.0.0\"\n[[bin]]\nname = \"b\"\n[[bin]]\nversion = \"3.0.0\"\n",
			[]located{{"bin[0].version", "1.0.0"}, {"bin[2].version", "3.0.0"}}, nil},
		{"array of version strings", "supported = [\"1.0.0\", \"v2.0.0\", 'x 3.0.0', [\"4.0.0\"]]\n",
			[]located{{"supported[0]", "1.0.0"}, {"supported[1]", "v2.0.0"}, {"supported[3][0]", "4.0.0"}}, nil},
		{"URL", "homepage = \"https://example.com/1.2.3/\"\nversion = \"1.2.3\"\n", []located{{"version", "1.2.3"}}, nil},
		{"multi-line string holding a key as text", "notes = \"\"\"\nversion = \"1.2.3\"\n\"\"\"\nversion = \"2.0.0\"\n",
			[]located{{"version", "2.0.0"}}, nil},
		{"multi-line literal holding a key as text", "notes = '''\nversion = '1.2.3'\n'''\n", nil, nil},
		{"multi-line string that is a version", "v = \"\"\"\n1.2.3\"\"\"\nw = '''1.2.4'''\n",
			[]located{{"v", "1.2.3"}, {"w", "1.2.4"}}, nil},
		{"comment holding a version", "# released as 1.2.3\nversion = \"2.0.0\" # was 1.9.9\n",
			[]located{{"version", "2.0.0"}}, nil},
		{"comment token after an array of tables", "[[bin]]\nversion = \"1.0.0\" # 1.0.0\n# 2.0.0\n",
			[]located{{"bin[0].version", "1.0.0"}}, nil},
		{"literal string", "version = '1.2.3'\n", []located{{"version", "1.2.3"}}, nil},
		{"ranges and specifiers", "requires-python = \">=3.9\"\ndeps = [\"requests>=2.31.0\", \"click==8.1.7\", \"~1.2.3\"]\n", nil, nil},
		{"escaped value is skipped", "version = \"1\\u002e2.3\"\nplain = \"1.2.3\"\n",
			[]located{{"plain", "1.2.3"}}, []string{"version=1.2.3"}},
		{"line-ending backslash", "v = \"\"\"1.2.\\\n   3\"\"\"\n", nil, []string{"v=1.2.3"}},
		{"CRLF", "[package]\r\nversion = \"1.2.3\"\r\n# 9.9.9\r\n", []located{{"package.version", "1.2.3"}}, nil},
		{"BOM", bomUTF8 + "version = \"1.2.3\"\n", []located{{"version", "1.2.3"}}, nil},
		{"dates and floats", "d = 1979-05-27T07:32:00.999Z\nf = 1.5\nversion = \"1.2.3\"\n", []located{{"version", "1.2.3"}}, nil},
		{"empty", "", nil, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := []byte(tt.in)
			loc, err := ParserTOML.Locate(data)
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
			assertRewritesCleanly(t, ParserTOML, data)
		})
	}
}

func TestLocateTOMLRejects(t *testing.T) {
	tests := []struct {
		name, in string
		line     int
	}{
		{"JSON", "{\"version\": \"1.2.3\"}\n", 1},
		{"missing value", "a = 1\nversion =\n", 2},
		{"duplicate key", "version = \"1.2.3\"\nversion = \"1.2.4\"\n", 2},
		{"bare version value", "\n\nversion = 1.2.3\n", 3},
		{"NUL", "\x00version = \"1.2.3\"", 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParserTOML.Locate([]byte(tt.in))
			var pe *ParseError
			if !errors.As(err, &pe) {
				t.Fatalf("Locate error = %v, want a *ParseError", err)
			}
			if pe.Line != tt.line || pe.Probe || pe.Parser != ParserTOML {
				t.Errorf("ParseError = %+v, want line %d", pe, tt.line)
			}
			if !strings.HasPrefix(pe.Error(), fmt.Sprintf("not TOML (line %d: ", tt.line)) {
				t.Errorf("Error() = %q", pe.Error())
			}
		})
	}
}

// The probe's answer is checked against the file before it is used. Where the
// decoder accepts TOML the specification forbids, masking a token in a key can
// change which of two colliding definitions it keeps, and a version naming an
// array of tables declared twice splits it in two; either is refused, like a
// file that does not parse. A table named by a version that is only extended
// splits harmlessly and is placed exactly.
func TestLocateTOMLProbeMustAgree(t *testing.T) {
	refused := map[string]string{
		"dotted key then the same key": "1.0.0 = \"x\"\n\"1\" = \"1.2.3\"\n",
		"bare key then the same key":   "[t]\nv1.2.3.a = \"x\"\nv1 = \"4.5.6\"\n",
		"array of tables named twice":  "[[1.2.3]]\nv = \"1.0.0\"\n[[1.2.3]]\nv = \"2.0.0\"\n",
	}
	for name, in := range refused {
		t.Run(name, func(t *testing.T) {
			_, err := ParserTOML.Locate([]byte(in))
			var pe *ParseError
			if !errors.As(err, &pe) || !pe.Probe {
				t.Fatalf("Locate error = %v, want the probe's refusal", err)
			}
			if !strings.HasPrefix(pe.Error(), "TOML whose versions incrmit cannot place (") {
				t.Errorf("Error() = %q", pe.Error())
			}
		})
	}

	data := []byte("[1.2.3]\nx = \"1.0.0\"\n[1.2.3.y]\nz = \"2.0.0\"\n[t]\n4.5.6.a = \"3.0.0\"\n4.5.6.b = \"4.0.0\"\n")
	loc, err := ParserTOML.Locate(data)
	if err != nil {
		t.Fatal(err)
	}
	want := []located{{"1.2.3.x", "1.0.0"}, {"1.2.3.y.z", "2.0.0"}, {"t.4.5.6.a", "3.0.0"}, {"t.4.5.6.b", "4.0.0"}}
	if got := locatedOf(t, data, loc.Candidates); !slices.Equal(got, want) {
		t.Errorf("candidates = %v, want %v", got, want)
	}
	assertRewritesCleanly(t, ParserTOML, data)
}

func TestParseErrorMessage(t *testing.T) {
	tests := []struct {
		err  *ParseError
		want string
	}{
		{&ParseError{Parser: ParserJSON, Line: 3, Msg: "invalid character '/'"}, "not JSON (line 3: invalid character '/')"},
		{&ParseError{Parser: ParserJSON, Msg: "unexpected EOF"}, "not JSON (unexpected EOF)"},
		{&ParseError{Parser: ParserTOML, Line: 2, Msg: "expected value"}, "not TOML (line 2: expected value)"},
		{&ParseError{Parser: ParserTOML, Line: 4, Msg: "duplicate key", Probe: true}, "TOML whose versions incrmit cannot place (line 4: duplicate key)"},
	}
	for _, tt := range tests {
		if got := tt.err.Error(); got != tt.want {
			t.Errorf("Error() = %q, want %q", got, tt.want)
		}
	}
}

// The probe gives every token a placeholder of its own. With more than ten
// tokens an unpadded counter would make one placeholder a prefix of another
// (...1 of ...10), and the prototype that had that bug reported a comment's
// token as part of bin[0].version. Every candidate here must sit at its own
// key, and no token in a comment may be one.
func TestLocateTOMLManyTokens(t *testing.T) {
	var b strings.Builder
	var want []located
	for i := 1; i <= 25; i++ {
		fmt.Fprintf(&b, "# release 1.%d.0 notes\n[[bin]]\nversion = \"1.%d.1\" # was 1.%d.0\n", i, i, i)
		want = append(want, located{fmt.Sprintf("bin[%d].version", i-1), fmt.Sprintf("1.%d.1", i)})
	}
	data := []byte(b.String())
	loc, err := ParserTOML.Locate(data)
	if err != nil {
		t.Fatal(err)
	}
	if got := locatedOf(t, data, loc.Candidates); !slices.Equal(got, want) {
		t.Errorf("candidates = %v\nwant %v", got, want)
	}
	assertRewritesCleanly(t, ParserTOML, data)
}

// A file can already hold the placeholder's digits, in a value, a key, a
// comment, or an escape that only decodes to them. The probe then chooses
// other digits, and still places every candidate exactly.
func TestLocateTOMLPlaceholderCollisions(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []located
	}{
		{"placeholder text in a comment", "# 987654321.987654321.000000001\nversion = \"1.2.3\"\n",
			[]located{{"version", "1.2.3"}}},
		{"placeholder text in a value", "id = \"987654321987654321000000001\"\nversion = \"1.2.3\"\n",
			[]located{{"version", "1.2.3"}}},
		{"placeholder digits as a key", "987654321 = { 987654321 = \"1.2.3\" }\nversion = \"4.5.6\"\n",
			[]located{{"987654321.987654321", "1.2.3"}, {"version", "4.5.6"}}},
		{"placeholder digits only once decoded", "k = \"9\\u003876543210\"\n[t]\nversion = \"1.2.3\"\n",
			[]located{{"t.version", "1.2.3"}}},
		{"placeholder digits in a quoted key with an escape", "\"9\\u003876543210\" = \"1.2.3\"\n",
			[]located{{"9876543210", "1.2.3"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := []byte(tt.in)
			loc, err := ParserTOML.Locate(data)
			if err != nil {
				t.Fatalf("Locate: %v", err)
			}
			if got := locatedOf(t, data, loc.Candidates); !slices.Equal(got, tt.want) {
				t.Errorf("candidates = %v, want %v", got, tt.want)
			}
			assertRewritesCleanly(t, ParserTOML, data)
		})
	}
}

// absentDigits finds digits the text does not hold even when a file is
// planted with the likely choices, in a bounded number of passes.
func TestAbsentDigits(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 2000; i++ {
		fmt.Fprintf(&b, "# %09d %09d\n", 987654321-i, i)
	}
	b.WriteString("version = \"1.2.3\"\n")
	data := []byte(b.String())
	doc, err := decodeTOML(data)
	if err != nil {
		t.Fatal(err)
	}
	got := absentDigits(data, doc)
	if len(got) != 9 || strings.Contains(string(data), got) {
		t.Fatalf("absentDigits = %q, which the file holds or is not nine digits", got)
	}
	loc, err := ParserTOML.Locate(data)
	if err != nil || len(loc.Candidates) != 1 || loc.Candidates[0].Key.String() != "version" {
		t.Fatalf("Locate = %+v, %v", loc, err)
	}
}

// All tokens share one probe decode. One decode per token took 2.8 s for a
// thousand tokens in the prototype and grows quadratically, so a file of
// several thousand would take minutes; one decode takes milliseconds.
func TestLocateTOMLIsLinearInTokens(t *testing.T) {
	n := 5000
	if testing.Short() {
		n = 1000
	}
	var b strings.Builder
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, "[[pkg]]\nname = \"p%d\" # 1.%d.0\nversion = \"2.%d.0\"\n", i, i, i)
	}
	data := []byte(b.String())
	testutil.Within(t, "locating versions in a TOML file of many tokens", func() {
		loc, err := ParserTOML.Locate(data)
		if err != nil {
			t.Error(err)
			return
		}
		if len(loc.Candidates) != n {
			t.Errorf("found %d candidates, want %d", len(loc.Candidates), n)
		}
	})
}

// Rewrite's check is what catches a parser placing a version wrongly: a
// candidate that is really a comment's token, or one recorded at the wrong
// key, would have the bump change something it did not plan, or fail to
// change what it did. Both are refused rather than written.
func TestRewriteRefusesUnplannedChanges(t *testing.T) {
	toml := []byte("[[bin]]\nversion = \"1.2.3\" # 1.2.3\nother = \"1.2.3\"\n")
	loc, err := ParserTOML.Locate(toml)
	if err != nil || len(loc.Candidates) != 2 {
		t.Fatalf("Locate = %+v, %v", loc, err)
	}
	bump := []Replacement{{Old: v123, New: v124}}

	comment := strings.Index(string(toml), "# 1.2.3") + 2
	tests := []struct {
		name  string
		p     Parser
		data  []byte
		cands []Candidate
		msg   string
	}{
		{"comment token reported as a value", ParserTOML, toml,
			[]Candidate{{Start: comment, End: comment + 5, Version: v123, Key: KeyPath{keyStep("bin"), indexStep(0), keyStep("version")}}},
			"did not change bin[0].version"},
		{"value reported at the wrong key", ParserTOML, toml,
			[]Candidate{{Start: loc.Candidates[0].Start, End: loc.Candidates[0].End, Version: v123, Key: KeyPath{keyStep("other")}}},
			"would change bin[0].version"},
		{"JSON value reported at the wrong key", ParserJSON, []byte(`{"a":"1.2.3","b":"1.2.3"}`),
			[]Candidate{{Start: 6, End: 11, Version: v123, Key: KeyPath{keyStep("b")}}},
			"would change a"},
		{"JSON key reported as a value", ParserJSON, []byte(`{"1.2.3":"x"}`),
			[]Candidate{{Start: 2, End: 7, Version: v123, Key: KeyPath{keyStep("1.2.3")}}},
			"changed the document's keys"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, _, err := Rewrite(tt.p, tt.data, tt.cands, bump)
			if err == nil || !strings.Contains(err.Error(), tt.msg) {
				t.Fatalf("Rewrite = %q, %v; want an error containing %q", out, err, tt.msg)
			}
		})
	}

	// A rewrite whose result is no longer the format, or whose keys change
	// shape, is refused too. Both need a range no parser reports (a token in
	// a table header, or a bare key), so they are built by hand.
	headers := []byte("[a.1.2.3]\nx = 1\n[a.1.2.4]\ny = 2\n")
	header := Candidate{Start: 3, End: 8, Version: v123, Key: KeyPath{keyStep("a")}}
	if _, _, err := Rewrite(ParserTOML, headers, []Candidate{header}, bump); err == nil || !strings.Contains(err.Error(), "no longer TOML") {
		t.Errorf("Rewrite into a duplicate table: %v", err)
	}
	bare := []byte("[t]\n1.2.3 = \"x\"\n")
	pre := version.Version{Major: 1, Minor: 2, Patch: 3, Prerelease: "rc.1"}
	if _, _, err := Rewrite(ParserTOML, bare, []Candidate{{Start: 4, End: 9, Version: v123}}, []Replacement{{Old: v123, New: pre}}); err == nil || !strings.Contains(err.Error(), "shape") {
		t.Errorf("Rewrite of a bare key into more keys: %v", err)
	}
	root := []byte(`"1.2.3"`)
	if _, _, err := Rewrite(ParserJSON, root, []Candidate{{Start: 1, End: 6, Version: v123, Key: KeyPath{keyStep("x")}}}, bump); err == nil || !strings.Contains(err.Error(), "the document's root value") {
		t.Errorf("Rewrite of the root value planned elsewhere: %v", err)
	}
	for _, p := range []Parser{ParserJSON, ParserTOML} {
		if _, _, err := Rewrite(p, []byte("1.2.3 = {"), []Candidate{{Start: 0, End: 5, Version: v123}}, bump); err == nil {
			t.Errorf("%s: Rewrite of input that does not parse succeeded", p)
		}
	}
	if _, _, err := Rewrite(Parser("yaml"), []byte("1.2.3"), []Candidate{{Start: 0, End: 5, Version: v123}}, bump); err == nil {
		t.Error("Rewrite with an unknown parser succeeded")
	}

	// The real candidates pass the same check.
	if _, _, err := Rewrite(ParserTOML, toml, loc.Candidates, bump); err != nil {
		t.Errorf("Rewrite of the located candidates: %v", err)
	}
}

// ReadVersion and ApplyBump read a file with the parser its name selects, and
// a file that parser cannot read is an error rather than scanned as text.
func TestReadVersionAndApplyBumpUseTheFilesParser(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	pkg := write("package.json", `{"version":"1.2.3","engines":{"node":">=20.0.0"}}`)
	if v, err := ReadVersion(pkg); err != nil || v != v123 {
		t.Errorf("ReadVersion = %v, %v; want 1.2.3 (the range is not a candidate)", v, err)
	}
	if _, _, err := ApplyBump(pkg, version.Version.BumpPatch, false); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(pkg); string(got) != `{"version":"1.2.4","engines":{"node":">=20.0.0"}}` {
		t.Errorf("ApplyBump wrote %s", got)
	}
	bad := write("tsconfig.json", "{ // c\n\"version\": \"1.2.3\" }")
	var pe *ParseError
	if _, err := ReadVersion(bad); !errors.As(err, &pe) {
		t.Errorf("ReadVersion of a file that is not JSON: %v", err)
	}
	if _, _, err := ApplyBump(bad, version.Version.BumpPatch, false); !errors.As(err, &pe) {
		t.Errorf("ApplyBump of a file that is not JSON: %v", err)
	}
}

// The text parser is version.FindTokens plus version.Parse and nothing else,
// so every file that is not JSON or TOML is read exactly as before.
func TestTextCandidatesAreTheScannersTokens(t *testing.T) {
	data := []byte("v1.2.3 192.168.1.1 3.9 app-1.2.3-rc.1.zip rev1.2.3 1.2.3-rc.1 1.02.3 {\"version\":\"4.5.6\"}")
	var want []Candidate
	for _, loc := range version.FindTokens(data) {
		if v, err := version.Parse(string(data[loc[0]:loc[1]])); err == nil {
			want = append(want, Candidate{Start: loc[0], End: loc[1], Version: v})
		}
	}
	loc, err := ParserText.Locate(data)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.EqualFunc(loc.Candidates, want, func(a, b Candidate) bool {
		return a.Start == b.Start && a.End == b.End && a.Version == b.Version && a.Key == nil
	}) {
		t.Errorf("text candidates = %+v, want %+v", loc.Candidates, want)
	}
	if _, err := Parser("yaml").Locate(data); err == nil {
		t.Error("an unknown parser located candidates")
	}
}
