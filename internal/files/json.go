package files

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
)

// The JSON parser walks the standard library's json.Decoder token by token,
// keeping a stack of object keys and array indexes, and takes each string
// value's bytes from Decoder.InputOffset. That needs no dependency beyond the
// standard library: encoding/json/jsontext reports offsets directly, but it
// is still behind GOEXPERIMENT=jsonv2 in Go 1.27.
//
// A candidate is a string value, never an object key, whose bytes between the
// quotes are exactly one token version.FindTokens finds in the file and
// version.Parse accepts: "1.2.3" and "v1.2.3" are candidates, and "^18.3.1",
// ">=20.0.0", and a URL holding 2.0.0 are not. A value whose bytes differ from
// its decoded text (an escape such as \u002e) is never a candidate; one that
// decodes to a version is reported as Skipped instead. encoding/json accepts
// duplicate keys, so a key written twice gives two candidates under one path.

// utf8BOM is the byte-order mark some Windows tools write at the start of a
// UTF-8 file. encoding/json rejects it, and §9.4 says incrmit keeps it, so
// the JSON parser decodes what follows it and offsets every range past it.
var utf8BOM = []byte("\xEF\xBB\xBF")

// splitBOM returns the length of a UTF-8 byte-order mark at the start of data
// (0 or 3) and the bytes after it.
func splitBOM(data []byte) (int, []byte) {
	if bytes.HasPrefix(data, utf8BOM) {
		return len(utf8BOM), data[len(utf8BOM):]
	}
	return 0, data
}

func locateJSON(data []byte) (*Located, error) {
	off, body := splitBOM(data)
	if err := checkJSON(data, off, body); err != nil {
		return nil, err
	}

	// Every candidate is one of the text parser's tokens, so look the
	// tokens up by where they start.
	tokens := make(map[int]Candidate)
	for _, c := range TextCandidates(data) {
		tokens[c.Start] = c
	}

	loc := &Located{}
	err := walkJSON(body, func(path KeyPath, tok json.Token, before, after int) {
		s, ok := tok.(string)
		if !ok {
			return
		}
		// InputOffset before a value is where the previous token ended, so
		// the separators and whitespace ahead of the value come first.
		q := before
		for q < after && body[q] != '"' {
			q++
		}
		start, end := off+q+1, off+after-1
		if string(data[start:end]) != s {
			if isVersion(s) {
				loc.Skipped = append(loc.Skipped, Skipped{Key: append(KeyPath(nil), path...), Line: lineAt(data, start), Value: s})
			}
			return
		}
		if c, ok := tokens[start]; ok && c.End == end {
			c.Key = append(KeyPath(nil), path...)
			loc.Candidates = append(loc.Candidates, c)
		}
	})
	if err != nil {
		return nil, err
	}
	return loc, nil
}

// checkJSON returns a *ParseError when body (data after a BOM of off bytes)
// is not one JSON value, with the line of the byte the decoder stopped at.
// json.Valid decides; the Decoder alone would read a second value after the
// first as a stream rather than refuse the file.
func checkJSON(data []byte, off int, body []byte) error {
	if json.Valid(body) {
		return nil
	}
	var raw json.RawMessage
	err := json.Unmarshal(body, &raw)
	pe := &ParseError{Parser: ParserJSON, Msg: "not a JSON value"}
	var syn *json.SyntaxError
	if errors.As(err, &syn) {
		pe.Msg = syn.Error()
		// Offset counts the bytes read, the offending one included.
		pe.Line = lineAt(data, off+max(int(syn.Offset)-1, 0))
	}
	return pe
}

// jsonFrame is one open object or array in walkJSON.
type jsonFrame struct {
	object  bool
	wantKey bool   // object: the next string is a key
	key     string // object: the key the next value sits under
	index   int    // array: the index of the next value
}

// walkJSON calls visit for every token of the JSON document body except
// object keys, which become part of the paths instead, with the path of the
// value the token opens, closes, or is, and the decoder's offsets before and
// after reading it. The path is reused between calls: visit copies it to
// keep it. A string value's bytes lie between the first '"' at or after
// before and after. body must be valid JSON (checkJSON).
func walkJSON(body []byte, visit func(path KeyPath, tok json.Token, before, after int)) error {
	dec := json.NewDecoder(bytes.NewReader(body))
	// Numbers stay text: as float64, 1e999 fails to decode although it is
	// valid JSON, and nothing here needs their value.
	dec.UseNumber()
	var stack []jsonFrame
	var path KeyPath
	advance := func() {
		if n := len(stack); n > 0 {
			if stack[n-1].object {
				stack[n-1].wantKey = true
			} else {
				stack[n-1].index++
			}
		}
	}
	valuePath := func() KeyPath {
		n := len(stack)
		switch {
		case n == 0:
			return path
		case stack[n-1].object:
			return append(path, keyStep(stack[n-1].key))
		default:
			return append(path, indexStep(stack[n-1].index))
		}
	}
	for {
		before := int(dec.InputOffset())
		tok, err := dec.Token()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return &ParseError{Parser: ParserJSON, Msg: err.Error()}
		}
		after := int(dec.InputOffset())
		switch t := tok.(type) {
		case json.Delim:
			switch t {
			case '{', '[':
				path = valuePath()
				visit(path, t, before, after)
				stack = append(stack, jsonFrame{object: t == '{', wantKey: t == '{'})
			default:
				visit(path, t, before, after)
				stack = stack[:len(stack)-1]
				if len(path) > 0 {
					path = path[:len(path)-1]
				}
				advance()
			}
		case string:
			if n := len(stack); n > 0 && stack[n-1].wantKey {
				stack[n-1].key, stack[n-1].wantKey = t, false
				continue
			}
			visit(valuePath(), t, before, after)
			advance()
		default:
			visit(valuePath(), t, before, after)
			advance()
		}
	}
}

// flattenJSON returns every value of a JSON document in order, for the check
// Rewrite makes: strings and numbers as their text, objects and arrays as
// container markers. Order is kept, so a duplicate key's two values stay two.
func flattenJSON(data []byte) ([]leaf, error) {
	off, body := splitBOM(data)
	if err := checkJSON(data, off, body); err != nil {
		return nil, err
	}
	var leaves []leaf
	err := walkJSON(body, func(path KeyPath, tok json.Token, _, _ int) {
		var v any = tok
		switch t := tok.(type) {
		case json.Delim:
			v = container{kind: string(t)}
		case json.Number:
			v = string(t)
		}
		leaves = append(leaves, leaf{path: path.String(), value: v})
	})
	return leaves, err
}
