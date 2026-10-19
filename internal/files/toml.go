package files

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
)

// The TOML parser leaves every question of TOML syntax to BurntSushi/toml,
// the same reader the config is loaded with, so the two can never disagree
// about what a file means. That reader decides whether a file is TOML and
// what each key holds, but v1.6.0 keeps key positions in an unexported field,
// so it cannot say where a value's bytes are. The parser finds them by
// probing instead of lexing:
//
//  1. Decode the file. One that does not decode is not TOML.
//  2. Copy it with every token the text parser finds replaced by a
//     placeholder of its own, and decode the copy once.
//  3. Walk the copy's values. A token whose placeholder is now a value's
//     whole text is a candidate at that value's key. One whose placeholder is
//     part of a longer string (a URL, a description, a multi-line string) is
//     not, and one whose placeholder is in no value was in a comment or a key.
//
// Token boundaries are the text parser's, so §9's single definition of a
// token holds in TOML files too. All tokens share one probe decode: one
// decode per token grows quadratically, and a file of a thousand tokens took
// seconds that way.
//
// A placeholder is digits and dots, so it is valid wherever its token was: in
// a string, a comment, and a bare key, whose characters include digits. Each
// dot-separated segment of a token becomes one segment of digits, so a token
// used as a bare dotted key (`[versions.1.2.3]`) splits into exactly as many
// keys as it did, and a key path can be mapped back to the file's keys
// segment by segment, even where a token is welded into a longer key
// (`x-1.2.3`). Every segment is the same prefix plus a counter of fixed
// width, so no placeholder is a prefix of another (an unpadded `...1` is a
// prefix of `...10`), and the prefix occurs nowhere in the file or in any
// string it decodes to, so text that holds the prefix in the copy holds a
// placeholder.
//
// The copy's answer is trusted only once it is checked against the file:
// with every placeholder put back, in keys and in strings, the copy must hold
// exactly the file's values, each at the same key (probe.agrees). Masking a
// token in a key can change what the decoder does with that key. v1.6.0
// accepts some TOML the specification forbids, such as `a.b = "x"` followed by
// `a = "y"`, and silently keeps only the first; mask a token in the first key
// and the two no longer collide, so the copy holds a value the file does not.
// A version used as the name of an array of tables declared more than once
// (`[[1.2.3]]`) splits into several arrays the same way. Either is refused as
// TOML whose versions cannot be placed, like a copy that does not decode.

func locateTOML(data []byte) (*Located, error) {
	doc, err := decodeTOML(data)
	if err != nil {
		return nil, tomlParseError(err, false)
	}
	loc := &Located{}
	toks := TextCandidates(data)
	if len(toks) == 0 {
		walkTOML(doc, nil, func(path KeyPath, v any) {
			if s, ok := v.(string); ok {
				if isVersion(s) {
					loc.Skipped = append(loc.Skipped, Skipped{Key: slices.Clone(path), Value: s})
				}
			}
		})
		return loc, nil
	}

	pr := newProbe(data, doc, toks)
	pdoc, err := decodeTOML(pr.data)
	if err != nil {
		return nil, tomlParseError(err, true)
	}
	if !pr.agrees(doc, pdoc) {
		return nil, &ParseError{Parser: ParserTOML, Probe: true,
			Msg: "masking them changes what the file holds, which a key defined twice can cause"}
	}
	walkTOML(pdoc, nil, func(path KeyPath, v any) {
		s, ok := v.(string)
		if !ok {
			return
		}
		if i, ok := pr.whole[s]; ok {
			c := toks[i]
			c.Key = pr.unmap(path)
			loc.Candidates = append(loc.Candidates, c)
			return
		}
		if strings.Contains(s, pr.prefix) {
			return // a longer string holding a token
		}
		if isVersion(s) {
			loc.Skipped = append(loc.Skipped, Skipped{Key: pr.unmap(path), Value: s})
		}
	})
	sort.Slice(loc.Candidates, func(i, j int) bool { return loc.Candidates[i].Start < loc.Candidates[j].Start })
	return loc, nil
}

func decodeTOML(data []byte) (map[string]any, error) {
	var doc map[string]any
	if _, err := toml.Decode(string(data), &doc); err != nil {
		return nil, err
	}
	return doc, nil
}

// tomlParseError renders a decoding error as a *ParseError, with the line
// the decoder reports. A placeholder holds no line break, so a probe copy's
// line numbers are the file's.
func tomlParseError(err error, probe bool) *ParseError {
	pe := &ParseError{Parser: ParserTOML, Msg: err.Error(), Probe: probe}
	var perr toml.ParseError
	if errors.As(err, &perr) {
		pe.Line = perr.Position.Line
		pe.Msg = perr.Message
	}
	return pe
}

// probe is the copy of a TOML file locateTOML decodes, with what is needed to
// read its answer back.
type probe struct {
	data   []byte
	prefix string         // the digits every placeholder segment starts with
	whole  map[string]int // a token's whole placeholder -> its index in toks
	back   *strings.Replacer
}

func newProbe(data []byte, doc map[string]any, toks []Candidate) *probe {
	segs := 0
	for _, c := range toks {
		segs += bytes.Count(data[c.Start:c.End], []byte(".")) + 1
	}
	pr := &probe{prefix: absentDigits(data, doc), whole: make(map[string]int, len(toks))}
	width := max(9, len(strconv.Itoa(segs)))

	var b bytes.Buffer
	b.Grow(len(data) + segs*(len(pr.prefix)+width))
	pairs := make([]string, 0, 2*segs)
	n, prev := 0, 0
	for i, c := range toks {
		b.Write(data[prev:c.Start])
		parts := strings.Split(string(data[c.Start:c.End]), ".")
		for j, part := range parts {
			n++
			seg := pr.prefix + fmt.Sprintf("%0*d", width, n)
			pairs = append(pairs, seg, part)
			parts[j] = seg
		}
		ph := strings.Join(parts, ".")
		b.WriteString(ph)
		pr.whole[ph] = i
		prev = c.End
	}
	b.Write(data[prev:])
	pr.data = b.Bytes()
	pr.back = strings.NewReplacer(pairs...)
	return pr
}

// unmap returns path with every placeholder segment in its keys put back to
// the token segment it stands for, so the path names the file's own keys.
func (pr *probe) unmap(path KeyPath) KeyPath {
	out := slices.Clone(path)
	for i, s := range out {
		if s.Index < 0 && strings.Contains(s.Name, pr.prefix) {
			out[i].Name = pr.back.Replace(s.Name)
		}
	}
	return out
}

// agrees reports whether pdoc, the decoded copy, holds exactly doc's values
// once every placeholder is put back: the same scalars at the same keys, none
// missing and none extra. Tables and arrays themselves are not compared,
// because a table named by a token splits into one table per occurrence in
// the copy (`[1.2.3]` and `[1.2.3.x]` become two), which puts every value back
// at its own key all the same.
func (pr *probe) agrees(doc, pdoc map[string]any) bool {
	want := make(map[string]any)
	walkTOML(doc, nil, func(path KeyPath, v any) {
		if isScalar(v) {
			want[path.String()] = v
		}
	})
	seen := make(map[string]bool, len(want))
	ok := true
	walkTOML(pdoc, nil, func(path KeyPath, v any) {
		if !ok || !isScalar(v) {
			return
		}
		if s, isString := v.(string); isString && strings.Contains(s, pr.prefix) {
			v = pr.back.Replace(s)
		}
		k := pr.unmap(path).String()
		w, found := want[k]
		ok = found && !seen[k] && leafEqual(w, v)
		seen[k] = true
	})
	return ok && len(seen) == len(want)
}

func isScalar(v any) bool {
	switch v.(type) {
	case map[string]any, []map[string]any, []any:
		return false
	default:
		return true
	}
}

// absentDigits returns a run of digits that occurs neither in data nor in any
// key or string doc decodes to, so every occurrence in a probe copy is a
// placeholder's. The run is 987654321 when that is free, as it nearly always
// is. Otherwise it is built a digit at a time, each time choosing the digit
// that fewest runs of that width in the file continue the prefix with: the
// count shrinks at least tenfold per digit, so after nine digits it is zero
// for any file with fewer than a billion digits in it, and the width grows
// for one with more. Each step is one pass over the text, so a file planted
// with likely candidates costs nine passes rather than one per candidate.
func absentDigits(data []byte, doc map[string]any) string {
	texts := []string{string(data)}
	walkTOML(doc, nil, func(path KeyPath, v any) {
		if n := len(path); n > 0 && path[n-1].Index < 0 {
			texts = append(texts, path[n-1].Name)
		}
		if s, ok := v.(string); ok {
			texts = append(texts, s)
		}
	})

	const preferred = "987654321"
	free := true
	digits := 0
	for _, t := range texts {
		free = free && !strings.Contains(t, preferred)
		for i := 0; i < len(t); i++ {
			if isDigit(t[i]) {
				digits++
			}
		}
	}
	if free {
		return preferred
	}

	width := 9
	for float64(digits) >= math.Pow10(width) {
		width++
	}
	prefix := ""
	for len(prefix) < width {
		var counts [10]int
		for _, t := range texts {
			forDigitRuns(t, width, func(run string) {
				if strings.HasPrefix(run, prefix) {
					counts[run[len(prefix)]-'0']++
				}
			})
		}
		best := 0
		for d := 1; d < 10; d++ {
			if counts[d] < counts[best] {
				best = d
			}
		}
		prefix += string(rune('0' + best))
	}
	return prefix
}

// forDigitRuns calls fn with every run of width digits in t, overlapping
// runs included.
func forDigitRuns(t string, width int, fn func(string)) {
	start := -1
	for i := 0; i <= len(t); i++ {
		if i < len(t) && isDigit(t[i]) {
			if start < 0 {
				start = i
			}
			continue
		}
		if start >= 0 {
			for j := start; j+width <= i; j++ {
				fn(t[j : j+width])
			}
			start = -1
		}
	}
}

func isDigit(b byte) bool { return b >= '0' && b <= '9' }

// walkTOML calls visit for v and everything below it, depth first with keys
// in sorted order, so two decodes of one file are walked identically. A
// table or an inline table is a map, an array of tables a slice of maps, and
// any other array a slice of values. The path is reused between calls: visit
// copies it to keep it.
func walkTOML(v any, path KeyPath, visit func(path KeyPath, v any)) {
	visit(path, v)
	switch t := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			walkTOML(t[k], append(path, keyStep(k)), visit)
		}
	case []map[string]any:
		for i, e := range t {
			walkTOML(e, append(path, indexStep(i)), visit)
		}
	case []any:
		for i, e := range t {
			walkTOML(e, append(path, indexStep(i)), visit)
		}
	}
}

// flattenTOML returns every value of a TOML document, for the check Rewrite
// makes, in walkTOML's order: scalars as decoded, tables and arrays as
// container markers.
func flattenTOML(data []byte) ([]leaf, error) {
	doc, err := decodeTOML(data)
	if err != nil {
		return nil, tomlParseError(err, false)
	}
	var leaves []leaf
	walkTOML(doc, nil, func(path KeyPath, v any) {
		switch t := v.(type) {
		case map[string]any:
			v = container{kind: "{", n: len(t)}
		case []map[string]any:
			v = container{kind: "[", n: len(t)}
		case []any:
			v = container{kind: "[", n: len(t)}
		}
		leaves = append(leaves, leaf{path: path.String(), value: v})
	})
	return leaves, nil
}
