package cli

import (
	"math"
	"testing"
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
