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

func TestStringHasToolName(t *testing.T) {
	s := String()
	if !strings.HasPrefix(s, "incrmit ") {
		t.Errorf("String() = %q, want prefix %q", s, "incrmit ")
	}
	if !strings.Contains(s, Version()) {
		t.Errorf("String() = %q, want it to contain Version() %q", s, Version())
	}
}

func TestVersionPrefersLdflagsValue(t *testing.T) {
	orig := version
	t.Cleanup(func() { version = orig })

	version = "9.9.9"
	if got := Version(); got != "9.9.9" {
		t.Errorf("Version() = %q, want %q", got, "9.9.9")
	}
	if got := String(); !strings.Contains(got, "incrmit 9.9.9") {
		t.Errorf("String() = %q, want it to contain %q", got, "incrmit 9.9.9")
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
