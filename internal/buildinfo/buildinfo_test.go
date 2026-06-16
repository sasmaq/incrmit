package buildinfo

import (
	"strings"
	"testing"
)

func TestVersionNonEmpty(t *testing.T) {
	if got := Version(); got == "" {
		t.Error("Version() returned empty string")
	}
}

func TestStringIsToolNameAndVersionOnly(t *testing.T) {
	s := String()
	want := "incrmit " + Version()
	if s != want {
		t.Errorf("String() = %q, want exactly %q", s, want)
	}
	// No build metadata (commit/build date) should be appended.
	if strings.ContainsAny(s, "()") {
		t.Errorf("String() = %q, want no build-metadata suffix", s)
	}
}

func TestVersionPrefersLdflagsValue(t *testing.T) {
	orig := version
	t.Cleanup(func() { version = orig })

	version = "9.9.9"
	if got := Version(); got != "9.9.9" {
		t.Errorf("Version() = %q, want %q", got, "9.9.9")
	}
	if got := String(); got != "incrmit 9.9.9" {
		t.Errorf("String() = %q, want %q", got, "incrmit 9.9.9")
	}
}

func TestVersionFallsBackToDev(t *testing.T) {
	orig := version
	t.Cleanup(func() { version = orig })

	// An empty injected value should fall back (to module version or "dev"),
	// never to the empty string.
	version = ""
	if got := Version(); got == "" {
		t.Error("Version() returned empty string on fallback")
	}
}
