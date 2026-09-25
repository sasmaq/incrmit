package cli

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"syscall"
	"testing"
)

func TestDisplayName(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"plain", "VERSION", "VERSION"},
		{"nested", "internal/buildinfo/buildinfo.go", "internal/buildinfo/buildinfo.go"},
		{"spaces", "my version file.txt", "my version file.txt"},
		{"non-ASCII", "versión-版本.txt", "versión-版本.txt"},
		{"Windows path", `C:\src\app\VERSION`, `C:\src\app\VERSION`},
		{"glob characters", "[ab]*?.txt", "[ab]*?.txt"},
		{"quote inside", `it's "the" version`, `it's "the" version`},
		{"empty", "", ""},
		// Everything below is quoted: a control or format character, a byte
		// that is not UTF-8, or a leading quote.
		{"ESC", "a\x1b[2Jb", `"a\x1b[2Jb"`},
		{"OSC title", "t\x1b]0;pwned\x07", `"t\x1b]0;pwned\a"`},
		{"newline", "VER\nSION", `"VER\nSION"`},
		{"carriage return", "VER\rSION", `"VER\rSION"`},
		{"tab", "a\tb", `"a\tb"`},
		{"DEL", "a\x7fb", `"a\x7fb"`},
		{"C1 control in UTF-8", "a\u009b31mb", `"a\u009b31mb"`},
		{"raw CSI byte", "a\x9b31mb", `"a\x9b31mb"`},
		{"invalid UTF-8", "caf\xe9", `"caf\xe9"`},
		{"right-to-left override", "invoice\u202efdp.exe", `"invoice\u202efdp.exe"`},
		{"bidi isolate", "a\u2066b\u2069", `"a\u2066b\u2069"`},
		{"zero-width space", "VER\u200bSION", `"VER\u200bSION"`},
		{"no-break space", "a\u00a0b", `"a\u00a0b"`},
		{"leading quote", `"VERSION"`, `"\"VERSION\""`},
		{"backslash in a quoted name", "C:\\a\x1b", `"C:\\a\x1b"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := displayName(tt.in); got != tt.want {
				t.Errorf("displayName(%q) = %s, want %s", tt.in, got, tt.want)
			}
		})
	}
}

// A literal backslash sequence in a name is printable and so shown as it is,
// while the control character it spells is quoted: the two never render alike.
func TestDisplayNameDistinguishesLookAlikes(t *testing.T) {
	pairs := [][2]string{
		{`a\x1b`, "a\x1b"},
		{`"a\x1b"`, "a\x1b"},
		{`\u202e`, "\u202e"},
		{"a b", "a\u00a0b"},
	}
	for _, p := range pairs {
		if a, b := displayName(p[0]), displayName(p[1]); a == b {
			t.Errorf("%q and %q both render as %s", p[0], p[1], a)
		}
	}
}

func TestTerminalText(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"plain", "version = 1.2.3\n", "version = 1.2.3\n"},
		{"tab kept", "VERSION\t:= 1.2.3\n", "VERSION\t:= 1.2.3\n"},
		{"non-ASCII kept", "版本 1.2.3 ✓", "版本 1.2.3 ✓"},
		{"clear screen", "v 1.2.3\x1b[2J", `v 1.2.3\x1b[2J`},
		{"hyperlink", "\x1b]8;;http://x\x1b\\y\x1b]8;;\x1b\\", `\x1b]8;;http://x\x1b\y\x1b]8;;\x1b\`},
		{"carriage return", "a\rb", `a\rb`},
		{"backspace and bell", "a\bb\a", `a\bb\a`},
		{"raw CSI byte", "a\x9b2J", `a\x9b2J`},
		{"C1 control", "a\u009b2J", `a\u009b2J`},
		{"bidi override", "a\u202eb", `a\u202eb`},
		{"invalid UTF-8", "caf\xe9!", `caf\xe9!`},
		{"literal backslashes kept", `C:\x1b`, `C:\x1b`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := terminalText(tt.in); got != tt.want {
				t.Errorf("terminalText(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// The writer escapes what reaches it and reports the caller's byte count, so a
// fmt.Fprintf through it behaves like one through the underlying writer.
func TestTerminalWriter(t *testing.T) {
	var buf bytes.Buffer
	w := terminalWriter{&buf}

	n, err := w.Write([]byte("safe line\n"))
	if err != nil || n != len("safe line\n") {
		t.Fatalf("Write = %d, %v", n, err)
	}
	hostile := []byte("L1: \x1b[2Jversion 1.2.3\n")
	n, err = w.Write(hostile)
	if err != nil || n != len(hostile) {
		t.Fatalf("Write = %d, %v; want %d, nil", n, err, len(hostile))
	}
	if want := "safe line\nL1: \\x1b[2Jversion 1.2.3\n"; buf.String() != want {
		t.Errorf("wrote %q, want %q", buf.String(), want)
	}

	failing := terminalWriter{errWriter{}}
	if n, err := failing.Write(hostile); err == nil || n != 0 {
		t.Errorf("Write to a failing writer = %d, %v; want 0 and the error", n, err)
	}
}

type errWriter struct{}

func (errWriter) Write([]byte) (int, error) { return 0, errors.New("broken pipe") }

// Main installs the writer, so even output incrmit does not format itself — the
// flag package echoing an unknown flag — reaches the terminal escaped.
func TestMainEscapesFlagPackageOutput(t *testing.T) {
	code, _, stderr := runMain(t, "", "--bogus\x1b]0;pwned\x07")
	if code != ExitUsage {
		t.Fatalf("exit = %d, want %d", code, ExitUsage)
	}
	if strings.ContainsAny(stderr, "\x1b\x07") {
		t.Errorf("stderr carries a raw escape: %q", stderr)
	}
	if !strings.Contains(stderr, `-bogus\x1b]0;pwned\a`) {
		t.Errorf("stderr = %q, want the flag shown escaped", stderr)
	}
}

// A path error names the path itself, raw. fsErrorMessage has already named it
// safely, so it keeps only the reason, and the message names the file once.
func TestFSErrorMessageDropsThePathErrorPath(t *testing.T) {
	err := &fs.PathError{Op: "stat", Path: "e\x1b[2J", Err: syscall.ENAMETOOLONG}
	got := fsErrorMessage("reading", "e\x1b[2J", err)
	if want := `reading "e\x1b[2J": ` + syscall.ENAMETOOLONG.Error(); got != want {
		t.Errorf("fsErrorMessage = %q, want %q", got, want)
	}
}

// The path is dropped from a wrapped path error too, which is how a failed
// write arrives from files.WriteAtomic.
func TestFSErrorMessageDropsAWrappedPathErrorPath(t *testing.T) {
	inner := &fs.PathError{Op: "write", Path: "/tmp/\x1b[2J/.incrmit-1.tmp", Err: syscall.ENOSPC}
	err := fmt.Errorf("files: writing temp file: %w", inner)
	got := fsErrorMessage("writing", "VERSION", err)
	if want := "writing VERSION: " + syscall.ENOSPC.Error(); got != want {
		t.Errorf("fsErrorMessage = %q, want %q", got, want)
	}
}
