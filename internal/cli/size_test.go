package cli

import (
	"flag"
	"io"
	"strings"
	"testing"
)

// A size may be written as a plain byte count or with a unit suffix, in any
// case, with or without a space before the unit.
func TestParseSize(t *testing.T) {
	tests := []struct {
		in   string
		want int64
	}{
		{"0", 0},
		{"1", 1},
		{"1048576", 1 << 20},
		{"512B", 512},
		{"1K", 1 << 10},
		{"1KB", 1000},
		{"1KiB", 1 << 10},
		{"32MiB", 32 << 20},
		{"32mib", 32 << 20},
		{"32 MiB", 32 << 20},
		{"  8M  ", 8 << 20},
		{"2GB", 2 * 1000 * 1000 * 1000},
		{"2GiB", 2 << 30},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, err := parseSize(tt.in)
			if err != nil {
				t.Fatalf("parseSize(%q): %v", tt.in, err)
			}
			if got != tt.want {
				t.Errorf("parseSize(%q) = %d, want %d", tt.in, got, tt.want)
			}
		})
	}
}

// A value that is not a size, is negative, or would overflow int64 is rejected
// rather than silently turned into some other limit.
func TestParseSizeRejectsBadValues(t *testing.T) {
	for _, in := range []string{"", "   ", "big", "1.5MB", "32MiBs", "-1", "-4KB", "9223372036854775807MiB"} {
		t.Run(in, func(t *testing.T) {
			if got, err := parseSize(in); err == nil {
				t.Errorf("parseSize(%q) = %d, want an error", in, got)
			}
		})
	}
}

// A limit is reported back in the units it was most likely written in, so an
// error message reads the same way as the flag the user passed.
func TestFormatSize(t *testing.T) {
	tests := []struct {
		in   int64
		want string
	}{
		{0, "0 bytes"},
		{512, "512 bytes"},
		{1000, "1000 bytes"},
		{1 << 10, "1KiB"},
		{32 << 20, "32MiB"},
		{2 << 30, "2GiB"},
		{(32 << 20) + 1, "33554433 bytes"},
	}
	for _, tt := range tests {
		if got := formatSize(tt.in); got != tt.want {
			t.Errorf("formatSize(%d) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// Both spellings of the size flag write to the same target, and the default
// applies when neither is passed.
func TestMaxFileSizeVarLongAndShort(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want int64
	}{
		{"default", nil, 32 << 20},
		{"long", []string{"--max-file-size", "1KiB"}, 1 << 10},
		{"short", []string{"-s", "1KiB"}, 1 << 10},
		{"unlimited", []string{"-s", "0"}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got int64
			fs := flag.NewFlagSet("test", flag.ContinueOnError)
			fs.SetOutput(io.Discard)
			maxFileSizeVar(fs, &got, 32<<20, "cap")
			if err := fs.Parse(tt.args); err != nil {
				t.Fatalf("Parse(%v): %v", tt.args, err)
			}
			if got != tt.want {
				t.Errorf("value = %d, want %d", got, tt.want)
			}
		})
	}
}

// A bad size is a parse error naming the offending value, so the command exits
// with a usage error instead of running with a surprising limit.
func TestMaxFileSizeVarRejectsBadValue(t *testing.T) {
	var got int64
	var out strings.Builder
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.SetOutput(&out)
	fs.Usage = func() {}
	maxFileSizeVar(fs, &got, 32<<20, "cap")

	if err := fs.Parse([]string{"--max-file-size", "huge"}); err == nil {
		t.Fatal("Parse succeeded, want an error")
	}
	if !strings.Contains(out.String(), `"huge"`) {
		t.Errorf("error output = %q, want it to name the bad value", out.String())
	}
}
