package cli

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode/utf8"
)

// incrmit prints bytes it did not write: the names of files in a tree it was
// pointed at, the lines of those files as dry-run context, paths recorded in
// the config and the journal, and OS errors that embed them. `discover` exists
// to be run over trees the user does not own, so any of those can hold a
// terminal escape sequence — one that clears the screen and hides the output,
// retitles the window, plants a hyperlink, or, on terminals that honor OSC 52,
// writes to the clipboard. CI logs render ANSI too.
//
// Two layers keep that from reaching the terminal:
//
//   - displayName renders a name — a path, a flag value, an ignore pattern — so
//     that it is unambiguous: unchanged when every character is printable,
//     Go-quoted when it is not. Two different names never render alike.
//   - terminalWriter wraps stdout and stderr in Main and escapes whatever control
//     or format character still arrives, from dry-run context, an OS error's
//     text, the flag package's messages, or a print site written later. It is the
//     guarantee; displayName is what makes a name readable under it.
//
// Escaping is always on, not only when stdout is a terminal: the logs of a CI
// run are not a terminal but render escape sequences all the same.

// displayName renders a name for the terminal. A name made only of printable
// characters (strconv.IsPrint: letters, marks, numbers, punctuation, symbols,
// and the ASCII space) comes back unchanged, so an ordinary path — non-ASCII,
// spaces, or Windows backslashes included — reads exactly as typed. Anything
// else is returned as a Go-quoted string, "tab\there", which is also how the
// messages that format a path with %q already show it.
//
// A name that begins with a double quote is quoted too, even when printable.
// That is what keeps the rendering injective: a quoted rendering always starts
// with `"`, and an unquoted one never does, so the output of a name can always
// be read back to the name, and a hostile file cannot be named to look like
// another one.
func displayName(s string) string {
	if !strings.HasPrefix(s, `"`) && isPrintable(s) {
		return s
	}
	return strconv.Quote(s)
}

// isPrintable reports whether s is valid UTF-8 made only of characters
// strconv.IsPrint accepts. That excludes every C0 and C1 control, DEL, the
// bidirectional overrides and isolates (U+202A–U+202E, U+2066–U+2069) and the
// other format characters such as zero-width spaces, and any byte that is not
// UTF-8 — which is how the one-byte CSI 0x9B arrives.
func isPrintable(s string) bool {
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		if (r == utf8.RuneError && size == 1) || !strconv.IsPrint(r) {
			return false
		}
		i += size
	}
	return true
}

// terminalText returns s with every character that is neither printable nor a
// newline or tab replaced by its Go escape (`\x1b`, `\u202e`, `\xff` for a byte
// that is not UTF-8). Unlike displayName it does not quote, because it is
// applied to whole messages and lines of context rather than to one name, and
// it is not injective: a literal `\x1b` in a file reads the same as an escape
// character. That is acceptable for context and error text, which are for
// reading, not for identifying a file.
//
// Newline is how every message ends. Tab passes through as well: it only moves
// the cursor forward, so it cannot overwrite or hide output, and it is how
// Makefiles and Go files indent, which is exactly the dry-run context it
// appears in. A tab in a name is still quoted by displayName, because there it
// would make two names look alike.
func terminalText(s string) string {
	if isTerminalSafe(s) {
		return s
	}
	var b strings.Builder
	b.Grow(len(s) + 8)
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		switch {
		case r == utf8.RuneError && size == 1:
			fmt.Fprintf(&b, `\x%02x`, s[i])
		case r == '\n' || r == '\t' || strconv.IsPrint(r):
			b.WriteString(s[i : i+size])
		default:
			q := strconv.QuoteRune(r)
			b.WriteString(q[1 : len(q)-1])
		}
		i += size
	}
	return b.String()
}

// isTerminalSafe reports whether terminalText would return s unchanged.
func isTerminalSafe(s string) bool {
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		if (r == utf8.RuneError && size == 1) || (r != '\n' && r != '\t' && !strconv.IsPrint(r)) {
			return false
		}
		i += size
	}
	return true
}

// terminalWriter passes everything written through terminalText. Every write
// incrmit makes is one whole formatted message, so a multi-byte character is
// never split across two writes; a partial one at the end of a write would be
// escaped byte by byte, which is safe, merely less readable.
type terminalWriter struct {
	w io.Writer
}

// Write reports len(p) on success, as io.Writer requires, even though the bytes
// that reach the underlying writer may be more than len(p) once escaped.
func (t terminalWriter) Write(p []byte) (int, error) {
	s := string(p)
	if isTerminalSafe(s) {
		return t.w.Write(p)
	}
	if _, err := io.WriteString(t.w, terminalText(s)); err != nil {
		return 0, err
	}
	return len(p), nil
}

// displayNames renders each of names with displayName, for a list printed on
// one line.
func displayNames(names []string) []string {
	out := make([]string, len(names))
	for i, n := range names {
		out[i] = displayName(n)
	}
	return out
}
