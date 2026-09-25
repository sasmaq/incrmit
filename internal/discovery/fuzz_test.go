package discovery

import (
	"bytes"
	"testing"
	"unicode/utf8"

	"github.com/sasmaq/incrmit/internal/version"
)

// FuzzScan checks what discovery reports about each occurrence against a
// reference that recomputes it from scratch. scan finds the line of every token
// with a cursor that only moves forward and steps over one terminator at a time,
// which is what keeps a minified file linear; the price is state, and state is
// where "\r\n" counted twice or a line skipped would hide.
//
// For every occurrence: the line number equals the number of line breaks before
// the token plus one, counting "\n", "\r\n", and a lone "\r" once each; the text
// holds the token, holds no line terminator, and stays within the context
// window however long the line is.
func FuzzScan(f *testing.F) {
	f.Add([]byte("version = 1.2.3\n"))
	f.Add([]byte("a\r\nb\nversion = 1.2.3\r\n"))
	f.Add([]byte("a\rb\r1.2.3\r"))
	f.Add([]byte("a\r\r\n1.2.3\n\r\n1.2.3"))
	f.Add([]byte("\xEF\xBB\xBF1.2.3"))
	f.Add([]byte("1.2.3 1.2.3\r1.2.3\n\n\n4.5.6"))
	f.Add(bytes.Repeat([]byte("x"), 300))
	f.Add(append(bytes.Repeat([]byte("版"), 60), []byte(" 1.2.3 \xA9\xA9\xA9")...))

	f.Fuzz(func(t *testing.T, data []byte) {
		occ := scan(data)

		var starts []int
		for _, loc := range version.FindTokens(data) {
			if _, err := version.Parse(string(data[loc[0]:loc[1]])); err == nil {
				starts = append(starts, loc[0])
			}
		}
		if len(occ) != len(starts) {
			t.Fatalf("scan reported %d occurrences, FindTokens has %d versions in %q", len(occ), len(starts), data)
		}

		limit := 2*maxContext + 2*utf8.UTFMax + 2*len("...")
		for i, o := range occ {
			if want := referenceLine(data, starts[i]); o.Line != want {
				t.Errorf("occurrence %d at byte %d: line %d, want %d in %q", i, starts[i], o.Line, want, data)
			}
			tok := o.Version.String()
			if !bytes.Contains([]byte(o.Text), []byte(tok)) {
				t.Errorf("occurrence %d: text %q does not hold its token %q", i, o.Text, tok)
			}
			if bytes.ContainsAny([]byte(o.Text), "\r\n") {
				t.Errorf("occurrence %d: text %q holds a line terminator", i, o.Text)
			}
			if len(o.Text) > limit+len(tok) {
				t.Errorf("occurrence %d: text is %d bytes, over the %d byte window", i, len(o.Text), limit+len(tok))
			}
		}
	})
}

// referenceLine is the 1-based line holding offset, counted directly: each "\n",
// each "\r\n", and each lone "\r" before it starts a new line.
func referenceLine(data []byte, offset int) int {
	line := 1
	for i := 0; i < offset; i++ {
		switch {
		case data[i] == '\n':
			line++
		case data[i] == '\r' && (i+1 >= len(data) || data[i+1] != '\n'):
			line++
		}
	}
	return line
}
