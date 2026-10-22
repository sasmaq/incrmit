package files

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/sasmaq/incrmit/internal/version"
)

// A key says which of a file's candidates is the project's version, for a
// file that holds others: dependency pins, which are whole string values in a
// manifest too, or a tool's version in a Makefile. It is one optional field of
// a [[files]] entry (and the bump's --key), and the entry's parser reads it:
//
//   - For JSON and TOML it is a path from the document's root, written the way
//     KeyPath.String writes one (package.version, bin[0].version,
//     dependencies."@types/node", "a.b" for one key holding a dot). It names
//     the whole path, never a suffix, and the value there must be a candidate.
//   - For text, which has no structure, it is the literal bytes right before
//     the version, on its line: "VERSION ?= " names the version in
//     `VERSION ?= 1.2.3` and not the one in `.../tool-1.2.3.tar.gz`. There is
//     no pattern language; matching is byte for byte, case and whitespace
//     included.
//
// One field serves every parser, so a key changes meaning when its entry's
// parser does. Neither direction can name a different occurrence than the one
// meant: a path read as text names nothing, since a path ends in a letter, a
// digit, a quote, or a bracket, and the scanner wants a word boundary before
// a version; and a text key read as a path is either not a path (a space, a
// "?", an "=") or names a key the file does not have. Both are errors.

// Key is an entry's `key` as its parser reads it. The zero value names
// nothing; use ParseKey or PathKey.
type Key struct {
	parser Parser
	raw    string  // the key as written
	path   KeyPath // for JSON and TOML, raw read as a path
}

// ParseKey reads s as a key for a file p reads. A key for the text parser is
// any text without a line break; one for JSON or TOML must be a path
// (ParsePath). An empty key is an error for every parser.
func ParseKey(p Parser, s string) (Key, error) {
	if s == "" {
		return Key{}, fmt.Errorf("key is empty; %s", keyHint(p))
	}
	switch p {
	case ParserText:
		if strings.ContainsAny(s, "\r\n") {
			return Key{}, fmt.Errorf("key %q holds a line break; %s", s, keyHint(p))
		}
		return Key{parser: p, raw: s}, nil
	case ParserJSON, ParserTOML:
		path, err := ParsePath(s)
		if err != nil {
			return Key{}, fmt.Errorf("key %q is not a key path (%w); %s", s, err, keyHint(p))
		}
		return Key{parser: p, raw: s, path: path}, nil
	default:
		return Key{}, fmt.Errorf("files: unknown parser %q", string(p))
	}
}

// keyHint says what a key for a file p reads looks like.
func keyHint(p Parser) string {
	if p == ParserText {
		return `for a text file, a key is the text right before the version on its line, such as "VERSION ?= "`
	}
	return fmt.Sprintf("for a %s file, a key is a path such as package.version", p.label())
}

// PathKey returns the key naming path in a file p reads, as discovery writes
// one for a candidate.
func PathKey(p Parser, path KeyPath) Key {
	return Key{parser: p, raw: path.String(), path: slices.Clone(path)}
}

// Raw returns the key as written, which is what a config and the journal
// record.
func (k Key) Raw() string { return k.raw }

// String renders the key for messages: a path as KeyPath.String writes it,
// however it was written, and text Go-quoted, so the whitespace it holds is
// visible.
func (k Key) String() string {
	if k.parser == ParserText {
		return strconv.Quote(k.raw)
	}
	return k.path.String()
}

// ParsePath reads a key path written the way KeyPath.String writes one: parts
// joined by ".", each either bare (ASCII letters, digits, "_", and "-") or
// double-quoted with TOML's basic-string escapes, and an array index written
// [N] right after the part it indexes, or first for a root array. Nothing
// else is allowed, whitespace included, so a path has one spelling apart from
// quoting a part that need not be quoted.
func ParsePath(s string) (KeyPath, error) {
	if s == "" {
		return nil, errors.New("it is empty")
	}
	var path KeyPath
	i := 0
	for i < len(s) {
		switch {
		case s[i] == '[':
			n, end, err := parseIndex(s, i)
			if err != nil {
				return nil, err
			}
			path = append(path, indexStep(n))
			i = end
			continue
		case len(path) > 0 && s[i] != '.':
			return nil, fmt.Errorf("%s cannot follow %q", quoteRuneAt(s, i), s[:i])
		case len(path) > 0:
			i++ // the dot before the next part
			if i == len(s) {
				return nil, errors.New("it ends in a dot")
			}
		}
		name, end, err := parseName(s, i)
		if err != nil {
			return nil, err
		}
		path = append(path, keyStep(name))
		i = end
	}
	return path, nil
}

// parseIndex reads an index "[N]" at s[i], N a decimal number written without
// leading zeros, and returns N and the offset past "]".
func parseIndex(s string, i int) (int, int, error) {
	end := strings.IndexByte(s[i:], ']')
	if end < 0 {
		return 0, 0, fmt.Errorf("the [ at byte %d is not closed", i+1)
	}
	digits := s[i+1 : i+end]
	n, err := strconv.Atoi(digits)
	if err != nil || n < 0 || strconv.Itoa(n) != digits {
		return 0, 0, fmt.Errorf("[%s] is not an array index, which is a number such as [0]", digits)
	}
	return n, i + end + 1, nil
}

// parseName reads one part of a path at s[i], bare or quoted, and returns it
// and the offset past it.
func parseName(s string, i int) (string, int, error) {
	if s[i] == '"' {
		return parseQuoted(s, i)
	}
	j := i
	for j < len(s) && (isAlnum(s[j]) || s[j] == '_' || s[j] == '-') {
		j++
	}
	if j == i {
		if i == 0 {
			return "", 0, fmt.Errorf("%s cannot start a path", quoteRuneAt(s, i))
		}
		return "", 0, fmt.Errorf("%s cannot follow %q", quoteRuneAt(s, i), s[:i])
	}
	return s[i:j], j, nil
}

// parseQuoted reads a double-quoted part at s[i], decoding the escapes a TOML
// basic string has, and returns it and the offset past the closing quote.
func parseQuoted(s string, i int) (string, int, error) {
	var b strings.Builder
	for j := i + 1; j < len(s); {
		c := s[j]
		switch {
		case c == '"':
			return b.String(), j + 1, nil
		case c == '\\':
			r, n, err := parseEscape(s, j)
			if err != nil {
				return "", 0, err
			}
			b.WriteRune(r)
			j += n
		case (c < 0x20 && c != '\t') || c == 0x7f:
			return "", 0, fmt.Errorf("a quoted part holds the control character %q; write it as an escape", c)
		default:
			b.WriteByte(c)
			j++
		}
	}
	return "", 0, fmt.Errorf("the quote at byte %d is not closed", i+1)
}

// parseEscape decodes the escape at s[j], which starts with a backslash, and
// returns the character and the escape's length.
func parseEscape(s string, j int) (rune, int, error) {
	if j+1 >= len(s) {
		return 0, 0, errors.New("it ends in a backslash")
	}
	switch c := s[j+1]; c {
	case 'b':
		return '\b', 2, nil
	case 't':
		return '\t', 2, nil
	case 'n':
		return '\n', 2, nil
	case 'f':
		return '\f', 2, nil
	case 'r':
		return '\r', 2, nil
	case '"', '\\':
		return rune(c), 2, nil
	case 'u', 'U':
		width := 4
		if c == 'U' {
			width = 8
		}
		hex := s[j+2 : min(j+2+width, len(s))]
		n, err := strconv.ParseUint(hex, 16, 32)
		if err != nil || len(hex) != width || !utf8.ValidRune(rune(n)) {
			return 0, 0, fmt.Errorf(`\%c%s is not a Unicode character`, c, hex)
		}
		return rune(n), 2 + width, nil
	default:
		return 0, 0, fmt.Errorf(`%s is not an escape a path may use`, quoteRuneAt(s, j))
	}
}

// quoteRuneAt quotes the character at s[i], or the escape at it, for a
// message.
func quoteRuneAt(s string, i int) string {
	r, size := utf8.DecodeRuneInString(s[i:])
	if s[i] == '\\' && i+1 < len(s) {
		return strconv.Quote(s[i : i+2])
	}
	if r == utf8.RuneError && size == 1 {
		return strconv.Quote(s[i : i+1])
	}
	return strconv.QuoteRune(r)
}

// KeyError reports that an entry's key does not name a version a bump can
// rewrite: a path the file does not have, has twice, or holds something other
// than a candidate there, or holds another version than the one pinned, or
// text no candidate follows. The message starts with the key.
type KeyError struct {
	Key Key
	Msg string
}

func (e *KeyError) Error() string {
	return "key " + e.Key.String() + " " + e.Msg
}

// keyIndex is what Named looks a path up in: every value of a JSON or TOML
// document by its path, in document order, and the candidates by theirs.
type keyIndex struct {
	values map[string][]any
	cands  map[string][]int // indexes into Located.Candidates
}

// Named returns the candidates k names in the file the candidates were
// located in, which must be read with the parser k was made for.
//
// For JSON and TOML that is exactly one candidate, the value at k's path. A
// path the file does not have, names more than one value (a JSON key written
// twice), or holds anything but a candidate is a *KeyError. For text it is
// every candidate whose bytes come right after k's, in byte order, and none
// is a *KeyError.
func (l *Located) Named(k Key) ([]Candidate, error) {
	if k.parser != l.parser {
		return nil, fmt.Errorf("files: a key for a %s file cannot name a value in a %s file", k.parser.label(), l.parser.label())
	}
	if l.parser == ParserText {
		var named []Candidate
		for _, c := range l.Candidates {
			if bytes.HasSuffix(l.data[:c.Start], []byte(k.raw)) {
				named = append(named, c)
			}
		}
		if len(named) == 0 {
			return nil, &KeyError{Key: k, Msg: "is not followed by a version anywhere in the file"}
		}
		return named, nil
	}

	idx, err := l.index()
	if err != nil {
		return nil, err
	}
	p := k.path.String()
	values := idx.values[p]
	switch len(values) {
	case 0:
		return nil, &KeyError{Key: k, Msg: "is not in the file"}
	case 1:
	case 2:
		return nil, &KeyError{Key: k, Msg: "appears twice in the file, so it names no one value"}
	default:
		return nil, &KeyError{Key: k, Msg: fmt.Sprintf("appears %d times in the file, so it names no one value", len(values))}
	}
	if is := idx.cands[p]; len(is) == 1 {
		return []Candidate{l.Candidates[is[0]]}, nil
	}
	for _, sk := range l.Skipped {
		if sk.Key.String() == p {
			return nil, &KeyError{Key: k, Msg: fmt.Sprintf("holds %q only once its escapes are decoded; incrmit rewrites a version written out plainly", sk.Value)}
		}
	}
	return nil, &KeyError{Key: k, Msg: describeValue(values[0])}
}

// Claim returns the start offsets, in byte order, of the candidates k names
// (Named) that hold pin: the occurrences a bump of an entry with that key and
// version rewrites. A key that names candidates holding only other versions
// is a *KeyError, as is one Named refuses.
func (l *Located) Claim(k Key, pin version.Version) ([]int, error) {
	named, err := l.Named(k)
	if err != nil {
		return nil, err
	}
	var at []int
	var held []string
	for _, c := range named {
		if _, ok := matchAt(l.data, c.Start, c.End, pin); ok {
			at = append(at, c.Start)
		} else if v := c.Version.String(); !slices.Contains(held, v) {
			held = append(held, v)
		}
	}
	if len(at) == 0 {
		verb := "holds"
		if l.parser == ParserText {
			verb = "is followed by"
		}
		return nil, &KeyError{Key: k, Msg: fmt.Sprintf("%s %s, not %s", verb, strings.Join(held, " and "), pin)}
	}
	return at, nil
}

// index builds the keyIndex of a JSON or TOML file on first use. It decodes
// the file again, which only an entry with a key costs.
func (l *Located) index() (*keyIndex, error) {
	if l.keys != nil {
		return l.keys, nil
	}
	idx := &keyIndex{values: make(map[string][]any), cands: make(map[string][]int, len(l.Candidates))}
	add := func(path KeyPath, v any) {
		p := path.String()
		idx.values[p] = append(idx.values[p], v)
	}
	switch l.parser {
	case ParserJSON:
		_, body := splitBOM(l.data)
		err := walkJSON(body, func(path KeyPath, tok json.Token, _, _ int) {
			// A container is visited where it opens and where it closes;
			// count it once.
			if tok != json.Delim('}') && tok != json.Delim(']') {
				add(path, tok)
			}
		})
		if err != nil {
			return nil, err
		}
	case ParserTOML:
		doc, err := decodeTOML(l.data)
		if err != nil {
			return nil, tomlParseError(err, false)
		}
		walkTOML(doc, nil, add)
	}
	for i, c := range l.Candidates {
		p := c.Key.String()
		idx.cands[p] = append(idx.cands[p], i)
	}
	l.keys = idx
	return idx, nil
}

// describeValue says what a value that is not a candidate holds, for a
// KeyError.
func describeValue(v any) string {
	switch t := v.(type) {
	case string:
		return "holds " + quoteClipped(t) + ", which is not a version"
	case json.Delim:
		if t == '[' {
			return "holds an array, not a version"
		}
		return "holds an object, not a version"
	case map[string]any:
		return "holds a table, not a version"
	case []any, []map[string]any:
		return "holds an array, not a version"
	case bool:
		return "holds a boolean, not a version"
	case nil:
		return "holds null, not a version"
	case json.Number, int64, float64:
		return "holds a number; a version is a string"
	default:
		// TOML's dates and times are the only values left.
		return "holds a date or time, not a version"
	}
}

// quoteClipped Go-quotes s for a message, cut to its first forty bytes at a
// character boundary.
func quoteClipped(s string) string {
	const limit = 40
	if len(s) <= limit {
		return strconv.Quote(s)
	}
	cut := limit
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return strconv.Quote(s[:cut]) + "..."
}

// Overlap reports the first candidate that two of repls would both rewrite,
// where at least one of the two is keyed: two entries with keys that name one
// occurrence, or a keyed entry and one without a key holding its version.
// Either way one of them would bump an occurrence the other claims, so the
// caller refuses the config rather than pick. Two replacements without a key
// never overlap here: the most specific pin takes an occurrence both hold, as
// SetKnownVersions does. i < j.
func Overlap(data []byte, cands []Candidate, repls []Replacement) (i, j int, c Candidate, ok bool) {
	for a, r := range repls {
		if !r.Keyed {
			continue
		}
		for _, start := range r.At {
			n, found := slices.BinarySearchFunc(cands, start, func(c Candidate, s int) int { return c.Start - s })
			if !found {
				continue
			}
			for b, other := range repls {
				if b != a && other.rewrites(data, cands[n]) {
					return min(a, b), max(a, b), cands[n], true
				}
			}
		}
	}
	return 0, 0, Candidate{}, false
}

// rewrites reports whether r may rewrite c: c is one of a keyed r's claims, or
// holds the version of an r without a key.
func (r Replacement) rewrites(data []byte, c Candidate) bool {
	if r.Keyed {
		_, found := slices.BinarySearch(r.At, c.Start)
		return found
	}
	_, ok := matchAt(data, c.Start, c.End, r.Old)
	return ok
}

// LineOf returns the 1-based line holding data[offset], counting line breaks
// as discovery does, for a message naming an occurrence.
func LineOf(data []byte, offset int) int {
	return lineAt(data, offset)
}
