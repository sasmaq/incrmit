package files

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
	"go.yaml.in/yaml/v3"
)

// The parser fuzz targets state what the JSON locator and the TOML and YAML
// probes promise, and check it against the decoder itself rather than against
// the parser's own walk: for any input the decoder accepts, every candidate's
// bytes are exactly the string the decoder reports at the candidate's key,
// and rewriting one candidate with a version of a different length, then
// decoding again, changes that key and no other. A range off by a byte, a
// candidate taken from a comment or a key, or a key path mapped back to the
// wrong key each break one of the two. Input the decoder rejects must be
// refused, never panicked on.
//
// Every candidate's key path, written out and read back as a key (ParseKey),
// must also claim exactly that candidate's range, unless the document holds
// more than one value at the path, as a JSON key written twice does, which no
// key can name and Claim must refuse. That is what discover relies on when it
// writes a candidate's key into a config.
//
// The seeds are the golden fixtures (parser_test.go) and the shapes that
// matter to each reader.

// maxRewrites bounds how many candidates of one input are rewritten and
// decoded again, so a large input costs the fuzzer a bounded number of
// decodes.
const maxRewrites = 8

// assertKeyResolves checks that c's key path, rendered and read back as a key
// for p, claims c's range alone in the file loc was located in, or, where the
// document holds n > 1 values at that path, is refused as naming no one value.
// A candidate with no key (a document that is one string) has nothing to read
// back.
func assertKeyResolves(t *testing.T, p Parser, loc *Located, c Candidate, n int, data []byte) {
	t.Helper()
	if len(c.Key) == 0 {
		return
	}
	k, err := ParseKey(p, c.Key.String())
	if err != nil {
		t.Fatalf("the key %s reported in %q does not read back: %v", c.Key, data, err)
	}
	at, err := loc.Claim(k, c.Version)
	if n > 1 {
		var keyErr *KeyError
		if !errors.As(err, &keyErr) || !strings.Contains(err.Error(), "no one value") {
			t.Fatalf("the key %s names %d values in %q, but Claim gave %v, %v", c.Key, n, data, at, err)
		}
		return
	}
	if err != nil || len(at) != 1 || at[0] != c.Start {
		t.Fatalf("the key %s claims %v, %v in %q, want the candidate at %d", c.Key, at, err, data, c.Start)
	}
}

// addFixtureSeeds adds every golden fixture the parser reads to f's corpus.
func addFixtureSeeds(f *testing.F, p Parser) {
	for _, fx := range parserFixtures {
		if ParserFor(strings.TrimSuffix(fx.input, ".input")) != p {
			continue
		}
		data, err := os.ReadFile(filepath.Join("testdata", fx.input))
		if err != nil {
			f.Fatal(err)
		}
		f.Add(data)
	}
}

// jsonLeaf is one scalar of a JSON document as the reference reads it.
type jsonLeaf struct {
	path  string
	value any
}

// refJSON reads body with encoding/json's Decoder, recursively and
// independently of walkJSON, and returns every scalar value in document
// order with its path, so a duplicate key's two values are two leaves.
func refJSON(t *testing.T, body []byte) []jsonLeaf {
	t.Helper()
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	var leaves []jsonLeaf
	var value func(path KeyPath)
	value = func(path KeyPath) {
		tok, err := dec.Token()
		if err != nil {
			t.Fatalf("reference decode: %v", err)
		}
		switch tok {
		case json.Delim('{'):
			for dec.More() {
				k, err := dec.Token()
				if err != nil {
					t.Fatalf("reference decode: %v", err)
				}
				value(append(append(KeyPath(nil), path...), keyStep(k.(string))))
			}
			_, _ = dec.Token()
			leaves = append(leaves, jsonLeaf{path.String(), "{}"})
		case json.Delim('['):
			for i := 0; dec.More(); i++ {
				value(append(append(KeyPath(nil), path...), indexStep(i)))
			}
			_, _ = dec.Token()
			leaves = append(leaves, jsonLeaf{path.String(), "[]"})
		default:
			if n, ok := tok.(json.Number); ok {
				tok = "number " + string(n)
			}
			leaves = append(leaves, jsonLeaf{path.String(), tok})
		}
	}
	value(nil)
	if _, err := dec.Token(); err != io.EOF {
		t.Fatalf("reference decode: trailing data after the value")
	}
	return leaves
}

func FuzzLocateJSON(f *testing.F) {
	addFixtureSeeds(f, ParserJSON)
	f.Add([]byte(`{"version":"1.2.3","version":"1.2.3"}`))
	f.Add([]byte(bomUTF8 + "{\r\n\t\"a\": [\"1.2.3\", {\"b\": \"v2.0.0-rc.1\"}]\r\n}"))
	f.Add([]byte(`{"esc":"1\u002e2.3","1.2.3":"x","url":"https://x.test/1.2.3/"}`))
	f.Add([]byte(`["1.2.3",[["4.5.6"]],"",{}]`))
	f.Add([]byte(`"1.2.3"`))
	f.Add([]byte("{\n  // comment 1.2.3\n}"))
	f.Add([]byte(`{"a":{"v":"1.2.3"},"a":{"v":"1.2.3"},"a.v":"1.2.3","\"":{"\u0001":"2.0.0"}}`))

	f.Fuzz(func(t *testing.T, data []byte) {
		loc, err := ParserJSON.Locate(data)
		_, body := splitBOM(data)
		if !json.Valid(body) {
			if err == nil {
				t.Fatalf("Locate accepted input encoding/json rejects: %q", data)
			}
			return
		}
		if err != nil {
			t.Fatalf("Locate refused valid JSON %q: %v", data, err)
		}

		leaves := refJSON(t, body)
		for _, c := range loc.Candidates {
			tok := string(data[c.Start:c.End])
			if tok != c.Version.String() {
				t.Fatalf("candidate bytes %q are not its version %s in %q", tok, c.Version, data)
			}
			found := false
			for _, l := range leaves {
				found = found || (l.path == c.Key.String() && l.value == tok)
			}
			if !found {
				t.Fatalf("no string %q at %s in %q", tok, c.Key, data)
			}
			n := 0
			for _, l := range leaves {
				if l.path == c.Key.String() {
					n++
				}
			}
			assertKeyResolves(t, ParserJSON, loc, c, n, data)
		}

		for _, c := range loc.Candidates[:min(len(loc.Candidates), maxRewrites)] {
			tok := string(data[c.Start:c.End])
			repl := tok + "0"
			out := append(append(append([]byte(nil), data[:c.Start]...), repl...), data[c.End:]...)
			_, outBody := splitBOM(out)
			if !json.Valid(outBody) {
				t.Fatalf("rewriting %s made %q invalid JSON", c.Key, out)
			}
			after := refJSON(t, outBody)
			if len(after) != len(leaves) {
				t.Fatalf("rewriting %s changed the document's shape: %q", c.Key, out)
			}
			changed := 0
			for i := range leaves {
				if leaves[i] == after[i] {
					continue
				}
				changed++
				if leaves[i].path != c.Key.String() || leaves[i].value != tok || after[i] != (jsonLeaf{c.Key.String(), repl}) {
					t.Fatalf("rewriting %s changed %v to %v in %q", c.Key, leaves[i], after[i], data)
				}
			}
			if changed != 1 {
				t.Fatalf("rewriting %s changed %d values in %q", c.Key, changed, data)
			}
		}
	})
}

// tomlAt returns the value at path in a decoded TOML document.
func tomlAt(doc any, path KeyPath) (any, bool) {
	v := doc
	for _, s := range path {
		switch t := v.(type) {
		case map[string]any:
			if s.Index >= 0 {
				return nil, false
			}
			e, ok := t[s.Name]
			if !ok {
				return nil, false
			}
			v = e
		case []map[string]any:
			if s.Index < 0 || s.Index >= len(t) {
				return nil, false
			}
			v = t[s.Index]
		case []any:
			if s.Index < 0 || s.Index >= len(t) {
				return nil, false
			}
			v = t[s.Index]
		default:
			return nil, false
		}
	}
	return v, true
}

// refTOML flattens a decoded TOML document to path -> scalar, with tables and
// arrays as markers, independently of walkTOML. TOML has no duplicate keys,
// so a path names one value.
func refTOML(v any, path KeyPath, out map[string]any) {
	switch t := v.(type) {
	case map[string]any:
		out[path.String()] = "table of " + itoa(len(t))
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			refTOML(t[k], append(append(KeyPath(nil), path...), keyStep(k)), out)
		}
	case []map[string]any:
		out[path.String()] = "tables of " + itoa(len(t))
		for i, e := range t {
			refTOML(e, append(append(KeyPath(nil), path...), indexStep(i)), out)
		}
	case []any:
		out[path.String()] = "array of " + itoa(len(t))
		for i, e := range t {
			refTOML(e, append(append(KeyPath(nil), path...), indexStep(i)), out)
		}
	default:
		out[path.String()] = v
	}
}

func itoa(n int) string { return strings.Repeat("|", n) }

func FuzzLocateTOML(f *testing.F) {
	addFixtureSeeds(f, ParserTOML)
	f.Add([]byte("[releases]\n1.2.3 = \"4.5.6\"\n\"1.2.3\" = \"1.2.3\"\n[x-1.2.3]\nv = \"1.0.0\" # 1.0.0\n"))
	f.Add([]byte("[[bin]]\nversion = \"1.0.0\"\n[[bin]]\nversion = [\"2.0.0\", { v = \"3.0.0\" }]\n"))
	f.Add([]byte("notes = \"\"\"\nversion = \"1.2.3\"\n\"\"\"\nv = '''1.2.4'''\nesc = \"1\\u002e2.3\"\n"))
	f.Add([]byte("987654321 = { 987654321 = \"1.2.3\" }\n# 987654321.987654321.000000001\n"))
	f.Add([]byte(bomUTF8 + "a.b.\"c.d\" = \"v1.2.3-rc.1+b.5\"\r\n"))
	f.Add([]byte("\"\\u0001\\\\ \\\"\" = { \"\" = [\"1.2.3\", [\"2.0.0\"]] }\n"))

	f.Fuzz(func(t *testing.T, data []byte) {
		loc, err := ParserTOML.Locate(data)
		var doc map[string]any
		if _, derr := toml.Decode(string(data), &doc); derr != nil {
			if err == nil {
				t.Fatalf("Locate accepted input the decoder rejects (%v): %q", derr, data)
			}
			return
		}
		if err != nil {
			// The one refusal allowed for TOML the decoder accepts is the
			// probe's own (see locateTOML).
			var pe *ParseError
			if !errors.As(err, &pe) || !pe.Probe {
				t.Fatalf("Locate refused TOML the decoder accepts, %q: %v", data, err)
			}
			return
		}

		for _, c := range loc.Candidates {
			tok := string(data[c.Start:c.End])
			if tok != c.Version.String() {
				t.Fatalf("candidate bytes %q are not its version %s in %q", tok, c.Version, data)
			}
			if v, ok := tomlAt(doc, c.Key); !ok || v != tok {
				t.Fatalf("value at %s is %#v (found %v), want the candidate's %q, in %q", c.Key, v, ok, tok, data)
			}
			// TOML has no key written twice, so a path names one value.
			assertKeyResolves(t, ParserTOML, loc, c, 1, data)
		}

		before := map[string]any{}
		refTOML(doc, nil, before)
		for _, c := range loc.Candidates[:min(len(loc.Candidates), maxRewrites)] {
			tok := string(data[c.Start:c.End])
			repl := tok + "0"
			out := append(append(append([]byte(nil), data[:c.Start]...), repl...), data[c.End:]...)
			var outDoc map[string]any
			if _, err := toml.Decode(string(out), &outDoc); err != nil {
				t.Fatalf("rewriting %s made %q invalid TOML: %v", c.Key, out, err)
			}
			after := map[string]any{}
			refTOML(outDoc, nil, after)
			if len(after) != len(before) {
				t.Fatalf("rewriting %s changed the document's shape: %q", c.Key, out)
			}
			for k, bv := range before {
				av, ok := after[k]
				if !ok {
					t.Fatalf("rewriting %s removed %s from %q", c.Key, k, data)
				}
				if leafEqual(av, bv) {
					continue
				}
				if k != c.Key.String() || bv != tok || av != repl {
					t.Fatalf("rewriting %s changed %s from %#v to %#v in %q", c.Key, k, bv, av, data)
				}
			}
			if av := after[c.Key.String()]; av != repl {
				t.Fatalf("rewriting %s left it %#v, want %q", c.Key, av, repl)
			}
		}
	})
}

// yamlRefLeaf is one node of a YAML document as the reference reads it: the
// path a key names it by, or "?" for a node no key names, and what it holds.
type yamlRefLeaf struct {
	path, value string
}

// refYAML decodes data with the decoder alone and returns how many documents
// it holds, every node of the first in document order, and whether the
// decoder accepted the stream. It reads the tree independently of walkYAML:
// a scalar key adds a step, a key that is not a scalar puts what is in and
// under it out of any key's reach, and an alias is the anchor it names.
func refYAML(data []byte) (docs int, leaves []yamlRefLeaf, ok bool) {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	for {
		var doc yaml.Node
		err := dec.Decode(&doc)
		if errors.Is(err, io.EOF) {
			return docs, leaves, true
		}
		if err != nil {
			return 0, nil, false
		}
		docs++
		if docs > 1 || len(doc.Content) == 0 {
			continue
		}
		// path is nil for a node no key names.
		var node func(n *yaml.Node, path KeyPath)
		node = func(n *yaml.Node, path KeyPath) {
			where := "?"
			if path != nil {
				where = path.String()
			}
			step := func(s KeyStep) KeyPath {
				if path == nil {
					return nil
				}
				return append(append(KeyPath{}, path...), s)
			}
			switch n.Kind {
			case yaml.MappingNode:
				leaves = append(leaves, yamlRefLeaf{where, "mapping of " + itoa(len(n.Content)/2)})
				for i := 0; i+1 < len(n.Content); i += 2 {
					k, v := n.Content[i], n.Content[i+1]
					node(k, nil)
					if k.Kind == yaml.ScalarNode {
						node(v, step(keyStep(k.Value)))
					} else {
						node(v, nil)
					}
				}
			case yaml.SequenceNode:
				leaves = append(leaves, yamlRefLeaf{where, "sequence of " + itoa(len(n.Content))})
				for i, e := range n.Content {
					node(e, step(indexStep(i)))
				}
			case yaml.AliasNode:
				leaves = append(leaves, yamlRefLeaf{where, "alias of " + n.Value})
			default:
				leaves = append(leaves, yamlRefLeaf{where, n.ShortTag() + " " + strconv.Itoa(int(n.Style)) + " " + n.Value})
			}
		}
		node(doc.Content[0], KeyPath{})
	}
}

func FuzzLocateYAML(f *testing.F) {
	addFixtureSeeds(f, ParserYAML)
	f.Add([]byte("base: &v 1.2.3\npinned: *v\ndefaults: &d\n  version: 2.0.0\nprod:\n  <<: *d\n"))
	f.Add([]byte("a: !!str 1.2.3\nb: !semver 1.2.3\nc: '1.2.3'\nd: \"1.2\\x2e3\"\ne: |-\n  1.2.3\n"))
	f.Add([]byte("version: 1.2.3\nversion: 1.2.3\n? [1.2.3]\n: 1.2.3\n1.2.3: x\n"))
	f.Add([]byte("{a: [1.2.3, {b: v2.0.0-rc.1}], \"c.d\": 3.0.0} # 4.0.0\n"))
	f.Add([]byte(bomUTF8 + "a:\r\n  - 1.2.3\r\n"))
	f.Add([]byte("a: 1.2.3\n---\nb: 1.2.3\n"))
	f.Add([]byte("987654321: {987654321: 1.2.3}\n# 987654321.987654321.000000001\n"))
	f.Add([]byte("1.0.0 1.0.1 1.0.2 1.0.3 1.0.4 1.0.5 1.0.6 1.0.7 1.0.8 1.0.9 1.0.10 1.0.11 1.0.12 1.0.13 1.0.14 1.0.15 1.0.16 1.0.17 1.0.18: x\n"))

	f.Fuzz(func(t *testing.T, data []byte) {
		loc, err := ParserYAML.Locate(data)
		docs, leaves, ok := refYAML(data)
		var pe *ParseError
		switch {
		case !ok:
			if err == nil {
				t.Fatalf("Locate accepted input the decoder rejects: %q", data)
			}
			return
		case docs > 1:
			if !errors.As(err, &pe) || pe.Docs != docs {
				t.Fatalf("Locate of %d documents gave %v, want the documents refused: %q", docs, err, data)
			}
			return
		case err != nil:
			// The one refusal allowed for one document the decoder
			// accepts is the probe's own (see locateYAML).
			if !errors.As(err, &pe) || !pe.Probe {
				t.Fatalf("Locate refused YAML the decoder accepts, %q: %v", data, err)
			}
			return
		}

		for _, c := range loc.Candidates {
			tok := string(data[c.Start:c.End])
			if tok != c.Version.String() {
				t.Fatalf("candidate bytes %q are not its version %s in %q", tok, c.Version, data)
			}
			found, n := false, 0
			for _, l := range leaves {
				if l.path != c.Key.String() {
					continue
				}
				n++
				for _, style := range []yaml.Style{0, yaml.TaggedStyle, yaml.SingleQuotedStyle, yaml.DoubleQuotedStyle, yaml.TaggedStyle | yaml.SingleQuotedStyle, yaml.TaggedStyle | yaml.DoubleQuotedStyle} {
					found = found || l.value == "!!str "+strconv.Itoa(int(style))+" "+tok
				}
			}
			if !found {
				t.Fatalf("no plain or quoted string %q at %s in %q", tok, c.Key, data)
			}
			assertKeyResolves(t, ParserYAML, loc, c, n, data)
		}

		for _, c := range loc.Candidates[:min(len(loc.Candidates), maxRewrites)] {
			tok := string(data[c.Start:c.End])
			repl := tok + "0"
			out := append(append(append([]byte(nil), data[:c.Start]...), repl...), data[c.End:]...)
			outDocs, after, ok := refYAML(out)
			if !ok || outDocs != 1 {
				t.Fatalf("rewriting %s made %q something other than one YAML document", c.Key, out)
			}
			if len(after) != len(leaves) {
				t.Fatalf("rewriting %s changed the document's shape: %q", c.Key, out)
			}
			changed := 0
			for i := range leaves {
				if leaves[i] == after[i] {
					continue
				}
				changed++
				b, a := leaves[i], after[i]
				if b.path != c.Key.String() || a.path != b.path || !strings.HasSuffix(b.value, " "+tok) ||
					strings.TrimSuffix(b.value, tok) != strings.TrimSuffix(a.value, repl) {
					t.Fatalf("rewriting %s changed %v to %v in %q", c.Key, b, a, data)
				}
			}
			if changed != 1 {
				t.Fatalf("rewriting %s changed %d values in %q", c.Key, changed, data)
			}
		}
	})
}
