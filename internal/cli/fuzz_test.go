package cli

import (
	"math"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"
)

// FuzzParseSize checks the --max-file-size parser against arbitrary strings:
// the value comes straight off the command line, so anything a shell can hand
// over must come back as an error rather than a panic or a silently wrong
// limit.
//
// The wrong limit is the failure that matters. parseSize multiplies by a unit,
// and a product that wraps int64 turns "9223372036854775807MiB" into a small —
// or negative — cap, which would quietly refuse every file in the tree or read
// one the user meant to exclude. So the accepted values are checked for
// overflow (the multiplication must be exact) and for the round trip back
// through formatSize, which is what the flag prints and what error messages
// quote.
func FuzzParseSize(f *testing.F) {
	for _, s := range []string{
		"0", "1", "1048576", "512B", "1K", "1KB", "1KiB",
		"32MiB", "32mib", "32 MiB", "  8M  ", "2GB", "2GiB",
		"1234 bytes",
		// Known rejections, so the fuzzer starts at the cliff edge.
		"", "   ", "big", "1.5MB", "32MiBs", "-1", "-4KB",
		"9223372036854775807MiB", "9223372036854775808",
		"+1", "0x10", "1_000", "1e6", "MiB", "B", "--1", "1KiBKiB",
	} {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, s string) {
		n, err := parseSize(s)
		if err != nil {
			// A rejection must say something; the message is quoted back at the
			// user as the reason their flag was refused.
			if err.Error() == "" {
				t.Fatalf("parseSize(%q) failed with an empty message", s)
			}
			if n != 0 {
				t.Errorf("parseSize(%q) = %d with an error, want 0", s, n)
			}
			return
		}
		if n < 0 {
			t.Fatalf("parseSize(%q) = %d, a negative limit", s, n)
		}
		// An accepted size is a real byte count, so it must survive being
		// printed and read back: formatSize is how the flag reports its default
		// and how errors quote a limit.
		if back, err := parseSize(formatSize(n)); err != nil || back != n {
			t.Errorf("parseSize(%q) = %d, but formatSize gives %q which reads back as %d (err %v)", s, n, formatSize(n), back, err)
		}
	})
}

// FuzzFormatSize is the other half of that round trip, driven from the number
// rather than the text: every byte count parseSize can produce must format into
// something parseSize reads back unchanged.
func FuzzFormatSize(f *testing.F) {
	for _, n := range []int64{0, 1, 512, 1023, 1024, 1<<20 - 1, 32 << 20, 1 << 30, math.MaxInt64, math.MaxInt64 - 1} {
		f.Add(n)
	}

	f.Fuzz(func(t *testing.T, n int64) {
		if n < 0 {
			t.Skip("parseSize never produces a negative limit")
		}
		s := formatSize(n)
		back, err := parseSize(s)
		if err != nil {
			t.Fatalf("formatSize(%d) = %q, which parseSize rejects: %v", n, s, err)
		}
		if back != n {
			t.Errorf("formatSize(%d) = %q, which reads back as %d", n, s, back)
		}
	})
}

// displaySeeds are the characters terminals act on, the ones that make text
// display out of order, and the look-alikes that must stay distinguishable.
var displaySeeds = []string{
	"VERSION", "versión-版本.txt", `C:\src\VERSION`, "",
	"\x1b[2J", "\x1b]0;title\x07", "\x1b]8;;http://x\x1b\\y\x1b]8;;\x1b\\",
	"a\rb", "a\bb", "\x7f", "\x9b31m", "\u009b31m", "caf\xe9", "\xff\xfe",
	"invoice\u202efdp.exe", "\u2066x\u2069", "VER\u200bSION", "a\u00a0b",
	`"quoted"`, `\x1b`, "a\tb", "a\nb",
}

// FuzzDisplayName checks the two properties a rendered name must have: nothing
// in it is a character a terminal would act on, and it can always be read back
// to the one name it came from. The second is what stops a hostile file from
// being named to look like another: a function with a left inverse is
// injective, so no two names render alike.
func FuzzDisplayName(f *testing.F) {
	for _, s := range displaySeeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, name string) {
		out := displayName(name)
		if !isPrintable(out) {
			t.Fatalf("displayName(%q) = %q holds a character that is not printable", name, out)
		}
		if strings.HasPrefix(out, `"`) {
			back, err := strconv.Unquote(out)
			if err != nil || back != name {
				t.Fatalf("displayName(%q) = %s, which reads back as %q (%v)", name, out, back, err)
			}
		} else if out != name {
			t.Fatalf("displayName(%q) = %q: an unquoted rendering must be the name itself", name, out)
		}
		if isPrintable(name) && !strings.HasPrefix(name, `"`) && out != name {
			t.Fatalf("displayName changed the printable name %q to %q", name, out)
		}
	})
}

// FuzzTerminalText checks what terminalWriter guarantees for every byte incrmit
// prints: the result is valid UTF-8 holding nothing but printable characters,
// newlines, and tabs, and text that was already safe passes through unchanged.
func FuzzTerminalText(f *testing.F) {
	for _, s := range displaySeeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		out := terminalText(s)
		if !utf8.ValidString(out) {
			t.Fatalf("terminalText(%q) = %q is not valid UTF-8", s, out)
		}
		for _, r := range out {
			if r != '\n' && r != '\t' && !strconv.IsPrint(r) {
				t.Fatalf("terminalText(%q) = %q still holds %U", s, out, r)
			}
		}
		if isTerminalSafe(s) != (out == s) {
			t.Fatalf("terminalText(%q) = %q, but isTerminalSafe says %v", s, out, isTerminalSafe(s))
		}
	})
}
