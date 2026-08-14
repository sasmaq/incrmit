package version

import (
	"reflect"
	"testing"
)

// tokens returns the text of every candidate token FindTokens locates in s.
func tokens(s string) []string {
	data := []byte(s)
	var out []string
	for _, loc := range FindTokens(data) {
		out = append(out, string(data[loc[0]:loc[1]]))
	}
	return out
}

func TestFindTokens(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []string
	}{
		{"bare", "1.2.3", []string{"1.2.3"}},
		{"quoted", `version = "1.2.3"`, []string{"1.2.3"}},
		{"v prefix", "tag: v1.2.3", []string{"v1.2.3"}},
		{"prerelease", `version = "1.2.3-rc.1"`, []string{"1.2.3-rc.1"}},
		{"build", "ver=1.2.3+build.7", []string{"1.2.3+build.7"}},
		{"both", "v = v2.0.0-beta.1+exp.sha.5114f85", []string{"v2.0.0-beta.1+exp.sha.5114f85"}},
		{"end of sentence", "Ships as 1.2.3-rc.1.", []string{"1.2.3-rc.1"}},
		{"markdown bullet", "- 1.2.3-rc.1", []string{"1.2.3-rc.1"}},
		{"leading hyphen at start", "-1.2.3-rc.1", []string{"1.2.3-rc.1"}},
		{"double hyphen", "--tag=-1.2.3-rc.1", []string{"1.2.3-rc.1"}},
		// "_" is a word character, so no boundary falls before it: the suffix is
		// not matched, and a version welded to a word is not matched at all.
		{"underscore suffix", "1.2.3-rc_1", []string{"1.2.3"}},
		{"underscore prefix", "incrmit_1.2.3_amd64.deb", nil},
		// FindTokens reports candidates; Parse is what rejects them. The prefix
		// in "rev1.2.3" is not at a word boundary, so what survives is the "2.3"
		// after the dot — two components, and rejected downstream.
		{"embedded word", "rev1.2.3", []string{"2.3"}},
		{"ipv4", "192.168.1.1", []string{"192.168.1.1"}},
		{"two components", "python 3.9", []string{"3.9"}},
		{"several", "from 1.2.3 to v2.0.0-rc.1", []string{"1.2.3", "v2.0.0-rc.1"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tokens(tt.in); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("FindTokens(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// A version welded into a longer hyphen-joined word — a release filename or a
// download URL — keeps only its numeric core. A hyphen is a legal prerelease
// character, so "incrmit-1.2.3-linux-amd64.tar.gz" would otherwise be read as
// 1.2.3 with the prerelease "linux-amd64.tar.gz", and rewriting that token would
// swallow the rest of the filename.
func TestFindTokensGuardsAgainstFilenames(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []string
	}{
		{"tarball", "incrmit-1.2.3-linux-amd64.tar.gz", []string{"1.2.3"}},
		{"zip", "incrmit-1.2.3-windows-amd64.zip", []string{"1.2.3"}},
		{"rpm", "incrmit-1.2.3-1.x86_64.rpm", []string{"1.2.3"}},
		{"v-prefixed filename", "incrmit-v1.2.3-darwin-arm64.tar.gz", []string{"v1.2.3"}},
		{"download url", "https://x/releases/download/v1.2.3/incrmit-1.2.3-linux-amd64.tar.gz", []string{"v1.2.3", "1.2.3"}},
		{"single letter prefix", "x-1.2.3-linux", []string{"1.2.3"}},
		{"digit prefix", "go1-1.2.3-linux", []string{"1.2.3"}},
		// A genuine prerelease inside a filename loses its suffix too. That is
		// the deliberate cost of the guard: the bump is not semver-correct, but
		// it rewrites only the numbers and leaves the filename intact.
		{"prerelease in filename", "app-1.2.3-rc.1.zip", []string{"1.2.3"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tokens(tt.in); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("FindTokens(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// The guard looks at what precedes the version, not at the suffix, so a
// prerelease that stands on its own is unaffected however it is delimited.
func TestFindTokensKeepsStandaloneSuffixes(t *testing.T) {
	for _, in := range []string{
		"1.2.3-rc.1",
		" 1.2.3-rc.1 ",
		"\t1.2.3-rc.1\n",
		`"1.2.3-rc.1"`,
		"'1.2.3-rc.1'",
		"=1.2.3-rc.1",
		"(1.2.3-rc.1)",
		"[1.2.3-rc.1]",
		"v=v1.2.3-rc.1",
		"version:1.2.3-rc.1",
	} {
		got := tokens(in)
		if len(got) != 1 {
			t.Errorf("FindTokens(%q) = %q, want one token", in, got)
			continue
		}
		if want := "1.2.3-rc.1"; got[0] != want && got[0] != "v"+want {
			t.Errorf("FindTokens(%q) = %q, want the full token", in, got)
		}
	}
}

// Every token FindTokens reports must be a byte range of the input, so callers
// can rewrite it in place.
func TestFindTokensRangesAreExact(t *testing.T) {
	data := []byte("a 1.2.3-rc.1 b incrmit-1.2.3-linux-amd64.tar.gz c")
	for _, loc := range FindTokens(data) {
		if loc[0] < 0 || loc[1] > len(data) || loc[0] >= loc[1] {
			t.Fatalf("range %v is not inside the input (len %d)", loc, len(data))
		}
		if _, err := Parse(string(data[loc[0]:loc[1]])); err != nil {
			t.Errorf("token %q does not parse: %v", data[loc[0]:loc[1]], err)
		}
	}
}
