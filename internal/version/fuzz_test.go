package version

import (
	"strings"
	"testing"
)

// seedTokens are the shapes the table tests already know matter: valid tokens,
// near misses that must be rejected (rev1.2.3, IPv4, two components), and the
// boundaries of the prerelease and build grammars. Feeding them to both targets
// starts the fuzzer at the known edges rather than making it rediscover them.
var seedTokens = []string{
	"1.2.3",
	"v1.2.3",
	"V0.1.9",
	"0.0.0",
	"10.20.30",
	"1.2.3-rc.1",
	"1.2.3-0",
	"1.2.3-beta",
	"1.2.3+build.7",
	"1.2.3+0007",
	"v2.0.0-beta.1+exp.sha.5114f85",
	"1.2.3+exp-1",
	// Near misses: each of these must be rejected, and the fuzzer should know
	// where the cliff is before it starts mutating.
	"",
	" ",
	"rev1.2.3",
	"192.168.1.1",
	"3.9",
	"1.2",
	"1.2.3.4",
	"01.02.03",
	"1.2.03",
	"1.2.3-rc.01",
	"1.2.3-",
	"1.2.3+",
	"1.2.3-rc..1",
	"1.2.3-rc_1",
	"v",
	"-1.2.3",
	"+1.2.3",
	"1.-2.3",
	"9999999999999999999999.1.1",
}

// FuzzParse pins the two properties every caller of Parse relies on: it never
// panics on arbitrary input, and whatever it accepts round-trips through
// String() back to the identical token.
//
// The round trip is not cosmetic. files.SetKnownVersions locates an occurrence
// by comparing the bytes in the file against pin.String(), so a token Parse
// accepts but String() re-spells differently is a token the rewriter can never
// match: the bump reports success and writes nothing. Leading zeros in the
// numeric core were exactly that hole, which is why Parse now rejects them.
func FuzzParse(f *testing.F) {
	for _, s := range seedTokens {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, s string) {
		v, err := Parse(s)
		if err != nil {
			return
		}
		// Parse ignores surrounding whitespace, so the token it accepted is the
		// trimmed string; that is what must come back out.
		want := strings.TrimSpace(s)
		if got := v.String(); got != want {
			t.Errorf("Parse(%q).String() = %q, want %q", s, got, want)
		}
		// A second pass must agree with the first, or the rewriter's and the
		// config's readings of one token could differ.
		again, err := Parse(v.String())
		if err != nil {
			t.Fatalf("Parse(%q) accepted but Parse(%q) (its own String) failed: %v", s, v.String(), err)
		}
		if again != v {
			t.Errorf("Parse(%q) = %#v, but reparsing its String() gives %#v", s, v, again)
		}

		// The two halves of the package must agree on where a token ends.
		// Parse is what the config is read through and FindTokens is what the
		// file is read through, so a version Parse accepts whole but FindTokens
		// only finds part of is a pin that can never be located in the file it
		// describes — the bump then reports success having matched nothing. A
		// build identifier ending in "-" was exactly that: legal semver, but
		// cut short by the scanner's word boundary.
		locs := FindTokens([]byte(want))
		if len(locs) != 1 || locs[0][0] != 0 || locs[0][1] != len(want) {
			t.Errorf("Parse accepts %q whole, but FindTokens reports %v, so the scanner cannot locate it", want, locs)
		}
	})
}

// FuzzFindTokens checks the shape of the ranges FindTokens returns, which is
// what the rewriter assumes when it walks them in a single pass: they are in
// bounds, strictly ordered, and non-overlapping, so writing data[prev:start]
// for each range can neither panic nor emit a byte twice.
//
// The fourth property is not that every range parses — FindTokens deliberately
// reports candidates (an IPv4 address, a two-component number) for Parse to
// reject. It is that a range Parse *accepts* spans exactly the bytes
// Version.String() produces, because that equality is how matchAt decides an
// occurrence is the pinned version.
func FuzzFindTokens(f *testing.F) {
	for _, s := range seedTokens {
		f.Add([]byte(s))
	}
	f.Add([]byte("version = \"1.2.3\"\n"))
	f.Add([]byte("incrmit-1.2.3-linux-amd64.tar.gz"))
	f.Add([]byte("from 1.2.3 to v2.0.0-rc.1"))
	f.Add([]byte("\x00\xff1.2.3\xff"))

	f.Fuzz(func(t *testing.T, data []byte) {
		prev := 0
		for i, loc := range FindTokens(data) {
			if len(loc) != 2 {
				t.Fatalf("range %d = %v, want a [start, end) pair", i, loc)
			}
			start, end := loc[0], loc[1]
			if start < 0 || end > len(data) {
				t.Fatalf("range %d = [%d, %d) is out of bounds for %d bytes", i, start, end, len(data))
			}
			if start >= end {
				t.Fatalf("range %d = [%d, %d) is empty or inverted", i, start, end)
			}
			if start < prev {
				t.Fatalf("range %d = [%d, %d) overlaps or precedes the previous range ending at %d", i, start, end, prev)
			}
			prev = end

			tok := string(data[start:end])
			v, err := Parse(tok)
			if err != nil {
				continue
			}
			if got := v.String(); got != tok {
				t.Errorf("token %q at [%d, %d) parses but re-spells as %q, so the rewriter can never match it", tok, start, end, got)
			}
		}
	})
}
