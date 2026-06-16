package version

import "testing"

func TestParse(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want Version
	}{
		{"simple", "1.2.3", Version{1, 2, 3}},
		{"zeros", "0.0.0", Version{0, 0, 0}},
		{"multi-digit", "10.20.30", Version{10, 20, 30}},
		{"leading-space", "  1.2.3", Version{1, 2, 3}},
		{"trailing-space", "1.2.3  ", Version{1, 2, 3}},
		{"surrounding-space", "  10.0.42  ", Version{10, 0, 42}},
		{"leading-zeros", "01.02.03", Version{1, 2, 3}},
		{"large", "2147483647.0.0", Version{2147483647, 0, 0}},
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
		{"trailing-text", "1.2.3-rc.1"},
		{"build-metadata", "1.2.3+build.5"},
		{"v-prefix", "v1.2.3"},
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
		{Version{1, 2, 3}, Version{2, 0, 0}},
		{Version{0, 0, 0}, Version{1, 0, 0}},
		{Version{1, 0, 0}, Version{2, 0, 0}},
		{Version{9, 9, 9}, Version{10, 0, 0}},
	}
	for _, tt := range tests {
		if got := tt.in.BumpMajor(); got != tt.want {
			t.Errorf("%v.BumpMajor() = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestBumpMinor(t *testing.T) {
	tests := []struct {
		in   Version
		want Version
	}{
		{Version{1, 2, 3}, Version{1, 3, 0}},
		{Version{0, 0, 0}, Version{0, 1, 0}},
		{Version{1, 0, 5}, Version{1, 1, 0}},
		{Version{2, 9, 9}, Version{2, 10, 0}},
	}
	for _, tt := range tests {
		if got := tt.in.BumpMinor(); got != tt.want {
			t.Errorf("%v.BumpMinor() = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestBumpPatch(t *testing.T) {
	tests := []struct {
		in   Version
		want Version
	}{
		{Version{1, 2, 3}, Version{1, 2, 4}},
		{Version{0, 0, 0}, Version{0, 0, 1}},
		{Version{1, 2, 9}, Version{1, 2, 10}},
	}
	for _, tt := range tests {
		if got := tt.in.BumpPatch(); got != tt.want {
			t.Errorf("%v.BumpPatch() = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestBumpDoesNotMutateReceiver(t *testing.T) {
	v := Version{1, 2, 3}
	v.BumpMajor()
	v.BumpMinor()
	v.BumpPatch()
	if want := (Version{1, 2, 3}); v != want {
		t.Errorf("receiver mutated: got %v, want %v", v, want)
	}
}

func TestString(t *testing.T) {
	tests := []struct {
		in   Version
		want string
	}{
		{Version{1, 2, 3}, "1.2.3"},
		{Version{0, 0, 0}, "0.0.0"},
		{Version{10, 20, 30}, "10.20.30"},
	}
	for _, tt := range tests {
		if got := tt.in.String(); got != tt.want {
			t.Errorf("%+v.String() = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestParseStringRoundTrip(t *testing.T) {
	inputs := []string{"0.0.0", "1.2.3", "10.20.30", "100.0.1"}
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
