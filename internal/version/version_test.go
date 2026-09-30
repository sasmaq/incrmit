package version

import (
	"errors"
	"math"
	"strconv"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want Version
	}{
		{"simple", "1.2.3", Version{Major: 1, Minor: 2, Patch: 3}},
		{"zeros", "0.0.0", Version{Major: 0, Minor: 0, Patch: 0}},
		{"multi-digit", "10.20.30", Version{Major: 10, Minor: 20, Patch: 30}},
		{"leading-space", "  1.2.3", Version{Major: 1, Minor: 2, Patch: 3}},
		{"trailing-space", "1.2.3  ", Version{Major: 1, Minor: 2, Patch: 3}},
		{"surrounding-space", "  10.0.42  ", Version{Major: 10, Minor: 0, Patch: 42}},
		{"large", "2147483647.0.0", Version{Major: 2147483647, Minor: 0, Patch: 0}},
		{"v-prefix", "v1.2.3", Version{Major: 1, Minor: 2, Patch: 3, Prefix: "v"}},
		{"V-prefix", "V1.2.3", Version{Major: 1, Minor: 2, Patch: 3, Prefix: "V"}},
		{"v-prefix-spaced", "  v10.0.42  ", Version{Major: 10, Minor: 0, Patch: 42, Prefix: "v"}},
		{"prerelease", "1.2.3-rc.1", Version{Major: 1, Minor: 2, Patch: 3, Prerelease: "rc.1"}},
		{"prerelease-word", "1.2.3-beta", Version{Major: 1, Minor: 2, Patch: 3, Prerelease: "beta"}},
		{"prerelease-hyphenated", "1.2.3-rc-1.2", Version{Major: 1, Minor: 2, Patch: 3, Prerelease: "rc-1.2"}},
		{"prerelease-numeric", "1.2.3-0", Version{Major: 1, Minor: 2, Patch: 3, Prerelease: "0"}},
		{"build", "1.2.3+build.7", Version{Major: 1, Minor: 2, Patch: 3, Build: "build.7"}},
		{"build-leading-zeros", "1.2.3+0007", Version{Major: 1, Minor: 2, Patch: 3, Build: "0007"}},
		{"build-hyphen", "1.2.3+exp-1", Version{Major: 1, Minor: 2, Patch: 3, Build: "exp-1"}},
		{"both", "1.2.3-rc.1+build.7", Version{Major: 1, Minor: 2, Patch: 3, Prerelease: "rc.1", Build: "build.7"}},
		{"both-v-prefixed", "v2.0.0-beta.1+exp.sha.5114f85", Version{
			Major: 2, Prefix: "v", Prerelease: "beta.1", Build: "exp.sha.5114f85",
		}},
		{"prerelease-spaced", "  1.2.3-rc.1  ", Version{Major: 1, Minor: 2, Patch: 3, Prerelease: "rc.1"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(tt.in)
			if err != nil {
				t.Fatalf("Parse(%q) unexpected error: %v", tt.in, err)
			}
			if got != tt.want {
				t.Errorf("Parse(%q) = %+v, want %+v", tt.in, got, tt.want)
			}
		})
	}
}

func TestParseErrors(t *testing.T) {
	tests := []struct {
		name string
		in   string
	}{
		{"empty", ""},
		{"whitespace-only", "   "},
		{"too-few", "1.2"},
		{"too-many", "1.2.3.4"},
		{"single", "1"},
		{"non-numeric", "a.b.c"},
		{"non-numeric-patch", "1.2.x"},
		{"empty-major", ".2.3"},
		{"empty-minor", "1..3"},
		{"empty-patch", "1.2."},
		{"signed-plus", "+1.2.3"},
		{"signed-minus-major", "-1.2.3"},
		{"signed-minus-minor", "1.-2.3"},
		{"negative-patch", "1.2.-3"},
		{"inner-space", "1. 2.3"},
		{"empty-prerelease", "1.2.3-"},
		{"empty-build", "1.2.3+"},
		{"empty-prerelease-identifier", "1.2.3-rc..1"},
		{"empty-build-identifier", "1.2.3+build..7"},
		{"prerelease-leading-zero", "1.2.3-rc.01"},
		// A leading zero in the numeric core is rejected because String() could
		// not write it back: accepting "1.02.3" as 1.2.3 gave the rewriter a
		// token it could never find in the file, so the bump "succeeded" without
		// changing a byte. See FuzzParse, whose round-trip property found it.
		{"leading-zero-major", "01.2.3"},
		{"leading-zero-minor", "1.02.3"},
		{"leading-zero-patch", "1.2.03"},
		{"leading-zeros-throughout", "01.02.03"},
		{"leading-zero-v-prefixed", "v1.2.03"},
		{"prerelease-bad-char", "1.2.3-rc_1"},
		{"build-bad-char", "1.2.3+build_7"},
		{"prerelease-only", "-rc.1"},
		{"prerelease-without-patch", "1.2-rc.1"},
		{"prefix-only", "v"},
		{"prefix-no-version", "vx.y.z"},
		{"float", "1.2"},
		{"comma", "1,2,3"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, err := Parse(tt.in); err == nil {
				t.Errorf("Parse(%q) = %+v, want error", tt.in, got)
			}
		})
	}
}

func TestBumpMajor(t *testing.T) {
	tests := []struct {
		in   Version
		want Version
	}{
		{Version{Major: 1, Minor: 2, Patch: 3}, Version{Major: 2, Minor: 0, Patch: 0}},
		{Version{Major: 0, Minor: 0, Patch: 0}, Version{Major: 1, Minor: 0, Patch: 0}},
		{Version{Major: 1, Minor: 0, Patch: 0}, Version{Major: 2, Minor: 0, Patch: 0}},
		{Version{Major: 9, Minor: 9, Patch: 9}, Version{Major: 10, Minor: 0, Patch: 0}},
		{Version{Major: 1, Minor: 2, Patch: 3, Prefix: "v"}, Version{Major: 2, Minor: 0, Patch: 0, Prefix: "v"}},
	}
	for _, tt := range tests {
		if got, err := tt.in.BumpMajor(); err != nil || got != tt.want {
			t.Errorf("%v.BumpMajor() = %v, %v, want %v", tt.in, got, err, tt.want)
		}
	}
}

func TestBumpMinor(t *testing.T) {
	tests := []struct {
		in   Version
		want Version
	}{
		{Version{Major: 1, Minor: 2, Patch: 3}, Version{Major: 1, Minor: 3, Patch: 0}},
		{Version{Major: 0, Minor: 0, Patch: 0}, Version{Major: 0, Minor: 1, Patch: 0}},
		{Version{Major: 1, Minor: 0, Patch: 5}, Version{Major: 1, Minor: 1, Patch: 0}},
		{Version{Major: 2, Minor: 9, Patch: 9}, Version{Major: 2, Minor: 10, Patch: 0}},
		{Version{Major: 1, Minor: 2, Patch: 3, Prefix: "V"}, Version{Major: 1, Minor: 3, Patch: 0, Prefix: "V"}},
	}
	for _, tt := range tests {
		if got, err := tt.in.BumpMinor(); err != nil || got != tt.want {
			t.Errorf("%v.BumpMinor() = %v, %v, want %v", tt.in, got, err, tt.want)
		}
	}
}

func TestBumpPatch(t *testing.T) {
	tests := []struct {
		in   Version
		want Version
	}{
		{Version{Major: 1, Minor: 2, Patch: 3}, Version{Major: 1, Minor: 2, Patch: 4}},
		{Version{Major: 0, Minor: 0, Patch: 0}, Version{Major: 0, Minor: 0, Patch: 1}},
		{Version{Major: 1, Minor: 2, Patch: 9}, Version{Major: 1, Minor: 2, Patch: 10}},
		{Version{Major: 1, Minor: 2, Patch: 3, Prefix: "v"}, Version{Major: 1, Minor: 2, Patch: 4, Prefix: "v"}},
	}
	for _, tt := range tests {
		if got, err := tt.in.BumpPatch(); err != nil || got != tt.want {
			t.Errorf("%v.BumpPatch() = %v, %v, want %v", tt.in, got, err, tt.want)
		}
	}
}

func TestBumpDoesNotMutateReceiver(t *testing.T) {
	v := Version{Major: 1, Minor: 2, Patch: 3}
	// Each bump returns a new value; the receiver must stay untouched.
	if got, _ := v.BumpMajor(); got == v {
		t.Errorf("BumpMajor() returned the receiver unchanged: %v", got)
	}
	if got, _ := v.BumpMinor(); got == v {
		t.Errorf("BumpMinor() returned the receiver unchanged: %v", got)
	}
	if got, _ := v.BumpPatch(); got == v {
		t.Errorf("BumpPatch() returned the receiver unchanged: %v", got)
	}
	if want := (Version{Major: 1, Minor: 2, Patch: 3}); v != want {
		t.Errorf("receiver mutated: got %v, want %v", v, want)
	}
}

// Parse reads a component up to math.MaxInt, and adding one to that wraps to a
// negative number that String() writes as a token Parse rejects, so a bump
// wrote a version no later command could find (Milestone 35). Each bump that
// would carry its number past math.MaxInt is refused with an *OverflowError
// naming the part and the version instead, and returns no version at all.
func TestBumpOverflow(t *testing.T) {
	maxN := strconv.Itoa(math.MaxInt)
	sameSeries := func(v Version) (Version, error) { return v.BumpPrerelease("rc") }
	tests := []struct {
		name string
		in   string
		bump func(Version) (Version, error)
		part string
	}{
		{"major", maxN + ".2.3", Version.BumpMajor, "major component"},
		{"minor", "1." + maxN + ".3", Version.BumpMinor, "minor component"},
		{"patch", "1.2." + maxN, Version.BumpPatch, "patch component"},
		{"patch with suffixes", "v1.2." + maxN + "-rc.1+build.7", Version.BumpPatch, "patch component"},
		{"counter", "1.2.3-rc." + maxN, Version.AdvancePrerelease, "prerelease counter"},
		{"bare counter", "1.2.3-" + maxN, Version.AdvancePrerelease, "prerelease counter"},
		{"counter past an int", "1.2.3-rc.99999999999999999999", Version.AdvancePrerelease, "prerelease counter"},
		{"same series", "1.2.3-rc." + maxN, sameSeries, "prerelease counter"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in, err := Parse(tt.in)
			if err != nil {
				t.Fatalf("Parse(%q): %v", tt.in, err)
			}
			got, err := tt.bump(in)
			var overflow *OverflowError
			if !errors.As(err, &overflow) {
				t.Fatalf("got %q, %v; want an *OverflowError", got, err)
			}
			if got != (Version{}) {
				t.Errorf("returned %q alongside the error, want the zero Version", got)
			}
			if overflow.Part != tt.part || overflow.Version != in {
				t.Errorf("error = %+v, want Part %q and Version %+v", overflow, tt.part, in)
			}
			for _, want := range []string{tt.part, tt.in, maxN} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q does not name %q", err, want)
				}
			}
		})
	}
}

// Up to the ceiling everything still counts. The last value an int holds is
// reachable, a higher component bump resets a full lower one, a counter
// advances beside a full numeric core, and starting a different series does
// not count at all.
func TestBumpUpToMaxInt(t *testing.T) {
	maxN, below := strconv.Itoa(math.MaxInt), strconv.Itoa(math.MaxInt-1)
	full := maxN + "." + maxN + "." + maxN
	newSeries := func(v Version) (Version, error) { return v.BumpPrerelease("beta") }
	tests := []struct {
		name string
		in   string
		bump func(Version) (Version, error)
		want string
	}{
		{"major", below + ".2.3", Version.BumpMajor, maxN + ".0.0"},
		{"minor", "1." + below + ".3", Version.BumpMinor, "1." + maxN + ".0"},
		{"patch", "1.2." + below, Version.BumpPatch, "1.2." + maxN},
		{"counter", "1.2.3-rc." + below, Version.AdvancePrerelease, "1.2.3-rc." + maxN},
		{"minor over a full patch", "1.2." + maxN, Version.BumpMinor, "1.3.0"},
		{"major over full minor and patch", "1." + maxN + "." + maxN, Version.BumpMajor, "2.0.0"},
		{"counter beside a full core", full + "-rc.1", Version.AdvancePrerelease, full + "-rc.2"},
		{"new series over a full counter", "1.2.3-rc." + maxN, newSeries, "1.2.3-beta.1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in, err := Parse(tt.in)
			if err != nil {
				t.Fatalf("Parse(%q): %v", tt.in, err)
			}
			got, err := tt.bump(in)
			if err != nil || got.String() != tt.want {
				t.Errorf("got %q, %v; want %q", got, err, tt.want)
			}
		})
	}
}

func TestString(t *testing.T) {
	tests := []struct {
		in   Version
		want string
	}{
		{Version{Major: 1, Minor: 2, Patch: 3}, "1.2.3"},
		{Version{Major: 0, Minor: 0, Patch: 0}, "0.0.0"},
		{Version{Major: 10, Minor: 20, Patch: 30}, "10.20.30"},
		{Version{Major: 1, Minor: 2, Patch: 3, Prefix: "v"}, "v1.2.3"},
		{Version{Major: 1, Minor: 2, Patch: 3, Prefix: "V"}, "V1.2.3"},
		{Version{Major: 1, Minor: 2, Patch: 3, Prerelease: "rc.1"}, "1.2.3-rc.1"},
		{Version{Major: 1, Minor: 2, Patch: 3, Build: "build.7"}, "1.2.3+build.7"},
		{Version{Major: 1, Minor: 2, Patch: 3, Prerelease: "rc.1", Build: "build.7"}, "1.2.3-rc.1+build.7"},
		{Version{Major: 2, Prefix: "v", Prerelease: "beta.1", Build: "exp.sha.5114f85"}, "v2.0.0-beta.1+exp.sha.5114f85"},
	}
	for _, tt := range tests {
		if got := tt.in.String(); got != tt.want {
			t.Errorf("%+v.String() = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestParseStringRoundTrip(t *testing.T) {
	inputs := []string{
		"0.0.0", "1.2.3", "10.20.30", "100.0.1", "v1.2.3", "V0.1.9",
		"1.2.3-rc.1", "1.2.3-beta", "1.2.3-0", "1.2.3+build.7", "1.2.3+0007",
		"1.2.3-rc.1+build.7", "v2.0.0-beta.1+exp.sha.5114f85", "V1.0.0-alpha-1+exp-2",
	}
	for _, in := range inputs {
		v, err := Parse(in)
		if err != nil {
			t.Fatalf("Parse(%q) error: %v", in, err)
		}
		if got := v.String(); got != in {
			t.Errorf("round trip of %q = %q", in, got)
		}
	}
}

// Every component bump drops the prerelease and build sections: a patch bump of
// 1.2.3-rc.1 is 1.2.4, never 1.2.4-rc.1. Build metadata is never carried
// forward, and the "v" prefix still is.
func TestBumpDropsPrereleaseAndBuild(t *testing.T) {
	in := Version{Major: 1, Minor: 2, Patch: 3, Prefix: "v", Prerelease: "rc.1", Build: "exp.sha.5114f85"}
	tests := []struct {
		name string
		bump func(Version) (Version, error)
		want string
	}{
		{"major", Version.BumpMajor, "v2.0.0"},
		{"minor", Version.BumpMinor, "v1.3.0"},
		{"patch", Version.BumpPatch, "v1.2.4"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.bump(in)
			if err != nil {
				t.Fatal(err)
			}
			if got.Prerelease != "" || got.Build != "" {
				t.Errorf("%v kept a prerelease or build section: %+v", tt.name, got)
			}
			if got := got.String(); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRelease(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"1.2.3-rc.1", "1.2.3"},
		{"v1.2.3-rc.1+build.7", "v1.2.3"},
		{"1.2.3+build.7", "1.2.3"},
		{"1.2.3", "1.2.3"},
	}
	for _, tt := range tests {
		v, err := Parse(tt.in)
		if err != nil {
			t.Fatalf("Parse(%q): %v", tt.in, err)
		}
		if got := v.Release().String(); got != tt.want {
			t.Errorf("Parse(%q).Release() = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestPrereleaseHelpers(t *testing.T) {
	tests := []struct {
		in      string
		id      string // PrereleaseID
		start   string // StartPrerelease("rc")
		advance string // AdvancePrerelease
		bumpPre string // BumpPrerelease("rc")
		isPre   bool
	}{
		{"1.2.3", "", "1.2.3-rc.1", "1.2.3", "1.2.3-rc.1", false},
		{"1.2.3-rc.1", "rc", "1.2.3-rc.1", "1.2.3-rc.2", "1.2.3-rc.2", true},
		{"1.2.3-rc.9", "rc", "1.2.3-rc.1", "1.2.3-rc.10", "1.2.3-rc.10", true},
		{"1.2.3-rc", "rc", "1.2.3-rc.1", "1.2.3-rc.1", "1.2.3-rc.1", true},
		{"1.2.3-beta.2", "beta", "1.2.3-rc.1", "1.2.3-beta.3", "1.2.3-rc.1", true},
		{"1.2.3-1", "", "1.2.3-rc.1", "1.2.3-2", "1.2.3-rc.1", true},
		{"v1.2.3-rc.1+b.1", "rc", "v1.2.3-rc.1", "v1.2.3-rc.2", "v1.2.3-rc.2", true},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			v, err := Parse(tt.in)
			if err != nil {
				t.Fatalf("Parse(%q): %v", tt.in, err)
			}
			if got := v.PrereleaseID(); got != tt.id {
				t.Errorf("PrereleaseID() = %q, want %q", got, tt.id)
			}
			if got := v.IsPrerelease(); got != tt.isPre {
				t.Errorf("IsPrerelease() = %v, want %v", got, tt.isPre)
			}
			if got := v.StartPrerelease("rc").String(); got != tt.start {
				t.Errorf("StartPrerelease(\"rc\") = %q, want %q", got, tt.start)
			}
			if got, err := v.AdvancePrerelease(); err != nil || got.String() != tt.advance {
				t.Errorf("AdvancePrerelease() = %q, %v, want %q", got, err, tt.advance)
			}
			if got, err := v.BumpPrerelease("rc"); err != nil || got.String() != tt.bumpPre {
				t.Errorf("BumpPrerelease(\"rc\") = %q, %v, want %q", got, err, tt.bumpPre)
			}
		})
	}
}

// Build metadata describes one build, so it never survives a prerelease step.
func TestPrereleaseStepsDropBuild(t *testing.T) {
	v, err := Parse("1.2.3-rc.1+build.7")
	if err != nil {
		t.Fatal(err)
	}
	advanced, err := v.AdvancePrerelease()
	if err != nil {
		t.Fatal(err)
	}
	bumped, err := v.BumpPrerelease("beta")
	if err != nil {
		t.Fatal(err)
	}
	for name, got := range map[string]Version{
		"StartPrerelease":   v.StartPrerelease("rc"),
		"AdvancePrerelease": advanced,
		"BumpPrerelease":    bumped,
		"Release":           v.Release(),
	} {
		if got.Build != "" {
			t.Errorf("%s kept build metadata %q", name, got.Build)
		}
	}
}

func TestValidPrereleaseID(t *testing.T) {
	valid := []string{"rc", "beta", "alpha.1", "rc-1", "0", "1", "x.7.z-92"}
	for _, id := range valid {
		if err := ValidPrereleaseID(id); err != nil {
			t.Errorf("ValidPrereleaseID(%q) = %v, want nil", id, err)
		}
	}
	invalid := []string{"", "rc.", ".rc", "rc..1", "rc_1", "rc 1", "rc.01", "rc+1"}
	for _, id := range invalid {
		if err := ValidPrereleaseID(id); err == nil {
			t.Errorf("ValidPrereleaseID(%q) = nil, want an error", id)
		}
	}
}

// Compare implements semver 2.0.0 precedence, including the prerelease rules
// from the spec's own example ordering.
func TestCompare(t *testing.T) {
	// Each entry precedes the next.
	ordered := []string{
		"1.0.0-alpha", "1.0.0-alpha.1", "1.0.0-alpha.beta", "1.0.0-beta",
		"1.0.0-beta.2", "1.0.0-beta.11", "1.0.0-rc.1", "1.0.0",
		"1.0.1", "1.1.0", "2.0.0",
	}
	for i := 0; i < len(ordered); i++ {
		for j := 0; j < len(ordered); j++ {
			a, err := Parse(ordered[i])
			if err != nil {
				t.Fatal(err)
			}
			b, err := Parse(ordered[j])
			if err != nil {
				t.Fatal(err)
			}
			want := cmpInt(i, j)
			if got := Compare(a, b); got != want {
				t.Errorf("Compare(%q, %q) = %d, want %d", ordered[i], ordered[j], got, want)
			}
		}
	}
}

// Semver puts no ceiling on a numeric prerelease identifier, so one can be too
// large for an int. The fallback compares by digit count and then by text,
// which orders zero-padding-free digits exactly as the numeric comparison
// would — including against an identifier small enough to parse.
func TestCompareOversizedNumericIdentifiers(t *testing.T) {
	// Each entry precedes the next. The last two are past math.MaxInt64.
	ordered := []string{
		"1.0.0-1",
		"1.0.0-9223372036854775807",
		"1.0.0-9223372036854775808",
		"1.0.0-99999999999999999999",
	}
	for i := range ordered {
		for j := range ordered {
			a, err := Parse(ordered[i])
			if err != nil {
				t.Fatalf("Parse(%q): %v", ordered[i], err)
			}
			b, err := Parse(ordered[j])
			if err != nil {
				t.Fatalf("Parse(%q): %v", ordered[j], err)
			}
			want := cmpInt(i, j)
			if got := Compare(a, b); got != want {
				t.Errorf("Compare(%q, %q) = %d, want %d", ordered[i], ordered[j], got, want)
			}
		}
	}
}

// A numeric identifier always ranks below an alphanumeric one, whatever the
// digits are, so an oversized number does not jump the type ordering.
func TestCompareNumericRanksBelowAlphanumeric(t *testing.T) {
	numeric, err := Parse("1.0.0-99999999999999999999")
	if err != nil {
		t.Fatal(err)
	}
	alpha, err := Parse("1.0.0-alpha")
	if err != nil {
		t.Fatal(err)
	}
	if got := Compare(numeric, alpha); got != -1 {
		t.Errorf("Compare(numeric, alpha) = %d, want -1", got)
	}
	if got := Compare(alpha, numeric); got != 1 {
		t.Errorf("Compare(alpha, numeric) = %d, want 1", got)
	}
}

// Compare takes a struct, so it can be handed a prerelease that Parse would
// have rejected. An empty identifier counts as alphanumeric rather than
// panicking or being read as the number zero.
func TestCompareHandsUnparseablePrereleaseGracefully(t *testing.T) {
	empty := Version{Major: 1, Prerelease: "1."}
	numeric := Version{Major: 1, Prerelease: "1.0"}
	if got := Compare(empty, numeric); got != 1 {
		t.Errorf("Compare(%q, %q) = %d, want 1 (empty sorts as alphanumeric)", empty.Prerelease, numeric.Prerelease, got)
	}
}

// Neither the "v" prefix nor build metadata affects precedence.
func TestCompareIgnoresPrefixAndBuild(t *testing.T) {
	equal := []string{"1.2.3", "v1.2.3", "V1.2.3", "1.2.3+build.7", "v1.2.3+exp.sha.5114f85"}
	base, err := Parse("1.2.3")
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range equal {
		v, err := Parse(s)
		if err != nil {
			t.Fatal(err)
		}
		if got := Compare(base, v); got != 0 {
			t.Errorf("Compare(1.2.3, %q) = %d, want 0", s, got)
		}
	}
}
