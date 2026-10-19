package files

import (
	"errors"
	"fmt"
	"math"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/sasmaq/incrmit/internal/version"
)

// Parser names how a file's bytes are read for versions. Every command takes
// the candidates it records, finds, or rewrites from Parser.Locate, so what
// discovery records is exactly what a bump rewrites, whichever parser read
// the file (doc/DEVELOPMENT.md section 9).
//
// The text parser is version.FindTokens plus version.Parse: every token Parse
// accepts is a candidate, wherever it sits. The JSON and TOML parsers read the
// format, so a candidate is a string value whose whole content is one such
// token, and each one knows the key it sits under. Their candidates are
// always a subset of the text parser's, with the same boundaries, so §9's
// single definition of a token holds in every file.
type Parser string

// The parsers a config's `parser` key can name.
const (
	ParserText Parser = "text"
	ParserJSON Parser = "json"
	ParserTOML Parser = "toml"
)

// ParserFor returns the parser a file gets by its name: JSON for a .json
// file, TOML for a .toml file, and text for anything else. The extension is
// compared ignoring case, so PACKAGE.JSON is JSON too.
func ParserFor(path string) Parser {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".json":
		return ParserJSON
	case ".toml":
		return ParserTOML
	default:
		return ParserText
	}
}

// ParseParser returns the parser a config's `parser` value names. The value
// is matched exactly, so "JSON" is an error rather than a guess.
func ParseParser(name string) (Parser, error) {
	switch p := Parser(name); p {
	case ParserText, ParserJSON, ParserTOML:
		return p, nil
	default:
		return "", fmt.Errorf("unknown parser %q; use %q, %q, or %q", name, ParserJSON, ParserTOML, ParserText)
	}
}

// label is the format's name as messages print it.
func (p Parser) label() string {
	return strings.ToUpper(string(p))
}

// Candidate is one version a parser found in a file: the token's byte range
// [Start, End), its parsed version, and the key it sits under. Key is nil
// from the text parser, which reads no structure.
type Candidate struct {
	Start, End int
	Version    version.Version
	Key        KeyPath
}

// Skipped is a string value that holds a version only once decoded, such as
// "1\u002e2.3" in JSON. It is not a candidate: rewriting it would mean
// re-encoding the string, and incrmit replaces a version's bytes and nothing
// else. Line is the 1-based line it is on, or 0 when the parser cannot tell
// (TOML).
type Skipped struct {
	Key   KeyPath
	Line  int
	Value string
}

// Located is what a parser found in a file: its candidates in byte order,
// and the values it skipped.
type Located struct {
	Candidates []Candidate
	Skipped    []Skipped
}

// Locate reads data with p and returns what it found. The text parser always
// succeeds. The JSON and TOML parsers return a *ParseError for data that is
// not their format, and the TOML parser also for TOML its probe cannot place
// versions in (see locateTOML).
func (p Parser) Locate(data []byte) (*Located, error) {
	switch p {
	case ParserText:
		return &Located{Candidates: TextCandidates(data)}, nil
	case ParserJSON:
		return locateJSON(data)
	case ParserTOML:
		return locateTOML(data)
	default:
		return nil, fmt.Errorf("files: unknown parser %q", string(p))
	}
}

// TextCandidates returns the text parser's candidates in data: every token
// version.FindTokens finds that version.Parse accepts, in the order they
// appear. A token Parse rejects (an IPv4 address, a two-component number) is
// not a candidate.
func TextCandidates(data []byte) []Candidate {
	locs := version.FindTokens(data)
	cands := make([]Candidate, 0, len(locs))
	for _, loc := range locs {
		v, err := version.Parse(string(data[loc[0]:loc[1]]))
		if err != nil {
			continue
		}
		cands = append(cands, Candidate{Start: loc[0], End: loc[1], Version: v})
	}
	return cands
}

// isVersion reports whether s is exactly a version as written, which a
// skipped value must be: version.Parse also accepts surrounding whitespace.
func isVersion(s string) bool {
	v, err := version.Parse(s)
	return err == nil && v.String() == s
}

// ParseError reports that a parser could not read a file. Line is the
// 1-based line of the problem, or 0 when the decoder did not say.
type ParseError struct {
	Parser Parser
	Line   int
	Msg    string
	// Probe is set when the file is TOML but the probe copy locateTOML
	// decodes is not, so the versions in it cannot be placed.
	Probe bool
}

func (e *ParseError) Error() string {
	what := "not " + e.Parser.label()
	if e.Probe {
		what = e.Parser.label() + " whose versions incrmit cannot place"
	}
	if e.Line > 0 {
		return fmt.Sprintf("%s (line %d: %s)", what, e.Line, e.Msg)
	}
	return fmt.Sprintf("%s (%s)", what, e.Msg)
}

// KeyStep is one step of a KeyPath: an object key or table name, or an array
// index.
type KeyStep struct {
	Name  string
	Index int // the array index, or -1 when the step is the key Name
}

// KeyPath is where a value sits in a JSON or TOML document: the keys and
// array indexes that lead to it from the root.
type KeyPath []KeyStep

// String renders the path the way TOML writes a dotted key: a key made only
// of ASCII letters, digits, "_", and "-" bare, any other double-quoted, and
// an array index as [N], so `bin[0].version`, `dependencies."@types/node"`,
// and `"a.b"` (one key holding a dot) all read unambiguously.
func (k KeyPath) String() string {
	var b strings.Builder
	for i, s := range k {
		if s.Index >= 0 {
			b.WriteString("[" + strconv.Itoa(s.Index) + "]")
			continue
		}
		if i > 0 {
			b.WriteByte('.')
		}
		if isBareKey(s.Name) {
			b.WriteString(s.Name)
		} else {
			b.WriteString(quoteKey(s.Name))
		}
	}
	return b.String()
}

func keyStep(name string) KeyStep { return KeyStep{Name: name, Index: -1} }
func indexStep(i int) KeyStep     { return KeyStep{Index: i} }

func isBareKey(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if c := s[i]; !isAlnum(c) && c != '_' && c != '-' {
			return false
		}
	}
	return true
}

// quoteKey renders s as a TOML basic string, escaping what TOML requires to
// be escaped there.
func quoteKey(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch {
		case r == '"' || r == '\\':
			b.WriteByte('\\')
			b.WriteRune(r)
		case r == '\b':
			b.WriteString(`\b`)
		case r == '\t':
			b.WriteString(`\t`)
		case r == '\n':
			b.WriteString(`\n`)
		case r == '\f':
			b.WriteString(`\f`)
		case r == '\r':
			b.WriteString(`\r`)
		case r < 0x20 || r == 0x7f:
			fmt.Fprintf(&b, `\u%04X`, r)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

// lineAt returns the 1-based line holding data[offset], counting "\n",
// "\r\n", and a lone "\r" as one line break each, as discovery numbers lines.
func lineAt(data []byte, offset int) int {
	offset = min(offset, len(data))
	line := 1
	for i := 0; i < offset; i++ {
		switch data[i] {
		case '\n':
			line++
		case '\r':
			if i+1 < len(data) && data[i+1] == '\n' {
				continue // counted at the "\n"
			}
			line++
		}
	}
	return line
}

// Rewrite returns data with repls applied to cands, exactly as
// SetKnownVersions does, and the same counts. For JSON and TOML it then
// decodes the result and checks it against data: every value must be as it
// was except the ones the rewrite planned, which must now hold the planned
// versions. A rewrite that would change anything else is an error, and the
// caller writes nothing. The check costs a decode of each side, and it is
// what makes the TOML probe's answer something a bump can trust: the probe
// reasons about where values are, and the decoder confirms it.
func Rewrite(p Parser, data []byte, cands []Candidate, repls []Replacement) ([]byte, map[string]int, error) {
	out, counts, done := setKnownVersions(data, cands, repls)
	if len(done) == 0 || p == ParserText {
		return out, counts, nil
	}
	var flatten func([]byte) ([]leaf, error)
	switch p {
	case ParserJSON:
		flatten = flattenJSON
	case ParserTOML:
		flatten = flattenTOML
	default:
		return nil, nil, fmt.Errorf("files: %s: unknown parser", p)
	}
	before, err := flatten(data)
	if err != nil {
		return nil, nil, err
	}
	after, err := flatten(out)
	if err != nil {
		return nil, nil, fmt.Errorf("files: the rewritten file is no longer %s: %w", p.label(), err)
	}
	if err := checkRewrite(before, after, done); err != nil {
		return nil, nil, err
	}
	return out, counts, nil
}

// rewrite is one candidate a rewrite replaced: its key, the token it held,
// and the token written in its place.
type rewrite struct {
	key      KeyPath
	old, new string
}

// leaf is one value of a decoded document, flattened to its path: a scalar,
// or a marker for a table, object, or array, so a container that appears,
// vanishes, or changes kind shows up too.
type leaf struct {
	path  string
	value any
}

// container stands for an object or array in a flattened document.
type container struct {
	kind string // "{" or "["
	n    int
}

// checkRewrite compares the documents before and after a rewrite, flattened
// in the same order, and reports the first difference the rewrite did not
// plan.
func checkRewrite(before, after []leaf, done []rewrite) error {
	planned := make(map[string][]rewrite, len(done))
	for _, r := range done {
		k := r.key.String()
		planned[k] = append(planned[k], r)
	}
	if len(before) != len(after) {
		return errors.New("files: the rewrite changed the document's shape, which was not planned")
	}
	for i := range before {
		b, a := before[i], after[i]
		if b.path != a.path {
			return fmt.Errorf("files: the rewrite changed the document's keys at %s, which was not planned", b.path)
		}
		if leafEqual(b.value, a.value) {
			continue
		}
		bs, bok := b.value.(string)
		as, aok := a.value.(string)
		rs := planned[b.path]
		j := -1
		for n, r := range rs {
			if bok && aok && r.old == bs && r.new == as {
				j = n
				break
			}
		}
		if j < 0 {
			return fmt.Errorf("files: the rewrite would change %s, which was not planned", displayPath(b.path))
		}
		planned[b.path] = append(rs[:j], rs[j+1:]...)
	}
	for k, rs := range planned {
		if len(rs) > 0 {
			return fmt.Errorf("files: the rewrite did not change %s to %s as planned", displayPath(k), rs[0].new)
		}
	}
	return nil
}

func displayPath(p string) string {
	if p == "" {
		return "the document's root value"
	}
	return p
}

// leafEqual compares two decoded values. Times compare by instant and zone
// name, since each decode builds its own *time.Location, and floats by bits,
// so a NaN equals itself.
func leafEqual(a, b any) bool {
	switch av := a.(type) {
	case time.Time:
		bv, ok := b.(time.Time)
		return ok && av.Equal(bv) && av.Location().String() == bv.Location().String()
	case float64:
		bv, ok := b.(float64)
		return ok && math.Float64bits(av) == math.Float64bits(bv)
	default:
		return reflect.DeepEqual(a, b)
	}
}
